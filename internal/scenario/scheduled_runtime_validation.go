package scenario

import (
	"fmt"
	"math"
	"slices"

	"github.com/rmsff/ad-hoc-radio/internal/node"
	"github.com/rmsff/ad-hoc-radio/internal/peer"
	"github.com/rmsff/ad-hoc-radio/internal/protocol"
	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
	"github.com/rmsff/ad-hoc-radio/internal/transport"
	"maps"
)

func normalizeScheduledRuntimeConfig(cfg ScheduledRuntimeConfig) (ScheduledRuntimeConfig, error) {
	if cfg.Source == nil {
		return ScheduledRuntimeConfig{}, fmt.Errorf("scheduled runtime: source must not be nil")
	}
	if cfg.FrameCount == 0 {
		return ScheduledRuntimeConfig{}, fmt.Errorf("scheduled runtime: frame_count must be > 0")
	}
	if cfg.Control != nil && (cfg.FrameCount > 65536 || cfg.Source.startMs > peer.MaxTimeMS || len(cfg.Versions) > peer.MaxTransactions || len(cfg.ReplanInputs) > peer.MaxTransactions) {
		return ScheduledRuntimeConfig{}, fmt.Errorf("scheduled runtime: causal control work bound")
	}
	if err := validateScheduledSourceHorizon(cfg.Source, cfg.FrameCount); err != nil {
		return ScheduledRuntimeConfig{}, err
	}
	if len(cfg.Reacquisitions) > 256 || (len(cfg.Reacquisitions) > 0 && cfg.Control == nil) {
		return ScheduledRuntimeConfig{}, fmt.Errorf("scheduled runtime: invalid reacquisition configuration")
	}
	cfg.Reacquisitions = append([]ScheduledReacquisition(nil), cfg.Reacquisitions...)
	for _, request := range cfg.Reacquisitions {
		known := request.PeerID == cfg.Source.id || cfg.Relays[request.PeerID] != nil || cfg.Receivers[request.PeerID] != nil
		if !known || request.AtMS < cfg.Source.startMs || request.AtMS >= cfg.Source.startMs+int64(cfg.FrameCount)*cfg.Source.framePeriodMs || request.AtMS > peer.MaxTimeMS {
			return ScheduledRuntimeConfig{}, fmt.Errorf("scheduled runtime: invalid reacquisition peer/time")
		}
	}
	if cfg.ForwardQueueMaxDepth == 0 {
		cfg.ForwardQueueMaxDepth = DefaultRelayForwardQueueMaxDepth
	}
	if cfg.ForwardQueueMaxDepth < 0 {
		return ScheduledRuntimeConfig{}, fmt.Errorf("scheduled runtime: forward_queue_max_depth must be >= 0")
	}
	if cfg.MissedParentThreshold == 0 {
		cfg.MissedParentThreshold = DefaultScheduledMissedParentThreshold
	}
	if cfg.MissedParentThreshold < 0 {
		return ScheduledRuntimeConfig{}, fmt.Errorf("scheduled runtime: missed_parent_threshold must be >= 0")
	}
	if len(cfg.Versions) == 0 {
		return ScheduledRuntimeConfig{}, fmt.Errorf("scheduled runtime: at least one version is required")
	}

	cfg.Versions = cloneScheduledVersions(cfg.Versions)
	cfg.ReplanInputs = cloneScheduledReplanInputs(cfg.ReplanInputs)
	cfg.Relays = cloneScheduledRelayMap(cfg.Relays)
	cfg.Receivers = cloneScheduledReceiverMap(cfg.Receivers)
	cfg.Interference = append([]ScheduledInterferenceWindow(nil), cfg.Interference...)
	for _, window := range cfg.Interference {
		if window.StartMS < 0 || window.EndMS <= window.StartMS {
			return ScheduledRuntimeConfig{}, fmt.Errorf("scheduled runtime: invalid interference interval")
		}
	}
	if err := validateScheduledRuntimeConfig(cfg); err != nil {
		return ScheduledRuntimeConfig{}, err
	}
	if err := prepareScheduledVoice(&cfg); err != nil {
		return ScheduledRuntimeConfig{}, err
	}
	return cfg, nil
}

func validateScheduledRuntimeConfig(cfg ScheduledRuntimeConfig) error {
	startFrame, err := cfg.Source.EmitCallStart()
	if err != nil {
		return fmt.Errorf("scheduled runtime: source call_start: %w", err)
	}
	startPayload, err := protocol.DecodeCallStartPayload(startFrame.Payload)
	if err != nil {
		return fmt.Errorf("scheduled runtime: decode source call_start: %w", err)
	}
	if startPayload.ExpectedFrameCount == nil || *startPayload.ExpectedFrameCount != cfg.FrameCount {
		return fmt.Errorf("scheduled runtime: source expected_frame_count must equal frame_count=%d", cfg.FrameCount)
	}
	voice := cfg.Source.EmitVoiceFrame(0)

	for id, relay := range cfg.Relays {
		if relay == nil || relay.ID() != id {
			return fmt.Errorf("scheduled runtime: relay map key=%d does not match relay", id)
		}
	}
	for id, receiver := range cfg.Receivers {
		if receiver == nil || receiver.ID() != id {
			return fmt.Errorf("scheduled runtime: receiver map key=%d does not match receiver", id)
		}
	}

	for i := range cfg.Versions {
		version := cfg.Versions[i]
		if version.Schedule.Binding.EffectiveSeq > cfg.FrameCount {
			return fmt.Errorf(
				"scheduled runtime: version[%d]: effective_seq=%d exceeds frame_count=%d",
				i,
				version.Schedule.Binding.EffectiveSeq,
				cfg.FrameCount,
			)
		}
		if err := validateScheduledVersion(version, cfg, startFrame, voice, startPayload.FramePeriodMs); err != nil {
			return fmt.Errorf("scheduled runtime: version[%d]: %w", i, err)
		}
		if i == 0 {
			if err := timing.ValidateInitialOpportunitySchedule(version.Schedule); err != nil {
				return fmt.Errorf("initial schedule: %w", err)
			}
			if version.Schedule.Binding.EffectiveSeq != 0 {
				return fmt.Errorf("initial schedule: effective_seq=%d must be 0", version.Schedule.Binding.EffectiveSeq)
			}
			continue
		}
		previous := cfg.Versions[i-1].Schedule
		next := version.Schedule
		if next.Binding.EffectiveSeq == 0 {
			return fmt.Errorf("successor schedule: effective_seq must be > 0")
		}
		if err := timing.ValidateScheduleSuccessor(previous, next, next.Binding.EffectiveSeq-1); err != nil {
			return err
		}
	}
	if err := validateScheduledPeerInventory(cfg); err != nil {
		return err
	}
	return validateScheduledReplanInputs(cfg)
}

func validateScheduledPeerInventory(cfg ScheduledRuntimeConfig) error {
	wantRelays := make(map[uint16]struct{})
	wantReceivers := make(map[uint16]struct{})
	for _, version := range cfg.Versions {
		for _, opportunity := range version.Schedule.Opportunities[1:] {
			wantRelays[opportunity.OwnerStationID] = struct{}{}
		}
		for _, receiverID := range version.Schedule.CoveredMemberIDs {
			wantReceivers[receiverID] = struct{}{}
		}
	}
	for _, input := range cfg.ReplanInputs {
		for _, state := range input.PlanRequest.RelayStates {
			wantRelays[state.StationID] = struct{}{}
		}
		for _, receiverID := range input.PlanRequest.Snapshot.MemberIDs {
			if receiverID != input.PlanRequest.SourceStationID {
				wantReceivers[receiverID] = struct{}{}
			}
		}
	}
	gotRelayIDs := sortedScheduledRelayIDs(cfg.Relays)
	wantRelayIDs := sortedScheduledIDSet(wantRelays)
	if !slices.Equal(gotRelayIDs, wantRelayIDs) {
		return fmt.Errorf("scheduled runtime: relay inventory station_ids=%v, want %v", gotRelayIDs, wantRelayIDs)
	}
	gotReceiverIDs := sortedScheduledReceiverIDs(cfg.Receivers)
	wantReceiverIDs := sortedScheduledIDSet(wantReceivers)
	if !slices.Equal(gotReceiverIDs, wantReceiverIDs) {
		return fmt.Errorf("scheduled runtime: receiver inventory station_ids=%v, want %v", gotReceiverIDs, wantReceiverIDs)
	}
	return nil
}

func sortedScheduledIDSet(set map[uint16]struct{}) []uint16 {
	ids := make([]uint16, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func validateScheduledSourceHorizon(source *SourceStation, frameCount uint32) error {
	if source.startMs < 0 {
		return fmt.Errorf("scheduled runtime: source start_ms=%d must be >= 0", source.startMs)
	}
	if source.framePeriodMs <= 0 {
		return fmt.Errorf("scheduled runtime: source frame_period_ms=%d must be > 0", source.framePeriodMs)
	}
	maxDelta := uint64(math.MaxInt64 - source.startMs)
	period := uint64(source.framePeriodMs)
	if uint64(frameCount) > maxDelta/period {
		return fmt.Errorf(
			"scheduled runtime: source time horizon overflows int64: start_ms=%d frame_period_ms=%d frame_count=%d",
			source.startMs,
			source.framePeriodMs,
			frameCount,
		)
	}
	return nil
}

func validateScheduledVersion(
	version ScheduledVersion,
	cfg ScheduledRuntimeConfig,
	startFrame protocol.Frame,
	voice protocol.Frame,
	framePeriodMS int64,
) error {
	if _, err := routing.CanonicalRelayTreeJSON(version.Tree); err != nil {
		return fmt.Errorf("tree: %w", err)
	}
	if err := timing.ValidateOpportunitySchedule(version.Schedule); err != nil {
		return fmt.Errorf("schedule: %w", err)
	}
	binding := version.Schedule.Binding
	if version.Tree.SourceStationID != binding.SourceStationID ||
		version.Tree.MembershipSnapshotID != binding.MembershipSnapshotID ||
		version.Tree.TopologyEpoch != binding.TopologyEpoch ||
		version.Tree.MaxRelayDepth != version.Schedule.MaxRelayDepth ||
		!slices.Equal(version.Tree.Coverage.CoveredMemberIDs, version.Schedule.CoveredMemberIDs) ||
		!slices.Equal(version.Tree.Coverage.UncoveredMemberIDs, version.Schedule.UncoveredMemberIDs) {
		return fmt.Errorf("tree and schedule binding mismatch")
	}
	if startFrame.NetID != binding.NetID || startFrame.SourceID != binding.SourceStationID ||
		startFrame.GroupID != binding.GroupID || startFrame.CallID != binding.CallID ||
		voice.NetID != binding.NetID || voice.SourceID != binding.SourceStationID ||
		voice.GroupID != binding.GroupID || voice.CallID != binding.CallID {
		return fmt.Errorf("source-generated frame binding does not match schedule")
	}
	if !cfg.AllowCycleTimingStress && uint64(framePeriodMS) < uint64(version.Schedule.CycleDurationMS()) {
		return fmt.Errorf("frame_period_ms=%d is shorter than schedule cycle=%d", framePeriodMS, version.Schedule.CycleDurationMS())
	}
	if err := validateScheduledVersionInventory(version, cfg); err != nil {
		return err
	}
	return validateScheduledTreeOpportunityPair(version)
}

func validateScheduledVersionInventory(version ScheduledVersion, cfg ScheduledRuntimeConfig) error {
	selected := make(map[ScheduledLinkKey]bool, len(version.Tree.Edges))
	for _, edge := range version.Tree.Edges {
		selected[ScheduledLinkKey{edge.ParentStationID, edge.ChildStationID}] = true
	}
	for key, link := range version.Links {
		if !selected[key] || uint16(link.From) != key.FromStationID || uint16(link.To) != key.ToStationID {
			return fmt.Errorf("scheduled runtime: transport link key/endpoints/tree mismatch")
		}
		check := link
		check.Enabled = true
		if err := (transport.Topology{Links: []transport.Link{check}}).Validate(); err != nil {
			return err
		}
		if link.JitterMs == math.MaxInt64 || link.FixedDelayMs != version.LinkDelayMS[key] {
			return fmt.Errorf("scheduled runtime: transport link delay inconsistent or jitter overflow")
		}
	}
	for _, id := range version.Schedule.CoveredMemberIDs {
		if cfg.Receivers[id] == nil {
			return fmt.Errorf("covered receiver station_id=%d is missing", id)
		}
	}
	for _, opportunity := range version.Schedule.Opportunities[1:] {
		if cfg.Relays[opportunity.OwnerStationID] == nil {
			return fmt.Errorf("relay owner station_id=%d is missing", opportunity.OwnerStationID)
		}
	}
	for _, edge := range version.Tree.Edges {
		key := ScheduledLinkKey{FromStationID: edge.ParentStationID, ToStationID: edge.ChildStationID}
		delay, ok := version.LinkDelayMS[key]
		if !ok {
			return fmt.Errorf("selected edge %d->%d has no link delay", edge.ParentStationID, edge.ChildStationID)
		}
		if delay < 0 {
			return fmt.Errorf("selected edge %d->%d has negative link delay=%d", edge.ParentStationID, edge.ChildStationID, delay)
		}
		last := cfg.Source.EmitVoiceFrame(cfg.FrameCount - 1).CreatedAtMs
		link := version.Links[key]
		if _, ok := checkedScheduledTime(last, uint64(version.Schedule.CycleDurationMS()), uint64(delay), uint64(link.JitterMs), 5); !ok {
			return fmt.Errorf("scheduled runtime: selected edge delivery time overflows int64")
		}
	}
	return nil
}

func validateScheduledTreeOpportunityPair(version ScheduledVersion) error {
	incoming := make(map[uint16]routing.RelayTreeEdge, len(version.Tree.Edges))
	outgoing := make(map[uint16]bool, len(version.Tree.Edges))
	for _, edge := range version.Tree.Edges {
		incoming[edge.ChildStationID] = edge
		outgoing[edge.ParentStationID] = true
	}
	wantOwners := 0
	for owner := range outgoing {
		if owner != version.Tree.SourceStationID {
			wantOwners++
		}
	}
	if len(version.Schedule.Opportunities) != wantOwners+1 {
		return fmt.Errorf("opportunity count=%d, want %d", len(version.Schedule.Opportunities), wantOwners+1)
	}
	for _, opportunity := range version.Schedule.Opportunities[1:] {
		edge, ok := incoming[opportunity.OwnerStationID]
		if !ok || !outgoing[opportunity.OwnerStationID] {
			return fmt.Errorf("opportunity owner station_id=%d is not a transit tree node", opportunity.OwnerStationID)
		}
		if opportunity.ParentStationID != edge.ParentStationID || opportunity.Hop != edge.Hop+1 {
			return fmt.Errorf("opportunity owner station_id=%d parent/hop mismatch", opportunity.OwnerStationID)
		}
	}
	return nil
}

func validateScheduledReplanInputs(cfg ScheduledRuntimeConfig) error {
	seenMissedTemplate := false
	for i := range cfg.ReplanInputs {
		input := cfg.ReplanInputs[i]
		if !validScheduledReplanReason(input.Reason) {
			return fmt.Errorf("scheduled runtime: replan[%d] reason=%q is invalid", i, input.Reason)
		}
		if input.Reason == ScheduledReplanMissedParentThreshold {
			if seenMissedTemplate {
				return fmt.Errorf("scheduled runtime: only one missed-parent replan template is allowed")
			}
			seenMissedTemplate = true
			continue
		}
		if input.AtMS < cfg.Source.startMs {
			return fmt.Errorf("scheduled runtime: replan[%d] at_ms=%d is before call start=%d", i, input.AtMS, cfg.Source.startMs)
		}
	}
	return nil
}

func validScheduledReplanReason(reason ScheduledReplanReason) bool {
	switch reason {
	case ScheduledReplanTopologyEpochChanged,
		ScheduledReplanMembershipSnapshotChanged,
		ScheduledReplanSelectedRelayUnavailable,
		ScheduledReplanTimingAuthorityLost,
		ScheduledReplanMissedParentThreshold:
		return true
	default:
		return false
	}
}

func cloneScheduledVersions(in []ScheduledVersion) []ScheduledVersion {
	out := make([]ScheduledVersion, len(in))
	for i := range in {
		out[i] = ScheduledVersion{
			Links:       maps.Clone(in[i].Links),
			Tree:        cloneScheduledTree(in[i].Tree),
			Schedule:    cloneOpportunitySchedule(in[i].Schedule),
			LinkDelayMS: cloneScheduledLinkDelays(in[i].LinkDelayMS),
		}
	}
	return out
}

func cloneScheduledTree(tree routing.RelayTree) routing.RelayTree {
	tree.Edges = append([]routing.RelayTreeEdge(nil), tree.Edges...)
	tree.Coverage.CoveredMemberIDs = append([]uint16(nil), tree.Coverage.CoveredMemberIDs...)
	tree.Coverage.UncoveredMemberIDs = append([]uint16(nil), tree.Coverage.UncoveredMemberIDs...)
	return tree
}

func cloneOpportunitySchedule(schedule timing.OpportunitySchedule) timing.OpportunitySchedule {
	schedule.Binding.ExpectedMemberIDs = append([]uint16(nil), schedule.Binding.ExpectedMemberIDs...)
	schedule.CoveredMemberIDs = append([]uint16(nil), schedule.CoveredMemberIDs...)
	schedule.UncoveredMemberIDs = append([]uint16(nil), schedule.UncoveredMemberIDs...)
	schedule.Opportunities = append([]timing.TxOpportunity(nil), schedule.Opportunities...)
	return schedule
}

func cloneScheduledLinkDelays(in map[ScheduledLinkKey]int64) map[ScheduledLinkKey]int64 {
	out := make(map[ScheduledLinkKey]int64, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneScheduledReplanInputs(in []ScheduledReplanInput) []ScheduledReplanInput {
	out := make([]ScheduledReplanInput, len(in))
	copy(out, in)
	for i := range out {
		out[i].PlanRequest.Snapshot.MemberIDs = append([]uint16(nil), in[i].PlanRequest.Snapshot.MemberIDs...)
		out[i].PlanRequest.Graph.Links = append([]routing.DirectedLink(nil), in[i].PlanRequest.Graph.Links...)
		out[i].PlanRequest.RelayStates = append([]routing.RelayState(nil), in[i].PlanRequest.RelayStates...)
		out[i].Binding.ExpectedMemberIDs = append([]uint16(nil), in[i].Binding.ExpectedMemberIDs...)
	}
	return out
}

func cloneScheduledRelayMap(in map[uint16]*node.RelayStation) map[uint16]*node.RelayStation {
	out := make(map[uint16]*node.RelayStation, len(in))
	for id, relay := range in {
		out[id] = relay
	}
	return out
}

func cloneScheduledReceiverMap(in map[uint16]*node.ReceiverStation) map[uint16]*node.ReceiverStation {
	out := make(map[uint16]*node.ReceiverStation, len(in))
	for id, receiver := range in {
		out[id] = receiver
	}
	return out
}
