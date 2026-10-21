package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func delayedGroupConversationEvidence(cfg DelayedGroupConversationConfig, report DelayedGroupConversationReport, eventHash string) ScenarioEvidence {
	return ScenarioEvidence{
		SchemaVersion: ScenarioEvidenceSchemaVersion,
		StoryID:       DelayedGroupConversationStoryID,
		ScenarioID:    cfg.ScenarioID,
		EvidenceType:  DelayedGroupConversationEvidenceType,
		EvidenceGrade: EvidenceGradeSyntheticProxy,
		Seed:          cfg.Seed,
		Determinism: ScenarioEvidenceDeterminism{
			EventHash:     eventHash,
			HashAlgorithm: ScenarioEvidenceHashAlgorithm,
			Serialization: ScenarioEvidenceSerialization,
		},
		AssumptionFlags: []string{
			"deterministic_timing_proxy",
			"not_subjective_mos",
			"not_real_codec",
			"not_real_fec",
			"report_only_group_conversation",
			"voice_quality_profiles_have_residual_degradation",
		},
		HardwareClaimAllowed: false,
		Outcome:              report.Outcome,
		DegradeReasons:       cloneDelayedGroupConversationStrings(report.DegradeReasons),
	}
}

func delayedGroupConversationHash(report DelayedGroupConversationReport) (string, error) {
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
