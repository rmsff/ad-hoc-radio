package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

func TestIT022_RunCommandWritesTwoHopShortSourceMetrics(t *testing.T) {
	metricsPath := filepath.Join(t.TempDir(), "out", "two-hop.json")

	if err := run([]string{"run", twoHopShortSourceConfigPath(t), "--metrics", metricsPath}); err != nil {
		t.Fatalf("run command failed: %v", err)
	}

	report := readMetricsReport(t, metricsPath)
	if report.ScenarioID != scenario.TwoHopLiveShortSourceScenarioID {
		t.Errorf("scenario_id = %q, want %q", report.ScenarioID, scenario.TwoHopLiveShortSourceScenarioID)
	}
	if report.Outcome != "pass" {
		t.Errorf("outcome = %q, want pass", report.Outcome)
	}
	if report.SourceFramesCreated != 8 {
		t.Errorf("source_frames_created = %d, want 8", report.SourceFramesCreated)
	}
	if report.SourceFramesSent != 8 {
		t.Errorf("source_frames_sent = %d, want 8", report.SourceFramesSent)
	}
	if report.Relay.FramesReceived != 16 {
		t.Errorf("relay.frames_received = %d, want 16", report.Relay.FramesReceived)
	}
	if report.Relay.FramesForwarded != 16 {
		t.Errorf("relay.frames_forwarded = %d, want 16", report.Relay.FramesForwarded)
	}
	if report.Receiver.FramesAccepted != 8 {
		t.Errorf("receiver.frames_accepted = %d, want 8", report.Receiver.FramesAccepted)
	}
	if report.Receiver.PlayoutFramesEmitted != 8 {
		t.Errorf("receiver.playout_frames_emitted = %d, want 8", report.Receiver.PlayoutFramesEmitted)
	}
	if report.Receiver.RelayedFramesAccepted != 8 {
		t.Errorf("receiver.relayed_frames_accepted = %d, want 8", report.Receiver.RelayedFramesAccepted)
	}
	if report.LatencyMs.P95 != 35 {
		t.Errorf("latency_ms.p95 = %v, want 35", report.LatencyMs.P95)
	}
	if report.OutOfSlotTxCount != 0 {
		t.Errorf("out_of_slot_tx_count = %d, want 0", report.OutOfSlotTxCount)
	}
	if report.TwoHopLive == nil {
		t.Fatal("two_hop_live section missing")
	}
	if report.ScheduledRelayTree != nil {
		t.Fatal("legacy two_hop_live report must omit scheduled_relay_tree")
	}
	if report.TwoHopLive.SourceWireMode != scenario.TwoHopLiveSourceWireShort {
		t.Errorf("two_hop_live.source_wire_mode = %q, want short", report.TwoHopLive.SourceWireMode)
	}
	if report.TwoHopLive.SourceFrameBytes != 22 || report.TwoHopLive.RelayFrameBytes != 22 {
		t.Errorf("two_hop_live wire bytes = source %d relay %d, want 22/22",
			report.TwoHopLive.SourceFrameBytes, report.TwoHopLive.RelayFrameBytes)
	}
	if report.TwoHopLive.CycleSpareMs != 45 {
		t.Errorf("two_hop_live.cycle_spare_ms = %d, want 45", report.TwoHopLive.CycleSpareMs)
	}
	if report.Determinism.EventCount != 24 {
		t.Errorf("determinism.event_count = %d, want 24", report.Determinism.EventCount)
	}
	if report.Determinism.EventHash != "57c8429aaa0f44a079a42440b7de14b296b51048f4da3a5010150fbc92115d73" {
		t.Errorf("determinism.event_hash = %q", report.Determinism.EventHash)
	}
}

func TestIT022_RunCommandTwoHopShortSourceDeterministicHash(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.json")
	second := filepath.Join(dir, "nested", "second.json")

	if err := run([]string{"run", twoHopShortSourceConfigPath(t), "--metrics", first}); err != nil {
		t.Fatalf("first run command failed: %v", err)
	}
	if err := run([]string{"run", twoHopShortSourceConfigPath(t), "--metrics", second}); err != nil {
		t.Fatalf("second run command failed: %v", err)
	}

	report1 := readMetricsReport(t, first)
	report2 := readMetricsReport(t, second)
	if report1.Determinism.EventHash != report2.Determinism.EventHash {
		t.Errorf("determinism.event_hash mismatch:\n  first=%s\n  second=%s",
			report1.Determinism.EventHash, report2.Determinism.EventHash)
	}
	if report1.ScenarioID != report2.ScenarioID {
		t.Errorf("scenario_id mismatch: %q vs %q", report1.ScenarioID, report2.ScenarioID)
	}
}

func TestRunCommandRejectsOneHopConfigWithTwoHopField(t *testing.T) {
	configPath := writeScenarioConfig(t, `{
  "scenario_id": "one_hop_nominal",
  "seed": 42,
  "source_wire_mode": "short"
}`)
	metricsPath := filepath.Join(t.TempDir(), "metrics.json")

	err := run([]string{"run", configPath, "--metrics", metricsPath})
	if err == nil {
		t.Fatal("run command accepted one-hop config with two-hop field")
	}
	if !strings.Contains(err.Error(), "does not support field") {
		t.Fatalf("error = %q, want unsupported field context", err.Error())
	}
	assertFileMissing(t, metricsPath)
}

func TestRunCommandRejectsTwoHopConfigWithOneHopField(t *testing.T) {
	configPath := writeScenarioConfig(t, `{
  "scenario_id": "two_hop_live_short_source_full_scenario",
  "frame_period_ms": 80
}`)
	metricsPath := filepath.Join(t.TempDir(), "metrics.json")

	err := run([]string{"run", configPath, "--metrics", metricsPath})
	if err == nil {
		t.Fatal("run command accepted two-hop config with one-hop field")
	}
	if !strings.Contains(err.Error(), "does not support field") {
		t.Fatalf("error = %q, want unsupported field context", err.Error())
	}
	assertFileMissing(t, metricsPath)
}

func TestRunCommandRejectsInvalidTwoHopShortSourceConfig(t *testing.T) {
	configPath := writeScenarioConfig(t, `{
  "scenario_id": "two_hop_live_short_source_full_scenario",
  "source_wire_mode": "canonical",
  "source_frame_bytes": 22,
  "relay_frame_bytes": 22
}`)
	metricsPath := filepath.Join(t.TempDir(), "metrics.json")

	err := run([]string{"run", configPath, "--metrics", metricsPath})
	if err == nil {
		t.Fatal("run command accepted invalid two-hop short-source config")
	}
	if !strings.Contains(err.Error(), "source_frame_bytes") {
		t.Fatalf("error = %q, want source_frame_bytes validation context", err.Error())
	}
	assertFileMissing(t, metricsPath)
}

func TestRunCommandAcceptsExplicitZeroTwoHopStartMs(t *testing.T) {
	configPath := writeScenarioConfig(t, `{
  "scenario_id": "two_hop_live_short_source_full_scenario",
  "start_ms": 0
}`)
	dir := t.TempDir()
	metricsPath := filepath.Join(dir, "metrics.json")
	defaultMetricsPath := filepath.Join(dir, "default.json")

	if err := run([]string{"run", configPath, "--metrics", metricsPath}); err != nil {
		t.Fatalf("run command rejected explicit start_ms=0: %v", err)
	}
	if err := run([]string{"run", twoHopShortSourceConfigPath(t), "--metrics", defaultMetricsPath}); err != nil {
		t.Fatalf("default run command failed: %v", err)
	}
	report := readMetricsReport(t, metricsPath)
	defaultReport := readMetricsReport(t, defaultMetricsPath)
	if report.SimDurationMs == 0 {
		t.Fatal("sim_duration_ms = 0, want non-zero scenario duration")
	}
	if report.SimDurationMs != defaultReport.SimDurationMs {
		t.Fatalf("sim_duration_ms = %d, want same duration as default %d", report.SimDurationMs, defaultReport.SimDurationMs)
	}
	if report.Determinism.EventHash == defaultReport.Determinism.EventHash {
		t.Fatal("explicit start_ms=0 produced same event_hash as default start_ms; start override was not applied")
	}
}

func TestRunCommandRejectsExplicitZeroTwoHopFields(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "frame count",
			body: `{
  "scenario_id": "two_hop_live_short_source_full_scenario",
  "frame_count": 0
}`,
			want: "frame_count",
		},
		{
			name: "receiver jitter buffer",
			body: `{
  "scenario_id": "two_hop_live_short_source_full_scenario",
  "receiver_jitter_buffer_ms": 0
}`,
			want: "receiver_jitter_buffer_ms",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			configPath := writeScenarioConfig(t, tc.body)
			metricsPath := filepath.Join(t.TempDir(), "metrics.json")

			err := run([]string{"run", configPath, "--metrics", metricsPath})
			if err == nil {
				t.Fatal("run command accepted explicit zero two-hop field")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want %q context", err.Error(), tc.want)
			}
			assertFileMissing(t, metricsPath)
		})
	}
}

func twoHopShortSourceConfigPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "configs", "scenarios", "two-hop-short-source.json")
}
