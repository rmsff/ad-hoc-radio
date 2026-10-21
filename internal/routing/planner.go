package routing

import (
	"cmp"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// EnumerateFeasibleRelayTrees retains the exhaustive reference search and its
// candidate/path limits. MaxSearchSteps applies only to PlanRelayTree.
func EnumerateFeasibleRelayTrees(request PlanRequest) (PlanResult, error) {
	if err := ValidatePlanRequest(request); err != nil {
		return PlanResult{}, err
	}

	linkIndex := indexDirectedLinks(request.Graph)
	relayIndex := indexRelayStates(request.RelayStates)
	pathsByMember, err := enumerateMemberPaths(request, relayIndex)
	if err != nil {
		return PlanResult{}, err
	}

	state := enumerationState{
		request:          request,
		linkIndex:        linkIndex,
		relayIndex:       relayIndex,
		seenTreeHashes:   make(map[string]struct{}),
		selectedByMember: make([]plannerPath, len(pathsByMember)),
	}
	if err := state.combine(pathsByMember, 0); err != nil {
		if errors.Is(err, ErrPlanningSearchLimitExceeded) {
			return PlanResult{}, err
		}
		return PlanResult{}, err
	}

	feasible, err := applyDepth3AdditionalCoverageRule(
		state.feasible,
		state.depth3Candidates,
		request.SearchLimits.MaxFeasibleTrees,
	)
	if err != nil {
		return PlanResult{}, err
	}
	if len(feasible) == 0 {
		return PlanResult{}, ErrNoFeasibleRelayTree
	}
	sortEvaluatedTrees(feasible)

	return PlanResult{
		Selected:                feasible[0],
		Feasible:                feasible,
		EvaluatedCandidateCount: state.evaluatedCandidateCount,
		CompleteEnumeration:     true,
	}, nil
}

// PlanRelayTree canonicalizes a copy, then finds the exact optimum or returns
// an error without a partial winner. Feasible contains only visited candidates.
func PlanRelayTree(request PlanRequest) (PlanResult, error) {
	return planBoundedRelayTree(request)
}

type enumerationState struct {
	request                 PlanRequest
	linkIndex               map[[2]uint16]DirectedLink
	relayIndex              map[uint16]RelayState
	seenTreeHashes          map[string]struct{}
	selectedByMember        []plannerPath
	feasible                []EvaluatedTree
	depth3Candidates        []EvaluatedTree
	evaluatedCandidateCount int
}

func (s *enumerationState) combine(pathsByMember []memberPaths, memberIndex int) error {
	if memberIndex == len(pathsByMember) {
		return s.evaluateSelectedPaths()
	}
	choices := pathsByMember[memberIndex].choices
	for _, choice := range choices {
		s.selectedByMember[memberIndex] = choice
		if err := s.combine(pathsByMember, memberIndex+1); err != nil {
			return err
		}
	}
	return nil
}

func (s *enumerationState) evaluateSelectedPaths() error {
	if s.evaluatedCandidateCount >= s.request.SearchLimits.MaxTreeCandidates {
		return fmt.Errorf("%w: max_tree_candidates=%d", ErrPlanningSearchLimitExceeded, s.request.SearchLimits.MaxTreeCandidates)
	}
	s.evaluatedCandidateCount++

	tree, ok := buildRelayTreeFromPaths(s.request, s.selectedByMember)
	if !ok {
		return nil
	}

	if err := ValidateRelayTree(s.request.Snapshot, s.request.Graph, s.request.Constraints, tree); err != nil {
		return nil
	}

	canonicalJSON, err := CanonicalRelayTreeJSON(tree)
	if err != nil {
		return err
	}
	hash, err := RelayTreeHash(tree)
	if err != nil {
		return err
	}
	if _, duplicate := s.seenTreeHashes[hash]; duplicate {
		return nil
	}
	s.seenTreeHashes[hash] = struct{}{}

	cost, feasible, err := evaluateTreeCost(s.request, tree, s.linkIndex, s.relayIndex)
	if err != nil {
		return err
	}
	if !feasible {
		return nil
	}
	evaluated := EvaluatedTree{
		Tree:          tree,
		TreeHash:      hash,
		CanonicalJSON: string(canonicalJSON),
		Cost:          cost,
	}
	if cost.SelectedMaxRelayDepth == 3 {
		s.depth3Candidates = append(s.depth3Candidates, evaluated)
		return nil
	}
	if len(s.feasible) >= s.request.SearchLimits.MaxFeasibleTrees {
		return fmt.Errorf("%w: max_feasible_trees=%d", ErrPlanningSearchLimitExceeded, s.request.SearchLimits.MaxFeasibleTrees)
	}
	s.feasible = append(s.feasible, evaluated)
	return nil
}

func applyDepth3AdditionalCoverageRule(shallower, depth3 []EvaluatedTree, maxFeasibleTrees int) ([]EvaluatedTree, error) {
	shallowerCovered := make(map[uint16]struct{})
	for _, candidate := range shallower {
		for _, memberID := range candidate.Tree.Coverage.CoveredMemberIDs {
			shallowerCovered[memberID] = struct{}{}
		}
	}

	filtered := make([]EvaluatedTree, len(shallower), len(shallower)+len(depth3))
	copy(filtered, shallower)
	for _, candidate := range depth3 {
		addsCoverage := false
		for _, memberID := range candidate.Tree.Coverage.CoveredMemberIDs {
			if _, alreadyCovered := shallowerCovered[memberID]; !alreadyCovered {
				addsCoverage = true
				break
			}
		}
		if addsCoverage {
			if len(filtered) >= maxFeasibleTrees {
				return nil, fmt.Errorf("%w: max_feasible_trees=%d", ErrPlanningSearchLimitExceeded, maxFeasibleTrees)
			}
			filtered = append(filtered, candidate)
		}
	}
	return filtered, nil
}

func sortEvaluatedTrees(trees []EvaluatedTree) {
	sort.Slice(trees, func(i, j int) bool {
		left := trees[i].Cost
		right := trees[j].Cost
		if left.CoveredMemberCount != right.CoveredMemberCount {
			return left.CoveredMemberCount > right.CoveredMemberCount
		}
		if left.SelectedMaxRelayDepth != right.SelectedMaxRelayDepth {
			return left.SelectedMaxRelayDepth < right.SelectedMaxRelayDepth
		}
		if left.TxOpportunityCount != right.TxOpportunityCount {
			return left.TxOpportunityCount < right.TxOpportunityCount
		}
		if left.AggregateLinkRiskUnits != right.AggregateLinkRiskUnits {
			return left.AggregateLinkRiskUnits < right.AggregateLinkRiskUnits
		}
		if left.AggregateBatteryCostUnits != right.AggregateBatteryCostUnits {
			return left.AggregateBatteryCostUnits < right.AggregateBatteryCostUnits
		}
		// TreeHash is a drift guard: it decides only if compareRelayTrees
		// misses a canonical field.
		return cmp.Or(
			compareRelayTrees(trees[i].Tree, trees[j].Tree),
			strings.Compare(trees[i].TreeHash, trees[j].TreeHash),
		) < 0
	})
}

func indexDirectedLinks(graph NeighborGraph) map[[2]uint16]DirectedLink {
	links := make(map[[2]uint16]DirectedLink, len(graph.Links))
	for _, link := range graph.Links {
		links[[2]uint16{link.FromStationID, link.ToStationID}] = link
	}
	return links
}

func indexRelayStates(states []RelayState) map[uint16]RelayState {
	index := make(map[uint16]RelayState, len(states))
	for _, state := range states {
		index[state.StationID] = state
	}
	return index
}
