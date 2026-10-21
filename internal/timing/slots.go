package timing

import (
	"fmt"
	"math"
)

// SlotOwner identifies which station role may transmit in a slot.
type SlotOwner string

const (
	SlotOwnerSource SlotOwner = "source"
	SlotOwnerRelay  SlotOwner = "relay"
)

const (
	SourceSlotName = "source_slot"
	RelaySlotName  = "relay_slot"
)

// Slot is one entry in a repeating superframe.
type Slot struct {
	Name  string
	Owner SlotOwner
}

// Schedule is an ideal simulated-clock slot schedule. Time is expressed
// in milliseconds; real RF synchronization is explicitly out of scope.
type Schedule struct {
	OriginMs       int64
	SlotDurationMs int64
	Slots          []Slot
}

// NewTwoSlotSFR returns the MVP two-slot SFR schedule:
// Slot 0 = source_slot, Slot 1 = relay_slot.
func NewTwoSlotSFR(originMs, slotDurationMs int64) Schedule {
	s := Schedule{
		OriginMs:       originMs,
		SlotDurationMs: slotDurationMs,
		Slots: []Slot{
			{Name: SourceSlotName, Owner: SlotOwnerSource},
			{Name: RelaySlotName, Owner: SlotOwnerRelay},
		},
	}
	if err := s.Validate(); err != nil {
		panic(fmt.Sprintf("timing: invalid two-slot SFR schedule: %v", err))
	}
	return s
}

// Validate checks schedule invariants that would break deterministic slot math.
func (s Schedule) Validate() error {
	if s.OriginMs < 0 {
		return fmt.Errorf("origin_ms=%d must be >= 0", s.OriginMs)
	}
	if s.SlotDurationMs <= 0 {
		return fmt.Errorf("slot_duration_ms=%d must be > 0", s.SlotDurationMs)
	}
	if len(s.Slots) == 0 {
		return fmt.Errorf("slots must not be empty")
	}
	if int64(len(s.Slots)) > math.MaxInt64/s.SlotDurationMs {
		return fmt.Errorf("superframe duration overflows int64: slots=%d slot_duration_ms=%d", len(s.Slots), s.SlotDurationMs)
	}
	lastSlotIndex := int64(len(s.Slots) - 1)
	if lastSlotIndex > 0 {
		lastSlotOffset := lastSlotIndex * s.SlotDurationMs
		if s.OriginMs > math.MaxInt64-lastSlotOffset {
			return fmt.Errorf("last slot start overflows int64: origin_ms=%d last_slot_offset_ms=%d", s.OriginMs, lastSlotOffset)
		}
	}
	seenNames := make(map[string]bool, len(s.Slots))
	seenOwners := make(map[SlotOwner]bool, len(s.Slots))
	for i, slot := range s.Slots {
		if slot.Name == "" {
			return fmt.Errorf("slot %d has empty name", i)
		}
		if seenNames[slot.Name] {
			return fmt.Errorf("slot %d duplicates name %q", i, slot.Name)
		}
		seenNames[slot.Name] = true
		if slot.Owner == "" {
			return fmt.Errorf("slot %d has empty owner", i)
		}
		seenOwners[slot.Owner] = true
	}
	if !seenOwners[SlotOwnerSource] {
		return fmt.Errorf("schedule has no source slot")
	}
	if !seenOwners[SlotOwnerRelay] {
		return fmt.Errorf("schedule has no relay slot")
	}
	return nil
}

// SuperframeDurationMs returns slot_duration_ms * len(slots).
func (s Schedule) SuperframeDurationMs() int64 {
	mustValidate(s)
	return s.SlotDurationMs * int64(len(s.Slots))
}

// SlotIndexAt returns the slot index active at timeMs.
func (s Schedule) SlotIndexAt(timeMs int64) int {
	mustValidate(s)
	if timeMs < s.OriginMs {
		panic(fmt.Sprintf("timing: time_ms=%d is before schedule origin_ms=%d", timeMs, s.OriginMs))
	}
	elapsed := timeMs - s.OriginMs
	slotNumber := elapsed / s.SlotDurationMs
	return int(slotNumber % int64(len(s.Slots)))
}

// SlotAt returns the active slot at timeMs.
func (s Schedule) SlotAt(timeMs int64) Slot {
	return s.Slots[s.SlotIndexAt(timeMs)]
}

// IsOwnerSlot reports whether owner may transmit at timeMs.
func (s Schedule) IsOwnerSlot(timeMs int64, owner SlotOwner) bool {
	return s.SlotAt(timeMs).Owner == owner
}

// NextOwnerSlotStart returns the earliest time >= nowMs when owner may
// transmit. If nowMs is already inside owner's slot, nowMs is returned so
// callers can transmit immediately within the active slot.
func (s Schedule) NextOwnerSlotStart(nowMs int64, owner SlotOwner) int64 {
	mustValidate(s)
	if owner == "" {
		panic("timing: empty slot owner")
	}
	if !s.hasOwner(owner) {
		panic(fmt.Sprintf("timing: owner %q is not present in schedule", owner))
	}
	if nowMs < 0 {
		panic(fmt.Sprintf("timing: now_ms=%d must be >= 0", nowMs))
	}
	if nowMs < s.OriginMs {
		return s.firstOwnerSlotFromOrigin(owner)
	}
	if s.IsOwnerSlot(nowMs, owner) {
		return nowMs
	}

	currentIdx := s.SlotIndexAt(nowMs)
	elapsed := nowMs - s.OriginMs
	currentSlotNumber := elapsed / s.SlotDurationMs
	if currentSlotNumber == math.MaxInt64 {
		panic("timing: next slot boundary overflows int64")
	}
	nextBoundarySlotNumber := currentSlotNumber + 1
	nextBoundary := addMs(s.OriginMs, mulMs(nextBoundarySlotNumber, s.SlotDurationMs))
	for offset := 0; offset < len(s.Slots); offset++ {
		idx := (currentIdx + 1 + offset) % len(s.Slots)
		candidate := addMs(nextBoundary, mulMs(int64(offset), s.SlotDurationMs))
		if s.Slots[idx].Owner == owner {
			return candidate
		}
	}
	panic(fmt.Sprintf("timing: owner %q disappeared during schedule scan", owner))
}

func (s Schedule) firstOwnerSlotFromOrigin(owner SlotOwner) int64 {
	for idx, slot := range s.Slots {
		if slot.Owner == owner {
			return addMs(s.OriginMs, mulMs(int64(idx), s.SlotDurationMs))
		}
	}
	panic(fmt.Sprintf("timing: owner %q is not present in schedule", owner))
}

func (s Schedule) hasOwner(owner SlotOwner) bool {
	for _, slot := range s.Slots {
		if slot.Owner == owner {
			return true
		}
	}
	return false
}

func mustValidate(s Schedule) {
	if err := s.Validate(); err != nil {
		panic(fmt.Sprintf("timing: invalid schedule: %v", err))
	}
}

func addMs(a, b int64) int64 {
	if a < 0 || b < 0 {
		panic("timing: negative simulated-time operand")
	}
	if a > math.MaxInt64-b {
		panic("timing: simulated-time addition overflow")
	}
	return a + b
}

func mulMs(a, b int64) int64 {
	if a < 0 || b < 0 {
		panic("timing: negative simulated-time operand")
	}
	if a != 0 && b > math.MaxInt64/a {
		panic("timing: simulated-time multiplication overflow")
	}
	return a * b
}
