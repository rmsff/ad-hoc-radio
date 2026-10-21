package scenario

import (
	"reflect"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/routing"
)

func TestIT054_CommonScheduledRuntimeBranchingTrees(t *testing.T) {
	tests := []struct {
		name               string
		edges              []routing.RelayTreeEdge
		covered            []uint16
		maxDepth           uint8
		relayTxPerFrame    int
		frameDeliveriesPer int
	}{
		{
			name: "one_relay_two_leaves",
			edges: []routing.RelayTreeEdge{
				{ParentStationID: 1, ChildStationID: 2, Hop: 0},
				{ParentStationID: 2, ChildStationID: 3, Hop: 1},
				{ParentStationID: 2, ChildStationID: 4, Hop: 1},
			},
			covered: []uint16{3, 4}, maxDepth: 1, relayTxPerFrame: 1, frameDeliveriesPer: 3,
		},
		{
			name: "two_relays_two_leaves",
			edges: []routing.RelayTreeEdge{
				{ParentStationID: 1, ChildStationID: 2, Hop: 0},
				{ParentStationID: 1, ChildStationID: 3, Hop: 0},
				{ParentStationID: 2, ChildStationID: 4, Hop: 1},
				{ParentStationID: 3, ChildStationID: 5, Hop: 1},
			},
			covered: []uint16{4, 5}, maxDepth: 1, relayTxPerFrame: 2, frameDeliveriesPer: 4,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			first := runScheduledBranchCase(t, tc.edges, tc.covered, tc.maxDepth)
			second := runScheduledBranchCase(t, tc.edges, tc.covered, tc.maxDepth)
			if first.Counters.RelayTransmissions != tc.relayTxPerFrame*3 {
				t.Fatalf("relay transmissions = %d, want %d", first.Counters.RelayTransmissions, tc.relayTxPerFrame*3)
			}
			if first.Counters.FrameDeliveries != tc.frameDeliveriesPer*3 {
				t.Fatalf("frame deliveries = %d, want %d", first.Counters.FrameDeliveries, tc.frameDeliveriesPer*3)
			}
			if !reflect.DeepEqual(first.DeliveredMemberIDs, tc.covered) {
				t.Fatalf("delivered members = %v, want %v", first.DeliveredMemberIDs, tc.covered)
			}
			for _, id := range tc.covered {
				counters := first.ReceiverCounters[id]
				if counters.FramesAccepted != 3 || counters.PlayoutFramesEmitted != 3 {
					t.Fatalf("receiver %d counters = %+v, want 3 accepted/emitted", id, counters)
				}
			}
			if first.Counters.DuplicatePlayoutCount != 0 {
				t.Fatalf("duplicate playout count = %d, want 0", first.Counters.DuplicatePlayoutCount)
			}
			if first.EventHash != second.EventHash || first.ResultHash != second.ResultHash {
				t.Fatalf("fixed-input hashes differ: event %q/%q result %q/%q", first.EventHash, second.EventHash, first.ResultHash, second.ResultHash)
			}
		})
	}
}

func runScheduledBranchCase(
	t *testing.T,
	edges []routing.RelayTreeEdge,
	covered []uint16,
	maxDepth uint8,
) ScheduledRuntimeResult {
	t.Helper()
	version := scheduledTestVersion(t, edges, covered, nil, maxDepth, 1, 0, 2)
	result, err := RunScheduledRuntime(scheduledTestRuntimeConfig(version, 3))
	if err != nil {
		t.Fatalf("RunScheduledRuntime failed: %v", err)
	}
	return result
}
