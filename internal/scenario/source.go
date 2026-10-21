package scenario

import (
	"fmt"

	"github.com/rmsff/ad-hoc-radio/internal/protocol"
)

// SourceStation generates frames for one Call per
// algorithm-and-experiments.md §7.1. It produces call_start, voice
// frames, and call_end with deterministic dummy payload.
type SourceStation struct {
	id              uint16
	netID           uint16
	groupID         uint16
	voiceGroupID    uint16
	callID          uint32
	framePeriodMs   int64
	startMs         int64
	ttl             uint8
	frameCount      int
	payloadBytes    int
	slotModeEnabled bool
	slotDurationMs  int64
}

// NewSourceStation creates a source from config.
func NewSourceStation(cfg ScenarioConfig) *SourceStation {
	return &SourceStation{
		id:              cfg.SourceID,
		netID:           cfg.NetID,
		groupID:         cfg.GroupID,
		voiceGroupID:    voiceGroupIDForConfig(cfg),
		callID:          cfg.CallID,
		framePeriodMs:   cfg.FramePeriodMs,
		startMs:         cfg.StartMs,
		ttl:             cfg.TTL,
		frameCount:      cfg.FrameCount,
		slotModeEnabled: cfg.SlotModeEnabled,
		slotDurationMs:  cfg.SlotDurationMs,
	}
}

// EmitCallStart creates a call_start frame per §8.1.
func (s *SourceStation) EmitCallStart() (protocol.Frame, error) {
	expectedCount := uint32(s.frameCount)
	var slotDuration *int64
	if s.slotModeEnabled {
		slotDuration = &s.slotDurationMs
	}
	raw, err := protocol.EncodeCallStartPayload(protocol.CallStartPayload{
		FramePeriodMs:      s.framePeriodMs,
		PayloadMode:        "dummy_counter_payload",
		ExpectedFrameCount: &expectedCount,
		SlotModeEnabled:    s.slotModeEnabled,
		SlotDurationMs:     slotDuration,
		TTLDefault:         s.ttl,
	})
	if err != nil {
		return protocol.Frame{}, err
	}
	return canonicalizeFrame(protocol.Frame{
		Version:     protocol.Version,
		FrameType:   protocol.FrameTypeCallStart,
		NetID:       s.netID,
		SourceID:    s.id,
		GroupID:     s.groupID,
		RelayID:     0,
		CallID:      s.callID,
		Seq:         0,
		Hop:         0,
		TTL:         s.ttl,
		Flags:       protocol.FlagCallStart,
		CreatedAtMs: s.startMs,
		SentAtMs:    s.startMs,
		PayloadLen:  uint16(len(raw)),
		Payload:     raw,
	})
}

// EmitVoiceFrame creates voice frame seq per §7.1 and §8.2.
// created_at_ms = startMs + seq * framePeriodMs.
func (s *SourceStation) EmitVoiceFrame(seq uint32) protocol.Frame {
	payload := []byte{byte(seq), 0x42}
	if s.payloadBytes > 0 {
		payload = make([]byte, s.payloadBytes)
		for i := range payload {
			payload[i] = byte(uint32(i) + seq)
		}
	}
	f, err := canonicalizeFrame(protocol.Frame{
		Version:     protocol.Version,
		FrameType:   protocol.FrameTypeVoice,
		NetID:       s.netID,
		SourceID:    s.id,
		GroupID:     s.voiceGroupID,
		RelayID:     0,
		CallID:      s.callID,
		Seq:         seq,
		Hop:         0,
		TTL:         s.ttl,
		Flags:       protocol.FlagDirect,
		CreatedAtMs: s.startMs + int64(seq)*s.framePeriodMs,
		SentAtMs:    s.startMs + int64(seq)*s.framePeriodMs,
		PayloadLen:  uint16(len(payload)),
		Payload:     payload,
	})
	if err != nil {
		// canonicalizeFrame only fails on Encode/Decode errors, which
		// indicate a bug in the frame construction. Panic to surface
		// it loudly (same convention as test helpers).
		panic(fmt.Sprintf("scenario: canonicalize voice frame seq=%d: %v", seq, err))
	}
	return f
}

// EmitCallEnd creates a call_end frame per §8.3 with last_seq.
func (s *SourceStation) EmitCallEnd() (protocol.Frame, error) {
	lastSeq := uint32(s.frameCount - 1)
	raw, err := protocol.EncodeCallEndPayload(protocol.CallEndPayload{
		LastSeq: lastSeq,
		Reason:  protocol.CallEndReasonNormal,
	})
	if err != nil {
		return protocol.Frame{}, err
	}
	endMs := s.startMs + int64(s.frameCount)*s.framePeriodMs
	return canonicalizeFrame(protocol.Frame{
		Version:     protocol.Version,
		FrameType:   protocol.FrameTypeCallEnd,
		NetID:       s.netID,
		SourceID:    s.id,
		GroupID:     s.groupID,
		RelayID:     0,
		CallID:      s.callID,
		Seq:         0,
		Hop:         0,
		TTL:         s.ttl,
		Flags:       protocol.FlagCallEnd,
		CreatedAtMs: endMs,
		SentAtMs:    endMs,
		PayloadLen:  uint16(len(raw)),
		Payload:     raw,
	})
}

// canonicalizeFrame encodes f and decodes the result, producing a
// Frame with valid HeaderCRC32 and PayloadCRC32. This is required
// because frame fields are set directly in Go, but CRCs are only
// computed during Encode. Callers that feed frames to
// ProcessCallStart/ProcessVoiceFrame (which call VerifyIntegrity) must
// provide frames with correct CRCs.
func canonicalizeFrame(f protocol.Frame) (protocol.Frame, error) {
	encoded, err := f.Encode()
	if err != nil {
		return protocol.Frame{}, fmt.Errorf("scenario: encode frame: %w", err)
	}
	decoded, err := protocol.Decode(encoded)
	if err != nil {
		return protocol.Frame{}, fmt.Errorf("scenario: decode frame: %w", err)
	}
	return decoded, nil
}
