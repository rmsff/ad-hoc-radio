package scenario

import (
	"fmt"

	"github.com/rmsff/ad-hoc-radio/internal/channel"
)

const GroupVoiceProfileScenarioID = "group_voice_profile_selection"

const (
	GroupVoiceProfileHigh      = "high"
	GroupVoiceProfileBalanced  = "balanced"
	GroupVoiceProfileRelaySafe = "relay_safe"
)

const (
	GroupVoiceOutcomePass     = "pass"
	GroupVoiceOutcomeDegraded = "degraded"
	GroupVoiceOutcomeFail     = "fail"
)

const (
	groupVoiceReasonPass                  = "group_covered_profile_selected"
	groupVoiceReasonNotAllMembersCovered  = "not_all_members_covered"
	groupVoiceReasonNoProfileSupportsPlan = "no_profile_supports_group_depth"
)

type GroupVoiceProfileScenarioConfig struct {
	ScenarioID            string
	Seed                  uint64
	SourceID              uint16
	GroupID               uint16
	CoverageMode          string
	ExpectedMembers       []uint16
	RequiredGroupCoverage float64
	SourceDirectReachable []uint16
	RelayCoverage         []GroupVoiceRelayCoverage
	UnreachableMembers    []uint16
	Profiles              []GroupVoiceProfile
}

type GroupVoiceRelayCoverage struct {
	RelayPath []uint16
	Members   []uint16
}

type GroupVoiceProfile struct {
	Name                string
	QualityRank         int
	MaxRelayDepth       int
	ChannelBitrateBps   int64
	SourceFrameBytes    int
	RelayFrameBytes     int
	SourceWireMode      string
	PreambleMs          int64
	SyncMs              int64
	TxRxSwitchMs        int64
	SourceSlotMs        int64
	RelaySlotMs         int64
	GuardTimeMs         int64
	VoicePeriodMs       int64
	LinkDelayMs         int64
	LatencyBudgetMs     int64
	FramePeriodBudgetMs int64
}

type GroupVoiceProfileReport struct {
	ScenarioID              string
	Seed                    uint64
	GroupID                 uint16
	SourceID                uint16
	CoverageMode            string
	ExpectedMembers         []uint16
	ExpectedRecipients      []uint16
	RequiredGroupCoverage   float64
	CoverageFraction        float64
	CoveragePass            bool
	RequiredRelayDepth      int
	CoveredDirect           []uint16
	CoveredViaRelay         []uint16
	CoverageByDepth         []GroupVoiceCoverageDepth
	UncoveredMembers        []uint16
	UnknownMembers          []uint16
	SelectedVoiceProfile    string
	ProfileAppliesToMembers []uint16
	ProfileEvaluations      []GroupVoiceProfileEvaluation
	Outcome                 string
	Reason                  string
	EventHash               string
}

type GroupVoiceCoverageDepth struct {
	Depth   int
	Members []uint16
}

type GroupVoiceProfileEvaluation struct {
	ProfileName        string
	QualityRank        int
	MaxRelayDepth      int
	Selected           bool
	Outcome            string
	RejectReasons      []string
	SourceMeasurement  channel.Measurement
	RelayMeasurement   channel.Measurement
	ChannelBudget      GroupVoiceChannelBudgetReport
	CycleOccupiedMs    int64
	CycleSpareMs       int64
	EndToEndLatencyMs  int64
	TimingFailReasons  []string
	ChannelBitrateBps  int64
	SourceFrameBytes   int
	RelayFrameBytes    int
	SourceWireMode     string
	PreambleMs         int64
	SyncMs             int64
	TxRxSwitchMs       int64
	SourceSlotMs       int64
	RelaySlotMs        int64
	GuardTimeMs        int64
	RequiredRelayDepth int
}

type GroupVoiceChannelBudgetReport struct {
	ChannelBitrateBps       int64
	SourceFrameBytes        int
	RelayFrameBytes         int
	SourceWireMode          string
	PreambleMs              int64
	SyncMs                  int64
	TxRxSwitchMs            int64
	GuardTimeMs             int64
	VoicePeriodMs           int64
	LinkDelayMs             int64
	SourceSlotMs            int64
	RelaySlotMs             int64
	SourceAirtimeMs         int64
	RelayAirtimeMs          int64
	SourceTxDurationMs      int64
	RelayTxDurationMs       int64
	SourceSlotUsedMs        int64
	RelaySlotUsedMs         int64
	SourceSlotSpareMs       int64
	RelaySlotSpareMs        int64
	SourceFitsSlot          bool
	RelayFitsSlot           bool
	RelayReady              bool
	RelayReadinessMissCount int
	CycleOccupiedMs         int64
	CycleSpareMs            int64
	CycleFitsPeriod         bool
	EndToEndLatencyMs       int64
	LatencyBudgetMs         int64
	LatencyBudgetPass       bool
	FramePeriodBudgetMs     int64
	CadencePass             bool
	OverallFits             bool
	FailReasons             []string
}

type groupVoiceCoverageClassification struct {
	recipients     []uint16
	coveredDirect  []uint16
	coveredByDepth map[int][]uint16
	uncovered      []uint16
	unknown        []uint16
	requiredDepth  int
}

func DefaultGroupVoiceProfileScenarioConfig() GroupVoiceProfileScenarioConfig {
	return GroupVoiceProfileScenarioConfig{
		ScenarioID:            GroupVoiceProfileScenarioID,
		Seed:                  DefaultScenarioSeed,
		SourceID:              DefaultSourceID,
		GroupID:               DefaultGroupID,
		CoverageMode:          RelayElectionCoverageModeClosedGroup,
		ExpectedMembers:       []uint16{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		RequiredGroupCoverage: 1,
		SourceDirectReachable: []uint16{8, 9, 10},
		RelayCoverage: []GroupVoiceRelayCoverage{
			{RelayPath: []uint16{20}, Members: []uint16{6, 7}},
			{RelayPath: []uint16{20, 21}, Members: []uint16{2, 3, 4, 5}},
		},
		Profiles: DefaultGroupVoiceProfiles(),
	}
}

func DefaultGroupVoiceProfiles() []GroupVoiceProfile {
	return []GroupVoiceProfile{
		{
			Name:                GroupVoiceProfileHigh,
			QualityRank:         0,
			MaxRelayDepth:       0,
			ChannelBitrateBps:   19200,
			SourceFrameBytes:    defaultTwoHopLiveSourceFrameBytes,
			RelayFrameBytes:     defaultTwoHopLiveRelayFrameBytes,
			SourceWireMode:      TwoHopLiveSourceWireCanonical,
			SourceSlotMs:        30,
			RelaySlotMs:         20,
			GuardTimeMs:         5,
			VoicePeriodMs:       defaultTwoHopLiveVoicePeriodMs,
			LinkDelayMs:         1,
			LatencyBudgetMs:     defaultLimitedMultiHopLatencyMs,
			FramePeriodBudgetMs: DefaultFramePeriodMs,
		},
		{
			Name:                GroupVoiceProfileBalanced,
			QualityRank:         1,
			MaxRelayDepth:       1,
			ChannelBitrateBps:   19200,
			SourceFrameBytes:    defaultTwoHopLiveShortSourceFrameBytes,
			RelayFrameBytes:     defaultTwoHopLiveRelayFrameBytes,
			SourceWireMode:      TwoHopLiveSourceWireShort,
			SourceSlotMs:        12,
			RelaySlotMs:         12,
			GuardTimeMs:         2,
			VoicePeriodMs:       defaultTwoHopLiveVoicePeriodMs,
			LinkDelayMs:         1,
			LatencyBudgetMs:     defaultLimitedMultiHopLatencyMs,
			FramePeriodBudgetMs: DefaultFramePeriodMs,
		},
		{
			Name:                GroupVoiceProfileRelaySafe,
			QualityRank:         2,
			MaxRelayDepth:       2,
			ChannelBitrateBps:   19200,
			SourceFrameBytes:    defaultTwoHopLiveShortSourceFrameBytes,
			RelayFrameBytes:     defaultTwoHopLiveRelayFrameBytes,
			SourceWireMode:      TwoHopLiveSourceWireShort,
			SourceSlotMs:        12,
			RelaySlotMs:         12,
			GuardTimeMs:         1,
			VoicePeriodMs:       defaultTwoHopLiveVoicePeriodMs,
			LinkDelayMs:         1,
			LatencyBudgetMs:     defaultLimitedMultiHopLatencyMs,
			FramePeriodBudgetMs: DefaultFramePeriodMs,
		},
	}
}

func RunGroupVoiceProfileScenario(cfg GroupVoiceProfileScenarioConfig) (GroupVoiceProfileReport, error) {
	cfg = normalizeGroupVoiceProfileScenarioConfig(cfg)
	if err := validateGroupVoiceProfileScenarioConfig(cfg); err != nil {
		return GroupVoiceProfileReport{}, err
	}

	classification := classifyGroupVoiceCoverage(cfg)
	report := GroupVoiceProfileReport{
		ScenarioID:            cfg.ScenarioID,
		Seed:                  cfg.Seed,
		GroupID:               cfg.GroupID,
		SourceID:              cfg.SourceID,
		CoverageMode:          cfg.CoverageMode,
		ExpectedMembers:       sortedUniqueUint16s(cfg.ExpectedMembers),
		ExpectedRecipients:    cloneGroupVoiceUint16s(classification.recipients),
		RequiredGroupCoverage: cfg.RequiredGroupCoverage,
		RequiredRelayDepth:    classification.requiredDepth,
		CoveredDirect:         cloneGroupVoiceUint16s(classification.coveredDirect),
		CoverageByDepth:       coverageDepthRows(classification.coveredByDepth),
		UncoveredMembers:      cloneGroupVoiceUint16s(classification.uncovered),
		UnknownMembers:        cloneGroupVoiceUint16s(classification.unknown),
	}
	report.CoveredViaRelay = coveredViaRelayFromDepths(report.CoverageByDepth)
	report.CoverageFraction = groupVoiceCoverageFraction(report)
	report.CoveragePass = report.CoverageFraction >= cfg.RequiredGroupCoverage
	report.ProfileEvaluations = evaluateGroupVoiceProfiles(cfg, report.RequiredRelayDepth)
	report.Outcome = GroupVoiceOutcomeFail
	report.Reason = groupVoiceReasonNoProfileSupportsPlan

	if !report.CoveragePass || len(report.UncoveredMembers) > 0 || len(report.UnknownMembers) > 0 {
		report.Outcome = GroupVoiceOutcomeDegraded
		report.Reason = groupVoiceReasonNotAllMembersCovered
	} else {
		for i := range report.ProfileEvaluations {
			if report.ProfileEvaluations[i].Outcome != GroupVoiceOutcomePass || len(report.ProfileEvaluations[i].RejectReasons) > 0 {
				continue
			}
			report.ProfileEvaluations[i].Selected = true
			report.SelectedVoiceProfile = report.ProfileEvaluations[i].ProfileName
			report.ProfileAppliesToMembers = cloneGroupVoiceUint16s(report.ExpectedRecipients)
			report.Outcome = GroupVoiceOutcomePass
			report.Reason = groupVoiceReasonPass
			break
		}
	}

	hash, err := groupVoiceProfileReportHash(report)
	if err != nil {
		return GroupVoiceProfileReport{}, fmt.Errorf("group voice profile scenario: event hash: %w", err)
	}
	report.EventHash = hash
	return report, nil
}

func normalizeGroupVoiceProfileScenarioConfig(cfg GroupVoiceProfileScenarioConfig) GroupVoiceProfileScenarioConfig {
	if cfg.ScenarioID == "" {
		cfg.ScenarioID = GroupVoiceProfileScenarioID
	}
	if cfg.Seed == 0 {
		cfg.Seed = DefaultScenarioSeed
	}
	if cfg.CoverageMode == "" {
		cfg.CoverageMode = RelayElectionCoverageModeClosedGroup
	}
	if cfg.RequiredGroupCoverage == 0 {
		cfg.RequiredGroupCoverage = 1
	}
	if len(cfg.Profiles) == 0 {
		cfg.Profiles = DefaultGroupVoiceProfiles()
	}
	return cfg
}
