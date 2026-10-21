package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const TYTBenchMeasurementScenarioID = "tyt_tx_rx_switch_bench_measurement"

const (
	TYTBenchMeasurementSourceFixtureOnly   = "fixture_only"
	TYTBenchMeasurementSourceBenchMeasured = "bench_measured"
	TYTBenchMeasurementSourceUnknown       = "unknown"
)

const (
	TYTBenchSummarySourceRawSamples  = "raw_samples"
	TYTBenchSummarySourcePrecomputed = "precomputed"
)

const (
	TYTBenchDecisionAggressive16ms   = "aggressive_16ms_viable"
	TYTBenchDecisionIntermediate20ms = "intermediate_20ms_viable"
	TYTBenchDecisionPrimary24ms      = "primary_24ms_viable"
	TYTBenchDecisionSafe30ms         = "safe_30ms_viable"
	TYTBenchDecisionNoLiveTwoHop     = "no_live_two_hop_tuple"
)

const (
	tytBenchMetricRxToTx         = "rx_to_tx"
	tytBenchMetricTxToRx         = "tx_to_rx"
	tytBenchMetricRxValidAfterTx = "rx_valid_after_tx"
)

type TYTBenchMeasurementConfig struct {
	ScenarioID              string  `json:"scenario_id"`
	Seed                    uint64  `json:"seed"`
	MeasurementSource       string  `json:"measurement_source"`
	RadioModel              string  `json:"radio_model"`
	TestFixture             bool    `json:"test_fixture"`
	MeasurementArtifactRef  string  `json:"measurement_artifact_ref"`
	FirmwarePath            string  `json:"firmware_path"`
	ChannelWidthHz          int     `json:"channel_width_hz"`
	GrossBitrateBps         int64   `json:"gross_bitrate_bps"`
	PreambleMs              int64   `json:"preamble_ms"`
	SyncMs                  int64   `json:"sync_ms"`
	LinkDelayMs             int64   `json:"link_delay_ms"`
	RxToTxSamplesMs         []int64 `json:"rx_to_tx_samples_ms"`
	TxToRxSamplesMs         []int64 `json:"tx_to_rx_samples_ms"`
	RxValidAfterTxSamplesMs []int64 `json:"rx_valid_after_tx_samples_ms"`
	RxToTxP50Ms             *int64  `json:"rx_to_tx_p50_ms"`
	RxToTxP95Ms             *int64  `json:"rx_to_tx_p95_ms"`
	TxToRxP50Ms             *int64  `json:"tx_to_rx_p50_ms"`
	TxToRxP95Ms             *int64  `json:"tx_to_rx_p95_ms"`
	RxValidAfterTxP50Ms     *int64  `json:"rx_valid_after_tx_p50_ms"`
	RxValidAfterTxP95Ms     *int64  `json:"rx_valid_after_tx_p95_ms"`
	Notes                   string  `json:"notes"`
}

type TYTBenchMeasurementReport struct {
	ScenarioID                 string                `json:"scenario_id"`
	Seed                       uint64                `json:"seed"`
	MeasurementSource          string                `json:"measurement_source"`
	RadioModel                 string                `json:"radio_model"`
	TestFixture                bool                  `json:"test_fixture"`
	MeasurementArtifactRef     string                `json:"measurement_artifact_ref"`
	FirmwarePath               string                `json:"firmware_path"`
	ChannelWidthHz             int                   `json:"channel_width_hz"`
	GrossBitrateBps            int64                 `json:"gross_bitrate_bps"`
	PreambleMs                 int64                 `json:"preamble_ms"`
	SyncMs                     int64                 `json:"sync_ms"`
	LinkDelayMs                int64                 `json:"link_delay_ms"`
	AssumptionOnly             bool                  `json:"assumption_only"`
	MeasuredHardwareCapability bool                  `json:"measured_hardware_capability"`
	HardwareClaimAllowed       bool                  `json:"hardware_claim_allowed"`
	RxToTx                     TYTBenchMetricSummary `json:"rx_to_tx"`
	TxToRx                     TYTBenchMetricSummary `json:"tx_to_rx"`
	RxValidAfterTx             TYTBenchMetricSummary `json:"rx_valid_after_tx"`
	MaxRequiredTurnaroundP95Ms int64                 `json:"max_required_turnaround_p95_ms"`
	LimitingComponents         []string              `json:"limiting_components"`
	CalibrationDecision        string                `json:"calibration_decision"`
	SelectedTupleSlotMs        int64                 `json:"selected_tuple_slot_ms"`
	TupleThresholdMs           int64                 `json:"tuple_threshold_ms"`
	Notes                      string                `json:"notes"`
	EventHash                  string                `json:"event_hash"`
}

type TYTBenchMetricSummary struct {
	Name          string `json:"name"`
	P50Ms         int64  `json:"p50_ms"`
	P95Ms         int64  `json:"p95_ms"`
	SummarySource string `json:"summary_source"`
	SampleCount   int    `json:"sample_count"`
}

func DefaultTYTBenchMeasurementConfig() TYTBenchMeasurementConfig {
	return TYTBenchMeasurementConfig{
		ScenarioID:              TYTBenchMeasurementScenarioID,
		Seed:                    DefaultScenarioSeed,
		MeasurementSource:       TYTBenchMeasurementSourceFixtureOnly,
		RadioModel:              tytLikelyRadioFamily,
		TestFixture:             true,
		FirmwarePath:            "openrtx_derived_bench",
		ChannelWidthHz:          tytLikelyChannelWidthHz,
		GrossBitrateBps:         19200,
		PreambleMs:              1,
		SyncMs:                  1,
		LinkDelayMs:             1,
		RxToTxSamplesMs:         []int64{6, 8, 8},
		TxToRxSamplesMs:         []int64{7, 8, 8},
		RxValidAfterTxSamplesMs: []int64{1, 1, 2},
		Notes:                   "fixture-only test data, not a hardware claim",
	}
}

func RunTYTBenchMeasurementCalibration(cfg TYTBenchMeasurementConfig) (TYTBenchMeasurementReport, error) {
	cfg = normalizeTYTBenchMeasurementConfig(cfg)
	if err := validateTYTBenchMeasurementConfig(cfg); err != nil {
		return TYTBenchMeasurementReport{}, err
	}

	rxToTx, err := tytBenchMetricSummary(tytBenchMetricRxToTx, cfg.RxToTxSamplesMs, cfg.RxToTxP50Ms, cfg.RxToTxP95Ms)
	if err != nil {
		return TYTBenchMeasurementReport{}, err
	}
	txToRx, err := tytBenchMetricSummary(tytBenchMetricTxToRx, cfg.TxToRxSamplesMs, cfg.TxToRxP50Ms, cfg.TxToRxP95Ms)
	if err != nil {
		return TYTBenchMeasurementReport{}, err
	}
	rxValidAfterTx, err := tytBenchMetricSummary(
		tytBenchMetricRxValidAfterTx,
		cfg.RxValidAfterTxSamplesMs,
		cfg.RxValidAfterTxP50Ms,
		cfg.RxValidAfterTxP95Ms,
	)
	if err != nil {
		return TYTBenchMeasurementReport{}, err
	}

	maxP95, limiting := tytBenchLimitingComponents(rxToTx, txToRx, rxValidAfterTx)
	decision, slotMs, thresholdMs := tytBenchCalibrationDecision(maxP95)
	hardwareClaimAllowed := cfg.MeasurementSource == TYTBenchMeasurementSourceBenchMeasured &&
		!cfg.TestFixture &&
		cfg.MeasurementArtifactRef != ""

	report := TYTBenchMeasurementReport{
		ScenarioID:                 cfg.ScenarioID,
		Seed:                       cfg.Seed,
		MeasurementSource:          cfg.MeasurementSource,
		RadioModel:                 cfg.RadioModel,
		TestFixture:                cfg.TestFixture,
		MeasurementArtifactRef:     cfg.MeasurementArtifactRef,
		FirmwarePath:               cfg.FirmwarePath,
		ChannelWidthHz:             cfg.ChannelWidthHz,
		GrossBitrateBps:            cfg.GrossBitrateBps,
		PreambleMs:                 cfg.PreambleMs,
		SyncMs:                     cfg.SyncMs,
		LinkDelayMs:                cfg.LinkDelayMs,
		AssumptionOnly:             !hardwareClaimAllowed,
		MeasuredHardwareCapability: hardwareClaimAllowed,
		HardwareClaimAllowed:       hardwareClaimAllowed,
		RxToTx:                     rxToTx,
		TxToRx:                     txToRx,
		RxValidAfterTx:             rxValidAfterTx,
		MaxRequiredTurnaroundP95Ms: maxP95,
		LimitingComponents:         limiting,
		CalibrationDecision:        decision,
		SelectedTupleSlotMs:        slotMs,
		TupleThresholdMs:           thresholdMs,
		Notes:                      cfg.Notes,
	}

	hash, err := tytBenchMeasurementHash(report)
	if err != nil {
		return TYTBenchMeasurementReport{}, err
	}
	report.EventHash = hash
	return report, nil
}

func (r TYTBenchMeasurementReport) MarshalReportJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

func normalizeTYTBenchMeasurementConfig(cfg TYTBenchMeasurementConfig) TYTBenchMeasurementConfig {
	if cfg.ScenarioID == "" {
		cfg.ScenarioID = TYTBenchMeasurementScenarioID
	}
	if cfg.Seed == 0 {
		cfg.Seed = DefaultScenarioSeed
	}
	cfg.MeasurementArtifactRef = strings.TrimSpace(cfg.MeasurementArtifactRef)
	return cfg
}

func validateTYTBenchMeasurementConfig(cfg TYTBenchMeasurementConfig) error {
	if cfg.ScenarioID != TYTBenchMeasurementScenarioID {
		return fmt.Errorf("tyt bench measurement: unsupported scenario_id %q", cfg.ScenarioID)
	}
	switch cfg.MeasurementSource {
	case TYTBenchMeasurementSourceFixtureOnly:
		if !cfg.TestFixture {
			return fmt.Errorf("tyt bench measurement: fixture_only requires test_fixture=true")
		}
	case TYTBenchMeasurementSourceBenchMeasured:
		if cfg.TestFixture {
			return fmt.Errorf("tyt bench measurement: bench_measured requires test_fixture=false")
		}
		if cfg.MeasurementArtifactRef == "" {
			return fmt.Errorf("tyt bench measurement: bench_measured requires measurement_artifact_ref")
		}
	case TYTBenchMeasurementSourceUnknown:
	default:
		return fmt.Errorf("tyt bench measurement: unsupported measurement_source %q", cfg.MeasurementSource)
	}
	if cfg.RadioModel == "" {
		return fmt.Errorf("tyt bench measurement: radio_model must not be empty")
	}
	if cfg.ChannelWidthHz != 12500 && cfg.ChannelWidthHz != 25000 {
		return fmt.Errorf("tyt bench measurement: channel_width_hz=%d unsupported for MVP-8.3", cfg.ChannelWidthHz)
	}
	if cfg.GrossBitrateBps != 9600 && cfg.GrossBitrateBps != 19200 {
		return fmt.Errorf("tyt bench measurement: gross_bitrate_bps=%d unsupported for MVP-8.3", cfg.GrossBitrateBps)
	}
	if cfg.PreambleMs != 1 {
		return fmt.Errorf("tyt bench measurement: preamble_ms=%d unsupported for MVP-8.3", cfg.PreambleMs)
	}
	if cfg.SyncMs != 1 {
		return fmt.Errorf("tyt bench measurement: sync_ms=%d unsupported for MVP-8.3", cfg.SyncMs)
	}
	if cfg.LinkDelayMs != 1 {
		return fmt.Errorf("tyt bench measurement: link_delay_ms=%d unsupported for MVP-8.3", cfg.LinkDelayMs)
	}
	if err := validateTYTBenchMetricInput(tytBenchMetricRxToTx, cfg.RxToTxSamplesMs, cfg.RxToTxP50Ms, cfg.RxToTxP95Ms); err != nil {
		return err
	}
	if err := validateTYTBenchMetricInput(tytBenchMetricTxToRx, cfg.TxToRxSamplesMs, cfg.TxToRxP50Ms, cfg.TxToRxP95Ms); err != nil {
		return err
	}
	return validateTYTBenchMetricInput(
		tytBenchMetricRxValidAfterTx,
		cfg.RxValidAfterTxSamplesMs,
		cfg.RxValidAfterTxP50Ms,
		cfg.RxValidAfterTxP95Ms,
	)
}

func validateTYTBenchMetricInput(name string, samples []int64, p50 *int64, p95 *int64) error {
	for _, sample := range samples {
		if sample < 0 {
			return fmt.Errorf("tyt bench measurement: %s sample %d must be >= 0", name, sample)
		}
	}
	if p50 != nil && *p50 < 0 {
		return fmt.Errorf("tyt bench measurement: %s p50_ms=%d must be >= 0", name, *p50)
	}
	if p95 != nil && *p95 < 0 {
		return fmt.Errorf("tyt bench measurement: %s p95_ms=%d must be >= 0", name, *p95)
	}
	if len(samples) > 0 {
		if p50 != nil || p95 != nil {
			return fmt.Errorf("tyt bench measurement: %s cannot mix raw samples and precomputed summary", name)
		}
		return nil
	}
	if p50 == nil || p95 == nil {
		return fmt.Errorf("tyt bench measurement: %s requires p50_ms and p95_ms summary or raw samples", name)
	}
	return nil
}

func tytBenchMetricSummary(name string, samples []int64, p50 *int64, p95 *int64) (TYTBenchMetricSummary, error) {
	if len(samples) > 0 {
		p50Value, err := tytBenchNearestRank(samples, 50)
		if err != nil {
			return TYTBenchMetricSummary{}, err
		}
		p95Value, err := tytBenchNearestRank(samples, 95)
		if err != nil {
			return TYTBenchMetricSummary{}, err
		}
		return TYTBenchMetricSummary{
			Name:          name,
			P50Ms:         p50Value,
			P95Ms:         p95Value,
			SummarySource: TYTBenchSummarySourceRawSamples,
			SampleCount:   len(samples),
		}, nil
	}
	if p50 == nil || p95 == nil {
		return TYTBenchMetricSummary{}, fmt.Errorf("tyt bench measurement: %s missing summary", name)
	}
	return TYTBenchMetricSummary{
		Name:          name,
		P50Ms:         *p50,
		P95Ms:         *p95,
		SummarySource: TYTBenchSummarySourcePrecomputed,
		SampleCount:   0,
	}, nil
}

func tytBenchNearestRank(samples []int64, percentile int) (int64, error) {
	if len(samples) == 0 {
		return 0, fmt.Errorf("tyt bench measurement: nearest-rank percentile needs samples")
	}
	if percentile <= 0 || percentile > 100 {
		return 0, fmt.Errorf("tyt bench measurement: percentile=%d must be in 1..100", percentile)
	}
	ordered := cloneMultiHopTimingInt64s(samples)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	index := (percentile*len(ordered) + 99) / 100
	if index < 1 {
		index = 1
	}
	return ordered[index-1], nil
}

func tytBenchLimitingComponents(metrics ...TYTBenchMetricSummary) (int64, []string) {
	var max int64
	for i, metric := range metrics {
		if i == 0 || metric.P95Ms > max {
			max = metric.P95Ms
		}
	}
	limiting := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		if metric.P95Ms == max {
			limiting = append(limiting, metric.Name)
		}
	}
	sort.Strings(limiting)
	return max, limiting
}

func tytBenchCalibrationDecision(maxP95Ms int64) (string, int64, int64) {
	switch {
	case maxP95Ms <= 2:
		return TYTBenchDecisionAggressive16ms, 16, 2
	case maxP95Ms <= 6:
		return TYTBenchDecisionIntermediate20ms, 20, 6
	case maxP95Ms <= 10:
		return TYTBenchDecisionPrimary24ms, 24, 10
	case maxP95Ms <= 16:
		return TYTBenchDecisionSafe30ms, 30, 16
	default:
		return TYTBenchDecisionNoLiveTwoHop, 0, 0
	}
}

func tytBenchMeasurementHash(report TYTBenchMeasurementReport) (string, error) {
	hashable := report
	hashable.EventHash = ""
	b, err := json.Marshal(hashable)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
