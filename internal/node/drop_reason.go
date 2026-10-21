package node

// DropReason is the runtime drop reason enum per
// frame-contract.md §15. Metrics `dropped_by_reason` keys are
// restricted to this enum by the metrics schema.
type DropReason string

const (
	DropReasonInvalid              DropReason = "invalid"
	DropReasonWrongGroup           DropReason = "wrong_group"
	DropReasonStaleFrame           DropReason = "stale_frame"
	DropReasonAlreadySeen          DropReason = "already_seen"
	DropReasonTTLExpired           DropReason = "ttl_expired"
	DropReasonQueueFull            DropReason = "queue_full"
	DropReasonCallNotActive        DropReason = "call_not_active"
	DropReasonLateFrame            DropReason = "late_frame"
	DropReasonUnsupportedFrameType DropReason = "unsupported_frame_type"
	DropReasonScheduleMismatch     DropReason = "schedule_mismatch"
	DropReasonScheduleExpired      DropReason = "schedule_expired"
	DropReasonNotScheduled         DropReason = "not_scheduled"
	DropReasonReserved             DropReason = "reserved"
)

// AllDropReasons returns every runtime drop reason that may appear in
// metrics dropped_by_reason maps.
func AllDropReasons() []DropReason {
	return []DropReason{
		DropReasonInvalid,
		DropReasonWrongGroup,
		DropReasonStaleFrame,
		DropReasonAlreadySeen,
		DropReasonTTLExpired,
		DropReasonQueueFull,
		DropReasonCallNotActive,
		DropReasonLateFrame,
		DropReasonUnsupportedFrameType,
		DropReasonScheduleMismatch,
		DropReasonScheduleExpired,
		DropReasonNotScheduled,
		DropReasonReserved,
	}
}
