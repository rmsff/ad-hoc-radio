package routing

import (
	"fmt"
	"testing"
)

func TestBoundedPlannerDirectGroups(t *testing.T) {
	for _, count := range []int{8, 16, 32} {
		for _, requireAll := range []bool{false, true} {
			t.Run(fmt.Sprintf("members=%d/all=%t", count, requireAll), func(t *testing.T) {
				request := directGroupRequest(count)
				request.Constraints.RequireAllExpectedMembers = requireAll
				result, err := PlanRelayTree(request)
				if err != nil {
					t.Fatal(err)
				}
				if result.Selected.Cost.CoveredMemberCount != count || result.Selected.Cost.TxOpportunityCount != 1 {
					t.Fatalf("wrong direct plan: %+v", result.Selected.Cost)
				}
				if result.EvaluatedCandidateCount > 2 {
					t.Fatalf("direct certificate enumerated %d candidates", result.EvaluatedCandidateCount)
				}
			})
		}
	}
}

func TestBoundedPlannerDirectGroupWithUnreachableMember(t *testing.T) {
	request := directGroupRequest(32)
	request.Snapshot.MemberIDs = append(request.Snapshot.MemberIDs, 99)
	result, err := PlanRelayTree(request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Selected.Cost.CoveredMemberCount != 32 || len(result.Selected.Tree.Coverage.UncoveredMemberIDs) != 1 || result.Selected.Tree.Coverage.UncoveredMemberIDs[0] != 99 {
		t.Fatalf("wrong coverage: %+v", result.Selected.Tree.Coverage)
	}
}

func directGroupRequest(count int) PlanRequest {
	members := make([]uint16, count)
	links := make([]DirectedLink, 0, count)
	for i := range members {
		members[i] = uint16(i + 2)
		links = append(links, plannerLinks([2]uint16{1, members[i]})...)
	}
	return plannerRequest(members, links, nil)
}
