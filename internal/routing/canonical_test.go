package routing

import (
	"bytes"
	"testing"
)

func TestUT020_CanonicalRelayTreeJSONAndHash(t *testing.T) {
	tree := testTree(1, []RelayTreeEdge{
		{ParentStationID: 1, ChildStationID: 10, Hop: 0},
		{ParentStationID: 10, ChildStationID: 2, Hop: 1},
		{ParentStationID: 10, ChildStationID: 3, Hop: 1},
	}, []uint16{2, 3}, []uint16{4})

	wantJSON := []byte(`{"covered_member_ids":[2,3],"edges":[{"child_station_id":10,"hop":0,"parent_station_id":1},{"child_station_id":2,"hop":1,"parent_station_id":10},{"child_station_id":3,"hop":1,"parent_station_id":10}],"max_relay_depth":1,"membership_snapshot_id":9,"source_station_id":1,"topology_epoch":7,"uncovered_member_ids":[4]}`)
	wantHash := "f0756f0e3a1ac9adea7800fa8ef6c2cc8ab5b1ce569870f970a23126cec27cf8"

	for i := 0; i < 20; i++ {
		gotJSON, err := CanonicalRelayTreeJSON(tree)
		if err != nil {
			t.Fatalf("CanonicalRelayTreeJSON failed: %v", err)
		}
		if !bytes.Equal(gotJSON, wantJSON) {
			t.Fatalf("canonical JSON = %s, want %s", gotJSON, wantJSON)
		}
		gotHash, err := RelayTreeHash(tree)
		if err != nil {
			t.Fatalf("RelayTreeHash failed: %v", err)
		}
		if gotHash != wantHash {
			t.Fatalf("hash = %q, want %q", gotHash, wantHash)
		}
	}
}

func TestUT020_RelayTreeHashChangesWithSemantics(t *testing.T) {
	base := testTree(1, []RelayTreeEdge{
		{ParentStationID: 1, ChildStationID: 10, Hop: 0},
		{ParentStationID: 10, ChildStationID: 2, Hop: 1},
	}, []uint16{2}, []uint16{3})
	baseHash, err := RelayTreeHash(base)
	if err != nil {
		t.Fatalf("RelayTreeHash(base) failed: %v", err)
	}

	mutations := []struct {
		name string
		edit func(*RelayTree)
	}{
		{name: "snapshot", edit: func(v *RelayTree) { v.MembershipSnapshotID++ }},
		{name: "topology", edit: func(v *RelayTree) { v.TopologyEpoch++ }},
		{name: "coverage", edit: func(v *RelayTree) {
			v.Coverage = CoverageSet{UncoveredMemberIDs: []uint16{2, 3}}
			v.Edges = nil
			v.MaxRelayDepth = 0
		}},
		{name: "edge", edit: func(v *RelayTree) {
			v.Edges[1].ChildStationID = 3
			v.Coverage = CoverageSet{CoveredMemberIDs: []uint16{3}, UncoveredMemberIDs: []uint16{2}}
		}},
	}

	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			candidate := cloneTree(base)
			tc.edit(&candidate)
			hash, err := RelayTreeHash(candidate)
			if err != nil {
				t.Fatalf("RelayTreeHash failed: %v", err)
			}
			if hash == baseHash {
				t.Fatalf("hash remained %q after semantic mutation", hash)
			}
		})
	}
}

func TestUT020_CanonicalRelayTreeJSONRejectsNonCanonicalInput(t *testing.T) {
	tree := testTree(1, []RelayTreeEdge{
		{ParentStationID: 10, ChildStationID: 2, Hop: 1},
		{ParentStationID: 1, ChildStationID: 10, Hop: 0},
	}, []uint16{2}, nil)
	if _, err := CanonicalRelayTreeJSON(tree); err == nil {
		t.Fatal("CanonicalRelayTreeJSON accepted non-canonical edge order")
	}
}

func TestUT020_CanonicalRelayTreeJSONRejectsDishonestCoverage(t *testing.T) {
	tree := testTree(0, []RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2, Hop: 0}}, nil, []uint16{2})
	if _, err := CanonicalRelayTreeJSON(tree); err == nil {
		t.Fatal("CanonicalRelayTreeJSON accepted reachable member as uncovered")
	}

	tree = testTree(0, nil, []uint16{2}, nil)
	if _, err := CanonicalRelayTreeJSON(tree); err == nil {
		t.Fatal("CanonicalRelayTreeJSON accepted unreachable member as covered")
	}

	tree = testTree(0, nil, []uint16{1}, nil)
	if _, err := CanonicalRelayTreeJSON(tree); err == nil {
		t.Fatal("CanonicalRelayTreeJSON accepted source as covered member")
	}

	tree = testTree(0, nil, nil, []uint16{1})
	if _, err := CanonicalRelayTreeJSON(tree); err == nil {
		t.Fatal("CanonicalRelayTreeJSON accepted source as uncovered member")
	}
}
