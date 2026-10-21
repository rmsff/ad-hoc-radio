package scenario

import (
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/routing"
)

func runScheduledUnchangedReplan(t *testing.T) ScheduledRuntimeResult {
	t.Helper()
	initial := scheduledTestVersion(t, []routing.RelayTreeEdge{
		{ParentStationID: 1, ChildStationID: 2, Hop: 0},
	}, []uint16{2}, nil, 0, 1, 0, 2)
	input := scheduledTestReplanInput(
		ScheduledReplanTopologyEpochChanged,
		1_040,
		[]uint16{2},
		[][2]uint16{{1, 2}},
		nil,
		7,
		9,
		2,
		false,
	)
	cfg := scheduledTestRuntimeConfig(initial, 3)
	cfg.ReplanInputs = []ScheduledReplanInput{input}
	return runScheduledRuntimeForTest(t, cfg)
}

func runScheduledSameFeasibleRecovery(t *testing.T) ScheduledRuntimeResult {
	t.Helper()
	initial := scheduledTestVersion(t, []routing.RelayTreeEdge{
		{ParentStationID: 1, ChildStationID: 10, Hop: 0},
		{ParentStationID: 10, ChildStationID: 2, Hop: 1},
	}, []uint16{2}, nil, 1, 1, 0, 3)
	unavailable := scheduledTestReplanInput(
		ScheduledReplanSelectedRelayUnavailable,
		1_040,
		[]uint16{2},
		[][2]uint16{{1, 10}, {10, 2}},
		[]uint16{10},
		7,
		9,
		3,
		true,
	)
	unavailable.PlanRequest.RelayStates[0].Busy = true
	recovered := scheduledTestReplanInput(
		ScheduledReplanSelectedRelayUnavailable,
		1_120,
		[]uint16{2},
		[][2]uint16{{1, 10}, {10, 2}},
		[]uint16{10},
		7,
		9,
		3,
		true,
	)
	cfg := scheduledTestRuntimeConfig(initial, 4)
	cfg.ReplanInputs = []ScheduledReplanInput{unavailable, recovered}
	return runScheduledRuntimeForTest(t, cfg)
}

func runScheduledLeaseExtension(t *testing.T) ScheduledRuntimeResult {
	t.Helper()
	initial := scheduledTestVersion(t, []routing.RelayTreeEdge{
		{ParentStationID: 1, ChildStationID: 2, Hop: 0},
	}, []uint16{2}, nil, 0, 1, 0, 1)
	input := scheduledTestReplanInput(
		ScheduledReplanTopologyEpochChanged,
		1_040,
		[]uint16{2},
		[][2]uint16{{1, 2}},
		nil,
		7,
		9,
		3,
		false,
	)
	cfg := scheduledTestRuntimeConfig(initial, 4)
	cfg.ReplanInputs = []ScheduledReplanInput{input}
	return runScheduledRuntimeForTest(t, cfg)
}

func runScheduledMissedParentReplan(t *testing.T) ScheduledRuntimeResult {
	t.Helper()
	initial := scheduledTestVersion(t, []routing.RelayTreeEdge{
		{ParentStationID: 1, ChildStationID: 2, Hop: 0},
		{ParentStationID: 2, ChildStationID: 3, Hop: 1},
	}, []uint16{3}, nil, 1, 1, 0, 3)
	initial.LinkDelayMS[ScheduledLinkKey{FromStationID: 1, ToStationID: 2}] = 2
	input := scheduledTestReplanInput(
		ScheduledReplanMissedParentThreshold,
		0,
		[]uint16{3},
		[][2]uint16{{1, 3}},
		nil,
		8,
		10,
		3,
		false,
	)
	cfg := scheduledTestRuntimeConfig(initial, 4)
	cfg.ReplanInputs = []ScheduledReplanInput{input}
	return runScheduledRuntimeForTest(t, cfg)
}

func runScheduledRuntimeForTest(t *testing.T, cfg ScheduledRuntimeConfig) ScheduledRuntimeResult {
	t.Helper()
	result, err := RunScheduledRuntime(cfg)
	if err != nil {
		t.Fatalf("RunScheduledRuntime failed: %v", err)
	}
	return result
}
