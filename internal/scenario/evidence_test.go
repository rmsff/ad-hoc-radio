package scenario

import "testing"

const testScenarioEvidenceHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestScenarioEvidence_ValidatesCanonicalEnvelope(t *testing.T) {
	evidence := validScenarioEvidence()
	if err := ValidateScenarioEvidence(evidence); err != nil {
		t.Fatalf("ValidateScenarioEvidence returned error: %v", err)
	}
}

func TestScenarioEvidence_RejectsUnsupportedGrade(t *testing.T) {
	evidence := validScenarioEvidence()
	evidence.EvidenceGrade = "forum_guess"
	if err := ValidateScenarioEvidence(evidence); err == nil {
		t.Fatal("ValidateScenarioEvidence accepted unsupported evidence grade")
	}
}

func TestScenarioEvidence_AcceptsAllSupportedGrades(t *testing.T) {
	tests := []struct {
		name         string
		grade        string
		artifactRefs []ScenarioEvidenceArtifact
	}{
		{
			name:         "synthetic_proxy",
			grade:        EvidenceGradeSyntheticProxy,
			artifactRefs: nil,
		},
		{
			name:         "fixture_metadata",
			grade:        EvidenceGradeFixtureMetadata,
			artifactRefs: nil,
		},
		{
			name:  "fixture_artifact",
			grade: EvidenceGradeFixtureArtifact,
			artifactRefs: []ScenarioEvidenceArtifact{
				validScenarioEvidenceArtifact(),
			},
		},
		{
			name:  "bench_measured",
			grade: EvidenceGradeBenchMeasured,
			artifactRefs: []ScenarioEvidenceArtifact{
				validScenarioEvidenceArtifact(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evidence := validScenarioEvidence()
			evidence.EvidenceGrade = tt.grade
			evidence.ArtifactRefs = tt.artifactRefs
			if err := ValidateScenarioEvidence(evidence); err != nil {
				t.Fatalf("ValidateScenarioEvidence rejected %s: %v", tt.grade, err)
			}
		})
	}
}

func TestScenarioEvidence_RejectsMissingEventHash(t *testing.T) {
	evidence := validScenarioEvidence()
	evidence.Determinism.EventHash = ""
	if err := ValidateScenarioEvidence(evidence); err == nil {
		t.Fatal("ValidateScenarioEvidence accepted missing event hash")
	}
}

func TestScenarioEvidence_RejectsMissingRequiredFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ScenarioEvidence)
	}{
		{
			name: "schema_version",
			mutate: func(e *ScenarioEvidence) {
				e.SchemaVersion = ""
			},
		},
		{
			name: "story_id",
			mutate: func(e *ScenarioEvidence) {
				e.StoryID = ""
			},
		},
		{
			name: "scenario_id",
			mutate: func(e *ScenarioEvidence) {
				e.ScenarioID = ""
			},
		},
		{
			name: "evidence_type",
			mutate: func(e *ScenarioEvidence) {
				e.EvidenceType = ""
			},
		},
		{
			name: "outcome",
			mutate: func(e *ScenarioEvidence) {
				e.Outcome = ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evidence := validScenarioEvidence()
			tt.mutate(&evidence)
			if err := ValidateScenarioEvidence(evidence); err == nil {
				t.Fatalf("ValidateScenarioEvidence accepted missing %s", tt.name)
			}
		})
	}
}

func TestScenarioEvidence_RejectsUnsupportedDeterminismMetadata(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ScenarioEvidence)
	}{
		{
			name: "schema_version",
			mutate: func(e *ScenarioEvidence) {
				e.SchemaVersion = "scenario-evidence-v2"
			},
		},
		{
			name: "hash_algorithm",
			mutate: func(e *ScenarioEvidence) {
				e.Determinism.HashAlgorithm = "md5"
			},
		},
		{
			name: "serialization",
			mutate: func(e *ScenarioEvidence) {
				e.Determinism.Serialization = "pretty-json"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evidence := validScenarioEvidence()
			tt.mutate(&evidence)
			if err := ValidateScenarioEvidence(evidence); err == nil {
				t.Fatalf("ValidateScenarioEvidence accepted unsupported %s", tt.name)
			}
		})
	}
}

func TestScenarioEvidence_RejectsUppercaseDigest(t *testing.T) {
	evidence := validScenarioEvidence()
	evidence.ArtifactRefs[0].SHA256 = "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF"
	if err := ValidateScenarioEvidence(evidence); err == nil {
		t.Fatal("ValidateScenarioEvidence accepted uppercase artifact digest")
	}
}

func TestScenarioEvidence_RejectsHashedArtifactWithoutPath(t *testing.T) {
	evidence := validScenarioEvidence()
	evidence.ArtifactRefs[0].Path = ""
	if err := ValidateScenarioEvidence(evidence); err == nil {
		t.Fatal("ValidateScenarioEvidence accepted hashed artifact without path")
	}
}

func TestScenarioEvidence_HardwareClaimRequiresBenchMeasured(t *testing.T) {
	evidence := validScenarioEvidence()
	evidence.HardwareClaimAllowed = true
	if err := ValidateScenarioEvidence(evidence); err == nil {
		t.Fatal("ValidateScenarioEvidence accepted hardware claim without bench-measured evidence")
	}

	evidence.EvidenceGrade = EvidenceGradeBenchMeasured
	if err := ValidateScenarioEvidence(evidence); err != nil {
		t.Fatalf("ValidateScenarioEvidence rejected bench-measured hardware claim: %v", err)
	}
}

func TestScenarioEvidence_FixtureArtifactRequiresHashedArtifact(t *testing.T) {
	evidence := validScenarioEvidence()
	evidence.ArtifactRefs[0].SHA256 = ""
	if err := ValidateScenarioEvidence(evidence); err == nil {
		t.Fatal("ValidateScenarioEvidence accepted fixture artifact evidence without hashed artifact")
	}
}

func validScenarioEvidence() ScenarioEvidence {
	return ScenarioEvidence{
		SchemaVersion: ScenarioEvidenceSchemaVersion,
		StoryID:       "MVP-8.8a",
		ScenarioID:    "scenario_evidence_envelope",
		EvidenceType:  "audio_intelligibility_fixture",
		EvidenceGrade: EvidenceGradeFixtureArtifact,
		Seed:          DefaultScenarioSeed,
		Determinism: ScenarioEvidenceDeterminism{
			EventHash:     testScenarioEvidenceHash,
			HashAlgorithm: ScenarioEvidenceHashAlgorithm,
			Serialization: ScenarioEvidenceSerialization,
		},
		ArtifactRefs: []ScenarioEvidenceArtifact{
			validScenarioEvidenceArtifact(),
		},
		AssumptionFlags:      []string{"fixture_only"},
		HardwareClaimAllowed: false,
		Outcome:              "ready",
		DegradeReasons:       []string{},
	}
}

func validScenarioEvidenceArtifact() ScenarioEvidenceArtifact {
	return ScenarioEvidenceArtifact{
		ArtifactID: "clean_voice_fixture",
		Path:       "fixtures/audio/clean_voice.wav",
		SHA256:     testScenarioEvidenceHash,
		Role:       "source_fixture",
	}
}
