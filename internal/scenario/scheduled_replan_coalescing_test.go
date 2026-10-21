package scenario

import (
	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"testing"
)

func TestScheduledReplanSameBoundaryReplacesProposal(t *testing.T) {
	initial := scheduledTestVersion(t, []routing.RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2}, {ParentStationID: 2, ChildStationID: 3, Hop: 1}}, []uint16{3}, nil, 1, 1, 0, 3)
	initial.LinkDelayMS[ScheduledLinkKey{1, 2}] = 2
	missed := scheduledTestReplanInput(ScheduledReplanMissedParentThreshold, 0, []uint16{3}, [][2]uint16{{1, 3}}, nil, 8, 10, 3, false)
	external := scheduledTestReplanInput(ScheduledReplanTopologyEpochChanged, 1100, []uint16{3}, [][2]uint16{{1, 3}}, nil, 9, 10, 3, false)
	cfg := scheduledTestRuntimeConfig(initial, 4)
	cfg.ReplanInputs = []ScheduledReplanInput{missed, external}
	result, err := RunScheduledRuntime(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result.Counters.VersionActivationCount != 1 || result.ActiveSchedule.Binding.ScheduleVersion != 2 || result.ActiveTree.TopologyEpoch != 9 {
		t.Fatalf("proposal collision: %+v", result)
	}
}
