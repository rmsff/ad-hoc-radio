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

func runCalibrateTYTCommand(args []string) error {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		printCalibrateTYTUsage(os.Stdout)
		return nil
	}

	inputPath, reportPath, err := parseCalibrateTYTArgs(args)
	if err != nil {
		return err
	}
	cfg, err := loadTYTBenchMeasurementConfig(inputPath)
	if err != nil {
		return err
	}
	report, err := scenario.RunTYTBenchMeasurementCalibration(cfg)
	if err != nil {
		return fmt.Errorf("calibrate-tyt: %w", err)
	}
	raw, err := report.MarshalReportJSON()
	if err != nil {
		return fmt.Errorf("marshal TYT calibration report: %w", err)
	}
	return writeReportFile(reportPath, raw)
}

func parseCalibrateTYTArgs(args []string) (string, string, error) {
	var inputPath string
	var reportPath string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--report":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("calibrate-tyt: --report requires a path")
			}
			if strings.HasPrefix(args[i+1], "-") {
				return "", "", fmt.Errorf("calibrate-tyt: --report requires a path, got flag-like value %q", args[i+1])
			}
			if reportPath != "" {
				return "", "", fmt.Errorf("calibrate-tyt: --report specified more than once")
			}
			reportPath = args[i+1]
			i++
		case strings.HasPrefix(arg, "-"):
			return "", "", fmt.Errorf("calibrate-tyt: unknown flag %q", arg)
		default:
			if inputPath != "" {
				return "", "", fmt.Errorf("calibrate-tyt: unexpected argument %q", arg)
			}
			inputPath = arg
		}
	}
	if inputPath == "" {
		return "", "", fmt.Errorf("calibrate-tyt: measurement path is required")
	}
	if reportPath == "" {
		return "", "", fmt.Errorf("calibrate-tyt: --report path is required")
	}
	return inputPath, reportPath, nil
}

func printCalibrateTYTUsage(w *os.File) {
	fmt.Fprintln(w, "usage: adhr-sim calibrate-tyt <measurement.json> --report <path>")
}

func loadTYTBenchMeasurementConfig(path string) (scenario.TYTBenchMeasurementConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return scenario.TYTBenchMeasurementConfig{}, fmt.Errorf("read TYT bench measurement %q: %w", path, err)
	}
	var cfg scenario.TYTBenchMeasurementConfig
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return scenario.TYTBenchMeasurementConfig{}, fmt.Errorf("decode TYT bench measurement %q: %w", path, err)
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return scenario.TYTBenchMeasurementConfig{}, fmt.Errorf("decode TYT bench measurement %q: trailing JSON token", path)
		}
		return scenario.TYTBenchMeasurementConfig{}, fmt.Errorf("decode TYT bench measurement %q: %w", path, err)
	}
	fields, err := decodeTYTBenchMeasurementFields(raw)
	if err != nil {
		return scenario.TYTBenchMeasurementConfig{}, fmt.Errorf("decode TYT bench measurement %q: %w", path, err)
	}
	if err := validateTYTBenchMeasurementJSONPresence(fields); err != nil {
		return scenario.TYTBenchMeasurementConfig{}, fmt.Errorf("TYT bench measurement %q: %w", path, err)
	}
	return cfg, nil
}

func decodeTYTBenchMeasurementFields(raw []byte) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func validateTYTBenchMeasurementJSONPresence(fields map[string]json.RawMessage) error {
	if err := requireTYTBenchScenarioID(fields); err != nil {
		return err
	}
	if err := requireTYTBenchField(fields, "seed"); err != nil {
		return err
	}

	metrics := []struct {
		samples string
		p50     string
		p95     string
	}{
		{samples: "rx_to_tx_samples_ms", p50: "rx_to_tx_p50_ms", p95: "rx_to_tx_p95_ms"},
		{samples: "tx_to_rx_samples_ms", p50: "tx_to_rx_p50_ms", p95: "tx_to_rx_p95_ms"},
		{samples: "rx_valid_after_tx_samples_ms", p50: "rx_valid_after_tx_p50_ms", p95: "rx_valid_after_tx_p95_ms"},
	}
	for _, metric := range metrics {
		if err := validateTYTBenchSampleField(fields, metric.samples, metric.p50, metric.p95); err != nil {
			return err
		}
	}
	return nil
}

func requireTYTBenchScenarioID(fields map[string]json.RawMessage) error {
	raw, err := requireTYTBenchFieldRaw(fields, "scenario_id")
	if err != nil {
		return err
	}
	var scenarioID string
	if err := json.Unmarshal(raw, &scenarioID); err != nil {
		return fmt.Errorf("scenario_id must be a string")
	}
	if scenarioID != scenario.TYTBenchMeasurementScenarioID {
		return fmt.Errorf("unsupported scenario_id %q", scenarioID)
	}
	return nil
}

func requireTYTBenchField(fields map[string]json.RawMessage, name string) error {
	_, err := requireTYTBenchFieldRaw(fields, name)
	return err
}

func requireTYTBenchFieldRaw(fields map[string]json.RawMessage, name string) (json.RawMessage, error) {
	raw, ok := fields[name]
	if !ok {
		return nil, fmt.Errorf("%s is required", name)
	}
	if isJSONNull(raw) {
		return nil, fmt.Errorf("%s must not be null", name)
	}
	return raw, nil
}

func validateTYTBenchSampleField(fields map[string]json.RawMessage, samplesField string, p50Field string, p95Field string) error {
	raw, hasSamples := fields[samplesField]
	if !hasSamples {
		return nil
	}
	if _, hasP50 := fields[p50Field]; hasP50 {
		return fmt.Errorf("%s cannot be combined with %s", samplesField, p50Field)
	}
	if _, hasP95 := fields[p95Field]; hasP95 {
		return fmt.Errorf("%s cannot be combined with %s", samplesField, p95Field)
	}
	if isJSONNull(raw) {
		return fmt.Errorf("%s must not be null", samplesField)
	}
	var samples []int64
	if err := json.Unmarshal(raw, &samples); err != nil {
		return fmt.Errorf("%s must be an integer array", samplesField)
	}
	if len(samples) == 0 {
		return fmt.Errorf("%s must be non-empty when present", samplesField)
	}
	return nil
}

func isJSONNull(raw json.RawMessage) bool {
	return string(bytes.TrimSpace(raw)) == "null"
}
