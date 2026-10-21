package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func groupFloorPTTEvidence(cfg GroupFloorPTTContentionConfig, report GroupFloorPTTContentionReport, eventHash string) ScenarioEvidence {
	return ScenarioEvidence{
		SchemaVersion: ScenarioEvidenceSchemaVersion,
		StoryID:       GroupFloorPTTContentionStoryID,
		ScenarioID:    cfg.ScenarioID,
		EvidenceType:  GroupFloorPTTContentionEvidenceType,
		EvidenceGrade: EvidenceGradeSyntheticProxy,
		Seed:          cfg.Seed,
		Determinism: ScenarioEvidenceDeterminism{
			EventHash:     eventHash,
			HashAlgorithm: ScenarioEvidenceHashAlgorithm,
			Serialization: ScenarioEvidenceSerialization,
		},
		AssumptionFlags: []string{
			"deterministic_floor_arbitration_proxy",
			"not_real_carrier_sense",
			"not_real_rf_capture_effects",
			"no_queueing_or_preemption",
			"report_only_ptt_contention",
		},
		HardwareClaimAllowed: false,
		Outcome:              report.Outcome,
		DegradeReasons:       cloneGroupFloorPTTStrings(report.DegradeReasons),
	}
}

func groupFloorPTTHash(report GroupFloorPTTContentionReport) (string, error) {
	hashable := report
	hashable.EventHash = ""
	hashable.Evidence.Determinism.EventHash = ""
	b, err := json.Marshal(hashable)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
