package node

import (
	"fmt"

	"github.com/rmsff/ad-hoc-radio/internal/protocol"
)

func validateVoiceFrameFlags(flags uint16) error {
	direct := flags&protocol.FlagDirect != 0
	relayed := flags&protocol.FlagRelayed != 0
	if direct == relayed {
		return fmt.Errorf("node: voice frame must set exactly one of direct/relayed flags")
	}
	if flags&(protocol.FlagCallStart|protocol.FlagCallEnd) != 0 {
		return fmt.Errorf("node: voice frame must not set call lifecycle flags")
	}
	return nil
}

// ageExceedsThreshold reports whether nowMs-createdAtMs > thresholdMs without
// int64 overflow. Callers ensure createdAtMs <= nowMs; then the unsigned
// difference is the true age (up to 2^64-1), even for createdAtMs near
// MinInt64. A negative threshold keeps the historic meaning: every
// non-negative age exceeds it.
func ageExceedsThreshold(createdAtMs, nowMs, thresholdMs int64) bool {
	if thresholdMs < 0 {
		return true
	}
	return uint64(nowMs)-uint64(createdAtMs) > uint64(thresholdMs)
}

// frameAgeDropReason applies frame-contract.md §11: a future frame is
// invalid, an old frame is stale, and a created_at_ms outside
// 0..protocol.MaxSafeCreatedAtMs is invalid. The range check runs after the
// stale check, so an ancient frame stays stale_frame while a negative
// timestamp within the threshold is invalid.
func frameAgeDropReason(f protocol.Frame, nowMs int64, staleThresholdMs int64) (DropReason, bool) {
	if f.CreatedAtMs > nowMs {
		return DropReasonInvalid, true
	}
	if ageExceedsThreshold(f.CreatedAtMs, nowMs, staleThresholdMs) {
		return DropReasonStaleFrame, true
	}
	if createdAtOutOfRange(f.CreatedAtMs) {
		return DropReasonInvalid, true
	}
	return "", false
}

// createdAtOutOfRange reports a created_at_ms outside
// 0..protocol.MaxSafeCreatedAtMs (frame-contract.md §8.1).
func createdAtOutOfRange(createdAtMs int64) bool {
	return createdAtMs < 0 || createdAtMs > protocol.MaxSafeCreatedAtMs
}

func validateFrameAge(f protocol.Frame, nowMs int64, staleThresholdMs int64) error {
	reason, drop := frameAgeDropReason(f, nowMs, staleThresholdMs)
	if !drop {
		return nil
	}
	if reason == DropReasonStaleFrame {
		return fmt.Errorf("node: frame created_at_ms=%d is stale at now_ms=%d with threshold_ms=%d", f.CreatedAtMs, nowMs, staleThresholdMs)
	}
	if f.CreatedAtMs > nowMs {
		return fmt.Errorf("node: frame created_at_ms=%d is after now_ms=%d", f.CreatedAtMs, nowMs)
	}
	return fmt.Errorf("node: frame created_at_ms=%d is outside 0..%d", f.CreatedAtMs, protocol.MaxSafeCreatedAtMs)
}
