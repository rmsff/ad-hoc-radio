package transport

import (
	"math"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/protocol"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
)

// Canonical Story 1.3 station ids per frame-contract.md §4.3 and
// algorithm-and-experiments.md §5.
const (
	stationA StationID = 1
	stationR StationID = 2
	stationB StationID = 3
	netID              = 0x0001
	groupID            = 0x0064
	callID             = 0x00009001
)

// canonicalTopology returns the A->R->B topology with A->B disabled,
// per algorithm-and-experiments.md §2 and §5.
func canonicalTopology() Topology {
	return Topology{
		Links: []Link{
			{From: stationA, To: stationR, Enabled: true, LossPct: 0, FixedDelayMs: 10},
			{From: stationR, To: stationB, Enabled: true, LossPct: 0, FixedDelayMs: 10},
			{From: stationA, To: stationB, Enabled: false},
		},
	}
}

// makeVoiceFrame builds a direct Voice Frame from A with the given seq.
func makeVoiceFrame(seq uint32) protocol.Frame {
	payload := []byte{byte(seq), 0x20, 0x30, 0x40}
	return protocol.Frame{
		Version:     protocol.Version,
		FrameType:   protocol.FrameTypeVoice,
		NetID:       netID,
		SourceID:    uint16(stationA),
		GroupID:     groupID,
		RelayID:     0,
		CallID:      callID,
		Seq:         seq,
		Hop:         0,
		TTL:         1,
		Flags:       protocol.FlagDirect,
		CreatedAtMs: int64(seq) * 80,
		SentAtMs:    int64(seq) * 80,
		PayloadLen:  uint16(len(payload)),
		Payload:     payload,
	}
}

// deliveryCollector records deliveries per destination station, so
// tests can assert exactly what B (or R) received, independently of
// what the other station received.
type deliveryCollector struct {
	toR []Delivery
	toB []Delivery
}

func (c *deliveryCollector) receiverFor(id StationID) Receiver {
	return func(d Delivery) {
		switch id {
		case stationR:
			c.toR = append(c.toR, d)
		case stationB:
			c.toB = append(c.toB, d)
		}
	}
}

// directFromA returns the deliveries to B that came directly from A
// (i.e. over the A->B link), which must be zero when A->B is disabled.
func (c *deliveryCollector) directFromAToB() int {
	n := 0
	for _, d := range c.toB {
		if d.From == stationA {
			n++
		}
	}
	return n
}

// TestIT001_TopologyDisabledLink is AC-1: with A->B disabled, A sends
// 100 frames, B receives exactly 0 frames directly from A.
//
// Traceability: IT-001 topology part (FR-1, FR-5).
func TestIT001_TopologyDisabledLink(t *testing.T) {
	const n = 100
	const seed = 42

	top := canonicalTopology()
	q := sim.NewEventQueue()
	tr := NewLocalTransport(top, q, seed)

	coll := &deliveryCollector{}
	tr.Register(stationR, coll.receiverFor(stationR))
	tr.Register(stationB, coll.receiverFor(stationB))

	for seq := uint32(0); seq < n; seq++ {
		tr.Send(stationA, makeVoiceFrame(seq), int64(seq)*80)
	}
	tr.Run()

	// AC-1: B receives exactly 0 frames directly from A.
	if got := coll.directFromAToB(); got != 0 {
		t.Errorf("B received %d frames directly from A; want 0 (A->B disabled)", got)
	}
	// R should receive all 100 from A over the enabled A->R link.
	if got, want := len(coll.toR), n; got != want {
		t.Errorf("R received %d frames from A; want %d", got, want)
	}
	// B receives nothing in this test (no relay echo configured).
	if got := len(coll.toB); got != 0 {
		t.Errorf("B received %d frames; want 0 (no relay echo in this test)", got)
	}
	// Counters confirm the disabled link delivered nothing.
	if got := tr.Counters().Delivered[linkKey(stationA, stationB)]; got != 0 {
		t.Errorf("A->B delivered counter = %d, want 0", got)
	}
	if got, want := tr.Counters().Delivered[linkKey(stationA, stationR)], n; got != want {
		t.Errorf("A->R delivered counter = %d, want %d", got, want)
	}
}

// TestIT001_TopologyEnabledNextHop is AC-2: with A->R and R->B enabled
// at 0% loss, frames are delivered to the enabled next hop. Here a
// minimal "echo relay" re-sends every frame it receives, so B receives
// frames from R, proving the R->B link delivers.
func TestIT001_TopologyEnabledNextHop(t *testing.T) {
	const n = 20
	const seed = 42

	top := canonicalTopology()
	q := sim.NewEventQueue()
	tr := NewLocalTransport(top, q, seed)

	coll := &deliveryCollector{}
	// R echoes every received frame back onto the transport as a
	// relayed copy (relay_id=R, hop=1, flags=relayed). This is NOT the
	// real relay state machine (Story 2.1) — just enough to prove the
	// R->B link delivers.
	tr.Register(stationR, func(d Delivery) {
		// Record R's receipt separately; do NOT pollute B's counters.
		coll.toR = append(coll.toR, d)
		if d.Corrupted {
			return // corrupted frame would fail validation; skip echo
		}
		relayed := d.Frame
		relayed.RelayID = uint16(stationR)
		relayed.Hop = 1
		relayed.TTL = 0
		relayed.Flags = protocol.FlagRelayed
		relayed.SentAtMs = d.ReceivedAt
		tr.Send(stationR, relayed, d.ReceivedAt)
	})
	tr.Register(stationB, coll.receiverFor(stationB))

	for seq := uint32(0); seq < n; seq++ {
		tr.Send(stationA, makeVoiceFrame(seq), int64(seq)*80)
	}
	tr.Run()

	// B received 0 directly from A (A->B disabled).
	if got := coll.directFromAToB(); got != 0 {
		t.Errorf("B received %d direct from A; want 0", got)
	}
	// B received n from R via the enabled R->B link.
	if got, want := len(coll.toB), n; got != want {
		t.Errorf("B received %d frames from R; want %d", got, want)
	}
	// Counters confirm both enabled links delivered n.
	if got, want := tr.Counters().Delivered[linkKey(stationA, stationR)], n; got != want {
		t.Errorf("A->R delivered = %d, want %d", got, want)
	}
	if got, want := tr.Counters().Delivered[linkKey(stationR, stationB)], n; got != want {
		t.Errorf("R->B delivered = %d, want %d", got, want)
	}
}

// TestAC3_DeterminismSameSeed is AC-3: the same topology config and
// seed run twice produces identical event count and event hash.
func TestAC3_DeterminismSameSeed(t *testing.T) {
	const n = 50
	const seed = 42

	run := func() (int, string) {
		top := canonicalTopology()
		q := sim.NewEventQueue()
		tr := NewLocalTransport(top, q, seed)
		tr.Register(stationR, func(d Delivery) {})
		tr.Register(stationB, func(d Delivery) {})
		for seq := uint32(0); seq < n; seq++ {
			tr.Send(stationA, makeVoiceFrame(seq), int64(seq)*80)
		}
		tr.Run()
		hash, err := q.EventHash()
		if err != nil {
			t.Fatalf("EventHash: %v", err)
		}
		return len(q.Emitted()), hash
	}

	count1, hash1 := run()
	count2, hash2 := run()
	if count1 != count2 {
		t.Errorf("event count differs across runs: %d vs %d", count1, count2)
	}
	if hash1 != hash2 {
		t.Errorf("event hash differs across runs:\n  run1=%s\n  run2=%s", hash1, hash2)
	}
	if count1 == 0 {
		t.Error("expected non-zero event count")
	}
}

// TestAC3_DifferentSeedProducesDifferentHash guards against the
// degenerate case where the seed is not actually wired into the
// impairment model: a different seed must produce a different event
// hash when loss/jitter are enabled.
func TestAC3_DifferentSeedProducesDifferentHash(t *testing.T) {
	const n = 50
	top := Topology{
		Links: []Link{
			{From: stationA, To: stationR, Enabled: true, LossPct: 20, FixedDelayMs: 10, JitterMs: 20},
			{From: stationR, To: stationB, Enabled: true, LossPct: 0, FixedDelayMs: 10},
		},
	}

	run := func(seed uint64) string {
		q := sim.NewEventQueue()
		tr := NewLocalTransport(top, q, seed)
		tr.Register(stationR, func(d Delivery) {})
		tr.Register(stationB, func(d Delivery) {})
		for seq := uint32(0); seq < n; seq++ {
			tr.Send(stationA, makeVoiceFrame(seq), int64(seq)*80)
		}
		tr.Run()
		hash, _ := q.EventHash()
		return hash
	}

	h1 := run(1)
	h2 := run(2)
	if h1 == h2 {
		t.Errorf("different seeds produced identical event hash with loss+jitter; seed not wired in:\n  h1=%s\n  h2=%s", h1, h2)
	}
}

// TestLinkLoss_Deterministic proves UT-009-style determinism: the same
// seed produces the same loss pattern across runs.
func TestLinkLoss_Deterministic(t *testing.T) {
	const n = 200
	top := Topology{
		Links: []Link{
			{From: stationA, To: stationR, Enabled: true, LossPct: 50},
		},
	}
	run := func(seed uint64) int {
		q := sim.NewEventQueue()
		tr := NewLocalTransport(top, q, seed)
		tr.Register(stationR, func(d Delivery) {})
		for seq := uint32(0); seq < n; seq++ {
			tr.Send(stationA, makeVoiceFrame(seq), int64(seq)*80)
		}
		tr.Run()
		return tr.Counters().Delivered[linkKey(stationA, stationR)]
	}
	d1 := run(42)
	d2 := run(42)
	if d1 != d2 {
		t.Errorf("loss not deterministic: run1 delivered %d, run2 delivered %d", d1, d2)
	}
	if d1 == n {
		t.Errorf("50%% loss link delivered all %d frames; loss not applied", n)
	}
	if d1 == 0 {
		t.Errorf("50%% loss link delivered 0 frames; loss too aggressive")
	}
}

// TestCorruption_ExercisesDecodeFailure proves that a corrupted
// delivery surfaces as a Delivery with Corrupted=true and a zero
// Frame (decode failure), so receivers can record an "invalid" drop.
func TestCorruption_ExercisesDecodeFailure(t *testing.T) {
	top := Topology{
		Links: []Link{
			{From: stationA, To: stationR, Enabled: true, CorruptionPct: 100},
		},
	}
	q := sim.NewEventQueue()
	tr := NewLocalTransport(top, q, 42)

	var got Delivery
	tr.Register(stationR, func(d Delivery) { got = d })
	tr.Send(stationA, makeVoiceFrame(0), 0)
	tr.Run()

	if !got.Corrupted {
		t.Error("delivery not marked corrupted with 100% corruption link")
	}
	// A corrupted frame must not decode: Frame stays zero, Raw is set.
	if got.Frame.Seq != 0 {
		t.Errorf("corrupted frame decoded unexpectedly: seq=%d", got.Frame.Seq)
	}
	if len(got.Raw) == 0 {
		t.Error("corrupted delivery has no raw bytes")
	}
	if _, err := protocol.Decode(got.Raw); err == nil {
		t.Error("corrupted raw bytes decoded successfully; corruption did not break CRC")
	}
	if got := tr.Counters().Corrupted[linkKey(stationA, stationR)]; got != 1 {
		t.Errorf("corrupted counter = %d, want 1", got)
	}
}

// TestDuplication_SchedulesSecondDelivery proves the duplicate_pct
// path schedules a second delivery flagged as a duplicate.
func TestDuplication_SchedulesSecondDelivery(t *testing.T) {
	top := Topology{
		Links: []Link{
			{From: stationA, To: stationR, Enabled: true, DuplicatePct: 100},
		},
	}
	q := sim.NewEventQueue()
	tr := NewLocalTransport(top, q, 42)

	var deliveries []Delivery
	tr.Register(stationR, func(d Delivery) { deliveries = append(deliveries, d) })
	tr.Send(stationA, makeVoiceFrame(0), 0)
	tr.Run()

	if got := len(deliveries); got != 2 {
		t.Fatalf("expected 2 deliveries (original + duplicate), got %d", got)
	}
	if deliveries[0].IsDuplicate {
		t.Error("first delivery flagged as duplicate; want original")
	}
	if !deliveries[1].IsDuplicate {
		t.Error("second delivery not flagged as a duplicate; want IsDuplicate=true")
	}
	if got := tr.Counters().Duplicated[linkKey(stationA, stationR)]; got != 1 {
		t.Errorf("duplicated counter = %d, want 1", got)
	}
}

// TestJitter_NonNegative ensures jitter draws are non-negative and
// bounded by the configured JitterMs.
func TestJitter_NonNegative(t *testing.T) {
	top := Topology{
		Links: []Link{
			{From: stationA, To: stationR, Enabled: true, FixedDelayMs: 100, JitterMs: 30},
		},
	}
	q := sim.NewEventQueue()
	tr := NewLocalTransport(top, q, 7)

	var got Delivery
	tr.Register(stationR, func(d Delivery) { got = d })
	tr.Send(stationA, makeVoiceFrame(0), 0)
	tr.Run()

	// arrival = 0 + 100 + jitter in [0, 30]
	if got.ReceivedAt < 100 || got.ReceivedAt > 130 {
		t.Errorf("arrival = %d, want in [100, 130] (fixed delay 100 + jitter [0,30])", got.ReceivedAt)
	}
}

// TestJitter_MaxInt64Panics is Review Finding 4: a jitter_ms at or
// above math.MaxInt64 would overflow jitter_ms+1 and panic inside
// rand.Int64N with a confusing message; the transport surfaces it as
// a clear misconfiguration panic instead.
func TestJitter_MaxInt64Panics(t *testing.T) {
	top := Topology{
		Links: []Link{
			{From: stationA, To: stationR, Enabled: true, FixedDelayMs: 0, JitterMs: math.MaxInt64},
		},
	}
	tr := NewLocalTransport(top, sim.NewEventQueue(), 1)
	tr.Register(stationR, func(d Delivery) {})
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Send with JitterMs=MaxInt64 did not panic")
		}
	}()
	tr.Send(stationA, makeVoiceFrame(0), 0)
}

// TestDefensiveCopy_RawNotShared is Review Finding 5: each delivery
// must own an exclusive copy of Raw, so a receiver mutating Raw cannot
// affect a sibling or a duplicate delivery that shares the same
// source buffer.
func TestDefensiveCopy_RawNotShared(t *testing.T) {
	top := Topology{
		Links: []Link{
			{From: stationA, To: stationR, Enabled: true, DuplicatePct: 100},
		},
	}
	q := sim.NewEventQueue()
	tr := NewLocalTransport(top, q, 42)

	var deliveries []Delivery
	tr.Register(stationR, func(d Delivery) {
		// Mutate ONLY the original delivery's Raw to prove the
		// duplicate's Raw is an independent copy.
		if !d.IsDuplicate && len(d.Raw) > 0 {
			d.Raw[0] = 0xFF
		}
		deliveries = append(deliveries, d)
	})
	tr.Send(stationA, makeVoiceFrame(0), 0)
	tr.Run()

	if len(deliveries) != 2 {
		t.Fatalf("expected 2 deliveries (original + duplicate), got %d", len(deliveries))
	}
	// The duplicate's Raw must not carry the mutation applied to the
	// original's Raw.
	if len(deliveries[1].Raw) > 0 && deliveries[1].Raw[0] == 0xFF {
		t.Fatal("duplicate Raw shares storage with original; defensive copy missing")
	}
}

// TestDefensiveCopy_AcrossLinks proves two links from the same sender
// get independent Raw slices, so corrupting one receiver's view does
// not leak to the other.
func TestDefensiveCopy_AcrossLinks(t *testing.T) {
	top := Topology{
		Links: []Link{
			{From: stationA, To: stationR, Enabled: true},
			{From: stationA, To: stationB, Enabled: true},
		},
	}
	q := sim.NewEventQueue()
	tr := NewLocalTransport(top, q, 42)

	var toR, toB Delivery
	tr.Register(stationR, func(d Delivery) {
		toR = d
		if len(d.Raw) > 0 {
			d.Raw[0] = 0xEE
		}
	})
	tr.Register(stationB, func(d Delivery) { toB = d })
	tr.Send(stationA, makeVoiceFrame(0), 0)
	tr.Run()

	if toR.From != stationA {
		t.Errorf("R did not receive a delivery; From = %d", toR.From)
	}
	if len(toB.Raw) > 0 && toB.Raw[0] == 0xEE {
		t.Fatal("B's Raw shares storage with R's; defensive copy missing across links")
	}
}

// TestTopology_RejectsEnabledSelfLoop is Review Finding 6: an enabled
// self-loop (From == To) is rejected at construction time because it
// is nonsensical for the MVP relay topology and can cause an infinite
// send/deliver cycle when a receiver re-sends during Run.
func TestTopology_RejectsEnabledSelfLoop(t *testing.T) {
	bad := Topology{
		Links: []Link{
			{From: stationA, To: stationA, Enabled: true},
		},
	}
	if err := bad.Validate(); err == nil {
		t.Error("Validate accepted an enabled self-loop; want error")
	}
}

// TestTopology_AllowsDisabledSelfLoop verifies a disabled self-loop is
// tolerated (it delivers nothing and may appear in configs that
// reuse a generic template).
func TestTopology_AllowsDisabledSelfLoop(t *testing.T) {
	ok := Topology{
		Links: []Link{
			{From: stationA, To: stationA, Enabled: false},
		},
	}
	if err := ok.Validate(); err != nil {
		t.Errorf("Validate rejected a disabled self-loop: %v", err)
	}
}

// TestNewLocalTransport_PanicsOnInvalidTopology is Review Finding 6:
// NewLocalTransport panics on an invalid topology, surfacing the
// config error loudly (consistent with SimulatedClock.Advance).
func TestNewLocalTransport_PanicsOnInvalidTopology(t *testing.T) {
	bad := Topology{
		Links: []Link{
			{From: stationA, To: stationA, Enabled: true},
		},
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("NewLocalTransport with enabled self-loop did not panic")
		}
	}()
	NewLocalTransport(bad, sim.NewEventQueue(), 1)
}

// TestRun_StopsAtNonFrameEvent is Review Finding 1: Run must stop at
// the first non-frame_delivery event and leave it in the queue for
// the simulator driver, not skip or discard it.
func TestRun_StopsAtNonFrameEvent(t *testing.T) {
	top := Topology{
		Links: []Link{
			{From: stationA, To: stationR, Enabled: true, FixedDelayMs: 10},
		},
	}
	q := sim.NewEventQueue()
	tr := NewLocalTransport(top, q, 42)
	tr.Register(stationR, func(d Delivery) {})

	// Schedule a source_tick at t=5, BEFORE the frame_delivery that
	// Send will schedule at t=0+10=10.
	q.Push(sim.Event{
		TimeMs: 5, EventPriority: sim.PrioritySourceTick, EventType: "source_tick",
		SourceStationID: 1, DestinationStationID: 0,
	})
	tr.Send(stationA, makeVoiceFrame(0), 0)

	tr.Run()

	// The source_tick must still be in the queue (Run stopped at it).
	if q.Len() == 0 {
		t.Fatal("Run drained the queue including the non-frame event; expected it to stop")
	}
	head, _ := q.Peek()
	if head.EventType != "source_tick" {
		t.Errorf("head event after Run = %q, want source_tick (Run must not consume non-frame events)", head.EventType)
	}
}

// TestTopology_RejectsZeroStationID is Review Finding 4 (extended):
// From=0 or To=0 is rejected because numeric_id 0 is reserved for
// relay_id=0 direct/source frames (frame-contract.md §4.3).
func TestTopology_RejectsZeroStationID(t *testing.T) {
	cases := []struct {
		name string
		link Link
	}{
		{"From=0", Link{From: 0, To: stationR, Enabled: true}},
		{"To=0", Link{From: stationA, To: 0, Enabled: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			top := Topology{Links: []Link{c.link}}
			if err := top.Validate(); err == nil {
				t.Errorf("Validate accepted link with %s; want error", c.name)
			}
		})
	}
}

// TestTopology_RejectsNegativeDelay is Review Finding 4: a negative
// fixed_delay_ms would schedule deliveries "in the past" and break
// the monotonic event timeline (§16).
func TestTopology_RejectsNegativeDelay(t *testing.T) {
	top := Topology{
		Links: []Link{
			{From: stationA, To: stationR, Enabled: true, FixedDelayMs: -10},
		},
	}
	if err := top.Validate(); err == nil {
		t.Error("Validate accepted fixed_delay_ms=-10; want error")
	}
}

// TestTopology_RejectsNegativeJitter verifies negative jitter is
// rejected.
func TestTopology_RejectsNegativeJitter(t *testing.T) {
	top := Topology{
		Links: []Link{
			{From: stationA, To: stationR, Enabled: true, JitterMs: -5},
		},
	}
	if err := top.Validate(); err == nil {
		t.Error("Validate accepted jitter_ms=-5; want error")
	}
}

// TestTopology_RejectsOutOfRangePct verifies loss/duplicate/corruption
// percentages outside [0, 100] are rejected, including NaN.
func TestTopology_RejectsOutOfRangePct(t *testing.T) {
	cases := []struct {
		name string
		link Link
	}{
		{"loss_pct=-1", Link{From: stationA, To: stationR, Enabled: true, LossPct: -1}},
		{"loss_pct=101", Link{From: stationA, To: stationR, Enabled: true, LossPct: 101}},
		{"loss_pct=NaN", Link{From: stationA, To: stationR, Enabled: true, LossPct: math.NaN()}},
		{"duplicate_pct=-5", Link{From: stationA, To: stationR, Enabled: true, DuplicatePct: -5}},
		{"corruption_pct=150", Link{From: stationA, To: stationR, Enabled: true, CorruptionPct: 150}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			top := Topology{Links: []Link{c.link}}
			if err := top.Validate(); err == nil {
				t.Errorf("Validate accepted %s; want error", c.name)
			}
		})
	}
}

// TestTopology_AcceptsValidBoundaries verifies boundary values 0 and
// 100 for percentages and 0 for delays are accepted (not just
// mid-range values).
func TestTopology_AcceptsValidBoundaries(t *testing.T) {
	top := Topology{
		Links: []Link{
			{From: stationA, To: stationR, Enabled: true, LossPct: 0, DuplicatePct: 100, CorruptionPct: 0, FixedDelayMs: 0, JitterMs: 0},
			{From: stationR, To: stationB, Enabled: true, LossPct: 100},
		},
	}
	if err := top.Validate(); err != nil {
		t.Errorf("Validate rejected valid boundary topology: %v", err)
	}
}

// TestCounters_IsSnapshot is Review Finding 5: Counters() must return
// a copy independent of the transport's internal state, so callers
// cannot mutate the transport's counters through the returned pointer.
func TestCounters_IsSnapshot(t *testing.T) {
	top := canonicalTopology()
	q := sim.NewEventQueue()
	tr := NewLocalTransport(top, q, 42)
	tr.Register(stationR, func(d Delivery) {})
	tr.Send(stationA, makeVoiceFrame(0), 0)
	tr.Run()

	snap := tr.Counters()
	delivered := snap.Delivered[linkKey(stationA, stationR)]
	if delivered == 0 {
		t.Fatal("expected at least one delivered frame before mutation test")
	}
	// Mutate the snapshot.
	snap.Delivered[linkKey(stationA, stationR)] = 999
	snap.Lost["bogus"] = 12345

	// The transport's internal counters must be unaffected.
	fresh := tr.Counters()
	if fresh.Delivered[linkKey(stationA, stationR)] == 999 {
		t.Error("Counters() returned a live pointer; mutating the snapshot changed the transport's internal state")
	}
	if _, ok := fresh.Lost["bogus"]; ok {
		t.Error("Counters() returned a live pointer; adding a Lost key in the snapshot leaked into the transport")
	}
}

// TestAC3_DifferentSeedDeterministicProperty is a deterministic
// companion to the probabilistic TestAC3_DifferentSeedProducesDiffer-
// entHash. It proves the seed is wired into the impairment model by
// asserting exact delivered counts for known seeds under a lossy link.
// This avoids relying on hash-uniqueness probability. The probabilistic
// test is retained alongside it as a broader guard with a documented
// vanishing collision probability.
func TestAC3_DifferentSeedDeterministicProperty(t *testing.T) {
	const n = 200
	top := Topology{
		Links: []Link{
			{From: stationA, To: stationR, Enabled: true, LossPct: 50},
		},
	}
	run := func(seed uint64) int {
		q := sim.NewEventQueue()
		tr := NewLocalTransport(top, q, seed)
		tr.Register(stationR, func(d Delivery) {})
		for seq := uint32(0); seq < n; seq++ {
			tr.Send(stationA, makeVoiceFrame(seq), int64(seq)*80)
		}
		tr.Run()
		return tr.Counters().Delivered[linkKey(stationA, stationR)]
	}
	d1 := run(1)
	d2 := run(2)
	const wantSeed1 = 105
	const wantSeed2 = 107
	if d1 != wantSeed1 {
		t.Errorf("seed 1 delivered %d/%d; want %d", d1, n, wantSeed1)
	}
	if d2 != wantSeed2 {
		t.Errorf("seed 2 delivered %d/%d; want %d", d2, n, wantSeed2)
	}
}
