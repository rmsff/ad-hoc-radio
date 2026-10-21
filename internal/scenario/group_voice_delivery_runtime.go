package scenario

import (
	"fmt"
	"sort"

	"github.com/rmsff/ad-hoc-radio/internal/protocol"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
)

func (rt *groupVoiceDeliveryRuntime) run() error {
	if rt.report.ProfileReport.Outcome != GroupVoiceOutcomePass {
		rt.report.Outcome = GroupVoiceOutcomeDegraded
		rt.report.FailReasons = groupVoiceDeliveryFailReasons(rt.report)
		return nil
	}
	if err := rt.processCallStart(); err != nil {
		return err
	}
	for seq := 0; seq < rt.cfg.FrameCount; seq++ {
		frame, err := twoHopLiveVoiceFrame(rt.twoHopCfg, uint32(seq))
		if err != nil {
			return err
		}
		rt.report.SourceFramesCreated++
		if !rt.channelBudget.SourceFitsSlot {
			rt.report.SlotMissCount++
			continue
		}
		if err := rt.scheduleSourceBroadcast(frame); err != nil {
			return err
		}
	}
	for rt.queue.Len() > 0 {
		ev, ok := rt.queue.Pop()
		if !ok {
			break
		}
		if err := rt.processDelivery(ev); err != nil {
			return err
		}
	}
	rt.evaluateReceiverJitterTimeline()
	if err := rt.processCallEnd(); err != nil {
		return err
	}
	return rt.finishReport()
}

func (rt *groupVoiceDeliveryRuntime) processCallStart() error {
	startFrame, err := twoHopLiveCallStartFrame(rt.twoHopCfg)
	if err != nil {
		return err
	}
	for _, relayID := range sortedGroupVoiceRelayIDs(rt.relays) {
		if err := rt.relays[relayID].ProcessCallStart(startFrame, rt.cfg.StartMs); err != nil {
			return fmt.Errorf("group voice delivery scenario: relay %d call_start: %w", relayID, err)
		}
	}
	for _, receiverID := range sortedGroupVoiceDeliveryReceiverIDs(rt.receivers) {
		if err := rt.receivers[receiverID].ProcessCallStart(startFrame, rt.cfg.StartMs); err != nil {
			return fmt.Errorf("group voice delivery scenario: receiver %d call_start: %w", receiverID, err)
		}
	}
	return nil
}

func (rt *groupVoiceDeliveryRuntime) processCallEnd() error {
	endFrame, err := twoHopLiveCallEndFrame(rt.twoHopCfg)
	if err != nil {
		return err
	}
	endMs := addLimitedMultiHopMs(rt.cfg.StartMs, mulLimitedMultiHopMs(int64(rt.cfg.FrameCount), rt.profile.VoicePeriodMs))
	for _, relayID := range sortedGroupVoiceRelayIDs(rt.relays) {
		if err := rt.relays[relayID].ProcessCallEnd(endFrame, endMs); err != nil {
			return fmt.Errorf("group voice delivery scenario: relay %d call_end: %w", relayID, err)
		}
	}
	for _, receiverID := range sortedGroupVoiceDeliveryReceiverIDs(rt.receivers) {
		if err := rt.receivers[receiverID].ProcessCallEnd(endFrame, endMs); err != nil {
			return fmt.Errorf("group voice delivery scenario: receiver %d call_end: %w", receiverID, err)
		}
	}
	return nil
}

func (rt *groupVoiceDeliveryRuntime) scheduleSourceBroadcast(frame protocol.Frame) error {
	raw, short, err := rt.encodeSourceVoice(frame)
	if err != nil {
		return fmt.Errorf("group voice delivery scenario: encode source seq=%d: %w", frame.Seq, err)
	}
	destinations := rt.sourceBroadcastDestinations()
	if len(destinations) == 0 {
		return nil
	}
	rt.report.SourceTransmissions++
	rxAt := addLimitedMultiHopMs(addLimitedMultiHopMs(frame.CreatedAtMs, rt.channelBudget.SourceTxDurationMs), rt.profile.LinkDelayMs)
	for _, toID := range destinations {
		rt.scheduleDelivery(rt.cfg.ProfileConfig.SourceID, toID, frame, raw, short, false, rt.shortContext(), protocol.CompressedRelayHopContext{}, rxAt)
	}
	return nil
}

func (rt *groupVoiceDeliveryRuntime) sourceBroadcastDestinations() []uint16 {
	destinationSet := make(map[uint16]struct{})
	for receiverID, path := range rt.receiverPaths {
		if len(path) == 0 {
			destinationSet[receiverID] = struct{}{}
			continue
		}
		destinationSet[path[0]] = struct{}{}
	}
	return sortedGroupVoiceIDSet(destinationSet)
}

func (rt *groupVoiceDeliveryRuntime) processDelivery(ev sim.Event) error {
	pending, ok := rt.pending[ev.EventID]
	if !ok {
		return fmt.Errorf("group voice delivery scenario: missing pending delivery for event_id=%d", ev.EventID)
	}
	delete(rt.pending, ev.EventID)

	frame, err := rt.decodeDelivery(pending)
	if err != nil {
		return err
	}
	if _, ok := rt.relays[ev.DestinationStationID]; ok {
		return rt.processRelayDelivery(ev, frame)
	}
	if _, ok := rt.receivers[ev.DestinationStationID]; ok {
		return rt.processReceiverDelivery(ev, frame)
	}
	return fmt.Errorf("group voice delivery scenario: unsupported destination station_id=%d", ev.DestinationStationID)
}

func (rt *groupVoiceDeliveryRuntime) processRelayDelivery(ev sim.Event, frame protocol.Frame) error {
	relay := rt.relays[ev.DestinationStationID]
	nextHop := int(frame.Hop) + 1
	txAt := rt.relayTxAt(frame.CreatedAtMs, nextHop)
	readyAt := addLimitedMultiHopMs(addLimitedMultiHopMs(ev.TimeMs, rt.profile.TxRxSwitchMs), rt.profile.GuardTimeMs)
	if readyAt > txAt {
		rt.report.RelayReadinessMissCount++
		return nil
	}
	result := relay.ProcessVoiceFrame(frame, ev.TimeMs)
	if result.Forwarded == nil {
		return nil
	}
	forwarded, err := canonicalizeRelayForward(*result.Forwarded, txAt)
	if err != nil {
		return err
	}
	rt.relayTxCounts[ev.DestinationStationID]++
	return rt.scheduleRelayBroadcast(forwarded, ev.DestinationStationID, txAt)
}

func (rt *groupVoiceDeliveryRuntime) scheduleRelayBroadcast(frame protocol.Frame, relayID uint16, txAt int64) error {
	if !rt.channelBudget.RelayFitsSlot {
		rt.report.SlotMissCount++
		return nil
	}
	ctx := rt.compressedContext(relayID)
	raw, err := protocol.EncodeCompressedRelayHop(frame, ctx)
	if err != nil {
		return fmt.Errorf("group voice delivery scenario: encode compressed relay-hop seq=%d from=%d: %w", frame.Seq, relayID, err)
	}
	rxAt := addLimitedMultiHopMs(addLimitedMultiHopMs(txAt, rt.channelBudget.RelayTxDurationMs), rt.profile.LinkDelayMs)
	for _, toID := range rt.relayBroadcastDestinations(relayID, int(frame.Hop)) {
		rt.scheduleDelivery(relayID, toID, frame, raw, false, true, protocol.ShortVoiceContext{}, ctx, rxAt)
	}
	return nil
}

func (rt *groupVoiceDeliveryRuntime) relayBroadcastDestinations(relayID uint16, depth int) []uint16 {
	destinationSet := make(map[uint16]struct{})
	for receiverID, path := range rt.receiverPaths {
		if len(path) < depth || path[depth-1] != relayID {
			continue
		}
		if len(path) == depth {
			destinationSet[receiverID] = struct{}{}
			continue
		}
		destinationSet[path[depth]] = struct{}{}
	}
	return sortedGroupVoiceIDSet(destinationSet)
}

func (rt *groupVoiceDeliveryRuntime) processReceiverDelivery(ev sim.Event, frame protocol.Frame) error {
	receiver := rt.receivers[ev.DestinationStationID]
	result := receiver.ProcessVoiceFrame(frame, ev.TimeMs)
	if result.Accepted != nil {
		rt.lastReceiverRxBySeq[ev.DestinationStationID][frame.Seq] = ev.TimeMs
	}
	return nil
}

func (rt *groupVoiceDeliveryRuntime) encodeSourceVoice(frame protocol.Frame) ([]byte, bool, error) {
	switch rt.profile.SourceWireMode {
	case TwoHopLiveSourceWireCanonical:
		raw, err := frame.Encode()
		return raw, false, err
	case TwoHopLiveSourceWireShort:
		raw, err := protocol.EncodeShortVoice(frame, rt.shortContext())
		return raw, true, err
	default:
		return nil, false, fmt.Errorf("group voice delivery scenario: unsupported source_wire_mode %q", rt.profile.SourceWireMode)
	}
}

func (rt *groupVoiceDeliveryRuntime) scheduleDelivery(fromID uint16, toID uint16, frame protocol.Frame, raw []byte, short bool, compressed bool, shortCtx protocol.ShortVoiceContext, compressedCtx protocol.CompressedRelayHopContext, rxAt int64) {
	ev := sim.Event{
		TimeMs:               rxAt,
		EventPriority:        sim.PriorityFrameDelivery,
		EventType:            "frame_delivery",
		SourceStationID:      fromID,
		DestinationStationID: toID,
		OriginKey:            twoHopLiveOriginKey(frame),
		CopyKey:              twoHopLiveCopyKey(frame),
		Delivery: &sim.FrameDeliveryMeta{
			PayloadLen: len(raw),
		},
	}
	rt.queue.Push(ev)
	id := rt.queue.LastPushedID()
	rawCopy := make([]byte, len(raw))
	copy(rawCopy, raw)
	rt.pending[id] = twoHopLivePendingDelivery{Raw: rawCopy, ShortWire: short, CompressedWire: compressed, ShortContext: shortCtx, CompressedContext: compressedCtx}
}

func (rt *groupVoiceDeliveryRuntime) decodeDelivery(pending twoHopLivePendingDelivery) (protocol.Frame, error) {
	if pending.ShortWire && pending.CompressedWire {
		return protocol.Frame{}, fmt.Errorf("group voice delivery scenario: delivery cannot be both short and compressed")
	}
	if pending.ShortWire {
		frame, err := protocol.DecodeShortVoice(pending.Raw, pending.ShortContext)
		if err != nil {
			return protocol.Frame{}, fmt.Errorf("group voice delivery scenario: decode short voice: %w", err)
		}
		return frame, nil
	}
	if pending.CompressedWire {
		frame, err := protocol.DecodeCompressedRelayHop(pending.Raw, pending.CompressedContext)
		if err != nil {
			return protocol.Frame{}, fmt.Errorf("group voice delivery scenario: decode compressed relay-hop: %w", err)
		}
		return frame, nil
	}
	frame, err := protocol.DecodeStrict(pending.Raw)
	if err != nil {
		return protocol.Frame{}, fmt.Errorf("group voice delivery scenario: decode canonical frame: %w", err)
	}
	return frame, nil
}

func (rt *groupVoiceDeliveryRuntime) evaluateReceiverJitterTimeline() {
	for _, receiverID := range sortedGroupVoiceDeliveryReceiverIDs(rt.receivers) {
		for seq := 0; seq < rt.cfg.FrameCount; seq++ {
			createdAt := addLimitedMultiHopMs(rt.cfg.StartMs, mulLimitedMultiHopMs(int64(seq), rt.profile.VoicePeriodMs))
			playoutAt := addLimitedMultiHopMs(createdAt, rt.cfg.ReceiverJitterBufferMs)
			if rxAt, ok := rt.lastReceiverRxBySeq[receiverID][uint32(seq)]; ok && rxAt > playoutAt {
				playoutAt = rxAt
			}
			rt.receivers[receiverID].EvaluateJitter(playoutAt)
		}
	}
}

func (rt *groupVoiceDeliveryRuntime) finishReport() error {
	hash, err := rt.queue.EventHash()
	if err != nil {
		return fmt.Errorf("group voice delivery scenario: event hash: %w", err)
	}
	rt.report.EventCount = len(rt.queue.Emitted())
	rt.report.EventHash = hash
	rt.report.RelayTransmissions = rt.relayTransmissionReports()
	for _, row := range rt.report.RelayTransmissions {
		rt.report.RelayTransmissionCount += row.FramesForwarded
	}
	rt.report.ReceiverReports = rt.receiverReports()
	rt.report.FailReasons = groupVoiceDeliveryFailReasons(rt.report)
	if len(rt.report.FailReasons) == 0 {
		rt.report.Outcome = GroupVoiceOutcomePass
	} else {
		rt.report.Outcome = GroupVoiceOutcomeFail
	}
	return nil
}

func (rt *groupVoiceDeliveryRuntime) relayTransmissionReports() []GroupVoiceRelayTransmission {
	relayIDs := sortedGroupVoiceRelayIDs(rt.relays)
	rows := make([]GroupVoiceRelayTransmission, 0, len(relayIDs))
	for _, relayID := range relayIDs {
		rows = append(rows, GroupVoiceRelayTransmission{
			RelayID:              relayID,
			FramesForwarded:      rt.relayTxCounts[relayID],
			RelayDepth:           rt.relayDepths[relayID],
			SelectedVoiceProfile: rt.profile.Name,
		})
	}
	return rows
}

func (rt *groupVoiceDeliveryRuntime) receiverReports() []GroupVoiceReceiverReport {
	receiverIDs := sortedGroupVoiceDeliveryReceiverIDs(rt.receivers)
	rows := make([]GroupVoiceReceiverReport, 0, len(receiverIDs))
	for _, receiverID := range receiverIDs {
		path := rt.receiverPaths[receiverID]
		rows = append(rows, GroupVoiceReceiverReport{
			ReceiverID:           receiverID,
			CoverageDepth:        groupVoiceReceiverPathDepth(path),
			RelayPath:            cloneGroupVoiceUint16s(path),
			SelectedVoiceProfile: rt.profile.Name,
			Counters:             rt.receivers[receiverID].Counters(),
			LastReceivedAtMs:     rt.lastReceiverRx(receiverID),
		})
	}
	return rows
}

func (rt *groupVoiceDeliveryRuntime) lastReceiverRx(receiverID uint16) int64 {
	var last int64
	for _, rxAt := range rt.lastReceiverRxBySeq[receiverID] {
		if rxAt > last {
			last = rxAt
		}
	}
	return last
}

func (rt *groupVoiceDeliveryRuntime) relayTxAt(createdAt int64, depth int) int64 {
	txAt := addLimitedMultiHopMs(createdAt, rt.profile.SourceSlotMs)
	for i := 1; i < depth; i++ {
		txAt = addLimitedMultiHopMs(txAt, rt.profile.RelaySlotMs)
	}
	return txAt
}

func (rt *groupVoiceDeliveryRuntime) shortContext() protocol.ShortVoiceContext {
	return protocol.ShortVoiceContext{
		NetID:           rt.cfg.NetID,
		SourceID:        rt.cfg.ProfileConfig.SourceID,
		GroupID:         rt.cfg.ProfileConfig.GroupID,
		CallID:          rt.cfg.CallID,
		BaseSeq:         0,
		BaseCreatedAtMs: rt.cfg.StartMs,
		FramePeriodMs:   rt.profile.VoicePeriodMs,
	}
}

func (rt *groupVoiceDeliveryRuntime) compressedContext(relayID uint16) protocol.CompressedRelayHopContext {
	return protocol.CompressedRelayHopContext{
		NetID:           rt.cfg.NetID,
		SourceID:        rt.cfg.ProfileConfig.SourceID,
		GroupID:         rt.cfg.ProfileConfig.GroupID,
		RelayID:         relayID,
		CallID:          rt.cfg.CallID,
		BaseSeq:         0,
		BaseCreatedAtMs: rt.cfg.StartMs,
		FramePeriodMs:   rt.profile.VoicePeriodMs,
	}
}

func sortedGroupVoiceIDSet(set map[uint16]struct{}) []uint16 {
	ids := make([]uint16, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
