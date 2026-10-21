package firmwareevidence

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/rmsff/ad-hoc-radio/internal/backendmeasurement"
	"github.com/rmsff/ad-hoc-radio/internal/radiobackend"
)

const checkedInEvidenceRoot = "firmware/openrtx-mvp10.3/evidence"

var canonicalEvidenceFiles = map[string]string{
	"bundle_source_hashes":      "bundle-source-hashes.txt",
	"applied_source_patch":      "applied-source.patch",
	"undefined_symbols":         "adhr-undefined-symbols.txt",
	"tx_guard_disassembly":      "tx-guard-disassembly.txt",
	"usb_try_write_disassembly": "usb-try-write-disassembly.txt",
	"protocol_strings":          "adhr-protocol-strings.txt",
	"source_isolation":          "source-isolation.txt",
	"size":                      "size.txt",
	"sections":                  "sections.txt",
	"toolchain":                 "toolchain.txt",
}

func VerifyCheckedInInputs(root string, evidence BuildEvidence, rollback RollbackPackage) error {
	if err := verifyCanonicalBindings(root, evidence.Bindings); err != nil {
		return err
	}
	if err := verifyFile(root, evidence.DependencyLock.Path, evidence.DependencyLock.SHA256, -1); err != nil {
		return err
	}
	if err := verifyDependencyLockBinding(root, evidence); err != nil {
		return err
	}
	for _, file := range evidence.BundleFiles {
		if err := verifyFile(root, file.Path, file.SHA256, -1); err != nil {
			return err
		}
	}
	if err := verifyCanonicalEvidenceFiles(root, checkedInEvidenceRoot,
		evidence.CanonicalEvidence); err != nil {
		return err
	}
	if err := verifyBundleSourceHashes(root, evidence); err != nil {
		return err
	}
	if err := verifyAppliedPatchCoverage(root, evidence); err != nil {
		return err
	}
	if err := verifyEvidenceContents(root, checkedInEvidenceRoot); err != nil {
		return err
	}
	if err := verifyFile(root, evidence.RollbackPackage.Path, evidence.RollbackPackage.SHA256, -1); err != nil {
		return err
	}
	if err := verifyFile(root, rollback.RollbackInputInventory.Path,
		rollback.RollbackInputInventory.SHA256, -1); err != nil {
		return err
	}
	for _, procedure := range rollback.Recovery.ProcedureRefs {
		if !safeRelativePath(procedure) {
			return fmt.Errorf("firmware evidence: unsafe procedure path %q", procedure)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(procedure))); err != nil {
			return fmt.Errorf("firmware evidence: procedure %q: %w", procedure, err)
		}
	}
	if rollback.ExactBuildOutput.FirmwareBinSHA256 != evidence.Runs[0].Artifacts[1].SHA256 ||
		rollback.ExactBuildOutput.WrappedFirmwareSHA256 != evidence.Runs[0].Artifacts[2].SHA256 {
		return fmt.Errorf("firmware evidence: rollback output binding mismatch")
	}
	return nil
}

func verifyDependencyLockBinding(root string, evidence BuildEvidence) error {
	raw, err := os.ReadFile(filepath.Join(root,
		filepath.FromSlash(evidence.DependencyLock.Path)))
	if err != nil {
		return fmt.Errorf("firmware evidence: read dependency lock: %w", err)
	}
	lock, err := DecodeDependencyLock(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	if lock.OpenRTXCommit != evidence.Source.Commit ||
		!reflect.DeepEqual(lock.Dependencies, evidence.DependencyLock.Dependencies) {
		return fmt.Errorf("firmware evidence: dependency lock binding mismatch")
	}
	return nil
}

func VerifyLocalBuildArtifacts(root string, evidence BuildEvidence) error {
	for _, run := range evidence.Runs {
		for _, artifact := range run.Artifacts {
			if err := verifyFile(root, artifact.Path, artifact.SHA256, artifact.SizeBytes); err != nil {
				return err
			}
		}
		if err := verifyFile(root, run.ArtifactRoot+"/build.log", run.BuildLogSHA256, -1); err != nil {
			return err
		}
		for _, canonical := range evidence.CanonicalEvidence {
			name, ok := canonicalEvidenceFiles[canonical.Name]
			if !ok {
				return fmt.Errorf("firmware evidence: unknown canonical artifact %q", canonical.Name)
			}
			if err := verifyFile(root, run.ArtifactRoot+"/"+name, canonical.SHA256, -1); err != nil {
				return err
			}
		}
	}
	return verifyEvidenceContents(root, evidence.Runs[0].ArtifactRoot)
}

func verifyCanonicalEvidenceFiles(root, evidenceRoot string,
	evidence []NamedEvidence) error {
	for _, canonical := range evidence {
		name, ok := canonicalEvidenceFiles[canonical.Name]
		if !ok {
			return fmt.Errorf("firmware evidence: unknown canonical artifact %q",
				canonical.Name)
		}
		if err := verifyFile(root, evidenceRoot+"/"+name,
			canonical.SHA256, -1); err != nil {
			return err
		}
	}
	return nil
}

func verifyBundleSourceHashes(root string, evidence BuildEvidence) error {
	path := filepath.Join(root, filepath.FromSlash(checkedInEvidenceRoot),
		"bundle-source-hashes.txt")
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("firmware evidence: read bundle source hashes: %w", err)
	}
	got, err := parseSourceHashes(raw)
	if err != nil {
		return err
	}
	want := make(map[string]string, len(evidence.BundleFiles)+1)
	want[evidence.DependencyLock.Path] = evidence.DependencyLock.SHA256
	for _, file := range evidence.BundleFiles {
		want[file.Path] = file.SHA256
	}
	if len(got) != len(want) {
		return fmt.Errorf("firmware evidence: bundle source inventory has %d files, want %d",
			len(got), len(want))
	}
	for path, hash := range want {
		if got[path] != hash {
			return fmt.Errorf("firmware evidence: bundle source binding %q = %q, want %q",
				path, got[path], hash)
		}
	}
	return nil
}

func parseSourceHashes(raw []byte) (map[string]string, error) {
	result := make(map[string]string)
	for lineNumber, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || !validSHA256(fields[0]) || !safeRelativePath(fields[1]) {
			return nil, fmt.Errorf("firmware evidence: invalid bundle hash line %d",
				lineNumber+1)
		}
		if _, exists := result[fields[1]]; exists {
			return nil, fmt.Errorf("firmware evidence: duplicate bundle hash path %q",
				fields[1])
		}
		result[fields[1]] = fields[0]
	}
	return result, nil
}

func verifyAppliedPatchCoverage(root string, evidence BuildEvidence) error {
	applied, err := os.ReadFile(filepath.Join(root,
		filepath.FromSlash(checkedInEvidenceRoot), "applied-source.patch"))
	if err != nil {
		return fmt.Errorf("firmware evidence: read applied source patch: %w", err)
	}
	want := make(map[string]struct{})
	const overlayPrefix = "firmware/openrtx-mvp10.3/overlay/"
	for _, file := range evidence.BundleFiles {
		if strings.HasPrefix(file.Path, overlayPrefix) {
			want[strings.TrimPrefix(file.Path, overlayPrefix)] = struct{}{}
		}
	}
	existing, err := os.ReadFile(filepath.Join(root,
		"firmware/openrtx-mvp10.3/existing-files.patch"))
	if err != nil {
		return fmt.Errorf("firmware evidence: read existing-files.patch: %w", err)
	}
	for _, path := range diffPaths(existing) {
		want[path] = struct{}{}
	}
	got := make(map[string]struct{})
	for _, path := range diffPaths(applied) {
		got[path] = struct{}{}
	}
	paths := make([]string, 0, len(want))
	for path := range want {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if _, ok := got[path]; !ok {
			return fmt.Errorf("firmware evidence: applied patch omits %q", path)
		}
	}
	return nil
}

func diffPaths(raw []byte) []string {
	var paths []string
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(line, "diff --git a/") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 4 && strings.HasPrefix(fields[2], "a/") &&
			strings.HasPrefix(fields[3], "b/") && fields[2][2:] == fields[3][2:] {
			paths = append(paths, fields[2][2:])
		}
	}
	return paths
}

func verifyCanonicalBindings(root string, bindings Bindings) error {
	handoffRaw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(bindings.Handoff.Path)))
	if err != nil {
		return fmt.Errorf("firmware evidence: read handoff: %w", err)
	}
	if got := sha256Hex(handoffRaw); got != bindings.Handoff.ContentSHA256 {
		return fmt.Errorf("firmware evidence: handoff content sha256 = %s", got)
	}
	handoff, err := radiobackend.DecodeContract(bytes.NewReader(handoffRaw))
	if err != nil {
		return err
	}
	handoffHash, err := radiobackend.ContractHash(handoff)
	if err != nil {
		return err
	}
	if handoffHash != bindings.Handoff.CanonicalSHA256 {
		return fmt.Errorf("firmware evidence: handoff canonical hash mismatch")
	}

	selectionRaw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(bindings.Selection.Path)))
	if err != nil {
		return fmt.Errorf("firmware evidence: read selection: %w", err)
	}
	if got := sha256Hex(selectionRaw); got != bindings.Selection.ContentSHA256 {
		return fmt.Errorf("firmware evidence: selection content sha256 = %s", got)
	}
	selection, err := radiobackend.DecodeSelectionReport(bytes.NewReader(selectionRaw))
	if err != nil {
		return err
	}
	selectionHash, err := radiobackend.SelectionReportHash(selection)
	if err != nil {
		return err
	}
	if selectionHash != bindings.Selection.CanonicalSHA256 {
		return fmt.Errorf("firmware evidence: selection canonical hash mismatch")
	}

	measurementRaw, err := os.ReadFile(filepath.Join(root,
		filepath.FromSlash(bindings.MeasurementContract.Path)))
	if err != nil {
		return fmt.Errorf("firmware evidence: read measurement contract: %w", err)
	}
	if got := sha256Hex(measurementRaw); got != bindings.MeasurementContract.ContentSHA256 {
		return fmt.Errorf("firmware evidence: measurement content sha256 = %s", got)
	}
	measurement, err := backendmeasurement.DecodeReport(bytes.NewReader(measurementRaw))
	if err != nil {
		return err
	}
	measurementHash, err := backendmeasurement.ReportHash(measurement)
	if err != nil {
		return err
	}
	if measurementHash != bindings.MeasurementContract.CanonicalSHA256 {
		return fmt.Errorf("firmware evidence: measurement canonical hash mismatch")
	}
	return nil
}

func verifyEvidenceContents(root, artifactRoot string) error {
	read := func(name string) ([]byte, error) {
		return os.ReadFile(filepath.Join(root, filepath.FromSlash(artifactRoot), name))
	}
	txGuard, err := read("tx-guard-disassembly.txt")
	if err != nil {
		return err
	}
	if !bytes.Contains(txGuard, []byte("<radio_enableTx>:")) ||
		!bytes.Contains(txGuard, []byte("bx\tlr")) {
		return fmt.Errorf("firmware evidence: TX guard is not immediate return")
	}
	usbTryWrite, err := read("usb-try-write-disassembly.txt")
	if err != nil {
		return err
	}
	if !bytes.Contains(usbTryWrite, []byte("<vcom_tryWriteBlock>:")) ||
		bytes.Contains(usbTryWrite, []byte("<delayMs>")) {
		return fmt.Errorf("firmware evidence: USB try-write is not non-blocking")
	}
	undefined, err := read("adhr-undefined-symbols.txt")
	if err != nil {
		return err
	}
	for _, forbidden := range []string{"codec2", "m17", "dmr", "ambe"} {
		if strings.Contains(strings.ToLower(string(undefined)), forbidden) {
			return fmt.Errorf("firmware evidence: boundary imports %s", forbidden)
		}
	}
	protocol, err := read("adhr-protocol-strings.txt")
	if err != nil {
		return err
	}
	for _, required := range []string{
		"ADHR_RX_ONLY:v1", "receive_delivery", "link_measurement",
		"timing_observation", "unsupported_operation", "command_timeout",
	} {
		if !bytes.Contains(protocol, []byte(required)) {
			return fmt.Errorf("firmware evidence: missing protocol string %q", required)
		}
	}
	return nil
}

func verifyFile(root, relativePath, expectedHash string, expectedSize int64) error {
	if !safeRelativePath(relativePath) {
		return fmt.Errorf("firmware evidence: unsafe artifact path %q", relativePath)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relativePath)))
	if err != nil {
		return fmt.Errorf("firmware evidence: read %q: %w", relativePath, err)
	}
	if expectedSize >= 0 && int64(len(raw)) != expectedSize {
		return fmt.Errorf("firmware evidence: %q size = %d, want %d",
			relativePath, len(raw), expectedSize)
	}
	if got := sha256Hex(raw); got != expectedHash {
		return fmt.Errorf("firmware evidence: %q sha256 = %s, want %s",
			relativePath, got, expectedHash)
	}
	return nil
}

func sha256Hex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
