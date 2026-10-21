package scenario

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"

	"github.com/rmsff/ad-hoc-radio/internal/routing"
)

func RunRelayTreeStressMatrix(cfg RelayTreeStressMatrixConfig) (RelayTreeStressMatrixReport, error) {
	if err := validateRelayTreeStressMatrixConfig(cfg); err != nil {
		return RelayTreeStressMatrixReport{}, err
	}
	cases := cloneRelayTreeStressCases(cfg.Cases)
	sort.Slice(cases, func(i, j int) bool { return cases[i].CaseID < cases[j].CaseID })
	report := RelayTreeStressMatrixReport{
		StoryID:             RelayTreeStressStoryID,
		ScenarioID:          RelayTreeStressScenarioID,
		EvidenceType:        RelayTreeStressType,
		ArchitectureVerdict: RelayTreeArchitectureAccepted,
		CaseCount:           len(cases),
		Rows:                make([]RelayTreeStressMatrixRow, 0, len(cases)*len(relayTreeStressProfileOrder)),
	}
	for _, testCase := range cases {
		comparison, err := RunTimingProfileComparison([]TimingProfileComparisonFixture{{
			FixtureID:     testCase.CaseID,
			BaseConfig:    testCase.BaseConfig,
			TimingOverlay: testCase.TimingOverlay,
		}})
		if err != nil {
			return RelayTreeStressMatrixReport{}, fmt.Errorf("relay-tree stress matrix: case %s: %w", testCase.CaseID, err)
		}
		for _, comparisonRow := range comparison.Rows {
			row, err := buildRelayTreeStressRow(testCase, comparisonRow)
			if err != nil {
				return RelayTreeStressMatrixReport{}, fmt.Errorf("relay-tree stress matrix: row %s: %w", comparisonRow.RowID, err)
			}
			report.Rows = append(report.Rows, row)
		}
	}
	report.RowCount = len(report.Rows)
	report.ArchitectureVerdict = relayTreeStressArchitectureVerdict(report.Rows)
	summaries, err := buildRelayTreeProfileSummaries(report.Rows, cfg.BackendAssessments)
	if err != nil {
		return RelayTreeStressMatrixReport{}, err
	}
	report.ProfileSummaries = summaries
	report.Recommendation = recommendRelayTreeStressProfile(summaries)
	if err := finalizeRelayTreeStressEvidence(&report); err != nil {
		return RelayTreeStressMatrixReport{}, err
	}
	return report, nil
}

func relayTreeStressArchitectureVerdict(rows []RelayTreeStressMatrixRow) RelayTreeArchitectureVerdict {
	type caseInvariantState struct {
		fitRows       int
		invariantFail int
		role          RelayTreeStressCaseRole
	}
	states := make(map[string]caseInvariantState)
	for _, row := range rows {
		state := states[row.CaseID]
		state.role = row.CaseRole
		if row.Comparison.ProfileFit {
			state.fitRows++
			if !row.InvariantPass {
				state.invariantFail++
			}
		}
		states[row.CaseID] = state
	}
	for _, state := range states {
		if state.role != RelayTreeStressRoleBoundary && state.fitRows > 0 && state.invariantFail == state.fitRows {
			return RelayTreeArchitectureReviewRequired
		}
	}
	return RelayTreeArchitectureAccepted
}

func buildRelayTreeStressRow(testCase RelayTreeStressCase, comparison TimingProfileComparisonRow) (RelayTreeStressMatrixRow, error) {
	row := RelayTreeStressMatrixRow{
		RowID:               comparison.RowID,
		CaseID:              testCase.CaseID,
		CaseClass:           testCase.Class,
		CaseRole:            testCase.Role,
		ExpectedGroupSize:   testCase.ExpectedGroupSize,
		RequestedRelayDepth: testCase.RequestedRelayDepth,
		BranchWidth:         testCase.BranchWidth,
		StressTags:          append([]RelayTreeStressTag(nil), testCase.StressTags...),
		InvariantPass:       true,
		ReasonCodes:         []string{},
		Comparison:          comparison,
	}
	if testCase.RequestedRelayDepth == 3 {
		counterfactual, err := relayTreeDepthThreeCounterfactualCoverage(testCase, comparison.Profile)
		if err != nil {
			return RelayTreeStressMatrixRow{}, err
		}
		row.CounterfactualEvaluated = true
		row.Depth3CounterfactualCovered = counterfactual
	}
	if !comparison.ProfileFit {
		row.ReasonCodes = append(row.ReasonCodes, string(comparison.FitReason))
		switch {
		case testCase.Role == RelayTreeStressRoleBoundary && testCase.Expectation.AllowProfileRejection:
			row.Classification = RelayTreeStressDegraded
		case testCase.Role == RelayTreeStressRoleCore && testCase.RequestedRelayDepth >= 2:
			row.Classification = RelayTreeStressDegraded
		default:
			row.Classification = RelayTreeStressNoGo
			row.InvariantPass = false
		}
		return row, nil
	}
	if comparison.RuntimeOutcome != TimingRuntimePass {
		row.Classification = RelayTreeStressNoGo
		row.InvariantPass = false
		row.ReasonCodes = append(row.ReasonCodes, "runtime_not_pass")
		return row, nil
	}

	row.DeliveryRatioPct = stressDeliveryRatio(comparison)
	row.MinMemberDeliveryPct = stressMinMemberDeliveryPct(comparison)
	row.ReasonCodes = append(row.ReasonCodes, relayTreeStressInvariantReasons(testCase, comparison, row.DeliveryRatioPct, row.MinMemberDeliveryPct)...)
	if testCase.RequestedRelayDepth == 3 {
		row.Depth3CoverageGain = len(comparison.CoveredMemberIDs) - row.Depth3CounterfactualCovered
		if row.Depth3CoverageGain < 0 {
			return RelayTreeStressMatrixRow{}, fmt.Errorf("depth-3 coverage gain is negative")
		}
	}
	if len(row.ReasonCodes) != 0 {
		row.Classification = RelayTreeStressNoGo
		row.InvariantPass = false
		return row, nil
	}
	row.Classification = RelayTreeStressPass
	if testCase.RequestedRelayDepth == 3 {
		row.Depth3AddsUsefulCoverage = row.Depth3CoverageGain > 0
	}
	return row, nil
}

func relayTreeStressInvariantReasons(
	testCase RelayTreeStressCase,
	row TimingProfileComparisonRow,
	deliveryRatio float64,
	minMemberDeliveryPct float64,
) []string {
	reasons := make([]string, 0, 8)
	if row.DuplicatePlayoutCount != 0 {
		reasons = append(reasons, "duplicate_playout_nonzero")
	}
	if row.OutOfScheduleTxCount != 0 {
		reasons = append(reasons, "out_of_schedule_tx_nonzero")
	}
	if !stressMemberPartitionMatches(row.ExpectedMemberIDs, row.CoveredMemberIDs, row.UncoveredMemberIDs) {
		reasons = append(reasons, "expected_member_partition_mismatch")
	}
	if !stressMemberPartitionMatches(row.ExpectedMemberIDs, row.DeliveredMemberIDs, row.UncoveredMemberIDs) {
		reasons = append(reasons, "delivered_member_partition_mismatch")
	}
	if !stressSubset(row.DeliveredMemberIDs, row.CoveredMemberIDs) {
		reasons = append(reasons, "delivered_member_not_covered")
	}
	if testCase.Role == RelayTreeStressRoleCore {
		if row.SelectedRelayDepth != testCase.RequestedRelayDepth {
			reasons = append(reasons, "selected_depth_mismatch")
		}
		if !slices.Equal(row.CoveredMemberIDs, row.ExpectedMemberIDs) || !slices.Equal(row.DeliveredMemberIDs, row.ExpectedMemberIDs) {
			reasons = append(reasons, "core_group_not_fully_delivered")
		}
		if deliveryRatio < 95 {
			reasons = append(reasons, "core_delivery_below_95_pct")
		}
		if minMemberDeliveryPct < 95 {
			reasons = append(reasons, "member_delivery_below_95_pct")
		}
	}
	if !slices.Equal(row.UncoveredMemberIDs, testCase.Expectation.ExpectedUncoveredMemberIDs) {
		reasons = append(reasons, "unexpected_uncovered_members")
	}
	if testCase.Expectation.RequireAllDelivered {
		if len(row.UncoveredMemberIDs) != 0 || !slices.Equal(row.DeliveredMemberIDs, row.ExpectedMemberIDs) {
			reasons = append(reasons, "required_members_not_delivered")
		}
	}
	if row.RouteReplanCount < testCase.Expectation.MinRouteReplans {
		reasons = append(reasons, "route_replan_count_below_expectation")
	}
	if row.VersionActivationCount < testCase.Expectation.MinVersionActivations {
		reasons = append(reasons, "version_activation_count_below_expectation")
	}
	return sortedUniqueStrings(reasons)
}

func stressDeliveryRatio(row TimingProfileComparisonRow) float64 {
	denominator := row.SourceFramesCreated * len(row.CoveredMemberIDs)
	if denominator == 0 {
		if row.DeliveredFrameCount == 0 {
			return 100
		}
		return 0
	}
	return math.Round(float64(row.DeliveredFrameCount)*10_000/float64(denominator)) / 100
}

func stressMinMemberDeliveryPct(row TimingProfileComparisonRow) float64 {
	if len(row.CoveredMemberIDs) == 0 {
		return 100
	}
	accepted := make(map[uint16]int, len(row.MemberDelivery))
	for _, member := range row.MemberDelivery {
		accepted[member.MemberID] = member.FramesAccepted
	}
	minimum := 100.0
	for _, memberID := range row.CoveredMemberIDs {
		pct := 0.0
		if row.SourceFramesCreated > 0 {
			pct = math.Round(float64(accepted[memberID])*10_000/float64(row.SourceFramesCreated)) / 100
		}
		if pct < minimum {
			minimum = pct
		}
	}
	return minimum
}

func relayTreeDepthThreeCounterfactualCoverage(testCase RelayTreeStressCase, profile TimingProfile) (int, error) {
	cfg := timingProfileScenarioConfig(testCase.BaseConfig, profile)
	plan := cloneTimingRelayTreePlan(cfg.InitialPlan)
	plan.Constraints.MaxRelayDepth = 2
	plan.Constraints.RequireAllExpectedMembers = false
	result, err := routing.PlanRelayTree(scheduledRelayTreePlanRequest(cfg.SourceStationID, plan))
	if errors.Is(err, routing.ErrNoFeasibleRelayTree) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("depth-3 counterfactual planning: %w", err)
	}
	return len(result.Selected.Tree.Coverage.CoveredMemberIDs), nil
}

func stressMemberPartitionMatches(expected, covered, uncovered []uint16) bool {
	combined := append(append([]uint16(nil), covered...), uncovered...)
	sort.Slice(combined, func(i, j int) bool { return combined[i] < combined[j] })
	return slices.Equal(combined, expected) && !stressSlicesOverlap(covered, uncovered)
}

func stressSubset(values, superset []uint16) bool {
	for _, value := range values {
		if !slices.Contains(superset, value) {
			return false
		}
	}
	return true
}

func stressSlicesOverlap(left, right []uint16) bool {
	for _, value := range left {
		if slices.Contains(right, value) {
			return true
		}
	}
	return false
}

func sortedUniqueStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := append([]string(nil), values...)
	sort.Strings(out)
	write := 0
	for _, value := range out {
		if write == 0 || out[write-1] != value {
			out[write] = value
			write++
		}
	}
	return out[:write]
}
