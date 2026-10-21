package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/rmsff/ad-hoc-radio/internal/channel"
)

const MultiHopTimingSweepScenarioID = "multihop_tdma_timing_sweep"

const (
	MultiHopScheduleNextSlot       = "next_slot"
	MultiHopScheduleNextSuperframe = "next_superframe"
	MultiHopSchedulePipelineLag    = "pipeline_lag"
)

const (
	MultiHopTimingOutcomePass     = "pass"
	MultiHopTimingOutcomeDegraded = "degraded"
	MultiHopTimingOutcomeFail     = "fail"
)

const (
	MultiHopTimingFailSlotMiss              = "slot_miss"
	MultiHopTimingFailRelayReadinessMiss    = "relay_readiness_miss"
	MultiHopTimingFailLatencyBudgetExceeded = "latency_budget_exceeded"
	MultiHopTimingFailFramePeriodExceeded   = "source_frame_period_budget_exceeded"
)

const maxMultiHopTimingSweepRows = 10_000

type MultiHopTimingSweepConfig struct {
	ScenarioID          string
	Seed                uint64
	ChannelBitratesBps  []int64
	FrameBytes          []int
	SlotDurationsMs     []int64
	GuardTimesMs        []int64
	ScheduleModes       []string
	LinkDelayMs         int64
	LatencyBudgetMs     int64
	FramePeriodBudgetMs int64
}

type MultiHopTimingSweepReport struct {
	ScenarioID   string                   `json:"scenario_id"`
	Seed         uint64                   `json:"seed"`
	RowCount     int                      `json:"row_count"`
	PassRows     int                      `json:"pass_rows"`
	DegradedRows int                      `json:"degraded_rows"`
	FailRows     int                      `json:"fail_rows"`
	Rows         []MultiHopTimingSweepRow `json:"rows"`
	EventHash    string                   `json:"event_hash"`
}

type MultiHopTimingSweepRow struct {
	RowID                   string   `json:"row_id"`
	ChannelBitrateBps       int64    `json:"channel_bitrate_bps"`
	FrameBytes              int      `json:"frame_bytes"`
	SlotDurationMs          int64    `json:"slot_duration_ms"`
	GuardTimeMs             int64    `json:"guard_time_ms"`
	ScheduleMode            string   `json:"schedule_mode"`
	LinkDelayMs             int64    `json:"link_delay_ms"`
	AirtimeMs               int64    `json:"airtime_ms"`
	SlotFit                 bool     `json:"slot_fit"`
	RelayReady              bool     `json:"relay_ready"`
	RelayReadinessMissCount int      `json:"relay_readiness_miss_count"`
	SuperframeDurationMs    int64    `json:"superframe_duration_ms"`
	EndToEndLatencyMs       int64    `json:"end_to_end_latency_ms"`
	LatencyBudgetMs         int64    `json:"latency_budget_ms"`
	LatencyBudgetPass       bool     `json:"latency_budget_pass"`
	SourceFramePeriodMs     int64    `json:"source_frame_period_ms"`
	FramePeriodBudgetMs     int64    `json:"frame_period_budget_ms"`
	CadencePass             bool     `json:"cadence_pass"`
	Outcome                 string   `json:"outcome"`
	FailReasons             []string `json:"fail_reasons"`
}

func DefaultMultiHopTimingSweepConfig() MultiHopTimingSweepConfig {
	return MultiHopTimingSweepConfig{
		ScenarioID:          MultiHopTimingSweepScenarioID,
		Seed:                DefaultScenarioSeed,
		ChannelBitratesBps:  []int64{9600, 19200},
		FrameBytes:          []int{56, 22},
		SlotDurationsMs:     []int64{20, 40, 60, 80},
		GuardTimesMs:        []int64{0, 2, 5, 10},
		ScheduleModes:       []string{MultiHopScheduleNextSlot, MultiHopScheduleNextSuperframe, MultiHopSchedulePipelineLag},
		LinkDelayMs:         1,
		LatencyBudgetMs:     defaultLimitedMultiHopLatencyMs,
		FramePeriodBudgetMs: DefaultFramePeriodMs,
	}
}

func RunMultiHopTimingSweep(cfg MultiHopTimingSweepConfig) (MultiHopTimingSweepReport, error) {
	cfg = normalizeMultiHopTimingSweepConfig(cfg)
	if err := validateMultiHopTimingSweepConfig(cfg); err != nil {
		return MultiHopTimingSweepReport{}, err
	}

	report := MultiHopTimingSweepReport{
		ScenarioID: cfg.ScenarioID,
		Seed:       cfg.Seed,
		Rows:       make([]MultiHopTimingSweepRow, 0, multiHopTimingSweepRowCount(cfg)),
	}

	for _, bitrate := range cfg.ChannelBitratesBps {
		for _, frameBytes := range cfg.FrameBytes {
			for _, slotDurationMs := range cfg.SlotDurationsMs {
				for _, guardTimeMs := range cfg.GuardTimesMs {
					for _, scheduleMode := range cfg.ScheduleModes {
						row, err := multiHopTimingSweepRow(cfg, bitrate, frameBytes, slotDurationMs, guardTimeMs, scheduleMode)
						if err != nil {
							return MultiHopTimingSweepReport{}, err
						}
						report.Rows = append(report.Rows, row)
						switch row.Outcome {
						case MultiHopTimingOutcomePass:
							report.PassRows++
						case MultiHopTimingOutcomeDegraded:
							report.DegradedRows++
						case MultiHopTimingOutcomeFail:
							report.FailRows++
						default:
							return MultiHopTimingSweepReport{}, fmt.Errorf("multi-hop timing sweep: unknown outcome %q", row.Outcome)
						}
					}
				}
			}
		}
	}
	report.RowCount = len(report.Rows)

	hash, err := multiHopTimingSweepHash(report)
	if err != nil {
		return MultiHopTimingSweepReport{}, err
	}
	report.EventHash = hash
	return report, nil
}

func normalizeMultiHopTimingSweepConfig(cfg MultiHopTimingSweepConfig) MultiHopTimingSweepConfig {
	def := DefaultMultiHopTimingSweepConfig()
	if cfg.ScenarioID == "" {
		cfg.ScenarioID = def.ScenarioID
	}
	if cfg.Seed == 0 {
		cfg.Seed = def.Seed
	}
	if cfg.ChannelBitratesBps == nil {
		cfg.ChannelBitratesBps = cloneMultiHopTimingInt64s(def.ChannelBitratesBps)
	} else {
		cfg.ChannelBitratesBps = sortedMultiHopTimingInt64s(cfg.ChannelBitratesBps)
	}
	if cfg.FrameBytes == nil {
		cfg.FrameBytes = cloneMultiHopTimingInts(def.FrameBytes)
	} else {
		cfg.FrameBytes = sortedMultiHopTimingInts(cfg.FrameBytes)
	}
	if cfg.SlotDurationsMs == nil {
		cfg.SlotDurationsMs = cloneMultiHopTimingInt64s(def.SlotDurationsMs)
	} else {
		cfg.SlotDurationsMs = sortedMultiHopTimingInt64s(cfg.SlotDurationsMs)
	}
	if cfg.GuardTimesMs == nil {
		cfg.GuardTimesMs = cloneMultiHopTimingInt64s(def.GuardTimesMs)
	} else {
		cfg.GuardTimesMs = sortedMultiHopTimingInt64s(cfg.GuardTimesMs)
	}
	if cfg.ScheduleModes == nil {
		cfg.ScheduleModes = cloneMultiHopTimingStrings(def.ScheduleModes)
	}
	if cfg.LatencyBudgetMs == 0 {
		cfg.LatencyBudgetMs = def.LatencyBudgetMs
	}
	if cfg.FramePeriodBudgetMs == 0 {
		cfg.FramePeriodBudgetMs = def.FramePeriodBudgetMs
	}
	return cfg
}

func validateMultiHopTimingSweepConfig(cfg MultiHopTimingSweepConfig) error {
	if cfg.ScenarioID == "" {
		return fmt.Errorf("multi-hop timing sweep: scenario_id must not be empty")
	}
	if err := validatePositiveInt64s("channel_bitrates_bps", cfg.ChannelBitratesBps); err != nil {
		return err
	}
	if err := validatePositiveInts("frame_bytes", cfg.FrameBytes); err != nil {
		return err
	}
	if err := validatePositiveInt64s("slot_durations_ms", cfg.SlotDurationsMs); err != nil {
		return err
	}
	if err := validateNonNegativeInt64s("guard_times_ms", cfg.GuardTimesMs); err != nil {
		return err
	}
	if err := validateMultiHopTimingScheduleModes(cfg.ScheduleModes); err != nil {
		return err
	}
	if cfg.LinkDelayMs < 0 {
		return fmt.Errorf("multi-hop timing sweep: link_delay_ms=%d must be >= 0", cfg.LinkDelayMs)
	}
	if cfg.LatencyBudgetMs <= 0 {
		return fmt.Errorf("multi-hop timing sweep: latency_budget_ms=%d must be > 0", cfg.LatencyBudgetMs)
	}
	if cfg.FramePeriodBudgetMs <= 0 {
		return fmt.Errorf("multi-hop timing sweep: frame_period_budget_ms=%d must be > 0", cfg.FramePeriodBudgetMs)
	}
	rows := multiHopTimingSweepRowCount(cfg)
	if rows <= 0 {
		return fmt.Errorf("multi-hop timing sweep: row count must be > 0")
	}
	if rows > maxMultiHopTimingSweepRows {
		return fmt.Errorf("multi-hop timing sweep: row count %d exceeds cap %d", rows, maxMultiHopTimingSweepRows)
	}
	return nil
}

func multiHopTimingSweepRow(
	cfg MultiHopTimingSweepConfig,
	bitrate int64,
	frameBytes int,
	slotDurationMs int64,
	guardTimeMs int64,
	scheduleMode string,
) (MultiHopTimingSweepRow, error) {
	measurement, err := channel.MeasureFrameBytes(frameBytes, channel.AirtimeConfig{
		ChannelBitrateBps: bitrate,
		SlotDurationMs:    slotDurationMs,
	})
	if err != nil {
		return MultiHopTimingSweepRow{}, err
	}

	txOffsets, sourceFramePeriodMs, err := multiHopTimingSchedule(slotDurationMs, scheduleMode)
	if err != nil {
		return MultiHopTimingSweepRow{}, err
	}
	superframeMs, err := checkedLimitedMultiHopMul(3, slotDurationMs)
	if err != nil {
		return MultiHopTimingSweepRow{}, err
	}
	slotFit := measurement.AirtimeMs <= slotDurationMs-guardTimeMs
	relayMisses, err := multiHopTimingRelayReadinessMisses(txOffsets, measurement.AirtimeMs, cfg.LinkDelayMs, guardTimeMs)
	if err != nil {
		return MultiHopTimingSweepRow{}, err
	}
	endToEndLatencyMs, err := multiHopTimingEndToEndLatencyMs(txOffsets, measurement.AirtimeMs, cfg.LinkDelayMs)
	if err != nil {
		return MultiHopTimingSweepRow{}, err
	}
	latencyPass := endToEndLatencyMs <= cfg.LatencyBudgetMs
	cadencePass := sourceFramePeriodMs <= cfg.FramePeriodBudgetMs
	failReasons := multiHopTimingFailReasons(slotFit, relayMisses, latencyPass, cadencePass)
	outcome := multiHopTimingOutcome(failReasons)

	return MultiHopTimingSweepRow{
		RowID:                   multiHopTimingRowID(bitrate, frameBytes, slotDurationMs, guardTimeMs, scheduleMode),
		ChannelBitrateBps:       bitrate,
		FrameBytes:              frameBytes,
		SlotDurationMs:          slotDurationMs,
		GuardTimeMs:             guardTimeMs,
		ScheduleMode:            scheduleMode,
		LinkDelayMs:             cfg.LinkDelayMs,
		AirtimeMs:               measurement.AirtimeMs,
		SlotFit:                 slotFit,
		RelayReady:              relayMisses == 0,
		RelayReadinessMissCount: relayMisses,
		SuperframeDurationMs:    superframeMs,
		EndToEndLatencyMs:       endToEndLatencyMs,
		LatencyBudgetMs:         cfg.LatencyBudgetMs,
		LatencyBudgetPass:       latencyPass,
		SourceFramePeriodMs:     sourceFramePeriodMs,
		FramePeriodBudgetMs:     cfg.FramePeriodBudgetMs,
		CadencePass:             cadencePass,
		Outcome:                 outcome,
		FailReasons:             failReasons,
	}, nil
}

func multiHopTimingSchedule(slotDurationMs int64, mode string) ([]int64, int64, error) {
	superframeMs, err := checkedLimitedMultiHopMul(3, slotDurationMs)
	if err != nil {
		return nil, 0, err
	}
	switch mode {
	case MultiHopScheduleNextSlot:
		secondRelayOffset, err := checkedLimitedMultiHopMul(2, slotDurationMs)
		if err != nil {
			return nil, 0, err
		}
		return []int64{0, slotDurationMs, secondRelayOffset}, superframeMs, nil
	case MultiHopScheduleNextSuperframe:
		txOffsets, err := multiHopTimingLaggedOffsets(slotDurationMs, superframeMs)
		if err != nil {
			return nil, 0, err
		}
		sourceFramePeriodMs, err := checkedLimitedMultiHopMul(3, superframeMs)
		if err != nil {
			return nil, 0, err
		}
		return txOffsets, sourceFramePeriodMs, nil
	case MultiHopSchedulePipelineLag:
		txOffsets, err := multiHopTimingLaggedOffsets(slotDurationMs, superframeMs)
		if err != nil {
			return nil, 0, err
		}
		return txOffsets, superframeMs, nil
	default:
		return nil, 0, fmt.Errorf("multi-hop timing sweep: unsupported schedule_mode %q", mode)
	}
}

func multiHopTimingLaggedOffsets(slotDurationMs, superframeMs int64) ([]int64, error) {
	firstRelayOffset, err := checkedLimitedMultiHopAdd(superframeMs, slotDurationMs)
	if err != nil {
		return nil, err
	}
	twoSuperframes, err := checkedLimitedMultiHopMul(2, superframeMs)
	if err != nil {
		return nil, err
	}
	twoSlots, err := checkedLimitedMultiHopMul(2, slotDurationMs)
	if err != nil {
		return nil, err
	}
	secondRelayOffset, err := checkedLimitedMultiHopAdd(twoSuperframes, twoSlots)
	if err != nil {
		return nil, err
	}
	return []int64{0, firstRelayOffset, secondRelayOffset}, nil
}

func multiHopTimingRelayReadinessMisses(txOffsets []int64, airtimeMs, linkDelayMs, guardTimeMs int64) (int, error) {
	misses := 0
	for i := 0; i < len(txOffsets)-1; i++ {
		rxAtMs, err := checkedLimitedMultiHopAdd(txOffsets[i], airtimeMs)
		if err != nil {
			return 0, err
		}
		rxAtMs, err = checkedLimitedMultiHopAdd(rxAtMs, linkDelayMs)
		if err != nil {
			return 0, err
		}
		readyAtMs, err := checkedLimitedMultiHopAdd(rxAtMs, guardTimeMs)
		if err != nil {
			return 0, err
		}
		if readyAtMs > txOffsets[i+1] {
			misses++
		}
	}
	return misses, nil
}

func multiHopTimingEndToEndLatencyMs(txOffsets []int64, airtimeMs, linkDelayMs int64) (int64, error) {
	lastTx := txOffsets[len(txOffsets)-1]
	withAirtime, err := checkedLimitedMultiHopAdd(lastTx, airtimeMs)
	if err != nil {
		return 0, err
	}
	return checkedLimitedMultiHopAdd(withAirtime, linkDelayMs)
}

func multiHopTimingFailReasons(slotFit bool, relayMisses int, latencyPass bool, cadencePass bool) []string {
	reasons := make([]string, 0, 4)
	if !slotFit {
		reasons = append(reasons, MultiHopTimingFailSlotMiss)
	}
	if relayMisses > 0 {
		reasons = append(reasons, MultiHopTimingFailRelayReadinessMiss)
	}
	if !latencyPass {
		reasons = append(reasons, MultiHopTimingFailLatencyBudgetExceeded)
	}
	if !cadencePass {
		reasons = append(reasons, MultiHopTimingFailFramePeriodExceeded)
	}
	return reasons
}

func multiHopTimingOutcome(failReasons []string) string {
	if len(failReasons) == 0 {
		return MultiHopTimingOutcomePass
	}
	for _, reason := range failReasons {
		if reason == MultiHopTimingFailSlotMiss || reason == MultiHopTimingFailRelayReadinessMiss {
			return MultiHopTimingOutcomeFail
		}
	}
	return MultiHopTimingOutcomeDegraded
}

func multiHopTimingRowID(bitrate int64, frameBytes int, slotDurationMs int64, guardTimeMs int64, scheduleMode string) string {
	return fmt.Sprintf("mht_bps_%05d_frame_%03d_slot_%03d_guard_%02d_%s",
		bitrate, frameBytes, slotDurationMs, guardTimeMs, scheduleMode)
}

func multiHopTimingSweepHash(report MultiHopTimingSweepReport) (string, error) {
	hashable := report
	hashable.EventHash = ""
	b, err := json.Marshal(hashable)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func multiHopTimingSweepRowCount(cfg MultiHopTimingSweepConfig) int {
	return len(cfg.ChannelBitratesBps) * len(cfg.FrameBytes) * len(cfg.SlotDurationsMs) * len(cfg.GuardTimesMs) * len(cfg.ScheduleModes)
}
