package backendmeasurement

import (
	"encoding/hex"
	"fmt"
	"path"
	"reflect"
	"slices"
	"strings"
)

type endpointSpec struct {
	gate  GateName
	start EndpointMarker
	end   EndpointMarker
}

var endpointSpecs = map[MeasurementKind]endpointSpec{
	KindChannelCapacity:   {gate: GateChannelCapacity, start: EndpointExternalTestFrameStart, end: EndpointValidReceiverFrameCompletion},
	KindRXToTXTurnaround:  {gate: GateTXRXTurnaround, start: EndpointLastValidExternalRXMarker, end: EndpointFirstValidTXMarker},
	KindTXToRXTurnaround:  {gate: GateTXRXTurnaround, start: EndpointLastValidTXMarker, end: EndpointFirstValidRXMarker},
	KindTimingAcquisition: {gate: GateTimingAcquisition, start: EndpointSourceSyncReference, end: EndpointBackendEpochLock},
}

func FinalizeReport(in MeasurementReport) (MeasurementReport, error) {
	out := CloneReport(in)
	if err := validateStructure(out); err != nil {
		return MeasurementReport{}, err
	}
	summary, err := SummarizeTrials(out)
	if err != nil {
		return MeasurementReport{}, err
	}
	out.Summary = summary
	out.NoPassReasons = evaluateNoPassReasons(out)
	if len(out.NoPassReasons) == 0 {
		out.Verdict = VerdictPassed
		out.Safety.GatePassClaim = true
	} else if prerequisitesComplete(out) {
		out.Verdict = VerdictFailed
		out.Safety.GatePassClaim = false
	} else {
		out.Verdict = VerdictNotEvaluated
		out.Safety.GatePassClaim = false
	}
	if err := ValidateReport(out); err != nil {
		return MeasurementReport{}, err
	}
	return out, nil
}

func ValidateReport(report MeasurementReport) error {
	if err := validateStructure(report); err != nil {
		return err
	}
	expectedSummary, err := SummarizeTrials(report)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(report.Summary, expectedSummary) {
		return fmt.Errorf("backend measurement: declared summary does not match trials")
	}
	expectedReasons := evaluateNoPassReasons(report)
	if !slices.Equal(report.NoPassReasons, expectedReasons) {
		return fmt.Errorf("backend measurement: no_pass_reasons do not match report inputs")
	}
	expectedVerdict := VerdictNotEvaluated
	if len(expectedReasons) == 0 {
		expectedVerdict = VerdictPassed
	} else if prerequisitesComplete(report) {
		expectedVerdict = VerdictFailed
	}
	if report.Verdict != expectedVerdict {
		return fmt.Errorf("backend measurement: verdict=%q, want %q", report.Verdict, expectedVerdict)
	}
	if report.Safety.GatePassClaim != (report.Verdict == VerdictPassed) {
		return fmt.Errorf("backend measurement: gate_pass_claim must match passed verdict")
	}
	return nil
}

func validateStructure(report MeasurementReport) error {
	if report.SchemaVersion != SchemaVersion || report.StoryID != StoryID {
		return fmt.Errorf("backend measurement: invalid schema/story identity")
	}
	if report.HandoffContractSHA256 != FrozenHandoffSHA256 || report.SelectionReportSHA256 != FrozenSelectionSHA256 {
		return fmt.Errorf("backend measurement: frozen handoff/selection binding mismatch")
	}
	if err := validateCandidate(report.Candidate); err != nil {
		return err
	}
	if err := validateDefinition(report.Definition); err != nil {
		return err
	}
	if err := validateClock(report.Clock); err != nil {
		return err
	}
	if report.Units != CanonicalUnits() {
		return fmt.Errorf("backend measurement: units must match canonical inventory")
	}
	if err := validateTrialPlan(report.Definition.Kind, report.Candidate.ProfileID, report.TrialPlan); err != nil {
		return err
	}
	if err := validateThresholds(report.Definition.Kind, report.Thresholds, report.TrialPlan); err != nil {
		return err
	}
	if report.EvidenceGrade != EvidenceFixture && report.EvidenceGrade != EvidenceSourceReviewed && report.EvidenceGrade != EvidenceBuildVerified && report.EvidenceGrade != EvidenceBenchMeasured {
		return fmt.Errorf("backend measurement: evidence_grade=%q is invalid", report.EvidenceGrade)
	}
	if err := validateArtifactRefs(report.RawArtifacts); err != nil {
		return err
	}
	if err := validateTrials(report); err != nil {
		return err
	}
	if err := validateClockQuantization(report); err != nil {
		return err
	}
	if report.Safety.HardwareClaimAllowed || report.Safety.RFSynchronizationClaim || report.Safety.FieldTXReady || report.Safety.FirmwareWriteAllowed {
		return fmt.Errorf("backend measurement: hardware/RF/TX/write claims must remain false")
	}
	if !slices.IsSorted(report.NoPassReasons) || hasDuplicateStrings(report.NoPassReasons) {
		return fmt.Errorf("backend measurement: no_pass_reasons must be sorted and unique")
	}
	return nil
}

func validateTrialPlan(kind MeasurementKind, selectedProfileID string, plan TrialPlan) error {
	if plan.PlannedTrialCount == 0 || plan.MinimumSampleCount == 0 || plan.MinimumSamplesPerCase == 0 || plan.MinimumSampleCount > plan.PlannedTrialCount || len(plan.MatrixCases) == 0 {
		return fmt.Errorf("backend measurement: invalid trial plan")
	}
	minimumMatrixSamples := uint64(plan.MinimumSamplesPerCase) * uint64(len(plan.MatrixCases))
	if uint64(plan.MinimumSampleCount) < minimumMatrixSamples || uint64(plan.PlannedTrialCount) < minimumMatrixSamples {
		return fmt.Errorf("backend measurement: trial plan cannot cover matrix cases")
	}
	if !slices.IsSortedFunc(plan.MatrixCases, func(a, b MatrixCase) int { return strings.Compare(a.CaseID, b.CaseID) }) {
		return fmt.Errorf("backend measurement: matrix cases must be sorted by case_id")
	}
	for i, matrixCase := range plan.MatrixCases {
		if !validIdentifier(matrixCase.CaseID) || strings.TrimSpace(matrixCase.ProfileID) == "" || matrixCase.ProfileID != strings.TrimSpace(matrixCase.ProfileID) || !validIdentifier(matrixCase.StateCondition) || !validIdentifier(matrixCase.PowerProfile) || !validIdentifier(matrixCase.ExpectedOutcome) {
			return fmt.Errorf("backend measurement: matrix case %d identity is invalid", i)
		}
		if i > 0 && matrixCase.CaseID == plan.MatrixCases[i-1].CaseID {
			return fmt.Errorf("backend measurement: duplicate matrix case %q", matrixCase.CaseID)
		}
		if matrixCase.ProfileID != selectedProfileID {
			return fmt.Errorf("backend measurement: matrix case %s profile must match accepted candidate profile", matrixCase.CaseID)
		}
		if kind == KindChannelCapacity {
			if err := validateChannelMatrixCase(matrixCase); err != nil {
				return err
			}
		} else if matrixCase.Channel != nil {
			return fmt.Errorf("backend measurement: non-channel matrix case %s carries channel fields", matrixCase.CaseID)
		}
	}
	return nil
}

func validateChannelMatrixCase(matrixCase MatrixCase) error {
	channel := matrixCase.Channel
	if channel == nil || channel.ChannelRateBPS == 0 || channel.PayloadBytes == 0 || channel.RequiredFrameBytes == 0 {
		return fmt.Errorf("backend measurement: channel matrix case %s is incomplete", matrixCase.CaseID)
	}
	if _, ok := addUint64(channel.PayloadBytes, channel.OverheadBytes); !ok {
		return fmt.Errorf("backend measurement: channel matrix case %s tested capacity overflows", matrixCase.CaseID)
	}
	if _, ok := addUint64(channel.RequiredFrameBytes, channel.GuardMarginBytes); !ok {
		return fmt.Errorf("backend measurement: channel matrix case %s required capacity overflows", matrixCase.CaseID)
	}
	return nil
}

func validateCandidate(candidate CandidateIdentity) error {
	if candidate.CandidateID != SelectedCandidateID || candidate.ProfileID != SelectedProfileID {
		return fmt.Errorf("backend measurement: candidate/profile must match accepted selection")
	}
	if candidate.FirmwareBase != SelectedFirmwareBase || candidate.SourceRef != SelectedSourceRef || candidate.SourceRevision != SelectedSourceRevision || candidate.Target != SelectedTarget {
		return fmt.Errorf("backend measurement: candidate source identity must match accepted selection")
	}
	if strings.TrimSpace(candidate.FirmwareBase) == "" || candidate.FirmwareBase != strings.TrimSpace(candidate.FirmwareBase) || strings.TrimSpace(candidate.SourceRef) == "" || candidate.SourceRef != strings.TrimSpace(candidate.SourceRef) || strings.TrimSpace(candidate.Target) == "" || candidate.Target != strings.TrimSpace(candidate.Target) {
		return fmt.Errorf("backend measurement: candidate text identity is invalid")
	}
	if len(candidate.SourceRevision) != 40 {
		return fmt.Errorf("backend measurement: source_revision must be lowercase git SHA-1")
	}
	if _, err := hex.DecodeString(candidate.SourceRevision); err != nil || strings.ToLower(candidate.SourceRevision) != candidate.SourceRevision {
		return fmt.Errorf("backend measurement: source_revision must be lowercase git SHA-1")
	}
	if !validSHA256(candidate.FirmwareSHA256) || !validSHA256(candidate.BinarySHA256) {
		return fmt.Errorf("backend measurement: firmware/binary SHA-256 is invalid")
	}
	return nil
}

func validateDefinition(definition MeasurementDefinition) error {
	spec, ok := endpointSpecs[definition.Kind]
	if !ok {
		return fmt.Errorf("backend measurement: kind=%q is invalid", definition.Kind)
	}
	if definition.GateName != spec.gate || definition.StartMarker != spec.start || definition.EndMarker != spec.end {
		return fmt.Errorf("backend measurement: gate/endpoints do not match kind %q", definition.Kind)
	}
	return nil
}

func validateClock(clock ClockDefinition) error {
	switch clock.Status {
	case ClockKnown:
		if !validIdentifier(clock.BackendClockID) || clock.BackendClockID == UnknownBackendClockID || clock.ResolutionNS == 0 {
			return fmt.Errorf("backend measurement: known clock requires ID and resolution")
		}
	case ClockUnknown:
		if clock.BackendClockID != UnknownBackendClockID || clock.ResolutionNS != 0 {
			return fmt.Errorf("backend measurement: unknown clock must use sentinel and zero resolution")
		}
	default:
		return fmt.Errorf("backend measurement: clock status=%q is invalid", clock.Status)
	}
	return nil
}

func validateThresholds(kind MeasurementKind, thresholds *ThresholdSet, plan TrialPlan) error {
	if thresholds == nil {
		return nil
	}
	if thresholds.Version != ThresholdVersion || thresholds.Duration.P95MaxUS == 0 || thresholds.Duration.MaxUS == 0 || thresholds.Duration.P95MaxUS > thresholds.Duration.MaxUS || thresholds.Duration.MaxFailedSamples > plan.PlannedTrialCount {
		return fmt.Errorf("backend measurement: invalid duration thresholds")
	}
	switch kind {
	case KindChannelCapacity:
		if thresholds.Channel == nil || thresholds.Timing != nil {
			return fmt.Errorf("backend measurement: channel kind requires only channel thresholds")
		}
	case KindTimingAcquisition:
		if thresholds.Timing == nil || thresholds.Channel != nil {
			return fmt.Errorf("backend measurement: timing kind requires only timing thresholds")
		}
		if thresholds.Timing.MaxAbsBoundaryErrorUS == 0 || thresholds.Timing.MaxAbsDriftMilliPPM == 0 {
			return fmt.Errorf("backend measurement: timing thresholds require boundary and drift limits")
		}
	default:
		if thresholds.Channel != nil || thresholds.Timing != nil {
			return fmt.Errorf("backend measurement: turnaround kind cannot carry channel/timing thresholds")
		}
	}
	return nil
}

func validateArtifactRefs(refs []ArtifactRef) error {
	if !slices.IsSortedFunc(refs, func(a, b ArtifactRef) int { return strings.Compare(a.Path, b.Path) }) {
		return fmt.Errorf("backend measurement: raw_artifacts must be sorted")
	}
	for i, ref := range refs {
		if ref.Path == "" || ref.Path != strings.TrimSpace(ref.Path) || path.IsAbs(ref.Path) || path.Clean(ref.Path) != ref.Path || strings.HasPrefix(ref.Path, "../") || strings.Contains(ref.Path, "\\") {
			return fmt.Errorf("backend measurement: raw_artifacts[%d] path is invalid", i)
		}
		if i > 0 && ref.Path == refs[i-1].Path {
			return fmt.Errorf("backend measurement: duplicate raw artifact %q", ref.Path)
		}
		if !validSHA256(ref.SHA256) {
			return fmt.Errorf("backend measurement: raw artifact %q SHA-256 is invalid", ref.Path)
		}
	}
	return nil
}

func validateTrials(report MeasurementReport) error {
	if !slices.IsSortedFunc(report.Trials, func(a, b Trial) int { return strings.Compare(a.TrialID, b.TrialID) }) {
		return fmt.Errorf("backend measurement: trials must be sorted by trial_id")
	}
	var attempted, warmups uint32
	for i, trial := range report.Trials {
		if !validIdentifier(trial.TrialID) {
			return fmt.Errorf("backend measurement: trials[%d].trial_id is invalid", i)
		}
		if i > 0 && trial.TrialID == report.Trials[i-1].TrialID {
			return fmt.Errorf("backend measurement: duplicate trial_id=%q", trial.TrialID)
		}
		matrixCase, ok := matrixCaseByID(report.TrialPlan.MatrixCases, trial.CaseID)
		if !ok {
			return fmt.Errorf("backend measurement: trial %s references unknown matrix case %q", trial.TrialID, trial.CaseID)
		}
		if trial.Warmup {
			warmups++
		} else {
			attempted++
		}
		if trial.Status == TrialSucceeded {
			if trial.DurationUS == 0 || trial.FailureReason != "" {
				return fmt.Errorf("backend measurement: successful trial %s requires duration and no failure reason", trial.TrialID)
			}
		} else if trial.Status == TrialFailed {
			if trial.DurationUS != 0 || strings.TrimSpace(trial.FailureReason) == "" || trial.FailureReason != strings.TrimSpace(trial.FailureReason) {
				return fmt.Errorf("backend measurement: failed trial %s requires zero duration and a trimmed reason", trial.TrialID)
			}
		} else {
			return fmt.Errorf("backend measurement: trial %s status=%q is invalid", trial.TrialID, trial.Status)
		}
		if err := validateTrialKind(report.Definition.Kind, trial, matrixCase); err != nil {
			return err
		}
	}
	if attempted > report.TrialPlan.PlannedTrialCount || warmups != report.TrialPlan.WarmupCount {
		return fmt.Errorf("backend measurement: trial counts do not match trial plan")
	}
	return nil
}

func validateTrialKind(kind MeasurementKind, trial Trial, matrixCase MatrixCase) error {
	switch kind {
	case KindChannelCapacity:
		if trial.Timing != nil || trial.Channel == nil || trial.Channel.PayloadBytes == 0 {
			return fmt.Errorf("backend measurement: channel trial %s has incompatible fields", trial.TrialID)
		}
		if trial.Channel.ProfileID != matrixCase.ProfileID || trial.Channel.ChannelRateBPS != matrixCase.Channel.ChannelRateBPS || trial.Channel.PayloadBytes != matrixCase.Channel.PayloadBytes || trial.Channel.OverheadBytes != matrixCase.Channel.OverheadBytes {
			return fmt.Errorf("backend measurement: channel trial %s does not match matrix case", trial.TrialID)
		}
	case KindTimingAcquisition:
		if trial.Channel != nil || (trial.Status == TrialSucceeded && trial.Timing == nil) {
			return fmt.Errorf("backend measurement: timing trial %s has incompatible fields", trial.TrialID)
		}
	default:
		if trial.Channel != nil || trial.Timing != nil {
			return fmt.Errorf("backend measurement: turnaround trial %s has incompatible fields", trial.TrialID)
		}
	}
	return nil
}

func matrixCaseByID(cases []MatrixCase, id string) (MatrixCase, bool) {
	index, found := slices.BinarySearchFunc(cases, id, func(matrixCase MatrixCase, target string) int {
		return strings.Compare(matrixCase.CaseID, target)
	})
	if !found {
		return MatrixCase{}, false
	}
	return cases[index], true
}

func validateClockQuantization(report MeasurementReport) error {
	if report.Clock.Status != ClockKnown {
		return nil
	}
	resolution := report.Clock.ResolutionNS
	for _, trial := range report.Trials {
		if trial.Status == TrialSucceeded && !isWholeClockTicks(trial.DurationUS, resolution) {
			return fmt.Errorf("backend measurement: trial %s duration is below or off clock resolution", trial.TrialID)
		}
		if trial.Timing != nil {
			if !isWholeClockTicks(absInt64(trial.Timing.BoundaryErrorUS), resolution) || !isWholeClockTicks(trial.Timing.GuardUsedUS, resolution) {
				return fmt.Errorf("backend measurement: trial %s timing values are off clock resolution", trial.TrialID)
			}
		}
	}
	if report.Thresholds != nil {
		if !isWholeClockTicks(report.Thresholds.Duration.P95MaxUS, resolution) || !isWholeClockTicks(report.Thresholds.Duration.MaxUS, resolution) {
			return fmt.Errorf("backend measurement: duration thresholds are off clock resolution")
		}
		if report.Thresholds.Timing != nil && (!isWholeClockTicks(report.Thresholds.Timing.MaxAbsBoundaryErrorUS, resolution) || !isWholeClockTicks(report.Thresholds.Timing.MaxGuardUsedUS, resolution)) {
			return fmt.Errorf("backend measurement: timing thresholds are off clock resolution")
		}
	}
	return nil
}

func isWholeClockTicks(microseconds, resolutionNS uint64) bool {
	if microseconds == 0 {
		return true
	}
	if resolutionNS == 0 || microseconds > ^uint64(0)/1000 {
		return false
	}
	return microseconds*1000%resolutionNS == 0
}

func validIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func validSHA256(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func hasDuplicateStrings(values []string) bool {
	for i := 1; i < len(values); i++ {
		if values[i] == values[i-1] {
			return true
		}
	}
	return false
}
