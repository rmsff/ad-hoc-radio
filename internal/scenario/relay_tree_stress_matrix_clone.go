package scenario

import (
	"encoding/json"
	"sort"

	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

func stressScenarioConfig(caseID string, plan ScheduledRelayTreePlanConfig, depth uint8) ScheduledRelayTreeScenarioConfig {
	id := stressStableID(caseID)
	return ScheduledRelayTreeScenarioConfig{
		ScenarioID:            stressScenarioID(depth),
		Seed:                  uint64(id),
		FrameCount:            5,
		StartMS:               stressStartMS,
		NetID:                 1,
		GroupID:               100,
		SourceStationID:       1,
		CallID:                id,
		ScheduleID:            id,
		ScheduleAuthorityID:   1,
		TimingStrategy:        timing.TimingStrategySourceLed,
		TimingEpochID:         id,
		TimingAuthorityID:     1,
		InitialPlan:           plan,
		OpportunityProfile:    stressOpportunityProfile(),
		ForwardQueueMaxDepth:  64,
		MissedParentThreshold: 2,
	}
}

func stressScenarioID(depth uint8) string {
	switch depth {
	case 0:
		return ScheduledRelayTreeDirectScenarioID
	case 1:
		return ScheduledRelayTreeOneRelayScenarioID
	case 2:
		return ScheduledRelayTreeTwoRelayScenarioID
	default:
		return ScheduledRelayTreeThreeRelayExceptionScenarioID
	}
}

func sortedStressTags(in []RelayTreeStressTag) []RelayTreeStressTag {
	out := append([]RelayTreeStressTag(nil), in...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func cloneRelayTreeStressCases(in []RelayTreeStressCase) []RelayTreeStressCase {
	out := make([]RelayTreeStressCase, len(in))
	for i, testCase := range in {
		out[i] = testCase
		out[i].StressTags = append([]RelayTreeStressTag(nil), testCase.StressTags...)
		out[i].BaseConfig = cloneTimingRelayTreeScenarioConfig(testCase.BaseConfig)
		out[i].TimingOverlay.Peers = append([]TimingAuthorityPeer(nil), testCase.TimingOverlay.Peers...)
		for peerIndex, peer := range testCase.TimingOverlay.Peers {
			if peer.JoinAtMS != nil {
				joinAt := *peer.JoinAtMS
				out[i].TimingOverlay.Peers[peerIndex].JoinAtMS = &joinAt
			}
		}
		if testCase.TimingOverlay.LeaderLoss != nil {
			loss := *testCase.TimingOverlay.LeaderLoss
			out[i].TimingOverlay.LeaderLoss = &loss
		}
		out[i].Expectation.ExpectedUncoveredMemberIDs = append([]uint16(nil), testCase.Expectation.ExpectedUncoveredMemberIDs...)
	}
	return out
}

func marshalRelayTreeStressCases(cases []RelayTreeStressCase) ([]byte, error) {
	return json.Marshal(cloneRelayTreeStressCases(cases))
}
