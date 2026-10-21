package codec

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestUT012_DummyCodecEncodesDeterministicPayload(t *testing.T) {
	codec := DummyCodec{}
	input := Input{Seq: 7, CreatedAtMs: 1_560}

	first, err := codec.Encode(input)
	if err != nil {
		t.Fatalf("Encode first failed: %v", err)
	}
	second, err := codec.Encode(input)
	if err != nil {
		t.Fatalf("Encode second failed: %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Fatalf("Encode produced different payloads: %x != %x", first, second)
	}
	if len(first) == 0 {
		t.Fatal("Encode returned empty payload")
	}
}

func TestUT012_DummyCodecRoundTripsInput(t *testing.T) {
	codec := DummyCodec{}
	input := Input{Seq: 42, CreatedAtMs: 4_360}

	payload, err := codec.Encode(input)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	decoded, err := codec.Decode(payload)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if decoded != input {
		t.Fatalf("Decode = %+v, want %+v", decoded, input)
	}
}

func TestUT012_DummyCodecRejectsInvalidPayload(t *testing.T) {
	codec := DummyCodec{}

	if _, err := codec.Encode(Input{Seq: 1, CreatedAtMs: -1}); err == nil {
		t.Fatal("Encode accepted negative CreatedAtMs")
	}

	if _, err := codec.Decode([]byte{0x01, 0x02}); err == nil {
		t.Fatal("Decode accepted invalid payload length")
	}

	payload, err := codec.Encode(Input{Seq: 1, CreatedAtMs: 1_000})
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	payload[0] ^= 0xFF
	if _, err := codec.Decode(payload); err == nil {
		t.Fatal("Decode accepted invalid magic")
	}

	payload, err = codec.Encode(Input{Seq: 2, CreatedAtMs: 2_000})
	if err != nil {
		t.Fatalf("Encode second payload failed: %v", err)
	}
	binary.BigEndian.PutUint64(payload[8:16], 0x8000000000000000)
	if _, err := codec.Decode(payload); err == nil {
		t.Fatal("Decode accepted negative CreatedAtMs encoding")
	}
}

func TestUT012_CodecSpyCountsOnlyHarnessCalls(t *testing.T) {
	spy := NewSpy(DummyCodec{})
	input := Input{Seq: 3, CreatedAtMs: 1_240}

	payload, err := spy.Encode(input)
	if err != nil {
		t.Fatalf("spy Encode failed: %v", err)
	}
	if spy.EncodeCalls() != 1 {
		t.Fatalf("EncodeCalls = %d, want 1", spy.EncodeCalls())
	}
	if spy.DecodeCalls() != 0 {
		t.Fatalf("DecodeCalls = %d, want 0", spy.DecodeCalls())
	}

	decoded, err := spy.Decode(payload)
	if err != nil {
		t.Fatalf("spy Decode failed: %v", err)
	}
	if decoded != input {
		t.Fatalf("Decode = %+v, want %+v", decoded, input)
	}
	if spy.EncodeCalls() != 1 {
		t.Fatalf("EncodeCalls after Decode = %d, want 1", spy.EncodeCalls())
	}
	if spy.DecodeCalls() != 1 {
		t.Fatalf("DecodeCalls = %d, want 1", spy.DecodeCalls())
	}
}
