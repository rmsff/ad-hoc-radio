package peer

import (
	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
	"github.com/rmsff/ad-hoc-radio/internal/transport"
	"math"
	"testing"
)

func peerFixture(t testing.TB) (Config, timing.OpportunitySchedule) {
	t.Helper()
	states := []State{}
	for _, id := range []uint16{1, 2, 3, 4} {
		states = append(states, State{ID: id, Mode: ADHR, RelayEnabled: true, BatteryFraction: 1, Clock: Clock{MaxUncertaintyMS: 2}})
	}
	links := []transport.Link{}
	for _, a := range states {
		for _, b := range states {
			if a.ID != b.ID {
				links = append(links, transport.Link{From: transport.StationID(a.ID), To: transport.StationID(b.ID), Enabled: true, FixedDelayMs: 1})
			}
		}
	}
	cfg := Config{Peers: states, Links: links, StartMS: 0, RetryMS: 100, Attempts: 3, ReplanLeadMS: 100, MinBattery: 0.2, Physical: timing.VoiceBudgetProfile{ID: "fast-control-fixture", CodecBitrateBPS: 1600, PacketPeriodMS: 80, GrossBitrateBPS: 1000000000, FECNumerator: 1, FECDenominator: 1}}
	return cfg, peerSchedule(t, 1, 10, []routing.RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2}, {ParentStationID: 2, ChildStationID: 3, Hop: 1}}, []uint16{2, 3})
}
func peerSchedule(t testing.TB, source uint16, call uint32, edges []routing.RelayTreeEdge, members []uint16) timing.OpportunitySchedule {
	t.Helper()
	depth := uint8(0)
	for _, e := range edges {
		if e.Hop > depth {
			depth = e.Hop
		}
	}
	tree := routing.RelayTree{SourceStationID: source, MembershipSnapshotID: 1, TopologyEpoch: 1, MaxRelayDepth: depth, Edges: edges, Coverage: routing.CoverageSet{CoveredMemberIDs: members}}
	binding := timing.ScheduleBinding{ScheduleID: 1, ScheduleVersion: 1, NetID: 1, GroupID: 1, SourceStationID: source, CallID: call, ScheduleAuthorityID: source, TimingAuthorityID: source, TimingStrategy: timing.TimingStrategySourceLed, TopologyEpoch: 1, MembershipSnapshotID: 1, MembershipFreshnessMS: 1000000, ValidThroughSeq: 99, ExpectedMemberIDs: members}
	s, err := timing.BuildOpportunitySchedule(binding, tree, timing.OpportunityProfile{SourceDurationMS: 10, RelayDurationMS: 10, GuardAfterMS: 1, MaxCycleMS: 80})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func drivePeers(t *testing.T, q *sim.EventQueue, through int64, controllers ...*Controller) []EventResult {
	t.Helper()
	results := []EventResult{}
	for count := 0; ; count++ {
		e, ok := q.Peek()
		if !ok || e.TimeMs > through {
			break
		}
		if count >= MaxControlEvents {
			t.Fatal("unbounded dispatch")
		}
		q.Pop()
		found := false
		for _, c := range controllers {
			if c.HasEvent(e.EventID) {
				r, err := c.Process(e)
				if err != nil {
					t.Fatal(err)
				}
				results = append(results, r)
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("unowned event=%+v", e)
		}
	}
	return results
}
func beginPeer(t *testing.T, cfg Config, s timing.OpportunitySchedule, q *sim.EventQueue) *Controller {
	t.Helper()
	c, err := New(cfg, q)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Begin(s, 1000, 80); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestPeerControlLossReorderAndDuplicate(t *testing.T) {
	for _, kind := range []Kind{Proposal, ACK, Commit} {
		t.Run(string(kind), func(t *testing.T) {
			cfg, s := peerFixture(t)
			cfg.Faults = []ControlFault{{Kind: kind, To: 2, LossPct: 100}}
			if kind == ACK {
				cfg.Faults[0].From = 2
				cfg.Faults[0].To = 1
			}
			q := sim.NewEventQueue()
			c := beginPeer(t, cfg, s, q)
			drivePeers(t, q, 1000, c)
			if c.CanTransmitVersion(2, 1, 0, 1000) {
				t.Fatal("unconfirmed relay can transmit")
			}
		})
	}
	cfg, s := peerFixture(t)
	cfg.Faults = []ControlFault{{Kind: Commit, To: 2, DelayMS: 2000}}
	q := sim.NewEventQueue()
	c := beginPeer(t, cfg, s, q)
	drivePeers(t, q, 4000, c)
	if c.CanTransmitVersion(2, 1, 40, 4200) {
		t.Fatal("late commit backdated permission")
	}
	cfg, s = peerFixture(t)
	for i := range cfg.Links {
		cfg.Links[i].DuplicatePct = 100
	}
	q = sim.NewEventQueue()
	c = beginPeer(t, cfg, s, q)
	drivePeers(t, q, 1000, c)
	if !c.CanTransmitVersion(2, 1, 0, 1011) || c.stats.Duplicates == 0 || c.stats.Attempts > 30 {
		t.Fatalf("duplicate handling: %+v", c.stats)
	}
}

func TestPeerReplanRequiresNewLocalCommit(t *testing.T) {
	for _, lost := range []bool{false, true} {
		cfg, s := peerFixture(t)
		q := sim.NewEventQueue()
		c := beginPeer(t, cfg, s, q)
		drivePeers(t, q, 1000, c)
		if lost {
			c.cfg.Faults = []ControlFault{{Kind: Commit, To: 2, LossPct: 100}}
		}
		next := cloneSchedule(s)
		next.Binding.ScheduleVersion = 2
		next.Binding.EffectiveSeq = 10
		next.Binding.TopologyEpoch = 2
		if err := c.Replan(next, 1100); err != nil {
			t.Fatal(err)
		}
		drivePeers(t, q, 1800, c)
		if !c.CanTransmitVersion(1, 2, 10, 1800) {
			t.Fatal("source successor not committed")
		}
		if c.CanTransmitVersion(2, 2, 10, 1811) == lost {
			t.Fatalf("lost=%t new relay permit incorrect", lost)
		}
		if c.AcceptVoice(2, 2, 10, 1810) == lost {
			t.Fatalf("lost=%t wire/local version binding incorrect", lost)
		}
		if err := c.Replan(next, 1900); err == nil {
			t.Fatal("accepted repeated/conflicting version")
		}
	}
}

func TestPeerLostEndAndStaleEndCannotClearNewCall(t *testing.T) {
	cfg, s := peerFixture(t)
	cfg.Faults = []ControlFault{{Kind: End, LossPct: 100}}
	fleet, err := NewFleet(cfg.Peers)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Fleet = fleet
	q := sim.NewEventQueue()
	old := beginPeer(t, cfg, s, q)
	drivePeers(t, q, 1000, old)
	if err := old.End(1100); err != nil {
		t.Fatal(err)
	}
	drivePeers(t, q, 1200, old)
	if !old.CanTransmitVersion(2, 1, 2, 1171) {
		t.Fatal("lost End revoked remote permit globally")
	}
	drivePeers(t, q, 9000, old)
	if old.Matches(2) {
		t.Fatal("permit did not expire")
	}
	nextCfg := cfg
	nextCfg.Faults = nil
	nextCfg.StartMS = 9100
	next, err := New(nextCfg, q)
	if err != nil {
		t.Fatal(err)
	}
	s.Binding.CallID++
	if err := next.Begin(s, 10000, 80); err != nil {
		t.Fatal(err)
	}
	drivePeers(t, q, 10000, old, next)
	if !next.Matches(1) || old.Matches(1) {
		t.Fatal("foreign call accepted by Matches")
	}
	old.ended = false // Exercise delayed old End, including its source-local release.
	if err := old.End(10010); err != nil {
		t.Fatal(err)
	}
	drivePeers(t, q, 10100, old, next)
	if !next.Matches(1) || !next.Matches(2) {
		t.Fatal("old End stole new call")
	}
}

func TestPeerClockExpiryAndDeliveredReacquisition(t *testing.T) {
	cfg, s := peerFixture(t)
	cfg.Peers[1].Clock = Clock{OffsetMS: 500, DriftPPM: 1000, DriftBoundPPM: 1000, MaxUncertaintyMS: 2}
	q := sim.NewEventQueue()
	c := beginPeer(t, cfg, s, q)
	drivePeers(t, q, 1000, c)
	if !c.CanTransmitVersion(2, 1, 0, 1011) {
		t.Fatal("acquired clock unusable")
	}
	if c.TransmitAt(2, 2011) == 2011 {
		t.Fatal("drift did not move TX")
	}
	if c.CanTransmitVersion(2, 1, 40, 4211) {
		t.Fatal("uncertain clock authorized TX")
	}
	if err := c.Reacquire(2, 4300); err != nil {
		t.Fatal(err)
	}
	drivePeers(t, q, 4550, c)
	if !c.CanTransmitVersion(2, 1, 45, 4611) || c.stats.Reacquisitions == 0 {
		t.Fatal("delivered sync did not recover")
	}
	c.cfg.Faults = []ControlFault{{Kind: Sync, LossPct: 100}}
	if err := c.Reacquire(2, 7000); err != nil {
		t.Fatal(err)
	}
	drivePeers(t, q, 7300, c)
	if c.CanTransmitVersion(2, 1, 80, 7411) {
		t.Fatal("lost sync refreshed clock")
	}
	drivePeers(t, q, 10000, c)
	if c.Matches(2) {
		t.Fatal("local expiry did not release")
	}
}

func TestPeerInputBounds(t *testing.T) {
	for _, value := range []int64{math.MinInt64, math.MaxInt64} {
		cfg, s := peerFixture(t)
		cfg.Peers[0].Clock.OffsetMS = value
		if _, err := New(cfg, sim.NewEventQueue()); err == nil {
			t.Fatal("accepted extreme offset")
		}
		cfg, s = peerFixture(t)
		c, err := New(cfg, sim.NewEventQueue())
		if err != nil {
			t.Fatal(err)
		}
		if err := c.Begin(s, 1000, value); err == nil {
			t.Fatal("accepted period overflow")
		}
	}
	for _, edit := range []func(*Config){func(c *Config) { c.MinBattery = math.NaN() }, func(c *Config) { c.Faults = []ControlFault{{Kind: End, DelayMS: math.MaxInt64}} }, func(c *Config) { c.Links[0].JitterMs = math.MaxInt64 }, func(c *Config) { c.Links = append(c.Links, c.Links[0]) }, func(c *Config) { c.Links[0].Enabled = false; c.Links[0].LossPct = math.NaN() }} {
		cfg, _ := peerFixture(t)
		edit(&cfg)
		if _, err := New(cfg, sim.NewEventQueue()); err == nil {
			t.Fatal("accepted invalid input")
		}
	}
	cfg, s := peerFixture(t)
	q := sim.NewEventQueue()
	c := beginPeer(t, cfg, s, q)
	c.eventCount = MaxControlEvents
	if err := c.push(1, pending{}, 1); err == nil {
		t.Fatal("event bound absent")
	}
}
