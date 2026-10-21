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

func runScoreAudioCommand(args []string) error {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		printScoreAudioUsage(os.Stdout)
		return nil
	}
	inputPath, reportPath, err := parseScoreAudioArgs(args)
	if err != nil {
		return err
	}
	cfg, err := loadAudioIntelligibilityConfig(inputPath)
	if err != nil {
		return err
	}
	report, err := scenario.RunAudioIntelligibilitySimulation(cfg)
	if err != nil {
		return fmt.Errorf("score-audio: %w", err)
	}
	raw, err := report.MarshalReportJSON()
	if err != nil {
		return fmt.Errorf("marshal audio fixture report: %w", err)
	}
	return writeReportFile(reportPath, raw)
}

func parseScoreAudioArgs(args []string) (string, string, error) {
	var inputPath string
	var reportPath string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--report":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("score-audio: --report requires a path")
			}
			if strings.HasPrefix(args[i+1], "-") {
				return "", "", fmt.Errorf("score-audio: --report requires a path, got flag-like value %q", args[i+1])
			}
			if reportPath != "" {
				return "", "", fmt.Errorf("score-audio: --report specified more than once")
			}
			reportPath = args[i+1]
			i++
		case strings.HasPrefix(arg, "-"):
			return "", "", fmt.Errorf("score-audio: unknown flag %q", arg)
		default:
			if inputPath != "" {
				return "", "", fmt.Errorf("score-audio: unexpected argument %q", arg)
			}
			inputPath = arg
		}
	}
	if inputPath == "" {
		return "", "", fmt.Errorf("score-audio: audio fixture path is required")
	}
	if reportPath == "" {
		return "", "", fmt.Errorf("score-audio: --report path is required")
	}
	return inputPath, reportPath, nil
}

func printScoreAudioUsage(w *os.File) {
	fmt.Fprintln(w, "usage: adhr-sim score-audio <audio.json> --report <path>")
}

func loadAudioIntelligibilityConfig(path string) (scenario.AudioIntelligibilityConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return scenario.AudioIntelligibilityConfig{}, fmt.Errorf("read audio fixture %q: %w", path, err)
	}
	var cfg scenario.AudioIntelligibilityConfig
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return scenario.AudioIntelligibilityConfig{}, fmt.Errorf("decode audio fixture %q: %w", path, err)
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return scenario.AudioIntelligibilityConfig{}, fmt.Errorf("decode audio fixture %q: trailing JSON token", path)
		}
		return scenario.AudioIntelligibilityConfig{}, fmt.Errorf("decode audio fixture %q: %w", path, err)
	}
	fields, err := decodeAudioFixtureFields(raw)
	if err != nil {
		return scenario.AudioIntelligibilityConfig{}, fmt.Errorf("decode audio fixture %q: %w", path, err)
	}
	if err := validateAudioFixtureJSONPresence(fields); err != nil {
		return scenario.AudioIntelligibilityConfig{}, fmt.Errorf("audio fixture %q: %w", path, err)
	}
	return cfg, nil
}

func decodeAudioFixtureFields(raw []byte) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func validateAudioFixtureJSONPresence(fields map[string]json.RawMessage) error {
	raw, err := requireAudioFixtureFieldRaw(fields, "scenario_id")
	if err != nil {
		return err
	}
	var scenarioID string
	if err := json.Unmarshal(raw, &scenarioID); err != nil {
		return fmt.Errorf("scenario_id must be a string")
	}
	if scenarioID != scenario.AudioIntelligibilityScenarioID {
		return fmt.Errorf("unsupported scenario_id %q", scenarioID)
	}
	for _, name := range []string{
		"seed",
		"fixture",
		"noise_profiles",
		"loss_profiles",
		"codec_class",
		"audio_bandwidth_hz",
		"scoring_method",
		"good_voice_threshold",
	} {
		if _, err := requireAudioFixtureFieldRaw(fields, name); err != nil {
			return err
		}
	}
	return nil
}

func requireAudioFixtureFieldRaw(fields map[string]json.RawMessage, name string) (json.RawMessage, error) {
	raw, ok := fields[name]
	if !ok {
		return nil, fmt.Errorf("%s is required", name)
	}
	if isJSONNull(raw) {
		return nil, fmt.Errorf("%s must not be null", name)
	}
	return raw, nil
}
