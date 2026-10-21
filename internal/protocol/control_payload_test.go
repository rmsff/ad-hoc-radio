package protocol

import (
	"strings"
	"testing"
)

func TestCallStartPayload_RoundTrip(t *testing.T) {
	expectedCount := uint32(100)
	slotDuration := int64(40)
	want := CallStartPayload{
		FramePeriodMs:      80,
		PayloadMode:        "dummy",
		ExpectedFrameCount: &expectedCount,
		SlotModeEnabled:    true,
		SlotDurationMs:     &slotDuration,
		TTLDefault:         1,
	}

	raw, err := EncodeCallStartPayload(want)
	if err != nil {
		t.Fatalf("EncodeCallStartPayload failed: %v", err)
	}
	got, err := DecodeCallStartPayload(raw)
	if err != nil {
		t.Fatalf("DecodeCallStartPayload failed: %v", err)
	}

	if got.FramePeriodMs != want.FramePeriodMs {
		t.Errorf("FramePeriodMs = %d, want %d", got.FramePeriodMs, want.FramePeriodMs)
	}
	if got.PayloadMode != want.PayloadMode {
		t.Errorf("PayloadMode = %q, want %q", got.PayloadMode, want.PayloadMode)
	}
	if got.ExpectedFrameCount == nil || *got.ExpectedFrameCount != expectedCount {
		t.Fatalf("ExpectedFrameCount = %v, want %d", got.ExpectedFrameCount, expectedCount)
	}
	if got.SlotDurationMs == nil || *got.SlotDurationMs != slotDuration {
		t.Fatalf("SlotDurationMs = %v, want %d", got.SlotDurationMs, slotDuration)
	}
	if !got.SlotModeEnabled {
		t.Error("SlotModeEnabled = false, want true")
	}
	if got.TTLDefault != want.TTLDefault {
		t.Errorf("TTLDefault = %d, want %d", got.TTLDefault, want.TTLDefault)
	}
}

func TestCallStartPayload_ScheduledContextRoundTrip(t *testing.T) {
	expectedCount := uint32(8)
	scheduleID := uint32(100)
	scheduleVersion := uint16(1)
	scheduleDigest := strings.Repeat("a", 64)
	timingStrategy := CallStartTimingStrategySourceLed
	want := CallStartPayload{
		FramePeriodMs:      80,
		PayloadMode:        "dummy",
		ExpectedFrameCount: &expectedCount,
		SlotModeEnabled:    false,
		TTLDefault:         3,
		ScheduleID:         &scheduleID,
		ScheduleVersion:    &scheduleVersion,
		ScheduleDigest:     &scheduleDigest,
		TimingStrategy:     &timingStrategy,
	}

	raw, err := EncodeCallStartPayload(want)
	if err != nil {
		t.Fatalf("EncodeCallStartPayload failed: %v", err)
	}
	got, err := DecodeCallStartPayload(raw)
	if err != nil {
		t.Fatalf("DecodeCallStartPayload failed: %v", err)
	}
	if got.ScheduleID == nil || *got.ScheduleID != scheduleID {
		t.Fatalf("ScheduleID = %v, want %d", got.ScheduleID, scheduleID)
	}
	if got.ScheduleVersion == nil || *got.ScheduleVersion != scheduleVersion {
		t.Fatalf("ScheduleVersion = %v, want %d", got.ScheduleVersion, scheduleVersion)
	}
	if got.ScheduleDigest == nil || *got.ScheduleDigest != scheduleDigest {
		t.Fatalf("ScheduleDigest = %v, want %q", got.ScheduleDigest, scheduleDigest)
	}
	if got.TimingStrategy == nil || *got.TimingStrategy != timingStrategy {
		t.Fatalf("TimingStrategy = %v, want %q", got.TimingStrategy, timingStrategy)
	}
}

func TestCallStartPayload_LegacyEncodingUnchanged(t *testing.T) {
	expectedCount := uint32(100)
	slotDuration := int64(40)
	payload := CallStartPayload{
		FramePeriodMs:      80,
		PayloadMode:        "dummy",
		ExpectedFrameCount: &expectedCount,
		SlotModeEnabled:    true,
		SlotDurationMs:     &slotDuration,
		TTLDefault:         1,
	}

	raw, err := EncodeCallStartPayload(payload)
	if err != nil {
		t.Fatalf("EncodeCallStartPayload failed: %v", err)
	}
	want := `{"frame_period_ms":80,"payload_mode":"dummy","expected_frame_count":100,"slot_mode_enabled":true,"slot_duration_ms":40,"ttl_default":1}`
	if string(raw) != want {
		t.Fatalf("legacy JSON = %s, want %s", raw, want)
	}
}

func TestCallStartPayload_RejectsInvalid(t *testing.T) {
	validSlot := int64(40)
	validCount := uint32(10)
	validScheduleID := uint32(100)
	validScheduleVersion := uint16(1)
	validDigest := strings.Repeat("a", 64)
	validStrategy := CallStartTimingStrategySourceLed
	cases := []struct {
		name    string
		payload CallStartPayload
	}{
		{
			name: "zero frame period",
			payload: CallStartPayload{
				PayloadMode:     "dummy",
				SlotModeEnabled: false,
				TTLDefault:      1,
			},
		},
		{
			name: "empty payload mode",
			payload: CallStartPayload{
				FramePeriodMs:   80,
				SlotModeEnabled: false,
				TTLDefault:      1,
			},
		},
		{
			name: "slot mode without duration",
			payload: CallStartPayload{
				FramePeriodMs:   80,
				PayloadMode:     "dummy",
				SlotModeEnabled: true,
				TTLDefault:      1,
			},
		},
		{
			name: "zero slot duration",
			payload: CallStartPayload{
				FramePeriodMs:   80,
				PayloadMode:     "dummy",
				SlotModeEnabled: true,
				SlotDurationMs:  ptrInt64(0),
				TTLDefault:      1,
			},
		},
		{
			name: "zero ttl default",
			payload: CallStartPayload{
				FramePeriodMs:   80,
				PayloadMode:     "dummy",
				SlotModeEnabled: false,
				TTLDefault:      0,
			},
		},
		{
			name: "zero expected frame count",
			payload: CallStartPayload{
				FramePeriodMs:      80,
				PayloadMode:        "dummy",
				ExpectedFrameCount: ptrUint32(0),
				SlotModeEnabled:    true,
				SlotDurationMs:     &validSlot,
				TTLDefault:         1,
			},
		},
		{
			name: "partial schedule context",
			payload: CallStartPayload{
				FramePeriodMs:   80,
				PayloadMode:     "dummy",
				SlotModeEnabled: false,
				TTLDefault:      1,
				ScheduleID:      &validScheduleID,
			},
		},
		{
			name: "zero schedule id",
			payload: scheduledCallStartTestPayload(
				0,
				validScheduleVersion,
				validDigest,
				validStrategy,
			),
		},
		{
			name: "zero schedule version",
			payload: scheduledCallStartTestPayload(
				validScheduleID,
				0,
				validDigest,
				validStrategy,
			),
		},
		{
			name: "uppercase schedule digest",
			payload: scheduledCallStartTestPayload(
				validScheduleID,
				validScheduleVersion,
				strings.Repeat("A", 64),
				validStrategy,
			),
		},
		{
			name: "short schedule digest",
			payload: scheduledCallStartTestPayload(
				validScheduleID,
				validScheduleVersion,
				"abc",
				validStrategy,
			),
		},
		{
			name: "invalid timing strategy",
			payload: scheduledCallStartTestPayload(
				validScheduleID,
				validScheduleVersion,
				validDigest,
				CallStartTimingStrategy("other"),
			),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := EncodeCallStartPayload(tc.payload); err == nil {
				t.Fatal("EncodeCallStartPayload accepted invalid payload; expected error")
			}
		})
	}

	if _, err := EncodeCallStartPayload(CallStartPayload{
		FramePeriodMs:      80,
		PayloadMode:        "dummy",
		ExpectedFrameCount: &validCount,
		SlotModeEnabled:    true,
		SlotDurationMs:     &validSlot,
		TTLDefault:         1,
	}); err != nil {
		t.Fatalf("valid control payload rejected: %v", err)
	}
}

func TestDecodeCallStartPayload_RejectsUnknownField(t *testing.T) {
	raw := []byte(`{"frame_period_ms":80,"payload_mode":"dummy","slot_mode_enabled":false,"ttl_default":1,"unexpected":true}`)

	_, err := DecodeCallStartPayload(raw)
	if err == nil {
		t.Fatal("DecodeCallStartPayload accepted unknown field; expected error")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("DecodeCallStartPayload error = %v, want unknown field", err)
	}
}

func TestDecodeCallStartPayload_RejectsMissingRequiredFields(t *testing.T) {
	cases := []string{
		`{"payload_mode":"dummy","slot_mode_enabled":false,"ttl_default":1}`,
		`{"frame_period_ms":80,"slot_mode_enabled":false,"ttl_default":1}`,
		`{"frame_period_ms":80,"payload_mode":"dummy","ttl_default":1}`,
		`{"frame_period_ms":80,"payload_mode":"dummy","slot_mode_enabled":false}`,
		`{"frame_period_ms":80,"payload_mode":"dummy","slot_mode_enabled":null,"ttl_default":1}`,
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			if _, err := DecodeCallStartPayload([]byte(raw)); err == nil {
				t.Fatal("DecodeCallStartPayload accepted payload with missing/null required field; expected error")
			}
		})
	}
}

func TestCallEndPayload_RoundTrip(t *testing.T) {
	want := CallEndPayload{LastSeq: 99, Reason: CallEndReasonNormal}

	raw, err := EncodeCallEndPayload(want)
	if err != nil {
		t.Fatalf("EncodeCallEndPayload failed: %v", err)
	}
	got, err := DecodeCallEndPayload(raw)
	if err != nil {
		t.Fatalf("DecodeCallEndPayload failed: %v", err)
	}
	if got != want {
		t.Fatalf("CallEndPayload = %+v, want %+v", got, want)
	}
}

func TestDecodeCallEndPayload_RejectsMissingLastSeq(t *testing.T) {
	cases := []string{
		`{"reason":"normal"}`,
		`{"last_seq":null,"reason":"normal"}`,
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			if _, err := DecodeCallEndPayload([]byte(raw)); err == nil {
				t.Fatal("DecodeCallEndPayload accepted missing/null last_seq; expected error")
			}
		})
	}
}

func TestCallEndPayload_RejectsInvalidReason(t *testing.T) {
	if _, err := EncodeCallEndPayload(CallEndPayload{LastSeq: 1, Reason: "other"}); err == nil {
		t.Fatal("EncodeCallEndPayload accepted invalid reason; expected error")
	}
	if _, err := DecodeCallEndPayload([]byte(`{"last_seq":1,"reason":"other"}`)); err == nil {
		t.Fatal("DecodeCallEndPayload accepted invalid reason; expected error")
	}
}

func ptrInt64(v int64) *int64 { return &v }

func ptrUint32(v uint32) *uint32 { return &v }

func scheduledCallStartTestPayload(scheduleID uint32, scheduleVersion uint16, digest string, strategy CallStartTimingStrategy) CallStartPayload {
	return CallStartPayload{
		FramePeriodMs:   80,
		PayloadMode:     "dummy",
		SlotModeEnabled: false,
		TTLDefault:      1,
		ScheduleID:      &scheduleID,
		ScheduleVersion: &scheduleVersion,
		ScheduleDigest:  &digest,
		TimingStrategy:  &strategy,
	}
}
