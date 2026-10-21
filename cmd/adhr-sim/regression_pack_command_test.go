package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIT047_RegressionPackCommandWritesSummaryAndArtifacts(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "regression")

	if err := run([]string{"regression-pack", regressionPackConfigPath(t), "--out", outDir}); err != nil {
		t.Fatalf("regression-pack command failed: %v", err)
	}

	summary := readRegressionPackSummary(t, filepath.Join(outDir, "summary.json"))
	if summary.PackID != "core_cli_regression" {
		t.Fatalf("pack_id = %q, want core_cli_regression", summary.PackID)
	}
	if summary.Outcome != regressionPackOutcomePass {
		t.Fatalf("outcome = %q, want pass", summary.Outcome)
	}
	if summary.CaseCount != 17 || len(summary.Cases) != 17 {
		t.Fatalf("case count = %d/%d, want 17/17", summary.CaseCount, len(summary.Cases))
	}
	if summary.ExpectedPassCount != 17 || summary.UnexpectedCount != 0 {
		t.Fatalf("expected/unexpected = %d/%d, want 17/0", summary.ExpectedPassCount, summary.UnexpectedCount)
	}
	if summary.Determinism.EventHash != "67dcf00f8d52398d0b79cc3dc33c4f14ca9ac89dcfd0a9f7a8db40ce90868376" {
		t.Fatalf("determinism.event_hash = %q", summary.Determinism.EventHash)
	}

	cases := regressionPackCasesByID(summary.Cases)
	assertRegressionPackCase(t, cases, outDir, "one_hop_nominal", "pass", "pass")
	assertRegressionPackCase(t, cases, outDir, "one_hop_direct_duplicate", "pass", "pass")
	assertRegressionPackCase(t, cases, outDir, "one_hop_corruption", "fail", "pass")
	assertRegressionPackCase(t, cases, outDir, "two_hop_short_source", "pass", "pass")
	assertRegressionPackCase(t, cases, outDir, "relay_tree_direct", "pass", "pass")
	assertRegressionPackCase(t, cases, outDir, "relay_tree_one_relay", "pass", "pass")
	assertRegressionPackCase(t, cases, outDir, "relay_tree_two_relay", "pass", "pass")
	assertRegressionPackCase(t, cases, outDir, "relay_tree_three_relay_exception", "pass", "pass")
	assertRegressionPackCase(t, cases, outDir, "relay_tree_branch_narrow", "pass", "pass")
	assertRegressionPackCase(t, cases, outDir, "relay_tree_branch_wide", "pass", "pass")
	assertRegressionPackCase(t, cases, outDir, "relay_tree_partition_recovery", "pass", "pass")
	assertRegressionPackCase(t, cases, outDir, "relay_forward_queue_backpressure", "fail", "pass")
	assertRegressionPackCase(t, cases, outDir, "group_floor_ptt_contention", "floor_degraded", "pass")
}

func TestIT048_RegressionPackHardwareReadinessChain(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "regression")

	if err := run([]string{"regression-pack", regressionPackConfigPath(t), "--out", outDir}); err != nil {
		t.Fatalf("regression-pack command failed: %v", err)
	}

	summary := readRegressionPackSummary(t, filepath.Join(outDir, "summary.json"))
	cases := regressionPackCasesByID(summary.Cases)
	assertRegressionPackCase(t, cases, outDir, "channel_bench_fixture", "channel_fixture_only", "pass")
	assertRegressionPackCase(t, cases, outDir, "tyt_turnaround_fixture", "primary_24ms_viable", "pass")
	assertRegressionPackCase(t, cases, outDir, "audio_fixture", "good_voice_pass", "pass")
	assertRegressionPackCase(t, cases, outDir, "hardware_readiness_fixture", "hardware_readiness_blocked", "pass")

	hardware := cases["hardware_readiness_fixture"]
	if hardware.Input != "" {
		t.Fatalf("hardware readiness input = %q, want empty", hardware.Input)
	}
	if hardware.Inputs == nil ||
		hardware.Inputs.Channel != "reports/channel-fixture.json" ||
		hardware.Inputs.Turnaround != "reports/tyt-turnaround-fixture.json" ||
		hardware.Inputs.Audio != "reports/audio-fixture.json" {
		t.Fatalf("hardware readiness inputs = %+v", hardware.Inputs)
	}
}

func TestIT047_RegressionPackCommandDeterministicSummary(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first")
	second := filepath.Join(dir, "second")

	if err := run([]string{"regression-pack", regressionPackConfigPath(t), "--out", first}); err != nil {
		t.Fatalf("first regression-pack command failed: %v", err)
	}
	if err := run([]string{"regression-pack", regressionPackConfigPath(t), "--out", second}); err != nil {
		t.Fatalf("second regression-pack command failed: %v", err)
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
		t.Fatalf("regression-pack summaries differ:\nfirst=%s\nsecond=%s", firstRaw, secondRaw)
	}
}

func TestRegressionPackCommandRejectsInvalidArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing pack", args: []string{"regression-pack"}, wantErr: "pack path"},
		{name: "missing out", args: []string{"regression-pack", "pack.json"}, wantErr: "--out"},
		{name: "flag like out", args: []string{"regression-pack", "pack.json", "--out", "--help"}, wantErr: "flag-like"},
		{name: "duplicate out", args: []string{"regression-pack", "pack.json", "--out", "a", "--out", "b"}, wantErr: "more than once"},
		{name: "unknown flag", args: []string{"regression-pack", "--bad"}, wantErr: "unknown flag"},
		{name: "unexpected arg", args: []string{"regression-pack", "a.json", "b.json", "--out", "out"}, wantErr: "unexpected argument"},
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

func TestRegressionPackCommandRejectsInvalidPack(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{name: "malformed", body: `{"pack_id": "bad"`, wantErr: "decode regression pack"},
		{name: "unknown field", body: `{"pack_id":"bad","seed":42,"unexpected":true,"cases":[]}`, wantErr: "unknown field"},
		{name: "trailing token", body: `{"pack_id":"bad","seed":42,"cases":[]}{}`, wantErr: "trailing JSON token"},
		{name: "missing pack id", body: `{"seed":42,"cases":[]}`, wantErr: "pack_id is required"},
		{name: "zero seed", body: `{"pack_id":"bad","seed":0,"cases":[]}`, wantErr: "seed must be > 0"},
		{name: "empty cases", body: `{"pack_id":"bad","seed":42,"cases":[]}`, wantErr: "cases must not be empty"},
		{name: "unsupported command", body: regressionPackFixtureCase(`"command":"bad-command"`), wantErr: "not supported"},
		{name: "duplicate case", body: `{"pack_id":"bad","seed":42,"cases":[{"case_id":"a","command":"run","input":"x.json","output":"a.json","expected_outcome":"pass"},{"case_id":"a","command":"run","input":"x.json","output":"b.json","expected_outcome":"pass"}]}`, wantErr: "duplicated"},
		{name: "duplicate output", body: `{"pack_id":"bad","seed":42,"cases":[{"case_id":"a","command":"run","input":"x.json","output":"a.json","expected_outcome":"pass"},{"case_id":"b","command":"run","input":"x.json","output":"a.json","expected_outcome":"pass"}]}`, wantErr: "output \"a.json\" is duplicated"},
		{name: "absolute input", body: regressionPackFixtureCase(`"input":"/tmp/x.json"`), wantErr: "input must be relative"},
		{name: "unsafe output", body: regressionPackFixtureCase(`"output":"../x.json"`), wantErr: "output must stay inside"},
		{name: "missing expected outcome", body: `{"pack_id":"bad","seed":42,"cases":[{"case_id":"a","command":"run","input":"x.json","output":"a.json"}]}`, wantErr: "expected_outcome is required"},
		{name: "inputs on single input command", body: `{"pack_id":"bad","seed":42,"cases":[{"case_id":"a","command":"run","input":"x.json","inputs":{"channel":"reports/a.json","turnaround":"reports/b.json","audio":"reports/c.json"},"output":"a.json","expected_outcome":"pass"}]}`, wantErr: "inputs is only supported"},
		{name: "hardware flags on single input command", body: `{"pack_id":"bad","seed":42,"cases":[{"case_id":"a","command":"run","input":"x.json","output":"a.json","expected_outcome":"pass","tx_requested":true}]}`, wantErr: "hardware readiness flags"},
		{name: "hardware missing inputs", body: `{"pack_id":"bad","seed":42,"cases":[{"case_id":"a","command":"hardware-readiness","output":"a.json","expected_outcome":"hardware_readiness_blocked"}]}`, wantErr: "inputs is required"},
		{name: "hardware rejects input", body: `{"pack_id":"bad","seed":42,"cases":[{"case_id":"a","command":"hardware-readiness","input":"x.json","inputs":{"channel":"reports/a.json","turnaround":"reports/b.json","audio":"reports/c.json"},"output":"a.json","expected_outcome":"hardware_readiness_blocked"}]}`, wantErr: "input must be empty"},
		{name: "hardware unsafe input", body: `{"pack_id":"bad","seed":42,"cases":[{"case_id":"a","command":"hardware-readiness","inputs":{"channel":"../a.json","turnaround":"reports/b.json","audio":"reports/c.json"},"output":"a.json","expected_outcome":"hardware_readiness_blocked"}]}`, wantErr: "inputs.channel must stay inside"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			packPath := writeRegressionPackConfig(t, tc.body)
			err := run([]string{"regression-pack", packPath, "--out", filepath.Join(t.TempDir(), "out")})
			if err == nil {
				t.Fatal("regression-pack accepted invalid pack")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestRegressionPackCommandRejectsNonEmptyOutputDir(t *testing.T) {
	outDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(outDir, "summary.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("seed stale summary failed: %v", err)
	}

	err := run([]string{"regression-pack", regressionPackConfigPath(t), "--out", outDir})
	if err == nil {
		t.Fatal("regression-pack accepted non-empty output directory")
	}
	if !strings.Contains(err.Error(), "must be empty") {
		t.Fatalf("error = %q, want must be empty", err.Error())
	}
}

func readRegressionPackSummary(t *testing.T, path string) regressionPackSummary {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read regression pack summary failed: %v", err)
	}
	var summary regressionPackSummary
	if err := json.Unmarshal(raw, &summary); err != nil {
		t.Fatalf("unmarshal regression pack summary failed: %v\n%s", err, raw)
	}
	return summary
}

func regressionPackCasesByID(rows []regressionPackCaseSummary) map[string]regressionPackCaseSummary {
	out := make(map[string]regressionPackCaseSummary, len(rows))
	for _, row := range rows {
		out[row.CaseID] = row
	}
	return out
}

func assertRegressionPackCase(t *testing.T, cases map[string]regressionPackCaseSummary, outDir string, caseID string, wantOutcome string, wantStatus string) {
	t.Helper()
	row, ok := cases[caseID]
	if !ok {
		t.Fatalf("case %q missing", caseID)
	}
	if row.Outcome != wantOutcome || row.Status != wantStatus {
		t.Fatalf("case %s outcome/status = %q/%q, want %q/%q", caseID, row.Outcome, row.Status, wantOutcome, wantStatus)
	}
	if row.EventHash == "" {
		t.Fatalf("case %s event_hash is empty", caseID)
	}
	if filepath.IsAbs(row.Output) {
		t.Fatalf("case %s output is absolute: %s", caseID, row.Output)
	}
	if _, err := os.Stat(filepath.Join(outDir, filepath.FromSlash(row.Output))); err != nil {
		t.Fatalf("case %s output artifact missing: %v", caseID, err)
	}
}

func regressionPackConfigPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "configs", "regression", "core-cli-regression.json")
}

func writeRegressionPackConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pack.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write regression pack failed: %v", err)
	}
	return path
}

func regressionPackFixtureCase(replacement string) string {
	body := `{"pack_id":"bad","seed":42,"cases":[{"case_id":"a","command":"run","input":"x.json","output":"a.json","expected_outcome":"pass"}]}`
	switch {
	case strings.HasPrefix(replacement, `"command"`):
		return strings.Replace(body, `"command":"run"`, replacement, 1)
	case strings.HasPrefix(replacement, `"input"`):
		return strings.Replace(body, `"input":"x.json"`, replacement, 1)
	case strings.HasPrefix(replacement, `"output"`):
		return strings.Replace(body, `"output":"a.json"`, replacement, 1)
	default:
		return body
	}
}
