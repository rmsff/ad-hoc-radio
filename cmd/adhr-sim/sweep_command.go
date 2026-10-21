package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

func runSweepCommand(args []string) error {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		printSweepUsage(os.Stdout)
		return nil
	}

	outDir, err := parseSweepArgs(args)
	if err != nil {
		return err
	}
	if err := prepareSweepOutputDir(outDir); err != nil {
		return err
	}

	result, err := scenario.RunExperimentSweeps()
	if err != nil {
		return err
	}
	for _, run := range result.Runs {
		raw, err := run.Metrics.MarshalReportJSON()
		if err != nil {
			return fmt.Errorf("marshal sweep metrics %s: %w", run.Row.RowID, err)
		}
		metricsPath := filepath.Join(outDir, filepath.FromSlash(run.Row.MetricsPath))
		if err := writeMetricsFile(metricsPath, raw); err != nil {
			return err
		}
	}

	summaryRaw, err := scenario.MarshalSweepSummaryJSON(result.Summary)
	if err != nil {
		return fmt.Errorf("marshal sweep summary: %w", err)
	}
	return writeMetricsFile(filepath.Join(outDir, "summary.json"), summaryRaw)
}

func parseSweepArgs(args []string) (string, error) {
	var outDir string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--out":
			if i+1 >= len(args) {
				return "", fmt.Errorf("sweep: --out requires a directory path")
			}
			if strings.HasPrefix(args[i+1], "-") {
				return "", fmt.Errorf("sweep: --out requires a directory path, got flag-like value %q", args[i+1])
			}
			if outDir != "" {
				return "", fmt.Errorf("sweep: --out specified more than once")
			}
			outDir = args[i+1]
			i++
		case strings.HasPrefix(arg, "-"):
			return "", fmt.Errorf("sweep: unknown flag %q", arg)
		default:
			return "", fmt.Errorf("sweep: unexpected argument %q", arg)
		}
	}
	if outDir == "" {
		return "", fmt.Errorf("sweep: --out directory path is required")
	}
	return outDir, nil
}

func printSweepUsage(w *os.File) {
	fmt.Fprintln(w, "usage: adhr-sim sweep --out <dir>")
}

func prepareSweepOutputDir(outDir string) error {
	info, err := os.Stat(outDir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return fmt.Errorf("create sweep output directory %q: %w", outDir, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect sweep output directory %q: %w", outDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("sweep output path %q must be a directory", outDir)
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return fmt.Errorf("read sweep output directory %q: %w", outDir, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("sweep output directory %q must be empty", outDir)
	}
	return nil
}
