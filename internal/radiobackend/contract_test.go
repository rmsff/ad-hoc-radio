package radiobackend

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const (
	handoffFixturePath           = "../../configs/firmware-contract/mvp9-radio-backend-handoff.json"
	expectedHandoffHash          = "d97a01b0160fe2c125574ffc1c1125a9da20c2d20e0508d35d565977d38ec39a"
	expectedLegacySimProfileHash = "f8451ef83f708808d1c9b2b7bc3b0547b811dfa89b09111f18303530623457d2"
	expectedLegacyBenchLogHash   = "7e505b583a2e5dbb4bf522d6d3b9beaa7c5f78fcbdcd3512c19534f8a2200e8c"
)

func TestIT080_CheckedInMVP9HandoffIsValidAndBounded(t *testing.T) {
	contract := loadHandoffFixture(t)
	if contract.ContractVersion != ContractVersion || contract.StoryID != StoryID {
		t.Fatalf("contract identity = %q/%q", contract.ContractVersion, contract.StoryID)
	}
	gate := contract.SimulatorGate
	if gate.ProfileID != "source_led/project_native_variable" || gate.ArchitectureVerdict != "accepted" || gate.ProfileVerdict != "go" {
		t.Fatalf("simulator gate = %+v", gate)
	}
	if gate.StressEvidenceHash != ApprovedMVP97StressEvidenceHash {
		t.Fatalf("stress evidence hash = %q, want %q", gate.StressEvidenceHash, ApprovedMVP97StressEvidenceHash)
	}
	if gate.FramePeriodMS != 80 || gate.MaxRelayDepth != 3 || gate.MaxSerializedBranchWidth != 4 || !gate.SerializedSingleFrequency || gate.SpatialReuseEnabled {
		t.Fatalf("operating envelope = %+v", gate)
	}
	if gate.DMRCompatible || gate.CompatibilityClaim || gate.AMBEDependency || gate.RFSynchronizationClaim {
		t.Fatalf("forbidden simulator claims = %+v", gate)
	}
	if !contract.Safety.ReceiveOnlyDefault || contract.Safety.TXEnabledByDefault || contract.Safety.FieldTXAllowed || contract.Safety.HardwareClaimAllowed || contract.Safety.RegulatoryProfileAccepted || contract.Safety.LabSafetyPlanAccepted {
		t.Fatalf("safety boundary = %+v", contract.Safety)
	}
	if len(contract.Operations) != 5 || len(contract.Candidates) != 2 {
		t.Fatalf("contract inventory = operations:%d candidates:%d", len(contract.Operations), len(contract.Candidates))
	}
	scheduledTransmit := operationByName(t, &contract, OperationScheduledTransmit)
	for _, field := range []string{"backend_clock_id", "deadline_monotonic_ms", "not_before_monotonic_ms", "opportunity_index", "seq"} {
		if !slices.Contains(scheduledTransmit.RequiredFields, field) {
			t.Fatalf("scheduled_transmit missing %q", field)
		}
	}
	for _, forbidden := range []string{"effective_seq", "expires_at_ms", "valid_through_seq"} {
		if slices.Contains(scheduledTransmit.RequiredFields, forbidden) {
			t.Fatalf("scheduled_transmit carries core-owned lease field %q", forbidden)
		}
	}
	legacyRaw, err := os.ReadFile(filepath.Clean("../../configs/firmware-contract/prestation-sim-profile.json"))
	if err != nil {
		t.Fatal(err)
	}
	var legacy struct {
		ContractVersion string `json:"contract_version"`
		SafetyProfile   struct {
			TXEnabledByDefault bool `json:"tx_enabled_by_default"`
			FieldTXAllowed     bool `json:"field_tx_allowed"`
		} `json:"safety_profile"`
	}
	if err := json.Unmarshal(legacyRaw, &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.ContractVersion != "adhr-sim-fw-contract-v0" || legacy.SafetyProfile.TXEnabledByDefault || legacy.SafetyProfile.FieldTXAllowed {
		t.Fatalf("legacy v0 fixture changed identity/safety: %+v", legacy)
	}
	if got := rawSHA256(legacyRaw); got != expectedLegacySimProfileHash {
		t.Fatalf("legacy sim profile hash = %q, want %q", got, expectedLegacySimProfileHash)
	}
	legacyBenchRaw, err := os.ReadFile(filepath.Clean("../../configs/firmware-contract/prestation-bench-log.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if got := rawSHA256(legacyBenchRaw); got != expectedLegacyBenchLogHash {
		t.Fatalf("legacy bench log hash = %q, want %q", got, expectedLegacyBenchLogHash)
	}
}

func TestIT081_BackendCandidatesExposeExactGatesWithoutClaims(t *testing.T) {
	contract := loadHandoffFixture(t)
	for _, candidate := range contract.Candidates {
		if candidate.Verdict != CandidateMeasurementRequired {
			t.Fatalf("candidate %s verdict = %q, want measurement_required", candidate.CandidateID, candidate.Verdict)
		}
		if len(candidate.Gates) != len(RequiredGateNames()) {
			t.Fatalf("candidate %s gate count = %d", candidate.CandidateID, len(candidate.Gates))
		}
		for i, gate := range candidate.Gates {
			if gate.Name != RequiredGateNames()[i] {
				t.Fatalf("candidate %s gate[%d] = %q", candidate.CandidateID, i, gate.Name)
			}
		}
		if candidate.DMRCompatible || candidate.CompatibilityClaim || candidate.AMBEDependency || candidate.HardwareClaimAllowed || candidate.FieldTXReady {
			t.Fatalf("candidate %s carries forbidden claims: %+v", candidate.CandidateID, candidate)
		}
	}
	if contract.Candidates[0].CandidateID != "opengd77_mduv380" || contract.Candidates[1].CandidateID != "openrtx_mduv3x0" {
		t.Fatalf("candidate canonical order = %v", []string{contract.Candidates[0].CandidateID, contract.Candidates[1].CandidateID})
	}
	openRTX := candidateByID(t, &contract, "openrtx_mduv3x0")
	codec := gateByName(t, openRTX, GateCodecCoupling)
	if codec.Status != GateNotEvaluated || codec.EvidenceGrade != EvidenceNone || len(codec.ArtifactRefs) != 0 {
		t.Fatalf("OpenRTX codec gate overclaims evidence: %+v", codec)
	}
}

func TestIT082_CanonicalContractBytesAndHashAreDeterministic(t *testing.T) {
	contract := loadHandoffFixture(t)
	before := CloneContract(contract)
	first, err := MarshalCanonical(contract)
	if err != nil {
		t.Fatal(err)
	}
	second, err := MarshalCanonical(contract)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("canonical contract bytes changed between runs")
	}
	firstHash, err := ContractHash(contract)
	if err != nil {
		t.Fatal(err)
	}
	secondHash, err := ContractHash(contract)
	if err != nil {
		t.Fatal(err)
	}
	if firstHash != expectedHandoffHash || firstHash != secondHash {
		t.Fatalf("contract hashes = %q/%q, want %q", firstHash, secondHash, expectedHandoffHash)
	}
	after, _ := MarshalCanonical(contract)
	beforeBytes, _ := MarshalCanonical(before)
	if !bytes.Equal(beforeBytes, after) {
		t.Fatal("canonical marshal mutated caller contract")
	}
	t.Logf("MVP-9.8 contract hash %s", firstHash)
}

func TestIT083_StrictDecoderRejectsUnknownAndTrailingJSON(t *testing.T) {
	raw := readHandoffFixture(t)
	unknown := bytes.Replace(raw, []byte(`"story_id"`), []byte(`"unknown":true,"story_id"`), 1)
	if _, err := DecodeContract(bytes.NewReader(unknown)); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown-field error = %v", err)
	}
	trailing := append(append([]byte(nil), raw...), []byte("\n{}")...)
	if _, err := DecodeContract(bytes.NewReader(trailing)); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("trailing JSON error = %v", err)
	}
}

func TestIT084_BackendFailureIsCandidateScoped(t *testing.T) {
	contract := loadHandoffFixture(t)
	failed := CloneContract(contract)
	openRTX := candidateByID(t, &failed, "openrtx_mduv3x0")
	license := gateByName(t, openRTX, GateLicense)
	license.Status = GateFailed
	license.EvidenceGrade = EvidenceSourceReviewed
	license.Reason = "license review failed for the selected source snapshot"
	openRTX.Verdict = CandidateRejected
	if err := ValidateContract(failed); err != nil {
		t.Fatalf("candidate-scoped failure contract invalid: %v", err)
	}
	if candidateByID(t, &failed, "opengd77_mduv380").Verdict != CandidateMeasurementRequired {
		t.Fatal("OpenRTX failure changed OpenGD77 verdict")
	}
	if failed.SimulatorGate.ProfileVerdict != "go" || failed.SimulatorGate.ArchitectureVerdict != "accepted" {
		t.Fatal("backend failure rewrote simulator/architecture verdict")
	}
}

func TestUT028_ContractValidationRejectsMalformedSemantics(t *testing.T) {
	base := loadHandoffFixture(t)

	t.Run("ownership overlap", func(t *testing.T) {
		contract := CloneContract(base)
		contract.Ownership.BackendOwned[0] = contract.Ownership.NetworkCoreOwned[0]
		if err := ValidateContract(contract); err == nil || !strings.Contains(err.Error(), "ownership") {
			t.Fatalf("ownership error = %v", err)
		}
	})

	t.Run("operation inventory", func(t *testing.T) {
		contract := CloneContract(base)
		contract.Operations = contract.Operations[:len(contract.Operations)-1]
		if err := ValidateContract(contract); err == nil || !strings.Contains(err.Error(), "operations") {
			t.Fatalf("operation inventory error = %v", err)
		}
	})

	t.Run("scheduled transmit execution identity", func(t *testing.T) {
		contract := CloneContract(base)
		operation := operationByName(t, &contract, OperationScheduledTransmit)
		operation.RequiredFields = slices.DeleteFunc(operation.RequiredFields, func(field string) bool {
			return field == "opportunity_index"
		})
		if err := ValidateContract(contract); err == nil || !strings.Contains(err.Error(), "required_fields") {
			t.Fatalf("scheduled transmit identity error = %v", err)
		}
	})

	t.Run("duplicate gate", func(t *testing.T) {
		contract := CloneContract(base)
		contract.Candidates[0].Gates[1].Name = contract.Candidates[0].Gates[0].Name
		if err := ValidateContract(contract); err == nil || !strings.Contains(err.Error(), "gates") {
			t.Fatalf("gate inventory error = %v", err)
		}
	})

	t.Run("simulator envelope", func(t *testing.T) {
		contract := CloneContract(base)
		contract.SimulatorGate.SpatialReuseEnabled = true
		if err := ValidateContract(contract); err == nil || !strings.Contains(err.Error(), "spatial_reuse_enabled") {
			t.Fatalf("simulator envelope error = %v", err)
		}
	})

	t.Run("approved stress evidence", func(t *testing.T) {
		contract := CloneContract(base)
		contract.SimulatorGate.StressEvidenceHash = strings.Repeat("0", 64)
		if err := ValidateContract(contract); err == nil || !strings.Contains(err.Error(), "approved MVP-9.7 evidence hash") {
			t.Fatalf("stress evidence error = %v", err)
		}
	})

	t.Run("canonical ordering", func(t *testing.T) {
		contract := CloneContract(base)
		slices.Reverse(contract.Operations)
		if err := ValidateContract(contract); err == nil || !strings.Contains(err.Error(), "sorted") {
			t.Fatalf("canonical ordering error = %v", err)
		}
	})
}

func TestUT029_EvidenceAndVerdictRulesAreStrict(t *testing.T) {
	base := loadHandoffFixture(t)

	t.Run("measurement pass requires bench evidence", func(t *testing.T) {
		contract := CloneContract(base)
		candidate := candidateByID(t, &contract, "openrtx_mduv3x0")
		gate := gateByName(t, candidate, GateTXRXTurnaround)
		gate.Status = GatePassed
		gate.EvidenceGrade = EvidenceBuildVerified
		gate.Reason = "build-only evidence is insufficient"
		if err := ValidateContract(contract); err == nil || !strings.Contains(err.Error(), "bench_measured") {
			t.Fatalf("bench-evidence error = %v", err)
		}
	})

	t.Run("passed gate requires artifact", func(t *testing.T) {
		contract := CloneContract(base)
		candidate := candidateByID(t, &contract, "openrtx_mduv3x0")
		gate := gateByName(t, candidate, GateCodecCoupling)
		gate.Status = GatePassed
		gate.EvidenceGrade = EvidenceSourceReviewed
		gate.Reason = "source review proves codec-independent forwarding"
		gate.ArtifactRefs = nil
		if err := ValidateContract(contract); err == nil || !strings.Contains(err.Error(), "artifact") {
			t.Fatalf("artifact error = %v", err)
		}
	})

	t.Run("eligible verdict requires all gates passed", func(t *testing.T) {
		contract := CloneContract(base)
		candidateByID(t, &contract, "openrtx_mduv3x0").Verdict = CandidateEligibleReceiveOnlyLab
		if err := ValidateContract(contract); err == nil || !strings.Contains(err.Error(), "verdict") {
			t.Fatalf("candidate verdict error = %v", err)
		}
	})

	t.Run("production package stays independent from project internals", func(t *testing.T) {
		files, err := filepath.Glob("*.go")
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatalf("parse %s: %v", file, err)
			}
			for _, importSpec := range parsed.Imports {
				importPath, err := strconv.Unquote(importSpec.Path.Value)
				if err != nil {
					t.Fatalf("parse import in %s: %v", file, err)
				}
				if strings.HasPrefix(importPath, "github.com/rmsff/ad-hoc-radio/") {
					t.Fatalf("%s imports project package %s", file, importPath)
				}
			}
		}
	})
}

func loadHandoffFixture(t *testing.T) Contract {
	t.Helper()
	contract, err := DecodeContract(bytes.NewReader(readHandoffFixture(t)))
	if err != nil {
		t.Fatalf("DecodeContract failed: %v", err)
	}
	return contract
}

func readHandoffFixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Clean(handoffFixturePath))
	if err != nil {
		t.Fatalf("read handoff fixture: %v", err)
	}
	return raw
}

func rawSHA256(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func candidateByID(t *testing.T, contract *Contract, id string) *CandidateEvaluation {
	t.Helper()
	for i := range contract.Candidates {
		if contract.Candidates[i].CandidateID == id {
			return &contract.Candidates[i]
		}
	}
	t.Fatalf("missing candidate %s", id)
	return nil
}

func gateByName(t *testing.T, candidate *CandidateEvaluation, name GateName) *EvaluationGate {
	t.Helper()
	for i := range candidate.Gates {
		if candidate.Gates[i].Name == name {
			return &candidate.Gates[i]
		}
	}
	t.Fatalf("candidate %s missing gate %s", candidate.CandidateID, name)
	return nil
}

func operationByName(t *testing.T, contract *Contract, name OperationName) *BoundaryOperation {
	t.Helper()
	for i := range contract.Operations {
		if contract.Operations[i].Name == name {
			return &contract.Operations[i]
		}
	}
	t.Fatalf("missing operation %s", name)
	return nil
}
