package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func portableHardwareEvidence(cfg PortableHardwareReadinessConfig, report PortableHardwareReadinessReport, eventHash string) ScenarioEvidence {
	return ScenarioEvidence{
		SchemaVersion: ScenarioEvidenceSchemaVersion,
		StoryID:       PortableHardwareReadinessStoryID,
		ScenarioID:    cfg.ScenarioID,
		EvidenceType:  PortableHardwareReadinessEvidenceType,
		EvidenceGrade: EvidenceGradeSyntheticProxy,
		Seed:          cfg.Seed,
		Determinism: ScenarioEvidenceDeterminism{
			EventHash:     eventHash,
			HashAlgorithm: ScenarioEvidenceHashAlgorithm,
			Serialization: ScenarioEvidenceSerialization,
		},
		AssumptionFlags: []string{
			"readiness_checklist_only",
			"not_hardware_measurement",
			"no_rf_transmission",
			"no_field_authorization",
			"report_only_hardware_gate",
			"separate_go_no_go_required",
		},
		HardwareClaimAllowed: false,
		Outcome:              report.Outcome,
		DegradeReasons:       clonePortableHardwareStrings(report.DegradeReasons),
	}
}

func portableHardwareHash(report PortableHardwareReadinessReport) (string, error) {
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
