package scenario

import "encoding/json"

const GroupFloorPTTContentionScenarioID = "group_floor_ptt_contention"
const GroupFloorPTTContentionStoryID = "MVP-8.10"
const GroupFloorPTTContentionEvidenceType = "group_floor_ptt_contention"

const (
	GroupFloorPolicyFirstArrivalWins = "first_arrival_wins"

	GroupFloorPTTOutcomeGranted    = "granted"
	GroupFloorPTTOutcomeDeniedBusy = "denied_due_busy"
	GroupFloorPTTOutcomeCollision  = "collision"
)

const (
	GroupFloorPTTReportOutcomePass     = "floor_pass"
	GroupFloorPTTReportOutcomeDegraded = "floor_degraded"
	GroupFloorPTTReportOutcomeFail     = "fail"
)

const groupFloorPTTMaxAttempts = 128

type GroupFloorPTTContentionConfig struct {
	ScenarioID          string                       `json:"scenario_id"`
	Seed                uint64                       `json:"seed"`
	FloorPolicy         string                       `json:"floor_policy"`
	AccessDecisionMs    int64                        `json:"access_decision_ms"`
	FloorReleaseGuardMs int64                        `json:"floor_release_guard_ms"`
	FirstAudioWindowMs  int64                        `json:"first_audio_window_ms"`
	Attempts            []GroupFloorPTTAttemptConfig `json:"attempts"`
}

type GroupFloorPTTAttemptConfig struct {
	EventID        string `json:"event_id"`
	SourceID       uint16 `json:"source_id"`
	PTTStartMs     int64  `json:"ptt_start_ms"`
	TalkDurationMs int64  `json:"talk_duration_ms"`
}

type GroupFloorPTTContentionReport struct {
	ScenarioID             string                    `json:"scenario_id"`
	Seed                   uint64                    `json:"seed"`
	Evidence               ScenarioEvidence          `json:"evidence"`
	FloorPolicy            string                    `json:"floor_policy"`
	AccessDecisionMs       int64                     `json:"access_decision_ms"`
	FloorReleaseGuardMs    int64                     `json:"floor_release_guard_ms"`
	FirstAudioWindowMs     int64                     `json:"first_audio_window_ms"`
	Attempts               []GroupFloorPTTAttemptRow `json:"attempts"`
	GrantedCount           int                       `json:"granted_count"`
	BusyCount              int                       `json:"busy_count"`
	DeniedDueBusyCount     int                       `json:"denied_due_busy_count"`
	CollisionCount         int                       `json:"collision_count"`
	ClippedFirstAudioMs    int64                     `json:"clipped_first_audio_ms"`
	MaxClippedFirstAudioMs int64                     `json:"max_clipped_first_audio_ms"`
	AccessLatencyP50Ms     int64                     `json:"access_latency_p50_ms"`
	AccessLatencyP95Ms     int64                     `json:"access_latency_p95_ms"`
	DuplicatePlaybackCount int                       `json:"duplicate_playback_count"`
	ActiveFloorCount       int                       `json:"active_floor_count"`
	Outcome                string                    `json:"outcome"`
	DegradeReasons         []string                  `json:"degrade_reasons"`
	EventHash              string                    `json:"event_hash"`
}

type GroupFloorPTTAttemptRow struct {
	EventID             string `json:"event_id"`
	SourceID            uint16 `json:"source_id"`
	PTTStartMs          int64  `json:"ptt_start_ms"`
	TalkDurationMs      int64  `json:"talk_duration_ms"`
	DecisionTimeMs      int64  `json:"decision_time_ms"`
	AccessLatencyMs     int64  `json:"access_latency_ms"`
	FloorStartMs        int64  `json:"floor_start_ms"`
	FloorEndMs          int64  `json:"floor_end_ms"`
	BlockedByEventID    string `json:"blocked_by_event_id"`
	BlockedBySourceID   uint16 `json:"blocked_by_source_id"`
	Outcome             string `json:"outcome"`
	Busy                bool   `json:"busy"`
	SuccessfulSpeaker   bool   `json:"successful_speaker"`
	Collision           bool   `json:"collision"`
	ClippedFirstAudioMs int64  `json:"clipped_first_audio_ms"`
}

type groupFloorPTTActiveFloor struct {
	EventID   string
	SourceID  uint16
	StartMs   int64
	EndMs     int64
	GuardEnd  int64
	Sequence  int
	AttemptIx int
}

func DefaultGroupFloorPTTContentionConfig() GroupFloorPTTContentionConfig {
	return GroupFloorPTTContentionConfig{
		ScenarioID:          GroupFloorPTTContentionScenarioID,
		Seed:                DefaultScenarioSeed,
		FloorPolicy:         GroupFloorPolicyFirstArrivalWins,
		AccessDecisionMs:    20,
		FloorReleaseGuardMs: 20,
		FirstAudioWindowMs:  250,
		Attempts: []GroupFloorPTTAttemptConfig{
			{EventID: "e1", SourceID: 1, PTTStartMs: 0, TalkDurationMs: 500},
			{EventID: "e2", SourceID: 11, PTTStartMs: 0, TalkDurationMs: 500},
			{EventID: "e3", SourceID: 12, PTTStartMs: 300, TalkDurationMs: 500},
			{EventID: "e4", SourceID: 11, PTTStartMs: 800, TalkDurationMs: 500},
			{EventID: "e5", SourceID: 12, PTTStartMs: 1250, TalkDurationMs: 500},
			{EventID: "e6", SourceID: 12, PTTStartMs: 1600, TalkDurationMs: 500},
		},
	}
}

func RunGroupFloorPTTContentionScenario(cfg GroupFloorPTTContentionConfig) (GroupFloorPTTContentionReport, error) {
	cfg = normalizeGroupFloorPTTContentionConfig(cfg)
	if err := validateGroupFloorPTTContentionConfig(cfg); err != nil {
		return GroupFloorPTTContentionReport{}, err
	}
	report := GroupFloorPTTContentionReport{
		ScenarioID:          cfg.ScenarioID,
		Seed:                cfg.Seed,
		FloorPolicy:         cfg.FloorPolicy,
		AccessDecisionMs:    cfg.AccessDecisionMs,
		FloorReleaseGuardMs: cfg.FloorReleaseGuardMs,
		FirstAudioWindowMs:  cfg.FirstAudioWindowMs,
		Attempts:            make([]GroupFloorPTTAttemptRow, 0, len(cfg.Attempts)),
	}
	attempts := sortedGroupFloorPTTAttempts(cfg.Attempts)
	activeFloors := make([]groupFloorPTTActiveFloor, 0, len(attempts))
	accessLatencies := make([]int64, 0, len(attempts))
	for _, attempt := range attempts {
		row, active, err := groupFloorPTTAttemptRow(cfg, attempt, activeFloors)
		if err != nil {
			return GroupFloorPTTContentionReport{}, err
		}
		report.Attempts = append(report.Attempts, row)
		accessLatencies = append(accessLatencies, row.AccessLatencyMs)
		groupFloorPTTAccumulate(&report, row)
		if active != nil {
			active.AttemptIx = len(report.Attempts) - 1
			activeFloors = append(activeFloors, *active)
		}
	}
	report.ActiveFloorCount = len(activeFloors)
	report.AccessLatencyP50Ms = groupFloorPTTNearestRank(accessLatencies, 50)
	report.AccessLatencyP95Ms = groupFloorPTTNearestRank(accessLatencies, 95)
	report.DuplicatePlaybackCount = groupFloorPTTDuplicatePlaybackCount(report.Attempts)
	report.Outcome, report.DegradeReasons = groupFloorPTTReportOutcome(report)
	report.Evidence = groupFloorPTTEvidence(cfg, report, "")
	hash, err := groupFloorPTTHash(report)
	if err != nil {
		return GroupFloorPTTContentionReport{}, err
	}
	report.EventHash = hash
	report.Evidence = groupFloorPTTEvidence(cfg, report, hash)
	if err := ValidateScenarioEvidence(report.Evidence); err != nil {
		return GroupFloorPTTContentionReport{}, err
	}
	return report, nil
}

func (r GroupFloorPTTContentionReport) MarshalReportJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

func groupFloorPTTAttemptRow(cfg GroupFloorPTTContentionConfig, attempt GroupFloorPTTAttemptConfig, activeFloors []groupFloorPTTActiveFloor) (GroupFloorPTTAttemptRow, *groupFloorPTTActiveFloor, error) {
	decisionTime, err := checkedLimitedMultiHopAdd(attempt.PTTStartMs, cfg.AccessDecisionMs)
	if err != nil {
		return GroupFloorPTTAttemptRow{}, nil, err
	}
	row := GroupFloorPTTAttemptRow{
		EventID:         attempt.EventID,
		SourceID:        attempt.SourceID,
		PTTStartMs:      attempt.PTTStartMs,
		TalkDurationMs:  attempt.TalkDurationMs,
		DecisionTimeMs:  decisionTime,
		AccessLatencyMs: cfg.AccessDecisionMs,
		FloorStartMs:    -1,
		FloorEndMs:      -1,
	}
	if blocking, ok := groupFloorPTTBlockingFloor(attempt, activeFloors); ok {
		row.Busy = true
		row.BlockedByEventID = blocking.EventID
		row.BlockedBySourceID = blocking.SourceID
		row.ClippedFirstAudioMs = groupFloorPTTClippedFirstAudio(cfg)
		if attempt.PTTStartMs == blocking.StartMs {
			row.Outcome = GroupFloorPTTOutcomeCollision
			row.Collision = true
			return row, nil, nil
		}
		row.Outcome = GroupFloorPTTOutcomeDeniedBusy
		return row, nil, nil
	}
	floorEnd, err := checkedLimitedMultiHopAdd(attempt.PTTStartMs, attempt.TalkDurationMs)
	if err != nil {
		return GroupFloorPTTAttemptRow{}, nil, err
	}
	guardEnd, err := checkedLimitedMultiHopAdd(floorEnd, cfg.FloorReleaseGuardMs)
	if err != nil {
		return GroupFloorPTTAttemptRow{}, nil, err
	}
	row.Outcome = GroupFloorPTTOutcomeGranted
	row.SuccessfulSpeaker = true
	row.FloorStartMs = attempt.PTTStartMs
	row.FloorEndMs = floorEnd
	active := &groupFloorPTTActiveFloor{
		EventID:  attempt.EventID,
		SourceID: attempt.SourceID,
		StartMs:  attempt.PTTStartMs,
		EndMs:    floorEnd,
		GuardEnd: guardEnd,
		Sequence: len(activeFloors),
	}
	return row, active, nil
}

func groupFloorPTTReportOutcome(report GroupFloorPTTContentionReport) (string, []string) {
	reasons := make([]string, 0, 4)
	if report.DuplicatePlaybackCount > 0 {
		reasons = append(reasons, "duplicate_playback_detected")
	}
	if report.GrantedCount == 0 {
		reasons = append(reasons, "no_granted_speaker")
	}
	if report.BusyCount > 0 {
		reasons = append(reasons, "ptt_contention_detected")
	}
	if report.DeniedDueBusyCount > 0 {
		reasons = append(reasons, "denied_due_busy")
	}
	if report.CollisionCount > 0 {
		reasons = append(reasons, "exact_simultaneous_collision")
	}
	if report.DuplicatePlaybackCount > 0 || report.GrantedCount == 0 {
		return GroupFloorPTTReportOutcomeFail, reasons
	}
	if report.BusyCount > 0 || report.CollisionCount > 0 {
		return GroupFloorPTTReportOutcomeDegraded, reasons
	}
	return GroupFloorPTTReportOutcomePass, nil
}

func groupFloorPTTAccumulate(report *GroupFloorPTTContentionReport, row GroupFloorPTTAttemptRow) {
	switch row.Outcome {
	case GroupFloorPTTOutcomeGranted:
		report.GrantedCount++
	case GroupFloorPTTOutcomeCollision:
		report.BusyCount++
		report.CollisionCount++
	case GroupFloorPTTOutcomeDeniedBusy:
		report.BusyCount++
		report.DeniedDueBusyCount++
	}
	report.ClippedFirstAudioMs += row.ClippedFirstAudioMs
	if row.ClippedFirstAudioMs > report.MaxClippedFirstAudioMs {
		report.MaxClippedFirstAudioMs = row.ClippedFirstAudioMs
	}
}
