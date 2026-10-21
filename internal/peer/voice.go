package peer

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

// VoiceBindingOverhead is a simulator envelope, not an accepted RF format.
const VoiceBindingOverhead = 6

func BindVoice(version uint16, raw []byte) ([]byte, error) {
	if version == 0 || len(raw) > 65535-VoiceBindingOverhead {
		return nil, fmt.Errorf("peer: invalid voice binding")
	}
	out := binary.BigEndian.AppendUint16(nil, version)
	out = append(out, raw...)
	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(out)), nil
}
func UnbindVoice(raw []byte) (uint16, []byte, error) {
	if len(raw) < VoiceBindingOverhead || len(raw) > 65535 || crc32.ChecksumIEEE(raw[:len(raw)-4]) != binary.BigEndian.Uint32(raw[len(raw)-4:]) {
		return 0, nil, fmt.Errorf("peer: voice binding CRC/size")
	}
	version := binary.BigEndian.Uint16(raw)
	if version == 0 {
		return 0, nil, fmt.Errorf("peer: zero voice version")
	}
	return version, raw[2 : len(raw)-4], nil
}
