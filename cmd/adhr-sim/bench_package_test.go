package main

import (
	"path/filepath"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

func TestIT043_BenchPackageFixtureFlowProducesBlockedReadiness(t *testing.T) {
	dir := t.TempDir()
	channelReport := filepath.Join(dir, "channel-report.json")
	turnaroundReport := filepath.Join(dir, "turnaround-report.json")
	audioReport := filepath.Join(dir, "audio-report.json")
	readinessReport := filepath.Join(dir, "hardware-readiness.json")

	if err := run([]string{
		"calibrate-channel",
		benchPackagePath("channel-fixture.json"),
		"--report", channelReport,
	}); err != nil {
		t.Fatalf("calibrate-channel bench package command failed: %v", err)
	}
	if err := run([]string{
		"calibrate-tyt",
		fixtureTYTMeasurementPath(),
		"--report", turnaroundReport,
	}); err != nil {
		t.Fatalf("calibrate-tyt bench package command failed: %v", err)
	}
	if err := run([]string{
		"score-audio",
		benchPackagePath("audio-fixture.json"),
		"--report", audioReport,
	}); err != nil {
		t.Fatalf("score-audio bench package command failed: %v", err)
	}
	if err := run([]string{
		"hardware-readiness",
		"--channel", channelReport,
		"--turnaround", turnaroundReport,
		"--audio", audioReport,
		"--report", readinessReport,
	}); err != nil {
		t.Fatalf("hardware-readiness bench package command failed: %v", err)
	}

	report := readPortableHardwareReadinessReport(t, readinessReport)
	if report.Outcome != scenario.PortableHardwareOutcomeBlocked {
		t.Fatalf("Outcome = %q, want %q", report.Outcome, scenario.PortableHardwareOutcomeBlocked)
	}
	if report.BenchExperimentReady {
		t.Fatal("BenchExperimentReady = true, want false for fixture-only bench package")
	}
	if report.HardwareClaimAllowed || report.FieldTXReady {
		t.Fatalf("hardware/field readiness = %v/%v, want false/false",
			report.HardwareClaimAllowed, report.FieldTXReady)
	}
	if !stringSliceContains(report.DegradeReasons, "channel_bitrate_bench_verified") ||
		!stringSliceContains(report.DegradeReasons, "tx_rx_turnaround_bench_measured") {
		t.Fatalf("DegradeReasons = %v, want channel and turnaround blockers", report.DegradeReasons)
	}
}

func stringSliceContains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func benchPackagePath(name string) string {
	return filepath.Join("..", "..", "configs", "bench", name)
}
