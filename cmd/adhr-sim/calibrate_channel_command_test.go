package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

const testChannelBenchArtifactSHA = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestIT040_CalibrateChannelCommandWritesReport(t *testing.T) {
	inputPath := writeChannelBenchArtifact(t, fixtureChannelBenchArtifactJSON())
	reportPath := filepath.Join(t.TempDir(), "out", "channel-report.json")

	if err := run([]string{"calibrate-channel", inputPath, "--report", reportPath}); err != nil {
		t.Fatalf("calibrate-channel command failed: %v", err)
	}

	report := readChannelBenchArtifactReport(t, reportPath)
	if report.ScenarioID != scenario.ChannelBenchArtifactScenarioID {
		t.Fatalf("scenario_id = %q, want %q", report.ScenarioID, scenario.ChannelBenchArtifactScenarioID)
	}
	if report.Outcome != scenario.ChannelBenchOutcomeVerified || !report.ChannelBenchVerified {
		t.Fatalf("outcome/verified = %q/%v, want verified/true", report.Outcome, report.ChannelBenchVerified)
	}
	if report.EventHash == "" {
		t.Fatal("event_hash is empty")
	}
}

func TestIT040_CalibrateChannelCommandDeterministicReport(t *testing.T) {
	inputPath := writeChannelBenchArtifact(t, fixtureChannelBenchArtifactJSON())
	dir := t.TempDir()
	first := filepath.Join(dir, "first.json")
	second := filepath.Join(dir, "nested", "second.json")

	if err := run([]string{"calibrate-channel", inputPath, "--report", first}); err != nil {
		t.Fatalf("first calibrate-channel command failed: %v", err)
	}
	if err := run([]string{"calibrate-channel", inputPath, "--report", second}); err != nil {
		t.Fatalf("second calibrate-channel command failed: %v", err)
	}
	firstRaw, err := os.ReadFile(first)
	if err != nil {
		t.Fatalf("read first report failed: %v", err)
	}
	secondRaw, err := os.ReadFile(second)
	if err != nil {
		t.Fatalf("read second report failed: %v", err)
	}
	if string(firstRaw) != string(secondRaw) {
		t.Fatalf("channel calibration reports differ:\nfirst=%s\nsecond=%s", firstRaw, secondRaw)
	}
}

func TestCalibrateChannelCommandRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{name: "malformed", body: `{"scenario_id": "channel_bench_artifact"`, wantErr: "decode channel bench artifact"},
		{name: "unknown field", body: strings.Replace(fixtureChannelBenchArtifactJSON(), `"notes":`, `"unexpected": true, "notes":`, 1), wantErr: "unknown field"},
		{name: "trailing token", body: fixtureChannelBenchArtifactJSON() + "\n{}", wantErr: "trailing JSON token"},
		{name: "bad scenario", body: strings.Replace(fixtureChannelBenchArtifactJSON(), scenario.ChannelBenchArtifactScenarioID, "other", 1), wantErr: "unsupported scenario_id"},
		{name: "missing seed", body: `{"scenario_id":"channel_bench_artifact"}`, wantErr: "seed is required"},
		{name: "invalid measurement", body: strings.Replace(fixtureChannelBenchArtifactJSON(), `"test_fixture": false`, `"test_fixture": true`, 1), wantErr: "test_fixture=false"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inputPath := writeChannelBenchArtifact(t, tc.body)
			reportPath := filepath.Join(t.TempDir(), "report.json")
			err := run([]string{"calibrate-channel", inputPath, "--report", reportPath})
			if err == nil {
				t.Fatal("calibrate-channel accepted invalid input")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want %q", err.Error(), tc.wantErr)
			}
			assertFileMissing(t, reportPath)
		})
	}
}

func TestCalibrateChannelCommandRejectsInvalidArgs(t *testing.T) {
	err := run([]string{"calibrate-channel"})
	if err == nil || !strings.Contains(err.Error(), "channel artifact path") {
		t.Fatalf("missing input error = %v, want channel artifact path", err)
	}
	err = run([]string{"calibrate-channel", "input.json"})
	if err == nil || !strings.Contains(err.Error(), "--report") {
		t.Fatalf("missing report error = %v, want --report", err)
	}
	err = run([]string{"calibrate-channel", "input.json", "--report", "--help"})
	if err == nil || !strings.Contains(err.Error(), "flag-like") {
		t.Fatalf("flag-like report error = %v, want flag-like", err)
	}
}

func writeChannelBenchArtifact(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "channel.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write channel artifact failed: %v", err)
	}
	return path
}

func readChannelBenchArtifactReport(t *testing.T, path string) scenario.ChannelBenchArtifactReport {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read channel report failed: %v", err)
	}
	var report scenario.ChannelBenchArtifactReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("decode channel report failed: %v\n%s", err, raw)
	}
	return report
}

func fixtureChannelBenchArtifactJSON() string {
	return `{
  "scenario_id": "channel_bench_artifact",
  "seed": 42,
  "measurement_source": "bench_measured",
  "radio_family": "TYT MD-UV380 / MD-UV3x0 class",
  "test_fixture": false,
  "measurement_artifact_ref": "bench/channel/tyt-19200-25khz.json",
  "measurement_artifact_sha256": "` + testChannelBenchArtifactSHA + `",
  "channel_width_hz": 25000,
  "gross_bitrate_bps": 19200,
  "modulation_profile": "custom_2fsk_25khz_bench",
  "frames_sent": 1000,
  "frames_decoded": 1000,
  "crc_error_count": 0,
  "duration_ms": 10000,
  "notes": "bench-measured example"
}`
}
