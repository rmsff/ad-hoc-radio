package protocol

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"math"
)

// Protocol version (frame-contract.md §3, MVP value: 1).
const Version uint8 = 1

// FrameType enum values per frame-contract.md §2.
const (
	FrameTypeVoice      uint8 = 0x01
	FrameTypeCallStart  uint8 = 0x02
	FrameTypeCallEnd    uint8 = 0x03
	FrameTypeBeacon     uint8 = 0x04
	FrameTypeRelayOffer uint8 = 0x05
)

// Flag bitset constants per frame-contract.md §5. flags is a uint16;
// undefined bits must be zero in MVP.
const (
	FlagDirect        uint16 = 0x0001
	FlagRelayed       uint16 = 0x0002
	FlagCallStart     uint16 = 0x0004
	FlagCallEnd       uint16 = 0x0008
	FlagLateAllowed   uint16 = 0x0010
	FlagTestCorrupted uint16 = 0x0020
)

// headerSize is the fixed canonical header length in bytes per
// frame-contract.md §4.2. Payload is appended after the header and is
// not part of the header CRC.
const headerSize = 48

// HeaderSize is the exported canonical header length, used by the
// transport layer to locate payload bytes when applying link
// corruption (Story 1.3).
const HeaderSize = headerSize

// definedFlagMask is the bitmask of all flag bits defined for MVP per
// frame-contract.md §5. Bits outside this mask must be zero in MVP.
const definedFlagMask uint16 = FlagDirect | FlagRelayed | FlagCallStart | FlagCallEnd | FlagLateAllowed | FlagTestCorrupted

// Frame is the canonical simulator-level frame envelope per
// frame-contract.md §3. All fields are required.
//
// Field widths and serialization order follow §4.2 exactly. Integer
// fields are big-endian; created_at_ms and sent_at_ms are two's
// complement big-endian int64.
type Frame struct {
	Version      uint8
	FrameType    uint8
	NetID        uint16
	SourceID     uint16
	GroupID      uint16
	RelayID      uint16
	CallID       uint32
	Seq          uint32
	Hop          uint8
	TTL          uint8
	Flags        uint16
	CreatedAtMs  int64
	SentAtMs     int64
	PayloadLen   uint16
	Payload      []byte
	HeaderCRC32  uint32
	PayloadCRC32 uint32
}

// Encode serializes f into the canonical binary layout per
// frame-contract.md §4.2 and computes both CRCs per §6.
//
// The encoding is deterministic: given equal field values and payload,
// the output bytes are identical (NFR-1).
func (f *Frame) Encode() ([]byte, error) {
	if f == nil {
		return nil, fmt.Errorf("protocol: encode nil frame")
	}
	// Guard against uint16 overflow: a payload longer than MaxUint16
	// cannot be represented in the uint16 payload_len field and would
	// wrap silently if cast first (frame-contract.md §3).
	if len(f.Payload) > math.MaxUint16 {
		return nil, fmt.Errorf("protocol: payload too large: %d bytes exceeds uint16 max %d", len(f.Payload), math.MaxUint16)
	}
	if f.PayloadLen != uint16(len(f.Payload)) {
		return nil, fmt.Errorf("protocol: payload_len %d does not match len(payload) %d", f.PayloadLen, len(f.Payload))
	}

	payloadCRC := crc32.ChecksumIEEE(f.Payload)

	header := make([]byte, headerSize)
	// Fields per §4.2 order, with header_crc32 = 0 during calculation.
	header[0] = f.Version
	header[1] = f.FrameType
	binary.BigEndian.PutUint16(header[2:4], f.NetID)
	binary.BigEndian.PutUint16(header[4:6], f.SourceID)
	binary.BigEndian.PutUint16(header[6:8], f.GroupID)
	binary.BigEndian.PutUint16(header[8:10], f.RelayID)
	binary.BigEndian.PutUint32(header[10:14], f.CallID)
	binary.BigEndian.PutUint32(header[14:18], f.Seq)
	header[18] = f.Hop
	header[19] = f.TTL
	binary.BigEndian.PutUint16(header[20:22], f.Flags)
	binary.BigEndian.PutUint64(header[22:30], uint64(f.CreatedAtMs))
	binary.BigEndian.PutUint64(header[30:38], uint64(f.SentAtMs))
	binary.BigEndian.PutUint16(header[38:40], f.PayloadLen)
	// header[40:44] = header_crc32 = 0 (left zeroed for CRC calc)
	binary.BigEndian.PutUint32(header[44:48], payloadCRC)

	headerCRC := crc32.ChecksumIEEE(header)

	// Write the real header_crc32 in place.
	binary.BigEndian.PutUint32(header[40:44], headerCRC)

	out := make([]byte, 0, headerSize+len(f.Payload))
	out = append(out, header...)
	out = append(out, f.Payload...)
	return out, nil
}

// Decode parses the canonical binary layout per frame-contract.md §4.2
// and validates both CRCs per §6. On CRC mismatch or malformed input
// it returns a non-nil error; the frame must not be accepted for relay
// or receiver processing in that case (AC-2).
func Decode(data []byte) (Frame, error) {
	if len(data) < headerSize {
		return Frame{}, fmt.Errorf("protocol: data too short: got %d bytes, need >= %d", len(data), headerSize)
	}
	header := data[:headerSize]

	// Reconstruct header with header_crc32 = 0 to recompute the check.
	calc := make([]byte, headerSize)
	copy(calc, header)
	binary.BigEndian.PutUint32(calc[40:44], 0)

	wantHeaderCRC := crc32.ChecksumIEEE(calc)
	gotHeaderCRC := binary.BigEndian.Uint32(header[40:44])
	if wantHeaderCRC != gotHeaderCRC {
		return Frame{}, fmt.Errorf("protocol: header_crc32 mismatch: got 0x%08X, want 0x%08X", gotHeaderCRC, wantHeaderCRC)
	}

	payloadLen := binary.BigEndian.Uint16(header[38:40])
	// Canonical single-frame decode requires exact length: header +
	// declared payload, no more (frame-contract.md §4.2). Trailing
	// bytes would silently pass CRC over the declared payload and
	// produce a non-canonical accepted frame.
	if len(data) != headerSize+int(payloadLen) {
		return Frame{}, fmt.Errorf("protocol: data length %d does not equal header %d + payload_len %d = %d", len(data), headerSize, payloadLen, headerSize+int(payloadLen))
	}
	payload := data[headerSize : headerSize+int(payloadLen)]

	wantPayloadCRC := crc32.ChecksumIEEE(payload)
	gotPayloadCRC := binary.BigEndian.Uint32(header[44:48])
	if wantPayloadCRC != gotPayloadCRC {
		return Frame{}, fmt.Errorf("protocol: payload_crc32 mismatch: got 0x%08X, want 0x%08X", gotPayloadCRC, wantPayloadCRC)
	}

	// MVP flag validation per frame-contract.md §5: undefined bits must
	// be zero, and direct and relayed must not both be set.
	flags := binary.BigEndian.Uint16(header[20:22])
	if err := validateFlags(flags); err != nil {
		return Frame{}, fmt.Errorf("protocol: invalid flags 0x%04X: %w", flags, err)
	}

	// Defensive copy so callers cannot mutate the input slice and alter
	// the decoded frame's payload.
	payloadCopy := make([]byte, len(payload))
	copy(payloadCopy, payload)

	return Frame{
		Version:      header[0],
		FrameType:    header[1],
		NetID:        binary.BigEndian.Uint16(header[2:4]),
		SourceID:     binary.BigEndian.Uint16(header[4:6]),
		GroupID:      binary.BigEndian.Uint16(header[6:8]),
		RelayID:      binary.BigEndian.Uint16(header[8:10]),
		CallID:       binary.BigEndian.Uint32(header[10:14]),
		Seq:          binary.BigEndian.Uint32(header[14:18]),
		Hop:          header[18],
		TTL:          header[19],
		Flags:        flags,
		CreatedAtMs:  int64(binary.BigEndian.Uint64(header[22:30])),
		SentAtMs:     int64(binary.BigEndian.Uint64(header[30:38])),
		PayloadLen:   payloadLen,
		Payload:      payloadCopy,
		HeaderCRC32:  gotHeaderCRC,
		PayloadCRC32: gotPayloadCRC,
	}, nil
}

// validateFlags enforces the MVP flag bitset rules per
// frame-contract.md §5: undefined bits must be zero, and direct and
// relayed must not both be set.
func validateFlags(flags uint16) error {
	if flags&^definedFlagMask != 0 {
		return fmt.Errorf("undefined flag bits set: 0x%04X", flags&^definedFlagMask)
	}
	if flags&FlagDirect != 0 && flags&FlagRelayed != 0 {
		return fmt.Errorf("direct and relayed flags must not both be set in MVP")
	}
	return nil
}

// knownFrameTypes is the set of FrameType values defined in
// frame-contract.md §2. Any other value is unsupported in MVP.
var knownFrameTypes = map[uint8]bool{
	FrameTypeVoice:      true,
	FrameTypeCallStart:  true,
	FrameTypeCallEnd:    true,
	FrameTypeBeacon:     true,
	FrameTypeRelayOffer: true,
}

// Validate checks the semantic invariants of f per frame-contract.md.
// It does NOT re-check binary CRC (that is enforced by Decode); it
// checks field-level and flag-bitset rules that a CRC-valid frame could
// still violate:
//
//   - Version == 1 (§3, MVP value).
//   - FrameType is one of the known enum values (§2).
//   - GroupID != 0 (§18: group_id = 0 is reserved and invalid for MVP).
//   - SourceID != 0 (§4.3: numeric_id = 0 is reserved for relay_id=0
//     direct/source frames and must not be assigned to a Station).
//   - Flags contain only defined bits and do not set direct+relayed
//     simultaneously (§5).
//   - Direct/relayed consistency with relay_id and hop (§5, §10):
//     a direct frame has relay_id=0 and hop=0; a relayed frame has
//     relay_id!=0 and hop>0.
//   - PayloadLen equals len(Payload), so manually constructed Frames
//     cannot bypass the binary encode/decode length invariant.
//
// Validate is safe to call on a Frame returned by Decode and on a Frame
// value built in code. Callers that want binary + semantic checks in one
// step should use DecodeStrict.
func (f Frame) Validate() error {
	if f.Version != Version {
		return fmt.Errorf("protocol: version %d is not the MVP version %d", f.Version, Version)
	}
	if !knownFrameTypes[f.FrameType] {
		return fmt.Errorf("protocol: frame_type 0x%02X is not a known MVP frame type", f.FrameType)
	}
	if f.GroupID == 0 {
		return fmt.Errorf("protocol: group_id 0 is reserved and invalid for MVP (frame-contract.md §18)")
	}
	if f.SourceID == 0 {
		return fmt.Errorf("protocol: source_id 0 is reserved and must not be assigned to a station (§4.3)")
	}
	if len(f.Payload) > math.MaxUint16 {
		return fmt.Errorf("protocol: payload too large: %d bytes exceeds uint16 max %d", len(f.Payload), math.MaxUint16)
	}
	if f.PayloadLen != uint16(len(f.Payload)) {
		return fmt.Errorf("protocol: payload_len %d does not match len(payload) %d", f.PayloadLen, len(f.Payload))
	}
	if err := validateFlags(f.Flags); err != nil {
		return fmt.Errorf("protocol: invalid flags 0x%04X: %w", f.Flags, err)
	}
	if err := f.validateDirectRelayedConsistency(); err != nil {
		return err
	}
	return nil
}

// validateDirectRelayedConsistency enforces the §5/§10 relationship
// between the direct/relayed flags and the relay_id/hop fields.
func (f Frame) validateDirectRelayedConsistency() error {
	direct := f.Flags&FlagDirect != 0
	relayed := f.Flags&FlagRelayed != 0
	switch {
	case direct && !relayed:
		if f.RelayID != 0 {
			return fmt.Errorf("protocol: direct frame has relay_id=%d, want 0 (§5)", f.RelayID)
		}
		if f.Hop != 0 {
			return fmt.Errorf("protocol: direct frame has hop=%d, want 0 (§10)", f.Hop)
		}
	case relayed && !direct:
		if f.RelayID == 0 {
			return fmt.Errorf("protocol: relayed frame has relay_id=0, want non-zero (§5)")
		}
		if f.Hop == 0 {
			return fmt.Errorf("protocol: relayed frame has hop=0, want >0 (§10)")
		}
	default:
		// Neither flag set, or both set. Both-set is already rejected
		// by validateFlags during Decode. Neither-set is permitted
		// here: control frames (call_start/call_end) may legitimately
		// carry neither direct nor relayed in some scenarios, and the
		// caller can apply stricter per-frame-type policy.
	}
	return nil
}

// DecodeStrict parses the canonical binary layout per
// frame-contract.md §4.2, validates both CRCs per §6, AND validates
// semantic invariants via Frame.Validate. Use this when a caller
// needs a frame that is safe to accept for relay or receiver
// processing. Use Decode when only canonical binary/CRC/flag-bitset
// correctness is needed (e.g. debug introspection of a semantically
// invalid frame).
func DecodeStrict(data []byte) (Frame, error) {
	f, err := Decode(data)
	if err != nil {
		return f, err
	}
	if err := f.Validate(); err != nil {
		return Frame{}, err
	}
	return f, nil
}

// VerifyIntegrity re-checks that f's HeaderCRC32 and PayloadCRC32 match
// the current field values and payload bytes, per frame-contract.md §6.
// This is a CRC re-validation for a Frame value that was previously
// Decode'd but may have been mutated in memory (e.g. by a test or a
// buggy caller). Decode already verifies CRC at parse time; Validate
// does NOT re-check CRC (it only checks semantic invariants). Callers
// that receive a Frame from an untrusted source (not directly from
// Decode) should call VerifyIntegrity to ensure the payload was not
// tampered with after decoding.
//
// Returns nil if both CRCs match, an error otherwise. The error is
// not the same instance Decode would produce, but describes the
// mismatch in the same format.
func (f Frame) VerifyIntegrity() error {
	// Recompute payload CRC.
	wantPayload := crc32.ChecksumIEEE(f.Payload)
	if wantPayload != f.PayloadCRC32 {
		return fmt.Errorf("protocol: integrity check failed: payload_crc32 mismatch: stored 0x%08X, recomputed 0x%08X", f.PayloadCRC32, wantPayload)
	}

	// Recompute header CRC with header_crc32=0 and the real
	// payload_crc32 in place, per §6.
	header := make([]byte, headerSize)
	header[0] = f.Version
	header[1] = f.FrameType
	binary.BigEndian.PutUint16(header[2:4], f.NetID)
	binary.BigEndian.PutUint16(header[4:6], f.SourceID)
	binary.BigEndian.PutUint16(header[6:8], f.GroupID)
	binary.BigEndian.PutUint16(header[8:10], f.RelayID)
	binary.BigEndian.PutUint32(header[10:14], f.CallID)
	binary.BigEndian.PutUint32(header[14:18], f.Seq)
	header[18] = f.Hop
	header[19] = f.TTL
	binary.BigEndian.PutUint16(header[20:22], f.Flags)
	binary.BigEndian.PutUint64(header[22:30], uint64(f.CreatedAtMs))
	binary.BigEndian.PutUint64(header[30:38], uint64(f.SentAtMs))
	binary.BigEndian.PutUint16(header[38:40], f.PayloadLen)
	// header[40:44] = 0 (header_crc32 zeroed for computation)
	binary.BigEndian.PutUint32(header[44:48], f.PayloadCRC32)

	wantHeader := crc32.ChecksumIEEE(header)
	if wantHeader != f.HeaderCRC32 {
		return fmt.Errorf("protocol: integrity check failed: header_crc32 mismatch: stored 0x%08X, recomputed 0x%08X", f.HeaderCRC32, wantHeader)
	}
	return nil
}
