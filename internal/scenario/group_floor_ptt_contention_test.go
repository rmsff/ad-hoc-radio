package scenario

import (
	"reflect"
	"testing"
)

func TestIT037_GroupFloorPTTDefaultReportDeterministic(t *testing.T) {
	first, err := RunGroupFloorPTTContentionScenario(DefaultGroupFloorPTTContentionConfig())
	if err != nil {
		t.Fatalf("RunGroupFloorPTTContentionScenario failed: %v", err)
	}
	second, err := RunGroupFloorPTTContentionScenario(DefaultGroupFloorPTTContentionConfig())
	if err != nil {
		t.Fatalf("second RunGroupFloorPTTContentionScenario failed: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("default group floor PTT report is not deterministic")
	}
	if first.ScenarioID != GroupFloorPTTContentionScenarioID {
		t.Fatalf("ScenarioID = %q, want %q", first.ScenarioID, GroupFloorPTTContentionScenarioID)
	}
	if first.FloorPolicy != GroupFloorPolicyFirstArrivalWins {
		t.Fatalf("FloorPolicy = %q, want %q", first.FloorPolicy, GroupFloorPolicyFirstArrivalWins)
	}
	if len(first.Attempts) != 6 {
		t.Fatalf("attempt count = %d, want 6", len(first.Attempts))
	}
	if first.GrantedCount != 3 || first.BusyCount != 3 || first.DeniedDueBusyCount != 2 || first.CollisionCount != 1 {
		t.Fatalf("granted/busy/denied/collision = %d/%d/%d/%d, want 3/3/2/1",
			first.GrantedCount, first.BusyCount, first.DeniedDueBusyCount, first.CollisionCount)
	}
	if first.ClippedFirstAudioMs != 60 || first.MaxClippedFirstAudioMs != 20 {
		t.Fatalf("clipped total/max = %d/%d, want 60/20", first.ClippedFirstAudioMs, first.MaxClippedFirstAudioMs)
	}
	if first.AccessLatencyP50Ms != 20 || first.AccessLatencyP95Ms != 20 {
		t.Fatalf("access p50/p95 = %d/%d, want 20/20", first.AccessLatencyP50Ms, first.AccessLatencyP95Ms)
	}
	if first.DuplicatePlaybackCount != 0 {
		t.Fatalf("DuplicatePlaybackCount = %d, want 0", first.DuplicatePlaybackCount)
	}
	if first.ActiveFloorCount != 3 {
		t.Fatalf("ActiveFloorCount = %d, want 3", first.ActiveFloorCount)
	}
	if first.Outcome != GroupFloorPTTReportOutcomeDegraded {
		t.Fatalf("Outcome = %q, want %q", first.Outcome, GroupFloorPTTReportOutcomeDegraded)
	}
	if first.EventHash == "" || first.EventHash != first.Evidence.Determinism.EventHash {
		t.Fatalf("event hash mismatch: report=%q evidence=%q", first.EventHash, first.Evidence.Determinism.EventHash)
	}
	if first.EventHash != "43372203df67b1bdf76d6448939865c51269209515a67e8df7418cc15d0fd159" {
		t.Fatalf("EventHash = %q, want pinned default hash", first.EventHash)
	}
	if !containsString(first.DegradeReasons, "ptt_contention_detected") ||
		!containsString(first.DegradeReasons, "denied_due_busy") ||
		!containsString(first.DegradeReasons, "exact_simultaneous_collision") {
		t.Fatalf("DegradeReasons = %v, want contention, busy, and collision reasons", first.DegradeReasons)
	}
	if first.Evidence.HardwareClaimAllowed {
		t.Fatal("group floor report unexpectedly allows hardware claims")
	}
	if err := ValidateScenarioEvidence(first.Evidence); err != nil {
		t.Fatalf("ValidateScenarioEvidence failed: %v", err)
	}
}

func TestIT037_ExactSimultaneousPTTUsesSourceIDTieBreaker(t *testing.T) {
	cfg := DefaultGroupFloorPTTContentionConfig()
	cfg.Attempts = []GroupFloorPTTAttemptConfig{
		{EventID: "z", SourceID: 30, PTTStartMs: 0, TalkDurationMs: 500},
		{EventID: "a", SourceID: 20, PTTStartMs: 0, TalkDurationMs: 500},
		{EventID: "b", SourceID: 20, PTTStartMs: 0, TalkDurationMs: 500},
	}
	report, err := RunGroupFloorPTTContentionScenario(cfg)
	if err != nil {
		t.Fatalf("RunGroupFloorPTTContentionScenario failed: %v", err)
	}
	if report.Attempts[0].EventID != "a" || report.Attempts[0].Outcome != GroupFloorPTTOutcomeGranted {
		t.Fatalf("first sorted attempt = %+v, want event a granted", report.Attempts[0])
	}
	if report.Attempts[1].EventID != "b" || report.Attempts[1].Outcome != GroupFloorPTTOutcomeCollision {
		t.Fatalf("second sorted attempt = %+v, want event b collision", report.Attempts[1])
	}
	if report.Attempts[2].EventID != "z" || report.Attempts[2].Outcome != GroupFloorPTTOutcomeCollision {
		t.Fatalf("third sorted attempt = %+v, want event z collision", report.Attempts[2])
	}
	if report.GrantedCount != 1 || report.CollisionCount != 2 {
		t.Fatalf("granted/collision = %d/%d, want 1/2", report.GrantedCount, report.CollisionCount)
	}
}

func TestIT037_OverlappingPTTDeniedDueBusy(t *testing.T) {
	cfg := DefaultGroupFloorPTTContentionConfig()
	cfg.Attempts = []GroupFloorPTTAttemptConfig{
		{EventID: "grant", SourceID: 1, PTTStartMs: 100, TalkDurationMs: 500},
		{EventID: "busy", SourceID: 2, PTTStartMs: 300, TalkDurationMs: 500},
	}
	report, err := RunGroupFloorPTTContentionScenario(cfg)
	if err != nil {
		t.Fatalf("RunGroupFloorPTTContentionScenario failed: %v", err)
	}
	granted := mustFindGroupFloorPTTRow(t, report.Attempts, "grant")
	busy := mustFindGroupFloorPTTRow(t, report.Attempts, "busy")
	if !granted.SuccessfulSpeaker || granted.Outcome != GroupFloorPTTOutcomeGranted {
		t.Fatalf("grant row = %+v, want successful grant", granted)
	}
	if busy.Outcome != GroupFloorPTTOutcomeDeniedBusy || !busy.Busy || busy.SuccessfulSpeaker {
		t.Fatalf("busy row = %+v, want denied busy non-speaker", busy)
	}
	if busy.BlockedByEventID != "grant" || busy.BlockedBySourceID != 1 {
		t.Fatalf("busy blocker = %q/%d, want grant/1", busy.BlockedByEventID, busy.BlockedBySourceID)
	}
}

func TestIT037_NonOverlappingAttemptsAllPass(t *testing.T) {
	cfg := DefaultGroupFloorPTTContentionConfig()
	cfg.Attempts = []GroupFloorPTTAttemptConfig{
		{EventID: "a", SourceID: 1, PTTStartMs: 0, TalkDurationMs: 200},
		{EventID: "b", SourceID: 2, PTTStartMs: 300, TalkDurationMs: 200},
		{EventID: "c", SourceID: 3, PTTStartMs: 600, TalkDurationMs: 200},
	}
	report, err := RunGroupFloorPTTContentionScenario(cfg)
	if err != nil {
		t.Fatalf("RunGroupFloorPTTContentionScenario failed: %v", err)
	}
	if report.Outcome != GroupFloorPTTReportOutcomePass {
		t.Fatalf("Outcome = %q, want pass; reasons=%v", report.Outcome, report.DegradeReasons)
	}
	if report.GrantedCount != 3 || report.BusyCount != 0 || report.DuplicatePlaybackCount != 0 {
		t.Fatalf("granted/busy/duplicate = %d/%d/%d, want 3/0/0",
			report.GrantedCount, report.BusyCount, report.DuplicatePlaybackCount)
	}
}

func TestGroupFloorPTTRejectsInvalidConfig(t *testing.T) {
	valid := func() GroupFloorPTTContentionConfig {
		return DefaultGroupFloorPTTContentionConfig()
	}
	tests := []struct {
		name string
		cfg  GroupFloorPTTContentionConfig
	}{
		{name: "unsupported scenario", cfg: func() GroupFloorPTTContentionConfig {
			cfg := valid()
			cfg.ScenarioID = "other"
			return cfg
		}()},
		{name: "unsupported policy", cfg: func() GroupFloorPTTContentionConfig {
			cfg := valid()
			cfg.FloorPolicy = "queue"
			return cfg
		}()},
		{name: "too few attempts", cfg: func() GroupFloorPTTContentionConfig {
			cfg := valid()
			cfg.Attempts = []GroupFloorPTTAttemptConfig{{EventID: "a", SourceID: 1, PTTStartMs: 0, TalkDurationMs: 100}}
			return cfg
		}()},
		{name: "duplicate event", cfg: func() GroupFloorPTTContentionConfig {
			cfg := valid()
			cfg.Attempts[1].EventID = cfg.Attempts[0].EventID
			return cfg
		}()},
		{name: "zero source", cfg: func() GroupFloorPTTContentionConfig {
			cfg := valid()
			cfg.Attempts[0].SourceID = 0
			return cfg
		}()},
		{name: "negative start", cfg: func() GroupFloorPTTContentionConfig {
			cfg := valid()
			cfg.Attempts[0].PTTStartMs = -1
			return cfg
		}()},
		{name: "zero duration", cfg: func() GroupFloorPTTContentionConfig {
			cfg := valid()
			cfg.Attempts[0].TalkDurationMs = 0
			return cfg
		}()},
		{name: "negative guard", cfg: func() GroupFloorPTTContentionConfig {
			cfg := valid()
			cfg.FloorReleaseGuardMs = -1
			return cfg
		}()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := RunGroupFloorPTTContentionScenario(tc.cfg); err == nil {
				t.Fatal("RunGroupFloorPTTContentionScenario accepted invalid config")
			}
		})
	}
}

func TestGroupFloorPTTRejectsTimingOverflow(t *testing.T) {
	cfg := DefaultGroupFloorPTTContentionConfig()
	cfg.Attempts = []GroupFloorPTTAttemptConfig{
		{EventID: "a", SourceID: 1, PTTStartMs: 9223372036854775800, TalkDurationMs: 100},
		{EventID: "b", SourceID: 2, PTTStartMs: 9223372036854775801, TalkDurationMs: 100},
	}
	if _, err := RunGroupFloorPTTContentionScenario(cfg); err == nil {
		t.Fatal("RunGroupFloorPTTContentionScenario accepted overflowing timing")
	}
}

func mustFindGroupFloorPTTRow(t *testing.T, rows []GroupFloorPTTAttemptRow, eventID string) GroupFloorPTTAttemptRow {
	t.Helper()
	for _, row := range rows {
		if row.EventID == eventID {
			return row
		}
	}
	t.Fatalf("row %q not found in %+v", eventID, rows)
	return GroupFloorPTTAttemptRow{}
}
