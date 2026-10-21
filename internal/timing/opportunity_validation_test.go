package timing

import (
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/routing"
)

func TestUT022_ValidationRejectsMalformedSchedules(t *testing.T) {
	valid := ut022ValidSchedule(t)
	tests := []struct {
		name string
		edit func(*OpportunitySchedule)
		want string
	}{
		{name: "zero schedule id", edit: func(s *OpportunitySchedule) { s.Binding.ScheduleID = 0 }, want: "schedule_id"},
		{name: "zero call id", edit: func(s *OpportunitySchedule) { s.Binding.CallID = 0 }, want: "call_id"},
		{name: "invalid timing strategy", edit: func(s *OpportunitySchedule) { s.Binding.TimingStrategy = TimingStrategy("other") }, want: "timing_strategy"},
		{name: "source led authority mismatch", edit: func(s *OpportunitySchedule) { s.Binding.TimingAuthorityID = 99 }, want: "must equal source_id"},
		{name: "invalid lease", edit: func(s *OpportunitySchedule) { s.Binding.EffectiveSeq = 20 }, want: "effective_seq"},
		{name: "member partition missing", edit: func(s *OpportunitySchedule) {
			s.Binding.ExpectedMemberIDs = []uint16{2, 3}
			s.CoveredMemberIDs = []uint16{2}
			s.UncoveredMemberIDs = nil
		}, want: "partition"},
		{name: "member partition overlap", edit: func(s *OpportunitySchedule) { s.UncoveredMemberIDs = []uint16{2} }, want: "overlap"},
		{name: "source in expected", edit: func(s *OpportunitySchedule) { s.Binding.ExpectedMemberIDs = []uint16{1, 2} }, want: "source"},
		{name: "duplicate owner", edit: func(s *OpportunitySchedule) {
			s.Opportunities = append(s.Opportunities, TxOpportunity{Index: 2, OwnerStationID: 10, ParentStationID: 10, Hop: 2, OffsetMS: 19, DurationMS: 5, GuardAfterMS: 2})
			s.MaxRelayDepth = 2
		}, want: "duplicate owner"},
		{name: "missing owner", edit: func(s *OpportunitySchedule) {
			s.Opportunities = s.Opportunities[:1]
		}, want: "max_relay_depth"},
		{name: "uncovered member owns opportunity", edit: func(s *OpportunitySchedule) {
			s.CoveredMemberIDs = nil
			s.UncoveredMemberIDs = []uint16{2}
			s.Opportunities[1].OwnerStationID = 2
		}, want: "uncovered member_id"},
		{name: "missing parent", edit: func(s *OpportunitySchedule) { s.Opportunities[1].ParentStationID = 99 }, want: "parent"},
		{name: "invalid relay hop", edit: func(s *OpportunitySchedule) { s.Opportunities[1].Hop = 0 }, want: "hop"},
		{name: "invalid index", edit: func(s *OpportunitySchedule) { s.Opportunities[1].Index = 2 }, want: "index"},
		{name: "invalid offset", edit: func(s *OpportunitySchedule) { s.Opportunities[1].OffsetMS = 11 }, want: "offset_ms"},
		{name: "parent dependency violation", edit: func(s *OpportunitySchedule) { s.Opportunities[1].OffsetMS = 5 }, want: "offset_ms"},
		{name: "zero duration", edit: func(s *OpportunitySchedule) { s.Opportunities[1].DurationMS = 0 }, want: "duration_ms"},
		{name: "source parent is not zero", edit: func(s *OpportunitySchedule) { s.Opportunities[0].ParentStationID = 1 }, want: "parent_station_id"},
		{name: "relay parent zero", edit: func(s *OpportunitySchedule) { s.Opportunities[1].ParentStationID = 0 }, want: "parent_station_id"},
		{name: "relay owners not canonical", edit: func(s *OpportunitySchedule) {
			s.Opportunities = []TxOpportunity{
				{Index: 0, OwnerStationID: 1, ParentStationID: 0, Hop: 0, OffsetMS: 0, DurationMS: 10, GuardAfterMS: 2},
				{Index: 1, OwnerStationID: 20, ParentStationID: 1, Hop: 1, OffsetMS: 12, DurationMS: 5, GuardAfterMS: 2},
				{Index: 2, OwnerStationID: 10, ParentStationID: 1, Hop: 1, OffsetMS: 19, DurationMS: 5, GuardAfterMS: 2},
			}
			s.MaxRelayDepth = 1
		}, want: "canonical"},
		{name: "cycle arithmetic overflow", edit: func(s *OpportunitySchedule) {
			s.Opportunities = []TxOpportunity{
				{Index: 0, OwnerStationID: 1, ParentStationID: 0, Hop: 0, OffsetMS: ^uint32(0), DurationMS: 10, GuardAfterMS: 0},
			}
		}, want: "overflow"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			candidate := cloneUT022Schedule(valid)
			tc.edit(&candidate)
			err := ValidateOpportunitySchedule(candidate)
			if err == nil {
				t.Fatalf("expected validation error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want substring %q", err, tc.want)
			}
		})
	}
}

func TestUT022_BuildRejectsProfileCycleAndTreeMismatch(t *testing.T) {
	binding := ut022Binding([]uint16{2})
	tree := ut022Tree(1, 9, 7, 1, []routing.RelayTreeEdge{
		{ParentStationID: 1, ChildStationID: 10, Hop: 0},
		{ParentStationID: 10, ChildStationID: 2, Hop: 1},
	}, []uint16{2}, nil)

	tests := []struct {
		name    string
		binding ScheduleBinding
		tree    routing.RelayTree
		profile OpportunityProfile
		want    string
	}{
		{name: "zero source duration", binding: binding, tree: tree, profile: OpportunityProfile{RelayDurationMS: 5, GuardAfterMS: 2, MaxCycleMS: 80}, want: "source_duration_ms"},
		{name: "zero relay duration", binding: binding, tree: tree, profile: OpportunityProfile{SourceDurationMS: 10, GuardAfterMS: 2, MaxCycleMS: 80}, want: "relay_duration_ms"},
		{name: "zero max cycle", binding: binding, tree: tree, profile: OpportunityProfile{SourceDurationMS: 10, RelayDurationMS: 5, GuardAfterMS: 2}, want: "max_cycle_ms"},
		{name: "cycle limit failure", binding: binding, tree: tree, profile: OpportunityProfile{SourceDurationMS: 10, RelayDurationMS: 5, GuardAfterMS: 2, MaxCycleMS: 18}, want: "max_cycle_ms"},
		{name: "tree source mismatch", binding: binding, tree: ut022Tree(5, 9, 7, 0, nil, nil, []uint16{2}), profile: ut022Profile(), want: "source"},
		{name: "tree topology mismatch", binding: binding, tree: ut022Tree(1, 9, 8, 0, nil, nil, []uint16{2}), profile: ut022Profile(), want: "topology_epoch"},
		{name: "tree coverage mismatch", binding: binding, tree: ut022Tree(1, 9, 7, 0, nil, nil, nil), profile: ut022Profile(), want: "partition"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := BuildOpportunitySchedule(tc.binding, tc.tree, tc.profile)
			if err == nil {
				t.Fatalf("expected build error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want substring %q", err, tc.want)
			}
		})
	}
}

func TestUT022_InitialVersionAndLookupValidation(t *testing.T) {
	schedule := ut022ValidSchedule(t)
	if err := ValidateInitialOpportunitySchedule(schedule); err != nil {
		t.Fatalf("valid initial schedule rejected: %v", err)
	}

	successorShaped := cloneUT022Schedule(schedule)
	successorShaped.Binding.ScheduleVersion = 2
	if err := ValidateOpportunitySchedule(successorShaped); err != nil {
		t.Fatalf("generic validation rejected non-zero successor version: %v", err)
	}
	if err := ValidateInitialOpportunitySchedule(successorShaped); err == nil || !strings.Contains(err.Error(), "must be 1") {
		t.Fatalf("initial validation error = %v, want version-1 rejection", err)
	}

	invalidLookup := cloneUT022Schedule(schedule)
	invalidLookup.Opportunities[1].OwnerStationID = 0
	defer func() {
		if recover() == nil {
			t.Fatal("lookup on invalid schedule did not panic")
		}
	}()
	invalidLookup.OpportunityAtOffset(0)
}

func ut022ValidSchedule(t *testing.T) OpportunitySchedule {
	t.Helper()
	schedule, err := BuildOpportunitySchedule(
		ut022Binding([]uint16{2}),
		ut022Tree(1, 9, 7, 1, []routing.RelayTreeEdge{
			{ParentStationID: 1, ChildStationID: 10, Hop: 0},
			{ParentStationID: 10, ChildStationID: 2, Hop: 1},
		}, []uint16{2}, nil),
		ut022Profile(),
	)
	if err != nil {
		t.Fatalf("BuildOpportunitySchedule failed: %v", err)
	}
	return schedule
}

func cloneUT022Schedule(schedule OpportunitySchedule) OpportunitySchedule {
	schedule.Binding.ExpectedMemberIDs = append([]uint16(nil), schedule.Binding.ExpectedMemberIDs...)
	schedule.CoveredMemberIDs = append([]uint16(nil), schedule.CoveredMemberIDs...)
	schedule.UncoveredMemberIDs = append([]uint16(nil), schedule.UncoveredMemberIDs...)
	schedule.Opportunities = append([]TxOpportunity(nil), schedule.Opportunities...)
	return schedule
}
