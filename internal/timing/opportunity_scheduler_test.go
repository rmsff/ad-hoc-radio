package timing

import (
	"reflect"
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/routing"
)

func TestUT022_BuildOpportunityScheduleDepthsAndBranches(t *testing.T) {
	tests := []struct {
		name        string
		binding     ScheduleBinding
		tree        routing.RelayTree
		wantDepth   uint8
		wantOpps    []TxOpportunity
		wantCovered []uint16
	}{
		{
			name:      "direct",
			binding:   ut022Binding([]uint16{2}),
			tree:      ut022Tree(1, 9, 7, 0, []routing.RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2, Hop: 0}}, []uint16{2}, nil),
			wantDepth: 0,
			wantOpps: []TxOpportunity{
				{Index: 0, OwnerStationID: 1, ParentStationID: 0, Hop: 0, OffsetMS: 0, DurationMS: 10, GuardAfterMS: 2},
			},
			wantCovered: []uint16{2},
		},
		{
			name:    "one relay chain",
			binding: ut022Binding([]uint16{2}),
			tree: ut022Tree(1, 9, 7, 1, []routing.RelayTreeEdge{
				{ParentStationID: 1, ChildStationID: 10, Hop: 0},
				{ParentStationID: 10, ChildStationID: 2, Hop: 1},
			}, []uint16{2}, nil),
			wantDepth: 1,
			wantOpps: []TxOpportunity{
				{Index: 0, OwnerStationID: 1, ParentStationID: 0, Hop: 0, OffsetMS: 0, DurationMS: 10, GuardAfterMS: 2},
				{Index: 1, OwnerStationID: 10, ParentStationID: 1, Hop: 1, OffsetMS: 12, DurationMS: 5, GuardAfterMS: 2},
			},
			wantCovered: []uint16{2},
		},
		{
			name:    "two relay chain",
			binding: ut022Binding([]uint16{2}),
			tree: ut022Tree(1, 9, 7, 2, []routing.RelayTreeEdge{
				{ParentStationID: 1, ChildStationID: 10, Hop: 0},
				{ParentStationID: 10, ChildStationID: 11, Hop: 1},
				{ParentStationID: 11, ChildStationID: 2, Hop: 2},
			}, []uint16{2}, nil),
			wantDepth: 2,
			wantOpps: []TxOpportunity{
				{Index: 0, OwnerStationID: 1, ParentStationID: 0, Hop: 0, OffsetMS: 0, DurationMS: 10, GuardAfterMS: 2},
				{Index: 1, OwnerStationID: 10, ParentStationID: 1, Hop: 1, OffsetMS: 12, DurationMS: 5, GuardAfterMS: 2},
				{Index: 2, OwnerStationID: 11, ParentStationID: 10, Hop: 2, OffsetMS: 19, DurationMS: 5, GuardAfterMS: 2},
			},
			wantCovered: []uint16{2},
		},
		{
			name:    "three relay chain",
			binding: ut022Binding([]uint16{2}),
			tree: ut022Tree(1, 9, 7, 3, []routing.RelayTreeEdge{
				{ParentStationID: 1, ChildStationID: 10, Hop: 0},
				{ParentStationID: 10, ChildStationID: 11, Hop: 1},
				{ParentStationID: 11, ChildStationID: 12, Hop: 2},
				{ParentStationID: 12, ChildStationID: 2, Hop: 3},
			}, []uint16{2}, nil),
			wantDepth: 3,
			wantOpps: []TxOpportunity{
				{Index: 0, OwnerStationID: 1, ParentStationID: 0, Hop: 0, OffsetMS: 0, DurationMS: 10, GuardAfterMS: 2},
				{Index: 1, OwnerStationID: 10, ParentStationID: 1, Hop: 1, OffsetMS: 12, DurationMS: 5, GuardAfterMS: 2},
				{Index: 2, OwnerStationID: 11, ParentStationID: 10, Hop: 2, OffsetMS: 19, DurationMS: 5, GuardAfterMS: 2},
				{Index: 3, OwnerStationID: 12, ParentStationID: 11, Hop: 3, OffsetMS: 26, DurationMS: 5, GuardAfterMS: 2},
			},
			wantCovered: []uint16{2},
		},
		{
			name:    "narrow branch one owner",
			binding: ut022Binding([]uint16{2, 3}),
			tree: ut022Tree(1, 9, 7, 1, []routing.RelayTreeEdge{
				{ParentStationID: 1, ChildStationID: 10, Hop: 0},
				{ParentStationID: 10, ChildStationID: 2, Hop: 1},
				{ParentStationID: 10, ChildStationID: 3, Hop: 1},
			}, []uint16{2, 3}, nil),
			wantDepth: 1,
			wantOpps: []TxOpportunity{
				{Index: 0, OwnerStationID: 1, ParentStationID: 0, Hop: 0, OffsetMS: 0, DurationMS: 10, GuardAfterMS: 2},
				{Index: 1, OwnerStationID: 10, ParentStationID: 1, Hop: 1, OffsetMS: 12, DurationMS: 5, GuardAfterMS: 2},
			},
			wantCovered: []uint16{2, 3},
		},
		{
			name:    "wide branch serializes sibling owners",
			binding: ut022Binding([]uint16{2, 3}),
			tree: ut022Tree(1, 9, 7, 1, []routing.RelayTreeEdge{
				{ParentStationID: 1, ChildStationID: 10, Hop: 0},
				{ParentStationID: 1, ChildStationID: 20, Hop: 0},
				{ParentStationID: 10, ChildStationID: 2, Hop: 1},
				{ParentStationID: 20, ChildStationID: 3, Hop: 1},
			}, []uint16{2, 3}, nil),
			wantDepth: 1,
			wantOpps: []TxOpportunity{
				{Index: 0, OwnerStationID: 1, ParentStationID: 0, Hop: 0, OffsetMS: 0, DurationMS: 10, GuardAfterMS: 2},
				{Index: 1, OwnerStationID: 10, ParentStationID: 1, Hop: 1, OffsetMS: 12, DurationMS: 5, GuardAfterMS: 2},
				{Index: 2, OwnerStationID: 20, ParentStationID: 1, Hop: 1, OffsetMS: 19, DurationMS: 5, GuardAfterMS: 2},
			},
			wantCovered: []uint16{2, 3},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			schedule, err := BuildOpportunitySchedule(tc.binding, tc.tree, ut022Profile())
			if err != nil {
				t.Fatalf("BuildOpportunitySchedule failed: %v", err)
			}
			if schedule.MaxRelayDepth != tc.wantDepth {
				t.Fatalf("max depth = %d, want %d", schedule.MaxRelayDepth, tc.wantDepth)
			}
			if !reflect.DeepEqual(schedule.Opportunities, tc.wantOpps) {
				t.Fatalf("opportunities = %+v, want %+v", schedule.Opportunities, tc.wantOpps)
			}
			if !reflect.DeepEqual(schedule.CoveredMemberIDs, tc.wantCovered) {
				t.Fatalf("covered = %v, want %v", schedule.CoveredMemberIDs, tc.wantCovered)
			}
			if err := ValidateOpportunitySchedule(schedule); err != nil {
				t.Fatalf("ValidateOpportunitySchedule failed: %v", err)
			}
		})
	}
}

func TestUT022_BuildOpportunityScheduleConsumesPlannerSelectedTree(t *testing.T) {
	request := routing.PlanRequest{
		SourceStationID: 1,
		Snapshot: routing.ExpectedGroupSnapshot{
			ID:           9,
			ObservedAtMS: 900,
			FreshnessMS:  1_000,
			MemberIDs:    []uint16{2, 3},
		},
		Graph: routing.NeighborGraph{
			Epoch:       7,
			FreshnessMS: 1_000,
			Links: []routing.DirectedLink{
				ut022Link(1, 10),
				ut022Link(1, 20),
				ut022Link(10, 2),
				ut022Link(20, 3),
			},
		},
		Constraints: routing.PlanningConstraints{
			PlanningAtMS:              1_000,
			MaxRelayDepth:             3,
			MaxPlayoutLatencyMS:       250,
			MaxVoiceAirtimePct:        100,
			RequireAllExpectedMembers: false,
			MinRelayBatteryFraction:   0,
			MinLinkQuality:            0,
			MaxLinkLoss:               1,
		},
		CostModel: routing.PlanningCostModel{
			SourceOpportunityMS: 10,
			RelayOpportunityMS:  5,
			GuardAfterMS:        2,
			FramePeriodMS:       80,
		},
		SearchLimits: routing.PlanningSearchLimits{
			MaxPathsPerMember: 20,
			MaxTreeCandidates: 200,
			MaxFeasibleTrees:  200,
		},
		RelayStates: []routing.RelayState{
			ut022RelayState(10),
			ut022RelayState(20),
		},
	}

	result, err := routing.PlanRelayTree(request)
	if err != nil {
		t.Fatalf("PlanRelayTree failed: %v", err)
	}
	schedule, err := BuildOpportunitySchedule(ut022Binding([]uint16{2, 3}), result.Selected.Tree, ut022Profile())
	if err != nil {
		t.Fatalf("BuildOpportunitySchedule from planner tree failed: %v", err)
	}
	if len(schedule.Opportunities) != result.Selected.Cost.TxOpportunityCount {
		t.Fatalf("schedule opportunities = %d, planner tx count = %d", len(schedule.Opportunities), result.Selected.Cost.TxOpportunityCount)
	}
	if got := []uint16{schedule.Opportunities[1].OwnerStationID, schedule.Opportunities[2].OwnerStationID}; !reflect.DeepEqual(got, []uint16{10, 20}) {
		t.Fatalf("planner wide branch owners = %v, want [10 20]", got)
	}
}

func TestUT022_LookupGuardSilenceAndLegacyTwoSlotCompatibility(t *testing.T) {
	binding := ut022Binding([]uint16{2})
	tree := ut022Tree(1, 9, 7, 1, []routing.RelayTreeEdge{
		{ParentStationID: 1, ChildStationID: 10, Hop: 0},
		{ParentStationID: 10, ChildStationID: 2, Hop: 1},
	}, []uint16{2}, nil)
	schedule, err := BuildOpportunitySchedule(binding, tree, ut022Profile())
	if err != nil {
		t.Fatalf("BuildOpportunitySchedule failed: %v", err)
	}
	if got := schedule.CycleDurationMS(); got != 19 {
		t.Fatalf("cycle duration = %d, want 19", got)
	}

	lookup := []struct {
		offset uint32
		owner  uint16
		ok     bool
	}{
		{0, 1, true},
		{9, 1, true},
		{10, 0, false},
		{11, 0, false},
		{12, 10, true},
		{16, 10, true},
		{17, 0, false},
		{18, 0, false},
		{19, 0, false},
	}
	for _, tc := range lookup {
		opp, ok := schedule.OpportunityAtOffset(tc.offset)
		if ok != tc.ok {
			t.Fatalf("OpportunityAtOffset(%d) ok = %v, want %v", tc.offset, ok, tc.ok)
		}
		if ok && opp.OwnerStationID != tc.owner {
			t.Fatalf("OpportunityAtOffset(%d) owner = %d, want %d", tc.offset, opp.OwnerStationID, tc.owner)
		}
		if schedule.IsOwnerWindow(tc.offset, 20) {
			t.Fatalf("station 20 must not own offset %d", tc.offset)
		}
	}
	if schedule.IsOwnerWindow(10, 1) || schedule.IsOwnerWindow(17, 10) {
		t.Fatal("guard intervals must be silent")
	}

	legacy := NewTwoSlotSFR(0, 40)
	compat, err := BuildOpportunitySchedule(
		ut022Binding([]uint16{2}),
		tree,
		OpportunityProfile{SourceDurationMS: 40, RelayDurationMS: 40, GuardAfterMS: 0, MaxCycleMS: 80},
	)
	if err != nil {
		t.Fatalf("compat schedule failed: %v", err)
	}
	if int64(compat.CycleDurationMS()) != legacy.SuperframeDurationMs() {
		t.Fatalf("canonical cycle = %d, legacy superframe = %d", compat.CycleDurationMS(), legacy.SuperframeDurationMs())
	}
	for _, offset := range []uint32{0, 39, 40, 79} {
		opp, ok := compat.OpportunityAtOffset(offset)
		if !ok {
			t.Fatalf("compat offset %d unexpectedly silent", offset)
		}
		wantLegacyOwner := SlotOwnerSource
		if opp.OwnerStationID == 10 {
			wantLegacyOwner = SlotOwnerRelay
		}
		if !legacy.IsOwnerSlot(int64(offset), wantLegacyOwner) {
			t.Fatalf("legacy owner mismatch at offset %d for opportunity %+v", offset, opp)
		}
	}
}

func TestUT022_CanonicalScheduleHashLeaseAndImmutability(t *testing.T) {
	binding := ut022Binding([]uint16{2})
	originalExpected := append([]uint16(nil), binding.ExpectedMemberIDs...)
	tree := ut022Tree(1, 9, 7, 1, []routing.RelayTreeEdge{
		{ParentStationID: 1, ChildStationID: 10, Hop: 0},
		{ParentStationID: 10, ChildStationID: 2, Hop: 1},
	}, []uint16{2}, nil)
	originalEdges := append([]routing.RelayTreeEdge(nil), tree.Edges...)
	originalCovered := append([]uint16(nil), tree.Coverage.CoveredMemberIDs...)

	schedule, err := BuildOpportunitySchedule(binding, tree, ut022Profile())
	if err != nil {
		t.Fatalf("BuildOpportunitySchedule failed: %v", err)
	}
	firstJSON, err := CanonicalOpportunityScheduleJSON(schedule)
	if err != nil {
		t.Fatalf("canonical JSON failed: %v", err)
	}
	secondJSON, err := CanonicalOpportunityScheduleJSON(schedule)
	if err != nil {
		t.Fatalf("canonical JSON second pass failed: %v", err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("canonical JSON changed between runs:\n%s\n%s", firstJSON, secondJSON)
	}
	wantJSON := `{"call_id":3,"covered_member_ids":[2],"effective_seq":0,"expected_member_ids":[2],"group_id":2,"max_relay_depth":1,"membership_freshness_ms":1000,"membership_observed_at_ms":900,"membership_snapshot_id":9,"net_id":1,"opportunities":[{"duration_ms":10,"guard_after_ms":2,"hop":0,"offset_ms":0,"opportunity_index":0,"owner_station_id":1,"parent_station_id":0},{"duration_ms":5,"guard_after_ms":2,"hop":1,"offset_ms":12,"opportunity_index":1,"owner_station_id":10,"parent_station_id":1}],"schedule_authority_id":4,"schedule_id":100,"schedule_version":1,"source_id":1,"timing_authority_id":1,"timing_strategy":"source_led","topology_epoch":7,"topology_observed_at_ms":900,"uncovered_member_ids":[],"valid_through_seq":10}`
	if string(firstJSON) != wantJSON {
		t.Fatalf("canonical JSON =\n%s\nwant\n%s", firstJSON, wantJSON)
	}
	firstHash, err := OpportunityScheduleHash(schedule)
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}
	secondHash, err := OpportunityScheduleHash(schedule)
	if err != nil {
		t.Fatalf("hash second pass failed: %v", err)
	}
	if firstHash != secondHash || len(firstHash) != 64 || strings.ToLower(firstHash) != firstHash {
		t.Fatalf("hash = %q / %q, want stable lowercase sha256", firstHash, secondHash)
	}

	changed := schedule
	changed.Binding.CallID++
	changedJSON, err := CanonicalOpportunityScheduleJSON(changed)
	if err != nil {
		t.Fatalf("changed canonical JSON failed: %v", err)
	}
	changedHash, err := OpportunityScheduleHash(changed)
	if err != nil {
		t.Fatalf("changed hash failed: %v", err)
	}
	if string(changedJSON) == string(firstJSON) || changedHash == firstHash {
		t.Fatal("call_id change must affect canonical bytes and hash")
	}
	if !reflect.DeepEqual(binding.ExpectedMemberIDs, originalExpected) {
		t.Fatalf("binding expected mutated: %v, want %v", binding.ExpectedMemberIDs, originalExpected)
	}
	if !reflect.DeepEqual(tree.Edges, originalEdges) || !reflect.DeepEqual(tree.Coverage.CoveredMemberIDs, originalCovered) {
		t.Fatalf("tree mutated: edges=%v covered=%v", tree.Edges, tree.Coverage.CoveredMemberIDs)
	}

	successor := schedule
	successor.Binding.ScheduleVersion = 2
	successor.Binding.EffectiveSeq = 11
	successor.Binding.ValidThroughSeq = 20
	if err := ValidateScheduleSuccessor(schedule, successor, 10); err != nil {
		t.Fatalf("valid successor rejected: %v", err)
	}
	overlappingPrevious := schedule
	overlappingPrevious.Binding.ValidThroughSeq = 100
	overlappingSuccessor := successor
	overlappingSuccessor.Binding.ValidThroughSeq = 120
	if err := ValidateScheduleSuccessor(overlappingPrevious, overlappingSuccessor, 10); err != nil {
		t.Fatalf("future successor must supersede an overlapping predecessor lease: %v", err)
	}
	nonAscendingPrevious := overlappingPrevious
	nonAscendingPrevious.Binding.EffectiveSeq = 11
	if err := ValidateScheduleSuccessor(nonAscendingPrevious, overlappingSuccessor, 10); err == nil {
		t.Fatal("successor with non-ascending effective_seq unexpectedly valid")
	}
	failures := []struct {
		name string
		edit func(*OpportunitySchedule)
	}{
		{name: "stable key changed", edit: func(s *OpportunitySchedule) { s.Binding.CallID++ }},
		{name: "version gap", edit: func(s *OpportunitySchedule) { s.Binding.ScheduleVersion = 3 }},
		{name: "future effective seq", edit: func(s *OpportunitySchedule) { s.Binding.EffectiveSeq = 10 }},
		{name: "invalid lease", edit: func(s *OpportunitySchedule) { s.Binding.EffectiveSeq = 30; s.Binding.ValidThroughSeq = 20 }},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			candidate := successor
			tc.edit(&candidate)
			if err := ValidateScheduleSuccessor(schedule, candidate, 10); err == nil {
				t.Fatalf("%s successor unexpectedly valid", tc.name)
			}
		})
	}
	wrappedPrevious := schedule
	wrappedPrevious.Binding.ScheduleVersion = ^uint16(0)
	wrappedNext := wrappedPrevious
	wrappedNext.Binding.ScheduleVersion = 0
	wrappedNext.Binding.EffectiveSeq = 11
	wrappedNext.Binding.ValidThroughSeq = 20
	if err := ValidateScheduleSuccessor(wrappedPrevious, wrappedNext, 10); err == nil {
		t.Fatal("successor version wrap unexpectedly valid")
	}
}

func ut022Binding(expected []uint16) ScheduleBinding {
	return ScheduleBinding{
		ScheduleID:             100,
		ScheduleVersion:        1,
		NetID:                  1,
		GroupID:                2,
		SourceStationID:        1,
		CallID:                 3,
		ScheduleAuthorityID:    4,
		TopologyEpoch:          7,
		TopologyObservedAtMS:   900,
		MembershipSnapshotID:   9,
		MembershipObservedAtMS: 900,
		MembershipFreshnessMS:  1_000,
		TimingStrategy:         TimingStrategySourceLed,
		TimingAuthorityID:      1,
		EffectiveSeq:           0,
		ValidThroughSeq:        10,
		ExpectedMemberIDs:      append([]uint16(nil), expected...),
	}
}

func ut022Profile() OpportunityProfile {
	return OpportunityProfile{SourceDurationMS: 10, RelayDurationMS: 5, GuardAfterMS: 2, MaxCycleMS: 80}
}

func ut022Tree(sourceID uint16, snapshotID uint32, topologyEpoch uint32, maxDepth uint8, edges []routing.RelayTreeEdge, covered, uncovered []uint16) routing.RelayTree {
	return routing.RelayTree{
		SourceStationID:      sourceID,
		MembershipSnapshotID: snapshotID,
		TopologyEpoch:        topologyEpoch,
		MaxRelayDepth:        maxDepth,
		Edges:                append([]routing.RelayTreeEdge(nil), edges...),
		Coverage: routing.CoverageSet{
			CoveredMemberIDs:   append([]uint16(nil), covered...),
			UncoveredMemberIDs: append([]uint16(nil), uncovered...),
		},
	}
}

func ut022Link(from, to uint16) routing.DirectedLink {
	return routing.DirectedLink{
		FromStationID: from,
		ToStationID:   to,
		Enabled:       true,
		Quality:       0.9,
		Loss:          0.01,
		DelayMS:       2,
		ObservedAtMS:  900,
	}
}

func ut022RelayState(id uint16) routing.RelayState {
	return routing.RelayState{
		StationID:       id,
		RelayCapable:    true,
		RelayEnabled:    true,
		Busy:            false,
		BatteryFraction: 0.9,
	}
}
