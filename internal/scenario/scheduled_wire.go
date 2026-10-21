package scenario

import (
	"fmt"
	"github.com/rmsff/ad-hoc-radio/internal/peer"
	"github.com/rmsff/ad-hoc-radio/internal/protocol"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

func (r *scheduledRuntime) shortVoiceContext() protocol.ShortVoiceContext {
	s := r.cfg.Source
	return protocol.ShortVoiceContext{NetID: s.netID, SourceID: s.id, GroupID: s.groupID, CallID: s.callID, BaseCreatedAtMs: s.startMs, FramePeriodMs: s.framePeriodMs}
}
func (r *scheduledRuntime) relayWireContext(parent uint16) protocol.CompressedRelayHopContext {
	c := r.shortVoiceContext()
	return protocol.CompressedRelayHopContext{NetID: c.NetID, SourceID: c.SourceID, GroupID: c.GroupID, CallID: c.CallID, RelayID: parent, BaseCreatedAtMs: c.BaseCreatedAtMs, FramePeriodMs: c.FramePeriodMs}
}
func (r *scheduledRuntime) encodeScheduledWire(frame protocol.Frame) ([]byte, error) {
	if r.cfg.VoiceProfile == nil {
		return frame.Encode()
	}
	if frame.Hop == 0 {
		return protocol.EncodeShortVoice(frame, r.shortVoiceContext())
	}
	return protocol.EncodeCompressedRelayHop(frame, r.relayWireContext(frame.RelayID))
}
func (r *scheduledRuntime) decodeScheduledWire(pending scheduledPendingDelivery) (protocol.Frame, error) {
	if r.cfg.VoiceProfile == nil {
		return protocol.DecodeStrict(pending.RawFrame)
	}
	if pending.ParentStationID == r.cfg.Source.id {
		return protocol.DecodeShortVoice(pending.RawFrame, r.shortVoiceContext())
	}
	return protocol.DecodeCompressedRelayHop(pending.RawFrame, r.relayWireContext(pending.ParentStationID))
}

func prepareScheduledVoice(cfg *ScheduledRuntimeConfig) error {
	if cfg.VoiceProfile == nil {
		return nil
	}
	p := *cfg.VoiceProfile
	n, err := p.PayloadBytes()
	if err != nil {
		return err
	}
	if p.PacketPeriodMS != cfg.Source.framePeriodMs || cfg.FrameCount > 65536 {
		return fmt.Errorf("scheduled wire: cadence/context range mismatch")
	}
	source := *cfg.Source
	source.payloadBytes = n
	cfg.Source = &source
	runtime := newScheduledRuntime(*cfg)
	raw, err := runtime.encodeScheduledWire(source.EmitVoiceFrame(0))
	if err != nil {
		return err
	}
	if cfg.Control != nil {
		raw, err = peer.BindVoice(1, raw)
		if err != nil {
			return err
		}
	}
	budget, err := p.Measure(raw)
	if err != nil {
		return err
	}
	if !budget.FitsBackend {
		return fmt.Errorf("scheduled wire: encoded %d bytes do not fit backend envelope", len(raw))
	}
	for _, v := range cfg.Versions {
		for _, op := range v.Schedule.Opportunities {
			if uint32(op.DurationMS) < budget.DurationMS || uint32(op.GuardAfterMS) < budget.GuardMS {
				return fmt.Errorf("scheduled wire: opportunity %d shorter than actual byte/turnaround budget", op.Index)
			}
		}
	}
	cfg.VoiceProfile = &p
	return nil
}

func scheduledVoiceOpportunityProfile(p timing.VoiceBudgetProfile) (timing.OpportunityProfile, error) {
	return scheduledVoiceOpportunityProfileWithControl(p, false)
}

func scheduledVoiceOpportunityProfileWithControl(p timing.VoiceBudgetProfile, bound bool) (timing.OpportunityProfile, error) {
	n, err := p.PayloadBytes()
	if err != nil {
		return timing.OpportunityProfile{}, err
	}
	size, err := protocol.ShortVoiceWireSize(n)
	if err != nil {
		return timing.OpportunityProfile{}, err
	}
	if bound {
		size += peer.VoiceBindingOverhead
	}
	b, err := p.Measure(make([]byte, size))
	if err != nil {
		return timing.OpportunityProfile{}, err
	}
	if !b.FitsBackend {
		return timing.OpportunityProfile{}, fmt.Errorf("scheduled voice: backend payload/slot does not fit")
	}
	if b.DurationMS > 65535 || b.GuardMS > 65535 {
		return timing.OpportunityProfile{}, fmt.Errorf("scheduled voice: opportunity exceeds uint16")
	}
	return timing.OpportunityProfile{SourceDurationMS: uint16(b.DurationMS), RelayDurationMS: uint16(b.DurationMS), GuardAfterMS: uint16(b.GuardMS), MaxCycleMS: uint32(p.PacketPeriodMS)}, nil
}
