package peer

import (
	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
	"github.com/rmsff/ad-hoc-radio/internal/transport"
	"testing"
)

func TestPeerSplitMergeHasNoGlobalFloorAndReportsRFOverlap(t *testing.T) {
	cfg, _ := peerFixture(t)
	cfg.Links = []transport.Link{{From: 1, To: 2, Enabled: true}, {From: 2, To: 1, Enabled: true}, {From: 3, To: 4, Enabled: true}, {From: 4, To: 3, Enabled: true}}
	f, err := NewFleet(cfg.Peers)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Fleet = f
	q := sim.NewEventQueue()
	a := beginPeer(t, cfg, peerSchedule(t, 1, 10, []routing.RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2}}, []uint16{2}), q)
	b := beginPeer(t, cfg, peerSchedule(t, 3, 20, []routing.RelayTreeEdge{{ParentStationID: 3, ChildStationID: 4}}, []uint16{4}), q)
	drivePeers(t, q, 1000, a, b)
	if !a.CanTransmit(1, 0, 1000) || !b.CanTransmit(3, 0, 1000) {
		t.Fatal("disjoint partitions incorrectly require global floor")
	}
	txA, err := a.ReserveAir(1, 1100, 1110)
	if err != nil {
		t.Fatal(err)
	}
	txB, err := b.ReserveAir(3, 1100, 1110)
	if err != nil {
		t.Fatal(err)
	}
	if a.AirCollided(txA) || b.AirCollided(txB) {
		t.Fatal("disjoint RF islands collided")
	}
	merged := append(append([]transport.Link(nil), cfg.Links...), transport.Link{From: 1, To: 4, Enabled: true})
	if err := a.UpdateLinks(merged, 1150, 1000); err != nil {
		t.Fatal(err)
	}
	if err := b.UpdateLinks(merged, 1150, 1000); err != nil {
		t.Fatal(err)
	}
	txA, err = a.ReserveAir(1, 1200, 1210)
	if err != nil {
		t.Fatal(err)
	}
	txB, err = b.ReserveAir(3, 1200, 1210)
	if err != nil {
		t.Fatal(err)
	}
	if !a.AirCollided(txA) || !b.AirCollided(txB) {
		t.Fatal("merged overlapping RF occupancy reported success")
	}
	if a.Matches(3) || b.Matches(1) {
		t.Fatal("foreign call active state leaked")
	}
}

func TestPeerConcurrentSharedRelayCannotAcquireBothCalls(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		cfg, s := peerFixture(t)
		f, err := NewFleet(cfg.Peers)
		if err != nil {
			t.Fatal(err)
		}
		cfg.Fleet = f
		q := sim.NewEventQueue()
		other := peerSchedule(t, 4, 20, []routing.RelayTreeEdge{{ParentStationID: 4, ChildStationID: 2}, {ParentStationID: 2, ChildStationID: 3, Hop: 1}}, []uint16{2, 3})
		var a, b *Controller
		if reverse {
			b = beginPeer(t, cfg, other, q)
			a = beginPeer(t, cfg, s, q)
		} else {
			a = beginPeer(t, cfg, s, q)
			b = beginPeer(t, cfg, other, q)
		}
		drivePeers(t, q, 1000, a, b)
		if a.Matches(2) && b.Matches(2) {
			t.Fatal("shared relay committed two calls")
		}
		if !a.Matches(2) && !b.Matches(2) {
			t.Fatal("bounded retries made no progress on healthy shared relay")
		}
		if a.stats.Attempts+b.stats.Attempts > 100 {
			t.Fatal("unbounded contention")
		}
	}
}

func TestPeerStaleSnapshotAndBoundedDeepDuplicates(t *testing.T) {
	cfg, s := peerFixture(t)
	cfg.LinksFreshnessMS = 1
	cfg.StartMS = 10
	q := sim.NewEventQueue()
	c := beginPeer(t, cfg, s, q)
	drivePeers(t, q, 1000, c)
	if c.CanTransmit(2, 0, 1011) {
		t.Fatal("stale local links authorized relay")
	}
	cfg, s = peerFixture(t)
	cfg.Links = []transport.Link{{From: 1, To: 4, Enabled: true, DuplicatePct: 100}, {From: 4, To: 2, Enabled: true, DuplicatePct: 100}, {From: 2, To: 3, Enabled: true, DuplicatePct: 100}, {From: 3, To: 2, Enabled: true, DuplicatePct: 100}, {From: 2, To: 4, Enabled: true, DuplicatePct: 100}, {From: 4, To: 1, Enabled: true, DuplicatePct: 100}}
	q = sim.NewEventQueue()
	c = beginPeer(t, cfg, s, q)
	drivePeers(t, q, 1000, c)
	if !c.CanTransmit(2, 0, 1011) || c.stats.Attempts > 50 || c.stats.Duplicates == 0 {
		t.Fatalf("deep duplicate amplification: %+v", c.stats)
	}
}
