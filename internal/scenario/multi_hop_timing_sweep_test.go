package scenario

import (
	"reflect"
	"strings"
	"testing"
)

func TestIT016_MultiHopTimingSweepDefaultMatrix(t *testing.T) {
	report, err := RunMultiHopTimingSweep(DefaultMultiHopTimingSweepConfig())
	if err != nil {
		t.Fatalf("RunMultiHopTimingSweep failed: %v", err)
	}

	if report.ScenarioID != MultiHopTimingSweepScenarioID {
		t.Fatalf("ScenarioID = %q, want %q", report.ScenarioID, MultiHopTimingSweepScenarioID)
	}
	if report.RowCount != 192 {
		t.Fatalf("RowCount = %d, want 192", report.RowCount)
	}
	if len(report.Rows) != report.RowCount {
		t.Fatalf("len(Rows) = %d, want %d", len(report.Rows), report.RowCount)
	}
	if report.PassRows == 0 {
		t.Fatal("PassRows = 0, want at least one viable research tuple")
	}
	if report.DegradedRows == 0 {
		t.Fatal("DegradedRows = 0, want at least one degraded tuple")
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
}

func TestIT016_MultiHopTimingSweepKnownDeadTuple(t *testing.T) {
	report, err := RunMultiHopTimingSweep(DefaultMultiHopTimingSweepConfig())
	if err != nil {
		t.Fatalf("RunMultiHopTimingSweep failed: %v", err)
	}

	row := requireMultiHopTimingRow(t, report, 9600, 56, 40, 0, MultiHopScheduleNextSlot)
	if row.AirtimeMs != 47 {
		t.Fatalf("AirtimeMs = %d, want 47", row.AirtimeMs)
	}
	if row.SlotFit {
		t.Fatal("SlotFit = true, want false for 56B @ 9600 bps in 40 ms")
	}
	if row.RelayReady {
		t.Fatal("RelayReady = true, want false because receive completes after next slot starts")
	}
	wantReasons := []string{
		MultiHopTimingFailSlotMiss,
		MultiHopTimingFailRelayReadinessMiss,
		MultiHopTimingFailFramePeriodExceeded,
	}
	if !reflect.DeepEqual(row.FailReasons, wantReasons) {
		t.Fatalf("FailReasons = %v, want %v", row.FailReasons, wantReasons)
	}
	if row.Outcome != MultiHopTimingOutcomeFail {
		t.Fatalf("Outcome = %q, want %q", row.Outcome, MultiHopTimingOutcomeFail)
	}
}

func TestIT016_MultiHopTimingSweepStrongPipelineCandidate(t *testing.T) {
	report, err := RunMultiHopTimingSweep(DefaultMultiHopTimingSweepConfig())
	if err != nil {
		t.Fatalf("RunMultiHopTimingSweep failed: %v", err)
	}

	row := requireMultiHopTimingRow(t, report, 19200, 22, 20, 10, MultiHopSchedulePipelineLag)
	if row.AirtimeMs != 10 {
		t.Fatalf("AirtimeMs = %d, want 10", row.AirtimeMs)
	}
	if !row.SlotFit {
		t.Fatal("SlotFit = false, want true")
	}
	if !row.RelayReady {
		t.Fatalf("RelayReady = false, misses=%d", row.RelayReadinessMissCount)
	}
	if row.EndToEndLatencyMs != 171 {
		t.Fatalf("EndToEndLatencyMs = %d, want 171", row.EndToEndLatencyMs)
	}
	if row.SourceFramePeriodMs != 60 {
		t.Fatalf("SourceFramePeriodMs = %d, want 60", row.SourceFramePeriodMs)
	}
	if !row.LatencyBudgetPass {
		t.Fatal("LatencyBudgetPass = false, want true")
	}
	if !row.CadencePass {
		t.Fatal("CadencePass = false, want true")
	}
	if row.Outcome != MultiHopTimingOutcomePass {
		t.Fatalf("Outcome = %q, want %q; reasons=%v", row.Outcome, MultiHopTimingOutcomePass, row.FailReasons)
	}
}

func TestIT016_MultiHopTimingSweepGuardCanBreakImmediateRelay(t *testing.T) {
	report, err := RunMultiHopTimingSweep(DefaultMultiHopTimingSweepConfig())
	if err != nil {
		t.Fatalf("RunMultiHopTimingSweep failed: %v", err)
	}

	row := requireMultiHopTimingRow(t, report, 19200, 22, 20, 10, MultiHopScheduleNextSlot)
	if !row.SlotFit {
		t.Fatal("SlotFit = false, want true because airtime + guard exactly equals slot")
	}
	if row.RelayReady {
		t.Fatal("RelayReady = true, want false because guard pushes receive readiness past next slot")
	}
	if row.RelayReadinessMissCount != 2 {
		t.Fatalf("RelayReadinessMissCount = %d, want 2", row.RelayReadinessMissCount)
	}
	if row.Outcome != MultiHopTimingOutcomeFail {
		t.Fatalf("Outcome = %q, want %q", row.Outcome, MultiHopTimingOutcomeFail)
	}
}

func TestIT016_MultiHopTimingSweepDeterministicReplay(t *testing.T) {
	first, err := RunMultiHopTimingSweep(DefaultMultiHopTimingSweepConfig())
	if err != nil {
		t.Fatalf("first RunMultiHopTimingSweep failed: %v", err)
	}
	second, err := RunMultiHopTimingSweep(DefaultMultiHopTimingSweepConfig())
	if err != nil {
		t.Fatalf("second RunMultiHopTimingSweep failed: %v", err)
	}

	if first.EventHash != second.EventHash {
		t.Fatalf("EventHash mismatch:\nfirst=%s\nsecond=%s", first.EventHash, second.EventHash)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same config produced different reports")
	}
}

func TestIT016_MultiHopTimingSweepRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  MultiHopTimingSweepConfig
		want string
	}{
		{name: "negative guard", cfg: func() MultiHopTimingSweepConfig {
			cfg := DefaultMultiHopTimingSweepConfig()
			cfg.GuardTimesMs = []int64{-1}
			return cfg
		}(), want: "guard_times_ms"},
		{name: "unknown schedule", cfg: func() MultiHopTimingSweepConfig {
			cfg := DefaultMultiHopTimingSweepConfig()
			cfg.ScheduleModes = []string{"parallel_magic"}
			return cfg
		}(), want: "unsupported schedule_mode"},
		{name: "duplicate bitrate", cfg: func() MultiHopTimingSweepConfig {
			cfg := DefaultMultiHopTimingSweepConfig()
			cfg.ChannelBitratesBps = []int64{9600, 9600}
			return cfg
		}(), want: "duplicate"},
		{name: "row cap", cfg: func() MultiHopTimingSweepConfig {
			cfg := DefaultMultiHopTimingSweepConfig()
			cfg.ChannelBitratesBps = make([]int64, 101)
			for i := range cfg.ChannelBitratesBps {
				cfg.ChannelBitratesBps[i] = int64(i + 1)
			}
			cfg.FrameBytes = make([]int, 101)
			for i := range cfg.FrameBytes {
				cfg.FrameBytes[i] = i + 1
			}
			return cfg
		}(), want: "row count"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RunMultiHopTimingSweep(tc.cfg)
			if err == nil {
				t.Fatal("RunMultiHopTimingSweep accepted invalid config")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want context %q", err.Error(), tc.want)
			}
		})
	}
}

func requireMultiHopTimingRow(
	t *testing.T,
	report MultiHopTimingSweepReport,
	bitrate int64,
	frameBytes int,
	slotDurationMs int64,
	guardTimeMs int64,
	scheduleMode string,
) MultiHopTimingSweepRow {
	t.Helper()
	for _, row := range report.Rows {
		if row.ChannelBitrateBps == bitrate &&
			row.FrameBytes == frameBytes &&
			row.SlotDurationMs == slotDurationMs &&
			row.GuardTimeMs == guardTimeMs &&
			row.ScheduleMode == scheduleMode {
			return row
		}
	}
	t.Fatalf("row not found: bitrate=%d frame=%d slot=%d guard=%d mode=%s",
		bitrate, frameBytes, slotDurationMs, guardTimeMs, scheduleMode)
	return MultiHopTimingSweepRow{}
}
