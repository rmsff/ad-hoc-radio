package timing

import "fmt"

// CycleDurationMS returns the checked duration of one schedule cycle.
func (s OpportunitySchedule) CycleDurationMS() uint32 {
	if err := ValidateOpportunitySchedule(s); err != nil {
		panic(fmt.Sprintf("timing: invalid opportunity schedule: %v", err))
	}
	cycle, err := cycleDurationMS(s.Opportunities)
	if err != nil {
		panic(fmt.Sprintf("timing: invalid opportunity schedule cycle: %v", err))
	}
	return cycle
}

// OpportunityAtOffset returns the transmit opportunity active at offsetMS.
// Guard intervals and offset == cycle_duration_ms are deliberately silent.
func (s OpportunitySchedule) OpportunityAtOffset(offsetMS uint32) (TxOpportunity, bool) {
	cycle := s.CycleDurationMS()
	if offsetMS >= cycle {
		return TxOpportunity{}, false
	}
	for _, opp := range s.Opportunities {
		txEnd, ok := checkedAddUint32(opp.OffsetMS, uint32(opp.DurationMS))
		if !ok {
			panic(fmt.Sprintf("timing: opportunity %d tx window overflows uint32", opp.Index))
		}
		if offsetMS >= opp.OffsetMS && offsetMS < txEnd {
			return opp, true
		}
	}
	return TxOpportunity{}, false
}

// IsOwnerWindow reports whether stationID owns a transmit window at offsetMS.
func (s OpportunitySchedule) IsOwnerWindow(offsetMS uint32, stationID uint16) bool {
	if stationID == 0 {
		return false
	}
	opp, ok := s.OpportunityAtOffset(offsetMS)
	return ok && opp.OwnerStationID == stationID
}
