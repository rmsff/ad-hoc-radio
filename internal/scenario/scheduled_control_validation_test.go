package scenario

import (
	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/transport"
	"math"
	"testing"
)

func TestScheduledTransportEvidenceValidation(t *testing.T) {
	for _, edit := range []func(*ScheduledRuntimeConfig){
		func(c *ScheduledRuntimeConfig) {
			c.Interference = []ScheduledInterferenceWindow{{StartMS: -1, EndMS: 1}}
		},
		func(c *ScheduledRuntimeConfig) {
			c.Interference = []ScheduledInterferenceWindow{{StartMS: 5, EndMS: 5}}
		},
		func(c *ScheduledRuntimeConfig) {
			c.Versions[0].Links[ScheduledLinkKey{1, 2}] = transport.Link{From: 2, To: 1, Enabled: true, FixedDelayMs: 1}
		},
		func(c *ScheduledRuntimeConfig) {
			c.Versions[0].Links[ScheduledLinkKey{1, 2}] = transport.Link{From: 1, To: 2, Enabled: true, FixedDelayMs: 2}
		},
		func(c *ScheduledRuntimeConfig) {
			c.Versions[0].Links[ScheduledLinkKey{1, 2}] = transport.Link{From: 1, To: 2, Enabled: false, FixedDelayMs: 1, LossPct: math.NaN()}
		},
		func(c *ScheduledRuntimeConfig) {
			c.Versions[0].Links[ScheduledLinkKey{1, 2}] = transport.Link{From: 1, To: 2, Enabled: true, FixedDelayMs: 1, JitterMs: math.MaxInt64}
		},
	} {
		v := scheduledTestVersion(t, []routing.RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2}}, []uint16{2}, nil, 0, 1, 0, 2)
		v.Links = map[ScheduledLinkKey]transport.Link{}
		cfg := scheduledTestRuntimeConfig(v, 3)
		edit(&cfg)
		if _, err := RunScheduledRuntime(cfg); err == nil {
			t.Fatal("invalid transport/interference accepted")
		}
	}
}

func TestScheduledSampledDelaySumIncludesJitterAndDuplicates(t *testing.T) {
	v := scheduledTestVersion(t, []routing.RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2}}, []uint16{2}, nil, 0, 1, 0, 2)
	key := ScheduledLinkKey{1, 2}
	link := transport.Link{From: 1, To: 2, Enabled: true, FixedDelayMs: 1, JitterMs: 7, DuplicatePct: 100}
	v.Links = map[ScheduledLinkKey]transport.Link{key: link}
	cfg := scheduledTestRuntimeConfig(v, 3)
	normalized, err := normalizeScheduledRuntimeConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	r := newScheduledRuntime(normalized)
	if err := r.run(); err != nil {
		t.Fatal(err)
	}
	want := int64(0)
	for seq := uint32(0); seq < 3; seq++ {
		frame := cfg.Source.EmitVoiceFrame(seq)
		raw, err := frame.Encode()
		if err != nil {
			t.Fatal(err)
		}
		serialized := frame.CreatedAtMs + 10
		ds, err := transport.SamplePacket(link, raw, serialized, scheduledPacketSeed(cfg.Seed, frame.CallID, seq, 1, 1, 2))
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range ds {
			want += d.ReceivedAt - serialized
		}
	}
	if r.linkDelaySumMS[key] != want || r.linkDeliveries[key] != 6 {
		t.Fatalf("sum=%d want=%d deliveries=%d", r.linkDelaySumMS[key], want, r.linkDeliveries[key])
	}
}
