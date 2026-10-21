package scenario

import "math"

func checkedScheduledTime(baseMS int64, deltas ...uint64) (int64, bool) {
	if baseMS < 0 {
		return 0, false
	}
	remaining := uint64(math.MaxInt64 - baseMS)
	var total uint64
	for _, delta := range deltas {
		if delta > remaining-total {
			return 0, false
		}
		total += delta
	}
	return baseMS + int64(total), true
}
