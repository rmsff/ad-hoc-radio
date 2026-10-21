package scenario

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

func RunTimingProfileComparison(fixtures []TimingProfileComparisonFixture) (TimingProfileComparisonReport, error) {
	if err := validateTimingProfileComparisonFixtures(fixtures); err != nil {
		return TimingProfileComparisonReport{}, err
	}
	ordered := append([]TimingProfileComparisonFixture(nil), fixtures...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].FixtureID < ordered[j].FixtureID })
	report := TimingProfileComparisonReport{
		StoryID:      TimingProfileComparisonStoryID,
		ScenarioID:   TimingProfileComparisonScenarioID,
		EvidenceType: TimingProfileComparisonType,
		Rows:         make([]TimingProfileComparisonRow, 0, len(ordered)*4),
	}
	for _, fixture := range ordered {
		for _, strategy := range timingComparisonStrategyOrder {
			for _, profile := range timingComparisonProfileOrder {
				row, err := runTimingProfileComparisonRow(fixture, strategy, profile)
				if err != nil {
					return TimingProfileComparisonReport{}, fmt.Errorf("timing profile comparison: row %s/%s/%s: %w", fixture.FixtureID, strategy, profile, err)
				}
				report.Rows = append(report.Rows, row)
			}
		}
	}
	report.Recommendation = timingProfileRecommendation(report.Rows)
	if err := finalizeTimingProfileEvidence(&report); err != nil {
		return TimingProfileComparisonReport{}, err
	}
	return report, nil
}

func runTimingProfileComparisonRow(
	fixture TimingProfileComparisonFixture,
	strategy timing.TimingStrategy,
	profile TimingProfile,
) (TimingProfileComparisonRow, error) {
	cfg := timingProfileScenarioConfig(fixture.BaseConfig, profile)
	row := newTimingProfileComparisonRow(fixture.FixtureID, strategy, profile, cfg.InitialPlan.CostModel.FramePeriodMS)
	if err := applyTimingStrategy(&cfg, fixture.TimingOverlay, strategy, &row); err != nil {
		return TimingProfileComparisonRow{}, err
	}
	if !row.ProfileFit {
		return row, nil
	}
	lateEntry, collisions, resyncs, err := timingProfileEvidence(
		fixture.TimingOverlay,
		cfg.StartMS,
		cfg.InitialPlan.CostModel.FramePeriodMS,
		cfg.FrameCount,
	)
	if err != nil {
		return TimingProfileComparisonRow{}, err
	}
	row.LateEntryAcquisitionMS = lateEntry
	row.DriftCollisionCount = collisions
	row.ResyncCount = resyncs

	metrics, err := RunScheduledRelayTreeScenario(cfg)
	if err != nil {
		if errors.Is(err, routing.ErrNoFeasibleRelayTree) {
			row.ProfileFit = false
			row.FitReason = TimingFitPlannerNoFeasibleTree
			row.RuntimeOutcome = TimingRuntimeNotRun
			return row, nil
		}
		if strings.Contains(err.Error(), "cycle") || strings.Contains(err.Error(), "MaxCycleMS") {
			row.ProfileFit = false
			row.FitReason = TimingFitScheduleCycleExceeded
			row.RuntimeOutcome = TimingRuntimeNotRun
			return row, nil
		}
		return TimingProfileComparisonRow{}, err
	}
	return populateTimingRuntimeEvidence(row, metrics)
}

func newTimingProfileComparisonRow(
	fixtureID string,
	strategy timing.TimingStrategy,
	profile TimingProfile,
	framePeriodMS int64,
) TimingProfileComparisonRow {
	row := TimingProfileComparisonRow{
		RowID:               fixtureID + "/" + string(strategy) + "/" + string(profile),
		FixtureID:           fixtureID,
		TimingStrategy:      string(strategy),
		Profile:             profile,
		ProfileFit:          true,
		FitReason:           TimingFit,
		RuntimeOutcome:      TimingRuntimeNotRun,
		FramePeriodMS:       framePeriodMS,
		ExpectedMemberIDs:   []uint16{},
		CoveredMemberIDs:    []uint16{},
		DeliveredMemberIDs:  []uint16{},
		UncoveredMemberIDs:  []uint16{},
		LatencyByRelayDepth: []RelayDepthLatencyReport{},
	}
	if profile == TimingProfileDMRInspired30MS {
		row.OpportunityIncludesLowerLayerOverhead = true
		row.ExtraTurnaroundAssumptionMS = 0
	}
	return row
}

func applyTimingStrategy(
	cfg *ScheduledRelayTreeScenarioConfig,
	overlay TimingProfileComparisonOverlay,
	strategy timing.TimingStrategy,
	row *TimingProfileComparisonRow,
) error {
	switch strategy {
	case timing.TimingStrategySourceLed:
		cfg.TimingStrategy = strategy
		cfg.TimingAuthorityID = cfg.SourceStationID
		cfg.TimingEpochID = cfg.CallID
		row.TimingAuthorityID = cfg.SourceStationID
		row.TimingEpochID = cfg.CallID
		return nil
	case timing.TimingStrategyGroupLeader:
		leader, ok := electTimingAuthority(overlay, cfg.StartMS, 0)
		if !ok {
			return fmt.Errorf("no eligible initial timing authority")
		}
		cfg.TimingStrategy = strategy
		cfg.TimingAuthorityID = leader
		cfg.TimingEpochID = 1
		row.TimingAuthorityID = leader
		row.TimingEpochID = 1
		if overlay.LeaderLoss == nil {
			return nil
		}
		return applyTimingAuthorityLoss(cfg, overlay, leader, row)
	default:
		return fmt.Errorf("timing_strategy=%q is invalid", strategy)
	}
}

func applyTimingAuthorityLoss(
	cfg *ScheduledRelayTreeScenarioConfig,
	overlay TimingProfileComparisonOverlay,
	initialLeader uint16,
	row *TimingProfileComparisonRow,
) error {
	loss := *overlay.LeaderLoss
	reacquiredAt := loss.AtMS + loss.LeaderReacquireMS
	row.LeaderLossCount = 1
	row.LeaderReacquireMS = loss.LeaderReacquireMS
	row.LossEventPriority = 12
	row.ReplanEventPriority = sim.PriorityScheduleUpdate
	row.TimingEpochID = 2
	nextTick, hasNextTick, err := nextTimingSourceTick(
		loss.AtMS,
		cfg.StartMS,
		cfg.InitialPlan.CostModel.FramePeriodMS,
		cfg.FrameCount,
	)
	if err != nil {
		return err
	}
	if !hasNextTick {
		row.ProfileFit = false
		row.FitReason = TimingFitLeaderLossOutsideCall
		row.RuntimeOutcome = TimingRuntimeNotRun
		return nil
	}
	if reacquiredAt > nextTick {
		row.ProfileFit = false
		row.FitReason = TimingFitLeaderReacquireExceedsNextTick
		row.RuntimeOutcome = TimingRuntimeNotRun
		return nil
	}
	if err := validateTimingReplanComposition(*cfg, reacquiredAt, nextTick); err != nil {
		return err
	}
	replacement, ok := electTimingAuthority(overlay, reacquiredAt, initialLeader)
	if !ok {
		row.ProfileFit = false
		row.FitReason = TimingFitNoEligibleReplacement
		row.RuntimeOutcome = TimingRuntimeNotRun
		row.TimingAuthorityID = 0
		return nil
	}
	row.TimingAuthorityID = replacement
	plan := timingPlanAtMS(*cfg, reacquiredAt)
	cfg.Replans = append(cfg.Replans, ScheduledRelayTreeReplanConfig{
		AtMS:              reacquiredAt,
		Reason:            ScheduledReplanTimingAuthorityLost,
		Plan:              plan,
		TimingAuthorityID: replacement,
	})
	sort.SliceStable(cfg.Replans, func(i, j int) bool { return cfg.Replans[i].AtMS < cfg.Replans[j].AtMS })
	return nil
}

func validateTimingReplanComposition(cfg ScheduledRelayTreeScenarioConfig, leaderReacquiredAt, leaderEffectiveAt int64) error {
	if err := validateDistinctScheduledReplanEffectiveSequences(cfg); err != nil {
		return err
	}
	seen := map[int64]string{
		leaderEffectiveAt: fmt.Sprintf("leader reacquisition at_ms=%d", leaderReacquiredAt),
	}
	for i, replan := range cfg.Replans {
		effectiveAt, hasEffective, err := nextTimingSourceTick(
			replan.AtMS,
			cfg.StartMS,
			cfg.InitialPlan.CostModel.FramePeriodMS,
			cfg.FrameCount,
		)
		if err != nil {
			return fmt.Errorf("replan[%d] effective source sequence: %w", i, err)
		}
		if !hasEffective {
			continue
		}
		label := fmt.Sprintf("replan[%d] at_ms=%d", i, replan.AtMS)
		if previous, ok := seen[effectiveAt]; ok {
			return fmt.Errorf(
				"%s and %s target the same effective source sequence at_ms=%d",
				previous,
				label,
				effectiveAt,
			)
		}
		seen[effectiveAt] = label
	}
	return nil
}

func timingPlanAtMS(cfg ScheduledRelayTreeScenarioConfig, atMS int64) ScheduledRelayTreePlanConfig {
	plan := cloneTimingRelayTreePlan(cfg.InitialPlan)
	for _, replan := range cfg.Replans {
		if replan.AtMS <= atMS {
			plan = cloneTimingRelayTreePlan(replan.Plan)
		}
	}
	return plan
}

func nextTimingSourceTick(atMS, startMS, framePeriodMS int64, frameCount uint32) (int64, bool, error) {
	if framePeriodMS <= 0 || atMS < startMS {
		return 0, false, fmt.Errorf("invalid source tick inputs at_ms=%d start_ms=%d frame_period_ms=%d", atMS, startMS, framePeriodMS)
	}
	delta := atMS - startMS
	steps := delta / framePeriodMS
	if delta%framePeriodMS != 0 {
		steps++
	}
	if uint64(steps) >= uint64(frameCount) {
		return 0, false, nil
	}
	if steps > (math.MaxInt64-startMS)/framePeriodMS {
		return 0, false, fmt.Errorf("next source tick overflows int64")
	}
	return startMS + steps*framePeriodMS, true, nil
}

func timingProfileEvidence(
	overlay TimingProfileComparisonOverlay,
	startMS int64,
	framePeriodMS int64,
	frameCount uint32,
) (int64, int, int, error) {
	if framePeriodMS <= 0 {
		return 0, 0, 0, fmt.Errorf("frame_period_ms=%d must be > 0", framePeriodMS)
	}
	var maxLateEntry int64
	for _, peer := range overlay.Peers {
		if peer.JoinAtMS != nil {
			wait, err := timingSyncWaitMS(*peer.JoinAtMS, startMS, framePeriodMS)
			if err != nil {
				return 0, 0, 0, err
			}
			if wait > maxLateEntry {
				maxLateEntry = wait
			}
		}
	}
	collisions := 0
	for _, peer := range overlay.Peers {
		peerCollisions, err := timingPeerDriftCollisions(
			peer,
			startMS,
			framePeriodMS,
			frameCount,
			overlay.SyncToleranceMS,
		)
		if err != nil {
			return 0, 0, 0, err
		}
		collisions, err = checkedTimingCollisionAdd(collisions, peerCollisions)
		if err != nil {
			return 0, 0, 0, err
		}
	}
	return maxLateEntry, collisions, collisions, nil
}

func timingPeerDriftCollisions(
	peer TimingAuthorityPeer,
	startMS int64,
	framePeriodMS int64,
	frameCount uint32,
	syncToleranceMS int64,
) (uint32, error) {
	syncCount, err := timingPeerSyncCount(peer, startMS, framePeriodMS, frameCount)
	if err != nil || syncCount == 0 {
		return 0, err
	}
	firstElapsedMS := framePeriodMS
	if peer.JoinAtMS != nil && *peer.JoinAtMS > startMS {
		firstElapsedMS, err = timingSyncWaitMS(*peer.JoinAtMS, startMS, framePeriodMS)
		if err != nil {
			return 0, err
		}
	}
	var collisions uint32
	firstDriftMS, err := checkedTimingDriftMS(firstElapsedMS, peer.DriftPPM)
	if err != nil {
		return 0, err
	}
	if firstDriftMS > syncToleranceMS {
		collisions++
	}
	if syncCount == 1 {
		return collisions, nil
	}
	fullDriftMS, err := checkedTimingDriftMS(framePeriodMS, peer.DriftPPM)
	if err != nil {
		return 0, err
	}
	if fullDriftMS > syncToleranceMS {
		collisions += syncCount - 1
	}
	return collisions, nil
}

func timingPeerSyncCount(peer TimingAuthorityPeer, startMS, framePeriodMS int64, frameCount uint32) (uint32, error) {
	if peer.JoinAtMS == nil || *peer.JoinAtMS <= startMS {
		return frameCount, nil
	}
	delta := *peer.JoinAtMS - startMS
	firstSync := delta / framePeriodMS
	if delta%framePeriodMS != 0 {
		firstSync++
	}
	if uint64(firstSync) >= uint64(frameCount) {
		return 0, nil
	}
	return frameCount - uint32(firstSync), nil
}

func checkedTimingCollisionAdd(total int, add uint32) (int, error) {
	if total < 0 || uint64(add) > uint64(maxIntValue()-total) {
		return 0, fmt.Errorf("timing collision count overflows int")
	}
	return total + int(add), nil
}

func timingSyncWaitMS(joinAtMS, epochStartMS, syncIntervalMS int64) (int64, error) {
	if joinAtMS < epochStartMS || syncIntervalMS <= 0 {
		return 0, fmt.Errorf("invalid sync acquisition inputs join_at_ms=%d epoch_start_ms=%d sync_interval_ms=%d", joinAtMS, epochStartMS, syncIntervalMS)
	}
	offset := (joinAtMS - epochStartMS) % syncIntervalMS
	if offset == 0 {
		return 0, nil
	}
	return syncIntervalMS - offset, nil
}

func populateTimingRuntimeEvidence(
	row TimingProfileComparisonRow,
	metrics ScenarioMetrics,
) (TimingProfileComparisonRow, error) {
	report := metrics.Report()
	scheduled := report.ScheduledRelayTree
	if scheduled == nil {
		return TimingProfileComparisonRow{}, fmt.Errorf("scheduled relay-tree evidence is missing")
	}
	switch report.Outcome {
	case "pass":
		row.RuntimeOutcome = TimingRuntimePass
	case "fail":
		row.RuntimeOutcome = TimingRuntimeFail
	default:
		row.RuntimeOutcome = TimingRuntimeInconclusive
	}
	row.ScheduleCycleDurationMS = scheduled.ScheduleCycleDurationMS
	row.FramePeriodMS = metrics.FramePeriodMs
	row.VoiceAirtimePct = scheduled.VoiceAirtimePct
	row.ControlAirtimePct = scheduled.ControlAirtimePct
	row.SelectedRelayDepth = scheduled.SelectedRelayDepth
	row.ExpectedMemberIDs = append([]uint16(nil), scheduled.ExpectedMemberIDs...)
	row.CoveredMemberIDs = append([]uint16(nil), scheduled.CoveredMemberIDs...)
	row.DeliveredMemberIDs = append([]uint16(nil), scheduled.DeliveredMemberIDs...)
	row.UncoveredMemberIDs = append([]uint16(nil), scheduled.UncoveredMemberIDs...)
	row.LatencyByRelayDepth = append([]RelayDepthLatencyReport(nil), scheduled.LatencyByRelayDepth...)
	row.DuplicatePlayoutCount = scheduled.DuplicatePlayoutCount
	row.OutOfScheduleTxCount = scheduled.OutOfScheduleTxCount
	row.RouteReplanCount = scheduled.RouteReplanCount
	row.VersionActivationCount = scheduled.VersionActivationCount
	row.ScheduleVersion = scheduled.ScheduleVersion
	row.SourceFramesCreated = metrics.SourceFramesCreated
	row.MemberDelivery = append([]ScheduledMemberDeliveryCount(nil), metrics.ScheduledMemberDelivery...)
	for _, latency := range scheduled.LatencyByRelayDepth {
		row.DeliveredFrameCount += latency.SampleCount
	}
	row.TreeHash = scheduled.TreeHash
	row.ScheduleHash = scheduled.ScheduleHash
	row.EventHash = report.Determinism.EventHash
	row.ResultHash = scheduled.ResultHash
	return row, nil
}

func timingProfileScenarioConfig(in ScheduledRelayTreeScenarioConfig, profile TimingProfile) ScheduledRelayTreeScenarioConfig {
	out := cloneTimingRelayTreeScenarioConfig(in)
	if profile != TimingProfileDMRInspired30MS {
		return out
	}
	mapPlan := func(plan ScheduledRelayTreePlanConfig) ScheduledRelayTreePlanConfig {
		plan.CostModel.SourceOpportunityMS = 30
		plan.CostModel.RelayOpportunityMS = 30
		plan.CostModel.GuardAfterMS = 0
		plan.CostModel.FramePeriodMS = 60
		return plan
	}
	out.InitialPlan = mapPlan(out.InitialPlan)
	for i := range out.Replans {
		out.Replans[i].Plan = mapPlan(out.Replans[i].Plan)
	}
	out.OpportunityProfile = timing.OpportunityProfile{
		SourceDurationMS: 30,
		RelayDurationMS:  30,
		GuardAfterMS:     0,
		MaxCycleMS:       60,
	}
	return out
}

func cloneTimingRelayTreeScenarioConfig(in ScheduledRelayTreeScenarioConfig) ScheduledRelayTreeScenarioConfig {
	out := in
	out.InitialPlan = cloneTimingRelayTreePlan(in.InitialPlan)
	out.Replans = make([]ScheduledRelayTreeReplanConfig, len(in.Replans))
	for i, replan := range in.Replans {
		out.Replans[i] = replan
		out.Replans[i].Plan = cloneTimingRelayTreePlan(replan.Plan)
	}
	return out
}

func cloneTimingRelayTreePlan(in ScheduledRelayTreePlanConfig) ScheduledRelayTreePlanConfig {
	out := in
	out.Snapshot = cloneScheduledRelayTreeSnapshot(in.Snapshot)
	out.Graph = cloneScheduledRelayTreeGraph(in.Graph)
	out.RelayStates = append([]routing.RelayState(nil), in.RelayStates...)
	return out
}
