package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

func TestIT042_HardwareReadinessCommandWritesBenchReadyReport(t *testing.T) {
	paths := writeHardwareReadinessInputReports(t, true)
	reportPath := filepath.Join(t.TempDir(), "out", "hardware-readiness.json")

	if err := run([]string{
		"hardware-readiness",
		"--channel", paths.channel,
		"--turnaround", paths.turnaround,
		"--audio", paths.audio,
		"--report", reportPath,
	}); err != nil {
		t.Fatalf("hardware-readiness command failed: %v", err)
	}

	report := readPortableHardwareReadinessReport(t, reportPath)
	if report.ScenarioID != scenario.PortableHardwareReadinessScenarioID {
		t.Fatalf("scenario_id = %q, want %q", report.ScenarioID, scenario.PortableHardwareReadinessScenarioID)
	}
	if !report.BenchExperimentReady || report.Outcome != scenario.PortableHardwareOutcomeBenchReady {
		t.Fatalf("bench/outcome = %v/%q, want true/bench_ready; reasons=%v",
			report.BenchExperimentReady, report.Outcome, report.DegradeReasons)
	}
	if report.HardwareClaimAllowed || report.FieldTXReady {
		t.Fatalf("claim gates = hardware:%v field:%v, want false/false", report.HardwareClaimAllowed, report.FieldTXReady)
	}
}

func TestIT042_HardwareReadinessCommandBlocksMissingEvidence(t *testing.T) {
	paths := writeHardwareReadinessInputReports(t, false)
	reportPath := filepath.Join(t.TempDir(), "hardware-readiness.json")

	if err := run([]string{
		"hardware-readiness",
		"--channel", paths.channel,
		"--turnaround", paths.turnaround,
		"--audio", paths.audio,
		"--report", reportPath,
	}); err != nil {
		t.Fatalf("hardware-readiness command failed: %v", err)
	}

	report := readPortableHardwareReadinessReport(t, reportPath)
	if report.BenchExperimentReady || report.Outcome != scenario.PortableHardwareOutcomeBlocked {
		t.Fatalf("bench/outcome = %v/%q, want false/blocked", report.BenchExperimentReady, report.Outcome)
	}
	if !stringSliceContains(report.DegradeReasons, "channel_bitrate_bench_verified") ||
		!stringSliceContains(report.DegradeReasons, "tx_rx_turnaround_bench_measured") {
		t.Fatalf("DegradeReasons = %v, want channel and turnaround blockers", report.DegradeReasons)
	}
}

func TestIT042_HardwareReadinessCommandBlocksTransmitWithoutLabGates(t *testing.T) {
	paths := writeHardwareReadinessInputReports(t, true)
	reportPath := filepath.Join(t.TempDir(), "hardware-readiness.json")

	if err := run([]string{
		"hardware-readiness",
		"--channel", paths.channel,
		"--turnaround", paths.turnaround,
		"--audio", paths.audio,
		"--tx-requested",
		"--report", reportPath,
	}); err != nil {
		t.Fatalf("hardware-readiness command failed: %v", err)
	}

	report := readPortableHardwareReadinessReport(t, reportPath)
	if report.Outcome != scenario.PortableHardwareOutcomeFieldBlocked {
		t.Fatalf("Outcome = %q, want field blocked; reasons=%v", report.Outcome, report.DegradeReasons)
	}
	if report.FieldTXReady || report.HardwareClaimAllowed {
		t.Fatalf("field/hardware = %v/%v, want false/false", report.FieldTXReady, report.HardwareClaimAllowed)
	}
}

func TestHardwareReadinessCommandRejectsInvalidArgsAndInputs(t *testing.T) {
	err := run([]string{"hardware-readiness"})
	if err == nil || !strings.Contains(err.Error(), "--channel") {
		t.Fatalf("missing channel error = %v, want --channel", err)
	}
	paths := writeHardwareReadinessInputReports(t, true)
	reportPath := filepath.Join(t.TempDir(), "hardware-readiness.json")
	err = run([]string{
		"hardware-readiness",
		"--channel", paths.channel,
		"--turnaround", paths.turnaround,
		"--audio", "--report",
		"--report", reportPath,
	})
	if err == nil || !strings.Contains(err.Error(), "flag-like") {
		t.Fatalf("flag-like audio error = %v, want flag-like", err)
	}

	badChannel := filepath.Join(t.TempDir(), "bad-channel.json")
	if err := os.WriteFile(badChannel, []byte(`{"scenario_id":"other"}`), 0o644); err != nil {
		t.Fatalf("write bad channel failed: %v", err)
	}
	err = run([]string{
		"hardware-readiness",
		"--channel", badChannel,
		"--turnaround", paths.turnaround,
		"--audio", paths.audio,
		"--report", reportPath,
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported scenario_id") {
		t.Fatalf("bad channel error = %v, want unsupported scenario", err)
	}
	assertFileMissing(t, reportPath)
}

func TestHardwareReadinessCommandRejectsForgedReportConsistency(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*scenario.ChannelBenchArtifactReport, *scenario.TYTBenchMeasurementReport, *scenario.AudioIntelligibilityReport)
		wantText string
	}{
		{
			name: "forged channel fixture marked verified",
			mutate: func(channel *scenario.ChannelBenchArtifactReport, _ *scenario.TYTBenchMeasurementReport, _ *scenario.AudioIntelligibilityReport) {
				channel.MeasurementSource = scenario.ChannelBenchMeasurementSourceFixtureOnly
				channel.TestFixture = true
			},
			wantText: "channel hardware claim flags",
		},
		{
			name: "forged channel derived counters",
			mutate: func(channel *scenario.ChannelBenchArtifactReport, _ *scenario.TYTBenchMeasurementReport, _ *scenario.AudioIntelligibilityReport) {
				channel.DecodeSuccessPermille = 0
				channel.CRCErrorPermille = 1000
			},
			wantText: "channel derived counters",
		},
		{
			name: "forged turnaround fixture marked measured",
			mutate: func(_ *scenario.ChannelBenchArtifactReport, turnaround *scenario.TYTBenchMeasurementReport, _ *scenario.AudioIntelligibilityReport) {
				turnaround.MeasurementSource = scenario.TYTBenchMeasurementSourceFixtureOnly
				turnaround.TestFixture = true
			},
			wantText: "TYT hardware claim flags",
		},
		{
			name: "forged turnaround stale max p95",
			mutate: func(_ *scenario.ChannelBenchArtifactReport, turnaround *scenario.TYTBenchMeasurementReport, _ *scenario.AudioIntelligibilityReport) {
				turnaround.RxToTx.P95Ms = 200
				turnaround.TxToRx.P95Ms = 180
				turnaround.RxValidAfterTx.P95Ms = 160
			},
			wantText: "TYT turnaround max p95",
		},
		{
			name: "forged audio degraded row above threshold",
			mutate: func(_ *scenario.ChannelBenchArtifactReport, _ *scenario.TYTBenchMeasurementReport, audio *scenario.AudioIntelligibilityReport) {
				audio.Rows[0].Outcome = scenario.AudioIntelligibilityOutcomeDegraded
				audio.GoodVoicePassRows = 0
			},
			wantText: "audio degraded row above threshold",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			channel := channelReportForReadiness(t, true)
			turnaround := turnaroundReportForReadiness(t, true)
			audio := audioReportForReadiness(t)
			tc.mutate(&channel, &turnaround, &audio)

			channelPath := writeJSONReport(t, filepath.Join(dir, "channel.json"), channel)
			turnaroundPath := writeJSONReport(t, filepath.Join(dir, "turnaround.json"), turnaround)
			audioPath := writeJSONReport(t, filepath.Join(dir, "audio.json"), audio)
			reportPath := filepath.Join(dir, "hardware-readiness.json")

			err := run([]string{
				"hardware-readiness",
				"--channel", channelPath,
				"--turnaround", turnaroundPath,
				"--audio", audioPath,
				"--report", reportPath,
			})
			if err == nil || !strings.Contains(err.Error(), tc.wantText) {
				t.Fatalf("forged report error = %v, want %q", err, tc.wantText)
			}
			assertFileMissing(t, reportPath)
		})
	}
}

type hardwareReadinessInputPaths struct {
	channel    string
	turnaround string
	audio      string
}

func writeHardwareReadinessInputReports(t *testing.T, complete bool) hardwareReadinessInputPaths {
	t.Helper()
	dir := t.TempDir()
	channelReport := channelReportForReadiness(t, complete)
	turnaroundReport := turnaroundReportForReadiness(t, complete)
	audioReport := audioReportForReadiness(t)
	return hardwareReadinessInputPaths{
		channel:    writeJSONReport(t, filepath.Join(dir, "channel.json"), channelReport),
		turnaround: writeJSONReport(t, filepath.Join(dir, "turnaround.json"), turnaroundReport),
		audio:      writeJSONReport(t, filepath.Join(dir, "audio.json"), audioReport),
	}
}

func channelReportForReadiness(t *testing.T, complete bool) scenario.ChannelBenchArtifactReport {
	t.Helper()
	cfg := scenario.DefaultChannelBenchArtifactConfig()
	if complete {
		cfg.MeasurementSource = scenario.ChannelBenchMeasurementSourceBenchMeasured
		cfg.TestFixture = false
		cfg.MeasurementArtifactRef = "bench/channel/tyt-19200-25khz.json"
		cfg.MeasurementArtifactSHA256 = testChannelBenchArtifactSHA
	}
	report, err := scenario.RunChannelBenchArtifactValidation(cfg)
	if err != nil {
		t.Fatalf("RunChannelBenchArtifactValidation failed: %v", err)
	}
	return report
}

func turnaroundReportForReadiness(t *testing.T, complete bool) scenario.TYTBenchMeasurementReport {
	t.Helper()
	cfg := scenario.DefaultTYTBenchMeasurementConfig()
	if complete {
		cfg.MeasurementSource = scenario.TYTBenchMeasurementSourceBenchMeasured
		cfg.TestFixture = false
		cfg.MeasurementArtifactRef = "bench/tyt-turnaround.json"
		cfg.RxToTxSamplesMs = nil
		cfg.TxToRxSamplesMs = nil
		cfg.RxValidAfterTxSamplesMs = nil
		rx := int64(6)
		tx := int64(5)
		valid := int64(2)
		cfg.RxToTxP50Ms = &rx
		cfg.RxToTxP95Ms = &rx
		cfg.TxToRxP50Ms = &tx
		cfg.TxToRxP95Ms = &tx
		cfg.RxValidAfterTxP50Ms = &valid
		cfg.RxValidAfterTxP95Ms = &valid
	}
	report, err := scenario.RunTYTBenchMeasurementCalibration(cfg)
	if err != nil {
		t.Fatalf("RunTYTBenchMeasurementCalibration failed: %v", err)
	}
	return report
}

func audioReportForReadiness(t *testing.T) scenario.AudioIntelligibilityReport {
	t.Helper()
	cfg := scenario.DefaultAudioIntelligibilityConfig()
	cfg.NoiseProfiles = []scenario.AudioIntelligibilityNoiseProfile{{NoiseProfileID: "clean", NoiseSNRDb: 40}}
	cfg.LossProfiles = []scenario.AudioIntelligibilityLossProfile{{LossProfileID: "no_loss", PacketLossPct: 0, BurstLossMs: 0}}
	report, err := scenario.RunAudioIntelligibilitySimulation(cfg)
	if err != nil {
		t.Fatalf("RunAudioIntelligibilitySimulation failed: %v", err)
	}
	return report
}

func writeJSONReport(t *testing.T, path string, value interface{}) string {
	t.Helper()
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal report failed: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write report failed: %v", err)
	}
	return path
}

func readPortableHardwareReadinessReport(t *testing.T, path string) scenario.PortableHardwareReadinessReport {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read readiness report failed: %v", err)
	}
	var report scenario.PortableHardwareReadinessReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("decode readiness report failed: %v\n%s", err, raw)
	}
	return report
}
