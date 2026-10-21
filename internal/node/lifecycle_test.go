package node

import (
	"reflect"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/protocol"
)

const (
	testNetID   uint16 = 1
	testSource  uint16 = 10
	testGroupID uint16 = 100
	testCallID  uint32 = 9001
)

func TestUT013_CallStartInitializesRelayAndReceiverState(t *testing.T) {
	start := makeCallStartFrame(t)
	key := CallKey{NetID: testNetID, SourceID: testSource, CallID: testCallID}

	relay := NewRelayLifecycle(DefaultLifecyclePolicy())
	receiver := NewReceiverLifecycle(DefaultLifecyclePolicy())

	if err := relay.ProcessCallStart(start, 1_000); err != nil {
		t.Fatalf("relay ProcessCallStart failed: %v", err)
	}
	if err := receiver.ProcessCallStart(start, 1_000); err != nil {
		t.Fatalf("receiver ProcessCallStart failed: %v", err)
	}

	relayState, ok := relay.Call(key)
	if !ok {
		t.Fatal("relay call state missing after call_start")
	}
	receiverState, ok := receiver.Call(key)
	if !ok {
		t.Fatal("receiver call state missing after call_start")
	}

	for name, state := range map[string]CallState{
		"relay":    relayState,
		"receiver": receiverState,
	} {
		if !state.Active {
			t.Errorf("%s call Active = false, want true", name)
		}
		if state.StartedAtMs != 1_000 {
			t.Errorf("%s StartedAtMs = %d, want 1000", name, state.StartedAtMs)
		}
		if state.Start.FramePeriodMs != 80 {
			t.Errorf("%s FramePeriodMs = %d, want 80", name, state.Start.FramePeriodMs)
		}
		if state.Start.TTLDefault != 1 {
			t.Errorf("%s TTLDefault = %d, want 1", name, state.Start.TTLDefault)
		}
	}
}

func TestRelaySeenCacheEvictsOldestWithinCallScope(t *testing.T) {
	start := makeCallStartFrame(t)
	policy := DefaultLifecyclePolicy()
	policy.RelaySeenCacheMaxEntries = 2
	relay := NewRelayLifecycle(policy)

	if err := relay.ProcessCallStart(start, 1_000); err != nil {
		t.Fatalf("ProcessCallStart failed: %v", err)
	}

	first := makeVoiceFrame(0).OriginKey()
	second := makeVoiceFrame(1).OriginKey()
	third := makeVoiceFrame(2).OriginKey()
	for _, key := range []protocol.OriginKey{first, second, third} {
		if err := relay.MarkSeen(key); err != nil {
			t.Fatalf("MarkSeen(%+v) failed: %v", key, err)
		}
	}

	if relay.Seen(first) {
		t.Fatal("oldest origin_key still present after overflow")
	}
	if !relay.Seen(second) || !relay.Seen(third) {
		t.Fatal("newer origin_keys missing after oldest-first eviction")
	}
	if got := relay.SeenEntryCount(); got != 2 {
		t.Fatalf("SeenEntryCount = %d, want 2", got)
	}
	if got := relay.SeenEvictionCount(); got != 1 {
		t.Fatalf("SeenEvictionCount = %d, want 1", got)
	}
}

func TestUT013_CallEndFlushesReceiverJitterStateUpToLastSeq(t *testing.T) {
	start := makeCallStartFrame(t)
	key := CallKey{NetID: testNetID, SourceID: testSource, CallID: testCallID}

	receiver := NewReceiverLifecycle(DefaultLifecyclePolicy())
	if err := receiver.ProcessCallStart(start, 1_000); err != nil {
		t.Fatalf("ProcessCallStart failed: %v", err)
	}

	for seq := uint32(0); seq <= 3; seq++ {
		if err := receiver.BufferJitterFrame(makeVoiceFrame(seq)); err != nil {
			t.Fatalf("BufferJitterFrame seq=%d failed: %v", seq, err)
		}
	}

	eval, err := receiver.ProcessCallEnd(makeCallEndFrame(t, 2), 2_000)
	if err != nil {
		t.Fatalf("ProcessCallEnd failed: %v", err)
	}
	if got, want := frameSeqs(eval.Emitted), []uint32{0, 1, 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("call_end emitted seqs = %v, want %v", got, want)
	}
	if len(eval.Missing) != 0 {
		t.Fatalf("call_end missing = %+v, want none", eval.Missing)
	}

	gotSeqs := receiver.BufferedSeqs(key)
	wantSeqs := []uint32{3}
	if !reflect.DeepEqual(gotSeqs, wantSeqs) {
		t.Fatalf("BufferedSeqs after call_end = %v, want %v", gotSeqs, wantSeqs)
	}
	flushed, ok := receiver.FlushedUpTo(key)
	if !ok {
		t.Fatal("FlushedUpTo missing after call_end")
	}
	if flushed != 2 {
		t.Fatalf("FlushedUpTo = %d, want 2", flushed)
	}

	state, ok := receiver.Call(key)
	if !ok {
		t.Fatal("receiver call state removed before cleanup grace elapsed")
	}
	if state.Active {
		t.Error("receiver call Active = true after call_end, want false")
	}
	if state.EndedAtMs != 2_000 {
		t.Errorf("EndedAtMs = %d, want 2000", state.EndedAtMs)
	}
	if state.CleanupAtMs != 4_000 {
		t.Errorf("CleanupAtMs = %d, want 4000", state.CleanupAtMs)
	}
	if state.End.LastSeq != 2 {
		t.Errorf("End.LastSeq = %d, want 2", state.End.LastSeq)
	}
}

func TestUT013_CallEndFinalizationCountsMissingGaps(t *testing.T) {
	start := makeCallStartFrameWithExpectedCount(t, 3)

	receiver := NewReceiverLifecycle(DefaultLifecyclePolicy())
	if err := receiver.ProcessCallStart(start, 1_000); err != nil {
		t.Fatalf("ProcessCallStart failed: %v", err)
	}
	if err := receiver.BufferJitterFrame(makeVoiceFrame(0)); err != nil {
		t.Fatalf("BufferJitterFrame seq=0 failed: %v", err)
	}
	if err := receiver.BufferJitterFrame(makeVoiceFrame(2)); err != nil {
		t.Fatalf("BufferJitterFrame seq=2 failed: %v", err)
	}

	eval, err := receiver.ProcessCallEnd(makeCallEndFrame(t, 2), 2_000)
	if err != nil {
		t.Fatalf("ProcessCallEnd failed: %v", err)
	}
	if got, want := frameSeqs(eval.Emitted), []uint32{0, 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("call_end emitted seqs = %v, want %v", got, want)
	}
	if len(eval.Missing) != 1 || eval.Missing[0].Seq != 1 {
		t.Fatalf("call_end missing = %+v, want seq=1", eval.Missing)
	}
}

func TestUT013_CallEndFinalizationDoesNotCapLongCalls(t *testing.T) {
	const frameCount uint32 = maxJitterEvaluationSteps + 4
	start := makeCallStartFrameWithExpectedCount(t, frameCount)

	receiver := NewReceiverLifecycle(DefaultLifecyclePolicy())
	if err := receiver.ProcessCallStart(start, 1_000); err != nil {
		t.Fatalf("ProcessCallStart failed: %v", err)
	}
	for seq := uint32(0); seq < frameCount; seq++ {
		if err := receiver.BufferJitterFrame(makeVoiceFrame(seq)); err != nil {
			t.Fatalf("BufferJitterFrame seq=%d failed: %v", seq, err)
		}
	}

	eval, err := receiver.ProcessCallEnd(makeCallEndFrame(t, frameCount-1), 400_000)
	if err != nil {
		t.Fatalf("ProcessCallEnd failed: %v", err)
	}
	if len(eval.Emitted) != int(frameCount) {
		t.Fatalf("call_end emitted count = %d, want %d", len(eval.Emitted), frameCount)
	}
	if len(eval.Missing) != 0 {
		t.Fatalf("call_end missing = %+v, want none", eval.Missing)
	}
}

func TestUT013_CallEndFinalizationHandlesMaxUint32Seq(t *testing.T) {
	const maxSeq = ^uint32(0)
	start := makeCallStartFrame(t)
	key := CallKey{NetID: testNetID, SourceID: testSource, CallID: testCallID}

	receiver := NewReceiverLifecycle(DefaultLifecyclePolicy())
	if err := receiver.ProcessCallStart(start, 1_000); err != nil {
		t.Fatalf("ProcessCallStart failed: %v", err)
	}
	receiver.nextPlayoutSeq[key] = maxSeq
	if err := receiver.BufferJitterFrame(makeVoiceFrame(maxSeq)); err != nil {
		t.Fatalf("BufferJitterFrame max seq failed: %v", err)
	}

	eval, err := receiver.ProcessCallEnd(makeCallEndFrame(t, maxSeq), 2_000)
	if err != nil {
		t.Fatalf("ProcessCallEnd failed: %v", err)
	}
	if got, want := frameSeqs(eval.Emitted), []uint32{maxSeq}; !reflect.DeepEqual(got, want) {
		t.Fatalf("call_end emitted seqs = %v, want %v", got, want)
	}
	if len(eval.Missing) != 0 {
		t.Fatalf("call_end missing = %+v, want none", eval.Missing)
	}
}

func TestUT013_CleanupEvictsRelaySeenAndReceiverAcceptedCachesDeterministically(t *testing.T) {
	start := makeCallStartFrame(t)
	end := makeCallEndFrame(t, 1)
	key := CallKey{NetID: testNetID, SourceID: testSource, CallID: testCallID}
	origin0 := protocol.OriginKey{NetID: testNetID, SourceID: testSource, CallID: testCallID, Seq: 0}
	origin1 := protocol.OriginKey{NetID: testNetID, SourceID: testSource, CallID: testCallID, Seq: 1}

	relay := NewRelayLifecycle(DefaultLifecyclePolicy())
	receiver := NewReceiverLifecycle(DefaultLifecyclePolicy())
	if err := relay.ProcessCallStart(start, 1_000); err != nil {
		t.Fatalf("relay start failed: %v", err)
	}
	if err := receiver.ProcessCallStart(start, 1_000); err != nil {
		t.Fatalf("receiver start failed: %v", err)
	}
	if err := relay.MarkSeen(origin0); err != nil {
		t.Fatalf("MarkSeen origin0 failed: %v", err)
	}
	if err := relay.MarkSeen(origin1); err != nil {
		t.Fatalf("MarkSeen origin1 failed: %v", err)
	}
	if err := receiver.MarkAccepted(origin0); err != nil {
		t.Fatalf("MarkAccepted origin0 failed: %v", err)
	}
	if err := receiver.MarkAccepted(origin1); err != nil {
		t.Fatalf("MarkAccepted origin1 failed: %v", err)
	}

	if err := relay.ProcessCallEnd(end, 2_000); err != nil {
		t.Fatalf("relay end failed: %v", err)
	}
	if _, err := receiver.ProcessCallEnd(end, 2_000); err != nil {
		t.Fatalf("receiver end failed: %v", err)
	}

	relay.Cleanup(3_999)
	receiver.Cleanup(3_999)
	if !relay.Seen(origin0) || !relay.Seen(origin1) {
		t.Fatal("relay seen-cache evicted before cleanup grace elapsed")
	}
	if !receiver.Accepted(origin0) || !receiver.Accepted(origin1) {
		t.Fatal("receiver accepted-key cache evicted before cleanup grace elapsed")
	}
	if !relay.HasCall(key) || !receiver.HasCall(key) {
		t.Fatal("call state evicted before cleanup grace elapsed")
	}

	relay.Cleanup(4_000)
	receiver.Cleanup(4_000)
	if relay.Seen(origin0) || relay.Seen(origin1) {
		t.Fatal("relay seen-cache still present after cleanup grace elapsed")
	}
	if receiver.Accepted(origin0) || receiver.Accepted(origin1) {
		t.Fatal("receiver accepted-key cache still present after cleanup grace elapsed")
	}
	if relay.HasCall(key) {
		t.Fatal("relay call state still present after cleanup grace elapsed")
	}
	if receiver.HasCall(key) {
		t.Fatal("receiver call state still present after cleanup grace elapsed")
	}
}

func TestLifecycleRejectsConflictingDuplicateCallStart(t *testing.T) {
	start := makeCallStartFrame(t)
	conflicting := start
	raw, err := protocol.EncodeCallStartPayload(protocol.CallStartPayload{
		FramePeriodMs:   160,
		PayloadMode:     "dummy",
		SlotModeEnabled: false,
		TTLDefault:      1,
	})
	if err != nil {
		t.Fatalf("EncodeCallStartPayload: %v", err)
	}
	conflicting.Payload = raw
	conflicting.PayloadLen = uint16(len(raw))
	conflicting = canonicalizeFrameForTest(conflicting)

	relay := NewRelayLifecycle(DefaultLifecyclePolicy())
	receiver := NewReceiverLifecycle(DefaultLifecyclePolicy())
	if err := relay.ProcessCallStart(start, 1_000); err != nil {
		t.Fatalf("relay start failed: %v", err)
	}
	if err := receiver.ProcessCallStart(start, 1_000); err != nil {
		t.Fatalf("receiver start failed: %v", err)
	}

	if err := relay.ProcessCallStart(conflicting, 1_010); err == nil {
		t.Fatal("relay accepted conflicting duplicate call_start; expected error")
	}
	if err := receiver.ProcessCallStart(conflicting, 1_010); err == nil {
		t.Fatal("receiver accepted conflicting duplicate call_start; expected error")
	}
}

func TestLifecycleRejectsDuplicateCallStartWithDifferentGroup(t *testing.T) {
	start := makeCallStartFrame(t)
	wrongGroup := makeCallStartFrameWith(t, 999, testCallID)

	relay := NewRelayLifecycle(DefaultLifecyclePolicy())
	receiver := NewReceiverLifecycle(DefaultLifecyclePolicy())
	if err := relay.ProcessCallStart(start, 1_000); err != nil {
		t.Fatalf("relay start failed: %v", err)
	}
	if err := receiver.ProcessCallStart(start, 1_000); err != nil {
		t.Fatalf("receiver start failed: %v", err)
	}

	if err := relay.ProcessCallStart(wrongGroup, 1_010); err == nil {
		t.Fatal("relay accepted duplicate call_start with different group_id; expected error")
	}
	if err := receiver.ProcessCallStart(wrongGroup, 1_010); err == nil {
		t.Fatal("receiver accepted duplicate call_start with different group_id; expected error")
	}
}

func TestLifecycleRejectsRestartBeforeCleanup(t *testing.T) {
	start := makeCallStartFrame(t)
	end := makeCallEndFrame(t, 1)

	relay := NewRelayLifecycle(DefaultLifecyclePolicy())
	receiver := NewReceiverLifecycle(DefaultLifecyclePolicy())
	if err := relay.ProcessCallStart(start, 1_000); err != nil {
		t.Fatalf("relay start failed: %v", err)
	}
	if err := receiver.ProcessCallStart(start, 1_000); err != nil {
		t.Fatalf("receiver start failed: %v", err)
	}
	if err := relay.ProcessCallEnd(end, 2_000); err != nil {
		t.Fatalf("relay end failed: %v", err)
	}
	if _, err := receiver.ProcessCallEnd(end, 2_000); err != nil {
		t.Fatalf("receiver end failed: %v", err)
	}

	if err := relay.ProcessCallStart(start, 2_500); err == nil {
		t.Fatal("relay accepted call restart before cleanup grace; expected error")
	}
	if err := receiver.ProcessCallStart(start, 2_500); err == nil {
		t.Fatal("receiver accepted call restart before cleanup grace; expected error")
	}
}

func TestLifecycleRejectsCacheMarkForInactiveCall(t *testing.T) {
	origin := protocol.OriginKey{NetID: testNetID, SourceID: testSource, CallID: testCallID, Seq: 0}
	relay := NewRelayLifecycle(DefaultLifecyclePolicy())
	receiver := NewReceiverLifecycle(DefaultLifecyclePolicy())

	if err := relay.MarkSeen(origin); err == nil {
		t.Fatal("relay MarkSeen accepted unknown call; expected error")
	}
	if err := receiver.MarkAccepted(origin); err == nil {
		t.Fatal("receiver MarkAccepted accepted unknown call; expected error")
	}

	start := makeCallStartFrame(t)
	end := makeCallEndFrame(t, 0)
	if err := relay.ProcessCallStart(start, 1_000); err != nil {
		t.Fatalf("relay start failed: %v", err)
	}
	if err := receiver.ProcessCallStart(start, 1_000); err != nil {
		t.Fatalf("receiver start failed: %v", err)
	}
	if err := relay.ProcessCallEnd(end, 2_000); err != nil {
		t.Fatalf("relay end failed: %v", err)
	}
	if _, err := receiver.ProcessCallEnd(end, 2_000); err != nil {
		t.Fatalf("receiver end failed: %v", err)
	}
	if err := relay.MarkSeen(origin); err == nil {
		t.Fatal("relay MarkSeen accepted ended call; expected error")
	}
	if err := receiver.MarkAccepted(origin); err == nil {
		t.Fatal("receiver MarkAccepted accepted ended call; expected error")
	}
}

func TestLifecycleRejectsPayloadLenMismatch(t *testing.T) {
	start := makeCallStartFrame(t)
	start.PayloadLen++
	relay := NewRelayLifecycle(DefaultLifecyclePolicy())
	receiver := NewReceiverLifecycle(DefaultLifecyclePolicy())

	if err := relay.ProcessCallStart(start, 1_000); err == nil {
		t.Fatal("relay accepted call_start with mismatched PayloadLen; expected error")
	}
	if err := receiver.ProcessCallStart(start, 1_000); err == nil {
		t.Fatal("receiver accepted call_start with mismatched PayloadLen; expected error")
	}

	voice := makeVoiceFrame(0)
	if err := receiver.ProcessCallStart(makeCallStartFrame(t), 1_000); err != nil {
		t.Fatalf("receiver start failed: %v", err)
	}
	voice.PayloadLen++
	if err := receiver.BufferJitterFrame(voice); err == nil {
		t.Fatal("receiver accepted voice frame with mismatched PayloadLen; expected error")
	}
}

func TestLifecycleRejectsCallEndForUnknownCall(t *testing.T) {
	relay := NewRelayLifecycle(DefaultLifecyclePolicy())
	receiver := NewReceiverLifecycle(DefaultLifecyclePolicy())
	end := makeCallEndFrame(t, 1)

	if err := relay.ProcessCallEnd(end, 1_000); err == nil {
		t.Fatal("relay accepted call_end for unknown call; expected error")
	}
	if _, err := receiver.ProcessCallEnd(end, 1_000); err == nil {
		t.Fatal("receiver accepted call_end for unknown call; expected error")
	}
}

func TestLifecycleRejectsCallEndWithDifferentGroup(t *testing.T) {
	start := makeCallStartFrame(t)
	wrongGroupEnd := makeCallEndFrameWith(t, 999, testCallID, 1)

	relay := NewRelayLifecycle(DefaultLifecyclePolicy())
	receiver := NewReceiverLifecycle(DefaultLifecyclePolicy())
	if err := relay.ProcessCallStart(start, 1_000); err != nil {
		t.Fatalf("relay start failed: %v", err)
	}
	if err := receiver.ProcessCallStart(start, 1_000); err != nil {
		t.Fatalf("receiver start failed: %v", err)
	}

	if err := relay.ProcessCallEnd(wrongGroupEnd, 2_000); err == nil {
		t.Fatal("relay accepted call_end with different group_id; expected error")
	}
	if _, err := receiver.ProcessCallEnd(wrongGroupEnd, 2_000); err == nil {
		t.Fatal("receiver accepted call_end with different group_id; expected error")
	}

	key := CallKey{NetID: testNetID, SourceID: testSource, CallID: testCallID}
	relayState, ok := relay.Call(key)
	if !ok || !relayState.Active {
		t.Fatal("relay call was ended by wrong-group call_end")
	}
	receiverState, ok := receiver.Call(key)
	if !ok || !receiverState.Active {
		t.Fatal("receiver call was ended by wrong-group call_end")
	}
}

func TestReceiverBufferRejectsVoiceFrameWithDifferentGroup(t *testing.T) {
	receiver := NewReceiverLifecycle(DefaultLifecyclePolicy())
	if err := receiver.ProcessCallStart(makeCallStartFrame(t), 1_000); err != nil {
		t.Fatalf("receiver start failed: %v", err)
	}

	wrongGroupVoice := makeVoiceFrameWith(0, func(f *protocol.Frame) {
		f.GroupID = 999
	})
	if err := receiver.BufferJitterFrame(wrongGroupVoice); err == nil {
		t.Fatal("receiver buffered voice frame with different group_id; expected error")
	}
}

func makeCallStartFrame(t *testing.T) protocol.Frame {
	return makeCallStartFrameWith(t, testGroupID, testCallID)
}

func makeCallStartFrameWith(t *testing.T, groupID uint16, callID uint32) protocol.Frame {
	t.Helper()
	expectedCount := uint32(4)
	return makeCallStartFrameWithExpectedCountAndIDs(t, groupID, callID, expectedCount)
}

func makeCallStartFrameWithExpectedCount(t *testing.T, expectedCount uint32) protocol.Frame {
	t.Helper()
	return makeCallStartFrameWithExpectedCountAndIDs(t, testGroupID, testCallID, expectedCount)
}

func makeCallStartFrameWithExpectedCountAndIDs(t *testing.T, groupID uint16, callID uint32, expectedCount uint32) protocol.Frame {
	t.Helper()
	raw, err := protocol.EncodeCallStartPayload(protocol.CallStartPayload{
		FramePeriodMs:      80,
		PayloadMode:        "dummy",
		ExpectedFrameCount: &expectedCount,
		SlotModeEnabled:    false,
		TTLDefault:         1,
	})
	if err != nil {
		t.Fatalf("EncodeCallStartPayload: %v", err)
	}
	return canonicalizeFrameForTest(protocol.Frame{
		Version:     protocol.Version,
		FrameType:   protocol.FrameTypeCallStart,
		NetID:       testNetID,
		SourceID:    testSource,
		GroupID:     groupID,
		RelayID:     0,
		CallID:      callID,
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

func makeCallEndFrame(t *testing.T, lastSeq uint32) protocol.Frame {
	return makeCallEndFrameWith(t, testGroupID, testCallID, lastSeq)
}

func makeCallEndFrameWith(t *testing.T, groupID uint16, callID uint32, lastSeq uint32) protocol.Frame {
	t.Helper()
	raw, err := protocol.EncodeCallEndPayload(protocol.CallEndPayload{LastSeq: lastSeq, Reason: protocol.CallEndReasonNormal})
	if err != nil {
		t.Fatalf("EncodeCallEndPayload: %v", err)
	}
	return canonicalizeFrameForTest(protocol.Frame{
		Version:     protocol.Version,
		FrameType:   protocol.FrameTypeCallEnd,
		NetID:       testNetID,
		SourceID:    testSource,
		GroupID:     groupID,
		RelayID:     0,
		CallID:      callID,
		Seq:         0,
		Hop:         0,
		TTL:         1,
		Flags:       protocol.FlagCallEnd,
		CreatedAtMs: 2_000,
		SentAtMs:    2_000,
		PayloadLen:  uint16(len(raw)),
		Payload:     raw,
	})
}

func makeVoiceFrame(seq uint32) protocol.Frame {
	return makeVoiceFrameForCall(seq, testGroupID, testCallID)
}

func makeVoiceFrameForCall(seq uint32, groupID uint16, callID uint32) protocol.Frame {
	return makeVoiceFrameWith(seq, func(f *protocol.Frame) {
		f.GroupID = groupID
		f.CallID = callID
	})
}

func makeVoiceFrameWith(seq uint32, mutate func(*protocol.Frame)) protocol.Frame {
	return makeVoiceFrameWithPayload(seq, []byte{byte(seq), 0x42}, mutate)
}

func makeVoiceFrameWithPayload(seq uint32, payload []byte, mutate func(*protocol.Frame)) protocol.Frame {
	f := protocol.Frame{
		Version:     protocol.Version,
		FrameType:   protocol.FrameTypeVoice,
		NetID:       testNetID,
		SourceID:    testSource,
		GroupID:     testGroupID,
		RelayID:     0,
		CallID:      testCallID,
		Seq:         seq,
		Hop:         0,
		TTL:         1,
		Flags:       protocol.FlagDirect,
		CreatedAtMs: 1_000 + int64(seq)*80,
		SentAtMs:    1_000 + int64(seq)*80,
		PayloadLen:  uint16(len(payload)),
		Payload:     payload,
	}
	if mutate != nil {
		mutate(&f)
	}
	return canonicalizeFrameForTest(f)
}

func canonicalizeFrameForTest(f protocol.Frame) protocol.Frame {
	encoded, err := f.Encode()
	if err != nil {
		panic("test helper Encode failed: " + err.Error())
	}
	decoded, err := protocol.Decode(encoded)
	if err != nil {
		panic("test helper Decode failed: " + err.Error())
	}
	return decoded
}
