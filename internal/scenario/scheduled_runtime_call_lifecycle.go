package scenario

import (
	"fmt"

	"github.com/rmsff/ad-hoc-radio/internal/peer"
	"github.com/rmsff/ad-hoc-radio/internal/protocol"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

func (r *scheduledRuntime) processCallStart() error {
	initial := r.activeVersions[0]
	digest, err := timing.OpportunityScheduleHash(initial.Schedule)
	if err != nil {
		return fmt.Errorf("scheduled runtime: initial schedule hash: %w", err)
	}
	frame, err := r.cfg.Source.EmitCallStart()
	if err != nil {
		return fmt.Errorf("scheduled runtime: emit call_start: %w", err)
	}
	payload, err := protocol.DecodeCallStartPayload(frame.Payload)
	if err != nil {
		return fmt.Errorf("scheduled runtime: decode call_start: %w", err)
	}
	scheduleID := initial.Schedule.Binding.ScheduleID
	scheduleVersion := initial.Schedule.Binding.ScheduleVersion
	strategy := protocol.CallStartTimingStrategy(initial.Schedule.Binding.TimingStrategy)
	payload.ScheduleID = &scheduleID
	payload.ScheduleVersion = &scheduleVersion
	payload.ScheduleDigest = &digest
	payload.TimingStrategy = &strategy
	payload.TTLDefault = r.maxInstalledRelayDepth()
	if payload.TTLDefault == 0 {
		payload.TTLDefault = 1
	}
	encoded, err := protocol.EncodeCallStartPayload(payload)
	if err != nil {
		return fmt.Errorf("scheduled runtime: encode scheduled call_start: %w", err)
	}
	frame.Payload = encoded
	frame.PayloadLen = uint16(len(encoded))
	frame.HeaderCRC32 = 0
	frame.PayloadCRC32 = 0
	frame, err = canonicalizeFrame(frame)
	if err != nil {
		return err
	}
	r.callStart = frame
	if r.cfg.Control != nil {
		r.control, err = peer.New(*r.cfg.Control, r.queue)
		if err != nil {
			return err
		}
		r.nowMS = r.cfg.Control.StartMS
		return r.control.Begin(initial.Schedule, r.cfg.Source.startMs, r.cfg.Source.framePeriodMs)
	}
	for _, id := range sortedScheduledRelayIDs(r.cfg.Relays) {
		if err := r.cfg.Relays[id].ProcessCallStart(frame, frame.CreatedAtMs); err != nil {
			return fmt.Errorf("scheduled runtime: relay %d call_start: %w", id, err)
		}
	}
	for _, id := range sortedScheduledReceiverIDs(r.cfg.Receivers) {
		if err := r.cfg.Receivers[id].ProcessCallStart(frame, frame.CreatedAtMs); err != nil {
			return fmt.Errorf("scheduled runtime: receiver %d call_start: %w", id, err)
		}
	}
	return nil
}

func (r *scheduledRuntime) processCallEnd() error {
	if r.control != nil {
		return r.control.End(r.nowMS)
	}
	frame, err := r.cfg.Source.EmitCallEnd()
	if err != nil {
		return fmt.Errorf("scheduled runtime: emit call_end: %w", err)
	}
	evaluateAt := r.nowMS
	if frame.CreatedAtMs > evaluateAt {
		evaluateAt = frame.CreatedAtMs
	}
	for _, id := range sortedScheduledRelayIDs(r.cfg.Relays) {
		if r.control != nil && !r.control.Active(id) {
			continue
		}
		if err := r.cfg.Relays[id].ProcessCallEnd(frame, evaluateAt); err != nil {
			return fmt.Errorf("scheduled runtime: relay %d call_end: %w", id, err)
		}
	}
	for _, id := range sortedScheduledReceiverIDs(r.cfg.Receivers) {
		if r.control != nil && !r.control.Active(id) {
			continue
		}
		evaluation, err := r.cfg.Receivers[id].ProcessCallEndWithEvaluation(frame, evaluateAt)
		if err != nil {
			return fmt.Errorf("scheduled runtime: receiver %d call_end: %w", id, err)
		}
		if len(evaluation.Emitted) != 0 {
			return fmt.Errorf("scheduled runtime: call end unexpectedly flushed unplayed frames for %d", id)
		}
	}
	r.nowMS = evaluateAt
	if r.control != nil {
		if err := r.control.End(evaluateAt); err != nil {
			return err
		}
	}
	return nil
}

func (r *scheduledRuntime) startLocalCall(id uint16, atMS int64) error {
	if r.localStarted[id] || r.localEnded[id] {
		return nil
	}
	if r.control != nil && !r.control.Matches(id) {
		return nil
	}
	r.localStarted[id] = true
	if relay := r.cfg.Relays[id]; relay != nil {
		if err := relay.Lifecycle().ProcessCallStart(r.callStart, atMS); err != nil {
			return err
		}
	}
	if receiver := r.cfg.Receivers[id]; receiver != nil {
		if err := receiver.Lifecycle().ProcessCallStart(r.callStart, atMS); err != nil {
			return err
		}
	}
	return nil
}

func (r *scheduledRuntime) endLocalCall(id uint16, at int64) error {
	if !r.localStarted[id] || r.localEnded[id] {
		return nil
	}
	// Project received End/expiry into the existing node lifecycle, not a new
	// over-air CALL_END invented by the global simulator.
	frame, err := r.cfg.Source.EmitCallEnd()
	if err != nil {
		return err
	}
	frame.CreatedAtMs = at
	frame.SentAtMs = at
	frame.HeaderCRC32 = 0
	frame.PayloadCRC32 = 0
	frame, err = canonicalizeFrame(frame)
	if err != nil {
		return err
	}
	if relay := r.cfg.Relays[id]; relay != nil {
		if err := relay.ProcessCallEnd(frame, at); err != nil {
			return err
		}
	}
	if receiver := r.cfg.Receivers[id]; receiver != nil {
		evaluation, err := receiver.ProcessCallEndWithEvaluation(frame, at)
		if err != nil {
			return err
		}
		if len(evaluation.Emitted) != 0 {
			return fmt.Errorf("scheduled runtime: local End flushed audio before its deadline")
		}
	}
	r.localEnded[id] = true
	return nil
}

func (r *scheduledRuntime) maxInstalledRelayDepth() uint8 {
	var maxDepth uint8
	for _, version := range r.cfg.Versions {
		if version.Tree.MaxRelayDepth > maxDepth {
			maxDepth = version.Tree.MaxRelayDepth
		}
	}
	return maxDepth
}
