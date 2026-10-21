package scenario

import (
	"bytes"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/node"
	"github.com/rmsff/ad-hoc-radio/internal/protocol"
	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

func TestIT056_OpportunityQueueTTLAndOpaquePayload(t *testing.T) {
	t.Run("duplicate_playout_counter_is_receiver_scoped", func(t *testing.T) {
		runtime := &scheduledRuntime{playoutSeen: make(map[uint16]map[protocol.OriginKey]struct{})}
		frame := protocol.Frame{NetID: 1, SourceID: 1, CallID: 9001, Seq: 7}
		runtime.recordScheduledPlayout(3, []protocol.Frame{frame, frame}, 160)
		runtime.recordScheduledPlayout(4, []protocol.Frame{frame}, 160)
		if runtime.counters.DuplicatePlayoutCount != 1 {
			t.Fatalf("duplicate playout count = %d, want 1", runtime.counters.DuplicatePlayoutCount)
		}
	})

	t.Run("one_use_opaque_payload_and_duplicate_playout", func(t *testing.T) {
		version := scheduledTestVersion(t, []routing.RelayTreeEdge{
			{ParentStationID: 1, ChildStationID: 2, Hop: 0},
			{ParentStationID: 2, ChildStationID: 3, Hop: 1},
		}, []uint16{3}, nil, 1, 1, 0, 0)
		runtime := newScheduledTestRuntime(t, version, 1)
		payload := []byte{0x00, 0xff, 0x42, 0x00, 0x7e}
		frame := scheduledTestSourceFrame(t, runtime, version, 0, payload)
		pending := scheduledTestPending(t, version, frame, version.Tree.Edges[0])
		delivery := scheduledTestDeliveryEvent(frame, pending.Edge, 11)
		delivery.EventID = 200
		runtime.pendingDeliveries[delivery.EventID] = pending
		if err := runtime.processScheduledDelivery(delivery); err != nil {
			t.Fatalf("processScheduledDelivery failed: %v", err)
		}
		if len(runtime.reservations) != 1 {
			t.Fatalf("reservations = %d, want 1", len(runtime.reservations))
		}
		for _, reservation := range runtime.reservations {
			if !bytes.Equal(reservation.Frame.Payload, payload) {
				t.Fatalf("reserved payload = %x, want %x", reservation.Frame.Payload, payload)
			}
		}

		opportunity := version.Schedule.Opportunities[1]
		tx := sim.Event{
			TimeMs:               runtime.sourceTimeForSeq(0) + int64(opportunity.OffsetMS),
			EventPriority:        sim.PriorityScheduledTransmit,
			EventID:              201,
			EventType:            scheduledEventRelayTransmit,
			SourceStationID:      opportunity.OwnerStationID,
			DestinationStationID: opportunity.OwnerStationID,
		}
		opportunityEvent := scheduledOpportunityEvent{
			ScheduleVersion: version.Schedule.Binding.ScheduleVersion,
			SourceSeq:       0,
			Opportunity:     opportunity,
		}
		runtime.opportunityEvents[tx.EventID] = opportunityEvent
		if err := runtime.processScheduledRelayTransmit(tx); err != nil {
			t.Fatalf("first relay transmit failed: %v", err)
		}
		if runtime.counters.RelayTransmissions != 1 {
			t.Fatalf("relay transmissions = %d, want 1", runtime.counters.RelayTransmissions)
		}

		tx.EventID = 202
		runtime.opportunityEvents[tx.EventID] = opportunityEvent
		if err := runtime.processScheduledRelayTransmit(tx); err != nil {
			t.Fatalf("duplicate relay transmit failed: %v", err)
		}
		if runtime.counters.RelayTransmissions != 1 || runtime.counters.OutOfScheduleTxCount != 1 {
			t.Fatalf("one-use counters = %+v, want relay_tx=1 out_of_schedule=1", runtime.counters)
		}

		receiverEvent, ok := runtime.queue.Pop()
		if !ok {
			t.Fatal("relay transmission did not schedule receiver delivery")
		}
		receiverPending := runtime.pendingDeliveries[receiverEvent.EventID]
		decoded, err := protocol.DecodeStrict(receiverPending.RawFrame)
		if err != nil {
			t.Fatalf("DecodeStrict relayed frame failed: %v", err)
		}
		if !bytes.Equal(decoded.Payload, payload) {
			t.Fatalf("delivered payload = %x, want %x", decoded.Payload, payload)
		}
		if err := runtime.processScheduledDelivery(receiverEvent); err != nil {
			t.Fatalf("receiver delivery failed: %v", err)
		}
		duplicate := receiverEvent
		duplicate.EventID = 203
		runtime.pendingDeliveries[duplicate.EventID] = receiverPending
		if err := runtime.processScheduledDelivery(duplicate); err != nil {
			t.Fatalf("duplicate receiver delivery failed: %v", err)
		}
		evaluation := runtime.cfg.Receivers[3].EvaluatePlayoutDeadline(runtime.cfg.Source.startMs + 160)
		runtime.recordScheduledPlayout(3, evaluation.Emitted, runtime.cfg.Source.startMs+160)
		if err := runtime.processCallEnd(); err != nil {
			t.Fatalf("processCallEnd failed: %v", err)
		}
		counters := runtime.cfg.Receivers[3].Counters()
		if counters.FramesAccepted != 1 || counters.DuplicatesSuppressed != 1 || counters.PlayoutFramesEmitted != 1 {
			t.Fatalf("receiver counters = %+v, want accepted=1 suppressed=1 emitted=1", counters)
		}
		if runtime.counters.DuplicatePlayoutCount != 0 {
			t.Fatalf("duplicate playout count = %d, want 0", runtime.counters.DuplicatePlayoutCount)
		}
	})

	t.Run("ttl_and_hop_guards", func(t *testing.T) {
		version := scheduledTestVersion(t, []routing.RelayTreeEdge{
			{ParentStationID: 1, ChildStationID: 2, Hop: 0},
			{ParentStationID: 2, ChildStationID: 3, Hop: 1},
		}, []uint16{3}, nil, 1, 1, 0, 0)

		ttlRuntime := newScheduledTestRuntime(t, version, 1)
		ttlFrame := scheduledTestSourceFrame(t, ttlRuntime, version, 0, nil)
		ttlFrame.TTL = 0
		ttlFrame.HeaderCRC32 = 0
		ttlFrame.PayloadCRC32 = 0
		var err error
		ttlFrame, err = canonicalizeFrame(ttlFrame)
		if err != nil {
			t.Fatalf("canonicalize TTL frame failed: %v", err)
		}
		ttlPending := scheduledTestPending(t, version, ttlFrame, version.Tree.Edges[0])
		ttlEvent := scheduledTestDeliveryEvent(ttlFrame, ttlPending.Edge, 11)
		ttlEvent.EventID = 210
		ttlRuntime.pendingDeliveries[ttlEvent.EventID] = ttlPending
		if err := ttlRuntime.processScheduledDelivery(ttlEvent); err != nil {
			t.Fatalf("TTL delivery failed: %v", err)
		}
		requireScheduledDrop(t, ttlRuntime.counters, node.DropReasonTTLExpired, 1)
		if len(ttlRuntime.reservations) != 0 {
			t.Fatal("TTL-expired frame created a reservation")
		}

		hopRuntime := newScheduledTestRuntime(t, version, 1)
		hopFrame := scheduledTestSourceFrame(t, hopRuntime, version, 0, nil)
		hopFrame.RelayID = 9
		hopFrame.Hop = 1
		hopFrame.Flags = protocol.FlagRelayed
		hopFrame.HeaderCRC32 = 0
		hopFrame.PayloadCRC32 = 0
		err = nil
		hopFrame, err = canonicalizeFrame(hopFrame)
		if err != nil {
			t.Fatalf("canonicalize wrong-parent frame failed: %v", err)
		}
		hopPending := scheduledTestPending(t, version, hopFrame, version.Tree.Edges[0])
		hopEvent := scheduledTestDeliveryEvent(hopFrame, hopPending.Edge, 11)
		hopEvent.EventID = 211
		hopRuntime.pendingDeliveries[hopEvent.EventID] = hopPending
		if err := hopRuntime.processScheduledDelivery(hopEvent); err != nil {
			t.Fatalf("wrong-hop delivery failed: %v", err)
		}
		requireScheduledDrop(t, hopRuntime.counters, node.DropReasonScheduleMismatch, 1)
		if hopRuntime.cfg.Relays[2].Lifecycle().Seen(hopFrame.OriginKey()) {
			t.Fatal("wrong-hop frame poisoned relay seen-cache")
		}
	})

	t.Run("reserved_candidate_stale_at_transmit", func(t *testing.T) {
		result := runScheduledStaleReservation(t, 0)
		repeated := runScheduledStaleReservation(t, 0)
		requireScheduledDrop(t, result.Counters, node.DropReasonStaleFrame, 1)
		if result.Counters.RelayTransmissions != 0 || result.Counters.MissedOpportunityCount != 0 {
			t.Fatalf("stale reservation counters = %+v, want no relay tx/missed", result.Counters)
		}
		requireScheduledHashesEqual(t, result, repeated)
	})

	t.Run("stale_at_transmit_uses_relay_policy", func(t *testing.T) {
		result := runScheduledStaleReservation(t, 700)
		repeated := runScheduledStaleReservation(t, 700)
		if result.Counters.RelayTransmissions != 1 {
			t.Fatalf("relay transmissions = %d, want 1", result.Counters.RelayTransmissions)
		}
		requireScheduledDrop(t, result.Counters, node.DropReasonStaleFrame, 0)
		requireScheduledHashesEqual(t, result, repeated)
	})

	t.Run("bounded_queue_drops_newest_after_validation", func(t *testing.T) {
		result, cfg := runScheduledQueueBound(t)
		repeated, _ := runScheduledQueueBound(t)
		if result.Counters.ForwardQueueDrops != 3 || result.Counters.ForwardQueueMaxDepth != 1 {
			t.Fatalf("queue counters = %+v, want drops=3 max_depth=1", result.Counters)
		}
		requireScheduledDrop(t, result.Counters, node.DropReasonQueueFull, 3)
		for seq := uint32(0); seq < 4; seq++ {
			if !cfg.Relays[2].Lifecycle().Seen(cfg.Source.EmitVoiceFrame(seq).OriginKey()) {
				t.Fatalf("queue-full candidate seq=%d was not validated/seen before drop", seq)
			}
		}
		requireScheduledHashesEqual(t, result, repeated)
	})
}

func runScheduledStaleReservation(t *testing.T, staleThresholdMS int64) ScheduledRuntimeResult {
	t.Helper()
	version := scheduledTestVersion(t, []routing.RelayTreeEdge{
		{ParentStationID: 1, ChildStationID: 2, Hop: 0},
		{ParentStationID: 2, ChildStationID: 3, Hop: 1},
	}, []uint16{3}, nil, 1, 1, 0, 0)
	version = scheduledTestVersionWithProfile(t, version, timing.OpportunityProfile{
		SourceDurationMS: 10,
		RelayDurationMS:  10,
		GuardAfterMS:     600,
		MaxCycleMS:       1_300,
	})
	cfg := scheduledTestRuntimeConfig(version, 1)
	cfg.Source.framePeriodMs = 1_300
	if staleThresholdMS > 0 {
		policy := node.DefaultRelayPolicy()
		policy.StaleThresholdMs = staleThresholdMS
		cfg.Relays[2] = node.NewRelayStation(2, node.NewRelayLifecycle(node.DefaultLifecyclePolicy()), policy)
	}
	result, err := RunScheduledRuntime(cfg)
	if err != nil {
		t.Fatalf("RunScheduledRuntime failed: %v", err)
	}
	return result
}

func runScheduledQueueBound(t *testing.T) (ScheduledRuntimeResult, ScheduledRuntimeConfig) {
	t.Helper()
	version := scheduledTestVersion(t, []routing.RelayTreeEdge{
		{ParentStationID: 1, ChildStationID: 2, Hop: 0},
		{ParentStationID: 2, ChildStationID: 3, Hop: 1},
	}, []uint16{3}, nil, 1, 1, 0, 3)
	version = scheduledTestVersionWithProfile(t, version, timing.OpportunityProfile{
		SourceDurationMS: 10,
		RelayDurationMS:  10,
		GuardAfterMS:     300,
		MaxCycleMS:       700,
	})
	cfg := scheduledTestRuntimeConfig(version, 4)
	cfg.ForwardQueueMaxDepth = 1
	cfg.AllowCycleTimingStress = true
	result, err := RunScheduledRuntime(cfg)
	if err != nil {
		t.Fatalf("RunScheduledRuntime failed: %v", err)
	}
	return result, cfg
}

func scheduledTestVersionWithProfile(
	t *testing.T,
	version ScheduledVersion,
	profile timing.OpportunityProfile,
) ScheduledVersion {
	t.Helper()
	schedule, err := timing.BuildOpportunitySchedule(version.Schedule.Binding, version.Tree, profile)
	if err != nil {
		t.Fatalf("BuildOpportunitySchedule failed: %v", err)
	}
	version.Schedule = schedule
	return version
}
