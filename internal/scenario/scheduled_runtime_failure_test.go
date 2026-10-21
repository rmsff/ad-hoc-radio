package scenario

import (
	"math"
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/node"
	"github.com/rmsff/ad-hoc-radio/internal/protocol"
	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
)

func TestIT055_InitializationRejectsBindingAndInventoryMismatch(t *testing.T) {
	tests := []struct {
		name    string
		build   func(*testing.T) ScheduledRuntimeConfig
		wantErr string
	}{
		{
			name: "tree_schedule_binding",
			build: func(t *testing.T) ScheduledRuntimeConfig {
				version := scheduledTestVersion(t, []routing.RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2, Hop: 0}}, []uint16{2}, nil, 0, 1, 0, 0)
				version.Tree.TopologyEpoch++
				return scheduledTestRuntimeConfig(version, 1)
			},
			wantErr: "tree and schedule binding mismatch",
		},
		{
			name: "source_generated_binding",
			build: func(t *testing.T) ScheduledRuntimeConfig {
				version := scheduledTestVersion(t, []routing.RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2, Hop: 0}}, []uint16{2}, nil, 0, 1, 0, 0)
				version.Schedule.Binding.GroupID++
				return scheduledTestRuntimeConfig(version, 1)
			},
			wantErr: "source-generated frame binding",
		},
		{
			name: "relay_owner_inventory",
			build: func(t *testing.T) ScheduledRuntimeConfig {
				version := scheduledTestVersion(t, []routing.RelayTreeEdge{
					{ParentStationID: 1, ChildStationID: 2, Hop: 0},
					{ParentStationID: 2, ChildStationID: 3, Hop: 1},
				}, []uint16{3}, nil, 1, 1, 0, 0)
				cfg := scheduledTestRuntimeConfig(version, 1)
				delete(cfg.Relays, 2)
				return cfg
			},
			wantErr: "relay owner station_id=2 is missing",
		},
		{
			name: "covered_receiver_inventory",
			build: func(t *testing.T) ScheduledRuntimeConfig {
				version := scheduledTestVersion(t, []routing.RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2, Hop: 0}}, []uint16{2}, nil, 0, 1, 0, 0)
				cfg := scheduledTestRuntimeConfig(version, 1)
				delete(cfg.Receivers, 2)
				return cfg
			},
			wantErr: "covered receiver station_id=2 is missing",
		},
		{
			name: "directed_link_delay_inventory",
			build: func(t *testing.T) ScheduledRuntimeConfig {
				version := scheduledTestVersion(t, []routing.RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2, Hop: 0}}, []uint16{2}, nil, 0, 1, 0, 0)
				delete(version.LinkDelayMS, ScheduledLinkKey{FromStationID: 1, ToStationID: 2})
				return scheduledTestRuntimeConfig(version, 1)
			},
			wantErr: "has no link delay",
		},
		{
			name: "relay_parent_opportunity",
			build: func(t *testing.T) ScheduledRuntimeConfig {
				version := scheduledTestVersion(t, []routing.RelayTreeEdge{
					{ParentStationID: 1, ChildStationID: 2, Hop: 0},
					{ParentStationID: 2, ChildStationID: 3, Hop: 1},
				}, []uint16{3}, nil, 1, 1, 0, 0)
				version.Schedule.Opportunities[1].ParentStationID = 99
				return scheduledTestRuntimeConfig(version, 1)
			},
			wantErr: "parent_station_id=99",
		},
		{
			name: "source_time_horizon_overflow",
			build: func(t *testing.T) ScheduledRuntimeConfig {
				version := scheduledTestVersion(t, []routing.RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2, Hop: 0}}, []uint16{2}, nil, 0, 1, 0, 0)
				cfg := scheduledTestRuntimeConfig(version, 1)
				cfg.Source.startMs = math.MaxInt64 - 40
				return cfg
			},
			wantErr: "source time horizon overflows int64",
		},
		{
			name: "unexpected_relay_inventory",
			build: func(t *testing.T) ScheduledRuntimeConfig {
				version := scheduledTestVersion(t, []routing.RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2, Hop: 0}}, []uint16{2}, nil, 0, 1, 0, 0)
				cfg := scheduledTestRuntimeConfig(version, 1)
				cfg.Relays[9] = node.NewRelayStation(9, node.NewRelayLifecycle(node.DefaultLifecyclePolicy()), node.DefaultRelayPolicy())
				return cfg
			},
			wantErr: "relay inventory station_ids=[9], want []",
		},
		{
			name: "unexpected_receiver_inventory",
			build: func(t *testing.T) ScheduledRuntimeConfig {
				version := scheduledTestVersion(t, []routing.RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2, Hop: 0}}, []uint16{2}, nil, 0, 1, 0, 0)
				cfg := scheduledTestRuntimeConfig(version, 1)
				cfg.Receivers[9] = node.NewReceiverStation(9, node.NewReceiverLifecycle(node.DefaultLifecyclePolicy()), node.DefaultReceiverPolicy())
				return cfg
			},
			wantErr: "receiver inventory station_ids=[2 9], want [2]",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RunScheduledRuntime(tc.build(t))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestIT055_RejectsSelectedEdgeDeliveryTimeOverflow(t *testing.T) {
	version := scheduledTestVersion(t, []routing.RelayTreeEdge{
		{ParentStationID: 1, ChildStationID: 2, Hop: 0},
	}, []uint16{2}, nil, 0, 1, 0, 0)
	version.LinkDelayMS[ScheduledLinkKey{FromStationID: 1, ToStationID: 2}] = math.MaxInt64
	_, err := RunScheduledRuntime(scheduledTestRuntimeConfig(version, 1))
	if err == nil || !strings.Contains(err.Error(), "delivery time overflows int64") {
		t.Fatalf("error = %v, want delivery-time overflow", err)
	}
}

func TestIT055_RejectsCallCleanupTimeOverflow(t *testing.T) {
	version := scheduledTestVersion(t, []routing.RelayTreeEdge{
		{ParentStationID: 1, ChildStationID: 2, Hop: 0},
	}, []uint16{2}, nil, 0, 1, 0, 0)
	// A start near MaxInt64 is rejected at call_start (above
	// protocol.MaxSafeCreatedAtMs), so a huge cleanup grace drives the
	// end-of-call cleanup time past int64 instead.
	cfg := scheduledTestRuntimeConfig(version, 1)
	cfg.Receivers[2] = node.NewReceiverStation(2,
		node.NewReceiverLifecycle(node.LifecyclePolicy{CallEndCleanupGraceMs: math.MaxInt64 - cfg.Source.startMs}),
		node.DefaultReceiverPolicy())
	_, err := RunScheduledRuntime(cfg)
	if err == nil || !strings.Contains(err.Error(), "cleanup time overflows int64") {
		t.Fatalf("error = %v, want cleanup overflow", err)
	}

	cfg = scheduledTestRuntimeConfig(version, 1)
	cfg.Source.startMs = math.MaxInt64 - 200
	_, err = RunScheduledRuntime(cfg)
	if err == nil || !strings.Contains(err.Error(), "is outside 0..") {
		t.Fatalf("error = %v, want call_start created_at_ms range error", err)
	}
}

func TestIT055_ScheduleAuthorizationLeaseAndMissingParent(t *testing.T) {
	t.Run("authorization_precedes_relay_seen_state", func(t *testing.T) {
		version := scheduledTestVersion(t, []routing.RelayTreeEdge{
			{ParentStationID: 1, ChildStationID: 2, Hop: 0},
			{ParentStationID: 2, ChildStationID: 3, Hop: 1},
		}, []uint16{3}, nil, 1, 1, 0, 0)
		runtime := newScheduledTestRuntime(t, version, 1)
		frame := scheduledTestSourceFrame(t, runtime, version, 0, nil)
		pending := scheduledTestPending(t, version, frame, version.Tree.Edges[0])
		event := scheduledTestDeliveryEvent(frame, pending.Edge, 11)

		staleVersion := pending
		staleVersion.ScheduleVersion++
		runtime.pendingDeliveries[100] = staleVersion
		event.EventID = 100
		if err := runtime.processScheduledDelivery(event); err != nil {
			t.Fatalf("stale-version delivery failed: %v", err)
		}
		if runtime.cfg.Relays[2].Lifecycle().Seen(frame.OriginKey()) {
			t.Fatal("stale schedule version poisoned relay seen-cache")
		}

		wrongOwner := pending
		wrongOwner.OwnerStationID = 99
		runtime.pendingDeliveries[101] = wrongOwner
		event.EventID = 101
		if err := runtime.processScheduledDelivery(event); err != nil {
			t.Fatalf("wrong-owner delivery failed: %v", err)
		}
		if runtime.cfg.Relays[2].Lifecycle().Seen(frame.OriginKey()) {
			t.Fatal("wrong owner poisoned relay seen-cache")
		}

		runtime.pendingDeliveries[102] = pending
		event.EventID = 102
		if err := runtime.processScheduledDelivery(event); err != nil {
			t.Fatalf("authorized delivery failed: %v", err)
		}
		if !runtime.cfg.Relays[2].Lifecycle().Seen(frame.OriginKey()) {
			t.Fatal("authorized delivery did not reach relay state")
		}
		if len(runtime.reservations) != 1 {
			t.Fatalf("reservations = %d, want 1", len(runtime.reservations))
		}
		if runtime.counters.ScheduleMismatchDropCount != 1 || runtime.counters.NotScheduledDropCount != 1 {
			t.Fatalf("authorization counters = %+v, want one mismatch and one not_scheduled", runtime.counters)
		}
	})

	t.Run("newest_effective_version_masks_predecessor", func(t *testing.T) {
		result := runScheduledMaskedLease(t)
		repeated := runScheduledMaskedLease(t)
		if result.Counters.VersionActivationCount != 1 || result.Counters.SourceTransmissions != 2 {
			t.Fatalf("activation/source counters = %+v, want one activation and two transmissions", result.Counters)
		}
		if result.Counters.ScheduleExpiredDropCount != 1 {
			t.Fatalf("schedule expired count = %d, want 1", result.Counters.ScheduleExpiredDropCount)
		}
		requireScheduledDrop(t, result.Counters, node.DropReasonScheduleExpired, 1)
		if got := result.ReceiverCounters[2].FramesAccepted; got != 2 {
			t.Fatalf("receiver accepted = %d, want 2", got)
		}
		requireScheduledHashesEqual(t, result, repeated)
	})

	t.Run("late_parent_is_missed_without_flooding", func(t *testing.T) {
		result, cfg := runScheduledLateParent(t)
		repeated, _ := runScheduledLateParent(t)
		if result.Counters.MissedOpportunityCount != 1 || result.Counters.ScheduleMismatchDropCount != 1 {
			t.Fatalf("late-parent counters = %+v, want one missed and one mismatch", result.Counters)
		}
		if result.Counters.RelayTransmissions != 0 || len(result.DeliveredMemberIDs) != 0 {
			t.Fatalf("late parent produced forwarding: relay_tx=%d delivered=%v", result.Counters.RelayTransmissions, result.DeliveredMemberIDs)
		}
		frame := cfg.Source.EmitVoiceFrame(0)
		if cfg.Relays[2].Lifecycle().Seen(frame.OriginKey()) {
			t.Fatal("late parent poisoned relay seen-cache")
		}
		requireScheduledHashesEqual(t, result, repeated)
	})
}

func runScheduledMaskedLease(t *testing.T) ScheduledRuntimeResult {
	t.Helper()
	edges := []routing.RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2, Hop: 0}}
	first := scheduledTestVersion(t, edges, []uint16{2}, nil, 0, 1, 0, 100)
	second := scheduledTestVersion(t, edges, []uint16{2}, nil, 0, 2, 1, 1)
	cfg := scheduledTestRuntimeConfig(first, 3)
	cfg.Versions = []ScheduledVersion{first, second}
	result, err := RunScheduledRuntime(cfg)
	if err != nil {
		t.Fatalf("RunScheduledRuntime failed: %v", err)
	}
	return result
}

func runScheduledLateParent(t *testing.T) (ScheduledRuntimeResult, ScheduledRuntimeConfig) {
	t.Helper()
	version := scheduledTestVersion(t, []routing.RelayTreeEdge{
		{ParentStationID: 1, ChildStationID: 2, Hop: 0},
		{ParentStationID: 2, ChildStationID: 3, Hop: 1},
	}, []uint16{3}, nil, 1, 1, 0, 0)
	version.LinkDelayMS[ScheduledLinkKey{FromStationID: 1, ToStationID: 2}] = 2
	cfg := scheduledTestRuntimeConfig(version, 1)
	result, err := RunScheduledRuntime(cfg)
	if err != nil {
		t.Fatalf("RunScheduledRuntime failed: %v", err)
	}
	return result, cfg
}

func newScheduledTestRuntime(t *testing.T, version ScheduledVersion, frameCount uint32) *scheduledRuntime {
	t.Helper()
	cfg, err := normalizeScheduledRuntimeConfig(scheduledTestRuntimeConfig(version, frameCount))
	if err != nil {
		t.Fatalf("normalizeScheduledRuntimeConfig failed: %v", err)
	}
	runtime := newScheduledRuntime(cfg)
	if err := runtime.processCallStart(); err != nil {
		t.Fatalf("processCallStart failed: %v", err)
	}
	return runtime
}

func scheduledTestSourceFrame(
	t *testing.T,
	runtime *scheduledRuntime,
	version ScheduledVersion,
	seq uint32,
	payload []byte,
) protocol.Frame {
	t.Helper()
	frame := runtime.cfg.Source.EmitVoiceFrame(seq)
	frame.TTL = version.Tree.MaxRelayDepth
	if payload != nil {
		frame.Payload = append([]byte(nil), payload...)
		frame.PayloadLen = uint16(len(payload))
	}
	frame.HeaderCRC32 = 0
	frame.PayloadCRC32 = 0
	frame, err := canonicalizeFrame(frame)
	if err != nil {
		t.Fatalf("canonicalizeFrame failed: %v", err)
	}
	return frame
}

func scheduledTestPending(
	t *testing.T,
	version ScheduledVersion,
	frame protocol.Frame,
	edge routing.RelayTreeEdge,
) scheduledPendingDelivery {
	t.Helper()
	raw, err := frame.Encode()
	if err != nil {
		t.Fatalf("frame.Encode failed: %v", err)
	}
	pending := scheduledPendingDelivery{
		ScheduleKey:     scheduleKeyFromBinding(version.Schedule.Binding),
		ScheduleVersion: version.Schedule.Binding.ScheduleVersion,
		ParentStationID: edge.ParentStationID,
		Edge:            edge,
		RawFrame:        raw,
	}
	if opportunity, ok := opportunityForOwner(version.Schedule, edge.ChildStationID); ok {
		pending.OwnerStationID = opportunity.OwnerStationID
		pending.OpportunityIndex = opportunity.Index
	}
	return pending
}

func scheduledTestDeliveryEvent(frame protocol.Frame, edge routing.RelayTreeEdge, offsetMS int64) sim.Event {
	return sim.Event{
		TimeMs:               frame.CreatedAtMs + offsetMS,
		EventPriority:        sim.PriorityFrameDelivery,
		EventType:            scheduledEventFrameDelivery,
		SourceStationID:      edge.ParentStationID,
		DestinationStationID: edge.ChildStationID,
	}
}

func requireScheduledDrop(t *testing.T, counters ScheduledRuntimeCounters, reason node.DropReason, want int) {
	t.Helper()
	if counters.DroppedByReason[reason] != want {
		t.Fatalf("drop %s = %d, want %d", reason, counters.DroppedByReason[reason], want)
	}
}
