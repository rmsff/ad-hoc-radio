package scenario

import (
	"fmt"
	"sort"

	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

const (
	stressStartMS       int64  = 1_000
	stressObservedAtMS  int64  = 900
	stressFreshnessMS   uint32 = 1_000
	stressFramePeriodMS int64  = 80
)

func BuildDefaultRelayTreeStressCases() []RelayTreeStressCase {
	cases := make([]RelayTreeStressCase, 0, 30)
	for _, groupSize := range []int{2, 4, 8, 16} {
		for depth := uint8(0); depth <= routing.MaxSupportedRelayDepth; depth++ {
			cases = append(cases, newStressCoreCase(groupSize, depth))
		}
	}
	for _, spec := range []struct {
		width int
		size  int
	}{{2, 4}, {4, 8}, {8, 16}} {
		cases = append(cases, newStressBranchCase(spec.width, spec.size))
	}
	cases = append(cases,
		newStressAsymmetricCase(),
		newStressPartitionCase(false),
		newStressPartitionCase(true),
		newStressMembershipCase(RelayTreeStressTagStaleMembership),
		newStressMembershipCase(RelayTreeStressTagMissingMember),
		newStressLateJoinDriftCase(),
		newStressLinkBoundaryCase(false),
		newStressLinkBoundaryCase(true),
		newStressRelayStateBoundaryCase(),
		newStressRelayLossCase(false),
		newStressRelayLossCase(true),
	)
	return cases
}

func newStressCoreCase(groupSize int, depth uint8) RelayTreeStressCase {
	caseID := fmt.Sprintf("core-g%02d-d%d", groupSize, depth)
	members := stressMemberIDs(groupSize)
	plan := stressChainPlan(caseID, members, depth)
	tags := []RelayTreeStressTag{RelayTreeStressTagNominal, RelayTreeStressTagChain}
	if depth == 0 {
		tags = []RelayTreeStressTag{RelayTreeStressTagNominal, RelayTreeStressTagDirect}
	}
	return RelayTreeStressCase{
		CaseID:              caseID,
		Class:               RelayTreeStressClassNominal,
		Role:                RelayTreeStressRoleCore,
		ExpectedGroupSize:   groupSize,
		RequestedRelayDepth: depth,
		StressTags:          sortedStressTags(tags),
		BaseConfig:          stressScenarioConfig(caseID, plan, depth),
		TimingOverlay:       stressTimingOverlay(members),
		Expectation: RelayTreeStressExpectation{
			RequireAllDelivered: true,
		},
	}
}

func newStressBranchCase(width, groupSize int) RelayTreeStressCase {
	caseID := fmt.Sprintf("branch-w%02d-g%02d", width, groupSize)
	members := stressMemberIDs(groupSize)
	plan := stressBranchPlan(caseID, members, width)
	cfg := stressScenarioConfig(caseID, plan, 1)
	if width > 2 {
		cfg.ScenarioID = ScheduledRelayTreeBranchWideScenarioID
	} else {
		cfg.ScenarioID = ScheduledRelayTreeBranchNarrowScenarioID
	}
	return RelayTreeStressCase{
		CaseID:              caseID,
		Class:               RelayTreeStressClassBranch,
		Role:                RelayTreeStressRoleBoundary,
		ExpectedGroupSize:   groupSize,
		RequestedRelayDepth: 1,
		BranchWidth:         width,
		StressTags:          sortedStressTags([]RelayTreeStressTag{RelayTreeStressTagBranch}),
		BaseConfig:          cfg,
		TimingOverlay:       stressTimingOverlay(members),
		Expectation: RelayTreeStressExpectation{
			RequireAllDelivered:   true,
			AllowProfileRejection: true,
		},
	}
}

func newStressAsymmetricCase() RelayTreeStressCase {
	caseID := "asymmetric-directed-one-relay"
	members := stressMemberIDs(4)
	plan := stressChainPlan(caseID, members, 1)
	return RelayTreeStressCase{
		CaseID:              caseID,
		Class:               RelayTreeStressClassAsymmetric,
		Role:                RelayTreeStressRoleResilience,
		ExpectedGroupSize:   len(members),
		RequestedRelayDepth: 1,
		StressTags:          sortedStressTags([]RelayTreeStressTag{RelayTreeStressTagAsymmetricLink}),
		BaseConfig:          stressScenarioConfig(caseID, plan, 1),
		TimingOverlay:       stressTimingOverlay(members),
		Expectation: RelayTreeStressExpectation{
			RequireAllDelivered: true,
		},
	}
}

func newStressPartitionCase(recovery bool) RelayTreeStressCase {
	caseID := "partition-partial"
	if recovery {
		caseID = "partition-recovery"
	}
	members := stressMemberIDs(4)
	fullPlan := stressChainPlan(caseID+"-full", members, 0)
	partialPlan := cloneTimingRelayTreePlan(fullPlan)
	partialPlan.Graph.Links = append([]routing.DirectedLink(nil), partialPlan.Graph.Links[:2]...)
	partialPlan.Constraints.RequireAllExpectedMembers = false
	cfg := stressScenarioConfig(caseID, partialPlan, 0)
	cfg.ScenarioID = ScheduledRelayTreePartitionRecoveryScenarioID
	expectation := RelayTreeStressExpectation{
		ExpectedUncoveredMemberIDs: append([]uint16(nil), members[2:]...),
	}
	tags := []RelayTreeStressTag{RelayTreeStressTagPartition}
	if recovery {
		replan := stressPlanAt(fullPlan, 1_120, 1)
		cfg.Replans = []ScheduledRelayTreeReplanConfig{{
			AtMS:   1_120,
			Reason: ScheduledReplanTopologyEpochChanged,
			Plan:   replan,
		}}
		expectation = RelayTreeStressExpectation{
			RequireAllDelivered:   true,
			MinRouteReplans:       1,
			MinVersionActivations: 1,
		}
		tags = append(tags, RelayTreeStressTagRecovery)
	}
	return RelayTreeStressCase{
		CaseID:              caseID,
		Class:               RelayTreeStressClassPartition,
		Role:                RelayTreeStressRoleResilience,
		ExpectedGroupSize:   len(members),
		RequestedRelayDepth: 0,
		StressTags:          sortedStressTags(tags),
		BaseConfig:          cfg,
		TimingOverlay:       stressTimingOverlay(members),
		Expectation:         expectation,
	}
}

func newStressMembershipCase(tag RelayTreeStressTag) RelayTreeStressCase {
	caseID := "membership-stale"
	if tag == RelayTreeStressTagMissingMember {
		caseID = "membership-missing"
	}
	members := stressMemberIDs(4)
	plan := stressChainPlan(caseID, members, 0)
	plan.Graph.Links = append([]routing.DirectedLink(nil), plan.Graph.Links[:3]...)
	plan.Constraints.RequireAllExpectedMembers = false
	return RelayTreeStressCase{
		CaseID:              caseID,
		Class:               RelayTreeStressClassMembership,
		Role:                RelayTreeStressRoleResilience,
		ExpectedGroupSize:   len(members),
		RequestedRelayDepth: 0,
		StressTags:          sortedStressTags([]RelayTreeStressTag{tag}),
		BaseConfig:          stressScenarioConfig(caseID, plan, 0),
		TimingOverlay:       stressTimingOverlay(members),
		Expectation: RelayTreeStressExpectation{
			ExpectedUncoveredMemberIDs: []uint16{members[len(members)-1]},
		},
	}
}

func newStressLateJoinDriftCase() RelayTreeStressCase {
	caseID := "membership-late-join-drift"
	members := stressMemberIDs(4)
	plan := stressChainPlan(caseID, members, 0)
	overlay := stressTimingOverlay(members)
	joinAt := int64(1_041)
	overlay.Peers[1].JoinAtMS = &joinAt
	overlay.Peers[1].DriftPPM = maxTimingDriftPPM
	return RelayTreeStressCase{
		CaseID:              caseID,
		Class:               RelayTreeStressClassMembership,
		Role:                RelayTreeStressRoleResilience,
		ExpectedGroupSize:   len(members),
		RequestedRelayDepth: 0,
		StressTags: sortedStressTags([]RelayTreeStressTag{
			RelayTreeStressTagLateJoin,
			RelayTreeStressTagClockDrift,
		}),
		BaseConfig:    stressScenarioConfig(caseID, plan, 0),
		TimingOverlay: overlay,
		Expectation: RelayTreeStressExpectation{
			RequireAllDelivered: true,
		},
	}
}

func newStressLinkBoundaryCase(rejected bool) RelayTreeStressCase {
	caseID := "link-boundary-accepted"
	if rejected {
		caseID = "link-boundary-rejected"
	}
	members := stressMemberIDs(4)
	plan := stressChainPlan(caseID, members, 1)
	plan.Constraints.MaxLinkLoss = 0.2
	for i := range plan.Graph.Links {
		plan.Graph.Links[i].Loss = 0.2
		plan.Graph.Links[i].DelayMS = 1
	}
	if rejected {
		plan.Graph.Links[len(plan.Graph.Links)-1].Loss = 0.21
		plan.Graph.Links[len(plan.Graph.Links)-1].DelayMS = 300
	}
	return RelayTreeStressCase{
		CaseID:              caseID,
		Class:               RelayTreeStressClassLink,
		Role:                RelayTreeStressRoleBoundary,
		ExpectedGroupSize:   len(members),
		RequestedRelayDepth: 1,
		StressTags: sortedStressTags([]RelayTreeStressTag{
			RelayTreeStressTagLinkLoss,
			RelayTreeStressTagLinkDelay,
		}),
		BaseConfig:    stressScenarioConfig(caseID, plan, 1),
		TimingOverlay: stressTimingOverlay(members),
		Expectation: RelayTreeStressExpectation{
			RequireAllDelivered:   !rejected,
			AllowProfileRejection: rejected,
		},
	}
}

func newStressRelayStateBoundaryCase() RelayTreeStressCase {
	caseID := "relay-state-busy-low-battery"
	members := stressMemberIDs(4)
	plan := stressChainPlan(caseID, members, 1)
	plan.Constraints.MinRelayBatteryFraction = 0.2
	plan.RelayStates[0].Busy = true
	plan.RelayStates[0].BatteryFraction = 0.1
	return RelayTreeStressCase{
		CaseID:              caseID,
		Class:               RelayTreeStressClassRelay,
		Role:                RelayTreeStressRoleBoundary,
		ExpectedGroupSize:   len(members),
		RequestedRelayDepth: 1,
		StressTags: sortedStressTags([]RelayTreeStressTag{
			RelayTreeStressTagRelayBusy,
			RelayTreeStressTagRelayLowBattery,
		}),
		BaseConfig:    stressScenarioConfig(caseID, plan, 1),
		TimingOverlay: stressTimingOverlay(members),
		Expectation: RelayTreeStressExpectation{
			AllowProfileRejection: true,
		},
	}
}

func newStressRelayLossCase(combined bool) RelayTreeStressCase {
	caseID := "relay-loss-replacement"
	class := RelayTreeStressClassRelay
	tags := []RelayTreeStressTag{RelayTreeStressTagRelayLoss, RelayTreeStressTagRecovery}
	if combined {
		caseID = "combined-leader-relay-loss"
		class = RelayTreeStressClassCombined
		tags = append(tags, RelayTreeStressTagLeaderLoss)
	}
	members := stressMemberIDs(4)
	initialPlan := stressChainPlan(caseID+"-initial", members, 1)
	replacementPlan := stressChainPlanWithRelay(caseID+"-replacement", members, 101)
	replacementPlan = stressPlanAt(replacementPlan, 1_120, 1)
	cfg := stressScenarioConfig(caseID, initialPlan, 1)
	cfg.FrameCount = 5
	cfg.Replans = []ScheduledRelayTreeReplanConfig{{
		AtMS:   1_120,
		Reason: ScheduledReplanSelectedRelayUnavailable,
		Plan:   replacementPlan,
	}}
	overlay := stressTimingOverlay(members)
	if combined {
		overlay.LeaderLoss = &TimingAuthorityLoss{
			AtMS:              1_040,
			LostStationID:     members[0],
			LeaderReacquireMS: 0,
		}
	}
	return RelayTreeStressCase{
		CaseID:              caseID,
		Class:               class,
		Role:                RelayTreeStressRoleResilience,
		ExpectedGroupSize:   len(members),
		RequestedRelayDepth: 1,
		StressTags:          sortedStressTags(tags),
		BaseConfig:          cfg,
		TimingOverlay:       overlay,
		Expectation: RelayTreeStressExpectation{
			RequireAllDelivered:   true,
			MinRouteReplans:       1,
			MinVersionActivations: 1,
		},
	}
}

func stressChainPlan(caseID string, members []uint16, depth uint8) ScheduledRelayTreePlanConfig {
	if depth == 0 {
		links := make([]routing.DirectedLink, 0, len(members))
		for _, memberID := range members {
			links = append(links, stressDirectedLink(1, memberID))
		}
		return stressPlan(caseID, members, links, nil)
	}
	relays := make([]uint16, depth)
	for i := range relays {
		relays[i] = uint16(100 + i)
	}
	return stressChainPlanForRelays(caseID, members, relays)
}

func stressChainPlanWithRelay(caseID string, members []uint16, relayID uint16) ScheduledRelayTreePlanConfig {
	return stressChainPlanForRelays(caseID, members, []uint16{relayID})
}

func stressChainPlanForRelays(caseID string, members []uint16, relays []uint16) ScheduledRelayTreePlanConfig {
	links := make([]routing.DirectedLink, 0, len(relays)+len(members))
	parent := uint16(1)
	states := make([]routing.RelayState, 0, len(relays))
	for _, relayID := range relays {
		links = append(links, stressDirectedLink(parent, relayID))
		states = append(states, stressRelayState(relayID))
		parent = relayID
	}
	for _, memberID := range members {
		links = append(links, stressDirectedLink(parent, memberID))
	}
	return stressPlan(caseID, members, links, states)
}

func stressBranchPlan(caseID string, members []uint16, width int) ScheduledRelayTreePlanConfig {
	links := make([]routing.DirectedLink, 0, width+len(members))
	states := make([]routing.RelayState, 0, width)
	for i := 0; i < width; i++ {
		relayID := uint16(100 + i)
		links = append(links, stressDirectedLink(1, relayID))
		states = append(states, stressRelayState(relayID))
	}
	for i, memberID := range members {
		links = append(links, stressDirectedLink(uint16(100+i%width), memberID))
	}
	return stressPlan(caseID, members, links, states)
}

func stressPlan(caseID string, members []uint16, links []routing.DirectedLink, states []routing.RelayState) ScheduledRelayTreePlanConfig {
	id := stressStableID(caseID)
	orderedLinks := append([]routing.DirectedLink(nil), links...)
	sort.Slice(orderedLinks, func(i, j int) bool {
		if orderedLinks[i].FromStationID != orderedLinks[j].FromStationID {
			return orderedLinks[i].FromStationID < orderedLinks[j].FromStationID
		}
		return orderedLinks[i].ToStationID < orderedLinks[j].ToStationID
	})
	return ScheduledRelayTreePlanConfig{
		Snapshot: routing.ExpectedGroupSnapshot{
			ID:           id,
			ObservedAtMS: stressObservedAtMS,
			FreshnessMS:  stressFreshnessMS,
			MemberIDs:    append([]uint16(nil), members...),
		},
		Graph: routing.NeighborGraph{
			Epoch:       id,
			FreshnessMS: stressFreshnessMS,
			Links:       orderedLinks,
		},
		Constraints: routing.PlanningConstraints{
			PlanningAtMS:              stressStartMS,
			MaxRelayDepth:             routing.MaxSupportedRelayDepth,
			MaxPlayoutLatencyMS:       250,
			MaxVoiceAirtimePct:        100,
			RequireAllExpectedMembers: true,
			MinRelayBatteryFraction:   0.2,
			MinLinkQuality:            0.5,
			MaxLinkLoss:               0.2,
		},
		CostModel: routing.PlanningCostModel{
			SourceOpportunityMS: 10,
			RelayOpportunityMS:  10,
			GuardAfterMS:        1,
			FramePeriodMS:       stressFramePeriodMS,
		},
		SearchLimits: routing.PlanningSearchLimits{
			MaxPathsPerMember: 8,
			MaxTreeCandidates: 4_096,
			MaxFeasibleTrees:  4_096,
		},
		RelayStates: append([]routing.RelayState(nil), states...),
	}
}

func stressPlanAt(in ScheduledRelayTreePlanConfig, atMS int64, version uint32) ScheduledRelayTreePlanConfig {
	out := cloneTimingRelayTreePlan(in)
	out.Snapshot.ID += version
	out.Snapshot.ObservedAtMS = atMS
	out.Graph.Epoch += version
	for i := range out.Graph.Links {
		out.Graph.Links[i].ObservedAtMS = atMS
	}
	out.Constraints.PlanningAtMS = atMS
	return out
}

func stressTimingOverlay(members []uint16) TimingProfileComparisonOverlay {
	return TimingProfileComparisonOverlay{
		Peers: []TimingAuthorityPeer{
			{
				StationID:     members[0],
				TimingCapable: true,
				LastSeenMS:    stressObservedAtMS,
				FreshnessMS:   stressFreshnessMS,
				BatteryPct:    90,
			},
			{
				StationID:     members[1],
				TimingCapable: true,
				LastSeenMS:    stressObservedAtMS,
				FreshnessMS:   stressFreshnessMS,
				BatteryPct:    90,
			},
		},
		ConfiguredLeaderID:  members[0],
		MinTimingBatteryPct: 20,
		SyncToleranceMS:     1,
	}
}

func stressOpportunityProfile() timing.OpportunityProfile {
	return timing.OpportunityProfile{
		SourceDurationMS: 10,
		RelayDurationMS:  10,
		GuardAfterMS:     1,
		MaxCycleMS:       uint32(stressFramePeriodMS),
	}
}

func stressDirectedLink(fromID, toID uint16) routing.DirectedLink {
	return routing.DirectedLink{
		FromStationID: fromID,
		ToStationID:   toID,
		Enabled:       true,
		Quality:       0.9,
		Loss:          0.01,
		DelayMS:       1,
		ObservedAtMS:  stressObservedAtMS,
	}
}

func stressRelayState(stationID uint16) routing.RelayState {
	return routing.RelayState{
		StationID:       stationID,
		RelayCapable:    true,
		RelayEnabled:    true,
		BatteryFraction: 0.9,
	}
}

func stressMemberIDs(size int) []uint16 {
	members := make([]uint16, size)
	for i := range members {
		members[i] = uint16(2 + i)
	}
	return members
}

func stressStableID(value string) uint32 {
	const offset uint32 = 2_166_136_261
	const prime uint32 = 16_777_619
	hash := offset
	for i := 0; i < len(value); i++ {
		hash ^= uint32(value[i])
		hash *= prime
	}
	if hash == 0 {
		return 1
	}
	return hash
}
