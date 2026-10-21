package firmwareevidence

import (
	"encoding/hex"
	"fmt"
	"path"
	"reflect"
	"strings"
)

const (
	expectedSourceCommit      = "801fc8dd313eb3ac6f83fd4d3b63ed895effdd56"
	expectedBinSHA256         = "d1cf3c92afa1bec4c32d9407fb9a149155e8ded85f9d96a9c5d57a2784c2c4aa"
	expectedWrapSHA256        = "31399b78bf65016b9a2a6186dd6cac471c9855a84b252dde84cd1ee7120ffc4e"
	expectedHandoffSHA256     = "d97a01b0160fe2c125574ffc1c1125a9da20c2d20e0508d35d565977d38ec39a"
	expectedSelectionSHA256   = "e83f4b00fa43e75b8483d6158fa30951c2ba956893fe9269e9f8cef3d81a630b"
	expectedMeasurementSHA256 = "1f217ed60af3b8ac67588d8f0bb605c0fb66c955af3d86641214d6d08ff03227"
)

var expectedBundlePaths = []string{
	"firmware/openrtx-mvp10.3/existing-files.patch",
	"firmware/openrtx-mvp10.3/overlay/openrtx/include/rtx/adhr_rxonly.h",
	"firmware/openrtx-mvp10.3/overlay/openrtx/include/rtx/adhr_tx_queue.h",
	"firmware/openrtx-mvp10.3/overlay/openrtx/include/rtx/adhr_usb_rxonly.h",
	"firmware/openrtx-mvp10.3/overlay/openrtx/src/rtx/adhr_rxonly.c",
	"firmware/openrtx-mvp10.3/overlay/openrtx/src/rtx/adhr_tx_queue.c",
	"firmware/openrtx-mvp10.3/overlay/openrtx/src/rtx/adhr_usb_rxonly.c",
	"firmware/openrtx-mvp10.3/host/adhr_rxonly_host_test.c",
	"firmware/openrtx-mvp10.3/host/adhr_usb_rxonly_host_test.c",
	"firmware/openrtx-mvp10.3/host/stubs/drivers/usb_vcom.h",
	"firmware/openrtx-mvp10.3/host/stubs/interfaces/delays.h",
	"firmware/openrtx-mvp10.3/run-host-tests.sh",
	"firmware/openrtx-mvp10.3/prepare-dependencies.sh",
	"firmware/openrtx-mvp10.3/build-isolated.sh",
}

func ValidateBuildEvidence(evidence BuildEvidence) error {
	if evidence.SchemaVersion != BuildSchemaVersion || evidence.StoryID != StoryID ||
		evidence.CandidateID != CandidateID {
		return fmt.Errorf("firmware evidence: invalid identity")
	}
	if evidence.Status != "build_verified_not_flashed" || evidence.GeneratedAt != "2026-07-12" {
		return fmt.Errorf("firmware evidence: invalid status or date")
	}
	if err := validateBindings(evidence.Bindings); err != nil {
		return err
	}
	if err := validateSourceAndDependencies(evidence); err != nil {
		return err
	}
	if err := validateBundle(evidence.BundleFiles); err != nil {
		return err
	}
	if err := validateBuildEnvironment(evidence.BuildEnvironment); err != nil {
		return err
	}
	if evidence.BuildCommand != "firmware/openrtx-mvp10.3/build-isolated.sh RUN_ID" {
		return fmt.Errorf("firmware evidence: invalid build_command")
	}
	if err := validateRuns(evidence.Runs); err != nil {
		return err
	}
	if err := validateReproducibility(evidence.Reproducibility); err != nil {
		return err
	}
	if err := validateResources(evidence.ResourceUsage); err != nil {
		return err
	}
	if err := validateBoundary(evidence.BoundaryEvidence); err != nil {
		return err
	}
	if err := validateCanonicalEvidence(evidence.CanonicalEvidence); err != nil {
		return err
	}
	if evidence.RollbackPackage.Path != "configs/firmware-contract/openrtx-mduv3x0-adhr-rxonly-rollback.json" ||
		!validSHA256(evidence.RollbackPackage.SHA256) ||
		evidence.RollbackPackage.ExecutionGate != "MVP-10.4" {
		return fmt.Errorf("firmware evidence: invalid rollback_package")
	}
	if !evidence.Safety.ReceiveOnlyDefault || evidence.Safety.TXEnabledByDefault ||
		evidence.Safety.FirmwareWriteAllowed || evidence.Safety.HardwareClaimAllowed ||
		evidence.Safety.RFSynchronizationClaim || evidence.Safety.FieldTXReady ||
		evidence.Safety.StationWritePerformed || evidence.Safety.USBDeviceAccess {
		return fmt.Errorf("firmware evidence: unsafe claims")
	}
	return nil
}

func ValidateRollbackPackage(rollback RollbackPackage) error {
	if rollback.SchemaVersion != RollbackSchemaVersion || rollback.StoryID != StoryID ||
		rollback.CandidateID != CandidateID || rollback.Status != "packaged_not_executed" {
		return fmt.Errorf("firmware rollback: invalid identity or status")
	}
	if rollback.GeneratedAt != "2026-07-12" ||
		rollback.ExactBuildOutput.SourceCommit != expectedSourceCommit ||
		rollback.ExactBuildOutput.Target != "mduv3x0" ||
		rollback.ExactBuildOutput.FirmwareBinSHA256 != expectedBinSHA256 ||
		rollback.ExactBuildOutput.WrappedFirmwareSHA256 != expectedWrapSHA256 {
		return fmt.Errorf("firmware rollback: exact build output mismatch")
	}
	input := rollback.RollbackInputInventory
	if input.Path != "configs/firmware-contract/md-uv380-rollback-inputs-20260712.json" ||
		!validSHA256(input.SHA256) || input.ObservedIntakeFirmware != "D219.043" ||
		input.ObservedCodeplugVersion != "V02.37" ||
		input.SameFamilyFirmware != "MD-UV380-AES255(REC)-D219.74" {
		return fmt.Errorf("firmware rollback: invalid input inventory")
	}
	if rollback.Recovery.Mode != "TYT firmware update/DFU mode" ||
		rollback.Recovery.ExecutionStory != "MVP-10.4" ||
		!rollback.Recovery.SameFamilyStockWriteObserved ||
		!rollback.Recovery.CodeplugReadbackAvailable || len(rollback.Recovery.ProcedureRefs) != 3 {
		return fmt.Errorf("firmware rollback: invalid recovery procedure")
	}
	if len(rollback.Unproven) != 4 {
		return fmt.Errorf("firmware rollback: missing unproven limitations")
	}
	claims := rollback.Claims
	if !claims.RollbackInputsAvailable || !claims.PackageCompleteForDryRun ||
		claims.ExecutedForThisBuild || claims.ExactIntakeRoundTripProven ||
		claims.CalibrationRoundTripProven || claims.EndToEndGatePassed ||
		claims.FirmwareWriteAllowed || claims.HardwareClaimAllowed || claims.FieldTXReady {
		return fmt.Errorf("firmware rollback: unsafe claims")
	}
	return nil
}

func ValidateDependencyLock(lock DependencyLockDocument) error {
	if lock.SchemaVersion != DependencyLockSchema ||
		lock.OpenRTXCommit != expectedSourceCommit {
		return fmt.Errorf("firmware dependency lock: invalid identity")
	}
	wants := expectedDependencies()
	if !reflect.DeepEqual(lock.Dependencies, wants) {
		return fmt.Errorf("firmware dependency lock: dependency inventory mismatch")
	}
	return nil
}

func validateBindings(bindings Bindings) error {
	wants := []struct {
		ref  CanonicalRef
		path string
		hash string
	}{
		{bindings.Handoff, "configs/firmware-contract/mvp9-radio-backend-handoff.json", expectedHandoffSHA256},
		{bindings.Selection, "configs/firmware-contract/mvp10-backend-selection.json", expectedSelectionSHA256},
		{bindings.MeasurementContract, "configs/firmware-contract/mvp10-backend-measurement-contract.json", expectedMeasurementSHA256},
	}
	for _, want := range wants {
		if want.ref.Path != want.path || want.ref.CanonicalSHA256 != want.hash ||
			!validSHA256(want.ref.ContentSHA256) {
			return fmt.Errorf("firmware evidence: invalid binding %s", want.path)
		}
	}
	return nil
}

func validateSourceAndDependencies(evidence BuildEvidence) error {
	source := evidence.Source
	if source.Repo != "https://github.com/OpenRTX/OpenRTX.git" || source.Tag != "v0.4.4" ||
		source.Commit != expectedSourceCommit || source.Target != "mduv3x0" ||
		source.CrossFile != "cross_cm4.txt" || source.SourceDateEpoch != 1777626376 {
		return fmt.Errorf("firmware evidence: invalid source pin")
	}
	lock := evidence.DependencyLock
	if lock.Path != "firmware/openrtx-mvp10.3/dependency-lock.json" ||
		!validSHA256(lock.SHA256) || lock.WrapMode != "nodownload" || len(lock.Dependencies) != 3 {
		return fmt.Errorf("firmware evidence: invalid dependency lock")
	}
	if !reflect.DeepEqual(lock.Dependencies, expectedDependencies()) {
		return fmt.Errorf("firmware evidence: invalid dependency inventory")
	}
	return nil
}

func expectedDependencies() []Dependency {
	return []Dependency{
		{
			Name: "codec2", Repo: "https://github.com/drowe67/codec2-dev",
			Commit:         "a298f3789d7ef9e829049ebe14da8845d97ba8c1",
			ContentSHA256:  "c1ad820af8063ee2b638c44c4312a8739b1fe629067326b09c709ce1ce4411ed",
			PackageOverlay: "subprojects/packagefiles/codec2",
		},
		{
			Name: "XPowersLib", Repo: "https://github.com/lewisxhe/XPowersLib.git",
			Commit:         "b0ed896c17d49a1b823ba0a6323cafd89b3a7d91",
			ContentSHA256:  "81e3c526eb0d39e7f7fd3d117924020a1d54ac494f78267c663e0f11cd286eea",
			PackageOverlay: "subprojects/packagefiles/XPowersLib",
		},
		{
			Name: "radio_tool", Repo: "https://github.com/v0l/radio_tool",
			Commit:         "6fe15235814a69a0b43f840935000dd5642aa2f4",
			ContentSHA256:  "04403bf468a6f7105b52167c8830fd1215b3ad262d9792931136f57c68016963",
			PackageOverlay: "subprojects/packagefiles/radio_tool",
		},
	}
}

func validateBundle(files []FileRef) error {
	if len(files) != len(expectedBundlePaths) {
		return fmt.Errorf("firmware evidence: bundle inventory mismatch")
	}
	for i, file := range files {
		if file.Path != expectedBundlePaths[i] || !safeRelativePath(file.Path) ||
			!validSHA256(file.SHA256) {
			return fmt.Errorf("firmware evidence: invalid bundle file %q", file.Path)
		}
	}
	return nil
}

func validateBuildEnvironment(environment BuildEnvironment) error {
	vars := BuildEnvironmentVariables{"C", "C", "0", "1777626376", "UTC", "1"}
	if environment.Isolation != "docker linux/amd64" ||
		environment.BuilderImage != "adhr-openrtx-builder:ubuntu24-miosix9.2" ||
		environment.BuilderImageDigest != "sha256:6a25272b232219b3b7201696125c32a54e0c6d018bca823f3a44a87a8f70a93e" ||
		environment.Meson != "1.3.2" || environment.MiosixGCC != "9.2.0" ||
		environment.Binutils != "2.32" || environment.ContainerSource != "/src" ||
		environment.Network != "none" || environment.USBDevicesExposed ||
		environment.Environment != vars {
		return fmt.Errorf("firmware evidence: non-reproducible build environment")
	}
	return nil
}

func validateRuns(runs []BuildRun) error {
	if len(runs) != 2 || runs[0].RunID != "build-a" || runs[1].RunID != "build-b" {
		return fmt.Errorf("firmware evidence: requires build-a and build-b")
	}
	artifactKinds := []string{"elf", "bin", "wrap", "map", "symbols", "disassembly"}
	artifactNames := []string{
		"openrtx_mduv3x0.elf", "openrtx_mduv3x0.bin",
		"openrtx_mduv3x0.wrap", "openrtx_mduv3x0.map",
		"symbols.txt", "disassembly.txt",
	}
	for runIndex, run := range runs {
		wantRoot := ".artifacts/openrtx-mvp10.3/" + run.RunID
		if !run.CleanCheckout || !safeRelativePath(run.ArtifactRoot) ||
			run.ArtifactRoot != wantRoot || !validSHA256(run.BuildLogSHA256) ||
			len(run.Artifacts) != len(artifactKinds) {
			return fmt.Errorf("firmware evidence: invalid run %s", run.RunID)
		}
		for artifactIndex, artifact := range run.Artifacts {
			if !safeRelativePath(artifact.Path) || artifact.SizeBytes <= 0 ||
				!validSHA256(artifact.SHA256) || artifact.Kind != artifactKinds[artifactIndex] ||
				artifact.Path != wantRoot+"/"+artifactNames[artifactIndex] {
				return fmt.Errorf("firmware evidence: invalid artifact in %s", run.RunID)
			}
			if runIndex > 0 {
				first := runs[0].Artifacts[artifactIndex]
				if artifact.Kind != first.Kind || artifact.SizeBytes != first.SizeBytes ||
					artifact.SHA256 != first.SHA256 {
					return fmt.Errorf("firmware evidence: reproducibility drift for %s", artifact.Kind)
				}
			}
		}
	}
	if runs[0].Artifacts[1].SHA256 != expectedBinSHA256 ||
		runs[0].Artifacts[2].SHA256 != expectedWrapSHA256 {
		return fmt.Errorf("firmware evidence: exact firmware output mismatch")
	}
	return nil
}

func validateReproducibility(repro Reproducibility) error {
	wants := []string{
		"elf", "bin", "wrap", "map", "size", "sections", "symbols",
		"disassembly", "ADHR object undefined symbols", "TX guard disassembly",
		"USB try-write disassembly", "ADHR protocol strings", "toolchain",
		"applied source patch", "bundle source hashes", "source isolation",
	}
	if repro.Verdict != "exact_match" ||
		!reflect.DeepEqual(repro.IdenticalArtifacts, wants) ||
		repro.BuildLogsIdentical || strings.TrimSpace(repro.BuildLogDriftReason) == "" {
		return fmt.Errorf("firmware evidence: invalid reproducibility verdict")
	}
	return nil
}

func validateResources(resources ResourceUsage) error {
	if resources.TextBytes != 236024 || resources.DataBytes != 1720 ||
		resources.BSSBytes != 56160 || resources.SmallRAMStaticBytes != 16920 ||
		resources.LargeRAMStaticBytes != 40960 || resources.FirmwareBinBytes != 237744 ||
		resources.WrappedFirmwareBytes != 238256 || !resources.PlanningLimitsPassed {
		return fmt.Errorf("firmware evidence: invalid resource measurement")
	}
	wantDelta := ResourceDelta{15428, 8, 3000, 3008, 0, 15432, 15432}
	wantRemaining := Remaining{630608, 48616, 89600}
	if resources.BaselineDelta != wantDelta || resources.Remaining != wantRemaining {
		return fmt.Errorf("firmware evidence: invalid resource arithmetic")
	}
	return nil
}

func validateBoundary(boundary BoundaryEvidence) error {
	wantGuards := []string{
		"initial rtxStatus.txDisable=1",
		"configured rtxStatus.txDisable=1",
		"radio_enableTx compiles to bx lr",
		"scheduled_transmit returns unsupported_operation",
	}
	if boundary.ProtocolVersion != "ADHR1" || boundary.MaxLineBytes != 384 ||
		boundary.MaxFrameBytes != 128 || boundary.PartialCommandTimeoutMS != 1000 ||
		boundary.NewThreadCreated || boundary.HeapUsed || !boundary.HostTestsPassed ||
		!reflect.DeepEqual(boundary.TXGuards, wantGuards) ||
		len(boundary.NewBoundaryCodecImports) != 0 || len(boundary.NewBoundaryCodecSymbols) != 0 ||
		boundary.BroadTargetCodecCoupling != "deferred" ||
		strings.TrimSpace(boundary.BroadTargetCodecCouplingReason) == "" {
		return fmt.Errorf("firmware evidence: invalid receive-only boundary")
	}
	return nil
}

func validateCanonicalEvidence(evidence []NamedEvidence) error {
	wants := []string{
		"bundle_source_hashes", "applied_source_patch", "undefined_symbols",
		"tx_guard_disassembly", "usb_try_write_disassembly", "protocol_strings",
		"source_isolation", "size", "sections", "toolchain",
	}
	if len(evidence) != len(wants) {
		return fmt.Errorf("firmware evidence: canonical evidence inventory mismatch")
	}
	for i, want := range wants {
		if evidence[i].Name != want || !validSHA256(evidence[i].SHA256) {
			return fmt.Errorf("firmware evidence: invalid canonical evidence %s", want)
		}
	}
	return nil
}

func safeRelativePath(value string) bool {
	return value != "" && !strings.HasPrefix(value, "/") &&
		path.Clean(value) == value && value != "." && !strings.HasPrefix(value, "../")
}

func validSHA256(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && value == strings.ToLower(value)
}
