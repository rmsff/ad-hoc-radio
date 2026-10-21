package routing

import "fmt"

// PlanningDiagnostics counts logical search work, excluding input validation,
// canonical sorting and per-step tree construction. It is not a wall-clock bound.
type PlanningDiagnostics struct {
	SearchStepLimit     int
	PathSteps           int
	TreeSteps           int
	PathsEnumerated     int
	EvaluatedCandidates int
	PrunedBranches      int
	DirectOptimal       bool
	ShallowCoveredCount int
}

// PlanningLimitError exposes bounded work, never a partial route authorization.
type PlanningLimitError struct {
	Limit       string
	Bound       int
	Diagnostics PlanningDiagnostics
}

func (e *PlanningLimitError) Error() string {
	return fmt.Sprintf("%v: %s=%d (path_steps=%d tree_steps=%d candidates=%d)",
		ErrPlanningSearchLimitExceeded, e.Limit, e.Bound,
		e.Diagnostics.PathSteps, e.Diagnostics.TreeSteps, e.Diagnostics.EvaluatedCandidates)
}

func (e *PlanningLimitError) Unwrap() error { return ErrPlanningSearchLimitExceeded }

type planningBudget struct {
	diagnostics PlanningDiagnostics
}

func newPlanningBudget(request PlanRequest) (*planningBudget, error) {
	limit := request.SearchLimits.MaxSearchSteps
	if limit == 0 {
		maxInt := int(^uint(0) >> 1)
		factor := int(request.Constraints.MaxRelayDepth) + 2
		members := len(request.Snapshot.MemberIDs) + 1
		if request.SearchLimits.MaxTreeCandidates > maxInt/members/factor {
			return nil, fmt.Errorf("planning search limits: derived max_search_steps overflows int; set it explicitly")
		}
		limit = request.SearchLimits.MaxTreeCandidates * members * factor
	}
	return &planningBudget{diagnostics: PlanningDiagnostics{SearchStepLimit: limit}}, nil
}

func (b *planningBudget) step(path bool) error {
	if b.diagnostics.PathSteps >= b.diagnostics.SearchStepLimit-b.diagnostics.TreeSteps {
		return b.exceeded("max_search_steps", b.diagnostics.SearchStepLimit)
	}
	if path {
		b.diagnostics.PathSteps++
	} else {
		b.diagnostics.TreeSteps++
	}
	return nil
}

func (b *planningBudget) exceeded(limit string, bound int) error {
	return &PlanningLimitError{Limit: limit, Bound: bound, Diagnostics: b.diagnostics}
}
