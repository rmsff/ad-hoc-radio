package scenario

import (
	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
	"sort"
)

// Forced schedules are diagnostic only. Ordinary admission never uses this path.
func runPeerAcceptanceStress(depth uint8, width int, seed uint64) (PeerAcceptanceRow, error) {
	cfg := peerAcceptanceConfig(depth, width, 19200, seed)
	controlStart := cfg.Control.StartMS
	cfg.Control = nil
	profile, err := scheduledVoiceOpportunityProfile(*cfg.VoiceProfile)
	if err != nil {
		return PeerAcceptanceRow{}, err
	}
	cfg.InitialPlan.CostModel = routing.PlanningCostModel{SourceOpportunityMS: int64(profile.SourceDurationMS), RelayOpportunityMS: int64(profile.RelayDurationMS), GuardAfterMS: int64(profile.GuardAfterMS), FramePeriodMS: 80}
	tree := routing.RelayTree{SourceStationID: 1, MembershipSnapshotID: 1, TopologyEpoch: 1, MaxRelayDepth: depth, Coverage: routing.CoverageSet{CoveredMemberIDs: append([]uint16(nil), cfg.InitialPlan.Snapshot.MemberIDs...)}}
	next := uint16(2)
	for branch := 0; branch < width; branch++ {
		parent := uint16(1)
		for hop := uint8(0); hop <= depth; hop++ {
			tree.Edges = append(tree.Edges, routing.RelayTreeEdge{ParentStationID: parent, ChildStationID: next, Hop: hop})
			parent = next
			next++
		}
	}
	// Canonical tree helpers require parent/child ordering, not branch order.
	sort.Slice(tree.Edges, func(i, j int) bool {
		a, b := tree.Edges[i], tree.Edges[j]
		if a.ParentStationID != b.ParentStationID {
			return a.ParentStationID < b.ParentStationID
		}
		return a.ChildStationID < b.ChildStationID
	})
	profile.MaxCycleMS = 10000
	schedule, err := timing.BuildOpportunitySchedule(scheduledRelayTreeBinding(cfg, cfg.InitialPlan, 1, 0), tree, profile)
	if err != nil {
		return PeerAcceptanceRow{}, err
	}
	v := ScheduledVersion{Tree: tree, Schedule: schedule, LinkDelayMS: selectedTreeLinkDelays(tree, cfg.InitialPlan.Graph), Links: selectedTreeLinks(tree, cfg.InitialPlan.Graph)}
	r, err := RunScheduledRuntime(ScheduledRuntimeConfig{Source: scheduledRelayTreeSource(cfg), Versions: []ScheduledVersion{v}, Relays: scheduledRelayTreeRelays(cfg, schedule), Receivers: scheduledRelayTreeReceivers(cfg, tree.Coverage.CoveredMemberIDs), VoiceProfile: cfg.VoiceProfile, Control: cfg.Control, FrameCount: cfg.FrameCount, Seed: seed, ForwardQueueMaxDepth: 8, MissedParentThreshold: 1000, AllowCycleTimingStress: true})
	if err != nil {
		return PeerAcceptanceRow{}, err
	}
	m := scheduledRelayTreeScenarioMetrics(cfg, r)
	row := PeerAcceptanceRow{ID: "forced-cycle-overload-diagnostic", Seed: seed, Depth: depth, Width: width, GrossBPS: 19200, Admission: "stress_override_not_admissible", ServiceOutcome: m.ScheduledRelayTree.Audio.ServiceOutcome, ExpectedFrames: len(cfg.InitialPlan.Snapshot.MemberIDs) * int(cfg.FrameCount), QueueMaxDepth: r.Counters.ForwardQueueMaxDepth, QueueDrops: r.Counters.ForwardQueueDrops, Evidence: m.ScheduledRelayTree, CollisionDrops: r.Counters.CollisionDropCount}
	for _, member := range r.MemberDelivery {
		row.PlayedFrames += member.FramesPlayed
		row.MissingFrames += member.FramesMissing
		if member.FramesPlayed == member.FramesExpected {
			row.ContinuousMembers++
		}
	}
	if m.ScheduledRelayTree.FirstPlayoutMS != nil {
		v := *m.ScheduledRelayTree.FirstPlayoutMS - controlStart
		row.FirstAudioFromPTTMS = &v
	}
	return row, nil
}
