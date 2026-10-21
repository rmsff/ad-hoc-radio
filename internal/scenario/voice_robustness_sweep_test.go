package scenario

import (
	"reflect"
	"testing"
)

func TestIT035_VoiceRobustnessDefaultReportDeterministic(t *testing.T) {
	first, err := RunVoiceRobustnessSweep(DefaultVoiceRobustnessSweepConfig())
	if err != nil {
		t.Fatalf("RunVoiceRobustnessSweep failed: %v", err)
	}
	second, err := RunVoiceRobustnessSweep(DefaultVoiceRobustnessSweepConfig())
	if err != nil {
		t.Fatalf("second RunVoiceRobustnessSweep failed: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("default voice robustness report is not deterministic")
	}
	if first.ScenarioID != VoiceRobustnessSweepScenarioID {
		t.Fatalf("ScenarioID = %q, want %q", first.ScenarioID, VoiceRobustnessSweepScenarioID)
	}
	if first.RowCount != 972 || len(first.Rows) != 972 {
		t.Fatalf("row count = %d/%d, want 972", first.RowCount, len(first.Rows))
	}
	if first.BaselineGoodVoicePassRows != 216 || first.BaselineDegradedRows != 486 || first.BaselineFailRows != 270 {
		t.Fatalf("baseline pass/degraded/fail rows = %d/%d/%d, want 216/486/270",
			first.BaselineGoodVoicePassRows, first.BaselineDegradedRows, first.BaselineFailRows)
	}
	if first.GoodVoicePassRows != 240 || first.DegradedRows != 318 || first.FailRows != 414 {
		t.Fatalf("adjusted pass/degraded/fail rows = %d/%d/%d, want 240/318/414",
			first.GoodVoicePassRows, first.DegradedRows, first.FailRows)
	}
	if first.ImprovedRows != 720 {
		t.Fatalf("ImprovedRows = %d, want 720", first.ImprovedRows)
	}
	if first.EventHash != "5727634f161391db573d73bb7d1fe1e92db52f268c3517e226d915f8e0a1369a" {
		t.Fatalf("EventHash = %q, want pinned default hash", first.EventHash)
	}
	if first.BaselineGoodVoicePassRows == 0 || first.BaselineDegradedRows == 0 || first.BaselineFailRows == 0 {
		t.Fatalf("baseline pass/degraded/fail rows = %d/%d/%d, want all non-zero",
			first.BaselineGoodVoicePassRows, first.BaselineDegradedRows, first.BaselineFailRows)
	}
	if first.GoodVoicePassRows <= first.BaselineGoodVoicePassRows {
		t.Fatalf("good voice rows = %d, want improvement over baseline %d",
			first.GoodVoicePassRows, first.BaselineGoodVoicePassRows)
	}
	if first.ImprovedRows == 0 {
		t.Fatal("ImprovedRows = 0, want at least one improved row")
	}
	if first.Evidence.Outcome != VoiceRobustnessOutcomeDegraded {
		t.Fatalf("report evidence outcome = %q, want %q for mixed matrix", first.Evidence.Outcome, VoiceRobustnessOutcomeDegraded)
	}
	if first.Evidence.HardwareClaimAllowed {
		t.Fatal("voice robustness report unexpectedly allows hardware claims")
	}
	if first.EventHash == "" || first.EventHash != first.Evidence.Determinism.EventHash {
		t.Fatalf("event hash mismatch: report=%q evidence=%q", first.EventHash, first.Evidence.Determinism.EventHash)
	}
	if err := ValidateScenarioEvidence(first.Evidence); err != nil {
		t.Fatalf("ValidateScenarioEvidence failed: %v", err)
	}
}

func TestIT035_VoiceRobustnessRowsCarryRequiredVocabulary(t *testing.T) {
	report, err := RunVoiceRobustnessSweep(DefaultVoiceRobustnessSweepConfig())
	if err != nil {
		t.Fatalf("RunVoiceRobustnessSweep failed: %v", err)
	}
	for _, row := range report.Rows {
		if row.RowID == "" || row.ProfileID == "" || row.ProfileClass == "" {
			t.Fatalf("row missing profile vocabulary: %+v", row)
		}
		if row.NoiseProfileID == "" || row.CodecClass == "" || row.AudioBandwidthHz <= 0 {
			t.Fatalf("row %q missing noise/codec vocabulary: %+v", row.RowID, row)
		}
		if row.BurstMs <= 0 || row.ProtectionOverheadPct < 0 || row.JitterBufferMs < 0 {
			t.Fatalf("row %q invalid robustness knobs: %+v", row.RowID, row)
		}
		if row.BaselineOutcome == "" || row.Outcome == "" {
			t.Fatalf("row %q missing outcomes: %+v", row.RowID, row)
		}
		if row.BaselineIntelligibilityProxyScore < 0 || row.BaselineIntelligibilityProxyScore > 100 {
			t.Fatalf("row %q baseline score = %d, want [0,100]", row.RowID, row.BaselineIntelligibilityProxyScore)
		}
		if row.AdjustedIntelligibilityProxyScore < 0 || row.AdjustedIntelligibilityProxyScore > 100 {
			t.Fatalf("row %q adjusted score = %d, want [0,100]", row.RowID, row.AdjustedIntelligibilityProxyScore)
		}
		if row.FirstAudioLatencyMs <= 0 || row.CompletionLatencyMs <= 0 || row.LatencyBudgetMs <= 0 {
			t.Fatalf("row %q invalid latency vocabulary: %+v", row.RowID, row)
		}
		if row.ProtectionAssumption == "" {
			t.Fatalf("row %q missing protection assumption", row.RowID)
		}
	}
}

func TestIT035_ImprovementIsExplicitAndLatencyBounded(t *testing.T) {
	report, err := RunVoiceRobustnessSweep(DefaultVoiceRobustnessSweepConfig())
	if err != nil {
		t.Fatalf("RunVoiceRobustnessSweep failed: %v", err)
	}
	row := mustFindVoiceRobustnessRow(t, report.Rows, "burst_250_protect_35_jitter_400_light_noise_loss_10_burst_120")
	if row.BaselineOutcome != VoiceRobustnessOutcomeDegraded {
		t.Fatalf("baseline outcome = %q, want degraded", row.BaselineOutcome)
	}
	if row.Outcome != VoiceRobustnessOutcomeGoodVoicePass {
		t.Fatalf("adjusted outcome = %q, want good_voice_pass; reasons=%v", row.Outcome, row.DegradeReasons)
	}
	if !row.ImprovedFromBaseline {
		t.Fatal("ImprovedFromBaseline = false, want true")
	}
	if row.FirstAudioLatencyMs > report.LatencyBudgetMs || row.CompletionLatencyMs > report.CompletionLatencyMaxMs {
		t.Fatalf("latency first/completion = %d/%d exceeds budgets %d/%d",
			row.FirstAudioLatencyMs, row.CompletionLatencyMs, report.LatencyBudgetMs, report.CompletionLatencyMaxMs)
	}
	if row.EffectiveBurstLossMs != 0 {
		t.Fatalf("EffectiveBurstLossMs = %d, want 0 after robust jitter/protection", row.EffectiveBurstLossMs)
	}
}

func TestIT035_SevereNoiseResidualRiskIsNotHidden(t *testing.T) {
	report, err := RunVoiceRobustnessSweep(DefaultVoiceRobustnessSweepConfig())
	if err != nil {
		t.Fatalf("RunVoiceRobustnessSweep failed: %v", err)
	}
	row := mustFindVoiceRobustnessRow(t, report.Rows, "burst_250_protect_35_jitter_400_severe_noise_loss_0_burst_0")
	if row.Outcome != VoiceRobustnessOutcomeDegraded {
		t.Fatalf("severe noise outcome = %q, want degraded", row.Outcome)
	}
	if row.ImprovedFromBaseline {
		t.Fatal("severe noise with no packet/burst loss unexpectedly improved")
	}
	if !containsString(row.ResidualRisk, "severe_snr_not_solved_by_packet_protection") {
		t.Fatalf("ResidualRisk = %v, want severe SNR risk", row.ResidualRisk)
	}
	if !containsString(row.DegradeReasons, "snr_residual_risk") {
		t.Fatalf("DegradeReasons = %v, want SNR residual reason", row.DegradeReasons)
	}
}

func TestIT035_WeakFixtureEvidenceCannotPass(t *testing.T) {
	cfg := DefaultVoiceRobustnessSweepConfig()
	cfg.Fixture.EvidenceGrade = EvidenceGradeSyntheticProxy
	cfg.Fixture.ArtifactSHA256 = ""
	cfg.NoiseProfiles = []AudioIntelligibilityNoiseProfile{{NoiseProfileID: "clean", NoiseSNRDb: 40}}
	cfg.BurstMsValues = []int{250}
	cfg.ProtectionOverheadPct = []int{35}
	cfg.JitterBufferMsValues = []int{400}
	cfg.PacketLossPctValues = []int{0}
	cfg.BurstLossMsValues = []int{0}
	report, err := RunVoiceRobustnessSweep(cfg)
	if err != nil {
		t.Fatalf("RunVoiceRobustnessSweep failed: %v", err)
	}
	if report.GoodVoicePassRows != 0 {
		t.Fatalf("GoodVoicePassRows = %d, want 0 for synthetic fixture evidence", report.GoodVoicePassRows)
	}
	row := report.Rows[0]
	if row.Outcome == VoiceRobustnessOutcomeGoodVoicePass {
		t.Fatalf("synthetic evidence row got forbidden good_voice_pass: %+v", row)
	}
	if !containsString(row.DegradeReasons, "insufficient_fixture_artifact") {
		t.Fatalf("DegradeReasons = %v, want insufficient fixture artifact", row.DegradeReasons)
	}
	if report.Evidence.Outcome == VoiceRobustnessOutcomeGoodVoicePass {
		t.Fatalf("synthetic evidence report got forbidden good_voice_pass: %+v", report.Evidence)
	}
}

func TestIT035_RecommendedProfilesAreLatencyBounded(t *testing.T) {
	report, err := RunVoiceRobustnessSweep(DefaultVoiceRobustnessSweepConfig())
	if err != nil {
		t.Fatalf("RunVoiceRobustnessSweep failed: %v", err)
	}
	if len(report.RecommendedProfiles) != 3 {
		t.Fatalf("recommended profile count = %d, want 3", len(report.RecommendedProfiles))
	}
	for _, profile := range report.RecommendedProfiles {
		if profile.FirstAudioLatencyMs > report.LatencyBudgetMs || profile.CompletionLatencyMs > report.CompletionLatencyMaxMs {
			t.Fatalf("recommended profile %+v exceeds latency budgets %d/%d",
				profile, report.LatencyBudgetMs, report.CompletionLatencyMaxMs)
		}
	}
	robust := mustFindVoiceRobustnessRecommendation(t, report.RecommendedProfiles, VoiceRobustnessProfileRobustDelayed)
	if robust.Outcome != VoiceRobustnessOutcomeDegraded {
		t.Fatalf("robust recommended outcome = %q, want degraded because severe rows remain", robust.Outcome)
	}
	if robust.ProtectionOverheadPct != 35 || robust.JitterBufferMs != 400 {
		t.Fatalf("robust recommendation knobs = protection %d jitter %d, want 35/400",
			robust.ProtectionOverheadPct, robust.JitterBufferMs)
	}
	if robust.GoodVoicePassRows != 18 || robust.DegradedRows != 15 || robust.FailRows != 3 || robust.ImprovedRows != 32 {
		t.Fatalf("robust recommendation rows = good/degraded/fail/improved %d/%d/%d/%d, want 18/15/3/32",
			robust.GoodVoicePassRows, robust.DegradedRows, robust.FailRows, robust.ImprovedRows)
	}
	if !robust.ResidualDegradationStillPossible {
		t.Fatal("robust recommendation should keep residual degradation visible")
	}
	protected := mustFindVoiceRobustnessRecommendation(t, report.RecommendedProfiles, VoiceRobustnessProfileProtected)
	if protected.ProtectionOverheadPct != 25 || protected.JitterBufferMs != 250 {
		t.Fatalf("protected recommendation knobs = protection %d jitter %d, want 25/250",
			protected.ProtectionOverheadPct, protected.JitterBufferMs)
	}
	if protected.GoodVoicePassRows != 13 || protected.DegradedRows != 17 || protected.FailRows != 6 || protected.ImprovedRows != 32 {
		t.Fatalf("protected recommendation rows = good/degraded/fail/improved %d/%d/%d/%d, want 13/17/6/32",
			protected.GoodVoicePassRows, protected.DegradedRows, protected.FailRows, protected.ImprovedRows)
	}
}

func TestIT035_RecommendationsUseFixedStoryLatencyCaps(t *testing.T) {
	cfg := DefaultVoiceRobustnessSweepConfig()
	cfg.TurnaroundMs = 2000
	cfg.LatencyBudgetMs = 10000
	cfg.CompletionLatencyMaxMs = 10000
	report, err := RunVoiceRobustnessSweep(cfg)
	if err != nil {
		t.Fatalf("RunVoiceRobustnessSweep failed: %v", err)
	}
	for _, profile := range report.RecommendedProfiles {
		if profile.FirstAudioLatencyMs > voiceRobustnessDefaultLatencyBudgetMs ||
			profile.CompletionLatencyMs > voiceRobustnessDefaultCompletionMs {
			t.Fatalf("recommendation exceeded fixed story caps despite relaxed config: %+v", profile)
		}
	}
}

func TestIT035_ReportOutcomePassRequiresAllRowsPass(t *testing.T) {
	cfg := DefaultVoiceRobustnessSweepConfig()
	cfg.NoiseProfiles = []AudioIntelligibilityNoiseProfile{{NoiseProfileID: "clean", NoiseSNRDb: 40}}
	cfg.BurstMsValues = []int{250}
	cfg.ProtectionOverheadPct = []int{15}
	cfg.JitterBufferMsValues = []int{160}
	cfg.PacketLossPctValues = []int{0}
	cfg.BurstLossMsValues = []int{0}
	report, err := RunVoiceRobustnessSweep(cfg)
	if err != nil {
		t.Fatalf("RunVoiceRobustnessSweep failed: %v", err)
	}
	if report.Evidence.Outcome != VoiceRobustnessOutcomeGoodVoicePass {
		t.Fatalf("single passing row evidence outcome = %q, want good_voice_pass", report.Evidence.Outcome)
	}

	cfg.NoiseProfiles = []AudioIntelligibilityNoiseProfile{{NoiseProfileID: "severe", NoiseSNRDb: -20}}
	cfg.PacketLossPctValues = []int{100}
	cfg.BurstLossMsValues = []int{1000}
	failed, err := RunVoiceRobustnessSweep(cfg)
	if err != nil {
		t.Fatalf("failed RunVoiceRobustnessSweep failed: %v", err)
	}
	if failed.Evidence.Outcome != VoiceRobustnessOutcomeFail {
		t.Fatalf("single failed row evidence outcome = %q, want fail", failed.Evidence.Outcome)
	}
}

func TestVoiceRobustnessRejectsInvalidConfig(t *testing.T) {
	valid := func() VoiceRobustnessSweepConfig {
		return DefaultVoiceRobustnessSweepConfig()
	}
	tests := []struct {
		name string
		cfg  VoiceRobustnessSweepConfig
	}{
		{name: "unsupported scenario", cfg: func() VoiceRobustnessSweepConfig {
			cfg := valid()
			cfg.ScenarioID = "other"
			return cfg
		}()},
		{name: "duplicate protection", cfg: func() VoiceRobustnessSweepConfig {
			cfg := valid()
			cfg.ProtectionOverheadPct = []int{15, 15}
			return cfg
		}()},
		{name: "negative jitter", cfg: func() VoiceRobustnessSweepConfig {
			cfg := valid()
			cfg.JitterBufferMsValues = []int{-1}
			return cfg
		}()},
		{name: "empty noise profiles", cfg: func() VoiceRobustnessSweepConfig {
			cfg := valid()
			cfg.NoiseProfiles = []AudioIntelligibilityNoiseProfile{}
			return cfg
		}()},
		{name: "bad threshold", cfg: func() VoiceRobustnessSweepConfig {
			cfg := valid()
			cfg.GoodVoiceThreshold = 101
			return cfg
		}()},
		{name: "bad scoring", cfg: func() VoiceRobustnessSweepConfig {
			cfg := valid()
			cfg.ScoringMethod = "mos_guess"
			return cfg
		}()},
		{name: "row cap", cfg: func() VoiceRobustnessSweepConfig {
			cfg := valid()
			cfg.BurstMsValues = []int{100, 200, 300, 400, 500}
			cfg.ProtectionOverheadPct = []int{0, 10, 20, 30, 40}
			cfg.JitterBufferMsValues = []int{0, 100, 200, 300, 400}
			cfg.PacketLossPctValues = []int{0, 5, 10, 15, 20}
			cfg.BurstLossMsValues = []int{0, 50, 100, 150, 200}
			return cfg
		}()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := RunVoiceRobustnessSweep(tc.cfg); err == nil {
				t.Fatal("RunVoiceRobustnessSweep accepted invalid config")
			}
		})
	}
}

func mustFindVoiceRobustnessRow(t *testing.T, rows []VoiceRobustnessSweepRow, id string) VoiceRobustnessSweepRow {
	t.Helper()
	for _, row := range rows {
		if row.RowID == id {
			return row
		}
	}
	t.Fatalf("missing voice robustness row %q", id)
	return VoiceRobustnessSweepRow{}
}

func mustFindVoiceRobustnessRecommendation(t *testing.T, profiles []VoiceRobustnessRecommendedProfile, class string) VoiceRobustnessRecommendedProfile {
	t.Helper()
	for _, profile := range profiles {
		if profile.ProfileClass == class {
			return profile
		}
	}
	t.Fatalf("missing voice robustness recommendation %q", class)
	return VoiceRobustnessRecommendedProfile{}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
