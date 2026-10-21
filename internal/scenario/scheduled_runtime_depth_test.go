package scenario

import (
	"reflect"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/routing"
)

func TestIT053_CommonScheduledRuntimeDepthOneTwoThree(t *testing.T) {
	tests := []struct {
		name       string
		edges      []routing.RelayTreeEdge
		covered    []uint16
		maxDepth   uint8
		leafID     uint16
		relayCount int
	}{
		{
			name: "depth_one",
			edges: []routing.RelayTreeEdge{
				{ParentStationID: 1, ChildStationID: 2, Hop: 0},
				{ParentStationID: 2, ChildStationID: 3, Hop: 1},
			},
			covered: []uint16{3}, maxDepth: 1, leafID: 3, relayCount: 1,
		},
		{
			name: "depth_two",
			edges: []routing.RelayTreeEdge{
				{ParentStationID: 1, ChildStationID: 2, Hop: 0},
				{ParentStationID: 2, ChildStationID: 3, Hop: 1},
				{ParentStationID: 3, ChildStationID: 4, Hop: 2},
			},
			covered: []uint16{4}, maxDepth: 2, leafID: 4, relayCount: 2,
		},
		{
			name: "depth_three",
			edges: []routing.RelayTreeEdge{
				{ParentStationID: 1, ChildStationID: 2, Hop: 0},
				{ParentStationID: 2, ChildStationID: 3, Hop: 1},
				{ParentStationID: 3, ChildStationID: 4, Hop: 2},
				{ParentStationID: 4, ChildStationID: 5, Hop: 3},
			},
			covered: []uint16{5}, maxDepth: 3, leafID: 5, relayCount: 3,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			first := runScheduledDepthCase(t, tc.edges, tc.covered, tc.maxDepth)
			second := runScheduledDepthCase(t, tc.edges, tc.covered, tc.maxDepth)
			if first.Counters.SourceTransmissions != 3 || first.Counters.RelayTransmissions != tc.relayCount*3 {
				t.Fatalf("transmission counters = %+v, want source=3 relay=%d", first.Counters, tc.relayCount*3)
			}
			if first.Counters.MissedOpportunityCount != 0 || first.Counters.OutOfScheduleTxCount != 0 {
				t.Fatalf("unexpected schedule counters: %+v", first.Counters)
			}
			if !reflect.DeepEqual(first.DeliveredMemberIDs, tc.covered) {
				t.Fatalf("delivered members = %v, want %v", first.DeliveredMemberIDs, tc.covered)
			}
			counters := first.ReceiverCounters[tc.leafID]
			if counters.FramesAccepted != 3 || counters.PlayoutFramesEmitted != 3 || counters.RelayedFramesAccepted != 3 {
				t.Fatalf("leaf receiver counters = %+v, want 3 relayed accepted/emitted", counters)
			}
			if first.EventHash != second.EventHash || first.ResultHash != second.ResultHash {
				t.Fatalf("fixed-input hashes differ: event %q/%q result %q/%q", first.EventHash, second.EventHash, first.ResultHash, second.ResultHash)
			}
		})
	}
}

func TestIT053_TransitRelayMayAlsoBeCoveredReceiver(t *testing.T) {
	version := scheduledTestVersion(t, []routing.RelayTreeEdge{
		{ParentStationID: 1, ChildStationID: 2, Hop: 0},
		{ParentStationID: 2, ChildStationID: 3, Hop: 1},
	}, []uint16{2, 3}, nil, 1, 1, 0, 2)
	result, err := RunScheduledRuntime(scheduledTestRuntimeConfig(version, 3))
	if err != nil {
		t.Fatalf("RunScheduledRuntime failed: %v", err)
	}
	if got := result.ReceiverCounters[2]; got.DirectFramesAccepted != 3 || got.PlayoutFramesEmitted != 3 {
		t.Fatalf("transit receiver counters = %+v, want 3 direct/emitted", got)
	}
	if got := result.ReceiverCounters[3]; got.RelayedFramesAccepted != 3 || got.PlayoutFramesEmitted != 3 {
		t.Fatalf("leaf receiver counters = %+v, want 3 relayed/emitted", got)
	}
	if result.Counters.RelayTransmissions != 3 {
		t.Fatalf("relay transmissions = %d, want 3", result.Counters.RelayTransmissions)
	}
}

func runScheduledDepthCase(
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
