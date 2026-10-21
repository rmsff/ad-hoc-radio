package sim

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// Event priorities per frame-contract.md §16.
const (
	PrioritySlotBoundary      = 10
	PriorityScheduleUpdate    = 15
	PrioritySourceTick        = 20
	PriorityFrameDelivery     = 30
	PriorityScheduledTransmit = 35
	PriorityJitterTick        = 40
	PriorityMetricsFlush      = 50
)

// FrameDeliveryMeta carries frame_delivery-specific metadata for a
// frame_delivery event. Event metadata is statically typed: the project
// forbids the any type and empty interfaces.
//
// For non-frame_delivery events this pointer is nil; future event
// types add their own optional metadata fields to Event.
type FrameDeliveryMeta struct {
	Corrupted       bool
	IsDuplicate     bool
	OriginalEventID uint64
	PayloadLen      int
}

// Event is a single simulation event in the deterministic event queue.
//
// Ordering follows frame-contract.md §16:
//  1. time_ms ascending
//  2. event_priority ascending
//  3. source_station_id ascending
//  4. destination_station_id ascending
//  5. event_id ascending (monotonically increasing)
//
// Fields used for the canonical JSONL event hash (§17) must include at
// least time_ms, event_priority, event_id, event_type,
// source_station_id, destination_station_id, and relevant
// origin_key/copy_key where applicable. Delivery metadata is carried
// by the typed Delivery field for frame_delivery events.
type Event struct {
	TimeMs               int64
	EventPriority        int
	EventID              uint64
	EventType            string
	SourceStationID      uint16
	DestinationStationID uint16
	OriginKey            *OriginKey
	CopyKey              *CopyKey
	Delivery             *FrameDeliveryMeta
}

// OriginKey and CopyKey mirror protocol.OriginKey / protocol.CopyKey
// but live in the sim package to keep the event stream serializable
// without an import cycle. They are populated from a protocol.Frame by
// the transport layer.
type OriginKey struct {
	NetID    uint16
	SourceID uint16
	CallID   uint32
	Seq      uint32
}

type CopyKey struct {
	NetID    uint16
	SourceID uint16
	RelayID  uint16
	CallID   uint32
	Seq      uint32
	Hop      uint8
}

// less reports whether event a should be processed before event b per
// the §16 tie-breaker chain.
func (a Event) less(b Event) bool {
	if a.TimeMs != b.TimeMs {
		return a.TimeMs < b.TimeMs
	}
	if a.EventPriority != b.EventPriority {
		return a.EventPriority < b.EventPriority
	}
	if a.SourceStationID != b.SourceStationID {
		return a.SourceStationID < b.SourceStationID
	}
	if a.DestinationStationID != b.DestinationStationID {
		return a.DestinationStationID < b.DestinationStationID
	}
	return a.EventID < b.EventID
}

// EventQueue is a deterministic priority queue of simulation events.
//
// Push assigns a monotonically increasing event_id to each event;
// Pop returns the next event per the §16 ordering. The queue is not
// safe for concurrent use — the MVP simulator is single-threaded.
//
// The queue is a SHARED simulator queue: it carries frame_delivery
// events (owned by the transport) AND non-frame events such as
// slot_boundary, source_tick, jitter_tick, and metrics_flush (owned by
// the simulator driver). Transport.Run only consumes frame_delivery
// events and stops at the first non-frame event so the driver can
// process it; it must not skip or discard non-frame events.
type EventQueue struct {
	events  []Event
	nextID  uint64
	emitted []Event
}

// NewEventQueue returns an empty queue.
func NewEventQueue() *EventQueue {
	return &EventQueue{}
}

// Push enqueues e, assigning it the next monotonically increasing
// event_id. The caller-supplied EventID is overwritten.
func (q *EventQueue) Push(e Event) {
	e.EventID = q.nextID
	q.nextID++
	q.events = append(q.events, e)
}

// Pop removes and returns the next event per §16 ordering. It returns
// (Event{}, false) when the queue is empty.
func (q *EventQueue) Pop() (Event, bool) {
	if len(q.events) == 0 {
		return Event{}, false
	}
	// Find the minimum per the §16 tie-breaker chain. A linear scan is
	// fine for MVP scenario sizes; a heap can replace it later if
	// scenarios grow large.
	minIdx := 0
	for i := 1; i < len(q.events); i++ {
		if q.events[i].less(q.events[minIdx]) {
			minIdx = i
		}
	}
	e := q.events[minIdx]
	q.events = append(q.events[:minIdx], q.events[minIdx+1:]...)
	q.emitted = append(q.emitted, e)
	return e, true
}

// Peek returns the next event per §16 ordering WITHOUT removing it.
// It returns (Event{}, false) when the queue is empty. Peek lets a
// caller inspect the head of the queue and decide whether to Pop
// (e.g. transport.Run peeks and stops when a non-frame_delivery event
// is next, so the simulator driver can process it).
func (q *EventQueue) Peek() (Event, bool) {
	if len(q.events) == 0 {
		return Event{}, false
	}
	minIdx := 0
	for i := 1; i < len(q.events); i++ {
		if q.events[i].less(q.events[minIdx]) {
			minIdx = i
		}
	}
	return q.events[minIdx], true
}

// Len returns the number of events currently enqueued.
func (q *EventQueue) Len() int { return len(q.events) }

// Emitted returns the events that have been popped so far, in the
// order they were popped (deterministic emission order).
func (q *EventQueue) Emitted() []Event { return q.emitted }

// Reset clears queued and emitted events and resets the event_id
// counter. Useful for re-running a scenario with the same seed.
func (q *EventQueue) Reset() {
	q.events = nil
	q.emitted = nil
	q.nextID = 0
}

// LastPushedID returns the event_id assigned to the most recently
// Pushed event, or 0 if nothing has been pushed. Useful for linking a
// duplicate delivery to its original event.
func (q *EventQueue) LastPushedID() uint64 {
	if q.nextID == 0 {
		return 0
	}
	return q.nextID - 1
}

// EventHash computes the canonical JSONL event hash per
// frame-contract.md §17: SHA-256 over the full canonical JSONL byte
// stream of the emitted events.
//
// Canonical JSONL rules (§17):
//   - one event per line;
//   - UTF-8;
//   - LF line ending;
//   - JSON object keys sorted lexicographically (including nested
//     objects such as origin_key and copy_key);
//   - no insignificant whitespace;
//   - events emitted after deterministic event ordering.
//
// The returned hash is lowercase hex SHA-256.
func (q *EventQueue) EventHash() (string, error) {
	var buf bytes.Buffer
	for i, e := range q.emitted {
		obj, err := canonicalEventJSON(e)
		if err != nil {
			return "", fmt.Errorf("event %d canonical encode: %w", i, err)
		}
		buf.Write(obj)
		buf.WriteByte('\n')
	}
	sum := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(sum[:]), nil
}

// canonicalEvent is the §17 canonical JSON object of an Event.
//
// json.Marshal emits struct fields in declaration order, so the fields
// below (and in the nested key structs) are declared in lexicographic
// order of their JSON keys. Optional fields are pointers with omitempty:
// omitempty drops only a nil pointer, so a set zero or false value is
// still emitted. Delivery metadata is flattened into the top-level
// object so the event hash stays stable across the Extra→Delivery
// migration.
type canonicalEvent struct {
	CopyKey              *canonicalCopyKey   `json:"copy_key,omitempty"`
	Corrupted            *bool               `json:"corrupted,omitempty"`
	DestinationStationID uint16              `json:"destination_station_id"`
	EventID              uint64              `json:"event_id"`
	EventPriority        int                 `json:"event_priority"`
	EventType            string              `json:"event_type"`
	IsDuplicate          *bool               `json:"is_duplicate,omitempty"`
	OriginKey            *canonicalOriginKey `json:"origin_key,omitempty"`
	OriginalEventID      *uint64             `json:"original_event_id,omitempty"`
	PayloadLen           *int                `json:"payload_len,omitempty"`
	SourceStationID      uint16              `json:"source_station_id"`
	TimeMs               int64               `json:"time_ms"`
}

// canonicalOriginKey is the §17 canonical JSON object of an OriginKey,
// fields in lexicographic order of their JSON keys.
type canonicalOriginKey struct {
	CallID   uint32 `json:"call_id"`
	NetID    uint16 `json:"net_id"`
	Seq      uint32 `json:"seq"`
	SourceID uint16 `json:"source_id"`
}

// canonicalCopyKey is the §17 canonical JSON object of a CopyKey,
// fields in lexicographic order of their JSON keys.
type canonicalCopyKey struct {
	CallID   uint32 `json:"call_id"`
	Hop      uint8  `json:"hop"`
	NetID    uint16 `json:"net_id"`
	RelayID  uint16 `json:"relay_id"`
	Seq      uint32 `json:"seq"`
	SourceID uint16 `json:"source_id"`
}

// canonicalEventJSON marshals e to deterministic JSON with ALL keys
// sorted lexicographically (including nested origin_key/copy_key) and
// no insignificant whitespace, per frame-contract.md §17. Key order
// comes from the field declaration order of canonicalEvent and the
// nested key structs.
func canonicalEventJSON(e Event) ([]byte, error) {
	c := canonicalEvent{
		DestinationStationID: e.DestinationStationID,
		EventID:              e.EventID,
		EventPriority:        e.EventPriority,
		EventType:            e.EventType,
		SourceStationID:      e.SourceStationID,
		TimeMs:               e.TimeMs,
	}
	if origin := e.OriginKey; origin != nil {
		c.OriginKey = &canonicalOriginKey{
			CallID:   origin.CallID,
			NetID:    origin.NetID,
			Seq:      origin.Seq,
			SourceID: origin.SourceID,
		}
	}
	if copyKey := e.CopyKey; copyKey != nil {
		c.CopyKey = &canonicalCopyKey{
			CallID:   copyKey.CallID,
			Hop:      copyKey.Hop,
			NetID:    copyKey.NetID,
			RelayID:  copyKey.RelayID,
			Seq:      copyKey.Seq,
			SourceID: copyKey.SourceID,
		}
	}
	if delivery := e.Delivery; delivery != nil {
		c.Corrupted = &delivery.Corrupted
		c.IsDuplicate = &delivery.IsDuplicate
		c.OriginalEventID = &delivery.OriginalEventID
		c.PayloadLen = &delivery.PayloadLen
	}
	return json.Marshal(c)
}

// SortEvents sorts a slice of events in-place per the §16 ordering.
// Useful when a caller has a batch of events and wants them ordered
// without using the queue's Push/Pop machinery.
func SortEvents(es []Event) {
	sort.SliceStable(es, func(i, j int) bool {
		return es[i].less(es[j])
	})
}
