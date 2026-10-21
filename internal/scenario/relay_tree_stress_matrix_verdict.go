package scenario

import (
	"fmt"
	"math"
)

func buildRelayTreeProfileSummaries(
	rows []RelayTreeStressMatrixRow,
	assessments []RelayTreeBackendAssessment,
) ([]RelayTreeProfileSummary, error) {
	backend := make(map[string]RelayTreeBackendAssessment, len(assessments))
	for _, assessment := range assessments {
		backend[relayTreeStressProfileID(assessment.TimingStrategy, assessment.Profile)] = assessment
	}
	summaries := make([]RelayTreeProfileSummary, 0, len(relayTreeStressProfileOrder))
	for _, key := range relayTreeStressProfileOrder {
		summary := RelayTreeProfileSummary{
			ProfileID:         relayTreeStressProfileID(key.Strategy, key.Profile),
			TimingStrategy:    key.Strategy,
			Profile:           key.Profile,
			BackendStatus:     RelayTreeBackendNotEvaluated,
			OperatingEnvelope: []string{},
			ReasonCodes:       []string{},
		}
		if assessment, ok := backend[summary.ProfileID]; ok {
			summary.BackendStatus = assessment.Status
			summary.BackendFailureReason = assessment.Reason
			if assessment.Evidence != nil {
				summary.BackendEvidenceGrade = assessment.Evidence.EvidenceGrade
				summary.BackendEvidenceHash = assessment.Evidence.Determinism.EventHash
			}
		}
		if err := aggregateRelayTreeProfileRows(&summary, rows); err != nil {
			return nil, fmt.Errorf("relay-tree stress matrix: profile %s: %w", summary.ProfileID, err)
		}
		finalizeRelayTreeProfileVerdict(&summary)
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

func aggregateRelayTreeProfileRows(summary *RelayTreeProfileSummary, rows []RelayTreeStressMatrixRow) error {
	depthTotal := make(map[uint8]int, 4)
	depthPass := make(map[uint8]int, 4)
	depthThreeUseful := false
	for _, row := range rows {
		if row.Comparison.TimingStrategy != string(summary.TimingStrategy) || row.Comparison.Profile != summary.Profile {
			continue
		}
		summary.TotalRows++
		switch row.Classification {
		case RelayTreeStressPass:
			summary.PassRows++
		case RelayTreeStressDegraded:
			summary.DegradedRows++
		case RelayTreeStressNoGo:
			summary.NoGoRows++
			summary.ReasonCodes = append(summary.ReasonCodes, row.ReasonCodes...)
		default:
			return fmt.Errorf("row %s classification=%q is invalid", row.RowID, row.Classification)
		}
		if row.CaseRole == RelayTreeStressRoleCore {
			summary.CoreRows++
			depthTotal[row.RequestedRelayDepth]++
			if row.Classification == RelayTreeStressPass {
				summary.CorePassRows++
				depthPass[row.RequestedRelayDepth]++
			}
			if row.RequestedRelayDepth == 3 && row.Depth3AddsUsefulCoverage {
				depthThreeUseful = true
			}
		}
		if row.CaseClass == RelayTreeStressClassBranch {
			if row.Comparison.ProfileFit && row.Classification == RelayTreeStressPass && row.BranchWidth > summary.MaxPassingBranchWidth {
				summary.MaxPassingBranchWidth = row.BranchWidth
			}
			if !row.Comparison.ProfileFit && (summary.FirstImpracticalBranchWidth == 0 || row.BranchWidth < summary.FirstImpracticalBranchWidth) {
				summary.FirstImpracticalBranchWidth = row.BranchWidth
			}
		}
		var err error
		summary.DriftCollisionCount, err = checkedStressIntAdd(summary.DriftCollisionCount, row.Comparison.DriftCollisionCount)
		if err != nil {
			return err
		}
		summary.ResyncCount, err = checkedStressIntAdd(summary.ResyncCount, row.Comparison.ResyncCount)
		if err != nil {
			return err
		}
		summary.RouteReplanCount, err = checkedStressIntAdd(summary.RouteReplanCount, row.Comparison.RouteReplanCount)
		if err != nil {
			return err
		}
		summary.PTTAccessDelayMS, err = checkedStressInt64Add(summary.PTTAccessDelayMS, row.Comparison.PTTAccessDelayMS)
		if err != nil {
			return err
		}
		summary.ScheduleAcquisitionMS, err = checkedStressInt64Add(summary.ScheduleAcquisitionMS, row.Comparison.ScheduleAcquisitionMS)
		if err != nil {
			return err
		}
		summary.LateEntryAcquisitionMS, err = checkedStressInt64Add(summary.LateEntryAcquisitionMS, row.Comparison.LateEntryAcquisitionMS)
		if err != nil {
			return err
		}
	}
	if summary.TotalRows == 0 {
		return fmt.Errorf("profile has no rows")
	}
	var err error
	summary.AccessAcquisitionTotalMS, err = checkedStressInt64Add(summary.PTTAccessDelayMS, summary.ScheduleAcquisitionMS)
	if err != nil {
		return err
	}
	summary.AccessAcquisitionTotalMS, err = checkedStressInt64Add(summary.AccessAcquisitionTotalMS, summary.LateEntryAcquisitionMS)
	if err != nil {
		return err
	}
	for depth := uint8(0); depth <= 3; depth++ {
		if depthTotal[depth] == 0 || depthPass[depth] != depthTotal[depth] {
			break
		}
		summary.MaxSupportedRelayDepth = depth
	}
	if summary.MaxSupportedRelayDepth >= 1 && summary.MaxPassingBranchWidth == 0 {
		summary.MaxPassingBranchWidth = 1
	}
	summary.Depth3AddsUsefulCoverage = depthTotal[3] > 0 && depthPass[3] == depthTotal[3] && depthThreeUseful
	return nil
}

func finalizeRelayTreeProfileVerdict(summary *RelayTreeProfileSummary) {
	switch {
	case summary.BackendStatus == RelayTreeBackendFailed:
		summary.Verdict = RelayTreeProfileNoGo
		summary.ReasonCodes = append(summary.ReasonCodes, "backend_failed")
	case summary.NoGoRows > 0 || summary.MaxSupportedRelayDepth < 1:
		summary.Verdict = RelayTreeProfileNoGo
		summary.ReasonCodes = append(summary.ReasonCodes, "mandatory_profile_invariant_failed")
	case summary.MaxSupportedRelayDepth < 3 || !summary.Depth3AddsUsefulCoverage:
		summary.Verdict = RelayTreeProfileDegradedOnly
		summary.ReasonCodes = append(summary.ReasonCodes, "full_depth_envelope_not_supported")
	default:
		summary.Verdict = RelayTreeProfileGo
	}
	if summary.FirstImpracticalBranchWidth != 0 {
		summary.SerializedScheduleAssessment = fmt.Sprintf("impractical_at_branch_width_%d", summary.FirstImpracticalBranchWidth)
	} else {
		summary.SerializedScheduleAssessment = "no_impractical_width_observed"
	}
	summary.OperatingEnvelope = []string{
		fmt.Sprintf("max_relay_depth=%d", summary.MaxSupportedRelayDepth),
		fmt.Sprintf("max_serialized_branch_width=%d", summary.MaxPassingBranchWidth),
		"simulator_only=true",
		fmt.Sprintf("backend_status=%s", summary.BackendStatus),
	}
	summary.ReasonCodes = sortedUniqueStrings(summary.ReasonCodes)
}

func recommendRelayTreeStressProfile(summaries []RelayTreeProfileSummary) string {
	if len(summaries) == 0 {
		return "insufficient_evidence"
	}
	best := summaries[0]
	tied := false
	for _, candidate := range summaries[1:] {
		comparison := compareRelayTreeProfileSummary(candidate, best)
		if comparison > 0 {
			best = candidate
			tied = false
		} else if comparison == 0 {
			tied = true
		}
	}
	if tied || best.Verdict == RelayTreeProfileNoGo {
		return "insufficient_evidence"
	}
	return best.ProfileID
}

func compareRelayTreeProfileSummary(left, right RelayTreeProfileSummary) int {
	if result := compareStressInt(profileVerdictRank(left.Verdict), profileVerdictRank(right.Verdict)); result != 0 {
		return result
	}
	if result := compareStressInt(int(left.MaxSupportedRelayDepth), int(right.MaxSupportedRelayDepth)); result != 0 {
		return result
	}
	if result := compareStressInt(left.MaxPassingBranchWidth, right.MaxPassingBranchWidth); result != 0 {
		return result
	}
	if result := compareStressInt(right.DriftCollisionCount, left.DriftCollisionCount); result != 0 {
		return result
	}
	if result := compareStressInt(right.ResyncCount, left.ResyncCount); result != 0 {
		return result
	}
	if result := compareStressInt(right.RouteReplanCount, left.RouteReplanCount); result != 0 {
		return result
	}
	return compareStressInt64(right.AccessAcquisitionTotalMS, left.AccessAcquisitionTotalMS)
}

func profileVerdictRank(verdict RelayTreeProfileVerdict) int {
	switch verdict {
	case RelayTreeProfileGo:
		return 3
	case RelayTreeProfileDegradedOnly:
		return 2
	case RelayTreeProfileNoGo:
		return 1
	default:
		return 0
	}
}

func compareStressInt(left, right int) int {
	switch {
	case left > right:
		return 1
	case left < right:
		return -1
	default:
		return 0
	}
}

func compareStressInt64(left, right int64) int {
	switch {
	case left > right:
		return 1
	case left < right:
		return -1
	default:
		return 0
	}
}

func checkedStressIntAdd(total, add int) (int, error) {
	if add < 0 || total > math.MaxInt-add {
		return 0, fmt.Errorf("aggregate integer count overflows int")
	}
	return total + add, nil
}

func checkedStressInt64Add(total, add int64) (int64, error) {
	if add < 0 || total > math.MaxInt64-add {
		return 0, fmt.Errorf("aggregate timing value overflows int64")
	}
	return total + add, nil
}
