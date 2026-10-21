package firmwareevidence

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/radiobackend"
)

const (
	expectedBuildEvidenceHash = "34c91553a236ff397065177e6d166ac9407749f35ee979873c3f716017d63612"
	expectedRollbackHash      = "97eaf2495c38f6c380a2358b76da36480761b6558b8064263d4bee0fe28ff78b"
)

func TestIT087_CheckedInFirmwareEvidenceIsStrictAndBound(t *testing.T) {
	root := filepath.Join("..", "..")
	evidence := mustReadBuildEvidence(t, root)
	rollback := mustReadRollback(t, root)

	buildHash, err := BuildEvidenceHash(evidence)
	if err != nil {
		t.Fatalf("BuildEvidenceHash() error = %v", err)
	}
	if buildHash != expectedBuildEvidenceHash {
		t.Fatalf("build evidence hash = %q, want %q", buildHash, expectedBuildEvidenceHash)
	}
	rollbackHash, err := RollbackPackageHash(rollback)
	if err != nil {
		t.Fatalf("RollbackPackageHash() error = %v", err)
	}
	if rollbackHash != expectedRollbackHash {
		t.Fatalf("rollback hash = %q, want %q", rollbackHash, expectedRollbackHash)
	}
	if err := VerifyCheckedInInputs(root, evidence, rollback); err != nil {
		t.Fatalf("VerifyCheckedInInputs() error = %v", err)
	}
}

func TestIT087_LocalBuildArtifactsMatchManifest(t *testing.T) {
	root := filepath.Join("..", "..")
	evidence := mustReadBuildEvidence(t, root)
	artifactRoot := filepath.Join(root, ".artifacts", "openrtx-mvp10.3")
	if _, err := os.Stat(artifactRoot); os.IsNotExist(err) {
		t.Skipf("local firmware artifacts are absent: %s", artifactRoot)
	} else if err != nil {
		t.Fatalf("stat local build artifacts: %v", err)
	}
	if err := VerifyLocalBuildArtifacts(root, evidence); err != nil {
		t.Fatalf("VerifyLocalBuildArtifacts() error = %v", err)
	}
}

func TestIT087_HostProtocolAndPatchApply(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	host := exec.Command(filepath.Join(root, "firmware", "openrtx-mvp10.3", "run-host-tests.sh"))
	host.Dir = root
	output, err := host.CombinedOutput()
	if err != nil {
		t.Fatalf("host protocol tests failed: %v\n%s", err, output)
	}
	if !bytes.Contains(output, []byte("adhr_rxonly_host_test: PASS")) {
		t.Fatalf("host protocol output = %q", output)
	}
	if !bytes.Contains(output, []byte("adhr_usb_rxonly_host_test: PASS")) {
		t.Fatalf("host USB adapter output = %q", output)
	}
	assertUnsafeScratchIDRejected(t, root, "prepare-dependencies.sh")
	assertUnsafeScratchIDRejected(t, root, "build-isolated.sh")

	checkout := "/tmp/adhr-mvp10-openrtx/OpenRTX"
	if _, err := os.Stat(filepath.Join(checkout, ".git")); err == nil {
		patch := filepath.Join(root, "firmware", "openrtx-mvp10.3", "existing-files.patch")
		command := exec.Command("git", "-C", checkout, "apply", "--check", patch)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git apply --check failed: %v\n%s", err, output)
		}
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat pinned checkout: %v", err)
	}
}

func TestIT087_FirmwareWireFieldsMatchFrozenHandoff(t *testing.T) {
	root := filepath.Join("..", "..")
	handoffRaw := readFile(t, filepath.Join(root, "configs", "firmware-contract",
		"mvp9-radio-backend-handoff.json"))
	handoff, err := radiobackend.DecodeContract(bytes.NewReader(handoffRaw))
	if err != nil {
		t.Fatalf("DecodeContract() error = %v", err)
	}
	source := string(readFile(t, filepath.Join(root, "firmware", "openrtx-mvp10.3",
		"overlay", "openrtx", "src", "rtx", "adhr_rxonly.c")))
	handlers := map[radiobackend.OperationName][2]string{
		radiobackend.OperationReceiveDelivery:   {"handle_rx", "handle_link"},
		radiobackend.OperationLinkMeasurement:   {"handle_link", "handle_timing"},
		radiobackend.OperationTimingObservation: {"handle_timing", "process_line"},
	}
	matched := 0
	for _, operation := range handoff.Operations {
		bounds, ok := handlers[operation.Name]
		if !ok {
			continue
		}
		matched++
		segment := cFunctionSegment(t, source, bounds[0], bounds[1])
		for _, field := range operation.RequiredFields {
			if !strings.Contains(segment, field+"=") {
				t.Errorf("%s handler omits frozen field %q", operation.Name, field)
			}
		}
	}
	if matched != len(handlers) {
		t.Fatalf("matched %d receive-side operations, want %d", matched, len(handlers))
	}
}

func TestParseSourceHashesRejectsDuplicateAndUnsafePaths(t *testing.T) {
	hash := strings.Repeat("a", 64)
	for _, input := range []string{
		hash + "  ../outside\n",
		hash + "  firmware/a\n" + hash + "  firmware/a\n",
		"not-a-hash  firmware/a\n",
	} {
		if _, err := parseSourceHashes([]byte(input)); err == nil {
			t.Fatalf("parseSourceHashes(%q) unexpectedly succeeded", input)
		}
	}
}

func TestBuildEvidenceRejectsUnsafeOrDriftingInput(t *testing.T) {
	base := mustReadBuildEvidence(t, filepath.Join("..", ".."))
	tests := []struct {
		name    string
		mutate  func(*BuildEvidence)
		wantErr string
	}{
		{
			name: "source drift",
			mutate: func(e *BuildEvidence) {
				e.Source.Commit = strings.Repeat("0", 40)
			},
			wantErr: "source pin",
		},
		{
			name: "network enabled",
			mutate: func(e *BuildEvidence) {
				e.BuildEnvironment.Network = "bridge"
			},
			wantErr: "build environment",
		},
		{
			name: "dependency row drift",
			mutate: func(e *BuildEvidence) {
				e.DependencyLock.Dependencies[0].Repo = "https://example.invalid/codec2"
			},
			wantErr: "dependency inventory",
		},
		{
			name: "second build drift",
			mutate: func(e *BuildEvidence) {
				e.Runs[1].Artifacts[1].SHA256 = strings.Repeat("0", 64)
			},
			wantErr: "reproducibility drift",
		},
		{
			name: "codec overclaim",
			mutate: func(e *BuildEvidence) {
				e.BoundaryEvidence.BroadTargetCodecCoupling = "passed"
			},
			wantErr: "receive-only boundary",
		},
		{
			name: "firmware write allowed",
			mutate: func(e *BuildEvidence) {
				e.Safety.FirmwareWriteAllowed = true
			},
			wantErr: "unsafe claims",
		},
		{
			name: "unsafe bundle path",
			mutate: func(e *BuildEvidence) {
				e.BundleFiles[0].Path = "../outside.patch"
			},
			wantErr: "bundle file",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evidence := cloneBuildEvidence(t, base)
			test.mutate(&evidence)
			if err := ValidateBuildEvidence(evidence); err == nil ||
				!strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("ValidateBuildEvidence() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestRollbackPackageRejectsUnsafeClaims(t *testing.T) {
	base := mustReadRollback(t, filepath.Join("..", ".."))
	tests := []struct {
		name    string
		mutate  func(*RollbackPackage)
		wantErr string
	}{
		{
			name: "wrong firmware output",
			mutate: func(r *RollbackPackage) {
				r.ExactBuildOutput.FirmwareBinSHA256 = strings.Repeat("0", 64)
			},
			wantErr: "exact build output",
		},
		{
			name: "claims execution",
			mutate: func(r *RollbackPackage) {
				r.Claims.ExecutedForThisBuild = true
			},
			wantErr: "unsafe claims",
		},
		{
			name: "claims calibration proof",
			mutate: func(r *RollbackPackage) {
				r.Claims.CalibrationRoundTripProven = true
			},
			wantErr: "unsafe claims",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rollback := cloneRollback(t, base)
			test.mutate(&rollback)
			if err := ValidateRollbackPackage(rollback); err == nil ||
				!strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("ValidateRollbackPackage() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestFirmwareEvidenceDecodeRejectsUnknownAndTrailingJSON(t *testing.T) {
	root := filepath.Join("..", "..")
	raw := readFile(t, filepath.Join(root, "configs", "firmware-contract",
		"openrtx-mduv3x0-adhr-rxonly-mvp10.3.json"))
	unknown := bytes.Replace(raw,
		[]byte(`"story_id": "MVP-10.3"`),
		[]byte(`"story_id": "MVP-10.3", "unexpected": true`), 1)
	if _, err := DecodeBuildEvidence(bytes.NewReader(unknown)); err == nil ||
		!strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown field error = %v", err)
	}
	if _, err := DecodeBuildEvidence(bytes.NewReader(append(raw, []byte(` {}`)...))); err == nil ||
		!strings.Contains(err.Error(), "trailing JSON") {
		t.Fatalf("trailing JSON error = %v", err)
	}
}

func TestDependencyLockDecodeRejectsUnknownAndTrailingJSON(t *testing.T) {
	root := filepath.Join("..", "..")
	raw := readFile(t, filepath.Join(root, "firmware", "openrtx-mvp10.3",
		"dependency-lock.json"))
	unknown := bytes.Replace(raw,
		[]byte(`"schema_version": "adhr-openrtx-dependency-lock-v1"`),
		[]byte(`"schema_version": "adhr-openrtx-dependency-lock-v1", "unexpected": true`), 1)
	if _, err := DecodeDependencyLock(bytes.NewReader(unknown)); err == nil ||
		!strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown field error = %v", err)
	}
	if _, err := DecodeDependencyLock(bytes.NewReader(append(raw, []byte(` {}`)...))); err == nil ||
		!strings.Contains(err.Error(), "trailing JSON") {
		t.Fatalf("trailing JSON error = %v", err)
	}
}

func mustReadBuildEvidence(t *testing.T, root string) BuildEvidence {
	t.Helper()
	raw := readFile(t, filepath.Join(root, "configs", "firmware-contract",
		"openrtx-mduv3x0-adhr-rxonly-mvp10.3.json"))
	evidence, err := DecodeBuildEvidence(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("DecodeBuildEvidence() error = %v", err)
	}
	return evidence
}

func mustReadRollback(t *testing.T, root string) RollbackPackage {
	t.Helper()
	raw := readFile(t, filepath.Join(root, "configs", "firmware-contract",
		"openrtx-mduv3x0-adhr-rxonly-rollback.json"))
	rollback, err := DecodeRollbackPackage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("DecodeRollbackPackage() error = %v", err)
	}
	return rollback
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return raw
}

func cloneBuildEvidence(t *testing.T, source BuildEvidence) BuildEvidence {
	t.Helper()
	var clone BuildEvidence
	cloneJSON(t, source, &clone)
	return clone
}

func cloneRollback(t *testing.T, source RollbackPackage) RollbackPackage {
	t.Helper()
	var clone RollbackPackage
	cloneJSON(t, source, &clone)
	return clone
}

func cloneJSON(t *testing.T, source, target any) {
	t.Helper()
	raw, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatal(err)
	}
}

func assertUnsafeScratchIDRejected(t *testing.T, root, script string) {
	t.Helper()
	command := exec.Command(filepath.Join(root, "firmware", "openrtx-mvp10.3", script),
		"../unsafe")
	command.Dir = root
	output, err := command.CombinedOutput()
	if err == nil || !bytes.Contains(output, []byte("invalid")) {
		t.Fatalf("%s unsafe-ID result: err=%v output=%q", script, err, output)
	}
}

func cFunctionSegment(t *testing.T, source, startName, endName string) string {
	t.Helper()
	start := strings.Index(source, "static void "+startName+"(")
	end := strings.Index(source, "static void "+endName+"(")
	if start < 0 || end <= start {
		t.Fatalf("cannot isolate C functions %s..%s", startName, endName)
	}
	return source[start:end]
}
