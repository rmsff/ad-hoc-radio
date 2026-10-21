package scenario

import (
	"fmt"
	"sort"
)

func validatePositiveInt64s(name string, values []int64) error {
	if len(values) == 0 {
		return fmt.Errorf("multi-hop timing sweep: %s must not be empty", name)
	}
	seen := make(map[int64]bool, len(values))
	for _, v := range values {
		if v <= 0 {
			return fmt.Errorf("multi-hop timing sweep: %s contains %d, want > 0", name, v)
		}
		if seen[v] {
			return fmt.Errorf("multi-hop timing sweep: %s contains duplicate %d", name, v)
		}
		seen[v] = true
	}
	return nil
}

func validatePositiveInts(name string, values []int) error {
	if len(values) == 0 {
		return fmt.Errorf("multi-hop timing sweep: %s must not be empty", name)
	}
	seen := make(map[int]bool, len(values))
	for _, v := range values {
		if v <= 0 {
			return fmt.Errorf("multi-hop timing sweep: %s contains %d, want > 0", name, v)
		}
		if seen[v] {
			return fmt.Errorf("multi-hop timing sweep: %s contains duplicate %d", name, v)
		}
		seen[v] = true
	}
	return nil
}

func validateNonNegativeInt64s(name string, values []int64) error {
	if len(values) == 0 {
		return fmt.Errorf("multi-hop timing sweep: %s must not be empty", name)
	}
	seen := make(map[int64]bool, len(values))
	for _, v := range values {
		if v < 0 {
			return fmt.Errorf("multi-hop timing sweep: %s contains %d, want >= 0", name, v)
		}
		if seen[v] {
			return fmt.Errorf("multi-hop timing sweep: %s contains duplicate %d", name, v)
		}
		seen[v] = true
	}
	return nil
}

func validateMultiHopTimingScheduleModes(modes []string) error {
	if len(modes) == 0 {
		return fmt.Errorf("multi-hop timing sweep: schedule_modes must not be empty")
	}
	seen := make(map[string]bool, len(modes))
	for _, mode := range modes {
		if mode != MultiHopScheduleNextSlot &&
			mode != MultiHopScheduleNextSuperframe &&
			mode != MultiHopSchedulePipelineLag {
			return fmt.Errorf("multi-hop timing sweep: unsupported schedule_mode %q", mode)
		}
		if seen[mode] {
			return fmt.Errorf("multi-hop timing sweep: schedule_modes contains duplicate %q", mode)
		}
		seen[mode] = true
	}
	return nil
}

func cloneMultiHopTimingInt64s(values []int64) []int64 {
	out := make([]int64, len(values))
	copy(out, values)
	return out
}

func cloneMultiHopTimingInts(values []int) []int {
	out := make([]int, len(values))
	copy(out, values)
	return out
}

func cloneMultiHopTimingStrings(values []string) []string {
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func sortedMultiHopTimingInt64s(values []int64) []int64 {
	out := cloneMultiHopTimingInt64s(values)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func sortedMultiHopTimingInts(values []int) []int {
	out := cloneMultiHopTimingInts(values)
	sort.Ints(out)
	return out
}
