package scenario

import (
	"reflect"
	"strings"
	"testing"
)

func TestIT026_TYTLikelyProfileDefaultReportDeterministic(t *testing.T) {
	first, err := RunTYTLikelyProfileSimulation(DefaultTYTLikelyProfileConfig())
	if err != nil {
		t.Fatalf("RunTYTLikelyProfileSimulation failed: %v", err)
	}
	second, err := RunTYTLikelyProfileSimulation(DefaultTYTLikelyProfileConfig())
	if err != nil {
		t.Fatalf("second RunTYTLikelyProfileSimulation failed: %v", err)
	}

	if first.ScenarioID != TYTLikelyProfileScenarioID {
		t.Fatalf("ScenarioID = %q, want %q", first.ScenarioID, TYTLikelyProfileScenarioID)
	}
	if first.RowCount != 48 || len(first.Rows) != first.RowCount {
		t.Fatalf("row count = %d len=%d, want 48", first.RowCount, len(first.Rows))
	}
	if first.ChannelWidthHz != 25000 || first.ChannelBitrateBps != 19200 {
		t.Fatalf("channel width/bitrate = %d/%d, want 25000/19200", first.ChannelWidthHz, first.ChannelBitrateBps)
	}
	if first.SourceFrameBytes != 22 || first.RelayFrameBytes != 22 || first.RequiredRelayDepth != 2 {
		t.Fatalf("frame bytes/depth = %d/%d/%d, want 22/22/2",
			first.SourceFrameBytes, first.RelayFrameBytes, first.RequiredRelayDepth)
	}
	if !first.AssumptionOnly {
		t.Fatal("AssumptionOnly = false, want true because this is not measured TYT capability")
	}
	if first.EventHash == "" || first.EventHash != second.EventHash {
		t.Fatalf("event hash mismatch: %q vs %q", first.EventHash, second.EventHash)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same config produced different TYT likely reports")
	}
}

func TestIT026_TYTLikelyThresholdRows(t *testing.T) {
	report, err := RunTYTLikelyProfileSimulation(DefaultTYTLikelyProfileConfig())
	if err != nil {
		t.Fatalf("RunTYTLikelyProfileSimulation failed: %v", err)
	}

	tests := []struct {
		slotMs            int64
		txSwitchMs        int64
		wantOutcome       string
		wantReadinessMiss bool
	}{
		{slotMs: 16, txSwitchMs: 2, wantOutcome: TYTLikelyOutcomePass},
		{slotMs: 16, txSwitchMs: 6, wantOutcome: TYTLikelyOutcomeFail, wantReadinessMiss: true},
		{slotMs: 16, txSwitchMs: 10, wantOutcome: TYTLikelyOutcomeFail, wantReadinessMiss: true},
		{slotMs: 16, txSwitchMs: 16, wantOutcome: TYTLikelyOutcomeFail, wantReadinessMiss: true},
		{slotMs: 20, txSwitchMs: 2, wantOutcome: TYTLikelyOutcomePass},
		{slotMs: 20, txSwitchMs: 6, wantOutcome: TYTLikelyOutcomePass},
		{slotMs: 20, txSwitchMs: 10, wantOutcome: TYTLikelyOutcomeFail, wantReadinessMiss: true},
		{slotMs: 20, txSwitchMs: 16, wantOutcome: TYTLikelyOutcomeFail, wantReadinessMiss: true},
		{slotMs: 24, txSwitchMs: 2, wantOutcome: TYTLikelyOutcomePass},
		{slotMs: 24, txSwitchMs: 6, wantOutcome: TYTLikelyOutcomePass},
		{slotMs: 24, txSwitchMs: 10, wantOutcome: TYTLikelyOutcomePass},
		{slotMs: 24, txSwitchMs: 16, wantOutcome: TYTLikelyOutcomeFail, wantReadinessMiss: true},
		{slotMs: 30, txSwitchMs: 2, wantOutcome: TYTLikelyOutcomePass},
		{slotMs: 30, txSwitchMs: 6, wantOutcome: TYTLikelyOutcomePass},
		{slotMs: 30, txSwitchMs: 10, wantOutcome: TYTLikelyOutcomePass},
		{slotMs: 30, txSwitchMs: 16, wantOutcome: TYTLikelyOutcomePass},
	}
	for _, tc := range tests {
		row := requireTYTLikelyRow(t, report, tc.slotMs, tc.txSwitchMs, 1)
		if row.Outcome != tc.wantOutcome {
			t.Fatalf("slot=%d switch=%d outcome=%q, want %q; reasons=%v",
				tc.slotMs, tc.txSwitchMs, row.Outcome, tc.wantOutcome, row.FailReasons)
		}
		if tc.wantReadinessMiss && !tytLikelyHasReason(row.FailReasons, TwoHopLiveFailRelayReadinessMiss) {
			t.Fatalf("slot=%d switch=%d fail_reasons=%v, want %q",
				tc.slotMs, tc.txSwitchMs, row.FailReasons, TwoHopLiveFailRelayReadinessMiss)
		}
	}
}

func TestIT026_TYTLikelyPlanningTuples(t *testing.T) {
	report, err := RunTYTLikelyProfileSimulation(DefaultTYTLikelyProfileConfig())
	if err != nil {
		t.Fatalf("RunTYTLikelyProfileSimulation failed: %v", err)
	}

	assertTuple := func(tuple TYTLikelyPlanningTuple, name string, slotMs, maxSwitchMs, latencyMs, spareMs int64) {
		t.Helper()
		if tuple.Name != name || tuple.SlotMs != slotMs || tuple.MaxTxRxSwitchMs != maxSwitchMs || tuple.GuardTimeMs != 1 {
			t.Fatalf("tuple %+v does not match %s/%d/%d/guard1", tuple, name, slotMs, maxSwitchMs)
		}
		if tuple.Outcome != TYTLikelyOutcomePass {
			t.Fatalf("tuple %s outcome=%q reasons=%v, want pass", name, tuple.Outcome, tuple.FailReasons)
		}
		if tuple.EndToEndLatencyMs != latencyMs || tuple.CycleSpareMs != spareMs {
			t.Fatalf("tuple %s latency/spare=%d/%d, want %d/%d",
				name, tuple.EndToEndLatencyMs, tuple.CycleSpareMs, latencyMs, spareMs)
		}
		if tuple.MeasuredHardwareCapability {
			t.Fatalf("tuple %s claims measured hardware capability", name)
		}
	}

	assertTuple(report.AggressiveTuple, tytLikelyTupleAggressive, 16, 2, 45, 35)
	assertTuple(report.PrimaryPlanningTuple, tytLikelyTuplePrimary, 24, 10, 61, 19)
	assertTuple(report.SafeFallbackTuple, tytLikelyTupleSafeFallback, 30, 16, 73, 7)
}

func TestIT026_TYTLikelyRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  TYTLikelyProfileConfig
		want string
	}{
		{name: "negative timing", cfg: func() TYTLikelyProfileConfig {
			cfg := DefaultTYTLikelyProfileConfig()
			cfg.PreambleMs = -1
			return cfg
		}(), want: "preamble_ms"},
		{name: "zero bitrate", cfg: func() TYTLikelyProfileConfig {
			cfg := DefaultTYTLikelyProfileConfig()
			cfg.ChannelBitrateBps = 0
			return cfg
		}(), want: "channel_bitrate_bps"},
		{name: "empty slots", cfg: func() TYTLikelyProfileConfig {
			cfg := DefaultTYTLikelyProfileConfig()
			cfg.SlotMsCandidates = []int64{}
			return cfg
		}(), want: "slot_ms_candidates"},
		{name: "empty switch candidates", cfg: func() TYTLikelyProfileConfig {
			cfg := DefaultTYTLikelyProfileConfig()
			cfg.TxRxSwitchMsCandidates = []int64{}
			return cfg
		}(), want: "tx_rx_switch_ms_candidates"},
		{name: "empty guard candidates", cfg: func() TYTLikelyProfileConfig {
			cfg := DefaultTYTLikelyProfileConfig()
			cfg.GuardTimeMsCandidates = []int64{}
			return cfg
		}(), want: "guard_time_ms_candidates"},
		{name: "row cap", cfg: func() TYTLikelyProfileConfig {
			cfg := DefaultTYTLikelyProfileConfig()
			cfg.SlotMsCandidates = tytLikelyTestInt64Range(1, 11)
			cfg.TxRxSwitchMsCandidates = tytLikelyTestInt64Range(20, 11)
			cfg.GuardTimeMsCandidates = tytLikelyTestInt64Range(40, 9)
			return cfg
		}(), want: "row count"},
		{name: "missing planning tuple", cfg: func() TYTLikelyProfileConfig {
			cfg := DefaultTYTLikelyProfileConfig()
			cfg.SlotMsCandidates = []int64{20, 24, 30}
			return cfg
		}(), want: tytLikelyFailPlanningTupleMissing},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RunTYTLikelyProfileSimulation(tc.cfg)
			if err == nil {
				t.Fatal("RunTYTLikelyProfileSimulation accepted invalid config")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want context %q", err.Error(), tc.want)
			}
		})
	}
}

func requireTYTLikelyRow(t *testing.T, report TYTLikelyProfileReport, slotMs, txSwitchMs, guardMs int64) TYTLikelyProfileRow {
	t.Helper()
	for _, row := range report.Rows {
		if row.SlotMs == slotMs && row.TxRxSwitchMs == txSwitchMs && row.GuardTimeMs == guardMs {
			return row
		}
	}
	t.Fatalf("missing TYT likely row slot=%d switch=%d guard=%d", slotMs, txSwitchMs, guardMs)
	return TYTLikelyProfileRow{}
}

func tytLikelyHasReason(reasons []string, want string) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}

func tytLikelyTestInt64Range(start int64, count int) []int64 {
	out := make([]int64, count)
	for i := range out {
		out[i] = start + int64(i)
	}
	return out
}
