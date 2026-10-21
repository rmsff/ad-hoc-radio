package scenario

import (
	"reflect"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/node"
	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

func TestIT052_CommonScheduledRuntimeDirect(t *testing.T) {
	first := runIT052Direct(t)
	second := runIT052Direct(t)

	if first.Counters.SourceFramesCreated != 3 {
		t.Fatalf("source frames created = %d, want 3", first.Counters.SourceFramesCreated)
	}
	if first.Counters.SourceTransmissions != 3 {
		t.Fatalf("source transmissions = %d, want 3", first.Counters.SourceTransmissions)
	}
	if first.Counters.RelayTransmissions != 0 {
		t.Fatalf("relay transmissions = %d, want 0", first.Counters.RelayTransmissions)
	}
	if first.Counters.MissedOpportunityCount != 0 || first.Counters.OutOfScheduleTxCount != 0 {
		t.Fatalf("unexpected schedule counters: %+v", first.Counters)
	}
	if !reflect.DeepEqual(first.DeliveredMemberIDs, []uint16{2, 3}) {
		t.Fatalf("delivered members = %v, want [2 3]", first.DeliveredMemberIDs)
	}
	for _, id := range []uint16{2, 3} {
		got := first.ReceiverCounters[id]
		if got.FramesAccepted != 3 || got.PlayoutFramesEmitted != 3 {
			t.Fatalf("receiver %d counters = %+v, want 3 accepted/emitted", id, got)
		}
	}
	if first.EventHash == "" || first.ResultHash == "" {
		t.Fatalf("hashes must be non-empty: event=%q result=%q", first.EventHash, first.ResultHash)
	}
	if first.EventHash != second.EventHash || first.ResultHash != second.ResultHash {
		t.Fatalf("fixed-input hashes differ: event %q/%q result %q/%q", first.EventHash, second.EventHash, first.ResultHash, second.ResultHash)
	}
}

func runIT052Direct(t *testing.T) ScheduledRuntimeResult {
	t.Helper()
	version := scheduledTestVersion(t, []routing.RelayTreeEdge{
		{ParentStationID: 1, ChildStationID: 2, Hop: 0},
		{ParentStationID: 1, ChildStationID: 3, Hop: 0},
	}, []uint16{2, 3}, nil, 0, 1, 0, 2)
	cfg := scheduledTestRuntimeConfig(version, 3)
	result, err := RunScheduledRuntime(cfg)
	if err != nil {
		t.Fatalf("RunScheduledRuntime failed: %v", err)
	}
	return result
}

func scheduledTestRuntimeConfig(version ScheduledVersion, frameCount uint32) ScheduledRuntimeConfig {
	sourceCfg := DefaultScenarioConfig()
	sourceCfg.FrameCount = int(frameCount)
	sourceCfg.FramePeriodMs = 80
	sourceCfg.StartMs = 1_000
	sourceCfg.SourceID = 1
	sourceCfg.NetID = 1
	sourceCfg.GroupID = 100
	sourceCfg.CallID = 9001
	sourceCfg.TTL = 3

	relays := make(map[uint16]*node.RelayStation)
	receivers := make(map[uint16]*node.ReceiverStation)
	for _, edge := range version.Tree.Edges {
		if scheduledTestNodeHasOutgoing(version.Tree, edge.ChildStationID) {
			if _, ok := relays[edge.ChildStationID]; !ok {
				relays[edge.ChildStationID] = node.NewRelayStation(
					edge.ChildStationID,
					node.NewRelayLifecycle(node.DefaultLifecyclePolicy()),
					node.DefaultRelayPolicy(),
				)
			}
		}
	}
	for _, id := range version.Schedule.CoveredMemberIDs {
		receivers[id] = node.NewReceiverStation(
			id,
			node.NewReceiverLifecycle(node.DefaultLifecyclePolicy()),
			node.DefaultReceiverPolicy(),
		)
	}
	return ScheduledRuntimeConfig{
		Versions:             []ScheduledVersion{version},
		Source:               NewSourceStation(sourceCfg),
		Relays:               relays,
		Receivers:            receivers,
		ForwardQueueMaxDepth: DefaultRelayForwardQueueMaxDepth,
		FrameCount:           frameCount,
	}
}

func scheduledTestVersion(
	t *testing.T,
	edges []routing.RelayTreeEdge,
	covered []uint16,
	uncovered []uint16,
	maxDepth uint8,
	version uint16,
	effectiveSeq uint32,
	validThroughSeq uint32,
) ScheduledVersion {
	t.Helper()
	tree := routing.RelayTree{
		SourceStationID:      1,
		MembershipSnapshotID: 9,
		TopologyEpoch:        7,
		MaxRelayDepth:        maxDepth,
		Edges:                append([]routing.RelayTreeEdge(nil), edges...),
		Coverage: routing.CoverageSet{
			CoveredMemberIDs:   append([]uint16(nil), covered...),
			UncoveredMemberIDs: append([]uint16(nil), uncovered...),
		},
	}
	expected := append(append([]uint16(nil), covered...), uncovered...)
	schedule, err := timing.BuildOpportunitySchedule(timing.ScheduleBinding{
		ScheduleID:             100,
		ScheduleVersion:        version,
		NetID:                  1,
		GroupID:                100,
		SourceStationID:        1,
		CallID:                 9001,
		ScheduleAuthorityID:    4,
		TopologyEpoch:          7,
		TopologyObservedAtMS:   900,
		MembershipSnapshotID:   9,
		MembershipObservedAtMS: 900,
		MembershipFreshnessMS:  1_000,
		TimingStrategy:         timing.TimingStrategySourceLed,
		TimingAuthorityID:      1,
		EffectiveSeq:           effectiveSeq,
		ValidThroughSeq:        validThroughSeq,
		ExpectedMemberIDs:      expected,
	}, tree, timing.OpportunityProfile{
		SourceDurationMS: 10,
		RelayDurationMS:  10,
		GuardAfterMS:     1,
		MaxCycleMS:       80,
	})
	if err != nil {
		t.Fatalf("BuildOpportunitySchedule failed: %v", err)
	}
	delays := make(map[ScheduledLinkKey]int64, len(edges))
	for _, edge := range edges {
		delays[ScheduledLinkKey{FromStationID: edge.ParentStationID, ToStationID: edge.ChildStationID}] = 1
	}
	return ScheduledVersion{Tree: tree, Schedule: schedule, LinkDelayMS: delays}
}

func scheduledTestNodeHasOutgoing(tree routing.RelayTree, stationID uint16) bool {
	for _, edge := range tree.Edges {
		if edge.ParentStationID == stationID {
			return true
		}
	}
	return false
}

func requireScheduledHashesEqual(t *testing.T, first, second ScheduledRuntimeResult) {
	t.Helper()
	if first.EventHash == "" || first.ResultHash == "" {
		t.Fatalf("hashes must be non-empty: event=%q result=%q", first.EventHash, first.ResultHash)
	}
	if first.EventHash != second.EventHash || first.ResultHash != second.ResultHash {
		t.Fatalf(
			"fixed-input hashes differ: event %q/%q result %q/%q",
			first.EventHash,
			second.EventHash,
			first.ResultHash,
			second.ResultHash,
		)
	}
}
