package protocol

import (
	"bytes"
	"encoding/binary"
	"math"
	"reflect"
	"strings"
	"testing"
)

func sampleCompressedRelayHopContext() CompressedRelayHopContext {
	return CompressedRelayHopContext{
		NetID:           0x0001,
		SourceID:        0x000A,
		GroupID:         0x0007,
		RelayID:         0x0002,
		CallID:          0x00009001,
		BaseSeq:         40,
		BaseCreatedAtMs: 1_000_000,
		FramePeriodMs:   80,
	}
}

func sampleCompressedRelayHopFrame() Frame {
	ctx := sampleCompressedRelayHopContext()
	payload := []byte{0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70, 0x80}
	return Frame{
		Version:     Version,
		FrameType:   FrameTypeVoice,
		NetID:       ctx.NetID,
		SourceID:    ctx.SourceID,
		GroupID:     ctx.GroupID,
		RelayID:     ctx.RelayID,
		CallID:      ctx.CallID,
		Seq:         ctx.BaseSeq + 2,
		Hop:         1,
		TTL:         3,
		Flags:       FlagRelayed | FlagLateAllowed,
		CreatedAtMs: ctx.BaseCreatedAtMs + 2*ctx.FramePeriodMs,
		SentAtMs:    ctx.BaseCreatedAtMs + 2*ctx.FramePeriodMs + 20,
		PayloadLen:  uint16(len(payload)),
		Payload:     payload,
	}
}

func canonicalizedFrame(t *testing.T, f Frame) Frame {
	t.Helper()
	raw, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode canonical frame failed: %v", err)
	}
	decoded, err := DecodeStrict(raw)
	if err != nil {
		t.Fatalf("DecodeStrict canonical frame failed: %v", err)
	}
	return decoded
}

func TestUT017_CompressedRelayHopRoundTripExpandsToCanonical(t *testing.T) {
	ctx := sampleCompressedRelayHopContext()
	frame := sampleCompressedRelayHopFrame()

	wire, err := EncodeCompressedRelayHop(frame, ctx)
	if err != nil {
		t.Fatalf("EncodeCompressedRelayHop failed: %v", err)
	}
	if got, want := len(wire), CompressedRelayHopHeaderSize+len(frame.Payload); got != want {
		t.Fatalf("compressed wire length = %d, want %d", got, want)
	}
	if got := len(wire); got != 22 {
		t.Fatalf("8-byte payload compressed wire length = %d, want exact 22-byte target", got)
	}

	decoded, err := DecodeCompressedRelayHop(wire, ctx)
	if err != nil {
		t.Fatalf("DecodeCompressedRelayHop failed: %v", err)
	}
	want := canonicalizedFrame(t, frame)
	if decoded.OriginKey() != want.OriginKey() {
		t.Fatalf("OriginKey = %+v, want %+v", decoded.OriginKey(), want.OriginKey())
	}
	if decoded.CopyKey() != want.CopyKey() {
		t.Fatalf("CopyKey = %+v, want %+v", decoded.CopyKey(), want.CopyKey())
	}
	if !bytes.Equal(decoded.Payload, want.Payload) {
		t.Fatalf("Payload = %x, want %x", decoded.Payload, want.Payload)
	}
	if !reflect.DeepEqual(decoded, want) {
		t.Fatalf("decoded canonical frame = %+v, want %+v", decoded, want)
	}
}

func TestUT017_CompressedRelayHopDeterministicBytes(t *testing.T) {
	ctx := sampleCompressedRelayHopContext()
	frame := sampleCompressedRelayHopFrame()

	first, err := EncodeCompressedRelayHop(frame, ctx)
	if err != nil {
		t.Fatalf("first EncodeCompressedRelayHop failed: %v", err)
	}
	second, err := EncodeCompressedRelayHop(frame, ctx)
	if err != nil {
		t.Fatalf("second EncodeCompressedRelayHop failed: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("compressed encoding is not deterministic:\nfirst=%x\nsecond=%x", first, second)
	}
}

func TestUT017_CompressedRelayHopRejectsDirectFrame(t *testing.T) {
	ctx := sampleCompressedRelayHopContext()
	frame := sampleVoiceFrame()
	frame.NetID = ctx.NetID
	frame.SourceID = ctx.SourceID
	frame.GroupID = ctx.GroupID
	frame.CallID = ctx.CallID

	if _, err := EncodeCompressedRelayHop(frame, ctx); err == nil {
		t.Fatal("EncodeCompressedRelayHop accepted direct/source voice; expected error")
	}
}

func TestUT017_CompressedRelayHopRejectsContextMismatch(t *testing.T) {
	ctx := sampleCompressedRelayHopContext()
	frame := sampleCompressedRelayHopFrame()
	frame.GroupID++

	if _, err := EncodeCompressedRelayHop(frame, ctx); err == nil {
		t.Fatal("EncodeCompressedRelayHop accepted frame whose group_id differs from context")
	}
}

func TestUT017_CompressedRelayHopRejectsMissingContext(t *testing.T) {
	frame := sampleCompressedRelayHopFrame()
	if _, err := EncodeCompressedRelayHop(frame, CompressedRelayHopContext{}); err == nil {
		t.Fatal("EncodeCompressedRelayHop accepted empty context")
	}
	wire, err := EncodeCompressedRelayHop(frame, sampleCompressedRelayHopContext())
	if err != nil {
		t.Fatalf("EncodeCompressedRelayHop failed: %v", err)
	}
	if _, err := DecodeCompressedRelayHop(wire, CompressedRelayHopContext{}); err == nil {
		t.Fatal("DecodeCompressedRelayHop accepted empty context")
	}
}

func TestUT017_CompressedRelayHopRejectsStaleContext(t *testing.T) {
	ctx := sampleCompressedRelayHopContext()
	frame := sampleCompressedRelayHopFrame()
	wire, err := EncodeCompressedRelayHop(frame, ctx)
	if err != nil {
		t.Fatalf("EncodeCompressedRelayHop failed: %v", err)
	}

	stale := ctx
	stale.GroupID++
	if _, err := DecodeCompressedRelayHop(wire, stale); err == nil {
		t.Fatal("DecodeCompressedRelayHop accepted stale context")
	}
}

func TestUT017_CompressedRelayHopRejectsUnsupportedExtraFlags(t *testing.T) {
	ctx := sampleCompressedRelayHopContext()
	frame := sampleCompressedRelayHopFrame()
	frame.Flags = FlagRelayed | FlagCallStart
	if _, err := EncodeCompressedRelayHop(frame, ctx); err == nil {
		t.Fatal("EncodeCompressedRelayHop accepted unsupported call_start extra flag")
	}

	frame = sampleCompressedRelayHopFrame()
	wire, err := EncodeCompressedRelayHop(frame, ctx)
	if err != nil {
		t.Fatalf("EncodeCompressedRelayHop failed: %v", err)
	}
	wire[1] = byte(FlagCallStart)
	binary.BigEndian.PutUint32(wire[10:14], compressedRelayHopEnvelopeCRC(ctx, wire[:10], wire[CompressedRelayHopHeaderSize:]))
	if _, err := DecodeCompressedRelayHop(wire, ctx); err == nil {
		t.Fatal("DecodeCompressedRelayHop accepted unsupported call_start extra flag")
	}
}

func TestUT017_CompressedRelayHopRejectsCorruptedHeader(t *testing.T) {
	ctx := sampleCompressedRelayHopContext()
	frame := sampleCompressedRelayHopFrame()
	wire, err := EncodeCompressedRelayHop(frame, ctx)
	if err != nil {
		t.Fatalf("EncodeCompressedRelayHop failed: %v", err)
	}
	wire[2] ^= 0x01

	if _, err := DecodeCompressedRelayHop(wire, ctx); err == nil {
		t.Fatal("DecodeCompressedRelayHop accepted corrupted compressed header")
	}
}

func TestUT017_CompressedRelayHopRejectsCorruptedPayload(t *testing.T) {
	ctx := sampleCompressedRelayHopContext()
	frame := sampleCompressedRelayHopFrame()
	wire, err := EncodeCompressedRelayHop(frame, ctx)
	if err != nil {
		t.Fatalf("EncodeCompressedRelayHop failed: %v", err)
	}
	wire[CompressedRelayHopHeaderSize] ^= 0x01

	if _, err := DecodeCompressedRelayHop(wire, ctx); err == nil {
		t.Fatal("DecodeCompressedRelayHop accepted corrupted compressed payload")
	}
}

func TestUT017_CompressedRelayHopRejectsDeltaOverflow(t *testing.T) {
	ctx := sampleCompressedRelayHopContext()
	frame := sampleCompressedRelayHopFrame()
	frame.Seq = ctx.BaseSeq + math.MaxUint16 + 1

	if _, err := EncodeCompressedRelayHop(frame, ctx); err == nil || !strings.Contains(err.Error(), "seq_delta") {
		t.Fatalf("EncodeCompressedRelayHop error = %v, want seq_delta overflow", err)
	}
}

func TestUT017_CompressedRelayHopRejectsCreatedAtMismatch(t *testing.T) {
	ctx := sampleCompressedRelayHopContext()
	frame := sampleCompressedRelayHopFrame()
	frame.CreatedAtMs++
	frame.SentAtMs++

	if _, err := EncodeCompressedRelayHop(frame, ctx); err == nil || !strings.Contains(err.Error(), "created_at_ms") {
		t.Fatalf("EncodeCompressedRelayHop error = %v, want created_at_ms mismatch", err)
	}
}

func TestUT017_CompressedRelayHopRejectsSentDeltaOverflow(t *testing.T) {
	ctx := sampleCompressedRelayHopContext()
	frame := sampleCompressedRelayHopFrame()
	frame.SentAtMs = frame.CreatedAtMs + 256

	if _, err := EncodeCompressedRelayHop(frame, ctx); err == nil || !strings.Contains(err.Error(), "sent_delta_ms") {
		t.Fatalf("EncodeCompressedRelayHop error = %v, want sent_delta_ms overflow", err)
	}
}

func TestUT017_CompressedRelayHopRejectsHopOrTTLNibbleOverflow(t *testing.T) {
	ctx := sampleCompressedRelayHopContext()
	for _, mutate := range []struct {
		name string
		fn   func(*Frame)
	}{
		{name: "hop", fn: func(f *Frame) { f.Hop = 16 }},
		{name: "ttl", fn: func(f *Frame) { f.TTL = 16 }},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			frame := sampleCompressedRelayHopFrame()
			mutate.fn(&frame)
			if _, err := EncodeCompressedRelayHop(frame, ctx); err == nil {
				t.Fatal("EncodeCompressedRelayHop accepted nibble overflow")
			}
		})
	}
}

func TestUT017_CompressedRelayHopPayloadDefensiveCopy(t *testing.T) {
	ctx := sampleCompressedRelayHopContext()
	frame := sampleCompressedRelayHopFrame()
	wire, err := EncodeCompressedRelayHop(frame, ctx)
	if err != nil {
		t.Fatalf("EncodeCompressedRelayHop failed: %v", err)
	}
	decoded, err := DecodeCompressedRelayHop(wire, ctx)
	if err != nil {
		t.Fatalf("DecodeCompressedRelayHop failed: %v", err)
	}

	wire[CompressedRelayHopHeaderSize] ^= 0xFF
	if bytes.Equal(decoded.Payload, wire[CompressedRelayHopHeaderSize:]) {
		t.Fatal("decoded compressed payload aliases input wire")
	}
}
