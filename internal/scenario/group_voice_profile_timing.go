package scenario

import (
	"sort"

	"github.com/rmsff/ad-hoc-radio/internal/channel"
)

func evaluateGroupVoiceProfiles(cfg GroupVoiceProfileScenarioConfig, requiredRelayDepth int) []GroupVoiceProfileEvaluation {
	profiles := cloneGroupVoiceProfiles(cfg.Profiles)
	sort.SliceStable(profiles, func(i, j int) bool {
		if profiles[i].QualityRank != profiles[j].QualityRank {
			return profiles[i].QualityRank < profiles[j].QualityRank
		}
		return profiles[i].Name < profiles[j].Name
	})

	evaluations := make([]GroupVoiceProfileEvaluation, 0, len(profiles))
	for _, profile := range profiles {
		evaluations = append(evaluations, evaluateGroupVoiceProfile(profile, requiredRelayDepth))
	}
	return evaluations
}

func evaluateGroupVoiceProfile(profile GroupVoiceProfile, requiredRelayDepth int) GroupVoiceProfileEvaluation {
	evaluation := GroupVoiceProfileEvaluation{
		ProfileName:        profile.Name,
		QualityRank:        profile.QualityRank,
		MaxRelayDepth:      profile.MaxRelayDepth,
		ChannelBitrateBps:  profile.ChannelBitrateBps,
		SourceFrameBytes:   profile.SourceFrameBytes,
		RelayFrameBytes:    profile.RelayFrameBytes,
		SourceWireMode:     profile.SourceWireMode,
		PreambleMs:         profile.PreambleMs,
		SyncMs:             profile.SyncMs,
		TxRxSwitchMs:       profile.TxRxSwitchMs,
		SourceSlotMs:       profile.SourceSlotMs,
		RelaySlotMs:        profile.RelaySlotMs,
		GuardTimeMs:        profile.GuardTimeMs,
		RequiredRelayDepth: requiredRelayDepth,
	}
	if profile.MaxRelayDepth < requiredRelayDepth {
		evaluation.Outcome = GroupVoiceOutcomeFail
		evaluation.RejectReasons = append(evaluation.RejectReasons, "relay_depth_exceeded")
		return evaluation
	}

	switch requiredRelayDepth {
	case 0:
		evaluateDirectGroupVoiceTiming(profile, &evaluation)
	case 1:
		evaluateOneRelayGroupVoiceTiming(profile, &evaluation)
	default:
		evaluateTwoRelayGroupVoiceTiming(profile, &evaluation)
	}
	return evaluation
}

func evaluateDirectGroupVoiceTiming(profile GroupVoiceProfile, evaluation *GroupVoiceProfileEvaluation) {
	budget, err := groupVoiceChannelBudget(profile, 0)
	if err != nil {
		evaluation.Outcome = GroupVoiceOutcomeFail
		evaluation.RejectReasons = append(evaluation.RejectReasons, err.Error())
		return
	}
	applyGroupVoiceBudgetEvaluation(evaluation, budget)
}

func evaluateOneRelayGroupVoiceTiming(profile GroupVoiceProfile, evaluation *GroupVoiceProfileEvaluation) {
	budget, err := groupVoiceChannelBudget(profile, 1)
	if err != nil {
		evaluation.Outcome = GroupVoiceOutcomeFail
		evaluation.RejectReasons = append(evaluation.RejectReasons, err.Error())
		return
	}
	applyGroupVoiceBudgetEvaluation(evaluation, budget)
}

func evaluateTwoRelayGroupVoiceTiming(profile GroupVoiceProfile, evaluation *GroupVoiceProfileEvaluation) {
	budget, err := groupVoiceChannelBudget(profile, 2)
	if err != nil {
		evaluation.Outcome = GroupVoiceOutcomeFail
		evaluation.RejectReasons = append(evaluation.RejectReasons, err.Error())
		return
	}
	applyGroupVoiceBudgetEvaluation(evaluation, budget)
}

func applyGroupVoiceBudgetEvaluation(evaluation *GroupVoiceProfileEvaluation, budget GroupVoiceChannelBudgetReport) {
	evaluation.ChannelBudget = budget
	evaluation.SourceMeasurement = groupVoiceBudgetMeasurement(budget, true)
	evaluation.RelayMeasurement = groupVoiceBudgetMeasurement(budget, false)
	evaluation.CycleOccupiedMs = budget.CycleOccupiedMs
	evaluation.CycleSpareMs = budget.CycleSpareMs
	evaluation.EndToEndLatencyMs = budget.EndToEndLatencyMs
	evaluation.TimingFailReasons = append(evaluation.TimingFailReasons, budget.FailReasons...)
	if len(evaluation.TimingFailReasons) == 0 {
		evaluation.Outcome = GroupVoiceOutcomePass
		return
	}
	evaluation.Outcome = GroupVoiceOutcomeFail
	evaluation.RejectReasons = append(evaluation.RejectReasons, evaluation.TimingFailReasons...)
}

func measureGroupVoiceProfileFrame(frameBytes int, bitrate int64, slotMs int64) (channel.Measurement, error) {
	return channel.MeasureFrameBytes(frameBytes, channel.AirtimeConfig{
		ChannelBitrateBps: bitrate,
		SlotDurationMs:    slotMs,
	})
}
