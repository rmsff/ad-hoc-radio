package scenario

import (
	"math"
	"reflect"
	"testing"
)

func TestIT036_DelayedGroupConversationDefaultReportDeterministic(t *testing.T) {
	first, err := RunDelayedGroupConversationScenario(DefaultDelayedGroupConversationConfig())
	if err != nil {
		t.Fatalf("RunDelayedGroupConversationScenario failed: %v", err)
	}
	second, err := RunDelayedGroupConversationScenario(DefaultDelayedGroupConversationConfig())
	if err != nil {
		t.Fatalf("second RunDelayedGroupConversationScenario failed: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("default delayed group conversation report is not deterministic")
	}
	if first.ScenarioID != DelayedGroupConversationScenarioID {
		t.Fatalf("ScenarioID = %q, want %q", first.ScenarioID, DelayedGroupConversationScenarioID)
	}
	if len(first.SpeakerIDs) != 3 {
		t.Fatalf("speaker count = %d, want 3", len(first.SpeakerIDs))
	}
	if len(first.Turns) != 5 {
		t.Fatalf("turn count = %d, want 5", len(first.Turns))
	}
	if len(first.ExpectedReceiverIDs) != 9 {
		t.Fatalf("expected receivers = %v, want 9 ids", first.ExpectedReceiverIDs)
	}
	if first.RowCount != 3 || len(first.Rows) != 3 {
		t.Fatalf("row count = %d/%d, want 3", first.RowCount, len(first.Rows))
	}
	if first.PassRows != 3 || first.DegradedRows != 0 || first.FailRows != 0 {
		t.Fatalf("pass/degraded/fail rows = %d/%d/%d, want 3/0/0", first.PassRows, first.DegradedRows, first.FailRows)
	}
	if first.Outcome != DelayedGroupConversationOutcomePass {
		t.Fatalf("report outcome = %q, want %q; reasons=%v", first.Outcome, DelayedGroupConversationOutcomePass, first.DegradeReasons)
	}
	if first.EventHash == "" || first.EventHash != first.Evidence.Determinism.EventHash {
		t.Fatalf("event hash mismatch: report=%q evidence=%q", first.EventHash, first.Evidence.Determinism.EventHash)
	}
	if first.EventHash != "b4ee277b1becb62de4e1030e5e766a35a722c3cfb9ecb600cfc7511fc250261b" {
		t.Fatalf("EventHash = %q, want pinned default hash", first.EventHash)
	}
	if first.Evidence.HardwareClaimAllowed {
		t.Fatal("delayed group conversation unexpectedly allows hardware claims")
	}
	if err := ValidateScenarioEvidence(first.Evidence); err != nil {
		t.Fatalf("ValidateScenarioEvidence failed: %v", err)
	}
}

func TestIT036_DelayedGroupConversationDepthSummaries(t *testing.T) {
	report, err := RunDelayedGroupConversationScenario(DefaultDelayedGroupConversationConfig())
	if err != nil {
		t.Fatalf("RunDelayedGroupConversationScenario failed: %v", err)
	}
	row := mustFindDelayedGroupConversationRow(t, report.Rows, "burst_250_protect_35_jitter_400")
	if row.MaxFirstAudioLatencyMs != 1437 || row.MaxCompletionLatencyMs != 1682 {
		t.Fatalf("robust max latency = %d/%d, want 1437/1682", row.MaxFirstAudioLatencyMs, row.MaxCompletionLatencyMs)
	}
	direct := mustFindDelayedGroupConversationGroup(t, row.ReceiverGroups, "direct")
	if direct.RelayDepth != 0 || direct.ReceiverCount != 3 {
		t.Fatalf("direct group = %+v, want depth 0 count 3", direct)
	}
	oneHop := mustFindDelayedGroupConversationGroup(t, row.ReceiverGroups, "one_hop")
	if oneHop.RelayDepth != 1 || oneHop.ReceiverCount != 2 {
		t.Fatalf("one-hop group = %+v, want depth 1 count 2", oneHop)
	}
	twoHop := mustFindDelayedGroupConversationGroup(t, row.ReceiverGroups, "two_hop")
	if twoHop.RelayDepth != 2 || twoHop.ReceiverCount != 4 {
		t.Fatalf("two-hop group = %+v, want depth 2 count 4", twoHop)
	}
	if twoHop.FirstAudioLatencyMs != 1437 || twoHop.CompletionLatencyMs != 1682 {
		t.Fatalf("two-hop latency = %d/%d, want 1437/1682", twoHop.FirstAudioLatencyMs, twoHop.CompletionLatencyMs)
	}
	if twoHop.StalePlayoutCount != 0 || twoHop.OverlapCount != 0 || twoHop.MissingUncoveredReceiverCount != 0 {
		t.Fatalf("two-hop counters = stale/overlap/missing %d/%d/%d, want 0/0/0",
			twoHop.StalePlayoutCount, twoHop.OverlapCount, twoHop.MissingUncoveredReceiverCount)
	}
}

func TestIT036_DefaultProfilesMatchVoiceRobustnessRecommendations(t *testing.T) {
	robustness, err := RunVoiceRobustnessSweep(DefaultVoiceRobustnessSweepConfig())
	if err != nil {
		t.Fatalf("RunVoiceRobustnessSweep failed: %v", err)
	}
	defaults := defaultDelayedGroupConversationProfiles()
	if len(defaults) != len(robustness.RecommendedProfiles) {
		t.Fatalf("default profile count = %d, recommendations = %d", len(defaults), len(robustness.RecommendedProfiles))
	}
	for i, profile := range defaults {
		recommendation := robustness.RecommendedProfiles[i]
		if profile.ProfileClass != recommendation.ProfileClass ||
			profile.ProfileID != recommendation.ProfileID ||
			profile.BurstMs != recommendation.BurstMs ||
			profile.ProtectionOverheadPct != recommendation.ProtectionOverheadPct ||
			profile.JitterBufferMs != recommendation.JitterBufferMs {
			t.Fatalf("default profile[%d] = %+v, recommendation = %+v", i, profile, recommendation)
		}
	}
}

func TestIT036_DelayedPlaybackOverlapDegradesConversation(t *testing.T) {
	cfg := DefaultDelayedGroupConversationConfig()
	cfg.TalkTurns = []DelayedGroupConversationTurnConfig{
		{TurnID: "turn_1", SpeakerID: 1, StartMs: 0},
		{TurnID: "turn_2", SpeakerID: 11, StartMs: 1000},
		{TurnID: "turn_3", SpeakerID: 12, StartMs: 2000},
		{TurnID: "turn_4", SpeakerID: 1, StartMs: 3000},
		{TurnID: "turn_5", SpeakerID: 11, StartMs: 4000},
	}
	cfg.Profiles = []DelayedGroupConversationProfile{defaultDelayedGroupConversationProfiles()[2]}
	report, err := RunDelayedGroupConversationScenario(cfg)
	if err != nil {
		t.Fatalf("RunDelayedGroupConversationScenario failed: %v", err)
	}
	if report.Outcome != DelayedGroupConversationOutcomeDegraded {
		t.Fatalf("report outcome = %q, want degraded", report.Outcome)
	}
	row := report.Rows[0]
	if row.StalePlayoutCount == 0 || row.OverlapCount == 0 {
		t.Fatalf("stale/overlap = %d/%d, want both >0", row.StalePlayoutCount, row.OverlapCount)
	}
	if row.Outcome != DelayedGroupConversationOutcomeDegraded {
		t.Fatalf("row outcome = %q, want degraded", row.Outcome)
	}
	if !containsString(row.DegradeReasons, "stale_playout_over_next_turn") ||
		!containsString(row.DegradeReasons, "delayed_playback_overlaps_later_talk_turn") {
		t.Fatalf("row reasons = %v, want stale and overlap reasons", row.DegradeReasons)
	}
}

func TestIT036_UncoveredReceiversPreventPass(t *testing.T) {
	cfg := DefaultDelayedGroupConversationConfig()
	cfg.UncoveredReceiverIDs = []uint16{99}
	report, err := RunDelayedGroupConversationScenario(cfg)
	if err != nil {
		t.Fatalf("RunDelayedGroupConversationScenario failed: %v", err)
	}
	if report.Outcome != DelayedGroupConversationOutcomeDegraded {
		t.Fatalf("report outcome = %q, want degraded", report.Outcome)
	}
	row := mustFindDelayedGroupConversationRow(t, report.Rows, "burst_250_protect_25_jitter_250")
	if row.ExpectedReceiverCount != 10 || row.AccountedReceiverCount != 9 || row.MissingUncoveredReceiverCount != 1 {
		t.Fatalf("receiver accounting expected/accounted/missing = %d/%d/%d, want 10/9/1",
			row.ExpectedReceiverCount, row.AccountedReceiverCount, row.MissingUncoveredReceiverCount)
	}
	if !containsString(row.DegradeReasons, "missing_uncovered_receivers") {
		t.Fatalf("row reasons = %v, want missing uncovered reason", row.DegradeReasons)
	}
	uncovered := mustFindDelayedGroupConversationGroup(t, row.ReceiverGroups, "uncovered")
	if uncovered.MissingUncoveredReceiverCount != 1 || uncovered.AccountedReceiverCount != 0 {
		t.Fatalf("uncovered group = %+v, want missing 1 accounted 0", uncovered)
	}
}

func TestIT036_PassUsesFixedStoryLatencyCap(t *testing.T) {
	cfg := DefaultDelayedGroupConversationConfig()
	cfg.TurnaroundMs = 1000
	cfg.MaxFirstAudioLatencyMs = 10000
	cfg.MaxCompletionMs = 10000
	cfg.Profiles = []DelayedGroupConversationProfile{defaultDelayedGroupConversationProfiles()[2]}
	report, err := RunDelayedGroupConversationScenario(cfg)
	if err != nil {
		t.Fatalf("RunDelayedGroupConversationScenario failed: %v", err)
	}
	row := report.Rows[0]
	if row.MaxFirstAudioLatencyMs <= delayedGroupConversationDefaultFirstLatency {
		t.Fatalf("test setup max first latency = %d, want > fixed cap %d",
			row.MaxFirstAudioLatencyMs, delayedGroupConversationDefaultFirstLatency)
	}
	if row.Outcome == DelayedGroupConversationOutcomePass {
		t.Fatalf("row got pass despite exceeding fixed story first-audio cap: %+v", row)
	}
	if !containsString(row.DegradeReasons, "first_audio_latency_budget_exceeded") {
		t.Fatalf("row reasons = %v, want first-audio cap reason", row.DegradeReasons)
	}
}

func TestIT036_ReportOutcomeDoesNotOverclaim(t *testing.T) {
	cfg := DefaultDelayedGroupConversationConfig()
	cfg.Profiles = []DelayedGroupConversationProfile{defaultDelayedGroupConversationProfiles()[0]}
	report, err := RunDelayedGroupConversationScenario(cfg)
	if err != nil {
		t.Fatalf("RunDelayedGroupConversationScenario failed: %v", err)
	}
	if report.Outcome != DelayedGroupConversationOutcomePass {
		t.Fatalf("single passing profile outcome = %q, want pass", report.Outcome)
	}

	cfg.MaxCompletionMs = 100
	failed, err := RunDelayedGroupConversationScenario(cfg)
	if err != nil {
		t.Fatalf("failed RunDelayedGroupConversationScenario failed: %v", err)
	}
	if failed.Outcome != DelayedGroupConversationOutcomeFail {
		t.Fatalf("all-fail report outcome = %q, want fail", failed.Outcome)
	}
}

func TestDelayedGroupConversationRejectsInvalidConfig(t *testing.T) {
	valid := func() DelayedGroupConversationConfig {
		return DefaultDelayedGroupConversationConfig()
	}
	tests := []struct {
		name string
		cfg  DelayedGroupConversationConfig
	}{
		{name: "unsupported scenario", cfg: func() DelayedGroupConversationConfig {
			cfg := valid()
			cfg.ScenarioID = "other"
			return cfg
		}()},
		{name: "too few speakers", cfg: func() DelayedGroupConversationConfig {
			cfg := valid()
			cfg.SpeakerIDs = []uint16{1, 2}
			return cfg
		}()},
		{name: "too few turns", cfg: func() DelayedGroupConversationConfig {
			cfg := valid()
			cfg.TalkTurns = cfg.TalkTurns[:4]
			return cfg
		}()},
		{name: "turn speaker missing", cfg: func() DelayedGroupConversationConfig {
			cfg := valid()
			cfg.TalkTurns[0].SpeakerID = 99
			return cfg
		}()},
		{name: "turns use too few distinct speakers", cfg: func() DelayedGroupConversationConfig {
			cfg := valid()
			for i := range cfg.TalkTurns {
				cfg.TalkTurns[i].SpeakerID = 1
			}
			return cfg
		}()},
		{name: "declared speaker unused", cfg: func() DelayedGroupConversationConfig {
			cfg := valid()
			cfg.SpeakerIDs = []uint16{1, 11, 12, 13}
			return cfg
		}()},
		{name: "receiver duplicated across groups", cfg: func() DelayedGroupConversationConfig {
			cfg := valid()
			cfg.OneHopReceiverIDs = []uint16{8}
			return cfg
		}()},
		{name: "empty two-hop group", cfg: func() DelayedGroupConversationConfig {
			cfg := valid()
			cfg.TwoHopReceiverIDs = []uint16{}
			return cfg
		}()},
		{name: "bad profile", cfg: func() DelayedGroupConversationConfig {
			cfg := valid()
			cfg.Profiles[0].JitterBufferMs = -1
			return cfg
		}()},
		{name: "negative timing", cfg: func() DelayedGroupConversationConfig {
			cfg := valid()
			cfg.TurnaroundMs = -1
			return cfg
		}()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := RunDelayedGroupConversationScenario(tc.cfg); err == nil {
				t.Fatal("RunDelayedGroupConversationScenario accepted invalid config")
			}
		})
	}
}

func TestDelayedGroupConversationRejectsPlaybackWindowOverflow(t *testing.T) {
	cfg := DefaultDelayedGroupConversationConfig()
	cfg.TalkTurns = []DelayedGroupConversationTurnConfig{
		{TurnID: "turn_1", SpeakerID: 1, StartMs: math.MaxInt64 - 5000},
		{TurnID: "turn_2", SpeakerID: 11, StartMs: math.MaxInt64 - 4000},
		{TurnID: "turn_3", SpeakerID: 12, StartMs: math.MaxInt64 - 3000},
		{TurnID: "turn_4", SpeakerID: 1, StartMs: math.MaxInt64 - 2000},
		{TurnID: "turn_5", SpeakerID: 11, StartMs: math.MaxInt64 - 100},
	}
	if _, err := RunDelayedGroupConversationScenario(cfg); err == nil {
		t.Fatal("RunDelayedGroupConversationScenario accepted overflowing playback windows")
	}
}

func mustFindDelayedGroupConversationRow(t *testing.T, rows []DelayedGroupConversationProfileRow, profileID string) DelayedGroupConversationProfileRow {
	t.Helper()
	for _, row := range rows {
		if row.ProfileID == profileID {
			return row
		}
	}
	t.Fatalf("missing delayed group conversation row %q", profileID)
	return DelayedGroupConversationProfileRow{}
}

func mustFindDelayedGroupConversationGroup(t *testing.T, groups []DelayedGroupConversationReceiverGroup, label string) DelayedGroupConversationReceiverGroup {
	t.Helper()
	for _, group := range groups {
		if group.DepthLabel == label {
			return group
		}
	}
	t.Fatalf("missing delayed group conversation group %q", label)
	return DelayedGroupConversationReceiverGroup{}
}
