package scenario

import (
	"github.com/rmsff/ad-hoc-radio/internal/peer"
	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
	"github.com/rmsff/ad-hoc-radio/internal/transport"
	"testing"
)

func scheduledControlConfig(t *testing.T) ScheduledRuntimeConfig {
	t.Helper()
	v := scheduledTestVersion(t, []routing.RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2}, {ParentStationID: 2, ChildStationID: 3, Hop: 1}}, []uint16{2, 3}, nil, 1, 1, 0, 9)
	v.Schedule.Binding.ScheduleAuthorityID = 1
	cfg := scheduledTestRuntimeConfig(v, 10)
	cfg.Source.startMs = 20000
	states := []peer.State{}
	for _, id := range []uint16{1, 2, 3} {
		states = append(states, peer.State{ID: id, Mode: peer.ADHR, RelayEnabled: true, BatteryFraction: 1, Clock: peer.Clock{MaxUncertaintyMS: 1}})
	}
	links := []transport.Link{}
	for _, edge := range [][2]uint16{{1, 2}, {2, 1}, {2, 3}, {3, 2}} {
		links = append(links, transport.Link{From: transport.StationID(edge[0]), To: transport.StationID(edge[1]), Enabled: true, FixedDelayMs: 1})
	}
	cfg.Control = &peer.Config{Peers: states, Links: links, StartMS: 0, RetryMS: 5000, Attempts: 2, Seed: 42, Physical: timing.VoiceBudgetProfile{ID: "control-test", CodecBitrateBPS: 1600, PacketPeriodMS: 80, GrossBitrateBPS: 19200, FECNumerator: 1, FECDenominator: 1}}
	return cfg
}

func TestScheduledControlIsCausal(t *testing.T) {
	tests := []struct {
		name       string
		edit       func(*ScheduledRuntimeConfig)
		wantPlayed bool
	}{
		{"normal", func(c *ScheduledRuntimeConfig) {}, true},
		{"lost_commit", func(c *ScheduledRuntimeConfig) {
			c.Control.Faults = []peer.ControlFault{{Kind: peer.Commit, To: 2, LossPct: 100}}
		}, false},
		{"late_commit", func(c *ScheduledRuntimeConfig) {
			c.Control.Faults = []peer.ControlFault{{Kind: peer.Commit, To: 2, DelayMS: 30000}}
		}, false},
		{"drift_expiry", func(c *ScheduledRuntimeConfig) {
			c.Control.Peers[1].Clock.DriftPPM = 10000
			c.Control.Peers[1].Clock.DriftBoundPPM = 10000
		}, false},
		{"reverse_ack_unreachable", func(c *ScheduledRuntimeConfig) { c.Control.Links[1].Enabled = false }, false},
		{"all_control_lost", func(c *ScheduledRuntimeConfig) {
			for i := range c.Control.Links {
				c.Control.Links[i].LossPct = 100
			}
		}, false},
		{"relay_disabled", func(c *ScheduledRuntimeConfig) { c.Control.Peers[1].RelayEnabled = false }, false},
		{"relay_fm", func(c *ScheduledRuntimeConfig) { c.Control.Peers[1].Mode = peer.AnalogFM }, false},
		{"relay_late_join", func(c *ScheduledRuntimeConfig) { c.Control.Peers[1].AvailableAtMS = 30000 }, false},
		{"clock_uncertainty", func(c *ScheduledRuntimeConfig) { c.Control.Peers[1].Clock.ResidualErrorMS = 10 }, false},
		{"duplicate_control", func(c *ScheduledRuntimeConfig) {
			for i := range c.Control.Links {
				c.Control.Links[i].DuplicatePct = 100
			}
		}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := scheduledControlConfig(t)
			tc.edit(&cfg)
			r, err := RunScheduledRuntime(cfg)
			if err != nil {
				t.Fatal(err)
			}
			played := r.ReceiverCounters[3].PlayoutFramesEmitted
			if (played == 10) != tc.wantPlayed {
				t.Fatalf("played=%d control=%+v runtime=%+v", played, r.ControlCounters, r.Counters)
			}
			if r.ControlCounters == nil || r.ControlCounters.Attempts > 80 {
				t.Fatalf("unbounded/no control: %+v", r.ControlCounters)
			}
		})
	}
}
