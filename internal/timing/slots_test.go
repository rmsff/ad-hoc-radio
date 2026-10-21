package timing

import (
	"math"
	"testing"
)

// TestUT010_TwoSlotIndexCalculation proves slot index and owner
// calculation for the canonical two-slot SFR schedule.
//
// Traceability: UT-010 (FR-6, NFR-1).
func TestUT010_TwoSlotIndexCalculation(t *testing.T) {
	s := NewTwoSlotSFR(0, 40)
	tests := []struct {
		timeMs int64
		index  int
		name   string
		owner  SlotOwner
	}{
		{0, 0, SourceSlotName, SlotOwnerSource},
		{39, 0, SourceSlotName, SlotOwnerSource},
		{40, 1, RelaySlotName, SlotOwnerRelay},
		{79, 1, RelaySlotName, SlotOwnerRelay},
		{80, 0, SourceSlotName, SlotOwnerSource},
		{120, 1, RelaySlotName, SlotOwnerRelay},
	}

	for _, tc := range tests {
		slot := s.SlotAt(tc.timeMs)
		if got := s.SlotIndexAt(tc.timeMs); got != tc.index {
			t.Errorf("SlotIndexAt(%d) = %d, want %d", tc.timeMs, got, tc.index)
		}
		if slot.Name != tc.name || slot.Owner != tc.owner {
			t.Errorf("SlotAt(%d) = {%q %q}, want {%q %q}", tc.timeMs, slot.Name, slot.Owner, tc.name, tc.owner)
		}
	}
}

// TestUT010_NextOwnerSlotStart proves the relay waits until its assigned
// slot when it receives a frame outside relay_slot.
func TestUT010_NextOwnerSlotStart(t *testing.T) {
	s := NewTwoSlotSFR(0, 40)
	tests := []struct {
		nowMs int64
		owner SlotOwner
		want  int64
	}{
		{0, SlotOwnerRelay, 40},
		{10, SlotOwnerRelay, 40},
		{40, SlotOwnerRelay, 40},
		{79, SlotOwnerRelay, 79},
		{80, SlotOwnerRelay, 120},
		{10, SlotOwnerSource, 10},
		{40, SlotOwnerSource, 80},
	}

	for _, tc := range tests {
		if got := s.NextOwnerSlotStart(tc.nowMs, tc.owner); got != tc.want {
			t.Errorf("NextOwnerSlotStart(%d, %q) = %d, want %d", tc.nowMs, tc.owner, got, tc.want)
		}
	}
}

func TestUT010_ScheduleOrigin(t *testing.T) {
	s := NewTwoSlotSFR(1000, 40)
	if got := s.NextOwnerSlotStart(990, SlotOwnerSource); got != 1000 {
		t.Errorf("source next before origin = %d, want 1000", got)
	}
	if got := s.NextOwnerSlotStart(990, SlotOwnerRelay); got != 1040 {
		t.Errorf("relay next before origin = %d, want 1040", got)
	}
	if !s.IsOwnerSlot(1040, SlotOwnerRelay) {
		t.Error("1040 should be relay slot with origin 1000")
	}
}

func TestUT010_InvalidSchedulesPanic(t *testing.T) {
	tests := []struct {
		name string
		fn   func()
	}{
		{"negative origin", func() { NewTwoSlotSFR(-1, 40) }},
		{"zero duration", func() { NewTwoSlotSFR(0, 0) }},
		{"overflow duration", func() { NewTwoSlotSFR(0, math.MaxInt64) }},
		{"origin last slot overflow", func() { NewTwoSlotSFR(math.MaxInt64, 1) }},
		{"missing relay owner", func() {
			s := Schedule{SlotDurationMs: 40, Slots: []Slot{{Name: SourceSlotName, Owner: SlotOwnerSource}}}
			s.SuperframeDurationMs()
		}},
		{"time before origin", func() {
			s := NewTwoSlotSFR(100, 40)
			s.SlotAt(99)
		}},
		{"owner not present", func() {
			s := NewTwoSlotSFR(0, 40)
			s.NextOwnerSlotStart(0, SlotOwner("other"))
		}},
		{"next boundary overflow", func() {
			s := NewTwoSlotSFR(0, 1)
			s.NextOwnerSlotStart(math.MaxInt64, SlotOwnerSource)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("%s did not panic", tc.name)
				}
			}()
			tc.fn()
		})
	}
}
