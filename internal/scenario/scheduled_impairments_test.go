package scenario

import (
	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/transport"
	"testing"
)

func TestScheduledSelectedLinkImpairmentsAreCausal(t *testing.T) {
	run := func(seed uint64, loss, corruption, duplicate float64) ScheduledRuntimeResult {
		v := scheduledTestVersion(t, []routing.RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2}}, []uint16{2}, nil, 0, 1, 0, 99)
		v.Links = map[ScheduledLinkKey]transport.Link{{1, 2}: {From: 1, To: 2, Enabled: true, LossPct: loss, CorruptionPct: corruption, DuplicatePct: duplicate, FixedDelayMs: 1}}
		cfg := scheduledTestRuntimeConfig(v, 100)
		cfg.Seed = seed
		result, err := RunScheduledRuntime(cfg)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	for _, mode := range []string{"loss", "corruption"} {
		t.Run(mode, func(t *testing.T) {
			loss, corruption := 0.0, 0.0
			if mode == "loss" {
				loss = 100
			} else {
				corruption = 100
			}
			result := run(42, loss, corruption, 0)
			if result.ReceiverCounters[2].FramesAccepted != 0 || len(result.PlayoutEvents) != 0 {
				t.Fatalf("%s bypassed: %+v", mode, result.Counters)
			}
		})
	}
	a, b, c := run(42, 50, 0, 0), run(42, 50, 0, 0), run(43, 50, 0, 0)
	if a.ResultHash != b.ResultHash || a.EventHash == c.EventHash {
		t.Fatal("seed not deterministic or ignored")
	}
	d := run(42, 0, 0, 100)
	if d.ReceiverCounters[2].DuplicatesSuppressed != 100 || len(d.PlayoutEvents) != 100 {
		t.Fatalf("duplicates: %+v", d.ReceiverCounters[2])
	}
}
