package scenario

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/rmsff/ad-hoc-radio/internal/peer"
	"github.com/rmsff/ad-hoc-radio/internal/protocol"
	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

// activationEventIDs returns pending activation event ids in ascending order,
// so scans over activationEvents never depend on map iteration order.
func (r *scheduledRuntime) activationEventIDs() []uint64 {
	return slices.Sorted(maps.Keys(r.activationEvents))
}

func (r *scheduledRuntime) scheduleActivation(atMS int64, activation scheduledActivation) {
	// Last event wins only while a proposal is still local and undistributed.
	// Replacing instead of adding keeps at most one pending activation per
	// effective_seq, so at most one id matches; the ascending scan is
	// defensive and would replace the lowest id if that invariant broke.
	for _, id := range r.activationEventIDs() {
		if r.activationEvents[id].EffectiveSeq == activation.EffectiveSeq {
			r.activationEvents[id] = activation
			return
		}
	}
	latest := r.activeVersions[len(r.activeVersions)-1]
	sourceID := latest.Schedule.Binding.SourceStationID
	authorityID := latest.Schedule.Binding.ScheduleAuthorityID
	if activation.Version != nil {
		sourceID = activation.Version.Schedule.Binding.SourceStationID
		authorityID = activation.Version.Schedule.Binding.ScheduleAuthorityID
	}
	event := sim.Event{
		TimeMs:               atMS,
		EventPriority:        sim.PriorityScheduleUpdate,
		EventType:            scheduledEventVersionUpdate,
		SourceStationID:      authorityID,
		DestinationStationID: sourceID,
	}
	if r.control != nil && activation.Version != nil {
		event.TimeMs = atMS - r.control.ReplanLeadMS()
		if event.TimeMs < r.nowMS {
			event.TimeMs = r.nowMS
		}
		// After topology/missed-parent events at this timestamp, before source TX.
		event.EventPriority = sim.PrioritySourceTick - 1
	}
	r.push(event)
	r.activationEvents[r.queue.LastPushedID()] = activation
}

func (r *scheduledRuntime) processScheduledActivation(event sim.Event) error {
	activation, ok := r.activationEvents[event.EventID]
	if !ok {
		return fmt.Errorf("scheduled runtime: missing activation event_id=%d", event.EventID)
	}
	delete(r.activationEvents, event.EventID)
	if activation.Noop {
		return nil
	}
	if activation.Block {
		clear(r.consecutiveMissedParents)
		if r.blockedFromSeq == nil || activation.EffectiveSeq < *r.blockedFromSeq {
			blocked := activation.EffectiveSeq
			r.blockedFromSeq = &blocked
		}
		return nil
	}
	if activation.Version == nil {
		return fmt.Errorf("scheduled runtime: activation event_id=%d has no version", event.EventID)
	}
	next := cloneScheduledVersions([]ScheduledVersion{*activation.Version})[0]
	if r.control != nil && !activation.FromControl {
		if err := r.control.Replan(next.Schedule, event.TimeMs); err != nil {
			return fmt.Errorf("scheduled runtime: disseminate successor: %w", err)
		}
		r.controlVersions[next.Schedule.Binding.ScheduleVersion] = next
		return nil
	}
	if r.hasCreatedSourceSeq && next.Schedule.Binding.EffectiveSeq <= r.highestSourceSeqCreated {
		return fmt.Errorf(
			"scheduled runtime: activation effective_seq=%d is not after highest_source_seq_created=%d",
			next.Schedule.Binding.EffectiveSeq,
			r.highestSourceSeqCreated,
		)
	}
	previous := r.activeVersions[len(r.activeVersions)-1]
	if activation.FromControl {
		// A missed intermediate commit may leave a gap in local versions. The
		// controller validated the complete announced version chain already.
		previous.Schedule.Binding.ScheduleVersion = next.Schedule.Binding.ScheduleVersion - 1
	}
	highestCreated := next.Schedule.Binding.EffectiveSeq - 1
	if r.hasCreatedSourceSeq {
		highestCreated = r.highestSourceSeqCreated
	}
	if err := timing.ValidateScheduleSuccessor(previous.Schedule, next.Schedule, highestCreated); err != nil {
		return fmt.Errorf("scheduled runtime: activate successor: %w", err)
	}
	r.activeVersions = append(r.activeVersions, next)
	clear(r.consecutiveMissedParents)
	r.counters.VersionActivationCount++
	if r.blockedFromSeq != nil && next.Schedule.Binding.EffectiveSeq >= *r.blockedFromSeq {
		r.blockedFromSeq = nil
	}
	return nil
}

func (r *scheduledRuntime) processScheduledReplanEvent(event sim.Event) error {
	input, ok := r.replanEvents[event.EventID]
	if !ok {
		return fmt.Errorf("scheduled runtime: missing replan event_id=%d", event.EventID)
	}
	delete(r.replanEvents, event.EventID)
	return r.processScheduledReplan(input, event.TimeMs)
}

func (r *scheduledRuntime) maybeTriggerMissedParentReplan(ownerID uint16, seq uint32, atMS int64) error {
	if r.consecutiveMissedParents[ownerID] < r.cfg.MissedParentThreshold || r.missedReplanTemplate == nil {
		return nil
	}
	// Several downstream relays can miss the same source frame. Publish at
	// most one recovery proposal for its next-sequence activation boundary.
	if r.missedParentReplanSeq != nil && *r.missedParentReplanSeq == seq {
		return nil
	}
	input := *r.missedReplanTemplate
	input.AtMS = atMS
	r.missedParentReplanSeq = cloneUint32Pointer(&seq)
	delete(r.consecutiveMissedParents, ownerID)
	return r.processScheduledReplan(input, atMS)
}

func (r *scheduledRuntime) processScheduledReplan(input ScheduledReplanInput, atMS int64) error {
	// Replanning cannot authorize a sequence after the final source frame.
	if r.hasCreatedSourceSeq && r.highestSourceSeqCreated >= r.cfg.FrameCount-1 {
		return nil
	}
	r.counters.RouteReplanCount++
	r.counters.ReplanByReason[input.Reason]++
	effectiveSeq, err := r.nextSourceSeq()
	if err != nil {
		return err
	}
	if r.control != nil {
		when, ok := checkedScheduledTime(atMS, uint64(r.control.ReplanLeadMS()))
		if !ok || when > peer.MaxTimeMS {
			return fmt.Errorf("scheduled runtime: control replan horizon")
		}
		delta := when - r.cfg.Source.startMs
		if delta > 0 {
			seq := (delta + r.cfg.Source.framePeriodMs - 1) / r.cfg.Source.framePeriodMs
			if seq > int64(^uint32(0)) {
				return fmt.Errorf("scheduled runtime: replan sequence overflow")
			}
			if uint32(seq) > effectiveSeq {
				effectiveSeq = uint32(seq)
			}
		}
		latest := r.control.LatestSchedule()
		if effectiveSeq <= latest.Binding.EffectiveSeq {
			effectiveSeq = latest.Binding.EffectiveSeq + 1
		}
		if effectiveSeq >= r.cfg.FrameCount {
			return nil
		}
	}
	if r.cfg.VoiceProfile != nil {
		p := *r.cfg.VoiceProfile
		n, err := p.PayloadBytes()
		if err != nil {
			return err
		}
		size, err := protocol.ShortVoiceWireSize(n)
		if err != nil {
			return err
		}
		if r.control != nil {
			size += peer.VoiceBindingOverhead
		}
		budget, err := p.Measure(make([]byte, size))
		if err != nil {
			return err
		}
		if !budget.FitsBackend || budget.DurationMS > 65535 || budget.GuardMS > 65535 {
			return fmt.Errorf("scheduled runtime: replan voice budget does not fit backend")
		}
		input.Profile = timing.OpportunityProfile{SourceDurationMS: uint16(budget.DurationMS), RelayDurationMS: uint16(budget.DurationMS), GuardAfterMS: uint16(budget.GuardMS), MaxCycleMS: uint32(p.PacketPeriodMS)}
		input.PlanRequest.CostModel = routing.PlanningCostModel{SourceOpportunityMS: int64(budget.DurationMS), RelayOpportunityMS: int64(budget.DurationMS), GuardAfterMS: int64(budget.GuardMS), FramePeriodMS: p.PacketPeriodMS}
	}
	prepared, err := routing.PreparePlanSnapshot(input.PlanRequest)
	if err != nil {
		return err
	}
	input.PlanRequest = prepared.Request
	plan, err := routing.PlanRelayTree(input.PlanRequest)
	if errors.Is(err, routing.ErrNoFeasibleRelayTree) {
		r.scheduleActivation(r.sourceTimeForSeq(effectiveSeq), scheduledActivation{
			Block:        true,
			EffectiveSeq: effectiveSeq,
		})
		return nil
	}
	if err != nil {
		return fmt.Errorf("scheduled runtime: replan reason=%s at_ms=%d: %w", input.Reason, atMS, err)
	}
	candidate, err := r.buildReplannedVersion(input, plan.Selected.Tree, effectiveSeq)
	if err != nil {
		return err
	}
	current := r.activeVersions[len(r.activeVersions)-1]
	if r.control != nil {
		current.Schedule = r.control.LatestSchedule()
	}
	same, err := sameScheduledPlan(current, candidate)
	if err != nil {
		return err
	}
	if same {
		if r.blockedFromSeq != nil {
			r.scheduleActivation(r.sourceTimeForSeq(effectiveSeq), scheduledActivation{
				Version:      &candidate,
				EffectiveSeq: effectiveSeq,
			})
		}
		if r.blockedFromSeq == nil {
			// At most one pending activation exists per effective_seq.
			for _, id := range r.activationEventIDs() {
				if r.activationEvents[id].EffectiveSeq == effectiveSeq {
					r.activationEvents[id] = scheduledActivation{Noop: true, EffectiveSeq: effectiveSeq}
				}
			}
		}
		return nil
	}
	r.scheduleActivation(r.sourceTimeForSeq(effectiveSeq), scheduledActivation{
		Version:      &candidate,
		EffectiveSeq: effectiveSeq,
	})
	return nil
}

func (r *scheduledRuntime) buildReplannedVersion(
	input ScheduledReplanInput,
	tree routing.RelayTree,
	effectiveSeq uint32,
) (ScheduledVersion, error) {
	current := r.activeVersions[len(r.activeVersions)-1]
	if r.control != nil {
		current.Schedule = r.control.LatestSchedule()
	}
	if current.Schedule.Binding.ScheduleVersion == ^uint16(0) {
		return ScheduledVersion{}, fmt.Errorf("scheduled runtime: schedule_version would wrap")
	}
	binding := input.Binding
	binding.ScheduleID = current.Schedule.Binding.ScheduleID
	binding.ScheduleVersion = current.Schedule.Binding.ScheduleVersion + 1
	binding.EffectiveSeq = effectiveSeq
	if binding.ValidThroughSeq < effectiveSeq {
		return ScheduledVersion{}, fmt.Errorf(
			"scheduled runtime: replan valid_through_seq=%d is before effective_seq=%d",
			binding.ValidThroughSeq,
			effectiveSeq,
		)
	}
	schedule, err := timing.BuildOpportunitySchedule(binding, tree, input.Profile)
	if err != nil {
		return ScheduledVersion{}, fmt.Errorf("scheduled runtime: build replan schedule: %w", err)
	}
	version := ScheduledVersion{
		Links:       selectedTreeLinks(tree, input.PlanRequest.Graph),
		Tree:        cloneScheduledTree(tree),
		Schedule:    schedule,
		LinkDelayMS: selectedTreeLinkDelays(tree, input.PlanRequest.Graph),
	}
	startFrame, err := r.cfg.Source.EmitCallStart()
	if err != nil {
		return ScheduledVersion{}, fmt.Errorf("scheduled runtime: replan source call_start: %w", err)
	}
	startPayload, err := protocol.DecodeCallStartPayload(startFrame.Payload)
	if err != nil {
		return ScheduledVersion{}, fmt.Errorf("scheduled runtime: decode replan source call_start: %w", err)
	}
	voice := r.cfg.Source.EmitVoiceFrame(effectiveSeq)
	if err := validateScheduledVersion(version, r.cfg, startFrame, voice, startPayload.FramePeriodMs); err != nil {
		return ScheduledVersion{}, fmt.Errorf("scheduled runtime: replanned version: %w", err)
	}
	highestCreated := effectiveSeq - 1
	if r.hasCreatedSourceSeq {
		highestCreated = r.highestSourceSeqCreated
	}
	if err := timing.ValidateScheduleSuccessor(current.Schedule, version.Schedule, highestCreated); err != nil {
		return ScheduledVersion{}, fmt.Errorf("scheduled runtime: replanned successor: %w", err)
	}
	return version, nil
}

func (r *scheduledRuntime) nextSourceSeq() (uint32, error) {
	if !r.hasCreatedSourceSeq {
		return 1, nil
	}
	if r.highestSourceSeqCreated == ^uint32(0) {
		return 0, fmt.Errorf("scheduled runtime: source sequence would wrap")
	}
	return r.highestSourceSeqCreated + 1, nil
}

func (r *scheduledRuntime) sourceTimeForSeq(seq uint32) int64 {
	return r.cfg.Source.startMs + int64(seq)*r.cfg.Source.framePeriodMs
}

func selectedTreeLinkDelays(tree routing.RelayTree, graph routing.NeighborGraph) map[ScheduledLinkKey]int64 {
	delays := make(map[ScheduledLinkKey]int64, len(tree.Edges))
	for _, edge := range tree.Edges {
		for _, link := range graph.Links {
			if link.FromStationID == edge.ParentStationID && link.ToStationID == edge.ChildStationID {
				delays[ScheduledLinkKey{FromStationID: edge.ParentStationID, ToStationID: edge.ChildStationID}] = link.DelayMS
				break
			}
		}
	}
	return delays
}

func sameScheduledPlan(current, candidate ScheduledVersion) (bool, error) {
	currentTreeHash, err := routing.RelayTreeHash(current.Tree)
	if err != nil {
		return false, fmt.Errorf("scheduled runtime: hash current tree: %w", err)
	}
	candidateTreeHash, err := routing.RelayTreeHash(candidate.Tree)
	if err != nil {
		return false, fmt.Errorf("scheduled runtime: hash candidate tree: %w", err)
	}
	if currentTreeHash != candidateTreeHash {
		return false, nil
	}
	normalized := cloneOpportunitySchedule(candidate.Schedule)
	normalized.Binding.ScheduleVersion = current.Schedule.Binding.ScheduleVersion
	normalized.Binding.EffectiveSeq = current.Schedule.Binding.EffectiveSeq
	currentHash, err := timing.OpportunityScheduleHash(current.Schedule)
	if err != nil {
		return false, fmt.Errorf("scheduled runtime: hash current schedule: %w", err)
	}
	candidateHash, err := timing.OpportunityScheduleHash(normalized)
	if err != nil {
		return false, fmt.Errorf("scheduled runtime: hash candidate schedule: %w", err)
	}
	return currentHash == candidateHash, nil
}
