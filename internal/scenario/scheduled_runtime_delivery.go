package scenario

import (
	"fmt"

	"github.com/rmsff/ad-hoc-radio/internal/node"
	"github.com/rmsff/ad-hoc-radio/internal/peer"
	"github.com/rmsff/ad-hoc-radio/internal/protocol"
	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
	"github.com/rmsff/ad-hoc-radio/internal/transport"
)

func (r *scheduledRuntime) scheduleSelectedEdgeDeliveries(
	version ScheduledVersion,
	parentOpportunity timing.TxOpportunity,
	frame protocol.Frame,
) error {
	raw, err := r.encodeScheduledWire(frame)
	if err != nil {
		return fmt.Errorf("scheduled runtime: encode seq=%d from station=%d: %w", frame.Seq, parentOpportunity.OwnerStationID, err)
	}
	var peerTX *peer.AirTransmission
	if r.control != nil {
		raw, err = peer.BindVoice(version.Schedule.Binding.ScheduleVersion, raw)
		if err != nil {
			return err
		}
		end, ok := checkedScheduledTime(frame.SentAtMs, uint64(parentOpportunity.DurationMS))
		if !ok {
			return fmt.Errorf("scheduled runtime: peer TX horizon")
		}
		peerTX, err = r.control.ReserveAir(parentOpportunity.OwnerStationID, frame.SentAtMs, end)
		if err != nil {
			return err
		}
	}
	key := scheduleKeyFromBinding(version.Schedule.Binding)
	transmissionIndex, err := r.reserveTransmission(parentOpportunity.OwnerStationID, frame.SentAtMs, parentOpportunity.DurationMS)
	if err != nil {
		return err
	}
	for _, edge := range version.Tree.Edges {
		if edge.ParentStationID != parentOpportunity.OwnerStationID {
			continue
		}
		linkKey := ScheduledLinkKey{FromStationID: edge.ParentStationID, ToStationID: edge.ChildStationID}
		delayMS, ok := version.LinkDelayMS[linkKey]
		if !ok {
			return fmt.Errorf("scheduled runtime: missing selected link delay %d->%d", edge.ParentStationID, edge.ChildStationID)
		}
		pending := scheduledPendingDelivery{
			PeerTransmission:  peerTX,
			SerializedAtMS:    frame.SentAtMs + int64(parentOpportunity.DurationMS),
			TransmissionIndex: transmissionIndex, HasTransmission: true,
			ScheduleKey:     key,
			ScheduleVersion: version.Schedule.Binding.ScheduleVersion,
			ParentStationID: edge.ParentStationID,
			Edge:            edge,
			RawFrame:        append([]byte(nil), raw...),
		}
		if opportunity, found := opportunityForOwner(version.Schedule, edge.ChildStationID); found {
			pending.OwnerStationID = opportunity.OwnerStationID
			pending.OpportunityIndex = opportunity.Index
		}
		arrivalMS, ok := checkedScheduledTime(
			frame.SentAtMs,
			uint64(parentOpportunity.DurationMS),
			uint64(delayMS),
		)
		if !ok {
			return fmt.Errorf(
				"scheduled runtime: delivery time overflows int64 for edge %d->%d seq=%d",
				edge.ParentStationID,
				edge.ChildStationID,
				frame.Seq,
			)
		}
		link, configured := version.Links[linkKey]
		if !configured {
			link = transport.Link{From: transport.StationID(edge.ParentStationID), To: transport.StationID(edge.ChildStationID), Enabled: true, FixedDelayMs: delayMS}
		}
		deliveries, err := transport.SamplePacket(link, raw, arrivalMS-delayMS, scheduledPacketSeed(r.cfg.Seed, frame.CallID, frame.Seq, pending.ScheduleVersion, edge.ParentStationID, edge.ChildStationID))
		if err != nil {
			return err
		}
		if len(deliveries) == 0 {
			r.linkLost[linkKey]++
		}
		var originalID uint64
		for _, delivery := range deliveries {
			if delivery.Corrupted {
				r.linkCorrupted[linkKey]++
			}
			if delivery.IsDuplicate {
				r.linkDuplicated[linkKey]++
			}
			pending.RawFrame = delivery.Raw
			event := sim.Event{
				TimeMs:               delivery.ReceivedAt,
				EventPriority:        sim.PriorityFrameDelivery,
				EventType:            scheduledEventFrameDelivery,
				SourceStationID:      edge.ParentStationID,
				DestinationStationID: edge.ChildStationID,
				OriginKey:            twoHopLiveOriginKey(frame),
				CopyKey:              twoHopLiveCopyKey(frame),
				Delivery: &sim.FrameDeliveryMeta{
					PayloadLen: len(raw),
					Corrupted:  delivery.Corrupted, IsDuplicate: delivery.IsDuplicate, OriginalEventID: originalID,
				},
			}
			r.push(event)
			r.pendingDeliveries[r.queue.LastPushedID()] = pending
			if !delivery.IsDuplicate {
				originalID = r.queue.LastPushedID()
			}
		}
	}
	return nil
}

func (r *scheduledRuntime) processScheduledDelivery(event sim.Event) error {
	pending, ok := r.pendingDeliveries[event.EventID]
	if !ok {
		return fmt.Errorf("scheduled runtime: missing delivery event_id=%d", event.EventID)
	}
	delete(r.pendingDeliveries, event.EventID)
	if pending.HasTransmission && r.transmissions[pending.TransmissionIndex].Collided || r.control != nil && r.control.AirCollided(pending.PeerTransmission) {
		r.counters.CollisionDropCount++
		r.linkLost[ScheduledLinkKey{event.SourceStationID, event.DestinationStationID}]++
		return nil
	}
	r.counters.FrameDeliveries++
	r.linkDeliveries[ScheduledLinkKey{
		FromStationID: event.SourceStationID,
		ToStationID:   event.DestinationStationID,
	}]++

	delay := event.TimeMs - pending.SerializedAtMS
	linkKey := ScheduledLinkKey{event.SourceStationID, event.DestinationStationID}
	if delay < 0 || r.linkDelaySumMS[linkKey] > (int64(^uint64(0)>>1)-delay) {
		return fmt.Errorf("scheduled runtime: sampled edge delay sum overflow")
	}
	r.linkDelaySumMS[linkKey] += delay
	if r.control != nil {
		version, raw, err := peer.UnbindVoice(pending.RawFrame)
		if err != nil {
			r.recordScheduledDrop(node.DropReasonInvalid)
			return nil
		}
		if version != pending.ScheduleVersion || !r.control.AcceptVoice(event.DestinationStationID, version, event.OriginKey.Seq, event.TimeMs) {
			r.recordScheduledDrop(node.DropReasonNotScheduled)
			return nil
		}
		pending.RawFrame = raw
	}
	frame, err := r.decodeScheduledWire(pending)
	if err != nil {
		r.recordScheduledDrop(node.DropReasonInvalid)
		return nil
	}
	version, ok := r.activeVersionForSeq(frame.Seq)
	if !ok {
		r.recordScheduledDrop(node.DropReasonScheduleExpired)
		return nil
	}
	if !deliveryAuthorized(version, pending, frame, event) {
		r.recordScheduledDrop(node.DropReasonScheduleMismatch)
		return nil
	}

	if receiver, exists := r.cfg.Receivers[event.DestinationStationID]; exists &&
		slicesContainsUint16(version.Schedule.CoveredMemberIDs, event.DestinationStationID) {
		result := receiver.ProcessVoiceFrame(frame, event.TimeMs)
		if result.Late != nil {
			r.lateByMember[event.DestinationStationID]++
		}
		if result.Dropped != nil {
			r.recordScheduledDrop(result.Dropped.Reason)
		}
		if result.Accepted != nil || result.Late != nil {
			r.latencyByDepth[pending.Edge.Hop] = append(
				r.latencyByDepth[pending.Edge.Hop],
				event.TimeMs-frame.CreatedAtMs,
			)
		}
	}

	if pending.OwnerStationID == 0 {
		return nil
	}
	opportunity, ok := opportunityByIndex(version.Schedule, pending.OpportunityIndex)
	if !ok || opportunity.OwnerStationID != pending.OwnerStationID ||
		opportunity.OwnerStationID != event.DestinationStationID ||
		opportunity.ParentStationID != pending.ParentStationID {
		r.recordScheduledDrop(node.DropReasonNotScheduled)
		return nil
	}
	opportunityAt, ok := checkedScheduledTime(frame.CreatedAtMs, uint64(opportunity.OffsetMS))
	if !ok {
		return fmt.Errorf("scheduled runtime: opportunity time overflows int64 for owner=%d seq=%d", opportunity.OwnerStationID, frame.Seq)
	}
	if r.control != nil {
		opportunityAt = r.control.TransmitAt(opportunity.OwnerStationID, opportunityAt)
	}
	if event.TimeMs > opportunityAt {
		r.recordScheduledDrop(node.DropReasonScheduleMismatch)
		return nil
	}
	relay, exists := r.cfg.Relays[opportunity.OwnerStationID]
	if !exists {
		r.recordScheduledDrop(node.DropReasonNotScheduled)
		return nil
	}
	r.counters.RelayFramesReceived++
	forward := relay.ProcessVoiceFrame(frame, event.TimeMs)
	if forward.Dropped != nil {
		r.recordScheduledDrop(forward.Dropped.Reason)
		return nil
	}
	if forward.Forwarded == nil {
		return fmt.Errorf("scheduled runtime: relay %d returned empty result for seq=%d", relay.ID(), frame.Seq)
	}
	r.counters.RelayFramesForwarded++
	if r.queueDepth[relay.ID()] >= r.cfg.ForwardQueueMaxDepth {
		r.counters.ForwardQueueDrops++
		r.recordScheduledDrop(node.DropReasonQueueFull)
		return nil
	}
	reservationKey := scheduledReservationKey{
		ScheduleKey:      pending.ScheduleKey,
		ScheduleVersion:  pending.ScheduleVersion,
		SourceSeq:        frame.Seq,
		ParentStationID:  pending.ParentStationID,
		OwnerStationID:   pending.OwnerStationID,
		OpportunityIndex: pending.OpportunityIndex,
	}
	if _, exists := r.reservations[reservationKey]; exists {
		r.recordScheduledDrop(node.DropReasonAlreadySeen)
		return nil
	}
	r.reservations[reservationKey] = scheduledReservation{Frame: *forward.Forwarded, Key: reservationKey}
	r.queueDepth[relay.ID()]++
	if r.queueDepth[relay.ID()] > r.counters.ForwardQueueMaxDepth {
		r.counters.ForwardQueueMaxDepth = r.queueDepth[relay.ID()]
	}
	return nil
}

func (r *scheduledRuntime) processScheduledRelayTransmit(event sim.Event) error {
	opportunityEvent, ok := r.opportunityEvents[event.EventID]
	if !ok {
		return fmt.Errorf("scheduled runtime: missing relay opportunity event_id=%d", event.EventID)
	}
	delete(r.opportunityEvents, event.EventID)
	if r.control != nil && !r.control.CanTransmitVersion(event.SourceStationID, opportunityEvent.ScheduleVersion, opportunityEvent.SourceSeq, event.TimeMs) {
		for key := range r.reservations {
			if key.OwnerStationID == event.SourceStationID && key.SourceSeq == opportunityEvent.SourceSeq && key.ScheduleVersion == opportunityEvent.ScheduleVersion {
				delete(r.reservations, key)
				r.queueDepth[key.OwnerStationID]--
			}
		}
		r.recordScheduledDrop(node.DropReasonNotScheduled)
		return nil
	}
	version, ok := r.activeVersionForSeq(opportunityEvent.SourceSeq)
	if !ok {
		r.recordScheduledDrop(node.DropReasonScheduleExpired)
		return nil
	}
	opportunity := opportunityEvent.Opportunity
	key := scheduledReservationKey{
		ScheduleKey:      scheduleKeyFromBinding(version.Schedule.Binding),
		ScheduleVersion:  opportunityEvent.ScheduleVersion,
		SourceSeq:        opportunityEvent.SourceSeq,
		ParentStationID:  opportunity.ParentStationID,
		OwnerStationID:   opportunity.OwnerStationID,
		OpportunityIndex: opportunity.Index,
	}
	expectedAtMS, validTime := checkedScheduledTime(r.sourceTimeForSeq(opportunityEvent.SourceSeq), uint64(opportunity.OffsetMS))
	if !validTime {
		return fmt.Errorf(
			"scheduled runtime: relay opportunity time overflows int64 for owner=%d seq=%d",
			opportunity.OwnerStationID,
			opportunityEvent.SourceSeq,
		)
	}
	if r.control != nil {
		expectedAtMS = r.control.TransmitAt(opportunity.OwnerStationID, expectedAtMS)
	}
	if version.Schedule.Binding.ScheduleVersion != opportunityEvent.ScheduleVersion ||
		!opportunityMatches(version.Schedule, opportunity) ||
		event.TimeMs != expectedAtMS ||
		event.SourceStationID != opportunity.OwnerStationID ||
		event.DestinationStationID != opportunity.OwnerStationID ||
		r.used[key] {
		r.counters.OutOfScheduleTxCount++
		return nil
	}
	r.used[key] = true
	reservation, exists := r.reservations[key]
	if !exists {
		r.counters.MissedOpportunityCount++
		r.consecutiveMissedParents[opportunity.OwnerStationID]++
		return r.maybeTriggerMissedParentReplan(opportunity.OwnerStationID, opportunityEvent.SourceSeq, event.TimeMs)
	}
	delete(r.reservations, key)
	if r.queueDepth[opportunity.OwnerStationID] > 0 {
		r.queueDepth[opportunity.OwnerStationID]--
	}
	relay := r.cfg.Relays[opportunity.OwnerStationID]
	if relay == nil {
		r.recordScheduledDrop(node.DropReasonNotScheduled)
		return nil
	}
	if event.TimeMs-reservation.Frame.CreatedAtMs > relay.StaleThresholdMs() {
		r.recordScheduledDrop(node.DropReasonStaleFrame)
		return nil
	}
	frame, err := canonicalizeRelayForward(reservation.Frame, event.TimeMs)
	if err != nil {
		return fmt.Errorf("scheduled runtime: canonicalize relay %d seq=%d: %w", opportunity.OwnerStationID, opportunityEvent.SourceSeq, err)
	}
	r.counters.RelayTransmissions++
	delete(r.consecutiveMissedParents, opportunity.OwnerStationID)
	return r.scheduleSelectedEdgeDeliveries(version, opportunity, frame)
}

func deliveryAuthorized(version ScheduledVersion, pending scheduledPendingDelivery, frame protocol.Frame, event sim.Event) bool {
	if pending.ScheduleKey != scheduleKeyFromBinding(version.Schedule.Binding) ||
		pending.ScheduleVersion != version.Schedule.Binding.ScheduleVersion ||
		pending.ParentStationID != event.SourceStationID ||
		pending.Edge.ChildStationID != event.DestinationStationID ||
		!treeContainsEdge(version.Tree, pending.Edge) {
		return false
	}
	if frame.NetID != version.Schedule.Binding.NetID || frame.GroupID != version.Schedule.Binding.GroupID ||
		frame.SourceID != version.Schedule.Binding.SourceStationID || frame.CallID != version.Schedule.Binding.CallID {
		return false
	}
	if pending.ParentStationID == version.Schedule.Binding.SourceStationID {
		return frame.RelayID == 0 && frame.Hop == 0 && frame.Flags&protocol.FlagDirect != 0
	}
	return frame.RelayID == pending.ParentStationID && frame.Hop == pending.Edge.Hop && frame.Flags&protocol.FlagRelayed != 0
}

func scheduleKeyFromBinding(binding timing.ScheduleBinding) runtimeScheduleKey {
	return runtimeScheduleKey{
		NetID:           binding.NetID,
		GroupID:         binding.GroupID,
		SourceStationID: binding.SourceStationID,
		CallID:          binding.CallID,
		ScheduleID:      binding.ScheduleID,
	}
}

func opportunityForOwner(schedule timing.OpportunitySchedule, ownerID uint16) (timing.TxOpportunity, bool) {
	for _, opportunity := range schedule.Opportunities {
		if opportunity.OwnerStationID == ownerID {
			return opportunity, true
		}
	}
	return timing.TxOpportunity{}, false
}

func opportunityByIndex(schedule timing.OpportunitySchedule, index uint16) (timing.TxOpportunity, bool) {
	if int(index) >= len(schedule.Opportunities) || schedule.Opportunities[index].Index != index {
		return timing.TxOpportunity{}, false
	}
	return schedule.Opportunities[index], true
}

func opportunityMatches(schedule timing.OpportunitySchedule, want timing.TxOpportunity) bool {
	got, ok := opportunityByIndex(schedule, want.Index)
	return ok && got == want
}

func treeContainsEdge(tree routing.RelayTree, want routing.RelayTreeEdge) bool {
	for _, edge := range tree.Edges {
		if edge == want {
			return true
		}
	}
	return false
}

func (r *scheduledRuntime) recordScheduledDrop(reason node.DropReason) {
	r.counters.DroppedByReason[reason]++
	switch reason {
	case node.DropReasonScheduleMismatch:
		r.counters.ScheduleMismatchDropCount++
	case node.DropReasonScheduleExpired:
		r.counters.ScheduleExpiredDropCount++
	case node.DropReasonNotScheduled:
		r.counters.NotScheduledDropCount++
	}
}
