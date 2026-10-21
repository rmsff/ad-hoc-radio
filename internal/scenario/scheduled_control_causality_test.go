package scenario

import (
	"github.com/rmsff/ad-hoc-radio/internal/node"
	"github.com/rmsff/ad-hoc-radio/internal/peer"
	"testing"
)

func TestControlListenerWithoutReversePathDoesNotVetoTX(t *testing.T) {
	cfg := scheduledControlConfig(t)
	cfg.Control.Links[3].Enabled = false
	result, err := RunScheduledRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result.Counters.SourceTransmissions != 10 || result.ReceiverCounters[2].PlayoutFramesEmitted != 10 {
		t.Fatalf("listener ACK vetoed TX: %+v", result.Counters)
	}
}

func TestControlLostEndDoesNotCloseLocalNode(t *testing.T) {
	cfg := scheduledControlConfig(t)
	cfg.Versions[0].Schedule.Binding.ValidThroughSeq = 100
	cfg.Control.Faults = []peer.ControlFault{{Kind: peer.End, LossPct: 100}}
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
		if err := r.processEvent(e); err != nil {
			t.Fatal(err)
		}
		if e.EventType == scheduledEventCallEnd {
			break
		}
	}
	key := node.CallKey{NetID: cfg.Source.netID, SourceID: cfg.Source.id, CallID: cfg.Source.callID}
	state, ok := r.cfg.Receivers[3].Lifecycle().Call(key)
	if !ok || !state.Active {
		t.Fatal("node closed before delivered End or expiry")
	}
}
