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

func runPTTContentionCommand(args []string) error {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		printPTTContentionUsage(os.Stdout)
		return nil
	}
	inputPath, reportPath, err := parsePTTContentionArgs(args)
	if err != nil {
		return err
	}
	cfg, err := loadPTTContentionConfig(inputPath)
	if err != nil {
		return err
	}
	report, err := scenario.RunGroupFloorPTTContentionScenario(cfg)
	if err != nil {
		return fmt.Errorf("ptt-contention: %w", err)
	}
	raw, err := report.MarshalReportJSON()
	if err != nil {
		return fmt.Errorf("marshal ptt contention report: %w", err)
	}
	return writeReportFile(reportPath, raw)
}

func parsePTTContentionArgs(args []string) (string, string, error) {
	var inputPath string
	var reportPath string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--report":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("ptt-contention: --report requires a path")
			}
			if strings.HasPrefix(args[i+1], "-") {
				return "", "", fmt.Errorf("ptt-contention: --report requires a path, got flag-like value %q", args[i+1])
			}
			if reportPath != "" {
				return "", "", fmt.Errorf("ptt-contention: --report specified more than once")
			}
			reportPath = args[i+1]
			i++
		case strings.HasPrefix(arg, "-"):
			return "", "", fmt.Errorf("ptt-contention: unknown flag %q", arg)
		default:
			if inputPath != "" {
				return "", "", fmt.Errorf("ptt-contention: unexpected argument %q", arg)
			}
			inputPath = arg
		}
	}
	if inputPath == "" {
		return "", "", fmt.Errorf("ptt-contention: scenario path is required")
	}
	if reportPath == "" {
		return "", "", fmt.Errorf("ptt-contention: --report path is required")
	}
	return inputPath, reportPath, nil
}

func printPTTContentionUsage(w *os.File) {
	fmt.Fprintln(w, "usage: adhr-sim ptt-contention <scenario.json> --report <path>")
}

func loadPTTContentionConfig(path string) (scenario.GroupFloorPTTContentionConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return scenario.GroupFloorPTTContentionConfig{}, fmt.Errorf("read ptt contention scenario %q: %w", path, err)
	}
	var cfg scenario.GroupFloorPTTContentionConfig
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return scenario.GroupFloorPTTContentionConfig{}, fmt.Errorf("decode ptt contention scenario %q: %w", path, err)
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return scenario.GroupFloorPTTContentionConfig{}, fmt.Errorf("decode ptt contention scenario %q: trailing JSON token", path)
		}
		return scenario.GroupFloorPTTContentionConfig{}, fmt.Errorf("decode ptt contention scenario %q: %w", path, err)
	}
	fields, err := decodePTTContentionFields(raw)
	if err != nil {
		return scenario.GroupFloorPTTContentionConfig{}, fmt.Errorf("decode ptt contention scenario %q: %w", path, err)
	}
	if err := validatePTTContentionJSONPresence(fields); err != nil {
		return scenario.GroupFloorPTTContentionConfig{}, fmt.Errorf("ptt contention scenario %q: %w", path, err)
	}
	return cfg, nil
}

func decodePTTContentionFields(raw []byte) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func validatePTTContentionJSONPresence(fields map[string]json.RawMessage) error {
	raw, err := requirePTTContentionFieldRaw(fields, "scenario_id")
	if err != nil {
		return err
	}
	var scenarioID string
	if err := json.Unmarshal(raw, &scenarioID); err != nil {
		return fmt.Errorf("scenario_id must be a string")
	}
	if scenarioID != scenario.GroupFloorPTTContentionScenarioID {
		return fmt.Errorf("unsupported scenario_id %q", scenarioID)
	}
	for _, name := range []string{
		"seed",
		"floor_policy",
		"access_decision_ms",
		"floor_release_guard_ms",
		"first_audio_window_ms",
		"attempts",
	} {
		if _, err := requirePTTContentionFieldRaw(fields, name); err != nil {
			return err
		}
	}
	if err := requirePTTContentionPositiveUint64(fields, "seed"); err != nil {
		return err
	}
	if err := requirePTTContentionNonEmptyString(fields, "floor_policy"); err != nil {
		return err
	}
	if err := requirePTTContentionPositiveInt64(fields, "access_decision_ms"); err != nil {
		return err
	}
	if err := requirePTTContentionNonNegativeInt64(fields, "floor_release_guard_ms"); err != nil {
		return err
	}
	if err := requirePTTContentionPositiveInt64(fields, "first_audio_window_ms"); err != nil {
		return err
	}
	if err := validatePTTContentionAttemptsPresence(fields["attempts"]); err != nil {
		return err
	}
	return nil
}

func requirePTTContentionFieldRaw(fields map[string]json.RawMessage, name string) (json.RawMessage, error) {
	raw, ok := fields[name]
	if !ok {
		return nil, fmt.Errorf("%s is required", name)
	}
	if isJSONNull(raw) {
		return nil, fmt.Errorf("%s must not be null", name)
	}
	return raw, nil
}

func requirePTTContentionPositiveUint64(fields map[string]json.RawMessage, name string) error {
	raw, err := requirePTTContentionFieldRaw(fields, name)
	if err != nil {
		return err
	}
	var value uint64
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("%s must be an unsigned integer", name)
	}
	if value == 0 {
		return fmt.Errorf("%s must be > 0", name)
	}
	return nil
}

func requirePTTContentionPositiveInt64(fields map[string]json.RawMessage, name string) error {
	value, err := pttContentionInt64Field(fields, name)
	if err != nil {
		return err
	}
	if value <= 0 {
		return fmt.Errorf("%s must be > 0", name)
	}
	return nil
}

func requirePTTContentionNonNegativeInt64(fields map[string]json.RawMessage, name string) error {
	value, err := pttContentionInt64Field(fields, name)
	if err != nil {
		return err
	}
	if value < 0 {
		return fmt.Errorf("%s must be >= 0", name)
	}
	return nil
}

func pttContentionInt64Field(fields map[string]json.RawMessage, name string) (int64, error) {
	raw, err := requirePTTContentionFieldRaw(fields, name)
	if err != nil {
		return 0, err
	}
	var value int64
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	return value, nil
}

func requirePTTContentionNonEmptyString(fields map[string]json.RawMessage, name string) error {
	raw, err := requirePTTContentionFieldRaw(fields, name)
	if err != nil {
		return err
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return fmt.Errorf("%s must be a string", name)
	}
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s must not be empty", name)
	}
	return nil
}

func validatePTTContentionAttemptsPresence(raw json.RawMessage) error {
	var attempts []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &attempts); err != nil {
		return fmt.Errorf("attempts must be an array")
	}
	if len(attempts) == 0 {
		return fmt.Errorf("attempts must be non-empty")
	}
	for i, attempt := range attempts {
		for _, name := range []string{"event_id", "source_id", "ptt_start_ms", "talk_duration_ms"} {
			if _, err := requirePTTContentionFieldRaw(attempt, name); err != nil {
				return fmt.Errorf("attempts[%d].%w", i, err)
			}
		}
	}
	return nil
}
