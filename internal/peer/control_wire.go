package peer

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
)

type WireFormat string

const (
	CompactV1              WireFormat = "compact_v1"
	JSONCRCv0              WireFormat = "json_crc_v0"
	MaxCompactControlBytes            = 2048
	CompactReferenceBytes             = 64
)

func (f WireFormat) Valid() bool { return f == "" || f == CompactV1 || f == JSONCRCv0 }

func encodeControl(m message, format WireFormat) ([]byte, error) {
	if format == JSONCRCv0 {
		raw, err := json.Marshal(m)
		if err != nil || len(raw) > 65531 {
			return nil, fmt.Errorf("peer: JSON control size/encoding")
		}
		return sealControl(raw), nil
	}
	if !format.Valid() {
		return nil, fmt.Errorf("peer: unsupported wire format")
	}
	code := kindCode(m.Kind)
	digest, err := hex.DecodeString(m.Permit.Digest)
	if err != nil || len(digest) != 32 || code == 0 || !validReference(m) {
		return nil, fmt.Errorf("peer: invalid compact reference")
	}
	w := wireWriter{raw: []byte{'A', 'C', 1, code}}
	w.u16(m.From)
	w.u16(m.To)
	w.u64(m.Nonce)
	w.raw = append(w.raw, digest...)
	w.u16(m.Permit.Schedule.Binding.ScheduleVersion)
	w.u64(uint64(m.Permit.EpochMS))
	w.u16(uint16(m.Permit.PeriodMS))
	if m.Kind == Proposal {
		p, err := makePermit(m.Permit.Schedule, m.Permit.EpochMS, m.Permit.PeriodMS)
		if err != nil || p.Digest != m.Permit.Digest || p.ExpiresMS != m.Permit.ExpiresMS || m.From != p.Schedule.Binding.SourceStationID {
			return nil, fmt.Errorf("peer: invalid compact proposal")
		}
		w.schedule(p.Schedule)
	}
	if len(w.raw)+4 > MaxCompactControlBytes {
		return nil, fmt.Errorf("peer: compact control size")
	}
	return sealControl(w.raw), nil
}

func decodeControl(raw []byte, format WireFormat) (message, error) {
	max := MaxCompactControlBytes
	if format == JSONCRCv0 {
		max = 65535
	}
	if !format.Valid() || len(raw) < 4 || len(raw) > max || binary.BigEndian.Uint32(raw[len(raw)-4:]) != crc32.ChecksumIEEE(raw[:len(raw)-4]) {
		return message{}, fmt.Errorf("peer: control CRC/size/format")
	}
	if format == JSONCRCv0 {
		dec := json.NewDecoder(bytes.NewReader(raw[:len(raw)-4]))
		dec.DisallowUnknownFields()
		var m message
		if err := dec.Decode(&m); err != nil {
			return m, err
		}
		if err := dec.Decode(&struct{}{}); err != io.EOF {
			return m, fmt.Errorf("peer: trailing control data")
		}
		return m, nil
	}
	if len(raw) < CompactReferenceBytes || !bytes.Equal(raw[:3], []byte{'A', 'C', 1}) {
		return message{}, fmt.Errorf("peer: compact header")
	}
	r := wireReader{raw: raw[4 : len(raw)-4]}
	m := message{Kind: codeKind(raw[3]), From: r.u16(), To: r.u16(), Nonce: r.u64()}
	m.Permit.Digest = hex.EncodeToString(r.take(32))
	m.Permit.Schedule.Binding.ScheduleVersion = r.u16()
	m.Permit.EpochMS = int64(r.u64())
	m.Permit.PeriodMS = int64(r.u16())
	if m.Kind == "" || !validReference(m) {
		return message{}, fmt.Errorf("peer: invalid compact reference")
	}
	if m.Kind == Proposal {
		m.Permit.Schedule = r.schedule(m.Permit.Schedule.Binding.ScheduleVersion)
		if r.err != nil {
			return message{}, r.err
		}
		p, err := makePermit(m.Permit.Schedule, m.Permit.EpochMS, m.Permit.PeriodMS)
		if err != nil || p.Digest != m.Permit.Digest || m.From != p.Schedule.Binding.SourceStationID {
			return message{}, fmt.Errorf("peer: compact schedule mismatch")
		}
		m.Permit = p
	}
	if r.err != nil || len(r.raw) != 0 {
		return message{}, fmt.Errorf("peer: truncated/trailing compact control")
	}
	return m, nil
}

func validReference(m message) bool {
	return m.From != 0 && m.To != 0 && m.From != m.To && m.Nonce > 0 &&
		m.Permit.Schedule.Binding.ScheduleVersion != 0 && validTime(m.Permit.EpochMS) && m.Permit.PeriodMS > 0 && m.Permit.PeriodMS <= 10000
}
func sealControl(raw []byte) []byte {
	return binary.BigEndian.AppendUint32(raw, crc32.ChecksumIEEE(raw))
}
func kindCode(k Kind) byte {
	switch k {
	case Proposal:
		return 1
	case ACK:
		return 2
	case Commit:
		return 3
	case End:
		return 4
	case Sync:
		return 5
	}
	return 0
}
func codeKind(k byte) Kind {
	switch k {
	case 1:
		return Proposal
	case 2:
		return ACK
	case 3:
		return Commit
	case 4:
		return End
	case 5:
		return Sync
	}
	return ""
}

// Resolve only from state received by this destination, never from controller
// transactions or a simulator transport event's attached source permit.
func resolveReference(l *localState, m message) (message, bool) {
	if l == nil {
		return m, false
	}
	p := l.Pending[m.Permit.Digest]
	if p == nil && l.Active != nil && l.Active.Digest == m.Permit.Digest {
		p = l.Active
	}
	if p == nil || !samePermitReference(*p, m.Permit) {
		return m, false
	}
	if m.Kind == ACK {
		if m.To != p.Schedule.Binding.SourceStationID {
			return m, false
		}
	} else if m.From != p.Schedule.Binding.SourceStationID {
		return m, false
	}
	m.Permit = *p
	return m, true
}
