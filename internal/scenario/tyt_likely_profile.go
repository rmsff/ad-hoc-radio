package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const TYTLikelyProfileScenarioID = "tyt_likely_profile_simulation"

const (
	TYTLikelyOutcomePass = "pass"
	TYTLikelyOutcomeFail = "fail"
)

const (
	tytLikelyRadioFamily       = "TYT MD-UV380 / MD-UV3x0 class"
	tytLikelyChannelWidthHz    = 25000
	maxTYTLikelyProfileRows    = 1_000
	tytLikelyTupleAggressive   = "aggressive_tuple"
	tytLikelyTuplePrimary      = "primary_planning_tuple"
	tytLikelyTupleSafeFallback = "safe_fallback_tuple"
)

const tytLikelyFailPlanningTupleMissing = "planning_tuple_missing"

type TYTLikelyProfileConfig struct {
	ScenarioID             string
	Seed                   uint64
	RadioFamily            string
	ChannelWidthHz         int
	ChannelBitrateBps      int64
	SourceFrameBytes       int
	RelayFrameBytes        int
	PreambleMs             int64
	SyncMs                 int64
	LinkDelayMs            int64
	VoicePeriodMs          int64
	RequiredRelayDepth     int
	LatencyBudgetMs        int64
	FramePeriodBudgetMs    int64
	SlotMsCandidates       []int64
	TxRxSwitchMsCandidates []int64
	GuardTimeMsCandidates  []int64
}

type TYTLikelyProfileReport struct {
	ScenarioID           string                 `json:"scenario_id"`
	Seed                 uint64                 `json:"seed"`
	RadioFamily          string                 `json:"radio_family"`
	ChannelWidthHz       int                    `json:"channel_width_hz"`
	ChannelBitrateBps    int64                  `json:"channel_bitrate_bps"`
	SourceFrameBytes     int                    `json:"source_frame_bytes"`
	RelayFrameBytes      int                    `json:"relay_frame_bytes"`
	RequiredRelayDepth   int                    `json:"required_relay_depth"`
	AssumptionOnly       bool                   `json:"assumption_only"`
	Rows                 []TYTLikelyProfileRow  `json:"rows"`
	RowCount             int                    `json:"row_count"`
	PassRows             int                    `json:"pass_rows"`
	FailRows             int                    `json:"fail_rows"`
	AggressiveTuple      TYTLikelyPlanningTuple `json:"aggressive_tuple"`
	PrimaryPlanningTuple TYTLikelyPlanningTuple `json:"primary_planning_tuple"`
	SafeFallbackTuple    TYTLikelyPlanningTuple `json:"safe_fallback_tuple"`
	EventHash            string                 `json:"event_hash"`
}

type TYTLikelyProfileRow struct {
	RowID                      string                        `json:"row_id"`
	SlotMs                     int64                         `json:"slot_ms"`
	SourceSlotMs               int64                         `json:"source_slot_ms"`
	RelaySlotMs                int64                         `json:"relay_slot_ms"`
	TxRxSwitchMs               int64                         `json:"tx_rx_switch_ms"`
	GuardTimeMs                int64                         `json:"guard_time_ms"`
	PreambleMs                 int64                         `json:"preamble_ms"`
	SyncMs                     int64                         `json:"sync_ms"`
	LinkDelayMs                int64                         `json:"link_delay_ms"`
	VoicePeriodMs              int64                         `json:"voice_period_ms"`
	RequiredRelayDepth         int                           `json:"required_relay_depth"`
	SourceAirtimeMs            int64                         `json:"source_airtime_ms"`
	RelayAirtimeMs             int64                         `json:"relay_airtime_ms"`
	SourceTxDurationMs         int64                         `json:"source_tx_duration_ms"`
	RelayTxDurationMs          int64                         `json:"relay_tx_duration_ms"`
	SourceSlotUsedMs           int64                         `json:"source_slot_used_ms"`
	RelaySlotUsedMs            int64                         `json:"relay_slot_used_ms"`
	SourceSlotSpareMs          int64                         `json:"source_slot_spare_ms"`
	RelaySlotSpareMs           int64                         `json:"relay_slot_spare_ms"`
	RelayReadinessMissCount    int                           `json:"relay_readiness_miss_count"`
	CycleOccupiedMs            int64                         `json:"cycle_occupied_ms"`
	CycleSpareMs               int64                         `json:"cycle_spare_ms"`
	EndToEndLatencyMs          int64                         `json:"end_to_end_latency_ms"`
	Outcome                    string                        `json:"outcome"`
	FailReasons                []string                      `json:"fail_reasons"`
	ChannelBudget              GroupVoiceChannelBudgetReport `json:"channel_budget"`
	MeasuredHardwareCapability bool                          `json:"measured_hardware_capability"`
}

type TYTLikelyPlanningTuple struct {
	Name                       string   `json:"name"`
	SlotMs                     int64    `json:"slot_ms"`
	MaxTxRxSwitchMs            int64    `json:"max_tx_rx_switch_ms"`
	GuardTimeMs                int64    `json:"guard_time_ms"`
	EndToEndLatencyMs          int64    `json:"end_to_end_latency_ms"`
	CycleSpareMs               int64    `json:"cycle_spare_ms"`
	Outcome                    string   `json:"outcome"`
	FailReasons                []string `json:"fail_reasons"`
	MeasuredHardwareCapability bool     `json:"measured_hardware_capability"`
}

func DefaultTYTLikelyProfileConfig() TYTLikelyProfileConfig {
	return TYTLikelyProfileConfig{
		ScenarioID:             TYTLikelyProfileScenarioID,
		Seed:                   DefaultScenarioSeed,
		RadioFamily:            tytLikelyRadioFamily,
		ChannelWidthHz:         tytLikelyChannelWidthHz,
		ChannelBitrateBps:      19200,
		SourceFrameBytes:       defaultTwoHopLiveShortSourceFrameBytes,
		RelayFrameBytes:        defaultTwoHopLiveRelayFrameBytes,
		PreambleMs:             1,
		SyncMs:                 1,
		LinkDelayMs:            1,
		VoicePeriodMs:          defaultTwoHopLiveVoicePeriodMs,
		RequiredRelayDepth:     2,
		LatencyBudgetMs:        defaultLimitedMultiHopLatencyMs,
		FramePeriodBudgetMs:    DefaultFramePeriodMs,
		SlotMsCandidates:       []int64{16, 20, 24, 30},
		TxRxSwitchMsCandidates: []int64{2, 6, 10, 16},
		GuardTimeMsCandidates:  []int64{1, 2, 5},
	}
}

func RunTYTLikelyProfileSimulation(cfg TYTLikelyProfileConfig) (TYTLikelyProfileReport, error) {
	cfg = normalizeTYTLikelyProfileConfig(cfg)
	if err := validateTYTLikelyProfileConfig(cfg); err != nil {
		return TYTLikelyProfileReport{}, err
	}
	rowCount, err := tytLikelyRowCount(cfg)
	if err != nil {
		return TYTLikelyProfileReport{}, err
	}

	report := TYTLikelyProfileReport{
		ScenarioID:         cfg.ScenarioID,
		Seed:               cfg.Seed,
		RadioFamily:        cfg.RadioFamily,
		ChannelWidthHz:     cfg.ChannelWidthHz,
		ChannelBitrateBps:  cfg.ChannelBitrateBps,
		SourceFrameBytes:   cfg.SourceFrameBytes,
		RelayFrameBytes:    cfg.RelayFrameBytes,
		RequiredRelayDepth: cfg.RequiredRelayDepth,
		AssumptionOnly:     true,
		Rows:               make([]TYTLikelyProfileRow, 0, rowCount),
	}

	for _, slotMs := range cfg.SlotMsCandidates {
		for _, switchMs := range cfg.TxRxSwitchMsCandidates {
			for _, guardMs := range cfg.GuardTimeMsCandidates {
				row, err := tytLikelyProfileRow(cfg, slotMs, switchMs, guardMs)
				if err != nil {
					return TYTLikelyProfileReport{}, err
				}
				report.Rows = append(report.Rows, row)
				switch row.Outcome {
				case TYTLikelyOutcomePass:
					report.PassRows++
				case TYTLikelyOutcomeFail:
					report.FailRows++
				default:
					return TYTLikelyProfileReport{}, fmt.Errorf("tyt likely profile: unknown outcome %q", row.Outcome)
				}
			}
		}
	}
	report.RowCount = len(report.Rows)
	report.AggressiveTuple, err = tytLikelyPlanningTuple(tytLikelyTupleAggressive, report.Rows, 16, 2, 1)
	if err != nil {
		return TYTLikelyProfileReport{}, err
	}
	report.PrimaryPlanningTuple, err = tytLikelyPlanningTuple(tytLikelyTuplePrimary, report.Rows, 24, 10, 1)
	if err != nil {
		return TYTLikelyProfileReport{}, err
	}
	report.SafeFallbackTuple, err = tytLikelyPlanningTuple(tytLikelyTupleSafeFallback, report.Rows, 30, 16, 1)
	if err != nil {
		return TYTLikelyProfileReport{}, err
	}

	hash, err := tytLikelyProfileHash(report)
	if err != nil {
		return TYTLikelyProfileReport{}, err
	}
	report.EventHash = hash
	return report, nil
}

func normalizeTYTLikelyProfileConfig(cfg TYTLikelyProfileConfig) TYTLikelyProfileConfig {
	def := DefaultTYTLikelyProfileConfig()
	if cfg.ScenarioID == "" {
		cfg.ScenarioID = def.ScenarioID
	}
	if cfg.Seed == 0 {
		cfg.Seed = def.Seed
	}
	if cfg.RadioFamily == "" {
		cfg.RadioFamily = def.RadioFamily
	}
	if cfg.ChannelWidthHz == 0 {
		cfg.ChannelWidthHz = def.ChannelWidthHz
	}
	if cfg.SourceFrameBytes == 0 {
		cfg.SourceFrameBytes = def.SourceFrameBytes
	}
	if cfg.RelayFrameBytes == 0 {
		cfg.RelayFrameBytes = def.RelayFrameBytes
	}
	if cfg.PreambleMs == 0 {
		cfg.PreambleMs = def.PreambleMs
	}
	if cfg.SyncMs == 0 {
		cfg.SyncMs = def.SyncMs
	}
	if cfg.LinkDelayMs == 0 {
		cfg.LinkDelayMs = def.LinkDelayMs
	}
	if cfg.VoicePeriodMs == 0 {
		cfg.VoicePeriodMs = def.VoicePeriodMs
	}
	if cfg.RequiredRelayDepth == 0 {
		cfg.RequiredRelayDepth = def.RequiredRelayDepth
	}
	if cfg.LatencyBudgetMs == 0 {
		cfg.LatencyBudgetMs = def.LatencyBudgetMs
	}
	if cfg.FramePeriodBudgetMs == 0 {
		cfg.FramePeriodBudgetMs = def.FramePeriodBudgetMs
	}
	if cfg.SlotMsCandidates == nil {
		cfg.SlotMsCandidates = cloneMultiHopTimingInt64s(def.SlotMsCandidates)
	} else {
		cfg.SlotMsCandidates = sortedMultiHopTimingInt64s(cfg.SlotMsCandidates)
	}
	if cfg.TxRxSwitchMsCandidates == nil {
		cfg.TxRxSwitchMsCandidates = cloneMultiHopTimingInt64s(def.TxRxSwitchMsCandidates)
	} else {
		cfg.TxRxSwitchMsCandidates = sortedMultiHopTimingInt64s(cfg.TxRxSwitchMsCandidates)
	}
	if cfg.GuardTimeMsCandidates == nil {
		cfg.GuardTimeMsCandidates = cloneMultiHopTimingInt64s(def.GuardTimeMsCandidates)
	} else {
		cfg.GuardTimeMsCandidates = sortedMultiHopTimingInt64s(cfg.GuardTimeMsCandidates)
	}
	return cfg
}

func validateTYTLikelyProfileConfig(cfg TYTLikelyProfileConfig) error {
	if cfg.ScenarioID == "" {
		return fmt.Errorf("tyt likely profile: scenario_id must not be empty")
	}
	if cfg.RadioFamily == "" {
		return fmt.Errorf("tyt likely profile: radio_family must not be empty")
	}
	if cfg.ChannelWidthHz <= 0 {
		return fmt.Errorf("tyt likely profile: channel_width_hz=%d must be > 0", cfg.ChannelWidthHz)
	}
	if cfg.ChannelBitrateBps <= 0 {
		return fmt.Errorf("tyt likely profile: channel_bitrate_bps=%d must be > 0", cfg.ChannelBitrateBps)
	}
	if cfg.SourceFrameBytes <= 0 || cfg.RelayFrameBytes <= 0 {
		return fmt.Errorf("tyt likely profile: source and relay frame sizes must be > 0")
	}
	if cfg.PreambleMs < 0 || cfg.SyncMs < 0 || cfg.LinkDelayMs < 0 {
		return fmt.Errorf("tyt likely profile: preamble_ms, sync_ms, and link_delay_ms must be >= 0")
	}
	if cfg.VoicePeriodMs <= 0 {
		return fmt.Errorf("tyt likely profile: voice_period_ms=%d must be > 0", cfg.VoicePeriodMs)
	}
	if cfg.RequiredRelayDepth <= 0 {
		return fmt.Errorf("tyt likely profile: required_relay_depth=%d must be > 0", cfg.RequiredRelayDepth)
	}
	if cfg.LatencyBudgetMs <= 0 || cfg.FramePeriodBudgetMs <= 0 {
		return fmt.Errorf("tyt likely profile: latency and frame-period budgets must be > 0")
	}
	if err := validatePositiveInt64s("slot_ms_candidates", cfg.SlotMsCandidates); err != nil {
		return err
	}
	if err := validateNonNegativeInt64s("tx_rx_switch_ms_candidates", cfg.TxRxSwitchMsCandidates); err != nil {
		return err
	}
	if err := validateNonNegativeInt64s("guard_time_ms_candidates", cfg.GuardTimeMsCandidates); err != nil {
		return err
	}
	rows, err := tytLikelyRowCount(cfg)
	if err != nil {
		return err
	}
	if rows <= 0 {
		return fmt.Errorf("tyt likely profile: row count must be > 0")
	}
	if rows > maxTYTLikelyProfileRows {
		return fmt.Errorf("tyt likely profile: row count %d exceeds cap %d", rows, maxTYTLikelyProfileRows)
	}
	return nil
}

func tytLikelyProfileRow(cfg TYTLikelyProfileConfig, slotMs int64, txRxSwitchMs int64, guardMs int64) (TYTLikelyProfileRow, error) {
	profile := GroupVoiceProfile{
		Name:                "tyt_likely",
		QualityRank:         0,
		MaxRelayDepth:       cfg.RequiredRelayDepth,
		ChannelBitrateBps:   cfg.ChannelBitrateBps,
		SourceFrameBytes:    cfg.SourceFrameBytes,
		RelayFrameBytes:     cfg.RelayFrameBytes,
		SourceWireMode:      TwoHopLiveSourceWireShort,
		PreambleMs:          cfg.PreambleMs,
		SyncMs:              cfg.SyncMs,
		TxRxSwitchMs:        txRxSwitchMs,
		SourceSlotMs:        slotMs,
		RelaySlotMs:         slotMs,
		GuardTimeMs:         guardMs,
		VoicePeriodMs:       cfg.VoicePeriodMs,
		LinkDelayMs:         cfg.LinkDelayMs,
		LatencyBudgetMs:     cfg.LatencyBudgetMs,
		FramePeriodBudgetMs: cfg.FramePeriodBudgetMs,
	}
	budget, err := groupVoiceChannelBudget(profile, cfg.RequiredRelayDepth)
	if err != nil {
		return TYTLikelyProfileRow{}, err
	}
	outcome := TYTLikelyOutcomeFail
	if budget.OverallFits {
		outcome = TYTLikelyOutcomePass
	}
	return TYTLikelyProfileRow{
		RowID:                      tytLikelyRowID(slotMs, txRxSwitchMs, guardMs),
		SlotMs:                     slotMs,
		SourceSlotMs:               slotMs,
		RelaySlotMs:                slotMs,
		TxRxSwitchMs:               txRxSwitchMs,
		GuardTimeMs:                guardMs,
		PreambleMs:                 cfg.PreambleMs,
		SyncMs:                     cfg.SyncMs,
		LinkDelayMs:                cfg.LinkDelayMs,
		VoicePeriodMs:              cfg.VoicePeriodMs,
		RequiredRelayDepth:         cfg.RequiredRelayDepth,
		SourceAirtimeMs:            budget.SourceAirtimeMs,
		RelayAirtimeMs:             budget.RelayAirtimeMs,
		SourceTxDurationMs:         budget.SourceTxDurationMs,
		RelayTxDurationMs:          budget.RelayTxDurationMs,
		SourceSlotUsedMs:           budget.SourceSlotUsedMs,
		RelaySlotUsedMs:            budget.RelaySlotUsedMs,
		SourceSlotSpareMs:          budget.SourceSlotSpareMs,
		RelaySlotSpareMs:           budget.RelaySlotSpareMs,
		RelayReadinessMissCount:    budget.RelayReadinessMissCount,
		CycleOccupiedMs:            budget.CycleOccupiedMs,
		CycleSpareMs:               budget.CycleSpareMs,
		EndToEndLatencyMs:          budget.EndToEndLatencyMs,
		Outcome:                    outcome,
		FailReasons:                cloneMultiHopTimingStrings(budget.FailReasons),
		ChannelBudget:              budget,
		MeasuredHardwareCapability: false,
	}, nil
}

func tytLikelyPlanningTuple(name string, rows []TYTLikelyProfileRow, slotMs, maxSwitchMs, guardMs int64) (TYTLikelyPlanningTuple, error) {
	for _, row := range rows {
		if row.SlotMs != slotMs || row.TxRxSwitchMs != maxSwitchMs || row.GuardTimeMs != guardMs {
			continue
		}
		return TYTLikelyPlanningTuple{
			Name:                       name,
			SlotMs:                     slotMs,
			MaxTxRxSwitchMs:            maxSwitchMs,
			GuardTimeMs:                guardMs,
			EndToEndLatencyMs:          row.EndToEndLatencyMs,
			CycleSpareMs:               row.CycleSpareMs,
			Outcome:                    row.Outcome,
			FailReasons:                cloneMultiHopTimingStrings(row.FailReasons),
			MeasuredHardwareCapability: false,
		}, nil
	}
	return TYTLikelyPlanningTuple{}, fmt.Errorf(
		"tyt likely profile: %s row missing for slot_ms=%d tx_rx_switch_ms=%d guard_time_ms=%d (%s)",
		name,
		slotMs,
		maxSwitchMs,
		guardMs,
		tytLikelyFailPlanningTupleMissing,
	)
}

func tytLikelyRowID(slotMs, txRxSwitchMs, guardMs int64) string {
	return fmt.Sprintf("tyt_slot_%03d_switch_%03d_guard_%02d", slotMs, txRxSwitchMs, guardMs)
}

func tytLikelyRowCount(cfg TYTLikelyProfileConfig) (int, error) {
	count, err := checkedTYTLikelyLenProduct(1, len(cfg.SlotMsCandidates))
	if err != nil {
		return 0, err
	}
	count, err = checkedTYTLikelyLenProduct(count, len(cfg.TxRxSwitchMsCandidates))
	if err != nil {
		return 0, err
	}
	return checkedTYTLikelyLenProduct(count, len(cfg.GuardTimeMsCandidates))
}

func checkedTYTLikelyLenProduct(a, b int) (int, error) {
	if a == 0 || b == 0 {
		return 0, nil
	}
	maxInt := int(^uint(0) >> 1)
	if a > maxInt/b {
		return 0, fmt.Errorf("tyt likely profile: row count overflows int")
	}
	return a * b, nil
}

func tytLikelyProfileHash(report TYTLikelyProfileReport) (string, error) {
	hashable := report
	hashable.EventHash = ""
	b, err := json.Marshal(hashable)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
