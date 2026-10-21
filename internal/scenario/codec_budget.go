package scenario

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
)

// CodecTrace contains actual exported packets, not a native codec implementation.
type CodecTrace struct {
	Version              int               `json:"version"`
	Codec                string            `json:"codec"`
	SourceManifestSHA256 string            `json:"source_manifest_sha256"`
	LibrarySHA256        string            `json:"library_sha256"`
	PacketMS             int64             `json:"packet_ms"`
	LookaheadUS          *int64            `json:"lookahead_us"`
	FlushSamples8K       int               `json:"flush_samples_8k"`
	License              string            `json:"license"`
	Attribution          string            `json:"attribution"`
	Samples              []OpusTraceSample `json:"samples"`
}

type CodecBudgetRow struct {
	Codec            string `json:"codec"`
	KnownLookaheadUS *int64 `json:"known_lookahead_us"`
	// Unknown codec delay must not silently become a zero-delay total estimate.
	CaptureAndCodecUS *int64        `json:"capture_and_known_codec_delay_us"`
	Envelope          OpusBudgetRow `json:"envelope"`
}

type CodecBudgetReport struct {
	Version       int              `json:"version"`
	InputSHA256   string           `json:"input_sha256"`
	Codec         string           `json:"codec"`
	EvidenceGrade string           `json:"evidence_grade"`
	Rows          []CodecBudgetRow `json:"rows"`
	Limits        []string         `json:"limits"`
	SHA256        string           `json:"sha256"`
}

func DecodeCodecTrace(raw []byte) (CodecTrace, error) {
	var t CodecTrace
	if len(raw) == 0 || len(raw) > MaxOpusTraceBytes {
		return t, fmt.Errorf("codec trace size")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&t); err != nil {
		return t, err
	}
	var extra json.RawMessage
	if d.Decode(&extra) != io.EOF {
		return t, fmt.Errorf("trailing codec trace JSON")
	}
	hash := regexp.MustCompile(`^[0-9a-f]{64}$`)
	known := map[string]int{"opus-6000": 0, "opus-8000": 0, "codec2-2400": 6, "codec2-3200": 8, "lyra-v2-3200": 8}
	fixed, ok := known[t.Codec]
	if !ok || t.Version != 1 || t.PacketMS != 20 || !hash.MatchString(t.SourceManifestSHA256) || !hash.MatchString(t.LibrarySHA256) ||
		t.License != "CC-BY-SA-4.0" || len(t.Attribution) == 0 || len(t.Attribution) > 16384 || len(t.Samples) < 1 || len(t.Samples) > 8 ||
		t.FlushSamples8K < 0 || t.FlushSamples8K > 8000 || (t.LookaheadUS != nil && (*t.LookaheadUS < 0 || *t.LookaheadUS > 1000000)) {
		return t, fmt.Errorf("unsupported codec trace metadata")
	}
	if fixed == 0 && (t.FlushSamples8K != 52 || t.LookaheadUS == nil || *t.LookaheadUS != 6500) {
		return t, fmt.Errorf("Opus delay contract")
	}
	if fixed != 0 && t.FlushSamples8K != 0 {
		return t, fmt.Errorf("fixed codec flush contract")
	}
	if (t.Codec == "codec2-2400" || t.Codec == "codec2-3200") && t.LookaheadUS != nil {
		return t, fmt.Errorf("Codec2 delay remains unmeasured in this experiment")
	}
	ids := map[string]bool{}
	id := regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
	for _, s := range t.Samples {
		if !id.MatchString(s.ID) || ids[s.ID] || s.InputSamples < 8000 || s.InputSamples > 480000 || len(s.PacketsHex) != (s.InputSamples+t.FlushSamples8K+159)/160 {
			return t, fmt.Errorf("codec trace sample count/identity")
		}
		ids[s.ID] = true
		for _, p := range s.PacketsHex {
			if len(p) < 2 || len(p) > 2550 || (fixed > 0 && len(p) != fixed*2) {
				return t, fmt.Errorf("codec packet size")
			}
			if _, err := hex.DecodeString(p); err != nil {
				return t, err
			}
		}
	}
	return t, nil
}

func RunCodecBudget(raw []byte) (CodecBudgetReport, error) {
	t, err := DecodeCodecTrace(raw)
	if err != nil {
		return CodecBudgetReport{}, err
	}
	r := CodecBudgetReport{Version: 1, Codec: t.Codec, InputSHA256: opusTraceHash(raw), EvidenceGrade: "real_packet_lengths_synthetic_size_envelope_not_audio_or_hardware",
		Limits: []string{"No live codec forwarding or hardware acceptance; same compact control, 2/3 external FEC, framing and guards as prior Opus experiment",
			"4800/19200 remain assumptions; 38400 is an unverified sensitivity hypothesis",
			"Observed maximum envelope, not a proven future encoder size cap; every selected branch transmitter is charged",
			"Two-byte lengths per grouped packet; loss playback and MCU timing are independent gates",
			"When codec delay is unknown, envelope capture allowance is batching-only lower bound; combined latency estimates are null",
			"Played latency excludes missing frames; check complete expected-member delivery; setup is not hidden in steady-state latency"}}
	look := int64(0)
	if t.LookaheadUS != nil {
		look = *t.LookaheadUS
	}
	adapter := OpusTrace{PacketMS: t.PacketMS, LookaheadUS: look}
	for _, s := range t.Samples {
		for _, group := range []int{1, 2, 4, 8} {
			for _, rate := range []int64{4800, 19200, 38400} {
				for _, shape := range [][2]int{{0, 1}, {1, 1}, {2, 1}, {3, 1}, {1, 2}, {1, 4}} {
					for _, setup := range []int64{500, 2000} {
						e, err := runPacketEnvelopeCase(adapter, s, group, uint8(shape[0]), shape[1], rate, setup, false, "codec-comparison-maximum-size-envelope-not-audio")
						if err != nil {
							return r, err
						}
						row := CodecBudgetRow{Codec: t.Codec, KnownLookaheadUS: t.LookaheadUS, Envelope: e}
						if t.LookaheadUS == nil {
							row.Envelope.CapturePlusP95US = nil
							row.Envelope.CaptureToPlayoutUS = nil
							row.Envelope.SerialPTTEstimateUS = nil
						} else {
							v := e.CaptureAllowanceUS
							row.CaptureAndCodecUS = &v
						}
						r.Rows = append(r.Rows, row)
					}
				}
			}
		}
	}
	encoded, err := json.Marshal(r)
	if err != nil {
		return r, err
	}
	r.SHA256 = opusTraceHash(encoded)
	return r, nil
}
