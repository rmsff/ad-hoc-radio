package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

type hardwareReadinessArgs struct {
	channelPath       string
	turnaroundPath    string
	audioPath         string
	reportPath        string
	txRequested       bool
	labDocumented     bool
	regulatoryProfile bool
}

func runHardwareReadinessCommand(args []string) error {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		printHardwareReadinessUsage(os.Stdout)
		return nil
	}
	parsed, err := parseHardwareReadinessArgs(args)
	if err != nil {
		return err
	}
	channelReport, err := loadChannelBenchArtifactReport(parsed.channelPath)
	if err != nil {
		return err
	}
	turnaroundReport, err := loadTYTBenchMeasurementReport(parsed.turnaroundPath)
	if err != nil {
		return err
	}
	audioReport, err := loadAudioIntelligibilityReport(parsed.audioPath)
	if err != nil {
		return err
	}
	cfg, err := portableHardwareConfigFromReports(channelReport, turnaroundReport, audioReport, parsed)
	if err != nil {
		return err
	}
	report, err := scenario.RunPortableHardwareReadinessGate(cfg)
	if err != nil {
		return fmt.Errorf("hardware-readiness: %w", err)
	}
	raw, err := report.MarshalReportJSON()
	if err != nil {
		return fmt.Errorf("marshal hardware readiness report: %w", err)
	}
	return writeReportFile(parsed.reportPath, raw)
}

func parseHardwareReadinessArgs(args []string) (hardwareReadinessArgs, error) {
	var parsed hardwareReadinessArgs
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--channel":
			value, err := nextHardwareReadinessArg(args, &i, arg)
			if err != nil {
				return hardwareReadinessArgs{}, err
			}
			if parsed.channelPath != "" {
				return hardwareReadinessArgs{}, fmt.Errorf("hardware-readiness: --channel specified more than once")
			}
			parsed.channelPath = value
		case "--turnaround":
			value, err := nextHardwareReadinessArg(args, &i, arg)
			if err != nil {
				return hardwareReadinessArgs{}, err
			}
			if parsed.turnaroundPath != "" {
				return hardwareReadinessArgs{}, fmt.Errorf("hardware-readiness: --turnaround specified more than once")
			}
			parsed.turnaroundPath = value
		case "--audio":
			value, err := nextHardwareReadinessArg(args, &i, arg)
			if err != nil {
				return hardwareReadinessArgs{}, err
			}
			if parsed.audioPath != "" {
				return hardwareReadinessArgs{}, fmt.Errorf("hardware-readiness: --audio specified more than once")
			}
			parsed.audioPath = value
		case "--report":
			value, err := nextHardwareReadinessArg(args, &i, arg)
			if err != nil {
				return hardwareReadinessArgs{}, err
			}
			if parsed.reportPath != "" {
				return hardwareReadinessArgs{}, fmt.Errorf("hardware-readiness: --report specified more than once")
			}
			parsed.reportPath = value
		case "--tx-requested":
			parsed.txRequested = true
		case "--lab-safety-documented":
			parsed.labDocumented = true
		case "--regulatory-profile-documented":
			parsed.regulatoryProfile = true
		default:
			if strings.HasPrefix(arg, "-") {
				return hardwareReadinessArgs{}, fmt.Errorf("hardware-readiness: unknown flag %q", arg)
			}
			return hardwareReadinessArgs{}, fmt.Errorf("hardware-readiness: unexpected argument %q", arg)
		}
	}
	if parsed.channelPath == "" {
		return hardwareReadinessArgs{}, fmt.Errorf("hardware-readiness: --channel path is required")
	}
	if parsed.turnaroundPath == "" {
		return hardwareReadinessArgs{}, fmt.Errorf("hardware-readiness: --turnaround path is required")
	}
	if parsed.audioPath == "" {
		return hardwareReadinessArgs{}, fmt.Errorf("hardware-readiness: --audio path is required")
	}
	if parsed.reportPath == "" {
		return hardwareReadinessArgs{}, fmt.Errorf("hardware-readiness: --report path is required")
	}
	return parsed, nil
}

func nextHardwareReadinessArg(args []string, index *int, flag string) (string, error) {
	if *index+1 >= len(args) {
		return "", fmt.Errorf("hardware-readiness: %s requires a path", flag)
	}
	value := args[*index+1]
	if strings.HasPrefix(value, "-") {
		return "", fmt.Errorf("hardware-readiness: %s requires a path, got flag-like value %q", flag, value)
	}
	*index = *index + 1
	return value, nil
}

func printHardwareReadinessUsage(w *os.File) {
	fmt.Fprintln(w, "usage: adhr-sim hardware-readiness --channel <report.json> --turnaround <report.json> --audio <report.json> --report <path>")
}

func portableHardwareConfigFromReports(
	channelReport scenario.ChannelBenchArtifactReport,
	turnaroundReport scenario.TYTBenchMeasurementReport,
	audioReport scenario.AudioIntelligibilityReport,
	args hardwareReadinessArgs,
) (scenario.PortableHardwareReadinessConfig, error) {
	if err := validateChannelReadinessReport(channelReport); err != nil {
		return scenario.PortableHardwareReadinessConfig{}, err
	}
	if err := validateTurnaroundReadinessReport(turnaroundReport); err != nil {
		return scenario.PortableHardwareReadinessConfig{}, err
	}
	if err := validateAudioReadinessReport(audioReport); err != nil {
		return scenario.PortableHardwareReadinessConfig{}, err
	}
	audioRow, err := bestAudioReadinessRow(audioReport)
	if err != nil {
		return scenario.PortableHardwareReadinessConfig{}, err
	}
	cfg := scenario.DefaultPortableHardwareReadinessConfig()
	cfg.ChannelBenchVerified = channelReport.ChannelBenchVerified
	cfg.ChannelArtifactRef = channelReport.MeasurementArtifactRef
	cfg.ChannelWidthHz = channelReport.ChannelWidthHz
	cfg.GrossBitrateBps = channelReport.GrossBitrateBps
	cfg.TurnaroundMeasurementSource = turnaroundReport.MeasurementSource
	cfg.TurnaroundArtifactRef = turnaroundReport.MeasurementArtifactRef
	cfg.MaxTurnaroundP95Ms = turnaroundReport.MaxRequiredTurnaroundP95Ms
	cfg.AudioFixtureArtifactRef = audioRow.FixturePath
	cfg.AudioFixtureSHA256 = audioRow.ArtifactSHA256
	cfg.AudioGoodVoicePass = audioRow.Outcome == scenario.AudioIntelligibilityOutcomeGoodVoicePass
	cfg.IntelligibilityProxyScore = audioRow.IntelligibilityProxyScore
	cfg.GoodVoiceThreshold = audioRow.GoodVoiceThreshold
	cfg.TransmitCapableWorkRequested = args.txRequested
	cfg.LabSafetyPlanDocumented = args.labDocumented
	cfg.RegulatoryProfileDocumented = args.regulatoryProfile
	return cfg, nil
}

func bestAudioReadinessRow(report scenario.AudioIntelligibilityReport) (scenario.AudioIntelligibilityRow, error) {
	if report.ScenarioID != scenario.AudioIntelligibilityScenarioID {
		return scenario.AudioIntelligibilityRow{}, fmt.Errorf("hardware-readiness: unsupported audio scenario_id %q", report.ScenarioID)
	}
	if len(report.Rows) == 0 {
		return scenario.AudioIntelligibilityRow{}, fmt.Errorf("hardware-readiness: audio report has no rows")
	}
	best := report.Rows[0]
	for _, row := range report.Rows {
		if row.Outcome == scenario.AudioIntelligibilityOutcomeGoodVoicePass {
			return row, nil
		}
		if row.IntelligibilityProxyScore > best.IntelligibilityProxyScore {
			best = row
		}
	}
	return best, nil
}

func loadChannelBenchArtifactReport(path string) (scenario.ChannelBenchArtifactReport, error) {
	var report scenario.ChannelBenchArtifactReport
	if err := loadStrictJSONReport(path, "channel bench report", func(d *json.Decoder) error { return d.Decode(&report) }); err != nil {
		return scenario.ChannelBenchArtifactReport{}, err
	}
	if report.ScenarioID != scenario.ChannelBenchArtifactScenarioID {
		return scenario.ChannelBenchArtifactReport{}, fmt.Errorf("channel bench report %q: unsupported scenario_id %q", path, report.ScenarioID)
	}
	return report, nil
}

func loadTYTBenchMeasurementReport(path string) (scenario.TYTBenchMeasurementReport, error) {
	var report scenario.TYTBenchMeasurementReport
	if err := loadStrictJSONReport(path, "TYT turnaround report", func(d *json.Decoder) error { return d.Decode(&report) }); err != nil {
		return scenario.TYTBenchMeasurementReport{}, err
	}
	if report.ScenarioID != scenario.TYTBenchMeasurementScenarioID {
		return scenario.TYTBenchMeasurementReport{}, fmt.Errorf("TYT turnaround report %q: unsupported scenario_id %q", path, report.ScenarioID)
	}
	return report, nil
}

func loadAudioIntelligibilityReport(path string) (scenario.AudioIntelligibilityReport, error) {
	var report scenario.AudioIntelligibilityReport
	if err := loadStrictJSONReport(path, "audio report", func(d *json.Decoder) error { return d.Decode(&report) }); err != nil {
		return scenario.AudioIntelligibilityReport{}, err
	}
	if report.ScenarioID != scenario.AudioIntelligibilityScenarioID {
		return scenario.AudioIntelligibilityReport{}, fmt.Errorf("audio report %q: unsupported scenario_id %q", path, report.ScenarioID)
	}
	return report, nil
}

func loadStrictJSONReport(path string, label string, decodeInto func(*json.Decoder) error) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s %q: %w", label, path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decodeInto(decoder); err != nil {
		return fmt.Errorf("decode %s %q: %w", label, path, err)
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode %s %q: trailing JSON token", label, path)
		}
		return fmt.Errorf("decode %s %q: %w", label, path, err)
	}
	return nil
}
