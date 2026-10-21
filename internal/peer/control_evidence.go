package peer

// CommitReceipt is observer evidence, not information available to the source
// over RF. In particular, a local source commit does not prove remote commit.
type CommitReceipt struct {
	StationID       uint16 `json:"station_id"`
	ScheduleVersion uint16 `json:"schedule_version"`
	Digest          string `json:"digest"`
	ReceivedAtMS    int64  `json:"received_at_ms"`
}

func (c *Controller) CommitReceipts() []CommitReceipt {
	return append([]CommitReceipt(nil), c.receipts...)
}
