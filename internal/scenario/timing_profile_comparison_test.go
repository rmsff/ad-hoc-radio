package scenario

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

func TestUT024_TimingAuthorityValidationElectionAndLoss(t *testing.T) {
	fixture := timingComparisonFixture("direct", scheduledMetricsScenarioConfig(
		ScheduledRelayTreeDirectScenarioID,
		[]uint16{2},
		scheduledMetricsLinks([2]uint16{1, 2}),
		nil,
	))
	fixture.TimingOverlay.Peers[0].LastSeenMS = 0
	fixture.TimingOverlay.Peers[0].FreshnessMS = 1_000
	fixture.TimingOverlay.Peers[0].BatteryPct = fixture.TimingOverlay.MinTimingBatteryPct
	report, err := RunTimingProfileComparison([]TimingProfileComparisonFixture{fixture})
	if err != nil {
		t.Fatalf("comparison failed: %v", err)
	}
	groupNative := timingComparisonRow(t, report, "direct/group_leader/project_native_variable")
	if groupNative.TimingAuthorityID != 1 || groupNative.TimingEpochID != 1 {
		t.Fatalf("initial leader/epoch = %d/%d, want 1/1", groupNative.TimingAuthorityID, groupNative.TimingEpochID)
	}
	fixture = timingComparisonFixture("future-join", fixture.BaseConfig)
	joinAfterStart := int64(1_050)
	fixture.TimingOverlay.Peers[0].JoinAtMS = &joinAfterStart
	report, err = RunTimingProfileComparison([]TimingProfileComparisonFixture{fixture})
	if err != nil {
		t.Fatalf("future-join comparison failed: %v", err)
	}
	groupNative = timingComparisonRow(t, report, "future-join/group_leader/project_native_variable")
	if groupNative.TimingAuthorityID != 2 {
		t.Fatalf("future-join leader = %d, want 2", groupNative.TimingAuthorityID)
	}

	fixture = timingComparisonFixture("direct", fixture.BaseConfig)
	fixture.TimingOverlay.ConfiguredLeaderID = 2
	if _, err := RunTimingProfileComparison([]TimingProfileComparisonFixture{fixture}); err == nil || !strings.Contains(err.Error(), "configured_leader_id") {
		t.Fatalf("configured leader mismatch error = %v", err)
	}

	fixture = timingComparisonFixture("direct", scheduledMetricsScenarioConfig(
		ScheduledRelayTreeDirectScenarioID,
		[]uint16{2},
		scheduledMetricsLinks([2]uint16{1, 2}),
		nil,
	))
	fixture.TimingOverlay.LeaderLoss = &TimingAuthorityLoss{
		AtMS: 1_020, LostStationID: 1, LeaderReacquireMS: 60,
	}
	report, err = RunTimingProfileComparison([]TimingProfileComparisonFixture{fixture})
	if err != nil {
		t.Fatalf("loss comparison failed: %v", err)
	}
	groupNative = timingComparisonRow(t, report, "direct/group_leader/project_native_variable")
	if !groupNative.ProfileFit || groupNative.TimingAuthorityID != 2 || groupNative.TimingEpochID != 2 ||
		groupNative.LeaderLossCount != 1 || groupNative.LeaderReacquireMS != 60 || groupNative.RouteReplanCount != 1 {
		t.Fatalf("loss evidence = %+v", groupNative)
	}
	if groupNative.LossEventPriority != 12 || groupNative.ReplanEventPriority != 15 {
		t.Fatalf("event priorities = %d/%d, want 12/15", groupNative.LossEventPriority, groupNative.ReplanEventPriority)
	}

	fixture.TimingOverlay.Peers[1].TimingCapable = false
	report, err = RunTimingProfileComparison([]TimingProfileComparisonFixture{fixture})
	if err != nil {
		t.Fatalf("no-replacement comparison failed: %v", err)
	}
	groupNative = timingComparisonRow(t, report, "direct/group_leader/project_native_variable")
	if groupNative.ProfileFit || groupNative.FitReason != TimingFitNoEligibleReplacement || groupNative.TimingAuthorityID != 0 {
		t.Fatalf("no-replacement evidence = %+v", groupNative)
	}
}

func TestUT025_TimingProfileMappingAndFitBoundaries(t *testing.T) {
	cfg := scheduledMetricsScenarioConfig(
		ScheduledRelayTreeOneRelayScenarioID,
		[]uint16{3},
		scheduledMetricsLinks([2]uint16{1, 2}, [2]uint16{2, 3}),
		[]routing.RelayState{scheduledMetricsRelayState(2)},
	)
	mapped := timingProfileScenarioConfig(cfg, TimingProfileDMRInspired30MS)
	if mapped.InitialPlan.CostModel.SourceOpportunityMS != 30 || mapped.InitialPlan.CostModel.RelayOpportunityMS != 30 ||
		mapped.InitialPlan.CostModel.GuardAfterMS != 0 || mapped.InitialPlan.CostModel.FramePeriodMS != 60 ||
		mapped.OpportunityProfile != (timing.OpportunityProfile{SourceDurationMS: 30, RelayDurationMS: 30, MaxCycleMS: 60}) {
		t.Fatalf("DMR-inspired mapping = %+v / %+v", mapped.InitialPlan.CostModel, mapped.OpportunityProfile)
	}
	if cfg.InitialPlan.CostModel.SourceOpportunityMS != 10 || cfg.OpportunityProfile.SourceDurationMS != 10 {
		t.Fatal("profile mapping mutated caller config")
	}

	report, err := RunTimingProfileComparison([]TimingProfileComparisonFixture{timingComparisonFixture("one", cfg)})
	if err != nil {
		t.Fatalf("one-relay comparison failed: %v", err)
	}
	row := timingComparisonRow(t, report, "one/source_led/dmr_inspired_30ms")
	if !row.ProfileFit || row.ScheduleCycleDurationMS != 60 || !row.OpportunityIncludesLowerLayerOverhead || row.ExtraTurnaroundAssumptionMS != 0 {
		t.Fatalf("exact 60ms fit = %+v", row)
	}

	twoRelay := scheduledMetricsScenarioConfig(
		ScheduledRelayTreeTwoRelayScenarioID,
		[]uint16{4},
		scheduledMetricsLinks([2]uint16{1, 2}, [2]uint16{2, 3}, [2]uint16{3, 4}),
		[]routing.RelayState{scheduledMetricsRelayState(2), scheduledMetricsRelayState(3)},
	)
	report, err = RunTimingProfileComparison([]TimingProfileComparisonFixture{timingComparisonFixture("two", twoRelay)})
	if err != nil {
		t.Fatalf("two-relay comparison failed: %v", err)
	}
	row = timingComparisonRow(t, report, "two/source_led/dmr_inspired_30ms")
	if row.ProfileFit || row.FitReason != TimingFitPlannerNoFeasibleTree || row.RuntimeOutcome != TimingRuntimeNotRun {
		t.Fatalf(">60ms no-fit = %+v", row)
	}
	if _, err := checkedTimingDriftMS(math.MaxInt64, 2); err == nil || !strings.Contains(err.Error(), "overflows") {
		t.Fatalf("checked drift overflow error = %v", err)
	}
	if _, err := checkedTimingCollisionAdd(maxIntValue()-1, 2); err == nil || !strings.Contains(err.Error(), "overflows") {
		t.Fatalf("checked collision overflow error = %v", err)
	}
	negative, err := checkedTimingDriftMS(80, -1_000_000)
	if err != nil || negative != 80 {
		t.Fatalf("negative max-safe drift = %d/%v, want 80/nil", negative, err)
	}
}

func TestIT065_TimingComparisonEmitsFourOrderedRows(t *testing.T) {
	fixture := timingComparisonFixture("direct", scheduledMetricsScenarioConfig(
		ScheduledRelayTreeDirectScenarioID,
		[]uint16{2},
		scheduledMetricsLinks([2]uint16{1, 2}),
		nil,
	))
	report, err := RunTimingProfileComparison([]TimingProfileComparisonFixture{fixture})
	if err != nil {
		t.Fatalf("comparison failed: %v", err)
	}
	want := []string{
		"direct/source_led/project_native_variable",
		"direct/source_led/dmr_inspired_30ms",
		"direct/group_leader/project_native_variable",
		"direct/group_leader/dmr_inspired_30ms",
	}
	if len(report.Rows) != len(want) {
		t.Fatalf("row count = %d, want %d", len(report.Rows), len(want))
	}
	for i := range want {
		if report.Rows[i].RowID != want[i] {
			t.Fatalf("row[%d] = %q, want %q", i, report.Rows[i].RowID, want[i])
		}
	}
}

func TestIT066_TimingLeaderLossBoundaryAndRuntimeHashFence(t *testing.T) {
	base := scheduledMetricsScenarioConfig(
		ScheduledRelayTreeDirectScenarioID,
		[]uint16{2},
		scheduledMetricsLinks([2]uint16{1, 2}),
		nil,
	)
	fixture := timingComparisonFixture("equal", base)
	fixture.TimingOverlay.LeaderLoss = &TimingAuthorityLoss{AtMS: 1_020, LostStationID: 1, LeaderReacquireMS: 60}
	report, err := RunTimingProfileComparison([]TimingProfileComparisonFixture{fixture})
	if err != nil {
		t.Fatalf("equal-boundary comparison failed: %v", err)
	}
	equal := timingComparisonRow(t, report, "equal/group_leader/project_native_variable")
	if !equal.ProfileFit || equal.EventHash == "" {
		t.Fatalf("equal-boundary row = %+v", equal)
	}

	fixture.FixtureID = "over"
	fixture.TimingOverlay.LeaderLoss.LeaderReacquireMS = 61
	report, err = RunTimingProfileComparison([]TimingProfileComparisonFixture{fixture})
	if err != nil {
		t.Fatalf("over-boundary comparison failed: %v", err)
	}
	over := timingComparisonRow(t, report, "over/group_leader/project_native_variable")
	if over.ProfileFit || over.FitReason != TimingFitLeaderReacquireExceedsNextTick || over.EventHash != "" {
		t.Fatalf("over-boundary row = %+v", over)
	}

	fixture.FixtureID = "outside"
	fixture.TimingOverlay.LeaderLoss.AtMS = base.StartMS + int64(base.FrameCount)*base.InitialPlan.CostModel.FramePeriodMS
	fixture.TimingOverlay.LeaderLoss.LeaderReacquireMS = 0
	fixture.TimingOverlay.Peers[1].TimingCapable = false
	report, err = RunTimingProfileComparison([]TimingProfileComparisonFixture{fixture})
	if err != nil {
		t.Fatalf("outside-call comparison failed: %v", err)
	}
	outside := timingComparisonRow(t, report, "outside/group_leader/project_native_variable")
	if outside.ProfileFit || outside.FitReason != TimingFitLeaderLossOutsideCall || outside.RuntimeOutcome != TimingRuntimeNotRun {
		t.Fatalf("outside-call row = %+v", outside)
	}
}

func TestIT067_TimingLateEntryDriftAndResyncEvidence(t *testing.T) {
	fixture := timingComparisonFixture("timing", scheduledMetricsScenarioConfig(
		ScheduledRelayTreeDirectScenarioID,
		[]uint16{2},
		scheduledMetricsLinks([2]uint16{1, 2}),
		nil,
	))
	join := int64(1_021)
	fixture.TimingOverlay.Peers[1].JoinAtMS = &join
	fixture.TimingOverlay.Peers[1].DriftPPM = 1_000_000
	fixture.TimingOverlay.SyncToleranceMS = 10
	report, err := RunTimingProfileComparison([]TimingProfileComparisonFixture{fixture})
	if err != nil {
		t.Fatalf("comparison failed: %v", err)
	}
	row := timingComparisonRow(t, report, "timing/source_led/project_native_variable")
	if row.LateEntryAcquisitionMS != 59 || row.DriftCollisionCount != 2 || row.ResyncCount != row.DriftCollisionCount {
		t.Fatalf("timing evidence = %+v", row)
	}

	join = 1_079
	fixture.TimingOverlay.Peers[1].JoinAtMS = &join
	report, err = RunTimingProfileComparison([]TimingProfileComparisonFixture{fixture})
	if err != nil {
		t.Fatalf("partial-drift comparison failed: %v", err)
	}
	row = timingComparisonRow(t, report, "timing/source_led/project_native_variable")
	if row.LateEntryAcquisitionMS != 1 || row.DriftCollisionCount != 1 {
		t.Fatalf("partial first-sync evidence = %+v, want acquisition/collisions 1/1", row)
	}
}

func TestIT068_TimingComparisonClassifiesDepthProfiles(t *testing.T) {
	direct := timingComparisonFixture("direct", scheduledMetricsScenarioConfig(
		ScheduledRelayTreeDirectScenarioID, []uint16{2}, scheduledMetricsLinks([2]uint16{1, 2}), nil,
	))
	two := timingComparisonFixture("two", scheduledMetricsScenarioConfig(
		ScheduledRelayTreeTwoRelayScenarioID,
		[]uint16{4},
		scheduledMetricsLinks([2]uint16{1, 2}, [2]uint16{2, 3}, [2]uint16{3, 4}),
		[]routing.RelayState{scheduledMetricsRelayState(2), scheduledMetricsRelayState(3)},
	))
	report, err := RunTimingProfileComparison([]TimingProfileComparisonFixture{two, direct})
	if err != nil {
		t.Fatalf("comparison failed: %v", err)
	}
	if len(report.Rows) != 8 {
		t.Fatalf("row count = %d, want 8", len(report.Rows))
	}
	if !timingComparisonRow(t, report, "direct/group_leader/dmr_inspired_30ms").ProfileFit ||
		timingComparisonRow(t, report, "two/group_leader/dmr_inspired_30ms").ProfileFit {
		t.Fatalf("unexpected profile fit rows: %+v", report.Rows)
	}
}

func TestIT069_TimingComparisonBytesArePermutationInvariant(t *testing.T) {
	a := timingComparisonFixture("a", scheduledMetricsScenarioConfig(
		ScheduledRelayTreeDirectScenarioID, []uint16{2}, scheduledMetricsLinks([2]uint16{1, 2}), nil,
	))
	b := timingComparisonFixture("b", scheduledMetricsScenarioConfig(
		ScheduledRelayTreeDirectScenarioID, []uint16{3}, scheduledMetricsLinks([2]uint16{1, 3}), nil,
	))
	first, err := RunTimingProfileComparison([]TimingProfileComparisonFixture{b, a})
	if err != nil {
		t.Fatalf("first comparison failed: %v", err)
	}
	second, err := RunTimingProfileComparison([]TimingProfileComparisonFixture{a, b})
	if err != nil {
		t.Fatalf("second comparison failed: %v", err)
	}
	firstJSON, err := MarshalTimingProfileComparisonReport(first)
	if err != nil {
		t.Fatalf("marshal first failed: %v", err)
	}
	secondJSON, err := MarshalTimingProfileComparisonReport(second)
	if err != nil {
		t.Fatalf("marshal second failed: %v", err)
	}
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatalf("permuted reports differ:\n%s\n%s", firstJSON, secondJSON)
	}
}

func TestIT070_TimingComparisonNoClaimAndInputGuards(t *testing.T) {
	fixture := timingComparisonFixture("guard", scheduledMetricsScenarioConfig(
		ScheduledRelayTreeDirectScenarioID, []uint16{2}, scheduledMetricsLinks([2]uint16{1, 2}), nil,
	))
	report, err := RunTimingProfileComparison([]TimingProfileComparisonFixture{fixture})
	if err != nil {
		t.Fatalf("comparison failed: %v", err)
	}
	if report.DMRCompatible || report.CompatibilityClaim || report.AMBEDependency || report.RFSynchronizationClaim {
		t.Fatalf("report claim flags = %+v", report)
	}
	if err := ValidateScenarioEvidence(report.Evidence); err != nil {
		t.Fatalf("canonical scenario evidence invalid: %v", err)
	}
	if report.Evidence.HardwareClaimAllowed || len(report.Evidence.Determinism.EventHash) != 64 {
		t.Fatalf("canonical evidence claim/hash = %+v", report.Evidence)
	}
	for _, row := range report.Rows {
		if row.DMRCompatible || row.CompatibilityClaim || row.AMBEDependency || row.RFSynchronizationClaim {
			t.Fatalf("row claim flags = %+v", row)
		}
	}

	fixture.TimingOverlay.Peers[0].DriftPPM = 1_000_001
	if _, err := RunTimingProfileComparison([]TimingProfileComparisonFixture{fixture}); err == nil || !strings.Contains(err.Error(), "drift_ppm") {
		t.Fatalf("drift range error = %v", err)
	}
	fixture = timingComparisonFixture("guard", fixture.BaseConfig)
	fixture.TimingOverlay.Peers = append(fixture.TimingOverlay.Peers, fixture.TimingOverlay.Peers[0])
	if _, err := RunTimingProfileComparison([]TimingProfileComparisonFixture{fixture}); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate peer error = %v", err)
	}
	fixture = timingComparisonFixture("", fixture.BaseConfig)
	if _, err := RunTimingProfileComparison([]TimingProfileComparisonFixture{fixture}); err == nil || !strings.Contains(err.Error(), "fixture_id") {
		t.Fatalf("blank fixture id error = %v", err)
	}
	fixture = timingComparisonFixture("duplicate", fixture.BaseConfig)
	if _, err := RunTimingProfileComparison([]TimingProfileComparisonFixture{fixture, fixture}); err == nil || !strings.Contains(err.Error(), "duplicate fixture_id") {
		t.Fatalf("duplicate fixture id error = %v", err)
	}
	fixture = timingComparisonFixture("loss-with-replan", fixture.BaseConfig)
	fixture.BaseConfig.Replans = []ScheduledRelayTreeReplanConfig{{
		AtMS: 1_010, Reason: ScheduledReplanTopologyEpochChanged, Plan: cloneTimingRelayTreePlan(fixture.BaseConfig.InitialPlan),
	}}
	fixture.TimingOverlay.LeaderLoss = &TimingAuthorityLoss{AtMS: 1_020, LostStationID: 1, LeaderReacquireMS: 60}
	if _, err := RunTimingProfileComparison([]TimingProfileComparisonFixture{fixture}); err == nil || !strings.Contains(err.Error(), "same effective source sequence") {
		t.Fatalf("loss/replan composition error = %v", err)
	}
}

func timingComparisonFixture(id string, cfg ScheduledRelayTreeScenarioConfig) TimingProfileComparisonFixture {
	return TimingProfileComparisonFixture{
		FixtureID:  id,
		BaseConfig: cfg,
		TimingOverlay: TimingProfileComparisonOverlay{
			MinTimingBatteryPct: 20,
			SyncToleranceMS:     1,
			Peers: []TimingAuthorityPeer{
				{StationID: cfg.SourceStationID, TimingCapable: true, LastSeenMS: cfg.StartMS, FreshnessMS: 1_000, BatteryPct: 90},
				{StationID: 2, TimingCapable: true, LastSeenMS: cfg.StartMS, FreshnessMS: 1_000, BatteryPct: 90},
			},
		},
	}
}

func timingComparisonRow(t *testing.T, report TimingProfileComparisonReport, id string) TimingProfileComparisonRow {
	t.Helper()
	for _, row := range report.Rows {
		if row.RowID == id {
			return row
		}
	}
	t.Fatalf("missing row %q in %+v", id, report.Rows)
	return TimingProfileComparisonRow{}
}
