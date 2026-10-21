package peer

import (
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/sim"
	"github.com/rmsff/ad-hoc-radio/internal/transport"
)

func TestPeerRFOverlapJoinsPartialSnapshots(t *testing.T) {
	a := []transport.Link{{From: 1, To: 2, Enabled: true}}
	b := []transport.Link{{From: 3, To: 2, Enabled: true}}
	if !interferesOn(a, b, 1, 3) || !interferesOn(b, a, 3, 1) {
		t.Fatal("partial snapshots hid a shared receiver")
	}
}

func TestPeerRFOverlapUsesBothObservedSnapshots(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		cfg, _ := peerFixture(t)
		fleet, err := NewFleet(cfg.Peers)
		if err != nil {
			t.Fatal(err)
		}
		cfg.Fleet = fleet
		cfg.Links = []transport.Link{{From: 1, To: 2, Enabled: true}, {From: 3, To: 2, Enabled: true}}
		a, err := New(cfg, sim.NewEventQueue())
		if err != nil {
			t.Fatal(err)
		}
		cfg.Links = []transport.Link{{From: 3, To: 4, Enabled: true}}
		b, err := New(cfg, sim.NewEventQueue())
		if err != nil {
			t.Fatal(err)
		}
		var x, y *AirTransmission
		if reverse {
			y, err = b.ReserveAir(3, 100, 110)
			if err == nil {
				x, err = a.ReserveAir(1, 100, 110)
			}
		} else {
			x, err = a.ReserveAir(1, 100, 110)
			if err == nil {
				y, err = b.ReserveAir(3, 100, 110)
			}
		}
		if err != nil {
			t.Fatal(err)
		}
		if !a.AirCollided(x) || !b.AirCollided(y) {
			t.Fatalf("reverse=%t ignored known common receiver", reverse)
		}
	}
}
