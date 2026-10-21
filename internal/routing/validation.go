package routing

import (
	"fmt"
	"math"
)

func ValidateExpectedGroupSnapshot(snapshot ExpectedGroupSnapshot, sourceID uint16, planningAtMS int64) error {
	if sourceID == 0 {
		return fmt.Errorf("expected group snapshot: source_id must be > 0")
	}
	if snapshot.ID == 0 {
		return fmt.Errorf("expected group snapshot: id must be > 0")
	}
	if snapshot.ObservedAtMS < 0 {
		return fmt.Errorf("expected group snapshot: observed_at_ms=%d must be >= 0", snapshot.ObservedAtMS)
	}
	if snapshot.FreshnessMS == 0 {
		return fmt.Errorf("expected group snapshot: freshness_ms must be > 0")
	}
	if err := validateObservationFreshness("expected group snapshot", snapshot.ObservedAtMS, snapshot.FreshnessMS, planningAtMS); err != nil {
		return err
	}
	if err := validateStrictlyAscendingStationIDs("expected group snapshot: member_ids", snapshot.MemberIDs); err != nil {
		return err
	}
	for _, memberID := range snapshot.MemberIDs {
		if memberID == sourceID {
			return fmt.Errorf("expected group snapshot: member_ids contains source_id=%d", sourceID)
		}
	}
	return nil
}

func ValidateNeighborGraph(graph NeighborGraph, planningAtMS int64) error {
	return validateNeighborGraph(graph, planningAtMS, false)
}

func validateNeighborGraph(graph NeighborGraph, planningAtMS int64, allowStale bool) error {
	if graph.Epoch == 0 {
		return fmt.Errorf("neighbor graph: epoch must be > 0")
	}
	if graph.FreshnessMS == 0 {
		return fmt.Errorf("neighbor graph: freshness_ms must be > 0")
	}
	if planningAtMS < 0 {
		return fmt.Errorf("neighbor graph: planning_at_ms=%d must be >= 0", planningAtMS)
	}

	for i, link := range graph.Links {
		if link.FromStationID == 0 {
			return fmt.Errorf("neighbor graph: link %d from_station_id must be > 0", i)
		}
		if link.ToStationID == 0 {
			return fmt.Errorf("neighbor graph: link %d to_station_id must be > 0", i)
		}
		if link.FromStationID == link.ToStationID {
			return fmt.Errorf("neighbor graph: link %d is a self link for station_id=%d", i, link.FromStationID)
		}
		if i > 0 && compareDirectedLinks(graph.Links[i-1], link) >= 0 {
			return fmt.Errorf("neighbor graph: links must be strictly ascending by (from_station_id, to_station_id); invalid index %d", i)
		}
		if err := validateUnitInterval(fmt.Sprintf("neighbor graph: link %d quality", i), link.Quality); err != nil {
			return err
		}
		if err := validateUnitInterval(fmt.Sprintf("neighbor graph: link %d loss", i), link.Loss); err != nil {
			return err
		}
		if link.DelayMS < 0 {
			return fmt.Errorf("neighbor graph: link %d delay_ms=%d must be >= 0", i, link.DelayMS)
		}
		if link.ObservedAtMS < 0 {
			return fmt.Errorf("neighbor graph: link %d observed_at_ms=%d must be >= 0", i, link.ObservedAtMS)
		}
		if allowStale && link.ObservedAtMS <= planningAtMS && planningAtMS-link.ObservedAtMS > int64(graph.FreshnessMS) {
			continue
		}
		if err := validateObservationFreshness(fmt.Sprintf("neighbor graph: link %d", i), link.ObservedAtMS, graph.FreshnessMS, planningAtMS); err != nil {
			return err
		}
	}
	return nil
}

func ValidatePlanningConstraints(constraints PlanningConstraints) error {
	if constraints.PlanningAtMS < 0 {
		return fmt.Errorf("planning constraints: planning_at_ms=%d must be >= 0", constraints.PlanningAtMS)
	}
	if constraints.MaxRelayDepth > MaxSupportedRelayDepth {
		return fmt.Errorf("planning constraints: max_relay_depth=%d must be in [0, %d]", constraints.MaxRelayDepth, MaxSupportedRelayDepth)
	}
	if constraints.MaxPlayoutLatencyMS <= 0 {
		return fmt.Errorf("planning constraints: max_playout_latency_ms=%d must be > 0", constraints.MaxPlayoutLatencyMS)
	}
	if math.IsNaN(constraints.MaxVoiceAirtimePct) || math.IsInf(constraints.MaxVoiceAirtimePct, 0) || constraints.MaxVoiceAirtimePct <= 0 || constraints.MaxVoiceAirtimePct > 100 {
		return fmt.Errorf("planning constraints: max_voice_airtime_pct=%g must be finite and in (0, 100]", constraints.MaxVoiceAirtimePct)
	}
	if err := validateUnitInterval("planning constraints: min_relay_battery_fraction", constraints.MinRelayBatteryFraction); err != nil {
		return err
	}
	if err := validateUnitInterval("planning constraints: min_link_quality", constraints.MinLinkQuality); err != nil {
		return err
	}
	if err := validateUnitInterval("planning constraints: max_link_loss", constraints.MaxLinkLoss); err != nil {
		return err
	}
	return nil
}

func ValidatePlanRequest(request PlanRequest) error {
	if request.SourceStationID == 0 {
		return fmt.Errorf("plan request: source_station_id must be > 0")
	}
	if err := ValidateExpectedGroupSnapshot(request.Snapshot, request.SourceStationID, request.Constraints.PlanningAtMS); err != nil {
		return err
	}
	if err := ValidateNeighborGraph(request.Graph, request.Constraints.PlanningAtMS); err != nil {
		return err
	}
	if err := ValidatePlanningConstraints(request.Constraints); err != nil {
		return err
	}
	if err := ValidatePlanningCostModel(request.CostModel); err != nil {
		return err
	}
	if err := ValidatePlanningSearchLimits(request.SearchLimits); err != nil {
		return err
	}
	if err := ValidateRelayStates(request.RelayStates, request.SourceStationID); err != nil {
		return err
	}
	return nil
}

func ValidatePlanningCostModel(cost PlanningCostModel) error {
	if cost.SourceOpportunityMS <= 0 {
		return fmt.Errorf("planning cost model: source_opportunity_ms=%d must be > 0", cost.SourceOpportunityMS)
	}
	if cost.RelayOpportunityMS <= 0 {
		return fmt.Errorf("planning cost model: relay_opportunity_ms=%d must be > 0", cost.RelayOpportunityMS)
	}
	if cost.GuardAfterMS < 0 {
		return fmt.Errorf("planning cost model: guard_after_ms=%d must be >= 0", cost.GuardAfterMS)
	}
	if cost.FramePeriodMS <= 0 {
		return fmt.Errorf("planning cost model: frame_period_ms=%d must be > 0", cost.FramePeriodMS)
	}
	if _, ok := checkedAddInt64(cost.SourceOpportunityMS, cost.GuardAfterMS); !ok {
		return fmt.Errorf("planning cost model: source_opportunity_ms + guard_after_ms overflows int64")
	}
	if _, ok := checkedAddInt64(cost.RelayOpportunityMS, cost.GuardAfterMS); !ok {
		return fmt.Errorf("planning cost model: relay_opportunity_ms + guard_after_ms overflows int64")
	}
	if _, ok := checkedMulInt64(cost.SourceOpportunityMS, 100_000); !ok {
		return fmt.Errorf("planning cost model: source airtime arithmetic overflows int64")
	}
	if _, ok := checkedMulInt64(cost.RelayOpportunityMS, 100_000); !ok {
		return fmt.Errorf("planning cost model: relay airtime arithmetic overflows int64")
	}
	return nil
}

func ValidatePlanningSearchLimits(limits PlanningSearchLimits) error {
	if limits.MaxSearchSteps < 0 {
		return fmt.Errorf("planning search limits: max_search_steps must be >= 0")
	}
	if limits.MaxPathsPerMember <= 0 {
		return fmt.Errorf("planning search limits: max_paths_per_member=%d must be > 0", limits.MaxPathsPerMember)
	}
	if limits.MaxTreeCandidates <= 0 {
		return fmt.Errorf("planning search limits: max_tree_candidates=%d must be > 0", limits.MaxTreeCandidates)
	}
	if limits.MaxFeasibleTrees <= 0 {
		return fmt.Errorf("planning search limits: max_feasible_trees=%d must be > 0", limits.MaxFeasibleTrees)
	}
	return nil
}

func ValidateRelayStates(states []RelayState, sourceID uint16) error {
	for i, state := range states {
		if state.StationID == 0 {
			return fmt.Errorf("relay states: state %d station_id must be > 0", i)
		}
		if state.StationID == sourceID {
			return fmt.Errorf("relay states: station_id=%d must exclude source", sourceID)
		}
		if i > 0 && states[i-1].StationID >= state.StationID {
			return fmt.Errorf("relay states: station_id values must be strictly ascending; invalid index %d", i)
		}
		if err := validateUnitInterval(fmt.Sprintf("relay states: state %d battery_fraction", i), state.BatteryFraction); err != nil {
			return err
		}
	}
	return nil
}

func ValidateRelayTree(snapshot ExpectedGroupSnapshot, graph NeighborGraph, constraints PlanningConstraints, tree RelayTree) error {
	if err := ValidatePlanningConstraints(constraints); err != nil {
		return err
	}
	reachable, err := validateRelayTreeStructure(tree)
	if err != nil {
		return err
	}
	if err := ValidateExpectedGroupSnapshot(snapshot, tree.SourceStationID, constraints.PlanningAtMS); err != nil {
		return err
	}
	if err := ValidateNeighborGraph(graph, constraints.PlanningAtMS); err != nil {
		return err
	}
	if tree.MembershipSnapshotID != snapshot.ID {
		return fmt.Errorf("relay tree: membership_snapshot_id=%d does not match snapshot id=%d", tree.MembershipSnapshotID, snapshot.ID)
	}
	if tree.TopologyEpoch != graph.Epoch {
		return fmt.Errorf("relay tree: topology_epoch=%d does not match graph epoch=%d", tree.TopologyEpoch, graph.Epoch)
	}
	if tree.MaxRelayDepth > constraints.MaxRelayDepth {
		return fmt.Errorf("relay tree: selected max_relay_depth=%d exceeds configured max_relay_depth=%d", tree.MaxRelayDepth, constraints.MaxRelayDepth)
	}

	if err := validateTreeLinkEvidence(tree, graph, constraints); err != nil {
		return err
	}
	if err := validateCoverage(snapshot.MemberIDs, tree.Coverage, reachable, constraints.RequireAllExpectedMembers); err != nil {
		return err
	}
	return nil
}

func validateRelayTreeStructure(tree RelayTree) (map[uint16]struct{}, error) {
	if tree.SourceStationID == 0 {
		return nil, fmt.Errorf("relay tree: source_station_id must be > 0")
	}
	if tree.MembershipSnapshotID == 0 {
		return nil, fmt.Errorf("relay tree: membership_snapshot_id must be > 0")
	}
	if tree.TopologyEpoch == 0 {
		return nil, fmt.Errorf("relay tree: topology_epoch must be > 0")
	}
	if tree.MaxRelayDepth > MaxSupportedRelayDepth {
		return nil, fmt.Errorf("relay tree: max_relay_depth=%d exceeds supported maximum %d", tree.MaxRelayDepth, MaxSupportedRelayDepth)
	}
	if err := validateStrictlyAscendingStationIDs("relay tree: covered_member_ids", tree.Coverage.CoveredMemberIDs); err != nil {
		return nil, err
	}
	if err := validateStrictlyAscendingStationIDs("relay tree: uncovered_member_ids", tree.Coverage.UncoveredMemberIDs); err != nil {
		return nil, err
	}
	if err := validateCoverageDisjoint(tree.Coverage); err != nil {
		return nil, err
	}
	for _, id := range tree.Coverage.CoveredMemberIDs {
		if id == tree.SourceStationID {
			return nil, fmt.Errorf("relay tree: source_station_id=%d cannot be a covered member", tree.SourceStationID)
		}
	}
	for _, id := range tree.Coverage.UncoveredMemberIDs {
		if id == tree.SourceStationID {
			return nil, fmt.Errorf("relay tree: source_station_id=%d cannot be an uncovered member", tree.SourceStationID)
		}
	}

	hasOutgoing := make(map[uint16]struct{}, len(tree.Edges))
	for i, edge := range tree.Edges {
		if edge.ParentStationID == 0 {
			return nil, fmt.Errorf("relay tree: edge %d parent_station_id must be > 0", i)
		}
		if edge.ChildStationID == 0 {
			return nil, fmt.Errorf("relay tree: edge %d child_station_id must be > 0", i)
		}
		if edge.ChildStationID == tree.SourceStationID {
			return nil, fmt.Errorf("relay tree: source_station_id=%d cannot be a child", tree.SourceStationID)
		}
		if edge.ParentStationID == edge.ChildStationID {
			return nil, fmt.Errorf("relay tree: edge %d is a self edge for station_id=%d", i, edge.ParentStationID)
		}
		if edge.Hop > MaxSupportedRelayDepth {
			return nil, fmt.Errorf("relay tree: edge %d hop=%d exceeds supported maximum %d", i, edge.Hop, MaxSupportedRelayDepth)
		}
		if i > 0 && compareRelayTreeEdges(tree.Edges[i-1], edge) >= 0 {
			return nil, fmt.Errorf("relay tree: edges must be strictly ascending by (hop, parent_station_id, child_station_id); invalid index %d", i)
		}
		hasOutgoing[edge.ParentStationID] = struct{}{}
	}

	reachable := map[uint16]struct{}{tree.SourceStationID: {}}
	incomingHop := make(map[uint16]uint8, len(tree.Edges))
	selectedDepth := uint8(0)
	for i, edge := range tree.Edges {
		if _, exists := incomingHop[edge.ChildStationID]; exists {
			return nil, fmt.Errorf("relay tree: child_station_id=%d has multiple parents", edge.ChildStationID)
		}
		expectedHop := uint8(0)
		if edge.ParentStationID != tree.SourceStationID {
			parentHop, exists := incomingHop[edge.ParentStationID]
			if !exists {
				return nil, fmt.Errorf("relay tree: parent_station_id=%d at edge %d is not reachable from source_station_id=%d", edge.ParentStationID, i, tree.SourceStationID)
			}
			expectedHop = parentHop + 1
		}
		if edge.Hop != expectedHop {
			return nil, fmt.Errorf("relay tree: edge %d hop=%d, want %d from parent_station_id=%d", i, edge.Hop, expectedHop, edge.ParentStationID)
		}

		incomingHop[edge.ChildStationID] = edge.Hop
		reachable[edge.ChildStationID] = struct{}{}
		if edge.Hop > selectedDepth {
			selectedDepth = edge.Hop
		}
	}
	if tree.MaxRelayDepth != selectedDepth {
		return nil, fmt.Errorf("relay tree: max_relay_depth=%d does not match selected edge depth=%d", tree.MaxRelayDepth, selectedDepth)
	}
	for _, id := range tree.Coverage.CoveredMemberIDs {
		if _, ok := reachable[id]; !ok {
			return nil, fmt.Errorf("relay tree: covered member_id=%d is not reachable through tree edges", id)
		}
	}
	for _, id := range tree.Coverage.UncoveredMemberIDs {
		if _, ok := reachable[id]; ok {
			return nil, fmt.Errorf("relay tree: reachable member_id=%d is marked uncovered", id)
		}
	}
	declaredExpected := make(map[uint16]struct{}, len(tree.Coverage.CoveredMemberIDs)+len(tree.Coverage.UncoveredMemberIDs))
	for _, id := range tree.Coverage.CoveredMemberIDs {
		declaredExpected[id] = struct{}{}
	}
	for _, id := range tree.Coverage.UncoveredMemberIDs {
		declaredExpected[id] = struct{}{}
	}
	for id := range reachable {
		if id == tree.SourceStationID {
			continue
		}
		if _, expected := declaredExpected[id]; expected {
			continue
		}
		if _, transit := hasOutgoing[id]; !transit {
			return nil, fmt.Errorf("relay tree: reachable non-expected leaf station_id=%d is not a transit relay", id)
		}
	}
	return reachable, nil
}

func validateTreeLinkEvidence(tree RelayTree, graph NeighborGraph, constraints PlanningConstraints) error {
	links := make(map[[2]uint16]DirectedLink, len(graph.Links))
	for _, link := range graph.Links {
		links[[2]uint16{link.FromStationID, link.ToStationID}] = link
	}
	for i, edge := range tree.Edges {
		key := [2]uint16{edge.ParentStationID, edge.ChildStationID}
		link, exists := links[key]
		if !exists {
			return fmt.Errorf("relay tree: edge %d missing directed link %d->%d", i, edge.ParentStationID, edge.ChildStationID)
		}
		if !link.Enabled {
			return fmt.Errorf("relay tree: edge %d directed link %d->%d is disabled", i, edge.ParentStationID, edge.ChildStationID)
		}
		if link.Quality < constraints.MinLinkQuality {
			return fmt.Errorf("relay tree: edge %d directed link %d->%d quality=%g is below min_link_quality=%g", i, edge.ParentStationID, edge.ChildStationID, link.Quality, constraints.MinLinkQuality)
		}
		if link.Loss > constraints.MaxLinkLoss {
			return fmt.Errorf("relay tree: edge %d directed link %d->%d loss=%g exceeds max_link_loss=%g", i, edge.ParentStationID, edge.ChildStationID, link.Loss, constraints.MaxLinkLoss)
		}
	}
	return nil
}

func validateCoverage(expectedIDs []uint16, coverage CoverageSet, reachable map[uint16]struct{}, requireAll bool) error {
	if err := validateStrictlyAscendingStationIDs("relay tree: covered_member_ids", coverage.CoveredMemberIDs); err != nil {
		return err
	}
	if err := validateStrictlyAscendingStationIDs("relay tree: uncovered_member_ids", coverage.UncoveredMemberIDs); err != nil {
		return err
	}
	if err := validateCoverageDisjoint(coverage); err != nil {
		return err
	}

	expected := make(map[uint16]struct{}, len(expectedIDs))
	for _, id := range expectedIDs {
		expected[id] = struct{}{}
	}
	classified := make(map[uint16]bool, len(expectedIDs))
	for _, id := range coverage.CoveredMemberIDs {
		if _, ok := expected[id]; !ok {
			return fmt.Errorf("relay tree: covered member_id=%d is not expected", id)
		}
		classified[id] = true
		if _, ok := reachable[id]; !ok {
			return fmt.Errorf("relay tree: covered member_id=%d is not reachable through tree edges", id)
		}
	}
	for _, id := range coverage.UncoveredMemberIDs {
		if _, ok := expected[id]; !ok {
			return fmt.Errorf("relay tree: uncovered member_id=%d is not expected", id)
		}
		classified[id] = false
		if _, ok := reachable[id]; ok {
			return fmt.Errorf("relay tree: reachable expected member_id=%d is marked uncovered", id)
		}
	}
	if len(classified) != len(expected) {
		return fmt.Errorf("relay tree: coverage does not partition expected members: classified=%d expected=%d", len(classified), len(expected))
	}
	for _, id := range expectedIDs {
		if _, ok := classified[id]; !ok {
			return fmt.Errorf("relay tree: coverage partition is missing expected member_id=%d", id)
		}
	}
	if requireAll && len(coverage.UncoveredMemberIDs) > 0 {
		return fmt.Errorf("relay tree: require_all_expected_members=true but uncovered_member_ids is not empty")
	}
	return nil
}

func validateCoverageDisjoint(coverage CoverageSet) error {
	covered := make(map[uint16]struct{}, len(coverage.CoveredMemberIDs))
	for _, id := range coverage.CoveredMemberIDs {
		covered[id] = struct{}{}
	}
	for _, id := range coverage.UncoveredMemberIDs {
		if _, ok := covered[id]; ok {
			return fmt.Errorf("relay tree: covered/uncovered member sets overlap at member_id=%d", id)
		}
	}
	return nil
}

func validateStrictlyAscendingStationIDs(name string, ids []uint16) error {
	for i, id := range ids {
		if id == 0 {
			return fmt.Errorf("%s[%d] must be > 0", name, i)
		}
		if i > 0 && ids[i-1] >= id {
			return fmt.Errorf("%s must be strictly ascending; invalid index %d", name, i)
		}
	}
	return nil
}

func validateObservationFreshness(name string, observedAtMS int64, freshnessMS uint32, planningAtMS int64) error {
	if planningAtMS < 0 {
		return fmt.Errorf("%s: planning_at_ms=%d must be >= 0", name, planningAtMS)
	}
	if observedAtMS > planningAtMS {
		return fmt.Errorf("%s: observed_at_ms=%d is in the future relative to planning_at_ms=%d", name, observedAtMS, planningAtMS)
	}
	ageMS := planningAtMS - observedAtMS
	if ageMS > int64(freshnessMS) {
		return fmt.Errorf("%s: stale age_ms=%d exceeds freshness_ms=%d", name, ageMS, freshnessMS)
	}
	return nil
}

func validateUnitInterval(name string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
		return fmt.Errorf("%s=%g must be finite and in [0, 1]", name, value)
	}
	return nil
}

func compareDirectedLinks(a, b DirectedLink) int {
	if a.FromStationID < b.FromStationID {
		return -1
	}
	if a.FromStationID > b.FromStationID {
		return 1
	}
	if a.ToStationID < b.ToStationID {
		return -1
	}
	if a.ToStationID > b.ToStationID {
		return 1
	}
	return 0
}

func compareRelayTreeEdges(a, b RelayTreeEdge) int {
	if a.Hop < b.Hop {
		return -1
	}
	if a.Hop > b.Hop {
		return 1
	}
	if a.ParentStationID < b.ParentStationID {
		return -1
	}
	if a.ParentStationID > b.ParentStationID {
		return 1
	}
	if a.ChildStationID < b.ChildStationID {
		return -1
	}
	if a.ChildStationID > b.ChildStationID {
		return 1
	}
	return 0
}
