package node

import "github.com/rmsff/ad-hoc-radio/internal/sim"

// PlayoutDelayMS is the receiver's configured fixed audio delay.
func (r *ReceiverStation) PlayoutDelayMS() int64 { return r.policy.JitterBufferMs }

// EvaluatePlayoutDeadline advances the audio clock even when a packet is absent.
// Callers deliver packets at a timestamp before evaluating its deadline. Unlike
// the legacy jitter evaluator, missing packets cannot stall subsequent audio.
func (r *ReceiverStation) EvaluatePlayoutDeadline(nowMS int64) JitterEvaluation {
	var out JitterEvaluation
	for _, key := range r.life.activeCallKeysSorted() {
		state := r.life.calls[key]
		limit, ok := r.life.jitterEvaluationLimitExclusive(key, state)
		if !ok {
			continue
		}
		for steps := 0; steps < maxJitterEvaluationSteps; steps++ {
			seq := r.life.nextPlayoutSeq[key]
			if uint64(seq) >= limit {
				break
			}
			deadline := sim.AddMs(expectedCreatedAtMs(state, seq), r.policy.JitterBufferMs)
			if nowMS < deadline {
				break
			}
			entry, present := r.life.jitter[key][seq]
			if present && entry.ReceivedAtMs <= deadline {
				out.Emitted = append(out.Emitted, cloneFrame(entry.Frame))
			} else {
				out.Missing = append(out.Missing, MissingSequence{CallKey: key, Seq: seq})
			}
			delete(r.life.jitter[key], seq)
			r.life.consumeLateSequence(key, seq)
			if !r.life.advanceNextPlayoutSeq(key) {
				break
			}
		}
	}
	r.missingSequences += len(out.Missing)
	r.playoutFramesEmitted += len(out.Emitted)
	return out
}
