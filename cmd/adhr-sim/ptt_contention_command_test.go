package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

func TestIT044_PTTContentionCommandWritesReport(t *testing.T) {
	reportPath := filepath.Join(t.TempDir(), "out", "ptt.json")

	if err := run([]string{"ptt-contention", pttContentionConfigPath(t), "--report", reportPath}); err != nil {
		t.Fatalf("ptt-contention command failed: %v", err)
	}

	report := readPTTContentionReport(t, reportPath)
	if report.ScenarioID != scenario.GroupFloorPTTContentionScenarioID {
		t.Fatalf("scenario_id = %q, want %q", report.ScenarioID, scenario.GroupFloorPTTContentionScenarioID)
	}
	if report.FloorPolicy != scenario.GroupFloorPolicyFirstArrivalWins {
		t.Fatalf("floor_policy = %q, want %q", report.FloorPolicy, scenario.GroupFloorPolicyFirstArrivalWins)
	}
	if report.GrantedCount != 3 || report.DeniedDueBusyCount != 2 || report.CollisionCount != 1 {
		t.Fatalf("granted/denied/collision = %d/%d/%d, want 3/2/1",
			report.GrantedCount, report.DeniedDueBusyCount, report.CollisionCount)
	}
	if report.DuplicatePlaybackCount != 0 {
		t.Fatalf("duplicate_playback_count = %d, want 0", report.DuplicatePlaybackCount)
	}
	if report.EventHash == "" || report.EventHash != report.Evidence.Determinism.EventHash {
		t.Fatalf("event hash mismatch: report=%q evidence=%q", report.EventHash, report.Evidence.Determinism.EventHash)
	}
	if report.Evidence.HardwareClaimAllowed {
		t.Fatal("ptt contention report unexpectedly allows hardware claims")
	}
}

func TestIT044_PTTContentionCommandDeterministicReport(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.json")
	second := filepath.Join(dir, "nested", "second.json")

	if err := run([]string{"ptt-contention", pttContentionConfigPath(t), "--report", first}); err != nil {
		t.Fatalf("first ptt-contention command failed: %v", err)
	}
	if err := run([]string{"ptt-contention", pttContentionConfigPath(t), "--report", second}); err != nil {
		t.Fatalf("second ptt-contention command failed: %v", err)
	}
	firstRaw, err := os.ReadFile(first)
	if err != nil {
		t.Fatalf("read first report failed: %v", err)
	}
	secondRaw, err := os.ReadFile(second)
	if err != nil {
		t.Fatalf("read second report failed: %v", err)
	}
	if string(firstRaw) != string(secondRaw) {
		t.Fatalf("ptt contention reports differ:\nfirst=%s\nsecond=%s", firstRaw, secondRaw)
	}
}

func TestPTTContentionCommandRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{name: "malformed", body: `{"scenario_id": "group_floor_ptt_contention"`, wantErr: "decode ptt contention scenario"},
		{name: "unknown field", body: strings.Replace(fixturePTTContentionConfigJSON(), `"attempts":`, `"unexpected": true, "attempts":`, 1), wantErr: "unknown field"},
		{name: "trailing token", body: fixturePTTContentionConfigJSON() + "\n{}", wantErr: "trailing JSON token"},
		{name: "bad scenario", body: strings.Replace(fixturePTTContentionConfigJSON(), scenario.GroupFloorPTTContentionScenarioID, "other", 1), wantErr: "unsupported scenario_id"},
		{name: "missing seed", body: `{"scenario_id":"group_floor_ptt_contention"}`, wantErr: "seed is required"},
		{name: "zero access decision", body: strings.Replace(fixturePTTContentionConfigJSON(), `"access_decision_ms": 20`, `"access_decision_ms": 0`, 1), wantErr: "access_decision_ms must be > 0"},
		{name: "zero first audio window", body: strings.Replace(fixturePTTContentionConfigJSON(), `"first_audio_window_ms": 250`, `"first_audio_window_ms": 0`, 1), wantErr: "first_audio_window_ms must be > 0"},
		{name: "missing attempt ptt start", body: strings.Replace(fixturePTTContentionConfigJSON(), `"ptt_start_ms": 0, `, ``, 1), wantErr: "attempts[0].ptt_start_ms is required"},
		{name: "null attempt ptt start", body: strings.Replace(fixturePTTContentionConfigJSON(), `"ptt_start_ms": 0`, `"ptt_start_ms": null`, 1), wantErr: "attempts[0].ptt_start_ms must not be null"},
		{name: "invalid config", body: strings.Replace(fixturePTTContentionConfigJSON(), `"floor_policy": "first_arrival_wins"`, `"floor_policy": "queue"`, 1), wantErr: "unsupported floor_policy"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inputPath := writePTTContentionConfig(t, tc.body)
			reportPath := filepath.Join(t.TempDir(), "report.json")
			err := run([]string{"ptt-contention", inputPath, "--report", reportPath})
			if err == nil {
				t.Fatal("ptt-contention accepted invalid input")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want %q", err.Error(), tc.wantErr)
			}
			assertFileMissing(t, reportPath)
		})
	}
}

func TestPTTContentionCommandRejectsInvalidArgs(t *testing.T) {
	err := run([]string{"ptt-contention"})
	if err == nil || !strings.Contains(err.Error(), "scenario path") {
		t.Fatalf("missing input error = %v, want scenario path", err)
	}
	err = run([]string{"ptt-contention", "input.json"})
	if err == nil || !strings.Contains(err.Error(), "--report") {
		t.Fatalf("missing report error = %v, want --report", err)
	}
	err = run([]string{"ptt-contention", "input.json", "--report", "--help"})
	if err == nil || !strings.Contains(err.Error(), "flag-like") {
		t.Fatalf("flag-like report error = %v, want flag-like", err)
	}
}

func pttContentionConfigPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "configs", "scenarios", "group-floor-ptt-contention.json")
}

func writePTTContentionConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ptt-contention.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write ptt contention config failed: %v", err)
	}
	return path
}

func readPTTContentionReport(t *testing.T, path string) scenario.GroupFloorPTTContentionReport {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ptt contention report failed: %v", err)
	}
	var report scenario.GroupFloorPTTContentionReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("decode ptt contention report failed: %v\n%s", err, raw)
	}
	return report
}

func fixturePTTContentionConfigJSON() string {
	raw, err := os.ReadFile(filepath.Join("..", "..", "configs", "scenarios", "group-floor-ptt-contention.json"))
	if err != nil {
		panic(err)
	}
	return string(raw)
}
