// Package radiobackend defines the versioned semantic handoff between the
// ADHR network core and future radio/firmware adapters. It does not implement
// a radio backend or own routing, scheduling, or RF behavior.
package radiobackend

const (
	ContractVersion                 = "adhr-sim-fw-contract-v1"
	StoryID                         = "MVP-9.8"
	ApprovedMVP97StressEvidenceHash = "7883ebade1df42ad1bd5c96302cabdf87e2fd6900b04b0b1507eb12f539aed29"
)

type Contract struct {
	ContractVersion string                `json:"contract_version"`
	StoryID         string                `json:"story_id"`
	SimulatorGate   SimulatorGate         `json:"simulator_gate"`
	Ownership       OwnershipBoundary     `json:"ownership"`
	Operations      []BoundaryOperation   `json:"operations"`
	Safety          SafetyBoundary        `json:"safety"`
	Candidates      []CandidateEvaluation `json:"candidates"`
}

type SimulatorGate struct {
	ProfileID                 string `json:"profile_id"`
	TimingStrategy            string `json:"timing_strategy"`
	OpportunityProfile        string `json:"opportunity_profile"`
	ArchitectureVerdict       string `json:"architecture_verdict"`
	ProfileVerdict            string `json:"profile_verdict"`
	StressEvidenceHash        string `json:"stress_evidence_hash"`
	FramePeriodMS             int64  `json:"frame_period_ms"`
	MaxRelayDepth             uint8  `json:"max_relay_depth"`
	MaxSerializedBranchWidth  int    `json:"max_serialized_branch_width"`
	SerializedSingleFrequency bool   `json:"serialized_single_frequency"`
	SpatialReuseEnabled       bool   `json:"spatial_reuse_enabled"`
	DMRCompatible             bool   `json:"dmr_compatible"`
	CompatibilityClaim        bool   `json:"compatibility_claim"`
	AMBEDependency            bool   `json:"ambe_dependency"`
	RFSynchronizationClaim    bool   `json:"rf_synchronization_claim"`
}

type OwnershipBoundary struct {
	NetworkCoreOwned []string `json:"network_core_owned"`
	BackendOwned     []string `json:"backend_owned"`
}

type OperationName string

const (
	OperationDiscoveryObservation OperationName = "discovery_observation"
	OperationLinkMeasurement      OperationName = "link_measurement"
	OperationReceiveDelivery      OperationName = "receive_delivery"
	OperationScheduledTransmit    OperationName = "scheduled_transmit"
	OperationTimingObservation    OperationName = "timing_observation"
)

type OperationDirection string

const (
	DirectionBackendToCore OperationDirection = "backend_to_core"
	DirectionCoreToBackend OperationDirection = "core_to_backend"
)

type BoundaryOperation struct {
	Name           OperationName      `json:"name"`
	Direction      OperationDirection `json:"direction"`
	RequiredFields []string           `json:"required_fields"`
}

type SafetyBoundary struct {
	ReceiveOnlyDefault        bool `json:"receive_only_default"`
	TXEnabledByDefault        bool `json:"tx_enabled_by_default"`
	FieldTXAllowed            bool `json:"field_tx_allowed"`
	HardwareClaimAllowed      bool `json:"hardware_claim_allowed"`
	RegulatoryProfileAccepted bool `json:"regulatory_profile_accepted"`
	LabSafetyPlanAccepted     bool `json:"lab_safety_plan_accepted"`
}

type GateName string

const (
	GateChannelCapacity    GateName = "channel_capacity"
	GateCodecCoupling      GateName = "codec_coupling"
	GateLabSafeMeasurement GateName = "lab_safe_measurement"
	GateLicense            GateName = "license"
	GateTimingAcquisition  GateName = "timing_acquisition"
	GateTXRXTurnaround     GateName = "tx_rx_turnaround"
)

var requiredGateNames = []GateName{
	GateChannelCapacity,
	GateCodecCoupling,
	GateLabSafeMeasurement,
	GateLicense,
	GateTimingAcquisition,
	GateTXRXTurnaround,
}

func RequiredGateNames() []GateName {
	return append([]GateName(nil), requiredGateNames...)
}

type GateStatus string

const (
	GateNotEvaluated GateStatus = "not_evaluated"
	GatePassed       GateStatus = "passed"
	GateFailed       GateStatus = "failed"
)

type EvidenceGrade string

const (
	EvidenceNone           EvidenceGrade = "none"
	EvidenceSourceReviewed EvidenceGrade = "source_reviewed"
	EvidenceBuildVerified  EvidenceGrade = "build_verified"
	EvidenceBenchMeasured  EvidenceGrade = "bench_measured"
)

type ArtifactRef struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type EvaluationGate struct {
	Name          GateName      `json:"name"`
	Status        GateStatus    `json:"status"`
	EvidenceGrade EvidenceGrade `json:"evidence_grade"`
	Reason        string        `json:"reason"`
	ArtifactRefs  []ArtifactRef `json:"artifact_refs"`
}

type CandidateVerdict string

const (
	CandidateMeasurementRequired    CandidateVerdict = "measurement_required"
	CandidateEligibleReceiveOnlyLab CandidateVerdict = "eligible_for_receive_only_lab"
	CandidateRejected               CandidateVerdict = "rejected"
)

type CandidateEvaluation struct {
	CandidateID          string           `json:"candidate_id"`
	FirmwareBase         string           `json:"firmware_base"`
	SourceRef            string           `json:"source_ref"`
	Verdict              CandidateVerdict `json:"verdict"`
	Gates                []EvaluationGate `json:"gates"`
	DMRCompatible        bool             `json:"dmr_compatible"`
	CompatibilityClaim   bool             `json:"compatibility_claim"`
	AMBEDependency       bool             `json:"ambe_dependency"`
	HardwareClaimAllowed bool             `json:"hardware_claim_allowed"`
	FieldTXReady         bool             `json:"field_tx_ready"`
}
