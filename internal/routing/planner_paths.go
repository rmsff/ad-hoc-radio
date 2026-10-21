package routing

import (
	"fmt"
	"sort"
)

type plannerPath struct {
	memberID uint16
	nodes    []uint16
}

type memberPaths struct {
	memberID uint16
	choices  []plannerPath
}

func enumerateMemberPaths(request PlanRequest, relayIndex map[uint16]RelayState) ([]memberPaths, error) {
	return enumerateMemberPathsWithBudget(request, relayIndex, nil)
}

func enumerateMemberPathsWithBudget(request PlanRequest, relayIndex map[uint16]RelayState, budget *planningBudget) ([]memberPaths, error) {
	adjacency := buildFeasibleAdjacency(request)
	pathsByMember := make([]memberPaths, 0, len(request.Snapshot.MemberIDs))
	for _, memberID := range request.Snapshot.MemberIDs {
		paths, err := enumerateSimplePathsToMemberWithBudget(request, memberID, adjacency, relayIndex, budget)
		if err != nil {
			return nil, err
		}
		choices := make([]plannerPath, 0, len(paths)+1)
		if !request.Constraints.RequireAllExpectedMembers {
			choices = append(choices, plannerPath{memberID: memberID})
		}
		choices = append(choices, paths...)
		pathsByMember = append(pathsByMember, memberPaths{memberID: memberID, choices: choices})
	}
	return pathsByMember, nil
}

func buildFeasibleAdjacency(request PlanRequest) map[uint16][]uint16 {
	adjacency := make(map[uint16][]uint16)
	for _, link := range request.Graph.Links {
		if !link.Enabled {
			continue
		}
		if link.Quality < request.Constraints.MinLinkQuality {
			continue
		}
		if link.Loss > request.Constraints.MaxLinkLoss {
			continue
		}
		adjacency[link.FromStationID] = append(adjacency[link.FromStationID], link.ToStationID)
	}
	return adjacency
}

func enumerateSimplePathsToMemberWithBudget(request PlanRequest, memberID uint16, adjacency map[uint16][]uint16, relayIndex map[uint16]RelayState, budget *planningBudget) ([]plannerPath, error) {
	visited := map[uint16]struct{}{request.SourceStationID: {}}
	nodes := []uint16{request.SourceStationID}
	paths := make([]plannerPath, 0)

	var walk func(current uint16) error
	walk = func(current uint16) error {
		for _, next := range adjacency[current] {
			if budget != nil {
				if err := budget.step(true); err != nil {
					return err
				}
			}
			if _, repeated := visited[next]; repeated {
				continue
			}
			if next == memberID {
				if len(paths) >= request.SearchLimits.MaxPathsPerMember {
					if budget != nil {
						return budget.exceeded("max_paths_per_member", request.SearchLimits.MaxPathsPerMember)
					}
					return fmt.Errorf("%w: max_paths_per_member=%d for member_id=%d", ErrPlanningSearchLimitExceeded, request.SearchLimits.MaxPathsPerMember, memberID)
				}
				pathNodes := append(append([]uint16(nil), nodes...), next)
				paths = append(paths, plannerPath{memberID: memberID, nodes: pathNodes})
				if budget != nil {
					budget.diagnostics.PathsEnumerated++
				}
				continue
			}

			relayDepthAfterAddingNext := len(nodes)
			if relayDepthAfterAddingNext > int(request.Constraints.MaxRelayDepth) {
				continue
			}
			if !relayEligible(next, request.Constraints, relayIndex) {
				continue
			}

			visited[next] = struct{}{}
			nodes = append(nodes, next)
			if err := walk(next); err != nil {
				return err
			}
			nodes = nodes[:len(nodes)-1]
			delete(visited, next)
		}
		return nil
	}
	if err := walk(request.SourceStationID); err != nil {
		return nil, err
	}
	return paths, nil
}

func relayEligible(stationID uint16, constraints PlanningConstraints, relayIndex map[uint16]RelayState) bool {
	state, ok := relayIndex[stationID]
	if !ok {
		return false
	}
	return state.RelayCapable &&
		state.RelayEnabled &&
		!state.Busy &&
		state.BatteryFraction >= constraints.MinRelayBatteryFraction
}

func buildRelayTreeFromPaths(request PlanRequest, selected []plannerPath) (RelayTree, bool) {
	edgesByChild := make(map[uint16]RelayTreeEdge)
	for _, path := range selected {
		if len(path.nodes) == 0 {
			continue
		}
		for i := 0; i < len(path.nodes)-1; i++ {
			edge := RelayTreeEdge{
				ParentStationID: path.nodes[i],
				ChildStationID:  path.nodes[i+1],
				Hop:             uint8(i),
			}
			if existing, exists := edgesByChild[edge.ChildStationID]; exists {
				if existing != edge {
					return RelayTree{}, false
				}
				continue
			}
			edgesByChild[edge.ChildStationID] = edge
		}
	}

	edges := make([]RelayTreeEdge, 0, len(edgesByChild))
	for _, edge := range edgesByChild {
		edges = append(edges, edge)
	}
	sort.Slice(edges, func(i, j int) bool {
		return compareRelayTreeEdges(edges[i], edges[j]) < 0
	})

	reachable := reachableStations(request.SourceStationID, edges)
	covered := make([]uint16, 0, len(request.Snapshot.MemberIDs))
	uncovered := make([]uint16, 0, len(request.Snapshot.MemberIDs))
	for _, memberID := range request.Snapshot.MemberIDs {
		if _, ok := reachable[memberID]; ok {
			covered = append(covered, memberID)
		} else {
			uncovered = append(uncovered, memberID)
		}
	}

	selectedDepth := uint8(0)
	for _, edge := range edges {
		if edge.Hop > selectedDepth {
			selectedDepth = edge.Hop
		}
	}

	return RelayTree{
		SourceStationID:      request.SourceStationID,
		MembershipSnapshotID: request.Snapshot.ID,
		TopologyEpoch:        request.Graph.Epoch,
		MaxRelayDepth:        selectedDepth,
		Edges:                edges,
		Coverage: CoverageSet{
			CoveredMemberIDs:   covered,
			UncoveredMemberIDs: uncovered,
		},
	}, true
}

func reachableStations(sourceID uint16, edges []RelayTreeEdge) map[uint16]struct{} {
	reachable := map[uint16]struct{}{sourceID: {}}
	for _, edge := range edges {
		if _, ok := reachable[edge.ParentStationID]; ok {
			reachable[edge.ChildStationID] = struct{}{}
		}
	}
	return reachable
}
