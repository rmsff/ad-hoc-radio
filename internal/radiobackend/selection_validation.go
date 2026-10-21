package radiobackend

import (
	"fmt"
	"path"
	"slices"
	"strings"
)

func ValidateSelectionReport(report SelectionReport) error {
	if report.SchemaVersion != SelectionSchemaVersion {
		return fmt.Errorf("backend selection: schema_version=%q, want %q", report.SchemaVersion, SelectionSchemaVersion)
	}
	if report.StoryID != SelectionStoryID {
		return fmt.Errorf("backend selection: story_id=%q, want %q", report.StoryID, SelectionStoryID)
	}
	if report.HandoffContractVersion != ContractVersion {
		return fmt.Errorf("backend selection: handoff_contract_version=%q, want %q", report.HandoffContractVersion, ContractVersion)
	}
	if report.HandoffContractSHA256 != FrozenHandoffContractSHA256 {
		return fmt.Errorf("backend selection: handoff_contract_sha256 must match frozen MVP-9.8 handoff")
	}
	if report.Scope != SelectionScopeReceiveOnlyFirmware {
		return fmt.Errorf("backend selection: scope=%q is invalid", report.Scope)
	}
	if report.Outcome != SelectionOutcomeSelected && report.Outcome != SelectionOutcomeNoGo {
		return fmt.Errorf("backend selection: outcome=%q is invalid", report.Outcome)
	}
	if err := validateSelectionSafety(report.Safety); err != nil {
		return err
	}
	return validateSelectionCandidates(report)
}

func validateSelectionSafety(safety SelectionSafety) error {
	if !safety.ReceiveOnlyDefault {
		return fmt.Errorf("backend selection: safety.receive_only_default must be true")
	}
	if safety.TXEnabledByDefault || safety.FirmwareWriteAllowed || safety.RegulatoryProfileAccepted || safety.LabSafetyPlanAccepted || safety.HardwareClaimAllowed || safety.FieldTXReady {
		return fmt.Errorf("backend selection: safety claims must remain false")
	}
	return nil
}

func validateSelectionCandidates(report SelectionReport) error {
	if len(report.Candidates) != len(requiredSelectionCandidateIDs) {
		return fmt.Errorf("backend selection: candidate inventory count=%d, want %d from frozen handoff", len(report.Candidates), len(requiredSelectionCandidateIDs))
	}
	if !slices.IsSortedFunc(report.Candidates, func(a, b SelectionCandidate) int {
		return strings.Compare(a.CandidateID, b.CandidateID)
	}) {
		return fmt.Errorf("backend selection: candidates must be sorted by candidate_id")
	}

	seen := make(map[string]struct{}, len(report.Candidates))
	selectedCount := 0
	selectedID := ""
	for i := range report.Candidates {
		candidate := report.Candidates[i]
		if candidate.CandidateID != requiredSelectionCandidateIDs[i] {
			return fmt.Errorf("backend selection: candidate inventory must match frozen handoff")
		}
		if !validIdentifier(candidate.CandidateID) {
			return fmt.Errorf("backend selection: candidates[%d].candidate_id=%q is invalid", i, candidate.CandidateID)
		}
		if _, exists := seen[candidate.CandidateID]; exists {
			return fmt.Errorf("backend selection: duplicate candidate_id=%q", candidate.CandidateID)
		}
		seen[candidate.CandidateID] = struct{}{}
		if err := validateSelectionCandidate(candidate); err != nil {
			return err
		}
		if candidate.Decision == SelectionDecisionSelectedReceiveOnly {
			selectedCount++
			selectedID = candidate.CandidateID
		}
	}

	switch report.Outcome {
	case SelectionOutcomeSelected:
		if selectedCount != 1 {
			return fmt.Errorf("backend selection: selected outcome requires exactly one selected candidate, got %d", selectedCount)
		}
		if report.SelectedCandidateID != selectedID {
			return fmt.Errorf("backend selection: selected_candidate_id=%q, want %q", report.SelectedCandidateID, selectedID)
		}
	case SelectionOutcomeNoGo:
		if selectedCount != 0 || report.SelectedCandidateID != "" {
			return fmt.Errorf("backend selection: no_go requires no selected candidate and empty selected_candidate_id")
		}
	}
	return nil
}

func validateSelectionCandidate(candidate SelectionCandidate) error {
	if strings.TrimSpace(candidate.FirmwareBase) == "" || candidate.FirmwareBase != strings.TrimSpace(candidate.FirmwareBase) {
		return fmt.Errorf("backend selection: candidate %s firmware_base is required and trimmed", candidate.CandidateID)
	}
	if strings.TrimSpace(candidate.SourceRef) == "" || candidate.SourceRef != strings.TrimSpace(candidate.SourceRef) {
		return fmt.Errorf("backend selection: candidate %s source_ref is required and trimmed", candidate.CandidateID)
	}
	if strings.TrimSpace(candidate.Target) == "" || candidate.Target != strings.TrimSpace(candidate.Target) {
		return fmt.Errorf("backend selection: candidate %s target is required and trimmed", candidate.CandidateID)
	}
	if candidate.Decision != SelectionDecisionSelectedReceiveOnly && candidate.Decision != SelectionDecisionDeferred && candidate.Decision != SelectionDecisionRejected {
		return fmt.Errorf("backend selection: candidate %s decision=%q is invalid", candidate.CandidateID, candidate.Decision)
	}
	if candidate.DMRCompatible || candidate.CompatibilityClaim || candidate.AMBEDependency || candidate.HardwareClaimAllowed || candidate.FieldTXReady {
		return fmt.Errorf("backend selection: candidate %s carries forbidden compatibility/hardware/TX claim", candidate.CandidateID)
	}
	if err := validateSelectionChecks(candidate); err != nil {
		return err
	}
	return validateSelectionDecision(candidate)
}

func validateSelectionChecks(candidate SelectionCandidate) error {
	if len(candidate.Checks) != len(requiredSelectionCheckNames) {
		return fmt.Errorf("backend selection: candidate %s checks count=%d, want %d", candidate.CandidateID, len(candidate.Checks), len(requiredSelectionCheckNames))
	}
	for i, check := range candidate.Checks {
		if check.Name != requiredSelectionCheckNames[i] {
			return fmt.Errorf("backend selection: candidate %s checks must match canonical inventory", candidate.CandidateID)
		}
		if check.Status != SelectionCheckPassed && check.Status != SelectionCheckDeferred && check.Status != SelectionCheckFailed {
			return fmt.Errorf("backend selection: candidate %s check %s status=%q is invalid", candidate.CandidateID, check.Name, check.Status)
		}
		if check.EvidenceGrade != EvidenceNone && check.EvidenceGrade != EvidenceSourceReviewed && check.EvidenceGrade != EvidenceBuildVerified && check.EvidenceGrade != EvidenceBenchMeasured {
			return fmt.Errorf("backend selection: candidate %s check %s evidence_grade=%q is invalid", candidate.CandidateID, check.Name, check.EvidenceGrade)
		}
		if strings.TrimSpace(check.Reason) == "" || check.Reason != strings.TrimSpace(check.Reason) {
			return fmt.Errorf("backend selection: candidate %s check %s requires a trimmed reason", candidate.CandidateID, check.Name)
		}
		if err := validateSelectionArtifactRefs(candidate.CandidateID, check); err != nil {
			return err
		}
		if check.Status == SelectionCheckPassed && (check.EvidenceGrade == EvidenceNone || len(check.ArtifactRefs) == 0) {
			return fmt.Errorf("backend selection: candidate %s passed check %s requires evidence refs", candidate.CandidateID, check.Name)
		}
		if check.Status == SelectionCheckPassed && check.EvidenceGrade != requiredPassedEvidenceGrade(check.Name) {
			return fmt.Errorf("backend selection: candidate %s passed check %s requires evidence_grade=%q", candidate.CandidateID, check.Name, requiredPassedEvidenceGrade(check.Name))
		}
	}
	return nil
}

func requiredPassedEvidenceGrade(name SelectionCheckName) EvidenceGrade {
	switch name {
	case SelectionCheckBuildReproducibility, SelectionCheckCustomADHRReceivePath:
		return EvidenceBuildVerified
	default:
		return EvidenceSourceReviewed
	}
}

func validateSelectionDecision(candidate SelectionCandidate) error {
	failed := false
	checks := make(map[SelectionCheckName]SelectionCheckStatus, len(candidate.Checks))
	for _, check := range candidate.Checks {
		checks[check.Name] = check.Status
		failed = failed || check.Status == SelectionCheckFailed
	}

	switch candidate.Decision {
	case SelectionDecisionSelectedReceiveOnly:
		if checks[SelectionCheckCodecCoupling] == SelectionCheckFailed {
			return fmt.Errorf("backend selection: selected candidate %s cannot fail codec_coupling", candidate.CandidateID)
		}
		if failed {
			return fmt.Errorf("backend selection: selected candidate %s cannot contain a failed check", candidate.CandidateID)
		}
		for _, name := range requiredSelectedPassChecks {
			if checks[name] != SelectionCheckPassed {
				return fmt.Errorf("backend selection: selected candidate %s requires passed %s check", candidate.CandidateID, name)
			}
		}
	case SelectionDecisionRejected:
		if !failed {
			return fmt.Errorf("backend selection: rejected candidate %s requires at least one failed check", candidate.CandidateID)
		}
	case SelectionDecisionDeferred:
		if !failed && allSelectionChecksPassed(candidate.Checks) {
			return fmt.Errorf("backend selection: deferred candidate %s must retain an unresolved check", candidate.CandidateID)
		}
	}
	return nil
}

func allSelectionChecksPassed(checks []SelectionCheck) bool {
	for _, check := range checks {
		if check.Status != SelectionCheckPassed {
			return false
		}
	}
	return true
}

func validateSelectionArtifactRefs(candidateID string, check SelectionCheck) error {
	if check.EvidenceGrade == EvidenceNone && len(check.ArtifactRefs) != 0 {
		return fmt.Errorf("backend selection: candidate %s check %s evidence none cannot carry artifacts", candidateID, check.Name)
	}
	if !slices.IsSortedFunc(check.ArtifactRefs, func(a, b ArtifactRef) int {
		return strings.Compare(a.Path, b.Path)
	}) {
		return fmt.Errorf("backend selection: candidate %s check %s artifact refs must be sorted", candidateID, check.Name)
	}
	for i, artifact := range check.ArtifactRefs {
		if artifact.Path == "" || artifact.Path != strings.TrimSpace(artifact.Path) || path.IsAbs(artifact.Path) || path.Clean(artifact.Path) != artifact.Path || strings.HasPrefix(artifact.Path, "../") || strings.Contains(artifact.Path, "\\") {
			return fmt.Errorf("backend selection: candidate %s check %s artifact[%d] path=%q is invalid", candidateID, check.Name, i, artifact.Path)
		}
		if i > 0 && artifact.Path == check.ArtifactRefs[i-1].Path {
			return fmt.Errorf("backend selection: candidate %s check %s contains duplicate artifact %q", candidateID, check.Name, artifact.Path)
		}
		if !validSHA256(artifact.SHA256) {
			return fmt.Errorf("backend selection: candidate %s check %s artifact %q sha256 is invalid", candidateID, check.Name, artifact.Path)
		}
	}
	return nil
}
