package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/rmsff/ad-hoc-radio/internal/channel"
)

const TwoHopLiveTimingScenarioID = "two_hop_live_variable_slot_timing"
const TwoHopLiveShortSourceTimingScenarioID = "two_hop_live_short_source_timing"

const (
	TwoHopLiveOutcomePass     = "pass"
	TwoHopLiveOutcomeDegraded = "degraded"
	TwoHopLiveOutcomeFail     = "fail"
)

const (
	TwoHopLiveFailSourceSlotMiss                 = "source_slot_miss"
	TwoHopLiveFailRelaySlotMiss                  = "relay_slot_miss"
	TwoHopLiveFailRelayReadinessMiss             = "relay_readiness_miss"
	TwoHopLiveFailCyclePeriodExceeded            = "cycle_period_exceeded"
	TwoHopLiveFailLatencyBudgetExceeded          = "latency_budget_exceeded"
	TwoHopLiveFailFramePeriodExceeded            = "source_frame_period_budget_exceeded"
	defaultTwoHopLiveSourceFrameBytes            = 56
	defaultTwoHopLiveRelayFrameBytes             = 22
	defaultTwoHopLiveShortSourceFrameBytes       = 22
	defaultTwoHopLiveVoicePeriodMs         int64 = 80
	maxTwoHopLiveTimingRows                      = 10_000
)

type TwoHopLiveTimingConfig struct {
	ScenarioID            string
	Seed                  uint64
	ChannelBitratesBps    []int64
	SourceFrameBytes      []int
	RelayFrameBytes       []int
	SourceSlotDurationsMs []int64
	RelaySlotDurationsMs  []int64
	GuardTimesMs          []int64
	VoicePeriodMs         int64
	LinkDelayMs           int64
	LatencyBudgetMs       int64
	FramePeriodBudgetMs   int64
}

type TwoHopLiveTimingReport struct {
	ScenarioID   string                `json:"scenario_id"`
	Seed         uint64                `json:"seed"`
	RowCount     int                   `json:"row_count"`
	PassRows     int                   `json:"pass_rows"`
	DegradedRows int                   `json:"degraded_rows"`
	FailRows     int                   `json:"fail_rows"`
	Rows         []TwoHopLiveTimingRow `json:"rows"`
	EventHash    string                `json:"event_hash"`
}

type TwoHopLiveTimingRow struct {
	RowID                   string   `json:"row_id"`
	ChannelBitrateBps       int64    `json:"channel_bitrate_bps"`
	SourceFrameBytes        int      `json:"source_frame_bytes"`
	RelayFrameBytes         int      `json:"relay_frame_bytes"`
	SourceSlotMs            int64    `json:"source_slot_ms"`
	RelaySlotMs             int64    `json:"relay_slot_ms"`
	GuardTimeMs             int64    `json:"guard_time_ms"`
	VoicePeriodMs           int64    `json:"voice_period_ms"`
	LinkDelayMs             int64    `json:"link_delay_ms"`
	SourceAirtimeMs         int64    `json:"source_airtime_ms"`
	RelayAirtimeMs          int64    `json:"relay_airtime_ms"`
	SourceSlotFit           bool     `json:"source_slot_fit"`
	RelaySlotFit            bool     `json:"relay_slot_fit"`
	RelayReady              bool     `json:"relay_ready"`
	RelayReadinessMissCount int      `json:"relay_readiness_miss_count"`
	TxOffsetsMs             []int64  `json:"tx_offsets_ms"`
	CycleOccupiedMs         int64    `json:"cycle_occupied_ms"`
	CycleSpareMs            int64    `json:"cycle_spare_ms"`
	CycleFitsPeriod         bool     `json:"cycle_fits_period"`
	EndToEndLatencyMs       int64    `json:"end_to_end_latency_ms"`
	LatencyBudgetMs         int64    `json:"latency_budget_ms"`
	LatencyBudgetPass       bool     `json:"latency_budget_pass"`
	FramePeriodBudgetMs     int64    `json:"frame_period_budget_ms"`
	CadencePass             bool     `json:"cadence_pass"`
	Outcome                 string   `json:"outcome"`
	FailReasons             []string `json:"fail_reasons"`
}

func DefaultTwoHopLiveTimingConfig() TwoHopLiveTimingConfig {
	return TwoHopLiveTimingConfig{
		ScenarioID:            TwoHopLiveTimingScenarioID,
		Seed:                  DefaultScenarioSeed,
		ChannelBitratesBps:    []int64{19200},
		SourceFrameBytes:      []int{defaultTwoHopLiveSourceFrameBytes},
		RelayFrameBytes:       []int{defaultTwoHopLiveRelayFrameBytes},
		SourceSlotDurationsMs: []int64{24, 26, 28, 30, 35, 40},
		RelaySlotDurationsMs:  []int64{10, 12, 15, 20},
		GuardTimesMs:          []int64{0, 2, 5, 10},
		VoicePeriodMs:         defaultTwoHopLiveVoicePeriodMs,
		LinkDelayMs:           1,
		LatencyBudgetMs:       defaultLimitedMultiHopLatencyMs,
		FramePeriodBudgetMs:   DefaultFramePeriodMs,
	}
}

func DefaultTwoHopLiveShortSourceTimingConfig() TwoHopLiveTimingConfig {
	cfg := DefaultTwoHopLiveTimingConfig()
	cfg.ScenarioID = TwoHopLiveShortSourceTimingScenarioID
	cfg.SourceFrameBytes = []int{defaultTwoHopLiveShortSourceFrameBytes, defaultTwoHopLiveSourceFrameBytes}
	cfg.SourceSlotDurationsMs = []int64{12, 15, 20, 24, 26, 28, 30}
	cfg.RelaySlotDurationsMs = []int64{10, 12, 15, 20}
	cfg.GuardTimesMs = []int64{1, 2, 5}
	return cfg
}

func RunTwoHopLiveTimingSweep(cfg TwoHopLiveTimingConfig) (TwoHopLiveTimingReport, error) {
	cfg = normalizeTwoHopLiveTimingConfig(cfg)
	if err := validateTwoHopLiveTimingConfig(cfg); err != nil {
		return TwoHopLiveTimingReport{}, err
	}

	report := TwoHopLiveTimingReport{
		ScenarioID: cfg.ScenarioID,
		Seed:       cfg.Seed,
		Rows:       make([]TwoHopLiveTimingRow, 0, twoHopLiveTimingRowCount(cfg)),
	}

	for _, bitrate := range cfg.ChannelBitratesBps {
		for _, sourceFrameBytes := range cfg.SourceFrameBytes {
			for _, relayFrameBytes := range cfg.RelayFrameBytes {
				for _, sourceSlotMs := range cfg.SourceSlotDurationsMs {
					for _, relaySlotMs := range cfg.RelaySlotDurationsMs {
						for _, guardTimeMs := range cfg.GuardTimesMs {
							row, err := twoHopLiveTimingRow(cfg, bitrate, sourceFrameBytes, relayFrameBytes, sourceSlotMs, relaySlotMs, guardTimeMs)
							if err != nil {
								return TwoHopLiveTimingReport{}, err
							}
							report.Rows = append(report.Rows, row)
							switch row.Outcome {
							case TwoHopLiveOutcomePass:
								report.PassRows++
							case TwoHopLiveOutcomeDegraded:
								report.DegradedRows++
							case TwoHopLiveOutcomeFail:
								report.FailRows++
							default:
								return TwoHopLiveTimingReport{}, fmt.Errorf("two-hop live timing: unknown outcome %q", row.Outcome)
							}
						}
					}
				}
			}
		}
	}
	report.RowCount = len(report.Rows)

	hash, err := twoHopLiveTimingHash(report)
	if err != nil {
		return TwoHopLiveTimingReport{}, err
	}
	report.EventHash = hash
	return report, nil
}

func normalizeTwoHopLiveTimingConfig(cfg TwoHopLiveTimingConfig) TwoHopLiveTimingConfig {
	def := DefaultTwoHopLiveTimingConfig()
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
	if cfg.SourceFrameBytes == nil {
		cfg.SourceFrameBytes = cloneMultiHopTimingInts(def.SourceFrameBytes)
	} else {
		cfg.SourceFrameBytes = sortedMultiHopTimingInts(cfg.SourceFrameBytes)
	}
	if cfg.RelayFrameBytes == nil {
		cfg.RelayFrameBytes = cloneMultiHopTimingInts(def.RelayFrameBytes)
	} else {
		cfg.RelayFrameBytes = sortedMultiHopTimingInts(cfg.RelayFrameBytes)
	}
	if cfg.SourceSlotDurationsMs == nil {
		cfg.SourceSlotDurationsMs = cloneMultiHopTimingInt64s(def.SourceSlotDurationsMs)
	} else {
		cfg.SourceSlotDurationsMs = sortedMultiHopTimingInt64s(cfg.SourceSlotDurationsMs)
	}
	if cfg.RelaySlotDurationsMs == nil {
		cfg.RelaySlotDurationsMs = cloneMultiHopTimingInt64s(def.RelaySlotDurationsMs)
	} else {
		cfg.RelaySlotDurationsMs = sortedMultiHopTimingInt64s(cfg.RelaySlotDurationsMs)
	}
	if cfg.GuardTimesMs == nil {
		cfg.GuardTimesMs = cloneMultiHopTimingInt64s(def.GuardTimesMs)
	} else {
		cfg.GuardTimesMs = sortedMultiHopTimingInt64s(cfg.GuardTimesMs)
	}
	if cfg.VoicePeriodMs == 0 {
		cfg.VoicePeriodMs = def.VoicePeriodMs
	}
	if cfg.LatencyBudgetMs == 0 {
		cfg.LatencyBudgetMs = def.LatencyBudgetMs
	}
	if cfg.FramePeriodBudgetMs == 0 {
		cfg.FramePeriodBudgetMs = def.FramePeriodBudgetMs
	}
	return cfg
}

func validateTwoHopLiveTimingConfig(cfg TwoHopLiveTimingConfig) error {
	if cfg.ScenarioID == "" {
		return fmt.Errorf("two-hop live timing: scenario_id must not be empty")
	}
	if err := validatePositiveInt64s("channel_bitrates_bps", cfg.ChannelBitratesBps); err != nil {
		return err
	}
	if err := validatePositiveInts("source_frame_bytes", cfg.SourceFrameBytes); err != nil {
		return err
	}
	if err := validatePositiveInts("relay_frame_bytes", cfg.RelayFrameBytes); err != nil {
		return err
	}
	if err := validatePositiveInt64s("source_slot_durations_ms", cfg.SourceSlotDurationsMs); err != nil {
		return err
	}
	if err := validatePositiveInt64s("relay_slot_durations_ms", cfg.RelaySlotDurationsMs); err != nil {
		return err
	}
	if err := validateNonNegativeInt64s("guard_times_ms", cfg.GuardTimesMs); err != nil {
		return err
	}
	if cfg.VoicePeriodMs <= 0 {
		return fmt.Errorf("two-hop live timing: voice_period_ms=%d must be > 0", cfg.VoicePeriodMs)
	}
	if cfg.LinkDelayMs < 0 {
		return fmt.Errorf("two-hop live timing: link_delay_ms=%d must be >= 0", cfg.LinkDelayMs)
	}
	if cfg.LatencyBudgetMs <= 0 {
		return fmt.Errorf("two-hop live timing: latency_budget_ms=%d must be > 0", cfg.LatencyBudgetMs)
	}
	if cfg.FramePeriodBudgetMs <= 0 {
		return fmt.Errorf("two-hop live timing: frame_period_budget_ms=%d must be > 0", cfg.FramePeriodBudgetMs)
	}
	rows := twoHopLiveTimingRowCount(cfg)
	if rows <= 0 {
		return fmt.Errorf("two-hop live timing: row count must be > 0")
	}
	if rows > maxTwoHopLiveTimingRows {
		return fmt.Errorf("two-hop live timing: row count %d exceeds cap %d", rows, maxTwoHopLiveTimingRows)
	}
	return nil
}

func twoHopLiveTimingRow(
	cfg TwoHopLiveTimingConfig,
	bitrate int64,
	sourceFrameBytes int,
	relayFrameBytes int,
	sourceSlotMs int64,
	relaySlotMs int64,
	guardTimeMs int64,
) (TwoHopLiveTimingRow, error) {
	sourceMeasurement, err := channel.MeasureFrameBytes(sourceFrameBytes, channel.AirtimeConfig{
		ChannelBitrateBps: bitrate,
		SlotDurationMs:    sourceSlotMs,
	})
	if err != nil {
		return TwoHopLiveTimingRow{}, err
	}
	relayMeasurement, err := channel.MeasureFrameBytes(relayFrameBytes, channel.AirtimeConfig{
		ChannelBitrateBps: bitrate,
		SlotDurationMs:    relaySlotMs,
	})
	if err != nil {
		return TwoHopLiveTimingRow{}, err
	}

	secondRelayOffset, err := checkedLimitedMultiHopAdd(sourceSlotMs, relaySlotMs)
	if err != nil {
		return TwoHopLiveTimingRow{}, err
	}
	txOffsets := []int64{0, sourceSlotMs, secondRelayOffset}
	sourceSlotFit := twoHopLiveFitsSlot(sourceMeasurement.AirtimeMs, sourceSlotMs, guardTimeMs)
	relaySlotFit := twoHopLiveFitsSlot(relayMeasurement.AirtimeMs, relaySlotMs, guardTimeMs)

	airtimes := []int64{sourceMeasurement.AirtimeMs, relayMeasurement.AirtimeMs, relayMeasurement.AirtimeMs}
	relayMisses, err := twoHopLiveRelayReadinessMisses(txOffsets, airtimes, cfg.LinkDelayMs, guardTimeMs)
	if err != nil {
		return TwoHopLiveTimingRow{}, err
	}
	cycleOccupiedMs, err := checkedLimitedMultiHopAdd(txOffsets[2], relayMeasurement.AirtimeMs)
	if err != nil {
		return TwoHopLiveTimingRow{}, err
	}
	cycleOccupiedMs, err = checkedLimitedMultiHopAdd(cycleOccupiedMs, guardTimeMs)
	if err != nil {
		return TwoHopLiveTimingRow{}, err
	}
	endToEndLatencyMs, err := checkedLimitedMultiHopAdd(txOffsets[2], relayMeasurement.AirtimeMs)
	if err != nil {
		return TwoHopLiveTimingRow{}, err
	}
	endToEndLatencyMs, err = checkedLimitedMultiHopAdd(endToEndLatencyMs, cfg.LinkDelayMs)
	if err != nil {
		return TwoHopLiveTimingRow{}, err
	}

	cycleFitsPeriod := cycleOccupiedMs <= cfg.VoicePeriodMs
	latencyPass := endToEndLatencyMs <= cfg.LatencyBudgetMs
	cadencePass := cfg.VoicePeriodMs <= cfg.FramePeriodBudgetMs
	failReasons := twoHopLiveFailReasons(sourceSlotFit, relaySlotFit, relayMisses, cycleFitsPeriod, latencyPass, cadencePass)
	outcome := twoHopLiveOutcome(failReasons)

	return TwoHopLiveTimingRow{
		RowID:                   twoHopLiveTimingRowID(bitrate, sourceFrameBytes, relayFrameBytes, sourceSlotMs, relaySlotMs, guardTimeMs),
		ChannelBitrateBps:       bitrate,
		SourceFrameBytes:        sourceFrameBytes,
		RelayFrameBytes:         relayFrameBytes,
		SourceSlotMs:            sourceSlotMs,
		RelaySlotMs:             relaySlotMs,
		GuardTimeMs:             guardTimeMs,
		VoicePeriodMs:           cfg.VoicePeriodMs,
		LinkDelayMs:             cfg.LinkDelayMs,
		SourceAirtimeMs:         sourceMeasurement.AirtimeMs,
		RelayAirtimeMs:          relayMeasurement.AirtimeMs,
		SourceSlotFit:           sourceSlotFit,
		RelaySlotFit:            relaySlotFit,
		RelayReady:              relayMisses == 0,
		RelayReadinessMissCount: relayMisses,
		TxOffsetsMs:             cloneMultiHopTimingInt64s(txOffsets),
		CycleOccupiedMs:         cycleOccupiedMs,
		CycleSpareMs:            cfg.VoicePeriodMs - cycleOccupiedMs,
		CycleFitsPeriod:         cycleFitsPeriod,
		EndToEndLatencyMs:       endToEndLatencyMs,
		LatencyBudgetMs:         cfg.LatencyBudgetMs,
		LatencyBudgetPass:       latencyPass,
		FramePeriodBudgetMs:     cfg.FramePeriodBudgetMs,
		CadencePass:             cadencePass,
		Outcome:                 outcome,
		FailReasons:             failReasons,
	}, nil
}

func twoHopLiveFitsSlot(airtimeMs, slotMs, guardTimeMs int64) bool {
	withGuard, err := checkedLimitedMultiHopAdd(airtimeMs, guardTimeMs)
	if err != nil {
		return false
	}
	return withGuard <= slotMs
}

func twoHopLiveRelayReadinessMisses(txOffsets []int64, airtimes []int64, linkDelayMs int64, guardTimeMs int64) (int, error) {
	misses := 0
	for i := 0; i < len(txOffsets)-1; i++ {
		readyAtMs, err := checkedLimitedMultiHopAdd(txOffsets[i], airtimes[i])
		if err != nil {
			return 0, err
		}
		readyAtMs, err = checkedLimitedMultiHopAdd(readyAtMs, linkDelayMs)
		if err != nil {
			return 0, err
		}
		readyAtMs, err = checkedLimitedMultiHopAdd(readyAtMs, guardTimeMs)
		if err != nil {
			return 0, err
		}
		if readyAtMs > txOffsets[i+1] {
			misses++
		}
	}
	return misses, nil
}

func twoHopLiveFailReasons(
	sourceSlotFit bool,
	relaySlotFit bool,
	relayMisses int,
	cycleFitsPeriod bool,
	latencyPass bool,
	cadencePass bool,
) []string {
	reasons := make([]string, 0, 6)
	if !sourceSlotFit {
		reasons = append(reasons, TwoHopLiveFailSourceSlotMiss)
	}
	if !relaySlotFit {
		reasons = append(reasons, TwoHopLiveFailRelaySlotMiss)
	}
	if relayMisses > 0 {
		reasons = append(reasons, TwoHopLiveFailRelayReadinessMiss)
	}
	if !cycleFitsPeriod {
		reasons = append(reasons, TwoHopLiveFailCyclePeriodExceeded)
	}
	if !latencyPass {
		reasons = append(reasons, TwoHopLiveFailLatencyBudgetExceeded)
	}
	if !cadencePass {
		reasons = append(reasons, TwoHopLiveFailFramePeriodExceeded)
	}
	return reasons
}

func twoHopLiveOutcome(failReasons []string) string {
	if len(failReasons) == 0 {
		return TwoHopLiveOutcomePass
	}
	for _, reason := range failReasons {
		if reason == TwoHopLiveFailSourceSlotMiss ||
			reason == TwoHopLiveFailRelaySlotMiss ||
			reason == TwoHopLiveFailRelayReadinessMiss ||
			reason == TwoHopLiveFailCyclePeriodExceeded {
			return TwoHopLiveOutcomeFail
		}
	}
	return TwoHopLiveOutcomeDegraded
}

func twoHopLiveTimingRowID(
	bitrate int64,
	sourceFrameBytes int,
	relayFrameBytes int,
	sourceSlotMs int64,
	relaySlotMs int64,
	guardTimeMs int64,
) string {
	return fmt.Sprintf("thl_bps_%05d_src_%03d_rel_%03d_srcslot_%03d_relslot_%03d_guard_%02d",
		bitrate, sourceFrameBytes, relayFrameBytes, sourceSlotMs, relaySlotMs, guardTimeMs)
}

func twoHopLiveTimingHash(report TwoHopLiveTimingReport) (string, error) {
	hashable := report
	hashable.EventHash = ""
	b, err := json.Marshal(hashable)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func twoHopLiveTimingRowCount(cfg TwoHopLiveTimingConfig) int {
	return len(cfg.ChannelBitratesBps) *
		len(cfg.SourceFrameBytes) *
		len(cfg.RelayFrameBytes) *
		len(cfg.SourceSlotDurationsMs) *
		len(cfg.RelaySlotDurationsMs) *
		len(cfg.GuardTimesMs)
}
