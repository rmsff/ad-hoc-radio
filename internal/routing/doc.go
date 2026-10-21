// Package routing defines deterministic, simulator-only domain contracts for
// expected-group snapshots, directed neighbor graphs, and bounded relay trees.
//
// PlanRelayTree selects an exact optimum within explicit search bounds; its
// Feasible list is not exhaustive. EnumerateFeasibleRelayTrees is the exhaustive
// reference. PreparePlanSnapshot filters expired link observations without
// changing expected membership or weakening raw evidence validation.
// The package does not schedule transmissions, forward voice, or model RF.
package routing
