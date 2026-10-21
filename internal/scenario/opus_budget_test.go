package scenario

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func opusTestTrace() OpusTrace {
	s := OpusTraceSample{ID: "test", InputSamples: 8000}
	for i := 0; i < 51; i++ {
		s.PacketsHex = append(s.PacketsHex, strings.Repeat("ab", 20))
	}
	return OpusTrace{Version: 1, Codec: "opus-8000-nb-20ms-cvbr", SourceManifestSHA256: strings.Repeat("a", 64),
		LibrarySHA256: strings.Repeat("b", 64), PacketMS: 20, LookaheadUS: 6500, License: "CC-BY-SA-4.0",
		Attribution: "synthetic unit fixture, not human speech", Samples: []OpusTraceSample{s}}
}

func TestOpusTraceValidation(t *testing.T) {
	base, _ := json.Marshal(opusTestTrace())
	for name, edit := range map[string]func(*OpusTrace){
		"codec":      func(v *OpusTrace) { v.Codec = "other" },
		"hash":       func(v *OpusTrace) { v.SourceManifestSHA256 = "" },
		"duplicates": func(v *OpusTrace) { v.Samples = append(v.Samples, v.Samples[0]) },
		"hex":        func(v *OpusTrace) { v.Samples[0].PacketsHex[0] = "xz" },
		"count":      func(v *OpusTrace) { v.Samples[0].PacketsHex = v.Samples[0].PacketsHex[:2] },
		"large":      func(v *OpusTrace) { v.Samples[0].PacketsHex[0] = strings.Repeat("00", 1276) },
		"duration":   func(v *OpusTrace) { v.Samples[0].InputSamples = 480001 },
		"delay":      func(v *OpusTrace) { v.LookaheadUS = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			var trace OpusTrace
			_ = json.Unmarshal(base, &trace)
			edit(&trace)
			raw, _ := json.Marshal(trace)
			if _, err := DecodeOpusTrace(raw); err == nil {
				t.Fatal("malformed trace accepted")
			}
		})
	}
	for _, raw := range [][]byte{nil, []byte(`{"unexpected":true}`), append(append([]byte(nil), base...), []byte(` {}`)...), bytes.Repeat([]byte(" "), MaxOpusTraceBytes+1)} {
		if _, err := DecodeOpusTrace(raw); err == nil {
			t.Fatal("bad JSON/bound accepted")
		}
	}
	if _, err := DecodeOpusTrace(base); err != nil {
		t.Fatal(err)
	}
}

func TestOpusGroupingPreservesLengthsAndPartialTail(t *testing.T) {
	s := OpusTraceSample{PacketsHex: []string{strings.Repeat("ab", 20), strings.Repeat("ab", 10), strings.Repeat("ab", 30), "ab", "abcd"}}
	got, err := opusBundlePayloads(s, 2)
	if err != nil || !reflect.DeepEqual(got, []int{34, 35, 4}) {
		t.Fatalf("grouped %v %v", got, err)
	}
	one, _ := opusBundlePayloads(s, 1)
	if !reflect.DeepEqual(one, []int{20, 10, 30, 1, 2}) {
		t.Fatal(one)
	}
	if _, err := opusBundlePayloads(s, 3); err == nil {
		t.Fatal("unsupported period accepted")
	}
	four, _ := opusBundlePayloads(s, 4)
	eight, _ := opusBundlePayloads(s, 8)
	if !reflect.DeepEqual(four, []int{69, 4}) || !reflect.DeepEqual(eight, []int{73}) {
		t.Fatalf("variable group4/8 accounting %v %v", four, eight)
	}
}

func TestOpusRealPeakNotNominalBitrate(t *testing.T) {
	v := opusTestTrace()
	v.Samples[0].PacketsHex[7] = strings.Repeat("ab", 29)
	r, err := runOpusBudgetCase(v, v.Samples[0], 1, 0, 1, 19200, 2000, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.Payload.MaxBytes != 29 || r.PeakBudget.WireBytes != 49 || r.PeakBudget.ProtectedBits != 636 ||
		r.PeakBudget.AirtimeUS != 33125 || r.ReservedCycleMS != 38 || r.Trial.Admission != "rejected" ||
		r.CaptureToPlayoutUS != nil || r.Trial.PlayedFrames != 0 || r.Trial.ExpectedFrames != 51 {
		t.Fatalf("bad peak envelope: %+v", r)
	}
}

func TestOpusMatrixDeterminismAndBoundaries(t *testing.T) {
	raw, _ := json.Marshal(opusTestTrace())
	a, err := RunOpusBudget(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Rows) != 289 || a.Verdict != "no_go_live_voice_hardware" {
		t.Fatal("wrong matrix boundary")
	}
	admitted, rejected, branch, l2 := 0, 0, 0, 0
	depthPass := map[uint8]bool{}
	for _, r := range a.Rows {
		if r.Opportunities != 1+int(r.Trial.Depth)*r.Trial.Width || r.Trial.PlayedFrames > r.Trial.ExpectedFrames ||
			r.Trial.ExpectedFrames != r.Payload.Count*(int(r.Trial.Depth)+1)*r.Trial.Width {
			t.Fatalf("denominator/opportunities: %+v", r)
		}
		if r.Trial.Admission == "rejected" {
			rejected++
			if r.Trial.FirstPlayoutFromPTTMS != nil || r.Trial.RuntimeControl != nil || r.CaptureToPlayoutUS != nil {
				t.Fatal("invented runtime on rejection")
			}
		} else {
			admitted++
			if r.ReservedCycleMS > r.Trial.CadenceMS || r.Trial.RuntimeControl == nil {
				t.Fatal("unaccounted voice/control")
			}
		}
		if r.Trial.ServiceOutcome == "pass" {
			depthPass[r.Trial.Depth] = true
			if r.Trial.PlayedFrames != r.Trial.ExpectedFrames || r.Trial.AllLocalCommitsMS == nil || r.CreationToPlayoutMS == nil ||
				*r.SerialPTTEstimateUS != *r.Trial.FirstPlayoutFromPTTMS*1000+r.CaptureAllowanceUS ||
				*r.CaptureToPlayoutUS != *r.CreationToPlayoutMS*1000+r.CaptureAllowanceUS {
				t.Fatal("false service/latency pass")
			}
			if r.AllMemberPlayout == nil || r.AllMemberPlayout.SampleCount != r.Trial.PlayedFrames || r.CapturePlusP95US == nil || !r.LatencyComplete {
				t.Fatal("missing all-member latency denominator")
			}
		}
		if r.Trial.Width > 1 {
			branch++
			if r.Opportunities <= int(r.Trial.Depth)+1 {
				t.Fatal("free branching")
			}
		}
		if r.Trial.Case == "l2_27byte_limit" {
			l2++
			if r.Trial.Admission != "rejected" || r.PeakBudget.FitsBackend {
				t.Fatal("27-byte backend incorrectly admitted")
			}
		}
	}
	if admitted == 0 || rejected == 0 || branch == 0 || l2 != 1 || len(depthPass) != 4 {
		t.Fatalf("missing positive/negative cases %d %d %d %d %v", admitted, rejected, branch, l2, depthPass)
	}
	b, err := RunOpusBudget(raw)
	if err != nil {
		t.Fatal(err)
	}
	aa, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	if !bytes.Equal(aa, bb) {
		t.Fatal("non-deterministic experiment")
	}
}

func TestOpusPinnedTraceControlAndCapacity(t *testing.T) {
	raw, err := os.ReadFile("../../configs/audio/opus-transport-trace-20260906.json")
	if err != nil {
		t.Fatal(err)
	}
	if opusTraceHash(raw) != "60735abd6d7771732b861f3ebe9985b6b49b6545ed521e077c219cdfa4cc223b" {
		t.Fatal("pinned packet trace bytes changed")
	}
	trace, err := DecodeOpusTrace(raw)
	if err != nil || trace.SourceManifestSHA256 != "38d90039f01044dbfb9319424f36dac4646540730fe3a2fc8e0722d8215e9b65" {
		t.Fatalf("pinned source changed: %v", err)
	}
	s := trace.Samples[1]
	if s.ID != "ru02" {
		t.Fatal("fixture order changed")
	}
	row, err := runOpusBudgetCase(trace, s, 4, 1, 1, 57600, 500, false)
	if err != nil || row.Trial.ServiceOutcome != "pass" || row.Payload.MaxBytes != 106 || row.ReservedCycleMS != 64 ||
		row.Trial.SetupControl.Bytes != 715 || *row.Trial.AllLocalCommitsMS != 184 || *row.Trial.FirstPlayoutFromPTTMS != 660 ||
		*row.CapturePlusP95US != 246500 {
		t.Fatalf("one-relay control/capacity: %+v %v", row, err)
	}
	for _, rate := range []int64{4800, 19200} {
		for _, group := range []int{1, 2, 4, 8} {
			r, err := runOpusBudgetCase(trace, s, group, 1, 1, rate, 2000, false)
			if err != nil || r.Trial.Admission != "rejected" {
				t.Fatal("current-rate relay unexpectedly fits", err)
			}
		}
	}
	for _, s := range trace.Samples {
		r, err := runOpusBudgetCase(trace, s, 4, 3, 1, 115200, 2000, false)
		if err != nil || r.Trial.ServiceOutcome != "pass" || !r.LatencyComplete {
			t.Fatal("higher-rate three-relay control", err)
		}
	}
}
