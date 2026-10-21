package radiobackend

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const expectedSelectionHash = "e83f4b00fa43e75b8483d6158fa30951c2ba956893fe9269e9f8cef3d81a630b"

func TestIT085_CheckedInBackendSelectionIsStrictAndDeterministic(t *testing.T) {
	raw := readSelectionFixture(t)
	report, err := DecodeSelectionReport(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("DecodeSelectionReport() error = %v", err)
	}

	if report.SchemaVersion != SelectionSchemaVersion || report.StoryID != SelectionStoryID {
		t.Fatalf("selection identity = %q/%q", report.SchemaVersion, report.StoryID)
	}
	if report.HandoffContractVersion != ContractVersion || report.HandoffContractSHA256 != expectedHandoffHash {
		t.Fatalf("handoff binding = %q/%q", report.HandoffContractVersion, report.HandoffContractSHA256)
	}
	if report.Outcome != SelectionOutcomeSelected || report.SelectedCandidateID != "openrtx_mduv3x0" {
		t.Fatalf("selection = %q/%q", report.Outcome, report.SelectedCandidateID)
	}
	if len(report.Candidates) != 2 || report.Candidates[0].CandidateID != "opengd77_mduv380" || report.Candidates[1].CandidateID != "openrtx_mduv3x0" {
		t.Fatalf("candidate order = %+v", report.Candidates)
	}
	if report.Candidates[0].Decision != SelectionDecisionDeferred || report.Candidates[1].Decision != SelectionDecisionSelectedReceiveOnly {
		t.Fatalf("candidate decisions = %q/%q", report.Candidates[0].Decision, report.Candidates[1].Decision)
	}
	if !report.Safety.ReceiveOnlyDefault || report.Safety.TXEnabledByDefault || report.Safety.HardwareClaimAllowed || report.Safety.FieldTXReady {
		t.Fatalf("unsafe selection flags = %+v", report.Safety)
	}

	canonicalFirst, err := MarshalSelectionCanonical(report)
	if err != nil {
		t.Fatalf("MarshalSelectionCanonical(first) error = %v", err)
	}
	canonicalSecond, err := MarshalSelectionCanonical(report)
	if err != nil {
		t.Fatalf("MarshalSelectionCanonical(second) error = %v", err)
	}
	if !bytes.Equal(canonicalFirst, canonicalSecond) {
		t.Fatal("canonical selection bytes differ")
	}
	hash, err := SelectionReportHash(report)
	if err != nil {
		t.Fatalf("SelectionReportHash() error = %v", err)
	}
	if hash != expectedSelectionHash {
		t.Fatalf("selection hash = %q, want %q", hash, expectedSelectionHash)
	}

	handoffRaw, err := os.ReadFile(filepath.Join("..", "..", "configs", "firmware-contract", "mvp9-radio-backend-handoff.json"))
	if err != nil {
		t.Fatalf("read handoff fixture: %v", err)
	}
	handoff, err := DecodeContract(bytes.NewReader(handoffRaw))
	if err != nil {
		t.Fatalf("DecodeContract() error = %v", err)
	}
	handoffHash, err := ContractHash(handoff)
	if err != nil {
		t.Fatalf("ContractHash() error = %v", err)
	}
	if handoffHash != expectedHandoffHash {
		t.Fatalf("handoff hash = %q, want frozen %q", handoffHash, expectedHandoffHash)
	}
	verifyCheckedInSelectionArtifacts(t, report)
}

func TestSelectionReportRejectsMalformedOrUnsafeInput(t *testing.T) {
	base := mustDecodeSelectionFixture(t)

	tests := []struct {
		name    string
		mutate  func(*SelectionReport)
		wantErr string
	}{
		{
			name: "missing frozen candidate",
			mutate: func(report *SelectionReport) {
				report.Candidates = report.Candidates[1:]
			},
			wantErr: "candidate inventory",
		},
		{
			name: "unknown candidate replaces frozen candidate",
			mutate: func(report *SelectionReport) {
				report.Candidates[0].CandidateID = "another_backend"
			},
			wantErr: "candidate inventory",
		},
		{
			name: "wrong handoff hash",
			mutate: func(report *SelectionReport) {
				report.HandoffContractSHA256 = strings.Repeat("0", 64)
			},
			wantErr: "handoff_contract_sha256",
		},
		{
			name: "passed build has insufficient grade",
			mutate: func(report *SelectionReport) {
				check := selectionCheckByName(t, &report.Candidates[1], SelectionCheckBuildReproducibility)
				check.EvidenceGrade = EvidenceSourceReviewed
			},
			wantErr: "requires evidence_grade",
		},
		{
			name: "two selected candidates",
			mutate: func(report *SelectionReport) {
				report.Candidates[0].Decision = SelectionDecisionSelectedReceiveOnly
				for i := range report.Candidates[0].Checks {
					if report.Candidates[0].Checks[i].Name != SelectionCheckCodecCoupling {
						report.Candidates[0].Checks[i].Status = SelectionCheckPassed
						report.Candidates[0].Checks[i].EvidenceGrade = requiredPassedEvidenceGrade(report.Candidates[0].Checks[i].Name)
					}
				}
			},
			wantErr: "exactly one selected",
		},
		{
			name: "selected id mismatch",
			mutate: func(report *SelectionReport) {
				report.SelectedCandidateID = "opengd77_mduv380"
			},
			wantErr: "selected_candidate_id",
		},
		{
			name: "missing required check",
			mutate: func(report *SelectionReport) {
				report.Candidates[1].Checks = report.Candidates[1].Checks[:len(report.Candidates[1].Checks)-1]
			},
			wantErr: "checks",
		},
		{
			name: "selected candidate failed check",
			mutate: func(report *SelectionReport) {
				report.Candidates[1].Checks[0].Status = SelectionCheckFailed
			},
			wantErr: "selected candidate",
		},
		{
			name: "selected candidate unresolved source",
			mutate: func(report *SelectionReport) {
				check := selectionCheckByName(t, &report.Candidates[1], SelectionCheckSourcePin)
				check.Status = SelectionCheckDeferred
			},
			wantErr: "source_pin",
		},
		{
			name: "codec failure cannot be selected",
			mutate: func(report *SelectionReport) {
				check := selectionCheckByName(t, &report.Candidates[1], SelectionCheckCodecCoupling)
				check.Status = SelectionCheckFailed
			},
			wantErr: "codec_coupling",
		},
		{
			name: "hardware claim",
			mutate: func(report *SelectionReport) {
				report.Safety.HardwareClaimAllowed = true
			},
			wantErr: "safety",
		},
		{
			name: "unsorted candidates",
			mutate: func(report *SelectionReport) {
				report.Candidates[0], report.Candidates[1] = report.Candidates[1], report.Candidates[0]
			},
			wantErr: "sorted",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := CloneSelectionReport(base)
			test.mutate(&report)
			if err := ValidateSelectionReport(report); err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("ValidateSelectionReport() error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}

func verifyCheckedInSelectionArtifacts(t *testing.T, report SelectionReport) {
	t.Helper()
	root := filepath.Join("..", "..")
	for _, candidate := range report.Candidates {
		for _, check := range candidate.Checks {
			for _, artifact := range check.ArtifactRefs {
				raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(artifact.Path)))
				if err != nil {
					t.Fatalf("read checked-in selection artifact %q: %v", artifact.Path, err)
				}
				sum := sha256.Sum256(raw)
				if got := hex.EncodeToString(sum[:]); got != artifact.SHA256 {
					t.Fatalf("selection artifact %q sha256 = %q, want %q", artifact.Path, got, artifact.SHA256)
				}
			}
		}
	}
}

func TestSelectionReportDecodeIsStrictAndCloneIsImmutable(t *testing.T) {
	raw := readSelectionFixture(t)
	unknown := append(bytes.TrimSpace(raw), []byte("\n")...)
	unknown = bytes.Replace(unknown, []byte(`"story_id": "MVP-10.1"`), []byte(`"story_id": "MVP-10.1", "unexpected": true`), 1)
	if _, err := DecodeSelectionReport(bytes.NewReader(unknown)); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown field error = %v", err)
	}
	if _, err := DecodeSelectionReport(bytes.NewReader(append(raw, []byte(` {}`)...))); err == nil || !strings.Contains(err.Error(), "trailing JSON") {
		t.Fatalf("trailing JSON error = %v", err)
	}

	report := mustDecodeSelectionFixture(t)
	clone := CloneSelectionReport(report)
	clone.Candidates[0].Checks[0].Reason = "mutated"
	clone.Candidates[0].Checks[0].ArtifactRefs[0].SHA256 = strings.Repeat("f", 64)
	if report.Candidates[0].Checks[0].Reason == "mutated" || report.Candidates[0].Checks[0].ArtifactRefs[0].SHA256 == strings.Repeat("f", 64) {
		t.Fatal("CloneSelectionReport() aliases caller input")
	}
}

func readSelectionFixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "configs", "firmware-contract", "mvp10-backend-selection.json"))
	if err != nil {
		t.Fatalf("read selection fixture: %v", err)
	}
	return raw
}

func mustDecodeSelectionFixture(t *testing.T) SelectionReport {
	t.Helper()
	report, err := DecodeSelectionReport(bytes.NewReader(readSelectionFixture(t)))
	if err != nil {
		t.Fatalf("DecodeSelectionReport() error = %v", err)
	}
	return report
}

func selectionCheckByName(t *testing.T, candidate *SelectionCandidate, name SelectionCheckName) *SelectionCheck {
	t.Helper()
	for i := range candidate.Checks {
		if candidate.Checks[i].Name == name {
			return &candidate.Checks[i]
		}
	}
	t.Fatalf("candidate %s missing check %s", candidate.CandidateID, name)
	return nil
}
