package radiobackend

const (
	SelectionSchemaVersion      = "adhr-backend-selection-v1"
	SelectionStoryID            = "MVP-10.1"
	FrozenHandoffContractSHA256 = "d97a01b0160fe2c125574ffc1c1125a9da20c2d20e0508d35d565977d38ec39a"
)

type SelectionReport struct {
	SchemaVersion          string               `json:"schema_version"`
	StoryID                string               `json:"story_id"`
	HandoffContractVersion string               `json:"handoff_contract_version"`
	HandoffContractSHA256  string               `json:"handoff_contract_sha256"`
	Scope                  SelectionScope       `json:"scope"`
	Outcome                SelectionOutcome     `json:"outcome"`
	SelectedCandidateID    string               `json:"selected_candidate_id"`
	Candidates             []SelectionCandidate `json:"candidates"`
	Safety                 SelectionSafety      `json:"safety"`
}

type SelectionScope string

const SelectionScopeReceiveOnlyFirmware SelectionScope = "receive_only_firmware_slice"

type SelectionOutcome string

const (
	SelectionOutcomeSelected SelectionOutcome = "selected"
	SelectionOutcomeNoGo     SelectionOutcome = "no_go"
)

type SelectionDecision string

const (
	SelectionDecisionSelectedReceiveOnly SelectionDecision = "selected_receive_only"
	SelectionDecisionDeferred            SelectionDecision = "deferred"
	SelectionDecisionRejected            SelectionDecision = "rejected"
)

type SelectionCandidate struct {
	CandidateID          string            `json:"candidate_id"`
	FirmwareBase         string            `json:"firmware_base"`
	SourceRef            string            `json:"source_ref"`
	Target               string            `json:"target"`
	Decision             SelectionDecision `json:"decision"`
	Checks               []SelectionCheck  `json:"checks"`
	DMRCompatible        bool              `json:"dmr_compatible"`
	CompatibilityClaim   bool              `json:"compatibility_claim"`
	AMBEDependency       bool              `json:"ambe_dependency"`
	HardwareClaimAllowed bool              `json:"hardware_claim_allowed"`
	FieldTXReady         bool              `json:"field_tx_ready"`
}

type SelectionCheckName string

const (
	SelectionCheckBuildReproducibility  SelectionCheckName = "build_reproducibility"
	SelectionCheckCodecCoupling         SelectionCheckName = "codec_coupling"
	SelectionCheckCustomADHRReceivePath SelectionCheckName = "custom_adhr_receive_path"
	SelectionCheckHardwareBoundary      SelectionCheckName = "hardware_evidence_boundary"
	SelectionCheckLicenseAllowedUse     SelectionCheckName = "license_allowed_use"
	SelectionCheckRollbackInputs        SelectionCheckName = "rollback_inputs"
	SelectionCheckSourcePin             SelectionCheckName = "source_pin"
	SelectionCheckTargetSupport         SelectionCheckName = "target_support"
)

var requiredSelectionCheckNames = []SelectionCheckName{
	SelectionCheckBuildReproducibility,
	SelectionCheckCodecCoupling,
	SelectionCheckCustomADHRReceivePath,
	SelectionCheckHardwareBoundary,
	SelectionCheckLicenseAllowedUse,
	SelectionCheckRollbackInputs,
	SelectionCheckSourcePin,
	SelectionCheckTargetSupport,
}

var requiredSelectionCandidateIDs = []string{
	"opengd77_mduv380",
	"openrtx_mduv3x0",
}

var requiredSelectedPassChecks = []SelectionCheckName{
	SelectionCheckBuildReproducibility,
	SelectionCheckCustomADHRReceivePath,
	SelectionCheckHardwareBoundary,
	SelectionCheckLicenseAllowedUse,
	SelectionCheckRollbackInputs,
	SelectionCheckSourcePin,
	SelectionCheckTargetSupport,
}

type SelectionCheckStatus string

const (
	SelectionCheckPassed   SelectionCheckStatus = "passed"
	SelectionCheckDeferred SelectionCheckStatus = "deferred"
	SelectionCheckFailed   SelectionCheckStatus = "failed"
)

type SelectionCheck struct {
	Name          SelectionCheckName   `json:"name"`
	Status        SelectionCheckStatus `json:"status"`
	EvidenceGrade EvidenceGrade        `json:"evidence_grade"`
	Reason        string               `json:"reason"`
	ArtifactRefs  []ArtifactRef        `json:"artifact_refs"`
}

type SelectionSafety struct {
	ReceiveOnlyDefault        bool `json:"receive_only_default"`
	TXEnabledByDefault        bool `json:"tx_enabled_by_default"`
	FirmwareWriteAllowed      bool `json:"firmware_write_allowed"`
	RegulatoryProfileAccepted bool `json:"regulatory_profile_accepted"`
	LabSafetyPlanAccepted     bool `json:"lab_safety_plan_accepted"`
	HardwareClaimAllowed      bool `json:"hardware_claim_allowed"`
	FieldTXReady              bool `json:"field_tx_ready"`
}
