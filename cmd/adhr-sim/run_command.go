package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

func runScenarioCommand(args []string) error {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		printRunUsage(os.Stdout)
		return nil
	}

	scenarioPath, metricsPath, err := parseRunArgs(args)
	if err != nil {
		return err
	}

	dto, cfg, err := loadScenarioConfig(scenarioPath)
	if err != nil {
		return err
	}

	metrics, err := executeScenario(cfg)
	if err != nil {
		return err
	}
	if metrics.ScenarioID != dto.ScenarioID {
		return fmt.Errorf("scenario_id mismatch: config=%q metrics=%q", dto.ScenarioID, metrics.ScenarioID)
	}

	raw, err := metrics.MarshalReportJSON()
	if err != nil {
		return fmt.Errorf("marshal metrics report: %w", err)
	}
	if err := writeMetricsFile(metricsPath, raw); err != nil {
		return err
	}
	return nil
}

func parseRunArgs(args []string) (string, string, error) {
	var scenarioPath string
	var metricsPath string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--metrics":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("run: --metrics requires a path")
			}
			if strings.HasPrefix(args[i+1], "-") {
				return "", "", fmt.Errorf("run: --metrics requires a path, got flag-like value %q", args[i+1])
			}
			if metricsPath != "" {
				return "", "", fmt.Errorf("run: --metrics specified more than once")
			}
			metricsPath = args[i+1]
			i++
		case strings.HasPrefix(arg, "-"):
			return "", "", fmt.Errorf("run: unknown flag %q", arg)
		default:
			if scenarioPath != "" {
				return "", "", fmt.Errorf("run: unexpected argument %q", arg)
			}
			scenarioPath = arg
		}
	}
	if scenarioPath == "" {
		return "", "", fmt.Errorf("run: scenario path is required")
	}
	if metricsPath == "" {
		return "", "", fmt.Errorf("run: --metrics path is required")
	}
	return scenarioPath, metricsPath, nil
}

func printRunUsage(w *os.File) {
	fmt.Fprintln(w, "usage: adhr-sim run <scenario.json> --metrics <path>")
}

func executeScenario(cfg runnableScenarioConfig) (metrics scenario.ScenarioMetrics, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("scenario validation failed: %v", recovered)
		}
	}()

	switch {
	case cfg.OneHop != nil:
		runner := scenario.NewScenarioRunner(*cfg.OneHop)
		return runner.Run()
	case cfg.TwoHopLive != nil:
		report, err := scenario.RunTwoHopLiveScenario(*cfg.TwoHopLive)
		if err != nil {
			return scenario.ScenarioMetrics{}, err
		}
		return scenario.TwoHopLiveScenarioMetrics(report)
	case cfg.ScheduledRelayTree != nil:
		return scenario.RunScheduledRelayTreeScenario(*cfg.ScheduledRelayTree)
	default:
		return scenario.ScenarioMetrics{}, fmt.Errorf("scenario config %q has no runnable scenario", cfg.ScenarioID)
	}
}

func writeMetricsFile(path string, raw []byte) error {
	return writeAtomicJSONFile("metrics", path, raw)
}

func writeReportFile(path string, raw []byte) error {
	return writeAtomicJSONFile("report", path, raw)
}

func writeAtomicJSONFile(label string, path string, raw []byte) error {
	if path == "" {
		return fmt.Errorf("%s path is required", label)
	}
	if isDirectoryShapedPath(path) {
		return fmt.Errorf("%s path %q must name a file, not a directory", label, path)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s directory %q: %w", label, dir, err)
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary %s file in %q: %w", label, dir, err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary %s file %q: %w", label, tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary %s file %q: %w", label, tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("write %s file %q: %w", label, path, err)
	}
	cleanup = false
	return nil
}

func isDirectoryShapedPath(path string) bool {
	if path == "." || path == string(os.PathSeparator) {
		return true
	}
	return strings.HasSuffix(path, string(os.PathSeparator))
}
