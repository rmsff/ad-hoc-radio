// Package backendmeasurement defines deterministic bench-evidence contracts.
// It does not implement radio control, routing, scheduling, or RF behavior.
package backendmeasurement

const (
	SchemaVersion          = "adhr-backend-measurement-v1"
	StoryID                = "MVP-10.2"
	ThresholdVersion       = "adhr-backend-thresholds-v1"
	FrozenHandoffSHA256    = "d97a01b0160fe2c125574ffc1c1125a9da20c2d20e0508d35d565977d38ec39a"
	FrozenSelectionSHA256  = "e83f4b00fa43e75b8483d6158fa30951c2ba956893fe9269e9f8cef3d81a630b"
	SelectedCandidateID    = "openrtx_mduv3x0"
	SelectedProfileID      = "source_led/project_native_variable"
	SelectedFirmwareBase   = "OpenRTX v0.4.4"
	SelectedSourceRef      = "https://github.com/OpenRTX/OpenRTX.git"
	SelectedSourceRevision = "801fc8dd313eb3ac6f83fd4d3b63ed895effdd56"
	SelectedTarget         = "mduv3x0"
	UnknownBackendClockID  = "unknown"
)

type MeasurementReport struct {
	SchemaVersion         string                `json:"schema_version"`
	StoryID               string                `json:"story_id"`
	HandoffContractSHA256 string                `json:"handoff_contract_sha256"`
	SelectionReportSHA256 string                `json:"selection_report_sha256"`
	Candidate             CandidateIdentity     `json:"candidate"`
	Definition            MeasurementDefinition `json:"definition"`
	Clock                 ClockDefinition       `json:"clock"`
	Units                 Units                 `json:"units"`
	TrialPlan             TrialPlan             `json:"trial_plan"`
	Thresholds            *ThresholdSet         `json:"thresholds"`
	EvidenceGrade         EvidenceGrade         `json:"evidence_grade"`
	RawArtifacts          []ArtifactRef         `json:"raw_artifacts"`
	Trials                []Trial               `json:"trials"`
	Summary               MeasurementSummary    `json:"summary"`
	Verdict               Verdict               `json:"verdict"`
	NoPassReasons         []string              `json:"no_pass_reasons"`
	Safety                SafetyBoundary        `json:"safety"`
}

type CandidateIdentity struct {
	CandidateID    string `json:"candidate_id"`
	ProfileID      string `json:"profile_id"`
	FirmwareBase   string `json:"firmware_base"`
	SourceRef      string `json:"source_ref"`
	SourceRevision string `json:"source_revision"`
	Target         string `json:"target"`
	FirmwareSHA256 string `json:"firmware_sha256"`
	BinarySHA256   string `json:"binary_sha256"`
}

type GateName string

const (
	GateChannelCapacity   GateName = "channel_capacity"
	GateTXRXTurnaround    GateName = "tx_rx_turnaround"
	GateTimingAcquisition GateName = "timing_acquisition"
)

type MeasurementKind string

const (
	KindChannelCapacity   MeasurementKind = "channel_capacity"
	KindRXToTXTurnaround  MeasurementKind = "rx_to_tx_turnaround"
	KindTXToRXTurnaround  MeasurementKind = "tx_to_rx_turnaround"
	KindTimingAcquisition MeasurementKind = "timing_acquisition"
)

type EndpointMarker string

const (
	EndpointExternalTestFrameStart       EndpointMarker = "external_test_frame_start"
	EndpointValidReceiverFrameCompletion EndpointMarker = "valid_receiver_frame_completion"
	EndpointLastValidExternalRXMarker    EndpointMarker = "last_valid_external_rx_marker"
	EndpointFirstValidTXMarker           EndpointMarker = "first_valid_tx_marker"
	EndpointLastValidTXMarker            EndpointMarker = "last_valid_tx_marker"
	EndpointFirstValidRXMarker           EndpointMarker = "first_valid_rx_marker"
	EndpointSourceSyncReference          EndpointMarker = "source_sync_reference"
	EndpointBackendEpochLock             EndpointMarker = "backend_epoch_lock"
)

type MeasurementDefinition struct {
	GateName    GateName        `json:"gate_name"`
	Kind        MeasurementKind `json:"kind"`
	StartMarker EndpointMarker  `json:"start_marker"`
	EndMarker   EndpointMarker  `json:"end_marker"`
}

type ClockStatus string

const (
	ClockKnown   ClockStatus = "known"
	ClockUnknown ClockStatus = "unknown"
)

type ClockDefinition struct {
	Status         ClockStatus `json:"status"`
	BackendClockID string      `json:"backend_clock_id"`
	ResolutionNS   uint64      `json:"resolution_ns"`
}

type Units struct {
	Duration      string `json:"duration"`
	Payload       string `json:"payload"`
	Overhead      string `json:"overhead"`
	BitErrors     string `json:"bit_errors"`
	FrameErrors   string `json:"frame_errors"`
	FramesLost    string `json:"frames_lost"`
	BoundaryError string `json:"boundary_error"`
	Drift         string `json:"drift"`
	Guard         string `json:"guard"`
	Resync        string `json:"resync"`
}

func CanonicalUnits() Units {
	return Units{
		Duration:      "microseconds",
		Payload:       "bytes",
		Overhead:      "bytes",
		BitErrors:     "count",
		FrameErrors:   "count",
		FramesLost:    "count",
		BoundaryError: "microseconds",
		Drift:         "milli_ppm",
		Guard:         "microseconds",
		Resync:        "count",
	}
}

type TrialPlan struct {
	PlannedTrialCount     uint32       `json:"planned_trial_count"`
	MinimumSampleCount    uint32       `json:"minimum_sample_count"`
	MinimumSamplesPerCase uint32       `json:"minimum_samples_per_case"`
	WarmupCount           uint32       `json:"warmup_count"`
	MatrixCases           []MatrixCase `json:"matrix_cases"`
}

type MatrixCase struct {
	CaseID          string             `json:"case_id"`
	ProfileID       string             `json:"profile_id"`
	StateCondition  string             `json:"state_condition"`
	PowerProfile    string             `json:"power_profile"`
	ExpectedOutcome string             `json:"expected_outcome"`
	Channel         *ChannelMatrixCase `json:"channel"`
}

type ChannelMatrixCase struct {
	ChannelRateBPS     uint64 `json:"channel_rate_bps"`
	PayloadBytes       uint64 `json:"payload_bytes"`
	OverheadBytes      uint64 `json:"overhead_bytes"`
	RequiredFrameBytes uint64 `json:"required_frame_bytes"`
	GuardMarginBytes   uint64 `json:"guard_margin_bytes"`
}

type ThresholdSet struct {
	Version  string            `json:"version"`
	Duration DurationThreshold `json:"duration"`
	Channel  *ChannelThreshold `json:"channel"`
	Timing   *TimingThreshold  `json:"timing"`
}

type DurationThreshold struct {
	P95MaxUS         uint64 `json:"p95_max_us"`
	MaxUS            uint64 `json:"max_us"`
	MaxFailedSamples uint32 `json:"max_failed_samples"`
}

type ChannelThreshold struct {
	MaxBitErrors   uint64 `json:"max_bit_errors"`
	MaxFrameErrors uint64 `json:"max_frame_errors"`
	MaxFramesLost  uint64 `json:"max_frames_lost"`
}

type TimingThreshold struct {
	MaxAbsBoundaryErrorUS uint64 `json:"max_abs_boundary_error_us"`
	MaxAbsDriftMilliPPM   uint64 `json:"max_abs_drift_milli_ppm"`
	MaxGuardUsedUS        uint64 `json:"max_guard_used_us"`
	MaxResyncCount        uint64 `json:"max_resync_count"`
}

type EvidenceGrade string

const (
	EvidenceFixture        EvidenceGrade = "fixture"
	EvidenceSourceReviewed EvidenceGrade = "source_reviewed"
	EvidenceBuildVerified  EvidenceGrade = "build_verified"
	EvidenceBenchMeasured  EvidenceGrade = "bench_measured"
)

type ArtifactRef struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type TrialStatus string

const (
	TrialSucceeded TrialStatus = "succeeded"
	TrialFailed    TrialStatus = "failed"
)

type Trial struct {
	TrialID       string              `json:"trial_id"`
	CaseID        string              `json:"case_id"`
	Warmup        bool                `json:"warmup"`
	Status        TrialStatus         `json:"status"`
	DurationUS    uint64              `json:"duration_us"`
	FailureReason string              `json:"failure_reason"`
	Channel       *ChannelObservation `json:"channel"`
	Timing        *TimingObservation  `json:"timing"`
}

type ChannelObservation struct {
	ProfileID      string `json:"profile_id"`
	ChannelRateBPS uint64 `json:"channel_rate_bps"`
	PayloadBytes   uint64 `json:"payload_bytes"`
	OverheadBytes  uint64 `json:"overhead_bytes"`
	BitErrors      uint64 `json:"bit_errors"`
	FrameErrors    uint64 `json:"frame_errors"`
	FramesLost     uint64 `json:"frames_lost"`
}

type TimingObservation struct {
	BoundaryErrorUS int64  `json:"boundary_error_us"`
	DriftMilliPPM   int64  `json:"drift_milli_ppm"`
	GuardUsedUS     uint64 `json:"guard_used_us"`
	ResyncCount     uint64 `json:"resync_count"`
}

type MeasurementSummary struct {
	AttemptedSampleCount  uint32               `json:"attempted_sample_count"`
	SuccessfulSampleCount uint32               `json:"successful_sample_count"`
	FailedSampleCount     uint32               `json:"failed_sample_count"`
	WarmupCount           uint32               `json:"warmup_count"`
	P50US                 uint64               `json:"p50_us"`
	P95US                 uint64               `json:"p95_us"`
	MaxUS                 uint64               `json:"max_us"`
	FailureReasons        []FailureReasonCount `json:"failure_reasons"`
	Channel               *ChannelSummary      `json:"channel"`
	Timing                *TimingSummary       `json:"timing"`
}

type FailureReasonCount struct {
	Reason string `json:"reason"`
	Count  uint32 `json:"count"`
}

type ChannelSummary struct {
	PayloadBytes  uint64 `json:"payload_bytes"`
	OverheadBytes uint64 `json:"overhead_bytes"`
	BitErrors     uint64 `json:"bit_errors"`
	FrameErrors   uint64 `json:"frame_errors"`
	FramesLost    uint64 `json:"frames_lost"`
}

type TimingSummary struct {
	MaxAbsBoundaryErrorUS uint64 `json:"max_abs_boundary_error_us"`
	MaxAbsDriftMilliPPM   uint64 `json:"max_abs_drift_milli_ppm"`
	MaxGuardUsedUS        uint64 `json:"max_guard_used_us"`
	ResyncCount           uint64 `json:"resync_count"`
}

type Verdict string

const (
	VerdictNotEvaluated Verdict = "not_evaluated"
	VerdictPassed       Verdict = "passed"
	VerdictFailed       Verdict = "failed"
)

const (
	ReasonChannelCapacityInsufficient = "channel_capacity_insufficient"
	ReasonChannelThresholdExceeded    = "channel_threshold_exceeded"
	ReasonDurationThresholdExceeded   = "duration_threshold_exceeded"
	ReasonFailedSamplesExceeded       = "failed_samples_exceeded"
	ReasonInsufficientSamples         = "insufficient_samples"
	ReasonMissingRawArtifacts         = "missing_raw_artifacts"
	ReasonMissingThresholds           = "missing_thresholds"
	ReasonMatrixCoverageIncomplete    = "matrix_case_coverage_incomplete"
	ReasonNoSuccessfulSamples         = "no_successful_samples"
	ReasonNotBenchMeasured            = "not_bench_measured"
	ReasonTimingThresholdExceeded     = "timing_threshold_exceeded"
	ReasonUnknownClock                = "unknown_clock"
)

type SafetyBoundary struct {
	GatePassClaim          bool `json:"gate_pass_claim"`
	HardwareClaimAllowed   bool `json:"hardware_claim_allowed"`
	RFSynchronizationClaim bool `json:"rf_synchronization_claim"`
	FieldTXReady           bool `json:"field_tx_ready"`
	FirmwareWriteAllowed   bool `json:"firmware_write_allowed"`
}
