package scenario

import (
	"github.com/rmsff/ad-hoc-radio/internal/peer"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
	"testing"
)

func longControlConfig(t *testing.T) ScheduledRuntimeConfig {
	cfg := scheduledControlConfig(t)
	cfg.FrameCount = 128
	cfg.Source.frameCount = 128
	cfg.Versions[0].Schedule.Binding.ValidThroughSeq = 127
	cfg.Control.Physical.GrossBitrateBPS = 1000000000
	cfg.Control.RetryMS = 20
	cfg.Control.Attempts = 2
	cfg.Control.ReplanLeadMS = 300
	return cfg
}
func controlReplanInput(t *testing.T, at int64, epoch uint32) ScheduledReplanInput {
	t.Helper()
	input := scheduledTestReplanInput(ScheduledReplanTopologyEpochChanged, at, []uint16{2, 3}, [][2]uint16{{1, 2}, {2, 3}}, []uint16{2}, epoch, 9, 127, true)
	input.Binding.ScheduleAuthorityID = 1
	for i := range input.PlanRequest.Graph.Links {
		input.PlanRequest.Graph.Links[i].Loss = 0
	}
	return input
}

func TestControlExpiryDrainsAll128FramesAtPeriodicDeadlines(t *testing.T) {
	for _, lostEnd := range []bool{false, true} {
		cfg := longControlConfig(t)
		if lostEnd {
			cfg.Control.Faults = []peer.ControlFault{{Kind: peer.End, LossPct: 100}}
		}
		r, err := RunScheduledRuntime(cfg)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []uint16{2, 3} {
			if r.ReceiverCounters[id].PlayoutFramesEmitted != 128 {
				t.Fatalf("lost_end=%t member=%d played=%d", lostEnd, id, r.ReceiverCounters[id].PlayoutFramesEmitted)
			}
		}
		last := int64(0)
		for _, e := range r.PlayoutEvents {
			if e.AtMS != cfg.Source.startMs+int64(e.Seq)*80+160 {
				t.Fatalf("early/nonperiodic flush: %+v", e)
			}
			if e.AtMS > last {
				last = e.AtMS
			}
		}
		if last != cfg.Source.startMs+128*80+80 {
			t.Fatalf("tail deadline=%d", last)
		}
	}
}

func TestControlRuntimeDistributedReplan(t *testing.T) {
	for _, lost := range []bool{false, true} {
		cfg := longControlConfig(t)
		cfg.ReplanInputs = []ScheduledReplanInput{controlReplanInput(t, 20100, 8)}
		if lost {
			cfg.Control.Faults = []peer.ControlFault{{Kind: peer.Commit, Version: 2, To: 2, LossPct: 100}}
		}
		normalized, err := normalizeScheduledRuntimeConfig(cfg)
		if err != nil {
			t.Fatal(err)
		}
		r := newScheduledRuntime(normalized)
		if err := r.processCallStart(); err != nil {
			t.Fatal(err)
		}
		if err := r.scheduleInitialEvents(); err != nil {
			t.Fatal(err)
		}
		for r.queue.Len() > 0 {
			e, _ := r.queue.Pop()
			r.nowMS = e.TimeMs
			before := r.counters.RelayTransmissions
			if err := r.processEvent(e); err != nil {
				t.Fatal(err)
			}
			if lost && len(r.activeVersions) > 1 && r.counters.RelayTransmissions != before {
				t.Fatal("relay transmitted successor voice without delivered successor commit")
			}
		}
		result, err := r.result()
		if err != nil {
			t.Fatal(err)
		}
		if result.ActiveSchedule.Binding.ScheduleVersion != 2 || result.Counters.VersionActivationCount != 1 {
			t.Fatalf("no distributed activation: %+v", result.Counters)
		}
		if !lost && result.ReceiverCounters[3].PlayoutFramesEmitted+result.Counters.CollisionDropCount != 128 {
			t.Fatalf("replan interrupted healthy voice: %+v", result.Counters)
		}
		if lost && (result.Counters.RelayTransmissions == 0 || result.Counters.RelayTransmissions > int(result.ActiveSchedule.Binding.EffectiveSeq) || result.Counters.NotScheduledDropCount == 0) {
			t.Fatalf("lost successor commit granted implicit permission: %+v", result.Counters)
		}
	}
}

func TestControlRuntimeCoalescesBeforeDissemination(t *testing.T) {
	cfg := longControlConfig(t)
	cfg.ReplanInputs = []ScheduledReplanInput{controlReplanInput(t, 20100, 8), controlReplanInput(t, 20100, 9), controlReplanInput(t, 20200, 10)}
	r, err := RunScheduledRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if r.ActiveSchedule.Binding.ScheduleVersion != 3 || r.ActiveTree.TopologyEpoch != 10 || r.Counters.VersionActivationCount != 2 {
		t.Fatalf("coalescing/version policy: %+v schedule=%+v", r.Counters, r.ActiveSchedule.Binding)
	}
}

func TestControlRuntimeNoImplicitPreinstalledVersionAuthorization(t *testing.T) {
	cfg := longControlConfig(t)
	next := cloneScheduledVersions(cfg.Versions)[0]
	next.Schedule.Binding.ScheduleVersion = 2
	next.Schedule.Binding.EffectiveSeq = 10
	cfg.Versions = append(cfg.Versions, next)
	cfg.Control.Faults = []peer.ControlFault{{Kind: peer.Commit, To: 2, LossPct: 100}}
	r, err := RunScheduledRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if r.Counters.RelayTransmissions != 0 || r.ReceiverCounters[3].PlayoutFramesEmitted != 0 {
		t.Fatal("preinstalled version granted absent relay commit")
	}
}

func TestControlLocalEndDoesNotDrainUnrelatedQueueEvents(t *testing.T) {
	cfg := longControlConfig(t)
	normalized, err := normalizeScheduledRuntimeConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	r := newScheduledRuntime(normalized)
	if err := r.processCallStart(); err != nil {
		t.Fatal(err)
	}
	r.queue.Push(sim.Event{TimeMs: 40000, EventType: "foreign_event"})
	before := r.queue.Len()
	r.nowMS = 30000
	if err := r.processCallEnd(); err != nil {
		t.Fatal(err)
	}
	if r.queue.Len() < before {
		t.Fatal("End drained another controller's events")
	}
}
