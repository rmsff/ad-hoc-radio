package scenario

import (
	"encoding/json"

	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

const (
	TimingProfileComparisonStoryID    = "MVP-9.6"
	TimingProfileComparisonScenarioID = "timing_authority_profile_comparison"
	TimingProfileComparisonType       = "timing_profile_comparison"
)

type TimingProfile string

const (
	TimingProfileProjectNative   TimingProfile = "project_native_variable"
	TimingProfileDMRInspired30MS TimingProfile = "dmr_inspired_30ms"
)

type TimingProfileFitReason string

const (
	TimingFit                               TimingProfileFitReason = "fit"
	TimingFitPlannerNoFeasibleTree          TimingProfileFitReason = "planner_no_feasible_tree"
	TimingFitScheduleCycleExceeded          TimingProfileFitReason = "schedule_cycle_exceeds_profile"
	TimingFitNoEligibleReplacement          TimingProfileFitReason = "no_eligible_replacement"
	TimingFitLeaderReacquireExceedsNextTick TimingProfileFitReason = "leader_reacquire_exceeds_next_tick"
	TimingFitLeaderLossOutsideCall          TimingProfileFitReason = "leader_loss_outside_call"
)

type TimingRuntimeOutcome string

const (
	TimingRuntimePass         TimingRuntimeOutcome = "pass"
	TimingRuntimeFail         TimingRuntimeOutcome = "fail"
	TimingRuntimeInconclusive TimingRuntimeOutcome = "inconclusive"
	TimingRuntimeNotRun       TimingRuntimeOutcome = "not_run"
)

type TimingAuthorityPeer struct {
	StationID     uint16
	TimingCapable bool
	LastSeenMS    int64
	FreshnessMS   uint32
	Busy          bool
	BatteryPct    uint8
	DriftPPM      int32
	JoinAtMS      *int64
}

type TimingAuthorityLoss struct {
	AtMS              int64
	LostStationID     uint16
	LeaderReacquireMS int64
}

type TimingProfileComparisonOverlay struct {
	Peers               []TimingAuthorityPeer
	ConfiguredLeaderID  uint16
	MinTimingBatteryPct uint8
	SyncToleranceMS     int64
	LeaderLoss          *TimingAuthorityLoss
}

type TimingProfileComparisonFixture struct {
	FixtureID     string
	BaseConfig    ScheduledRelayTreeScenarioConfig
	TimingOverlay TimingProfileComparisonOverlay
}

type TimingProfileComparisonReport struct {
	StoryID                string                       `json:"story_id"`
	ScenarioID             string                       `json:"scenario_id"`
	EvidenceType           string                       `json:"evidence_type"`
	Evidence               ScenarioEvidence             `json:"evidence"`
	DMRCompatible          bool                         `json:"dmr_compatible"`
	CompatibilityClaim     bool                         `json:"compatibility_claim"`
	AMBEDependency         bool                         `json:"ambe_dependency"`
	RFSynchronizationClaim bool                         `json:"rf_synchronization_claim"`
	Recommendation         string                       `json:"recommendation"`
	Rows                   []TimingProfileComparisonRow `json:"rows"`
}

type TimingProfileComparisonRow struct {
	RowID                                 string                         `json:"row_id"`
	FixtureID                             string                         `json:"fixture_id"`
	TimingStrategy                        string                         `json:"timing_strategy"`
	Profile                               TimingProfile                  `json:"profile"`
	ProfileFit                            bool                           `json:"profile_fit"`
	FitReason                             TimingProfileFitReason         `json:"fit_reason"`
	DMRCompatible                         bool                           `json:"dmr_compatible"`
	CompatibilityClaim                    bool                           `json:"compatibility_claim"`
	AMBEDependency                        bool                           `json:"ambe_dependency"`
	RFSynchronizationClaim                bool                           `json:"rf_synchronization_claim"`
	OpportunityIncludesLowerLayerOverhead bool                           `json:"opportunity_includes_lower_layer_overhead"`
	ExtraTurnaroundAssumptionMS           int64                          `json:"extra_turnaround_assumption_ms"`
	TimingAuthorityID                     uint16                         `json:"timing_authority_id"`
	TimingEpochID                         uint32                         `json:"timing_epoch_id"`
	LeaderLossCount                       int                            `json:"leader_loss_count"`
	LeaderReacquireMS                     int64                          `json:"leader_reacquire_ms"`
	LossEventPriority                     int                            `json:"loss_event_priority"`
	ReplanEventPriority                   int                            `json:"replan_event_priority"`
	PTTAccessDelayMS                      int64                          `json:"ptt_access_delay_ms"`
	ScheduleAcquisitionMS                 int64                          `json:"schedule_acquisition_ms"`
	LateEntryAcquisitionMS                int64                          `json:"late_entry_acquisition_ms"`
	DriftCollisionCount                   int                            `json:"drift_collision_count"`
	ResyncCount                           int                            `json:"resync_count"`
	ScheduleCycleDurationMS               int64                          `json:"schedule_cycle_duration_ms"`
	FramePeriodMS                         int64                          `json:"frame_period_ms"`
	VoiceAirtimePct                       float64                        `json:"voice_airtime_pct"`
	ControlAirtimePct                     float64                        `json:"control_airtime_pct"`
	RuntimeOutcome                        TimingRuntimeOutcome           `json:"runtime_outcome"`
	SelectedRelayDepth                    uint8                          `json:"selected_relay_depth"`
	ExpectedMemberIDs                     []uint16                       `json:"expected_member_ids"`
	CoveredMemberIDs                      []uint16                       `json:"covered_member_ids"`
	DeliveredMemberIDs                    []uint16                       `json:"delivered_member_ids"`
	UncoveredMemberIDs                    []uint16                       `json:"uncovered_member_ids"`
	LatencyByRelayDepth                   []RelayDepthLatencyReport      `json:"latency_by_relay_depth"`
	DuplicatePlayoutCount                 int                            `json:"duplicate_playout_count"`
	OutOfScheduleTxCount                  int                            `json:"out_of_schedule_tx_count"`
	RouteReplanCount                      int                            `json:"route_replan_count"`
	VersionActivationCount                int                            `json:"version_activation_count"`
	ScheduleVersion                       uint16                         `json:"schedule_version"`
	SourceFramesCreated                   int                            `json:"source_frames_created"`
	DeliveredFrameCount                   int                            `json:"delivered_frame_count"`
	MemberDelivery                        []ScheduledMemberDeliveryCount `json:"member_delivery"`
	TreeHash                              string                         `json:"tree_hash"`
	ScheduleHash                          string                         `json:"schedule_hash"`
	EventHash                             string                         `json:"event_hash"`
	ResultHash                            string                         `json:"result_hash"`
}

func MarshalTimingProfileComparisonReport(report TimingProfileComparisonReport) ([]byte, error) {
	return json.MarshalIndent(cloneTimingProfileComparisonReport(report), "", "  ")
}

func cloneTimingProfileComparisonReport(in TimingProfileComparisonReport) TimingProfileComparisonReport {
	out := in
	out.Evidence.ArtifactRefs = append([]ScenarioEvidenceArtifact(nil), in.Evidence.ArtifactRefs...)
	out.Evidence.AssumptionFlags = append([]string(nil), in.Evidence.AssumptionFlags...)
	out.Evidence.DegradeReasons = append([]string(nil), in.Evidence.DegradeReasons...)
	out.Rows = make([]TimingProfileComparisonRow, len(in.Rows))
	for i, row := range in.Rows {
		out.Rows[i] = row
		out.Rows[i].ExpectedMemberIDs = append([]uint16(nil), row.ExpectedMemberIDs...)
		out.Rows[i].CoveredMemberIDs = append([]uint16(nil), row.CoveredMemberIDs...)
		out.Rows[i].DeliveredMemberIDs = append([]uint16(nil), row.DeliveredMemberIDs...)
		out.Rows[i].UncoveredMemberIDs = append([]uint16(nil), row.UncoveredMemberIDs...)
		out.Rows[i].LatencyByRelayDepth = append([]RelayDepthLatencyReport(nil), row.LatencyByRelayDepth...)
		out.Rows[i].MemberDelivery = append([]ScheduledMemberDeliveryCount(nil), row.MemberDelivery...)
	}
	return out
}

var timingComparisonStrategyOrder = []timing.TimingStrategy{
	timing.TimingStrategySourceLed,
	timing.TimingStrategyGroupLeader,
}

var timingComparisonProfileOrder = []TimingProfile{
	TimingProfileProjectNative,
	TimingProfileDMRInspired30MS,
}
