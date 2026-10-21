package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

func TestIT030_CalibrateTYTCommandWritesReport(t *testing.T) {
	reportPath := filepath.Join(t.TempDir(), "out", "tyt-calibration.json")

	if err := run([]string{"calibrate-tyt", fixtureTYTMeasurementPath(), "--report", reportPath}); err != nil {
		t.Fatalf("calibrate-tyt command failed: %v", err)
	}

	report := readTYTBenchMeasurementReport(t, reportPath)
	if report.ScenarioID != scenario.TYTBenchMeasurementScenarioID {
		t.Fatalf("scenario_id = %q, want %q", report.ScenarioID, scenario.TYTBenchMeasurementScenarioID)
	}
	if report.CalibrationDecision != scenario.TYTBenchDecisionPrimary24ms {
		t.Fatalf("calibration_decision = %q, want %q", report.CalibrationDecision, scenario.TYTBenchDecisionPrimary24ms)
	}
	if report.SelectedTupleSlotMs != 24 || report.TupleThresholdMs != 10 {
		t.Fatalf("slot/threshold = %d/%d, want 24/10", report.SelectedTupleSlotMs, report.TupleThresholdMs)
	}
	if report.HardwareClaimAllowed || report.MeasuredHardwareCapability || !report.AssumptionOnly {
		t.Fatalf("claim gates = allowed:%v measured:%v assumption:%v, want false/false/true",
			report.HardwareClaimAllowed, report.MeasuredHardwareCapability, report.AssumptionOnly)
	}
	if report.RxToTx.SummarySource != scenario.TYTBenchSummarySourceRawSamples || report.RxToTx.SampleCount != 3 {
		t.Fatalf("rx_to_tx summary = %q/%d, want raw_samples/3", report.RxToTx.SummarySource, report.RxToTx.SampleCount)
	}
	if report.EventHash == "" {
		t.Fatal("event_hash is empty")
	}
}

func TestIT030_CalibrateTYTCommandDeterministicReport(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.json")
	second := filepath.Join(dir, "nested", "second.json")

	if err := run([]string{"calibrate-tyt", fixtureTYTMeasurementPath(), "--report", first}); err != nil {
		t.Fatalf("first calibrate-tyt command failed: %v", err)
	}
	if err := run([]string{"calibrate-tyt", fixtureTYTMeasurementPath(), "--report", second}); err != nil {
		t.Fatalf("second calibrate-tyt command failed: %v", err)
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
		t.Fatalf("calibration reports differ:\nfirst=%s\nsecond=%s", firstRaw, secondRaw)
	}
}

func TestIT030_CalibrateTYTCommandAcceptsPrecomputedSummary(t *testing.T) {
	inputPath := writeTYTMeasurement(t, `{
  "scenario_id": "tyt_tx_rx_switch_bench_measurement",
  "seed": 42,
  "measurement_source": "bench_measured",
  "radio_model": "TYT MD-UV380 / MD-UV3x0 class",
  "test_fixture": false,
  "measurement_artifact_ref": "bench/tyt-md-uv380/2026-07-01/switch-p95.json",
  "firmware_path": "openrtx_derived_bench",
  "channel_width_hz": 25000,
  "gross_bitrate_bps": 19200,
  "preamble_ms": 1,
  "sync_ms": 1,
  "link_delay_ms": 1,
  "rx_to_tx_p50_ms": 6,
  "rx_to_tx_p95_ms": 6,
  "tx_to_rx_p50_ms": 5,
  "tx_to_rx_p95_ms": 5,
  "rx_valid_after_tx_p50_ms": 2,
  "rx_valid_after_tx_p95_ms": 2,
  "notes": "bench-measured example with non-test artifact provenance"
}`)
	reportPath := filepath.Join(t.TempDir(), "report.json")

	if err := run([]string{"calibrate-tyt", inputPath, "--report", reportPath}); err != nil {
		t.Fatalf("calibrate-tyt command failed: %v", err)
	}

	report := readTYTBenchMeasurementReport(t, reportPath)
	if report.CalibrationDecision != scenario.TYTBenchDecisionIntermediate20ms {
		t.Fatalf("calibration_decision = %q, want %q", report.CalibrationDecision, scenario.TYTBenchDecisionIntermediate20ms)
	}
	if !report.HardwareClaimAllowed || !report.MeasuredHardwareCapability || report.AssumptionOnly {
		t.Fatalf("claim gates = allowed:%v measured:%v assumption:%v, want true/true/false",
			report.HardwareClaimAllowed, report.MeasuredHardwareCapability, report.AssumptionOnly)
	}
	if report.RxToTx.SummarySource != scenario.TYTBenchSummarySourcePrecomputed || report.RxToTx.SampleCount != 0 {
		t.Fatalf("rx_to_tx summary = %q/%d, want precomputed/0", report.RxToTx.SummarySource, report.RxToTx.SampleCount)
	}
}

func TestCalibrateTYTCommandRejectsInvalidArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing input", args: []string{"calibrate-tyt"}, wantErr: "measurement path"},
		{name: "missing report", args: []string{"calibrate-tyt", fixtureTYTMeasurementPath()}, wantErr: "--report"},
		{name: "flag like report", args: []string{"calibrate-tyt", fixtureTYTMeasurementPath(), "--report", "--help"}, wantErr: "flag-like"},
		{name: "duplicate report", args: []string{"calibrate-tyt", fixtureTYTMeasurementPath(), "--report", "a", "--report", "b"}, wantErr: "more than once"},
		{name: "unknown flag", args: []string{"calibrate-tyt", "--unknown"}, wantErr: "unknown flag"},
		{name: "unexpected arg", args: []string{"calibrate-tyt", fixtureTYTMeasurementPath(), "extra", "--report", "out.json"}, wantErr: "unexpected argument"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := run(tc.args)
			if err == nil {
				t.Fatalf("run(%v) succeeded, want error", tc.args)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestCalibrateTYTCommandRejectsMissingMeasurementFile(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "missing.json")
	reportPath := filepath.Join(t.TempDir(), "report.json")

	err := run([]string{"calibrate-tyt", inputPath, "--report", reportPath})
	if err == nil {
		t.Fatal("calibrate-tyt accepted missing measurement file")
	}
	if !strings.Contains(err.Error(), "read TYT bench measurement") {
		t.Fatalf("error = %q, want read context", err.Error())
	}
	assertFileMissing(t, reportPath)
}

func TestCalibrateTYTCommandRejectsMalformedJSON(t *testing.T) {
	inputPath := writeTYTMeasurement(t, `{"scenario_id": "tyt_tx_rx_switch_bench_measurement"`)
	reportPath := filepath.Join(t.TempDir(), "report.json")

	err := run([]string{"calibrate-tyt", inputPath, "--report", reportPath})
	if err == nil {
		t.Fatal("calibrate-tyt accepted malformed JSON")
	}
	if !strings.Contains(err.Error(), "decode TYT bench measurement") {
		t.Fatalf("error = %q, want decode context", err.Error())
	}
	assertFileMissing(t, reportPath)
}

func TestCalibrateTYTCommandRejectsUnknownJSONField(t *testing.T) {
	inputPath := writeTYTMeasurement(t, `{
  "scenario_id": "tyt_tx_rx_switch_bench_measurement",
  "measurement_source": "fixture_only",
  "radio_model": "TYT MD-UV380 / MD-UV3x0 class",
  "test_fixture": true,
  "channel_width_hz": 25000,
  "gross_bitrate_bps": 19200,
  "preamble_ms": 1,
  "sync_ms": 1,
  "link_delay_ms": 1,
  "rx_to_tx_samples_ms": [6, 8, 8],
  "tx_to_rx_samples_ms": [7, 8, 8],
  "rx_valid_after_tx_samples_ms": [1, 1, 2],
  "unexpected": true
}`)
	reportPath := filepath.Join(t.TempDir(), "report.json")

	err := run([]string{"calibrate-tyt", inputPath, "--report", reportPath})
	if err == nil {
		t.Fatal("calibrate-tyt accepted unknown JSON field")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("error = %q, want unknown field context", err.Error())
	}
	assertFileMissing(t, reportPath)
}

func TestCalibrateTYTCommandRejectsTrailingJSONToken(t *testing.T) {
	inputPath := writeTYTMeasurement(t, `{
  "scenario_id": "tyt_tx_rx_switch_bench_measurement",
  "measurement_source": "fixture_only",
  "radio_model": "TYT MD-UV380 / MD-UV3x0 class",
  "test_fixture": true,
  "channel_width_hz": 25000,
  "gross_bitrate_bps": 19200,
  "preamble_ms": 1,
  "sync_ms": 1,
  "link_delay_ms": 1,
  "rx_to_tx_samples_ms": [6, 8, 8],
  "tx_to_rx_samples_ms": [7, 8, 8],
  "rx_valid_after_tx_samples_ms": [1, 1, 2]
}
{}`)
	reportPath := filepath.Join(t.TempDir(), "report.json")

	err := run([]string{"calibrate-tyt", inputPath, "--report", reportPath})
	if err == nil {
		t.Fatal("calibrate-tyt accepted trailing JSON token")
	}
	if !strings.Contains(err.Error(), "trailing JSON token") {
		t.Fatalf("error = %q, want trailing token context", err.Error())
	}
	assertFileMissing(t, reportPath)
}

func TestCalibrateTYTCommandRejectsInvalidMeasurement(t *testing.T) {
	inputPath := writeTYTMeasurement(t, `{
  "scenario_id": "tyt_tx_rx_switch_bench_measurement",
  "seed": 42,
  "measurement_source": "bench_measured",
  "radio_model": "TYT MD-UV380 / MD-UV3x0 class",
  "test_fixture": true,
  "measurement_artifact_ref": "bench/fake.json",
  "channel_width_hz": 25000,
  "gross_bitrate_bps": 19200,
  "preamble_ms": 1,
  "sync_ms": 1,
  "link_delay_ms": 1,
  "rx_to_tx_samples_ms": [6, 8, 8],
  "tx_to_rx_samples_ms": [7, 8, 8],
  "rx_valid_after_tx_samples_ms": [1, 1, 2]
}`)
	reportPath := filepath.Join(t.TempDir(), "report.json")

	err := run([]string{"calibrate-tyt", inputPath, "--report", reportPath})
	if err == nil {
		t.Fatal("calibrate-tyt accepted invalid measurement")
	}
	if !strings.Contains(err.Error(), "test_fixture=false") {
		t.Fatalf("error = %q, want validation context", err.Error())
	}
	assertFileMissing(t, reportPath)
}

func TestCalibrateTYTCommandRejectsWhitespaceBenchArtifactRef(t *testing.T) {
	inputPath := writeTYTMeasurement(t, `{
  "scenario_id": "tyt_tx_rx_switch_bench_measurement",
  "seed": 42,
  "measurement_source": "bench_measured",
  "radio_model": "TYT MD-UV380 / MD-UV3x0 class",
  "test_fixture": false,
  "measurement_artifact_ref": "   ",
  "channel_width_hz": 25000,
  "gross_bitrate_bps": 19200,
  "preamble_ms": 1,
  "sync_ms": 1,
  "link_delay_ms": 1,
  "rx_to_tx_samples_ms": [6, 8, 8],
  "tx_to_rx_samples_ms": [7, 8, 8],
  "rx_valid_after_tx_samples_ms": [1, 1, 2]
}`)
	reportPath := filepath.Join(t.TempDir(), "report.json")

	err := run([]string{"calibrate-tyt", inputPath, "--report", reportPath})
	if err == nil {
		t.Fatal("calibrate-tyt accepted whitespace-only bench artifact ref")
	}
	if !strings.Contains(err.Error(), "measurement_artifact_ref") {
		t.Fatalf("error = %q, want measurement_artifact_ref context", err.Error())
	}
	assertFileMissing(t, reportPath)
}

func TestCalibrateTYTCommandRejectsUnsupportedScenarioID(t *testing.T) {
	inputPath := writeTYTMeasurement(t, `{
  "scenario_id": "not_tyt_calibration",
  "measurement_source": "fixture_only",
  "radio_model": "TYT MD-UV380 / MD-UV3x0 class",
  "test_fixture": true,
  "channel_width_hz": 25000,
  "gross_bitrate_bps": 19200,
  "preamble_ms": 1,
  "sync_ms": 1,
  "link_delay_ms": 1,
  "rx_to_tx_samples_ms": [6, 8, 8],
  "tx_to_rx_samples_ms": [7, 8, 8],
  "rx_valid_after_tx_samples_ms": [1, 1, 2]
}`)
	reportPath := filepath.Join(t.TempDir(), "report.json")

	err := run([]string{"calibrate-tyt", inputPath, "--report", reportPath})
	if err == nil {
		t.Fatal("calibrate-tyt accepted unsupported scenario_id")
	}
	if !strings.Contains(err.Error(), "unsupported scenario_id") {
		t.Fatalf("error = %q, want unsupported scenario_id context", err.Error())
	}
	assertFileMissing(t, reportPath)
}

func TestCalibrateTYTCommandRejectsMissingScenarioIdentity(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "missing scenario id", want: "scenario_id is required", body: `{
  "seed": 42,
  "measurement_source": "fixture_only",
  "radio_model": "TYT MD-UV380 / MD-UV3x0 class",
  "test_fixture": true,
  "channel_width_hz": 25000,
  "gross_bitrate_bps": 19200,
  "preamble_ms": 1,
  "sync_ms": 1,
  "link_delay_ms": 1,
  "rx_to_tx_samples_ms": [6, 8, 8],
  "tx_to_rx_samples_ms": [7, 8, 8],
  "rx_valid_after_tx_samples_ms": [1, 1, 2]
}`},
		{name: "missing seed", want: "seed is required", body: `{
  "scenario_id": "tyt_tx_rx_switch_bench_measurement",
  "measurement_source": "fixture_only",
  "radio_model": "TYT MD-UV380 / MD-UV3x0 class",
  "test_fixture": true,
  "channel_width_hz": 25000,
  "gross_bitrate_bps": 19200,
  "preamble_ms": 1,
  "sync_ms": 1,
  "link_delay_ms": 1,
  "rx_to_tx_samples_ms": [6, 8, 8],
  "tx_to_rx_samples_ms": [7, 8, 8],
  "rx_valid_after_tx_samples_ms": [1, 1, 2]
}`},
		{name: "null scenario id", want: "scenario_id must not be null", body: `{
  "scenario_id": null,
  "seed": 42,
  "measurement_source": "fixture_only",
  "radio_model": "TYT MD-UV380 / MD-UV3x0 class",
  "test_fixture": true,
  "channel_width_hz": 25000,
  "gross_bitrate_bps": 19200,
  "preamble_ms": 1,
  "sync_ms": 1,
  "link_delay_ms": 1,
  "rx_to_tx_samples_ms": [6, 8, 8],
  "tx_to_rx_samples_ms": [7, 8, 8],
  "rx_valid_after_tx_samples_ms": [1, 1, 2]
}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inputPath := writeTYTMeasurement(t, tc.body)
			reportPath := filepath.Join(t.TempDir(), "report.json")

			err := run([]string{"calibrate-tyt", inputPath, "--report", reportPath})
			if err == nil {
				t.Fatal("calibrate-tyt accepted missing/invalid identity")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want %q", err.Error(), tc.want)
			}
			assertFileMissing(t, reportPath)
		})
	}
}

func TestCalibrateTYTCommandRejectsAmbiguousSampleFields(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "empty samples with summary", want: "cannot be combined", body: `{
  "scenario_id": "tyt_tx_rx_switch_bench_measurement",
  "seed": 42,
  "measurement_source": "fixture_only",
  "radio_model": "TYT MD-UV380 / MD-UV3x0 class",
  "test_fixture": true,
  "channel_width_hz": 25000,
  "gross_bitrate_bps": 19200,
  "preamble_ms": 1,
  "sync_ms": 1,
  "link_delay_ms": 1,
  "rx_to_tx_samples_ms": [],
  "rx_to_tx_p50_ms": 8,
  "rx_to_tx_p95_ms": 8,
  "tx_to_rx_samples_ms": [7, 8, 8],
  "rx_valid_after_tx_samples_ms": [1, 1, 2]
}`},
		{name: "null samples", want: "must not be null", body: `{
  "scenario_id": "tyt_tx_rx_switch_bench_measurement",
  "seed": 42,
  "measurement_source": "fixture_only",
  "radio_model": "TYT MD-UV380 / MD-UV3x0 class",
  "test_fixture": true,
  "channel_width_hz": 25000,
  "gross_bitrate_bps": 19200,
  "preamble_ms": 1,
  "sync_ms": 1,
  "link_delay_ms": 1,
  "rx_to_tx_samples_ms": null,
  "tx_to_rx_samples_ms": [7, 8, 8],
  "rx_valid_after_tx_samples_ms": [1, 1, 2]
}`},
		{name: "empty samples without summary", want: "must be non-empty", body: `{
  "scenario_id": "tyt_tx_rx_switch_bench_measurement",
  "seed": 42,
  "measurement_source": "fixture_only",
  "radio_model": "TYT MD-UV380 / MD-UV3x0 class",
  "test_fixture": true,
  "channel_width_hz": 25000,
  "gross_bitrate_bps": 19200,
  "preamble_ms": 1,
  "sync_ms": 1,
  "link_delay_ms": 1,
  "rx_to_tx_samples_ms": [],
  "tx_to_rx_samples_ms": [7, 8, 8],
  "rx_valid_after_tx_samples_ms": [1, 1, 2]
}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inputPath := writeTYTMeasurement(t, tc.body)
			reportPath := filepath.Join(t.TempDir(), "report.json")

			err := run([]string{"calibrate-tyt", inputPath, "--report", reportPath})
			if err == nil {
				t.Fatal("calibrate-tyt accepted ambiguous sample field")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want %q", err.Error(), tc.want)
			}
			assertFileMissing(t, reportPath)
		})
	}
}

func TestCalibrateTYTCommandDoesNotLeavePartialReportOnWriteError(t *testing.T) {
	parentFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parentFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write parent file failed: %v", err)
	}
	reportPath := filepath.Join(parentFile, "report.json")

	err := run([]string{"calibrate-tyt", fixtureTYTMeasurementPath(), "--report", reportPath})
	if err == nil {
		t.Fatal("calibrate-tyt succeeded with unwritable report path")
	}
	assertFileMissing(t, reportPath)
}

func TestCalibrateTYTCommandRejectsDirectoryShapedReportPath(t *testing.T) {
	reportPath := filepath.Join(t.TempDir(), "out") + string(os.PathSeparator)

	err := run([]string{"calibrate-tyt", fixtureTYTMeasurementPath(), "--report", reportPath})
	if err == nil {
		t.Fatal("calibrate-tyt accepted directory-shaped report path")
	}
	if !strings.Contains(err.Error(), "must name a file") {
		t.Fatalf("error = %q, want file path context", err.Error())
	}
	if _, err := os.Stat(reportPath); !os.IsNotExist(err) {
		t.Fatalf("directory-shaped report target exists or returned unexpected error: %v", err)
	}
}

func TestCalibrateTYTCommandHelpDoesNotRequireMeasurementPath(t *testing.T) {
	if err := run([]string{"calibrate-tyt", "--help"}); err != nil {
		t.Fatalf("calibrate-tyt --help returned error: %v", err)
	}
}

func readTYTBenchMeasurementReport(t *testing.T, path string) scenario.TYTBenchMeasurementReport {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read TYT calibration report failed: %v", err)
	}
	var report scenario.TYTBenchMeasurementReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("unmarshal TYT calibration report failed: %v\n%s", err, raw)
	}
	return report
}

func fixtureTYTMeasurementPath() string {
	return filepath.Join("..", "..", "configs", "measurements", "tyt-md-uv380-fixture.json")
}

func writeTYTMeasurement(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tyt-measurement.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write TYT measurement failed: %v", err)
	}
	return path
}
