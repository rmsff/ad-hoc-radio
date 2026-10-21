package routing

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestUT021_PlanRequestValidationRejectsNonCanonicalInputs(t *testing.T) {
	valid := plannerRequest(
		[]uint16{2},
		plannerLinks([2]uint16{1, 2}),
		plannerStates(10),
	)

	tests := []struct {
		name string
		edit func(*PlanRequest)
		want string
	}{
		{name: "zero source", edit: func(v *PlanRequest) { v.SourceStationID = 0 }, want: "source_station_id"},
		{name: "source in relay state", edit: func(v *PlanRequest) { v.RelayStates = []RelayState{plannerState(1)} }, want: "source"},
		{name: "unsorted relay states", edit: func(v *PlanRequest) { v.RelayStates = []RelayState{plannerState(20), plannerState(10)} }, want: "strictly ascending"},
		{name: "duplicate relay state", edit: func(v *PlanRequest) { v.RelayStates = []RelayState{plannerState(10), plannerState(10)} }, want: "strictly ascending"},
		{name: "zero relay state id", edit: func(v *PlanRequest) { v.RelayStates = []RelayState{{}} }, want: "station_id"},
		{name: "relay battery NaN", edit: func(v *PlanRequest) { v.RelayStates[0].BatteryFraction = math.NaN() }, want: "battery_fraction"},
		{name: "min relay battery Inf", edit: func(v *PlanRequest) { v.Constraints.MinRelayBatteryFraction = math.Inf(1) }, want: "min_relay_battery_fraction"},
		{name: "min link quality below zero", edit: func(v *PlanRequest) { v.Constraints.MinLinkQuality = -0.1 }, want: "min_link_quality"},
		{name: "max link loss over one", edit: func(v *PlanRequest) { v.Constraints.MaxLinkLoss = 1.1 }, want: "max_link_loss"},
		{name: "zero source opportunity", edit: func(v *PlanRequest) { v.CostModel.SourceOpportunityMS = 0 }, want: "source_opportunity_ms"},
		{name: "zero relay opportunity", edit: func(v *PlanRequest) { v.CostModel.RelayOpportunityMS = 0 }, want: "relay_opportunity_ms"},
		{name: "negative guard", edit: func(v *PlanRequest) { v.CostModel.GuardAfterMS = -1 }, want: "guard_after_ms"},
		{name: "zero frame period", edit: func(v *PlanRequest) { v.CostModel.FramePeriodMS = 0 }, want: "frame_period_ms"},
		{name: "source cycle overflow", edit: func(v *PlanRequest) { v.CostModel.SourceOpportunityMS = maxInt64 }, want: "overflows"},
		{name: "zero max paths", edit: func(v *PlanRequest) { v.SearchLimits.MaxPathsPerMember = 0 }, want: "max_paths_per_member"},
		{name: "zero max candidates", edit: func(v *PlanRequest) { v.SearchLimits.MaxTreeCandidates = 0 }, want: "max_tree_candidates"},
		{name: "zero max feasible", edit: func(v *PlanRequest) { v.SearchLimits.MaxFeasibleTrees = 0 }, want: "max_feasible_trees"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := clonePlanRequest(valid)
			tc.edit(&request)
			err := ValidatePlanRequest(request)
			if err == nil {
				t.Fatalf("expected validation error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want substring %q", err, tc.want)
			}
		})
	}
}

func TestUT021_SearchLimitExceededReturnsNoPartialResult(t *testing.T) {
	tests := []struct {
		name string
		edit func(*PlanRequest)
	}{
		{
			name: "max paths per member",
			edit: func(request *PlanRequest) {
				request.SearchLimits.MaxPathsPerMember = 1
			},
		},
		{
			name: "max tree candidates",
			edit: func(request *PlanRequest) {
				request.SearchLimits.MaxTreeCandidates = 1
			},
		},
		{
			name: "max feasible trees",
			edit: func(request *PlanRequest) {
				request.SearchLimits.MaxFeasibleTrees = 1
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := plannerRequest(
				[]uint16{2, 3},
				plannerLinks(
					[2]uint16{1, 2},
					[2]uint16{1, 3},
					[2]uint16{1, 10},
					[2]uint16{1, 20},
					[2]uint16{10, 2},
					[2]uint16{20, 3},
				),
				plannerStates(10, 20),
			)
			tc.edit(&request)
			result, err := EnumerateFeasibleRelayTrees(request)
			if !errors.Is(err, ErrPlanningSearchLimitExceeded) {
				t.Fatalf("error = %v, want ErrPlanningSearchLimitExceeded", err)
			}
			assertNoPartialResult(t, result)
		})
	}
}

func TestUT021_SearchLimitCountsConflictingParentAssignments(t *testing.T) {
	request := plannerRequest(
		[]uint16{2, 3},
		plannerLinks(
			[2]uint16{1, 10},
			[2]uint16{1, 20},
			[2]uint16{10, 2},
			[2]uint16{20, 2},
			[2]uint16{20, 3},
		),
		plannerStates(10, 20),
	)
	request.SearchLimits.MaxTreeCandidates = 1
	result, err := EnumerateFeasibleRelayTrees(request)
	if !errors.Is(err, ErrPlanningSearchLimitExceeded) {
		t.Fatalf("error = %v, want ErrPlanningSearchLimitExceeded", err)
	}
	assertNoPartialResult(t, result)
}

func TestUT021_AirtimeRoundingOverflowReturnsNoPartialResult(t *testing.T) {
	request := plannerRequest(
		[]uint16{2},
		plannerLinks([2]uint16{1, 2}),
		nil,
	)
	request.CostModel.SourceOpportunityMS = maxInt64 / 100_000
	request.CostModel.RelayOpportunityMS = 1
	request.CostModel.GuardAfterMS = 0
	request.CostModel.FramePeriodMS = maxInt64

	result, err := PlanRelayTree(request)
	if err == nil || !strings.Contains(err.Error(), "milli-percent overflows") {
		t.Fatalf("error = %v, want checked milli-percent overflow", err)
	}
	assertNoPartialResult(t, result)
}

func plannerRequest(members []uint16, links []DirectedLink, states []RelayState) PlanRequest {
	return PlanRequest{
		SourceStationID: 1,
		Snapshot:        plannerSnapshot(members),
		Graph:           plannerGraph(links),
		Constraints: PlanningConstraints{
			PlanningAtMS:              ut020PlanningAtMS,
			MaxRelayDepth:             3,
			MaxPlayoutLatencyMS:       250,
			MaxVoiceAirtimePct:        100,
			RequireAllExpectedMembers: false,
			MinRelayBatteryFraction:   0,
			MinLinkQuality:            0,
			MaxLinkLoss:               1,
		},
		CostModel:    plannerCostModel(),
		SearchLimits: plannerSearchLimits(),
		RelayStates:  append([]RelayState(nil), states...),
	}
}

func plannerSnapshot(memberIDs []uint16) ExpectedGroupSnapshot {
	return ExpectedGroupSnapshot{ID: 9, ObservedAtMS: 900, FreshnessMS: 1_000, MemberIDs: append([]uint16(nil), memberIDs...)}
}

func plannerGraph(links []DirectedLink) NeighborGraph {
	return NeighborGraph{Epoch: 7, FreshnessMS: 1_000, Links: append([]DirectedLink(nil), links...)}
}

func plannerLinks(pairs ...[2]uint16) []DirectedLink {
	links := make([]DirectedLink, 0, len(pairs))
	for _, pair := range pairs {
		links = append(links, DirectedLink{
			FromStationID: pair[0],
			ToStationID:   pair[1],
			Enabled:       true,
			Quality:       0.9,
			Loss:          0.01,
			DelayMS:       2,
			ObservedAtMS:  900,
		})
	}
	return links
}

func plannerStates(ids ...uint16) []RelayState {
	states := make([]RelayState, 0, len(ids))
	for _, id := range ids {
		states = append(states, plannerState(id))
	}
	return states
}

func plannerState(id uint16) RelayState {
	return RelayState{
		StationID:       id,
		RelayCapable:    true,
		RelayEnabled:    true,
		Busy:            false,
		BatteryFraction: 0.9,
	}
}

func plannerCostModel() PlanningCostModel {
	return PlanningCostModel{
		SourceOpportunityMS: 10,
		RelayOpportunityMS:  10,
		GuardAfterMS:        1,
		FramePeriodMS:       80,
	}
}

func plannerSearchLimits() PlanningSearchLimits {
	return PlanningSearchLimits{
		MaxPathsPerMember: 20,
		MaxTreeCandidates: 200,
		MaxFeasibleTrees:  200,
	}
}

func clonePlanRequest(request PlanRequest) PlanRequest {
	request.Snapshot.MemberIDs = append([]uint16(nil), request.Snapshot.MemberIDs...)
	request.Graph.Links = append([]DirectedLink(nil), request.Graph.Links...)
	request.RelayStates = append([]RelayState(nil), request.RelayStates...)
	return request
}
