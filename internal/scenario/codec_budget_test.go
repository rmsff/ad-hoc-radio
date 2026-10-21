package scenario

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func codecTestTrace() CodecTrace {
	s := OpusTraceSample{ID: "unit", InputSamples: 8000}
	for i := 0; i < 50; i++ {
		s.PacketsHex = append(s.PacketsHex, "010203040506")
	}
	return CodecTrace{Version: 1, Codec: "codec2-2400", PacketMS: 20, SourceManifestSHA256: strings.Repeat("a", 64), LibrarySHA256: strings.Repeat("b", 64), License: "CC-BY-SA-4.0", Attribution: "Synthetic test fixture, not human speech", Samples: []OpusTraceSample{s}}
}

func TestCodecTraceValidation(t *testing.T) {
	for _, mutate := range []func(*CodecTrace){
		func(c *CodecTrace) { c.Codec = "invented" }, func(c *CodecTrace) { c.PacketMS = 80 },
		func(c *CodecTrace) { c.Samples[0].PacketsHex[0] = "zz" }, func(c *CodecTrace) { c.Samples[0].PacketsHex[0] = "00" },
		func(c *CodecTrace) { c.Samples = append(c.Samples, c.Samples[0]) }, func(c *CodecTrace) { c.FlushSamples8K = 52 },
		func(c *CodecTrace) { c.Samples[0].PacketsHex = c.Samples[0].PacketsHex[:49] },
		func(c *CodecTrace) { c.LibrarySHA256 = "missing" }, func(c *CodecTrace) { v := int64(-1); c.LookaheadUS = &v },
		func(c *CodecTrace) { v := int64(0); c.LookaheadUS = &v },
	} {
		c := codecTestTrace()
		mutate(&c)
		b, _ := json.Marshal(c)
		if _, err := DecodeCodecTrace(b); err == nil {
			t.Fatal("accepted bad trace")
		}
	}
	b, _ := json.Marshal(codecTestTrace())
	if _, err := DecodeCodecTrace(b); err != nil {
		t.Fatal(err)
	}
	for _, raw := range [][]byte{nil, append(b, []byte(" {}")...), bytes.Repeat([]byte("x"), MaxOpusTraceBytes+1), []byte(strings.Replace(string(b), `"version":1`, `"unknown":1,"version":1`, 1))} {
		if _, err := DecodeCodecTrace(raw); err == nil {
			t.Fatal("accepted malformed trace")
		}
	}
}

func TestCodecBudgetDeterminismAndUnknownDelay(t *testing.T) {
	b, _ := json.Marshal(codecTestTrace())
	a, err := RunCodecBudget(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Rows) != 144 {
		t.Fatal(len(a.Rows))
	}
	pass, reject, branch := 0, 0, 0
	for _, row := range a.Rows {
		e := row.Envelope
		if row.KnownLookaheadUS != nil || row.CaptureAndCodecUS != nil || e.CapturePlusP95US != nil || e.CaptureToPlayoutUS != nil || e.SerialPTTEstimateUS != nil {
			t.Fatal("invented unknown codec delay")
		}
		if e.Trial.ServiceOutcome == "pass" {
			pass++
			if !e.LatencyComplete {
				t.Fatal("incomplete pass")
			}
		}
		if e.Trial.Admission == "rejected" {
			reject++
		}
		if e.Opportunities == 5 {
			branch++
		}
		if e.GroupCount == 4 && e.Payload.MaxBytes != 32 {
			t.Fatal("group lengths not charged", e.Payload)
		}
	}
	if pass == 0 || reject == 0 || branch == 0 {
		t.Fatal("matrix coverage")
	}
	c, err := RunCodecBudget(b)
	if err != nil {
		t.Fatal(err)
	}
	aa, _ := json.Marshal(a)
	cc, _ := json.Marshal(c)
	if !bytes.Equal(aa, cc) {
		t.Fatal("non-deterministic report")
	}
}
