package scenario

import (
	"reflect"
	"testing"
)

func TestIT032_DelayedRelayVoiceDefaultReportDeterministic(t *testing.T) {
	first, err := RunDelayedRelayVoiceSimulation(DefaultDelayedRelayVoiceConfig())
	if err != nil {
		t.Fatalf("RunDelayedRelayVoiceSimulation failed: %v", err)
	}
	second, err := RunDelayedRelayVoiceSimulation(DefaultDelayedRelayVoiceConfig())
	if err != nil {
		t.Fatalf("second RunDelayedRelayVoiceSimulation failed: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("default delayed relay voice report is not deterministic")
	}
	if first.ScenarioID != DelayedRelayVoiceScenarioID {
		t.Fatalf("ScenarioID = %q, want %q", first.ScenarioID, DelayedRelayVoiceScenarioID)
	}
	if first.LiveTwoHopViable {
		t.Fatal("LiveTwoHopViable = true, want false for 200 ms turnaround")
	}
	if !first.AssumptionOnly {
		t.Fatal("AssumptionOnly = false, want true")
	}
	if first.RowCount != 36 || len(first.Rows) != 36 {
		t.Fatalf("row count = %d/%d, want 36", first.RowCount, len(first.Rows))
	}
	if first.PassRows != 0 || first.DegradedRows != 20 || first.FailRows != 16 {
		t.Fatalf("pass/degraded/fail = %d/%d/%d, want 0/20/16", first.PassRows, first.DegradedRows, first.FailRows)
	}
	if first.EventHash == "" {
		t.Fatal("EventHash is empty")
	}

	best := first.BestTwoRelayRow
	if best.RowID != "bitrate_19200_frame_22B_burst_500ms_relays_2" {
		t.Fatalf("best two-relay row = %q, want bitrate_19200_frame_22B_burst_500ms_relays_2", best.RowID)
	}
	if best.Outcome != DelayedRelayVoiceOutcomeDegraded {
		t.Fatalf("best outcome = %q, want %q", best.Outcome, DelayedRelayVoiceOutcomeDegraded)
	}
	if best.FirstAudioLatencyMs != 997 || best.CompletionMs != 1075 || best.MaxUserPerceivedLatencyMs != 997 {
		t.Fatalf("best latency first/completion/max = %d/%d/%d, want 997/1075/997",
			best.FirstAudioLatencyMs, best.CompletionMs, best.MaxUserPerceivedLatencyMs)
	}
	if !best.BurstDrainFasterThanSpeech {
		t.Fatal("best row BurstDrainFasterThanSpeech = false, want true")
	}
	if !best.StoreForwardRequired {
		t.Fatal("best row StoreForwardRequired = false, want true")
	}
	if best.MeasuredHardwareCapability {
		t.Fatal("best row MeasuredHardwareCapability = true, want false")
	}
}

func TestIT032_DelayedRelayVoiceWorstDefaultRowFailsTwoSecondBudget(t *testing.T) {
	report, err := RunDelayedRelayVoiceSimulation(DefaultDelayedRelayVoiceConfig())
	if err != nil {
		t.Fatalf("RunDelayedRelayVoiceSimulation failed: %v", err)
	}
	row := mustFindDelayedRelayVoiceRow(t, report.Rows, "bitrate_9600_frame_56B_burst_2000ms_relays_3")
	if row.Outcome != DelayedRelayVoiceOutcomeFail {
		t.Fatalf("worst default outcome = %q, want %q", row.Outcome, DelayedRelayVoiceOutcomeFail)
	}
	if row.FirstAudioLatencyMs != 5120 || row.CompletionMs != 6320 || row.MaxUserPerceivedLatencyMs != 5120 {
		t.Fatalf("worst latency first/completion/max = %d/%d/%d, want 5120/6320/5120",
			row.FirstAudioLatencyMs, row.CompletionMs, row.MaxUserPerceivedLatencyMs)
	}
	if row.MaxUserPerceivedLatencyMs <= report.LatencyBudgetMs {
		t.Fatalf("worst max latency = %d, budget = %d; want over budget", row.MaxUserPerceivedLatencyMs, report.LatencyBudgetMs)
	}
	if !containsString(row.FailReasons, "latency_budget_exceeded") {
		t.Fatalf("worst fail reasons = %v, want latency budget reason", row.FailReasons)
	}
}

func TestDelayedRelayVoiceCanRepresentFastLiveTurnaround(t *testing.T) {
	cfg := DefaultDelayedRelayVoiceConfig()
	cfg.TurnaroundMs = 10
	cfg.ChannelBitrateBpsCandidates = []int64{19200}
	cfg.FrameBytesCandidates = []int{22}
	cfg.BurstDurationMsCandidates = []int64{500}
	cfg.RelayDepthCandidates = []int{2}
	report, err := RunDelayedRelayVoiceSimulation(cfg)
	if err != nil {
		t.Fatalf("RunDelayedRelayVoiceSimulation failed: %v", err)
	}
	if !report.LiveTwoHopViable {
		t.Fatal("LiveTwoHopViable = false, want true")
	}
	row := report.Rows[0]
	if row.StoreForwardRequired {
		t.Fatal("StoreForwardRequired = true, want false")
	}
	if row.Outcome != DelayedRelayVoiceOutcomePass {
		t.Fatalf("outcome = %q, want %q", row.Outcome, DelayedRelayVoiceOutcomePass)
	}
}

func TestDelayedRelayVoiceRejectsInvalidConfig(t *testing.T) {
	valid := DefaultDelayedRelayVoiceConfig()
	tests := []struct {
		name string
		cfg  DelayedRelayVoiceConfig
	}{
		{name: "unsupported scenario", cfg: func() DelayedRelayVoiceConfig {
			cfg := valid
			cfg.ScenarioID = "other"
			cfg.ChannelBitrateBpsCandidates = []int64{19200}
			return cfg
		}()},
		{name: "duplicate bitrate", cfg: func() DelayedRelayVoiceConfig {
			cfg := valid
			cfg.ChannelBitrateBpsCandidates = []int64{19200, 19200}
			return cfg
		}()},
		{name: "negative turnaround", cfg: func() DelayedRelayVoiceConfig {
			cfg := valid
			cfg.TurnaroundMs = -1
			return cfg
		}()},
		{name: "zero relay depth", cfg: func() DelayedRelayVoiceConfig {
			cfg := valid
			cfg.RelayDepthCandidates = []int{0}
			return cfg
		}()},
		{name: "row cap", cfg: func() DelayedRelayVoiceConfig {
			cfg := valid
			cfg.ChannelBitrateBpsCandidates = []int64{1, 2, 3, 4, 5}
			cfg.FrameBytesCandidates = []int{10, 11, 12, 13, 14}
			cfg.BurstDurationMsCandidates = []int64{100, 200, 300}
			cfg.RelayDepthCandidates = []int{1, 2}
			return cfg
		}()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := RunDelayedRelayVoiceSimulation(tc.cfg); err == nil {
				t.Fatal("RunDelayedRelayVoiceSimulation accepted invalid config")
			}
		})
	}
}

func mustFindDelayedRelayVoiceRow(t *testing.T, rows []DelayedRelayVoiceRow, id string) DelayedRelayVoiceRow {
	t.Helper()
	for _, row := range rows {
		if row.RowID == id {
			return row
		}
	}
	t.Fatalf("missing delayed relay voice row %q", id)
	return DelayedRelayVoiceRow{}
}
