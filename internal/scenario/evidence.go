package scenario

import (
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	ScenarioEvidenceSchemaVersion = "scenario-evidence-v1"
	ScenarioEvidenceHashAlgorithm = "sha256"
	ScenarioEvidenceSerialization = "canonical-json-v1"

	EvidenceGradeSyntheticProxy  = "synthetic_proxy"
	EvidenceGradeFixtureMetadata = "fixture_metadata"
	EvidenceGradeFixtureArtifact = "fixture_artifact"
	EvidenceGradeBenchMeasured   = "bench_measured"
)

type ScenarioEvidence struct {
	SchemaVersion        string                      `json:"schema_version"`
	StoryID              string                      `json:"story_id"`
	ScenarioID           string                      `json:"scenario_id"`
	EvidenceType         string                      `json:"evidence_type"`
	EvidenceGrade        string                      `json:"evidence_grade"`
	Seed                 uint64                      `json:"seed"`
	Determinism          ScenarioEvidenceDeterminism `json:"determinism"`
	ArtifactRefs         []ScenarioEvidenceArtifact  `json:"artifact_refs"`
	AssumptionFlags      []string                    `json:"assumption_flags"`
	HardwareClaimAllowed bool                        `json:"hardware_claim_allowed"`
	Outcome              string                      `json:"outcome"`
	DegradeReasons       []string                    `json:"degrade_reasons"`
}

type ScenarioEvidenceDeterminism struct {
	EventHash     string `json:"event_hash"`
	HashAlgorithm string `json:"hash_algorithm"`
	Serialization string `json:"serialization"`
}

type ScenarioEvidenceArtifact struct {
	ArtifactID string `json:"artifact_id"`
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	Role       string `json:"role"`
}

func ValidateScenarioEvidence(e ScenarioEvidence) error {
	if strings.TrimSpace(e.SchemaVersion) == "" {
		return fmt.Errorf("scenario evidence: schema_version is required")
	}
	if e.SchemaVersion != ScenarioEvidenceSchemaVersion {
		return fmt.Errorf("scenario evidence: unsupported schema_version %q", e.SchemaVersion)
	}
	if strings.TrimSpace(e.StoryID) == "" {
		return fmt.Errorf("scenario evidence: story_id is required")
	}
	if strings.TrimSpace(e.ScenarioID) == "" {
		return fmt.Errorf("scenario evidence: scenario_id is required")
	}
	if strings.TrimSpace(e.EvidenceType) == "" {
		return fmt.Errorf("scenario evidence: evidence_type is required")
	}
	if !validScenarioEvidenceGrade(e.EvidenceGrade) {
		return fmt.Errorf("scenario evidence: unsupported evidence_grade %q", e.EvidenceGrade)
	}
	if strings.TrimSpace(e.Outcome) == "" {
		return fmt.Errorf("scenario evidence: outcome is required")
	}
	if err := validateScenarioEvidenceDeterminism(e.Determinism); err != nil {
		return err
	}
	if err := validateScenarioEvidenceArtifacts(e); err != nil {
		return err
	}
	if e.HardwareClaimAllowed && e.EvidenceGrade != EvidenceGradeBenchMeasured {
		return fmt.Errorf("scenario evidence: hardware_claim_allowed requires evidence_grade %q", EvidenceGradeBenchMeasured)
	}
	return nil
}

func validScenarioEvidenceGrade(grade string) bool {
	switch grade {
	case EvidenceGradeSyntheticProxy,
		EvidenceGradeFixtureMetadata,
		EvidenceGradeFixtureArtifact,
		EvidenceGradeBenchMeasured:
		return true
	default:
		return false
	}
}

func validateScenarioEvidenceDeterminism(d ScenarioEvidenceDeterminism) error {
	if strings.TrimSpace(d.EventHash) == "" {
		return fmt.Errorf("scenario evidence: determinism.event_hash is required")
	}
	if !isLowerHexSHA256(d.EventHash) {
		return fmt.Errorf("scenario evidence: determinism.event_hash must be a lowercase sha256 hex digest")
	}
	if d.HashAlgorithm != ScenarioEvidenceHashAlgorithm {
		return fmt.Errorf("scenario evidence: determinism.hash_algorithm must be %q", ScenarioEvidenceHashAlgorithm)
	}
	if d.Serialization != ScenarioEvidenceSerialization {
		return fmt.Errorf("scenario evidence: determinism.serialization must be %q", ScenarioEvidenceSerialization)
	}
	return nil
}

func validateScenarioEvidenceArtifacts(e ScenarioEvidence) error {
	requiresHashedArtifact := e.EvidenceGrade == EvidenceGradeFixtureArtifact || e.EvidenceGrade == EvidenceGradeBenchMeasured
	hasHashedArtifact := false
	for i, artifact := range e.ArtifactRefs {
		if strings.TrimSpace(artifact.ArtifactID) == "" {
			return fmt.Errorf("scenario evidence: artifact_refs[%d].artifact_id is required", i)
		}
		if strings.TrimSpace(artifact.Role) == "" {
			return fmt.Errorf("scenario evidence: artifact_refs[%d].role is required", i)
		}
		if artifact.SHA256 != "" {
			if strings.TrimSpace(artifact.Path) == "" {
				return fmt.Errorf("scenario evidence: artifact_refs[%d].path is required when sha256 is present", i)
			}
			if !isLowerHexSHA256(artifact.SHA256) {
				return fmt.Errorf("scenario evidence: artifact_refs[%d].sha256 must be a lowercase sha256 hex digest", i)
			}
			hasHashedArtifact = true
		}
	}
	if requiresHashedArtifact && !hasHashedArtifact {
		return fmt.Errorf("scenario evidence: evidence_grade %q requires at least one hashed artifact_ref", e.EvidenceGrade)
	}
	return nil
}

func isLowerHexSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
