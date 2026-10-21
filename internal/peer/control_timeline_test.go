package peer

import (
	"github.com/rmsff/ad-hoc-radio/internal/sim"
	"testing"
)

func TestCompactRejectsConflictingProposalTimeline(t *testing.T) {
	for _, timeline := range [][2]int64{{2000, 80}, {1000, 100}, {2000, 100}} {
		cfg, s := peerFixture(t)
		c, err := New(cfg, sim.NewEventQueue())
		if err != nil {
			t.Fatal(err)
		}
		a, _ := makePermit(s, 1000, 80)
		b, _ := makePermit(s, timeline[0], timeline[1])
		if a.Digest != b.Digest {
			t.Fatal("requires same schedule")
		}
		l := c.locals[2]
		if !c.acceptProposal(l, a, 0) {
			t.Fatal("initial proposal rejected")
		}
		m := message{Kind: Proposal, From: 1, To: 2, Nonce: 2, Permit: b}
		raw, err := encodeControl(m, CompactV1)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decodeControl(raw, CompactV1)
		if err != nil {
			t.Fatal(err)
		}
		if c.acceptProposal(l, decoded.Permit, 1) {
			t.Fatal("conflicting timeline accepted")
		}
		if err = c.installCommit(l, b, 2); err != nil {
			t.Fatal(err)
		}
		if l.Pending[a.Digest].Committed {
			t.Fatal("foreign commit granted old timeline")
		}
		if !c.acceptProposal(l, a, 3) {
			t.Fatal("exact duplicate no longer idempotent")
		}
	}
}

func TestStaleControllerReacquireRejectsReplacement(t *testing.T) {
	cfg, s := peerFixture(t)
	fleet, err := NewFleet(cfg.Peers)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Fleet = fleet
	q := sim.NewEventQueue()
	old := beginPeer(t, cfg, s, q)
	drivePeers(t, q, 9000, old)
	cfg.StartMS = 9100
	s.Binding.TopologyEpoch++
	next, err := New(cfg, q)
	if err != nil {
		t.Fatal(err)
	}
	if err = next.Begin(s, 10000, 80); err != nil {
		t.Fatal(err)
	}
	drivePeers(t, q, 10000, old, next)
	if !next.Matches(1) || old.Matches(1) {
		t.Fatal("replacement setup failed")
	}
	before := old.stats.Attempts
	if err = old.Reacquire(2, 10010); err == nil || old.stats.Attempts != before {
		t.Fatal("stale controller sent synchronization")
	}
}

func TestReacquireUsesActiveNotLatestTransaction(t *testing.T) {
	cfg, s := peerFixture(t)
	q := sim.NewEventQueue()
	c := beginPeer(t, cfg, s, q)
	drivePeers(t, q, 1000, c)
	active := *c.locals[1].Active
	next := cloneSchedule(s)
	next.Binding.ScheduleVersion = 2
	next.Binding.EffectiveSeq = 10
	next.Binding.TopologyEpoch = 2
	if err := c.Replan(next, 1100); err != nil {
		t.Fatal(err)
	}
	if c.permit.Digest == active.Digest {
		t.Fatal("requires pending successor")
	}
	if err := c.Reacquire(2, 1101); err != nil {
		t.Fatalf("active version must remain usable: %v", err)
	}
	tx := c.transactions[active.Digest]
	delete(c.transactions, active.Digest)
	if err := c.Reacquire(2, 1102); err == nil {
		t.Fatal("missing transaction accepted")
	}
	c.transactions[active.Digest] = tx
	tx.permit.PeriodMS++
	if err := c.Reacquire(2, 1103); err == nil {
		t.Fatal("mismatched active transaction accepted")
	}
}

func TestOldTimelineEventsCannotMutateReplacement(t *testing.T) {
	for _, event := range []string{"expire", "end", "activate"} {
		t.Run(event, func(t *testing.T) {
			cfg, s := peerFixture(t)
			c, err := New(cfg, sim.NewEventQueue())
			if err != nil {
				t.Fatal(err)
			}
			a, _ := makePermit(s, 1000, 80)
			b, _ := makePermit(s, 10000, 80)
			l := c.locals[2]
			if !c.acceptProposal(l, a, 0) || !c.acceptProposal(l, b, a.ExpiresMS) {
				t.Fatal("timeline setup")
			}
			if err = c.installCommit(l, b, a.ExpiresMS); err != nil {
				t.Fatal(err)
			}
			if event == "end" {
				c.release(l, a, a.ExpiresMS, false)
			} else {
				if _, err = c.processLocal(pending{Local: event, Message: message{Permit: a, To: 2}}, a.ExpiresMS); err != nil {
					t.Fatal(err)
				}
			}
			if l.Pending[b.Digest] == nil || l.Tentative == nil || l.Active != nil {
				t.Fatal("old event deleted/activated replacement")
			}
			if _, err = c.processLocal(pending{Local: "activate", Message: message{Permit: b, To: 2}}, 10000); err != nil {
				t.Fatal(err)
			}
			if l.Active == nil || !samePermitReference(*l.Active, b) {
				t.Fatal("replacement activation failed")
			}
			c.source = 1
			c.permit = a
			if c.Matches(2) || c.AcceptVoice(2, 1, 0, 10000) {
				t.Fatal("old controller gained replacement authority")
			}
			c.release(l, a, 10001, true)
			if l.Active == nil {
				t.Fatal("old expiry removed new active")
			}
		})
	}
}
