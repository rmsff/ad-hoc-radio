package routing

import "sort"

type boundedPlanner struct {
	request  PlanRequest
	budget   *planningBudget
	links    map[[2]uint16]DirectedLink
	relays   map[uint16]RelayState
	chosen   []plannerPath
	seen     map[string]bool
	shallow  map[uint16]bool
	best     *EvaluatedTree
	feasible []EvaluatedTree
}

func planBoundedRelayTree(request PlanRequest) (PlanResult, error) {
	request = canonicalPlanRequestCopy(request)
	if err := ValidatePlanRequest(request); err != nil {
		return PlanResult{}, err
	}
	budget, err := newPlanningBudget(request)
	if err != nil {
		return PlanResult{}, err
	}
	s := boundedPlanner{
		request: request, budget: budget,
		links: indexDirectedLinks(request.Graph), relays: indexRelayStates(request.RelayStates),
		chosen: make([]plannerPath, len(request.Snapshot.MemberIDs)),
		seen:   make(map[string]bool), shallow: make(map[uint16]bool),
	}
	if ok, err := s.tryDirectCertificate(); err != nil {
		return PlanResult{}, err
	} else if ok {
		return s.result(), nil
	}
	paths, err := enumerateMemberPathsWithBudget(request, s.relays, budget)
	if err != nil {
		return PlanResult{}, err
	}
	// Visit short paths before omission so useful coverage bounds appear early.
	for i := range paths {
		sort.SliceStable(paths[i].choices, func(a, b int) bool {
			x, y := len(paths[i].choices[a].nodes), len(paths[i].choices[b].nodes)
			return y == 0 && x != 0 || x != 0 && y != 0 && x < y
		})
	}
	shallowPaths := make([]memberPaths, len(paths))
	hasDepth3 := false
	for i, member := range paths {
		shallowPaths[i].memberID = member.memberID
		for _, path := range member.choices {
			if len(path.nodes) <= 4 {
				shallowPaths[i].choices = append(shallowPaths[i].choices, path)
			} else {
				hasDepth3 = true
			}
		}
	}
	if err := s.walk(shallowPaths, 0, false); err != nil {
		return PlanResult{}, err
	}
	budget.diagnostics.ShallowCoveredCount = len(s.shallow)
	if hasDepth3 && len(s.shallow) < len(request.Snapshot.MemberIDs) {
		if err := s.walk(paths, 0, true); err != nil {
			return PlanResult{}, err
		}
	}
	if s.best == nil {
		return PlanResult{}, ErrNoFeasibleRelayTree
	}
	return s.result(), nil
}

func (s *boundedPlanner) tryDirectCertificate() (bool, error) {
	selected := make([]plannerPath, len(s.chosen))
	for i, member := range s.request.Snapshot.MemberIDs {
		link, ok := s.links[[2]uint16{s.request.SourceStationID, member}]
		if !ok || !link.Enabled || link.Quality < s.request.Constraints.MinLinkQuality || link.Loss > s.request.Constraints.MaxLinkLoss {
			return false, nil
		}
		selected[i] = plannerPath{memberID: member, nodes: []uint16{s.request.SourceStationID, member}}
	}
	tree, _ := buildRelayTreeFromPaths(s.request, selected)
	cost, feasible, err := evaluateTreeCost(s.request, tree, s.links, s.relays)
	if err != nil || !feasible {
		return false, err
	}
	if err := s.budget.step(false); err != nil {
		return false, err
	}
	if err := s.retain(tree, cost); err != nil {
		return false, err
	}
	// Full coverage, depth zero and one transmitter have a unique possible tree.
	s.budget.diagnostics.DirectOptimal = true
	s.budget.diagnostics.ShallowCoveredCount = len(tree.Coverage.CoveredMemberIDs)
	return true, nil
}

func (s *boundedPlanner) walk(paths []memberPaths, index int, depth3 bool) error {
	if err := s.budget.step(false); err != nil {
		return err
	}
	partial, ok := buildRelayTreeFromPaths(s.request, s.chosen[:index])
	if !ok {
		s.budget.diagnostics.PrunedBranches++
		return nil
	}
	cost, feasible, err := evaluateTreeCost(s.request, partial, s.links, s.relays)
	if err != nil {
		return err
	}
	if !feasible {
		s.budget.diagnostics.PrunedBranches++
		return nil
	}
	possible := possibleCoverage(s.request.Snapshot.MemberIDs, partial, paths[index:])
	if s.canPrune(cost, possible, depth3) {
		s.budget.diagnostics.PrunedBranches++
		return nil
	}
	if index == len(paths) {
		if depth3 && partial.MaxRelayDepth != 3 {
			return nil
		}
		if err := ValidateRelayTree(s.request.Snapshot, s.request.Graph, s.request.Constraints, partial); err != nil {
			return nil
		}
		if !depth3 {
			for _, id := range partial.Coverage.CoveredMemberIDs {
				s.shallow[id] = true
			}
		}
		return s.retain(partial, cost)
	}
	for _, choice := range paths[index].choices {
		s.chosen[index] = choice
		if err := s.walk(paths, index+1, depth3); err != nil {
			return err
		}
	}
	return nil
}

func possibleCoverage(expected []uint16, partial RelayTree, remaining []memberPaths) map[uint16]bool {
	reachable := make(map[uint16]bool)
	for _, id := range partial.Coverage.CoveredMemberIDs {
		reachable[id] = true
	}
	for _, member := range remaining {
		for _, path := range member.choices {
			for _, id := range path.nodes {
				reachable[id] = true
			}
		}
	}
	possible := make(map[uint16]bool)
	for _, id := range expected {
		if reachable[id] {
			possible[id] = true
		}
	}
	return possible
}

func (s *boundedPlanner) canPrune(lower TreeCost, possible map[uint16]bool, depth3 bool) bool {
	addsShallowWitness := false
	for id := range possible {
		if !s.shallow[id] {
			addsShallowWitness = true
			break
		}
	}
	if depth3 && !addsShallowWitness {
		return true
	}
	if s.request.Constraints.RequireAllExpectedMembers && len(possible) < len(s.request.Snapshot.MemberIDs) {
		return true
	}
	// Even an inferior shallow tree can veto depth 3 by witnessing coverage.
	if s.best == nil || !depth3 && addsShallowWitness {
		return false
	}
	best := s.best.Cost
	if len(possible) != best.CoveredMemberCount {
		return len(possible) < best.CoveredMemberCount
	}
	if lower.SelectedMaxRelayDepth != best.SelectedMaxRelayDepth {
		return lower.SelectedMaxRelayDepth > best.SelectedMaxRelayDepth
	}
	// Do not prune equality: link risk, battery and canonical order still matter.
	return lower.TxOpportunityCount > best.TxOpportunityCount
}

func (s *boundedPlanner) retain(tree RelayTree, cost TreeCost) error {
	hash, err := RelayTreeHash(tree)
	if err != nil {
		return err
	}
	if s.seen[hash] {
		return nil
	}
	if s.budget.diagnostics.EvaluatedCandidates >= s.request.SearchLimits.MaxTreeCandidates {
		return s.budget.exceeded("max_tree_candidates", s.request.SearchLimits.MaxTreeCandidates)
	}
	s.budget.diagnostics.EvaluatedCandidates++
	if len(s.feasible) >= s.request.SearchLimits.MaxFeasibleTrees {
		return s.budget.exceeded("max_feasible_trees", s.request.SearchLimits.MaxFeasibleTrees)
	}
	encoded, err := CanonicalRelayTreeJSON(tree)
	if err != nil {
		return err
	}
	s.seen[hash] = true
	candidate := EvaluatedTree{Tree: tree, TreeHash: hash, CanonicalJSON: string(encoded), Cost: cost}
	s.feasible = append(s.feasible, candidate)
	if s.best == nil {
		s.best = &candidate
	} else {
		pair := []EvaluatedTree{*s.best, candidate}
		sortEvaluatedTrees(pair)
		s.best = &pair[0]
	}
	return nil
}

func (s *boundedPlanner) result() PlanResult {
	sortEvaluatedTrees(s.feasible)
	return PlanResult{
		Selected: s.feasible[0], Feasible: s.feasible,
		EvaluatedCandidateCount: s.budget.diagnostics.EvaluatedCandidates,
		Diagnostics:             s.budget.diagnostics,
	}
}
