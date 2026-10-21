package timing

import (
	"fmt"
	"sort"

	"github.com/rmsff/ad-hoc-radio/internal/routing"
)

type relayOpportunitySeed struct {
	ownerID  uint16
	parentID uint16
	hop      uint8
}

// BuildOpportunitySchedule converts a planner-selected RelayTree into the
// safe serialized transmitter-opportunity baseline.
func BuildOpportunitySchedule(binding ScheduleBinding, tree routing.RelayTree, profile OpportunityProfile) (OpportunitySchedule, error) {
	if err := validateBindingFields(binding); err != nil {
		return OpportunitySchedule{}, err
	}
	if err := validateOpportunityProfile(profile); err != nil {
		return OpportunitySchedule{}, err
	}
	if err := validateTreeBinding(binding, tree); err != nil {
		return OpportunitySchedule{}, err
	}

	seeds, err := relayOpportunitySeeds(binding.SourceStationID, tree)
	if err != nil {
		return OpportunitySchedule{}, err
	}
	opportunities, err := buildSerializedOpportunities(binding.SourceStationID, seeds, profile)
	if err != nil {
		return OpportunitySchedule{}, err
	}

	schedule := OpportunitySchedule{
		Binding:            copyScheduleBinding(binding),
		MaxRelayDepth:      tree.MaxRelayDepth,
		CoveredMemberIDs:   copyUint16s(tree.Coverage.CoveredMemberIDs),
		UncoveredMemberIDs: copyUint16s(tree.Coverage.UncoveredMemberIDs),
		Opportunities:      opportunities,
	}
	if err := ValidateOpportunitySchedule(schedule); err != nil {
		return OpportunitySchedule{}, err
	}
	cycle, err := cycleDurationMS(schedule.Opportunities)
	if err != nil {
		return OpportunitySchedule{}, err
	}
	if cycle > profile.MaxCycleMS {
		return OpportunitySchedule{}, fmt.Errorf("opportunity schedule: cycle_duration_ms=%d exceeds max_cycle_ms=%d", cycle, profile.MaxCycleMS)
	}
	return schedule, nil
}

func validateOpportunityProfile(profile OpportunityProfile) error {
	if profile.SourceDurationMS == 0 {
		return fmt.Errorf("opportunity profile: source_duration_ms must be > 0")
	}
	if profile.RelayDurationMS == 0 {
		return fmt.Errorf("opportunity profile: relay_duration_ms must be > 0")
	}
	if profile.MaxCycleMS == 0 {
		return fmt.Errorf("opportunity profile: max_cycle_ms must be > 0")
	}
	return nil
}

func validateTreeBinding(binding ScheduleBinding, tree routing.RelayTree) error {
	if _, err := routing.CanonicalRelayTreeJSON(tree); err != nil {
		return fmt.Errorf("opportunity schedule tree: %w", err)
	}
	if tree.SourceStationID != binding.SourceStationID {
		return fmt.Errorf("opportunity schedule tree: source_station_id=%d does not match binding source_id=%d", tree.SourceStationID, binding.SourceStationID)
	}
	if tree.MembershipSnapshotID != binding.MembershipSnapshotID {
		return fmt.Errorf("opportunity schedule tree: membership_snapshot_id=%d does not match binding membership_snapshot_id=%d", tree.MembershipSnapshotID, binding.MembershipSnapshotID)
	}
	if tree.TopologyEpoch != binding.TopologyEpoch {
		return fmt.Errorf("opportunity schedule tree: topology_epoch=%d does not match binding topology_epoch=%d", tree.TopologyEpoch, binding.TopologyEpoch)
	}
	if err := validateMemberPartition(binding.ExpectedMemberIDs, binding.SourceStationID, tree.Coverage.CoveredMemberIDs, tree.Coverage.UncoveredMemberIDs); err != nil {
		return err
	}
	return nil
}

func relayOpportunitySeeds(sourceID uint16, tree routing.RelayTree) ([]relayOpportunitySeed, error) {
	incomingByChild := make(map[uint16]routing.RelayTreeEdge, len(tree.Edges))
	hasOutgoing := make(map[uint16]struct{}, len(tree.Edges))
	for _, edge := range tree.Edges {
		incomingByChild[edge.ChildStationID] = edge
		hasOutgoing[edge.ParentStationID] = struct{}{}
	}

	seeds := make([]relayOpportunitySeed, 0, len(hasOutgoing))
	for ownerID := range hasOutgoing {
		if ownerID == sourceID {
			continue
		}
		incoming, ok := incomingByChild[ownerID]
		if !ok {
			return nil, fmt.Errorf("opportunity schedule tree: relay owner station_id=%d has no incoming edge", ownerID)
		}
		if incoming.Hop == routing.MaxSupportedRelayDepth {
			return nil, fmt.Errorf("opportunity schedule tree: relay owner station_id=%d hop exceeds maximum after incoming hop=%d", ownerID, incoming.Hop)
		}
		seeds = append(seeds, relayOpportunitySeed{
			ownerID:  ownerID,
			parentID: incoming.ParentStationID,
			hop:      incoming.Hop + 1,
		})
	}
	sort.Slice(seeds, func(i, j int) bool {
		if seeds[i].hop != seeds[j].hop {
			return seeds[i].hop < seeds[j].hop
		}
		if seeds[i].parentID != seeds[j].parentID {
			return seeds[i].parentID < seeds[j].parentID
		}
		return seeds[i].ownerID < seeds[j].ownerID
	})
	return seeds, nil
}

func buildSerializedOpportunities(sourceID uint16, seeds []relayOpportunitySeed, profile OpportunityProfile) ([]TxOpportunity, error) {
	opportunities := make([]TxOpportunity, 0, len(seeds)+1)
	opportunities = append(opportunities, TxOpportunity{
		Index:           0,
		OwnerStationID:  sourceID,
		ParentStationID: 0,
		Hop:             0,
		OffsetMS:        0,
		DurationMS:      profile.SourceDurationMS,
		GuardAfterMS:    profile.GuardAfterMS,
	})

	nextOffset, err := opportunityEndMS(opportunities[0])
	if err != nil {
		return nil, err
	}
	for _, seed := range seeds {
		if len(opportunities) > int(^uint16(0)) {
			return nil, fmt.Errorf("opportunity schedule: too many opportunities for uint16 index")
		}
		opp := TxOpportunity{
			Index:           uint16(len(opportunities)),
			OwnerStationID:  seed.ownerID,
			ParentStationID: seed.parentID,
			Hop:             seed.hop,
			OffsetMS:        nextOffset,
			DurationMS:      profile.RelayDurationMS,
			GuardAfterMS:    profile.GuardAfterMS,
		}
		opportunities = append(opportunities, opp)
		nextOffset, err = opportunityEndMS(opp)
		if err != nil {
			return nil, err
		}
	}
	return opportunities, nil
}
