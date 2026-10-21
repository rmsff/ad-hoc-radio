package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

func TestIT011_SweepCommandWritesSummaryAndMetrics(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "sweeps")

	if err := run([]string{"sweep", "--out", outDir}); err != nil {
		t.Fatalf("sweep command failed: %v", err)
	}

	summary := readSweepSummary(t, filepath.Join(outDir, "summary.json"))
	if len(summary.Rows) != 118 {
		t.Fatalf("summary rows = %d, want 118", len(summary.Rows))
	}

	for _, row := range summary.Rows {
		if filepath.IsAbs(row.MetricsPath) {
			t.Errorf("row %s metrics_path is absolute: %s", row.RowID, row.MetricsPath)
		}
		metricsPath := filepath.Join(outDir, row.MetricsPath)
		report := readMetricsReport(t, metricsPath)
		if report.Determinism.EventHash != row.EventHash {
			t.Errorf("row %s event_hash = %s, metrics event_hash = %s",
				row.RowID, row.EventHash, report.Determinism.EventHash)
		}
		if report.ScenarioID != row.ScenarioID {
			t.Errorf("row %s scenario_id = %q, metrics scenario_id = %q",
				row.RowID, row.ScenarioID, report.ScenarioID)
		}
	}
}

func TestSweepCommandDeterministicSummary(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first")
	second := filepath.Join(dir, "second")

	if err := run([]string{"sweep", "--out", first}); err != nil {
		t.Fatalf("first sweep command failed: %v", err)
	}
	if err := run([]string{"sweep", "--out", second}); err != nil {
		t.Fatalf("second sweep command failed: %v", err)
	}

	firstRaw, err := os.ReadFile(filepath.Join(first, "summary.json"))
	if err != nil {
		t.Fatalf("read first summary failed: %v", err)
	}
	secondRaw, err := os.ReadFile(filepath.Join(second, "summary.json"))
	if err != nil {
		t.Fatalf("read second summary failed: %v", err)
	}
	if string(firstRaw) != string(secondRaw) {
		t.Fatalf("sweep summaries differ:\nfirst=%s\nsecond=%s", firstRaw, secondRaw)
	}
}

func TestSweepCommandRejectsInvalidArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing out", args: []string{"sweep"}, wantErr: "--out"},
		{name: "flag like out", args: []string{"sweep", "--out", "--help"}, wantErr: "flag-like"},
		{name: "duplicate out", args: []string{"sweep", "--out", "a", "--out", "b"}, wantErr: "more than once"},
		{name: "unknown flag", args: []string{"sweep", "--unknown"}, wantErr: "unknown flag"},
		{name: "unexpected arg", args: []string{"sweep", "--out", "a", "extra"}, wantErr: "unexpected argument"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := run(tc.args)
			if err == nil {
				t.Fatalf("run(%v) succeeded, want error", tc.args)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestSweepCommandRejectsNonEmptyOutputDir(t *testing.T) {
	outDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(outDir, "summary.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("seed stale summary failed: %v", err)
	}

	err := run([]string{"sweep", "--out", outDir})
	if err == nil {
		t.Fatal("sweep command accepted non-empty output directory")
	}
	if !strings.Contains(err.Error(), "must be empty") {
		t.Fatalf("error = %q, want must be empty", err.Error())
	}
}

func TestSweepCommandRejectsFileOutputPath(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "sweeps")
	if err := os.WriteFile(outPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("write output file failed: %v", err)
	}

	err := run([]string{"sweep", "--out", outPath})
	if err == nil {
		t.Fatal("sweep command accepted file output path")
	}
	if !strings.Contains(err.Error(), "must be a directory") {
		t.Fatalf("error = %q, want directory context", err.Error())
	}
}

func TestSweepCommandHelpDoesNotRequireOut(t *testing.T) {
	if err := run([]string{"sweep", "--help"}); err != nil {
		t.Fatalf("sweep --help returned error: %v", err)
	}
}

func readSweepSummary(t *testing.T, path string) scenario.SweepSummary {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sweep summary failed: %v", err)
	}
	var summary scenario.SweepSummary
	if err := json.Unmarshal(raw, &summary); err != nil {
		t.Fatalf("unmarshal sweep summary failed: %v\n%s", err, raw)
	}
	return summary
}
