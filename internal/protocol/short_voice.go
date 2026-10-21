package protocol

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"math"
)

// ShortVoiceHeaderSize is the fixed ADR-0012 source/direct voice wire
// adapter header length. An 8-byte Codec2-class payload occupies exactly
// 22 bytes in this context-bound source/direct representation.
const ShortVoiceHeaderSize = 14

const (
	shortVoiceVersion        uint8  = 1
	shortVoiceExtraFlagMask  uint16 = FlagLateAllowed | FlagTestCorrupted
	shortVoiceMaxSentDeltaMs int64  = 255
	shortVoiceMaxTTL         uint8  = 15
)

// ShortVoiceContext is the active call context required by ADR-0012 to
// expand source/direct short voice bytes back to the canonical Frame model.
type ShortVoiceContext struct {
	NetID           uint16
	SourceID        uint16
	GroupID         uint16
	CallID          uint32
	BaseSeq         uint32
	BaseCreatedAtMs int64
	FramePeriodMs   int64
}

// ShortVoiceWireSize returns the short source/direct voice wire size for
// a payload length. It must be used by timing evidence instead of hard-coded
// synthetic sizes.
func ShortVoiceWireSize(payloadLen int) (int, error) {
	if payloadLen < 0 {
		return 0, fmt.Errorf("protocol: short voice payload length %d must be >= 0", payloadLen)
	}
	if payloadLen > math.MaxUint16 {
		return 0, fmt.Errorf("protocol: short voice payload length %d exceeds uint16 max %d", payloadLen, math.MaxUint16)
	}
	return ShortVoiceHeaderSize + payloadLen, nil
}

// EncodeShortVoice encodes a canonical direct/source voice Frame as the
// ADR-0012 context-bound short voice wire representation.
func EncodeShortVoice(f Frame, ctx ShortVoiceContext) ([]byte, error) {
	if err := ctx.validate(); err != nil {
		return nil, err
	}
	if err := validateShortVoiceFrame(f, ctx); err != nil {
		return nil, err
	}

	seqDelta, err := shortVoiceSeqDelta(f.Seq, ctx.BaseSeq)
	if err != nil {
		return nil, err
	}
	createdAtMs, err := shortVoiceCreatedAtMs(ctx, seqDelta)
	if err != nil {
		return nil, err
	}
	if f.CreatedAtMs != createdAtMs {
		return nil, fmt.Errorf("protocol: short voice created_at_ms=%d does not match context-derived value %d", f.CreatedAtMs, createdAtMs)
	}
	sentDeltaMs, err := shortVoiceSentDeltaMs(f.CreatedAtMs, f.SentAtMs)
	if err != nil {
		return nil, err
	}

	wireSize, err := ShortVoiceWireSize(len(f.Payload))
	if err != nil {
		return nil, err
	}
	out := make([]byte, wireSize)
	out[0] = shortVoiceVersion
	out[1] = byte(f.Flags & shortVoiceExtraFlagMask)
	binary.BigEndian.PutUint16(out[2:4], uint16(seqDelta))
	out[4] = f.Hop<<4 | f.TTL
	out[5] = uint8(sentDeltaMs)
	binary.BigEndian.PutUint32(out[6:10], crc32.ChecksumIEEE(f.Payload))
	copy(out[ShortVoiceHeaderSize:], f.Payload)
	binary.BigEndian.PutUint32(out[10:14], shortVoiceEnvelopeCRC(ctx, out[:10], out[ShortVoiceHeaderSize:]))
	return out, nil
}

// DecodeShortVoice decodes an ADR-0012 context-bound short source/direct
// voice wire representation into a canonical Frame. Missing or stale context
// is a hard error; fields are never guessed.
func DecodeShortVoice(data []byte, ctx ShortVoiceContext) (Frame, error) {
	if err := ctx.validate(); err != nil {
		return Frame{}, err
	}
	if len(data) < ShortVoiceHeaderSize {
		return Frame{}, fmt.Errorf("protocol: short voice data too short: got %d bytes, need >= %d", len(data), ShortVoiceHeaderSize)
	}
	if data[0] != shortVoiceVersion {
		return Frame{}, fmt.Errorf("protocol: short voice version %d is not supported", data[0])
	}

	flagsExtra := uint16(data[1])
	if err := validateShortVoiceExtraFlags(flagsExtra); err != nil {
		return Frame{}, err
	}
	gotEnvelopeCRC := binary.BigEndian.Uint32(data[10:14])
	wantEnvelopeCRC := shortVoiceEnvelopeCRC(ctx, data[:10], data[ShortVoiceHeaderSize:])
	if gotEnvelopeCRC != wantEnvelopeCRC {
		return Frame{}, fmt.Errorf("protocol: short voice envelope_crc32 mismatch: got 0x%08X, want 0x%08X", gotEnvelopeCRC, wantEnvelopeCRC)
	}

	payload := data[ShortVoiceHeaderSize:]
	if len(payload) > math.MaxUint16 {
		return Frame{}, fmt.Errorf("protocol: short voice payload length %d exceeds uint16 max %d", len(payload), math.MaxUint16)
	}
	gotPayloadCRC := binary.BigEndian.Uint32(data[6:10])
	wantPayloadCRC := crc32.ChecksumIEEE(payload)
	if gotPayloadCRC != wantPayloadCRC {
		return Frame{}, fmt.Errorf("protocol: short voice payload_crc32 mismatch: got 0x%08X, want 0x%08X", gotPayloadCRC, wantPayloadCRC)
	}

	seqDelta := uint32(binary.BigEndian.Uint16(data[2:4]))
	if ctx.BaseSeq > math.MaxUint32-seqDelta {
		return Frame{}, fmt.Errorf("protocol: short voice seq overflow: base_seq=%d seq_delta=%d", ctx.BaseSeq, seqDelta)
	}
	createdAtMs, err := shortVoiceCreatedAtMs(ctx, seqDelta)
	if err != nil {
		return Frame{}, err
	}
	sentAtMs, err := shortVoiceAddMs(createdAtMs, int64(data[5]))
	if err != nil {
		return Frame{}, fmt.Errorf("protocol: short voice sent_at_ms: %w", err)
	}

	hop := data[4] >> 4
	ttl := data[4] & 0x0F
	if hop != 0 {
		return Frame{}, fmt.Errorf("protocol: short voice hop=%d is invalid for direct/source voice", hop)
	}

	payloadCopy := make([]byte, len(payload))
	copy(payloadCopy, payload)
	f := Frame{
		Version:     Version,
		FrameType:   FrameTypeVoice,
		NetID:       ctx.NetID,
		SourceID:    ctx.SourceID,
		GroupID:     ctx.GroupID,
		RelayID:     0,
		CallID:      ctx.CallID,
		Seq:         ctx.BaseSeq + seqDelta,
		Hop:         hop,
		TTL:         ttl,
		Flags:       FlagDirect | flagsExtra,
		CreatedAtMs: createdAtMs,
		SentAtMs:    sentAtMs,
		PayloadLen:  uint16(len(payloadCopy)),
		Payload:     payloadCopy,
	}
	return canonicalizeShortVoiceFrame(f)
}

func (ctx ShortVoiceContext) validate() error {
	if ctx.SourceID == 0 {
		return fmt.Errorf("protocol: short voice context source_id must be > 0")
	}
	if ctx.GroupID == 0 {
		return fmt.Errorf("protocol: short voice context group_id must be > 0")
	}
	if ctx.FramePeriodMs <= 0 {
		return fmt.Errorf("protocol: short voice context frame_period_ms=%d must be > 0", ctx.FramePeriodMs)
	}
	return nil
}

func validateShortVoiceFrame(f Frame, ctx ShortVoiceContext) error {
	if err := f.Validate(); err != nil {
		return err
	}
	if f.FrameType != FrameTypeVoice {
		return fmt.Errorf("protocol: short voice supports voice frames only, got frame_type=0x%02X", f.FrameType)
	}
	if f.Flags&FlagDirect == 0 || f.Flags&FlagRelayed != 0 {
		return fmt.Errorf("protocol: short voice requires direct voice flags")
	}
	if err := validateShortVoiceExtraFlags(f.Flags &^ FlagDirect); err != nil {
		return err
	}
	if f.NetID != ctx.NetID {
		return fmt.Errorf("protocol: short voice net_id=%d does not match context %d", f.NetID, ctx.NetID)
	}
	if f.SourceID != ctx.SourceID {
		return fmt.Errorf("protocol: short voice source_id=%d does not match context %d", f.SourceID, ctx.SourceID)
	}
	if f.GroupID != ctx.GroupID {
		return fmt.Errorf("protocol: short voice group_id=%d does not match context %d", f.GroupID, ctx.GroupID)
	}
	if f.RelayID != 0 {
		return fmt.Errorf("protocol: short voice relay_id=%d, want 0", f.RelayID)
	}
	if f.CallID != ctx.CallID {
		return fmt.Errorf("protocol: short voice call_id=%d does not match context %d", f.CallID, ctx.CallID)
	}
	if f.Hop != 0 {
		return fmt.Errorf("protocol: short voice hop=%d, want 0", f.Hop)
	}
	if f.TTL > shortVoiceMaxTTL {
		return fmt.Errorf("protocol: short voice ttl=%d exceeds 4-bit max %d", f.TTL, shortVoiceMaxTTL)
	}
	return nil
}

func validateShortVoiceExtraFlags(flags uint16) error {
	if flags&^shortVoiceExtraFlagMask != 0 {
		return fmt.Errorf("protocol: short voice unsupported extra flags 0x%04X", flags&^shortVoiceExtraFlagMask)
	}
	return nil
}

func shortVoiceSeqDelta(seq uint32, baseSeq uint32) (uint32, error) {
	if seq < baseSeq {
		return 0, fmt.Errorf("protocol: short voice seq=%d is before base_seq=%d", seq, baseSeq)
	}
	delta := seq - baseSeq
	if delta > math.MaxUint16 {
		return 0, fmt.Errorf("protocol: short voice seq_delta=%d exceeds uint16 max %d", delta, math.MaxUint16)
	}
	return delta, nil
}

func shortVoiceCreatedAtMs(ctx ShortVoiceContext, seqDelta uint32) (int64, error) {
	offset, err := shortVoiceMulMs(int64(seqDelta), ctx.FramePeriodMs)
	if err != nil {
		return 0, fmt.Errorf("protocol: short voice created_at_ms offset: %w", err)
	}
	createdAtMs, err := shortVoiceAddMs(ctx.BaseCreatedAtMs, offset)
	if err != nil {
		return 0, fmt.Errorf("protocol: short voice created_at_ms: %w", err)
	}
	return createdAtMs, nil
}

func shortVoiceSentDeltaMs(createdAtMs int64, sentAtMs int64) (int64, error) {
	if sentAtMs < createdAtMs {
		return 0, fmt.Errorf("protocol: short voice sent_delta_ms is negative: sent_at_ms=%d created_at_ms=%d", sentAtMs, createdAtMs)
	}
	maxSentAtMs := int64(math.MaxInt64)
	if createdAtMs <= math.MaxInt64-shortVoiceMaxSentDeltaMs {
		maxSentAtMs = createdAtMs + shortVoiceMaxSentDeltaMs
	}
	if sentAtMs > maxSentAtMs {
		return 0, fmt.Errorf("protocol: short voice sent_delta_ms exceeds uint8 max %d", shortVoiceMaxSentDeltaMs)
	}
	return sentAtMs - createdAtMs, nil
}

func shortVoiceMulMs(a int64, b int64) (int64, error) {
	if a < 0 || b < 0 {
		return 0, fmt.Errorf("negative simulated-time operand")
	}
	if a != 0 && b > math.MaxInt64/a {
		return 0, fmt.Errorf("simulated-time multiplication overflow")
	}
	return a * b, nil
}

func shortVoiceAddMs(a int64, b int64) (int64, error) {
	if b > 0 && a > math.MaxInt64-b {
		return 0, fmt.Errorf("simulated-time addition overflow")
	}
	if b < 0 && a < math.MinInt64-b {
		return 0, fmt.Errorf("simulated-time addition overflow")
	}
	return a + b, nil
}

func shortVoiceEnvelopeCRC(ctx ShortVoiceContext, prefix []byte, payload []byte) uint32 {
	h := crc32.NewIEEE()
	contextBytes := shortVoiceContextBytes(ctx)
	_, _ = h.Write(contextBytes[:])
	_, _ = h.Write(prefix)
	_, _ = h.Write(payload)
	return h.Sum32()
}

func shortVoiceContextBytes(ctx ShortVoiceContext) [30]byte {
	var out [30]byte
	binary.BigEndian.PutUint16(out[0:2], ctx.NetID)
	binary.BigEndian.PutUint16(out[2:4], ctx.SourceID)
	binary.BigEndian.PutUint16(out[4:6], ctx.GroupID)
	binary.BigEndian.PutUint32(out[6:10], ctx.CallID)
	binary.BigEndian.PutUint32(out[10:14], ctx.BaseSeq)
	binary.BigEndian.PutUint64(out[14:22], uint64(ctx.BaseCreatedAtMs))
	binary.BigEndian.PutUint64(out[22:30], uint64(ctx.FramePeriodMs))
	return out
}

func canonicalizeShortVoiceFrame(f Frame) (Frame, error) {
	raw, err := f.Encode()
	if err != nil {
		return Frame{}, err
	}
	return DecodeStrict(raw)
}
