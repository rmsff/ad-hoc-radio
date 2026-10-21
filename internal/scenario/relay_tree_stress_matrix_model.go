package scenario

import (
	"encoding/json"

	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

const (
	RelayTreeStressStoryID    = "MVP-9.7"
	RelayTreeStressScenarioID = "relay_tree_stress_matrix"
	RelayTreeStressType       = "relay_tree_stress_matrix"
)

type RelayTreeStressCaseClass string

const (
	RelayTreeStressClassNominal    RelayTreeStressCaseClass = "nominal"
	RelayTreeStressClassBranch     RelayTreeStressCaseClass = "branch"
	RelayTreeStressClassAsymmetric RelayTreeStressCaseClass = "asymmetric"
	RelayTreeStressClassPartition  RelayTreeStressCaseClass = "partition"
	RelayTreeStressClassMembership RelayTreeStressCaseClass = "membership"
	RelayTreeStressClassLink       RelayTreeStressCaseClass = "link_impairment"
	RelayTreeStressClassRelay      RelayTreeStressCaseClass = "relay_state"
	RelayTreeStressClassCombined   RelayTreeStressCaseClass = "combined_failure"
)

type RelayTreeStressCaseRole string

const (
	RelayTreeStressRoleCore       RelayTreeStressCaseRole = "core"
	RelayTreeStressRoleResilience RelayTreeStressCaseRole = "resilience"
	RelayTreeStressRoleBoundary   RelayTreeStressCaseRole = "boundary"
)

type RelayTreeStressTag string

const (
	RelayTreeStressTagNominal         RelayTreeStressTag = "nominal"
	RelayTreeStressTagDirect          RelayTreeStressTag = "direct"
	RelayTreeStressTagChain           RelayTreeStressTag = "chain"
	RelayTreeStressTagBranch          RelayTreeStressTag = "branch"
	RelayTreeStressTagAsymmetricLink  RelayTreeStressTag = "asymmetric_link"
	RelayTreeStressTagPartition       RelayTreeStressTag = "partition"
	RelayTreeStressTagRecovery        RelayTreeStressTag = "recovery"
	RelayTreeStressTagStaleMembership RelayTreeStressTag = "stale_membership"
	RelayTreeStressTagMissingMember   RelayTreeStressTag = "missing_member"
	RelayTreeStressTagLateJoin        RelayTreeStressTag = "late_join"
	RelayTreeStressTagLinkLoss        RelayTreeStressTag = "link_loss"
	RelayTreeStressTagLinkDelay       RelayTreeStressTag = "link_delay"
	RelayTreeStressTagClockDrift      RelayTreeStressTag = "clock_drift"
	RelayTreeStressTagRelayBusy       RelayTreeStressTag = "relay_busy"
	RelayTreeStressTagRelayLowBattery RelayTreeStressTag = "relay_low_battery"
	RelayTreeStressTagRelayLoss       RelayTreeStressTag = "relay_loss"
	RelayTreeStressTagLeaderLoss      RelayTreeStressTag = "leader_loss"
)

type RelayTreeStressExpectation struct {
	ExpectedUncoveredMemberIDs []uint16
	RequireAllDelivered        bool
	MinRouteReplans            int
	MinVersionActivations      int
	AllowProfileRejection      bool
}

type RelayTreeStressCase struct {
	CaseID              string
	Class               RelayTreeStressCaseClass
	Role                RelayTreeStressCaseRole
	ExpectedGroupSize   int
	RequestedRelayDepth uint8
	BranchWidth         int
	StressTags          []RelayTreeStressTag
	BaseConfig          ScheduledRelayTreeScenarioConfig
	TimingOverlay       TimingProfileComparisonOverlay
	Expectation         RelayTreeStressExpectation
}

type RelayTreeBackendStatus string

const (
	RelayTreeBackendNotEvaluated RelayTreeBackendStatus = "not_evaluated"
	RelayTreeBackendPassed       RelayTreeBackendStatus = "passed"
	RelayTreeBackendFailed       RelayTreeBackendStatus = "failed"
)

type RelayTreeBackendAssessment struct {
	TimingStrategy timing.TimingStrategy  `json:"timing_strategy"`
	Profile        TimingProfile          `json:"profile"`
	Status         RelayTreeBackendStatus `json:"status"`
	Reason         string                 `json:"reason,omitempty"`
	Evidence       *ScenarioEvidence      `json:"evidence,omitempty"`
}

type RelayTreeStressMatrixConfig struct {
	Cases              []RelayTreeStressCase
	BackendAssessments []RelayTreeBackendAssessment
}

type RelayTreeStressClassification string

const (
	RelayTreeStressPass     RelayTreeStressClassification = "pass"
	RelayTreeStressDegraded RelayTreeStressClassification = "degraded"
	RelayTreeStressNoGo     RelayTreeStressClassification = "no_go"
)

type RelayTreeStressMatrixRow struct {
	RowID                       string                        `json:"row_id"`
	CaseID                      string                        `json:"case_id"`
	CaseClass                   RelayTreeStressCaseClass      `json:"case_class"`
	CaseRole                    RelayTreeStressCaseRole       `json:"case_role"`
	ExpectedGroupSize           int                           `json:"expected_group_size"`
	RequestedRelayDepth         uint8                         `json:"requested_relay_depth"`
	BranchWidth                 int                           `json:"branch_width"`
	StressTags                  []RelayTreeStressTag          `json:"stress_tags"`
	Classification              RelayTreeStressClassification `json:"classification"`
	InvariantPass               bool                          `json:"invariant_pass"`
	DeliveryRatioPct            float64                       `json:"delivery_ratio_pct"`
	MinMemberDeliveryPct        float64                       `json:"min_member_delivery_pct"`
	Depth3CounterfactualCovered int                           `json:"depth_3_counterfactual_covered"`
	CounterfactualEvaluated     bool                          `json:"counterfactual_evaluated"`
	Depth3CoverageGain          int                           `json:"depth_3_coverage_gain"`
	Depth3AddsUsefulCoverage    bool                          `json:"depth_3_adds_useful_coverage"`
	ReasonCodes                 []string                      `json:"reason_codes"`
	Comparison                  TimingProfileComparisonRow    `json:"comparison"`
}

type RelayTreeProfileVerdict string

const (
	RelayTreeProfileGo           RelayTreeProfileVerdict = "go"
	RelayTreeProfileDegradedOnly RelayTreeProfileVerdict = "degraded-only"
	RelayTreeProfileNoGo         RelayTreeProfileVerdict = "no-go"
)

type RelayTreeProfileSummary struct {
	ProfileID                    string                  `json:"profile_id"`
	TimingStrategy               timing.TimingStrategy   `json:"timing_strategy"`
	Profile                      TimingProfile           `json:"profile"`
	Verdict                      RelayTreeProfileVerdict `json:"verdict"`
	BackendStatus                RelayTreeBackendStatus  `json:"backend_status"`
	BackendFailureReason         string                  `json:"backend_failure_reason,omitempty"`
	BackendEvidenceGrade         string                  `json:"backend_evidence_grade,omitempty"`
	BackendEvidenceHash          string                  `json:"backend_evidence_hash,omitempty"`
	TotalRows                    int                     `json:"total_rows"`
	PassRows                     int                     `json:"pass_rows"`
	DegradedRows                 int                     `json:"degraded_rows"`
	NoGoRows                     int                     `json:"no_go_rows"`
	CoreRows                     int                     `json:"core_rows"`
	CorePassRows                 int                     `json:"core_pass_rows"`
	MaxSupportedRelayDepth       uint8                   `json:"max_supported_relay_depth"`
	MaxPassingBranchWidth        int                     `json:"max_passing_branch_width"`
	FirstImpracticalBranchWidth  int                     `json:"first_impractical_branch_width"`
	Depth3AddsUsefulCoverage     bool                    `json:"depth_3_adds_useful_coverage"`
	DriftCollisionCount          int                     `json:"drift_collision_count"`
	ResyncCount                  int                     `json:"resync_count"`
	RouteReplanCount             int                     `json:"route_replan_count"`
	PTTAccessDelayMS             int64                   `json:"ptt_access_delay_ms"`
	ScheduleAcquisitionMS        int64                   `json:"schedule_acquisition_ms"`
	LateEntryAcquisitionMS       int64                   `json:"late_entry_acquisition_ms"`
	AccessAcquisitionTotalMS     int64                   `json:"access_acquisition_total_ms"`
	SerializedScheduleAssessment string                  `json:"serialized_schedule_assessment"`
	OperatingEnvelope            []string                `json:"operating_envelope"`
	ReasonCodes                  []string                `json:"reason_codes"`
}

type RelayTreeArchitectureVerdict string

const (
	RelayTreeArchitectureAccepted       RelayTreeArchitectureVerdict = "accepted"
	RelayTreeArchitectureReviewRequired RelayTreeArchitectureVerdict = "review_required"
)

type RelayTreeStressMatrixReport struct {
	StoryID                string                       `json:"story_id"`
	ScenarioID             string                       `json:"scenario_id"`
	EvidenceType           string                       `json:"evidence_type"`
	Evidence               ScenarioEvidence             `json:"evidence"`
	DMRCompatible          bool                         `json:"dmr_compatible"`
	CompatibilityClaim     bool                         `json:"compatibility_claim"`
	AMBEDependency         bool                         `json:"ambe_dependency"`
	RFSynchronizationClaim bool                         `json:"rf_synchronization_claim"`
	ArchitectureVerdict    RelayTreeArchitectureVerdict `json:"architecture_verdict"`
	Recommendation         string                       `json:"recommendation"`
	CaseCount              int                          `json:"case_count"`
	RowCount               int                          `json:"row_count"`
	ProfileSummaries       []RelayTreeProfileSummary    `json:"profile_summaries"`
	Rows                   []RelayTreeStressMatrixRow   `json:"rows"`
}

func MarshalRelayTreeStressMatrixReport(report RelayTreeStressMatrixReport) ([]byte, error) {
	return json.MarshalIndent(cloneRelayTreeStressMatrixReport(report), "", "  ")
}

func cloneRelayTreeStressMatrixReport(in RelayTreeStressMatrixReport) RelayTreeStressMatrixReport {
	out := in
	out.Evidence.ArtifactRefs = append([]ScenarioEvidenceArtifact(nil), in.Evidence.ArtifactRefs...)
	out.Evidence.AssumptionFlags = append([]string(nil), in.Evidence.AssumptionFlags...)
	out.Evidence.DegradeReasons = append([]string(nil), in.Evidence.DegradeReasons...)
	out.ProfileSummaries = make([]RelayTreeProfileSummary, len(in.ProfileSummaries))
	for i, summary := range in.ProfileSummaries {
		out.ProfileSummaries[i] = summary
		out.ProfileSummaries[i].OperatingEnvelope = append([]string(nil), summary.OperatingEnvelope...)
		out.ProfileSummaries[i].ReasonCodes = append([]string(nil), summary.ReasonCodes...)
	}
	out.Rows = make([]RelayTreeStressMatrixRow, len(in.Rows))
	for i, row := range in.Rows {
		out.Rows[i] = row
		out.Rows[i].StressTags = append([]RelayTreeStressTag(nil), row.StressTags...)
		out.Rows[i].ReasonCodes = append([]string(nil), row.ReasonCodes...)
		out.Rows[i].Comparison.ExpectedMemberIDs = append([]uint16(nil), row.Comparison.ExpectedMemberIDs...)
		out.Rows[i].Comparison.CoveredMemberIDs = append([]uint16(nil), row.Comparison.CoveredMemberIDs...)
		out.Rows[i].Comparison.DeliveredMemberIDs = append([]uint16(nil), row.Comparison.DeliveredMemberIDs...)
		out.Rows[i].Comparison.UncoveredMemberIDs = append([]uint16(nil), row.Comparison.UncoveredMemberIDs...)
		out.Rows[i].Comparison.LatencyByRelayDepth = append([]RelayDepthLatencyReport(nil), row.Comparison.LatencyByRelayDepth...)
		out.Rows[i].Comparison.MemberDelivery = append([]ScheduledMemberDeliveryCount(nil), row.Comparison.MemberDelivery...)
	}
	return out
}

var relayTreeStressProfileOrder = []struct {
	Strategy timing.TimingStrategy
	Profile  TimingProfile
}{
	{timing.TimingStrategySourceLed, TimingProfileProjectNative},
	{timing.TimingStrategySourceLed, TimingProfileDMRInspired30MS},
	{timing.TimingStrategyGroupLeader, TimingProfileProjectNative},
	{timing.TimingStrategyGroupLeader, TimingProfileDMRInspired30MS},
}

func relayTreeStressProfileID(strategy timing.TimingStrategy, profile TimingProfile) string {
	return string(strategy) + "/" + string(profile)
}
