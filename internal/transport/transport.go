// Package transport provides the transport abstraction used by the
// simulator per ARCHITECTURE-SPINE.md AD-5 and
// algorithm-and-experiments.md §4.2 and §8.
//
// The initial implementation is a local in-process bus that delivers
// frames between stations according to a directional topology and
// per-link impairment model. Protocol and node logic run over this
// abstraction; baseband and RF transports are deferred adapters and
// must not be referenced by protocol or node code (AD-5).
package transport

import (
	"fmt"
	"math"
	"math/rand/v2"

	"github.com/rmsff/ad-hoc-radio/internal/protocol"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
)

// StationID is the numeric station identifier used by the transport
// (frame-contract.md §4.3: uint16). relay_id=0 is reserved for
// direct/source frames and must not be assigned to a Station.
type StationID uint16

// Link is a directed link between two stations per
// algorithm-and-experiments.md §8.
//
// Disabled links deliver no frames. Enabled links apply loss, delay,
// jitter, duplication, and corruption according to their percentages
// and the seeded PRNG provided to the transport.
type Link struct {
	From          StationID
	To            StationID
	Enabled       bool
	LossPct       float64
	FixedDelayMs  int64
	JitterMs      int64
	DuplicatePct  float64
	CorruptionPct float64
}

// Topology is the set of directed links governing delivery.
type Topology struct {
	Links []Link
}

// LinkFor returns the directed link from src to dst, or nil if none is
// configured.
func (t Topology) LinkFor(src, dst StationID) *Link {
	for i := range t.Links {
		if t.Links[i].From == src && t.Links[i].To == dst {
			return &t.Links[i]
		}
	}
	return nil
}

// Validate checks the topology for configuration errors that would
// break simulator invariants. It returns an error describing the first
// problem found.
//
// Rules (per link, when enabled):
//   - From and To must not be 0 (§4.3: numeric_id 0 is reserved for
//     relay_id=0 direct/source frames and must not be a station).
//   - From != To: enabled self-loops are nonsensical for the MVP relay
//     topology and can cause an infinite send/deliver cycle when a
//     receiver re-sends during Run. Disabled self-loops are tolerated.
//   - LossPct, DuplicatePct, CorruptionPct must be in [0, 100] and not
//     NaN. drawPct clamps, but an out-of-range value signals a
//     misconfiguration that should be surfaced, not silently clamped.
//   - FixedDelayMs and JitterMs must be >= 0. A negative delay would
//     schedule deliveries "in the past" and break the monotonic event
//     timeline (§16 ordering by time_ms ascending).
func (t Topology) Validate() error {
	for i, l := range t.Links {
		if !l.Enabled {
			continue
		}
		if l.From == 0 {
			return fmt.Errorf("topology: link %d has From=0, reserved for relay_id=0 direct frames (§4.3)", i)
		}
		if l.To == 0 {
			return fmt.Errorf("topology: link %d has To=0, reserved for relay_id=0 direct frames (§4.3)", i)
		}
		if l.From == l.To {
			return fmt.Errorf("topology: link %d is an enabled self-loop (%d->%d); self-loops are not supported", i, l.From, l.To)
		}
		if err := validatePct("loss_pct", l.LossPct); err != nil {
			return fmt.Errorf("topology: link %d %w", i, err)
		}
		if err := validatePct("duplicate_pct", l.DuplicatePct); err != nil {
			return fmt.Errorf("topology: link %d %w", i, err)
		}
		if err := validatePct("corruption_pct", l.CorruptionPct); err != nil {
			return fmt.Errorf("topology: link %d %w", i, err)
		}
		if l.FixedDelayMs < 0 {
			return fmt.Errorf("topology: link %d has fixed_delay_ms=%d, must be >= 0", i, l.FixedDelayMs)
		}
		if l.JitterMs < 0 {
			return fmt.Errorf("topology: link %d has jitter_ms=%d, must be >= 0", i, l.JitterMs)
		}
	}
	return nil
}

// validatePct checks that a percentage field is a finite number in
// [0, 100]. NaN and Inf are rejected because they signal a
// misconfiguration that drawPct's clamping would silently hide.
func validatePct(name string, v float64) error {
	if math.IsNaN(v) {
		return fmt.Errorf("%s is NaN", name)
	}
	if math.IsInf(v, 0) {
		return fmt.Errorf("%s is Inf", name)
	}
	if v < 0 || v > 100 {
		return fmt.Errorf("%s=%g is out of range [0, 100]", name, v)
	}
	return nil
}

// DeliveryCounters records how many frames each (from,to) link
// delivered, lost, corrupted, or duplicated during a scenario. Keys
// are "from->to" strings for metrics-friendly reporting.
type DeliveryCounters struct {
	Delivered  map[string]int
	Lost       map[string]int
	Corrupted  map[string]int
	Duplicated map[string]int
}

func newDeliveryCounters() *DeliveryCounters {
	return &DeliveryCounters{
		Delivered:  make(map[string]int),
		Lost:       make(map[string]int),
		Corrupted:  make(map[string]int),
		Duplicated: make(map[string]int),
	}
}

func linkKey(from, to StationID) string {
	return fmt.Sprintf("%d->%d", from, to)
}

// Delivery is a single delivered frame event surfaced by the transport
// to a receiver callback. Raw is the (possibly corrupted) encoded
// frame bytes owned exclusively by this Delivery (the transport makes
// a defensive copy per delivery so receivers may mutate Raw without
// affecting siblings or duplicates). ReceivedAt is the simulated
// arrival time in ms.
//
// When Corrupted is true or Decode(Raw) fails, Frame is the zero
// value and the receiver should record an "invalid" drop.
//
// IsDuplicate is true for the second delivery of a duplicated frame;
// OriginalEventID references the original delivery's event_id (use
// IsDuplicate to distinguish, since event_id 0 is a valid id).
type Delivery struct {
	From            StationID
	To              StationID
	Frame           protocol.Frame
	Raw             []byte
	ReceivedAt      int64
	Corrupted       bool
	IsDuplicate     bool
	OriginalEventID uint64
}

// Receiver is called by the transport when a frame is delivered to a
// station. Implementations record the delivery, drive a node state
// machine, or collect test evidence.
type Receiver func(d Delivery)

// Transport is the abstract transport over which protocol and node
// logic run (AD-5).
type Transport interface {
	// Send broadcasts f from src to all enabled next hops according to
	// the topology and link impairment model. Delivery happens via the
	// registered receivers when Run is called.
	Send(src StationID, f protocol.Frame, nowMs int64)
	// Register associates a receiver with a station id.
	Register(id StationID, r Receiver)
	// Counters returns a snapshot of per-link delivery counters.
	Counters() *DeliveryCounters
}

// deterministicDuplicateDelayMs is the fixed offset between an original
// delivery and its duplicate (§8: deterministic_duplicate_delay). MVP
// pins it to a small constant; a later ADR may make it configurable.
const deterministicDuplicateDelayMs int64 = 5

// LocalTransport is the in-process implementation of Transport.
//
// It is deterministic given the same topology and seed (NFR-1): all
// randomness flows from a single math/rand/v2 PCG PRNG. Delivery is
// scheduled on the provided event queue and dispatched in §16 order
// when Run is called.
type LocalTransport struct {
	top       Topology
	queue     *sim.EventQueue
	rng       *rand.Rand
	seed      uint64
	receivers map[StationID]Receiver
	counters  *DeliveryCounters
	rawStash  map[uint64][]byte // event_id -> exclusive encoded bytes, for Run to retrieve
}

// NewLocalTransport builds a local transport over top, driving
// deliveries onto queue using a PRNG seeded with seed. It panics if
// top is invalid (enabled self-loops); topology is config-time input
// and an invalid one is a programmer error, surfaced loudly to match
// SimulatedClock.Advance's panic-on-misuse convention.
func NewLocalTransport(top Topology, queue *sim.EventQueue, seed uint64) *LocalTransport {
	if err := top.Validate(); err != nil {
		panic(fmt.Sprintf("transport: invalid topology: %v", err))
	}
	return &LocalTransport{
		top:       top,
		queue:     queue,
		rng:       rand.New(rand.NewPCG(seed, seed)),
		seed:      seed,
		receivers: make(map[StationID]Receiver),
		counters:  newDeliveryCounters(),
		rawStash:  make(map[uint64][]byte),
	}
}

// Seed returns the PRNG seed, useful for determinism logging.
func (t *LocalTransport) Seed() uint64 { return t.seed }

// Register associates r with id. Sending to id will invoke r on
// delivery.
func (t *LocalTransport) Register(id StationID, r Receiver) {
	t.receivers[id] = r
}

// Counters returns a snapshot copy of per-link delivery counters. The
// returned value is independent of the transport's internal state, so
// callers may inspect or retain it without risk of mutation by later
// transport activity.
func (t *LocalTransport) Counters() *DeliveryCounters {
	return &DeliveryCounters{
		Delivered:  copyIntMap(t.counters.Delivered),
		Lost:       copyIntMap(t.counters.Lost),
		Corrupted:  copyIntMap(t.counters.Corrupted),
		Duplicated: copyIntMap(t.counters.Duplicated),
	}
}

func copyIntMap(m map[string]int) map[string]int {
	if m == nil {
		return nil
	}
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// Send broadcasts f from src to all enabled links where link.From ==
// src, applying the §8 impairment model.
//
// Loss, jitter, duplication, and corruption draws are deterministic
// given the seed. Delivery is scheduled on the event queue and
// physically dispatched by Run. Each scheduled delivery gets its own
// defensive copy of the encoded bytes, so receivers may mutate
// Delivery.Raw without affecting siblings or duplicates.
func (t *LocalTransport) Send(src StationID, f protocol.Frame, nowMs int64) {
	encoded, err := f.Encode()
	if err != nil {
		// A sender producing an unencodable frame is a programmer error
		// in the simulator; surface it loudly rather than silently
		// dropping.
		panic(fmt.Sprintf("transport: encode failed for send from %d: %v", src, err))
	}

	for i := range t.top.Links {
		link := &t.top.Links[i]
		if link.From != src || !link.Enabled {
			continue
		}

		// Loss draw (§8).
		if t.drawPct(link.LossPct) {
			t.counters.Lost[linkKey(link.From, link.To)]++
			continue
		}

		// Corruption draw (§8): flip a deterministic payload byte so
		// the receiver's Decode fails (exercises Story 1.2 UT-002 path).
		corrupted := false
		deliveryBytes := encoded
		if t.drawPct(link.CorruptionPct) {
			corrupted = true
			t.counters.Corrupted[linkKey(link.From, link.To)]++
			deliveryBytes = append([]byte(nil), encoded...)
			t.corrupt(deliveryBytes)
		}

		// Delay + jitter (§8), overflow-safe.
		jitter := t.drawJitter(link.JitterMs)
		delay := sim.AddMs(link.FixedDelayMs, jitter)
		arrival := sim.AddMs(nowMs, delay)

		ok := &sim.OriginKey{
			NetID:    f.NetID,
			SourceID: f.SourceID,
			CallID:   f.CallID,
			Seq:      f.Seq,
		}
		ck := &sim.CopyKey{
			NetID:    f.NetID,
			SourceID: f.SourceID,
			RelayID:  f.RelayID,
			CallID:   f.CallID,
			Seq:      f.Seq,
			Hop:      f.Hop,
		}
		evID := t.scheduleDelivery(link, arrival, deliveryBytes, corrupted, false, 0, ok, ck)
		t.counters.Delivered[linkKey(link.From, link.To)]++

		// Duplication draw (§8): schedule a second delivery with a
		// small deterministic offset, flagged as a duplicate of evID.
		if t.drawPct(link.DuplicatePct) {
			t.counters.Duplicated[linkKey(link.From, link.To)]++
			dupAt := sim.AddMs(arrival, deterministicDuplicateDelayMs)
			t.scheduleDelivery(link, dupAt, deliveryBytes, corrupted, true, evID, ok, ck)
			t.counters.Delivered[linkKey(link.From, link.To)]++
		}
	}
}

// scheduleDelivery enqueues a frame delivery event, stashes an
// EXCLUSIVE copy of the raw bytes for Run to retrieve, and returns the
// assigned event_id. isDuplicate is false for the original delivery
// and true for a scheduled duplicate; originalID is the event_id of
// the original delivery (0 is a valid id, so use IsDuplicate to
// distinguish).
//
// The bytes are copied here so each event owns an independent slice;
// a receiver mutating Delivery.Raw cannot affect a sibling or a
// duplicate delivery that shares the same source buffer.
func (t *LocalTransport) scheduleDelivery(link *Link, atMs int64, bytes []byte, corrupted, isDuplicate bool, originalID uint64, ok *sim.OriginKey, ck *sim.CopyKey) uint64 {
	ev := sim.Event{
		TimeMs:               atMs,
		EventPriority:        sim.PriorityFrameDelivery,
		EventType:            "frame_delivery",
		SourceStationID:      uint16(link.From),
		DestinationStationID: uint16(link.To),
		OriginKey:            ok,
		CopyKey:              ck,
		Delivery: &sim.FrameDeliveryMeta{
			Corrupted:       corrupted,
			IsDuplicate:     isDuplicate,
			OriginalEventID: originalID,
			PayloadLen:      len(bytes),
		},
	}
	t.queue.Push(ev)
	id := t.queue.LastPushedID()
	// Defensive copy per event so receivers cannot mutate shared state.
	exclusive := make([]byte, len(bytes))
	copy(exclusive, bytes)
	t.rawStash[id] = exclusive
	return id
}

// Run drains frame_delivery events from the HEAD of the shared event
// queue, dispatching each to its registered receiver in §16 order. It
// STOPS (returns) without consuming when the next event is NOT a
// frame_delivery — the simulator driver owns slot_boundary,
// source_tick, jitter_tick, and metrics_flush events and must process
// them between transport runs. Run must never skip or discard
// non-frame events.
//
// For transport-only test scenarios where the queue contains only
// frame_delivery events, Run drains the queue completely. For mixed
// scenarios, call Run after each non-frame event is handled by the
// driver; Run will process any frame_delivery events that became
// ready and yield back at the next non-frame event.
func (t *LocalTransport) Run() {
	for {
		ev, ok := t.queue.Peek()
		if !ok {
			return
		}
		if ev.EventType != "frame_delivery" {
			// Leave non-frame events in the queue for the simulator
			// driver. Do not Pop, do not skip.
			return
		}
		t.queue.Pop()
		dst := StationID(ev.DestinationStationID)
		r, exists := t.receivers[dst]
		raw := t.rawStash[ev.EventID]
		delete(t.rawStash, ev.EventID)
		if !exists {
			// No receiver: drop the stash entry (already done) and move
			// on; the event is still recorded in queue.Emitted for the
			// event hash.
			continue
		}

		d := Delivery{
			From:       StationID(ev.SourceStationID),
			To:         dst,
			Raw:        raw,
			ReceivedAt: ev.TimeMs,
		}
		if ev.Delivery != nil {
			d.Corrupted = ev.Delivery.Corrupted
			d.IsDuplicate = ev.Delivery.IsDuplicate
			d.OriginalEventID = ev.Delivery.OriginalEventID
		}
		if decoded, err := protocol.Decode(raw); err == nil {
			d.Frame = decoded
		}
		r(d)
	}
}

// drawPct returns true with probability pct/100 using the transport's
// PRNG. pct <= 0 never triggers; pct >= 100 always triggers.
func (t *LocalTransport) drawPct(pct float64) bool {
	if pct <= 0 {
		return false
	}
	if pct >= 100 {
		return true
	}
	return t.rng.Float64()*100 < pct
}

// drawJitter returns a non-negative jitter in ms drawn uniformly from
// [0, jitterMs] (§8: random_jitter). jitterMs <= 0 yields 0. jitterMs
// at or above math.MaxInt64 is rejected: jitterMs+1 would overflow
// and Int64N would panic on a non-positive bound, so the misconfigured
// link is surfaced loudly instead.
func (t *LocalTransport) drawJitter(jitterMs int64) int64 {
	if jitterMs <= 0 {
		return 0
	}
	if jitterMs >= math.MaxInt64 {
		panic(fmt.Sprintf("transport: jitter_ms %d too large (>= MaxInt64); misconfigured link", jitterMs))
	}
	return t.rng.Int64N(jitterMs + 1)
}

// corrupt mutates b in place so a subsequent protocol.Decode fails.
// It flips the first payload byte (header is protocol.HeaderSize bytes
// per §4.2); if there is no payload, it flips the last header byte.
func (t *LocalTransport) corrupt(b []byte) {
	if len(b) > protocol.HeaderSize {
		b[protocol.HeaderSize] ^= 0xFF
	} else if len(b) > 0 {
		b[len(b)-1] ^= 0xFF
	}
}
