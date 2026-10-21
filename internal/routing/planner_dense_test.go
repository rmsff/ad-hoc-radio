package routing

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

func TestBoundedPlannerDenseDirectGroups(t *testing.T) {
	for _, count := range []int{8, 16, 32} {
		for _, all := range []bool{false, true} {
			t.Run(fmt.Sprintf("members=%d/all=%t", count, all), func(t *testing.T) {
				request := directGroupRequest(count)
				request.Constraints.RequireAllExpectedMembers = all
				for _, from := range request.Snapshot.MemberIDs {
					request.RelayStates = append(request.RelayStates, plannerState(from))
					for to := uint16(1); to <= uint16(count+1); to++ {
						if from != to {
							request.Graph.Links = append(request.Graph.Links, plannerLinks([2]uint16{from, to})...)
						}
					}
				}
				request.SearchLimits.MaxSearchSteps = 1
				result := assertPlannerPermutations(t, request)
				if !result.Diagnostics.DirectOptimal || result.Diagnostics.PathSteps != 0 || result.Diagnostics.TreeSteps != 1 || result.EvaluatedCandidateCount != 1 {
					t.Fatalf("missing direct certificate: %+v", result.Diagnostics)
				}
			})
		}
	}
}

func TestBoundedPlannerDenseAlternatesAndEligibility(t *testing.T) {
	for _, count := range []int{8, 16, 32} {
		for _, all := range []bool{false, true} {
			t.Run(fmt.Sprintf("members=%d/all=%t", count, all), func(t *testing.T) {
				request := denseAlternateRequest(count)
				request.Constraints.RequireAllExpectedMembers = all
				result := assertPlannerPermutations(t, request)
				if result.Selected.Cost.CoveredMemberCount != count || result.Selected.Cost.TxOpportunityCount != 2 || !reflect.DeepEqual(selectedRelayOwners(1, result.Selected.Tree.Edges), []uint16{101}) {
					t.Fatalf("wrong eligible optimum: %+v", result.Selected)
				}
				if result.Diagnostics.PrunedBranches == 0 || result.Diagnostics.DirectOptimal {
					t.Fatalf("did not exercise branch pruning: %+v", result.Diagnostics)
				}
				if result.Diagnostics.TreeSteps+result.Diagnostics.PathSteps > result.Diagnostics.SearchStepLimit {
					t.Fatalf("exceeded work bound: %+v", result.Diagnostics)
				}
			})
		}
	}
	for _, all := range []bool{false, true} {
		request := denseAlternateRequest(3)
		request.Constraints.RequireAllExpectedMembers = all
		request.SearchLimits = paritySearchLimits()
		assertBoundedParity(t, request, 0)
	}
}

func denseAlternateRequest(count int) PlanRequest {
	request := directGroupRequest(count)
	request.Graph.Links = nil
	request.RelayStates = plannerStates(100, 101, 102, 103, 104, 105)
	request.RelayStates[0].BatteryFraction = 0.6
	request.RelayStates[2].RelayEnabled = false
	request.RelayStates[3].RelayCapable = false
	request.RelayStates[4].Busy = true
	request.RelayStates[5].BatteryFraction = 0.1
	request.Constraints.MinRelayBatteryFraction = 0.5
	for _, state := range request.RelayStates {
		id := state.StationID
		request.Graph.Links = append(request.Graph.Links, plannerLinks([2]uint16{1, id})...)
		for _, member := range request.Snapshot.MemberIDs {
			request.Graph.Links = append(request.Graph.Links, plannerLinks([2]uint16{id, member})...)
		}
		for _, other := range request.RelayStates {
			if id != other.StationID {
				request.Graph.Links = append(request.Graph.Links, plannerLinks([2]uint16{id, other.StationID})...)
			}
		}
	}
	return canonicalPlanRequestCopy(request)
}

func assertPlannerPermutations(t *testing.T, request PlanRequest) PlanResult {
	t.Helper()
	want, err := PlanRelayTree(request)
	if err != nil {
		t.Fatal(err)
	}
	for seed := int64(0); seed < 10; seed++ {
		shuffled := canonicalPlanRequestCopy(request)
		rng := rand.New(rand.NewSource(seed))
		rng.Shuffle(len(shuffled.Graph.Links), func(i, j int) {
			shuffled.Graph.Links[i], shuffled.Graph.Links[j] = shuffled.Graph.Links[j], shuffled.Graph.Links[i]
		})
		rng.Shuffle(len(shuffled.RelayStates), func(i, j int) {
			shuffled.RelayStates[i], shuffled.RelayStates[j] = shuffled.RelayStates[j], shuffled.RelayStates[i]
		})
		rng.Shuffle(len(shuffled.Snapshot.MemberIDs), func(i, j int) {
			shuffled.Snapshot.MemberIDs[i], shuffled.Snapshot.MemberIDs[j] = shuffled.Snapshot.MemberIDs[j], shuffled.Snapshot.MemberIDs[i]
		})
		before := shuffled
		before.Graph.Links = append([]DirectedLink(nil), shuffled.Graph.Links...)
		before.RelayStates = append([]RelayState(nil), shuffled.RelayStates...)
		before.Snapshot.MemberIDs = append([]uint16(nil), shuffled.Snapshot.MemberIDs...)
		got, err := PlanRelayTree(shuffled)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("seed=%d changed selection or diagnostics: err=%v\ngot=%+v\nwant=%+v", seed, err, got, want)
		}
		if !reflect.DeepEqual(shuffled, before) {
			t.Fatalf("seed=%d mutated caller input", seed)
		}
	}
	return want
}
