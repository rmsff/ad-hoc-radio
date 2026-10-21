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

func runCalibrateChannelCommand(args []string) error {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		printCalibrateChannelUsage(os.Stdout)
		return nil
	}
	inputPath, reportPath, err := parseCalibrateChannelArgs(args)
	if err != nil {
		return err
	}
	cfg, err := loadChannelBenchArtifactConfig(inputPath)
	if err != nil {
		return err
	}
	report, err := scenario.RunChannelBenchArtifactValidation(cfg)
	if err != nil {
		return fmt.Errorf("calibrate-channel: %w", err)
	}
	raw, err := report.MarshalReportJSON()
	if err != nil {
		return fmt.Errorf("marshal channel calibration report: %w", err)
	}
	return writeReportFile(reportPath, raw)
}

func parseCalibrateChannelArgs(args []string) (string, string, error) {
	var inputPath string
	var reportPath string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--report":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("calibrate-channel: --report requires a path")
			}
			if strings.HasPrefix(args[i+1], "-") {
				return "", "", fmt.Errorf("calibrate-channel: --report requires a path, got flag-like value %q", args[i+1])
			}
			if reportPath != "" {
				return "", "", fmt.Errorf("calibrate-channel: --report specified more than once")
			}
			reportPath = args[i+1]
			i++
		case strings.HasPrefix(arg, "-"):
			return "", "", fmt.Errorf("calibrate-channel: unknown flag %q", arg)
		default:
			if inputPath != "" {
				return "", "", fmt.Errorf("calibrate-channel: unexpected argument %q", arg)
			}
			inputPath = arg
		}
	}
	if inputPath == "" {
		return "", "", fmt.Errorf("calibrate-channel: channel artifact path is required")
	}
	if reportPath == "" {
		return "", "", fmt.Errorf("calibrate-channel: --report path is required")
	}
	return inputPath, reportPath, nil
}

func printCalibrateChannelUsage(w *os.File) {
	fmt.Fprintln(w, "usage: adhr-sim calibrate-channel <channel.json> --report <path>")
}

func loadChannelBenchArtifactConfig(path string) (scenario.ChannelBenchArtifactConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return scenario.ChannelBenchArtifactConfig{}, fmt.Errorf("read channel bench artifact %q: %w", path, err)
	}
	var cfg scenario.ChannelBenchArtifactConfig
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return scenario.ChannelBenchArtifactConfig{}, fmt.Errorf("decode channel bench artifact %q: %w", path, err)
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return scenario.ChannelBenchArtifactConfig{}, fmt.Errorf("decode channel bench artifact %q: trailing JSON token", path)
		}
		return scenario.ChannelBenchArtifactConfig{}, fmt.Errorf("decode channel bench artifact %q: %w", path, err)
	}
	fields, err := decodeChannelBenchArtifactFields(raw)
	if err != nil {
		return scenario.ChannelBenchArtifactConfig{}, fmt.Errorf("decode channel bench artifact %q: %w", path, err)
	}
	if err := validateChannelBenchArtifactJSONPresence(fields); err != nil {
		return scenario.ChannelBenchArtifactConfig{}, fmt.Errorf("channel bench artifact %q: %w", path, err)
	}
	return cfg, nil
}

func decodeChannelBenchArtifactFields(raw []byte) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func validateChannelBenchArtifactJSONPresence(fields map[string]json.RawMessage) error {
	raw, err := requireChannelBenchFieldRaw(fields, "scenario_id")
	if err != nil {
		return err
	}
	var scenarioID string
	if err := json.Unmarshal(raw, &scenarioID); err != nil {
		return fmt.Errorf("scenario_id must be a string")
	}
	if scenarioID != scenario.ChannelBenchArtifactScenarioID {
		return fmt.Errorf("unsupported scenario_id %q", scenarioID)
	}
	for _, name := range []string{
		"seed",
		"measurement_source",
		"radio_family",
		"test_fixture",
		"channel_width_hz",
		"gross_bitrate_bps",
		"modulation_profile",
		"frames_sent",
		"frames_decoded",
		"crc_error_count",
		"duration_ms",
	} {
		if _, err := requireChannelBenchFieldRaw(fields, name); err != nil {
			return err
		}
	}
	return nil
}

func requireChannelBenchFieldRaw(fields map[string]json.RawMessage, name string) (json.RawMessage, error) {
	raw, ok := fields[name]
	if !ok {
		return nil, fmt.Errorf("%s is required", name)
	}
	if isJSONNull(raw) {
		return nil, fmt.Errorf("%s must not be null", name)
	}
	return raw, nil
}
