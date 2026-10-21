package scenario

import (
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/routing"
)

func TestScheduledScenarioKeepsUnselectedCandidatesOutOfRuntimeInventory(t *testing.T) {
	for _, direct := range []bool{true, false} {
		name := "alternate_relay"
		links := scheduledMetricsLinks([2]uint16{1, 10}, [2]uint16{1, 11}, [2]uint16{10, 2}, [2]uint16{11, 2})
		wantOpportunities := 2
		if direct {
			name = "direct_with_relay_capable_recipient"
			links = scheduledMetricsLinks([2]uint16{1, 2})
			wantOpportunities = 1
		}
		t.Run(name, func(t *testing.T) {
			states := []routing.RelayState{scheduledMetricsRelayState(10), scheduledMetricsRelayState(11)}
			if direct {
				states = []routing.RelayState{scheduledMetricsRelayState(2)}
			}
			cfg := scheduledMetricsScenarioConfig(ScheduledRelayTreeOneRelayScenarioID, []uint16{2}, links, states)
			metrics, err := RunScheduledRelayTreeScenario(cfg)
			if err != nil {
				t.Fatalf("valid candidate pool rejected: %v", err)
			}
			report := metrics.Report().ScheduledRelayTree
			if report == nil || len(report.DeliveredMemberIDs) != 1 || report.DeliveredMemberIDs[0] != 2 {
				t.Fatalf("recipient not delivered: %+v", report)
			}
			if report.TxOpportunityCount != wantOpportunities || report.OutOfScheduleTxCount != 0 {
				t.Fatalf("unselected candidate affected schedule: %+v", report)
			}
			if len(cfg.InitialPlan.RelayStates) != len(states) {
				t.Fatal("planner candidate pool was mutated")
			}
		})
	}
}

func TestScheduledMissedParentOnLastFramePreservesResult(t *testing.T) {
	version := scheduledTestVersion(t, []routing.RelayTreeEdge{
		{ParentStationID: 1, ChildStationID: 2, Hop: 0},
		{ParentStationID: 2, ChildStationID: 4, Hop: 1},
	}, []uint16{4}, nil, 1, 1, 0, 1)
	version.LinkDelayMS[ScheduledLinkKey{FromStationID: 1, ToStationID: 2}] = 2
	cfg := scheduledTestRuntimeConfig(version, 2)
	cfg.ReplanInputs = []ScheduledReplanInput{scheduledTestReplanInput(
		ScheduledReplanMissedParentThreshold, 0, []uint16{4},
		[][2]uint16{{1, 4}}, nil, 8, 10, 1, true,
	)}
	result := runScheduledRuntimeForTest(t, cfg)
	if result.Counters.MissedOpportunityCount != 2 || result.Counters.RouteReplanCount != 0 ||
		result.ActiveSchedule.Binding.ScheduleVersion != 1 || result.ReceiverCounters[4].FramesAccepted != 0 {
		t.Fatalf("last-frame miss lost or produced an impossible future replan: %+v", result)
	}
}

func TestScheduledMissedParentThresholdIsPerRelay(t *testing.T) {
	t.Run("healthy_sibling_does_not_mask_failed_branch", func(t *testing.T) {
		version := scheduledTestVersion(t, []routing.RelayTreeEdge{
			{ParentStationID: 1, ChildStationID: 2, Hop: 0},
			{ParentStationID: 1, ChildStationID: 3, Hop: 0},
			{ParentStationID: 2, ChildStationID: 4, Hop: 1},
			{ParentStationID: 3, ChildStationID: 5, Hop: 1},
		}, []uint16{4, 5}, nil, 1, 1, 0, 3)
		version.LinkDelayMS[ScheduledLinkKey{FromStationID: 1, ToStationID: 2}] = 2
		cfg := scheduledTestRuntimeConfig(version, 4)
		cfg.ReplanInputs = []ScheduledReplanInput{scheduledTestReplanInput(
			ScheduledReplanMissedParentThreshold, 0, []uint16{4, 5},
			[][2]uint16{{1, 4}, {1, 5}}, nil, 8, 10, 3, true,
		)}
		result := runScheduledRuntimeForTest(t, cfg)
		if result.Counters.RouteReplanCount != 1 || result.Counters.MissedOpportunityCount != 2 {
			t.Fatalf("failed branch did not replan after its two missed frames: %+v", result.Counters)
		}
		if result.ReceiverCounters[4].FramesAccepted != 2 || result.ReceiverCounters[5].FramesAccepted != 4 {
			t.Fatalf("unexpected recovery delivery: %+v", result.ReceiverCounters)
		}
	})

	t.Run("cascaded_misses_do_not_count_as_multiple_source_frames", func(t *testing.T) {
		version := scheduledTestVersion(t, []routing.RelayTreeEdge{
			{ParentStationID: 1, ChildStationID: 2, Hop: 0},
			{ParentStationID: 2, ChildStationID: 3, Hop: 1},
			{ParentStationID: 3, ChildStationID: 4, Hop: 2},
		}, []uint16{4}, nil, 2, 1, 0, 3)
		version.LinkDelayMS[ScheduledLinkKey{FromStationID: 1, ToStationID: 2}] = 2
		cfg := scheduledTestRuntimeConfig(version, 4)
		cfg.ReplanInputs = []ScheduledReplanInput{scheduledTestReplanInput(
			ScheduledReplanMissedParentThreshold, 0, []uint16{4},
			[][2]uint16{{1, 4}}, nil, 8, 10, 3, true,
		)}
		result := runScheduledRuntimeForTest(t, cfg)
		if result.ActiveSchedule.Binding.EffectiveSeq != 2 || result.ReceiverCounters[4].FramesAccepted != 2 {
			t.Fatalf("different relays' misses were combined: effective_seq=%d counters=%+v",
				result.ActiveSchedule.Binding.EffectiveSeq, result.ReceiverCounters)
		}
	})
}
