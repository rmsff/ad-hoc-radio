package scenario

import (
	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"testing"
)

func TestScheduledCollisionsChangeDelivery(t *testing.T) {
	for _, external := range []bool{false, true} {
		v := scheduledTestVersion(t, []routing.RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2}, {ParentStationID: 2, ChildStationID: 3, Hop: 1}}, []uint16{3}, nil, 1, 1, 0, 9)
		cfg := scheduledTestRuntimeConfig(v, 10)
		if external {
			cfg.Interference = []ScheduledInterferenceWindow{{0, 100000}}
		} else {
			cfg.Source.framePeriodMs = 10
			cfg.AllowCycleTimingStress = true
		}
		result, err := RunScheduledRuntime(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if result.Counters.CollisionDropCount == 0 || len(result.PlayoutEvents) == 10 {
			t.Fatalf("collision ignored external=%t: %+v", external, result.Counters)
		}
	}
}
