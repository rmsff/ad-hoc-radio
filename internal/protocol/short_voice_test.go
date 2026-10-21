package protocol

import (
	"bytes"
	"encoding/binary"
	"math"
	"reflect"
	"strings"
	"testing"
)

func sampleShortVoiceContext() ShortVoiceContext {
	return ShortVoiceContext{
		NetID:           0x0001,
		SourceID:        0x000A,
		GroupID:         0x0007,
		CallID:          0x00009001,
		BaseSeq:         40,
		BaseCreatedAtMs: 1_000_000,
		FramePeriodMs:   80,
	}
}

func sampleShortVoiceFrame() Frame {
	ctx := sampleShortVoiceContext()
	payload := []byte{0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70, 0x80}
	return Frame{
		Version:     Version,
		FrameType:   FrameTypeVoice,
		NetID:       ctx.NetID,
		SourceID:    ctx.SourceID,
		GroupID:     ctx.GroupID,
		RelayID:     0,
		CallID:      ctx.CallID,
		Seq:         ctx.BaseSeq + 2,
		Hop:         0,
		TTL:         3,
		Flags:       FlagDirect | FlagLateAllowed,
		CreatedAtMs: ctx.BaseCreatedAtMs + 2*ctx.FramePeriodMs,
		SentAtMs:    ctx.BaseCreatedAtMs + 2*ctx.FramePeriodMs + 20,
		PayloadLen:  uint16(len(payload)),
		Payload:     payload,
	}
}

func TestUT018_ShortVoiceRoundTripExpandsToCanonical(t *testing.T) {
	ctx := sampleShortVoiceContext()
	frame := sampleShortVoiceFrame()

	wire, err := EncodeShortVoice(frame, ctx)
	if err != nil {
		t.Fatalf("EncodeShortVoice failed: %v", err)
	}
	if got, want := len(wire), ShortVoiceHeaderSize+len(frame.Payload); got != want {
		t.Fatalf("short wire length = %d, want %d", got, want)
	}
	if got := len(wire); got != 22 {
		t.Fatalf("8-byte payload short wire length = %d, want exact 22-byte target", got)
	}

	decoded, err := DecodeShortVoice(wire, ctx)
	if err != nil {
		t.Fatalf("DecodeShortVoice failed: %v", err)
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

func TestUT018_ShortVoiceDeterministicBytes(t *testing.T) {
	ctx := sampleShortVoiceContext()
	frame := sampleShortVoiceFrame()

	first, err := EncodeShortVoice(frame, ctx)
	if err != nil {
		t.Fatalf("first EncodeShortVoice failed: %v", err)
	}
	second, err := EncodeShortVoice(frame, ctx)
	if err != nil {
		t.Fatalf("second EncodeShortVoice failed: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("short encoding is not deterministic:\nfirst=%x\nsecond=%x", first, second)
	}
}

func TestUT018_ShortVoiceRejectsRelayedFrame(t *testing.T) {
	ctx := sampleShortVoiceContext()
	frame := sampleCompressedRelayHopFrame()
	frame.NetID = ctx.NetID
	frame.SourceID = ctx.SourceID
	frame.GroupID = ctx.GroupID
	frame.CallID = ctx.CallID

	if _, err := EncodeShortVoice(frame, ctx); err == nil {
		t.Fatal("EncodeShortVoice accepted relayed voice; expected error")
	}
}

func TestUT018_ShortVoiceRejectsContextMismatch(t *testing.T) {
	ctx := sampleShortVoiceContext()
	frame := sampleShortVoiceFrame()
	frame.GroupID++

	if _, err := EncodeShortVoice(frame, ctx); err == nil {
		t.Fatal("EncodeShortVoice accepted frame whose group_id differs from context")
	}
}

func TestUT018_ShortVoiceRejectsMissingContext(t *testing.T) {
	frame := sampleShortVoiceFrame()
	if _, err := EncodeShortVoice(frame, ShortVoiceContext{}); err == nil {
		t.Fatal("EncodeShortVoice accepted empty context")
	}
	wire, err := EncodeShortVoice(frame, sampleShortVoiceContext())
	if err != nil {
		t.Fatalf("EncodeShortVoice failed: %v", err)
	}
	if _, err := DecodeShortVoice(wire, ShortVoiceContext{}); err == nil {
		t.Fatal("DecodeShortVoice accepted empty context")
	}
}

func TestUT018_ShortVoiceRejectsStaleContext(t *testing.T) {
	ctx := sampleShortVoiceContext()
	frame := sampleShortVoiceFrame()
	wire, err := EncodeShortVoice(frame, ctx)
	if err != nil {
		t.Fatalf("EncodeShortVoice failed: %v", err)
	}

	stale := ctx
	stale.GroupID++
	if _, err := DecodeShortVoice(wire, stale); err == nil {
		t.Fatal("DecodeShortVoice accepted stale context")
	}
}

func TestUT018_ShortVoiceRejectsUnsupportedExtraFlags(t *testing.T) {
	ctx := sampleShortVoiceContext()
	frame := sampleShortVoiceFrame()
	frame.Flags = FlagDirect | FlagCallStart
	if _, err := EncodeShortVoice(frame, ctx); err == nil {
		t.Fatal("EncodeShortVoice accepted unsupported call_start extra flag")
	}

	frame = sampleShortVoiceFrame()
	wire, err := EncodeShortVoice(frame, ctx)
	if err != nil {
		t.Fatalf("EncodeShortVoice failed: %v", err)
	}
	wire[1] = byte(FlagCallStart)
	binary.BigEndian.PutUint32(wire[10:14], shortVoiceEnvelopeCRC(ctx, wire[:10], wire[ShortVoiceHeaderSize:]))
	if _, err := DecodeShortVoice(wire, ctx); err == nil {
		t.Fatal("DecodeShortVoice accepted unsupported call_start extra flag")
	}
}

func TestUT018_ShortVoiceRejectsCorruptedHeader(t *testing.T) {
	ctx := sampleShortVoiceContext()
	frame := sampleShortVoiceFrame()
	wire, err := EncodeShortVoice(frame, ctx)
	if err != nil {
		t.Fatalf("EncodeShortVoice failed: %v", err)
	}
	wire[2] ^= 0x01

	if _, err := DecodeShortVoice(wire, ctx); err == nil {
		t.Fatal("DecodeShortVoice accepted corrupted short header")
	}
}

func TestUT018_ShortVoiceRejectsCorruptedPayload(t *testing.T) {
	ctx := sampleShortVoiceContext()
	frame := sampleShortVoiceFrame()
	wire, err := EncodeShortVoice(frame, ctx)
	if err != nil {
		t.Fatalf("EncodeShortVoice failed: %v", err)
	}
	wire[ShortVoiceHeaderSize] ^= 0x01

	if _, err := DecodeShortVoice(wire, ctx); err == nil {
		t.Fatal("DecodeShortVoice accepted corrupted short payload")
	}
}

func TestUT018_ShortVoiceRejectsDeltaOverflow(t *testing.T) {
	ctx := sampleShortVoiceContext()
	frame := sampleShortVoiceFrame()
	frame.Seq = ctx.BaseSeq + math.MaxUint16 + 1

	if _, err := EncodeShortVoice(frame, ctx); err == nil || !strings.Contains(err.Error(), "seq_delta") {
		t.Fatalf("EncodeShortVoice error = %v, want seq_delta overflow", err)
	}
}

func TestUT018_ShortVoiceRejectsCreatedAtMismatch(t *testing.T) {
	ctx := sampleShortVoiceContext()
	frame := sampleShortVoiceFrame()
	frame.CreatedAtMs++
	frame.SentAtMs++

	if _, err := EncodeShortVoice(frame, ctx); err == nil || !strings.Contains(err.Error(), "created_at_ms") {
		t.Fatalf("EncodeShortVoice error = %v, want created_at_ms mismatch", err)
	}
}

func TestUT018_ShortVoiceRejectsSentDeltaOverflow(t *testing.T) {
	ctx := sampleShortVoiceContext()
	frame := sampleShortVoiceFrame()
	frame.SentAtMs = frame.CreatedAtMs + 256

	if _, err := EncodeShortVoice(frame, ctx); err == nil || !strings.Contains(err.Error(), "sent_delta_ms") {
		t.Fatalf("EncodeShortVoice error = %v, want sent_delta_ms overflow", err)
	}
}

func TestUT018_ShortVoiceRejectsHopOrTTLNibbleOverflow(t *testing.T) {
	ctx := sampleShortVoiceContext()
	for _, mutate := range []struct {
		name string
		fn   func(*Frame)
	}{
		{name: "hop", fn: func(f *Frame) { f.Hop = 1 }},
		{name: "ttl", fn: func(f *Frame) { f.TTL = 16 }},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			frame := sampleShortVoiceFrame()
			mutate.fn(&frame)
			if _, err := EncodeShortVoice(frame, ctx); err == nil {
				t.Fatal("EncodeShortVoice accepted source hop/ttl violation")
			}
		})
	}
}

func TestUT018_ShortVoicePayloadDefensiveCopy(t *testing.T) {
	ctx := sampleShortVoiceContext()
	frame := sampleShortVoiceFrame()
	wire, err := EncodeShortVoice(frame, ctx)
	if err != nil {
		t.Fatalf("EncodeShortVoice failed: %v", err)
	}
	decoded, err := DecodeShortVoice(wire, ctx)
	if err != nil {
		t.Fatalf("DecodeShortVoice failed: %v", err)
	}

	wire[ShortVoiceHeaderSize] ^= 0xFF
	if bytes.Equal(decoded.Payload, wire[ShortVoiceHeaderSize:]) {
		t.Fatal("decoded short payload aliases input wire")
	}
}
