package routing

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestPreparePlanSnapshotPrunesOnlyStaleLinks(t *testing.T) {
	request := plannerRequest([]uint16{2, 3}, plannerLinks(
		[2]uint16{1, 2}, [2]uint16{1, 3}, [2]uint16{3, 1},
	), nil)
	request.Graph.FreshnessMS = 100
	request.Graph.Links[1].ObservedAtMS = 899
	before := clonePlanRequest(request)
	if _, err := PlanRelayTree(request); err == nil {
		t.Fatal("raw planner accepted stale graph")
	}
	prepared, err := PreparePlanSnapshot(request)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.PrunedStaleLinkCount != 1 || len(prepared.Request.Graph.Links) != 2 {
		t.Fatalf("wrong pruning: %+v", prepared)
	}
	if !reflect.DeepEqual(prepared.Request.Snapshot, request.Snapshot) || prepared.Request.Graph.Epoch != request.Graph.Epoch {
		t.Fatal("provider changed membership evidence or epoch")
	}
	if prepared.Request.Graph.Links[1].FromStationID != 3 || prepared.Request.Graph.Links[1].ToStationID != 1 {
		t.Fatal("reverse observation was lost or synthesized")
	}
	result, err := PlanRelayTree(prepared.Request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Selected.Tree.Coverage, CoverageSet{CoveredMemberIDs: []uint16{2}, UncoveredMemberIDs: []uint16{3}}) {
		t.Fatalf("missing member disappeared: %+v", result.Selected.Tree.Coverage)
	}
	prepared.Request.Constraints.RequireAllExpectedMembers = true
	if _, err := PlanRelayTree(prepared.Request); !errors.Is(err, ErrNoFeasibleRelayTree) {
		t.Fatalf("require-all error=%v", err)
	}
	prepared.Request.Snapshot.MemberIDs[0] = 50
	prepared.Request.Graph.Links[0].Quality = 0
	if !reflect.DeepEqual(request, before) {
		t.Fatal("provider aliases caller memory")
	}
	if err := ValidateNeighborGraph(request.Graph, request.Constraints.PlanningAtMS); err == nil {
		t.Fatal("raw stale graph validation was weakened")
	}
}

func TestPreparePlanSnapshotRejectsInvalidEvidenceBeforePruning(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*PlanRequest)
	}{
		{"future observation", func(r *PlanRequest) { r.Graph.Links[0].ObservedAtMS = 1_001 }},
		{"negative observation", func(r *PlanRequest) { r.Graph.Links[0].ObservedAtMS = -1 }},
		{"stale NaN link", func(r *PlanRequest) { r.Graph.Links[0].Quality = math.NaN() }},
		{"stale duplicate", func(r *PlanRequest) { r.Graph.Links = append(r.Graph.Links, r.Graph.Links[0]) }},
		{"stale membership", func(r *PlanRequest) { r.Snapshot.ObservedAtMS = 0; r.Snapshot.FreshnessMS = 100 }},
		{"zero freshness", func(r *PlanRequest) { r.Graph.FreshnessMS = 0 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := directGroupRequest(1)
			request.Graph.FreshnessMS = 100
			request.Graph.Links[0].ObservedAtMS = 0
			tc.edit(&request)
			if _, err := PreparePlanSnapshot(request); err == nil {
				t.Fatal("invalid evidence hidden by pruning")
			}
		})
	}
}

func TestPreparePlanSnapshotCanonicalizesWithoutRefreshing(t *testing.T) {
	request := plannerRequest([]uint16{3, 2}, plannerLinks([2]uint16{1, 3}, [2]uint16{1, 2}), plannerStates(11, 10))
	request.Graph.Links[0].Enabled = false
	prepared, err := PreparePlanSnapshot(request)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.PrunedStaleLinkCount != 0 || prepared.Request.Graph.Links[1].Enabled {
		t.Fatal("provider removed or enabled a fresh disabled link")
	}
	if err := ValidatePlanRequest(prepared.Request); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePlanRequest(request); err == nil {
		t.Fatal("raw canonical validation weakened")
	}
	prepared.Request.RelayStates[0].Busy = true
	if request.RelayStates[1].Busy {
		t.Fatal("provider aliases relay states")
	}
}
