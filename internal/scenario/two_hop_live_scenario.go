package scenario

import (
	"fmt"

	"github.com/rmsff/ad-hoc-radio/internal/channel"
	"github.com/rmsff/ad-hoc-radio/internal/node"
	"github.com/rmsff/ad-hoc-radio/internal/protocol"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
)

const TwoHopLiveScenarioID = "two_hop_live_full_scenario"
const TwoHopLiveShortSourceScenarioID = "two_hop_live_short_source_full_scenario"

const (
	TwoHopLiveSourceWireCanonical = "canonical"
	TwoHopLiveSourceWireShort     = "short"
)

const (
	defaultTwoHopLiveScenarioFrameCount       = defaultLimitedMultiHopFrameCount
	defaultTwoHopLiveTTL                uint8 = 2
)

type TwoHopLiveScenarioConfig struct {
	ScenarioID              string
	Seed                    uint64
	FrameCount              int
	StartMs                 int64
	StartMsExplicit         bool
	VoicePeriodMs           int64
	SourceSlotMs            int64
	RelaySlotMs             int64
	GuardTimeMs             int64
	LinkDelayMs             int64
	ChannelBitrateBps       int64
	SourceFrameBytes        int
	RelayFrameBytes         int
	SourceWireMode          string
	LatencyBudgetMs         int64
	SourceID                uint16
	Relay1ID                uint16
	Relay2ID                uint16
	ReceiverID              uint16
	NetID                   uint16
	GroupID                 uint16
	CallID                  uint32
	TTL                     uint8
	ReceiverJitterBufferMs  int64
	ReceiverLateThresholdMs int64
}

type TwoHopLiveScenarioReport struct {
	ScenarioID              string
	Seed                    uint64
	FrameCount              int
	SourceID                uint16
	RelayPath               []uint16
	ReceiverID              uint16
	StartMs                 int64
	VoicePeriodMs           int64
	SourceSlotMs            int64
	RelaySlotMs             int64
	GuardTimeMs             int64
	LinkDelayMs             int64
	ChannelBitrateBps       int64
	SourceFrameBytes        int
	RelayFrameBytes         int
	SourceWireMode          string
	SourceMeasurement       channel.Measurement
	RelayMeasurement        channel.Measurement
	SourceFramesCreated     int
	Relay1FramesForwarded   int
	Relay2FramesForwarded   int
	ReceiverCounters        node.ReceiverCounters
	SlotMissCount           int
	RelayReadinessMissCount int
	CycleOccupiedMs         int64
	CycleSpareMs            int64
	EndToEndLatencyMs       int64
	LatencyP95Ms            int64
	LatencyBudgetMs         int64
	ForwardingTrace         []TwoHopLiveForward
	EventCount              int
	EventHash               string
	Decision                string
	FailReasons             []string
}

type TwoHopLiveForward struct {
	Seq             uint32
	HopIndex        int
	FromID          uint16
	ToID            uint16
	TxAtMs          int64
	RxAtMs          int64
	WireBytes       int
	AirtimeMs       int64
	ShortWire       bool
	CompressedWire  bool
	RelayReady      bool
	ReceivedByNode  bool
	ForwardedByNode bool
	AcceptedByNode  bool
}

type twoHopLivePendingDelivery struct {
	Raw               []byte
	ShortWire         bool
	CompressedWire    bool
	ShortContext      protocol.ShortVoiceContext
	CompressedContext protocol.CompressedRelayHopContext
}

type twoHopLiveRuntime struct {
	cfg                 TwoHopLiveScenarioConfig
	queue               *sim.EventQueue
	pending             map[uint64]twoHopLivePendingDelivery
	relay1              *node.RelayStation
	relay2              *node.RelayStation
	receiver            *node.ReceiverStation
	sourceMeasurement   channel.Measurement
	relayMeasurement    channel.Measurement
	report              TwoHopLiveScenarioReport
	lastReceiverRxBySeq map[uint32]int64
}

func DefaultTwoHopLiveScenarioConfig() TwoHopLiveScenarioConfig {
	return TwoHopLiveScenarioConfig{
		ScenarioID:              TwoHopLiveScenarioID,
		Seed:                    DefaultScenarioSeed,
		FrameCount:              defaultTwoHopLiveScenarioFrameCount,
		StartMs:                 DefaultStartMs,
		VoicePeriodMs:           defaultTwoHopLiveVoicePeriodMs,
		SourceSlotMs:            30,
		RelaySlotMs:             20,
		GuardTimeMs:             5,
		LinkDelayMs:             1,
		ChannelBitrateBps:       19200,
		SourceFrameBytes:        defaultTwoHopLiveSourceFrameBytes,
		RelayFrameBytes:         defaultTwoHopLiveRelayFrameBytes,
		SourceWireMode:          TwoHopLiveSourceWireCanonical,
		LatencyBudgetMs:         defaultLimitedMultiHopLatencyMs,
		SourceID:                DefaultSourceID,
		Relay1ID:                DefaultRelayID,
		Relay2ID:                defaultLimitedMultiHopRelay2ID,
		ReceiverID:              defaultLimitedMultiHopReceiverID,
		NetID:                   DefaultNetID,
		GroupID:                 DefaultGroupID,
		CallID:                  DefaultCallID,
		TTL:                     defaultTwoHopLiveTTL,
		ReceiverJitterBufferMs:  node.DefaultReceiverJitterBufferMs,
		ReceiverLateThresholdMs: node.DefaultReceiverLateThresholdMs,
	}
}

func DefaultTwoHopLiveShortSourceScenarioConfig() TwoHopLiveScenarioConfig {
	cfg := DefaultTwoHopLiveScenarioConfig()
	cfg.ScenarioID = TwoHopLiveShortSourceScenarioID
	cfg.SourceFrameBytes = defaultTwoHopLiveShortSourceFrameBytes
	cfg.RelayFrameBytes = defaultTwoHopLiveRelayFrameBytes
	cfg.SourceWireMode = TwoHopLiveSourceWireShort
	cfg.SourceSlotMs = 12
	cfg.RelaySlotMs = 12
	cfg.GuardTimeMs = 1
	return cfg
}

func RunTwoHopLiveScenario(cfg TwoHopLiveScenarioConfig) (TwoHopLiveScenarioReport, error) {
	cfg = normalizeTwoHopLiveScenarioConfig(cfg)
	if err := validateTwoHopLiveScenarioConfig(cfg); err != nil {
		return TwoHopLiveScenarioReport{}, err
	}

	sourceMeasurement, relayMeasurement, err := twoHopLiveScenarioMeasurements(cfg)
	if err != nil {
		return TwoHopLiveScenarioReport{}, err
	}

	rt := &twoHopLiveRuntime{
		cfg:                 cfg,
		queue:               sim.NewEventQueue(),
		pending:             make(map[uint64]twoHopLivePendingDelivery),
		relay1:              node.NewRelayStation(cfg.Relay1ID, node.NewRelayLifecycle(node.DefaultLifecyclePolicy()), node.DefaultRelayPolicy()),
		relay2:              node.NewRelayStation(cfg.Relay2ID, node.NewRelayLifecycle(node.DefaultLifecyclePolicy()), node.DefaultRelayPolicy()),
		receiver:            node.NewReceiverStation(cfg.ReceiverID, node.NewReceiverLifecycle(node.DefaultLifecyclePolicy()), twoHopLiveReceiverPolicy(cfg)),
		sourceMeasurement:   sourceMeasurement,
		relayMeasurement:    relayMeasurement,
		lastReceiverRxBySeq: make(map[uint32]int64),
		report: TwoHopLiveScenarioReport{
			ScenarioID:        cfg.ScenarioID,
			Seed:              cfg.Seed,
			FrameCount:        cfg.FrameCount,
			SourceID:          cfg.SourceID,
			RelayPath:         []uint16{cfg.Relay1ID, cfg.Relay2ID},
			ReceiverID:        cfg.ReceiverID,
			StartMs:           cfg.StartMs,
			VoicePeriodMs:     cfg.VoicePeriodMs,
			SourceSlotMs:      cfg.SourceSlotMs,
			RelaySlotMs:       cfg.RelaySlotMs,
			GuardTimeMs:       cfg.GuardTimeMs,
			LinkDelayMs:       cfg.LinkDelayMs,
			ChannelBitrateBps: cfg.ChannelBitrateBps,
			SourceFrameBytes:  cfg.SourceFrameBytes,
			RelayFrameBytes:   cfg.RelayFrameBytes,
			SourceWireMode:    cfg.SourceWireMode,
			SourceMeasurement: sourceMeasurement,
			RelayMeasurement:  relayMeasurement,
			LatencyBudgetMs:   cfg.LatencyBudgetMs,
			ForwardingTrace:   make([]TwoHopLiveForward, 0, cfg.FrameCount*3),
		},
	}

	if err := rt.run(); err != nil {
		return TwoHopLiveScenarioReport{}, err
	}
	return rt.report, nil
}

func (rt *twoHopLiveRuntime) run() error {
	startFrame, err := twoHopLiveCallStartFrame(rt.cfg)
	if err != nil {
		return err
	}
	if err := rt.relay1.ProcessCallStart(startFrame, rt.cfg.StartMs); err != nil {
		return fmt.Errorf("two-hop live scenario: relay1 call_start: %w", err)
	}
	if err := rt.relay2.ProcessCallStart(startFrame, rt.cfg.StartMs); err != nil {
		return fmt.Errorf("two-hop live scenario: relay2 call_start: %w", err)
	}
	if err := rt.receiver.ProcessCallStart(startFrame, rt.cfg.StartMs); err != nil {
		return fmt.Errorf("two-hop live scenario: receiver call_start: %w", err)
	}

	for seq := 0; seq < rt.cfg.FrameCount; seq++ {
		frame, err := twoHopLiveVoiceFrame(rt.cfg, uint32(seq))
		if err != nil {
			return err
		}
		rt.report.SourceFramesCreated++
		txAt := frame.CreatedAtMs
		if !twoHopLiveFitsSlot(rt.sourceMeasurement.AirtimeMs, rt.cfg.SourceSlotMs, rt.cfg.GuardTimeMs) {
			rt.report.SlotMissCount++
			continue
		}
		raw, shortWire, err := rt.encodeSourceVoice(frame)
		if err != nil {
			return fmt.Errorf("two-hop live scenario: encode source seq=%d: %w", seq, err)
		}
		rxAt := addLimitedMultiHopMs(addLimitedMultiHopMs(txAt, rt.sourceMeasurement.AirtimeMs), rt.cfg.LinkDelayMs)
		rt.scheduleDelivery(rt.cfg.SourceID, rt.cfg.Relay1ID, frame, raw, shortWire, false, rt.shortContext(), protocol.CompressedRelayHopContext{}, rxAt)
		rt.report.ForwardingTrace = append(rt.report.ForwardingTrace, TwoHopLiveForward{
			Seq:            uint32(seq),
			HopIndex:       0,
			FromID:         rt.cfg.SourceID,
			ToID:           rt.cfg.Relay1ID,
			TxAtMs:         txAt,
			RxAtMs:         rxAt,
			WireBytes:      len(raw),
			AirtimeMs:      rt.sourceMeasurement.AirtimeMs,
			ShortWire:      shortWire,
			CompressedWire: false,
			RelayReady:     true,
		})
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
	endFrame, err := twoHopLiveCallEndFrame(rt.cfg)
	if err != nil {
		return err
	}
	endMs := addLimitedMultiHopMs(rt.cfg.StartMs, mulLimitedMultiHopMs(int64(rt.cfg.FrameCount), rt.cfg.VoicePeriodMs))
	if err := rt.receiver.ProcessCallEnd(endFrame, endMs); err != nil {
		return fmt.Errorf("two-hop live scenario: receiver call_end: %w", err)
	}

	hash, err := rt.queue.EventHash()
	if err != nil {
		return fmt.Errorf("two-hop live scenario: event hash: %w", err)
	}
	rt.report.EventCount = len(rt.queue.Emitted())
	rt.report.EventHash = hash
	rt.report.ReceiverCounters = rt.receiver.Counters()
	rt.report.CycleOccupiedMs = twoHopLiveScenarioCycleOccupiedMs(rt.cfg, rt.relayMeasurement.AirtimeMs)
	rt.report.CycleSpareMs = rt.cfg.VoicePeriodMs - rt.report.CycleOccupiedMs
	rt.report.EndToEndLatencyMs = addLimitedMultiHopMs(addLimitedMultiHopMs(rt.secondRelayTxAt(rt.cfg.StartMs), rt.relayMeasurement.AirtimeMs), rt.cfg.LinkDelayMs) - rt.cfg.StartMs
	rt.report.LatencyP95Ms = rt.report.EndToEndLatencyMs
	rt.report.FailReasons = twoHopLiveScenarioFailReasons(rt.report)
	if len(rt.report.FailReasons) == 0 {
		rt.report.Decision = TwoHopLiveOutcomePass
	} else {
		rt.report.Decision = TwoHopLiveOutcomeFail
	}
	return nil
}

func (rt *twoHopLiveRuntime) encodeSourceVoice(frame protocol.Frame) ([]byte, bool, error) {
	switch rt.cfg.SourceWireMode {
	case TwoHopLiveSourceWireCanonical:
		raw, err := frame.Encode()
		return raw, false, err
	case TwoHopLiveSourceWireShort:
		raw, err := protocol.EncodeShortVoice(frame, rt.shortContext())
		return raw, true, err
	default:
		return nil, false, fmt.Errorf("two-hop live scenario: unsupported source_wire_mode %q", rt.cfg.SourceWireMode)
	}
}

func (rt *twoHopLiveRuntime) processDelivery(ev sim.Event) error {
	pending, ok := rt.pending[ev.EventID]
	if !ok {
		return fmt.Errorf("two-hop live scenario: missing pending delivery for event_id=%d", ev.EventID)
	}
	delete(rt.pending, ev.EventID)

	frame, err := rt.decodeDelivery(pending)
	if err != nil {
		return err
	}

	switch {
	case ev.SourceStationID == rt.cfg.SourceID && ev.DestinationStationID == rt.cfg.Relay1ID:
		return rt.processRelay1Delivery(ev, frame)
	case ev.SourceStationID == rt.cfg.Relay1ID && ev.DestinationStationID == rt.cfg.Relay2ID:
		return rt.processRelay2Delivery(ev, frame)
	case ev.SourceStationID == rt.cfg.Relay2ID && ev.DestinationStationID == rt.cfg.ReceiverID:
		return rt.processReceiverDelivery(ev, frame)
	default:
		return fmt.Errorf("two-hop live scenario: unsupported delivery %d->%d", ev.SourceStationID, ev.DestinationStationID)
	}
}

func (rt *twoHopLiveRuntime) processRelay1Delivery(ev sim.Event, frame protocol.Frame) error {
	rt.markTraceNodeReceive(frame.Seq, 0)
	txAt := rt.relay1TxAt(frame.CreatedAtMs)
	ready := addLimitedMultiHopMs(ev.TimeMs, rt.cfg.GuardTimeMs) <= txAt
	rt.markTraceRelayReady(frame.Seq, 0, ready)
	if !ready {
		rt.report.RelayReadinessMissCount++
		return nil
	}
	result := rt.relay1.ProcessVoiceFrame(frame, ev.TimeMs)
	if result.Forwarded == nil {
		return nil
	}
	rt.report.Relay1FramesForwarded++
	forwarded, err := canonicalizeRelayForward(*result.Forwarded, txAt)
	if err != nil {
		return err
	}
	return rt.scheduleCompressedRelayHop(frame.Seq, 1, rt.cfg.Relay1ID, rt.cfg.Relay2ID, forwarded, txAt)
}

func (rt *twoHopLiveRuntime) processRelay2Delivery(ev sim.Event, frame protocol.Frame) error {
	rt.markTraceNodeReceive(frame.Seq, 1)
	txAt := rt.secondRelayTxAt(frame.CreatedAtMs)
	ready := addLimitedMultiHopMs(ev.TimeMs, rt.cfg.GuardTimeMs) <= txAt
	rt.markTraceRelayReady(frame.Seq, 1, ready)
	if !ready {
		rt.report.RelayReadinessMissCount++
		return nil
	}
	result := rt.relay2.ProcessVoiceFrame(frame, ev.TimeMs)
	if result.Forwarded == nil {
		return nil
	}
	rt.report.Relay2FramesForwarded++
	forwarded, err := canonicalizeRelayForward(*result.Forwarded, txAt)
	if err != nil {
		return err
	}
	return rt.scheduleCompressedRelayHop(frame.Seq, 2, rt.cfg.Relay2ID, rt.cfg.ReceiverID, forwarded, txAt)
}

func (rt *twoHopLiveRuntime) processReceiverDelivery(ev sim.Event, frame protocol.Frame) error {
	rt.markTraceNodeReceive(frame.Seq, 2)
	result := rt.receiver.ProcessVoiceFrame(frame, ev.TimeMs)
	if result.Accepted != nil {
		rt.markTraceReceiverAccepted(frame.Seq)
		rt.lastReceiverRxBySeq[frame.Seq] = ev.TimeMs
	}
	return nil
}

func (rt *twoHopLiveRuntime) scheduleCompressedRelayHop(seq uint32, hopIndex int, fromID uint16, toID uint16, frame protocol.Frame, txAt int64) error {
	if !twoHopLiveFitsSlot(rt.relayMeasurement.AirtimeMs, rt.cfg.RelaySlotMs, rt.cfg.GuardTimeMs) {
		rt.report.SlotMissCount++
		return nil
	}
	ctx := rt.compressedContext(fromID)
	raw, err := protocol.EncodeCompressedRelayHop(frame, ctx)
	if err != nil {
		return fmt.Errorf("two-hop live scenario: encode compressed relay-hop seq=%d from=%d: %w", seq, fromID, err)
	}
	rxAt := addLimitedMultiHopMs(addLimitedMultiHopMs(txAt, rt.relayMeasurement.AirtimeMs), rt.cfg.LinkDelayMs)
	rt.scheduleDelivery(fromID, toID, frame, raw, false, true, protocol.ShortVoiceContext{}, ctx, rxAt)
	rt.report.ForwardingTrace = append(rt.report.ForwardingTrace, TwoHopLiveForward{
		Seq:             seq,
		HopIndex:        hopIndex,
		FromID:          fromID,
		ToID:            toID,
		TxAtMs:          txAt,
		RxAtMs:          rxAt,
		WireBytes:       len(raw),
		AirtimeMs:       rt.relayMeasurement.AirtimeMs,
		CompressedWire:  true,
		RelayReady:      true,
		ForwardedByNode: true,
	})
	return nil
}

func (rt *twoHopLiveRuntime) scheduleDelivery(fromID uint16, toID uint16, frame protocol.Frame, raw []byte, short bool, compressed bool, shortCtx protocol.ShortVoiceContext, compressedCtx protocol.CompressedRelayHopContext, rxAt int64) {
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

func (rt *twoHopLiveRuntime) decodeDelivery(pending twoHopLivePendingDelivery) (protocol.Frame, error) {
	if pending.ShortWire && pending.CompressedWire {
		return protocol.Frame{}, fmt.Errorf("two-hop live scenario: delivery cannot be both short and compressed")
	}
	if pending.ShortWire {
		frame, err := protocol.DecodeShortVoice(pending.Raw, pending.ShortContext)
		if err != nil {
			return protocol.Frame{}, fmt.Errorf("two-hop live scenario: decode short voice: %w", err)
		}
		return frame, nil
	}
	if pending.CompressedWire {
		frame, err := protocol.DecodeCompressedRelayHop(pending.Raw, pending.CompressedContext)
		if err != nil {
			return protocol.Frame{}, fmt.Errorf("two-hop live scenario: decode compressed relay-hop: %w", err)
		}
		return frame, nil
	}
	frame, err := protocol.DecodeStrict(pending.Raw)
	if err != nil {
		return protocol.Frame{}, fmt.Errorf("two-hop live scenario: decode canonical frame: %w", err)
	}
	return frame, nil
}

func (rt *twoHopLiveRuntime) evaluateReceiverJitterTimeline() {
	for seq := 0; seq < rt.cfg.FrameCount; seq++ {
		createdAt := addLimitedMultiHopMs(rt.cfg.StartMs, mulLimitedMultiHopMs(int64(seq), rt.cfg.VoicePeriodMs))
		playoutAt := addLimitedMultiHopMs(createdAt, rt.cfg.ReceiverJitterBufferMs)
		if rxAt, ok := rt.lastReceiverRxBySeq[uint32(seq)]; ok && rxAt > playoutAt {
			playoutAt = rxAt
		}
		rt.receiver.EvaluateJitter(playoutAt)
	}
}

func (rt *twoHopLiveRuntime) relay1TxAt(createdAt int64) int64 {
	return addLimitedMultiHopMs(createdAt, rt.cfg.SourceSlotMs)
}

func (rt *twoHopLiveRuntime) secondRelayTxAt(createdAt int64) int64 {
	return addLimitedMultiHopMs(rt.relay1TxAt(createdAt), rt.cfg.RelaySlotMs)
}
