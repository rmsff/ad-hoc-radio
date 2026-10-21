package node

import (
	"reflect"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/protocol"
)

func receiverJitterTestSetup(t *testing.T, policy ReceiverPolicy) *ReceiverStation {
	t.Helper()
	life := NewReceiverLifecycle(DefaultLifecyclePolicy())
	receiver := NewReceiverStation(3, life, policy)
	if err := receiver.ProcessCallStart(makeCallStartFrame(t), 1_000); err != nil {
		t.Fatalf("ProcessCallStart: %v", err)
	}
	return receiver
}

func receiverJitterTestSetupWithExpectedCount(t *testing.T, policy ReceiverPolicy, expectedCount uint32) *ReceiverStation {
	t.Helper()
	life := NewReceiverLifecycle(DefaultLifecyclePolicy())
	receiver := NewReceiverStation(3, life, policy)
	if err := receiver.ProcessCallStart(makeCallStartFrameWithExpectedCount(t, expectedCount), 1_000); err != nil {
		t.Fatalf("ProcessCallStart: %v", err)
	}
	return receiver
}

func TestUT014_JitterEmitsOutOfOrderFramesInSequenceOrder(t *testing.T) {
	policy := DefaultReceiverPolicy()
	policy.JitterBufferMs = 120
	policy.LateThresholdMs = 500
	receiver := receiverJitterTestSetup(t, policy)

	seq1 := makeVoiceFrame(1)
	if r := receiver.ProcessVoiceFrame(seq1, 1_100); r.Accepted == nil {
		t.Fatalf("seq=1 not accepted: dropped=%v suppressed=%v", r.Dropped, r.Suppressed)
	}
	seq0 := makeVoiceFrame(0)
	if r := receiver.ProcessVoiceFrame(seq0, 1_020); r.Accepted == nil {
		t.Fatalf("seq=0 not accepted: dropped=%v suppressed=%v", r.Dropped, r.Suppressed)
	}

	eval := receiver.EvaluateJitter(1_200)
	got := frameSeqs(eval.Emitted)
	want := []uint32{0, 1}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("emitted seqs = %v, want %v", got, want)
	}
	if receiver.Counters().PlayoutFramesEmitted != 2 {
		t.Errorf("playout_frames_emitted = %d, want 2", receiver.Counters().PlayoutFramesEmitted)
	}
}

func TestUT014_LateFrameCountedAndNotEmitted(t *testing.T) {
	policy := DefaultReceiverPolicy()
	policy.JitterBufferMs = 120
	policy.LateThresholdMs = 50
	receiver := receiverJitterTestSetupWithExpectedCount(t, policy, 1)

	late := makeVoiceFrame(0)
	result := receiver.ProcessVoiceFrame(late, 1_060)
	if result.Late == nil {
		t.Fatalf("late frame should be accepted as non-playout late result: accepted=%v dropped=%v suppressed=%v", result.Accepted, result.Dropped, result.Suppressed)
	}
	if result.Late.Reason != LateReasonThresholdExpired {
		t.Fatalf("late reason = %q, want %q", result.Late.Reason, LateReasonThresholdExpired)
	}
	if result.Accepted != nil {
		t.Fatal("late frame returned as normal accepted playout candidate")
	}

	eval := receiver.EvaluateJitter(2_000)
	if len(eval.Emitted) != 0 {
		t.Fatalf("late frame emitted as normal playout: %+v", eval.Emitted)
	}
	counters := receiver.Counters()
	if counters.LateFrames != 1 {
		t.Errorf("late_frames = %d, want 1", counters.LateFrames)
	}
	if counters.MissingSequences != 0 {
		t.Errorf("missing_sequences = %d, want 0 for already-counted late frame", counters.MissingSequences)
	}
	if counters.PlayoutFramesEmitted != 0 {
		t.Errorf("playout_frames_emitted = %d, want 0", counters.PlayoutFramesEmitted)
	}
	if seqs := receiver.Lifecycle().BufferedSeqs(CallKey{NetID: testNetID, SourceID: testSource, CallID: testCallID}); len(seqs) != 0 {
		t.Fatalf("late frame remained buffered: %v", seqs)
	}
}

func TestUT014_MissedPlayoutNotCountedAsLateThreshold(t *testing.T) {
	policy := DefaultReceiverPolicy()
	policy.JitterBufferMs = 120
	policy.LateThresholdMs = 500
	receiver := receiverJitterTestSetup(t, policy)

	eval := receiver.EvaluateJitter(1_501)
	if len(eval.Missing) != 1 || eval.Missing[0].Seq != 0 {
		t.Fatalf("initial missing = %+v, want seq=0", eval.Missing)
	}

	result := receiver.ProcessVoiceFrame(makeVoiceFrameWith(0, func(f *protocol.Frame) {
		f.CreatedAtMs = 1_300
		f.SentAtMs = 1_300
	}), 1_502)
	if result.Late == nil {
		t.Fatalf("missed-playout frame should be non-playout late result: accepted=%v dropped=%v", result.Accepted, result.Dropped)
	}
	if result.Late.Reason != LateReasonPlayoutExpired {
		t.Fatalf("late reason = %q, want %q", result.Late.Reason, LateReasonPlayoutExpired)
	}

	counters := receiver.Counters()
	if counters.LateFrames != 0 {
		t.Errorf("late_frames = %d, want 0 for playout-expired before late_threshold", counters.LateFrames)
	}
	if counters.MissingSequences != 1 {
		t.Errorf("missing_sequences = %d, want 1", counters.MissingSequences)
	}
}

func TestUT014_MissingWaitsForLateThresholdNotJitterDelay(t *testing.T) {
	policy := DefaultReceiverPolicy()
	policy.JitterBufferMs = 120
	policy.LateThresholdMs = 250
	receiver := receiverJitterTestSetup(t, policy)

	if eval := receiver.EvaluateJitter(1_120); len(eval.Missing) != 0 {
		t.Fatalf("missing at playout deadline = %+v, want none before late threshold", eval.Missing)
	}
	if eval := receiver.EvaluateJitter(1_250); len(eval.Missing) != 0 {
		t.Fatalf("missing at strict late threshold = %+v, want none", eval.Missing)
	}

	eval := receiver.EvaluateJitter(1_251)
	if len(eval.Missing) != 1 || eval.Missing[0].Seq != 0 {
		t.Fatalf("missing after late threshold = %+v, want seq=0", eval.Missing)
	}
}

func TestUT014_MissingSequenceCountedAndLaterFramesContinue(t *testing.T) {
	policy := DefaultReceiverPolicy()
	policy.JitterBufferMs = 120
	policy.LateThresholdMs = 150
	receiver := receiverJitterTestSetup(t, policy)

	if r := receiver.ProcessVoiceFrame(makeVoiceFrame(0), 1_020); r.Accepted == nil {
		t.Fatalf("seq=0 not accepted: dropped=%v", r.Dropped)
	}
	if r := receiver.ProcessVoiceFrame(makeVoiceFrame(2), 1_180); r.Accepted == nil {
		t.Fatalf("seq=2 not accepted: dropped=%v", r.Dropped)
	}

	eval := receiver.EvaluateJitter(1_280)
	got := frameSeqs(eval.Emitted)
	want := []uint32{0, 2}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("emitted seqs = %v, want %v", got, want)
	}
	if len(eval.Missing) != 1 || eval.Missing[0].Seq != 1 {
		t.Fatalf("missing = %+v, want seq=1", eval.Missing)
	}
	counters := receiver.Counters()
	if counters.MissingSequences != 1 {
		t.Errorf("missing_sequences = %d, want 1", counters.MissingSequences)
	}
	if counters.PlayoutFramesEmitted != 2 {
		t.Errorf("playout_frames_emitted = %d, want 2", counters.PlayoutFramesEmitted)
	}
}

func TestUT014_PlayoutReadinessUsesFrameCreatedAt(t *testing.T) {
	policy := DefaultReceiverPolicy()
	policy.JitterBufferMs = 120
	policy.LateThresholdMs = 500
	receiver := receiverJitterTestSetup(t, policy)

	frame := makeVoiceFrameWith(0, func(f *protocol.Frame) {
		f.CreatedAtMs = 1_050
		f.SentAtMs = 1_050
	})
	if r := receiver.ProcessVoiceFrame(frame, 1_060); r.Accepted == nil {
		t.Fatalf("frame not accepted: dropped=%v suppressed=%v late=%v", r.Dropped, r.Suppressed, r.Late)
	}

	if eval := receiver.EvaluateJitter(1_120); len(eval.Emitted) != 0 {
		t.Fatalf("emitted at expected-created deadline = %+v, want none", eval.Emitted)
	}
	eval := receiver.EvaluateJitter(1_170)
	if got, want := frameSeqs(eval.Emitted), []uint32{0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("emitted seqs = %v, want %v", got, want)
	}
}

func TestUT014_EmittedFramesAreDefensiveCopies(t *testing.T) {
	policy := DefaultReceiverPolicy()
	policy.JitterBufferMs = 120
	policy.LateThresholdMs = 500
	receiver := receiverJitterTestSetup(t, policy)

	frame := makeVoiceFrameWithPayload(0, []byte{0xAA, 0xBB}, nil)
	result := receiver.ProcessVoiceFrame(frame, 1_020)
	if result.Accepted == nil {
		t.Fatalf("frame not accepted: dropped=%v suppressed=%v", result.Dropped, result.Suppressed)
	}

	eval := receiver.EvaluateJitter(1_120)
	if len(eval.Emitted) != 1 {
		t.Fatalf("emitted frame count = %d, want 1", len(eval.Emitted))
	}
	eval.Emitted[0].Payload[0] = 0x00

	if result.Accepted.Payload[0] != 0xAA {
		t.Fatalf("mutating emitted frame changed accepted copy: got payload[0]=0x%02X", result.Accepted.Payload[0])
	}
}

func TestUT014_BufferRejectsDuplicateSeqOverwrite(t *testing.T) {
	policy := DefaultReceiverPolicy()
	policy.JitterBufferMs = 120
	policy.LateThresholdMs = 500
	receiver := receiverJitterTestSetup(t, policy)

	first := makeVoiceFrameWithPayload(0, []byte{0xAA}, nil)
	second := makeVoiceFrameWithPayload(0, []byte{0xBB}, nil)
	if err := receiver.Lifecycle().BufferJitterFrameAt(first, 1_020); err != nil {
		t.Fatalf("first BufferJitterFrameAt failed: %v", err)
	}
	if err := receiver.Lifecycle().BufferJitterFrameAt(second, 1_030); err == nil {
		t.Fatal("duplicate seq overwrite was accepted; expected error")
	}

	eval := receiver.Lifecycle().EvaluateJitter(1_120, policy.JitterBufferMs, policy.LateThresholdMs)
	if len(eval.Emitted) != 1 {
		t.Fatalf("emitted count = %d, want 1", len(eval.Emitted))
	}
	if got := eval.Emitted[0].Payload[0]; got != 0xAA {
		t.Fatalf("buffered payload[0] = 0x%02X, want original 0xAA", got)
	}
}

func TestUT014_EvaluateJitterBoundedWithoutExpectedFrameCount(t *testing.T) {
	policy := DefaultReceiverPolicy()
	policy.JitterBufferMs = 120
	policy.LateThresholdMs = 500
	receiver := receiverJitterNoExpectedCountSetup(t, policy)

	highSeq := uint32(maxJitterEvaluationSteps + 10)
	if err := receiver.Lifecycle().BufferJitterFrameAt(makeVoiceFrame(highSeq), 1_020); err != nil {
		t.Fatalf("BufferJitterFrameAt high seq failed: %v", err)
	}

	eval := receiver.Lifecycle().EvaluateJitter(1_000_000, policy.JitterBufferMs, policy.LateThresholdMs)
	if len(eval.Missing) != maxJitterEvaluationSteps {
		t.Fatalf("missing count = %d, want bounded %d", len(eval.Missing), maxJitterEvaluationSteps)
	}
	if len(eval.Emitted) != 0 {
		t.Fatalf("emitted count = %d, want 0 before bounded gap is consumed", len(eval.Emitted))
	}
}

func frameSeqs(frames []protocol.Frame) []uint32 {
	seqs := make([]uint32, 0, len(frames))
	for _, f := range frames {
		seqs = append(seqs, f.Seq)
	}
	return seqs
}

func receiverJitterNoExpectedCountSetup(t *testing.T, policy ReceiverPolicy) *ReceiverStation {
	t.Helper()
	life := NewReceiverLifecycle(DefaultLifecyclePolicy())
	receiver := NewReceiverStation(3, life, policy)
	if err := receiver.ProcessCallStart(makeCallStartFrameWithoutExpectedCount(t), 1_000); err != nil {
		t.Fatalf("ProcessCallStart without expected count: %v", err)
	}
	return receiver
}

func makeCallStartFrameWithoutExpectedCount(t *testing.T) protocol.Frame {
	t.Helper()
	raw, err := protocol.EncodeCallStartPayload(protocol.CallStartPayload{
		FramePeriodMs:   80,
		PayloadMode:     "dummy",
		SlotModeEnabled: false,
		TTLDefault:      1,
	})
	if err != nil {
		t.Fatalf("EncodeCallStartPayload: %v", err)
	}
	return canonicalizeFrameForTest(protocol.Frame{
		Version:     protocol.Version,
		FrameType:   protocol.FrameTypeCallStart,
		NetID:       testNetID,
		SourceID:    testSource,
		GroupID:     testGroupID,
		RelayID:     0,
		CallID:      testCallID,
		Seq:         0,
		Hop:         0,
		TTL:         1,
		Flags:       protocol.FlagCallStart,
		CreatedAtMs: 1_000,
		SentAtMs:    1_000,
		PayloadLen:  uint16(len(raw)),
		Payload:     raw,
	})
}
