package scenario

import (
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/peer"
	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
	"github.com/rmsff/ad-hoc-radio/internal/transport"
)

func concurrentControlConfig(t *testing.T, source, relay, listener uint16) ScheduledRuntimeConfig {
	t.Helper()
	base := longControlConfig(t)
	v := base.Versions[0]
	v.Tree.SourceStationID = source
	v.Tree.Edges = []routing.RelayTreeEdge{{ParentStationID: source, ChildStationID: relay}, {ParentStationID: relay, ChildStationID: listener, Hop: 1}}
	v.Tree.Coverage.CoveredMemberIDs = []uint16{relay, listener}
	b := v.Schedule.Binding
	b.SourceStationID, b.ScheduleAuthorityID, b.TimingAuthorityID = source, source, source
	b.CallID = uint32(source) + 9000
	b.ExpectedMemberIDs = []uint16{relay, listener}
	var err error
	v.Schedule, err = timing.BuildOpportunitySchedule(b, v.Tree, timing.OpportunityProfile{SourceDurationMS: 10, RelayDurationMS: 10, GuardAfterMS: 1, MaxCycleMS: 80})
	if err != nil {
		t.Fatal(err)
	}
	v.LinkDelayMS = map[ScheduledLinkKey]int64{{FromStationID: source, ToStationID: relay}: 1, {FromStationID: relay, ToStationID: listener}: 1}
	cfg := scheduledTestRuntimeConfig(v, 128)
	cfg.Source.id, cfg.Source.callID, cfg.Source.startMs = source, b.CallID, 20000
	cfg.Control = base.Control
	return cfg
}

func sharedControlFleet(t *testing.T, configs []ScheduledRuntimeConfig, connected bool) {
	t.Helper()
	states := []peer.State{}
	for _, id := range []uint16{1, 2, 3, 4, 5, 6} {
		states = append(states, peer.State{ID: id, Mode: peer.ADHR, RelayEnabled: true, BatteryFraction: 1, Clock: peer.Clock{MaxUncertaintyMS: 2}})
	}
	fleet, err := peer.NewFleet(states)
	if err != nil {
		t.Fatal(err)
	}
	links := []transport.Link{}
	for _, a := range states {
		for _, b := range states {
			if a.ID != b.ID && (connected || (a.ID <= 3) == (b.ID <= 3)) {
				links = append(links, transport.Link{From: transport.StationID(a.ID), To: transport.StationID(b.ID), Enabled: true, FixedDelayMs: 1})
			}
		}
	}
	for i := range configs {
		configs[i].Control.Fleet = fleet
		configs[i].Control.Peers = states
		configs[i].Control.Links = links
	}
}

func TestControlConcurrentRuntimeSharedRelayBoundsBothPTTOrders(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		configs := []ScheduledRuntimeConfig{concurrentControlConfig(t, 1, 2, 3), concurrentControlConfig(t, 4, 2, 3)}
		sharedControlFleet(t, configs, true)
		if reverse {
			configs[0], configs[1] = configs[1], configs[0]
		}
		results, err := RunConcurrentScheduledRuntimes(configs)
		if err != nil {
			t.Fatal(err)
		}
		winners, attempts := 0, 0
		for _, r := range results {
			if r.Counters.SourceTransmissions > 0 {
				winners++
				if r.Counters.RelayTransmissions != 128 || r.ReceiverCounters[3].PlayoutFramesEmitted != 128 {
					t.Fatalf("winner lost healthy voice: %+v", r.Counters)
				}
			} else if r.Counters.RelayTransmissions != 0 || r.ReceiverCounters[3].PlayoutFramesEmitted != 0 {
				t.Fatal("losing call used foreign permit")
			}
			attempts += r.ControlCounters.Attempts
		}
		if winners != 1 || attempts > 100 {
			t.Fatalf("winners=%d attempts=%d", winners, attempts)
		}
	}
}

func TestControlConcurrentRuntimeIslandsAndSharedRFAreDistinct(t *testing.T) {
	for _, connected := range []bool{false, true} {
		configs := []ScheduledRuntimeConfig{concurrentControlConfig(t, 1, 2, 3), concurrentControlConfig(t, 4, 5, 6)}
		sharedControlFleet(t, configs, connected)
		// Separate setup intervals establish both local calls before voice overlaps.
		configs[1].Control.StartMS = 1000
		results, err := RunConcurrentScheduledRuntimes(configs)
		if err != nil {
			t.Fatal(err)
		}
		for i, r := range results {
			if r.Counters.SourceTransmissions != 128 {
				t.Fatalf("partition source vetoed: %+v", r.Counters)
			}
			played := r.ReceiverCounters[configs[i].Versions[0].Tree.Edges[1].ChildStationID].PlayoutFramesEmitted
			if !connected && played != 128 {
				t.Fatalf("isolated call played=%d", played)
			}
			if connected && (played != 0 || r.Counters.CollisionDropCount == 0) {
				t.Fatalf("overlapping shared RF claimed success: played=%d counters=%+v", played, r.Counters)
			}
		}
	}
}

func TestControlRuntimeDeliveredClockReacquisitionRestoresVoice(t *testing.T) {
	played := [2]int{}
	for i, lost := range []bool{true, false} {
		cfg := longControlConfig(t)
		cfg.Control.StartMS = 19500
		cfg.Control.Peers[1].Clock = peer.Clock{OffsetMS: 500, DriftPPM: -1000, DriftBoundPPM: 1000, MaxUncertaintyMS: 2}
		cfg.Reacquisitions = []ScheduledReacquisition{{PeerID: 2, AtMS: 22100}, {PeerID: 2, AtMS: 24100}, {PeerID: 2, AtMS: 26100}, {PeerID: 2, AtMS: 28100}}
		if lost {
			cfg.Control.Faults = []peer.ControlFault{{Kind: peer.Sync, LossPct: 100}}
		}
		r, err := RunScheduledRuntime(cfg)
		if err != nil {
			t.Fatal(err)
		}
		played[i] = r.ReceiverCounters[3].PlayoutFramesEmitted
		if r.Counters.OutOfScheduleTxCount != 0 {
			t.Fatalf("mapped TX mismatch: %+v", r.Counters)
		}
		if (r.ControlCounters.Reacquisitions == 0) != lost {
			t.Fatalf("lost=%t sync=%+v", lost, r.ControlCounters)
		}
	}
	if played[0] == 0 || played[1] <= played[0] {
		t.Fatalf("reacquisition did not causally recover voice: lost/delivered=%v", played)
	}
}
