package node

import (
	"math"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/protocol"
)

func TestAgeExceedsThresholdWithoutOverflow(t *testing.T) {
	tests := []struct {
		name      string
		created   int64
		now       int64
		threshold int64
		want      bool
	}{
		{"age below threshold", 1_000, 1_500, 1_000, false},
		{"age equals threshold", 1_000, 2_000, 1_000, false},
		{"age above threshold", 1_000, 2_001, 1_000, true},
		{"zero age zero threshold", 1_000, 1_000, 0, false},
		{"zero age negative threshold keeps historic semantics", 5, 5, -1, true},
		{"created near MinInt64 now zero", math.MinInt64, 0, 1_000, true},
		{"age 2^63 exceeds MaxInt64 threshold", math.MinInt64, 0, math.MaxInt64, true},
		{"age MaxInt64 equals MaxInt64 threshold", math.MinInt64 + 1, 0, math.MaxInt64, false},
		{"maximum age 2^64-1", math.MinInt64, math.MaxInt64, math.MaxInt64, true},
		{"age MaxInt64 from zero equals threshold", 0, math.MaxInt64, math.MaxInt64, false},
		{"age MaxInt64 from zero exceeds smaller threshold", 0, math.MaxInt64, math.MaxInt64 - 1, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ageExceedsThreshold(tc.created, tc.now, tc.threshold); got != tc.want {
				t.Fatalf("ageExceedsThreshold(created=%d, now=%d, threshold=%d) = %v, want %v",
					tc.created, tc.now, tc.threshold, got, tc.want)
			}
		})
	}
}

func TestFrameAgeDropReasonAtInt64Boundaries(t *testing.T) {
	maxSafe := protocol.MaxSafeCreatedAtMs
	tests := []struct {
		name       string
		created    int64
		now        int64
		wantReason DropReason
		wantDrop   bool
	}{
		{"fresh frame", 1_000, 1_500, "", false},
		{"age equals threshold", 1_000, 2_000, "", false},
		{"age above threshold", 1_000, 2_001, DropReasonStaleFrame, true},
		{"future frame", 2_000, 1_000, DropReasonInvalid, true},
		{"created near MinInt64", math.MinInt64, 0, DropReasonStaleFrame, true},
		{"created MinInt64 now MaxInt64", math.MinInt64, math.MaxInt64, DropReasonStaleFrame, true},
		{"negative created beyond threshold stays stale", -2_000, 0, DropReasonStaleFrame, true},
		{"negative created within threshold", -100, 100, DropReasonInvalid, true},
		{"created zero", 0, 0, "", false},
		{"created at MaxSafeCreatedAtMs", maxSafe, maxSafe, "", false},
		{"created above MaxSafeCreatedAtMs", maxSafe + 1, maxSafe + 1, DropReasonInvalid, true},
		{"created MaxInt64 now MaxInt64", math.MaxInt64, math.MaxInt64, DropReasonInvalid, true},
		{"created MaxInt64 is in the future", math.MaxInt64, 0, DropReasonInvalid, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := protocol.Frame{CreatedAtMs: tc.created}
			reason, drop := frameAgeDropReason(f, tc.now, DefaultReceiverStaleThresholdMs)
			if drop != tc.wantDrop || reason != tc.wantReason {
				t.Fatalf("frameAgeDropReason = (%q, %v), want (%q, %v)", reason, drop, tc.wantReason, tc.wantDrop)
			}
		})
	}
}

func TestFrameLateByThresholdAtInt64Boundaries(t *testing.T) {
	life := NewReceiverLifecycle(DefaultLifecyclePolicy())
	tests := []struct {
		name       string
		created    int64
		receivedAt int64
		threshold  int64
		want       bool
	}{
		{"within threshold", 1_000, 1_100, 250, false},
		{"exactly at threshold", 1_000, 1_250, 250, false},
		{"beyond threshold", 1_000, 1_251, 250, true},
		{"received before created", 1_000, 999, 250, false},
		{"created near MinInt64", math.MinInt64, 0, 250, true},
		{"age 2^63 exceeds MaxInt64 threshold", math.MinInt64, 0, math.MaxInt64, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := protocol.Frame{CreatedAtMs: tc.created}
			if got := life.frameLateByThreshold(f, tc.receivedAt, tc.threshold); got != tc.want {
				t.Fatalf("frameLateByThreshold = %v, want %v", got, tc.want)
			}
		})
	}
}

// An ancient but CRC-valid frame must be dropped as stale at the receiver and
// the relay instead of overflowing the age subtraction and passing as fresh.
func TestAncientCRCValidVoiceFrameDroppedAsStale(t *testing.T) {
	ancient := makeVoiceFrameWith(0, func(f *protocol.Frame) {
		f.CreatedAtMs = math.MinInt64 + 5
		f.SentAtMs = math.MinInt64 + 5
	})
	if err := ancient.VerifyIntegrity(); err != nil {
		t.Fatalf("fixture frame must be CRC-valid: %v", err)
	}

	t.Run("receiver", func(t *testing.T) {
		receiver := receiverTestSetup(t)
		result := receiver.ProcessVoiceFrame(ancient, 1_000)
		if result.Dropped == nil || result.Dropped.Reason != DropReasonStaleFrame {
			t.Fatalf("result = %+v, want drop %s", result, DropReasonStaleFrame)
		}
		receiver.EvaluateJitter(10_000)
	})

	t.Run("relay", func(t *testing.T) {
		relay := relayTestSetup(t)
		result := relay.ProcessVoiceFrame(ancient, 1_000)
		if result.Dropped == nil || result.Dropped.Reason != DropReasonStaleFrame {
			t.Fatalf("result = %+v, want drop %s", result, DropReasonStaleFrame)
		}
		if result.Forwarded != nil {
			t.Fatalf("ancient frame was forwarded: %+v", result.Forwarded)
		}
	})
}

// A CRC-valid voice frame whose created_at_ms is negative but within the stale
// threshold, or above MaxSafeCreatedAtMs, is invalid. Before the range check
// the negative frame reached the jitter buffer and EvaluateJitter panicked in
// sim.AddMs.
func TestOutOfRangeVoiceFrameCreatedAtDroppedAsInvalid(t *testing.T) {
	maxSafe := protocol.MaxSafeCreatedAtMs
	tests := []struct {
		name       string
		callStart  int64
		created    int64
		receivedAt int64
	}{
		{"negative within threshold", 0, -100, 100},
		{"above MaxSafeCreatedAtMs", maxSafe, maxSafe + 1, maxSafe + 1},
	}
	for _, tc := range tests {
		frame := makeVoiceFrameWith(0, func(f *protocol.Frame) {
			f.CreatedAtMs = tc.created
			f.SentAtMs = tc.created
		})
		start := makeCallStartFrameAt(t, tc.callStart, nil)

		t.Run(tc.name+"/receiver", func(t *testing.T) {
			receiver := NewReceiverStation(3, NewReceiverLifecycle(DefaultLifecyclePolicy()), DefaultReceiverPolicy())
			if err := receiver.ProcessCallStart(start, tc.callStart); err != nil {
				t.Fatalf("ProcessCallStart: %v", err)
			}
			result := receiver.ProcessVoiceFrame(frame, tc.receivedAt)
			if result.Dropped == nil || result.Dropped.Reason != DropReasonInvalid {
				t.Fatalf("result = %+v, want drop %s", result, DropReasonInvalid)
			}
			for _, now := range []int64{tc.receivedAt, tc.receivedAt + 160, tc.callStart + 10_000} {
				if eval := receiver.EvaluateJitter(now); len(eval.Emitted) != 0 {
					t.Fatalf("EvaluateJitter(%d) emitted dropped frame: %+v", now, eval.Emitted)
				}
			}
		})

		t.Run(tc.name+"/relay", func(t *testing.T) {
			relay := NewRelayStation(2, NewRelayLifecycle(DefaultLifecyclePolicy()), DefaultRelayPolicy())
			if err := relay.ProcessCallStart(start, tc.callStart); err != nil {
				t.Fatalf("ProcessCallStart: %v", err)
			}
			result := relay.ProcessVoiceFrame(frame, tc.receivedAt)
			if result.Dropped == nil || result.Dropped.Reason != DropReasonInvalid || result.Forwarded != nil {
				t.Fatalf("result = %+v, want drop %s without forward", result, DropReasonInvalid)
			}
		})
	}
}

// makeCallStartFrameAt returns a CRC-valid call_start created at createdAtMs.
// A nil rawPayload keeps the default payload (period 80 ms, four frames).
func makeCallStartFrameAt(t *testing.T, createdAtMs int64, rawPayload []byte) protocol.Frame {
	t.Helper()
	start := makeCallStartFrame(t)
	start.CreatedAtMs = createdAtMs
	start.SentAtMs = createdAtMs
	if rawPayload != nil {
		start.Payload = rawPayload
		start.PayloadLen = uint16(len(rawPayload))
	}
	return canonicalizeFrameForTest(start)
}
