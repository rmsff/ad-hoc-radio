package node

import (
	"fmt"
	"math"
	"sort"

	"github.com/rmsff/ad-hoc-radio/internal/protocol"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
)

type jitterEntry struct {
	Frame        protocol.Frame
	ReceivedAtMs int64
}

const maxJitterEvaluationSteps = 4096

// LateReason classifies valid frames that are accepted for duplicate
// accounting but cannot be normal playout candidates.
type LateReason string

const (
	LateReasonThresholdExpired LateReason = "late_threshold_expired"
	LateReasonPlayoutExpired   LateReason = "playout_expired"
)

// MissingSequence records one sequence that expired before a normal
// playout frame was available.
type MissingSequence struct {
	CallKey CallKey
	Seq     uint32
}

// JitterEvaluation is the deterministic result of receiver jitter
// evaluation at one simulated timestamp.
type JitterEvaluation struct {
	Emitted []protocol.Frame
	Missing []MissingSequence
}

// BufferJitterFrame stores f using f.SentAtMs as the receive timestamp.
//
// Deprecated: use BufferJitterFrameAt when receive time matters. This
// helper is only for lifecycle callers that do not model transport
// arrival separately.
func (r *ReceiverLifecycle) BufferJitterFrame(f protocol.Frame) error {
	return r.BufferJitterFrameAt(f, f.SentAtMs)
}

// BufferJitterFrameAt stores f in the receiver's jitter state. The call
// must already be active. receivedAtMs is caller-supplied simulated time;
// this package never reads wall-clock time.
func (r *ReceiverLifecycle) BufferJitterFrameAt(f protocol.Frame, receivedAtMs int64) error {
	if receivedAtMs < 0 {
		return fmt.Errorf("node: received_at_ms=%d must be >= 0", receivedAtMs)
	}
	if f.FrameType != protocol.FrameTypeVoice {
		return fmt.Errorf("node: jitter buffer only accepts voice frames, got frame_type=0x%02X", f.FrameType)
	}
	if err := f.VerifyIntegrity(); err != nil {
		return err
	}
	if err := f.Validate(); err != nil {
		return err
	}
	key := CallKeyFromFrame(f)
	state, ok := r.calls[key]
	if !ok || !state.Active {
		return fmt.Errorf("node: voice frame for inactive or unknown call %+v", key)
	}
	if state.GroupID != 0 && f.GroupID != state.GroupID {
		return fmt.Errorf("node: voice frame group_id=%d does not match active call group_id=%d for call %+v", f.GroupID, state.GroupID, key)
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
	if _, exists := r.jitter[key][f.Seq]; exists {
		return fmt.Errorf("node: jitter buffer already has seq=%d for call %+v", f.Seq, key)
	}
	r.jitter[key][f.Seq] = jitterEntry{
		Frame:        cloneFrame(f),
		ReceivedAtMs: receivedAtMs,
	}
	return nil
}

func (r *ReceiverLifecycle) markLateSequence(f protocol.Frame) error {
	key := CallKeyFromFrame(f)
	state, ok := r.calls[key]
	if !ok || !state.Active {
		return fmt.Errorf("node: cannot mark late sequence for inactive or unknown call %+v", key)
	}
	if state.GroupID != 0 && f.GroupID != state.GroupID {
		return fmt.Errorf("node: late frame group_id=%d does not match active call group_id=%d for call %+v", f.GroupID, state.GroupID, key)
	}
	if _, ok := r.lateSeqs[key]; !ok {
		r.lateSeqs[key] = make(map[uint32]struct{})
	}
	r.lateSeqs[key][f.Seq] = struct{}{}
	return nil
}

func (r *ReceiverLifecycle) frameLateByThreshold(f protocol.Frame, receivedAtMs, lateThresholdMs int64) bool {
	if receivedAtMs < f.CreatedAtMs {
		return false
	}
	return ageExceedsThreshold(f.CreatedAtMs, receivedAtMs, lateThresholdMs)
}

func (r *ReceiverLifecycle) framePastPlayout(f protocol.Frame) bool {
	key := CallKeyFromFrame(f)
	nextSeq := r.nextPlayoutSeq[key]
	return f.Seq < nextSeq
}

func (r *ReceiverLifecycle) EvaluateJitter(nowMs, jitterBufferMs, lateThresholdMs int64) JitterEvaluation {
	if nowMs < 0 {
		panic(fmt.Sprintf("node: jitter evaluate now_ms=%d must be >= 0", nowMs))
	}
	if jitterBufferMs < 0 {
		panic(fmt.Sprintf("node: jitter_buffer_ms=%d must be >= 0", jitterBufferMs))
	}
	if lateThresholdMs < 0 {
		panic(fmt.Sprintf("node: late_threshold_ms=%d must be >= 0", lateThresholdMs))
	}

	var out JitterEvaluation
	for _, key := range r.activeCallKeysSorted() {
		state := r.calls[key]
		r.evaluateCallJitter(key, state, nowMs, jitterBufferMs, lateThresholdMs, &out)
	}
	return out
}

func (r *ReceiverLifecycle) evaluateCallJitter(key CallKey, state CallState, nowMs, jitterBufferMs, lateThresholdMs int64, out *JitterEvaluation) {
	if _, ok := r.nextPlayoutSeq[key]; !ok {
		r.nextPlayoutSeq[key] = 0
	}
	if _, ok := r.jitter[key]; !ok {
		r.jitter[key] = make(map[uint32]jitterEntry)
	}
	if _, ok := r.lateSeqs[key]; !ok {
		r.lateSeqs[key] = make(map[uint32]struct{})
	}

	limitSeq, hasLimit := r.jitterEvaluationLimitExclusive(key, state)
	if !hasLimit {
		return
	}

	steps := 0
	for {
		if steps >= maxJitterEvaluationSteps {
			return
		}
		nextSeq := r.nextPlayoutSeq[key]
		if uint64(nextSeq) >= limitSeq {
			return
		}

		if entry, ok := r.jitter[key][nextSeq]; ok {
			playoutDeadline := sim.AddMs(entry.Frame.CreatedAtMs, jitterBufferMs)
			if nowMs < playoutDeadline {
				return
			}
			out.Emitted = append(out.Emitted, cloneFrame(entry.Frame))
			delete(r.jitter[key], nextSeq)
			if !r.advanceNextPlayoutSeq(key) {
				return
			}
			steps++
			continue
		}

		if r.consumeLateSequence(key, nextSeq) {
			if !r.advanceNextPlayoutSeq(key) {
				return
			}
			steps++
			continue
		}

		missingDeadline := sim.AddMs(expectedCreatedAtMs(state, nextSeq), lateThresholdMs)
		if nowMs <= missingDeadline {
			return
		}

		out.Missing = append(out.Missing, MissingSequence{CallKey: key, Seq: nextSeq})
		if !r.advanceNextPlayoutSeq(key) {
			return
		}
		steps++
	}
}

func (r *ReceiverLifecycle) finalizeCallJitterThroughLastSeq(key CallKey, lastSeq uint32) JitterEvaluation {
	var out JitterEvaluation
	if _, ok := r.nextPlayoutSeq[key]; !ok {
		r.nextPlayoutSeq[key] = 0
	}
	if _, ok := r.jitter[key]; !ok {
		r.jitter[key] = make(map[uint32]jitterEntry)
	}

	for {
		nextSeq := r.nextPlayoutSeq[key]
		if nextSeq > lastSeq {
			return out
		}
		if entry, ok := r.jitter[key][nextSeq]; ok {
			out.Emitted = append(out.Emitted, cloneFrame(entry.Frame))
			delete(r.jitter[key], nextSeq)
			if !r.advanceNextPlayoutSeq(key) {
				return out
			}
			continue
		}
		if r.consumeLateSequence(key, nextSeq) {
			if !r.advanceNextPlayoutSeq(key) {
				return out
			}
			continue
		}
		out.Missing = append(out.Missing, MissingSequence{CallKey: key, Seq: nextSeq})
		if !r.advanceNextPlayoutSeq(key) {
			return out
		}
	}
}

func (r *ReceiverLifecycle) jitterEvaluationLimitExclusive(key CallKey, state CallState) (uint64, bool) {
	if state.Start.ExpectedFrameCount != nil {
		return uint64(*state.Start.ExpectedFrameCount), true
	}
	highest, ok := r.highestKnownSeq(key)
	if !ok {
		return 0, false
	}
	return uint64(highest) + 1, true
}

func (r *ReceiverLifecycle) highestKnownSeq(key CallKey) (uint32, bool) {
	var highest uint32
	found := false
	for seq := range r.jitter[key] {
		if !found || seq > highest {
			highest = seq
			found = true
		}
	}
	for seq := range r.lateSeqs[key] {
		if !found || seq > highest {
			highest = seq
			found = true
		}
	}
	return highest, found
}

func (r *ReceiverLifecycle) consumeLateSequence(key CallKey, seq uint32) bool {
	lateSeqs := r.lateSeqs[key]
	if lateSeqs == nil {
		return false
	}
	if _, ok := lateSeqs[seq]; !ok {
		return false
	}
	delete(lateSeqs, seq)
	return true
}

func expectedCreatedAtMs(state CallState, seq uint32) int64 {
	period := state.Start.FramePeriodMs
	if period <= 0 {
		panic(fmt.Sprintf("node: invalid frame_period_ms=%d for call %+v", period, state.Key))
	}
	offset := mulMs(int64(seq), period)
	return sim.AddMs(state.CreatedAtMs, offset)
}

func (r *ReceiverLifecycle) advanceNextPlayoutSeq(key CallKey) bool {
	nextSeq := r.nextPlayoutSeq[key]
	if nextSeq == math.MaxUint32 {
		return false
	}
	r.nextPlayoutSeq[key] = nextSeq + 1
	return true
}

func (r *ReceiverLifecycle) activeCallKeysSorted() []CallKey {
	keys := make([]CallKey, 0, len(r.calls))
	for key, state := range r.calls {
		if state.Active {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].NetID != keys[j].NetID {
			return keys[i].NetID < keys[j].NetID
		}
		if keys[i].SourceID != keys[j].SourceID {
			return keys[i].SourceID < keys[j].SourceID
		}
		return keys[i].CallID < keys[j].CallID
	})
	return keys
}

func mulMs(a, b int64) int64 {
	if a < 0 || b < 0 {
		panic("node: negative simulated-time operand")
	}
	if a != 0 && b > math.MaxInt64/a {
		panic("node: simulated-time multiplication overflow")
	}
	return a * b
}
