package node

import (
	"fmt"
	"math"
	"reflect"
	"sort"

	"github.com/rmsff/ad-hoc-radio/internal/protocol"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
)

// DefaultCallEndCleanupGraceMs is the MVP cache cleanup grace from
// frame-contract.md §13.
const DefaultCallEndCleanupGraceMs int64 = 2_000

// DefaultRelaySeenCacheMaxEntries is the MVP relay seen-cache bound
// from frame-contract.md §13.
const DefaultRelaySeenCacheMaxEntries = 4096

// LifecyclePolicy controls deterministic lifecycle cleanup. The
// caller supplies simulated time; this package never reads wall-clock
// time or starts timers.
type LifecyclePolicy struct {
	CallEndCleanupGraceMs    int64
	RelaySeenCacheMaxEntries int
}

// DefaultLifecyclePolicy returns the frame-contract.md §13 cleanup
// policy.
func DefaultLifecyclePolicy() LifecyclePolicy {
	return LifecyclePolicy{
		CallEndCleanupGraceMs:    DefaultCallEndCleanupGraceMs,
		RelaySeenCacheMaxEntries: DefaultRelaySeenCacheMaxEntries,
	}
}

func normalizeLifecyclePolicy(p LifecyclePolicy) LifecyclePolicy {
	if p.CallEndCleanupGraceMs == 0 {
		p.CallEndCleanupGraceMs = DefaultCallEndCleanupGraceMs
	}
	if p.RelaySeenCacheMaxEntries == 0 {
		p.RelaySeenCacheMaxEntries = DefaultRelaySeenCacheMaxEntries
	}
	if p.CallEndCleanupGraceMs < 0 {
		panic("node: call_end_cleanup_grace_ms must be >= 0")
	}
	if p.RelaySeenCacheMaxEntries < 0 {
		panic("node: relay_seen_cache_max_entries must be >= 0")
	}
	return p
}

// CallKey identifies one PTT-like call lifecycle. Per-frame seq is
// intentionally excluded; seq belongs to protocol.OriginKey.
type CallKey struct {
	NetID    uint16
	SourceID uint16
	CallID   uint32
}

// CallKeyFromFrame returns the lifecycle key for f.
func CallKeyFromFrame(f protocol.Frame) CallKey {
	return CallKey{NetID: f.NetID, SourceID: f.SourceID, CallID: f.CallID}
}

func callKeyFromOriginKey(k protocol.OriginKey) CallKey {
	return CallKey{NetID: k.NetID, SourceID: k.SourceID, CallID: k.CallID}
}

// CallState is the node-local lifecycle state for one call.
type CallState struct {
	Key         CallKey
	Active      bool
	CreatedAtMs int64
	StartedAtMs int64
	EndedAtMs   int64
	CleanupAtMs int64
	GroupID     uint16 // from the call_start frame envelope (§3)
	Start       protocol.CallStartPayload
	End         protocol.CallEndPayload
}

func (s CallState) copy() CallState {
	s.Start = copyCallStartPayload(s.Start)
	return s
}

func copyCallStartPayload(p protocol.CallStartPayload) protocol.CallStartPayload {
	out := p
	if p.ExpectedFrameCount != nil {
		v := *p.ExpectedFrameCount
		out.ExpectedFrameCount = &v
	}
	if p.SlotDurationMs != nil {
		v := *p.SlotDurationMs
		out.SlotDurationMs = &v
	}
	return out
}

// RelayLifecycle owns call state and seen-cache scope for a Relay
// Station. Forwarding behavior lands in Story 2.1; this type only
// initializes, ends, and cleans the per-call state Story 2.0 requires.
type RelayLifecycle struct {
	policy        LifecyclePolicy
	calls         map[CallKey]CallState
	seen          map[CallKey]map[protocol.OriginKey]struct{}
	seenOrder     map[CallKey][]protocol.OriginKey
	seenEvictions int
}

// NewRelayLifecycle creates empty relay lifecycle state.
func NewRelayLifecycle(policy LifecyclePolicy) *RelayLifecycle {
	return &RelayLifecycle{
		policy:    normalizeLifecyclePolicy(policy),
		calls:     make(map[CallKey]CallState),
		seen:      make(map[CallKey]map[protocol.OriginKey]struct{}),
		seenOrder: make(map[CallKey][]protocol.OriginKey),
	}
}

// ProcessCallStart initializes per-call relay state from a call_start
// control frame. Duplicate call_start for an already-active call is
// idempotent and does not clear seen-cache state.
func (r *RelayLifecycle) ProcessCallStart(f protocol.Frame, nowMs int64) error {
	key, payload, err := decodeCallStartFrame(f)
	if err != nil {
		return err
	}
	if createdAtOutOfRange(f.CreatedAtMs) {
		return fmt.Errorf("node: relay call_start created_at_ms=%d is outside 0..%d", f.CreatedAtMs, protocol.MaxSafeCreatedAtMs)
	}
	if existing, ok := r.calls[key]; ok && existing.Active {
		if existing.GroupID != f.GroupID {
			return fmt.Errorf("node: relay conflicting duplicate call_start for call %+v: group_id=%d, want %d", key, f.GroupID, existing.GroupID)
		}
		if !reflect.DeepEqual(existing.Start, payload) {
			return fmt.Errorf("node: relay conflicting duplicate call_start for call %+v", key)
		}
		return nil
	}
	if existing, ok := r.calls[key]; ok && !existing.Active {
		return fmt.Errorf("node: relay call_start for ended call %+v before cleanup", key)
	}
	r.calls[key] = CallState{
		Key:         key,
		Active:      true,
		CreatedAtMs: f.CreatedAtMs,
		StartedAtMs: nowMs,
		GroupID:     f.GroupID,
		Start:       payload,
	}
	if _, ok := r.seen[key]; !ok {
		r.seen[key] = make(map[protocol.OriginKey]struct{})
	}
	if _, ok := r.seenOrder[key]; !ok {
		r.seenOrder[key] = nil
	}
	return nil
}

// ProcessCallEnd marks a relay call ended and schedules deterministic
// seen-cache cleanup after the configured grace period.
func (r *RelayLifecycle) ProcessCallEnd(f protocol.Frame, nowMs int64) error {
	key, payload, err := decodeCallEndFrame(f)
	if err != nil {
		return err
	}
	state, ok := r.calls[key]
	if !ok {
		return fmt.Errorf("node: relay call_end for unknown call %+v", key)
	}
	if state.GroupID != 0 && f.GroupID != state.GroupID {
		return fmt.Errorf("node: relay call_end group_id=%d does not match active call group_id=%d for call %+v", f.GroupID, state.GroupID, key)
	}
	if !state.Active {
		return nil
	}
	if nowMs < 0 || nowMs > math.MaxInt64-r.policy.CallEndCleanupGraceMs {
		return fmt.Errorf("node: cleanup time overflows int64")
	}
	state.Active = false
	state.EndedAtMs = nowMs
	state.CleanupAtMs = sim.AddMs(nowMs, r.policy.CallEndCleanupGraceMs)
	state.End = payload
	r.calls[key] = state
	return nil
}

// MarkSeen records an origin_key in the relay seen-cache. The owning
// call must be active; otherwise a cache-only scope would have no
// lifecycle owner and could not be deterministically cleaned.
func (r *RelayLifecycle) MarkSeen(k protocol.OriginKey) error {
	callKey := callKeyFromOriginKey(k)
	state, ok := r.calls[callKey]
	if !ok || !state.Active {
		return fmt.Errorf("node: cannot mark seen for inactive or unknown call %+v", callKey)
	}
	if _, ok := r.seen[callKey]; !ok {
		r.seen[callKey] = make(map[protocol.OriginKey]struct{})
	}
	if _, exists := r.seen[callKey][k]; exists {
		return nil
	}
	if r.policy.RelaySeenCacheMaxEntries == 0 {
		r.seenEvictions++
		return nil
	}
	for len(r.seen[callKey]) >= r.policy.RelaySeenCacheMaxEntries {
		r.evictOldestSeen(callKey)
	}
	r.seen[callKey][k] = struct{}{}
	r.seenOrder[callKey] = append(r.seenOrder[callKey], k)
	return nil
}

func (r *RelayLifecycle) evictOldestSeen(callKey CallKey) {
	order := r.seenOrder[callKey]
	if len(order) == 0 {
		return
	}
	oldest := order[0]
	copy(order, order[1:])
	order = order[:len(order)-1]
	r.seenOrder[callKey] = order
	if _, ok := r.seen[callKey][oldest]; ok {
		delete(r.seen[callKey], oldest)
		r.seenEvictions++
	}
}

// Seen reports whether k is present in the relay seen-cache.
func (r *RelayLifecycle) Seen(k protocol.OriginKey) bool {
	callKey := callKeyFromOriginKey(k)
	entries, ok := r.seen[callKey]
	if !ok {
		return false
	}
	_, ok = entries[k]
	return ok
}

// SeenEntryCount returns the total number of origin_key entries still
// retained in the relay seen-cache across all calls.
func (r *RelayLifecycle) SeenEntryCount() int {
	total := 0
	for _, entries := range r.seen {
		total += len(entries)
	}
	return total
}

// SeenEvictionCount returns the number of relay seen-cache entries
// evicted because RelaySeenCacheMaxEntries was exceeded.
func (r *RelayLifecycle) SeenEvictionCount() int {
	return r.seenEvictions
}

// Call returns a copy of the relay call state.
func (r *RelayLifecycle) Call(key CallKey) (CallState, bool) {
	state, ok := r.calls[key]
	if !ok {
		return CallState{}, false
	}
	return state.copy(), true
}

// HasCall reports whether call state exists for key.
func (r *RelayLifecycle) HasCall(key CallKey) bool {
	_, ok := r.calls[key]
	return ok
}

// Cleanup evicts ended calls whose cleanup grace has elapsed, along
// with their per-call seen-cache entries.
func (r *RelayLifecycle) Cleanup(nowMs int64) {
	for key, state := range r.calls {
		if state.Active || state.CleanupAtMs == 0 || nowMs < state.CleanupAtMs {
			continue
		}
		delete(r.calls, key)
		delete(r.seen, key)
		delete(r.seenOrder, key)
	}
}

// ReceiverLifecycle owns call state, accepted-key cache scope, and
// deterministic jitter state for a Receiver Station. ReceiverStation
// layers duplicate suppression counters and playout metrics on top.
type ReceiverLifecycle struct {
	policy         LifecyclePolicy
	calls          map[CallKey]CallState
	accepted       map[CallKey]map[protocol.OriginKey]struct{}
	jitter         map[CallKey]map[uint32]jitterEntry
	lateSeqs       map[CallKey]map[uint32]struct{}
	nextPlayoutSeq map[CallKey]uint32
	flushedUpTo    map[CallKey]uint32
}

// NewReceiverLifecycle creates empty receiver lifecycle state.
func NewReceiverLifecycle(policy LifecyclePolicy) *ReceiverLifecycle {
	return &ReceiverLifecycle{
		policy:         normalizeLifecyclePolicy(policy),
		calls:          make(map[CallKey]CallState),
		accepted:       make(map[CallKey]map[protocol.OriginKey]struct{}),
		jitter:         make(map[CallKey]map[uint32]jitterEntry),
		lateSeqs:       make(map[CallKey]map[uint32]struct{}),
		nextPlayoutSeq: make(map[CallKey]uint32),
		flushedUpTo:    make(map[CallKey]uint32),
	}
}

// ProcessCallStart initializes per-call receiver state. Duplicate
// call_start for an already-active call is idempotent and does not
// clear accepted-key or jitter state.
func (r *ReceiverLifecycle) ProcessCallStart(f protocol.Frame, nowMs int64) error {
	key, payload, err := decodeCallStartFrame(f)
	if err != nil {
		return err
	}
	if createdAtOutOfRange(f.CreatedAtMs) {
		return fmt.Errorf("node: receiver call_start created_at_ms=%d is outside 0..%d", f.CreatedAtMs, protocol.MaxSafeCreatedAtMs)
	}
	if existing, ok := r.calls[key]; ok && existing.Active {
		if existing.GroupID != f.GroupID {
			return fmt.Errorf("node: receiver conflicting duplicate call_start for call %+v: group_id=%d, want %d", key, f.GroupID, existing.GroupID)
		}
		if !reflect.DeepEqual(existing.Start, payload) {
			return fmt.Errorf("node: receiver conflicting duplicate call_start for call %+v", key)
		}
		return nil
	}
	if existing, ok := r.calls[key]; ok && !existing.Active {
		return fmt.Errorf("node: receiver call_start for ended call %+v before cleanup", key)
	}
	r.calls[key] = CallState{
		Key:         key,
		Active:      true,
		CreatedAtMs: f.CreatedAtMs,
		StartedAtMs: nowMs,
		GroupID:     f.GroupID,
		Start:       payload,
	}
	if _, ok := r.accepted[key]; !ok {
		r.accepted[key] = make(map[protocol.OriginKey]struct{})
	}
	if _, ok := r.jitter[key]; !ok {
		r.jitter[key] = make(map[uint32]jitterEntry)
	}
	if _, ok := r.lateSeqs[key]; !ok {
		r.lateSeqs[key] = make(map[uint32]struct{})
	}
	if _, ok := r.nextPlayoutSeq[key]; !ok {
		r.nextPlayoutSeq[key] = 0
	}
	return nil
}

// ProcessCallEnd finalizes receiver jitter state through last_seq, marks
// the call ended, and schedules deterministic cache cleanup after the
// configured grace period.
func (r *ReceiverLifecycle) ProcessCallEnd(f protocol.Frame, nowMs int64) (JitterEvaluation, error) {
	key, payload, err := decodeCallEndFrame(f)
	if err != nil {
		return JitterEvaluation{}, err
	}
	state, ok := r.calls[key]
	if !ok {
		return JitterEvaluation{}, fmt.Errorf("node: receiver call_end for unknown call %+v", key)
	}
	if state.GroupID != 0 && f.GroupID != state.GroupID {
		return JitterEvaluation{}, fmt.Errorf("node: receiver call_end group_id=%d does not match active call group_id=%d for call %+v", f.GroupID, state.GroupID, key)
	}
	if !state.Active {
		return JitterEvaluation{}, nil
	}
	if nowMs < 0 || nowMs > math.MaxInt64-r.policy.CallEndCleanupGraceMs {
		return JitterEvaluation{}, fmt.Errorf("node: cleanup time overflows int64")
	}
	eval := r.finalizeCallJitterThroughLastSeq(key, payload.LastSeq)
	state.Active = false
	state.EndedAtMs = nowMs
	state.CleanupAtMs = sim.AddMs(nowMs, r.policy.CallEndCleanupGraceMs)
	state.End = payload
	r.calls[key] = state
	r.flushJitterUpTo(key, payload.LastSeq)
	return eval, nil
}

func (r *ReceiverLifecycle) flushJitterUpTo(key CallKey, lastSeq uint32) {
	frames := r.jitter[key]
	for seq := range frames {
		if seq <= lastSeq {
			delete(frames, seq)
		}
	}
	lateSeqs := r.lateSeqs[key]
	for seq := range lateSeqs {
		if seq <= lastSeq {
			delete(lateSeqs, seq)
		}
	}
	if next, ok := r.nextPlayoutSeq[key]; !ok || next <= lastSeq {
		if lastSeq == ^uint32(0) {
			r.nextPlayoutSeq[key] = lastSeq
		} else {
			r.nextPlayoutSeq[key] = lastSeq + 1
		}
	}
	r.flushedUpTo[key] = lastSeq
}

// MarkAccepted records an origin_key in the receiver accepted-key
// cache. Story 2.2 will use this cache to suppress duplicates. The
// owning call must be active; otherwise a cache-only scope would have
// no lifecycle owner and could not be deterministically cleaned.
func (r *ReceiverLifecycle) MarkAccepted(k protocol.OriginKey) error {
	callKey := callKeyFromOriginKey(k)
	state, ok := r.calls[callKey]
	if !ok || !state.Active {
		return fmt.Errorf("node: cannot mark accepted for inactive or unknown call %+v", callKey)
	}
	if _, ok := r.accepted[callKey]; !ok {
		r.accepted[callKey] = make(map[protocol.OriginKey]struct{})
	}
	r.accepted[callKey][k] = struct{}{}
	return nil
}

// Accepted reports whether k is present in the receiver accepted-key
// cache.
func (r *ReceiverLifecycle) Accepted(k protocol.OriginKey) bool {
	callKey := callKeyFromOriginKey(k)
	entries, ok := r.accepted[callKey]
	if !ok {
		return false
	}
	_, ok = entries[k]
	return ok
}

// BufferedSeqs returns sorted sequence numbers still present in the
// receiver's minimal jitter state.
func (r *ReceiverLifecycle) BufferedSeqs(key CallKey) []uint32 {
	frames := r.jitter[key]
	seqs := make([]uint32, 0, len(frames))
	for seq := range frames {
		seqs = append(seqs, seq)
	}
	sort.Slice(seqs, func(i, j int) bool { return seqs[i] < seqs[j] })
	return seqs
}

// FlushedUpTo returns the last_seq most recently finalized for key.
func (r *ReceiverLifecycle) FlushedUpTo(key CallKey) (uint32, bool) {
	seq, ok := r.flushedUpTo[key]
	return seq, ok
}

// Call returns a copy of the receiver call state.
func (r *ReceiverLifecycle) Call(key CallKey) (CallState, bool) {
	state, ok := r.calls[key]
	if !ok {
		return CallState{}, false
	}
	return state.copy(), true
}

// HasCall reports whether call state exists for key.
func (r *ReceiverLifecycle) HasCall(key CallKey) bool {
	_, ok := r.calls[key]
	return ok
}

// Cleanup evicts ended calls whose cleanup grace has elapsed, along
// with accepted-key, jitter, and flushed-finalization state.
func (r *ReceiverLifecycle) Cleanup(nowMs int64) {
	for key, state := range r.calls {
		if state.Active || state.CleanupAtMs == 0 || nowMs < state.CleanupAtMs {
			continue
		}
		delete(r.calls, key)
		delete(r.accepted, key)
		delete(r.jitter, key)
		delete(r.lateSeqs, key)
		delete(r.nextPlayoutSeq, key)
		delete(r.flushedUpTo, key)
	}
}
