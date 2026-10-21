package scenario

import (
	"reflect"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/node"
	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

func TestIT057_DeterministicActivationAndBaselineReplanning(t *testing.T) {
	externalReasons := []ScheduledReplanReason{
		ScheduledReplanTopologyEpochChanged,
		ScheduledReplanMembershipSnapshotChanged,
		ScheduledReplanSelectedRelayUnavailable,
		ScheduledReplanTimingAuthorityLost,
	}
	for _, reason := range externalReasons {
		t.Run(string(reason), func(t *testing.T) {
			first := runScheduledChangedReplan(t, reason)
			second := runScheduledChangedReplan(t, reason)
			if first.Counters.RouteReplanCount != 1 || first.Counters.ReplanByReason[reason] != 1 {
				t.Fatalf("replan counters = %+v, want one %s", first.Counters, reason)
			}
			if first.Counters.VersionActivationCount != 1 {
				t.Fatalf("version activations = %d, want 1", first.Counters.VersionActivationCount)
			}
			if first.Counters.SourceFramesCreated != 4 || first.Counters.SourceTransmissions != 4 || first.Counters.RelayTransmissions != 3 {
				t.Fatalf("execution counters = %+v, want source=4 relay=3", first.Counters)
			}
			if got := first.ReceiverCounters[2]; got.FramesAccepted != 4 || got.DirectFramesAccepted != 1 || got.RelayedFramesAccepted != 3 {
				t.Fatalf("receiver counters = %+v, want one direct plus three relayed", got)
			}
			if first.EventHash != second.EventHash || first.ResultHash != second.ResultHash {
				t.Fatalf("fixed-input hashes differ: event %q/%q result %q/%q", first.EventHash, second.EventHash, first.ResultHash, second.ResultHash)
			}
		})
	}

	t.Run("unchanged_plan_retains_version", func(t *testing.T) {
		result := runScheduledUnchangedReplan(t)
		repeated := runScheduledUnchangedReplan(t)
		if result.Counters.RouteReplanCount != 1 || result.Counters.VersionActivationCount != 0 {
			t.Fatalf("unchanged-plan counters = %+v, want replan=1 activation=0", result.Counters)
		}
		if result.Counters.SourceTransmissions != 3 || result.Counters.RelayTransmissions != 0 {
			t.Fatalf("unchanged plan altered execution: %+v", result.Counters)
		}
		requireScheduledHashesEqual(t, result, repeated)
	})

	t.Run("no_feasible_blocks_future_without_fallback", func(t *testing.T) {
		result := runScheduledNoFeasible(t, false)
		repeated := runScheduledNoFeasible(t, false)
		if result.BlockedFromSeq == nil || *result.BlockedFromSeq != 1 {
			t.Fatalf("blocked_from_seq = %v, want 1", result.BlockedFromSeq)
		}
		if result.Counters.SourceFramesCreated != 3 || result.Counters.SourceTransmissions != 1 {
			t.Fatalf("source counters = %+v, want three created and one transmitted", result.Counters)
		}
		if result.Counters.ScheduleExpiredDropCount != 2 || result.Counters.RelayTransmissions != 0 {
			t.Fatalf("blocked execution counters = %+v", result.Counters)
		}
		if got := result.ReceiverCounters[2].FramesAccepted; got != 1 {
			t.Fatalf("receiver accepted = %d, want only current seq 0", got)
		}
		requireScheduledHashesEqual(t, result, repeated)
	})

	t.Run("explicit_successor_recovers_after_block", func(t *testing.T) {
		result := runScheduledNoFeasible(t, true)
		repeated := runScheduledNoFeasible(t, true)
		if result.BlockedFromSeq != nil {
			t.Fatalf("block was not cleared by explicit successor: %v", *result.BlockedFromSeq)
		}
		if result.Counters.VersionActivationCount != 1 || result.Counters.SourceTransmissions != 2 {
			t.Fatalf("recovery counters = %+v, want one activation and seq 0/2 transmissions", result.Counters)
		}
		if result.Counters.ScheduleExpiredDropCount != 1 {
			t.Fatalf("schedule expired count = %d, want only blocked seq 1", result.Counters.ScheduleExpiredDropCount)
		}
		requireScheduledHashesEqual(t, result, repeated)
	})

	t.Run("same_feasible_plan_recovers_block_with_successor", func(t *testing.T) {
		result := runScheduledSameFeasibleRecovery(t)
		repeated := runScheduledSameFeasibleRecovery(t)
		if result.BlockedFromSeq != nil {
			t.Fatalf("same feasible plan did not clear block: %v", *result.BlockedFromSeq)
		}
		if result.Counters.RouteReplanCount != 2 || result.Counters.VersionActivationCount != 1 {
			t.Fatalf("recovery counters = %+v, want two replans and one activation", result.Counters)
		}
		if result.Counters.SourceTransmissions != 3 || result.Counters.RelayTransmissions != 3 {
			t.Fatalf("recovery transmissions = %+v, want seq 0/2/3", result.Counters)
		}
		if got := result.ReceiverCounters[2].FramesAccepted; got != 3 {
			t.Fatalf("receiver accepted = %d, want 3", got)
		}
		requireScheduledHashesEqual(t, result, repeated)
	})

	t.Run("lease_end_change_activates_successor", func(t *testing.T) {
		result := runScheduledLeaseExtension(t)
		repeated := runScheduledLeaseExtension(t)
		if result.Counters.VersionActivationCount != 1 || result.Counters.SourceTransmissions != 4 {
			t.Fatalf("lease successor counters = %+v, want one activation and four transmissions", result.Counters)
		}
		if got := result.ReceiverCounters[2].FramesAccepted; got != 4 {
			t.Fatalf("receiver accepted = %d, want 4", got)
		}
		requireScheduledHashesEqual(t, result, repeated)
	})

	t.Run("two_missed_parents_trigger_future_direct_version", func(t *testing.T) {
		result := runScheduledMissedParentReplan(t)
		repeated := runScheduledMissedParentReplan(t)
		if result.Counters.MissedOpportunityCount != 2 || result.Counters.RouteReplanCount != 1 ||
			result.Counters.ReplanByReason[ScheduledReplanMissedParentThreshold] != 1 {
			t.Fatalf("missed-parent counters = %+v", result.Counters)
		}
		if result.Counters.VersionActivationCount != 1 || result.Counters.RelayTransmissions != 0 {
			t.Fatalf("missed-parent activation counters = %+v", result.Counters)
		}
		if got := result.ReceiverCounters[3]; got.FramesAccepted != 2 || got.DirectFramesAccepted != 2 {
			t.Fatalf("receiver counters = %+v, want only future seq 2/3 direct", got)
		}
		if !reflect.DeepEqual(result.DeliveredMemberIDs, []uint16{3}) {
			t.Fatalf("delivered members = %v, want [3]", result.DeliveredMemberIDs)
		}
		requireScheduledHashesEqual(t, result, repeated)
	})
}

func runScheduledChangedReplan(t *testing.T, reason ScheduledReplanReason) ScheduledRuntimeResult {
	t.Helper()
	initial := scheduledTestVersion(t, []routing.RelayTreeEdge{
		{ParentStationID: 1, ChildStationID: 2, Hop: 0},
	}, []uint16{2}, nil, 0, 1, 0, 3)
	input := scheduledTestReplanInput(
		reason,
		1_040,
		[]uint16{2},
		[][2]uint16{{1, 10}, {10, 2}},
		[]uint16{10},
		8,
		10,
		3,
		false,
	)
	cfg := scheduledTestRuntimeConfig(initial, 4)
	cfg.Relays[10] = node.NewRelayStation(10, node.NewRelayLifecycle(node.DefaultLifecyclePolicy()), node.DefaultRelayPolicy())
	cfg.ReplanInputs = []ScheduledReplanInput{input}
	result, err := RunScheduledRuntime(cfg)
	if err != nil {
		t.Fatalf("RunScheduledRuntime failed: %v", err)
	}
	return result
}

func runScheduledNoFeasible(t *testing.T, recover bool) ScheduledRuntimeResult {
	t.Helper()
	edges := []routing.RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2, Hop: 0}}
	initial := scheduledTestVersion(t, edges, []uint16{2}, nil, 0, 1, 0, 100)
	cfg := scheduledTestRuntimeConfig(initial, 3)
	if recover {
		successor := scheduledTestVersion(t, edges, []uint16{2}, nil, 0, 2, 2, 2)
		cfg.Versions = []ScheduledVersion{initial, successor}
	}
	cfg.ReplanInputs = []ScheduledReplanInput{scheduledTestReplanInput(
		ScheduledReplanTopologyEpochChanged,
		1_040,
		[]uint16{2},
		nil,
		nil,
		8,
		10,
		2,
		true,
	)}
	result, err := RunScheduledRuntime(cfg)
	if err != nil {
		t.Fatalf("RunScheduledRuntime failed: %v", err)
	}
	return result
}

func scheduledTestReplanInput(
	reason ScheduledReplanReason,
	atMS int64,
	members []uint16,
	pairs [][2]uint16,
	relayIDs []uint16,
	topologyEpoch uint32,
	snapshotID uint32,
	validThroughSeq uint32,
	requireAll bool,
) ScheduledReplanInput {
	links := make([]routing.DirectedLink, 0, len(pairs))
	for _, pair := range pairs {
		links = append(links, routing.DirectedLink{
			FromStationID: pair[0],
			ToStationID:   pair[1],
			Enabled:       true,
			Quality:       0.9,
			Loss:          0.01,
			DelayMS:       1,
			ObservedAtMS:  900,
		})
	}
	states := make([]routing.RelayState, 0, len(relayIDs))
	for _, id := range relayIDs {
		states = append(states, routing.RelayState{
			StationID:       id,
			RelayCapable:    true,
			RelayEnabled:    true,
			BatteryFraction: 0.9,
		})
	}
	request := routing.PlanRequest{
		SourceStationID: 1,
		Snapshot: routing.ExpectedGroupSnapshot{
			ID:           snapshotID,
			ObservedAtMS: 900,
			FreshnessMS:  1_000,
			MemberIDs:    append([]uint16(nil), members...),
		},
		Graph: routing.NeighborGraph{
			Epoch:       topologyEpoch,
			FreshnessMS: 1_000,
			Links:       links,
		},
		Constraints: routing.PlanningConstraints{
			PlanningAtMS:              1_000,
			MaxRelayDepth:             3,
			MaxPlayoutLatencyMS:       250,
			MaxVoiceAirtimePct:        100,
			RequireAllExpectedMembers: requireAll,
			MaxLinkLoss:               1,
		},
		CostModel: routing.PlanningCostModel{
			SourceOpportunityMS: 10,
			RelayOpportunityMS:  10,
			GuardAfterMS:        1,
			FramePeriodMS:       80,
		},
		SearchLimits: routing.PlanningSearchLimits{
			MaxPathsPerMember: 20,
			MaxTreeCandidates: 200,
			MaxFeasibleTrees:  200,
		},
		RelayStates: states,
	}
	return ScheduledReplanInput{
		AtMS:        atMS,
		Reason:      reason,
		PlanRequest: request,
		Binding: timing.ScheduleBinding{
			ScheduleID:             100,
			NetID:                  1,
			GroupID:                100,
			SourceStationID:        1,
			CallID:                 9001,
			ScheduleAuthorityID:    4,
			TopologyEpoch:          topologyEpoch,
			TopologyObservedAtMS:   900,
			MembershipSnapshotID:   snapshotID,
			MembershipObservedAtMS: 900,
			MembershipFreshnessMS:  1_000,
			TimingStrategy:         timing.TimingStrategySourceLed,
			TimingAuthorityID:      1,
			ValidThroughSeq:        validThroughSeq,
			ExpectedMemberIDs:      append([]uint16(nil), members...),
		},
		Profile: timing.OpportunityProfile{
			SourceDurationMS: 10,
			RelayDurationMS:  10,
			GuardAfterMS:     1,
			MaxCycleMS:       80,
		},
	}
}
