package timing

import "fmt"

// ValidateOpportunitySchedule checks schedule context, membership partition,
// serialized timing, owner uniqueness, and parent-before-child dependencies.
func ValidateOpportunitySchedule(schedule OpportunitySchedule) error {
	if err := validateBindingFields(schedule.Binding); err != nil {
		return err
	}
	if schedule.MaxRelayDepth > 3 {
		return fmt.Errorf("opportunity schedule: max_relay_depth=%d must be in [0, 3]", schedule.MaxRelayDepth)
	}
	if err := validateMemberPartition(schedule.Binding.ExpectedMemberIDs, schedule.Binding.SourceStationID, schedule.CoveredMemberIDs, schedule.UncoveredMemberIDs); err != nil {
		return err
	}
	if err := validateOpportunities(schedule); err != nil {
		return err
	}
	if _, err := cycleDurationMS(schedule.Opportunities); err != nil {
		return err
	}
	return nil
}

// ValidateInitialOpportunitySchedule validates a first schedule version for a
// call. Later versions must be checked with ValidateScheduleSuccessor.
func ValidateInitialOpportunitySchedule(schedule OpportunitySchedule) error {
	if err := ValidateOpportunitySchedule(schedule); err != nil {
		return err
	}
	if schedule.Binding.ScheduleVersion != 1 {
		return fmt.Errorf("initial opportunity schedule: schedule_version=%d must be 1", schedule.Binding.ScheduleVersion)
	}
	return nil
}

func validateBindingFields(binding ScheduleBinding) error {
	if binding.ScheduleID == 0 {
		return fmt.Errorf("schedule binding: schedule_id must be > 0")
	}
	if binding.ScheduleVersion == 0 {
		return fmt.Errorf("schedule binding: schedule_version must be > 0")
	}
	if binding.NetID == 0 {
		return fmt.Errorf("schedule binding: net_id must be > 0")
	}
	if binding.GroupID == 0 {
		return fmt.Errorf("schedule binding: group_id must be > 0")
	}
	if binding.SourceStationID == 0 {
		return fmt.Errorf("schedule binding: source_id must be > 0")
	}
	if binding.CallID == 0 {
		return fmt.Errorf("schedule binding: call_id must be > 0")
	}
	if binding.ScheduleAuthorityID == 0 {
		return fmt.Errorf("schedule binding: schedule_authority_id must be > 0")
	}
	if binding.TopologyEpoch == 0 {
		return fmt.Errorf("schedule binding: topology_epoch must be > 0")
	}
	if binding.TopologyObservedAtMS < 0 {
		return fmt.Errorf("schedule binding: topology_observed_at_ms=%d must be >= 0", binding.TopologyObservedAtMS)
	}
	if binding.MembershipSnapshotID == 0 {
		return fmt.Errorf("schedule binding: membership_snapshot_id must be > 0")
	}
	if binding.MembershipObservedAtMS < 0 {
		return fmt.Errorf("schedule binding: membership_observed_at_ms=%d must be >= 0", binding.MembershipObservedAtMS)
	}
	if binding.MembershipFreshnessMS == 0 {
		return fmt.Errorf("schedule binding: membership_freshness_ms must be > 0")
	}
	if binding.TimingStrategy != TimingStrategySourceLed && binding.TimingStrategy != TimingStrategyGroupLeader {
		return fmt.Errorf("schedule binding: timing_strategy=%q is invalid", binding.TimingStrategy)
	}
	if binding.TimingAuthorityID == 0 {
		return fmt.Errorf("schedule binding: timing_authority_id must be > 0")
	}
	if binding.TimingStrategy == TimingStrategySourceLed && binding.TimingAuthorityID != binding.SourceStationID {
		return fmt.Errorf("schedule binding: source_led timing_authority_id=%d must equal source_id=%d", binding.TimingAuthorityID, binding.SourceStationID)
	}
	if binding.EffectiveSeq > binding.ValidThroughSeq {
		return fmt.Errorf("schedule binding: effective_seq=%d must be <= valid_through_seq=%d", binding.EffectiveSeq, binding.ValidThroughSeq)
	}
	if err := validateStrictlyAscendingIDs("schedule binding: expected_member_ids", binding.ExpectedMemberIDs); err != nil {
		return err
	}
	for _, id := range binding.ExpectedMemberIDs {
		if id == binding.SourceStationID {
			return fmt.Errorf("schedule binding: expected_member_ids contains source_id=%d", binding.SourceStationID)
		}
	}
	return nil
}

func validateMemberPartition(expected []uint16, sourceID uint16, covered, uncovered []uint16) error {
	if err := validateStrictlyAscendingIDs("opportunity schedule: covered_member_ids", covered); err != nil {
		return err
	}
	if err := validateStrictlyAscendingIDs("opportunity schedule: uncovered_member_ids", uncovered); err != nil {
		return err
	}
	expectedSet := make(map[uint16]struct{}, len(expected))
	for _, id := range expected {
		if id == sourceID {
			return fmt.Errorf("opportunity schedule: source_id=%d cannot be in member partition", sourceID)
		}
		expectedSet[id] = struct{}{}
	}
	classified := make(map[uint16]struct{}, len(expected))
	for _, id := range covered {
		if _, ok := expectedSet[id]; !ok {
			return fmt.Errorf("opportunity schedule: covered member_id=%d is not expected", id)
		}
		classified[id] = struct{}{}
	}
	for _, id := range uncovered {
		if _, ok := expectedSet[id]; !ok {
			return fmt.Errorf("opportunity schedule: uncovered member_id=%d is not expected", id)
		}
		if _, overlap := classified[id]; overlap {
			return fmt.Errorf("opportunity schedule: covered/uncovered member sets overlap at member_id=%d", id)
		}
		classified[id] = struct{}{}
	}
	if len(classified) != len(expectedSet) {
		return fmt.Errorf("opportunity schedule: member partition classified=%d expected=%d", len(classified), len(expectedSet))
	}
	for _, id := range expected {
		if _, ok := classified[id]; !ok {
			return fmt.Errorf("opportunity schedule: member partition is missing expected member_id=%d", id)
		}
	}
	return nil
}

func validateStrictlyAscendingIDs(name string, ids []uint16) error {
	for i, id := range ids {
		if id == 0 {
			return fmt.Errorf("%s[%d] must be > 0", name, i)
		}
		if i > 0 && ids[i-1] >= id {
			return fmt.Errorf("%s must be strictly ascending; invalid index %d", name, i)
		}
	}
	return nil
}

func validateOpportunities(schedule OpportunitySchedule) error {
	if len(schedule.Opportunities) == 0 {
		return fmt.Errorf("opportunity schedule: opportunities must not be empty")
	}
	seenOwner := make(map[uint16]TxOpportunity, len(schedule.Opportunities))
	uncovered := make(map[uint16]struct{}, len(schedule.UncoveredMemberIDs))
	for _, memberID := range schedule.UncoveredMemberIDs {
		uncovered[memberID] = struct{}{}
	}
	maxHop := uint8(0)
	expectedOffset := uint32(0)
	var previousRelay TxOpportunity
	for i, opp := range schedule.Opportunities {
		if opp.OwnerStationID == 0 {
			return fmt.Errorf("opportunity schedule: opportunity %d owner_station_id must be > 0", i)
		}
		if _, isUncovered := uncovered[opp.OwnerStationID]; isUncovered {
			return fmt.Errorf("opportunity schedule: uncovered member_id=%d cannot own a transmit opportunity", opp.OwnerStationID)
		}
		if opp.DurationMS == 0 {
			return fmt.Errorf("opportunity schedule: opportunity %d duration_ms must be > 0", i)
		}
		end, err := opportunityEndMS(opp)
		if err != nil {
			return err
		}
		if int(opp.Index) != i {
			return fmt.Errorf("opportunity schedule: opportunity %d index=%d must be contiguous", i, opp.Index)
		}
		if _, duplicate := seenOwner[opp.OwnerStationID]; duplicate {
			return fmt.Errorf("opportunity schedule: duplicate owner_station_id=%d", opp.OwnerStationID)
		}
		if i == 0 {
			if opp.OwnerStationID != schedule.Binding.SourceStationID {
				return fmt.Errorf("opportunity schedule: opportunity 0 owner_station_id=%d must equal source_id=%d", opp.OwnerStationID, schedule.Binding.SourceStationID)
			}
			if opp.ParentStationID != 0 {
				return fmt.Errorf("opportunity schedule: opportunity 0 parent_station_id must be 0")
			}
			if opp.Hop != 0 {
				return fmt.Errorf("opportunity schedule: opportunity 0 hop=%d must be 0", opp.Hop)
			}
			if opp.OffsetMS != 0 {
				return fmt.Errorf("opportunity schedule: opportunity 0 offset_ms=%d must be 0", opp.OffsetMS)
			}
		} else {
			if opp.OwnerStationID == schedule.Binding.SourceStationID {
				return fmt.Errorf("opportunity schedule: source_id=%d appears after opportunity 0", schedule.Binding.SourceStationID)
			}
			if opp.ParentStationID == 0 {
				return fmt.Errorf("opportunity schedule: opportunity %d parent_station_id must be > 0", i)
			}
			parent, ok := seenOwner[opp.ParentStationID]
			if !ok {
				return fmt.Errorf("opportunity schedule: opportunity %d parent_station_id=%d has no earlier opportunity", i, opp.ParentStationID)
			}
			if opp.Hop != parent.Hop+1 {
				return fmt.Errorf("opportunity schedule: opportunity %d hop=%d, want parent hop + 1 = %d", i, opp.Hop, parent.Hop+1)
			}
			parentEnd, err := opportunityEndMS(parent)
			if err != nil {
				return err
			}
			if opp.OffsetMS < parentEnd {
				return fmt.Errorf("opportunity schedule: opportunity %d offset_ms=%d starts before parent end=%d", i, opp.OffsetMS, parentEnd)
			}
			if i > 1 && compareRelayOpportunityOrder(previousRelay, opp) >= 0 {
				return fmt.Errorf("opportunity schedule: opportunity %d is not in canonical relay order", i)
			}
			previousRelay = opp
		}
		if opp.OffsetMS != expectedOffset {
			return fmt.Errorf("opportunity schedule: opportunity %d offset_ms=%d, want serialized offset_ms=%d", i, opp.OffsetMS, expectedOffset)
		}
		if opp.Hop > 3 {
			return fmt.Errorf("opportunity schedule: opportunity %d hop=%d exceeds maximum 3", i, opp.Hop)
		}
		if opp.Hop > maxHop {
			maxHop = opp.Hop
		}
		seenOwner[opp.OwnerStationID] = opp
		expectedOffset = end
	}
	if maxHop != schedule.MaxRelayDepth {
		return fmt.Errorf("opportunity schedule: max_relay_depth=%d does not match max opportunity hop=%d", schedule.MaxRelayDepth, maxHop)
	}
	return nil
}

func compareRelayOpportunityOrder(a, b TxOpportunity) int {
	if a.Hop < b.Hop {
		return -1
	}
	if a.Hop > b.Hop {
		return 1
	}
	if a.ParentStationID < b.ParentStationID {
		return -1
	}
	if a.ParentStationID > b.ParentStationID {
		return 1
	}
	if a.OwnerStationID < b.OwnerStationID {
		return -1
	}
	if a.OwnerStationID > b.OwnerStationID {
		return 1
	}
	return 0
}

func cycleDurationMS(opportunities []TxOpportunity) (uint32, error) {
	if len(opportunities) == 0 {
		return 0, fmt.Errorf("opportunity schedule: opportunities must not be empty")
	}
	return opportunityEndMS(opportunities[len(opportunities)-1])
}

// ValidateScheduleSuccessor checks stable-key, version, and sequence-lease
// semantics for a schedule update.
func ValidateScheduleSuccessor(previous, next OpportunitySchedule, highestCreatedSeq uint32) error {
	if err := ValidateOpportunitySchedule(previous); err != nil {
		return fmt.Errorf("previous schedule invalid: %w", err)
	}
	if err := ValidateOpportunitySchedule(next); err != nil {
		return fmt.Errorf("next schedule invalid: %w", err)
	}
	if previous.Binding.NetID != next.Binding.NetID ||
		previous.Binding.GroupID != next.Binding.GroupID ||
		previous.Binding.SourceStationID != next.Binding.SourceStationID ||
		previous.Binding.CallID != next.Binding.CallID ||
		previous.Binding.ScheduleID != next.Binding.ScheduleID {
		return fmt.Errorf("schedule successor: stable schedule key changed")
	}
	if previous.Binding.ScheduleVersion == ^uint16(0) {
		return fmt.Errorf("schedule successor: schedule_version cannot wrap")
	}
	wantVersion := previous.Binding.ScheduleVersion + 1
	if next.Binding.ScheduleVersion != wantVersion {
		return fmt.Errorf("schedule successor: schedule_version=%d, want %d", next.Binding.ScheduleVersion, wantVersion)
	}
	if next.Binding.EffectiveSeq <= previous.Binding.EffectiveSeq {
		return fmt.Errorf(
			"schedule successor: effective_seq=%d must be > previous effective_seq=%d",
			next.Binding.EffectiveSeq,
			previous.Binding.EffectiveSeq,
		)
	}
	if next.Binding.EffectiveSeq <= highestCreatedSeq {
		return fmt.Errorf("schedule successor: effective_seq=%d must be > highest_created_seq=%d", next.Binding.EffectiveSeq, highestCreatedSeq)
	}
	return nil
}
