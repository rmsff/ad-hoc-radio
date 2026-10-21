package routing

import (
	"math"
	"strings"
	"testing"
)

const ut020PlanningAtMS int64 = 1_000

func TestUT020_ValidRelayTreeShapes(t *testing.T) {
	tests := []struct {
		name     string
		members  []uint16
		links    []DirectedLink
		tree     RelayTree
		maxDepth uint8
	}{
		{
			name:    "direct",
			members: []uint16{2},
			links:   testLinks([2]uint16{1, 2}),
			tree:    testTree(0, []RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2, Hop: 0}}, []uint16{2}, nil),
		},
		{
			name:    "one relay",
			members: []uint16{2},
			links:   testLinks([2]uint16{1, 10}, [2]uint16{10, 2}),
			tree: testTree(1, []RelayTreeEdge{
				{ParentStationID: 1, ChildStationID: 10, Hop: 0},
				{ParentStationID: 10, ChildStationID: 2, Hop: 1},
			}, []uint16{2}, nil),
			maxDepth: 1,
		},
		{
			name:    "two relays",
			members: []uint16{2},
			links:   testLinks([2]uint16{1, 10}, [2]uint16{10, 11}, [2]uint16{11, 2}),
			tree: testTree(2, []RelayTreeEdge{
				{ParentStationID: 1, ChildStationID: 10, Hop: 0},
				{ParentStationID: 10, ChildStationID: 11, Hop: 1},
				{ParentStationID: 11, ChildStationID: 2, Hop: 2},
			}, []uint16{2}, nil),
			maxDepth: 2,
		},
		{
			name:    "three relays",
			members: []uint16{2},
			links:   testLinks([2]uint16{1, 10}, [2]uint16{10, 11}, [2]uint16{11, 12}, [2]uint16{12, 2}),
			tree: testTree(3, []RelayTreeEdge{
				{ParentStationID: 1, ChildStationID: 10, Hop: 0},
				{ParentStationID: 10, ChildStationID: 11, Hop: 1},
				{ParentStationID: 11, ChildStationID: 12, Hop: 2},
				{ParentStationID: 12, ChildStationID: 2, Hop: 3},
			}, []uint16{2}, nil),
			maxDepth: 3,
		},
		{
			name:    "branch with transit relay outside expected group",
			members: []uint16{2, 3, 4},
			links:   testLinks([2]uint16{1, 10}, [2]uint16{10, 2}, [2]uint16{10, 3}),
			tree: testTree(1, []RelayTreeEdge{
				{ParentStationID: 1, ChildStationID: 10, Hop: 0},
				{ParentStationID: 10, ChildStationID: 2, Hop: 1},
				{ParentStationID: 10, ChildStationID: 3, Hop: 1},
			}, []uint16{2, 3}, []uint16{4}),
			maxDepth: 1,
		},
		{
			name:    "wide branch with multiple relay owners at the same hop",
			members: []uint16{2, 3},
			links:   testLinks([2]uint16{1, 10}, [2]uint16{1, 20}, [2]uint16{10, 2}, [2]uint16{20, 3}),
			tree: testTree(1, []RelayTreeEdge{
				{ParentStationID: 1, ChildStationID: 10, Hop: 0},
				{ParentStationID: 1, ChildStationID: 20, Hop: 0},
				{ParentStationID: 10, ChildStationID: 2, Hop: 1},
				{ParentStationID: 20, ChildStationID: 3, Hop: 1},
			}, []uint16{2, 3}, nil),
			maxDepth: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			constraints := testConstraints()
			constraints.MaxRelayDepth = tc.maxDepth
			if err := ValidateRelayTree(testSnapshot(tc.members), testGraph(tc.links), constraints, tc.tree); err != nil {
				t.Fatalf("ValidateRelayTree failed: %v", err)
			}
		})
	}
}

func TestUT020_ExpectedGroupSnapshotValidation(t *testing.T) {
	valid := testSnapshot([]uint16{2, 3})
	if err := ValidateExpectedGroupSnapshot(valid, 1, ut020PlanningAtMS); err != nil {
		t.Fatalf("valid snapshot rejected: %v", err)
	}

	tests := []struct {
		name string
		edit func(*ExpectedGroupSnapshot)
		want string
	}{
		{name: "zero id", edit: func(v *ExpectedGroupSnapshot) { v.ID = 0 }, want: "id"},
		{name: "negative observation", edit: func(v *ExpectedGroupSnapshot) { v.ObservedAtMS = -1 }, want: "observed_at_ms"},
		{name: "zero freshness", edit: func(v *ExpectedGroupSnapshot) { v.FreshnessMS = 0 }, want: "freshness_ms"},
		{name: "future observation", edit: func(v *ExpectedGroupSnapshot) { v.ObservedAtMS = ut020PlanningAtMS + 1 }, want: "future"},
		{name: "stale", edit: func(v *ExpectedGroupSnapshot) { v.ObservedAtMS = 0; v.FreshnessMS = 999 }, want: "stale"},
		{name: "zero member", edit: func(v *ExpectedGroupSnapshot) { v.MemberIDs = []uint16{0, 2} }, want: "member_ids"},
		{name: "source member", edit: func(v *ExpectedGroupSnapshot) { v.MemberIDs = []uint16{1, 2} }, want: "source"},
		{name: "duplicate member", edit: func(v *ExpectedGroupSnapshot) { v.MemberIDs = []uint16{2, 2} }, want: "strictly ascending"},
		{name: "unsorted member", edit: func(v *ExpectedGroupSnapshot) { v.MemberIDs = []uint16{3, 2} }, want: "strictly ascending"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			candidate := cloneSnapshot(valid)
			tc.edit(&candidate)
			requireRoutingError(t, ValidateExpectedGroupSnapshot(candidate, 1, ut020PlanningAtMS), tc.want)
		})
	}
}

func TestUT020_NeighborGraphValidation(t *testing.T) {
	valid := testGraph(testLinks([2]uint16{1, 2}, [2]uint16{2, 3}))
	if err := ValidateNeighborGraph(valid, ut020PlanningAtMS); err != nil {
		t.Fatalf("valid graph rejected: %v", err)
	}
	if err := ValidateNeighborGraph(testGraph(testLinks([2]uint16{1, 2})), ut020PlanningAtMS); err != nil {
		t.Fatalf("asymmetric graph rejected: %v", err)
	}

	tests := []struct {
		name string
		edit func(*NeighborGraph)
		want string
	}{
		{name: "zero epoch", edit: func(v *NeighborGraph) { v.Epoch = 0 }, want: "epoch"},
		{name: "zero freshness", edit: func(v *NeighborGraph) { v.FreshnessMS = 0 }, want: "freshness_ms"},
		{name: "zero from", edit: func(v *NeighborGraph) { v.Links[0].FromStationID = 0 }, want: "from_station_id"},
		{name: "zero to", edit: func(v *NeighborGraph) { v.Links[0].ToStationID = 0 }, want: "to_station_id"},
		{name: "self link", edit: func(v *NeighborGraph) { v.Links[0].ToStationID = v.Links[0].FromStationID }, want: "self"},
		{name: "duplicate link", edit: func(v *NeighborGraph) { v.Links[1] = v.Links[0] }, want: "strictly ascending"},
		{name: "unsorted links", edit: func(v *NeighborGraph) { v.Links[0], v.Links[1] = v.Links[1], v.Links[0] }, want: "strictly ascending"},
		{name: "quality NaN", edit: func(v *NeighborGraph) { v.Links[0].Quality = math.NaN() }, want: "quality"},
		{name: "quality Inf", edit: func(v *NeighborGraph) { v.Links[0].Quality = math.Inf(1) }, want: "quality"},
		{name: "quality over one", edit: func(v *NeighborGraph) { v.Links[0].Quality = 1.1 }, want: "quality"},
		{name: "loss under zero", edit: func(v *NeighborGraph) { v.Links[0].Loss = -0.1 }, want: "loss"},
		{name: "negative delay", edit: func(v *NeighborGraph) { v.Links[0].DelayMS = -1 }, want: "delay_ms"},
		{name: "negative observation", edit: func(v *NeighborGraph) { v.Links[0].ObservedAtMS = -1 }, want: "observed_at_ms"},
		{name: "future observation", edit: func(v *NeighborGraph) { v.Links[0].ObservedAtMS = ut020PlanningAtMS + 1 }, want: "future"},
		{name: "stale link", edit: func(v *NeighborGraph) { v.Links[0].ObservedAtMS = 0; v.FreshnessMS = 999 }, want: "stale"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			candidate := cloneGraph(valid)
			tc.edit(&candidate)
			requireRoutingError(t, ValidateNeighborGraph(candidate, ut020PlanningAtMS), tc.want)
		})
	}
}

func TestUT020_PlanningConstraintsValidation(t *testing.T) {
	valid := testConstraints()
	if err := ValidatePlanningConstraints(valid); err != nil {
		t.Fatalf("valid constraints rejected: %v", err)
	}

	tests := []struct {
		name string
		edit func(*PlanningConstraints)
		want string
	}{
		{name: "negative planning time", edit: func(v *PlanningConstraints) { v.PlanningAtMS = -1 }, want: "planning_at_ms"},
		{name: "depth over three", edit: func(v *PlanningConstraints) { v.MaxRelayDepth = 4 }, want: "max_relay_depth"},
		{name: "zero latency", edit: func(v *PlanningConstraints) { v.MaxPlayoutLatencyMS = 0 }, want: "max_playout_latency_ms"},
		{name: "airtime zero", edit: func(v *PlanningConstraints) { v.MaxVoiceAirtimePct = 0 }, want: "max_voice_airtime_pct"},
		{name: "airtime over 100", edit: func(v *PlanningConstraints) { v.MaxVoiceAirtimePct = 100.1 }, want: "max_voice_airtime_pct"},
		{name: "airtime NaN", edit: func(v *PlanningConstraints) { v.MaxVoiceAirtimePct = math.NaN() }, want: "max_voice_airtime_pct"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			candidate := valid
			tc.edit(&candidate)
			requireRoutingError(t, ValidatePlanningConstraints(candidate), tc.want)
		})
	}
}

func TestUT020_FreshnessBoundaryIsInclusive(t *testing.T) {
	snapshot := testSnapshot([]uint16{2})
	snapshot.ObservedAtMS = 0
	snapshot.FreshnessMS = 1_000
	if err := ValidateExpectedGroupSnapshot(snapshot, 1, 1_000); err != nil {
		t.Fatalf("snapshot at inclusive freshness boundary rejected: %v", err)
	}
	requireRoutingError(t, ValidateExpectedGroupSnapshot(snapshot, 1, 1_001), "stale")

	graph := testGraph(testLinks([2]uint16{1, 2}))
	graph.Links[0].ObservedAtMS = 0
	graph.FreshnessMS = 1_000
	if err := ValidateNeighborGraph(graph, 1_000); err != nil {
		t.Fatalf("link at inclusive freshness boundary rejected: %v", err)
	}
	requireRoutingError(t, ValidateNeighborGraph(graph, 1_001), "stale")
}

func TestUT020_RequireAllExpectedMembersPolicy(t *testing.T) {
	snapshot := testSnapshot([]uint16{2, 3})
	graph := testGraph(testLinks([2]uint16{1, 2}))
	partial := testTree(0, []RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2, Hop: 0}}, []uint16{2}, []uint16{3})

	bestEffort := testConstraints()
	if err := ValidateRelayTree(snapshot, graph, bestEffort, partial); err != nil {
		t.Fatalf("best-effort partial coverage rejected: %v", err)
	}

	requireAll := bestEffort
	requireAll.RequireAllExpectedMembers = true
	requireRoutingError(t, ValidateRelayTree(snapshot, graph, requireAll, partial), "require_all_expected_members")
}

func TestUT020_EmptyTreeHasSelectedDepthZero(t *testing.T) {
	snapshot := testSnapshot([]uint16{2})
	tree := testTree(0, nil, nil, []uint16{2})
	if err := ValidateRelayTree(snapshot, testGraph(nil), testConstraints(), tree); err != nil {
		t.Fatalf("valid empty best-effort tree rejected: %v", err)
	}

	tree.MaxRelayDepth = 1
	requireRoutingError(t, ValidateRelayTree(snapshot, testGraph(nil), testConstraints(), tree), "selected edge depth=0")
}

func TestUT020_RelayTreeRejectsMalformedStructure(t *testing.T) {
	snapshot := testSnapshot([]uint16{2})
	graph := testGraph(testLinks([2]uint16{1, 10}, [2]uint16{10, 2}, [2]uint16{10, 11}, [2]uint16{11, 10}))
	valid := testTree(1, []RelayTreeEdge{
		{ParentStationID: 1, ChildStationID: 10, Hop: 0},
		{ParentStationID: 10, ChildStationID: 2, Hop: 1},
	}, []uint16{2}, nil)

	tests := []struct {
		name string
		edit func(*RelayTree)
		want string
	}{
		{name: "zero source", edit: func(v *RelayTree) { v.SourceStationID = 0 }, want: "source_station_id"},
		{name: "snapshot mismatch", edit: func(v *RelayTree) { v.MembershipSnapshotID++ }, want: "membership_snapshot_id"},
		{name: "topology mismatch", edit: func(v *RelayTree) { v.TopologyEpoch++ }, want: "topology_epoch"},
		{name: "source as child", edit: func(v *RelayTree) { v.Edges[0].ChildStationID = 1 }, want: "source"},
		{name: "duplicate edge", edit: func(v *RelayTree) { v.Edges = append(v.Edges, v.Edges[1]) }, want: "strictly ascending"},
		{name: "multiple parents", edit: func(v *RelayTree) {
			v.Edges = []RelayTreeEdge{
				{ParentStationID: 1, ChildStationID: 10, Hop: 0},
				{ParentStationID: 10, ChildStationID: 2, Hop: 1},
				{ParentStationID: 11, ChildStationID: 2, Hop: 1},
			}
		}, want: "multiple parents"},
		{name: "disconnected", edit: func(v *RelayTree) { v.Edges = []RelayTreeEdge{{ParentStationID: 10, ChildStationID: 2, Hop: 1}} }, want: "reachable"},
		{name: "cycle", edit: func(v *RelayTree) {
			v.Edges = []RelayTreeEdge{
				{ParentStationID: 1, ChildStationID: 10, Hop: 0},
				{ParentStationID: 10, ChildStationID: 11, Hop: 1},
				{ParentStationID: 11, ChildStationID: 10, Hop: 2},
			}
			v.Coverage = CoverageSet{UncoveredMemberIDs: []uint16{2}}
			v.MaxRelayDepth = 2
		}, want: "multiple parents"},
		{name: "noncanonical order", edit: func(v *RelayTree) { v.Edges[0], v.Edges[1] = v.Edges[1], v.Edges[0] }, want: "strictly ascending"},
		{name: "hop mismatch", edit: func(v *RelayTree) { v.Edges[1].Hop = 0 }, want: "hop"},
		{name: "max depth mismatch", edit: func(v *RelayTree) { v.MaxRelayDepth = 2 }, want: "max_relay_depth"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			candidate := cloneTree(valid)
			tc.edit(&candidate)
			requireRoutingError(t, ValidateRelayTree(snapshot, graph, testConstraints(), candidate), tc.want)
		})
	}
}

func TestUT020_RelayTreeRejectsDepthOverBounds(t *testing.T) {
	snapshot := testSnapshot([]uint16{2})
	links := testLinks([2]uint16{1, 10}, [2]uint16{10, 11}, [2]uint16{11, 12}, [2]uint16{12, 13}, [2]uint16{13, 2})
	tree := testTree(4, []RelayTreeEdge{
		{ParentStationID: 1, ChildStationID: 10, Hop: 0},
		{ParentStationID: 10, ChildStationID: 11, Hop: 1},
		{ParentStationID: 11, ChildStationID: 12, Hop: 2},
		{ParentStationID: 12, ChildStationID: 13, Hop: 3},
		{ParentStationID: 13, ChildStationID: 2, Hop: 4},
	}, []uint16{2}, nil)
	requireRoutingError(t, ValidateRelayTree(snapshot, testGraph(links), testConstraints(), tree), "max_relay_depth")

	validThreeEdges := []RelayTreeEdge{
		{ParentStationID: 1, ChildStationID: 10, Hop: 0},
		{ParentStationID: 10, ChildStationID: 11, Hop: 1},
		{ParentStationID: 11, ChildStationID: 12, Hop: 2},
		{ParentStationID: 12, ChildStationID: 2, Hop: 3},
	}
	validThree := testTree(3, validThreeEdges, []uint16{2}, nil)
	constraints := testConstraints()
	constraints.MaxRelayDepth = 2
	validThreeLinks := testLinks([2]uint16{1, 10}, [2]uint16{10, 11}, [2]uint16{11, 12}, [2]uint16{12, 2})
	requireRoutingError(t, ValidateRelayTree(snapshot, testGraph(validThreeLinks), constraints, validThree), "configured")
}

func TestUT020_RelayTreeRequiresExactFreshDirectedEvidence(t *testing.T) {
	snapshot := testSnapshot([]uint16{2})
	tree := testTree(0, []RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2, Hop: 0}}, []uint16{2}, nil)

	tests := []struct {
		name  string
		graph NeighborGraph
		want  string
	}{
		{name: "missing", graph: testGraph(nil), want: "missing directed link"},
		{name: "reverse only", graph: testGraph(testLinks([2]uint16{2, 1})), want: "missing directed link"},
		{name: "disabled", graph: func() NeighborGraph { g := testGraph(testLinks([2]uint16{1, 2})); g.Links[0].Enabled = false; return g }(), want: "disabled"},
		{name: "stale", graph: func() NeighborGraph {
			g := testGraph(testLinks([2]uint16{1, 2}))
			g.Links[0].ObservedAtMS = 0
			g.FreshnessMS = 999
			return g
		}(), want: "stale"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			requireRoutingError(t, ValidateRelayTree(snapshot, tc.graph, testConstraints(), tree), tc.want)
		})
	}
}

func TestUT020_RelayTreeCoveragePartitionValidation(t *testing.T) {
	snapshot := testSnapshot([]uint16{2, 3})
	graph := testGraph(testLinks([2]uint16{1, 2}))
	valid := testTree(0, []RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2, Hop: 0}}, []uint16{2}, []uint16{3})

	tests := []struct {
		name string
		edit func(*RelayTree)
		want string
	}{
		{name: "covered duplicate", edit: func(v *RelayTree) { v.Coverage.CoveredMemberIDs = []uint16{2, 2} }, want: "strictly ascending"},
		{name: "uncovered unsorted", edit: func(v *RelayTree) { v.Coverage.UncoveredMemberIDs = []uint16{4, 3} }, want: "strictly ascending"},
		{name: "overlap", edit: func(v *RelayTree) { v.Coverage.CoveredMemberIDs = []uint16{2, 3} }, want: "overlap"},
		{name: "missing expected member", edit: func(v *RelayTree) { v.Coverage.UncoveredMemberIDs = nil }, want: "partition"},
		{name: "extra member", edit: func(v *RelayTree) { v.Coverage.UncoveredMemberIDs = []uint16{3, 4} }, want: "not expected"},
		{name: "reachable marked uncovered", edit: func(v *RelayTree) { v.Coverage = CoverageSet{UncoveredMemberIDs: []uint16{2, 3}} }, want: "reachable"},
		{name: "unreachable marked covered", edit: func(v *RelayTree) { v.Coverage = CoverageSet{CoveredMemberIDs: []uint16{2, 3}} }, want: "not reachable"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			candidate := cloneTree(valid)
			tc.edit(&candidate)
			requireRoutingError(t, ValidateRelayTree(snapshot, graph, testConstraints(), candidate), tc.want)
		})
	}
}

func TestUT020_RelayTreeRejectsDanglingNonExpectedLeaf(t *testing.T) {
	snapshot := testSnapshot([]uint16{2})
	graph := testGraph(testLinks([2]uint16{1, 2}, [2]uint16{1, 99}))
	tree := testTree(0, []RelayTreeEdge{
		{ParentStationID: 1, ChildStationID: 2, Hop: 0},
		{ParentStationID: 1, ChildStationID: 99, Hop: 0},
	}, []uint16{2}, nil)
	requireRoutingError(t, ValidateRelayTree(snapshot, graph, testConstraints(), tree), "non-expected leaf")
}

func testSnapshot(memberIDs []uint16) ExpectedGroupSnapshot {
	return ExpectedGroupSnapshot{ID: 9, ObservedAtMS: 900, FreshnessMS: 1_000, MemberIDs: append([]uint16(nil), memberIDs...)}
}

func testGraph(links []DirectedLink) NeighborGraph {
	return NeighborGraph{Epoch: 7, FreshnessMS: 1_000, Links: append([]DirectedLink(nil), links...)}
}

func testLinks(pairs ...[2]uint16) []DirectedLink {
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

func testTree(maxDepth uint8, edges []RelayTreeEdge, covered, uncovered []uint16) RelayTree {
	return RelayTree{
		SourceStationID:      1,
		MembershipSnapshotID: 9,
		TopologyEpoch:        7,
		MaxRelayDepth:        maxDepth,
		Edges:                append([]RelayTreeEdge(nil), edges...),
		Coverage: CoverageSet{
			CoveredMemberIDs:   append([]uint16(nil), covered...),
			UncoveredMemberIDs: append([]uint16(nil), uncovered...),
		},
	}
}

func testConstraints() PlanningConstraints {
	return PlanningConstraints{
		PlanningAtMS:              ut020PlanningAtMS,
		MaxRelayDepth:             3,
		MaxPlayoutLatencyMS:       250,
		MaxVoiceAirtimePct:        100,
		RequireAllExpectedMembers: false,
		MaxLinkLoss:               1,
	}
}

func cloneSnapshot(snapshot ExpectedGroupSnapshot) ExpectedGroupSnapshot {
	snapshot.MemberIDs = append([]uint16(nil), snapshot.MemberIDs...)
	return snapshot
}

func cloneGraph(graph NeighborGraph) NeighborGraph {
	graph.Links = append([]DirectedLink(nil), graph.Links...)
	return graph
}

func cloneTree(tree RelayTree) RelayTree {
	tree.Edges = append([]RelayTreeEdge(nil), tree.Edges...)
	tree.Coverage.CoveredMemberIDs = append([]uint16(nil), tree.Coverage.CoveredMemberIDs...)
	tree.Coverage.UncoveredMemberIDs = append([]uint16(nil), tree.Coverage.UncoveredMemberIDs...)
	return tree
}

func requireRoutingError(t *testing.T, err error, substring string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q", substring)
	}
	if !strings.Contains(err.Error(), substring) {
		t.Fatalf("error = %q, want substring %q", err, substring)
	}
}
