package routing

import (
	"cmp"
	"slices"
)

// compareRelayTrees is the final planner tie-break (ADR-0015 objective item
// 6). It compares trees numerically, field by field, in the field order of
// the canonical JSON encoding (canonical.go): covered_member_ids, edges as
// (child, hop, parent), max_relay_depth, membership_snapshot_id,
// source_station_id, topology_epoch, uncovered_member_ids. Sequences compare
// element by element and a prefix ranks first.
//
// Comparing the canonical JSON text instead ranked station 10 before 9.
func compareRelayTrees(left, right RelayTree) int {
	if c := slices.Compare(left.Coverage.CoveredMemberIDs, right.Coverage.CoveredMemberIDs); c != 0 {
		return c
	}
	if c := slices.CompareFunc(left.Edges, right.Edges, compareCanonicalEdges); c != 0 {
		return c
	}
	return cmp.Or(
		cmp.Compare(left.MaxRelayDepth, right.MaxRelayDepth),
		cmp.Compare(left.MembershipSnapshotID, right.MembershipSnapshotID),
		cmp.Compare(left.SourceStationID, right.SourceStationID),
		cmp.Compare(left.TopologyEpoch, right.TopologyEpoch),
		slices.Compare(left.Coverage.UncoveredMemberIDs, right.Coverage.UncoveredMemberIDs),
	)
}

// compareCanonicalEdges orders one edge by the field order of
// canonicalRelayTreeEdge: child, hop, parent. It differs from
// compareRelayTreeEdges, which sorts edges inside a tree by hop, parent, child.
func compareCanonicalEdges(left, right RelayTreeEdge) int {
	return cmp.Or(
		cmp.Compare(left.ChildStationID, right.ChildStationID),
		cmp.Compare(left.Hop, right.Hop),
		cmp.Compare(left.ParentStationID, right.ParentStationID),
	)
}
