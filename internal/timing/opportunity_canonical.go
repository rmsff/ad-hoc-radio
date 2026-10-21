package timing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

type canonicalOpportunitySchedule struct {
	CallID                 uint32                   `json:"call_id"`
	CoveredMemberIDs       []uint16                 `json:"covered_member_ids"`
	EffectiveSeq           uint32                   `json:"effective_seq"`
	ExpectedMemberIDs      []uint16                 `json:"expected_member_ids"`
	GroupID                uint16                   `json:"group_id"`
	MaxRelayDepth          uint8                    `json:"max_relay_depth"`
	MembershipFreshnessMS  uint32                   `json:"membership_freshness_ms"`
	MembershipObservedAtMS int64                    `json:"membership_observed_at_ms"`
	MembershipSnapshotID   uint32                   `json:"membership_snapshot_id"`
	NetID                  uint16                   `json:"net_id"`
	Opportunities          []canonicalTxOpportunity `json:"opportunities"`
	ScheduleAuthorityID    uint16                   `json:"schedule_authority_id"`
	ScheduleID             uint32                   `json:"schedule_id"`
	ScheduleVersion        uint16                   `json:"schedule_version"`
	SourceID               uint16                   `json:"source_id"`
	TimingAuthorityID      uint16                   `json:"timing_authority_id"`
	TimingStrategy         TimingStrategy           `json:"timing_strategy"`
	TopologyEpoch          uint32                   `json:"topology_epoch"`
	TopologyObservedAtMS   int64                    `json:"topology_observed_at_ms"`
	UncoveredMemberIDs     []uint16                 `json:"uncovered_member_ids"`
	ValidThroughSeq        uint32                   `json:"valid_through_seq"`
}

type canonicalTxOpportunity struct {
	DurationMS       uint16 `json:"duration_ms"`
	GuardAfterMS     uint16 `json:"guard_after_ms"`
	Hop              uint8  `json:"hop"`
	OffsetMS         uint32 `json:"offset_ms"`
	OpportunityIndex uint16 `json:"opportunity_index"`
	OwnerStationID   uint16 `json:"owner_station_id"`
	ParentStationID  uint16 `json:"parent_station_id"`
}

// CanonicalOpportunityScheduleJSON returns canonical-json-v1 bytes for the
// schedule fields, excluding any digest field.
func CanonicalOpportunityScheduleJSON(schedule OpportunitySchedule) ([]byte, error) {
	if err := ValidateOpportunitySchedule(schedule); err != nil {
		return nil, err
	}
	opportunities := make([]canonicalTxOpportunity, len(schedule.Opportunities))
	for i, opp := range schedule.Opportunities {
		opportunities[i] = canonicalTxOpportunity{
			DurationMS:       opp.DurationMS,
			GuardAfterMS:     opp.GuardAfterMS,
			Hop:              opp.Hop,
			OffsetMS:         opp.OffsetMS,
			OpportunityIndex: opp.Index,
			OwnerStationID:   opp.OwnerStationID,
			ParentStationID:  opp.ParentStationID,
		}
	}
	canonical := canonicalOpportunitySchedule{
		CallID:                 schedule.Binding.CallID,
		CoveredMemberIDs:       copyUint16s(schedule.CoveredMemberIDs),
		EffectiveSeq:           schedule.Binding.EffectiveSeq,
		ExpectedMemberIDs:      copyUint16s(schedule.Binding.ExpectedMemberIDs),
		GroupID:                schedule.Binding.GroupID,
		MaxRelayDepth:          schedule.MaxRelayDepth,
		MembershipFreshnessMS:  schedule.Binding.MembershipFreshnessMS,
		MembershipObservedAtMS: schedule.Binding.MembershipObservedAtMS,
		MembershipSnapshotID:   schedule.Binding.MembershipSnapshotID,
		NetID:                  schedule.Binding.NetID,
		Opportunities:          opportunities,
		ScheduleAuthorityID:    schedule.Binding.ScheduleAuthorityID,
		ScheduleID:             schedule.Binding.ScheduleID,
		ScheduleVersion:        schedule.Binding.ScheduleVersion,
		SourceID:               schedule.Binding.SourceStationID,
		TimingAuthorityID:      schedule.Binding.TimingAuthorityID,
		TimingStrategy:         schedule.Binding.TimingStrategy,
		TopologyEpoch:          schedule.Binding.TopologyEpoch,
		TopologyObservedAtMS:   schedule.Binding.TopologyObservedAtMS,
		UncoveredMemberIDs:     copyUint16s(schedule.UncoveredMemberIDs),
		ValidThroughSeq:        schedule.Binding.ValidThroughSeq,
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return nil, fmt.Errorf("opportunity schedule: encode canonical JSON: %w", err)
	}
	return encoded, nil
}

// OpportunityScheduleHash returns lowercase SHA-256 over canonical schedule JSON.
func OpportunityScheduleHash(schedule OpportunitySchedule) (string, error) {
	encoded, err := CanonicalOpportunityScheduleJSON(schedule)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
