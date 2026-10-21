package scenario

import (
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/routing"
)

func TestScheduledPlayoutDuringLongCall(t *testing.T) {
	v := scheduledTestVersion(t, []routing.RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2}}, []uint16{2}, nil, 0, 1, 0, 19)
	result, err := RunScheduledRuntime(scheduledTestRuntimeConfig(v, 20))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.PlayoutEvents) != 20 {
		t.Fatalf("playout events=%d", len(result.PlayoutEvents))
	}
	for i, event := range result.PlayoutEvents {
		if event.Seq != uint32(i) || event.AtMS != 1160+int64(i)*80 || event.LatencyMS != 160 {
			t.Fatalf("event %d: %+v", i, event)
		}
	}
	if result.PlayoutEvents[0].AtMS >= 1000+20*80 {
		t.Fatal("first audio held until call end")
	}
}

func TestScheduledLateAcceptanceIsNotPlayout(t *testing.T) {
	v := scheduledTestVersion(t, []routing.RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2}}, []uint16{2}, nil, 0, 1, 0, 2)
	v.LinkDelayMS[ScheduledLinkKey{1, 2}] = 200
	result, err := RunScheduledRuntime(scheduledTestRuntimeConfig(v, 3))
	if err != nil {
		t.Fatal(err)
	}
	if result.ReceiverCounters[2].FramesAccepted != 2 || len(result.PlayoutEvents) != 0 || len(result.DeliveredMemberIDs) != 0 {
		t.Fatalf("late packets reported delivered: %+v", result)
	}
	member := result.MemberDelivery[0]
	if member.FramesPlayed != 0 || member.FramesLate != 2 || member.FramesMissing != 3 {
		t.Fatalf("member=%+v", member)
	}
	metrics := scheduledRelayTreeScenarioMetrics(ScheduledRelayTreeScenarioConfig{FrameCount: 3}, result)
	if metrics.OutcomeOverride != "fail" || len(metrics.PlayoutLatencySamplesMs) != 0 {
		t.Fatalf("false success: %+v", metrics)
	}
}

func TestScheduledUncoveredMemberRemainsInDenominator(t *testing.T) {
	v := scheduledTestVersion(t, []routing.RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2}}, []uint16{2}, []uint16{3}, 0, 1, 0, 2)
	result, err := RunScheduledRuntime(scheduledTestRuntimeConfig(v, 3))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.MemberDelivery) != 2 || result.MemberDelivery[1].MemberID != 3 || result.MemberDelivery[1].FramesMissing != 3 {
		t.Fatalf("missing recipient omitted: %+v", result.MemberDelivery)
	}
}
