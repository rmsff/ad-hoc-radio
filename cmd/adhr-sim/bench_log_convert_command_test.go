package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

const testBenchLogSHA = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"

func TestIT050_ConvertBenchLogWritesChannelInput(t *testing.T) {
	inputPath := writeBenchLog(t, filledChannelBenchLogJSON())
	outPath := filepath.Join(t.TempDir(), "out", "channel.json")

	if err := run([]string{"convert-bench-log", inputPath, "--out", outPath}); err != nil {
		t.Fatalf("convert-bench-log channel failed: %v", err)
	}

	cfg := readConvertedChannelConfig(t, outPath)
	if cfg.ScenarioID != scenario.ChannelBenchArtifactScenarioID {
		t.Fatalf("scenario_id = %q", cfg.ScenarioID)
	}
	if cfg.MeasurementSource != scenario.ChannelBenchMeasurementSourceBenchMeasured || cfg.TestFixture {
		t.Fatalf("measurement/test_fixture = %q/%v", cfg.MeasurementSource, cfg.TestFixture)
	}
	if cfg.ChannelWidthHz != 25000 || cfg.GrossBitrateBps != 19200 || cfg.MeasurementArtifactSHA256 != testBenchLogSHA {
		t.Fatalf("converted channel fields wrong: %+v", cfg)
	}
	reportPath := filepath.Join(t.TempDir(), "report.json")
	if err := run([]string{"calibrate-channel", outPath, "--report", reportPath}); err != nil {
		t.Fatalf("converted channel input rejected by calibrate-channel: %v", err)
	}
}

func TestIT050_ConvertBenchLogWritesTurnaroundInput(t *testing.T) {
	inputPath := writeBenchLog(t, filledTurnaroundBenchLogJSON())
	outPath := filepath.Join(t.TempDir(), "out", "turnaround.json")

	if err := run([]string{"convert-bench-log", inputPath, "--out", outPath}); err != nil {
		t.Fatalf("convert-bench-log turnaround failed: %v", err)
	}

	cfg := readConvertedTurnaroundConfig(t, outPath)
	if cfg.ScenarioID != scenario.TYTBenchMeasurementScenarioID {
		t.Fatalf("scenario_id = %q", cfg.ScenarioID)
	}
	if cfg.MeasurementSource != scenario.TYTBenchMeasurementSourceBenchMeasured || cfg.TestFixture {
		t.Fatalf("measurement/test_fixture = %q/%v", cfg.MeasurementSource, cfg.TestFixture)
	}
	if len(cfg.RxToTxSamplesMs) != 3 || cfg.RxToTxSamplesMs[2] != 8 {
		t.Fatalf("rx_to_tx_samples_ms = %+v", cfg.RxToTxSamplesMs)
	}
	if !strings.Contains(cfg.Notes, "measurement_artifact_sha256="+testBenchLogSHA) {
		t.Fatalf("turnaround notes = %q, want measurement_artifact_sha256", cfg.Notes)
	}
	reportPath := filepath.Join(t.TempDir(), "report.json")
	if err := run([]string{"calibrate-tyt", outPath, "--report", reportPath}); err != nil {
		t.Fatalf("converted turnaround input rejected by calibrate-tyt: %v", err)
	}
}

func TestIT050_ConvertBenchLogWritesAudioInput(t *testing.T) {
	inputPath := writeBenchLog(t, filledAudioBenchLogJSON())
	outPath := filepath.Join(t.TempDir(), "out", "audio.json")

	if err := run([]string{"convert-bench-log", inputPath, "--out", outPath}); err != nil {
		t.Fatalf("convert-bench-log audio failed: %v", err)
	}

	cfg := readConvertedAudioConfig(t, outPath)
	if cfg.ScenarioID != scenario.AudioIntelligibilityScenarioID {
		t.Fatalf("scenario_id = %q", cfg.ScenarioID)
	}
	if cfg.Fixture.EvidenceGrade != scenario.EvidenceGradeBenchMeasured || cfg.Fixture.ArtifactSHA256 != testBenchLogSHA {
		t.Fatalf("fixture = %+v", cfg.Fixture)
	}
	if len(cfg.NoiseProfiles) != 1 || len(cfg.LossProfiles) != 1 {
		t.Fatalf("profile counts = %d/%d", len(cfg.NoiseProfiles), len(cfg.LossProfiles))
	}
	reportPath := filepath.Join(t.TempDir(), "report.json")
	if err := run([]string{"score-audio", outPath, "--report", reportPath}); err != nil {
		t.Fatalf("converted audio input rejected by score-audio: %v", err)
	}
}

func TestConvertBenchLogDeterministicOutput(t *testing.T) {
	inputPath := writeBenchLog(t, filledChannelBenchLogJSON())
	dir := t.TempDir()
	first := filepath.Join(dir, "first.json")
	second := filepath.Join(dir, "nested", "second.json")

	if err := run([]string{"convert-bench-log", inputPath, "--out", first}); err != nil {
		t.Fatalf("first convert-bench-log failed: %v", err)
	}
	if err := run([]string{"convert-bench-log", inputPath, "--out", second}); err != nil {
		t.Fatalf("second convert-bench-log failed: %v", err)
	}
	firstRaw, err := os.ReadFile(first)
	if err != nil {
		t.Fatalf("read first output failed: %v", err)
	}
	secondRaw, err := os.ReadFile(second)
	if err != nil {
		t.Fatalf("read second output failed: %v", err)
	}
	if string(firstRaw) != string(secondRaw) {
		t.Fatalf("converted outputs differ:\nfirst=%s\nsecond=%s", firstRaw, secondRaw)
	}
}

func TestConvertBenchLogRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{name: "malformed", body: `{"schema_version": 1`, wantErr: "decode bench log"},
		{name: "unknown field", body: strings.Replace(filledChannelBenchLogJSON(), `"notes":`, `"unexpected": true, "notes":`, 1), wantErr: "unknown field"},
		{name: "trailing token", body: filledChannelBenchLogJSON() + "\n{}", wantErr: "trailing JSON token"},
		{name: "unsupported artifact type", body: strings.Replace(filledChannelBenchLogJSON(), benchLogArtifactChannel, "other", 1), wantErr: "unsupported artifact_type"},
		{name: "template only", body: strings.Replace(filledChannelBenchLogJSON(), `"status": "captured"`, `"status": "template_only"`, 1), wantErr: "template_only"},
		{name: "tbd convert_to", body: strings.Replace(filledChannelBenchLogJSON(), `"convert_to": "configs/bench/channel-2026-07-04.json"`, `"convert_to": "TBD"`, 1), wantErr: "convert_to"},
		{name: "tbd operator", body: strings.Replace(filledChannelBenchLogJSON(), `"operator": "bench-operator"`, `"operator": "TBD"`, 1), wantErr: "operator"},
		{name: "tbd artifact", body: strings.Replace(filledChannelBenchLogJSON(), `"measurement_artifact_ref": "captures/channel/raw.json"`, `"measurement_artifact_ref": "TBD"`, 1), wantErr: "measurement_artifact_ref"},
		{name: "null hardware claim", body: strings.Replace(filledChannelBenchLogJSON(), `"hardware_claim_allowed": false`, `"hardware_claim_allowed": null`, 1), wantErr: "hardware_claim_allowed must not be null"},
		{name: "zero selected channel width", body: strings.Replace(filledChannelBenchLogJSON(), `"selected_channel_width_hz": 25000`, `"selected_channel_width_hz": 0`, 1), wantErr: "selected_channel_width_hz"},
		{name: "zero channel counter", body: strings.Replace(filledChannelBenchLogJSON(), `"frames_sent": 1000`, `"frames_sent": 0`, 1), wantErr: "frames_sent"},
		{name: "hardware claim true", body: strings.Replace(filledChannelBenchLogJSON(), `"hardware_claim_allowed": false`, `"hardware_claim_allowed": true`, 1), wantErr: "hardware_claim_allowed"},
		{name: "invalid canonical channel", body: strings.Replace(filledChannelBenchLogJSON(), `"frames_decoded": 1000`, `"frames_decoded": 1001`, 1), wantErr: "decoded/error counters"},
		{name: "null audio profiles", body: strings.Replace(filledAudioBenchLogJSON(), `"noise_profiles": [
    { "noise_profile_id": "clean", "noise_snr_db": 40 }
  ]`, `"noise_profiles": null`, 1), wantErr: "noise_profiles must not be null"},
		{name: "dry run audio cannot be bench measured", body: strings.Replace(filledAudioBenchLogJSON(), `"status": "captured"`, `"status": "dry_run_fixture"`, 1), wantErr: "dry_run_fixture audio evidence_grade"},
		{name: "zero audio threshold", body: strings.Replace(filledAudioBenchLogJSON(), `"good_voice_threshold": 75`, `"good_voice_threshold": 0`, 1), wantErr: "good_voice_threshold"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inputPath := writeBenchLog(t, tc.body)
			outPath := filepath.Join(t.TempDir(), "out.json")
			err := run([]string{"convert-bench-log", inputPath, "--out", outPath})
			if err == nil {
				t.Fatal("convert-bench-log accepted invalid input")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want %q", err.Error(), tc.wantErr)
			}
			assertFileMissing(t, outPath)
		})
	}
}

func TestConvertBenchLogRejectsInvalidArgs(t *testing.T) {
	err := run([]string{"convert-bench-log"})
	if err == nil || !strings.Contains(err.Error(), "raw log path") {
		t.Fatalf("missing input error = %v, want raw log path", err)
	}
	err = run([]string{"convert-bench-log", "input.json"})
	if err == nil || !strings.Contains(err.Error(), "--out") {
		t.Fatalf("missing out error = %v, want --out", err)
	}
	err = run([]string{"convert-bench-log", "input.json", "--out", "--help"})
	if err == nil || !strings.Contains(err.Error(), "flag-like") {
		t.Fatalf("flag-like out error = %v, want flag-like", err)
	}
}

func writeBenchLog(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bench-log.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write bench log failed: %v", err)
	}
	return path
}

func readConvertedChannelConfig(t *testing.T, path string) scenario.ChannelBenchArtifactConfig {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read channel config failed: %v", err)
	}
	var cfg scenario.ChannelBenchArtifactConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("decode channel config failed: %v\n%s", err, raw)
	}
	return cfg
}

func readConvertedTurnaroundConfig(t *testing.T, path string) scenario.TYTBenchMeasurementConfig {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read turnaround config failed: %v", err)
	}
	var cfg scenario.TYTBenchMeasurementConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("decode turnaround config failed: %v\n%s", err, raw)
	}
	return cfg
}

func readConvertedAudioConfig(t *testing.T, path string) scenario.AudioIntelligibilityConfig {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audio config failed: %v", err)
	}
	var cfg scenario.AudioIntelligibilityConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("decode audio config failed: %v\n%s", err, raw)
	}
	return cfg
}

func filledChannelBenchLogJSON() string {
	return `{
  "schema_version": 1,
  "artifact_type": "channel_bench_log_template",
  "status": "captured",
  "hardware_claim_allowed": false,
  "field_tx_ready": false,
  "target_radio_family": "TYT MD-UV380 / MD-UV3x0 class",
  "operator": "bench-operator",
  "captured_at_utc": "2026-07-04T12:00:00Z",
  "lab_setup_ref": "bench/no-rf/channel-v1",
  "convert_to": "configs/bench/channel-2026-07-04.json",
  "candidate_channel_width_hz": [12500, 25000],
  "candidate_gross_bitrate_bps": [9600, 19200],
  "selected_channel_width_hz": 25000,
  "selected_gross_bitrate_bps": 19200,
  "modulation_profile": "custom_2fsk_25khz_bench",
  "frames_sent": 1000,
  "frames_decoded": 1000,
  "crc_error_count": 0,
  "duration_ms": 10000,
  "measurement_artifact_ref": "captures/channel/raw.json",
  "measurement_artifact_sha256": "` + testBenchLogSHA + `",
  "notes": "bench capture example"
}`
}

func filledTurnaroundBenchLogJSON() string {
	return `{
  "schema_version": 1,
  "artifact_type": "turnaround_bench_log_template",
  "status": "captured",
  "hardware_claim_allowed": false,
  "field_tx_ready": false,
  "target_radio_family": "TYT MD-UV380 / MD-UV3x0 class",
  "operator": "bench-operator",
  "captured_at_utc": "2026-07-04T12:00:00Z",
  "lab_setup_ref": "bench/no-rf/turnaround-v1",
  "convert_to": "configs/measurements/tyt-md-uv380-2026-07-04.json",
  "firmware_path": "openrtx_derived_bench",
  "control_path_under_test": "radio_enableRx/radio_enableTx",
  "channel_width_hz": 25000,
  "gross_bitrate_bps": 19200,
  "preamble_ms": 1,
  "sync_ms": 1,
  "link_delay_ms": 1,
  "rx_to_tx_samples_ms": [6, 8, 8],
  "tx_to_rx_samples_ms": [7, 8, 8],
  "rx_valid_after_tx_samples_ms": [1, 1, 2],
  "measurement_artifact_ref": "captures/turnaround/raw.json",
  "measurement_artifact_sha256": "` + testBenchLogSHA + `",
  "notes": "bench capture example"
}`
}

func filledAudioBenchLogJSON() string {
	return `{
  "schema_version": 1,
  "artifact_type": "audio_bench_log_template",
  "status": "captured",
  "hardware_claim_allowed": false,
  "field_tx_ready": false,
  "target_radio_family": "TYT MD-UV380 / MD-UV3x0 class",
  "operator": "bench-operator",
  "captured_at_utc": "2026-07-04T12:00:00Z",
  "lab_setup_ref": "bench/no-rf/audio-v1",
  "convert_to": "configs/bench/audio-2026-07-04.json",
  "fixture_or_capture_id": "tyt_audio_capture_v1",
  "fixture_or_capture_path": "captures/audio/voice.wav",
  "artifact_sha256": "` + testBenchLogSHA + `",
  "evidence_grade": "bench_measured",
  "codec_class": "opus_class_wideband_speech_proxy",
  "audio_bandwidth_hz": 8000,
  "scoring_method": "deterministic_fixture_proxy_v1",
  "good_voice_threshold": 75,
  "noise_profiles": [
    { "noise_profile_id": "clean", "noise_snr_db": 40 }
  ],
  "loss_profiles": [
    { "loss_profile_id": "no_loss", "packet_loss_pct": 0, "burst_loss_ms": 0 }
  ],
  "notes": "bench capture example"
}`
}
