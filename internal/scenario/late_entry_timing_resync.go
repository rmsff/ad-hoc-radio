package scenario

const LateEntryTimingResyncScenarioID = "late_entry_timing_resync"
const LateEntryTimingResyncStoryID = "MVP-8.11"
const LateEntryTimingResyncEvidenceType = "late_entry_timing_resync"

const (
	LateEntryTimingSourceScenarioMetadata = "scenario_metadata"

	LateEntryClassificationNormal    = "normal"
	LateEntryClassificationAcquired  = "late_join_acquired"
	LateEntryClassificationDegraded  = "late_join_degraded"
	LateEntryClassificationUncovered = "late_join_uncovered"
)

const (
	LateEntryOutcomePass     = "late_entry_pass"
	LateEntryOutcomeDegraded = "late_entry_degraded"
	LateEntryOutcomeFail     = "fail"
)

const lateEntryMaxReceivers = 128

type LateEntryTimingResyncConfig struct {
	ScenarioID                  string
	Seed                        uint64
	TimingSourceType            string
	CallStartMs                 int64
	CallEndMs                   int64
	AcquisitionBudgetMs         int64
	GuardMs                     int64
	SyncIntervalMs              int64
	MaxMissedSyncBeforeDegraded int
	Receivers                   []LateEntryReceiverConfig
}

type LateEntryReceiverConfig struct {
	ReceiverID          uint16 `json:"receiver_id"`
	CoverageEnterMs     int64  `json:"coverage_enter_ms"`
	HasCallStart        bool   `json:"has_call_start"`
	TimingHintAvailable bool   `json:"timing_hint_available"`
	TimingHintDelayMs   int64  `json:"timing_hint_delay_ms"`
	MissedSyncCount     int    `json:"missed_sync_count"`
	DriftPPM            int64  `json:"drift_ppm"`
	DriftObservationMs  int64  `json:"drift_observation_ms"`
}

type LateEntryTimingResyncReport struct {
	ScenarioID                  string                 `json:"scenario_id"`
	Seed                        uint64                 `json:"seed"`
	Evidence                    ScenarioEvidence       `json:"evidence"`
	TimingSourceType            string                 `json:"timing_source_type"`
	CallStartMs                 int64                  `json:"call_start_ms"`
	CallEndMs                   int64                  `json:"call_end_ms"`
	AcquisitionBudgetMs         int64                  `json:"acquisition_budget_ms"`
	GuardMs                     int64                  `json:"guard_ms"`
	SyncIntervalMs              int64                  `json:"sync_interval_ms"`
	MaxMissedSyncBeforeDegraded int                    `json:"max_missed_sync_before_degraded"`
	Receivers                   []LateEntryReceiverRow `json:"receivers"`
	ReceiverCount               int                    `json:"receiver_count"`
	NormalReceiverCount         int                    `json:"normal_receiver_count"`
	AcquiredReceiverCount       int                    `json:"acquired_receiver_count"`
	DegradedReceiverCount       int                    `json:"degraded_receiver_count"`
	UncoveredReceiverCount      int                    `json:"uncovered_receiver_count"`
	MissedCallStartCount        int                    `json:"missed_call_start_count"`
	ResyncEvents                int                    `json:"resync_events"`
	MissedSyncCount             int                    `json:"missed_sync_count"`
	GuardExhaustedCount         int                    `json:"guard_exhausted_count"`
	DriftCollisionCount         int                    `json:"drift_collision_count"`
	MaxAcquisitionMs            int64                  `json:"max_acquisition_ms"`
	Outcome                     string                 `json:"outcome"`
	DegradeReasons              []string               `json:"degrade_reasons"`
	EventHash                   string                 `json:"event_hash"`
}

type LateEntryReceiverRow struct {
	ReceiverID          uint16   `json:"receiver_id"`
	CoverageEnterMs     int64    `json:"coverage_enter_ms"`
	JoinOffsetMs        int64    `json:"join_offset_ms"`
	HasCallStart        bool     `json:"has_call_start"`
	TimingSourceType    string   `json:"timing_source_type"`
	TimingHintAvailable bool     `json:"timing_hint_available"`
	TimingHintDelayMs   int64    `json:"timing_hint_delay_ms"`
	AcquisitionBudgetMs int64    `json:"acquisition_budget_ms"`
	AcquisitionMs       int64    `json:"acquisition_ms"`
	AudioStartMs        int64    `json:"audio_start_ms"`
	ResyncEvents        int      `json:"resync_events"`
	MissedSyncCount     int      `json:"missed_sync_count"`
	GuardExhaustedCount int      `json:"guard_exhausted_count"`
	DriftPPM            int64    `json:"drift_ppm"`
	DriftObservationMs  int64    `json:"drift_observation_ms"`
	DriftMs             int64    `json:"drift_ms"`
	DriftCollisionCount int      `json:"drift_collision_count"`
	Classification      string   `json:"classification"`
	DegradeReasons      []string `json:"degrade_reasons"`
}

func DefaultLateEntryTimingResyncConfig() LateEntryTimingResyncConfig {
	return LateEntryTimingResyncConfig{
		ScenarioID:                  LateEntryTimingResyncScenarioID,
		Seed:                        DefaultScenarioSeed,
		TimingSourceType:            LateEntryTimingSourceScenarioMetadata,
		CallStartMs:                 0,
		CallEndMs:                   5000,
		AcquisitionBudgetMs:         500,
		GuardMs:                     5,
		SyncIntervalMs:              250,
		MaxMissedSyncBeforeDegraded: 2,
		Receivers: []LateEntryReceiverConfig{
			{ReceiverID: 8, CoverageEnterMs: 0, HasCallStart: true, TimingHintAvailable: true, TimingHintDelayMs: 0},
			{ReceiverID: 2, CoverageEnterMs: 300, HasCallStart: false, TimingHintAvailable: true, TimingHintDelayMs: 120, MissedSyncCount: 1, DriftPPM: 10, DriftObservationMs: 100000},
			{ReceiverID: 3, CoverageEnterMs: 900, HasCallStart: false, TimingHintAvailable: true, TimingHintDelayMs: 700, MissedSyncCount: 2, DriftPPM: 20, DriftObservationMs: 150000},
			{ReceiverID: 4, CoverageEnterMs: 1200, HasCallStart: false, TimingHintAvailable: true, TimingHintDelayMs: 180, MissedSyncCount: 1, DriftPPM: 100, DriftObservationMs: 100000},
			{ReceiverID: 5, CoverageEnterMs: 2500, HasCallStart: false, TimingHintAvailable: false, MissedSyncCount: 3},
		},
	}
}

func RunLateEntryTimingResyncScenario(cfg LateEntryTimingResyncConfig) (LateEntryTimingResyncReport, error) {
	cfg = normalizeLateEntryTimingResyncConfig(cfg)
	if err := validateLateEntryTimingResyncConfig(cfg); err != nil {
		return LateEntryTimingResyncReport{}, err
	}
	report := LateEntryTimingResyncReport{
		ScenarioID:                  cfg.ScenarioID,
		Seed:                        cfg.Seed,
		TimingSourceType:            cfg.TimingSourceType,
		CallStartMs:                 cfg.CallStartMs,
		CallEndMs:                   cfg.CallEndMs,
		AcquisitionBudgetMs:         cfg.AcquisitionBudgetMs,
		GuardMs:                     cfg.GuardMs,
		SyncIntervalMs:              cfg.SyncIntervalMs,
		MaxMissedSyncBeforeDegraded: cfg.MaxMissedSyncBeforeDegraded,
		Receivers:                   make([]LateEntryReceiverRow, 0, len(cfg.Receivers)),
	}
	for _, receiver := range sortedLateEntryReceivers(cfg.Receivers) {
		row, err := lateEntryReceiverRow(cfg, receiver)
		if err != nil {
			return LateEntryTimingResyncReport{}, err
		}
		report.Receivers = append(report.Receivers, row)
		lateEntryAccumulate(&report, row)
	}
	report.ReceiverCount = len(report.Receivers)
	report.Outcome, report.DegradeReasons = lateEntryReportOutcome(report)
	report.Evidence = lateEntryEvidence(cfg, report, "")
	hash, err := lateEntryHash(report)
	if err != nil {
		return LateEntryTimingResyncReport{}, err
	}
	report.EventHash = hash
	report.Evidence = lateEntryEvidence(cfg, report, hash)
	if err := ValidateScenarioEvidence(report.Evidence); err != nil {
		return LateEntryTimingResyncReport{}, err
	}
	return report, nil
}
