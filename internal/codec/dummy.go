package codec

import (
	"bytes"
	"encoding/binary"
	"errors"
)

const (
	dummyMagic      = "ADHR"
	dummyPayloadLen = 16
)

// Input is the deterministic dummy input needed to create one voice payload.
type Input struct {
	Seq         uint32
	CreatedAtMs int64
}

// Codec defines the narrow boundary between payload creation and frame logic.
type Codec interface {
	Encode(Input) ([]byte, error)
	Decode([]byte) (Input, error)
}

// DummyCodec provides deterministic payload bytes without real audio coding.
type DummyCodec struct{}

func (DummyCodec) Encode(input Input) ([]byte, error) {
	if input.CreatedAtMs < 0 {
		return nil, errors.New("codec: created_at_ms must be non-negative")
	}

	payload := make([]byte, dummyPayloadLen)
	copy(payload[0:4], dummyMagic)
	binary.BigEndian.PutUint32(payload[4:8], input.Seq)
	binary.BigEndian.PutUint64(payload[8:16], uint64(input.CreatedAtMs))
	return payload, nil
}

func (DummyCodec) Decode(payload []byte) (Input, error) {
	if len(payload) != dummyPayloadLen {
		return Input{}, errors.New("codec: invalid dummy payload length")
	}
	if !bytes.Equal(payload[0:4], []byte(dummyMagic)) {
		return Input{}, errors.New("codec: invalid dummy payload magic")
	}

	createdAtMs := int64(binary.BigEndian.Uint64(payload[8:16]))
	if createdAtMs < 0 {
		return Input{}, errors.New("codec: created_at_ms must be non-negative")
	}

	return Input{
		Seq:         binary.BigEndian.Uint32(payload[4:8]),
		CreatedAtMs: createdAtMs,
	}, nil
}

// Spy counts calls made through the codec boundary by source/test harness code.
type Spy struct {
	codec       Codec
	encodeCalls int
	decodeCalls int
}

func NewSpy(codec Codec) *Spy {
	return &Spy{codec: codec}
}

func (s *Spy) Encode(input Input) ([]byte, error) {
	s.encodeCalls++
	return s.codec.Encode(input)
}

func (s *Spy) Decode(payload []byte) (Input, error) {
	s.decodeCalls++
	return s.codec.Decode(payload)
}

func (s *Spy) EncodeCalls() int {
	return s.encodeCalls
}

func (s *Spy) DecodeCalls() int {
	return s.decodeCalls
}
