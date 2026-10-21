package scenario

import (
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/protocol"
)

func TestIT018_TwoHopLiveTimingDefaultMatrix(t *testing.T) {
	report, err := RunTwoHopLiveTimingSweep(DefaultTwoHopLiveTimingConfig())
	if err != nil {
		t.Fatalf("RunTwoHopLiveTimingSweep failed: %v", err)
	}

	if report.ScenarioID != TwoHopLiveTimingScenarioID {
		t.Fatalf("ScenarioID = %q, want %q", report.ScenarioID, TwoHopLiveTimingScenarioID)
	}
	if report.RowCount != 96 {
		t.Fatalf("RowCount = %d, want 96", report.RowCount)
	}
	if len(report.Rows) != report.RowCount {
		t.Fatalf("len(Rows) = %d, want %d", len(report.Rows), report.RowCount)
	}
	if report.PassRows == 0 {
		t.Fatal("PassRows = 0, want at least one viable variable-slot two-hop tuple")
	}
	if report.FailRows == 0 {
		t.Fatal("FailRows = 0, want at least one failing tuple")
	}
	if report.PassRows+report.DegradedRows+report.FailRows != report.RowCount {
		t.Fatalf("row classification mismatch: pass=%d degraded=%d fail=%d row_count=%d",
			report.PassRows, report.DegradedRows, report.FailRows, report.RowCount)
	}
	if report.EventHash == "" {
		t.Fatal("EventHash is empty")
	}
	t.Logf("two-hop live matrix: rows=%d pass=%d degraded=%d fail=%d",
		report.RowCount, report.PassRows, report.DegradedRows, report.FailRows)
}

func TestIT018_TwoHopLiveTimingFindsViableVariableSlotPlan(t *testing.T) {
	relayBytes, err := protocol.CompressedRelayHopWireSize(8)
	if err != nil {
		t.Fatalf("CompressedRelayHopWireSize failed: %v", err)
	}

	report, err := RunTwoHopLiveTimingSweep(DefaultTwoHopLiveTimingConfig())
	if err != nil {
		t.Fatalf("RunTwoHopLiveTimingSweep failed: %v", err)
	}

	row := requireTwoHopLiveTimingRow(t, report, 19200, 56, relayBytes, 30, 20, 5)
	if row.SourceAirtimeMs != 24 {
		t.Fatalf("SourceAirtimeMs = %d, want 24", row.SourceAirtimeMs)
	}
	if row.RelayAirtimeMs != 10 {
		t.Fatalf("RelayAirtimeMs = %d, want 10", row.RelayAirtimeMs)
	}
	wantOffsets := []int64{0, 30, 50}
	if !reflect.DeepEqual(row.TxOffsetsMs, wantOffsets) {
		t.Fatalf("TxOffsetsMs = %v, want %v", row.TxOffsetsMs, wantOffsets)
	}
	if !row.SourceSlotFit {
		t.Fatal("SourceSlotFit = false, want true")
	}
	if !row.RelaySlotFit {
		t.Fatal("RelaySlotFit = false, want true")
	}
	if !row.RelayReady {
		t.Fatalf("RelayReady = false, misses=%d", row.RelayReadinessMissCount)
	}
	if !row.CycleFitsPeriod {
		t.Fatalf("CycleFitsPeriod = false, occupied=%d voice_period=%d", row.CycleOccupiedMs, row.VoicePeriodMs)
	}
	if row.CycleOccupiedMs != 65 {
		t.Fatalf("CycleOccupiedMs = %d, want 65", row.CycleOccupiedMs)
	}
	if row.CycleSpareMs != 15 {
		t.Fatalf("CycleSpareMs = %d, want 15", row.CycleSpareMs)
	}
	if row.EndToEndLatencyMs != 61 {
		t.Fatalf("EndToEndLatencyMs = %d, want 61", row.EndToEndLatencyMs)
	}
	if !row.CadencePass {
		t.Fatal("CadencePass = false, want true for 80 ms voice period")
	}
	if !row.LatencyBudgetPass {
		t.Fatal("LatencyBudgetPass = false, want true")
	}
	if row.Outcome != TwoHopLiveOutcomePass {
		t.Fatalf("Outcome = %q, want %q; reasons=%v", row.Outcome, TwoHopLiveOutcomePass, row.FailReasons)
	}

	t.Logf("viable two-hop live plan: bitrate=%d source=%dB/%dms relay=%dB/%dms guard=%dms latency=%dms spare=%dms",
		row.ChannelBitrateBps, row.SourceFrameBytes, row.SourceSlotMs, row.RelayFrameBytes, row.RelaySlotMs,
		row.GuardTimeMs, row.EndToEndLatencyMs, row.CycleSpareMs)
}

func TestIT018_TwoHopLiveTimingGuardCanBreakVariableSlotPlan(t *testing.T) {
	report, err := RunTwoHopLiveTimingSweep(DefaultTwoHopLiveTimingConfig())
	if err != nil {
		t.Fatalf("RunTwoHopLiveTimingSweep failed: %v", err)
	}

	row := requireTwoHopLiveTimingRow(t, report, 19200, 56, defaultTwoHopLiveRelayFrameBytes, 30, 20, 10)
	if row.SourceSlotFit {
		t.Fatal("SourceSlotFit = true, want false because 24 ms airtime + 10 ms guard exceeds 30 ms source slot")
	}
	if !row.RelaySlotFit {
		t.Fatal("RelaySlotFit = false, want true because 10 ms airtime + 10 ms guard fits 20 ms relay slot")
	}
	if row.RelayReady {
		t.Fatal("RelayReady = true, want false because source receive readiness misses relay1 tx")
	}
	wantReasons := []string{
		TwoHopLiveFailSourceSlotMiss,
		TwoHopLiveFailRelayReadinessMiss,
	}
	if !reflect.DeepEqual(row.FailReasons, wantReasons) {
		t.Fatalf("FailReasons = %v, want %v", row.FailReasons, wantReasons)
	}
	if row.Outcome != TwoHopLiveOutcomeFail {
		t.Fatalf("Outcome = %q, want %q", row.Outcome, TwoHopLiveOutcomeFail)
	}
}

func TestIT018_TwoHopLiveTimingRejectsCycleOverflow(t *testing.T) {
	cfg := DefaultTwoHopLiveTimingConfig()
	cfg.SourceSlotDurationsMs = []int64{40}
	cfg.RelaySlotDurationsMs = []int64{21}
	cfg.GuardTimesMs = []int64{10}

	report, err := RunTwoHopLiveTimingSweep(cfg)
	if err != nil {
		t.Fatalf("RunTwoHopLiveTimingSweep failed: %v", err)
	}

	row := requireTwoHopLiveTimingRow(t, report, 19200, 56, defaultTwoHopLiveRelayFrameBytes, 40, 21, 10)
	if row.CycleFitsPeriod {
		t.Fatal("CycleFitsPeriod = true, want false because occupied cycle exceeds 80 ms")
	}
	if row.CycleSpareMs != -1 {
		t.Fatalf("CycleSpareMs = %d, want -1", row.CycleSpareMs)
	}
	if !containsTwoHopLiveReason(row.FailReasons, TwoHopLiveFailCyclePeriodExceeded) {
		t.Fatalf("FailReasons = %v, want %q", row.FailReasons, TwoHopLiveFailCyclePeriodExceeded)
	}
	if row.Outcome != TwoHopLiveOutcomeFail {
		t.Fatalf("Outcome = %q, want %q", row.Outcome, TwoHopLiveOutcomeFail)
	}
}

func TestIT018_TwoHopLiveTimingDeterministicReplay(t *testing.T) {
	first, err := RunTwoHopLiveTimingSweep(DefaultTwoHopLiveTimingConfig())
	if err != nil {
		t.Fatalf("first RunTwoHopLiveTimingSweep failed: %v", err)
	}
	second, err := RunTwoHopLiveTimingSweep(DefaultTwoHopLiveTimingConfig())
	if err != nil {
		t.Fatalf("second RunTwoHopLiveTimingSweep failed: %v", err)
	}

	if first.EventHash != second.EventHash {
		t.Fatalf("EventHash mismatch:\nfirst=%s\nsecond=%s", first.EventHash, second.EventHash)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same config produced different reports")
	}
}

func TestIT018_TwoHopLiveTimingRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  TwoHopLiveTimingConfig
		want string
	}{
		{name: "empty source slots", cfg: func() TwoHopLiveTimingConfig {
			cfg := DefaultTwoHopLiveTimingConfig()
			cfg.SourceSlotDurationsMs = []int64{}
			return cfg
		}(), want: "source_slot_durations_ms"},
		{name: "negative relay slot", cfg: func() TwoHopLiveTimingConfig {
			cfg := DefaultTwoHopLiveTimingConfig()
			cfg.RelaySlotDurationsMs = []int64{-1}
			return cfg
		}(), want: "relay_slot_durations_ms"},
		{name: "duplicate guard", cfg: func() TwoHopLiveTimingConfig {
			cfg := DefaultTwoHopLiveTimingConfig()
			cfg.GuardTimesMs = []int64{2, 2}
			return cfg
		}(), want: "duplicate"},
		{name: "zero voice period", cfg: func() TwoHopLiveTimingConfig {
			cfg := DefaultTwoHopLiveTimingConfig()
			cfg.VoicePeriodMs = -1
			return cfg
		}(), want: "voice_period_ms"},
		{name: "row cap", cfg: func() TwoHopLiveTimingConfig {
			cfg := DefaultTwoHopLiveTimingConfig()
			cfg.ChannelBitratesBps = make([]int64, 101)
			for i := range cfg.ChannelBitratesBps {
				cfg.ChannelBitratesBps[i] = int64(i + 1)
			}
			cfg.SourceFrameBytes = make([]int, 101)
			for i := range cfg.SourceFrameBytes {
				cfg.SourceFrameBytes[i] = i + 1
			}
			return cfg
		}(), want: "row count"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RunTwoHopLiveTimingSweep(tc.cfg)
			if err == nil {
				t.Fatal("RunTwoHopLiveTimingSweep accepted invalid config")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want context %q", err.Error(), tc.want)
			}
		})
	}
}

func TestIT018_TwoHopLiveTimingRejectsHugeFrameBytesWithoutAllocation(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("huge byte-count overflow guard is only reachable on 64-bit int platforms")
	}
	cfg := DefaultTwoHopLiveTimingConfig()
	cfg.SourceFrameBytes = []int{int(int64(math.MaxInt64/8) + 1)}
	cfg.RelayFrameBytes = []int{22}
	cfg.SourceSlotDurationsMs = []int64{40}
	cfg.RelaySlotDurationsMs = []int64{40}
	cfg.GuardTimesMs = []int64{1}

	_, err := RunTwoHopLiveTimingSweep(cfg)
	if err == nil {
		t.Fatal("RunTwoHopLiveTimingSweep accepted huge source_frame_bytes")
	}
	if !strings.Contains(err.Error(), "frame_bytes") {
		t.Fatalf("error = %q, want frame_bytes context", err.Error())
	}
}

func TestIT020_TwoHopLiveShortSourceTimingMatrix(t *testing.T) {
	report, err := RunTwoHopLiveTimingSweep(DefaultTwoHopLiveShortSourceTimingConfig())
	if err != nil {
		t.Fatalf("RunTwoHopLiveTimingSweep failed: %v", err)
	}

	if report.ScenarioID != TwoHopLiveShortSourceTimingScenarioID {
		t.Fatalf("ScenarioID = %q, want %q", report.ScenarioID, TwoHopLiveShortSourceTimingScenarioID)
	}
	if report.RowCount != 168 {
		t.Fatalf("RowCount = %d, want 168", report.RowCount)
	}
	if report.PassRows == 0 {
		t.Fatal("PassRows = 0, want viable short-source rows")
	}
	if report.FailRows == 0 {
		t.Fatal("FailRows = 0, want failing rows to keep the sweep meaningful")
	}
	if report.PassRows+report.DegradedRows+report.FailRows != report.RowCount {
		t.Fatalf("row classification mismatch: pass=%d degraded=%d fail=%d row_count=%d",
			report.PassRows, report.DegradedRows, report.FailRows, report.RowCount)
	}
	if report.EventHash == "" {
		t.Fatal("EventHash is empty")
	}

	t.Logf("short-source matrix: rows=%d pass=%d degraded=%d fail=%d hash=%s",
		report.RowCount, report.PassRows, report.DegradedRows, report.FailRows, report.EventHash)
}

func TestIT020_TwoHopLiveShortSourceReducesSlotAndLatency(t *testing.T) {
	report, err := RunTwoHopLiveTimingSweep(DefaultTwoHopLiveShortSourceTimingConfig())
	if err != nil {
		t.Fatalf("RunTwoHopLiveTimingSweep failed: %v", err)
	}

	shortRow := requireTwoHopLiveTimingRow(t, report, 19200, 22, defaultTwoHopLiveRelayFrameBytes, 12, 12, 1)
	if shortRow.SourceAirtimeMs != 10 {
		t.Fatalf("short source airtime = %d, want 10", shortRow.SourceAirtimeMs)
	}
	if shortRow.RelayAirtimeMs != 10 {
		t.Fatalf("short relay airtime = %d, want 10", shortRow.RelayAirtimeMs)
	}
	if shortRow.CycleOccupiedMs != 35 {
		t.Fatalf("short cycle occupied = %d, want 35", shortRow.CycleOccupiedMs)
	}
	if shortRow.CycleSpareMs != 45 {
		t.Fatalf("short cycle spare = %d, want 45", shortRow.CycleSpareMs)
	}
	if shortRow.EndToEndLatencyMs != 35 {
		t.Fatalf("short latency = %d, want 35", shortRow.EndToEndLatencyMs)
	}
	if shortRow.Outcome != TwoHopLiveOutcomePass {
		t.Fatalf("short outcome = %q, want %q; reasons=%v", shortRow.Outcome, TwoHopLiveOutcomePass, shortRow.FailReasons)
	}

	canonicalRow := requireTwoHopLiveTimingRow(t, report, 19200, 56, defaultTwoHopLiveRelayFrameBytes, 30, 20, 5)
	if canonicalRow.Outcome != TwoHopLiveOutcomePass {
		t.Fatalf("canonical comparison outcome = %q, want %q; reasons=%v", canonicalRow.Outcome, TwoHopLiveOutcomePass, canonicalRow.FailReasons)
	}
	if shortRow.SourceSlotMs >= canonicalRow.SourceSlotMs {
		t.Fatalf("short source slot = %d, want less than canonical %d", shortRow.SourceSlotMs, canonicalRow.SourceSlotMs)
	}
	if shortRow.RelaySlotMs >= canonicalRow.RelaySlotMs {
		t.Fatalf("short relay slot = %d, want less than canonical %d", shortRow.RelaySlotMs, canonicalRow.RelaySlotMs)
	}
	if shortRow.CycleSpareMs <= canonicalRow.CycleSpareMs {
		t.Fatalf("short spare = %d, want more than canonical %d", shortRow.CycleSpareMs, canonicalRow.CycleSpareMs)
	}
	if shortRow.EndToEndLatencyMs >= canonicalRow.EndToEndLatencyMs {
		t.Fatalf("short latency = %d, want less than canonical %d", shortRow.EndToEndLatencyMs, canonicalRow.EndToEndLatencyMs)
	}

	t.Logf("short source result: source=%dB/%dms relay=%dB/%dms guard=%dms latency=%dms spare=%dms",
		shortRow.SourceFrameBytes, shortRow.SourceSlotMs, shortRow.RelayFrameBytes, shortRow.RelaySlotMs,
		shortRow.GuardTimeMs, shortRow.EndToEndLatencyMs, shortRow.CycleSpareMs)
}

func TestIT020_TwoHopLiveShortSourceDeterministicReplay(t *testing.T) {
	first, err := RunTwoHopLiveTimingSweep(DefaultTwoHopLiveShortSourceTimingConfig())
	if err != nil {
		t.Fatalf("first RunTwoHopLiveTimingSweep failed: %v", err)
	}
	second, err := RunTwoHopLiveTimingSweep(DefaultTwoHopLiveShortSourceTimingConfig())
	if err != nil {
		t.Fatalf("second RunTwoHopLiveTimingSweep failed: %v", err)
	}

	if first.EventHash != second.EventHash {
		t.Fatalf("EventHash mismatch:\nfirst=%s\nsecond=%s", first.EventHash, second.EventHash)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same short-source config produced different reports")
	}
}

func requireTwoHopLiveTimingRow(
	t *testing.T,
	report TwoHopLiveTimingReport,
	bitrate int64,
	sourceFrameBytes int,
	relayFrameBytes int,
	sourceSlotMs int64,
	relaySlotMs int64,
	guardTimeMs int64,
) TwoHopLiveTimingRow {
	t.Helper()
	for _, row := range report.Rows {
		if row.ChannelBitrateBps == bitrate &&
			row.SourceFrameBytes == sourceFrameBytes &&
			row.RelayFrameBytes == relayFrameBytes &&
			row.SourceSlotMs == sourceSlotMs &&
			row.RelaySlotMs == relaySlotMs &&
			row.GuardTimeMs == guardTimeMs {
			return row
		}
	}
	t.Fatalf("row not found: bitrate=%d source=%d relay=%d source_slot=%d relay_slot=%d guard=%d",
		bitrate, sourceFrameBytes, relayFrameBytes, sourceSlotMs, relaySlotMs, guardTimeMs)
	return TwoHopLiveTimingRow{}
}

func containsTwoHopLiveReason(reasons []string, want string) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}
