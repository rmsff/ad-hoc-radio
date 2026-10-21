package node

import (
	"fmt"

	"github.com/rmsff/ad-hoc-radio/internal/protocol"
)

// DefaultReceiverStaleThresholdMs is the MVP receiver stale-frame
// threshold from frame-contract.md §11.
const DefaultReceiverStaleThresholdMs int64 = 1000

// DefaultReceiverJitterBufferMs is the first MVP jitter-buffer default
// from docs/simulation/latency-and-airtime-budget.md §3.
const DefaultReceiverJitterBufferMs int64 = 160

// DefaultReceiverLateThresholdMs aligns late playout handling with the
// SM-3 p95 latency target.
const DefaultReceiverLateThresholdMs int64 = 250

// ReceiverPolicy controls receiver acceptance and duplicate
// suppression behavior.
type ReceiverPolicy struct {
	// StaleThresholdMs: frames older than this (by created_at_ms) are
	// dropped as stale. Default 1000 ms per §11.
	StaleThresholdMs int64
	// SubscribedGroups: group_ids this receiver is subscribed to. A
	// frame whose group_id is not in this set is dropped as
	// wrong_group. Empty means "subscribe to all groups" (no group
	// filtering).
	SubscribedGroups map[uint16]bool
	// JitterBufferMs: simulated delay before contiguous frames become
	// eligible for playout. Default 160 ms.
	JitterBufferMs int64
	// LateThresholdMs: frames arriving later than this relative to
	// created_at_ms are counted late and not emitted as normal playout.
	// Default 250 ms.
	LateThresholdMs int64
}

// DefaultReceiverPolicy returns the frame-contract.md defaults.
func DefaultReceiverPolicy() ReceiverPolicy {
	return ReceiverPolicy{
		StaleThresholdMs: DefaultReceiverStaleThresholdMs,
		JitterBufferMs:   DefaultReceiverJitterBufferMs,
		LateThresholdMs:  DefaultReceiverLateThresholdMs,
	}
}

func normalizeReceiverPolicy(p ReceiverPolicy) ReceiverPolicy {
	if p.StaleThresholdMs == 0 {
		p.StaleThresholdMs = DefaultReceiverStaleThresholdMs
	}
	if p.JitterBufferMs == 0 {
		p.JitterBufferMs = DefaultReceiverJitterBufferMs
	}
	if p.LateThresholdMs == 0 {
		p.LateThresholdMs = DefaultReceiverLateThresholdMs
	}
	if p.StaleThresholdMs < 0 {
		panic("node: receiver stale_threshold_ms must be >= 0")
	}
	if p.JitterBufferMs < 0 {
		panic("node: receiver jitter_buffer_ms must be >= 0")
	}
	if p.LateThresholdMs < 0 {
		panic("node: receiver late_threshold_ms must be >= 0")
	}
	// Defensive copy of SubscribedGroups so the caller cannot mutate
	// the receiver's policy by editing the map after construction.
	if p.SubscribedGroups != nil {
		cloned := make(map[uint16]bool, len(p.SubscribedGroups))
		for k, v := range p.SubscribedGroups {
			cloned[k] = v
		}
		p.SubscribedGroups = cloned
	}
	return p
}

// SuppressedResult records a frame that was suppressed as a duplicate.
// In the MVP the only suppression reason is "duplicate" (origin_key
// already in accepted-key cache per §7.4).
type SuppressedResult struct {
	Frame protocol.Frame
}

// LateResult records a valid frame accepted for duplicate accounting but
// excluded from normal playout because its late threshold or playout
// deadline has expired.
type LateResult struct {
	Frame  protocol.Frame
	Reason LateReason
}

// AcceptResult is the outcome of ProcessVoiceFrame: an accepted playout
// candidate, a suppressed duplicate, a late non-playout frame, or a drop.
// Exactly one of Accepted, Suppressed, Late, Dropped is non-nil.
type AcceptResult struct {
	Accepted   *protocol.Frame
	Suppressed *SuppressedResult
	Late       *LateResult
	Dropped    *DropResult
}

// ReceiverCounters is a snapshot of receiver counters. Fields align
// with docs/protocol/metrics-schema.json receiver object where
// applicable.
type ReceiverCounters struct {
	FramesReceived        int
	FramesAccepted        int
	DuplicatesSuppressed  int
	DirectFramesAccepted  int
	RelayedFramesAccepted int
	LateFrames            int
	MissingSequences      int
	PlayoutFramesEmitted  int
	DroppedByReason       map[DropReason]int
}

// ReceiverStation is the receiver acceptance and duplicate suppression
// state machine. It wraps a ReceiverLifecycle (call state +
// accepted-key cache + jitter buffer from Story 2.0) with the
// per-frame acceptance logic from algorithm-and-experiments.md §7.3
// and frame-contract.md §11, §13, §7.4.
//
// The receiver does NOT send frames. ProcessVoiceFrame returns an
// AcceptResult; the caller (Story 2.3 scenario runner) is responsible
// for reading the accepted frame and wiring playout/metrics.
//
// The receiver does NOT decode or re-encode audio (AD-3, NFR-3). It
// operates on frame metadata and payload bytes only.
//
// ReceiverStation is NOT safe for concurrent use. The MVP simulator is
// single-threaded (SimulatedClock, event-driven). Add synchronization
// before any multi-goroutine use.
type ReceiverStation struct {
	id     uint16
	policy ReceiverPolicy
	life   *ReceiverLifecycle

	framesReceived        int
	framesAccepted        int
	duplicatesSuppressed  int
	directFramesAccepted  int
	relayedFramesAccepted int
	lateFrames            int
	missingSequences      int
	playoutFramesEmitted  int
	dropCounters          map[DropReason]int
}

// NewReceiverStation creates a receiver with station id, lifecycle,
// and policy. The lifecycle must already be initialized (Story 2.0);
// ProcessCallStart must have been called for any call whose voice
// frames this receiver will process.
//
// Panics if id == 0: numeric_id 0 is reserved for relay_id=0
// direct/source frames (frame-contract.md §4.3).
//
// Panics if life is nil.
func NewReceiverStation(id uint16, life *ReceiverLifecycle, policy ReceiverPolicy) *ReceiverStation {
	if id == 0 {
		panic("node: receiver station id 0 is reserved (§4.3)")
	}
	if life == nil {
		panic("node: receiver lifecycle must not be nil")
	}
	return &ReceiverStation{
		id:           id,
		policy:       normalizeReceiverPolicy(policy),
		life:         life,
		dropCounters: make(map[DropReason]int),
	}
}

// ID returns the receiver station id.
func (r *ReceiverStation) ID() uint16 { return r.id }

// ProcessVoiceFrame applies the receiver acceptance and duplicate
// suppression logic to a voice frame per algorithm-and-experiments.md
// §7.3 (non-slot mode):
//
//  1. Verify frame integrity and semantic validity. Invalid → drop "invalid".
//  2. Check group subscription. Wrong group → drop "wrong_group".
//  3. Check frame_type == voice and voice-only flags. Non-voice →
//     drop "unsupported_frame_type"; malformed voice flags → drop
//     "invalid".
//  4. Check call is active. Inactive/unknown → drop "call_not_active".
//  5. Check group consistency (call's group_id vs frame's group_id).
//     Mismatch → drop "wrong_group".
//  6. Check staleness (§11). Stale → drop "stale_frame". Also reject
//     "future" frames (CreatedAtMs > nowMs) as invalid — a frame
//     cannot be received before it was created.
//  7. Check accepted-key cache. Already accepted → suppress duplicate.
//  8. Mark accepted BEFORE jitter insert per §7.3 (the frame is
//     recorded even if jitter insert fails, so a second arrival is
//     suppressed as duplicate, not re-attempted).
//  9. Check jitter late threshold. Threshold-late frames are
//     accepted for duplicate accounting, counted as late, and
//     returned as Late rather than normal Accepted playout
//     candidates.
//  10. Check playout advancement. If the jitter buffer already marked
//     this seq missing, accept it for duplicate accounting but do not
//     count it as a threshold-late frame.
//  11. Insert into jitter buffer via lifecycle.BufferJitterFrameAt
//     with defensive payload copy.
//
// 12. Update counters (frames_accepted, direct/relayed).
//
// nowMs is the simulated time at which the receiver received the frame,
// supplied by the caller. No wall-clock is used (NFR-1).
func (r *ReceiverStation) ProcessVoiceFrame(f protocol.Frame, nowMs int64) AcceptResult {
	r.framesReceived++

	// 1. Verify integrity and semantic validity.
	if err := f.VerifyIntegrity(); err != nil {
		return r.drop(DropReasonInvalid, f)
	}
	if err := f.Validate(); err != nil {
		return r.drop(DropReasonInvalid, f)
	}

	// 2. Group subscription.
	if len(r.policy.SubscribedGroups) > 0 && !r.policy.SubscribedGroups[f.GroupID] {
		return r.drop(DropReasonWrongGroup, f)
	}

	// 3. Frame type check (defensive: non-voice should not reach here).
	if f.FrameType != protocol.FrameTypeVoice {
		return r.drop(DropReasonUnsupportedFrameType, f)
	}
	if err := validateVoiceFrameFlags(f.Flags); err != nil {
		return r.drop(DropReasonInvalid, f)
	}

	// 4. Call active check.
	callKey := CallKeyFromFrame(f)
	state, ok := r.life.Call(callKey)
	if !ok || !state.Active {
		return r.drop(DropReasonCallNotActive, f)
	}

	// 5. Group consistency (call_start established the call's
	// group_id; a voice frame with a different group_id for the same
	// call is invalid).
	if state.GroupID != 0 && f.GroupID != state.GroupID {
		return r.drop(DropReasonWrongGroup, f)
	}

	// 6. Stale check (§11) + future-frame guard.
	if reason, drop := frameAgeDropReason(f, nowMs, r.policy.StaleThresholdMs); drop {
		return r.drop(reason, f)
	}

	// 7. Accepted-key cache check (§7.4: suppress by origin_key, not
	// by copy_key — direct and relayed copies share origin_key).
	originKey := f.OriginKey()
	if r.life.Accepted(originKey) {
		r.duplicatesSuppressed++
		return AcceptResult{Suppressed: &SuppressedResult{Frame: cloneFrame(f)}}
	}

	// 8. Mark accepted BEFORE jitter insert per §7.3
	// (accepted_origin_keys.add() then jitter_buffer.insert()).
	if err := r.life.MarkAccepted(originKey); err != nil {
		// MarkAccepted only fails if the call became inactive between
		// step 4 and now (should not happen in single-threaded MVP).
		return r.drop(DropReasonCallNotActive, f)
	}

	if r.life.frameLateByThreshold(f, nowMs, r.policy.LateThresholdMs) {
		if err := r.life.markLateSequence(f); err != nil {
			return r.drop(DropReasonInvalid, f)
		}
		r.lateFrames++
		r.recordAcceptedCounters(f)
		late := cloneFrame(f)
		return AcceptResult{Late: &LateResult{Frame: late, Reason: LateReasonThresholdExpired}}
	}

	if r.life.framePastPlayout(f) {
		r.recordAcceptedCounters(f)
		expired := cloneFrame(f)
		return AcceptResult{Late: &LateResult{Frame: expired, Reason: LateReasonPlayoutExpired}}
	}

	// 11. Insert into jitter buffer with defensive payload copy so the
	// caller cannot mutate the buffered frame's payload.
	buffered := f
	buffered.Payload = clonePayload(f.Payload)
	if err := r.life.BufferJitterFrameAt(buffered, nowMs); err != nil {
		// Invariant violation in single-threaded MVP: all preconditions
		// were checked above. Record as invalid drop; origin_key is
		// already in accepted cache, so a duplicate will be suppressed.
		return r.drop(DropReasonInvalid, f)
	}

	// 12. Update acceptance counters.
	r.recordAcceptedCounters(f)

	// Return accepted frame as an independent copy (separate from the
	// jitter buffer entry) so the caller cannot mutate the jitter
	// buffer by editing the returned frame.
	accepted := cloneFrame(buffered)
	return AcceptResult{Accepted: &accepted}
}

func (r *ReceiverStation) recordAcceptedCounters(f protocol.Frame) {
	r.framesAccepted++
	if f.Flags&protocol.FlagRelayed != 0 {
		r.relayedFramesAccepted++
	} else {
		r.directFramesAccepted++
	}
}

// drop records a drop reason in the counters and returns an
// AcceptResult carrying the DropResult. The dropped frame's payload is
// defensively copied so the caller cannot mutate the DropResult by
// editing the original frame's payload slice.
func (r *ReceiverStation) drop(reason DropReason, f protocol.Frame) AcceptResult {
	r.dropCounters[reason]++
	f.Payload = clonePayload(f.Payload)
	return AcceptResult{Dropped: &DropResult{Reason: reason, Frame: f}}
}

// cloneFrame returns an independent copy of f with a defensive
// payload copy.
func cloneFrame(f protocol.Frame) protocol.Frame {
	out := f
	out.Payload = clonePayload(f.Payload)
	return out
}

// Counters returns a snapshot copy of receiver counters. The returned
// DroppedByReason map is independent of the receiver's internal state.
func (r *ReceiverStation) Counters() ReceiverCounters {
	out := ReceiverCounters{
		FramesReceived:        r.framesReceived,
		FramesAccepted:        r.framesAccepted,
		DuplicatesSuppressed:  r.duplicatesSuppressed,
		DirectFramesAccepted:  r.directFramesAccepted,
		RelayedFramesAccepted: r.relayedFramesAccepted,
		LateFrames:            r.lateFrames,
		MissingSequences:      r.missingSequences,
		PlayoutFramesEmitted:  r.playoutFramesEmitted,
		DroppedByReason:       make(map[DropReason]int, len(r.dropCounters)),
	}
	for k, v := range r.dropCounters {
		out.DroppedByReason[k] = v
	}
	return out
}

// EvaluateJitter emits ready contiguous frames and records missing
// sequence counters using caller-supplied simulated time.
func (r *ReceiverStation) EvaluateJitter(nowMs int64) JitterEvaluation {
	eval := r.life.EvaluateJitter(nowMs, r.policy.JitterBufferMs, r.policy.LateThresholdMs)
	r.missingSequences += len(eval.Missing)
	r.playoutFramesEmitted += len(eval.Emitted)
	return eval
}

// ProcessCallStart validates the call_start frame, checks the
// receiver's SubscribedGroups, then delegates to the receiver lifecycle
// to initialize per-call state.
func (r *ReceiverStation) ProcessCallStart(f protocol.Frame, nowMs int64) error {
	if _, _, err := decodeCallStartFrame(f); err != nil {
		return err
	}
	if len(r.policy.SubscribedGroups) > 0 && !r.policy.SubscribedGroups[f.GroupID] {
		return fmt.Errorf("node: receiver call_start group_id=%d not in subscribed groups", f.GroupID)
	}
	if err := validateFrameAge(f, nowMs, r.policy.StaleThresholdMs); err != nil {
		return err
	}
	return r.life.ProcessCallStart(f, nowMs)
}

// ProcessCallEnd validates the call_end frame, checks the receiver's
// SubscribedGroups, then delegates to the receiver lifecycle to end
// per-call state and schedule cleanup.
func (r *ReceiverStation) ProcessCallEnd(f protocol.Frame, nowMs int64) error {
	_, err := r.ProcessCallEndWithEvaluation(f, nowMs)
	return err
}

// ProcessCallEndWithEvaluation is ProcessCallEnd with the final jitter
// evaluation exposed to integration runtimes that need playout evidence.
func (r *ReceiverStation) ProcessCallEndWithEvaluation(f protocol.Frame, nowMs int64) (JitterEvaluation, error) {
	if _, _, err := decodeCallEndFrame(f); err != nil {
		return JitterEvaluation{}, err
	}
	if len(r.policy.SubscribedGroups) > 0 && !r.policy.SubscribedGroups[f.GroupID] {
		return JitterEvaluation{}, fmt.Errorf("node: receiver call_end group_id=%d not in subscribed groups", f.GroupID)
	}
	if err := validateFrameAge(f, nowMs, r.policy.StaleThresholdMs); err != nil {
		return JitterEvaluation{}, err
	}
	eval, err := r.life.ProcessCallEnd(f, nowMs)
	if err != nil {
		return JitterEvaluation{}, err
	}
	r.missingSequences += len(eval.Missing)
	r.playoutFramesEmitted += len(eval.Emitted)
	return eval, nil
}

// Cleanup delegates to the receiver lifecycle to evict ended-call
// state after the grace period.
func (r *ReceiverStation) Cleanup(nowMs int64) {
	r.life.Cleanup(nowMs)
}

// Lifecycle returns the underlying receiver lifecycle, for test
// inspection.
func (r *ReceiverStation) Lifecycle() *ReceiverLifecycle { return r.life }

// String returns a debug summary of the receiver station.
func (r *ReceiverStation) String() string {
	return fmt.Sprintf("ReceiverStation(id=%d, stale_threshold_ms=%d, received=%d, accepted=%d, duplicates=%d, direct=%d, relayed=%d, drops=%v)",
		r.id, r.policy.StaleThresholdMs, r.framesReceived, r.framesAccepted,
		r.duplicatesSuppressed, r.directFramesAccepted, r.relayedFramesAccepted,
		r.dropCounters)
}
