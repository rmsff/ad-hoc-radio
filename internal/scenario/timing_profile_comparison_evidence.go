package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

type timingRecommendationScore struct {
	fitRows         int
	driftCollisions int
	delayMS         int64
}

func timingProfileRecommendation(rows []TimingProfileComparisonRow) string {
	scores := make(map[string]timingRecommendationScore, len(timingComparisonStrategyOrder))
	for _, row := range rows {
		if !row.ProfileFit {
			continue
		}
		score := scores[row.TimingStrategy]
		score.fitRows++
		score.driftCollisions += row.DriftCollisionCount
		score.delayMS += row.PTTAccessDelayMS + row.LateEntryAcquisitionMS + row.LeaderReacquireMS
		scores[row.TimingStrategy] = score
	}
	source := scores[string(timing.TimingStrategySourceLed)]
	leader := scores[string(timing.TimingStrategyGroupLeader)]
	comparison := compareTimingRecommendationScores(source, leader)
	switch {
	case comparison > 0:
		return string(timing.TimingStrategySourceLed) + "_simulator_candidate"
	case comparison < 0:
		return string(timing.TimingStrategyGroupLeader) + "_simulator_candidate"
	default:
		return "insufficient_evidence"
	}
}

func compareTimingRecommendationScores(a, b timingRecommendationScore) int {
	if a.fitRows != b.fitRows {
		if a.fitRows > b.fitRows {
			return 1
		}
		return -1
	}
	if a.driftCollisions != b.driftCollisions {
		if a.driftCollisions < b.driftCollisions {
			return 1
		}
		return -1
	}
	if a.delayMS != b.delayMS {
		if a.delayMS < b.delayMS {
			return 1
		}
		return -1
	}
	return 0
}

func finalizeTimingProfileEvidence(report *TimingProfileComparisonReport) error {
	outcome := "pass"
	degradeReasons := []string{}
	for _, row := range report.Rows {
		if !row.ProfileFit || row.RuntimeOutcome != TimingRuntimePass {
			outcome = "degraded"
			degradeReasons = []string{"profile_no_fit_or_runtime_not_pass"}
			break
		}
	}
	report.Evidence = ScenarioEvidence{
		SchemaVersion: ScenarioEvidenceSchemaVersion,
		StoryID:       TimingProfileComparisonStoryID,
		ScenarioID:    TimingProfileComparisonScenarioID,
		EvidenceType:  TimingProfileComparisonType,
		EvidenceGrade: EvidenceGradeSyntheticProxy,
		Seed:          0,
		Determinism: ScenarioEvidenceDeterminism{
			HashAlgorithm: ScenarioEvidenceHashAlgorithm,
			Serialization: ScenarioEvidenceSerialization,
		},
		ArtifactRefs: []ScenarioEvidenceArtifact{},
		AssumptionFlags: []string{
			"ideal_simulated_clock",
			"no_ambe_dependency",
			"no_dmr_compatibility_claim",
			"no_ota_control_airtime_modeled",
			"not_real_rf_synchronization",
			"report_only_timing_comparison",
		},
		HardwareClaimAllowed: false,
		Outcome:              outcome,
		DegradeReasons:       degradeReasons,
	}
	hash, err := timingProfileComparisonHash(*report)
	if err != nil {
		return err
	}
	report.Evidence.Determinism.EventHash = hash
	return ValidateScenarioEvidence(report.Evidence)
}

func timingProfileComparisonHash(report TimingProfileComparisonReport) (string, error) {
	hashable := cloneTimingProfileComparisonReport(report)
	hashable.Evidence.Determinism.EventHash = ""
	encoded, err := json.Marshal(hashable)
	if err != nil {
		return "", fmt.Errorf("timing profile comparison: encode deterministic evidence: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
