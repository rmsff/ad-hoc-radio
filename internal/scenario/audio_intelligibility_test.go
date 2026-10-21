package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestIT034_AudioIntelligibilityDefaultReportDeterministic(t *testing.T) {
	first, err := RunAudioIntelligibilitySimulation(DefaultAudioIntelligibilityConfig())
	if err != nil {
		t.Fatalf("RunAudioIntelligibilitySimulation failed: %v", err)
	}
	second, err := RunAudioIntelligibilitySimulation(DefaultAudioIntelligibilityConfig())
	if err != nil {
		t.Fatalf("second RunAudioIntelligibilitySimulation failed: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("default audio intelligibility report is not deterministic")
	}
	if first.ScenarioID != AudioIntelligibilityScenarioID {
		t.Fatalf("ScenarioID = %q, want %q", first.ScenarioID, AudioIntelligibilityScenarioID)
	}
	if first.RowCount != 12 || len(first.Rows) != 12 {
		t.Fatalf("row count = %d/%d, want 12", first.RowCount, len(first.Rows))
	}
	if first.GoodVoicePassRows == 0 || first.DegradedRows == 0 || first.FailRows == 0 {
		t.Fatalf("pass/degraded/fail rows = %d/%d/%d, want all non-zero", first.GoodVoicePassRows, first.DegradedRows, first.FailRows)
	}
	if first.Evidence.Outcome != AudioIntelligibilityOutcomeDegraded {
		t.Fatalf("report evidence outcome = %q, want %q for mixed matrix", first.Evidence.Outcome, AudioIntelligibilityOutcomeDegraded)
	}
	if got := first.Evidence.DegradeReasons; len(got) != 1 || got[0] != "matrix_contains_degraded_or_failed_rows" {
		t.Fatalf("report evidence degrade reasons = %v, want matrix mixed reason", got)
	}
	if first.Evidence.HardwareClaimAllowed {
		t.Fatal("audio fixture/proxy report unexpectedly allows hardware claims")
	}
	if first.GoodVoiceThreshold != AudioIntelligibilityDefaultThreshold {
		t.Fatalf("GoodVoiceThreshold = %d, want %d", first.GoodVoiceThreshold, AudioIntelligibilityDefaultThreshold)
	}
	if first.EventHash == "" || first.EventHash != first.Evidence.Determinism.EventHash {
		t.Fatalf("event hash mismatch: report=%q evidence=%q", first.EventHash, first.Evidence.Determinism.EventHash)
	}
	if err := ValidateScenarioEvidence(first.Evidence); err != nil {
		t.Fatalf("ValidateScenarioEvidence failed: %v", err)
	}
}

func TestIT034_AudioIntelligibilityRequiredRowVocabulary(t *testing.T) {
	report, err := RunAudioIntelligibilitySimulation(DefaultAudioIntelligibilityConfig())
	if err != nil {
		t.Fatalf("RunAudioIntelligibilitySimulation failed: %v", err)
	}
	for _, row := range report.Rows {
		if row.FixtureID == "" || row.FixturePath == "" || row.ArtifactSHA256 == "" {
			t.Fatalf("row %q missing fixture vocabulary: %+v", row.RowID, row)
		}
		if row.EvidenceGrade == "" || row.NoiseProfileID == "" || row.LossProfileID == "" {
			t.Fatalf("row %q missing profile vocabulary: %+v", row.RowID, row)
		}
		if row.CodecClass == "" || row.AudioBandwidthHz <= 0 || row.ScoringMethod == "" {
			t.Fatalf("row %q missing codec/scoring vocabulary: %+v", row.RowID, row)
		}
		if row.IntelligibilityProxyScore < 0 || row.IntelligibilityProxyScore > 100 {
			t.Fatalf("row %q score = %d, want [0,100]", row.RowID, row.IntelligibilityProxyScore)
		}
		if row.GoodVoiceThreshold != AudioIntelligibilityDefaultThreshold {
			t.Fatalf("row %q threshold = %d, want %d", row.RowID, row.GoodVoiceThreshold, AudioIntelligibilityDefaultThreshold)
		}
		if row.Outcome == "" {
			t.Fatalf("row %q missing outcome", row.RowID)
		}
	}
}

func TestIT034_GoodVoicePassRequiresStrongEvidenceAndThreshold(t *testing.T) {
	report, err := RunAudioIntelligibilitySimulation(DefaultAudioIntelligibilityConfig())
	if err != nil {
		t.Fatalf("RunAudioIntelligibilitySimulation failed: %v", err)
	}
	for _, row := range report.Rows {
		if row.Outcome != AudioIntelligibilityOutcomeGoodVoicePass {
			continue
		}
		if row.EvidenceGrade != EvidenceGradeFixtureArtifact && row.EvidenceGrade != EvidenceGradeBenchMeasured {
			t.Fatalf("good_voice_pass row %q has evidence_grade %q", row.RowID, row.EvidenceGrade)
		}
		if row.ArtifactSHA256 == "" {
			t.Fatalf("good_voice_pass row %q has empty artifact_sha256", row.RowID)
		}
		if row.IntelligibilityProxyScore < row.GoodVoiceThreshold {
			t.Fatalf("good_voice_pass row %q score=%d threshold=%d",
				row.RowID, row.IntelligibilityProxyScore, row.GoodVoiceThreshold)
		}
	}

	cfg := DefaultAudioIntelligibilityConfig()
	cfg.Fixture.EvidenceGrade = EvidenceGradeSyntheticProxy
	cfg.Fixture.ArtifactSHA256 = ""
	synthetic, err := RunAudioIntelligibilitySimulation(cfg)
	if err != nil {
		t.Fatalf("synthetic RunAudioIntelligibilitySimulation failed: %v", err)
	}
	for _, row := range synthetic.Rows {
		if row.Outcome == AudioIntelligibilityOutcomeGoodVoicePass {
			t.Fatalf("synthetic row %q got forbidden good_voice_pass", row.RowID)
		}
	}
}

func TestIT034_ReportOutcomePassRequiresAllRowsPass(t *testing.T) {
	cfg := DefaultAudioIntelligibilityConfig()
	cfg.NoiseProfiles = []AudioIntelligibilityNoiseProfile{
		{NoiseProfileID: "clean", NoiseSNRDb: 40},
	}
	cfg.LossProfiles = []AudioIntelligibilityLossProfile{
		{LossProfileID: "no_loss", PacketLossPct: 0, BurstLossMs: 0},
	}
	report, err := RunAudioIntelligibilitySimulation(cfg)
	if err != nil {
		t.Fatalf("RunAudioIntelligibilitySimulation failed: %v", err)
	}
	if report.Evidence.Outcome != AudioIntelligibilityOutcomeGoodVoicePass {
		t.Fatalf("single passing row report outcome = %q, want %q",
			report.Evidence.Outcome, AudioIntelligibilityOutcomeGoodVoicePass)
	}
	if len(report.Evidence.DegradeReasons) != 0 {
		t.Fatalf("single passing row degrade reasons = %v, want none", report.Evidence.DegradeReasons)
	}

	cfg.NoiseProfiles = []AudioIntelligibilityNoiseProfile{
		{NoiseProfileID: "severe", NoiseSNRDb: -20},
	}
	cfg.LossProfiles = []AudioIntelligibilityLossProfile{
		{LossProfileID: "total_loss", PacketLossPct: 100, BurstLossMs: 1000},
	}
	failed, err := RunAudioIntelligibilitySimulation(cfg)
	if err != nil {
		t.Fatalf("failed RunAudioIntelligibilitySimulation failed: %v", err)
	}
	if failed.Evidence.Outcome != AudioIntelligibilityOutcomeFail {
		t.Fatalf("single failed row report outcome = %q, want %q",
			failed.Evidence.Outcome, AudioIntelligibilityOutcomeFail)
	}
	if got := failed.Evidence.DegradeReasons; len(got) != 1 || got[0] != "all_rows_failed" {
		t.Fatalf("single failed row degrade reasons = %v, want all_rows_failed", got)
	}
}

func TestIT034_BenchMeasuredAudioFixtureDoesNotAllowHardwareClaim(t *testing.T) {
	cfg := DefaultAudioIntelligibilityConfig()
	cfg.Fixture.EvidenceGrade = EvidenceGradeBenchMeasured
	report, err := RunAudioIntelligibilitySimulation(cfg)
	if err != nil {
		t.Fatalf("RunAudioIntelligibilitySimulation failed: %v", err)
	}
	if report.Evidence.HardwareClaimAllowed {
		t.Fatal("audio fixture report must not allow hardware claims even when evidence grade is bench_measured")
	}
}

func TestIT034_AudioIntelligibilityDefaultMatrixCoversBaselineAndCartesian(t *testing.T) {
	report, err := RunAudioIntelligibilitySimulation(DefaultAudioIntelligibilityConfig())
	if err != nil {
		t.Fatalf("RunAudioIntelligibilitySimulation failed: %v", err)
	}
	if report.RowCount < 10 {
		t.Fatalf("RowCount = %d, want at least 10", report.RowCount)
	}
	if row := mustFindAudioIntelligibilityRow(t, report.Rows, "clean_no_loss"); row.Outcome != AudioIntelligibilityOutcomeGoodVoicePass {
		t.Fatalf("clean baseline outcome = %q, want %q", row.Outcome, AudioIntelligibilityOutcomeGoodVoicePass)
	}
	noiseIDs := make(map[string]struct{})
	lossIDs := make(map[string]struct{})
	for _, row := range report.Rows {
		noiseIDs[row.NoiseProfileID] = struct{}{}
		lossIDs[row.LossProfileID] = struct{}{}
	}
	if len(noiseIDs) < 3 || len(lossIDs) < 3 {
		t.Fatalf("noise/loss cardinality = %d/%d, want at least 3/3", len(noiseIDs), len(lossIDs))
	}
	if report.RowCount != len(noiseIDs)*len(lossIDs) {
		t.Fatalf("RowCount = %d, want cartesian %d", report.RowCount, len(noiseIDs)*len(lossIDs))
	}
}

func TestIT034_AudioIntelligibilityFixtureHashMatchesFile(t *testing.T) {
	cfg := DefaultAudioIntelligibilityConfig()
	path := filepath.Join("..", "..", cfg.Fixture.FixturePath)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", path, err)
	}
	sum := sha256.Sum256(b)
	if got := hex.EncodeToString(sum[:]); got != cfg.Fixture.ArtifactSHA256 {
		t.Fatalf("fixture sha256 = %s, want %s", got, cfg.Fixture.ArtifactSHA256)
	}
}

func TestAudioIntelligibilityRejectsInvalidConfig(t *testing.T) {
	valid := func() AudioIntelligibilityConfig {
		return DefaultAudioIntelligibilityConfig()
	}
	tests := []struct {
		name string
		cfg  AudioIntelligibilityConfig
	}{
		{name: "unsupported scenario", cfg: func() AudioIntelligibilityConfig {
			cfg := valid()
			cfg.ScenarioID = "other"
			return cfg
		}()},
		{name: "bad threshold", cfg: func() AudioIntelligibilityConfig {
			cfg := valid()
			cfg.GoodVoiceThreshold = 101
			return cfg
		}()},
		{name: "bad grade", cfg: func() AudioIntelligibilityConfig {
			cfg := valid()
			cfg.Fixture.EvidenceGrade = "forum_guess"
			return cfg
		}()},
		{name: "bad artifact hash", cfg: func() AudioIntelligibilityConfig {
			cfg := valid()
			cfg.Fixture.ArtifactSHA256 = "abc"
			return cfg
		}()},
		{name: "duplicate noise", cfg: func() AudioIntelligibilityConfig {
			cfg := valid()
			cfg.NoiseProfiles[1].NoiseProfileID = cfg.NoiseProfiles[0].NoiseProfileID
			return cfg
		}()},
		{name: "duplicate loss", cfg: func() AudioIntelligibilityConfig {
			cfg := valid()
			cfg.LossProfiles[1].LossProfileID = cfg.LossProfiles[0].LossProfileID
			return cfg
		}()},
		{name: "row cap", cfg: func() AudioIntelligibilityConfig {
			cfg := valid()
			cfg.NoiseProfiles = make([]AudioIntelligibilityNoiseProfile, 9)
			cfg.LossProfiles = make([]AudioIntelligibilityLossProfile, 9)
			for i := range cfg.NoiseProfiles {
				cfg.NoiseProfiles[i] = AudioIntelligibilityNoiseProfile{
					NoiseProfileID: "noise_" + string(rune('a'+i)),
					NoiseSNRDb:     20,
				}
			}
			for i := range cfg.LossProfiles {
				cfg.LossProfiles[i] = AudioIntelligibilityLossProfile{
					LossProfileID: "loss_" + string(rune('a'+i)),
				}
			}
			return cfg
		}()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := RunAudioIntelligibilitySimulation(tc.cfg); err == nil {
				t.Fatal("RunAudioIntelligibilitySimulation accepted invalid config")
			}
		})
	}
}

func mustFindAudioIntelligibilityRow(t *testing.T, rows []AudioIntelligibilityRow, id string) AudioIntelligibilityRow {
	t.Helper()
	for _, row := range rows {
		if row.RowID == id {
			return row
		}
	}
	t.Fatalf("missing audio intelligibility row %q", id)
	return AudioIntelligibilityRow{}
}
