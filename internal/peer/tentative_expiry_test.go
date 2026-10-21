package peer

import (
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
)

func TestPeerFailedPTTsReleaseTentativesAndRecover(t *testing.T) {
	for _, lostKind := range []Kind{ACK, Commit} {
		t.Run(string(lostKind), func(t *testing.T) {
			cfg, schedule := peerFixture(t)
			fleet, err := NewFleet(cfg.Peers)
			if err != nil {
				t.Fatal(err)
			}
			cfg.Fleet = fleet
			cfg.Faults = []ControlFault{{Kind: lostKind, LossPct: 100}}
			schedule.Binding.ValidThroughSeq = 0
			for i := 0; i < MaxTransactions+2; i++ {
				cfg.StartMS = int64(i) * 2000
				schedule.Binding.CallID = uint32(i + 1)
				q := sim.NewEventQueue()
				c, err := New(cfg, q)
				if err != nil {
					t.Fatal(err)
				}
				if err := c.Begin(schedule, cfg.StartMS+1000, 80); err != nil {
					t.Fatal(err)
				}
				drivePeers(t, q, cfg.StartMS+1500, c)
			}
			for id, local := range fleet.locals {
				if len(local.Pending) != 0 || local.Tentative != nil || local.Active != nil {
					t.Fatalf("peer %d retained expired failed-call state: pending=%d tentative=%v active=%v", id, len(local.Pending), local.Tentative != nil, local.Active != nil)
				}
			}
			cfg.StartMS = int64(MaxTransactions+2) * 2000
			cfg.Faults = nil
			schedule = peerSchedule(t, 4, 1000, []routing.RelayTreeEdge{{ParentStationID: 4, ChildStationID: 2}, {ParentStationID: 2, ChildStationID: 3, Hop: 1}}, []uint16{2, 3})
			q := sim.NewEventQueue()
			c, err := New(cfg, q)
			if err != nil {
				t.Fatal(err)
			}
			if err := c.Begin(schedule, cfg.StartMS+1000, 80); err != nil {
				t.Fatal(err)
			}
			drivePeers(t, q, cfg.StartMS+1000, c)
			if !c.CanTransmitVersion(4, 1, 0, cfg.StartMS+1000) || !c.CanTransmitVersion(2, 1, 0, cfg.StartMS+1011) {
				t.Fatal("healthy new-source PTT could not recover after failed calls")
			}
		})
	}
}

func TestPeerExpiredTentativeCannotReleaseForeignActive(t *testing.T) {
	cfg, schedule := peerFixture(t)
	q := sim.NewEventQueue()
	c := beginPeer(t, cfg, schedule, q)
	drivePeers(t, q, 1000, c)
	l := c.locals[2]
	active := l.Active
	old := *active
	old.Digest = "expired-uncommitted"
	old.ExpiresMS = 1100
	old.Committed = false
	l.Pending[old.Digest] = &old
	l.Tentative = &old
	c.release(l, old, 1100, true)
	if l.Active != active || l.Tentative != nil || l.Pending[old.Digest] != nil {
		t.Fatal("tentative expiry changed another active permit or retained own state")
	}
}
