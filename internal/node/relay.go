package node

import (
	"fmt"
	"math"

	"github.com/rmsff/ad-hoc-radio/internal/protocol"
)

// DefaultRelayStaleThresholdMs is the MVP relay stale-frame threshold
// from frame-contract.md §11.
const DefaultRelayStaleThresholdMs int64 = 500

// RelayPolicy controls relay forwarding behavior.
type RelayPolicy struct {
	// StaleThresholdMs: frames older than this (by created_at_ms) are
	// dropped as stale. Default 500 ms per §11.
	StaleThresholdMs int64
	// SubscribedGroups: group_ids this relay is subscribed to. A frame
	// whose group_id is not in this set is dropped as wrong_group.
	// Empty means "subscribe to all groups" (no group filtering).
	SubscribedGroups map[uint16]bool
}

// DefaultRelayPolicy returns the frame-contract.md defaults.
func DefaultRelayPolicy() RelayPolicy {
	return RelayPolicy{
		StaleThresholdMs: DefaultRelayStaleThresholdMs,
	}
}

func normalizeRelayPolicy(p RelayPolicy) RelayPolicy {
	if p.StaleThresholdMs == 0 {
		p.StaleThresholdMs = DefaultRelayStaleThresholdMs
	}
	if p.StaleThresholdMs < 0 {
		panic("node: relay stale_threshold_ms must be >= 0")
	}
	// Defensive copy of SubscribedGroups so the caller cannot mutate
	// the relay's policy by editing the map after construction.
	if p.SubscribedGroups != nil {
		cloned := make(map[uint16]bool, len(p.SubscribedGroups))
		for k, v := range p.SubscribedGroups {
			cloned[k] = v
		}
		p.SubscribedGroups = cloned
	}
	return p
}

// DropResult records why a frame was not forwarded.
type DropResult struct {
	Reason DropReason
	Frame  protocol.Frame
}

// ForwardResult is the outcome of ProcessVoiceFrame: either a forwarded
// copy or a drop.
//
// When Forwarded is non-nil, the forwarded Frame has its HeaderCRC32
// and PayloadCRC32 zeroed — they are stale from the original frame and
// will be recomputed by protocol.Frame.Encode() when the caller sends
// the frame. Callers must NOT read the CRC fields directly without
// calling Encode() first.
type ForwardResult struct {
	Forwarded *protocol.Frame
	Dropped   *DropResult
}

// RelayStation is the fixed-relay forwarding state machine. It wraps a
// RelayLifecycle (call state + seen-cache from Story 2.0) with the
// per-frame forwarding logic from algorithm-and-experiments.md §7.2
// and frame-contract.md §10, §11, §13.
//
// The relay does NOT send frames onto a transport. ProcessVoiceFrame
// returns a ForwardResult; the caller (Story 2.3 scenario runner) is
// responsible for wiring a forwarded frame to transport.Send.
//
// The relay does NOT decode or re-encode audio (AD-3, NFR-3). It
// operates on frame metadata and payload bytes only.
//
// RelayStation is NOT safe for concurrent use. The MVP simulator is
// single-threaded (SimulatedClock, event-driven). Add synchronization
// before any multi-goroutine use.
type RelayStation struct {
	id     uint16
	policy RelayPolicy
	life   *RelayLifecycle
	// dropCounters records how many frames were dropped per reason.
	// Unexported; use DropCounters() for a snapshot.
	dropCounters map[DropReason]int
}

// NewRelayStation creates a relay with station id, lifecycle, and
// policy. The lifecycle must already be initialized (Story 2.0);
// ProcessCallStart must have been called for any call whose voice
// frames this relay will process.
//
// Panics if id == 0: numeric_id 0 is reserved for relay_id=0
// direct/source frames (frame-contract.md §4.3) and a relay with id=0
// would produce relayed frames that fail validation downstream.
func NewRelayStation(id uint16, life *RelayLifecycle, policy RelayPolicy) *RelayStation {
	if id == 0 {
		panic("node: relay station id 0 is reserved for direct/source frames (§4.3)")
	}
	if life == nil {
		panic("node: relay lifecycle must not be nil")
	}
	return &RelayStation{
		id:           id,
		policy:       normalizeRelayPolicy(policy),
		life:         life,
		dropCounters: make(map[DropReason]int),
	}
}

// ID returns the relay station id.
func (r *RelayStation) ID() uint16 { return r.id }

// StaleThresholdMs returns the relay's normalized stale-frame threshold.
// Scheduled runtimes use the same threshold when a previously accepted
// reservation reaches its transmit opportunity.
func (r *RelayStation) StaleThresholdMs() int64 { return r.policy.StaleThresholdMs }

// ProcessVoiceFrame applies the relay forwarding logic to a voice
// frame per algorithm-and-experiments.md §7.2 (non-slot mode):
//
//  1. Verify frame integrity and semantic validity. Invalid → drop "invalid".
//  2. Check frame_type == voice and voice-only flags. Non-voice →
//     drop "unsupported_frame_type" (control frames are handled by the
//     station ProcessCallStart/ProcessCallEnd wrappers); malformed
//     voice flags → drop "invalid".
//  3. Check group subscription. Wrong group → drop "wrong_group".
//  4. Check call is active. Inactive/unknown → drop "call_not_active".
//  5. Check staleness. Stale → drop "stale_frame". Also reject
//     "future" frames (CreatedAtMs > nowMs) as invalid — a frame
//     cannot be received before it was created.
//  6. Check seen-cache. Already seen → drop "already_seen".
//  7. Guard hop overflow: hop == MaxUint8 → drop "invalid".
//  8. Mark seen (BEFORE TTL check per §7.2 relay algorithm: the
//     frame is recorded even if its TTL is expired, so a second
//     arrival is "already_seen" not "ttl_expired").
//  9. Check TTL. Expired → drop "ttl_expired".
//  10. Build forwarded copy: relay_id=R, hop+1, ttl-1, flags=relayed
//     (direct cleared), sent_at_ms=nowMs, preserved source fields,
//     defensive payload copy, CRCs zeroed (caller's Encode() will
//     recompute them).
//
// nowMs is the simulated time at which the relay received the frame,
// supplied by the caller. No wall-clock is used (NFR-1).
func (r *RelayStation) ProcessVoiceFrame(f protocol.Frame, nowMs int64) ForwardResult {
	// 1. Verify integrity and semantic validity.
	if err := f.VerifyIntegrity(); err != nil {
		return r.drop(DropReasonInvalid, f)
	}
	if err := f.Validate(); err != nil {
		return r.drop(DropReasonInvalid, f)
	}

	// 2. Frame type check.
	if f.FrameType != protocol.FrameTypeVoice {
		return r.drop(DropReasonUnsupportedFrameType, f)
	}
	if err := validateVoiceFrameFlags(f.Flags); err != nil {
		return r.drop(DropReasonInvalid, f)
	}

	// 3. Group subscription.
	if len(r.policy.SubscribedGroups) > 0 && !r.policy.SubscribedGroups[f.GroupID] {
		return r.drop(DropReasonWrongGroup, f)
	}

	// 4. Call active check + group consistency (call_start established
	// the call's group_id; a voice frame with a different group_id for
	// the same (net_id, source_id, call_id) is invalid).
	callKey := CallKeyFromFrame(f)
	state, ok := r.life.Call(callKey)
	if !ok || !state.Active {
		return r.drop(DropReasonCallNotActive, f)
	}
	if state.GroupID != 0 && f.GroupID != state.GroupID {
		// The call was started with a specific group_id; a voice frame
		// for the same call but a different group is a protocol error.
		return r.drop(DropReasonWrongGroup, f)
	}

	// 5. Stale check (§11) + future-frame guard.
	if reason, drop := frameAgeDropReason(f, nowMs, r.policy.StaleThresholdMs); drop {
		return r.drop(reason, f)
	}

	// 6. Seen-cache check (§13).
	originKey := f.OriginKey()
	if r.life.Seen(originKey) {
		return r.drop(DropReasonAlreadySeen, f)
	}

	// 7. Hop overflow guard BEFORE MarkSeen: a frame with hop=255 must
	// not poison the seen-cache, or a later valid frame with the same
	// origin_key would be rejected as already_seen.
	if f.Hop == math.MaxUint8 {
		return r.drop(DropReasonInvalid, f)
	}

	// 8. Mark seen BEFORE TTL check (§7.2 relay algorithm: the frame
	// is recorded even if its TTL is expired, so a second arrival of
	// the same origin_key is "already_seen", not "ttl_expired").
	if err := r.life.MarkSeen(originKey); err != nil {
		// MarkSeen only fails if the call became inactive between step
		// 4 and now (should not happen in single-threaded MVP). Record
		// as call_not_active.
		return r.drop(DropReasonCallNotActive, f)
	}

	// 9. TTL check (§10).
	if f.TTL <= 0 {
		return r.drop(DropReasonTTLExpired, f)
	}

	// 10. Build forwarded copy per §8.2 and §10.
	forwarded := f
	forwarded.RelayID = r.id
	forwarded.Hop = f.Hop + 1
	forwarded.TTL = f.TTL - 1
	forwarded.Flags = (f.Flags &^ protocol.FlagDirect) | protocol.FlagRelayed
	forwarded.SentAtMs = nowMs
	// Defensive payload copy so the caller cannot mutate the forwarded
	// frame's payload by mutating the original frame's slice.
	forwarded.Payload = clonePayload(f.Payload)
	// Zero CRCs: they are stale from the original frame and will be
	// recomputed by Encode() when the caller sends the forwarded copy.
	forwarded.HeaderCRC32 = 0
	forwarded.PayloadCRC32 = 0

	return ForwardResult{Forwarded: &forwarded}
}

// drop records a drop reason in the counters and returns a ForwardResult
// carrying the DropResult. The dropped frame's payload is defensively
// copied so the caller cannot mutate the DropResult by editing the
// original frame's payload slice.
func (r *RelayStation) drop(reason DropReason, f protocol.Frame) ForwardResult {
	r.dropCounters[reason]++
	f.Payload = clonePayload(f.Payload)
	return ForwardResult{Dropped: &DropResult{Reason: reason, Frame: f}}
}

// clonePayload returns an independent copy of b, preserving nil while
// avoiding shared backing storage even for zero-length non-nil slices.
func clonePayload(b []byte) []byte {
	if b == nil {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

// DropCounters returns a snapshot copy of per-reason drop counters.
// The returned map is independent of the relay's internal state.
func (r *RelayStation) DropCounters() map[DropReason]int {
	out := make(map[DropReason]int, len(r.dropCounters))
	for k, v := range r.dropCounters {
		out[k] = v
	}
	return out
}

// ProcessCallStart validates the call_start frame, checks the relay's
// SubscribedGroups, then delegates to the relay lifecycle to initialize
// per-call state.
func (r *RelayStation) ProcessCallStart(f protocol.Frame, nowMs int64) error {
	if _, _, err := decodeCallStartFrame(f); err != nil {
		return err
	}
	if len(r.policy.SubscribedGroups) > 0 && !r.policy.SubscribedGroups[f.GroupID] {
		return fmt.Errorf("node: relay call_start group_id=%d not in subscribed groups", f.GroupID)
	}
	if err := validateFrameAge(f, nowMs, r.policy.StaleThresholdMs); err != nil {
		return err
	}
	return r.life.ProcessCallStart(f, nowMs)
}

// ProcessCallEnd validates the call_end frame, checks the relay's
// SubscribedGroups, then delegates to the relay lifecycle to end
// per-call state and schedule cleanup.
func (r *RelayStation) ProcessCallEnd(f protocol.Frame, nowMs int64) error {
	if _, _, err := decodeCallEndFrame(f); err != nil {
		return err
	}
	if len(r.policy.SubscribedGroups) > 0 && !r.policy.SubscribedGroups[f.GroupID] {
		return fmt.Errorf("node: relay call_end group_id=%d not in subscribed groups", f.GroupID)
	}
	if err := validateFrameAge(f, nowMs, r.policy.StaleThresholdMs); err != nil {
		return err
	}
	return r.life.ProcessCallEnd(f, nowMs)
}

// Cleanup delegates to the relay lifecycle to evict ended-call state
// after the grace period.
func (r *RelayStation) Cleanup(nowMs int64) {
	r.life.Cleanup(nowMs)
}

// Lifecycle returns the underlying relay lifecycle, for test inspection.
func (r *RelayStation) Lifecycle() *RelayLifecycle { return r.life }

// String returns a debug summary of the relay station.
func (r *RelayStation) String() string {
	return fmt.Sprintf("RelayStation(id=%d, stale_threshold_ms=%d, drops=%v)", r.id, r.policy.StaleThresholdMs, r.dropCounters)
}
