package routing

import "errors"

const MaxSupportedRelayDepth uint8 = 3

var (
	ErrNoFeasibleRelayTree         = errors.New("no feasible relay tree")
	ErrPlanningSearchLimitExceeded = errors.New("planning search limit exceeded")
)

type ExpectedGroupSnapshot struct {
	ID           uint32
	ObservedAtMS int64
	FreshnessMS  uint32
	MemberIDs    []uint16
}

type DirectedLink struct {
	FromStationID uint16
	ToStationID   uint16
	Enabled       bool
	Quality       float64
	Loss          float64
	DelayMS       int64
	ObservedAtMS  int64
}

type NeighborGraph struct {
	Epoch       uint32
	FreshnessMS uint32
	Links       []DirectedLink
}

// RelayTreeEdge describes the transmission that reaches ChildStationID.
// Hop is zero for a source transmission and one through three for relay copies.
type RelayTreeEdge struct {
	ParentStationID uint16
	ChildStationID  uint16
	Hop             uint8
}

type CoverageSet struct {
	CoveredMemberIDs   []uint16
	UncoveredMemberIDs []uint16
}

type RelayTree struct {
	SourceStationID      uint16
	MembershipSnapshotID uint32
	TopologyEpoch        uint32
	MaxRelayDepth        uint8
	Edges                []RelayTreeEdge
	Coverage             CoverageSet
}

type PlanningConstraints struct {
	PlanningAtMS              int64
	MaxRelayDepth             uint8
	MaxPlayoutLatencyMS       int64
	MaxVoiceAirtimePct        float64
	RequireAllExpectedMembers bool
	MinRelayBatteryFraction   float64
	MinLinkQuality            float64
	MaxLinkLoss               float64
}

type RelayState struct {
	StationID       uint16
	RelayCapable    bool
	RelayEnabled    bool
	Busy            bool
	BatteryFraction float64
}

type PlanningCostModel struct {
	SourceOpportunityMS int64
	RelayOpportunityMS  int64
	GuardAfterMS        int64
	FramePeriodMS       int64
}

type PlanningSearchLimits struct {
	MaxPathsPerMember int
	MaxTreeCandidates int
	MaxFeasibleTrees  int
	// MaxSearchSteps bounds path expansion and branch visits in PlanRelayTree.
	// Zero derives a checked bound from MaxTreeCandidates, member count and depth.
	MaxSearchSteps int
}

type PlanRequest struct {
	SourceStationID uint16
	Snapshot        ExpectedGroupSnapshot
	Graph           NeighborGraph
	Constraints     PlanningConstraints
	CostModel       PlanningCostModel
	SearchLimits    PlanningSearchLimits
	RelayStates     []RelayState
}

type TreeCost struct {
	CoveredMemberCount            int
	SelectedMaxRelayDepth         uint8
	TxOpportunityCount            int
	AggregateLinkRiskUnits        int64
	AggregateBatteryCostUnits     int64
	EstimatedScheduleCycleMS      int64
	EstimatedMaxDeliveryLatencyMS int64
	VoiceAirtimeMilliPct          int64
}

type EvaluatedTree struct {
	Tree          RelayTree
	TreeHash      string
	CanonicalJSON string
	Cost          TreeCost
}

type PlanResult struct {
	Selected                EvaluatedTree
	Feasible                []EvaluatedTree
	EvaluatedCandidateCount int
	// Only EnumerateFeasibleRelayTrees promises the complete Feasible list.
	CompleteEnumeration bool
	Diagnostics         PlanningDiagnostics
}
