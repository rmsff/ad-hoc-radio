package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

func finalizeRelayTreeStressEvidence(report *RelayTreeStressMatrixReport) error {
	degradeReasons := make([]string, 0, len(report.ProfileSummaries))
	hasGo := false
	hasNonGo := false
	for _, summary := range report.ProfileSummaries {
		if summary.Verdict == RelayTreeProfileGo {
			hasGo = true
		} else {
			hasNonGo = true
			degradeReasons = append(degradeReasons, summary.ProfileID+":"+string(summary.Verdict))
		}
	}
	outcome := "pass"
	switch {
	case !hasGo || report.ArchitectureVerdict == RelayTreeArchitectureReviewRequired:
		outcome = "fail"
	case hasNonGo:
		outcome = "degraded"
	}
	report.Evidence = ScenarioEvidence{
		SchemaVersion: ScenarioEvidenceSchemaVersion,
		StoryID:       RelayTreeStressStoryID,
		ScenarioID:    RelayTreeStressScenarioID,
		EvidenceType:  RelayTreeStressType,
		EvidenceGrade: EvidenceGradeSyntheticProxy,
		Seed:          42,
		Determinism: ScenarioEvidenceDeterminism{
			HashAlgorithm: ScenarioEvidenceHashAlgorithm,
			Serialization: ScenarioEvidenceSerialization,
		},
		ArtifactRefs: []ScenarioEvidenceArtifact{},
		AssumptionFlags: []string{
			"backend_assessments_are_profile_scoped",
			"bounded_30_case_matrix",
			"ideal_simulated_clock",
			"serialized_single_frequency_schedule",
			"simulator_evidence_only",
		},
		HardwareClaimAllowed: false,
		Outcome:              outcome,
		DegradeReasons:       sortedUniqueStrings(degradeReasons),
	}
	hash, err := relayTreeStressMatrixHash(*report)
	if err != nil {
		return err
	}
	report.Evidence.Determinism.EventHash = hash
	if err := ValidateScenarioEvidence(report.Evidence); err != nil {
		return fmt.Errorf("relay-tree stress matrix: evidence: %w", err)
	}
	return nil
}

func relayTreeStressMatrixHash(report RelayTreeStressMatrixReport) (string, error) {
	hashable := cloneRelayTreeStressMatrixReport(report)
	hashable.Evidence.Determinism.EventHash = ""
	encoded, err := json.Marshal(hashable)
	if err != nil {
		return "", fmt.Errorf("relay-tree stress matrix: encode deterministic evidence: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
