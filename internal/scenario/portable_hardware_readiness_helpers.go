package scenario

import (
	"fmt"
	"strings"
)

func normalizePortableHardwareReadinessConfig(cfg PortableHardwareReadinessConfig) PortableHardwareReadinessConfig {
	def := DefaultPortableHardwareReadinessConfig()
	if cfg.ScenarioID == "" {
		cfg.ScenarioID = def.ScenarioID
	}
	if cfg.Seed == 0 {
		cfg.Seed = def.Seed
	}
	if cfg.CandidateID == "" {
		cfg.CandidateID = def.CandidateID
	}
	if cfg.RadioFamily == "" {
		cfg.RadioFamily = def.RadioFamily
	}
	if cfg.FirmwarePath == "" {
		cfg.FirmwarePath = def.FirmwarePath
	}
	if cfg.ChannelWidthHz == 0 {
		cfg.ChannelWidthHz = def.ChannelWidthHz
	}
	if cfg.GrossBitrateBps == 0 {
		cfg.GrossBitrateBps = def.GrossBitrateBps
	}
	if cfg.TurnaroundMeasurementSource == "" {
		cfg.TurnaroundMeasurementSource = def.TurnaroundMeasurementSource
	}
	if cfg.GoodVoiceThreshold == 0 {
		cfg.GoodVoiceThreshold = def.GoodVoiceThreshold
	}
	cfg.CandidateID = strings.TrimSpace(cfg.CandidateID)
	cfg.RadioFamily = strings.TrimSpace(cfg.RadioFamily)
	cfg.FirmwarePath = strings.TrimSpace(cfg.FirmwarePath)
	cfg.ChannelArtifactRef = strings.TrimSpace(cfg.ChannelArtifactRef)
	cfg.TurnaroundArtifactRef = strings.TrimSpace(cfg.TurnaroundArtifactRef)
	cfg.AudioFixtureArtifactRef = strings.TrimSpace(cfg.AudioFixtureArtifactRef)
	cfg.AudioFixtureSHA256 = strings.TrimSpace(cfg.AudioFixtureSHA256)
	if !cfg.TransmitRequiresSeparateGoNoGo {
		cfg.TransmitRequiresSeparateGoNoGo = def.TransmitRequiresSeparateGoNoGo
	}
	return cfg
}

func validatePortableHardwareReadinessConfig(cfg PortableHardwareReadinessConfig) error {
	if cfg.ScenarioID != PortableHardwareReadinessScenarioID {
		return fmt.Errorf("portable hardware readiness: unsupported scenario_id %q", cfg.ScenarioID)
	}
	if cfg.CandidateID == "" || cfg.RadioFamily == "" {
		return fmt.Errorf("portable hardware readiness: candidate_id and radio_family are required")
	}
	if cfg.ChannelWidthHz != 12500 && cfg.ChannelWidthHz != 25000 {
		return fmt.Errorf("portable hardware readiness: channel_width_hz=%d unsupported", cfg.ChannelWidthHz)
	}
	if cfg.GrossBitrateBps != 9600 && cfg.GrossBitrateBps != 19200 {
		return fmt.Errorf("portable hardware readiness: gross_bitrate_bps=%d unsupported", cfg.GrossBitrateBps)
	}
	switch cfg.TurnaroundMeasurementSource {
	case TYTBenchMeasurementSourceFixtureOnly, TYTBenchMeasurementSourceBenchMeasured, TYTBenchMeasurementSourceUnknown:
	default:
		return fmt.Errorf("portable hardware readiness: unsupported turnaround_measurement_source %q", cfg.TurnaroundMeasurementSource)
	}
	if cfg.TurnaroundMeasurementSource == TYTBenchMeasurementSourceBenchMeasured && cfg.TurnaroundArtifactRef == "" {
		return fmt.Errorf("portable hardware readiness: bench_measured turnaround requires turnaround_artifact_ref")
	}
	if cfg.ChannelBenchVerified && cfg.ChannelArtifactRef == "" {
		return fmt.Errorf("portable hardware readiness: verified channel requires channel_artifact_ref")
	}
	if cfg.AudioFixtureArtifactRef != "" && cfg.AudioFixtureSHA256 == "" {
		return fmt.Errorf("portable hardware readiness: audio fixture artifact requires sha256")
	}
	if cfg.AudioFixtureSHA256 != "" && !isLowerHexSHA256(cfg.AudioFixtureSHA256) {
		return fmt.Errorf("portable hardware readiness: audio_fixture_sha256 must be lowercase sha256")
	}
	if cfg.MaxTurnaroundP95Ms < 0 {
		return fmt.Errorf("portable hardware readiness: max_turnaround_p95_ms must be >= 0")
	}
	if cfg.IntelligibilityProxyScore < 0 || cfg.IntelligibilityProxyScore > 100 ||
		cfg.GoodVoiceThreshold <= 0 || cfg.GoodVoiceThreshold > 100 {
		return fmt.Errorf("portable hardware readiness: intelligibility score and threshold must be in range")
	}
	return nil
}

func portableHardwareReadinessGates(cfg PortableHardwareReadinessConfig, report PortableHardwareReadinessReport) []PortableHardwareReadinessGateRow {
	rows := make([]PortableHardwareReadinessGateRow, 0, 6)
	rows = append(rows, portableHardwareGateCandidateRow(cfg))
	rows = append(rows, portableHardwareGateFirmwareRow(cfg))
	rows = append(rows, portableHardwareGateChannelRow(cfg))
	rows = append(rows, portableHardwareGateTurnaroundRow(cfg, report))
	rows = append(rows, portableHardwareGateVoiceRow(cfg))
	rows = append(rows, portableHardwareGateTxLabRow(cfg))
	return rows
}

func portableHardwareGateCandidateRow(cfg PortableHardwareReadinessConfig) PortableHardwareReadinessGateRow {
	return PortableHardwareReadinessGateRow{
		GateID:      portableHardwareGateCandidate,
		Category:    "hardware_candidate",
		RequiredFor: "bench",
		Status:      PortableHardwareGateStatusPass,
		Observed:    cfg.CandidateID,
		Required:    "selected portable candidate",
		EvidenceRef: "docs/research/011-tyt-md-uv380-channel-feasibility.md",
		Reason:      "candidate selected for bench exploration",
	}
}

func portableHardwareGateFirmwareRow(cfg PortableHardwareReadinessConfig) PortableHardwareReadinessGateRow {
	row := PortableHardwareReadinessGateRow{
		GateID:      portableHardwareGateFirmware,
		Category:    "firmware",
		RequiredFor: "bench",
		Observed:    cfg.FirmwarePath,
		Required:    "inspectable firmware path for bench build",
		EvidenceRef: cfg.FirmwarePath,
	}
	if cfg.FirmwarePath == "" || !cfg.FirmwareInspectable {
		row.Status = PortableHardwareGateStatusMissing
		row.Blocking = true
		row.Reason = "firmware path is not inspectable"
		return row
	}
	row.Status = PortableHardwareGateStatusPass
	row.Reason = "firmware path is available for bench planning"
	return row
}

func portableHardwareGateChannelRow(cfg PortableHardwareReadinessConfig) PortableHardwareReadinessGateRow {
	row := PortableHardwareReadinessGateRow{
		GateID:      portableHardwareGateChannel,
		Category:    "channel",
		RequiredFor: "bench",
		Observed:    fmt.Sprintf("%d Hz / %d bps", cfg.ChannelWidthHz, cfg.GrossBitrateBps),
		Required:    "bench-verified channel bitrate and bandwidth artifact",
		EvidenceRef: cfg.ChannelArtifactRef,
	}
	if !cfg.ChannelBenchVerified {
		row.Status = PortableHardwareGateStatusMissing
		row.Blocking = true
		row.Reason = "channel bitrate is simulator/research evidence only"
		return row
	}
	row.Status = PortableHardwareGateStatusPass
	row.Reason = "channel bitrate has a bench verification artifact"
	return row
}

func portableHardwareGateTurnaroundRow(cfg PortableHardwareReadinessConfig, report PortableHardwareReadinessReport) PortableHardwareReadinessGateRow {
	row := PortableHardwareReadinessGateRow{
		GateID:      portableHardwareGateTurnaround,
		Category:    "turnaround",
		RequiredFor: "bench",
		Observed:    fmt.Sprintf("%s / p95=%d ms / decision=%s", cfg.TurnaroundMeasurementSource, cfg.MaxTurnaroundP95Ms, report.CalibrationDecision),
		Required:    "bench-measured TYT turnaround artifact with viable tuple",
		EvidenceRef: cfg.TurnaroundArtifactRef,
	}
	if cfg.TurnaroundMeasurementSource != TYTBenchMeasurementSourceBenchMeasured {
		row.Status = PortableHardwareGateStatusMissing
		row.Blocking = true
		row.Reason = "turnaround is not measured on the target hardware"
		return row
	}
	if report.CalibrationDecision == TYTBenchDecisionNoLiveTwoHop {
		row.Status = PortableHardwareGateStatusInsufficient
		row.Blocking = true
		row.Reason = "measured turnaround does not support current live two-hop tuple"
		return row
	}
	row.Status = PortableHardwareGateStatusPass
	row.Reason = "bench turnaround supports at least one current tuple"
	return row
}

func portableHardwareGateVoiceRow(cfg PortableHardwareReadinessConfig) PortableHardwareReadinessGateRow {
	row := PortableHardwareReadinessGateRow{
		GateID:      portableHardwareGateVoice,
		Category:    "voice_quality",
		RequiredFor: "bench",
		Observed:    fmt.Sprintf("score=%d threshold=%d", cfg.IntelligibilityProxyScore, cfg.GoodVoiceThreshold),
		Required:    "hashed audio fixture artifact at or above good-voice threshold",
		EvidenceRef: cfg.AudioFixtureArtifactRef,
	}
	if cfg.AudioFixtureArtifactRef == "" || cfg.AudioFixtureSHA256 == "" {
		row.Status = PortableHardwareGateStatusMissing
		row.Blocking = true
		row.Reason = "audio intelligibility artifact is missing"
		return row
	}
	if !cfg.AudioGoodVoicePass {
		row.Status = PortableHardwareGateStatusInsufficient
		row.Blocking = true
		row.Reason = "audio report does not contain a good_voice_pass row"
		return row
	}
	if cfg.IntelligibilityProxyScore < cfg.GoodVoiceThreshold {
		row.Status = PortableHardwareGateStatusInsufficient
		row.Blocking = true
		row.Reason = "audio intelligibility score is below threshold"
		return row
	}
	row.Status = PortableHardwareGateStatusPass
	row.Reason = "audio fixture meets good-voice threshold"
	return row
}

func portableHardwareGateTxLabRow(cfg PortableHardwareReadinessConfig) PortableHardwareReadinessGateRow {
	row := PortableHardwareReadinessGateRow{
		GateID:      portableHardwareGateTxLab,
		Category:    "rf_safety",
		RequiredFor: "tx_lab",
		Observed:    fmt.Sprintf("regulatory=%v lab_safety=%v", cfg.RegulatoryProfileDocumented, cfg.LabSafetyPlanDocumented),
		Required:    "regulatory profile and lab safety plan before transmit-capable work",
		EvidenceRef: "docs/research/003-regulatory-feasibility.md",
	}
	if !cfg.TransmitCapableWorkRequested {
		row.Status = PortableHardwareGateStatusDeferred
		row.Reason = "transmit-capable work is not requested in this story"
		return row
	}
	if !cfg.RegulatoryProfileDocumented || !cfg.LabSafetyPlanDocumented {
		row.Status = PortableHardwareGateStatusMissing
		row.Blocking = true
		row.Reason = "transmit-capable lab preconditions are incomplete"
		return row
	}
	row.Status = PortableHardwareGateStatusPass
	row.Reason = "transmit-capable preconditions are documented but still require separate go/no-go"
	return row
}

func portableHardwareAccumulateGate(report *PortableHardwareReadinessReport, row PortableHardwareReadinessGateRow) {
	switch row.Status {
	case PortableHardwareGateStatusPass:
		report.PassedGateCount++
	case PortableHardwareGateStatusMissing:
		report.MissingEvidenceCount++
	case PortableHardwareGateStatusInsufficient:
		report.InsufficientEvidenceCount++
	case PortableHardwareGateStatusDeferred:
		report.DeferredGateCount++
	}
	if row.Blocking {
		report.ReadinessBlockerCount++
	}
}

func portableHardwareOnlyTxLabBlocks(rows []PortableHardwareReadinessGateRow) bool {
	for _, row := range rows {
		if row.Blocking && row.GateID != portableHardwareGateTxLab {
			return false
		}
	}
	return true
}

func portableHardwareReadinessOutcome(report PortableHardwareReadinessReport) (string, []string) {
	reasons := make([]string, 0, 5)
	for _, gate := range report.Gates {
		if !gate.Blocking {
			continue
		}
		reasons = append(reasons, gate.GateID)
	}
	if report.TransmitCapableWorkRequested && !report.TransmitPreconditionsDocumented {
		return PortableHardwareOutcomeFieldBlocked, reasons
	}
	if report.BenchExperimentReady {
		return PortableHardwareOutcomeBenchReady, reasons
	}
	return PortableHardwareOutcomeBlocked, reasons
}

func clonePortableHardwareStrings(values []string) []string {
	return append([]string(nil), values...)
}
