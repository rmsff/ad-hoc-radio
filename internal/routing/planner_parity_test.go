package routing

import (
	"errors"
	"math/rand"
	"reflect"
	"testing"
)

func TestBoundedPlannerExhaustiveFourNodeGraphs(t *testing.T) {
	// All 2^12 directed simple graphs, both membership policies. The reference
	// retains the original Cartesian enumeration and depth-3 union filter.
	for mask := 0; mask < 1<<12; mask++ {
		links := make([]DirectedLink, 0, 12)
		bit := 0
		for from := uint16(1); from <= 4; from++ {
			for to := uint16(1); to <= 4; to++ {
				if from == to {
					continue
				}
				if mask&(1<<bit) != 0 {
					link := plannerLinks([2]uint16{from, to})[0]
					link.Quality = float64(6+(from+to)%5) / 10
					link.Loss = float64((from*to)%3) / 10
					link.DelayMS = int64(to % 3)
					links = append(links, link)
				}
				bit++
			}
		}
		request := plannerRequest([]uint16{2, 3, 4}, links, plannerStates(2, 3, 4))
		request.SearchLimits = paritySearchLimits()
		for _, all := range []bool{false, true} {
			request.Constraints.RequireAllExpectedMembers = all
			assertBoundedParity(t, request, int64(mask))
		}
		request.Constraints.MaxRelayDepth = uint8(mask % 4)
		request.Constraints.MaxPlayoutLatencyMS = []int64{12, 25, 80, 250}[(mask/4)%4]
		request.CostModel.FramePeriodMS = []int64{22, 33, 44, 80}[(mask/16)%4]
		request.Constraints.MaxVoiceAirtimePct = []float64{40, 70, 100}[(mask/64)%3]
		request.Constraints.MinLinkQuality = 0.7
		request.Constraints.MaxLinkLoss = 0.1
		for _, all := range []bool{false, true} {
			request.Constraints.RequireAllExpectedMembers = all
			assertBoundedParity(t, request, int64(mask))
		}
	}
}

func TestBoundedPlannerSeededDeepGraphsAndPermutations(t *testing.T) {
	for seed := int64(0); seed < 200; seed++ {
		rng := rand.New(rand.NewSource(seed))
		links := make([]DirectedLink, 0)
		for from := uint16(1); from <= 6; from++ {
			for to := uint16(1); to <= 6; to++ {
				if from == to || rng.Intn(100) >= 45 {
					continue
				}
				link := plannerLinks([2]uint16{from, to})[0]
				link.Quality = float64(rng.Intn(5)+6) / 10
				link.Loss = float64(rng.Intn(3)) / 10
				link.DelayMS = int64(rng.Intn(6))
				links = append(links, link)
			}
		}
		states := plannerStates(2, 3, 4, 5, 6)
		for i := range states {
			states[i].BatteryFraction = float64(rng.Intn(11)) / 10
			states[i].Busy = rng.Intn(10) == 0
			states[i].RelayEnabled = rng.Intn(10) != 0
			states[i].RelayCapable = rng.Intn(10) != 0
		}
		request := plannerRequest([]uint16{2, 3}, links, states)
		request.SearchLimits = paritySearchLimits()
		request.Constraints.MinRelayBatteryFraction = 0.2
		request.Constraints.MinLinkQuality = 0.7
		request.Constraints.MaxLinkLoss = 0.1
		request.CostModel.FramePeriodMS = []int64{33, 44, 80}[seed%3]
		for _, all := range []bool{false, true} {
			request.Constraints.RequireAllExpectedMembers = all
			assertBoundedParity(t, request, seed)
			permuted := clonePlanRequest(request)
			rng.Shuffle(len(permuted.Graph.Links), func(i, j int) {
				permuted.Graph.Links[i], permuted.Graph.Links[j] = permuted.Graph.Links[j], permuted.Graph.Links[i]
			})
			rng.Shuffle(len(permuted.RelayStates), func(i, j int) {
				permuted.RelayStates[i], permuted.RelayStates[j] = permuted.RelayStates[j], permuted.RelayStates[i]
			})
			permuted.Snapshot.MemberIDs[0], permuted.Snapshot.MemberIDs[1] = permuted.Snapshot.MemberIDs[1], permuted.Snapshot.MemberIDs[0]
			before := clonePlanRequest(permuted)
			want, wantErr := PlanRelayTree(request)
			got, gotErr := PlanRelayTree(permuted)
			if !errors.Is(gotErr, wantErr) || !reflect.DeepEqual(got, want) {
				t.Fatalf("seed=%d all=%t permutation differs: %v / %v", seed, all, gotErr, wantErr)
			}
			if !reflect.DeepEqual(permuted, before) {
				t.Fatalf("seed=%d mutated caller input", seed)
			}
		}
	}
}

func TestBoundedPlannerPreservesShallowCoverageUnion(t *testing.T) {
	request := plannerRequest([]uint16{2, 3}, plannerLinks(
		[2]uint16{1, 10}, [2]uint16{1, 20}, [2]uint16{1, 30},
		[2]uint16{10, 11}, [2]uint16{11, 2},
		[2]uint16{20, 21}, [2]uint16{21, 3},
		[2]uint16{30, 31}, [2]uint16{31, 32},
		[2]uint16{32, 2}, [2]uint16{32, 3},
	), plannerStates(10, 11, 20, 21, 30, 31, 32))
	request.CostModel.FramePeriodMS = 44
	request.SearchLimits = paritySearchLimits()
	for _, all := range []bool{false, true} {
		request.Constraints.RequireAllExpectedMembers = all
		assertBoundedParity(t, request, 0)
		got, err := PlanRelayTree(request)
		if err != nil {
			t.Fatal(err)
		}
		if !all && (got.Selected.Cost.CoveredMemberCount != 1 || got.Diagnostics.ShallowCoveredCount != 2) {
			t.Fatalf("pruning lost the other shallow witness: %+v", got)
		}
		if all && got.Selected.Cost.SelectedMaxRelayDepth != 3 {
			t.Fatal("require-all feasible-set semantics changed")
		}
	}
}

func assertBoundedParity(t *testing.T, request PlanRequest, seed int64) {
	t.Helper()
	want, wantErr := EnumerateFeasibleRelayTrees(request)
	got, gotErr := PlanRelayTree(request)
	if wantErr != nil {
		if !errors.Is(wantErr, ErrNoFeasibleRelayTree) || !errors.Is(gotErr, ErrNoFeasibleRelayTree) {
			t.Fatalf("seed=%d all=%t errors reference=%v bounded=%v", seed, request.Constraints.RequireAllExpectedMembers, wantErr, gotErr)
		}
		return
	}
	if gotErr != nil || !reflect.DeepEqual(got.Selected, want.Selected) {
		t.Fatalf("seed=%d all=%t bounded error=%v\nwant=%+v\ngot=%+v", seed, request.Constraints.RequireAllExpectedMembers, gotErr, want.Selected, got.Selected)
	}
	if !want.CompleteEnumeration || got.CompleteEnumeration {
		t.Fatal("incorrect completeness label")
	}
}

func paritySearchLimits() PlanningSearchLimits {
	return PlanningSearchLimits{MaxPathsPerMember: 100, MaxTreeCandidates: 20_000, MaxFeasibleTrees: 20_000, MaxSearchSteps: 1_000_000}
}
