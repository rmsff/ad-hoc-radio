package scenario

import (
	"fmt"
	"math"

	"github.com/rmsff/ad-hoc-radio/internal/peer"
	"github.com/rmsff/ad-hoc-radio/internal/protocol"
	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
	"github.com/rmsff/ad-hoc-radio/internal/transport"
)

const (
	ScheduledRelayTreeDirectScenarioID              = "scheduled_relay_tree_direct"
	ScheduledRelayTreeOneRelayScenarioID            = "scheduled_relay_tree_one_relay"
	ScheduledRelayTreeTwoRelayScenarioID            = "scheduled_relay_tree_two_relay"
	ScheduledRelayTreeThreeRelayExceptionScenarioID = "scheduled_relay_tree_three_relay_exception"
	ScheduledRelayTreeBranchNarrowScenarioID        = "scheduled_relay_tree_branch_narrow"
	ScheduledRelayTreeBranchWideScenarioID          = "scheduled_relay_tree_branch_wide"
	ScheduledRelayTreePartitionRecoveryScenarioID   = "scheduled_relay_tree_partition_recovery"
)

var scheduledRelayTreeScenarioIDs = map[string]struct{}{
	ScheduledRelayTreeDirectScenarioID:              {},
	ScheduledRelayTreeOneRelayScenarioID:            {},
	ScheduledRelayTreeTwoRelayScenarioID:            {},
	ScheduledRelayTreeThreeRelayExceptionScenarioID: {},
	ScheduledRelayTreeBranchNarrowScenarioID:        {},
	ScheduledRelayTreeBranchWideScenarioID:          {},
	ScheduledRelayTreePartitionRecoveryScenarioID:   {},
}

type ScheduledRelayTreePlanConfig struct {
	Snapshot     routing.ExpectedGroupSnapshot
	Graph        routing.NeighborGraph
	Constraints  routing.PlanningConstraints
	CostModel    routing.PlanningCostModel
	SearchLimits routing.PlanningSearchLimits
	RelayStates  []routing.RelayState
}

type ScheduledRelayTreeReplanConfig struct {
	AtMS              int64
	Reason            ScheduledReplanReason
	Plan              ScheduledRelayTreePlanConfig
	TimingAuthorityID uint16
}

type ScheduledRelayTreeScenarioConfig struct {
	VoiceLinks             []transport.Link
	ExpectedServiceOutcome string
	Control                *peer.Config
	VoiceProfile           *timing.VoiceBudgetProfile
	ScenarioID             string
	Seed                   uint64
	FrameCount             uint32
	StartMS                int64
	NetID                  uint16
	GroupID                uint16
	SourceStationID        uint16
	CallID                 uint32
	ScheduleID             uint32
	ScheduleAuthorityID    uint16
	TimingStrategy         timing.TimingStrategy
	TimingEpochID          uint32
	TimingAuthorityID      uint16
	InitialPlan            ScheduledRelayTreePlanConfig
	Replans                []ScheduledRelayTreeReplanConfig
	OpportunityProfile     timing.OpportunityProfile
	ForwardQueueMaxDepth   int
	MissedParentThreshold  int
	AllowCycleTimingStress bool
}

func IsScheduledRelayTreeScenarioID(id string) bool {
	_, ok := scheduledRelayTreeScenarioIDs[id]
	return ok
}

func validateScheduledRelayTreeScenarioConfig(cfg ScheduledRelayTreeScenarioConfig) error {
	if len(cfg.VoiceLinks) > 0 && len(cfg.Replans) > 0 {
		return fmt.Errorf("scheduled scenario: explicit voice impairments with graph replans require a per-version link contract")
	}
	if !IsScheduledRelayTreeScenarioID(cfg.ScenarioID) {
		return fmt.Errorf("scheduled relay-tree scenario: unsupported scenario_id %q", cfg.ScenarioID)
	}
	if cfg.Seed == 0 {
		return fmt.Errorf("scheduled relay-tree scenario: seed must be > 0")
	}
	if cfg.FrameCount == 0 {
		return fmt.Errorf("scheduled relay-tree scenario: frame_count must be > 0")
	}
	if uint64(cfg.FrameCount) > uint64(maxIntValue()) {
		return fmt.Errorf("scheduled relay-tree scenario: frame_count=%d exceeds int range", cfg.FrameCount)
	}
	if cfg.StartMS < 0 {
		return fmt.Errorf("scheduled relay-tree scenario: start_ms=%d must be >= 0", cfg.StartMS)
	}
	if cfg.NetID == 0 || cfg.GroupID == 0 || cfg.SourceStationID == 0 || cfg.CallID == 0 || cfg.ScheduleID == 0 {
		return fmt.Errorf("scheduled relay-tree scenario: net_id, group_id, source_station_id, call_id, and schedule_id must be > 0")
	}
	if cfg.ScheduleAuthorityID == 0 || cfg.TimingAuthorityID == 0 || cfg.TimingEpochID == 0 {
		return fmt.Errorf("scheduled relay-tree scenario: schedule/timing authority and timing_epoch_id must be > 0")
	}
	switch cfg.TimingStrategy {
	case timing.TimingStrategySourceLed:
		if cfg.TimingAuthorityID != cfg.SourceStationID {
			return fmt.Errorf("scheduled relay-tree scenario: source_led timing_authority_id=%d must equal source_station_id=%d", cfg.TimingAuthorityID, cfg.SourceStationID)
		}
		if cfg.TimingEpochID != cfg.CallID {
			return fmt.Errorf("scheduled relay-tree scenario: source_led timing_epoch_id=%d must equal call_id=%d", cfg.TimingEpochID, cfg.CallID)
		}
	case timing.TimingStrategyGroupLeader:
		if cfg.TimingEpochID != 1 {
			return fmt.Errorf("scheduled relay-tree scenario: initial group_leader timing_epoch_id=%d must equal 1", cfg.TimingEpochID)
		}
	default:
		return fmt.Errorf("scheduled relay-tree scenario: timing_strategy=%q is invalid", cfg.TimingStrategy)
	}
	if cfg.ForwardQueueMaxDepth <= 0 {
		return fmt.Errorf("scheduled relay-tree scenario: forward_queue_max_depth must be > 0")
	}
	if cfg.MissedParentThreshold <= 0 {
		return fmt.Errorf("scheduled relay-tree scenario: missed_parent_threshold must be > 0")
	}
	if err := validateScheduledRelayTreeProfile(cfg.InitialPlan.CostModel, cfg.OpportunityProfile); err != nil {
		return err
	}
	if err := validateScheduledRelayTreePlan(cfg.SourceStationID, cfg.InitialPlan); err != nil {
		return fmt.Errorf("scheduled relay-tree scenario: initial plan: %w", err)
	}
	for i, replan := range cfg.Replans {
		if replan.AtMS < cfg.StartMS {
			return fmt.Errorf("scheduled relay-tree scenario: replan[%d] at_ms=%d is before start_ms=%d", i, replan.AtMS, cfg.StartMS)
		}
		if !validScheduledReplanReason(replan.Reason) {
			return fmt.Errorf("scheduled relay-tree scenario: replan[%d] reason=%q is invalid", i, replan.Reason)
		}
		if replan.Reason == ScheduledReplanMissedParentThreshold {
			return fmt.Errorf("scheduled relay-tree scenario: replan[%d] missed-parent templates are deferred from strict MVP-9.5 fixtures", i)
		}
		if replan.TimingAuthorityID != 0 {
			if cfg.TimingStrategy != timing.TimingStrategyGroupLeader || replan.Reason != ScheduledReplanTimingAuthorityLost {
				return fmt.Errorf("scheduled relay-tree scenario: replan[%d] timing_authority_id is valid only for group_leader timing_authority_lost", i)
			}
			if replan.TimingAuthorityID == cfg.TimingAuthorityID {
				return fmt.Errorf("scheduled relay-tree scenario: replan[%d] replacement timing_authority_id must differ from initial authority", i)
			}
		}
		if err := validateScheduledRelayTreeProfile(replan.Plan.CostModel, cfg.OpportunityProfile); err != nil {
			return fmt.Errorf("scheduled relay-tree scenario: replan[%d]: %w", i, err)
		}
		if replan.Plan.CostModel.FramePeriodMS != cfg.InitialPlan.CostModel.FramePeriodMS {
			return fmt.Errorf(
				"scheduled relay-tree scenario: replan[%d] frame_period_ms=%d must equal initial frame_period_ms=%d",
				i,
				replan.Plan.CostModel.FramePeriodMS,
				cfg.InitialPlan.CostModel.FramePeriodMS,
			)
		}
		if err := validateScheduledRelayTreePlan(cfg.SourceStationID, replan.Plan); err != nil {
			return fmt.Errorf("scheduled relay-tree scenario: replan[%d]: %w", i, err)
		}
	}
	if err := validateDistinctScheduledReplanEffectiveSequences(cfg); err != nil {
		return fmt.Errorf("scheduled relay-tree scenario: %w", err)
	}
	return nil
}

func validateDistinctScheduledReplanEffectiveSequences(cfg ScheduledRelayTreeScenarioConfig) error {
	seen := make(map[int64]int, len(cfg.Replans))
	for i, replan := range cfg.Replans {
		effectiveAt, hasEffective, err := nextTimingSourceTick(
			replan.AtMS,
			cfg.StartMS,
			cfg.InitialPlan.CostModel.FramePeriodMS,
			cfg.FrameCount,
		)
		if err != nil {
			return fmt.Errorf("replan[%d] effective source sequence: %w", i, err)
		}
		if !hasEffective {
			continue
		}
		if previous, ok := seen[effectiveAt]; ok {
			return fmt.Errorf(
				"replan[%d] at_ms=%d and replan[%d] at_ms=%d target the same effective source sequence at_ms=%d",
				previous,
				cfg.Replans[previous].AtMS,
				i,
				replan.AtMS,
				effectiveAt,
			)
		}
		seen[effectiveAt] = i
	}
	return nil
}

func validateScheduledRelayTreePlan(sourceID uint16, cfg ScheduledRelayTreePlanConfig) error {
	return routing.ValidatePlanRequest(scheduledRelayTreePlanRequest(sourceID, cfg))
}

func validateScheduledRelayTreeProfile(cost routing.PlanningCostModel, profile timing.OpportunityProfile) error {
	if cost.SourceOpportunityMS > math.MaxUint16 || cost.RelayOpportunityMS > math.MaxUint16 || cost.GuardAfterMS > math.MaxUint16 {
		return fmt.Errorf("scheduled relay-tree scenario: opportunity cost values must fit uint16")
	}
	if int64(profile.SourceDurationMS) != cost.SourceOpportunityMS ||
		int64(profile.RelayDurationMS) != cost.RelayOpportunityMS ||
		int64(profile.GuardAfterMS) != cost.GuardAfterMS {
		return fmt.Errorf("scheduled relay-tree scenario: opportunity_profile must match planning cost model durations and guard")
	}
	if cost.FramePeriodMS <= 0 || cost.FramePeriodMS > protocol.MaxFramePeriodMs {
		return fmt.Errorf("scheduled relay-tree scenario: frame_period_ms=%d must be in 1..%d", cost.FramePeriodMS, protocol.MaxFramePeriodMs)
	}
	if profile.MaxCycleMS == 0 || int64(profile.MaxCycleMS) > cost.FramePeriodMS {
		return fmt.Errorf("scheduled relay-tree scenario: max_cycle_ms=%d must be within frame_period_ms=%d", profile.MaxCycleMS, cost.FramePeriodMS)
	}
	return nil
}

func scheduledRelayTreePlanRequest(sourceID uint16, cfg ScheduledRelayTreePlanConfig) routing.PlanRequest {
	return routing.PlanRequest{
		SourceStationID: sourceID,
		Snapshot:        cloneScheduledRelayTreeSnapshot(cfg.Snapshot),
		Graph:           cloneScheduledRelayTreeGraph(cfg.Graph),
		Constraints:     cfg.Constraints,
		CostModel:       cfg.CostModel,
		SearchLimits:    cfg.SearchLimits,
		RelayStates:     append([]routing.RelayState(nil), cfg.RelayStates...),
	}
}

func cloneScheduledRelayTreeSnapshot(snapshot routing.ExpectedGroupSnapshot) routing.ExpectedGroupSnapshot {
	snapshot.MemberIDs = append([]uint16(nil), snapshot.MemberIDs...)
	return snapshot
}

func cloneScheduledRelayTreeGraph(graph routing.NeighborGraph) routing.NeighborGraph {
	graph.Links = append([]routing.DirectedLink(nil), graph.Links...)
	return graph
}

func maxIntValue() int {
	return int(^uint(0) >> 1)
}
