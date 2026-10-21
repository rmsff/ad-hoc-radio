package routing

import (
	"errors"
	"reflect"
	"testing"
)

func TestUT021_PlannerSelectsDirectAndRelayChains(t *testing.T) {
	tests := []struct {
		name      string
		members   []uint16
		links     []DirectedLink
		states    []RelayState
		wantDepth uint8
		wantCover []uint16
		wantTx    int
	}{
		{
			name:      "direct",
			members:   []uint16{2},
			links:     plannerLinks([2]uint16{1, 2}),
			wantDepth: 0,
			wantCover: []uint16{2},
			wantTx:    1,
		},
		{
			name:      "one relay chain",
			members:   []uint16{2},
			links:     plannerLinks([2]uint16{1, 10}, [2]uint16{10, 2}),
			states:    plannerStates(10),
			wantDepth: 1,
			wantCover: []uint16{2},
			wantTx:    2,
		},
		{
			name:      "two relay chain",
			members:   []uint16{2},
			links:     plannerLinks([2]uint16{1, 10}, [2]uint16{10, 11}, [2]uint16{11, 2}),
			states:    plannerStates(10, 11),
			wantDepth: 2,
			wantCover: []uint16{2},
			wantTx:    3,
		},
		{
			name:      "three relay chain",
			members:   []uint16{2},
			links:     plannerLinks([2]uint16{1, 10}, [2]uint16{10, 11}, [2]uint16{11, 12}, [2]uint16{12, 2}),
			states:    plannerStates(10, 11, 12),
			wantDepth: 3,
			wantCover: []uint16{2},
			wantTx:    4,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := plannerRequest(tc.members, tc.links, tc.states)
			result, err := PlanRelayTree(request)
			if err != nil {
				t.Fatalf("PlanRelayTree failed: %v", err)
			}
			if result.Selected.Cost.SelectedMaxRelayDepth != tc.wantDepth {
				t.Fatalf("selected depth = %d, want %d", result.Selected.Cost.SelectedMaxRelayDepth, tc.wantDepth)
			}
			if result.Selected.Cost.TxOpportunityCount != tc.wantTx {
				t.Fatalf("tx opportunities = %d, want %d", result.Selected.Cost.TxOpportunityCount, tc.wantTx)
			}
			if !reflect.DeepEqual(result.Selected.Tree.Coverage.CoveredMemberIDs, tc.wantCover) {
				t.Fatalf("covered = %v, want %v", result.Selected.Tree.Coverage.CoveredMemberIDs, tc.wantCover)
			}
			assertSelectedIsFirstFeasible(t, result)
			assertCycleFitsFramePeriod(t, result.Selected)
		})
	}
}

func TestUT021_PlannerHandlesBranchesAndPartialPartition(t *testing.T) {
	t.Run("narrow branch shares one relay owner", func(t *testing.T) {
		request := plannerRequest(
			[]uint16{2, 3},
			plannerLinks([2]uint16{1, 10}, [2]uint16{10, 2}, [2]uint16{10, 3}),
			plannerStates(10),
		)
		result, err := PlanRelayTree(request)
		if err != nil {
			t.Fatalf("PlanRelayTree failed: %v", err)
		}
		if got := result.Selected.Cost.TxOpportunityCount; got != 2 {
			t.Fatalf("tx opportunities = %d, want 2", got)
		}
		if got := result.Selected.Cost.CoveredMemberCount; got != 2 {
			t.Fatalf("covered count = %d, want 2", got)
		}
	})

	t.Run("wide branch serializes two relay owners", func(t *testing.T) {
		request := plannerRequest(
			[]uint16{2, 3},
			plannerLinks([2]uint16{1, 10}, [2]uint16{1, 20}, [2]uint16{10, 2}, [2]uint16{20, 3}),
			plannerStates(10, 20),
		)
		result, err := PlanRelayTree(request)
		if err != nil {
			t.Fatalf("PlanRelayTree failed: %v", err)
		}
		if got := result.Selected.Cost.TxOpportunityCount; got != 3 {
			t.Fatalf("tx opportunities = %d, want 3", got)
		}
		if got := result.Selected.Cost.SelectedMaxRelayDepth; got != 1 {
			t.Fatalf("depth = %d, want 1", got)
		}
	})

	t.Run("partial partition keeps uncovered visible in best effort", func(t *testing.T) {
		request := plannerRequest(
			[]uint16{2, 3},
			plannerLinks([2]uint16{1, 2}),
			nil,
		)
		result, err := PlanRelayTree(request)
		if err != nil {
			t.Fatalf("PlanRelayTree failed: %v", err)
		}
		if !reflect.DeepEqual(result.Selected.Tree.Coverage.CoveredMemberIDs, []uint16{2}) {
			t.Fatalf("covered = %v, want [2]", result.Selected.Tree.Coverage.CoveredMemberIDs)
		}
		if !reflect.DeepEqual(result.Selected.Tree.Coverage.UncoveredMemberIDs, []uint16{3}) {
			t.Fatalf("uncovered = %v, want [3]", result.Selected.Tree.Coverage.UncoveredMemberIDs)
		}
	})

	t.Run("all-uncovered best effort remains explicit", func(t *testing.T) {
		request := plannerRequest([]uint16{2}, nil, nil)
		result, err := PlanRelayTree(request)
		if err != nil {
			t.Fatalf("PlanRelayTree failed: %v", err)
		}
		if len(result.Selected.Tree.Coverage.CoveredMemberIDs) != 0 {
			t.Fatalf("covered = %v, want empty", result.Selected.Tree.Coverage.CoveredMemberIDs)
		}
		if !reflect.DeepEqual(result.Selected.Tree.Coverage.UncoveredMemberIDs, []uint16{2}) {
			t.Fatalf("uncovered = %v, want [2]", result.Selected.Tree.Coverage.UncoveredMemberIDs)
		}
		if got := result.Selected.Cost.TxOpportunityCount; got != 1 {
			t.Fatalf("tx opportunities = %d, want source-only 1", got)
		}
	})
}

func TestUT021_PlannerRejectsHardFeasibilityFailures(t *testing.T) {
	tests := []struct {
		name string
		edit func(*PlanRequest)
	}{
		{
			name: "asymmetric reverse-only link",
			edit: func(request *PlanRequest) {
				request.Graph = plannerGraph(plannerLinks([2]uint16{2, 1}))
			},
		},
		{
			name: "relay not capable",
			edit: func(request *PlanRequest) {
				request.RelayStates[0].RelayCapable = false
			},
		},
		{
			name: "disabled relay",
			edit: func(request *PlanRequest) {
				request.RelayStates[0].RelayEnabled = false
			},
		},
		{
			name: "busy relay",
			edit: func(request *PlanRequest) {
				request.RelayStates[0].Busy = true
			},
		},
		{
			name: "low battery relay",
			edit: func(request *PlanRequest) {
				request.Constraints.MinRelayBatteryFraction = 0.8
				request.RelayStates[0].BatteryFraction = 0.7
			},
		},
		{
			name: "missing relay state",
			edit: func(request *PlanRequest) {
				request.RelayStates = nil
			},
		},
		{
			name: "weak link quality",
			edit: func(request *PlanRequest) {
				request.Constraints.MinLinkQuality = 0.95
				request.Graph.Links[0].Quality = 0.9
			},
		},
		{
			name: "lossy link",
			edit: func(request *PlanRequest) {
				request.Constraints.MaxLinkLoss = 0.005
				request.Graph.Links[0].Loss = 0.01
			},
		},
		{
			name: "latency failure",
			edit: func(request *PlanRequest) {
				request.Constraints.MaxPlayoutLatencyMS = 5
			},
		},
		{
			name: "airtime failure",
			edit: func(request *PlanRequest) {
				request.Constraints.MaxVoiceAirtimePct = 10
			},
		},
		{
			name: "schedule cycle does not fit frame period",
			edit: func(request *PlanRequest) {
				request.CostModel.FramePeriodMS = 20
			},
		},
		{
			name: "all required cannot cover partition",
			edit: func(request *PlanRequest) {
				request.Snapshot = plannerSnapshot([]uint16{2, 3})
				request.Graph = plannerGraph(plannerLinks([2]uint16{1, 10}, [2]uint16{10, 2}))
				request.Constraints.RequireAllExpectedMembers = true
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := plannerRequest(
				[]uint16{2},
				plannerLinks([2]uint16{1, 10}, [2]uint16{10, 2}),
				plannerStates(10),
			)
			request.Constraints.RequireAllExpectedMembers = true
			tc.edit(&request)
			result, err := PlanRelayTree(request)
			if !errors.Is(err, ErrNoFeasibleRelayTree) {
				t.Fatalf("error = %v, want ErrNoFeasibleRelayTree", err)
			}
			assertNoPartialResult(t, result)
		})
	}
}

func TestUT021_PlannerUsesExactDeterministicObjective(t *testing.T) {
	t.Run("coverage beats shallow depth", func(t *testing.T) {
		request := plannerRequest(
			[]uint16{2, 3},
			plannerLinks([2]uint16{1, 2}, [2]uint16{1, 10}, [2]uint16{10, 2}, [2]uint16{10, 3}),
			plannerStates(10),
		)
		result, err := PlanRelayTree(request)
		if err != nil {
			t.Fatalf("PlanRelayTree failed: %v", err)
		}
		if got := result.Selected.Cost.CoveredMemberCount; got != 2 {
			t.Fatalf("covered count = %d, want 2", got)
		}
		if got := result.Selected.Cost.SelectedMaxRelayDepth; got != 1 {
			t.Fatalf("depth = %d, want 1", got)
		}
	})

	t.Run("risk then battery then numeric tree order break ties", func(t *testing.T) {
		request := plannerRequest(
			[]uint16{2},
			plannerLinks([2]uint16{1, 10}, [2]uint16{1, 20}, [2]uint16{10, 2}, [2]uint16{20, 2}),
			[]RelayState{
				plannerState(10),
				plannerState(20),
			},
		)
		request.Graph.Links[1].Quality = 0.99 // 1 -> 20
		request.Graph.Links[3].Quality = 0.99 // 20 -> 2
		result, err := PlanRelayTree(request)
		if err != nil {
			t.Fatalf("PlanRelayTree failed: %v", err)
		}
		assertTreeEdges(t, result.Selected.Tree.Edges, []RelayTreeEdge{
			{ParentStationID: 1, ChildStationID: 20, Hop: 0},
			{ParentStationID: 20, ChildStationID: 2, Hop: 1},
		})

		request.Graph.Links[1].Quality = 0.9
		request.Graph.Links[3].Quality = 0.9
		request.RelayStates[0].BatteryFraction = 0.99
		request.RelayStates[1].BatteryFraction = 0.8
		result, err = PlanRelayTree(request)
		if err != nil {
			t.Fatalf("PlanRelayTree failed after battery edit: %v", err)
		}
		assertTreeEdges(t, result.Selected.Tree.Edges, []RelayTreeEdge{
			{ParentStationID: 1, ChildStationID: 10, Hop: 0},
			{ParentStationID: 10, ChildStationID: 2, Hop: 1},
		})
	})

	t.Run("transmitter opportunities precede link risk", func(t *testing.T) {
		request := plannerRequest(
			[]uint16{2, 3},
			plannerLinks(
				[2]uint16{1, 10},
				[2]uint16{1, 20},
				[2]uint16{1, 30},
				[2]uint16{10, 2},
				[2]uint16{10, 3},
				[2]uint16{20, 2},
				[2]uint16{30, 3},
			),
			plannerStates(10, 20, 30),
		)
		// Make the two-owner branch lower risk. The exact objective must still
		// prefer the one-owner branch before considering aggregate link risk.
		for i := range request.Graph.Links {
			if request.Graph.Links[i].FromStationID != 10 {
				request.Graph.Links[i].Quality = 1
				request.Graph.Links[i].Loss = 0
			}
		}

		result, err := PlanRelayTree(request)
		if err != nil {
			t.Fatalf("PlanRelayTree failed: %v", err)
		}
		if got := result.Selected.Cost.TxOpportunityCount; got != 2 {
			t.Fatalf("selected tx opportunities = %d, want 2", got)
		}
		foundLowerRiskWideBranch := false
		alternatives, err := EnumerateFeasibleRelayTrees(request)
		if err != nil {
			t.Fatalf("enumerate alternatives: %v", err)
		}
		if result.Selected.TreeHash != alternatives.Selected.TreeHash {
			t.Fatal("bounded selection differs from exhaustive selection")
		}
		for _, candidate := range alternatives.Feasible {
			if candidate.Cost.TxOpportunityCount == 3 && candidate.Cost.AggregateLinkRiskUnits < result.Selected.Cost.AggregateLinkRiskUnits {
				foundLowerRiskWideBranch = true
				break
			}
		}
		if !foundLowerRiskWideBranch {
			t.Fatal("fixture did not retain a lower-risk three-opportunity competitor")
		}
	})

	t.Run("numeric tree order tie is stable", func(t *testing.T) {
		request := plannerRequest(
			[]uint16{2},
			plannerLinks([2]uint16{1, 10}, [2]uint16{1, 20}, [2]uint16{10, 2}, [2]uint16{20, 2}),
			plannerStates(10, 20),
		)
		for i := 0; i < 20; i++ {
			result, err := PlanRelayTree(request)
			if err != nil {
				t.Fatalf("run %d failed: %v", i, err)
			}
			assertTreeEdges(t, result.Selected.Tree.Edges, []RelayTreeEdge{
				{ParentStationID: 1, ChildStationID: 10, Hop: 0},
				{ParentStationID: 10, ChildStationID: 2, Hop: 1},
			})
		}
	})
}

func TestUT021_Depth3RequiresAdditionalFeasibleCoverage(t *testing.T) {
	t.Run("depth 3 adds coverage", func(t *testing.T) {
		request := plannerRequest(
			[]uint16{2, 3},
			plannerLinks(
				[2]uint16{1, 2},
				[2]uint16{1, 10},
				[2]uint16{10, 11},
				[2]uint16{11, 12},
				[2]uint16{12, 3},
			),
			plannerStates(10, 11, 12),
		)
		result, err := PlanRelayTree(request)
		if err != nil {
			t.Fatalf("PlanRelayTree failed: %v", err)
		}
		if got := result.Selected.Cost.SelectedMaxRelayDepth; got != 3 {
			t.Fatalf("depth = %d, want 3", got)
		}
		if !reflect.DeepEqual(result.Selected.Tree.Coverage.CoveredMemberIDs, []uint16{2, 3}) {
			t.Fatalf("covered = %v, want [2 3]", result.Selected.Tree.Coverage.CoveredMemberIDs)
		}
	})

	t.Run("depth 3 union-negative is excluded", func(t *testing.T) {
		request := plannerRequest(
			[]uint16{2, 3},
			plannerLinks(
				[2]uint16{1, 10},
				[2]uint16{1, 20},
				[2]uint16{10, 2},
				[2]uint16{10, 30},
				[2]uint16{20, 3},
				[2]uint16{30, 31},
				[2]uint16{31, 3},
			),
			plannerStates(10, 20, 30, 31),
		)
		result, err := EnumerateFeasibleRelayTrees(request)
		if err != nil {
			t.Fatalf("EnumerateFeasibleRelayTrees failed: %v", err)
		}
		for _, candidate := range result.Feasible {
			if candidate.Cost.SelectedMaxRelayDepth == 3 {
				t.Fatalf("depth-3 candidate survived union-negative rule: %s", candidate.CanonicalJSON)
			}
		}
		if result.Selected.Cost.SelectedMaxRelayDepth != 1 {
			t.Fatalf("selected depth = %d, want 1", result.Selected.Cost.SelectedMaxRelayDepth)
		}
	})

	t.Run("excluded depth 3 does not consume feasible cap", func(t *testing.T) {
		request := plannerRequest(
			[]uint16{100},
			plannerLinks(
				[2]uint16{1, 2},
				[2]uint16{1, 100},
				[2]uint16{2, 3},
				[2]uint16{3, 4},
				[2]uint16{4, 100},
			),
			plannerStates(2, 3, 4),
		)
		request.Constraints.RequireAllExpectedMembers = true
		request.SearchLimits.MaxFeasibleTrees = 1

		result, err := EnumerateFeasibleRelayTrees(request)
		if err != nil {
			t.Fatalf("EnumerateFeasibleRelayTrees failed: %v", err)
		}
		if len(result.Feasible) != 1 {
			t.Fatalf("feasible count = %d, want 1", len(result.Feasible))
		}
		if result.Selected.Cost.SelectedMaxRelayDepth != 0 {
			t.Fatalf("selected depth = %d, want direct depth 0", result.Selected.Cost.SelectedMaxRelayDepth)
		}
	})
}

func assertSelectedIsFirstFeasible(t *testing.T, result PlanResult) {
	t.Helper()
	if len(result.Feasible) == 0 {
		t.Fatal("result has no feasible candidates")
	}
	if result.Selected.TreeHash != result.Feasible[0].TreeHash {
		t.Fatalf("selected hash = %s, first feasible hash = %s", result.Selected.TreeHash, result.Feasible[0].TreeHash)
	}
}

func assertCycleFitsFramePeriod(t *testing.T, tree EvaluatedTree) {
	t.Helper()
	if tree.Cost.EstimatedScheduleCycleMS > plannerCostModel().FramePeriodMS {
		t.Fatalf("schedule cycle = %d exceeds frame period", tree.Cost.EstimatedScheduleCycleMS)
	}
}

func assertNoPartialResult(t *testing.T, result PlanResult) {
	t.Helper()
	if result.Selected.TreeHash != "" || len(result.Feasible) != 0 || result.EvaluatedCandidateCount != 0 {
		t.Fatalf("expected no partial result, got selected=%q feasible=%d evaluated=%d", result.Selected.TreeHash, len(result.Feasible), result.EvaluatedCandidateCount)
	}
}

func assertTreeEdges(t *testing.T, got, want []RelayTreeEdge) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("edges = %+v, want %+v", got, want)
	}
}
