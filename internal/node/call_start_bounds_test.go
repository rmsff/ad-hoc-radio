package node

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/protocol"
)

// A CRC-valid call_start with a negative created_at_ms within the stale
// threshold, or one above MaxSafeCreatedAtMs, must be rejected by receiver and
// relay. Accepting it made Receiver.EvaluateJitter panic in sim.AddMs.
func TestCallStartWithOutOfRangeCreatedAtRejected(t *testing.T) {
	maxSafe := protocol.MaxSafeCreatedAtMs
	tests := []struct {
		name    string
		created int64
		now     int64
	}{
		{"negative within threshold", -100, 100},
		{"above MaxSafeCreatedAtMs", maxSafe + 1, maxSafe + 1},
		{"near MaxInt64", math.MaxInt64 - 100, math.MaxInt64 - 100},
	}
	for _, tc := range tests {
		start := makeCallStartFrameAt(t, tc.created, nil)
		if err := start.VerifyIntegrity(); err != nil {
			t.Fatalf("%s: fixture must be CRC-valid: %v", tc.name, err)
		}

		t.Run(tc.name+"/receiver", func(t *testing.T) {
			receiver := NewReceiverStation(3, NewReceiverLifecycle(DefaultLifecyclePolicy()), DefaultReceiverPolicy())
			if err := receiver.ProcessCallStart(start, tc.now); err == nil {
				t.Fatal("call_start accepted, want created_at_ms range error")
			}
			if receiver.Lifecycle().HasCall(CallKeyFromFrame(start)) {
				t.Fatal("call state created for rejected call_start")
			}
			receiver.EvaluateJitter(tc.now)
		})

		t.Run(tc.name+"/relay", func(t *testing.T) {
			relay := NewRelayStation(2, NewRelayLifecycle(DefaultLifecyclePolicy()), DefaultRelayPolicy())
			if err := relay.ProcessCallStart(start, tc.now); err == nil {
				t.Fatal("call_start accepted, want created_at_ms range error")
			}
			if relay.Lifecycle().HasCall(CallKeyFromFrame(start)) {
				t.Fatal("call state created for rejected call_start")
			}
		})
	}
}

// Lifecycle APIs called directly (scheduled peer-control start) reject an
// out-of-range created_at_ms too, so EvaluateJitter never sees it.
func TestLifecycleCallStartRejectsOutOfRangeCreatedAt(t *testing.T) {
	for _, created := range []int64{-100, protocol.MaxSafeCreatedAtMs + 1} {
		start := makeCallStartFrameAt(t, created, nil)

		receiver := NewReceiverLifecycle(DefaultLifecyclePolicy())
		if err := receiver.ProcessCallStart(start, 100); err == nil || !strings.Contains(err.Error(), "is outside 0..") {
			t.Fatalf("created=%d: receiver lifecycle error = %v, want range error", created, err)
		}
		if receiver.HasCall(CallKeyFromFrame(start)) {
			t.Fatalf("created=%d: receiver lifecycle created call state", created)
		}
		receiver.EvaluateJitter(100, DefaultReceiverJitterBufferMs, DefaultReceiverLateThresholdMs)

		relay := NewRelayLifecycle(DefaultLifecyclePolicy())
		if err := relay.ProcessCallStart(start, 100); err == nil || !strings.Contains(err.Error(), "is outside 0..") {
			t.Fatalf("created=%d: relay lifecycle error = %v, want range error", created, err)
		}
		if relay.HasCall(CallKeyFromFrame(start)) {
			t.Fatalf("created=%d: relay lifecycle created call state", created)
		}
	}
}

// Each age failure of call_start keeps its own error text at both stations.
func TestCallStartAgeErrorTexts(t *testing.T) {
	maxSafe := protocol.MaxSafeCreatedAtMs
	tests := []struct {
		name    string
		created int64
		now     int64
		want    string
	}{
		{"stale", 1_000, 10_000, "is stale at"},
		{"future", 2_000, 1_000, "is after now_ms"},
		{"negative within threshold", -100, 100, "is outside 0.."},
		{"above MaxSafeCreatedAtMs", maxSafe + 1, maxSafe + 1, "is outside 0.."},
	}
	for _, tc := range tests {
		start := makeCallStartFrameAt(t, tc.created, nil)
		receiver := NewReceiverStation(3, NewReceiverLifecycle(DefaultLifecyclePolicy()), DefaultReceiverPolicy())
		if err := receiver.ProcessCallStart(start, tc.now); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: receiver error = %v, want %q", tc.name, err, tc.want)
		}
		relay := NewRelayStation(2, NewRelayLifecycle(DefaultLifecyclePolicy()), DefaultRelayPolicy())
		if err := relay.ProcessCallStart(start, tc.now); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: relay error = %v, want %q", tc.name, err, tc.want)
		}
	}
}

func TestAncientCallStartRejectedAsStale(t *testing.T) {
	start := makeCallStartFrameAt(t, math.MinInt64+5, nil)
	receiver := NewReceiverStation(3, NewReceiverLifecycle(DefaultLifecyclePolicy()), DefaultReceiverPolicy())
	err := receiver.ProcessCallStart(start, 1_000)
	if err == nil || receiver.Lifecycle().HasCall(CallKeyFromFrame(start)) {
		t.Fatalf("ancient call_start accepted: err=%v", err)
	}
	if reason, drop := frameAgeDropReason(start, 1_000, DefaultReceiverStaleThresholdMs); !drop || reason != DropReasonStaleFrame {
		t.Fatalf("frameAgeDropReason = (%q, %v), want stale", reason, drop)
	}
}

// Both period bounds and both created_at_ms bounds are accepted and
// EvaluateJitter walks every expected sequence without panicking.
func TestEvaluateJitterAtFramePeriodAndCreatedAtBounds(t *testing.T) {
	for _, period := range []int64{1, protocol.MaxFramePeriodMs} {
		for _, created := range []int64{0, protocol.MaxSafeCreatedAtMs} {
			t.Run(fmt.Sprintf("period=%d/created=%d", period, created), func(t *testing.T) {
				raw, err := protocol.EncodeCallStartPayload(protocol.CallStartPayload{
					FramePeriodMs:      period,
					PayloadMode:        "dummy",
					ExpectedFrameCount: uint32Ptr(3),
					TTLDefault:         1,
				})
				if err != nil {
					t.Fatalf("EncodeCallStartPayload: %v", err)
				}
				start := makeCallStartFrameAt(t, created, raw)
				receiver := NewReceiverStation(3, NewReceiverLifecycle(DefaultLifecyclePolicy()), DefaultReceiverPolicy())
				if err := receiver.ProcessCallStart(start, created); err != nil {
					t.Fatalf("ProcessCallStart: %v", err)
				}
				eval := receiver.EvaluateJitter(created + 3*period + DefaultReceiverLateThresholdMs + 1)
				if len(eval.Missing) != 3 {
					t.Fatalf("missing = %+v, want 3 sequences", eval.Missing)
				}
			})
		}
	}
}

// MaxSafeCreatedAtMs is exact: the last 32-bit seq at the maximum period lands
// on MaxInt64 without overflow.
func TestExpectedCreatedAtMsFitsInt64AtSafeBounds(t *testing.T) {
	state := CallState{
		Key:         CallKey{NetID: testNetID, SourceID: testSource, CallID: testCallID},
		CreatedAtMs: protocol.MaxSafeCreatedAtMs,
		Start:       protocol.CallStartPayload{FramePeriodMs: protocol.MaxFramePeriodMs},
	}
	if got := expectedCreatedAtMs(state, math.MaxUint32); got != math.MaxInt64 {
		t.Fatalf("expectedCreatedAtMs = %d, want MaxInt64", got)
	}
}

func TestCallStartWithOverlongFramePeriodRejected(t *testing.T) {
	for _, period := range []string{"9223372036854775807", "60001"} {
		raw := []byte(`{"frame_period_ms":` + period + `,"payload_mode":"dummy","slot_mode_enabled":false,"ttl_default":1}`)
		start := makeCallStartFrameAt(t, 1_000, raw)

		receiver := NewReceiverStation(3, NewReceiverLifecycle(DefaultLifecyclePolicy()), DefaultReceiverPolicy())
		if err := receiver.ProcessCallStart(start, 1_000); err == nil {
			t.Fatalf("receiver: frame_period_ms=%s accepted, want validation error", period)
		}
		if receiver.Lifecycle().HasCall(CallKeyFromFrame(start)) {
			t.Fatalf("receiver: frame_period_ms=%s created call state", period)
		}
		receiver.EvaluateJitter(math.MaxInt32)

		relay := NewRelayStation(2, NewRelayLifecycle(DefaultLifecyclePolicy()), DefaultRelayPolicy())
		if err := relay.ProcessCallStart(start, 1_000); err == nil {
			t.Fatalf("relay: frame_period_ms=%s accepted, want validation error", period)
		}
	}
}

func uint32Ptr(v uint32) *uint32 { return &v }
