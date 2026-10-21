package protocol

import (
	"fmt"
	"math"
	"testing"
)

func TestCallStartPayloadFramePeriodBounds(t *testing.T) {
	if MaxFramePeriodMs != 60000 {
		t.Fatalf("MaxFramePeriodMs = %d, want 60000", MaxFramePeriodMs)
	}
	tests := []struct {
		name    string
		period  int64
		wantErr bool
	}{
		{"lower bound", 1, false},
		{"typical", 80, false},
		{"upper bound", MaxFramePeriodMs, false},
		{"above upper bound", MaxFramePeriodMs + 1, true},
		{"MaxInt64", math.MaxInt64, true},
		{"zero", 0, true},
		{"negative", -1, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload := CallStartPayload{FramePeriodMs: tc.period, PayloadMode: "dummy", TTLDefault: 1}
			if err := payload.Validate(); (err != nil) != tc.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
			if _, err := EncodeCallStartPayload(payload); (err != nil) != tc.wantErr {
				t.Fatalf("EncodeCallStartPayload error = %v, wantErr %v", err, tc.wantErr)
			}
			raw := []byte(fmt.Sprintf(`{"frame_period_ms":%d,"payload_mode":"dummy","slot_mode_enabled":false,"ttl_default":1}`, tc.period))
			decoded, err := DecodeCallStartPayload(raw)
			if (err != nil) != tc.wantErr {
				t.Fatalf("DecodeCallStartPayload error = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && decoded.FramePeriodMs != tc.period {
				t.Fatalf("decoded FramePeriodMs = %d, want %d", decoded.FramePeriodMs, tc.period)
			}
		})
	}
}

// MaxSafeCreatedAtMs leaves exactly enough headroom for created_at_ms +
// seq*frame_period_ms with every 32-bit seq at the maximum period.
func TestMaxSafeCreatedAtMsHeadroom(t *testing.T) {
	headroom := uint64(math.MaxInt64) - uint64(MaxSafeCreatedAtMs)
	if want := uint64(math.MaxUint32) * uint64(MaxFramePeriodMs); headroom != want {
		t.Fatalf("headroom = %d, want MaxUint32*MaxFramePeriodMs = %d", headroom, want)
	}
	if MaxSafeCreatedAtMs <= 0 {
		t.Fatalf("MaxSafeCreatedAtMs = %d, want positive", MaxSafeCreatedAtMs)
	}
}
