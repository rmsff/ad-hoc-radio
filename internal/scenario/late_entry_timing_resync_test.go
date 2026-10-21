package scenario

import (
	"reflect"
	"testing"
)

func TestIT038_LateEntryTimingResyncDefaultReportDeterministic(t *testing.T) {
	first, err := RunLateEntryTimingResyncScenario(DefaultLateEntryTimingResyncConfig())
	if err != nil {
		t.Fatalf("RunLateEntryTimingResyncScenario failed: %v", err)
	}
	second, err := RunLateEntryTimingResyncScenario(DefaultLateEntryTimingResyncConfig())
	if err != nil {
		t.Fatalf("second RunLateEntryTimingResyncScenario failed: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("default late-entry report is not deterministic")
	}
	if first.ScenarioID != LateEntryTimingResyncScenarioID {
		t.Fatalf("ScenarioID = %q, want %q", first.ScenarioID, LateEntryTimingResyncScenarioID)
	}
	if first.TimingSourceType != LateEntryTimingSourceScenarioMetadata {
		t.Fatalf("TimingSourceType = %q, want scenario metadata", first.TimingSourceType)
	}
	if first.ReceiverCount != 5 || len(first.Receivers) != 5 {
		t.Fatalf("receiver count = %d/%d, want 5", first.ReceiverCount, len(first.Receivers))
	}
	if first.NormalReceiverCount != 1 || first.AcquiredReceiverCount != 1 ||
		first.DegradedReceiverCount != 2 || first.UncoveredReceiverCount != 1 {
		t.Fatalf("normal/acquired/degraded/uncovered = %d/%d/%d/%d, want 1/1/2/1",
			first.NormalReceiverCount, first.AcquiredReceiverCount, first.DegradedReceiverCount, first.UncoveredReceiverCount)
	}
	if first.MissedCallStartCount != 4 || first.ResyncEvents != 3 || first.MissedSyncCount != 7 {
		t.Fatalf("missed call_start/resync/missed_sync = %d/%d/%d, want 4/3/7",
			first.MissedCallStartCount, first.ResyncEvents, first.MissedSyncCount)
	}
	if first.GuardExhaustedCount != 1 || first.DriftCollisionCount != 1 {
		t.Fatalf("guard/drift counters = %d/%d, want 1/1", first.GuardExhaustedCount, first.DriftCollisionCount)
	}
	if first.MaxAcquisitionMs != 700 {
		t.Fatalf("MaxAcquisitionMs = %d, want 700", first.MaxAcquisitionMs)
	}
	if first.Outcome != LateEntryOutcomeDegraded {
		t.Fatalf("Outcome = %q, want %q; reasons=%v", first.Outcome, LateEntryOutcomeDegraded, first.DegradeReasons)
	}
	if first.EventHash == "" || first.EventHash != first.Evidence.Determinism.EventHash {
		t.Fatalf("event hash mismatch: report=%q evidence=%q", first.EventHash, first.Evidence.Determinism.EventHash)
	}
	if first.EventHash != "dbb316e47845e266f127b3b5889e24584227dfa841825d8d1311f19d5050b0e1" {
		t.Fatalf("EventHash = %q, want pinned default hash", first.EventHash)
	}
	if first.Evidence.HardwareClaimAllowed {
		t.Fatal("late-entry report unexpectedly allows hardware claims")
	}
	if !containsString(first.Evidence.AssumptionFlags, "not_real_rf_synchronization") ||
		!containsString(first.Evidence.AssumptionFlags, "no_new_wire_control_frame") {
		t.Fatalf("AssumptionFlags = %v, want no RF sync/no wire-frame flags", first.Evidence.AssumptionFlags)
	}
	if err := ValidateScenarioEvidence(first.Evidence); err != nil {
		t.Fatalf("ValidateScenarioEvidence failed: %v", err)
	}
}

func TestIT038_MissedCallStartCannotBeNormalReceiver(t *testing.T) {
	report, err := RunLateEntryTimingResyncScenario(DefaultLateEntryTimingResyncConfig())
	if err != nil {
		t.Fatalf("RunLateEntryTimingResyncScenario failed: %v", err)
	}
	for _, row := range report.Receivers {
		if !row.HasCallStart && row.Classification == LateEntryClassificationNormal {
			t.Fatalf("receiver %d missed call_start but was classified normal: %+v", row.ReceiverID, row)
		}
	}
	acquired := mustFindLateEntryReceiver(t, report.Receivers, 2)
	if acquired.Classification != LateEntryClassificationAcquired || acquired.AcquisitionMs != 120 || acquired.AudioStartMs != 420 {
		t.Fatalf("receiver 2 = %+v, want acquired at 120/420", acquired)
	}
	if !containsLateEntryReason(acquired.DegradeReasons, "missed_call_start") {
		t.Fatalf("receiver 2 reasons = %v, want missed_call_start", acquired.DegradeReasons)
	}
}

func TestIT038_LateReceiverOutcomesAreExplicit(t *testing.T) {
	report, err := RunLateEntryTimingResyncScenario(DefaultLateEntryTimingResyncConfig())
	if err != nil {
		t.Fatalf("RunLateEntryTimingResyncScenario failed: %v", err)
	}
	degraded := mustFindLateEntryReceiver(t, report.Receivers, 3)
	if degraded.Classification != LateEntryClassificationDegraded || degraded.AcquisitionMs != 700 {
		t.Fatalf("receiver 3 = %+v, want degraded over budget", degraded)
	}
	if !containsLateEntryReason(degraded.DegradeReasons, "acquisition_budget_exceeded") {
		t.Fatalf("receiver 3 reasons = %v, want acquisition budget reason", degraded.DegradeReasons)
	}
	drift := mustFindLateEntryReceiver(t, report.Receivers, 4)
	if drift.Classification != LateEntryClassificationDegraded || drift.GuardExhaustedCount != 1 || drift.DriftCollisionCount != 1 {
		t.Fatalf("receiver 4 = %+v, want drift/guard degraded", drift)
	}
	uncovered := mustFindLateEntryReceiver(t, report.Receivers, 5)
	if uncovered.Classification != LateEntryClassificationUncovered || uncovered.AcquisitionMs != -1 || uncovered.AudioStartMs != -1 {
		t.Fatalf("receiver 5 = %+v, want uncovered without acquisition", uncovered)
	}
}

func TestIT038_AllNormalReceiversPass(t *testing.T) {
	cfg := DefaultLateEntryTimingResyncConfig()
	cfg.Receivers = []LateEntryReceiverConfig{
		{ReceiverID: 8, CoverageEnterMs: 0, HasCallStart: true, TimingHintAvailable: true},
		{ReceiverID: 9, CoverageEnterMs: 0, HasCallStart: true, TimingHintAvailable: true},
	}
	report, err := RunLateEntryTimingResyncScenario(cfg)
	if err != nil {
		t.Fatalf("RunLateEntryTimingResyncScenario failed: %v", err)
	}
	if report.Outcome != LateEntryOutcomePass {
		t.Fatalf("Outcome = %q, want pass; reasons=%v", report.Outcome, report.DegradeReasons)
	}
	if report.MissedCallStartCount != 0 || report.ResyncEvents != 0 {
		t.Fatalf("missed/resync = %d/%d, want 0/0", report.MissedCallStartCount, report.ResyncEvents)
	}
}

func TestLateEntryTimingResyncRejectsInvalidConfig(t *testing.T) {
	valid := func() LateEntryTimingResyncConfig {
		return DefaultLateEntryTimingResyncConfig()
	}
	tests := []struct {
		name string
		cfg  LateEntryTimingResyncConfig
	}{
		{name: "unsupported scenario", cfg: func() LateEntryTimingResyncConfig {
			cfg := valid()
			cfg.ScenarioID = "other"
			return cfg
		}()},
		{name: "unsupported timing source", cfg: func() LateEntryTimingResyncConfig {
			cfg := valid()
			cfg.TimingSourceType = "wire_frame"
			return cfg
		}()},
		{name: "bad call window", cfg: func() LateEntryTimingResyncConfig {
			cfg := valid()
			cfg.CallStartMs = 1000
			cfg.CallEndMs = 999
			return cfg
		}()},
		{name: "empty receivers", cfg: func() LateEntryTimingResyncConfig {
			cfg := valid()
			cfg.Receivers = []LateEntryReceiverConfig{}
			return cfg
		}()},
		{name: "duplicate receiver", cfg: func() LateEntryTimingResyncConfig {
			cfg := valid()
			cfg.Receivers[1].ReceiverID = cfg.Receivers[0].ReceiverID
			return cfg
		}()},
		{name: "negative timing", cfg: func() LateEntryTimingResyncConfig {
			cfg := valid()
			cfg.Receivers[0].CoverageEnterMs = -1
			return cfg
		}()},
		{name: "call start after call window start", cfg: func() LateEntryTimingResyncConfig {
			cfg := valid()
			cfg.Receivers[1].HasCallStart = true
			cfg.Receivers[1].CoverageEnterMs = cfg.CallStartMs + 1
			return cfg
		}()},
		{name: "drift overflow", cfg: func() LateEntryTimingResyncConfig {
			cfg := valid()
			cfg.Receivers[0].DriftPPM = 9223372036854775807
			cfg.Receivers[0].DriftObservationMs = 2
			return cfg
		}()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := RunLateEntryTimingResyncScenario(tc.cfg); err == nil {
				t.Fatal("RunLateEntryTimingResyncScenario accepted invalid config")
			}
		})
	}
}

func mustFindLateEntryReceiver(t *testing.T, rows []LateEntryReceiverRow, receiverID uint16) LateEntryReceiverRow {
	t.Helper()
	for _, row := range rows {
		if row.ReceiverID == receiverID {
			return row
		}
	}
	t.Fatalf("receiver %d not found in %+v", receiverID, rows)
	return LateEntryReceiverRow{}
}
