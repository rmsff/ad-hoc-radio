package main

import (
	"path/filepath"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

func TestIT051_BenchSessionDryRunRawLogsProduceBlockedReadiness(t *testing.T) {
	dir := t.TempDir()
	inputDir := filepath.Join(dir, "inputs")
	reportDir := filepath.Join(dir, "reports")
	channelInput := filepath.Join(inputDir, "channel.json")
	turnaroundInput := filepath.Join(inputDir, "turnaround.json")
	audioInput := filepath.Join(inputDir, "audio.json")
	channelReportPath := filepath.Join(reportDir, "channel-report.json")
	turnaroundReportPath := filepath.Join(reportDir, "turnaround-report.json")
	audioReportPath := filepath.Join(reportDir, "audio-report.json")
	readinessReportPath := filepath.Join(reportDir, "hardware-readiness.json")

	if err := run([]string{
		"convert-bench-log",
		benchDryRunPath("channel-dry-run-log.json"),
		"--out", channelInput,
	}); err != nil {
		t.Fatalf("convert-bench-log channel dry-run failed: %v", err)
	}
	if err := run([]string{
		"convert-bench-log",
		benchDryRunPath("turnaround-dry-run-log.json"),
		"--out", turnaroundInput,
	}); err != nil {
		t.Fatalf("convert-bench-log turnaround dry-run failed: %v", err)
	}
	if err := run([]string{
		"convert-bench-log",
		benchDryRunPath("audio-dry-run-log.json"),
		"--out", audioInput,
	}); err != nil {
		t.Fatalf("convert-bench-log audio dry-run failed: %v", err)
	}

	if err := run([]string{"calibrate-channel", channelInput, "--report", channelReportPath}); err != nil {
		t.Fatalf("calibrate-channel dry-run failed: %v", err)
	}
	if err := run([]string{"calibrate-tyt", turnaroundInput, "--report", turnaroundReportPath}); err != nil {
		t.Fatalf("calibrate-tyt dry-run failed: %v", err)
	}
	if err := run([]string{"score-audio", audioInput, "--report", audioReportPath}); err != nil {
		t.Fatalf("score-audio dry-run failed: %v", err)
	}
	if err := run([]string{
		"hardware-readiness",
		"--channel", channelReportPath,
		"--turnaround", turnaroundReportPath,
		"--audio", audioReportPath,
		"--report", readinessReportPath,
	}); err != nil {
		t.Fatalf("hardware-readiness dry-run failed: %v", err)
	}

	channel := readChannelBenchArtifactReport(t, channelReportPath)
	if channel.MeasurementSource != scenario.ChannelBenchMeasurementSourceFixtureOnly || !channel.TestFixture {
		t.Fatalf("channel source/fixture = %q/%v, want fixture_only/true", channel.MeasurementSource, channel.TestFixture)
	}
	if channel.HardwareClaimAllowed || channel.ChannelBenchVerified {
		t.Fatalf("channel claim/verified = %v/%v, want false/false", channel.HardwareClaimAllowed, channel.ChannelBenchVerified)
	}

	turnaround := readTYTBenchMeasurementReport(t, turnaroundReportPath)
	if turnaround.MeasurementSource != scenario.TYTBenchMeasurementSourceFixtureOnly || !turnaround.TestFixture {
		t.Fatalf("turnaround source/fixture = %q/%v, want fixture_only/true", turnaround.MeasurementSource, turnaround.TestFixture)
	}
	if turnaround.HardwareClaimAllowed || turnaround.MeasuredHardwareCapability {
		t.Fatalf("turnaround claim/measured = %v/%v, want false/false", turnaround.HardwareClaimAllowed, turnaround.MeasuredHardwareCapability)
	}

	audio := readAudioIntelligibilityReport(t, audioReportPath)
	if audio.GoodVoicePassRows == 0 {
		t.Fatalf("audio GoodVoicePassRows = 0, want fixture dry-run audio evidence to score")
	}

	readiness := readPortableHardwareReadinessReport(t, readinessReportPath)
	if readiness.Outcome != scenario.PortableHardwareOutcomeBlocked || readiness.BenchExperimentReady {
		t.Fatalf("readiness outcome/bench = %q/%v, want blocked/false", readiness.Outcome, readiness.BenchExperimentReady)
	}
	if readiness.HardwareClaimAllowed || readiness.FieldTXReady {
		t.Fatalf("readiness claim/field = %v/%v, want false/false", readiness.HardwareClaimAllowed, readiness.FieldTXReady)
	}
	if !stringSliceContains(readiness.DegradeReasons, "channel_bitrate_bench_verified") ||
		!stringSliceContains(readiness.DegradeReasons, "tx_rx_turnaround_bench_measured") {
		t.Fatalf("readiness DegradeReasons = %v, want channel and turnaround blockers", readiness.DegradeReasons)
	}
}

func benchDryRunPath(name string) string {
	return filepath.Join("..", "..", "configs", "bench", "dry-run", name)
}
