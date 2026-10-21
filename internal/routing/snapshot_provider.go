package routing

import "sort"

type PreparedPlanSnapshot struct {
	Request              PlanRequest
	PrunedStaleLinkCount int
}

// PreparePlanSnapshot removes expired directed observations, not group members.
// The caller owns epoch/version changes; this helper never refreshes evidence.
// Raw graph/request validators and PlanRelayTree still reject stale graphs.
func PreparePlanSnapshot(request PlanRequest) (PreparedPlanSnapshot, error) {
	request = canonicalPlanRequestCopy(request)
	at := request.Constraints.PlanningAtMS
	if err := validateNeighborGraph(request.Graph, at, true); err != nil {
		return PreparedPlanSnapshot{}, err
	}
	fresh := make([]DirectedLink, 0, len(request.Graph.Links))
	pruned := 0
	for _, link := range request.Graph.Links {
		if at-link.ObservedAtMS > int64(request.Graph.FreshnessMS) {
			pruned++
			continue
		}
		fresh = append(fresh, link)
	}
	request.Graph.Links = fresh
	if err := ValidatePlanRequest(request); err != nil {
		return PreparedPlanSnapshot{}, err
	}
	return PreparedPlanSnapshot{Request: request, PrunedStaleLinkCount: pruned}, nil
}

func canonicalPlanRequestCopy(request PlanRequest) PlanRequest {
	request.Snapshot.MemberIDs = append([]uint16(nil), request.Snapshot.MemberIDs...)
	request.Graph.Links = append([]DirectedLink(nil), request.Graph.Links...)
	request.RelayStates = append([]RelayState(nil), request.RelayStates...)
	sort.Slice(request.Snapshot.MemberIDs, func(i, j int) bool { return request.Snapshot.MemberIDs[i] < request.Snapshot.MemberIDs[j] })
	sort.Slice(request.Graph.Links, func(i, j int) bool { return compareDirectedLinks(request.Graph.Links[i], request.Graph.Links[j]) < 0 })
	sort.Slice(request.RelayStates, func(i, j int) bool { return request.RelayStates[i].StationID < request.RelayStates[j].StationID })
	return request
}
