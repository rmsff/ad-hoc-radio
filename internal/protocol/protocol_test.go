package protocol

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"math"
	"testing"
)

// sampleVoiceFrame builds a canonical direct Voice Frame with all
// required fields populated, for use across Story 1.2 tests.
func sampleVoiceFrame() Frame {
	payload := []byte{0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70, 0x80}
	return Frame{
		Version:      Version,
		FrameType:    FrameTypeVoice,
		NetID:        0x0001,
		SourceID:     0x000A,
		GroupID:      0x0007,
		RelayID:      0x0000,
		CallID:       0x00009001,
		Seq:          0x0000002A,
		Hop:          0,
		TTL:          1,
		Flags:        FlagDirect,
		CreatedAtMs:  1_000_000,
		SentAtMs:     1_000_005,
		PayloadLen:   uint16(len(payload)),
		Payload:      payload,
		HeaderCRC32:  0,
		PayloadCRC32: 0,
	}
}

// TestUT001_FrameRoundTrip is AC-1: a valid Voice Frame with all
// required canonical fields round-trips exactly through Encode/Decode.
//
// Traceability: UT-001 (FR-2).
func TestUT001_FrameRoundTrip(t *testing.T) {
	orig := sampleVoiceFrame()

	encoded, err := orig.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Header is exactly 48 bytes; payload follows.
	if got := len(encoded); got != headerSize+len(orig.Payload) {
		t.Fatalf("encoded length = %d, want %d", got, headerSize+len(orig.Payload))
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Field-by-field exact comparison (AC-1: "all fields round-trip exactly").
	// CRC fields are computed by Encode, not set by the caller, so they are
	// verified separately below against recomputed values.
	cases := []struct {
		name string
		got  uint64
		want uint64
	}{
		{"Version", uint64(decoded.Version), uint64(orig.Version)},
		{"FrameType", uint64(decoded.FrameType), uint64(orig.FrameType)},
		{"NetID", uint64(decoded.NetID), uint64(orig.NetID)},
		{"SourceID", uint64(decoded.SourceID), uint64(orig.SourceID)},
		{"GroupID", uint64(decoded.GroupID), uint64(orig.GroupID)},
		{"RelayID", uint64(decoded.RelayID), uint64(orig.RelayID)},
		{"CallID", uint64(decoded.CallID), uint64(orig.CallID)},
		{"Seq", uint64(decoded.Seq), uint64(orig.Seq)},
		{"Hop", uint64(decoded.Hop), uint64(orig.Hop)},
		{"TTL", uint64(decoded.TTL), uint64(orig.TTL)},
		{"Flags", uint64(decoded.Flags), uint64(orig.Flags)},
		{"PayloadLen", uint64(decoded.PayloadLen), uint64(orig.PayloadLen)},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, c.got, c.want)
		}
	}
	if decoded.CreatedAtMs != orig.CreatedAtMs {
		t.Errorf("CreatedAtMs: got %d, want %d", decoded.CreatedAtMs, orig.CreatedAtMs)
	}
	if decoded.SentAtMs != orig.SentAtMs {
		t.Errorf("SentAtMs: got %d, want %d", decoded.SentAtMs, orig.SentAtMs)
	}
	if !bytes.Equal(decoded.Payload, orig.Payload) {
		t.Errorf("Payload: got %x, want %x", decoded.Payload, orig.Payload)
	}
	// CRCs must be non-zero after encode and match the recomputed values.
	if decoded.HeaderCRC32 == 0 {
		t.Error("HeaderCRC32 should be non-zero after decode")
	}
	if decoded.PayloadCRC32 == 0 {
		t.Error("PayloadCRC32 should be non-zero after decode")
	}
}

// TestUT002_InvalidHeaderCRC is AC-2: a frame with invalid
// header_crc32 must be rejected by Decode.
//
// Traceability: UT-002 (FR-2).
func TestUT002_InvalidHeaderCRC(t *testing.T) {
	orig := sampleVoiceFrame()
	encoded, err := orig.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Corrupt header_crc32 (bytes 40:44).
	binary.BigEndian.PutUint32(encoded[40:44], 0xDEADBEEF)

	if _, err := Decode(encoded); err == nil {
		t.Fatal("Decode accepted a frame with invalid header_crc32; expected error")
	}
}

// TestUT002_InvalidPayloadCRC is AC-2: a frame with invalid
// payload_crc32 must be rejected by Decode.
func TestUT002_InvalidPayloadCRC(t *testing.T) {
	orig := sampleVoiceFrame()
	encoded, err := orig.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Corrupt payload_crc32 (bytes 44:48). Header CRC was computed with
	// the original payload_crc32, so this must fail at header CRC check
	// OR payload CRC check depending on order — either way it must fail.
	binary.BigEndian.PutUint32(encoded[44:48], 0xCAFEBABE)

	if _, err := Decode(encoded); err == nil {
		t.Fatal("Decode accepted a frame with invalid payload_crc32; expected error")
	}
}

// TestUT002_CorruptedPayloadBytes is AC-2 from the payload integrity
// angle: a frame whose payload bytes are changed (so payload_crc32 no
// longer matches) must be rejected.
func TestUT002_CorruptedPayloadBytes(t *testing.T) {
	orig := sampleVoiceFrame()
	encoded, err := orig.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Flip a payload bit. payload_crc32 in the header is unchanged but
	// recomputed payload CRC will not match.
	encoded[headerSize+1] ^= 0xFF

	if _, err := Decode(encoded); err == nil {
		t.Fatal("Decode accepted a frame with corrupted payload bytes; expected error")
	}
}

// TestUT003_OriginKeyIdenticalDirectAndRelayed is AC-3: direct and
// relayed copies of the same source frame produce identical OriginKey
// = (net_id, source_id, call_id, seq), while CopyKey differs.
//
// Traceability: UT-003 (FR-4).
func TestUT003_OriginKeyIdenticalDirectAndRelayed(t *testing.T) {
	direct := sampleVoiceFrame() // relay_id=0, hop=0, flags=direct

	relayed := direct
	relayed.RelayID = 0x0002
	relayed.Hop = 1
	relayed.TTL = 0
	relayed.Flags = FlagRelayed
	relayed.SentAtMs = direct.SentAtMs + 60

	dok := direct.OriginKey()
	rok := relayed.OriginKey()
	if dok != rok {
		t.Fatalf("OriginKey differs between direct and relayed copies:\n  direct  = %+v\n  relayed = %+v", dok, rok)
	}

	// Sanity: OriginKey matches the canonical (net_id, source_id, call_id, seq).
	want := OriginKey{NetID: direct.NetID, SourceID: direct.SourceID, CallID: direct.CallID, Seq: direct.Seq}
	if dok != want {
		t.Errorf("OriginKey = %+v, want %+v", dok, want)
	}

	// CopyKey must differ (different relay_id and hop).
	dck := direct.CopyKey()
	rck := relayed.CopyKey()
	if dck == rck {
		t.Fatalf("CopyKey equal for direct and relayed copies:\n  direct  = %+v\n  relayed = %+v", dck, rck)
	}
	if dck.RelayID != 0 || dck.Hop != 0 {
		t.Errorf("direct CopyKey should have relay_id=0 hop=0; got %+v", dck)
	}
	if rck.RelayID != relayed.RelayID || rck.Hop != relayed.Hop {
		t.Errorf("relayed CopyKey should carry relay_id and hop; got %+v", rck)
	}
}

// TestEncode_RejectsPayloadLenMismatch guards against a caller setting
// PayloadLen inconsistently with len(Payload).
func TestEncode_RejectsPayloadLenMismatch(t *testing.T) {
	f := sampleVoiceFrame()
	f.PayloadLen = f.PayloadLen + 1
	if _, err := f.Encode(); err == nil {
		t.Fatal("Encode accepted mismatched PayloadLen; expected error")
	}
}

// TestDecode_RejectsShortInput guards against truncated input.
func TestDecode_RejectsShortInput(t *testing.T) {
	short := make([]byte, headerSize-1)
	if _, err := Decode(short); err == nil {
		t.Fatal("Decode accepted input shorter than header; expected error")
	}
}

// TestEncodeDecode_DeterministicBytes ensures two encodes of the same
// frame produce identical bytes (NFR-1 / frame-contract.md §1.7).
func TestEncodeDecode_DeterministicBytes(t *testing.T) {
	f := sampleVoiceFrame()
	a, err := f.Encode()
	if err != nil {
		t.Fatalf("first Encode failed: %v", err)
	}
	b, err := f.Encode()
	if err != nil {
		t.Fatalf("second Encode failed: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("encode not deterministic:\n  a=%x\n  b=%x", a, b)
	}
}

// TestDecode_PayloadDefensiveCopy ensures Decode copies payload bytes
// so mutating the input buffer after decode does not affect the
// returned Frame.Payload.
func TestDecode_PayloadDefensiveCopy(t *testing.T) {
	orig := sampleVoiceFrame()
	encoded, err := orig.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	encoded[headerSize] = 0xFF
	if decoded.Payload[0] == 0xFF {
		t.Fatal("decoded Payload aliases input buffer; expected a defensive copy")
	}
}

func TestValidate_RejectsPayloadLenMismatch(t *testing.T) {
	f := sampleVoiceFrame()
	f.PayloadLen++

	if err := f.Validate(); err == nil {
		t.Fatal("Validate accepted mismatched PayloadLen; expected error")
	}
}

// TestUT002_DecodeRejectsTrailingBytes is Review Finding 1: canonical
// single-frame Decode must require exact length header + payload_len.
// Input with trailing garbage after the declared payload must be
// rejected (frame-contract.md §4.2).
func TestUT002_DecodeRejectsTrailingBytes(t *testing.T) {
	orig := sampleVoiceFrame()
	encoded, err := orig.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	// Append trailing bytes after the canonical frame.
	withTrailing := append([]byte(nil), encoded...)
	withTrailing = append(withTrailing, 0xDE, 0xAD, 0xBE, 0xEF)

	if _, err := Decode(withTrailing); err == nil {
		t.Fatal("Decode accepted input with trailing bytes after declared payload_len; expected error")
	}
}

// TestEncode_RejectsOversizedPayload is Review Finding 2: Encode must
// reject payloads larger than the uint16 payload_len range, because
// the uint16 cast would wrap silently and produce non-canonical,
// non-round-trippable bytes (frame-contract.md §3).
func TestEncode_RejectsOversizedPayload(t *testing.T) {
	f := sampleVoiceFrame()
	f.Payload = make([]byte, math.MaxUint16+1) // 65536 bytes
	f.PayloadLen = 0                           // would wrap-match via uint16 cast

	if _, err := f.Encode(); err == nil {
		t.Fatal("Encode accepted a payload larger than uint16 max; expected error")
	}
}

// TestEncode_RejectsOversizedPayloadEvenWithMatchingLen ensures the
// overflow guard fires before the len/payload_len equality check, so a
// genuinely oversized payload is reported as too-large rather than
// mismatched.
func TestEncode_RejectsOversizedPayloadEvenWithMatchingLen(t *testing.T) {
	f := sampleVoiceFrame()
	f.Payload = make([]byte, math.MaxUint16+1)
	f.PayloadLen = uint16(len(f.Payload)) // intentionally wrapped to 0

	if _, err := f.Encode(); err == nil {
		t.Fatal("Encode accepted a 65536-byte payload with wrapped PayloadLen; expected too-large error")
	}
}

// TestUT002_DecodeRejectsUndefinedFlagBits is Review Finding 3a:
// undefined flag bits must be zero in MVP (frame-contract.md §5). A
// frame with bit 15 set must be rejected even if CRCs pass.
func TestUT002_DecodeRejectsUndefinedFlagBits(t *testing.T) {
	orig := sampleVoiceFrame()
	encoded, err := orig.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	// Set an undefined bit (0x8000) and fix up header_crc32 so the
	// header CRC check passes, isolating the flag validation.
	binary.BigEndian.PutUint16(encoded[20:22], orig.Flags|0x8000)
	fixHeaderCRC(encoded)

	if _, err := Decode(encoded); err == nil {
		t.Fatal("Decode accepted a frame with undefined flag bit 0x8000 set; expected error")
	}
}

// TestUT002_DecodeRejectsDirectAndRelayedBothSet is Review Finding 3b:
// direct and relayed must not both be set in MVP
// (frame-contract.md §5).
func TestUT002_DecodeRejectsDirectAndRelayedBothSet(t *testing.T) {
	orig := sampleVoiceFrame()
	encoded, err := orig.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	// Set both direct and relayed, fix up header_crc32.
	binary.BigEndian.PutUint16(encoded[20:22], FlagDirect|FlagRelayed)
	fixHeaderCRC(encoded)

	if _, err := Decode(encoded); err == nil {
		t.Fatal("Decode accepted a frame with both direct and relayed flags set; expected error")
	}
}

// TestDecode_AcceptsAllDefinedFlags proves that a frame using every
// defined flag bit except the mutually-exclusive direct/relayed pair
// still decodes, so the flag validation is not over-restrictive.
func TestDecode_AcceptsAllDefinedFlags(t *testing.T) {
	orig := sampleVoiceFrame()
	orig.Flags = FlagDirect | FlagCallStart | FlagCallEnd | FlagLateAllowed | FlagTestCorrupted
	encoded, err := orig.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode rejected a frame with all defined (non-conflicting) flags: %v", err)
	}
	if decoded.Flags != orig.Flags {
		t.Errorf("flags round-trip: got 0x%04X, want 0x%04X", decoded.Flags, orig.Flags)
	}
}

// fixHeaderCRC recomputes and writes header_crc32 into an encoded frame
// so a test can mutate header fields and still pass the header CRC
// check, isolating downstream validation (flag checks, length checks).
func fixHeaderCRC(encoded []byte) {
	binary.BigEndian.PutUint32(encoded[40:44], 0)
	headerCRC := crc32.ChecksumIEEE(encoded[:headerSize])
	binary.BigEndian.PutUint32(encoded[40:44], headerCRC)
}

// reencodeValidFrame encodes f and is used by Validate tests as a
// baseline for a known-good frame before they mutate a single field
// and re-fix the CRC.
func encodeWithFixedCRC(t *testing.T, f Frame) []byte {
	t.Helper()
	enc, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return enc
}

// mutateFieldAndFixCRC changes a single header field in an encoded
// frame and re-fixes the header CRC so Decode's CRC check passes and
// the mutated field reaches Validate. mutator operates on the 48-byte
// header slice in place.
func mutateFieldAndFixCRC(t *testing.T, enc []byte, mutator func(header []byte)) []byte {
	t.Helper()
	out := append([]byte(nil), enc...)
	mutator(out[:headerSize])
	fixHeaderCRC(out)
	return out
}

// TestValidate_RejectsWrongVersion is Review Finding 3: a frame with
// Version != 1 must fail semantic validation even if CRCs pass.
func TestValidate_RejectsWrongVersion(t *testing.T) {
	enc := encodeWithFixedCRC(t, sampleVoiceFrame())
	bad := mutateFieldAndFixCRC(t, enc, func(h []byte) { h[0] = 2 })
	if _, err := DecodeStrict(bad); err == nil {
		t.Fatal("DecodeStrict accepted a frame with version=2; want Validate error")
	}
}

// TestValidate_RejectsUnknownFrameType verifies an unknown frame_type
// (0xFF) is rejected by Validate.
func TestValidate_RejectsUnknownFrameType(t *testing.T) {
	enc := encodeWithFixedCRC(t, sampleVoiceFrame())
	bad := mutateFieldAndFixCRC(t, enc, func(h []byte) { h[1] = 0xFF })
	if _, err := DecodeStrict(bad); err == nil {
		t.Fatal("DecodeStrict accepted a frame with frame_type=0xFF; want Validate error")
	}
}

// TestValidate_RejectsGroupIDZero verifies group_id=0 is rejected per
// frame-contract.md §18.
func TestValidate_RejectsGroupIDZero(t *testing.T) {
	enc := encodeWithFixedCRC(t, sampleVoiceFrame())
	bad := mutateFieldAndFixCRC(t, enc, func(h []byte) {
		binary.BigEndian.PutUint16(h[6:8], 0)
	})
	if _, err := DecodeStrict(bad); err == nil {
		t.Fatal("DecodeStrict accepted a frame with group_id=0; want Validate error (§18)")
	}
}

// TestValidate_RejectsSourceIDZero verifies source_id=0 is rejected
// per frame-contract.md §4.3.
func TestValidate_RejectsSourceIDZero(t *testing.T) {
	enc := encodeWithFixedCRC(t, sampleVoiceFrame())
	bad := mutateFieldAndFixCRC(t, enc, func(h []byte) {
		binary.BigEndian.PutUint16(h[4:6], 0)
	})
	if _, err := DecodeStrict(bad); err == nil {
		t.Fatal("DecodeStrict accepted a frame with source_id=0; want Validate error (§4.3)")
	}
}

// TestValidate_DirectRelayedConsistency verifies the §5/§10
// relationship: a direct frame must have relay_id=0 and hop=0; a
// relayed frame must have relay_id!=0 and hop>0.
func TestValidate_DirectRelayedConsistency(t *testing.T) {
	// Direct frame with relay_id != 0.
	enc := encodeWithFixedCRC(t, sampleVoiceFrame()) // flags=direct, relay_id=0, hop=0
	bad := mutateFieldAndFixCRC(t, enc, func(h []byte) {
		binary.BigEndian.PutUint16(h[8:10], 5) // relay_id=5 but flags=direct
	})
	if _, err := DecodeStrict(bad); err == nil {
		t.Error("DecodeStrict accepted a direct frame with relay_id=5; want Validate error (§5)")
	}

	// Direct frame with hop != 0.
	bad2 := mutateFieldAndFixCRC(t, enc, func(h []byte) { h[18] = 2 })
	if _, err := DecodeStrict(bad2); err == nil {
		t.Error("DecodeStrict accepted a direct frame with hop=2; want Validate error (§10)")
	}

	// Relayed frame with relay_id = 0.
	relayed := sampleVoiceFrame()
	relayed.Flags = FlagRelayed
	relayed.RelayID = 2
	relayed.Hop = 1
	relayedEnc := encodeWithFixedCRC(t, relayed)
	bad3 := mutateFieldAndFixCRC(t, relayedEnc, func(h []byte) {
		binary.BigEndian.PutUint16(h[8:10], 0) // relay_id=0 but flags=relayed
	})
	if _, err := DecodeStrict(bad3); err == nil {
		t.Error("DecodeStrict accepted a relayed frame with relay_id=0; want Validate error (§5)")
	}

	// Relayed frame with hop = 0.
	bad4 := mutateFieldAndFixCRC(t, relayedEnc, func(h []byte) { h[18] = 0 })
	if _, err := DecodeStrict(bad4); err == nil {
		t.Error("DecodeStrict accepted a relayed frame with hop=0; want Validate error (§10)")
	}
}

// TestValidate_RejectsInvalidFlagsOnFrameValue verifies that Validate is
// safe on manually constructed Frame values too, not only after Decode.
// Decode already rejects these flags on the binary path; Validate must
// also reject them on the direct Frame path.
func TestValidate_RejectsInvalidFlagsOnFrameValue(t *testing.T) {
	cases := []struct {
		name  string
		flags uint16
	}{
		{"direct_and_relayed", FlagDirect | FlagRelayed},
		{"undefined_bit", FlagDirect | 0x8000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := sampleVoiceFrame()
			f.Flags = c.flags
			if err := f.Validate(); err == nil {
				t.Fatalf("Validate accepted flags 0x%04X; want error", c.flags)
			}
		})
	}
}

// TestValidate_AcceptsValidVoiceFrame proves the canonical sample
// passes DecodeStrict without error.
func TestValidate_AcceptsValidVoiceFrame(t *testing.T) {
	enc := encodeWithFixedCRC(t, sampleVoiceFrame())
	if _, err := DecodeStrict(enc); err != nil {
		t.Fatalf("DecodeStrict rejected a valid voice frame: %v", err)
	}
}

// TestValidate_AcceptsValidRelayedFrame proves a properly-formed
// relayed frame passes DecodeStrict.
func TestValidate_AcceptsValidRelayedFrame(t *testing.T) {
	relayed := sampleVoiceFrame()
	relayed.Flags = FlagRelayed
	relayed.RelayID = 2
	relayed.Hop = 1
	relayed.TTL = 0
	enc := encodeWithFixedCRC(t, relayed)
	if _, err := DecodeStrict(enc); err != nil {
		t.Fatalf("DecodeStrict rejected a valid relayed frame: %v", err)
	}
}

// TestDecode_AcceptsCRCValidSemanticFieldInvalid proves that plain
// Decode accepts a CRC-valid frame whose semantic fields are invalid
// but whose binary structure and flag bitset are canonical. This
// confirms the two-layer separation:
// Decode = canonical binary integrity + flag-bitset legality;
// Validate = higher-level protocol semantics.
func TestDecode_AcceptsCRCValidSemanticFieldInvalid(t *testing.T) {
	enc := encodeWithFixedCRC(t, sampleVoiceFrame())
	bad := mutateFieldAndFixCRC(t, enc, func(h []byte) { h[1] = 0xFF })
	if _, err := Decode(bad); err != nil {
		t.Fatalf("Decode rejected a CRC-valid frame with canonical binary/flags but invalid semantic fields: %v", err)
	}
	if _, err := DecodeStrict(bad); err == nil {
		t.Fatal("DecodeStrict accepted a semantically invalid frame; want Validate error")
	}
}

// TestVerifyIntegrity_ValidFrame proves VerifyIntegrity passes on a
// freshly decoded frame whose CRCs are correct.
func TestVerifyIntegrity_ValidFrame(t *testing.T) {
	f := sampleVoiceFrame()
	enc, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := Decode(enc)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if err := decoded.VerifyIntegrity(); err != nil {
		t.Errorf("VerifyIntegrity on freshly decoded frame: %v", err)
	}
}

// TestVerifyIntegrity_MutatedPayload is review finding 1: after Decode,
// if a caller mutates the payload, VerifyIntegrity must detect the
// tampering (payload_crc32 no longer matches).
func TestVerifyIntegrity_MutatedPayload(t *testing.T) {
	f := sampleVoiceFrame()
	enc, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := Decode(enc)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	// Mutate payload after decode.
	decoded.Payload[0] ^= 0xFF
	if err := decoded.VerifyIntegrity(); err == nil {
		t.Fatal("VerifyIntegrity accepted a frame with mutated payload; expected CRC mismatch")
	}
}

// TestVerifyIntegrity_MutatedHeaderField proves VerifyIntegrity detects
// a mutated header field (e.g. TTL changed after decode).
func TestVerifyIntegrity_MutatedHeaderField(t *testing.T) {
	f := sampleVoiceFrame()
	enc, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := Decode(enc)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	// Mutate a header field after decode (don't update CRCs).
	decoded.TTL = 99
	if err := decoded.VerifyIntegrity(); err == nil {
		t.Fatal("VerifyIntegrity accepted a frame with mutated header field; expected CRC mismatch")
	}
}
