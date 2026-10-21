package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func validateRegressionPackConfig(cfg regressionPackConfig) error {
	if strings.TrimSpace(cfg.PackID) == "" {
		return fmt.Errorf("pack_id is required")
	}
	if cfg.Seed == 0 {
		return fmt.Errorf("seed must be > 0")
	}
	if len(cfg.Cases) == 0 {
		return fmt.Errorf("cases must not be empty")
	}
	if len(cfg.Cases) > 64 {
		return fmt.Errorf("cases length %d exceeds max 64", len(cfg.Cases))
	}
	seen := make(map[string]struct{}, len(cfg.Cases))
	outputs := make(map[string]struct{}, len(cfg.Cases))
	for i, c := range cfg.Cases {
		if err := validateRegressionPackCase(i, c); err != nil {
			return err
		}
		if _, ok := seen[c.CaseID]; ok {
			return fmt.Errorf("cases[%d].case_id %q is duplicated", i, c.CaseID)
		}
		seen[c.CaseID] = struct{}{}
		output := filepath.ToSlash(filepath.Clean(c.Output))
		if _, ok := outputs[output]; ok {
			return fmt.Errorf("cases[%d].output %q is duplicated", i, c.Output)
		}
		outputs[output] = struct{}{}
	}
	return nil
}

func validateRegressionPackCase(index int, c regressionPackCaseIn) error {
	if strings.TrimSpace(c.CaseID) == "" {
		return fmt.Errorf("cases[%d].case_id is required", index)
	}
	if strings.TrimSpace(c.Command) == "" {
		return fmt.Errorf("cases[%d].command is required", index)
	}
	if !regressionPackCommandSupported(c.Command) {
		return fmt.Errorf("cases[%d].command %q is not supported", index, c.Command)
	}
	if c.Command == "hardware-readiness" {
		if err := validateRegressionPackHardwareCase(index, c); err != nil {
			return err
		}
	} else {
		if err := validateRegressionPackSingleInputCase(index, c); err != nil {
			return err
		}
	}
	if strings.TrimSpace(c.Output) == "" {
		return fmt.Errorf("cases[%d].output is required", index)
	}
	if filepath.IsAbs(c.Output) {
		return fmt.Errorf("cases[%d].output must be relative", index)
	}
	if isDirectoryShapedPath(c.Output) {
		return fmt.Errorf("cases[%d].output must name a file", index)
	}
	if !regressionPackRelativePathSafe(c.Output) {
		return fmt.Errorf("cases[%d].output must stay inside output directory", index)
	}
	if strings.TrimSpace(c.ExpectedOutcome) == "" {
		return fmt.Errorf("cases[%d].expected_outcome is required", index)
	}
	return nil
}

func validateRegressionPackSingleInputCase(index int, c regressionPackCaseIn) error {
	if strings.TrimSpace(c.Input) == "" {
		return fmt.Errorf("cases[%d].input is required", index)
	}
	if c.Inputs != nil {
		return fmt.Errorf("cases[%d].inputs is only supported for hardware-readiness", index)
	}
	if c.TxRequested || c.LabSafetyDocumented || c.RegulatoryProfileDocumented {
		return fmt.Errorf("cases[%d].hardware readiness flags are only supported for hardware-readiness", index)
	}
	if filepath.IsAbs(c.Input) {
		return fmt.Errorf("cases[%d].input must be relative", index)
	}
	return nil
}

func validateRegressionPackHardwareCase(index int, c regressionPackCaseIn) error {
	if strings.TrimSpace(c.Input) != "" {
		return fmt.Errorf("cases[%d].input must be empty for hardware-readiness", index)
	}
	if c.Inputs == nil {
		return fmt.Errorf("cases[%d].inputs is required for hardware-readiness", index)
	}
	if err := validateRegressionPackHardwareInput(index, "channel", c.Inputs.Channel); err != nil {
		return err
	}
	if err := validateRegressionPackHardwareInput(index, "turnaround", c.Inputs.Turnaround); err != nil {
		return err
	}
	return validateRegressionPackHardwareInput(index, "audio", c.Inputs.Audio)
}

func validateRegressionPackHardwareInput(index int, field string, path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("cases[%d].inputs.%s is required", index, field)
	}
	if filepath.IsAbs(path) {
		return fmt.Errorf("cases[%d].inputs.%s must be relative", index, field)
	}
	if isDirectoryShapedPath(path) {
		return fmt.Errorf("cases[%d].inputs.%s must name a file", index, field)
	}
	if !regressionPackRelativePathSafe(path) {
		return fmt.Errorf("cases[%d].inputs.%s must stay inside output directory", index, field)
	}
	return nil
}

func regressionPackCommandSupported(command string) bool {
	switch command {
	case "run", "ptt-contention", "calibrate-channel", "calibrate-tyt", "score-audio", "hardware-readiness":
		return true
	default:
		return false
	}
}

func regressionPackRelativePathSafe(path string) bool {
	clean := filepath.Clean(path)
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, ".."+string(os.PathSeparator))
}
