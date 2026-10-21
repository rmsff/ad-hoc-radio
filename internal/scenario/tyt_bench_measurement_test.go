package scenario

import (
	"reflect"
	"strings"
	"testing"
)

func TestIT029_TYTBenchMeasurementDefaultReportDeterministic(t *testing.T) {
	first, err := RunTYTBenchMeasurementCalibration(DefaultTYTBenchMeasurementConfig())
	if err != nil {
		t.Fatalf("RunTYTBenchMeasurementCalibration failed: %v", err)
	}
	second, err := RunTYTBenchMeasurementCalibration(DefaultTYTBenchMeasurementConfig())
	if err != nil {
		t.Fatalf("second RunTYTBenchMeasurementCalibration failed: %v", err)
	}

	if first.ScenarioID != TYTBenchMeasurementScenarioID {
		t.Fatalf("ScenarioID = %q, want %q", first.ScenarioID, TYTBenchMeasurementScenarioID)
	}
	if first.MeasurementSource != TYTBenchMeasurementSourceFixtureOnly || !first.TestFixture {
		t.Fatalf("fixture provenance = %q/%v, want fixture_only/true", first.MeasurementSource, first.TestFixture)
	}
	if !first.AssumptionOnly || first.MeasuredHardwareCapability || first.HardwareClaimAllowed {
		t.Fatalf("claim gates = assumption:%v measured:%v allowed:%v, want true/false/false",
			first.AssumptionOnly, first.MeasuredHardwareCapability, first.HardwareClaimAllowed)
	}
	if first.CalibrationDecision != TYTBenchDecisionPrimary24ms || first.SelectedTupleSlotMs != 24 || first.TupleThresholdMs != 10 {
		t.Fatalf("decision/slot/threshold = %q/%d/%d, want %q/24/10",
			first.CalibrationDecision, first.SelectedTupleSlotMs, first.TupleThresholdMs, TYTBenchDecisionPrimary24ms)
	}
	if first.MaxRequiredTurnaroundP95Ms != 8 {
		t.Fatalf("MaxRequiredTurnaroundP95Ms = %d, want 8", first.MaxRequiredTurnaroundP95Ms)
	}
	if !reflect.DeepEqual(first.LimitingComponents, []string{tytBenchMetricRxToTx, tytBenchMetricTxToRx}) {
		t.Fatalf("LimitingComponents = %v, want [rx_to_tx tx_to_rx]", first.LimitingComponents)
	}
	if first.EventHash == "" || first.EventHash != second.EventHash {
		t.Fatalf("event hash mismatch: %q vs %q", first.EventHash, second.EventHash)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same config produced different TYT bench measurement reports")
	}
}

func TestIT029_TYTBenchMeasurementHardwareClaimGates(t *testing.T) {
	fixture, err := RunTYTBenchMeasurementCalibration(DefaultTYTBenchMeasurementConfig())
	if err != nil {
		t.Fatalf("fixture calibration failed: %v", err)
	}
	if fixture.HardwareClaimAllowed || fixture.MeasuredHardwareCapability || !fixture.AssumptionOnly {
		t.Fatalf("fixture gates = allowed:%v measured:%v assumption:%v, want false/false/true",
			fixture.HardwareClaimAllowed, fixture.MeasuredHardwareCapability, fixture.AssumptionOnly)
	}

	unknownCfg := DefaultTYTBenchMeasurementConfig()
	unknownCfg.MeasurementSource = TYTBenchMeasurementSourceUnknown
	unknownCfg.TestFixture = false
	unknown, err := RunTYTBenchMeasurementCalibration(unknownCfg)
	if err != nil {
		t.Fatalf("unknown calibration failed: %v", err)
	}
	if unknown.HardwareClaimAllowed || unknown.MeasuredHardwareCapability || !unknown.AssumptionOnly {
		t.Fatalf("unknown gates = allowed:%v measured:%v assumption:%v, want false/false/true",
			unknown.HardwareClaimAllowed, unknown.MeasuredHardwareCapability, unknown.AssumptionOnly)
	}

	measuredCfg := DefaultTYTBenchMeasurementConfig()
	measuredCfg.MeasurementSource = TYTBenchMeasurementSourceBenchMeasured
	measuredCfg.TestFixture = false
	measuredCfg.MeasurementArtifactRef = "bench/tyt-md-uv380/2026-07-01/switch-p95.json"
	measured, err := RunTYTBenchMeasurementCalibration(measuredCfg)
	if err != nil {
		t.Fatalf("bench_measured calibration failed: %v", err)
	}
	if !measured.HardwareClaimAllowed || !measured.MeasuredHardwareCapability || measured.AssumptionOnly {
		t.Fatalf("bench gates = allowed:%v measured:%v assumption:%v, want true/true/false",
			measured.HardwareClaimAllowed, measured.MeasuredHardwareCapability, measured.AssumptionOnly)
	}
}

func TestIT029_TYTBenchMeasurementThresholdBoundaries(t *testing.T) {
	tests := []struct {
		name      string
		p95       int64
		decision  string
		slotMs    int64
		threshold int64
	}{
		{name: "aggressive 16", p95: 2, decision: TYTBenchDecisionAggressive16ms, slotMs: 16, threshold: 2},
		{name: "intermediate 20", p95: 6, decision: TYTBenchDecisionIntermediate20ms, slotMs: 20, threshold: 6},
		{name: "primary 24", p95: 10, decision: TYTBenchDecisionPrimary24ms, slotMs: 24, threshold: 10},
		{name: "safe 30", p95: 16, decision: TYTBenchDecisionSafe30ms, slotMs: 30, threshold: 16},
		{name: "no live two hop", p95: 17, decision: TYTBenchDecisionNoLiveTwoHop},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tytBenchPrecomputedConfig(tc.p95, 1, 1)
			report, err := RunTYTBenchMeasurementCalibration(cfg)
			if err != nil {
				t.Fatalf("RunTYTBenchMeasurementCalibration failed: %v", err)
			}
			if report.CalibrationDecision != tc.decision || report.SelectedTupleSlotMs != tc.slotMs || report.TupleThresholdMs != tc.threshold {
				t.Fatalf("decision/slot/threshold = %q/%d/%d, want %q/%d/%d",
					report.CalibrationDecision, report.SelectedTupleSlotMs, report.TupleThresholdMs,
					tc.decision, tc.slotMs, tc.threshold)
			}
			if report.MaxRequiredTurnaroundP95Ms != tc.p95 {
				t.Fatalf("MaxRequiredTurnaroundP95Ms = %d, want %d", report.MaxRequiredTurnaroundP95Ms, tc.p95)
			}
		})
	}
}

func TestIT029_TYTBenchMeasurementNearestRankAndLimiters(t *testing.T) {
	cfg := DefaultTYTBenchMeasurementConfig()
	cfg.RxToTxSamplesMs = []int64{101, 1, 2, 100}
	cfg.TxToRxSamplesMs = []int64{4, 4, 4, 4}
	cfg.RxValidAfterTxSamplesMs = []int64{101, 2, 3, 100}

	report, err := RunTYTBenchMeasurementCalibration(cfg)
	if err != nil {
		t.Fatalf("RunTYTBenchMeasurementCalibration failed: %v", err)
	}
	if report.RxToTx.P50Ms != 2 || report.RxToTx.P95Ms != 101 {
		t.Fatalf("rx_to_tx p50/p95 = %d/%d, want 2/101", report.RxToTx.P50Ms, report.RxToTx.P95Ms)
	}
	wantLimiters := []string{tytBenchMetricRxToTx, tytBenchMetricRxValidAfterTx}
	if !reflect.DeepEqual(report.LimitingComponents, wantLimiters) {
		t.Fatalf("LimitingComponents = %v, want %v", report.LimitingComponents, wantLimiters)
	}
}

func TestIT029_TYTBenchMeasurementRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		cfg  TYTBenchMeasurementConfig
		want string
	}{
		{name: "unsupported source", cfg: func() TYTBenchMeasurementConfig {
			cfg := DefaultTYTBenchMeasurementConfig()
			cfg.MeasurementSource = "magic"
			return cfg
		}(), want: "measurement_source"},
		{name: "empty radio model", cfg: func() TYTBenchMeasurementConfig {
			cfg := DefaultTYTBenchMeasurementConfig()
			cfg.RadioModel = ""
			return cfg
		}(), want: "radio_model"},
		{name: "fixture source must be fixture", cfg: func() TYTBenchMeasurementConfig {
			cfg := DefaultTYTBenchMeasurementConfig()
			cfg.TestFixture = false
			return cfg
		}(), want: "fixture_only"},
		{name: "bench measured needs artifact", cfg: func() TYTBenchMeasurementConfig {
			cfg := DefaultTYTBenchMeasurementConfig()
			cfg.MeasurementSource = TYTBenchMeasurementSourceBenchMeasured
			cfg.TestFixture = false
			return cfg
		}(), want: "measurement_artifact_ref"},
		{name: "bench measured needs nonblank artifact", cfg: func() TYTBenchMeasurementConfig {
			cfg := DefaultTYTBenchMeasurementConfig()
			cfg.MeasurementSource = TYTBenchMeasurementSourceBenchMeasured
			cfg.TestFixture = false
			cfg.MeasurementArtifactRef = "   "
			return cfg
		}(), want: "measurement_artifact_ref"},
		{name: "fixture cannot claim bench measured", cfg: func() TYTBenchMeasurementConfig {
			cfg := DefaultTYTBenchMeasurementConfig()
			cfg.MeasurementSource = TYTBenchMeasurementSourceBenchMeasured
			cfg.MeasurementArtifactRef = "bench/fake.json"
			return cfg
		}(), want: "test_fixture=false"},
		{name: "unsupported channel", cfg: func() TYTBenchMeasurementConfig {
			cfg := DefaultTYTBenchMeasurementConfig()
			cfg.ChannelWidthHz = 50000
			return cfg
		}(), want: "channel_width_hz"},
		{name: "unsupported bitrate", cfg: func() TYTBenchMeasurementConfig {
			cfg := DefaultTYTBenchMeasurementConfig()
			cfg.GrossBitrateBps = 38400
			return cfg
		}(), want: "gross_bitrate_bps"},
		{name: "unsupported preamble", cfg: func() TYTBenchMeasurementConfig {
			cfg := DefaultTYTBenchMeasurementConfig()
			cfg.PreambleMs = 2
			return cfg
		}(), want: "preamble_ms"},
		{name: "unsupported sync", cfg: func() TYTBenchMeasurementConfig {
			cfg := DefaultTYTBenchMeasurementConfig()
			cfg.SyncMs = 2
			return cfg
		}(), want: "sync_ms"},
		{name: "unsupported link delay", cfg: func() TYTBenchMeasurementConfig {
			cfg := DefaultTYTBenchMeasurementConfig()
			cfg.LinkDelayMs = 2
			return cfg
		}(), want: "link_delay_ms"},
		{name: "negative sample", cfg: func() TYTBenchMeasurementConfig {
			cfg := DefaultTYTBenchMeasurementConfig()
			cfg.RxToTxSamplesMs = []int64{-1}
			return cfg
		}(), want: "rx_to_tx"},
		{name: "mixed raw and summary", cfg: func() TYTBenchMeasurementConfig {
			cfg := DefaultTYTBenchMeasurementConfig()
			value := int64(1)
			cfg.RxToTxP50Ms = &value
			return cfg
		}(), want: "cannot mix"},
		{name: "missing p95", cfg: func() TYTBenchMeasurementConfig {
			cfg := DefaultTYTBenchMeasurementConfig()
			cfg.RxToTxSamplesMs = nil
			value := int64(1)
			cfg.RxToTxP50Ms = &value
			return cfg
		}(), want: "rx_to_tx"},
		{name: "negative summary", cfg: func() TYTBenchMeasurementConfig {
			cfg := tytBenchPrecomputedConfig(2, 1, 1)
			value := int64(-1)
			cfg.TxToRxP95Ms = &value
			return cfg
		}(), want: "tx_to_rx"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RunTYTBenchMeasurementCalibration(tc.cfg)
			if err == nil {
				t.Fatal("RunTYTBenchMeasurementCalibration accepted invalid input")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want context %q", err.Error(), tc.want)
			}
		})
	}
}

func tytBenchPrecomputedConfig(rxToTxP95, txToRxP95, rxValidP95 int64) TYTBenchMeasurementConfig {
	cfg := DefaultTYTBenchMeasurementConfig()
	cfg.RxToTxSamplesMs = nil
	cfg.TxToRxSamplesMs = nil
	cfg.RxValidAfterTxSamplesMs = nil
	cfg.RxToTxP50Ms = int64Ptr(rxToTxP95)
	cfg.RxToTxP95Ms = int64Ptr(rxToTxP95)
	cfg.TxToRxP50Ms = int64Ptr(txToRxP95)
	cfg.TxToRxP95Ms = int64Ptr(txToRxP95)
	cfg.RxValidAfterTxP50Ms = int64Ptr(rxValidP95)
	cfg.RxValidAfterTxP95Ms = int64Ptr(rxValidP95)
	return cfg
}
