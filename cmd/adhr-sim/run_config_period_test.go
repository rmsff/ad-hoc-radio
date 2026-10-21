package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// A frame_period_ms above protocol.MaxFramePeriodMs fails config validation
// before the run, not later when the source encodes call_start.
func TestRunCommandRejectsFramePeriodAboveMax(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name: "relay_forward_queue_backpressure",
			body: `{
  "scenario_id": "relay_forward_queue_backpressure",
  "frame_period_ms": 60001,
  "slot_mode_enabled": true,
  "slot_duration_ms": 40,
  "allow_slot_timing_stress": true,
  "relay_forward_queue_max_depth": 1
}`,
			wantErr: "requires frame_period_ms in 1..60000",
		},
		{
			name:    "one_hop_nominal",
			body:    `{"scenario_id": "one_hop_nominal", "frame_period_ms": 60001}`,
			wantErr: "FramePeriodMs=60001 must be in 1..60000",
		},
		{
			name:    "two_hop_live_short_source_full_scenario",
			body:    `{"scenario_id": "two_hop_live_short_source_full_scenario", "voice_period_ms": 60001}`,
			wantErr: "two-hop live scenario: voice_period_ms=60001 must be in 1..60000",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			configPath := writeScenarioConfig(t, tc.body)
			metricsPath := filepath.Join(t.TempDir(), "metrics.json")
			err := run([]string{"run", configPath, "--metrics", metricsPath})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
			assertFileMissing(t, metricsPath)
		})
	}
}

func TestBackpressureConfigAcceptsMaxFramePeriod(t *testing.T) {
	d := scenarioConfigDTO{ScenarioID: "relay_forward_queue_backpressure"}
	period, slot, depth := int64(60000), int64(30001), 1
	enabled := true
	d.FramePeriodMs, d.SlotDurationMs, d.RelayForwardQueueMaxDepth = &period, &slot, &depth
	d.SlotModeEnabled, d.AllowSlotTimingStress = &enabled, &enabled
	cfg, err := d.toRelayForwardQueueBackpressureScenarioConfig()
	if err != nil || cfg.FramePeriodMs != period {
		t.Fatalf("config = %+v, err = %v; want frame_period_ms %d accepted", cfg, err, period)
	}
}
