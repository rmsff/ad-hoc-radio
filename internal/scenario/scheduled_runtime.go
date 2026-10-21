package scenario

import (
	"fmt"
	"sort"

	"github.com/rmsff/ad-hoc-radio/internal/node"
	"github.com/rmsff/ad-hoc-radio/internal/peer"
	"github.com/rmsff/ad-hoc-radio/internal/protocol"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

const (
	scheduledEventSourceTick    = "scheduled_source_tick"
	scheduledEventFrameDelivery = "scheduled_frame_delivery"
	scheduledEventRelayTransmit = "scheduled_relay_transmit"
	scheduledEventVersionUpdate = "scheduled_version_update"
	scheduledEventReplan        = "scheduled_replan"
	scheduledEventPlayout       = "scheduled_playout"
	scheduledEventCallEnd       = "scheduled_call_end"
	scheduledEventLocalEnd      = "scheduled_local_end"
	scheduledEventReacquire     = "scheduled_reacquire"
)

type scheduledActivation struct {
	FromControl  bool
	Noop         bool
	Version      *ScheduledVersion
	Block        bool
	EffectiveSeq uint32
}

type scheduledOpportunityEvent struct {
	ScheduleVersion uint16
	SourceSeq       uint32
	Opportunity     timing.TxOpportunity
}

type scheduledRuntime struct {
	cfg             ScheduledRuntimeConfig
	control         *peer.Controller
	callStart       protocol.Frame
	transmissions   []scheduledTransmission
	controlVersions map[uint16]ScheduledVersion
	localStarted    map[uint16]bool
	localEnded      map[uint16]bool
	audioHorizonMS  int64
	linkDelaySumMS  map[ScheduledLinkKey]int64

	queue                *sim.EventQueue
	ownedEvents          map[uint64]bool
	pendingDeliveries    map[uint64]scheduledPendingDelivery
	sourceTicks          map[uint64]uint32
	opportunityEvents    map[uint64]scheduledOpportunityEvent
	activationEvents     map[uint64]scheduledActivation
	replanEvents         map[uint64]ScheduledReplanInput
	missedReplanTemplate *ScheduledReplanInput

	activeVersions []ScheduledVersion
	reservations   map[scheduledReservationKey]scheduledReservation
	used           map[scheduledReservationKey]bool
	queueDepth     map[uint16]int
	playoutSeen    map[uint16]map[protocol.OriginKey]struct{}
	linkDeliveries map[ScheduledLinkKey]int
	linkLost       map[ScheduledLinkKey]int
	linkCorrupted  map[ScheduledLinkKey]int
	linkDuplicated map[ScheduledLinkKey]int
	latencyByDepth map[uint8][]int64
	playoutEvents  []ScheduledPlayoutEvent
	lateByMember   map[uint16]int

	counters                 ScheduledRuntimeCounters
	highestSourceSeqCreated  uint32
	hasCreatedSourceSeq      bool
	blockedFromSeq           *uint32
	consecutiveMissedParents map[uint16]int
	missedParentReplanSeq    *uint32
	nowMS                    int64
}

func RunScheduledRuntime(cfg ScheduledRuntimeConfig) (ScheduledRuntimeResult, error) {
	normalized, err := normalizeScheduledRuntimeConfig(cfg)
	if err != nil {
		return ScheduledRuntimeResult{}, err
	}
	runtime := newScheduledRuntime(normalized)
	if err := runtime.run(); err != nil {
		return ScheduledRuntimeResult{}, err
	}
	return runtime.result()
}

func newScheduledRuntime(cfg ScheduledRuntimeConfig) *scheduledRuntime {
	return &scheduledRuntime{
		cfg:                      cfg,
		controlVersions:          map[uint16]ScheduledVersion{cfg.Versions[0].Schedule.Binding.ScheduleVersion: cfg.Versions[0]},
		localStarted:             map[uint16]bool{},
		localEnded:               map[uint16]bool{},
		linkDelaySumMS:           map[ScheduledLinkKey]int64{},
		queue:                    sim.NewEventQueue(),
		ownedEvents:              map[uint64]bool{},
		pendingDeliveries:        make(map[uint64]scheduledPendingDelivery),
		sourceTicks:              make(map[uint64]uint32),
		opportunityEvents:        make(map[uint64]scheduledOpportunityEvent),
		activationEvents:         make(map[uint64]scheduledActivation),
		replanEvents:             make(map[uint64]ScheduledReplanInput),
		activeVersions:           []ScheduledVersion{cfg.Versions[0]},
		reservations:             make(map[scheduledReservationKey]scheduledReservation),
		used:                     make(map[scheduledReservationKey]bool),
		queueDepth:               make(map[uint16]int),
		playoutSeen:              make(map[uint16]map[protocol.OriginKey]struct{}),
		linkDeliveries:           make(map[ScheduledLinkKey]int),
		linkLost:                 make(map[ScheduledLinkKey]int),
		linkCorrupted:            make(map[ScheduledLinkKey]int),
		linkDuplicated:           make(map[ScheduledLinkKey]int),
		latencyByDepth:           make(map[uint8][]int64),
		lateByMember:             make(map[uint16]int),
		consecutiveMissedParents: make(map[uint16]int),
		counters: ScheduledRuntimeCounters{
			DroppedByReason: make(map[node.DropReason]int),
			ReplanByReason:  make(map[ScheduledReplanReason]int),
		},
		nowMS: cfg.Source.startMs,
	}
}

func (r *scheduledRuntime) run() error {
	if err := r.processCallStart(); err != nil {
		return err
	}
	if err := r.scheduleInitialEvents(); err != nil {
		return err
	}
	for r.queue.Len() > 0 {
		event, ok := r.queue.Pop()
		if !ok {
			break
		}
		if event.TimeMs > r.nowMS {
			r.nowMS = event.TimeMs
		}
		if err := r.processEvent(event); err != nil {
			return err
		}
	}
	return nil
}

func (r *scheduledRuntime) scheduleInitialEvents() error {
	end, err := r.cfg.Source.EmitCallEnd()
	if err != nil {
		return err
	}
	endAt := end.CreatedAtMs
	last := r.cfg.Source.EmitVoiceFrame(r.cfg.FrameCount - 1)
	for _, receiver := range r.cfg.Receivers {
		deadline, ok := checkedScheduledTime(last.CreatedAtMs, uint64(receiver.PlayoutDelayMS()))
		if !ok {
			return fmt.Errorf("scheduled runtime: playout end overflows")
		}
		if deadline > endAt {
			endAt = deadline
		}
	}
	r.audioHorizonMS = endAt
	r.push(sim.Event{TimeMs: endAt, EventPriority: sim.PriorityMetricsFlush, EventType: scheduledEventCallEnd, SourceStationID: r.cfg.Source.id})
	for _, request := range r.cfg.Reacquisitions {
		r.push(sim.Event{TimeMs: request.AtMS, EventPriority: sim.PriorityScheduleUpdate, EventType: scheduledEventReacquire, DestinationStationID: request.PeerID})
	}
	for i := uint32(0); i < r.cfg.FrameCount; i++ {
		frame := r.cfg.Source.EmitVoiceFrame(i)
		event := sim.Event{
			TimeMs:               frame.CreatedAtMs,
			EventPriority:        sim.PrioritySourceTick,
			EventType:            scheduledEventSourceTick,
			SourceStationID:      frame.SourceID,
			DestinationStationID: frame.SourceID,
			OriginKey:            twoHopLiveOriginKey(frame),
			CopyKey:              twoHopLiveCopyKey(frame),
		}
		r.push(event)
		r.sourceTicks[r.queue.LastPushedID()] = i
		for _, id := range sortedScheduledReceiverIDs(r.cfg.Receivers) {
			at, ok := checkedScheduledTime(frame.CreatedAtMs, uint64(r.cfg.Receivers[id].PlayoutDelayMS()))
			if !ok {
				return fmt.Errorf("scheduled runtime: playout deadline overflows")
			}
			r.push(sim.Event{TimeMs: at, EventPriority: sim.PriorityJitterTick, EventType: scheduledEventPlayout, SourceStationID: frame.SourceID, DestinationStationID: id, OriginKey: twoHopLiveOriginKey(frame)})
		}
	}
	for i := 1; i < len(r.cfg.Versions); i++ {
		version := cloneScheduledVersions(r.cfg.Versions[i : i+1])[0]
		at := r.cfg.Source.EmitVoiceFrame(version.Schedule.Binding.EffectiveSeq).CreatedAtMs
		r.scheduleActivation(at, scheduledActivation{
			Version:      &version,
			EffectiveSeq: version.Schedule.Binding.EffectiveSeq,
		})
	}
	for _, input := range r.cfg.ReplanInputs {
		if input.Reason == ScheduledReplanMissedParentThreshold {
			copyInput := input
			r.missedReplanTemplate = &copyInput
			continue
		}
		event := sim.Event{
			TimeMs:               input.AtMS,
			EventPriority:        sim.PriorityScheduleUpdate,
			EventType:            scheduledEventReplan,
			SourceStationID:      input.Binding.ScheduleAuthorityID,
			DestinationStationID: input.Binding.SourceStationID,
		}
		r.push(event)
		r.replanEvents[r.queue.LastPushedID()] = input
	}
	return nil
}

func (r *scheduledRuntime) processEvent(event sim.Event) error {
	delete(r.ownedEvents, event.EventID)
	switch event.EventType {
	case scheduledEventReacquire:
		return r.control.Reacquire(event.DestinationStationID, event.TimeMs)
	case scheduledEventLocalEnd:
		return r.endLocalCall(event.DestinationStationID, event.TimeMs)
	case scheduledEventCallEnd:
		return r.processCallEnd()
	case peer.EventType:
		result, err := r.control.Process(event)
		if err != nil {
			return err
		}
		if result.Activated != 0 {
			if result.Activated == r.cfg.Source.id && result.Version > 1 {
				version, ok := r.controlVersions[result.Version]
				if !ok {
					return fmt.Errorf("scheduled runtime: unknown committed control version")
				}
				r.activationEvents[event.EventID] = scheduledActivation{Version: &version, EffectiveSeq: version.Schedule.Binding.EffectiveSeq, FromControl: true}
				if err := r.processScheduledActivation(event); err != nil {
					return err
				}
			}
			return r.startLocalCall(result.Activated, event.TimeMs)
		}
		if result.Ended != 0 {
			at := event.TimeMs
			// TX expiry revokes immediately; buffered audio drains at its deadlines.
			if result.Expired && at < r.audioHorizonMS {
				at = r.audioHorizonMS
			}
			r.push(sim.Event{TimeMs: at, EventPriority: sim.PriorityMetricsFlush, EventType: scheduledEventLocalEnd, SourceStationID: r.cfg.Source.id, DestinationStationID: result.Ended})
		}
		return nil
	case scheduledEventSourceTick:
		seq, ok := r.sourceTicks[event.EventID]
		if !ok {
			return fmt.Errorf("scheduled runtime: missing source tick event_id=%d", event.EventID)
		}
		delete(r.sourceTicks, event.EventID)
		return r.processSourceTick(seq)
	case scheduledEventFrameDelivery:
		return r.processScheduledDelivery(event)
	case scheduledEventRelayTransmit:
		return r.processScheduledRelayTransmit(event)
	case scheduledEventVersionUpdate:
		return r.processScheduledActivation(event)
	case scheduledEventReplan:
		return r.processScheduledReplanEvent(event)
	case scheduledEventPlayout:
		evaluation := r.cfg.Receivers[event.DestinationStationID].EvaluatePlayoutDeadline(event.TimeMs)
		r.recordScheduledPlayout(event.DestinationStationID, evaluation.Emitted, event.TimeMs)
		return nil
	default:
		return fmt.Errorf("scheduled runtime: unsupported event_type=%q", event.EventType)
	}
}

func (r *scheduledRuntime) recordScheduledPlayout(receiverID uint16, frames []protocol.Frame, atMS int64) {
	seen := r.playoutSeen[receiverID]
	if seen == nil {
		seen = make(map[protocol.OriginKey]struct{}, len(frames))
		r.playoutSeen[receiverID] = seen
	}
	for _, frame := range frames {
		key := frame.OriginKey()
		if _, exists := seen[key]; exists {
			r.counters.DuplicatePlayoutCount++
			continue
		}
		seen[key] = struct{}{}
		r.playoutEvents = append(r.playoutEvents, ScheduledPlayoutEvent{MemberID: receiverID, Seq: frame.Seq, AtMS: atMS, LatencyMS: atMS - frame.CreatedAtMs})
	}
}

func (r *scheduledRuntime) processSourceTick(seq uint32) error {
	frame := r.cfg.Source.EmitVoiceFrame(seq)
	r.counters.SourceFramesCreated++
	r.highestSourceSeqCreated = seq
	r.hasCreatedSourceSeq = true

	version, ok := r.activeVersionForSeq(seq)
	if !ok {
		r.recordScheduledDrop(node.DropReasonScheduleExpired)
		return nil
	}
	if r.control != nil && !r.control.CanTransmitVersion(frame.SourceID, version.Schedule.Binding.ScheduleVersion, seq, frame.CreatedAtMs) {
		r.recordScheduledDrop(node.DropReasonNotScheduled)
		return nil
	}
	frame.TTL = version.Tree.MaxRelayDepth
	frame.HeaderCRC32 = 0
	frame.PayloadCRC32 = 0
	var err error
	frame, err = canonicalizeFrame(frame)
	if err != nil {
		return fmt.Errorf("scheduled runtime: canonicalize source seq=%d: %w", seq, err)
	}

	for _, opportunity := range version.Schedule.Opportunities[1:] {
		txAtMS, ok := checkedScheduledTime(frame.CreatedAtMs, uint64(opportunity.OffsetMS))
		if !ok {
			return fmt.Errorf(
				"scheduled runtime: relay opportunity time overflows int64 for owner=%d seq=%d",
				opportunity.OwnerStationID,
				seq,
			)
		}
		if r.control != nil {
			txAtMS = r.control.TransmitAt(opportunity.OwnerStationID, txAtMS)
		}
		if txAtMS < frame.CreatedAtMs {
			r.recordScheduledDrop(node.DropReasonNotScheduled)
			continue
		}
		event := sim.Event{
			TimeMs:               txAtMS,
			EventPriority:        sim.PriorityScheduledTransmit,
			EventType:            scheduledEventRelayTransmit,
			SourceStationID:      opportunity.OwnerStationID,
			DestinationStationID: opportunity.OwnerStationID,
			OriginKey:            twoHopLiveOriginKey(frame),
		}
		r.push(event)
		r.opportunityEvents[r.queue.LastPushedID()] = scheduledOpportunityEvent{
			ScheduleVersion: version.Schedule.Binding.ScheduleVersion,
			SourceSeq:       seq,
			Opportunity:     opportunity,
		}
	}

	sourceOpportunity := version.Schedule.Opportunities[0]
	if sourceOpportunity.OwnerStationID != frame.SourceID || sourceOpportunity.OffsetMS != 0 {
		r.counters.OutOfScheduleTxCount++
		return nil
	}
	r.counters.SourceTransmissions++
	return r.scheduleSelectedEdgeDeliveries(version, sourceOpportunity, frame)
}

func (r *scheduledRuntime) activeVersionForSeq(seq uint32) (ScheduledVersion, bool) {
	if r.blockedFromSeq != nil && seq >= *r.blockedFromSeq {
		return ScheduledVersion{}, false
	}
	index := -1
	for i := range r.activeVersions {
		if r.activeVersions[i].Schedule.Binding.EffectiveSeq <= seq {
			index = i
		}
	}
	if index < 0 {
		return ScheduledVersion{}, false
	}
	version := r.activeVersions[index]
	if seq > version.Schedule.Binding.ValidThroughSeq {
		return ScheduledVersion{}, false
	}
	return version, true
}

func cloneScheduledCounters(in ScheduledRuntimeCounters) ScheduledRuntimeCounters {
	out := in
	out.DroppedByReason = make(map[node.DropReason]int, len(in.DroppedByReason))
	for reason, count := range in.DroppedByReason {
		out.DroppedByReason[reason] = count
	}
	out.ReplanByReason = make(map[ScheduledReplanReason]int, len(in.ReplanByReason))
	for reason, count := range in.ReplanByReason {
		out.ReplanByReason[reason] = count
	}
	return out
}

func sortedScheduledRelayIDs(relays map[uint16]*node.RelayStation) []uint16 {
	ids := make([]uint16, 0, len(relays))
	for id := range relays {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func sortedScheduledReceiverIDs(receivers map[uint16]*node.ReceiverStation) []uint16 {
	ids := make([]uint16, 0, len(receivers))
	for id := range receivers {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func slicesContainsUint16(ids []uint16, want uint16) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func cloneUint32Pointer(value *uint32) *uint32 {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}
