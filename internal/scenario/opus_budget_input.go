package scenario

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
)

const MaxOpusTraceBytes = 2 * 1024 * 1024

// OpusTrace is an offline export validated by libopus, not a Go codec implementation.
type OpusTrace struct {
	Version              int               `json:"version"`
	Codec                string            `json:"codec"`
	SourceManifestSHA256 string            `json:"source_manifest_sha256"`
	LibrarySHA256        string            `json:"library_sha256"`
	PacketMS             int64             `json:"packet_ms"`
	LookaheadUS          int64             `json:"lookahead_us"`
	License              string            `json:"license"`
	Attribution          string            `json:"attribution"`
	Samples              []OpusTraceSample `json:"samples"`
}

type OpusTraceSample struct {
	ID           string   `json:"id"`
	InputSamples int      `json:"input_samples"`
	PacketsHex   []string `json:"packets_hex"`
}

func DecodeOpusTrace(raw []byte) (OpusTrace, error) {
	var trace OpusTrace
	if len(raw) == 0 || len(raw) > MaxOpusTraceBytes {
		return trace, fmt.Errorf("opus trace: empty/oversize input")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&trace); err != nil {
		return trace, fmt.Errorf("opus trace: %w", err)
	}
	var extra json.RawMessage
	if d.Decode(&extra) != io.EOF {
		return trace, fmt.Errorf("opus trace: trailing JSON")
	}
	return trace, validateOpusTrace(trace)
}

func validateOpusTrace(t OpusTrace) error {
	hash := regexp.MustCompile(`^[0-9a-f]{64}$`)
	id := regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
	if t.Version != 1 || t.Codec != "opus-8000-nb-20ms-cvbr" || t.PacketMS != 20 || t.LookaheadUS != 6500 ||
		!hash.MatchString(t.SourceManifestSHA256) || !hash.MatchString(t.LibrarySHA256) ||
		t.License != "CC-BY-SA-4.0" || len(t.Attribution) == 0 || len(t.Attribution) > 16384 || len(t.Samples) == 0 || len(t.Samples) > 8 {
		return fmt.Errorf("opus trace: unsupported identity/provenance/settings")
	}
	seen := map[string]bool{}
	for _, s := range t.Samples {
		if !id.MatchString(s.ID) || seen[s.ID] || s.InputSamples < 8000 || s.InputSamples > 480000 ||
			len(s.PacketsHex) != (s.InputSamples+52+159)/160 || len(s.PacketsHex) > 3004 {
			return fmt.Errorf("opus trace: sample identity/count/duration")
		}
		seen[s.ID] = true
		for _, encoded := range s.PacketsHex {
			if len(encoded) < 2 || len(encoded) > 2550 {
				return fmt.Errorf("opus trace: packet size")
			}
			if _, err := hex.DecodeString(encoded); err != nil {
				return fmt.Errorf("opus trace: invalid hex")
			}
		}
	}
	return nil
}

func opusTraceHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// Bundle framing is a hypothetical uint16-length prefix per embedded packet.
// A single-packet case retains the original packet without an extra prefix.
func opusBundlePayloads(s OpusTraceSample, group int) ([]int, error) {
	if group != 1 && group != 2 && group != 4 && group != 8 {
		return nil, fmt.Errorf("opus budget: unsupported group count")
	}
	var sizes []int
	for i := 0; i < len(s.PacketsHex); i += group {
		size := 0
		for j := i; j < i+group && j < len(s.PacketsHex); j++ {
			size += len(s.PacketsHex[j]) / 2
			if group > 1 {
				size += 2
			}
		}
		sizes = append(sizes, size)
	}
	return sizes, nil
}
