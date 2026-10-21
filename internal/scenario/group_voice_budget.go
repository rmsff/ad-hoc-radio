package scenario

import "github.com/rmsff/ad-hoc-radio/internal/channel"

func groupVoiceChannelBudget(profile GroupVoiceProfile, requiredRelayDepth int) (GroupVoiceChannelBudgetReport, error) {
	sourceMeasurement, err := measureGroupVoiceProfileFrame(profile.SourceFrameBytes, profile.ChannelBitrateBps, profile.SourceSlotMs)
	if err != nil {
		return GroupVoiceChannelBudgetReport{}, err
	}
	sourceTxDurationMs, err := groupVoiceTxDuration(sourceMeasurement.AirtimeMs, profile)
	if err != nil {
		return GroupVoiceChannelBudgetReport{}, err
	}
	sourceSlotUsedMs, err := checkedLimitedMultiHopAdd(sourceTxDurationMs, profile.GuardTimeMs)
	if err != nil {
		return GroupVoiceChannelBudgetReport{}, err
	}

	budget := GroupVoiceChannelBudgetReport{
		ChannelBitrateBps:   profile.ChannelBitrateBps,
		SourceFrameBytes:    profile.SourceFrameBytes,
		RelayFrameBytes:     profile.RelayFrameBytes,
		SourceWireMode:      profile.SourceWireMode,
		PreambleMs:          profile.PreambleMs,
		SyncMs:              profile.SyncMs,
		TxRxSwitchMs:        profile.TxRxSwitchMs,
		GuardTimeMs:         profile.GuardTimeMs,
		VoicePeriodMs:       profile.VoicePeriodMs,
		LinkDelayMs:         profile.LinkDelayMs,
		SourceSlotMs:        profile.SourceSlotMs,
		RelaySlotMs:         profile.RelaySlotMs,
		SourceAirtimeMs:     sourceMeasurement.AirtimeMs,
		SourceTxDurationMs:  sourceTxDurationMs,
		SourceSlotUsedMs:    sourceSlotUsedMs,
		SourceSlotSpareMs:   profile.SourceSlotMs - sourceSlotUsedMs,
		SourceFitsSlot:      sourceSlotUsedMs <= profile.SourceSlotMs,
		RelayFitsSlot:       true,
		RelayReady:          true,
		LatencyBudgetMs:     profile.LatencyBudgetMs,
		FramePeriodBudgetMs: profile.FramePeriodBudgetMs,
		CadencePass:         profile.VoicePeriodMs <= profile.FramePeriodBudgetMs,
	}

	txOffsetsMs := []int64{0}
	txDurationsMs := []int64{sourceTxDurationMs}
	if requiredRelayDepth > 0 {
		relayMeasurement, err := measureGroupVoiceProfileFrame(profile.RelayFrameBytes, profile.ChannelBitrateBps, profile.RelaySlotMs)
		if err != nil {
			return GroupVoiceChannelBudgetReport{}, err
		}
		relayTxDurationMs, err := groupVoiceTxDuration(relayMeasurement.AirtimeMs, profile)
		if err != nil {
			return GroupVoiceChannelBudgetReport{}, err
		}
		relaySlotUsedMs, err := checkedLimitedMultiHopAdd(relayTxDurationMs, profile.GuardTimeMs)
		if err != nil {
			return GroupVoiceChannelBudgetReport{}, err
		}
		budget.RelayAirtimeMs = relayMeasurement.AirtimeMs
		budget.RelayTxDurationMs = relayTxDurationMs
		budget.RelaySlotUsedMs = relaySlotUsedMs
		budget.RelaySlotSpareMs = profile.RelaySlotMs - relaySlotUsedMs
		budget.RelayFitsSlot = relaySlotUsedMs <= profile.RelaySlotMs
		for depth := 1; depth <= requiredRelayDepth; depth++ {
			offsetMs := profile.SourceSlotMs
			if depth > 1 {
				extraRelaySlots, err := checkedLimitedMultiHopMul(int64(depth-1), profile.RelaySlotMs)
				if err != nil {
					return GroupVoiceChannelBudgetReport{}, err
				}
				offsetMs, err = checkedLimitedMultiHopAdd(offsetMs, extraRelaySlots)
				if err != nil {
					return GroupVoiceChannelBudgetReport{}, err
				}
			}
			txOffsetsMs = append(txOffsetsMs, offsetMs)
			txDurationsMs = append(txDurationsMs, relayTxDurationMs)
		}
	}

	relayMisses, err := groupVoiceRelayReadinessMisses(txOffsetsMs, txDurationsMs, profile)
	if err != nil {
		return GroupVoiceChannelBudgetReport{}, err
	}
	budget.RelayReadinessMissCount = relayMisses
	budget.RelayReady = relayMisses == 0

	lastOffsetMs := txOffsetsMs[len(txOffsetsMs)-1]
	lastDurationMs := txDurationsMs[len(txDurationsMs)-1]
	cycleOccupiedMs, err := checkedLimitedMultiHopAdd(lastOffsetMs, lastDurationMs)
	if err != nil {
		return GroupVoiceChannelBudgetReport{}, err
	}
	cycleOccupiedMs, err = checkedLimitedMultiHopAdd(cycleOccupiedMs, profile.GuardTimeMs)
	if err != nil {
		return GroupVoiceChannelBudgetReport{}, err
	}
	latencyMs, err := checkedLimitedMultiHopAdd(lastOffsetMs, lastDurationMs)
	if err != nil {
		return GroupVoiceChannelBudgetReport{}, err
	}
	latencyMs, err = checkedLimitedMultiHopAdd(latencyMs, profile.LinkDelayMs)
	if err != nil {
		return GroupVoiceChannelBudgetReport{}, err
	}

	budget.CycleOccupiedMs = cycleOccupiedMs
	budget.CycleSpareMs = profile.VoicePeriodMs - cycleOccupiedMs
	budget.CycleFitsPeriod = cycleOccupiedMs <= profile.VoicePeriodMs
	budget.EndToEndLatencyMs = latencyMs
	budget.LatencyBudgetPass = latencyMs <= profile.LatencyBudgetMs
	budget.FailReasons = twoHopLiveFailReasons(
		budget.SourceFitsSlot,
		budget.RelayFitsSlot,
		budget.RelayReadinessMissCount,
		budget.CycleFitsPeriod,
		budget.LatencyBudgetPass,
		budget.CadencePass,
	)
	budget.OverallFits = len(budget.FailReasons) == 0
	return budget, nil
}

func groupVoiceTxDuration(airtimeMs int64, profile GroupVoiceProfile) (int64, error) {
	withPreamble, err := checkedLimitedMultiHopAdd(airtimeMs, profile.PreambleMs)
	if err != nil {
		return 0, err
	}
	return checkedLimitedMultiHopAdd(withPreamble, profile.SyncMs)
}

func groupVoiceRelayReadinessMisses(txOffsetsMs []int64, txDurationsMs []int64, profile GroupVoiceProfile) (int, error) {
	misses := 0
	for i := 0; i < len(txOffsetsMs)-1; i++ {
		readyAtMs, err := checkedLimitedMultiHopAdd(txOffsetsMs[i], txDurationsMs[i])
		if err != nil {
			return 0, err
		}
		readyAtMs, err = checkedLimitedMultiHopAdd(readyAtMs, profile.LinkDelayMs)
		if err != nil {
			return 0, err
		}
		readyAtMs, err = checkedLimitedMultiHopAdd(readyAtMs, profile.TxRxSwitchMs)
		if err != nil {
			return 0, err
		}
		readyAtMs, err = checkedLimitedMultiHopAdd(readyAtMs, profile.GuardTimeMs)
		if err != nil {
			return 0, err
		}
		if readyAtMs > txOffsetsMs[i+1] {
			misses++
		}
	}
	return misses, nil
}

func groupVoiceBudgetMeasurement(budget GroupVoiceChannelBudgetReport, source bool) channel.Measurement {
	if source {
		return channel.Measurement{
			FrameBytes:        budget.SourceFrameBytes,
			FrameBits:         int64(budget.SourceFrameBytes) * 8,
			ChannelBitrateBps: budget.ChannelBitrateBps,
			AirtimeMs:         budget.SourceAirtimeMs,
			SlotDurationMs:    budget.SourceSlotMs,
			FitsSlot:          budget.SourceFitsSlot,
		}
	}
	return channel.Measurement{
		FrameBytes:        budget.RelayFrameBytes,
		FrameBits:         int64(budget.RelayFrameBytes) * 8,
		ChannelBitrateBps: budget.ChannelBitrateBps,
		AirtimeMs:         budget.RelayAirtimeMs,
		SlotDurationMs:    budget.RelaySlotMs,
		FitsSlot:          budget.RelayFitsSlot,
	}
}
