package routing

import (
	"reflect"
	"slices"
	"testing"
)

func orderBaseTree() RelayTree {
	return RelayTree{
		SourceStationID:      5,
		MembershipSnapshotID: 5,
		TopologyEpoch:        5,
		MaxRelayDepth:        1,
		Edges: []RelayTreeEdge{
			{ParentStationID: 5, ChildStationID: 9, Hop: 0},
			{ParentStationID: 9, ChildStationID: 2, Hop: 1},
		},
		Coverage: CoverageSet{CoveredMemberIDs: []uint16{2}, UncoveredMemberIDs: []uint16{4}},
	}
}

func assertTreeOrder(t *testing.T, smaller, larger RelayTree) {
	t.Helper()
	if got := compareRelayTrees(smaller, larger); got >= 0 {
		t.Fatalf("compare(smaller, larger) = %d, want < 0\nsmaller=%+v\nlarger=%+v", got, smaller, larger)
	}
	if got := compareRelayTrees(larger, smaller); got <= 0 {
		t.Fatalf("compare(larger, smaller) = %d, want > 0", got)
	}
	if compareRelayTrees(smaller, smaller) != 0 || compareRelayTrees(larger, larger) != 0 {
		t.Fatal("a tree does not compare equal to itself")
	}
}

// Each field on its own: numeric order, a prefix first. Several rows rank
// opposite to the canonical JSON text ("[10]" < "[2]", "[2,3]" < "[2]").
func TestCompareRelayTreesSingleFieldOrder(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*RelayTree)
	}{
		{"covered member id 2 before 10", func(tr *RelayTree) { tr.Coverage.CoveredMemberIDs = []uint16{10} }},
		{"covered prefix first", func(tr *RelayTree) { tr.Coverage.CoveredMemberIDs = []uint16{2, 3} }},
		{"edge child 9 before 10", func(tr *RelayTree) { tr.Edges[0].ChildStationID = 10 }},
		{"edge hop", func(tr *RelayTree) { tr.Edges[0].Hop = 1 }},
		{"edge parent", func(tr *RelayTree) { tr.Edges[1].ParentStationID = 10 }},
		{"edges prefix first", func(tr *RelayTree) {
			tr.Edges = append(tr.Edges, RelayTreeEdge{ParentStationID: 2, ChildStationID: 3, Hop: 2})
		}},
		{"max relay depth", func(tr *RelayTree) { tr.MaxRelayDepth = 2 }},
		{"membership snapshot id", func(tr *RelayTree) { tr.MembershipSnapshotID = 10 }},
		{"source station id", func(tr *RelayTree) { tr.SourceStationID = 10 }},
		{"topology epoch", func(tr *RelayTree) { tr.TopologyEpoch = 10 }},
		{"uncovered member id", func(tr *RelayTree) { tr.Coverage.UncoveredMemberIDs = []uint16{10} }},
		{"uncovered prefix first", func(tr *RelayTree) { tr.Coverage.UncoveredMemberIDs = []uint16{4, 6} }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			larger := orderBaseTree()
			tc.mutate(&larger)
			assertTreeOrder(t, orderBaseTree(), larger)
		})
	}
}

// Each row makes two neighbouring keys disagree while all other keys are
// equal. Swapping any two neighbours, and so any permutation of the key
// order, flips at least one row.
func TestCompareRelayTreesFieldPrecedence(t *testing.T) {
	tests := []struct {
		name            string
		smaller, larger func(*RelayTree)
	}{
		{"covered before edges",
			func(tr *RelayTree) { tr.Coverage.CoveredMemberIDs = []uint16{2}; tr.Edges[0].ChildStationID = 10 },
			func(tr *RelayTree) { tr.Coverage.CoveredMemberIDs = []uint16{3}; tr.Edges[0].ChildStationID = 9 }},
		{"edges before max relay depth",
			func(tr *RelayTree) { tr.Edges[0].ChildStationID = 9; tr.MaxRelayDepth = 2 },
			func(tr *RelayTree) { tr.Edges[0].ChildStationID = 10; tr.MaxRelayDepth = 1 }},
		{"max relay depth before snapshot",
			func(tr *RelayTree) { tr.MaxRelayDepth = 1; tr.MembershipSnapshotID = 10 },
			func(tr *RelayTree) { tr.MaxRelayDepth = 2; tr.MembershipSnapshotID = 5 }},
		{"snapshot before source",
			func(tr *RelayTree) { tr.MembershipSnapshotID = 5; tr.SourceStationID = 10 },
			func(tr *RelayTree) { tr.MembershipSnapshotID = 6; tr.SourceStationID = 5 }},
		{"source before epoch",
			func(tr *RelayTree) { tr.SourceStationID = 5; tr.TopologyEpoch = 10 },
			func(tr *RelayTree) { tr.SourceStationID = 6; tr.TopologyEpoch = 5 }},
		{"epoch before uncovered",
			func(tr *RelayTree) { tr.TopologyEpoch = 5; tr.Coverage.UncoveredMemberIDs = []uint16{10} },
			func(tr *RelayTree) { tr.TopologyEpoch = 6; tr.Coverage.UncoveredMemberIDs = []uint16{4} }},
		{"edge child before hop",
			func(tr *RelayTree) { tr.Edges[0].ChildStationID = 9; tr.Edges[0].Hop = 1 },
			func(tr *RelayTree) { tr.Edges[0].ChildStationID = 10; tr.Edges[0].Hop = 0 }},
		{"edge hop before parent",
			func(tr *RelayTree) { tr.Edges[0].Hop = 0; tr.Edges[0].ParentStationID = 10 },
			func(tr *RelayTree) { tr.Edges[0].Hop = 1; tr.Edges[0].ParentStationID = 5 }},
		{"earlier edge before later edge",
			func(tr *RelayTree) { tr.Edges[0].ChildStationID = 9; tr.Edges[1].ChildStationID = 20 },
			func(tr *RelayTree) { tr.Edges[0].ChildStationID = 10; tr.Edges[1].ChildStationID = 2 }},
		{"earlier covered id before later id",
			func(tr *RelayTree) { tr.Coverage.CoveredMemberIDs = []uint16{2, 9} },
			func(tr *RelayTree) { tr.Coverage.CoveredMemberIDs = []uint16{3, 1} }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			smaller, larger := orderBaseTree(), orderBaseTree()
			tc.smaller(&smaller)
			tc.larger(&larger)
			assertTreeOrder(t, smaller, larger)
		})
	}
}

// compareRelayTrees lists every canonical field by hand; a new field must be
// added there too, or only the TreeHash drift guard would order it.
func TestCompareRelayTreesCoversCanonicalFields(t *testing.T) {
	if got := reflect.TypeOf(canonicalRelayTree{}).NumField(); got != 7 {
		t.Fatalf("canonicalRelayTree has %d fields, compareRelayTrees covers 7: update compareRelayTrees", got)
	}
	if got := reflect.TypeOf(canonicalRelayTreeEdge{}).NumField(); got != 3 {
		t.Fatalf("canonicalRelayTreeEdge has %d fields, compareCanonicalEdges covers 3: update compareRelayTrees", got)
	}
}

// Canonical JSON writes nil and empty member lists alike, so the order must
// not separate them either.
func TestCompareRelayTreesNilEqualsEmpty(t *testing.T) {
	withNil, withEmpty := orderBaseTree(), orderBaseTree()
	withNil.Coverage.UncoveredMemberIDs = nil
	withEmpty.Coverage.UncoveredMemberIDs = []uint16{}
	if got := compareRelayTrees(withNil, withEmpty); got != 0 {
		t.Fatalf("compare(nil, empty) = %d, want 0", got)
	}
}

func TestSortEvaluatedTreesUsesTreeHashLast(t *testing.T) {
	smallTree := orderBaseTree()
	largeTree := orderBaseTree()
	largeTree.TopologyEpoch = 6
	for _, input := range [][]string{{"b", "a", "z"}, {"z", "a", "b"}, {"a", "z", "b"}} {
		trees := make([]EvaluatedTree, 0, len(input))
		for _, hash := range input {
			tree := largeTree
			if hash == "z" {
				tree = smallTree
			}
			trees = append(trees, EvaluatedTree{Tree: tree, TreeHash: hash})
		}
		sortEvaluatedTrees(trees)
		var got []string
		for _, tree := range trees {
			got = append(got, tree.TreeHash)
		}
		if want := []string{"z", "a", "b"}; !slices.Equal(got, want) {
			t.Fatalf("input %v sorted to %v, want %v", input, got, want)
		}
	}
}
