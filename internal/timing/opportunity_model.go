package timing

import "fmt"

// TimingStrategy identifies the accepted simulator timing authority mode.
type TimingStrategy string

const (
	TimingStrategySourceLed   TimingStrategy = "source_led"
	TimingStrategyGroupLeader TimingStrategy = "group_leader"
)

// ScheduleBinding is the frame-contract schedule context for one call schedule.
type ScheduleBinding struct {
	ScheduleID             uint32
	ScheduleVersion        uint16
	NetID                  uint16
	GroupID                uint16
	SourceStationID        uint16
	CallID                 uint32
	ScheduleAuthorityID    uint16
	TopologyEpoch          uint32
	TopologyObservedAtMS   int64
	MembershipSnapshotID   uint32
	MembershipObservedAtMS int64
	MembershipFreshnessMS  uint32
	TimingStrategy         TimingStrategy
	TimingAuthorityID      uint16
	EffectiveSeq           uint32
	ValidThroughSeq        uint32
	ExpectedMemberIDs      []uint16
}

// OpportunityProfile describes the conservative serialized schedule profile.
type OpportunityProfile struct {
	SourceDurationMS uint16
	RelayDurationMS  uint16
	GuardAfterMS     uint16
	MaxCycleMS       uint32
}

// TxOpportunity is one transmitter window within a schedule cycle.
type TxOpportunity struct {
	Index           uint16
	OwnerStationID  uint16
	ParentStationID uint16
	Hop             uint8
	OffsetMS        uint32
	DurationMS      uint16
	GuardAfterMS    uint16
}

// OpportunitySchedule is the canonical direct/relay transmitter schedule.
type OpportunitySchedule struct {
	Binding            ScheduleBinding
	MaxRelayDepth      uint8
	CoveredMemberIDs   []uint16
	UncoveredMemberIDs []uint16
	Opportunities      []TxOpportunity
}

func copyScheduleBinding(binding ScheduleBinding) ScheduleBinding {
	binding.ExpectedMemberIDs = copyUint16s(binding.ExpectedMemberIDs)
	return binding
}

func copyUint16s(ids []uint16) []uint16 {
	if len(ids) == 0 {
		return []uint16{}
	}
	out := make([]uint16, len(ids))
	copy(out, ids)
	return out
}

func nonNilOpportunities(opportunities []TxOpportunity) []TxOpportunity {
	if len(opportunities) == 0 {
		return []TxOpportunity{}
	}
	out := make([]TxOpportunity, len(opportunities))
	copy(out, opportunities)
	return out
}

func checkedAddUint32(parts ...uint32) (uint32, bool) {
	var total uint32
	for _, part := range parts {
		if total > ^uint32(0)-part {
			return 0, false
		}
		total += part
	}
	return total, true
}

func opportunityEndMS(opp TxOpportunity) (uint32, error) {
	end, ok := checkedAddUint32(opp.OffsetMS, uint32(opp.DurationMS), uint32(opp.GuardAfterMS))
	if !ok {
		return 0, fmt.Errorf("opportunity %d timing overflows uint32", opp.Index)
	}
	return end, nil
}
