package scenario

import "github.com/rmsff/ad-hoc-radio/internal/channel"

func delayedGroupConversationLatencies(cfg DelayedGroupConversationConfig, profile DelayedGroupConversationProfile) (map[int]delayedGroupConversationLatency, error) {
	out := make(map[int]delayedGroupConversationLatency, 3)
	for _, relayDepth := range []int{0, 1, 2} {
		latency, err := delayedGroupConversationLatencyForDepth(cfg, profile, relayDepth)
		if err != nil {
			return nil, err
		}
		out[relayDepth] = latency
	}
	return out, nil
}

func delayedGroupConversationLatencyForDepth(cfg DelayedGroupConversationConfig, profile DelayedGroupConversationProfile, relayDepth int) (delayedGroupConversationLatency, error) {
	voiceProfile := VoiceQualityProfile{
		ProfileID:                  profile.ProfileID,
		CodecClass:                 cfg.CodecClass,
		QualityTarget:              VoiceQualityTargetFMLike,
		CodecBitrateBps:            12000,
		AudioBandwidthHz:           cfg.AudioBandwidthHz,
		ChannelBitrateBps:          cfg.ChannelBitrateBps,
		BurstDurationMs:            int64(profile.BurstMs),
		FramePayloadBytes:          cfg.FramePayloadBytes,
		WireHeaderBytes:            cfg.WireHeaderBytes,
		ProtectionOverheadPermille: int64(profile.ProtectionOverheadPct * 10),
		QualityScore:               82,
		FMLikeProxy:                true,
	}
	_, protectedBytes, err := voiceQualityPayloadBytes(voiceProfile)
	if err != nil {
		return delayedGroupConversationLatency{}, err
	}
	frameCount, err := voiceQualityFrameCount(protectedBytes, voiceProfile.FramePayloadBytes)
	if err != nil {
		return delayedGroupConversationLatency{}, err
	}
	maxWireFrameBytes := voiceProfile.FramePayloadBytes + voiceProfile.WireHeaderBytes
	measurement, err := channel.MeasureFrameBytes(maxWireFrameBytes, channel.AirtimeConfig{
		ChannelBitrateBps: voiceProfile.ChannelBitrateBps,
		SlotDurationMs:    voiceProfile.BurstDurationMs,
	})
	if err != nil {
		return delayedGroupConversationLatency{}, err
	}
	frameTxDuration, err := checkedLimitedMultiHopAdd(measurement.AirtimeMs, cfg.PreambleMs)
	if err != nil {
		return delayedGroupConversationLatency{}, err
	}
	frameTxDuration, err = checkedLimitedMultiHopAdd(frameTxDuration, cfg.SyncMs)
	if err != nil {
		return delayedGroupConversationLatency{}, err
	}
	burstTxDuration, err := delayedRelayVoiceBurstTxDuration(frameCount, frameTxDuration, cfg.InterFrameGapMs)
	if err != nil {
		return delayedGroupConversationLatency{}, err
	}
	sourceAvailable := int64(profile.BurstMs)
	if burstTxDuration > sourceAvailable {
		sourceAvailable = burstTxDuration
	}
	sourceAvailable, err = checkedLimitedMultiHopAdd(sourceAvailable, cfg.LinkDelayMs)
	if err != nil {
		return delayedGroupConversationLatency{}, err
	}
	jitter := int64(profile.JitterBufferMs)
	if relayDepth == 0 {
		first, err := checkedLimitedMultiHopAdd(frameTxDuration, cfg.LinkDelayMs)
		if err != nil {
			return delayedGroupConversationLatency{}, err
		}
		first, err = checkedLimitedMultiHopAdd(first, jitter)
		if err != nil {
			return delayedGroupConversationLatency{}, err
		}
		completion, err := checkedLimitedMultiHopAdd(sourceAvailable, jitter)
		if err != nil {
			return delayedGroupConversationLatency{}, err
		}
		return delayedGroupConversationLatency{FirstAudioMs: first, CompletionMs: completion}, nil
	}
	first, completion, err := delayedRelayVoiceRelayTimeline(sourceAvailable, relayDepth, cfg.TurnaroundMs, burstTxDuration, frameTxDuration, cfg.LinkDelayMs)
	if err != nil {
		return delayedGroupConversationLatency{}, err
	}
	first, err = checkedLimitedMultiHopAdd(first, jitter)
	if err != nil {
		return delayedGroupConversationLatency{}, err
	}
	completion, err = checkedLimitedMultiHopAdd(completion, jitter)
	if err != nil {
		return delayedGroupConversationLatency{}, err
	}
	return delayedGroupConversationLatency{FirstAudioMs: first, CompletionMs: completion}, nil
}

func delayedGroupConversationPlaybackConflicts(turns []DelayedGroupConversationTurnConfig, index int, turnStart int64, burstMs int64, firstAudioLatency int64, completionLatency int64) (int, int, error) {
	playStart, err := checkedLimitedMultiHopAdd(turnStart, firstAudioLatency)
	if err != nil {
		return 0, 0, err
	}
	playEnd, err := checkedLimitedMultiHopAdd(turnStart, completionLatency)
	if err != nil {
		return 0, 0, err
	}
	stale := 0
	overlap := 0
	if index+1 < len(turns) && playEnd > turns[index+1].StartMs {
		stale = 1
	}
	for later := index + 1; later < len(turns); later++ {
		laterStart := turns[later].StartMs
		laterEnd, err := checkedLimitedMultiHopAdd(laterStart, burstMs)
		if err != nil {
			return 0, 0, err
		}
		if playStart < laterEnd && playEnd > laterStart {
			overlap++
		}
	}
	return stale, overlap, nil
}

func delayedGroupConversationFirstLatencyCap(cfg DelayedGroupConversationConfig) int64 {
	if cfg.MaxFirstAudioLatencyMs < delayedGroupConversationDefaultFirstLatency {
		return cfg.MaxFirstAudioLatencyMs
	}
	return delayedGroupConversationDefaultFirstLatency
}

func delayedGroupConversationCompletionCap(cfg DelayedGroupConversationConfig) int64 {
	if cfg.MaxCompletionMs < delayedGroupConversationDefaultCompletionMax {
		return cfg.MaxCompletionMs
	}
	return delayedGroupConversationDefaultCompletionMax
}
