package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"

	"github.com/rmsff/ad-hoc-radio/internal/channel"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

const LimitedMultiHopScenarioID = "limited_multi_hop_two_relay"

const (
	LimitedMultiHopDecisionGo   = "go"
	LimitedMultiHopDecisionNoGo = "no_go"
)

const (
	LimitedMultiHopFailSlotMiss              = "slot_miss"
	LimitedMultiHopFailRelayReadinessMiss    = "relay_readiness_miss"
	LimitedMultiHopFailLatencyBudgetExceeded = "latency_budget_exceeded"
	LimitedMultiHopFailFramePeriodExceeded   = "source_frame_period_budget_exceeded"
)

const (
	defaultLimitedMultiHopRelay2ID   uint16 = 3
	defaultLimitedMultiHopReceiverID uint16 = 4
	defaultLimitedMultiHopFrameBytes        = 56
	defaultLimitedMultiHopFrameCount        = 8
	defaultLimitedMultiHopLatencyMs  int64  = 250
	maxLimitedMultiHopTraceRows             = 1_000_000
	maxLimitedMultiHopFrameBytes            = 1_000_000
)

type LimitedMultiHopExperimentConfig struct {
	ScenarioID          string
	Seed                uint64
	SourceID            uint16
	Relay1ID            uint16
	Relay2ID            uint16
	ReceiverID          uint16
	FrameCount          int
	StartMs             int64
	SlotDurationMs      int64
	LinkDelayMs         int64
	LatencyBudgetMs     int64
	FramePeriodBudgetMs int64
	FrameBytes          int
	SourceFrameBytes    int
	RelayFrameBytes     int
	ChannelBitrateBps   int64
}

type LimitedMultiHopExperimentReport struct {
	ScenarioID              string
	Seed                    uint64
	SourceID                uint16
	RelayPath               []uint16
	ReceiverID              uint16
	FrameCount              int
	HopCount                int
	SlotsPerSuperframe      int
	SuperframeDurationMs    int64
	SlotDurationMs          int64
	LinkDelayMs             int64
	LatencyBudgetMs         int64
	FramePeriodBudgetMs     int64
	SourceFramePeriodMs     int64
	EndToEndLatencyMs       int64
	LatencyP95Ms            int64
	Measurement             channel.Measurement
	SourceFrameBytes        int
	RelayFrameBytes         int
	SourceMeasurement       channel.Measurement
	RelayMeasurement        channel.Measurement
	SlotMissCount           int
	RelayReadinessMissCount int
	ForwardingTrace         []LimitedMultiHopForward
	TimingBudgetPass        bool
	Decision                string
	FailReasons             []string
	EventHash               string
}

type LimitedMultiHopForward struct {
	Seq       uint32
	HopIndex  int
	FromID    uint16
	ToID      uint16
	SlotIndex int
	SlotName  string
	TxAtMs    int64
	RxAtMs    int64
	AirtimeMs int64
	FitsSlot  bool
}

func DefaultLimitedMultiHopExperimentConfig() LimitedMultiHopExperimentConfig {
	return LimitedMultiHopExperimentConfig{
		ScenarioID:          LimitedMultiHopScenarioID,
		Seed:                DefaultScenarioSeed,
		SourceID:            DefaultSourceID,
		Relay1ID:            DefaultRelayID,
		Relay2ID:            defaultLimitedMultiHopRelay2ID,
		ReceiverID:          defaultLimitedMultiHopReceiverID,
		FrameCount:          defaultLimitedMultiHopFrameCount,
		StartMs:             DefaultStartMs,
		SlotDurationMs:      channel.DefaultSlotDurationMs,
		LinkDelayMs:         DefaultLinkDelayMs,
		LatencyBudgetMs:     defaultLimitedMultiHopLatencyMs,
		FramePeriodBudgetMs: DefaultFramePeriodMs,
		FrameBytes:          defaultLimitedMultiHopFrameBytes,
		ChannelBitrateBps:   channel.DefaultChannelBitrateBps,
	}
}

func RunLimitedMultiHopExperiment(cfg LimitedMultiHopExperimentConfig) (LimitedMultiHopExperimentReport, error) {
	cfg = normalizeLimitedMultiHopExperimentConfig(cfg)
	if err := validateLimitedMultiHopExperimentConfig(cfg); err != nil {
		return LimitedMultiHopExperimentReport{}, err
	}

	schedule := limitedMultiHopSchedule(cfg)
	sourceMeasurement, relayMeasurement, err := limitedMultiHopMeasurements(cfg)
	if err != nil {
		return LimitedMultiHopExperimentReport{}, err
	}

	superframeMs := schedule.SuperframeDurationMs()
	endToEndLatencyMs := limitedMultiHopEndToEndLatencyMs(cfg)
	report := LimitedMultiHopExperimentReport{
		ScenarioID:           cfg.ScenarioID,
		Seed:                 cfg.Seed,
		SourceID:             cfg.SourceID,
		RelayPath:            []uint16{cfg.Relay1ID, cfg.Relay2ID},
		ReceiverID:           cfg.ReceiverID,
		FrameCount:           cfg.FrameCount,
		HopCount:             2,
		SlotsPerSuperframe:   len(schedule.Slots),
		SuperframeDurationMs: superframeMs,
		SlotDurationMs:       cfg.SlotDurationMs,
		LinkDelayMs:          cfg.LinkDelayMs,
		LatencyBudgetMs:      cfg.LatencyBudgetMs,
		FramePeriodBudgetMs:  cfg.FramePeriodBudgetMs,
		SourceFramePeriodMs:  superframeMs,
		EndToEndLatencyMs:    endToEndLatencyMs,
		LatencyP95Ms:         endToEndLatencyMs,
		Measurement:          sourceMeasurement,
		SourceFrameBytes:     cfg.SourceFrameBytes,
		RelayFrameBytes:      cfg.RelayFrameBytes,
		SourceMeasurement:    sourceMeasurement,
		RelayMeasurement:     relayMeasurement,
		ForwardingTrace:      make([]LimitedMultiHopForward, 0, cfg.FrameCount*len(schedule.Slots)),
	}

	path := []uint16{cfg.SourceID, cfg.Relay1ID, cfg.Relay2ID, cfg.ReceiverID}
	for seq := 0; seq < cfg.FrameCount; seq++ {
		frameStartMs := addLimitedMultiHopMs(cfg.StartMs, mulLimitedMultiHopMs(int64(seq), superframeMs))
		for slotIndex := 0; slotIndex < len(schedule.Slots); slotIndex++ {
			measurement := limitedMultiHopSlotMeasurement(slotIndex, sourceMeasurement, relayMeasurement)
			slotStartMs := addLimitedMultiHopMs(frameStartMs, mulLimitedMultiHopMs(int64(slotIndex), cfg.SlotDurationMs))
			rxAtMs := limitedMultiHopReceiveAtMs(slotStartMs, measurement.AirtimeMs, cfg.LinkDelayMs)
			if !measurement.FitsSlot {
				report.SlotMissCount++
			}
			if slotIndex+1 < len(schedule.Slots) {
				nextSlotStartMs := addLimitedMultiHopMs(frameStartMs, mulLimitedMultiHopMs(int64(slotIndex+1), cfg.SlotDurationMs))
				if rxAtMs > nextSlotStartMs {
					report.RelayReadinessMissCount++
				}
			}
			report.ForwardingTrace = append(report.ForwardingTrace, LimitedMultiHopForward{
				Seq:       uint32(seq),
				HopIndex:  slotIndex,
				FromID:    path[slotIndex],
				ToID:      path[slotIndex+1],
				SlotIndex: slotIndex,
				SlotName:  schedule.Slots[slotIndex].Name,
				TxAtMs:    slotStartMs,
				RxAtMs:    rxAtMs,
				AirtimeMs: measurement.AirtimeMs,
				FitsSlot:  measurement.FitsSlot,
			})
		}
	}

	report.FailReasons = limitedMultiHopFailReasons(report)
	report.TimingBudgetPass = len(report.FailReasons) == 0
	if report.TimingBudgetPass {
		report.Decision = LimitedMultiHopDecisionGo
	} else {
		report.Decision = LimitedMultiHopDecisionNoGo
	}

	hash, err := limitedMultiHopReportHash(report)
	if err != nil {
		return LimitedMultiHopExperimentReport{}, err
	}
	report.EventHash = hash
	return report, nil
}

func normalizeLimitedMultiHopExperimentConfig(cfg LimitedMultiHopExperimentConfig) LimitedMultiHopExperimentConfig {
	def := DefaultLimitedMultiHopExperimentConfig()
	if cfg.ScenarioID == "" {
		cfg.ScenarioID = def.ScenarioID
	}
	if cfg.Seed == 0 {
		cfg.Seed = def.Seed
	}
	if cfg.ChannelBitrateBps == 0 {
		cfg.ChannelBitrateBps = def.ChannelBitrateBps
	}
	if cfg.SourceFrameBytes == 0 {
		cfg.SourceFrameBytes = cfg.FrameBytes
	}
	if cfg.RelayFrameBytes == 0 {
		cfg.RelayFrameBytes = cfg.FrameBytes
	}
	return cfg
}

func validateLimitedMultiHopExperimentConfig(cfg LimitedMultiHopExperimentConfig) error {
	if cfg.FrameCount <= 0 {
		return fmt.Errorf("limited multi-hop: frame_count=%d must be > 0", cfg.FrameCount)
	}
	if cfg.FrameCount > maxLimitedMultiHopTraceRows/3 {
		return fmt.Errorf("limited multi-hop: frame_count=%d would exceed trace cap %d", cfg.FrameCount, maxLimitedMultiHopTraceRows)
	}
	if uint64(cfg.FrameCount) > uint64(^uint32(0)) {
		return fmt.Errorf("limited multi-hop: frame_count=%d must fit uint32 sequence space", cfg.FrameCount)
	}
	if cfg.FrameBytes <= 0 {
		return fmt.Errorf("limited multi-hop: frame_bytes=%d must be > 0", cfg.FrameBytes)
	}
	if cfg.FrameBytes > maxLimitedMultiHopFrameBytes {
		return fmt.Errorf("limited multi-hop: frame_bytes=%d exceeds cap %d", cfg.FrameBytes, maxLimitedMultiHopFrameBytes)
	}
	if cfg.SourceFrameBytes <= 0 {
		return fmt.Errorf("limited multi-hop: source_frame_bytes=%d must be > 0", cfg.SourceFrameBytes)
	}
	if cfg.SourceFrameBytes > maxLimitedMultiHopFrameBytes {
		return fmt.Errorf("limited multi-hop: source_frame_bytes=%d exceeds cap %d", cfg.SourceFrameBytes, maxLimitedMultiHopFrameBytes)
	}
	if cfg.RelayFrameBytes <= 0 {
		return fmt.Errorf("limited multi-hop: relay_frame_bytes=%d must be > 0", cfg.RelayFrameBytes)
	}
	if cfg.RelayFrameBytes > maxLimitedMultiHopFrameBytes {
		return fmt.Errorf("limited multi-hop: relay_frame_bytes=%d exceeds cap %d", cfg.RelayFrameBytes, maxLimitedMultiHopFrameBytes)
	}
	if cfg.StartMs < 0 {
		return fmt.Errorf("limited multi-hop: start_ms=%d must be >= 0", cfg.StartMs)
	}
	if cfg.SlotDurationMs <= 0 {
		return fmt.Errorf("limited multi-hop: slot_duration_ms=%d must be > 0", cfg.SlotDurationMs)
	}
	if cfg.LinkDelayMs < 0 {
		return fmt.Errorf("limited multi-hop: link_delay_ms=%d must be >= 0", cfg.LinkDelayMs)
	}
	if cfg.LinkDelayMs > cfg.SlotDurationMs {
		return fmt.Errorf("limited multi-hop: link_delay_ms=%d must be <= slot_duration_ms=%d", cfg.LinkDelayMs, cfg.SlotDurationMs)
	}
	if cfg.LatencyBudgetMs <= 0 {
		return fmt.Errorf("limited multi-hop: latency_budget_ms=%d must be > 0", cfg.LatencyBudgetMs)
	}
	if cfg.FramePeriodBudgetMs <= 0 {
		return fmt.Errorf("limited multi-hop: frame_period_budget_ms=%d must be > 0", cfg.FramePeriodBudgetMs)
	}
	if err := validateDistinctLimitedMultiHopIDs(cfg); err != nil {
		return err
	}
	if err := limitedMultiHopSchedule(cfg).Validate(); err != nil {
		return err
	}
	if _, _, err := limitedMultiHopMeasurements(cfg); err != nil {
		return err
	}
	if err := validateLimitedMultiHopTimeHorizon(cfg); err != nil {
		return err
	}
	return nil
}

func validateDistinctLimitedMultiHopIDs(cfg LimitedMultiHopExperimentConfig) error {
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
			return fmt.Errorf("limited multi-hop: %s must be > 0", item.name)
		}
		if previous, ok := seen[item.id]; ok {
			return fmt.Errorf("limited multi-hop: %s=%d duplicates %s", item.name, item.id, previous)
		}
		seen[item.id] = item.name
	}
	return nil
}

func validateLimitedMultiHopTimeHorizon(cfg LimitedMultiHopExperimentConfig) error {
	superframeMs, err := checkedLimitedMultiHopMul(3, cfg.SlotDurationMs)
	if err != nil {
		return fmt.Errorf("limited multi-hop: superframe duration: %w", err)
	}
	lastFrameOffset, err := checkedLimitedMultiHopMul(int64(cfg.FrameCount-1), superframeMs)
	if err != nil {
		return fmt.Errorf("limited multi-hop: last frame offset: %w", err)
	}
	lastFrameStart, err := checkedLimitedMultiHopAdd(cfg.StartMs, lastFrameOffset)
	if err != nil {
		return fmt.Errorf("limited multi-hop: last frame start: %w", err)
	}
	lastRelayOffset, err := checkedLimitedMultiHopMul(2, cfg.SlotDurationMs)
	if err != nil {
		return fmt.Errorf("limited multi-hop: last relay slot offset: %w", err)
	}
	lastRelaySlot, err := checkedLimitedMultiHopAdd(lastFrameStart, lastRelayOffset)
	if err != nil {
		return fmt.Errorf("limited multi-hop: last relay slot: %w", err)
	}
	_, relayMeasurement, err := limitedMultiHopMeasurements(cfg)
	if err != nil {
		return err
	}
	withAirtime, err := checkedLimitedMultiHopAdd(lastRelaySlot, relayMeasurement.AirtimeMs)
	if err != nil {
		return fmt.Errorf("limited multi-hop: last receive airtime: %w", err)
	}
	if _, err := checkedLimitedMultiHopAdd(withAirtime, cfg.LinkDelayMs); err != nil {
		return fmt.Errorf("limited multi-hop: last receive time: %w", err)
	}
	return nil
}

func limitedMultiHopSchedule(cfg LimitedMultiHopExperimentConfig) timing.Schedule {
	return timing.Schedule{
		OriginMs:       cfg.StartMs,
		SlotDurationMs: cfg.SlotDurationMs,
		Slots: []timing.Slot{
			{Name: timing.SourceSlotName, Owner: timing.SlotOwnerSource},
			{Name: "relay1_slot", Owner: timing.SlotOwnerRelay},
			{Name: "relay2_slot", Owner: timing.SlotOwnerRelay},
		},
	}
}

func limitedMultiHopEndToEndLatencyMs(cfg LimitedMultiHopExperimentConfig) int64 {
	_, relayMeasurement, err := limitedMultiHopMeasurements(cfg)
	if err != nil {
		panic(err.Error())
	}
	return limitedMultiHopReceiveAtMs(mulLimitedMultiHopMs(2, cfg.SlotDurationMs), relayMeasurement.AirtimeMs, cfg.LinkDelayMs)
}

func limitedMultiHopMeasurements(cfg LimitedMultiHopExperimentConfig) (channel.Measurement, channel.Measurement, error) {
	sourceMeasurement, err := channel.MeasureFrameBytes(cfg.SourceFrameBytes, channel.AirtimeConfig{
		ChannelBitrateBps: cfg.ChannelBitrateBps,
		SlotDurationMs:    cfg.SlotDurationMs,
	})
	if err != nil {
		return channel.Measurement{}, channel.Measurement{}, err
	}
	relayMeasurement, err := channel.MeasureFrameBytes(cfg.RelayFrameBytes, channel.AirtimeConfig{
		ChannelBitrateBps: cfg.ChannelBitrateBps,
		SlotDurationMs:    cfg.SlotDurationMs,
	})
	if err != nil {
		return channel.Measurement{}, channel.Measurement{}, err
	}
	return sourceMeasurement, relayMeasurement, nil
}

func limitedMultiHopSlotMeasurement(slotIndex int, sourceMeasurement channel.Measurement, relayMeasurement channel.Measurement) channel.Measurement {
	if slotIndex == 0 {
		return sourceMeasurement
	}
	return relayMeasurement
}

func limitedMultiHopFailReasons(report LimitedMultiHopExperimentReport) []string {
	reasons := make([]string, 0, 4)
	if report.SlotMissCount > 0 {
		reasons = append(reasons, LimitedMultiHopFailSlotMiss)
	}
	if report.RelayReadinessMissCount > 0 {
		reasons = append(reasons, LimitedMultiHopFailRelayReadinessMiss)
	}
	if report.LatencyP95Ms > report.LatencyBudgetMs {
		reasons = append(reasons, LimitedMultiHopFailLatencyBudgetExceeded)
	}
	if report.SourceFramePeriodMs > report.FramePeriodBudgetMs {
		reasons = append(reasons, LimitedMultiHopFailFramePeriodExceeded)
	}
	return reasons
}

func limitedMultiHopReceiveAtMs(txAtMs, airtimeMs, linkDelayMs int64) int64 {
	return addLimitedMultiHopMs(addLimitedMultiHopMs(txAtMs, airtimeMs), linkDelayMs)
}

func limitedMultiHopReportHash(report LimitedMultiHopExperimentReport) (string, error) {
	hashable := report
	hashable.EventHash = ""
	b, err := json.Marshal(hashable)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func addLimitedMultiHopMs(a, b int64) int64 {
	result, err := checkedLimitedMultiHopAdd(a, b)
	if err != nil {
		panic(err.Error())
	}
	return result
}

func mulLimitedMultiHopMs(a, b int64) int64 {
	result, err := checkedLimitedMultiHopMul(a, b)
	if err != nil {
		panic(err.Error())
	}
	return result
}

func checkedLimitedMultiHopAdd(a, b int64) (int64, error) {
	if a < 0 || b < 0 {
		return 0, fmt.Errorf("negative simulated-time operand")
	}
	if a > math.MaxInt64-b {
		return 0, fmt.Errorf("simulated-time addition overflow")
	}
	return a + b, nil
}

func checkedLimitedMultiHopMul(a, b int64) (int64, error) {
	if a < 0 || b < 0 {
		return 0, fmt.Errorf("negative simulated-time operand")
	}
	if a != 0 && b > math.MaxInt64/a {
		return 0, fmt.Errorf("simulated-time multiplication overflow")
	}
	return a * b, nil
}
