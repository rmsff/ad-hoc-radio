package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func lateEntryEvidence(cfg LateEntryTimingResyncConfig, report LateEntryTimingResyncReport, eventHash string) ScenarioEvidence {
	return ScenarioEvidence{
		SchemaVersion: ScenarioEvidenceSchemaVersion,
		StoryID:       LateEntryTimingResyncStoryID,
		ScenarioID:    cfg.ScenarioID,
		EvidenceType:  LateEntryTimingResyncEvidenceType,
		EvidenceGrade: EvidenceGradeSyntheticProxy,
		Seed:          cfg.Seed,
		Determinism: ScenarioEvidenceDeterminism{
			EventHash:     eventHash,
			HashAlgorithm: ScenarioEvidenceHashAlgorithm,
			Serialization: ScenarioEvidenceSerialization,
		},
		AssumptionFlags: []string{
			"scenario_metadata_timing_hint",
			"no_new_wire_control_frame",
			"not_real_rf_synchronization",
			"not_phy_modem_sync",
			"missed_call_start_is_late",
			"report_only_late_entry",
		},
		HardwareClaimAllowed: false,
		Outcome:              report.Outcome,
		DegradeReasons:       cloneLateEntryStrings(report.DegradeReasons),
	}
}

func lateEntryHash(report LateEntryTimingResyncReport) (string, error) {
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
