package routing

import (
	"math/rand"
	"reflect"
	"testing"
)

// equalCostPair returns the two feasible trees whose first edge reaches
// firstChild and otherChild and checks that their costs are equal.
func equalCostPair(t *testing.T, result PlanResult, firstChild, otherChild uint16) (EvaluatedTree, EvaluatedTree) {
	t.Helper()
	var first, other EvaluatedTree
	for _, candidate := range result.Feasible {
		if candidate.Cost.CoveredMemberCount == 0 {
			continue
		}
		switch candidate.Tree.Edges[0].ChildStationID {
		case firstChild:
			first = candidate
		case otherChild:
			other = candidate
		}
	}
	if first.TreeHash == "" || other.TreeHash == "" {
		t.Fatalf("fixture must produce trees through %d and %d, got %d feasible", firstChild, otherChild, len(result.Feasible))
	}
	if !reflect.DeepEqual(first.Cost, other.Cost) {
		t.Fatalf("fixture trees must have equal cost: %+v vs %+v", first.Cost, other.Cost)
	}
	return first, other
}

func assertPlannerSelects(t *testing.T, request PlanRequest, want []RelayTreeEdge) {
	t.Helper()
	enumerated, err := EnumerateFeasibleRelayTrees(request)
	if err != nil {
		t.Fatalf("EnumerateFeasibleRelayTrees failed: %v", err)
	}
	planned, err := PlanRelayTree(request)
	if err != nil {
		t.Fatalf("PlanRelayTree failed: %v", err)
	}
	assertTreeEdges(t, planned.Selected.Tree.Edges, want)
	assertTreeEdges(t, enumerated.Selected.Tree.Edges, want)
	assertSelectedIsFirstFeasible(t, enumerated)
}

// Equal cost and equal covered members: only the numeric versus textual order
// of the relay id separates the trees through relay 9 and relay 10.
func TestPlannerTieBreakPrefersNumericallySmallerRelay(t *testing.T) {
	request := plannerRequest(
		[]uint16{2},
		plannerLinks([2]uint16{1, 9}, [2]uint16{1, 10}, [2]uint16{9, 2}, [2]uint16{10, 2}),
		plannerStates(9, 10),
	)
	enumerated, err := EnumerateFeasibleRelayTrees(request)
	if err != nil {
		t.Fatalf("EnumerateFeasibleRelayTrees failed: %v", err)
	}
	relay9, relay10 := equalCostPair(t, enumerated, 9, 10)
	if relay10.CanonicalJSON >= relay9.CanonicalJSON {
		t.Fatalf("fixture no longer separates text order from numeric order:\n%s\n%s", relay9.CanonicalJSON, relay10.CanonicalJSON)
	}
	assertPlannerSelects(t, request, relay9.Tree.Edges)
}

// Equal cost, different covered members: covered [9] through relay 20 must
// beat covered [10] through relay 3. Text order ("[10]" < "[9]") and an
// edges-first order (child 3 < 20) would both pick the other tree.
func TestPlannerTieBreakComparesCoveredBeforeEdges(t *testing.T) {
	request := plannerRequest(
		[]uint16{9, 10},
		plannerLinks([2]uint16{1, 3}, [2]uint16{1, 20}, [2]uint16{3, 10}, [2]uint16{20, 9}),
		plannerStates(3, 20),
	)
	// Two relays need a 33 ms cycle; 22 ms admits exactly one relay per tree.
	request.CostModel.FramePeriodMS = 22
	enumerated, err := EnumerateFeasibleRelayTrees(request)
	if err != nil {
		t.Fatalf("EnumerateFeasibleRelayTrees failed: %v", err)
	}
	via20, via3 := equalCostPair(t, enumerated, 20, 3)
	if !reflect.DeepEqual(via20.Tree.Coverage.CoveredMemberIDs, []uint16{9}) ||
		!reflect.DeepEqual(via3.Tree.Coverage.CoveredMemberIDs, []uint16{10}) {
		t.Fatalf("covered = %v and %v, want [9] and [10]", via20.Tree.Coverage.CoveredMemberIDs, via3.Tree.Coverage.CoveredMemberIDs)
	}
	if via3.CanonicalJSON >= via20.CanonicalJSON || compareCanonicalEdges(via3.Tree.Edges[0], via20.Tree.Edges[0]) >= 0 {
		t.Fatal("fixture no longer separates covered-first numeric order from text and edges-first orders")
	}
	assertPlannerSelects(t, request, via20.Tree.Edges)
}

// sortEvaluatedTrees gives one order for every permutation of its input.
func TestSortEvaluatedTreesIsIndependentOfInputOrder(t *testing.T) {
	request := plannerRequest(
		[]uint16{2, 3},
		plannerLinks(
			[2]uint16{1, 2}, [2]uint16{1, 3}, [2]uint16{1, 9}, [2]uint16{1, 10},
			[2]uint16{9, 2}, [2]uint16{9, 3}, [2]uint16{10, 2}, [2]uint16{10, 3},
		),
		plannerStates(9, 10),
	)
	enumerated, err := EnumerateFeasibleRelayTrees(request)
	if err != nil {
		t.Fatalf("EnumerateFeasibleRelayTrees failed: %v", err)
	}
	if len(enumerated.Feasible) < 10 {
		t.Fatalf("feasible count = %d, want a rich fixture", len(enumerated.Feasible))
	}
	for i := 1; i < len(enumerated.Feasible); i++ {
		if testEvaluatedTreeOutranks(enumerated.Feasible[i], enumerated.Feasible[i-1]) {
			t.Fatalf("sorted order is not monotonic under the independent oracle at index %d", i)
		}
	}
	want := treeHashes(enumerated.Feasible)
	rng := rand.New(rand.NewSource(20261002))
	for run := 0; run < 100; run++ {
		shuffled := append([]EvaluatedTree(nil), enumerated.Feasible...)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		sortEvaluatedTrees(shuffled)
		if got := treeHashes(shuffled); !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d: order differs after shuffle:\n got %v\nwant %v", run, got, want)
		}
	}
}

func TestUT021_EnumerateFeasibleOrderMatchesPlanSelection(t *testing.T) {
	request := plannerRequest(
		[]uint16{2, 3},
		plannerLinks([2]uint16{1, 2}, [2]uint16{1, 10}, [2]uint16{10, 2}, [2]uint16{10, 3}),
		plannerStates(10),
	)
	result, err := EnumerateFeasibleRelayTrees(request)
	if err != nil {
		t.Fatalf("EnumerateFeasibleRelayTrees failed: %v", err)
	}
	if len(result.Feasible) < 2 {
		t.Fatalf("feasible count = %d, want at least 2", len(result.Feasible))
	}
	assertSelectedIsFirstFeasible(t, result)
	for i, candidate := range result.Feasible {
		if testEvaluatedTreeOutranks(candidate, result.Selected) {
			t.Fatalf("feasible[%d] independently outranks selected: %s", i, candidate.CanonicalJSON)
		}
		if i > 0 && testEvaluatedTreeOutranks(candidate, result.Feasible[i-1]) {
			t.Fatalf("feasible order is not monotonic at index %d", i)
		}
	}

	planned, err := PlanRelayTree(request)
	if err != nil {
		t.Fatalf("PlanRelayTree failed: %v", err)
	}
	if planned.Selected.TreeHash != result.Feasible[0].TreeHash {
		t.Fatalf("PlanRelayTree hash = %s, want first feasible hash %s", planned.Selected.TreeHash, result.Feasible[0].TreeHash)
	}
}

// testEvaluatedTreeOutranks is the independent oracle of the ADR-0015
// objective. It spells out the tie-break tuple itself instead of calling
// compareRelayTrees.
func testEvaluatedTreeOutranks(left, right EvaluatedTree) bool {
	l, r := left.Cost, right.Cost
	switch {
	case l.CoveredMemberCount != r.CoveredMemberCount:
		return l.CoveredMemberCount > r.CoveredMemberCount
	case l.SelectedMaxRelayDepth != r.SelectedMaxRelayDepth:
		return l.SelectedMaxRelayDepth < r.SelectedMaxRelayDepth
	case l.TxOpportunityCount != r.TxOpportunityCount:
		return l.TxOpportunityCount < r.TxOpportunityCount
	case l.AggregateLinkRiskUnits != r.AggregateLinkRiskUnits:
		return l.AggregateLinkRiskUnits < r.AggregateLinkRiskUnits
	case l.AggregateBatteryCostUnits != r.AggregateBatteryCostUnits:
		return l.AggregateBatteryCostUnits < r.AggregateBatteryCostUnits
	}
	lt, rt := testTreeTuple(left.Tree), testTreeTuple(right.Tree)
	for i := 0; i < len(lt) && i < len(rt); i++ {
		if lt[i] != rt[i] {
			return lt[i] < rt[i]
		}
	}
	if len(lt) != len(rt) {
		return len(lt) < len(rt)
	}
	return left.TreeHash < right.TreeHash
}

// testTreeTuple writes the tie-break key as one integer sequence: covered ids,
// edges as (child, hop, parent), max relay depth, snapshot id, source id,
// topology epoch, uncovered ids. Each list element is preceded by 1 and the
// list is closed by 0, so a list that is a prefix of another ranks first.
func testTreeTuple(tree RelayTree) []int64 {
	var tuple []int64
	for _, id := range tree.Coverage.CoveredMemberIDs {
		tuple = append(tuple, 1, int64(id))
	}
	tuple = append(tuple, 0)
	for _, edge := range tree.Edges {
		tuple = append(tuple, 1, int64(edge.ChildStationID), int64(edge.Hop), int64(edge.ParentStationID))
	}
	tuple = append(tuple, 0,
		int64(tree.MaxRelayDepth),
		int64(tree.MembershipSnapshotID),
		int64(tree.SourceStationID),
		int64(tree.TopologyEpoch),
	)
	for _, id := range tree.Coverage.UncoveredMemberIDs {
		tuple = append(tuple, 1, int64(id))
	}
	return append(tuple, 0)
}

func treeHashes(trees []EvaluatedTree) []string {
	hashes := make([]string, len(trees))
	for i, tree := range trees {
		hashes[i] = tree.TreeHash
	}
	return hashes
}
