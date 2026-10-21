package node

import (
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/protocol"
)

// relayTestSetup creates a relay with an active call so voice frames
// can be processed. Returns the relay and a factory for voice frames.
func relayTestSetup(t *testing.T) *RelayStation {
	t.Helper()
	life := NewRelayLifecycle(DefaultLifecyclePolicy())
	relay := NewRelayStation(2, life, DefaultRelayPolicy())
	start := makeCallStartFrame(t)
	if err := relay.ProcessCallStart(start, 1_000); err != nil {
		t.Fatalf("ProcessCallStart: %v", err)
	}
	return relay
}

// TestUT004_ForwardedCopyCorrectness is AC-1: a valid Voice Frame
// with TTL > 0 is forwarded with relay_id=R, hop+1, ttl-1,
// flags=relayed (direct cleared), and preserved source fields.
//
// Traceability: UT-004 (FR-3).
func TestUT004_ForwardedCopyCorrectness(t *testing.T) {
	relay := relayTestSetup(t)
	orig := makeVoiceFrame(5)

	result := relay.ProcessVoiceFrame(orig, 1_400)
	if result.Dropped != nil {
		t.Fatalf("frame dropped: %s", result.Dropped.Reason)
	}
	if result.Forwarded == nil {
		t.Fatal("expected forwarded frame, got nil")
	}
	fwd := *result.Forwarded

	// AC-1: relay_id = R (2).
	if fwd.RelayID != 2 {
		t.Errorf("relay_id = %d, want 2", fwd.RelayID)
	}
	// AC-1: hop = orig + 1.
	if fwd.Hop != orig.Hop+1 {
		t.Errorf("hop = %d, want %d", fwd.Hop, orig.Hop+1)
	}
	// AC-1: ttl = orig - 1.
	if fwd.TTL != orig.TTL-1 {
		t.Errorf("ttl = %d, want %d", fwd.TTL, orig.TTL-1)
	}
	// AC-1: flags has relayed set, direct cleared.
	if fwd.Flags&protocol.FlagRelayed == 0 {
		t.Errorf("flags = 0x%04X, want FlagRelayed set", fwd.Flags)
	}
	if fwd.Flags&protocol.FlagDirect != 0 {
		t.Errorf("flags = 0x%04X, want FlagDirect cleared", fwd.Flags)
	}
	// AC-1: preserved source fields.
	if fwd.SourceID != orig.SourceID {
		t.Errorf("source_id = %d, want %d (preserved)", fwd.SourceID, orig.SourceID)
	}
	if fwd.GroupID != orig.GroupID {
		t.Errorf("group_id = %d, want %d (preserved)", fwd.GroupID, orig.GroupID)
	}
	if fwd.CallID != orig.CallID {
		t.Errorf("call_id = %d, want %d (preserved)", fwd.CallID, orig.CallID)
	}
	if fwd.Seq != orig.Seq {
		t.Errorf("seq = %d, want %d (preserved)", fwd.Seq, orig.Seq)
	}
	if fwd.CreatedAtMs != orig.CreatedAtMs {
		t.Errorf("created_at_ms = %d, want %d (preserved)", fwd.CreatedAtMs, orig.CreatedAtMs)
	}
	// AC-1: payload preserved (bytes and length).
	if fwd.PayloadLen != orig.PayloadLen {
		t.Errorf("payload_len = %d, want %d (preserved)", fwd.PayloadLen, orig.PayloadLen)
	}
	if string(fwd.Payload) != string(orig.Payload) {
		t.Errorf("payload = %x, want %x (preserved)", fwd.Payload, orig.Payload)
	}
	// AC-1: sent_at_ms updated to nowMs.
	if fwd.SentAtMs != 1_400 {
		t.Errorf("sent_at_ms = %d, want 1400 (updated)", fwd.SentAtMs)
	}
	// Preserved: net_id, version, frame_type.
	if fwd.NetID != orig.NetID {
		t.Errorf("net_id = %d, want %d", fwd.NetID, orig.NetID)
	}
	if fwd.Version != orig.Version {
		t.Errorf("version = %d, want %d", fwd.Version, orig.Version)
	}
	if fwd.FrameType != orig.FrameType {
		t.Errorf("frame_type = 0x%02X, want 0x%02X", fwd.FrameType, orig.FrameType)
	}
}

// TestUT004_TTLExpiredDrop is AC-3: a frame with expired TTL is
// dropped with reason ttl_expired.
//
// Traceability: UT-004 (FR-3).
func TestUT004_TTLExpiredDrop(t *testing.T) {
	relay := relayTestSetup(t)
	frame := makeVoiceFrameWith(0, func(f *protocol.Frame) {
		f.TTL = 0 // expired
	})

	result := relay.ProcessVoiceFrame(frame, 1_400)
	if result.Forwarded != nil {
		t.Fatal("frame with ttl=0 was forwarded; expected drop")
	}
	if result.Dropped == nil {
		t.Fatal("expected drop result, got nil")
	}
	if result.Dropped.Reason != DropReasonTTLExpired {
		t.Errorf("drop reason = %s, want %s", result.Dropped.Reason, DropReasonTTLExpired)
	}
	if relay.DropCounters()[DropReasonTTLExpired] != 1 {
		t.Errorf("ttl_expired counter = %d, want 1", relay.DropCounters()[DropReasonTTLExpired])
	}
}

// TestUT004_TTLDecrementedToZero verifies that a frame with ttl=1
// forwards with ttl=0 (one-hop MVP: exactly one relay forwards).
func TestUT004_TTLDecrementedToZero(t *testing.T) {
	relay := relayTestSetup(t)
	frame := makeVoiceFrame(0)

	result := relay.ProcessVoiceFrame(frame, 1_400)
	if result.Dropped != nil {
		t.Fatalf("frame with ttl=1 dropped: %s", result.Dropped.Reason)
	}
	if result.Forwarded.TTL != 0 {
		t.Errorf("forwarded ttl = %d, want 0 (decremented from 1)", result.Forwarded.TTL)
	}
}

// TestUT005_SeenCacheSuppressesDuplicate is AC-2: the same origin_key
// forwarded once; the second arrival is dropped as already_seen.
//
// Traceability: UT-005 (FR-3).
func TestUT005_SeenCacheSuppressesDuplicate(t *testing.T) {
	relay := relayTestSetup(t)
	frame := makeVoiceFrame(3)

	first := relay.ProcessVoiceFrame(frame, 1_400)
	if first.Dropped != nil {
		t.Fatalf("first frame dropped: %s", first.Dropped.Reason)
	}

	// Second arrival of the same origin_key.
	second := relay.ProcessVoiceFrame(frame, 1_410)
	if second.Forwarded != nil {
		t.Fatal("duplicate frame was forwarded; expected drop")
	}
	if second.Dropped == nil {
		t.Fatal("expected drop result for duplicate, got nil")
	}
	if second.Dropped.Reason != DropReasonAlreadySeen {
		t.Errorf("drop reason = %s, want %s", second.Dropped.Reason, DropReasonAlreadySeen)
	}
	if relay.DropCounters()[DropReasonAlreadySeen] != 1 {
		t.Errorf("already_seen counter = %d, want 1", relay.DropCounters()[DropReasonAlreadySeen])
	}
}

// TestRelay_StaleFrameDrop is AC-4: a frame older than
// relay_stale_threshold_ms is dropped as stale_frame.
func TestRelay_StaleFrameDrop(t *testing.T) {
	relay := relayTestSetup(t)
	frame := makeVoiceFrame(0)
	// created_at_ms = 1000; relay_stale_threshold = 500ms.
	// nowMs = 1601 → 1601 - 1000 = 601 > 500 → stale.
	result := relay.ProcessVoiceFrame(frame, 1_601)
	if result.Forwarded != nil {
		t.Fatal("stale frame was forwarded; expected drop")
	}
	if result.Dropped == nil || result.Dropped.Reason != DropReasonStaleFrame {
		t.Errorf("expected stale_frame drop, got %v", result.Dropped)
	}
	if relay.DropCounters()[DropReasonStaleFrame] != 1 {
		t.Errorf("stale_frame counter = %d, want 1", relay.DropCounters()[DropReasonStaleFrame])
	}
}

// TestRelay_StaleBoundaryNotDropped verifies a frame at exactly the
// threshold boundary (age == threshold) is NOT stale (strict > per §11).
func TestRelay_StaleBoundaryNotDropped(t *testing.T) {
	relay := relayTestSetup(t)
	frame := makeVoiceFrame(0)
	// created_at_ms = 1000; threshold = 500; nowMs = 1500 → age = 500.
	// §11: "if now_ms - frame.created_at_ms > threshold" → 500 > 500 is false.
	result := relay.ProcessVoiceFrame(frame, 1_500)
	if result.Dropped != nil {
		t.Fatalf("frame at exact threshold boundary dropped: %s", result.Dropped.Reason)
	}
}

// TestRelay_GroupMismatchDrop verifies a frame with a group_id not in
// the relay's subscribed groups is dropped as wrong_group.
func TestRelay_GroupMismatchDrop(t *testing.T) {
	life := NewRelayLifecycle(DefaultLifecyclePolicy())
	policy := RelayPolicy{
		StaleThresholdMs: DefaultRelayStaleThresholdMs,
		SubscribedGroups: map[uint16]bool{100: true, 200: true},
	}
	relay := NewRelayStation(2, life, policy)
	if err := relay.ProcessCallStart(makeCallStartFrame(t), 1_000); err != nil {
		t.Fatalf("ProcessCallStart: %v", err)
	}

	frame := makeVoiceFrameWith(0, func(f *protocol.Frame) {
		f.GroupID = 999 // not subscribed
	})

	result := relay.ProcessVoiceFrame(frame, 1_400)
	if result.Forwarded != nil {
		t.Fatal("wrong-group frame was forwarded; expected drop")
	}
	if result.Dropped == nil || result.Dropped.Reason != DropReasonWrongGroup {
		t.Errorf("expected wrong_group drop, got %v", result.Dropped)
	}
}

// TestRelay_GroupMatchForwarded verifies a frame with a subscribed
// group_id is forwarded normally.
func TestRelay_GroupMatchForwarded(t *testing.T) {
	life := NewRelayLifecycle(DefaultLifecyclePolicy())
	policy := RelayPolicy{
		StaleThresholdMs: DefaultRelayStaleThresholdMs,
		SubscribedGroups: map[uint16]bool{100: true},
	}
	relay := NewRelayStation(2, life, policy)
	if err := relay.ProcessCallStart(makeCallStartFrame(t), 1_000); err != nil {
		t.Fatalf("ProcessCallStart: %v", err)
	}

	frame := makeVoiceFrame(0) // group_id=100
	result := relay.ProcessVoiceFrame(frame, 1_400)
	if result.Dropped != nil {
		t.Fatalf("subscribed-group frame dropped: %s", result.Dropped.Reason)
	}
}

// TestRelay_InvalidFrameDrop verifies that an invalid frame is dropped as
// invalid.
func TestRelay_InvalidFrameDrop(t *testing.T) {
	relay := relayTestSetup(t)
	// Build a frame that fails integrity/validation: mismatched PayloadLen.
	frame := makeVoiceFrame(0)
	frame.PayloadLen++

	result := relay.ProcessVoiceFrame(frame, 1_400)
	if result.Forwarded != nil {
		t.Fatal("invalid frame was forwarded; expected drop")
	}
	if result.Dropped == nil || result.Dropped.Reason != DropReasonInvalid {
		t.Errorf("expected invalid drop, got %v", result.Dropped)
	}
}

// TestRelay_VerifyIntegrityRejectsMutatedDecodedPayload proves the relay
// boundary re-checks CRCs on Frame values that were decoded and then mutated
// in memory.
func TestRelay_VerifyIntegrityRejectsMutatedDecodedPayload(t *testing.T) {
	relay := relayTestSetup(t)
	frame := makeVoiceFrame(0)
	frame.Payload[0] ^= 0xFF

	result := relay.ProcessVoiceFrame(frame, 1_400)
	if result.Forwarded != nil {
		t.Fatal("relay forwarded a decoded frame whose payload no longer matches payload_crc32")
	}
	if result.Dropped == nil || result.Dropped.Reason != DropReasonInvalid {
		t.Fatalf("expected invalid drop for mutated payload, got %v", result.Dropped)
	}
}

// TestRelay_VerifyIntegrityRejectsMutatedDecodedHeader proves header CRC is
// rechecked too, not only payload CRC.
func TestRelay_VerifyIntegrityRejectsMutatedDecodedHeader(t *testing.T) {
	relay := relayTestSetup(t)
	frame := makeVoiceFrame(0)
	frame.TTL = 2

	result := relay.ProcessVoiceFrame(frame, 1_400)
	if result.Forwarded != nil {
		t.Fatal("relay forwarded a decoded frame whose header no longer matches header_crc32")
	}
	if result.Dropped == nil || result.Dropped.Reason != DropReasonInvalid {
		t.Fatalf("expected invalid drop for mutated header, got %v", result.Dropped)
	}
}

// TestRelay_CallNotActiveDrop verifies that a voice frame for a call
// that was not started (or has ended) is dropped as call_not_active.
func TestRelay_CallNotActiveDrop(t *testing.T) {
	// No call_start → call not active.
	life := NewRelayLifecycle(DefaultLifecyclePolicy())
	relay := NewRelayStation(2, life, DefaultRelayPolicy())

	frame := makeVoiceFrame(0)
	result := relay.ProcessVoiceFrame(frame, 1_400)
	if result.Forwarded != nil {
		t.Fatal("voice frame for inactive call was forwarded; expected drop")
	}
	if result.Dropped == nil || result.Dropped.Reason != DropReasonCallNotActive {
		t.Errorf("expected call_not_active drop, got %v", result.Dropped)
	}
}

// TestRelay_CallNotActiveAfterCallEnd verifies that after call_end,
// voice frames for that call are dropped as call_not_active.
func TestRelay_CallNotActiveAfterCallEnd(t *testing.T) {
	relay := relayTestSetup(t)
	end := makeCallEndFrame(t, 10)
	if err := relay.ProcessCallEnd(end, 2_000); err != nil {
		t.Fatalf("ProcessCallEnd: %v", err)
	}

	frame := makeVoiceFrame(0)
	result := relay.ProcessVoiceFrame(frame, 2_100)
	if result.Forwarded != nil {
		t.Fatal("voice frame after call_end was forwarded; expected drop")
	}
	if result.Dropped == nil || result.Dropped.Reason != DropReasonCallNotActive {
		t.Errorf("expected call_not_active drop after end, got %v", result.Dropped)
	}
}

// TestRelay_UnsupportedFrameTypeDrop verifies that a non-voice frame
// passed to ProcessVoiceFrame is dropped as unsupported_frame_type.
func TestRelay_UnsupportedFrameTypeDrop(t *testing.T) {
	relay := relayTestSetup(t)
	// Pass a call_end frame to ProcessVoiceFrame.
	end := makeCallEndFrame(t, 0)
	result := relay.ProcessVoiceFrame(end, 1_400)
	if result.Forwarded != nil {
		t.Fatal("non-voice frame was forwarded; expected drop")
	}
	if result.Dropped == nil || result.Dropped.Reason != DropReasonUnsupportedFrameType {
		t.Errorf("expected unsupported_frame_type drop, got %v", result.Dropped)
	}
}

func TestRelay_CallStartWrongGroupRejected(t *testing.T) {
	relay := NewRelayStation(2, NewRelayLifecycle(DefaultLifecyclePolicy()), RelayPolicy{
		StaleThresholdMs: DefaultRelayStaleThresholdMs,
		SubscribedGroups: map[uint16]bool{200: true},
	})

	err := relay.ProcessCallStart(makeCallStartFrameWith(t, testGroupID, testCallID), 1_000)
	if err == nil {
		t.Fatal("relay accepted call_start for non-subscribed group")
	}
	if relay.Lifecycle().HasCall(CallKey{NetID: testNetID, SourceID: testSource, CallID: testCallID}) {
		t.Fatal("wrong-group call_start created relay call state")
	}
}

func TestRelay_CallEndWrongGroupRejected(t *testing.T) {
	relay := NewRelayStation(2, NewRelayLifecycle(DefaultLifecyclePolicy()), RelayPolicy{
		StaleThresholdMs: DefaultRelayStaleThresholdMs,
		SubscribedGroups: map[uint16]bool{testGroupID: true},
	})
	if err := relay.ProcessCallStart(makeCallStartFrame(t), 1_000); err != nil {
		t.Fatalf("ProcessCallStart: %v", err)
	}

	err := relay.ProcessCallEnd(makeCallEndFrameWith(t, 999, testCallID, 0), 2_000)
	if err == nil {
		t.Fatal("relay accepted call_end for non-subscribed group")
	}
	state, ok := relay.Lifecycle().Call(CallKey{NetID: testNetID, SourceID: testSource, CallID: testCallID})
	if !ok || !state.Active {
		t.Fatal("wrong-group call_end ended or removed relay call state")
	}
}

func TestRelay_CallStartValidatesBeforeGroupSubscription(t *testing.T) {
	relay := NewRelayStation(2, NewRelayLifecycle(DefaultLifecyclePolicy()), RelayPolicy{
		StaleThresholdMs: DefaultRelayStaleThresholdMs,
		SubscribedGroups: map[uint16]bool{200: true},
	})
	frame := makeCallStartFrameWith(t, testGroupID, testCallID)
	frame.Payload[0] ^= 0xFF

	err := relay.ProcessCallStart(frame, 1_000)
	if err == nil {
		t.Fatal("corrupted call_start accepted; expected error")
	}
	if strings.Contains(err.Error(), "not in subscribed groups") {
		t.Fatalf("ProcessCallStart checked group before integrity validation: %v", err)
	}
	if relay.Lifecycle().HasCall(CallKeyFromFrame(frame)) {
		t.Fatal("corrupted call_start created relay call state")
	}
}

func TestRelay_CallEndValidatesBeforeGroupSubscription(t *testing.T) {
	relay := NewRelayStation(2, NewRelayLifecycle(DefaultLifecyclePolicy()), RelayPolicy{
		StaleThresholdMs: DefaultRelayStaleThresholdMs,
		SubscribedGroups: map[uint16]bool{testGroupID: true},
	})
	if err := relay.ProcessCallStart(makeCallStartFrame(t), 1_000); err != nil {
		t.Fatalf("ProcessCallStart: %v", err)
	}
	frame := makeCallEndFrameWith(t, 999, testCallID, 0)
	frame.Payload[0] ^= 0xFF

	err := relay.ProcessCallEnd(frame, 2_000)
	if err == nil {
		t.Fatal("corrupted call_end accepted; expected error")
	}
	if strings.Contains(err.Error(), "not in subscribed groups") {
		t.Fatalf("ProcessCallEnd checked group before integrity validation: %v", err)
	}
	state, ok := relay.Lifecycle().Call(CallKeyFromFrame(frame))
	if !ok || !state.Active {
		t.Fatal("corrupted call_end ended or removed relay call state")
	}
}

func TestRelay_StaleAndFutureCallStartRejected(t *testing.T) {
	cases := []struct {
		name  string
		nowMs int64
	}{
		{name: "stale", nowMs: 1_501},
		{name: "future", nowMs: 999},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			relay := NewRelayStation(2, NewRelayLifecycle(DefaultLifecyclePolicy()), DefaultRelayPolicy())
			start := makeCallStartFrame(t)

			err := relay.ProcessCallStart(start, c.nowMs)
			if err == nil {
				t.Fatalf("%s call_start accepted; expected error", c.name)
			}
			if relay.Lifecycle().HasCall(CallKeyFromFrame(start)) {
				t.Fatalf("%s call_start created relay call state", c.name)
			}
		})
	}
}

func TestRelay_StaleAndFutureCallEndRejected(t *testing.T) {
	cases := []struct {
		name  string
		nowMs int64
	}{
		{name: "stale", nowMs: 2_501},
		{name: "future", nowMs: 1_999},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			relay := relayTestSetup(t)
			end := makeCallEndFrame(t, 0)

			err := relay.ProcessCallEnd(end, c.nowMs)
			if err == nil {
				t.Fatalf("%s call_end accepted; expected error", c.name)
			}
			state, ok := relay.Lifecycle().Call(CallKeyFromFrame(end))
			if !ok || !state.Active {
				t.Fatalf("%s call_end ended or removed relay call state", c.name)
			}
		})
	}
}

// TestRelay_MalformedVoiceFlagsDropped proves relay voice handling
// requires exactly one path flag and rejects call lifecycle flags.
func TestRelay_MalformedVoiceFlagsDropped(t *testing.T) {
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
			relay := relayTestSetup(t)
			frame := makeVoiceFrameWith(0, func(f *protocol.Frame) {
				f.Flags = c.flags
			})

			result := relay.ProcessVoiceFrame(frame, 1_400)
			if result.Forwarded != nil {
				t.Fatal("malformed voice flags were forwarded")
			}
			if result.Dropped == nil || result.Dropped.Reason != DropReasonInvalid {
				t.Fatalf("expected invalid drop, got %+v", result.Dropped)
			}
			if relay.Lifecycle().Seen(frame.OriginKey()) {
				t.Fatal("malformed voice frame polluted seen cache")
			}
		})
	}
}

// TestRelay_SequenceForwarding verifies that multiple frames in
// sequence are all forwarded (seen-cache does not over-suppress).
func TestRelay_SequenceForwarding(t *testing.T) {
	relay := relayTestSetup(t)
	for seq := uint32(0); seq < 10; seq++ {
		frame := makeVoiceFrame(seq)
		result := relay.ProcessVoiceFrame(frame, 1_400+int64(seq)*80)
		if result.Dropped != nil {
			t.Fatalf("frame seq=%d dropped: %s", seq, result.Dropped.Reason)
		}
		if result.Forwarded == nil {
			t.Fatalf("frame seq=%d not forwarded", seq)
		}
		if result.Forwarded.Seq != seq {
			t.Errorf("forwarded seq = %d, want %d", result.Forwarded.Seq, seq)
		}
	}
}

// TestRelay_ForwardedFramePreservesOtherFlags verifies that non-
// direct/relayed flags (e.g. late_allowed) are preserved in the
// forwarded copy.
func TestRelay_ForwardedFramePreservesOtherFlags(t *testing.T) {
	relay := relayTestSetup(t)
	frame := makeVoiceFrameWith(0, func(f *protocol.Frame) {
		f.Flags = protocol.FlagDirect | protocol.FlagLateAllowed
	})

	result := relay.ProcessVoiceFrame(frame, 1_400)
	if result.Dropped != nil {
		t.Fatalf("frame dropped: %s", result.Dropped.Reason)
	}
	fwd := *result.Forwarded
	if fwd.Flags&protocol.FlagLateAllowed == 0 {
		t.Errorf("late_allowed flag not preserved: flags = 0x%04X", fwd.Flags)
	}
	if fwd.Flags&protocol.FlagDirect != 0 {
		t.Errorf("direct flag not cleared: flags = 0x%04X", fwd.Flags)
	}
	if fwd.Flags&protocol.FlagRelayed == 0 {
		t.Errorf("relayed flag not set: flags = 0x%04X", fwd.Flags)
	}
}

// TestRelay_ForwardedFrameEncodesAndRoundTrips verifies that the
// forwarded copy is a valid encodable frame that round-trips through
// protocol.Encode / DecodeStrict.
func TestRelay_ForwardedFrameEncodesAndRoundTrips(t *testing.T) {
	relay := relayTestSetup(t)
	frame := makeVoiceFrame(0)
	result := relay.ProcessVoiceFrame(frame, 1_400)
	if result.Dropped != nil {
		t.Fatalf("frame dropped: %s", result.Dropped.Reason)
	}

	encoded, err := result.Forwarded.Encode()
	if err != nil {
		t.Fatalf("forwarded frame Encode: %v", err)
	}
	decoded, err := protocol.DecodeStrict(encoded)
	if err != nil {
		t.Fatalf("forwarded frame DecodeStrict: %v", err)
	}
	if decoded.RelayID != 2 {
		t.Errorf("decoded relay_id = %d, want 2", decoded.RelayID)
	}
	if decoded.Hop != 1 {
		t.Errorf("decoded hop = %d, want 1", decoded.Hop)
	}
	if decoded.TTL != 0 {
		t.Errorf("decoded ttl = %d, want 0", decoded.TTL)
	}
	if decoded.Flags&protocol.FlagRelayed == 0 || decoded.Flags&protocol.FlagDirect != 0 {
		t.Errorf("decoded flags = 0x%04X, want relayed set + direct cleared", decoded.Flags)
	}
}

// TestRelay_EmptySubscribedGroupsMeansAll verifies that an empty
// SubscribedGroups map means "no group subscription filtering" — but
// the call's group_id consistency is still enforced (voice frame group
// must match the call_start group_id).
func TestRelay_EmptySubscribedGroupsMeansAll(t *testing.T) {
	relay := relayTestSetup(t) // DefaultRelayPolicy → empty SubscribedGroups
	// call_start was for group 100; voice frame must match.
	frame := makeVoiceFrame(0) // group_id=100
	result := relay.ProcessVoiceFrame(frame, 1_400)
	if result.Dropped != nil {
		t.Fatalf("frame with call-matching group dropped under empty subscription: %s", result.Dropped.Reason)
	}
}

// TestRelay_GroupConsistencyWithCallStart verifies that a voice frame
// whose group_id differs from the call_start's group_id is rejected as
// wrong_group, even when SubscribedGroups is empty (all-subscribed).
func TestRelay_GroupConsistencyWithCallStart(t *testing.T) {
	relay := relayTestSetup(t) // call_start group=100
	frame := makeVoiceFrameWith(0, func(f *protocol.Frame) {
		f.GroupID = 999 // different from call_start's 100
	})

	result := relay.ProcessVoiceFrame(frame, 1_400)
	if result.Forwarded != nil {
		t.Fatal("voice frame with mismatched group was forwarded; expected drop")
	}
	if result.Dropped == nil || result.Dropped.Reason != DropReasonWrongGroup {
		t.Errorf("expected wrong_group drop, got %v", result.Dropped)
	}
}

// TestRelay_FutureFrameRejected is a review finding: a frame with
// CreatedAtMs > nowMs ("from the future") must be rejected as invalid,
// not forwarded with negative latency.
func TestRelay_FutureFrameRejected(t *testing.T) {
	relay := relayTestSetup(t)
	frame := makeVoiceFrame(0)
	// created_at_ms = 1000; nowMs = 500 → frame is "from the future".
	result := relay.ProcessVoiceFrame(frame, 500)
	if result.Forwarded != nil {
		t.Fatal("future frame was forwarded; expected drop")
	}
	if result.Dropped == nil || result.Dropped.Reason != DropReasonInvalid {
		t.Errorf("expected invalid drop for future frame, got %v", result.Dropped)
	}
}

// TestRelay_ZeroIDPanics verifies that NewRelayStation panics on id=0
// (reserved for direct/source frames per §4.3).
func TestRelay_ZeroIDPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("NewRelayStation(0, ...) did not panic")
		}
	}()
	NewRelayStation(0, NewRelayLifecycle(DefaultLifecyclePolicy()), DefaultRelayPolicy())
}

// TestRelay_NilLifecyclePanics verifies that NewRelayStation panics on
// nil lifecycle.
func TestRelay_NilLifecyclePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("NewRelayStation(_, nil, _) did not panic")
		}
	}()
	NewRelayStation(2, nil, DefaultRelayPolicy())
}

// TestRelay_DefensivePayloadCopy verifies that the forwarded frame's
// payload is an independent copy, so mutating the original frame's
// payload after ProcessVoiceFrame does not affect the forwarded copy.
func TestRelay_DefensivePayloadCopy(t *testing.T) {
	relay := relayTestSetup(t)
	frame := makeVoiceFrame(0)
	result := relay.ProcessVoiceFrame(frame, 1_400)
	if result.Dropped != nil {
		t.Fatalf("frame dropped: %s", result.Dropped.Reason)
	}
	// Mutate the original frame's payload.
	frame.Payload[0] = 0xFF
	// The forwarded copy must be unaffected.
	if result.Forwarded.Payload[0] == 0xFF {
		t.Fatal("forwarded payload shares storage with original; defensive copy missing")
	}
}

// TestRelay_DefensivePayloadCopy_ZeroLengthNonNil verifies zero-length
// payload slices with spare capacity do not share backing storage with the
// forwarded frame.
func TestRelay_DefensivePayloadCopy_ZeroLengthNonNil(t *testing.T) {
	relay := relayTestSetup(t)
	frame := makeVoiceFrameWith(0, func(f *protocol.Frame) {
		f.Payload = nil
		f.PayloadLen = 0
	})
	frame.Payload = make([]byte, 0, 8)

	result := relay.ProcessVoiceFrame(frame, 1_400)
	if result.Dropped != nil {
		t.Fatalf("frame dropped: %s", result.Dropped.Reason)
	}
	if cap(result.Forwarded.Payload) != 0 {
		t.Fatalf("forwarded zero-length payload has cap=%d, want 0 to avoid sharing backing storage", cap(result.Forwarded.Payload))
	}
}

// TestRelay_ForwardedCRCsZeroed verifies that the forwarded frame has
// zeroed CRCs (they are stale and will be recomputed by Encode()).
func TestRelay_ForwardedCRCsZeroed(t *testing.T) {
	relay := relayTestSetup(t)
	// First, encode the original frame so it has non-zero CRCs.
	orig := makeVoiceFrame(0)
	encoded, err := orig.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := protocol.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.HeaderCRC32 == 0 {
		t.Fatal("test setup: decoded frame should have non-zero HeaderCRC32")
	}

	result := relay.ProcessVoiceFrame(decoded, 1_400)
	if result.Dropped != nil {
		t.Fatalf("frame dropped: %s", result.Dropped.Reason)
	}
	if result.Forwarded.HeaderCRC32 != 0 {
		t.Errorf("forwarded HeaderCRC32 = 0x%08X, want 0 (stale; recomputed by Encode())", result.Forwarded.HeaderCRC32)
	}
	if result.Forwarded.PayloadCRC32 != 0 {
		t.Errorf("forwarded PayloadCRC32 = 0x%08X, want 0 (stale; recomputed by Encode())", result.Forwarded.PayloadCRC32)
	}
}

// TestRelay_MarkSeenBeforeTTLCheck verifies the algorithm-and-experiments.md
// §7.2 relay order: a frame with TTL=0 IS marked seen, so a second arrival
// of the same origin_key is "already_seen", not "ttl_expired".
func TestRelay_MarkSeenBeforeTTLCheck(t *testing.T) {
	relay := relayTestSetup(t)
	frame := makeVoiceFrameWith(0, func(f *protocol.Frame) {
		f.TTL = 0
	})

	// First arrival: TTL=0 → dropped as ttl_expired, but marked seen.
	first := relay.ProcessVoiceFrame(frame, 1_400)
	if first.Dropped == nil || first.Dropped.Reason != DropReasonTTLExpired {
		t.Fatalf("first: expected ttl_expired, got %v", first.Dropped)
	}

	// Second arrival: same origin_key → already_seen (NOT ttl_expired).
	second := relay.ProcessVoiceFrame(frame, 1_410)
	if second.Dropped == nil || second.Dropped.Reason != DropReasonAlreadySeen {
		t.Fatalf("second: expected already_seen (MarkSeen before TTL), got %v", second.Dropped)
	}
}

// TestRelay_HopOverflowGuard verifies that a frame with hop=255 is
// rejected as invalid (hop+1 would wrap uint8 to 0).
func TestRelay_HopOverflowGuard(t *testing.T) {
	relay := relayTestSetup(t)
	frame := makeVoiceFrameWith(0, func(f *protocol.Frame) {
		f.RelayID = 7
		f.Hop = 255
		f.TTL = 1
		f.Flags = protocol.FlagRelayed
	})

	result := relay.ProcessVoiceFrame(frame, 1_400)
	if result.Forwarded != nil {
		t.Fatal("frame with hop=255 was forwarded; expected drop")
	}
	if result.Dropped == nil || result.Dropped.Reason != DropReasonInvalid {
		t.Errorf("expected invalid drop for hop=255, got %v", result.Dropped)
	}
}

// TestRelay_HopOverflowDoesNotPoisonSeenCache is review finding 2:
// a frame with hop=255 must NOT be marked seen, so a subsequent valid
// frame with the same origin_key is forwarded (not rejected as
// already_seen).
func TestRelay_HopOverflowDoesNotPoisonSeenCache(t *testing.T) {
	relay := relayTestSetup(t)
	bad := makeVoiceFrameWith(0, func(f *protocol.Frame) {
		f.RelayID = 7
		f.Hop = 255
		f.TTL = 1
		f.Flags = protocol.FlagRelayed
	})

	// First: hop=255 → dropped invalid, must NOT mark seen.
	first := relay.ProcessVoiceFrame(bad, 1_400)
	if first.Dropped == nil || first.Dropped.Reason != DropReasonInvalid {
		t.Fatalf("first: expected invalid, got %v", first.Dropped)
	}

	// Second: same origin_key, valid hop → must forward (not already_seen).
	good := makeVoiceFrame(0) // hop=0, ttl=1
	second := relay.ProcessVoiceFrame(good, 1_410)
	if second.Dropped != nil {
		t.Fatalf("second: valid frame rejected as %s; hop=255 poisoned seen-cache", second.Dropped.Reason)
	}
}

// TestRelay_DropCountersSnapshot verifies that the DropCounters()
// accessor returns a copy independent of the relay's internal state.
func TestRelay_DropCountersSnapshot(t *testing.T) {
	relay := relayTestSetup(t)
	frame := makeVoiceFrameWith(0, func(f *protocol.Frame) {
		f.TTL = 0
	})
	relay.ProcessVoiceFrame(frame, 1_400)

	snap := relay.DropCounters()
	snap[DropReasonTTLExpired] = 999
	snap[DropReasonInvalid] = 123

	fresh := relay.DropCounters()
	if fresh[DropReasonTTLExpired] == 999 {
		t.Error("DropCounters() returned a live map; mutating snapshot changed internal state")
	}
	if _, ok := fresh[DropReasonInvalid]; ok {
		t.Error("DropCounters() returned a live map; adding a key in snapshot leaked into internal state")
	}
}

// TestRelay_DropResultPayloadDefensiveCopy is review finding 5: the
// DropResult.Frame.Payload must be an independent copy so mutating the
// original frame's payload after ProcessVoiceFrame does not change the
// DropResult.
func TestRelay_DropResultPayloadDefensiveCopy(t *testing.T) {
	relay := relayTestSetup(t)
	frame := makeVoiceFrameWith(0, func(f *protocol.Frame) {
		f.TTL = 0 // force drop
	})

	result := relay.ProcessVoiceFrame(frame, 1_400)
	if result.Dropped == nil {
		t.Fatal("expected drop")
	}
	// Mutate the original frame's payload.
	frame.Payload[0] = 0xFF
	// The DropResult's payload must be unaffected.
	if result.Dropped.Frame.Payload[0] == 0xFF {
		t.Fatal("DropResult payload shares storage with original; defensive copy missing")
	}
}

// TestRelay_DropResultPayloadDefensiveCopy_ZeroLengthNonNil verifies dropped
// frames also avoid sharing backing storage for zero-length non-nil payloads.
func TestRelay_DropResultPayloadDefensiveCopy_ZeroLengthNonNil(t *testing.T) {
	relay := relayTestSetup(t)
	frame := makeVoiceFrameWith(0, func(f *protocol.Frame) {
		f.Payload = nil
		f.PayloadLen = 0
		f.TTL = 0
	})
	frame.Payload = make([]byte, 0, 8)

	result := relay.ProcessVoiceFrame(frame, 1_400)
	if result.Dropped == nil {
		t.Fatal("expected drop")
	}
	if cap(result.Dropped.Frame.Payload) != 0 {
		t.Fatalf("dropped zero-length payload has cap=%d, want 0 to avoid sharing backing storage", cap(result.Dropped.Frame.Payload))
	}
}

// TestRelay_SubscribedGroupsCloned is review finding 4: mutating the
// SubscribedGroups map after construction must not change the relay's
// policy.
func TestRelay_SubscribedGroupsCloned(t *testing.T) {
	groups := map[uint16]bool{100: true}
	life := NewRelayLifecycle(DefaultLifecyclePolicy())
	relay := NewRelayStation(2, life, RelayPolicy{
		StaleThresholdMs: DefaultRelayStaleThresholdMs,
		SubscribedGroups: groups,
	})
	if err := relay.ProcessCallStart(makeCallStartFrame(t), 1_000); err != nil {
		t.Fatalf("ProcessCallStart: %v", err)
	}

	// Mutate the original map after construction.
	groups[999] = true
	delete(groups, 100)

	// Frame with group=100 (subscribed at construction) must still
	// forward; group=999 (added after construction) must not.
	frame100 := makeVoiceFrame(0) // group=100
	result := relay.ProcessVoiceFrame(frame100, 1_400)
	if result.Dropped != nil {
		t.Errorf("group 100 dropped after external map mutation: %s", result.Dropped.Reason)
	}

	const callID999 uint32 = 9002
	if err := relay.ProcessCallStart(makeCallStartFrameWith(t, 999, callID999), 1_010); err == nil {
		t.Fatal("ProcessCallStart group 999 accepted after external map mutation; expected error")
	}
}
