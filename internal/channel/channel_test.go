package channel

import (
	"bytes"
	"math"
	"reflect"
	"strconv"
	"testing"
)

func TestUT015_MeasureFrameRecordsSlotMissWithoutError(t *testing.T) {
	frame := make([]byte, 56)

	m, err := MeasureFrame(frame, AirtimeConfig{
		ChannelBitrateBps: 9600,
		SlotDurationMs:    40,
	})
	if err != nil {
		t.Fatalf("MeasureFrame failed: %v", err)
	}

	if m.FrameBytes != 56 {
		t.Fatalf("FrameBytes = %d, want 56", m.FrameBytes)
	}
	if m.FrameBits != 448 {
		t.Fatalf("FrameBits = %d, want 448", m.FrameBits)
	}
	if m.AirtimeMs != 47 {
		t.Fatalf("AirtimeMs = %d, want 47", m.AirtimeMs)
	}
	if m.FitsSlot {
		t.Fatal("FitsSlot = true, want false for 47 ms frame in 40 ms slot")
	}
}

func TestUT015_MeasureFrameCompressedCandidateFitsSlot(t *testing.T) {
	frame := make([]byte, 22)

	m, err := MeasureFrame(frame, AirtimeConfig{
		ChannelBitrateBps: 9600,
		SlotDurationMs:    40,
	})
	if err != nil {
		t.Fatalf("MeasureFrame failed: %v", err)
	}

	if m.AirtimeMs != 19 {
		t.Fatalf("AirtimeMs = %d, want 19", m.AirtimeMs)
	}
	if !m.FitsSlot {
		t.Fatal("FitsSlot = false, want true for 19 ms frame in 40 ms slot")
	}
}

func TestUT015_MeasureFrameBytesMatchesMeasureFrame(t *testing.T) {
	cfg := AirtimeConfig{
		ChannelBitrateBps: 9600,
		SlotDurationMs:    40,
	}
	fromBytes, err := MeasureFrameBytes(22, cfg)
	if err != nil {
		t.Fatalf("MeasureFrameBytes failed: %v", err)
	}
	fromFrame, err := MeasureFrame(make([]byte, 22), cfg)
	if err != nil {
		t.Fatalf("MeasureFrame failed: %v", err)
	}
	if !reflect.DeepEqual(fromBytes, fromFrame) {
		t.Fatalf("MeasureFrameBytes != MeasureFrame:\nbytes=%+v\nframe=%+v", fromBytes, fromFrame)
	}
}

func TestUT015_MeasureFrameBytesRejectsHugeSizeWithoutAllocation(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("huge byte-count overflow guard is only reachable on 64-bit int platforms")
	}
	huge := int(int64(math.MaxInt64/8) + 1)

	_, err := MeasureFrameBytes(huge, AirtimeConfig{
		ChannelBitrateBps: 9600,
		SlotDurationMs:    40,
	})
	if err == nil {
		t.Fatal("MeasureFrameBytes accepted overflowing frame byte count")
	}
}

func TestUT015_LoopbackSlotMissDoesNotDrop(t *testing.T) {
	report, err := RunLoopback([]Frame{
		{ID: "canonical-voice", Bytes: make([]byte, 56), SentAtMs: 100},
	}, Config{
		ChannelBitrateBps: 9600,
		SlotDurationMs:    40,
	})
	if err != nil {
		t.Fatalf("RunLoopback failed: %v", err)
	}

	if report.Counters.SlotMisses != 1 {
		t.Fatalf("SlotMisses = %d, want 1", report.Counters.SlotMisses)
	}
	if report.Counters.Delivered != 1 {
		t.Fatalf("Delivered = %d, want 1; slot miss must not drop", report.Counters.Delivered)
	}
	if report.Counters.Lost != 0 {
		t.Fatalf("Lost = %d, want 0", report.Counters.Lost)
	}
	if len(report.Deliveries) != 1 {
		t.Fatalf("deliveries = %d, want 1", len(report.Deliveries))
	}
	if report.Deliveries[0].ReceivedAtMs != 147 {
		t.Fatalf("ReceivedAtMs = %d, want 147", report.Deliveries[0].ReceivedAtMs)
	}
}

func TestUT015_LoopbackDeterministicForSameSeed(t *testing.T) {
	frames := []Frame{
		{ID: "a", Bytes: []byte{0x01, 0x02, 0x03}, SentAtMs: 10},
		{ID: "b", Bytes: []byte{0x04, 0x05, 0x06}, SentAtMs: 20},
		{ID: "c", Bytes: []byte{0x07, 0x08, 0x09}, SentAtMs: 30},
	}
	cfg := Config{
		ChannelBitrateBps: 9600,
		SlotDurationMs:    40,
		Seed:              42,
		FixedDelayMs:      3,
		JitterMs:          7,
		LossPct:           25,
		DuplicatePct:      50,
		CorruptionPct:     50,
	}

	first, err := RunLoopback(frames, cfg)
	if err != nil {
		t.Fatalf("first RunLoopback failed: %v", err)
	}
	second, err := RunLoopback(frames, cfg)
	if err != nil {
		t.Fatalf("second RunLoopback failed: %v", err)
	}

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("same seed produced different reports:\nfirst=%+v\nsecond=%+v", first, second)
	}
}

func TestUT015_LoopbackDeliveryBytesAreDefensiveCopies(t *testing.T) {
	input := []byte{0xAA, 0xBB, 0xCC}
	report, err := RunLoopback([]Frame{
		{ID: "dup", Bytes: input, SentAtMs: 0},
	}, Config{
		ChannelBitrateBps: 9600,
		SlotDurationMs:    40,
		DuplicatePct:      100,
	})
	if err != nil {
		t.Fatalf("RunLoopback failed: %v", err)
	}
	if len(report.Deliveries) != 2 {
		t.Fatalf("deliveries = %d, want original + duplicate", len(report.Deliveries))
	}

	report.Deliveries[0].Bytes[0] = 0x00
	if !bytes.Equal(input, []byte{0xAA, 0xBB, 0xCC}) {
		t.Fatalf("input mutated through delivery: %x", input)
	}
	if !bytes.Equal(report.Deliveries[1].Bytes, []byte{0xAA, 0xBB, 0xCC}) {
		t.Fatalf("duplicate delivery shares bytes with original: %x", report.Deliveries[1].Bytes)
	}
}

func TestUT015_LoopbackCorruptionDoesNotMutateInput(t *testing.T) {
	input := []byte{0xAA, 0xBB, 0xCC}
	report, err := RunLoopback([]Frame{
		{ID: "corrupt", Bytes: input, SentAtMs: 0},
	}, Config{
		ChannelBitrateBps: 9600,
		SlotDurationMs:    40,
		CorruptionPct:     100,
	})
	if err != nil {
		t.Fatalf("RunLoopback failed: %v", err)
	}
	if len(report.Deliveries) != 1 {
		t.Fatalf("deliveries = %d, want 1", len(report.Deliveries))
	}
	if !report.Deliveries[0].Corrupted {
		t.Fatal("Corrupted = false, want true")
	}
	if !bytes.Equal(input, []byte{0xAA, 0xBB, 0xCC}) {
		t.Fatalf("input mutated by corruption: %x", input)
	}
	if !bytes.Equal(report.Deliveries[0].Bytes, []byte{0x55, 0xBB, 0xCC}) {
		t.Fatalf("delivery bytes = %x, want 55bbcc", report.Deliveries[0].Bytes)
	}
}

func TestUT015_LoopbackDoesNotCountEmptyFrameAsCorrupted(t *testing.T) {
	report, err := RunLoopback([]Frame{
		{ID: "empty", Bytes: nil, SentAtMs: 0},
	}, Config{
		ChannelBitrateBps: 9600,
		SlotDurationMs:    40,
		CorruptionPct:     100,
	})
	if err != nil {
		t.Fatalf("RunLoopback failed: %v", err)
	}
	if report.Counters.Corrupted != 0 {
		t.Fatalf("Corrupted counter = %d, want 0 for empty frame", report.Counters.Corrupted)
	}
	if len(report.Deliveries) != 1 {
		t.Fatalf("deliveries = %d, want 1", len(report.Deliveries))
	}
	if report.Deliveries[0].Corrupted {
		t.Fatal("empty delivery Corrupted = true, want false because no byte was mutated")
	}
}

func TestUT015_LoopbackRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{name: "negative bitrate", cfg: Config{ChannelBitrateBps: -1, SlotDurationMs: 40}},
		{name: "negative slot", cfg: Config{ChannelBitrateBps: 9600, SlotDurationMs: -1}},
		{name: "negative delay", cfg: Config{ChannelBitrateBps: 9600, SlotDurationMs: 40, FixedDelayMs: -1}},
		{name: "negative jitter", cfg: Config{ChannelBitrateBps: 9600, SlotDurationMs: 40, JitterMs: -1}},
		{name: "loss out of range", cfg: Config{ChannelBitrateBps: 9600, SlotDurationMs: 40, LossPct: 101}},
		{name: "loss NaN", cfg: Config{ChannelBitrateBps: 9600, SlotDurationMs: 40, LossPct: math.NaN()}},
		{name: "duplicate out of range", cfg: Config{ChannelBitrateBps: 9600, SlotDurationMs: 40, DuplicatePct: -1}},
		{name: "duplicate Inf", cfg: Config{ChannelBitrateBps: 9600, SlotDurationMs: 40, DuplicatePct: math.Inf(1)}},
		{name: "corruption out of range", cfg: Config{ChannelBitrateBps: 9600, SlotDurationMs: 40, CorruptionPct: 101}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := RunLoopback([]Frame{{ID: "x", Bytes: []byte{0x01}}}, tc.cfg); err == nil {
				t.Fatal("RunLoopback accepted invalid config")
			}
		})
	}
}

func TestUT015_LoopbackRejectsNegativeSentAt(t *testing.T) {
	_, err := RunLoopback([]Frame{
		{ID: "negative-time", Bytes: []byte{0x01}, SentAtMs: -1},
	}, Config{
		ChannelBitrateBps: 9600,
		SlotDurationMs:    40,
	})
	if err == nil {
		t.Fatal("RunLoopback accepted negative sent_at_ms")
	}
}

func TestUT015_LoopbackReturnsErrorOnReceiveTimeOverflow(t *testing.T) {
	_, err := RunLoopback([]Frame{
		{ID: "overflow", Bytes: []byte{0x01}, SentAtMs: math.MaxInt64},
	}, Config{
		ChannelBitrateBps: 9600,
		SlotDurationMs:    40,
	})
	if err == nil {
		t.Fatal("RunLoopback accepted overflowing receive time")
	}
}
