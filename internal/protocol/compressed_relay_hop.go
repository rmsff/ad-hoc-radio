package protocol

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"math"
)

// CompressedRelayHopHeaderSize is the fixed ADR-0011 relay-hop wire
// adapter header length. An 8-byte Codec2-class payload therefore
// occupies exactly 22 bytes on this compressed relay hop.
const CompressedRelayHopHeaderSize = 14

const (
	compressedRelayHopVersion        uint8  = 1
	compressedRelayHopExtraFlagMask  uint16 = FlagLateAllowed | FlagTestCorrupted
	compressedRelayHopMaxNibble      uint8  = 15
	compressedRelayHopMaxSentDeltaMs int64  = 255
)

// CompressedRelayHopContext is the active call/path context required
// by ADR-0011 to expand a compressed relayed voice frame back to the
// canonical Frame model.
type CompressedRelayHopContext struct {
	NetID           uint16
	SourceID        uint16
	GroupID         uint16
	RelayID         uint16
	CallID          uint32
	BaseSeq         uint32
	BaseCreatedAtMs int64
	FramePeriodMs   int64
}

// CompressedRelayHopWireSize returns the compressed relay-hop wire
// size for a payload length. It is useful for timing evidence that
// must use actual encoded sizes rather than synthetic all-hop rows.
func CompressedRelayHopWireSize(payloadLen int) (int, error) {
	if payloadLen < 0 {
		return 0, fmt.Errorf("protocol: compressed relay-hop payload length %d must be >= 0", payloadLen)
	}
	if payloadLen > math.MaxUint16 {
		return 0, fmt.Errorf("protocol: compressed relay-hop payload length %d exceeds uint16 max %d", payloadLen, math.MaxUint16)
	}
	return CompressedRelayHopHeaderSize + payloadLen, nil
}

// EncodeCompressedRelayHop encodes a canonical relayed voice Frame as
// the ADR-0011 context-bound relay-hop wire representation.
//
// Scope is intentionally narrow: source/direct voice and control
// frames are not compressed. The decoder expands bytes back to the
// canonical Frame before receiver, relay, metrics, or event-hash use.
func EncodeCompressedRelayHop(f Frame, ctx CompressedRelayHopContext) ([]byte, error) {
	if err := ctx.validate(); err != nil {
		return nil, err
	}
	if err := validateCompressedRelayHopFrame(f, ctx); err != nil {
		return nil, err
	}

	seqDelta, err := compressedRelayHopSeqDelta(f.Seq, ctx.BaseSeq)
	if err != nil {
		return nil, err
	}
	createdAtMs, err := compressedRelayHopCreatedAtMs(ctx, seqDelta)
	if err != nil {
		return nil, err
	}
	if f.CreatedAtMs != createdAtMs {
		return nil, fmt.Errorf("protocol: compressed relay-hop created_at_ms=%d does not match context-derived value %d", f.CreatedAtMs, createdAtMs)
	}
	sentDeltaMs, err := compressedRelayHopSentDeltaMs(f.CreatedAtMs, f.SentAtMs)
	if err != nil {
		return nil, err
	}

	wireSize, err := CompressedRelayHopWireSize(len(f.Payload))
	if err != nil {
		return nil, err
	}
	out := make([]byte, wireSize)
	out[0] = compressedRelayHopVersion
	out[1] = byte(f.Flags & compressedRelayHopExtraFlagMask)
	binary.BigEndian.PutUint16(out[2:4], uint16(seqDelta))
	out[4] = f.Hop<<4 | f.TTL
	out[5] = uint8(sentDeltaMs)
	binary.BigEndian.PutUint32(out[6:10], crc32.ChecksumIEEE(f.Payload))
	copy(out[CompressedRelayHopHeaderSize:], f.Payload)
	binary.BigEndian.PutUint32(out[10:14], compressedRelayHopEnvelopeCRC(ctx, out[:10], out[CompressedRelayHopHeaderSize:]))
	return out, nil
}

// DecodeCompressedRelayHop decodes an ADR-0011 context-bound relay-hop
// wire representation into a canonical Frame. Missing or stale context
// is a hard error; fields are never guessed.
func DecodeCompressedRelayHop(data []byte, ctx CompressedRelayHopContext) (Frame, error) {
	if err := ctx.validate(); err != nil {
		return Frame{}, err
	}
	if len(data) < CompressedRelayHopHeaderSize {
		return Frame{}, fmt.Errorf("protocol: compressed relay-hop data too short: got %d bytes, need >= %d", len(data), CompressedRelayHopHeaderSize)
	}
	if data[0] != compressedRelayHopVersion {
		return Frame{}, fmt.Errorf("protocol: compressed relay-hop version %d is not supported", data[0])
	}

	flagsExtra := uint16(data[1])
	if err := validateCompressedRelayHopExtraFlags(flagsExtra); err != nil {
		return Frame{}, err
	}
	gotEnvelopeCRC := binary.BigEndian.Uint32(data[10:14])
	wantEnvelopeCRC := compressedRelayHopEnvelopeCRC(ctx, data[:10], data[CompressedRelayHopHeaderSize:])
	if gotEnvelopeCRC != wantEnvelopeCRC {
		return Frame{}, fmt.Errorf("protocol: compressed relay-hop envelope_crc32 mismatch: got 0x%08X, want 0x%08X", gotEnvelopeCRC, wantEnvelopeCRC)
	}

	payload := data[CompressedRelayHopHeaderSize:]
	if len(payload) > math.MaxUint16 {
		return Frame{}, fmt.Errorf("protocol: compressed relay-hop payload length %d exceeds uint16 max %d", len(payload), math.MaxUint16)
	}
	gotPayloadCRC := binary.BigEndian.Uint32(data[6:10])
	wantPayloadCRC := crc32.ChecksumIEEE(payload)
	if gotPayloadCRC != wantPayloadCRC {
		return Frame{}, fmt.Errorf("protocol: compressed relay-hop payload_crc32 mismatch: got 0x%08X, want 0x%08X", gotPayloadCRC, wantPayloadCRC)
	}

	seqDelta := uint32(binary.BigEndian.Uint16(data[2:4]))
	if ctx.BaseSeq > math.MaxUint32-seqDelta {
		return Frame{}, fmt.Errorf("protocol: compressed relay-hop seq overflow: base_seq=%d seq_delta=%d", ctx.BaseSeq, seqDelta)
	}
	createdAtMs, err := compressedRelayHopCreatedAtMs(ctx, seqDelta)
	if err != nil {
		return Frame{}, err
	}
	sentAtMs, err := compressedRelayHopAddMs(createdAtMs, int64(data[5]))
	if err != nil {
		return Frame{}, fmt.Errorf("protocol: compressed relay-hop sent_at_ms: %w", err)
	}

	hop := data[4] >> 4
	ttl := data[4] & 0x0F
	if hop == 0 {
		return Frame{}, fmt.Errorf("protocol: compressed relay-hop hop=0 is invalid for relayed voice")
	}

	payloadCopy := make([]byte, len(payload))
	copy(payloadCopy, payload)
	f := Frame{
		Version:     Version,
		FrameType:   FrameTypeVoice,
		NetID:       ctx.NetID,
		SourceID:    ctx.SourceID,
		GroupID:     ctx.GroupID,
		RelayID:     ctx.RelayID,
		CallID:      ctx.CallID,
		Seq:         ctx.BaseSeq + seqDelta,
		Hop:         hop,
		TTL:         ttl,
		Flags:       FlagRelayed | flagsExtra,
		CreatedAtMs: createdAtMs,
		SentAtMs:    sentAtMs,
		PayloadLen:  uint16(len(payloadCopy)),
		Payload:     payloadCopy,
	}
	return canonicalizeCompressedRelayHopFrame(f)
}

func (ctx CompressedRelayHopContext) validate() error {
	if ctx.SourceID == 0 {
		return fmt.Errorf("protocol: compressed relay-hop context source_id must be > 0")
	}
	if ctx.GroupID == 0 {
		return fmt.Errorf("protocol: compressed relay-hop context group_id must be > 0")
	}
	if ctx.RelayID == 0 {
		return fmt.Errorf("protocol: compressed relay-hop context relay_id must be > 0")
	}
	if ctx.FramePeriodMs <= 0 {
		return fmt.Errorf("protocol: compressed relay-hop context frame_period_ms=%d must be > 0", ctx.FramePeriodMs)
	}
	return nil
}

func validateCompressedRelayHopFrame(f Frame, ctx CompressedRelayHopContext) error {
	if err := f.Validate(); err != nil {
		return err
	}
	if f.FrameType != FrameTypeVoice {
		return fmt.Errorf("protocol: compressed relay-hop supports voice frames only, got frame_type=0x%02X", f.FrameType)
	}
	if f.Flags&FlagRelayed == 0 || f.Flags&FlagDirect != 0 {
		return fmt.Errorf("protocol: compressed relay-hop requires relayed voice flags")
	}
	if err := validateCompressedRelayHopExtraFlags(f.Flags &^ FlagRelayed); err != nil {
		return err
	}
	if f.NetID != ctx.NetID {
		return fmt.Errorf("protocol: compressed relay-hop net_id=%d does not match context %d", f.NetID, ctx.NetID)
	}
	if f.SourceID != ctx.SourceID {
		return fmt.Errorf("protocol: compressed relay-hop source_id=%d does not match context %d", f.SourceID, ctx.SourceID)
	}
	if f.GroupID != ctx.GroupID {
		return fmt.Errorf("protocol: compressed relay-hop group_id=%d does not match context %d", f.GroupID, ctx.GroupID)
	}
	if f.RelayID != ctx.RelayID {
		return fmt.Errorf("protocol: compressed relay-hop relay_id=%d does not match context %d", f.RelayID, ctx.RelayID)
	}
	if f.CallID != ctx.CallID {
		return fmt.Errorf("protocol: compressed relay-hop call_id=%d does not match context %d", f.CallID, ctx.CallID)
	}
	if f.Hop > compressedRelayHopMaxNibble {
		return fmt.Errorf("protocol: compressed relay-hop hop=%d exceeds 4-bit max %d", f.Hop, compressedRelayHopMaxNibble)
	}
	if f.TTL > compressedRelayHopMaxNibble {
		return fmt.Errorf("protocol: compressed relay-hop ttl=%d exceeds 4-bit max %d", f.TTL, compressedRelayHopMaxNibble)
	}
	return nil
}

func validateCompressedRelayHopExtraFlags(flags uint16) error {
	if flags&^compressedRelayHopExtraFlagMask != 0 {
		return fmt.Errorf("protocol: compressed relay-hop unsupported extra flags 0x%04X", flags&^compressedRelayHopExtraFlagMask)
	}
	return nil
}

func compressedRelayHopSeqDelta(seq uint32, baseSeq uint32) (uint32, error) {
	if seq < baseSeq {
		return 0, fmt.Errorf("protocol: compressed relay-hop seq=%d is before base_seq=%d", seq, baseSeq)
	}
	delta := seq - baseSeq
	if delta > math.MaxUint16 {
		return 0, fmt.Errorf("protocol: compressed relay-hop seq_delta=%d exceeds uint16 max %d", delta, math.MaxUint16)
	}
	return delta, nil
}

func compressedRelayHopCreatedAtMs(ctx CompressedRelayHopContext, seqDelta uint32) (int64, error) {
	offset, err := compressedRelayHopMulMs(int64(seqDelta), ctx.FramePeriodMs)
	if err != nil {
		return 0, fmt.Errorf("protocol: compressed relay-hop created_at_ms offset: %w", err)
	}
	createdAtMs, err := compressedRelayHopAddMs(ctx.BaseCreatedAtMs, offset)
	if err != nil {
		return 0, fmt.Errorf("protocol: compressed relay-hop created_at_ms: %w", err)
	}
	return createdAtMs, nil
}

func compressedRelayHopSentDeltaMs(createdAtMs int64, sentAtMs int64) (int64, error) {
	if sentAtMs < createdAtMs {
		return 0, fmt.Errorf("protocol: compressed relay-hop sent_delta_ms is negative: sent_at_ms=%d created_at_ms=%d", sentAtMs, createdAtMs)
	}
	maxSentAtMs := int64(math.MaxInt64)
	if createdAtMs <= math.MaxInt64-compressedRelayHopMaxSentDeltaMs {
		maxSentAtMs = createdAtMs + compressedRelayHopMaxSentDeltaMs
	}
	if sentAtMs > maxSentAtMs {
		return 0, fmt.Errorf("protocol: compressed relay-hop sent_delta_ms exceeds uint8 max %d", compressedRelayHopMaxSentDeltaMs)
	}
	return sentAtMs - createdAtMs, nil
}

func compressedRelayHopMulMs(a int64, b int64) (int64, error) {
	if a < 0 || b < 0 {
		return 0, fmt.Errorf("negative simulated-time operand")
	}
	if a != 0 && b > math.MaxInt64/a {
		return 0, fmt.Errorf("simulated-time multiplication overflow")
	}
	return a * b, nil
}

func compressedRelayHopAddMs(a int64, b int64) (int64, error) {
	if b > 0 && a > math.MaxInt64-b {
		return 0, fmt.Errorf("simulated-time addition overflow")
	}
	if b < 0 && a < math.MinInt64-b {
		return 0, fmt.Errorf("simulated-time addition overflow")
	}
	return a + b, nil
}

func compressedRelayHopEnvelopeCRC(ctx CompressedRelayHopContext, prefix []byte, payload []byte) uint32 {
	h := crc32.NewIEEE()
	contextBytes := compressedRelayHopContextBytes(ctx)
	_, _ = h.Write(contextBytes[:])
	_, _ = h.Write(prefix)
	_, _ = h.Write(payload)
	return h.Sum32()
}

func compressedRelayHopContextBytes(ctx CompressedRelayHopContext) [32]byte {
	var out [32]byte
	binary.BigEndian.PutUint16(out[0:2], ctx.NetID)
	binary.BigEndian.PutUint16(out[2:4], ctx.SourceID)
	binary.BigEndian.PutUint16(out[4:6], ctx.GroupID)
	binary.BigEndian.PutUint16(out[6:8], ctx.RelayID)
	binary.BigEndian.PutUint32(out[8:12], ctx.CallID)
	binary.BigEndian.PutUint32(out[12:16], ctx.BaseSeq)
	binary.BigEndian.PutUint64(out[16:24], uint64(ctx.BaseCreatedAtMs))
	binary.BigEndian.PutUint64(out[24:32], uint64(ctx.FramePeriodMs))
	return out
}

func canonicalizeCompressedRelayHopFrame(f Frame) (Frame, error) {
	raw, err := f.Encode()
	if err != nil {
		return Frame{}, err
	}
	return DecodeStrict(raw)
}
