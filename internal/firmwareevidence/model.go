package firmwareevidence

const (
	BuildSchemaVersion    = "adhr-openrtx-rxonly-build-evidence-v1"
	RollbackSchemaVersion = "adhr-firmware-rollback-package-v1"
	DependencyLockSchema  = "adhr-openrtx-dependency-lock-v1"
	StoryID               = "MVP-10.3"
	CandidateID           = "openrtx_mduv3x0"
)

type BuildEvidence struct {
	SchemaVersion     string            `json:"schema_version"`
	StoryID           string            `json:"story_id"`
	CandidateID       string            `json:"candidate_id"`
	Status            string            `json:"status"`
	GeneratedAt       string            `json:"generated_at"`
	Bindings          Bindings          `json:"bindings"`
	Source            Source            `json:"source"`
	DependencyLock    DependencyLock    `json:"dependency_lock"`
	BundleFiles       []FileRef         `json:"bundle_files"`
	BuildEnvironment  BuildEnvironment  `json:"build_environment"`
	BuildCommand      string            `json:"build_command"`
	Runs              []BuildRun        `json:"runs"`
	Reproducibility   Reproducibility   `json:"reproducibility"`
	ResourceUsage     ResourceUsage     `json:"resource_usage"`
	BoundaryEvidence  BoundaryEvidence  `json:"boundary_evidence"`
	CanonicalEvidence []NamedEvidence   `json:"canonical_evidence"`
	RollbackPackage   RollbackReference `json:"rollback_package"`
	Safety            Safety            `json:"safety"`
}

type Bindings struct {
	Handoff             CanonicalRef `json:"handoff"`
	Selection           CanonicalRef `json:"selection"`
	MeasurementContract CanonicalRef `json:"measurement_contract"`
}

type CanonicalRef struct {
	Path            string `json:"path"`
	CanonicalSHA256 string `json:"canonical_sha256"`
	ContentSHA256   string `json:"content_sha256"`
}

type Source struct {
	Repo            string `json:"repo"`
	Tag             string `json:"tag"`
	Commit          string `json:"commit"`
	Target          string `json:"target"`
	CrossFile       string `json:"cross_file"`
	SourceDateEpoch int64  `json:"source_date_epoch"`
}

type DependencyLock struct {
	Path         string       `json:"path"`
	SHA256       string       `json:"sha256"`
	WrapMode     string       `json:"wrap_mode"`
	Dependencies []Dependency `json:"dependencies"`
}

type Dependency struct {
	Name           string `json:"name"`
	Repo           string `json:"repo"`
	Commit         string `json:"commit"`
	ContentSHA256  string `json:"content_sha256"`
	PackageOverlay string `json:"package_overlay"`
}

type DependencyLockDocument struct {
	SchemaVersion string       `json:"schema_version"`
	OpenRTXCommit string       `json:"openrtx_commit"`
	Dependencies  []Dependency `json:"dependencies"`
}

type FileRef struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type BuildEnvironment struct {
	Isolation          string                    `json:"isolation"`
	BuilderImage       string                    `json:"builder_image"`
	BuilderImageDigest string                    `json:"builder_image_digest"`
	Meson              string                    `json:"meson"`
	MiosixGCC          string                    `json:"miosix_gcc"`
	Binutils           string                    `json:"binutils"`
	ContainerSource    string                    `json:"container_source_path"`
	Network            string                    `json:"network"`
	USBDevicesExposed  bool                      `json:"usb_devices_exposed"`
	Environment        BuildEnvironmentVariables `json:"environment"`
}

type BuildEnvironmentVariables struct {
	Lang            string `json:"LANG"`
	LCAll           string `json:"LC_ALL"`
	PythonHashSeed  string `json:"PYTHONHASHSEED"`
	SourceDateEpoch string `json:"SOURCE_DATE_EPOCH"`
	TZ              string `json:"TZ"`
	ZeroARDate      string `json:"ZERO_AR_DATE"`
}

type BuildRun struct {
	RunID          string          `json:"run_id"`
	CleanCheckout  bool            `json:"clean_checkout"`
	ArtifactRoot   string          `json:"artifact_root"`
	Artifacts      []BuildArtifact `json:"artifacts"`
	BuildLogSHA256 string          `json:"build_log_sha256"`
}

type BuildArtifact struct {
	Kind      string `json:"kind"`
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

type Reproducibility struct {
	Verdict             string   `json:"verdict"`
	IdenticalArtifacts  []string `json:"identical_artifacts"`
	BuildLogsIdentical  bool     `json:"build_logs_identical"`
	BuildLogDriftReason string   `json:"build_log_drift_reason"`
}

type ResourceUsage struct {
	TextBytes            int64         `json:"text_bytes"`
	DataBytes            int64         `json:"data_bytes"`
	BSSBytes             int64         `json:"bss_bytes"`
	SmallRAMStaticBytes  int64         `json:"smallram_static_bytes"`
	LargeRAMStaticBytes  int64         `json:"largeram_static_bytes"`
	FirmwareBinBytes     int64         `json:"firmware_bin_bytes"`
	WrappedFirmwareBytes int64         `json:"wrapped_firmware_bytes"`
	BaselineDelta        ResourceDelta `json:"baseline_delta"`
	Remaining            Remaining     `json:"remaining"`
	PlanningLimitsPassed bool          `json:"planning_limits_passed"`
}

type ResourceDelta struct {
	TextBytes            int64 `json:"text_bytes"`
	DataBytes            int64 `json:"data_bytes"`
	BSSBytes             int64 `json:"bss_bytes"`
	SmallRAMStaticBytes  int64 `json:"smallram_static_bytes"`
	LargeRAMStaticBytes  int64 `json:"largeram_static_bytes"`
	FirmwareBinBytes     int64 `json:"firmware_bin_bytes"`
	WrappedFirmwareBytes int64 `json:"wrapped_firmware_bytes"`
}

type Remaining struct {
	FlashBytes    int64 `json:"flash_bytes"`
	SmallRAMBytes int64 `json:"smallram_bytes"`
	LargeRAMBytes int64 `json:"largeram_bytes"`
}

type BoundaryEvidence struct {
	ProtocolVersion                string   `json:"protocol_version"`
	MaxLineBytes                   int      `json:"max_line_bytes"`
	MaxFrameBytes                  int      `json:"max_frame_bytes"`
	PartialCommandTimeoutMS        int      `json:"partial_command_timeout_ms"`
	NewThreadCreated               bool     `json:"new_thread_created"`
	HeapUsed                       bool     `json:"heap_used"`
	HostTestsPassed                bool     `json:"host_tests_passed"`
	TXGuards                       []string `json:"tx_guards"`
	NewBoundaryCodecImports        []string `json:"new_boundary_codec_imports"`
	NewBoundaryCodecSymbols        []string `json:"new_boundary_codec_symbols"`
	BroadTargetCodecCoupling       string   `json:"broad_target_codec_coupling"`
	BroadTargetCodecCouplingReason string   `json:"broad_target_codec_coupling_reason"`
}

type NamedEvidence struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}

type RollbackReference struct {
	Path          string `json:"path"`
	SHA256        string `json:"sha256"`
	ExecutionGate string `json:"execution_gate"`
}

type Safety struct {
	ReceiveOnlyDefault     bool `json:"receive_only_default"`
	TXEnabledByDefault     bool `json:"tx_enabled_by_default"`
	FirmwareWriteAllowed   bool `json:"firmware_write_allowed"`
	HardwareClaimAllowed   bool `json:"hardware_claim_allowed"`
	RFSynchronizationClaim bool `json:"rf_synchronization_claim"`
	FieldTXReady           bool `json:"field_tx_ready"`
	StationWritePerformed  bool `json:"station_write_performed"`
	USBDeviceAccess        bool `json:"usb_device_access_performed"`
}

type RollbackPackage struct {
	SchemaVersion          string                 `json:"schema_version"`
	StoryID                string                 `json:"story_id"`
	CandidateID            string                 `json:"candidate_id"`
	Status                 string                 `json:"status"`
	GeneratedAt            string                 `json:"generated_at"`
	ExactBuildOutput       ExactBuildOutput       `json:"exact_build_output"`
	RollbackInputInventory RollbackInputInventory `json:"rollback_input_inventory"`
	Recovery               Recovery               `json:"recovery"`
	Unproven               []string               `json:"unproven"`
	Claims                 RollbackClaims         `json:"claims"`
}

type ExactBuildOutput struct {
	SourceCommit          string `json:"source_commit"`
	Target                string `json:"target"`
	FirmwareBinSHA256     string `json:"firmware_bin_sha256"`
	WrappedFirmwareSHA256 string `json:"wrapped_firmware_sha256"`
}

type RollbackInputInventory struct {
	Path                    string `json:"path"`
	SHA256                  string `json:"sha256"`
	ObservedIntakeFirmware  string `json:"observed_intake_firmware"`
	ObservedCodeplugVersion string `json:"observed_intake_codeplug_version"`
	SameFamilyFirmware      string `json:"same_family_recovery_firmware"`
}

type Recovery struct {
	Mode                         string   `json:"mode"`
	ProcedureRefs                []string `json:"procedure_refs"`
	SameFamilyStockWriteObserved bool     `json:"same_family_stock_write_previously_observed"`
	CodeplugReadbackAvailable    bool     `json:"codeplug_readback_available"`
	ExecutionStory               string   `json:"execution_story"`
}

type RollbackClaims struct {
	RollbackInputsAvailable    bool `json:"rollback_inputs_available"`
	PackageCompleteForDryRun   bool `json:"rollback_package_complete_for_dry_run"`
	ExecutedForThisBuild       bool `json:"rollback_executed_for_this_build"`
	ExactIntakeRoundTripProven bool `json:"exact_intake_firmware_round_trip_proven"`
	CalibrationRoundTripProven bool `json:"calibration_round_trip_proven"`
	EndToEndGatePassed         bool `json:"end_to_end_rollback_gate_passed"`
	FirmwareWriteAllowed       bool `json:"firmware_write_allowed"`
	HardwareClaimAllowed       bool `json:"hardware_claim_allowed"`
	FieldTXReady               bool `json:"field_tx_ready"`
}
