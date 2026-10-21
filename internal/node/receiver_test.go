package node

import (
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/protocol"
)

// receiverTestSetup creates a receiver with an active call so voice
// frames can be processed. Returns the receiver.
func receiverTestSetup(t *testing.T) *ReceiverStation {
	t.Helper()
	life := NewReceiverLifecycle(DefaultLifecyclePolicy())
	receiver := NewReceiverStation(3, life, DefaultReceiverPolicy())
	start := makeCallStartFrame(t)
	if err := receiver.ProcessCallStart(start, 1_000); err != nil {
		t.Fatalf("ProcessCallStart: %v", err)
	}
	return receiver
}

// makeRelayedVoiceFrame creates a relayed copy of a voice frame with
// the same origin_key as makeVoiceFrame(seq) but with relay_id, hop,
// and flags set for a relayed frame. The frame is canonicalized
// (encode → decode) so CRCs are valid.
func makeRelayedVoiceFrame(seq uint32, relayID uint16) protocol.Frame {
	return makeVoiceFrameWith(seq, func(f *protocol.Frame) {
		f.RelayID = relayID
		f.Hop = 1
		f.TTL = 0
		f.Flags = (f.Flags &^ protocol.FlagDirect) | protocol.FlagRelayed
	})
}

// TestUT006_DirectThenRelayedSuppressed is AC-1: Receiver gets a
// direct frame and then a relayed copy with the same Origin Key; only
// the first accepted copy reaches jitter buffer.
//
// Traceability: UT-006 (FR-4).
func TestUT006_DirectThenRelayedSuppressed(t *testing.T) {
	receiver := receiverTestSetup(t)
	direct := makeVoiceFrame(0)
	relayed := makeRelayedVoiceFrame(0, 2)

	// First arrival (direct) → accepted.
	r1 := receiver.ProcessVoiceFrame(direct, 1_000)
	if r1.Accepted == nil {
		t.Fatalf("first (direct) frame not accepted; dropped=%v suppressed=%v", r1.Dropped, r1.Suppressed)
	}
	if r1.Suppressed != nil {
		t.Fatal("first (direct) frame was suppressed; expected accept")
	}
	if r1.Dropped != nil {
		t.Fatalf("first (direct) frame dropped: %s", r1.Dropped.Reason)
	}

	// Second arrival (relayed) with same origin_key → suppressed.
	r2 := receiver.ProcessVoiceFrame(relayed, 1_050)
	if r2.Suppressed == nil {
		t.Fatal("second (relayed) frame not suppressed; expected duplicate suppression")
	}
	if r2.Accepted != nil {
		t.Fatal("second (relayed) frame was accepted; expected suppression")
	}

	// AC-1: only one frame in jitter buffer.
	seqs := receiver.Lifecycle().BufferedSeqs(CallKey{NetID: testNetID, SourceID: testSource, CallID: testCallID})
	if len(seqs) != 1 {
		t.Errorf("jitter buffer has %d frames, want 1 (only first accepted)", len(seqs))
	}
}

// TestUT006_RelayedThenDirectSuppressed proves order-independence: if
// the relayed copy arrives first, the direct copy is suppressed.
//
// Traceability: UT-006 (FR-4).
func TestUT006_RelayedThenDirectSuppressed(t *testing.T) {
	receiver := receiverTestSetup(t)
	relayed := makeRelayedVoiceFrame(0, 2)
	direct := makeVoiceFrame(0)

	r1 := receiver.ProcessVoiceFrame(relayed, 1_000)
	if r1.Accepted == nil {
		t.Fatalf("first (relayed) frame not accepted; dropped=%v", r1.Dropped)
	}

	r2 := receiver.ProcessVoiceFrame(direct, 1_050)
	if r2.Suppressed == nil {
		t.Fatal("second (direct) frame not suppressed; expected duplicate suppression")
	}

	if receiver.Counters().RelayedFramesAccepted != 1 {
		t.Errorf("relayed_frames_accepted = %d, want 1", receiver.Counters().RelayedFramesAccepted)
	}
	if receiver.Counters().DirectFramesAccepted != 0 {
		t.Errorf("direct_frames_accepted = %d, want 0 (direct was suppressed)", receiver.Counters().DirectFramesAccepted)
	}
}

// TestUT006_MultipleDuplicatesAllSuppressed proves that after the
// first accept, N additional copies are all suppressed.
//
// Traceability: UT-006 (FR-4).
func TestUT006_MultipleDuplicatesAllSuppressed(t *testing.T) {
	receiver := receiverTestSetup(t)
	direct := makeVoiceFrame(0)

	r1 := receiver.ProcessVoiceFrame(direct, 1_000)
	if r1.Accepted == nil {
		t.Fatalf("first frame not accepted; dropped=%v", r1.Dropped)
	}

	for i := 0; i < 5; i++ {
		relayed := makeRelayedVoiceFrame(0, 2)
		ri := receiver.ProcessVoiceFrame(relayed, 1_050+int64(i)*10)
		if ri.Suppressed == nil {
			t.Fatalf("duplicate %d not suppressed", i+1)
		}
	}

	if receiver.Counters().DuplicatesSuppressed != 5 {
		t.Errorf("duplicates_suppressed = %d, want 5", receiver.Counters().DuplicatesSuppressed)
	}
}

// TestUT007_RelayedOnlyAccepted is AC-2: Receiver misses the direct
// frame but receives the relayed copy; Receiver accepts it.
//
// Traceability: UT-007 (FR-4).
func TestUT007_RelayedOnlyAccepted(t *testing.T) {
	receiver := receiverTestSetup(t)
	relayed := makeRelayedVoiceFrame(5, 2)

	r := receiver.ProcessVoiceFrame(relayed, 1_400)
	if r.Accepted == nil {
		t.Fatalf("relayed-only frame not accepted; dropped=%v suppressed=%v", r.Dropped, r.Suppressed)
	}
	if r.Suppressed != nil {
		t.Fatal("relayed-only frame was suppressed; expected accept (no prior direct)")
	}

	if receiver.Counters().RelayedFramesAccepted != 1 {
		t.Errorf("relayed_frames_accepted = %d, want 1", receiver.Counters().RelayedFramesAccepted)
	}
	if receiver.Counters().DirectFramesAccepted != 0 {
		t.Errorf("direct_frames_accepted = %d, want 0", receiver.Counters().DirectFramesAccepted)
	}
}

// TestUT006_DuplicatesSuppressedCounter is AC-3: after a duplicate
// scenario, duplicates_suppressed > 0.
//
// Traceability: UT-006 (FR-4).
func TestUT006_DuplicatesSuppressedCounter(t *testing.T) {
	receiver := receiverTestSetup(t)
	direct := makeVoiceFrame(0)
	relayed := makeRelayedVoiceFrame(0, 2)

	receiver.ProcessVoiceFrame(direct, 1_000)
	receiver.ProcessVoiceFrame(relayed, 1_050)

	c := receiver.Counters()
	if c.DuplicatesSuppressed == 0 {
		t.Error("duplicates_suppressed = 0, want > 0 for duplicate scenario (AC-3)")
	}
	if c.DuplicatesSuppressed != 1 {
		t.Errorf("duplicates_suppressed = %d, want 1", c.DuplicatesSuppressed)
	}
}

// TestReceiver_InvalidFrameDropped proves that a frame with mutated
// payload (failing VerifyIntegrity) is dropped as invalid.
func TestReceiver_InvalidFrameDropped(t *testing.T) {
	receiver := receiverTestSetup(t)
	frame := makeVoiceFrame(0)
	frame.Payload[0] ^= 0xFF // mutate after canonicalization

	r := receiver.ProcessVoiceFrame(frame, 1_000)
	if r.Dropped == nil {
		t.Fatal("frame with mutated payload not dropped; expected invalid")
	}
	if r.Dropped.Reason != DropReasonInvalid {
		t.Errorf("drop reason = %s, want %s", r.Dropped.Reason, DropReasonInvalid)
	}
}

// TestReceiver_MutatedHeaderFieldDropped proves that a frame with a
// mutated header field (failing VerifyIntegrity) is dropped as invalid.
func TestReceiver_MutatedHeaderFieldDropped(t *testing.T) {
	receiver := receiverTestSetup(t)
	frame := makeVoiceFrame(0)
	frame.TTL = 99 // mutate after canonicalization; CRCs now stale

	r := receiver.ProcessVoiceFrame(frame, 1_000)
	if r.Dropped == nil {
		t.Fatal("frame with mutated header not dropped; expected invalid")
	}
	if r.Dropped.Reason != DropReasonInvalid {
		t.Errorf("drop reason = %s, want %s", r.Dropped.Reason, DropReasonInvalid)
	}
}

// TestReceiver_WrongGroupDropped proves that a frame whose group_id
// is not in the receiver's SubscribedGroups is dropped.
func TestReceiver_WrongGroupDropped(t *testing.T) {
	life := NewReceiverLifecycle(DefaultLifecyclePolicy())
	receiver := NewReceiverStation(3, life, ReceiverPolicy{
		StaleThresholdMs: DefaultReceiverStaleThresholdMs,
		SubscribedGroups: map[uint16]bool{testGroupID: true},
	})
	start := makeCallStartFrame(t)
	if err := receiver.ProcessCallStart(start, 1_000); err != nil {
		t.Fatalf("ProcessCallStart: %v", err)
	}

	wrongGroup := makeVoiceFrameWith(0, func(f *protocol.Frame) {
		f.GroupID = 999
	})

	r := receiver.ProcessVoiceFrame(wrongGroup, 1_000)
	if r.Dropped == nil {
		t.Fatal("wrong-group frame not dropped")
	}
	if r.Dropped.Reason != DropReasonWrongGroup {
		t.Errorf("drop reason = %s, want %s", r.Dropped.Reason, DropReasonWrongGroup)
	}
}

// TestReceiver_EmptySubscribedGroupsMeansAll proves that an empty
// SubscribedGroups map means "subscribe to all groups".
func TestReceiver_EmptySubscribedGroupsMeansAll(t *testing.T) {
	receiver := receiverTestSetup(t)

	frame := makeVoiceFrame(0)
	r := receiver.ProcessVoiceFrame(frame, 1_000)
	if r.Accepted == nil {
		t.Fatalf("frame not accepted with empty SubscribedGroups; dropped=%v", r.Dropped)
	}
}

// TestReceiver_UnsupportedFrameTypeDropped proves that a non-voice
// frame reaching ProcessVoiceFrame is dropped.
func TestReceiver_UnsupportedFrameTypeDropped(t *testing.T) {
	receiver := receiverTestSetup(t)

	start := makeCallStartFrame(t) // frame_type == call_start
	r := receiver.ProcessVoiceFrame(start, 1_000)
	if r.Dropped == nil {
		t.Fatal("non-voice frame not dropped")
	}
	if r.Dropped.Reason != DropReasonUnsupportedFrameType {
		t.Errorf("drop reason = %s, want %s", r.Dropped.Reason, DropReasonUnsupportedFrameType)
	}
}

// TestReceiver_MalformedVoiceFlagsDropped proves that voice frames must
// carry exactly one path flag (direct or relayed) and must not carry
// call lifecycle flags.
func TestReceiver_MalformedVoiceFlagsDropped(t *testing.T) {
	cases := []struct {
		name  string
		flags uint16
	}{
		{name: "neither_direct_nor_relayed", flags: 0},
		{name: "call_start_on_voice", flags: protocol.FlagDirect | protocol.FlagCallStart},
		{name: "call_end_on_voice", flags: protocol.FlagDirect | protocol.FlagCallEnd},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			receiver := receiverTestSetup(t)
			frame := makeVoiceFrameWith(0, func(f *protocol.Frame) {
				f.Flags = c.flags
			})

			result := receiver.ProcessVoiceFrame(frame, 1_000)
			if result.Accepted != nil {
				t.Fatal("malformed voice flags were accepted")
			}
			if result.Dropped == nil || result.Dropped.Reason != DropReasonInvalid {
				t.Fatalf("expected invalid drop, got %+v", result.Dropped)
			}
			if receiver.Lifecycle().Accepted(frame.OriginKey()) {
				t.Fatal("malformed voice frame polluted accepted cache")
			}
			if got := len(receiver.Lifecycle().BufferedSeqs(CallKeyFromFrame(frame))); got != 0 {
				t.Fatalf("malformed voice frame reached jitter buffer; buffered seq count=%d", got)
			}
		})
	}
}

// TestReceiver_CallNotActiveDropped proves that a voice frame for an
// inactive or unknown call is dropped.
func TestReceiver_CallNotActiveDropped(t *testing.T) {
	life := NewReceiverLifecycle(DefaultLifecyclePolicy())
	receiver := NewReceiverStation(3, life, DefaultReceiverPolicy())
	// No ProcessCallStart → call not active.

	frame := makeVoiceFrame(0)
	r := receiver.ProcessVoiceFrame(frame, 1_000)
	if r.Dropped == nil {
		t.Fatal("voice frame for inactive call not dropped")
	}
	if r.Dropped.Reason != DropReasonCallNotActive {
		t.Errorf("drop reason = %s, want %s", r.Dropped.Reason, DropReasonCallNotActive)
	}
}

// TestReceiver_StaleFrameDropped is AC from §11: a frame older than
// receiver_stale_threshold_ms is dropped.
func TestReceiver_StaleFrameDropped(t *testing.T) {
	receiver := receiverTestSetup(t)
	frame := makeVoiceFrame(0) // CreatedAtMs = 1_000

	// nowMs = 1_000 + 1001 = 2001 → age = 1001 > 1000 threshold.
	r := receiver.ProcessVoiceFrame(frame, 2_001)
	if r.Dropped == nil {
		t.Fatal("stale frame not dropped")
	}
	if r.Dropped.Reason != DropReasonStaleFrame {
		t.Errorf("drop reason = %s, want %s", r.Dropped.Reason, DropReasonStaleFrame)
	}
}

// TestReceiver_StaleAtExactThresholdAccepted proves that a frame with
// age == threshold is NOT stale (check is > not >=).
func TestReceiver_StaleAtExactThresholdAccepted(t *testing.T) {
	policy := DefaultReceiverPolicy()
	policy.LateThresholdMs = policy.StaleThresholdMs + 1
	life := NewReceiverLifecycle(DefaultLifecyclePolicy())
	receiver := NewReceiverStation(3, life, policy)
	if err := receiver.ProcessCallStart(makeCallStartFrame(t), 1_000); err != nil {
		t.Fatalf("ProcessCallStart: %v", err)
	}
	frame := makeVoiceFrame(0) // CreatedAtMs = 1_000

	// nowMs = 2_000 → age = 1_000 == threshold → not stale.
	r := receiver.ProcessVoiceFrame(frame, 2_000)
	if r.Dropped != nil {
		t.Errorf("frame at exact threshold dropped: %s", r.Dropped.Reason)
	}
	if r.Accepted == nil {
		t.Fatal("frame at exact threshold not accepted")
	}
}

// TestReceiver_FutureFrameRejected proves that a frame with
// CreatedAtMs > nowMs is rejected as invalid.
func TestReceiver_FutureFrameRejected(t *testing.T) {
	receiver := receiverTestSetup(t)
	frame := makeVoiceFrame(5) // CreatedAtMs = 1_400

	// nowMs = 1_300 < 1_400 → age = -100 < 0 → invalid.
	r := receiver.ProcessVoiceFrame(frame, 1_300)
	if r.Dropped == nil {
		t.Fatal("future frame not dropped")
	}
	if r.Dropped.Reason != DropReasonInvalid {
		t.Errorf("drop reason = %s, want %s", r.Dropped.Reason, DropReasonInvalid)
	}
}

// TestReceiver_GroupConsistencyWithCallStart proves that a voice frame
// with a different group_id than the call_start's group_id is dropped
// as wrong_group.
func TestReceiver_GroupConsistencyWithCallStart(t *testing.T) {
	receiver := receiverTestSetup(t)

	wrongGroup := makeVoiceFrameWith(0, func(f *protocol.Frame) {
		f.GroupID = 999
	})

	r := receiver.ProcessVoiceFrame(wrongGroup, 1_000)
	if r.Dropped == nil {
		t.Fatal("wrong-group voice frame not dropped")
	}
	if r.Dropped.Reason != DropReasonWrongGroup {
		t.Errorf("drop reason = %s, want %s", r.Dropped.Reason, DropReasonWrongGroup)
	}
}

// TestReceiver_AcceptedIntoJitter proves that an accepted frame is
// stored in the jitter buffer.
func TestReceiver_AcceptedIntoJitter(t *testing.T) {
	receiver := receiverTestSetup(t)
	frame := makeVoiceFrame(0)

	r := receiver.ProcessVoiceFrame(frame, 1_000)
	if r.Accepted == nil {
		t.Fatalf("frame not accepted; dropped=%v", r.Dropped)
	}

	key := CallKey{NetID: testNetID, SourceID: testSource, CallID: testCallID}
	seqs := receiver.Lifecycle().BufferedSeqs(key)
	if len(seqs) != 1 {
		t.Fatalf("jitter buffer has %d frames, want 1", len(seqs))
	}
	if seqs[0] != 0 {
		t.Errorf("jitter seq = %d, want 0", seqs[0])
	}
}

// TestReceiver_DirectFramesAcceptedCounter proves that accepting a
// direct frame increments direct_frames_accepted.
func TestReceiver_DirectFramesAcceptedCounter(t *testing.T) {
	receiver := receiverTestSetup(t)
	direct := makeVoiceFrame(0)

	receiver.ProcessVoiceFrame(direct, 1_000)

	c := receiver.Counters()
	if c.DirectFramesAccepted != 1 {
		t.Errorf("direct_frames_accepted = %d, want 1", c.DirectFramesAccepted)
	}
	if c.RelayedFramesAccepted != 0 {
		t.Errorf("relayed_frames_accepted = %d, want 0", c.RelayedFramesAccepted)
	}
	if c.FramesAccepted != 1 {
		t.Errorf("frames_accepted = %d, want 1", c.FramesAccepted)
	}
}

// TestReceiver_RelayedFramesAcceptedCounter proves that accepting a
// relayed frame increments relayed_frames_accepted.
func TestReceiver_RelayedFramesAcceptedCounter(t *testing.T) {
	receiver := receiverTestSetup(t)
	relayed := makeRelayedVoiceFrame(0, 2)

	receiver.ProcessVoiceFrame(relayed, 1_000)

	c := receiver.Counters()
	if c.RelayedFramesAccepted != 1 {
		t.Errorf("relayed_frames_accepted = %d, want 1", c.RelayedFramesAccepted)
	}
	if c.DirectFramesAccepted != 0 {
		t.Errorf("direct_frames_accepted = %d, want 0", c.DirectFramesAccepted)
	}
}

// TestReceiver_FramesReceivedCounter proves that frames_received
// counts all ProcessVoiceFrame calls (accepted, suppressed, and
// dropped).
func TestReceiver_FramesReceivedCounter(t *testing.T) {
	receiver := receiverTestSetup(t)
	direct := makeVoiceFrame(0)
	relayed := makeRelayedVoiceFrame(0, 2)
	stale := makeVoiceFrame(1)

	// 1 accepted (direct)
	receiver.ProcessVoiceFrame(direct, 1_000)
	// 1 suppressed (relayed duplicate)
	receiver.ProcessVoiceFrame(relayed, 1_050)
	// 1 dropped (stale: seq=1 CreatedAtMs=1080, nowMs=3001 → age=1921 > 1000)
	receiver.ProcessVoiceFrame(stale, 3_001)

	c := receiver.Counters()
	if c.FramesReceived != 3 {
		t.Errorf("frames_received = %d, want 3", c.FramesReceived)
	}
	if c.FramesAccepted != 1 {
		t.Errorf("frames_accepted = %d, want 1", c.FramesAccepted)
	}
	if c.DuplicatesSuppressed != 1 {
		t.Errorf("duplicates_suppressed = %d, want 1", c.DuplicatesSuppressed)
	}
}

// TestReceiver_DropCountersSnapshot proves that the DroppedByReason
// map returned by Counters() is independent of internal state.
func TestReceiver_DropCountersSnapshot(t *testing.T) {
	receiver := receiverTestSetup(t)
	stale := makeVoiceFrame(0)

	receiver.ProcessVoiceFrame(stale, 2_001) // stale drop

	snap := receiver.Counters()
	snap.DroppedByReason[DropReasonStaleFrame] = 999

	snap2 := receiver.Counters()
	if snap2.DroppedByReason[DropReasonStaleFrame] == 999 {
		t.Error("mutating snapshot affected internal drop counters")
	}
}

// TestReceiver_SubscribedGroupsCloned proves that the SubscribedGroups
// map is cloned at construction, so post-construction mutation of the
// caller's map does not affect the receiver.
func TestReceiver_SubscribedGroupsCloned(t *testing.T) {
	groups := map[uint16]bool{testGroupID: true}
	life := NewReceiverLifecycle(DefaultLifecyclePolicy())
	receiver := NewReceiverStation(3, life, ReceiverPolicy{
		StaleThresholdMs: DefaultReceiverStaleThresholdMs,
		SubscribedGroups: groups,
	})
	start := makeCallStartFrame(t)
	if err := receiver.ProcessCallStart(start, 1_000); err != nil {
		t.Fatalf("ProcessCallStart: %v", err)
	}

	// Mutate caller's map after construction.
	groups[999] = true
	delete(groups, testGroupID)

	// A new group added after construction must not become subscribed.
	if err := receiver.ProcessCallStart(makeCallStartFrameWith(t, 999, 999), 1_100); err == nil {
		t.Fatal("call_start for group 999 accepted after caller map mutation; expected error")
	}

	// A group deleted from the caller map after construction must remain
	// subscribed in the receiver's cloned policy.
	r := receiver.ProcessVoiceFrame(makeVoiceFrame(0), 1_000)
	if r.Accepted == nil {
		t.Fatalf("frame for original group was not accepted after caller map mutation; dropped=%v", r.Dropped)
	}
}

func TestReceiver_CallStartValidatesBeforeGroupSubscription(t *testing.T) {
	life := NewReceiverLifecycle(DefaultLifecyclePolicy())
	receiver := NewReceiverStation(3, life, ReceiverPolicy{
		StaleThresholdMs: DefaultReceiverStaleThresholdMs,
		SubscribedGroups: map[uint16]bool{200: true},
	})
	frame := makeCallStartFrameWith(t, testGroupID, testCallID)
	frame.Payload[0] ^= 0xFF

	err := receiver.ProcessCallStart(frame, 1_000)
	if err == nil {
		t.Fatal("corrupted call_start accepted; expected error")
	}
	if strings.Contains(err.Error(), "not in subscribed groups") {
		t.Fatalf("ProcessCallStart checked group before integrity validation: %v", err)
	}
	if receiver.Lifecycle().HasCall(CallKeyFromFrame(frame)) {
		t.Fatal("corrupted call_start created receiver call state")
	}
}

func TestReceiver_CallEndValidatesBeforeGroupSubscription(t *testing.T) {
	life := NewReceiverLifecycle(DefaultLifecyclePolicy())
	receiver := NewReceiverStation(3, life, ReceiverPolicy{
		StaleThresholdMs: DefaultReceiverStaleThresholdMs,
		SubscribedGroups: map[uint16]bool{testGroupID: true},
	})
	if err := receiver.ProcessCallStart(makeCallStartFrame(t), 1_000); err != nil {
		t.Fatalf("ProcessCallStart: %v", err)
	}
	frame := makeCallEndFrameWith(t, 999, testCallID, 0)
	frame.Payload[0] ^= 0xFF

	err := receiver.ProcessCallEnd(frame, 2_000)
	if err == nil {
		t.Fatal("corrupted call_end accepted; expected error")
	}
	if strings.Contains(err.Error(), "not in subscribed groups") {
		t.Fatalf("ProcessCallEnd checked group before integrity validation: %v", err)
	}
	state, ok := receiver.Lifecycle().Call(CallKeyFromFrame(frame))
	if !ok || !state.Active {
		t.Fatal("corrupted call_end ended or removed receiver call state")
	}
}

func TestReceiver_StaleAndFutureCallStartRejected(t *testing.T) {
	cases := []struct {
		name  string
		nowMs int64
	}{
		{name: "stale", nowMs: 2_001},
		{name: "future", nowMs: 999},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			receiver := NewReceiverStation(3, NewReceiverLifecycle(DefaultLifecyclePolicy()), DefaultReceiverPolicy())
			start := makeCallStartFrame(t)

			err := receiver.ProcessCallStart(start, c.nowMs)
			if err == nil {
				t.Fatalf("%s call_start accepted; expected error", c.name)
			}
			if receiver.Lifecycle().HasCall(CallKeyFromFrame(start)) {
				t.Fatalf("%s call_start created receiver call state", c.name)
			}
		})
	}
}

func TestReceiver_StaleAndFutureCallEndRejected(t *testing.T) {
	cases := []struct {
		name  string
		nowMs int64
	}{
		{name: "stale", nowMs: 3_001},
		{name: "future", nowMs: 1_999},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			receiver := receiverTestSetup(t)
			end := makeCallEndFrame(t, 0)

			err := receiver.ProcessCallEnd(end, c.nowMs)
			if err == nil {
				t.Fatalf("%s call_end accepted; expected error", c.name)
			}
			state, ok := receiver.Lifecycle().Call(CallKeyFromFrame(end))
			if !ok || !state.Active {
				t.Fatalf("%s call_end ended or removed receiver call state", c.name)
			}
		})
	}
}

// TestReceiver_DropResultPayloadDefensiveCopy proves that the
// DropResult.Frame payload is an independent copy: mutating the
// caller's frame payload after drop() does not affect the DropResult.
func TestReceiver_DropResultPayloadDefensiveCopy(t *testing.T) {
	receiver := receiverTestSetup(t)
	frame := makeVoiceFrame(0)

	// Mutate payload to make frame invalid (CRC mismatch) so it's dropped.
	frame.Payload[0] ^= 0xFF

	r := receiver.ProcessVoiceFrame(frame, 1_000)
	if r.Dropped == nil {
		t.Fatal("frame not dropped; expected invalid")
	}

	// Save the DropResult's payload state.
	beforeMutation := make([]byte, len(r.Dropped.Frame.Payload))
	copy(beforeMutation, r.Dropped.Frame.Payload)

	// Mutate the original frame's payload after drop.
	frame.Payload[0] ^= 0xFF

	// DropResult payload should not have changed.
	for i := range r.Dropped.Frame.Payload {
		if r.Dropped.Frame.Payload[i] != beforeMutation[i] {
			t.Fatal("DropResult payload was mutated by caller; expected defensive copy")
		}
	}
}

// TestReceiver_SuppressedResultPayloadDefensiveCopy proves that the
// SuppressedResult.Frame payload is an independent copy.
func TestReceiver_SuppressedResultPayloadDefensiveCopy(t *testing.T) {
	receiver := receiverTestSetup(t)
	direct := makeVoiceFrame(0)
	relayed := makeRelayedVoiceFrame(0, 2)

	receiver.ProcessVoiceFrame(direct, 1_000)
	r := receiver.ProcessVoiceFrame(relayed, 1_050)
	if r.Suppressed == nil {
		t.Fatal("second frame not suppressed")
	}

	beforeMutation := make([]byte, len(r.Suppressed.Frame.Payload))
	copy(beforeMutation, r.Suppressed.Frame.Payload)

	// Mutate the original frame's payload after suppression.
	relayed.Payload[0] ^= 0xFF

	for i := range r.Suppressed.Frame.Payload {
		if r.Suppressed.Frame.Payload[i] != beforeMutation[i] {
			t.Fatal("SuppressedResult payload was mutated by caller; expected defensive copy")
		}
	}
}

// TestReceiver_AcceptedPayloadIndependentFromJitter proves that the
// AcceptResult.Accepted frame's payload is independent of the jitter
// buffer entry.
func TestReceiver_AcceptedPayloadIndependentFromJitter(t *testing.T) {
	receiver := receiverTestSetup(t)
	frame := makeVoiceFrame(0)

	r := receiver.ProcessVoiceFrame(frame, 1_000)
	if r.Accepted == nil {
		t.Fatal("frame not accepted")
	}

	// Mutate the returned accepted frame's payload.
	r.Accepted.Payload[0] ^= 0xFF

	// Jitter buffer entry should not be affected.
	key := CallKey{NetID: testNetID, SourceID: testSource, CallID: testCallID}
	seqs := receiver.Lifecycle().BufferedSeqs(key)
	if len(seqs) != 1 {
		t.Fatalf("jitter has %d frames, want 1", len(seqs))
	}

	// Retrieve the buffered frame from lifecycle.
	jitterMap := receiver.Lifecycle().jitter[key]
	bufferedFrame := jitterMap[0]

	if bufferedFrame.Frame.Payload[0] == r.Accepted.Payload[0] {
		t.Fatal("mutating AcceptResult.Accepted payload affected jitter buffer; expected independent copy")
	}
}

// TestReceiver_AcceptedPayloadIndependentFromCaller proves that
// mutating the caller's frame payload after acceptance does not affect
// the jitter buffer.
func TestReceiver_AcceptedPayloadIndependentFromCaller(t *testing.T) {
	receiver := receiverTestSetup(t)
	frame := makeVoiceFrame(0)

	r := receiver.ProcessVoiceFrame(frame, 1_000)
	if r.Accepted == nil {
		t.Fatal("frame not accepted")
	}
	_ = r

	// Mutate the original frame's payload after ProcessVoiceFrame.
	frame.Payload[0] ^= 0xFF

	// Jitter buffer entry should not be affected.
	key := CallKey{NetID: testNetID, SourceID: testSource, CallID: testCallID}
	jitterMap := receiver.Lifecycle().jitter[key]
	bufferedFrame := jitterMap[0]

	expected := byte(0) // seq=0 → payload[0] = 0x00
	if bufferedFrame.Frame.Payload[0] != expected {
		t.Errorf("jitter payload[0] = 0x%02X, want 0x%02X (caller mutation propagated)", bufferedFrame.Frame.Payload[0], expected)
	}
}

// TestReceiver_ZeroLengthNonNilPayloadCopies proves receiver result and
// jitter copies preserve non-nil zero-length payload slices.
func TestReceiver_ZeroLengthNonNilPayloadCopies(t *testing.T) {
	acceptedReceiver := receiverTestSetup(t)
	acceptedFrame := makeVoiceFrameWithPayload(0, make([]byte, 0), nil)
	acceptedFrame.Payload = make([]byte, 0, 8)

	accepted := acceptedReceiver.ProcessVoiceFrame(acceptedFrame, 1_000)
	if accepted.Accepted == nil {
		t.Fatalf("zero-length payload frame not accepted; dropped=%v", accepted.Dropped)
	}
	if accepted.Accepted.Payload == nil {
		t.Fatal("accepted zero-length payload became nil")
	}
	if cap(accepted.Accepted.Payload) != 0 {
		t.Fatalf("accepted zero-length payload has cap=%d, want 0 to avoid sharing backing storage", cap(accepted.Accepted.Payload))
	}
	jitterFrame := acceptedReceiver.Lifecycle().jitter[CallKeyFromFrame(acceptedFrame)][0]
	if jitterFrame.Frame.Payload == nil {
		t.Fatal("jitter zero-length payload became nil")
	}
	if cap(jitterFrame.Frame.Payload) != 0 {
		t.Fatalf("jitter zero-length payload has cap=%d, want 0 to avoid sharing backing storage", cap(jitterFrame.Frame.Payload))
	}

	suppressedReceiver := receiverTestSetup(t)
	first := makeVoiceFrameWithPayload(0, make([]byte, 0), nil)
	second := makeVoiceFrameWithPayload(0, make([]byte, 0), func(f *protocol.Frame) {
		f.RelayID = 2
		f.Hop = 1
		f.TTL = 0
		f.Flags = protocol.FlagRelayed
	})
	first.Payload = make([]byte, 0, 8)
	second.Payload = make([]byte, 0, 8)
	if r := suppressedReceiver.ProcessVoiceFrame(first, 1_000); r.Accepted == nil {
		t.Fatalf("first zero-length payload frame not accepted; dropped=%v", r.Dropped)
	}
	suppressed := suppressedReceiver.ProcessVoiceFrame(second, 1_050)
	if suppressed.Suppressed == nil {
		t.Fatalf("duplicate zero-length payload frame not suppressed; dropped=%v", suppressed.Dropped)
	}
	if suppressed.Suppressed.Frame.Payload == nil {
		t.Fatal("suppressed zero-length payload became nil")
	}
	if cap(suppressed.Suppressed.Frame.Payload) != 0 {
		t.Fatalf("suppressed zero-length payload has cap=%d, want 0 to avoid sharing backing storage", cap(suppressed.Suppressed.Frame.Payload))
	}

	dropReceiver := NewReceiverStation(3, NewReceiverLifecycle(DefaultLifecyclePolicy()), DefaultReceiverPolicy())
	droppedFrame := makeVoiceFrameWithPayload(0, make([]byte, 0), nil)
	droppedFrame.Payload = make([]byte, 0, 8)
	dropped := dropReceiver.ProcessVoiceFrame(droppedFrame, 1_000)
	if dropped.Dropped == nil {
		t.Fatal("zero-length payload frame without active call was not dropped")
	}
	if dropped.Dropped.Frame.Payload == nil {
		t.Fatal("dropped zero-length payload became nil")
	}
	if cap(dropped.Dropped.Frame.Payload) != 0 {
		t.Fatalf("dropped zero-length payload has cap=%d, want 0 to avoid sharing backing storage", cap(dropped.Dropped.Frame.Payload))
	}
}

// TestReceiver_NilLifecyclePanics proves that NewReceiverStation
// panics if the lifecycle is nil.
func TestReceiver_NilLifecyclePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewReceiverStation with nil lifecycle did not panic")
		}
	}()
	NewReceiverStation(3, nil, DefaultReceiverPolicy())
}

// TestReceiver_ZeroIDPanics proves that NewReceiverStation rejects
// numeric_id=0, which is reserved by frame-contract.md §4.3.
func TestReceiver_ZeroIDPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewReceiverStation with id=0 did not panic")
		}
	}()
	NewReceiverStation(0, NewReceiverLifecycle(DefaultLifecyclePolicy()), DefaultReceiverPolicy())
}

// TestReceiver_DuplicateSuppressedAfterAcceptedFrame proves that after
// a normal accept, a second arrival with the same origin_key is
// suppressed by the accepted cache.
func TestReceiver_DuplicateSuppressedAfterAcceptedFrame(t *testing.T) {
	receiver := receiverTestSetup(t)
	direct := makeVoiceFrame(0)

	// First: normal accept.
	r1 := receiver.ProcessVoiceFrame(direct, 1_000)
	if r1.Accepted == nil {
		t.Fatalf("first frame not accepted; dropped=%v", r1.Dropped)
	}

	// Second: relayed duplicate must be suppressed because origin_key is
	// already in accepted cache.
	relayed := makeRelayedVoiceFrame(0, 2)
	r2 := receiver.ProcessVoiceFrame(relayed, 1_050)
	if r2.Suppressed == nil {
		t.Fatal("second frame not suppressed; expected duplicate suppression after MarkAccepted")
	}

	// Verify accepted cache has the origin_key.
	originKey := direct.OriginKey()
	if !receiver.Lifecycle().Accepted(originKey) {
		t.Fatal("origin_key not in accepted cache after first accept")
	}
}

// TestReceiver_DirectAndRelayedDifferentSeqBothAccepted proves that
// direct frame for seq=0 and relayed frame for seq=1 (different
// origin_keys) are both accepted.
func TestReceiver_DirectAndRelayedDifferentSeqBothAccepted(t *testing.T) {
	receiver := receiverTestSetup(t)
	direct := makeVoiceFrame(0)            // origin_key seq=0
	relayed := makeRelayedVoiceFrame(1, 2) // origin_key seq=1

	r1 := receiver.ProcessVoiceFrame(direct, 1_000)
	if r1.Accepted == nil {
		t.Fatalf("direct seq=0 not accepted; dropped=%v", r1.Dropped)
	}

	r2 := receiver.ProcessVoiceFrame(relayed, 1_080)
	if r2.Accepted == nil {
		t.Fatalf("relayed seq=1 not accepted; dropped=%v", r2.Dropped)
	}

	c := receiver.Counters()
	if c.FramesAccepted != 2 {
		t.Errorf("frames_accepted = %d, want 2", c.FramesAccepted)
	}
	if c.DuplicatesSuppressed != 0 {
		t.Errorf("duplicates_suppressed = %d, want 0 (different origin_keys)", c.DuplicatesSuppressed)
	}
	if c.DirectFramesAccepted != 1 {
		t.Errorf("direct_frames_accepted = %d, want 1", c.DirectFramesAccepted)
	}
	if c.RelayedFramesAccepted != 1 {
		t.Errorf("relayed_frames_accepted = %d, want 1", c.RelayedFramesAccepted)
	}
}

// TestReceiver_ProcessCallStartAndEnd proves that the convenience
// methods delegate to the lifecycle correctly.
func TestReceiver_ProcessCallStartAndEnd(t *testing.T) {
	receiver := receiverTestSetup(t)

	key := CallKey{NetID: testNetID, SourceID: testSource, CallID: testCallID}
	state, ok := receiver.Lifecycle().Call(key)
	if !ok || !state.Active {
		t.Fatal("call not active after ProcessCallStart")
	}

	end := makeCallEndFrame(t, 10)
	if err := receiver.ProcessCallEnd(end, 2_000); err != nil {
		t.Fatalf("ProcessCallEnd: %v", err)
	}

	state, ok = receiver.Lifecycle().Call(key)
	if !ok {
		t.Fatal("call state missing after ProcessCallEnd")
	}
	if state.Active {
		t.Error("call still active after ProcessCallEnd")
	}
}

// TestReceiver_NegativeStaleThresholdPanics proves that constructing a
// receiver with StaleThresholdMs < 0 panics.
func TestReceiver_NegativeStaleThresholdPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewReceiverStation with StaleThresholdMs < 0 did not panic")
		}
	}()
	life := NewReceiverLifecycle(DefaultLifecyclePolicy())
	NewReceiverStation(3, life, ReceiverPolicy{StaleThresholdMs: -1})
}

// TestReceiver_RelayedTTLZeroAccepted_NoTTLCheck proves that a relayed
// frame with TTL=0 is accepted by the receiver. The receiver does NOT
// check TTL (Dev Notes: "No TTL check — TTL is relay's concern"). A
// relayed frame's TTL is decremented by the relay and may be 0.
func TestReceiver_RelayedTTLZeroAccepted_NoTTLCheck(t *testing.T) {
	receiver := receiverTestSetup(t)
	relayed := makeRelayedVoiceFrame(0, 2) // TTL=0 by helper default

	r := receiver.ProcessVoiceFrame(relayed, 1_000)
	if r.Accepted == nil {
		t.Fatalf("relayed frame with TTL=0 not accepted; dropped=%v", r.Dropped)
	}
	if r.Dropped != nil {
		t.Errorf("relayed frame with TTL=0 dropped: %s; receiver must not check TTL", r.Dropped.Reason)
	}
}

// TestReceiver_CallStartWrongGroupRejected proves that ProcessCallStart
// rejects a call_start frame whose group_id is not in SubscribedGroups
// (§7.3 step 2: group subscription check before call_start dispatch).
func TestReceiver_CallStartWrongGroupRejected(t *testing.T) {
	life := NewReceiverLifecycle(DefaultLifecyclePolicy())
	receiver := NewReceiverStation(3, life, ReceiverPolicy{
		StaleThresholdMs: DefaultReceiverStaleThresholdMs,
		SubscribedGroups: map[uint16]bool{200: true},
	})

	// call_start for group 100, but receiver subscribed to group 200.
	wrongGroupStart := makeCallStartFrameWith(t, 100, testCallID)
	err := receiver.ProcessCallStart(wrongGroupStart, 1_000)
	if err == nil {
		t.Fatal("ProcessCallStart accepted call_start for non-subscribed group; expected error")
	}
}

// TestReceiver_CallEndWrongGroupRejected proves that ProcessCallEnd
// rejects a call_end frame whose group_id is not in SubscribedGroups.
func TestReceiver_CallEndWrongGroupRejected(t *testing.T) {
	life := NewReceiverLifecycle(DefaultLifecyclePolicy())
	receiver := NewReceiverStation(3, life, ReceiverPolicy{
		StaleThresholdMs: DefaultReceiverStaleThresholdMs,
		SubscribedGroups: map[uint16]bool{testGroupID: true},
	})
	start := makeCallStartFrame(t) // group=testGroupID
	if err := receiver.ProcessCallStart(start, 1_000); err != nil {
		t.Fatalf("ProcessCallStart: %v", err)
	}

	// call_end for group 999, but receiver subscribed to group 100.
	wrongGroupEnd := makeCallEndFrameWith(t, 999, testCallID, 10)
	err := receiver.ProcessCallEnd(wrongGroupEnd, 2_000)
	if err == nil {
		t.Fatal("ProcessCallEnd accepted call_end for non-subscribed group; expected error")
	}
}
