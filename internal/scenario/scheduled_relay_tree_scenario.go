package scenario

import (
	"fmt"
	"sort"

	"github.com/rmsff/ad-hoc-radio/internal/node"
	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

func RunScheduledRelayTreeScenario(cfg ScheduledRelayTreeScenarioConfig) (ScenarioMetrics, error) {
	if cfg.ExpectedServiceOutcome != "" && cfg.ExpectedServiceOutcome != "pass" && cfg.ExpectedServiceOutcome != "fail" {
		return ScenarioMetrics{}, fmt.Errorf("scheduled scenario: expected_service_outcome must be pass or fail")
	}
	if cfg.VoiceProfile != nil {
		profile, err := scheduledVoiceOpportunityProfileWithControl(*cfg.VoiceProfile, cfg.Control != nil)
		if err != nil {
			return ScenarioMetrics{}, err
		}
		cfg.OpportunityProfile = profile
		cost := routing.PlanningCostModel{SourceOpportunityMS: int64(profile.SourceDurationMS), RelayOpportunityMS: int64(profile.RelayDurationMS), GuardAfterMS: int64(profile.GuardAfterMS), FramePeriodMS: cfg.VoiceProfile.PacketPeriodMS}
		cfg.InitialPlan.CostModel = cost
		cfg.Replans = append([]ScheduledRelayTreeReplanConfig(nil), cfg.Replans...)
		for i := range cfg.Replans {
			cfg.Replans[i].Plan.CostModel = cost
		}
	}
	if err := validateScheduledRelayTreeScenarioConfig(cfg); err != nil {
		return ScenarioMetrics{}, err
	}
	initialRequest := scheduledRelayTreePlanRequest(cfg.SourceStationID, cfg.InitialPlan)
	plan, err := routing.PlanRelayTree(initialRequest)
	if err != nil {
		return ScenarioMetrics{}, fmt.Errorf("scheduled relay-tree scenario: initial planning: %w", err)
	}
	initialBinding := scheduledRelayTreeBinding(cfg, cfg.InitialPlan, 1, 0)
	initialSchedule, err := timing.BuildOpportunitySchedule(initialBinding, plan.Selected.Tree, cfg.OpportunityProfile)
	if err != nil {
		return ScenarioMetrics{}, fmt.Errorf("scheduled relay-tree scenario: initial schedule: %w", err)
	}
	initialVersion := ScheduledVersion{
		Links:       selectedTreeLinks(plan.Selected.Tree, cfg.InitialPlan.Graph),
		Tree:        plan.Selected.Tree,
		Schedule:    initialSchedule,
		LinkDelayMS: selectedTreeLinkDelays(plan.Selected.Tree, cfg.InitialPlan.Graph),
	}
	if err := applyVoiceLinkOverrides(&initialVersion, cfg.VoiceLinks); err != nil {
		return ScenarioMetrics{}, err
	}

	runtimeCfg := ScheduledRuntimeConfig{
		Control:                cfg.Control,
		VoiceProfile:           cfg.VoiceProfile,
		Seed:                   cfg.Seed,
		Versions:               []ScheduledVersion{initialVersion},
		ReplanInputs:           scheduledRelayTreeReplanInputs(cfg),
		Source:                 scheduledRelayTreeSource(cfg),
		Relays:                 scheduledRelayTreeRelays(cfg, initialSchedule),
		Receivers:              scheduledRelayTreeReceivers(cfg, initialVersion.Tree.Coverage.CoveredMemberIDs),
		ForwardQueueMaxDepth:   cfg.ForwardQueueMaxDepth,
		FrameCount:             cfg.FrameCount,
		AllowCycleTimingStress: cfg.AllowCycleTimingStress,
		MissedParentThreshold:  cfg.MissedParentThreshold,
	}
	result, err := RunScheduledRuntime(runtimeCfg)
	if err != nil {
		return ScenarioMetrics{}, fmt.Errorf("scheduled relay-tree scenario: runtime: %w", err)
	}
	return scheduledRelayTreeScenarioMetrics(cfg, result), nil
}

func scheduledRelayTreeSource(cfg ScheduledRelayTreeScenarioConfig) *SourceStation {
	sourceCfg := DefaultScenarioConfig()
	sourceCfg.ScenarioID = cfg.ScenarioID
	sourceCfg.Seed = cfg.Seed
	sourceCfg.FrameCount = int(cfg.FrameCount)
	sourceCfg.FramePeriodMs = cfg.InitialPlan.CostModel.FramePeriodMS
	sourceCfg.StartMs = cfg.StartMS
	sourceCfg.SourceID = cfg.SourceStationID
	sourceCfg.NetID = cfg.NetID
	sourceCfg.GroupID = cfg.GroupID
	sourceCfg.CallID = cfg.CallID
	sourceCfg.TTL = routing.MaxSupportedRelayDepth
	return NewSourceStation(sourceCfg)
}

func scheduledRelayTreeBinding(
	cfg ScheduledRelayTreeScenarioConfig,
	plan ScheduledRelayTreePlanConfig,
	version uint16,
	effectiveSeq uint32,
) timing.ScheduleBinding {
	return timing.ScheduleBinding{
		ScheduleID:             cfg.ScheduleID,
		ScheduleVersion:        version,
		NetID:                  cfg.NetID,
		GroupID:                cfg.GroupID,
		SourceStationID:        cfg.SourceStationID,
		CallID:                 cfg.CallID,
		ScheduleAuthorityID:    cfg.ScheduleAuthorityID,
		TopologyEpoch:          plan.Graph.Epoch,
		TopologyObservedAtMS:   scheduledRelayTreeTopologyObservedAt(plan.Graph),
		MembershipSnapshotID:   plan.Snapshot.ID,
		MembershipObservedAtMS: plan.Snapshot.ObservedAtMS,
		MembershipFreshnessMS:  plan.Snapshot.FreshnessMS,
		TimingStrategy:         cfg.TimingStrategy,
		TimingAuthorityID:      cfg.TimingAuthorityID,
		EffectiveSeq:           effectiveSeq,
		ValidThroughSeq:        cfg.FrameCount - 1,
		ExpectedMemberIDs:      append([]uint16(nil), plan.Snapshot.MemberIDs...),
	}
}

func scheduledRelayTreeTopologyObservedAt(graph routing.NeighborGraph) int64 {
	var observedAt int64
	for _, link := range graph.Links {
		if link.ObservedAtMS > observedAt {
			observedAt = link.ObservedAtMS
		}
	}
	return observedAt
}

func scheduledRelayTreeReplanInputs(cfg ScheduledRelayTreeScenarioConfig) []ScheduledReplanInput {
	replans := append([]ScheduledRelayTreeReplanConfig(nil), cfg.Replans...)
	sort.SliceStable(replans, func(i, j int) bool { return replans[i].AtMS < replans[j].AtMS })
	inputs := make([]ScheduledReplanInput, 0, len(replans))
	timingAuthorityID := cfg.TimingAuthorityID
	for _, replan := range replans {
		// The common runtime replaces these placeholder version/sequence fields
		// with current+1 and the next uncreated source sequence.
		binding := scheduledRelayTreeBinding(cfg, replan.Plan, 1, 0)
		if replan.TimingAuthorityID != 0 {
			timingAuthorityID = replan.TimingAuthorityID
		}
		binding.TimingAuthorityID = timingAuthorityID
		inputs = append(inputs, ScheduledReplanInput{
			AtMS:        replan.AtMS,
			Reason:      replan.Reason,
			PlanRequest: scheduledRelayTreePlanRequest(cfg.SourceStationID, replan.Plan),
			Binding:     binding,
			Profile:     cfg.OpportunityProfile,
		})
	}
	return inputs
}

func scheduledRelayTreeRelays(cfg ScheduledRelayTreeScenarioConfig, initial timing.OpportunitySchedule) map[uint16]*node.RelayStation {
	ids := make(map[uint16]struct{})
	// Candidates remain planner inputs; only installed or replan-declared owners
	// belong to the runtime inventory.
	for _, opportunity := range initial.Opportunities[1:] {
		ids[opportunity.OwnerStationID] = struct{}{}
	}
	for _, replan := range cfg.Replans {
		for _, state := range replan.Plan.RelayStates {
			ids[state.StationID] = struct{}{}
		}
	}
	relays := make(map[uint16]*node.RelayStation, len(ids))
	for id := range ids {
		relays[id] = node.NewRelayStation(
			id,
			node.NewRelayLifecycle(node.DefaultLifecyclePolicy()),
			node.DefaultRelayPolicy(),
		)
	}
	return relays
}

func scheduledRelayTreeReceivers(cfg ScheduledRelayTreeScenarioConfig, initialCoveredMemberIDs []uint16) map[uint16]*node.ReceiverStation {
	ids := make(map[uint16]struct{})
	for _, id := range initialCoveredMemberIDs {
		ids[id] = struct{}{}
	}
	for _, replan := range cfg.Replans {
		for _, id := range replan.Plan.Snapshot.MemberIDs {
			ids[id] = struct{}{}
		}
	}
	ordered := make([]int, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, int(id))
	}
	sort.Ints(ordered)
	receivers := make(map[uint16]*node.ReceiverStation, len(ids))
	for _, rawID := range ordered {
		id := uint16(rawID)
		receivers[id] = node.NewReceiverStation(
			id,
			node.NewReceiverLifecycle(node.DefaultLifecyclePolicy()),
			node.DefaultReceiverPolicy(),
		)
	}
	return receivers
}
