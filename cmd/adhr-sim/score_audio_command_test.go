package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

func TestIT041_ScoreAudioCommandWritesReport(t *testing.T) {
	inputPath := writeAudioFixture(t, fixtureAudioConfigJSON())
	reportPath := filepath.Join(t.TempDir(), "out", "audio-report.json")

	if err := run([]string{"score-audio", inputPath, "--report", reportPath}); err != nil {
		t.Fatalf("score-audio command failed: %v", err)
	}

	report := readAudioIntelligibilityReport(t, reportPath)
	if report.ScenarioID != scenario.AudioIntelligibilityScenarioID {
		t.Fatalf("scenario_id = %q, want %q", report.ScenarioID, scenario.AudioIntelligibilityScenarioID)
	}
	if report.RowCount != 1 || report.GoodVoicePassRows != 1 || report.Evidence.Outcome != scenario.AudioIntelligibilityOutcomeGoodVoicePass {
		t.Fatalf("rows/pass/outcome = %d/%d/%q, want 1/1/good_voice_pass",
			report.RowCount, report.GoodVoicePassRows, report.Evidence.Outcome)
	}
	if report.EventHash == "" || report.EventHash != report.Evidence.Determinism.EventHash {
		t.Fatalf("event hash mismatch: %q/%q", report.EventHash, report.Evidence.Determinism.EventHash)
	}
}

func TestIT041_ScoreAudioCommandDeterministicReport(t *testing.T) {
	inputPath := writeAudioFixture(t, fixtureAudioConfigJSON())
	dir := t.TempDir()
	first := filepath.Join(dir, "first.json")
	second := filepath.Join(dir, "nested", "second.json")

	if err := run([]string{"score-audio", inputPath, "--report", first}); err != nil {
		t.Fatalf("first score-audio command failed: %v", err)
	}
	if err := run([]string{"score-audio", inputPath, "--report", second}); err != nil {
		t.Fatalf("second score-audio command failed: %v", err)
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
		t.Fatalf("audio reports differ:\nfirst=%s\nsecond=%s", firstRaw, secondRaw)
	}
}

func TestScoreAudioCommandRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{name: "malformed", body: `{"scenario_id": "audio_intelligibility_fixture_envelope"`, wantErr: "decode audio fixture"},
		{name: "unknown field", body: strings.Replace(fixtureAudioConfigJSON(), `"good_voice_threshold":`, `"unexpected": true, "good_voice_threshold":`, 1), wantErr: "unknown field"},
		{name: "trailing token", body: fixtureAudioConfigJSON() + "\n{}", wantErr: "trailing JSON token"},
		{name: "bad scenario", body: strings.Replace(fixtureAudioConfigJSON(), scenario.AudioIntelligibilityScenarioID, "other", 1), wantErr: "unsupported scenario_id"},
		{name: "missing seed", body: `{"scenario_id":"audio_intelligibility_fixture_envelope"}`, wantErr: "seed is required"},
		{name: "invalid config", body: strings.Replace(fixtureAudioConfigJSON(), `"good_voice_threshold": 75`, `"good_voice_threshold": 101`, 1), wantErr: "good_voice_threshold"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inputPath := writeAudioFixture(t, tc.body)
			reportPath := filepath.Join(t.TempDir(), "report.json")
			err := run([]string{"score-audio", inputPath, "--report", reportPath})
			if err == nil {
				t.Fatal("score-audio accepted invalid input")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want %q", err.Error(), tc.wantErr)
			}
			assertFileMissing(t, reportPath)
		})
	}
}

func TestScoreAudioCommandRejectsInvalidArgs(t *testing.T) {
	err := run([]string{"score-audio"})
	if err == nil || !strings.Contains(err.Error(), "audio fixture path") {
		t.Fatalf("missing input error = %v, want audio fixture path", err)
	}
	err = run([]string{"score-audio", "input.json"})
	if err == nil || !strings.Contains(err.Error(), "--report") {
		t.Fatalf("missing report error = %v, want --report", err)
	}
	err = run([]string{"score-audio", "input.json", "--report", "--help"})
	if err == nil || !strings.Contains(err.Error(), "flag-like") {
		t.Fatalf("flag-like report error = %v, want flag-like", err)
	}
}

func writeAudioFixture(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "audio.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write audio fixture failed: %v", err)
	}
	return path
}

func readAudioIntelligibilityReport(t *testing.T, path string) scenario.AudioIntelligibilityReport {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audio report failed: %v", err)
	}
	var report scenario.AudioIntelligibilityReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("decode audio report failed: %v\n%s", err, raw)
	}
	return report
}

func fixtureAudioConfigJSON() string {
	cfg := scenario.DefaultAudioIntelligibilityConfig()
	return `{
  "scenario_id": "audio_intelligibility_fixture_envelope",
  "seed": 42,
  "fixture": {
    "fixture_id": "` + cfg.Fixture.FixtureID + `",
    "fixture_path": "` + cfg.Fixture.FixturePath + `",
    "artifact_sha256": "` + cfg.Fixture.ArtifactSHA256 + `",
    "evidence_grade": "fixture_artifact"
  },
  "noise_profiles": [
    { "noise_profile_id": "clean", "noise_snr_db": 40 }
  ],
  "loss_profiles": [
    { "loss_profile_id": "no_loss", "packet_loss_pct": 0, "burst_loss_ms": 0 }
  ],
  "codec_class": "opus_class_wideband_speech_proxy",
  "audio_bandwidth_hz": 8000,
  "scoring_method": "deterministic_fixture_proxy_v1",
  "good_voice_threshold": 75
}`
}
