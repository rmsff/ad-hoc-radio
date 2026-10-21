package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/rmsff/ad-hoc-radio/internal/protocol"
	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

type scenarioConfigDTO struct {
	ScenarioID                    string   `json:"scenario_id"`
	Seed                          *uint64  `json:"seed"`
	FrameCount                    *int     `json:"frame_count"`
	FramePeriodMs                 *int64   `json:"frame_period_ms"`
	VoicePeriodMs                 *int64   `json:"voice_period_ms"`
	StartMs                       *int64   `json:"start_ms"`
	LinkDelayMs                   *int64   `json:"link_delay_ms"`
	GuardTimeMs                   *int64   `json:"guard_time_ms"`
	ChannelBitrateBps             *int64   `json:"channel_bitrate_bps"`
	SourceSlotMs                  *int64   `json:"source_slot_ms"`
	RelaySlotMs                   *int64   `json:"relay_slot_ms"`
	SourceFrameBytes              *int     `json:"source_frame_bytes"`
	RelayFrameBytes               *int     `json:"relay_frame_bytes"`
	SourceWireMode                *string  `json:"source_wire_mode"`
	LatencyBudgetMs               *int64   `json:"latency_budget_ms"`
	SourceToRelayLossPct          *float64 `json:"source_to_relay_loss_pct"`
	RelayToReceiverLossPct        *float64 `json:"relay_to_receiver_loss_pct"`
	SourceToReceiverEnabled       *bool    `json:"source_to_receiver_enabled"`
	SourceToReceiverLossPct       *float64 `json:"source_to_receiver_loss_pct"`
	SourceToRelayDuplicatePct     *float64 `json:"source_to_relay_duplicate_pct"`
	RelayToReceiverDuplicatePct   *float64 `json:"relay_to_receiver_duplicate_pct"`
	SourceToReceiverDuplicatePct  *float64 `json:"source_to_receiver_duplicate_pct"`
	SourceToRelayCorruptionPct    *float64 `json:"source_to_relay_corruption_pct"`
	RelayToReceiverCorruptionPct  *float64 `json:"relay_to_receiver_corruption_pct"`
	SourceToReceiverCorruptionPct *float64 `json:"source_to_receiver_corruption_pct"`
	SourceToRelayJitterMs         *int64   `json:"source_to_relay_jitter_ms"`
	RelayToReceiverJitterMs       *int64   `json:"relay_to_receiver_jitter_ms"`
	SourceToReceiverJitterMs      *int64   `json:"source_to_receiver_jitter_ms"`
	ReceiverJitterBufferMs        *int64   `json:"receiver_jitter_buffer_ms"`
	ReceiverLateThresholdMs       *int64   `json:"receiver_late_threshold_ms"`
	RelaySeenCacheMaxEntries      *int     `json:"relay_seen_cache_max_entries"`
	RelayForwardQueueMaxDepth     *int     `json:"relay_forward_queue_max_depth"`
	SourceID                      *uint16  `json:"source_id"`
	RelayID                       *uint16  `json:"relay_id"`
	Relay1ID                      *uint16  `json:"relay1_id"`
	Relay2ID                      *uint16  `json:"relay2_id"`
	ReceiverID                    *uint16  `json:"receiver_id"`
	NetID                         *uint16  `json:"net_id"`
	GroupID                       *uint16  `json:"group_id"`
	VoiceGroupIDOverride          *uint16  `json:"voice_group_id_override"`
	CallID                        *uint32  `json:"call_id"`
	TTL                           *uint8   `json:"ttl"`
	SlotModeEnabled               *bool    `json:"slot_mode_enabled"`
	SlotDurationMs                *int64   `json:"slot_duration_ms"`
	AllowSlotTimingStress         *bool    `json:"allow_slot_timing_stress"`
}

type runnableScenarioConfig struct {
	ScenarioID         string
	OneHop             *scenario.ScenarioConfig
	TwoHopLive         *scenario.TwoHopLiveScenarioConfig
	ScheduledRelayTree *scenario.ScheduledRelayTreeScenarioConfig
}

func loadScenarioConfig(path string) (scenarioConfigDTO, runnableScenarioConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return scenarioConfigDTO{}, runnableScenarioConfig{}, fmt.Errorf("read scenario config %q: %w", path, err)
	}
	if scenarioID, scheduled := scheduledRelayTreeScenarioID(raw); scheduled {
		cfg, err := decodeScheduledRelayTreeScenarioConfig(raw)
		if err != nil {
			return scenarioConfigDTO{}, runnableScenarioConfig{}, fmt.Errorf("decode scenario config %q: %w", path, err)
		}
		return scenarioConfigDTO{ScenarioID: scenarioID}, runnableScenarioConfig{
			ScenarioID:         scenarioID,
			ScheduledRelayTree: &cfg,
		}, nil
	}

	var dto scenarioConfigDTO
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&dto); err != nil {
		return scenarioConfigDTO{}, runnableScenarioConfig{}, fmt.Errorf("decode scenario config %q: %w", path, err)
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return scenarioConfigDTO{}, runnableScenarioConfig{}, fmt.Errorf("decode scenario config %q: trailing JSON token", path)
		}
		return scenarioConfigDTO{}, runnableScenarioConfig{}, fmt.Errorf("decode scenario config %q: %w", path, err)
	}

	keys, err := decodeScenarioConfigKeys(raw)
	if err != nil {
		return scenarioConfigDTO{}, runnableScenarioConfig{}, fmt.Errorf("decode scenario config %q: %w", path, err)
	}
	if err := validateScenarioConfigFields(dto.ScenarioID, keys); err != nil {
		return scenarioConfigDTO{}, runnableScenarioConfig{}, fmt.Errorf("scenario config %q: %w", path, err)
	}

	cfg, err := dto.toRunnableScenarioConfig()
	if err != nil {
		return scenarioConfigDTO{}, runnableScenarioConfig{}, fmt.Errorf("scenario config %q: %w", path, err)
	}
	return dto, cfg, nil
}

func (d scenarioConfigDTO) toRunnableScenarioConfig() (runnableScenarioConfig, error) {
	if d.ScenarioID == "" {
		return runnableScenarioConfig{}, fmt.Errorf("scenario_id is required")
	}
	switch d.ScenarioID {
	case scenario.ScenarioID:
		cfg, err := d.toOneHopScenarioConfig()
		if err != nil {
			return runnableScenarioConfig{}, err
		}
		return runnableScenarioConfig{ScenarioID: d.ScenarioID, OneHop: &cfg}, nil
	case scenario.RelayForwardQueueBackpressureScenarioID:
		cfg, err := d.toRelayForwardQueueBackpressureScenarioConfig()
		if err != nil {
			return runnableScenarioConfig{}, err
		}
		return runnableScenarioConfig{ScenarioID: d.ScenarioID, OneHop: &cfg}, nil
	case scenario.TwoHopLiveShortSourceScenarioID:
		if err := d.validateTwoHopLiveExplicitFields(); err != nil {
			return runnableScenarioConfig{}, err
		}
		cfg := d.toTwoHopLiveScenarioConfig()
		return runnableScenarioConfig{ScenarioID: d.ScenarioID, TwoHopLive: &cfg}, nil
	default:
		return runnableScenarioConfig{}, fmt.Errorf("unsupported scenario_id %q", d.ScenarioID)
	}
}

func (d scenarioConfigDTO) toOneHopScenarioConfig() (scenario.ScenarioConfig, error) {
	return d.toOneHopScenarioConfigAllowingSlotMode(false)
}

func (d scenarioConfigDTO) toRelayForwardQueueBackpressureScenarioConfig() (scenario.ScenarioConfig, error) {
	cfg, err := d.toOneHopScenarioConfigAllowingSlotMode(true)
	if err != nil {
		return scenario.ScenarioConfig{}, err
	}
	if !cfg.SlotModeEnabled {
		return scenario.ScenarioConfig{}, fmt.Errorf("scenario_id %q requires slot_mode_enabled=true", d.ScenarioID)
	}
	if !cfg.AllowSlotTimingStress {
		return scenario.ScenarioConfig{}, fmt.Errorf("scenario_id %q requires allow_slot_timing_stress=true", d.ScenarioID)
	}
	if d.FramePeriodMs == nil {
		return scenario.ScenarioConfig{}, fmt.Errorf("scenario_id %q requires frame_period_ms", d.ScenarioID)
	}
	if d.SlotDurationMs == nil {
		return scenario.ScenarioConfig{}, fmt.Errorf("scenario_id %q requires slot_duration_ms", d.ScenarioID)
	}
	if d.RelayForwardQueueMaxDepth == nil {
		return scenario.ScenarioConfig{}, fmt.Errorf("scenario_id %q requires relay_forward_queue_max_depth", d.ScenarioID)
	}
	if cfg.FramePeriodMs <= 0 || cfg.FramePeriodMs > protocol.MaxFramePeriodMs {
		return scenario.ScenarioConfig{}, fmt.Errorf("scenario_id %q requires frame_period_ms in 1..%d", d.ScenarioID, protocol.MaxFramePeriodMs)
	}
	if cfg.SlotDurationMs <= 0 {
		return scenario.ScenarioConfig{}, fmt.Errorf("scenario_id %q requires slot_duration_ms > 0", d.ScenarioID)
	}
	if cfg.RelayForwardQueueMaxDepth <= 0 {
		return scenario.ScenarioConfig{}, fmt.Errorf("scenario_id %q requires relay_forward_queue_max_depth > 0", d.ScenarioID)
	}
	if cfg.FramePeriodMs/2 >= cfg.SlotDurationMs {
		return scenario.ScenarioConfig{}, fmt.Errorf("scenario_id %q requires frame_period_ms < 2*slot_duration_ms for queue stress", d.ScenarioID)
	}
	cfg.ScenarioID = scenario.RelayForwardQueueBackpressureScenarioID
	return cfg, nil
}

func (d scenarioConfigDTO) toOneHopScenarioConfigAllowingSlotMode(allowSlotMode bool) (scenario.ScenarioConfig, error) {
	cfg := scenario.DefaultScenarioConfig()
	if d.Seed != nil {
		cfg.Seed = *d.Seed
	}
	if d.FrameCount != nil {
		cfg.FrameCount = *d.FrameCount
	}
	if d.FramePeriodMs != nil {
		cfg.FramePeriodMs = *d.FramePeriodMs
	}
	if d.StartMs != nil {
		cfg.StartMs = *d.StartMs
	}
	if d.LinkDelayMs != nil {
		cfg.LinkDelayMs = *d.LinkDelayMs
	}
	if d.SourceToRelayLossPct != nil {
		cfg.SourceToRelayLossPct = *d.SourceToRelayLossPct
	}
	if d.RelayToReceiverLossPct != nil {
		cfg.RelayToReceiverLossPct = *d.RelayToReceiverLossPct
	}
	if d.SourceToReceiverEnabled != nil {
		cfg.SourceToReceiverEnabled = *d.SourceToReceiverEnabled
	}
	if d.SourceToReceiverLossPct != nil {
		cfg.SourceToReceiverLossPct = *d.SourceToReceiverLossPct
	}
	if d.SourceToRelayDuplicatePct != nil {
		cfg.SourceToRelayDuplicatePct = *d.SourceToRelayDuplicatePct
	}
	if d.RelayToReceiverDuplicatePct != nil {
		cfg.RelayToReceiverDuplicatePct = *d.RelayToReceiverDuplicatePct
	}
	if d.SourceToReceiverDuplicatePct != nil {
		cfg.SourceToReceiverDuplicatePct = *d.SourceToReceiverDuplicatePct
	}
	if d.SourceToRelayCorruptionPct != nil {
		cfg.SourceToRelayCorruptionPct = *d.SourceToRelayCorruptionPct
	}
	if d.RelayToReceiverCorruptionPct != nil {
		cfg.RelayToReceiverCorruptionPct = *d.RelayToReceiverCorruptionPct
	}
	if d.SourceToReceiverCorruptionPct != nil {
		cfg.SourceToReceiverCorruptionPct = *d.SourceToReceiverCorruptionPct
	}
	if d.SourceToRelayJitterMs != nil {
		cfg.SourceToRelayJitterMs = *d.SourceToRelayJitterMs
	}
	if d.RelayToReceiverJitterMs != nil {
		cfg.RelayToReceiverJitterMs = *d.RelayToReceiverJitterMs
	}
	if d.SourceToReceiverJitterMs != nil {
		cfg.SourceToReceiverJitterMs = *d.SourceToReceiverJitterMs
	}
	if d.ReceiverJitterBufferMs != nil {
		cfg.ReceiverJitterBufferMs = *d.ReceiverJitterBufferMs
	}
	if d.ReceiverLateThresholdMs != nil {
		cfg.ReceiverLateThresholdMs = *d.ReceiverLateThresholdMs
	}
	if d.RelaySeenCacheMaxEntries != nil {
		cfg.RelaySeenCacheMaxEntries = *d.RelaySeenCacheMaxEntries
	}
	if d.RelayForwardQueueMaxDepth != nil {
		cfg.RelayForwardQueueMaxDepth = *d.RelayForwardQueueMaxDepth
	}
	if d.SourceID != nil {
		cfg.SourceID = *d.SourceID
	}
	if d.RelayID != nil {
		cfg.RelayID = *d.RelayID
	}
	if d.ReceiverID != nil {
		cfg.ReceiverID = *d.ReceiverID
	}
	if d.NetID != nil {
		cfg.NetID = *d.NetID
	}
	if d.GroupID != nil {
		cfg.GroupID = *d.GroupID
	}
	if d.VoiceGroupIDOverride != nil {
		cfg.VoiceGroupIDOverride = *d.VoiceGroupIDOverride
	}
	if d.CallID != nil {
		cfg.CallID = *d.CallID
	}
	if d.TTL != nil {
		cfg.TTL = *d.TTL
	}
	if d.SlotModeEnabled != nil {
		cfg.SlotModeEnabled = *d.SlotModeEnabled
	}
	if d.SlotDurationMs != nil {
		cfg.SlotDurationMs = *d.SlotDurationMs
	}
	if d.AllowSlotTimingStress != nil {
		cfg.AllowSlotTimingStress = *d.AllowSlotTimingStress
	}
	if cfg.SlotModeEnabled && !allowSlotMode {
		return scenario.ScenarioConfig{}, fmt.Errorf("scenario_id %q does not support slot_mode_enabled=true", d.ScenarioID)
	}
	return cfg, nil
}

func (d scenarioConfigDTO) toTwoHopLiveScenarioConfig() scenario.TwoHopLiveScenarioConfig {
	cfg := scenario.DefaultTwoHopLiveShortSourceScenarioConfig()
	if d.Seed != nil {
		cfg.Seed = *d.Seed
	}
	if d.FrameCount != nil {
		cfg.FrameCount = *d.FrameCount
	}
	if d.StartMs != nil {
		cfg.StartMs = *d.StartMs
		cfg.StartMsExplicit = true
	}
	if d.VoicePeriodMs != nil {
		cfg.VoicePeriodMs = *d.VoicePeriodMs
	}
	if d.SourceSlotMs != nil {
		cfg.SourceSlotMs = *d.SourceSlotMs
	}
	if d.RelaySlotMs != nil {
		cfg.RelaySlotMs = *d.RelaySlotMs
	}
	if d.GuardTimeMs != nil {
		cfg.GuardTimeMs = *d.GuardTimeMs
	}
	if d.LinkDelayMs != nil {
		cfg.LinkDelayMs = *d.LinkDelayMs
	}
	if d.ChannelBitrateBps != nil {
		cfg.ChannelBitrateBps = *d.ChannelBitrateBps
	}
	if d.SourceFrameBytes != nil {
		cfg.SourceFrameBytes = *d.SourceFrameBytes
	}
	if d.RelayFrameBytes != nil {
		cfg.RelayFrameBytes = *d.RelayFrameBytes
	}
	if d.SourceWireMode != nil {
		cfg.SourceWireMode = *d.SourceWireMode
	}
	if d.LatencyBudgetMs != nil {
		cfg.LatencyBudgetMs = *d.LatencyBudgetMs
	}
	if d.SourceID != nil {
		cfg.SourceID = *d.SourceID
	}
	if d.Relay1ID != nil {
		cfg.Relay1ID = *d.Relay1ID
	}
	if d.Relay2ID != nil {
		cfg.Relay2ID = *d.Relay2ID
	}
	if d.ReceiverID != nil {
		cfg.ReceiverID = *d.ReceiverID
	}
	if d.NetID != nil {
		cfg.NetID = *d.NetID
	}
	if d.GroupID != nil {
		cfg.GroupID = *d.GroupID
	}
	if d.CallID != nil {
		cfg.CallID = *d.CallID
	}
	if d.TTL != nil {
		cfg.TTL = *d.TTL
	}
	if d.ReceiverJitterBufferMs != nil {
		cfg.ReceiverJitterBufferMs = *d.ReceiverJitterBufferMs
	}
	if d.ReceiverLateThresholdMs != nil {
		cfg.ReceiverLateThresholdMs = *d.ReceiverLateThresholdMs
	}
	return cfg
}

func (d scenarioConfigDTO) validateTwoHopLiveExplicitFields() error {
	if d.Seed != nil && *d.Seed == 0 {
		return fmt.Errorf("seed must be > 0 when explicitly set")
	}
	if err := requirePositiveInt("frame_count", d.FrameCount); err != nil {
		return err
	}
	if err := requirePositiveInt64("voice_period_ms", d.VoicePeriodMs); err != nil {
		return err
	}
	if err := requireNonNegativeInt64("start_ms", d.StartMs); err != nil {
		return err
	}
	if err := requirePositiveInt64("source_slot_ms", d.SourceSlotMs); err != nil {
		return err
	}
	if err := requirePositiveInt64("relay_slot_ms", d.RelaySlotMs); err != nil {
		return err
	}
	if err := requireNonNegativeInt64("guard_time_ms", d.GuardTimeMs); err != nil {
		return err
	}
	if err := requireNonNegativeInt64("link_delay_ms", d.LinkDelayMs); err != nil {
		return err
	}
	if err := requirePositiveInt64("channel_bitrate_bps", d.ChannelBitrateBps); err != nil {
		return err
	}
	if err := requirePositiveInt("source_frame_bytes", d.SourceFrameBytes); err != nil {
		return err
	}
	if err := requirePositiveInt("relay_frame_bytes", d.RelayFrameBytes); err != nil {
		return err
	}
	if err := requirePositiveInt64("latency_budget_ms", d.LatencyBudgetMs); err != nil {
		return err
	}
	if err := requirePositiveInt64("receiver_jitter_buffer_ms", d.ReceiverJitterBufferMs); err != nil {
		return err
	}
	if err := requirePositiveInt64("receiver_late_threshold_ms", d.ReceiverLateThresholdMs); err != nil {
		return err
	}
	if err := requirePositiveUint16("source_id", d.SourceID); err != nil {
		return err
	}
	if err := requirePositiveUint16("relay1_id", d.Relay1ID); err != nil {
		return err
	}
	if err := requirePositiveUint16("relay2_id", d.Relay2ID); err != nil {
		return err
	}
	if err := requirePositiveUint16("receiver_id", d.ReceiverID); err != nil {
		return err
	}
	if err := requirePositiveUint16("net_id", d.NetID); err != nil {
		return err
	}
	if err := requirePositiveUint16("group_id", d.GroupID); err != nil {
		return err
	}
	if d.CallID != nil && *d.CallID == 0 {
		return fmt.Errorf("call_id must be > 0 when explicitly set")
	}
	if d.TTL != nil && *d.TTL < 2 {
		return fmt.Errorf("ttl must be >= 2 when explicitly set")
	}
	return nil
}
