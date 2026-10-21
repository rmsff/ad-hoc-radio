package radiobackend

import (
	"encoding/hex"
	"fmt"
	"path"
	"slices"
	"strings"
)

var requiredNetworkCoreOwnership = []string{
	"canonical_frame_identity",
	"duplicate_suppression",
	"expected_group_snapshot",
	"metrics_semantics",
	"relay_tree_planning",
	"schedule_versioning",
	"transmitter_opportunity_order",
}

var requiredBackendOwnership = []string{
	"backend_fec",
	"backend_framing",
	"backend_modulation",
	"channel_capacity_measurement",
	"link_observation",
	"radio_control",
	"timing_acquisition_measurement",
	"tx_rx_turnaround_measurement",
}

type operationSpec struct {
	direction OperationDirection
	fields    []string
}

var requiredOperations = map[OperationName]operationSpec{
	OperationDiscoveryObservation: {
		direction: DirectionBackendToCore,
		fields:    []string{"freshness_ms", "observed_at_ms", "station_id"},
	},
	OperationLinkMeasurement: {
		direction: DirectionBackendToCore,
		fields:    []string{"delay_ms", "from_station_id", "loss_milli_pct", "observed_at_ms", "to_station_id"},
	},
	OperationReceiveDelivery: {
		direction: DirectionBackendToCore,
		fields:    []string{"frame_bytes", "monotonic_ms", "rssi_dbm"},
	},
	OperationScheduledTransmit: {
		direction: DirectionCoreToBackend,
		fields: []string{
			"backend_clock_id", "call_id", "deadline_monotonic_ms", "frame_bytes", "group_id",
			"guard_after_ms", "net_id", "not_before_monotonic_ms", "opportunity_duration_ms",
			"opportunity_index", "opportunity_offset_ms", "owner_station_id", "parent_station_id",
			"schedule_id", "schedule_version", "seq", "source_id", "timing_authority_id",
			"timing_epoch_id", "timing_strategy",
		},
	},
	OperationTimingObservation: {
		direction: DirectionBackendToCore,
		fields:    []string{"acquisition_ms", "drift_ppm", "monotonic_ms", "timing_authority_id", "timing_epoch_id"},
	},
}

func ValidateContract(contract Contract) error {
	if contract.ContractVersion != ContractVersion {
		return fmt.Errorf("radio backend contract: contract_version=%q, want %q", contract.ContractVersion, ContractVersion)
	}
	if contract.StoryID != StoryID {
		return fmt.Errorf("radio backend contract: story_id=%q, want %q", contract.StoryID, StoryID)
	}
	if err := validateSimulatorGate(contract.SimulatorGate); err != nil {
		return err
	}
	if err := validateOwnership(contract.Ownership); err != nil {
		return err
	}
	if err := validateOperations(contract.Operations); err != nil {
		return err
	}
	if err := validateSafety(contract.Safety); err != nil {
		return err
	}
	if err := validateCandidates(contract.Candidates); err != nil {
		return err
	}
	return nil
}

func validateSimulatorGate(gate SimulatorGate) error {
	if gate.ProfileID != "source_led/project_native_variable" || gate.TimingStrategy != "source_led" || gate.OpportunityProfile != "project_native_variable" {
		return fmt.Errorf("radio backend contract: simulator gate profile is not the approved MVP-9.7 candidate")
	}
	if gate.ArchitectureVerdict != "accepted" || gate.ProfileVerdict != "go" {
		return fmt.Errorf("radio backend contract: simulator gate verdict must be accepted/go")
	}
	if !validSHA256(gate.StressEvidenceHash) {
		return fmt.Errorf("radio backend contract: stress_evidence_hash must be lowercase SHA-256")
	}
	if gate.StressEvidenceHash != ApprovedMVP97StressEvidenceHash {
		return fmt.Errorf("radio backend contract: stress_evidence_hash must match approved MVP-9.7 evidence hash")
	}
	if gate.FramePeriodMS != 80 || gate.MaxRelayDepth != 3 || gate.MaxSerializedBranchWidth != 4 {
		return fmt.Errorf("radio backend contract: simulator operating envelope must remain 80ms/depth3/branch4")
	}
	if !gate.SerializedSingleFrequency {
		return fmt.Errorf("radio backend contract: serialized_single_frequency must be true")
	}
	if gate.SpatialReuseEnabled {
		return fmt.Errorf("radio backend contract: spatial_reuse_enabled must be false")
	}
	if gate.DMRCompatible || gate.CompatibilityClaim || gate.AMBEDependency || gate.RFSynchronizationClaim {
		return fmt.Errorf("radio backend contract: simulator gate carries a forbidden compatibility/RF claim")
	}
	return nil
}

func validateOwnership(ownership OwnershipBoundary) error {
	if err := validateExactStrings("ownership.network_core_owned", ownership.NetworkCoreOwned, requiredNetworkCoreOwnership); err != nil {
		return err
	}
	if err := validateExactStrings("ownership.backend_owned", ownership.BackendOwned, requiredBackendOwnership); err != nil {
		return err
	}
	for _, value := range ownership.NetworkCoreOwned {
		if slices.Contains(ownership.BackendOwned, value) {
			return fmt.Errorf("radio backend contract: ownership sets overlap at %q", value)
		}
	}
	return nil
}

func validateOperations(operations []BoundaryOperation) error {
	if len(operations) != len(requiredOperations) {
		return fmt.Errorf("radio backend contract: operations count=%d, want %d", len(operations), len(requiredOperations))
	}
	if !slices.IsSortedFunc(operations, func(a, b BoundaryOperation) int { return strings.Compare(string(a.Name), string(b.Name)) }) {
		return fmt.Errorf("radio backend contract: operations must be sorted by name")
	}
	seen := make(map[OperationName]struct{}, len(operations))
	for i, operation := range operations {
		spec, ok := requiredOperations[operation.Name]
		if !ok {
			return fmt.Errorf("radio backend contract: operations[%d].name=%q is invalid", i, operation.Name)
		}
		if _, ok := seen[operation.Name]; ok {
			return fmt.Errorf("radio backend contract: operations contain duplicate %q", operation.Name)
		}
		seen[operation.Name] = struct{}{}
		if operation.Direction != spec.direction {
			return fmt.Errorf("radio backend contract: operation %s direction=%q, want %q", operation.Name, operation.Direction, spec.direction)
		}
		if err := validateExactStrings("operation "+string(operation.Name)+" required_fields", operation.RequiredFields, spec.fields); err != nil {
			return err
		}
	}
	return nil
}

func validateSafety(safety SafetyBoundary) error {
	if !safety.ReceiveOnlyDefault {
		return fmt.Errorf("radio backend contract: receive_only_default must be true")
	}
	if safety.TXEnabledByDefault || safety.FieldTXAllowed || safety.HardwareClaimAllowed || safety.RegulatoryProfileAccepted || safety.LabSafetyPlanAccepted {
		return fmt.Errorf("radio backend contract: transmit/hardware/regulatory safety gates must remain false")
	}
	return nil
}

func validateCandidates(candidates []CandidateEvaluation) error {
	if len(candidates) == 0 {
		return fmt.Errorf("radio backend contract: at least one candidate is required")
	}
	if !slices.IsSortedFunc(candidates, func(a, b CandidateEvaluation) int { return strings.Compare(a.CandidateID, b.CandidateID) }) {
		return fmt.Errorf("radio backend contract: candidates must be sorted by candidate_id")
	}
	seen := make(map[string]struct{}, len(candidates))
	for i := range candidates {
		candidate := candidates[i]
		if !validIdentifier(candidate.CandidateID) {
			return fmt.Errorf("radio backend contract: candidates[%d].candidate_id=%q is invalid", i, candidate.CandidateID)
		}
		if _, ok := seen[candidate.CandidateID]; ok {
			return fmt.Errorf("radio backend contract: duplicate candidate_id=%q", candidate.CandidateID)
		}
		seen[candidate.CandidateID] = struct{}{}
		if strings.TrimSpace(candidate.FirmwareBase) == "" || strings.TrimSpace(candidate.SourceRef) == "" {
			return fmt.Errorf("radio backend contract: candidate %s firmware_base and source_ref are required", candidate.CandidateID)
		}
		if candidate.FirmwareBase != strings.TrimSpace(candidate.FirmwareBase) || candidate.SourceRef != strings.TrimSpace(candidate.SourceRef) {
			return fmt.Errorf("radio backend contract: candidate %s text fields must be trimmed", candidate.CandidateID)
		}
		if candidate.DMRCompatible || candidate.CompatibilityClaim || candidate.AMBEDependency || candidate.HardwareClaimAllowed || candidate.FieldTXReady {
			return fmt.Errorf("radio backend contract: candidate %s carries forbidden compatibility/hardware/TX claims", candidate.CandidateID)
		}
		if err := validateCandidateGates(candidate); err != nil {
			return err
		}
	}
	return nil
}

func validateCandidateGates(candidate CandidateEvaluation) error {
	if len(candidate.Gates) != len(requiredGateNames) {
		return fmt.Errorf("radio backend contract: candidate %s gates count=%d, want %d", candidate.CandidateID, len(candidate.Gates), len(requiredGateNames))
	}
	allPassed := true
	anyFailed := false
	for i, gate := range candidate.Gates {
		if gate.Name != requiredGateNames[i] {
			return fmt.Errorf("radio backend contract: candidate %s gates must match canonical inventory", candidate.CandidateID)
		}
		if err := validateGate(candidate.CandidateID, gate); err != nil {
			return err
		}
		allPassed = allPassed && gate.Status == GatePassed
		anyFailed = anyFailed || gate.Status == GateFailed
	}
	expected := CandidateMeasurementRequired
	if anyFailed {
		expected = CandidateRejected
	} else if allPassed {
		expected = CandidateEligibleReceiveOnlyLab
	}
	if candidate.Verdict != expected {
		return fmt.Errorf("radio backend contract: candidate %s verdict=%q, want %q from gate statuses", candidate.CandidateID, candidate.Verdict, expected)
	}
	return nil
}

func validateGate(candidateID string, gate EvaluationGate) error {
	if gate.Status != GateNotEvaluated && gate.Status != GatePassed && gate.Status != GateFailed {
		return fmt.Errorf("radio backend contract: candidate %s gate %s status=%q is invalid", candidateID, gate.Name, gate.Status)
	}
	if gate.EvidenceGrade != EvidenceNone && gate.EvidenceGrade != EvidenceSourceReviewed && gate.EvidenceGrade != EvidenceBuildVerified && gate.EvidenceGrade != EvidenceBenchMeasured {
		return fmt.Errorf("radio backend contract: candidate %s gate %s evidence_grade=%q is invalid", candidateID, gate.Name, gate.EvidenceGrade)
	}
	if strings.TrimSpace(gate.Reason) == "" || gate.Reason != strings.TrimSpace(gate.Reason) {
		return fmt.Errorf("radio backend contract: candidate %s gate %s requires a trimmed reason", candidateID, gate.Name)
	}
	if err := validateArtifactRefs(candidateID, gate); err != nil {
		return err
	}
	if gate.Status == GatePassed {
		if gate.EvidenceGrade == EvidenceNone || len(gate.ArtifactRefs) == 0 {
			return fmt.Errorf("radio backend contract: candidate %s passed gate %s requires evidence artifact refs", candidateID, gate.Name)
		}
		if measurementGate(gate.Name) && gate.EvidenceGrade != EvidenceBenchMeasured {
			return fmt.Errorf("radio backend contract: candidate %s passed gate %s requires bench_measured evidence", candidateID, gate.Name)
		}
	}
	return nil
}

func validateArtifactRefs(candidateID string, gate EvaluationGate) error {
	if gate.EvidenceGrade == EvidenceNone && len(gate.ArtifactRefs) != 0 {
		return fmt.Errorf("radio backend contract: candidate %s gate %s evidence none cannot carry artifacts", candidateID, gate.Name)
	}
	if !slices.IsSortedFunc(gate.ArtifactRefs, func(a, b ArtifactRef) int { return strings.Compare(a.Path, b.Path) }) {
		return fmt.Errorf("radio backend contract: candidate %s gate %s artifact refs must be sorted", candidateID, gate.Name)
	}
	for i, artifact := range gate.ArtifactRefs {
		if artifact.Path == "" || artifact.Path != strings.TrimSpace(artifact.Path) || path.IsAbs(artifact.Path) || path.Clean(artifact.Path) != artifact.Path || strings.HasPrefix(artifact.Path, "../") || strings.Contains(artifact.Path, "\\") {
			return fmt.Errorf("radio backend contract: candidate %s gate %s artifact[%d] path=%q is invalid", candidateID, gate.Name, i, artifact.Path)
		}
		if i > 0 && artifact.Path == gate.ArtifactRefs[i-1].Path {
			return fmt.Errorf("radio backend contract: candidate %s gate %s contains duplicate artifact %q", candidateID, gate.Name, artifact.Path)
		}
		if !validSHA256(artifact.SHA256) {
			return fmt.Errorf("radio backend contract: candidate %s gate %s artifact %q sha256 is invalid", candidateID, gate.Name, artifact.Path)
		}
	}
	return nil
}

func validateExactStrings(label string, values, expected []string) error {
	if !slices.IsSorted(values) {
		return fmt.Errorf("radio backend contract: %s must be sorted", label)
	}
	for i := 1; i < len(values); i++ {
		if values[i] == values[i-1] {
			return fmt.Errorf("radio backend contract: %s contains duplicate %q", label, values[i])
		}
	}
	if !slices.Equal(values, expected) {
		return fmt.Errorf("radio backend contract: %s does not match the v1 ownership/operation contract", label)
	}
	return nil
}

func measurementGate(name GateName) bool {
	return name == GateChannelCapacity || name == GateTXRXTurnaround || name == GateTimingAcquisition || name == GateLabSafeMeasurement
}

func validIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

func validSHA256(value string) bool {
	if len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
