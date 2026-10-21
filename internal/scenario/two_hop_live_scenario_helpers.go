package scenario

import (
	"fmt"

	"github.com/rmsff/ad-hoc-radio/internal/channel"
	"github.com/rmsff/ad-hoc-radio/internal/node"
	"github.com/rmsff/ad-hoc-radio/internal/protocol"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
)

func normalizeTwoHopLiveScenarioConfig(cfg TwoHopLiveScenarioConfig) TwoHopLiveScenarioConfig {
	def := DefaultTwoHopLiveScenarioConfig()
	if cfg.ScenarioID == "" {
		cfg.ScenarioID = def.ScenarioID
	}
	if cfg.Seed == 0 {
		cfg.Seed = def.Seed
	}
	if cfg.FrameCount == 0 {
		cfg.FrameCount = def.FrameCount
	}
	if cfg.StartMs == 0 && !cfg.StartMsExplicit {
		cfg.StartMs = def.StartMs
	}
	if cfg.VoicePeriodMs == 0 {
		cfg.VoicePeriodMs = def.VoicePeriodMs
	}
	if cfg.SourceSlotMs == 0 {
		cfg.SourceSlotMs = def.SourceSlotMs
	}
	if cfg.RelaySlotMs == 0 {
		cfg.RelaySlotMs = def.RelaySlotMs
	}
	if cfg.ChannelBitrateBps == 0 {
		cfg.ChannelBitrateBps = def.ChannelBitrateBps
	}
	if cfg.SourceFrameBytes == 0 {
		cfg.SourceFrameBytes = def.SourceFrameBytes
	}
	if cfg.RelayFrameBytes == 0 {
		cfg.RelayFrameBytes = def.RelayFrameBytes
	}
	if cfg.SourceWireMode == "" {
		cfg.SourceWireMode = def.SourceWireMode
	}
	if cfg.LatencyBudgetMs == 0 {
		cfg.LatencyBudgetMs = def.LatencyBudgetMs
	}
	if cfg.SourceID == 0 {
		cfg.SourceID = def.SourceID
	}
	if cfg.Relay1ID == 0 {
		cfg.Relay1ID = def.Relay1ID
	}
	if cfg.Relay2ID == 0 {
		cfg.Relay2ID = def.Relay2ID
	}
	if cfg.ReceiverID == 0 {
		cfg.ReceiverID = def.ReceiverID
	}
	if cfg.NetID == 0 {
		cfg.NetID = def.NetID
	}
	if cfg.GroupID == 0 {
		cfg.GroupID = def.GroupID
	}
	if cfg.CallID == 0 {
		cfg.CallID = def.CallID
	}
	if cfg.TTL == 0 {
		cfg.TTL = def.TTL
	}
	if cfg.ReceiverJitterBufferMs == 0 {
		cfg.ReceiverJitterBufferMs = def.ReceiverJitterBufferMs
	}
	if cfg.ReceiverLateThresholdMs == 0 {
		cfg.ReceiverLateThresholdMs = def.ReceiverLateThresholdMs
	}
	return cfg
}

func validateTwoHopLiveScenarioConfig(cfg TwoHopLiveScenarioConfig) error {
	if cfg.ScenarioID == "" {
		return fmt.Errorf("two-hop live scenario: scenario_id must not be empty")
	}
	if cfg.FrameCount <= 0 {
		return fmt.Errorf("two-hop live scenario: frame_count=%d must be > 0", cfg.FrameCount)
	}
	if cfg.FrameCount > maxLimitedMultiHopTraceRows/3 {
		return fmt.Errorf("two-hop live scenario: frame_count=%d would exceed trace cap %d", cfg.FrameCount, maxLimitedMultiHopTraceRows)
	}
	if uint64(cfg.FrameCount) > uint64(^uint32(0)) {
		return fmt.Errorf("two-hop live scenario: frame_count=%d must fit uint32 sequence space", cfg.FrameCount)
	}
	if cfg.StartMs < 0 {
		return fmt.Errorf("two-hop live scenario: start_ms=%d must be >= 0", cfg.StartMs)
	}
	if cfg.VoicePeriodMs <= 0 || cfg.VoicePeriodMs > protocol.MaxFramePeriodMs {
		return fmt.Errorf("two-hop live scenario: voice_period_ms=%d must be in 1..%d", cfg.VoicePeriodMs, protocol.MaxFramePeriodMs)
	}
	if cfg.SourceSlotMs <= 0 {
		return fmt.Errorf("two-hop live scenario: source_slot_ms=%d must be > 0", cfg.SourceSlotMs)
	}
	if cfg.RelaySlotMs <= 0 {
		return fmt.Errorf("two-hop live scenario: relay_slot_ms=%d must be > 0", cfg.RelaySlotMs)
	}
	if cfg.GuardTimeMs < 0 {
		return fmt.Errorf("two-hop live scenario: guard_time_ms=%d must be >= 0", cfg.GuardTimeMs)
	}
	if cfg.LinkDelayMs < 0 {
		return fmt.Errorf("two-hop live scenario: link_delay_ms=%d must be >= 0", cfg.LinkDelayMs)
	}
	if cfg.ChannelBitrateBps <= 0 {
		return fmt.Errorf("two-hop live scenario: channel_bitrate_bps=%d must be > 0", cfg.ChannelBitrateBps)
	}
	sourcePayloadBytes, err := twoHopLiveSourcePayloadBytes(cfg)
	if err != nil {
		return err
	}
	relayFrameBytes, err := protocol.CompressedRelayHopWireSize(sourcePayloadBytes)
	if err != nil {
		return err
	}
	if cfg.RelayFrameBytes != relayFrameBytes {
		return fmt.Errorf("two-hop live scenario: relay_frame_bytes=%d must equal compressed wire size %d for source payload %d", cfg.RelayFrameBytes, relayFrameBytes, sourcePayloadBytes)
	}
	if cfg.LatencyBudgetMs <= 0 {
		return fmt.Errorf("two-hop live scenario: latency_budget_ms=%d must be > 0", cfg.LatencyBudgetMs)
	}
	if cfg.TTL < 2 {
		return fmt.Errorf("two-hop live scenario: ttl=%d must be >= 2 for two relay hops", cfg.TTL)
	}
	if cfg.ReceiverJitterBufferMs < 0 {
		return fmt.Errorf("two-hop live scenario: receiver_jitter_buffer_ms=%d must be >= 0", cfg.ReceiverJitterBufferMs)
	}
	if cfg.ReceiverLateThresholdMs < 0 {
		return fmt.Errorf("two-hop live scenario: receiver_late_threshold_ms=%d must be >= 0", cfg.ReceiverLateThresholdMs)
	}
	if err := validateDistinctTwoHopLiveScenarioIDs(cfg); err != nil {
		return err
	}
	if err := validateTwoHopLiveScenarioTimeHorizon(cfg); err != nil {
		return err
	}
	return nil
}

func twoHopLiveSourcePayloadBytes(cfg TwoHopLiveScenarioConfig) (int, error) {
	switch cfg.SourceWireMode {
	case TwoHopLiveSourceWireCanonical:
		if cfg.SourceFrameBytes <= protocol.HeaderSize {
			return 0, fmt.Errorf("two-hop live scenario: source_frame_bytes=%d must exceed canonical header size %d", cfg.SourceFrameBytes, protocol.HeaderSize)
		}
		return cfg.SourceFrameBytes - protocol.HeaderSize, nil
	case TwoHopLiveSourceWireShort:
		if cfg.SourceFrameBytes <= protocol.ShortVoiceHeaderSize {
			return 0, fmt.Errorf("two-hop live scenario: source_frame_bytes=%d must exceed short voice header size %d", cfg.SourceFrameBytes, protocol.ShortVoiceHeaderSize)
		}
		return cfg.SourceFrameBytes - protocol.ShortVoiceHeaderSize, nil
	default:
		return 0, fmt.Errorf("two-hop live scenario: source_wire_mode %q is not supported", cfg.SourceWireMode)
	}
}

func validateDistinctTwoHopLiveScenarioIDs(cfg TwoHopLiveScenarioConfig) error {
	ids := []struct {
		name string
		id   uint16
	}{
		{name: "source_id", id: cfg.SourceID},
		{name: "relay1_id", id: cfg.Relay1ID},
		{name: "relay2_id", id: cfg.Relay2ID},
		{name: "receiver_id", id: cfg.ReceiverID},
	}
	seen := make(map[uint16]string, len(ids))
	for _, item := range ids {
		if item.id == 0 {
			return fmt.Errorf("two-hop live scenario: %s must be > 0", item.name)
		}
		if previous, ok := seen[item.id]; ok {
			return fmt.Errorf("two-hop live scenario: %s=%d duplicates %s", item.name, item.id, previous)
		}
		seen[item.id] = item.name
	}
	return nil
}

func validateTwoHopLiveScenarioTimeHorizon(cfg TwoHopLiveScenarioConfig) error {
	lastSeqOffset, err := checkedLimitedMultiHopMul(int64(cfg.FrameCount-1), cfg.VoicePeriodMs)
	if err != nil {
		return fmt.Errorf("two-hop live scenario: last seq offset: %w", err)
	}
	lastCreatedAt, err := checkedLimitedMultiHopAdd(cfg.StartMs, lastSeqOffset)
	if err != nil {
		return fmt.Errorf("two-hop live scenario: last created_at: %w", err)
	}
	lastRelayTx, err := checkedLimitedMultiHopAdd(lastCreatedAt, cfg.SourceSlotMs)
	if err != nil {
		return fmt.Errorf("two-hop live scenario: last relay1 tx: %w", err)
	}
	lastRelayTx, err = checkedLimitedMultiHopAdd(lastRelayTx, cfg.RelaySlotMs)
	if err != nil {
		return fmt.Errorf("two-hop live scenario: last relay2 tx: %w", err)
	}
	relayMeasurement, err := channel.MeasureFrameBytes(cfg.RelayFrameBytes, channel.AirtimeConfig{
		ChannelBitrateBps: cfg.ChannelBitrateBps,
		SlotDurationMs:    cfg.RelaySlotMs,
	})
	if err != nil {
		return err
	}
	lastRx, err := checkedLimitedMultiHopAdd(lastRelayTx, relayMeasurement.AirtimeMs)
	if err != nil {
		return fmt.Errorf("two-hop live scenario: last receiver rx airtime: %w", err)
	}
	lastRx, err = checkedLimitedMultiHopAdd(lastRx, cfg.LinkDelayMs)
	if err != nil {
		return fmt.Errorf("two-hop live scenario: last receiver rx link delay: %w", err)
	}
	if _, err := checkedLimitedMultiHopAdd(lastRx, cfg.ReceiverJitterBufferMs); err != nil {
		return fmt.Errorf("two-hop live scenario: last playout horizon: %w", err)
	}
	return nil
}

func twoHopLiveScenarioMeasurements(cfg TwoHopLiveScenarioConfig) (channel.Measurement, channel.Measurement, error) {
	sourceMeasurement, err := channel.MeasureFrameBytes(cfg.SourceFrameBytes, channel.AirtimeConfig{
		ChannelBitrateBps: cfg.ChannelBitrateBps,
		SlotDurationMs:    cfg.SourceSlotMs,
	})
	if err != nil {
		return channel.Measurement{}, channel.Measurement{}, err
	}
	relayMeasurement, err := channel.MeasureFrameBytes(cfg.RelayFrameBytes, channel.AirtimeConfig{
		ChannelBitrateBps: cfg.ChannelBitrateBps,
		SlotDurationMs:    cfg.RelaySlotMs,
	})
	if err != nil {
		return channel.Measurement{}, channel.Measurement{}, err
	}
	return sourceMeasurement, relayMeasurement, nil
}

func twoHopLiveReceiverPolicy(cfg TwoHopLiveScenarioConfig) node.ReceiverPolicy {
	return node.ReceiverPolicy{
		StaleThresholdMs: node.DefaultReceiverStaleThresholdMs,
		JitterBufferMs:   cfg.ReceiverJitterBufferMs,
		LateThresholdMs:  cfg.ReceiverLateThresholdMs,
	}
}

func twoHopLiveCallStartFrame(cfg TwoHopLiveScenarioConfig) (protocol.Frame, error) {
	expectedCount := uint32(cfg.FrameCount)
	raw, err := protocol.EncodeCallStartPayload(protocol.CallStartPayload{
		FramePeriodMs:      cfg.VoicePeriodMs,
		PayloadMode:        "dummy_8b_counter_payload",
		ExpectedFrameCount: &expectedCount,
		SlotModeEnabled:    false,
		TTLDefault:         cfg.TTL,
	})
	if err != nil {
		return protocol.Frame{}, err
	}
	return canonicalizeFrame(protocol.Frame{
		Version:     protocol.Version,
		FrameType:   protocol.FrameTypeCallStart,
		NetID:       cfg.NetID,
		SourceID:    cfg.SourceID,
		GroupID:     cfg.GroupID,
		RelayID:     0,
		CallID:      cfg.CallID,
		Seq:         0,
		Hop:         0,
		TTL:         cfg.TTL,
		Flags:       protocol.FlagCallStart,
		CreatedAtMs: cfg.StartMs,
		SentAtMs:    cfg.StartMs,
		PayloadLen:  uint16(len(raw)),
		Payload:     raw,
	})
}

func twoHopLiveCallEndFrame(cfg TwoHopLiveScenarioConfig) (protocol.Frame, error) {
	lastSeq := uint32(cfg.FrameCount - 1)
	raw, err := protocol.EncodeCallEndPayload(protocol.CallEndPayload{
		LastSeq: lastSeq,
		Reason:  protocol.CallEndReasonNormal,
	})
	if err != nil {
		return protocol.Frame{}, err
	}
	endMs := addLimitedMultiHopMs(cfg.StartMs, mulLimitedMultiHopMs(int64(cfg.FrameCount), cfg.VoicePeriodMs))
	return canonicalizeFrame(protocol.Frame{
		Version:     protocol.Version,
		FrameType:   protocol.FrameTypeCallEnd,
		NetID:       cfg.NetID,
		SourceID:    cfg.SourceID,
		GroupID:     cfg.GroupID,
		RelayID:     0,
		CallID:      cfg.CallID,
		Seq:         0,
		Hop:         0,
		TTL:         cfg.TTL,
		Flags:       protocol.FlagCallEnd,
		CreatedAtMs: endMs,
		SentAtMs:    endMs,
		PayloadLen:  uint16(len(raw)),
		Payload:     raw,
	})
}

func twoHopLiveVoiceFrame(cfg TwoHopLiveScenarioConfig, seq uint32) (protocol.Frame, error) {
	payloadLen, err := twoHopLiveSourcePayloadBytes(cfg)
	if err != nil {
		return protocol.Frame{}, err
	}
	payload := make([]byte, payloadLen)
	payload[0] = byte(seq)
	for i := 1; i < len(payload); i++ {
		payload[i] = byte(0x40 + i)
	}
	createdAt := addLimitedMultiHopMs(cfg.StartMs, mulLimitedMultiHopMs(int64(seq), cfg.VoicePeriodMs))
	return canonicalizeFrame(protocol.Frame{
		Version:     protocol.Version,
		FrameType:   protocol.FrameTypeVoice,
		NetID:       cfg.NetID,
		SourceID:    cfg.SourceID,
		GroupID:     cfg.GroupID,
		RelayID:     0,
		CallID:      cfg.CallID,
		Seq:         seq,
		Hop:         0,
		TTL:         cfg.TTL,
		Flags:       protocol.FlagDirect,
		CreatedAtMs: createdAt,
		SentAtMs:    createdAt,
		PayloadLen:  uint16(len(payload)),
		Payload:     payload,
	})
}

func canonicalizeRelayForward(f protocol.Frame, txAt int64) (protocol.Frame, error) {
	f.SentAtMs = txAt
	f.HeaderCRC32 = 0
	f.PayloadCRC32 = 0
	return canonicalizeFrame(f)
}

func (rt *twoHopLiveRuntime) compressedContext(relayID uint16) protocol.CompressedRelayHopContext {
	return protocol.CompressedRelayHopContext{
		NetID:           rt.cfg.NetID,
		SourceID:        rt.cfg.SourceID,
		GroupID:         rt.cfg.GroupID,
		RelayID:         relayID,
		CallID:          rt.cfg.CallID,
		BaseSeq:         0,
		BaseCreatedAtMs: rt.cfg.StartMs,
		FramePeriodMs:   rt.cfg.VoicePeriodMs,
	}
}

func (rt *twoHopLiveRuntime) shortContext() protocol.ShortVoiceContext {
	return protocol.ShortVoiceContext{
		NetID:           rt.cfg.NetID,
		SourceID:        rt.cfg.SourceID,
		GroupID:         rt.cfg.GroupID,
		CallID:          rt.cfg.CallID,
		BaseSeq:         0,
		BaseCreatedAtMs: rt.cfg.StartMs,
		FramePeriodMs:   rt.cfg.VoicePeriodMs,
	}
}

func twoHopLiveOriginKey(f protocol.Frame) *sim.OriginKey {
	return &sim.OriginKey{
		NetID:    f.NetID,
		SourceID: f.SourceID,
		CallID:   f.CallID,
		Seq:      f.Seq,
	}
}

func twoHopLiveCopyKey(f protocol.Frame) *sim.CopyKey {
	return &sim.CopyKey{
		NetID:    f.NetID,
		SourceID: f.SourceID,
		RelayID:  f.RelayID,
		CallID:   f.CallID,
		Seq:      f.Seq,
		Hop:      f.Hop,
	}
}

func (rt *twoHopLiveRuntime) markTraceNodeReceive(seq uint32, hopIndex int) {
	for i := range rt.report.ForwardingTrace {
		if rt.report.ForwardingTrace[i].Seq == seq && rt.report.ForwardingTrace[i].HopIndex == hopIndex {
			rt.report.ForwardingTrace[i].ReceivedByNode = true
			return
		}
	}
}

func (rt *twoHopLiveRuntime) markTraceRelayReady(seq uint32, hopIndex int, ready bool) {
	for i := range rt.report.ForwardingTrace {
		if rt.report.ForwardingTrace[i].Seq == seq && rt.report.ForwardingTrace[i].HopIndex == hopIndex {
			rt.report.ForwardingTrace[i].RelayReady = ready
			return
		}
	}
}

func (rt *twoHopLiveRuntime) markTraceReceiverAccepted(seq uint32) {
	for i := range rt.report.ForwardingTrace {
		if rt.report.ForwardingTrace[i].Seq == seq && rt.report.ForwardingTrace[i].HopIndex == 2 {
			rt.report.ForwardingTrace[i].AcceptedByNode = true
			return
		}
	}
}

func twoHopLiveScenarioCycleOccupiedMs(cfg TwoHopLiveScenarioConfig, relayAirtimeMs int64) int64 {
	return addLimitedMultiHopMs(addLimitedMultiHopMs(cfg.SourceSlotMs, cfg.RelaySlotMs), addLimitedMultiHopMs(relayAirtimeMs, cfg.GuardTimeMs))
}

func twoHopLiveScenarioFailReasons(report TwoHopLiveScenarioReport) []string {
	reasons := make([]string, 0, 5)
	if report.SlotMissCount > 0 {
		reasons = append(reasons, "slot_miss")
	}
	if report.RelayReadinessMissCount > 0 {
		reasons = append(reasons, "relay_readiness_miss")
	}
	if report.CycleOccupiedMs > report.VoicePeriodMs {
		reasons = append(reasons, "cycle_period_exceeded")
	}
	if report.LatencyP95Ms > report.LatencyBudgetMs {
		reasons = append(reasons, "latency_budget_exceeded")
	}
	if report.ReceiverCounters.FramesAccepted != report.FrameCount {
		reasons = append(reasons, "receiver_acceptance_incomplete")
	}
	if report.ReceiverCounters.PlayoutFramesEmitted != report.FrameCount {
		reasons = append(reasons, "playout_incomplete")
	}
	return reasons
}
