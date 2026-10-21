package routing

import (
	"errors"
	"reflect"
	"testing"
)

func TestBoundedPlannerExplicitLimitsNeverReturnPartialWinner(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*PlanningSearchLimits)
	}{
		{"max_search_steps", func(l *PlanningSearchLimits) { l.MaxSearchSteps = 1 }},
		{"max_paths_per_member", func(l *PlanningSearchLimits) { l.MaxPathsPerMember = 1 }},
		{"max_tree_candidates", func(l *PlanningSearchLimits) { l.MaxTreeCandidates = 1 }},
		{"max_feasible_trees", func(l *PlanningSearchLimits) { l.MaxFeasibleTrees = 1 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := plannerRequest([]uint16{2}, plannerLinks([2]uint16{1, 10}, [2]uint16{1, 11}, [2]uint16{10, 2}, [2]uint16{11, 2}), plannerStates(10, 11))
			request.RelayStates[1].BatteryFraction = 1
			request.SearchLimits.MaxSearchSteps = 1000
			test.edit(&request.SearchLimits)
			var previous *PlanningLimitError
			for attempt := 0; attempt < 2; attempt++ {
				result, err := PlanRelayTree(request)
				var limit *PlanningLimitError
				if !errors.Is(err, ErrPlanningSearchLimitExceeded) || !errors.As(err, &limit) || limit.Limit != test.name || limit.Bound != 1 {
					t.Fatalf("unexpected bounded failure: %v", err)
				}
				assertNoPartialResult(t, result)
				if limit.Diagnostics.PathSteps+limit.Diagnostics.TreeSteps > limit.Diagnostics.SearchStepLimit {
					t.Fatalf("exceeded work bound: %+v", limit)
				}
				if previous != nil && !reflect.DeepEqual(previous, limit) {
					t.Fatalf("non-deterministic diagnostics: %+v != %+v", previous, limit)
				}
				previous = limit
			}
		})
	}
}

func TestBoundedPlannerBoundsDeadEndPathExpansion(t *testing.T) {
	request := denseAlternateRequest(3)
	request.Snapshot.MemberIDs = []uint16{99}
	request.SearchLimits.MaxSearchSteps = 3
	result, err := PlanRelayTree(request)
	var limit *PlanningLimitError
	if !errors.As(err, &limit) || limit.Limit != "max_search_steps" || limit.Diagnostics.PathSteps != 3 || limit.Diagnostics.PathsEnumerated != 0 {
		t.Fatalf("unbounded dead-end expansion: %v", err)
	}
	assertNoPartialResult(t, result)
}

func TestBoundedPlannerDerivedBudgetIsChecked(t *testing.T) {
	request := directGroupRequest(8)
	result, err := PlanRelayTree(request)
	if err != nil {
		t.Fatal(err)
	}
	want := request.SearchLimits.MaxTreeCandidates * 9 * 5
	if result.Diagnostics.SearchStepLimit != want {
		t.Fatalf("derived budget=%d, want %d", result.Diagnostics.SearchStepLimit, want)
	}
	request.SearchLimits.MaxTreeCandidates = int(^uint(0) >> 1)
	if _, err := PlanRelayTree(request); err == nil {
		t.Fatal("accepted derived budget overflow")
	}
	request.SearchLimits.MaxSearchSteps = 1
	if _, err := PlanRelayTree(request); err != nil {
		t.Fatalf("explicit budget should avoid derivation overflow: %v", err)
	}
	request.SearchLimits.MaxSearchSteps = -1
	if err := ValidatePlanningSearchLimits(request.SearchLimits); err == nil {
		t.Fatal("accepted negative search budget")
	}
}
