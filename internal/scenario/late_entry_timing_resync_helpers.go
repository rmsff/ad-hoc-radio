package scenario

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

func normalizeLateEntryTimingResyncConfig(cfg LateEntryTimingResyncConfig) LateEntryTimingResyncConfig {
	def := DefaultLateEntryTimingResyncConfig()
	if cfg.ScenarioID == "" {
		cfg.ScenarioID = def.ScenarioID
	}
	if cfg.Seed == 0 {
		cfg.Seed = def.Seed
	}
	if cfg.TimingSourceType == "" {
		cfg.TimingSourceType = def.TimingSourceType
	}
	if cfg.CallEndMs == 0 {
		cfg.CallEndMs = def.CallEndMs
	}
	if cfg.AcquisitionBudgetMs == 0 {
		cfg.AcquisitionBudgetMs = def.AcquisitionBudgetMs
	}
	if cfg.GuardMs == 0 {
		cfg.GuardMs = def.GuardMs
	}
	if cfg.SyncIntervalMs == 0 {
		cfg.SyncIntervalMs = def.SyncIntervalMs
	}
	if cfg.MaxMissedSyncBeforeDegraded == 0 {
		cfg.MaxMissedSyncBeforeDegraded = def.MaxMissedSyncBeforeDegraded
	}
	if cfg.Receivers == nil {
		cfg.Receivers = cloneLateEntryReceiverConfigs(def.Receivers)
	} else {
		cfg.Receivers = cloneLateEntryReceiverConfigs(cfg.Receivers)
	}
	return cfg
}

func validateLateEntryTimingResyncConfig(cfg LateEntryTimingResyncConfig) error {
	if cfg.ScenarioID != LateEntryTimingResyncScenarioID {
		return fmt.Errorf("late entry timing resync: unsupported scenario_id %q", cfg.ScenarioID)
	}
	if cfg.TimingSourceType != LateEntryTimingSourceScenarioMetadata {
		return fmt.Errorf("late entry timing resync: unsupported timing_source_type %q", cfg.TimingSourceType)
	}
	if cfg.CallStartMs < 0 || cfg.CallEndMs <= cfg.CallStartMs {
		return fmt.Errorf("late entry timing resync: call window must be non-negative and increasing")
	}
	if cfg.AcquisitionBudgetMs <= 0 || cfg.SyncIntervalMs <= 0 {
		return fmt.Errorf("late entry timing resync: acquisition_budget_ms and sync_interval_ms must be > 0")
	}
	if cfg.GuardMs < 0 || cfg.MaxMissedSyncBeforeDegraded < 0 {
		return fmt.Errorf("late entry timing resync: guard and missed-sync threshold must be >= 0")
	}
	if len(cfg.Receivers) == 0 || len(cfg.Receivers) > lateEntryMaxReceivers {
		return fmt.Errorf("late entry timing resync: receiver count must be in [1,%d]", lateEntryMaxReceivers)
	}
	seen := make(map[uint16]struct{}, len(cfg.Receivers))
	for i, receiver := range cfg.Receivers {
		if receiver.ReceiverID == 0 {
			return fmt.Errorf("late entry timing resync: receivers[%d].receiver_id must not be zero", i)
		}
		if _, ok := seen[receiver.ReceiverID]; ok {
			return fmt.Errorf("late entry timing resync: duplicate receiver_id %d", receiver.ReceiverID)
		}
		seen[receiver.ReceiverID] = struct{}{}
		if receiver.CoverageEnterMs < 0 || receiver.TimingHintDelayMs < 0 || receiver.MissedSyncCount < 0 || receiver.DriftPPM < 0 || receiver.DriftObservationMs < 0 {
			return fmt.Errorf("late entry timing resync: receiver %d has negative timing values", receiver.ReceiverID)
		}
		if receiver.HasCallStart && receiver.CoverageEnterMs > cfg.CallStartMs {
			return fmt.Errorf("late entry timing resync: receiver %d has_call_start requires coverage_enter_ms <= call_start_ms", receiver.ReceiverID)
		}
		if receiver.DriftObservationMs > 0 && receiver.DriftPPM > math.MaxInt64/receiver.DriftObservationMs {
			return fmt.Errorf("late entry timing resync: receiver %d drift calculation would overflow", receiver.ReceiverID)
		}
	}
	return nil
}

func lateEntryReceiverRow(cfg LateEntryTimingResyncConfig, receiver LateEntryReceiverConfig) (LateEntryReceiverRow, error) {
	joinOffset := receiver.CoverageEnterMs - cfg.CallStartMs
	row := LateEntryReceiverRow{
		ReceiverID:          receiver.ReceiverID,
		CoverageEnterMs:     receiver.CoverageEnterMs,
		JoinOffsetMs:        joinOffset,
		HasCallStart:        receiver.HasCallStart,
		TimingSourceType:    cfg.TimingSourceType,
		TimingHintAvailable: receiver.TimingHintAvailable,
		TimingHintDelayMs:   receiver.TimingHintDelayMs,
		AcquisitionBudgetMs: cfg.AcquisitionBudgetMs,
		AcquisitionMs:       -1,
		AudioStartMs:        -1,
		MissedSyncCount:     receiver.MissedSyncCount,
		DriftPPM:            receiver.DriftPPM,
		DriftObservationMs:  receiver.DriftObservationMs,
		DriftMs:             lateEntryDriftMs(receiver),
		DegradeReasons:      make([]string, 0, 4),
	}
	if receiver.HasCallStart && receiver.CoverageEnterMs <= cfg.CallStartMs {
		row.Classification = LateEntryClassificationNormal
		row.AcquisitionMs = 0
		row.AudioStartMs = cfg.CallStartMs
		return row, nil
	}
	row.DegradeReasons = append(row.DegradeReasons, "missed_call_start")
	if receiver.CoverageEnterMs >= cfg.CallEndMs || !receiver.TimingHintAvailable {
		row.Classification = LateEntryClassificationUncovered
		row.DegradeReasons = append(row.DegradeReasons, "timing_context_unavailable")
		return row, nil
	}
	audioStart, err := checkedLimitedMultiHopAdd(receiver.CoverageEnterMs, receiver.TimingHintDelayMs)
	if err != nil {
		return LateEntryReceiverRow{}, err
	}
	row.AcquisitionMs = receiver.TimingHintDelayMs
	row.AudioStartMs = audioStart
	row.ResyncEvents = 1
	if receiver.MissedSyncCount > cfg.MaxMissedSyncBeforeDegraded {
		row.DegradeReasons = append(row.DegradeReasons, "missed_sync_threshold_exceeded")
	}
	if row.DriftMs > cfg.GuardMs {
		row.GuardExhaustedCount = 1
		row.DriftCollisionCount = 1
		row.DegradeReasons = append(row.DegradeReasons, "guard_exhausted_by_drift")
	}
	if row.AcquisitionMs > cfg.AcquisitionBudgetMs {
		row.DegradeReasons = append(row.DegradeReasons, "acquisition_budget_exceeded")
	}
	if len(row.DegradeReasons) == 1 {
		row.Classification = LateEntryClassificationAcquired
	} else {
		row.Classification = LateEntryClassificationDegraded
	}
	return row, nil
}

func lateEntryAccumulate(report *LateEntryTimingResyncReport, row LateEntryReceiverRow) {
	switch row.Classification {
	case LateEntryClassificationNormal:
		report.NormalReceiverCount++
	case LateEntryClassificationAcquired:
		report.AcquiredReceiverCount++
	case LateEntryClassificationDegraded:
		report.DegradedReceiverCount++
	case LateEntryClassificationUncovered:
		report.UncoveredReceiverCount++
	}
	if !row.HasCallStart {
		report.MissedCallStartCount++
	}
	report.ResyncEvents += row.ResyncEvents
	report.MissedSyncCount += row.MissedSyncCount
	report.GuardExhaustedCount += row.GuardExhaustedCount
	report.DriftCollisionCount += row.DriftCollisionCount
	if row.AcquisitionMs > report.MaxAcquisitionMs {
		report.MaxAcquisitionMs = row.AcquisitionMs
	}
}

func lateEntryReportOutcome(report LateEntryTimingResyncReport) (string, []string) {
	reasons := make([]string, 0, 5)
	if report.MissedCallStartCount > 0 {
		reasons = append(reasons, "missed_call_start_receivers")
	}
	if report.DegradedReceiverCount > 0 {
		reasons = append(reasons, "late_join_degraded")
	}
	if report.UncoveredReceiverCount > 0 {
		reasons = append(reasons, "late_join_uncovered")
	}
	if report.GuardExhaustedCount > 0 {
		reasons = append(reasons, "guard_exhausted")
	}
	if report.DriftCollisionCount > 0 {
		reasons = append(reasons, "drift_collision")
	}
	if report.UncoveredReceiverCount == report.ReceiverCount {
		return LateEntryOutcomeFail, reasons
	}
	if len(reasons) > 0 {
		return LateEntryOutcomeDegraded, reasons
	}
	return LateEntryOutcomePass, nil
}

func lateEntryDriftMs(receiver LateEntryReceiverConfig) int64 {
	return receiver.DriftPPM * receiver.DriftObservationMs / 1000000
}

func sortedLateEntryReceivers(receivers []LateEntryReceiverConfig) []LateEntryReceiverConfig {
	out := cloneLateEntryReceiverConfigs(receivers)
	sort.Slice(out, func(i, j int) bool {
		if out[i].CoverageEnterMs != out[j].CoverageEnterMs {
			return out[i].CoverageEnterMs < out[j].CoverageEnterMs
		}
		return out[i].ReceiverID < out[j].ReceiverID
	})
	return out
}

func cloneLateEntryReceiverConfigs(values []LateEntryReceiverConfig) []LateEntryReceiverConfig {
	return append([]LateEntryReceiverConfig(nil), values...)
}

func cloneLateEntryStrings(values []string) []string {
	return append([]string(nil), values...)
}

func containsLateEntryReason(values []string, reason string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == reason {
			return true
		}
	}
	return false
}
