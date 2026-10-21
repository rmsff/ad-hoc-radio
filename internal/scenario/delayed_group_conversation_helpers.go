package scenario

import "sort"

func defaultDelayedGroupConversationTurns() []DelayedGroupConversationTurnConfig {
	return []DelayedGroupConversationTurnConfig{
		{TurnID: "turn_1", SpeakerID: 1, StartMs: 0},
		{TurnID: "turn_2", SpeakerID: 11, StartMs: 2500},
		{TurnID: "turn_3", SpeakerID: 12, StartMs: 5000},
		{TurnID: "turn_4", SpeakerID: 1, StartMs: 7500},
		{TurnID: "turn_5", SpeakerID: 11, StartMs: 10000},
	}
}

func defaultDelayedGroupConversationProfiles() []DelayedGroupConversationProfile {
	return []DelayedGroupConversationProfile{
		{
			ProfileID:             "burst_250_protect_15_jitter_160",
			ProfileClass:          VoiceRobustnessProfileNormal,
			BurstMs:               250,
			ProtectionOverheadPct: 15,
			JitterBufferMs:        160,
		},
		{
			ProfileID:             "burst_250_protect_25_jitter_250",
			ProfileClass:          VoiceRobustnessProfileProtected,
			BurstMs:               250,
			ProtectionOverheadPct: 25,
			JitterBufferMs:        250,
		},
		{
			ProfileID:             "burst_250_protect_35_jitter_400",
			ProfileClass:          VoiceRobustnessProfileRobustDelayed,
			BurstMs:               250,
			ProtectionOverheadPct: 35,
			JitterBufferMs:        400,
		},
	}
}

func normalizeDelayedGroupConversationConfig(cfg DelayedGroupConversationConfig) DelayedGroupConversationConfig {
	def := DefaultDelayedGroupConversationConfig()
	if cfg.ScenarioID == "" {
		cfg.ScenarioID = def.ScenarioID
	}
	if cfg.Seed == 0 {
		cfg.Seed = def.Seed
	}
	if cfg.SpeakerIDs == nil {
		cfg.SpeakerIDs = cloneDelayedGroupConversationUint16s(def.SpeakerIDs)
	} else {
		cfg.SpeakerIDs = sortedDelayedGroupConversationUint16s(cfg.SpeakerIDs)
	}
	if cfg.TalkTurns == nil {
		cfg.TalkTurns = cloneDelayedGroupConversationTurns(def.TalkTurns)
	}
	if cfg.DirectReceiverIDs == nil {
		cfg.DirectReceiverIDs = cloneDelayedGroupConversationUint16s(def.DirectReceiverIDs)
	} else {
		cfg.DirectReceiverIDs = sortedDelayedGroupConversationUint16s(cfg.DirectReceiverIDs)
	}
	if cfg.OneHopReceiverIDs == nil {
		cfg.OneHopReceiverIDs = cloneDelayedGroupConversationUint16s(def.OneHopReceiverIDs)
	} else {
		cfg.OneHopReceiverIDs = sortedDelayedGroupConversationUint16s(cfg.OneHopReceiverIDs)
	}
	if cfg.TwoHopReceiverIDs == nil {
		cfg.TwoHopReceiverIDs = cloneDelayedGroupConversationUint16s(def.TwoHopReceiverIDs)
	} else {
		cfg.TwoHopReceiverIDs = sortedDelayedGroupConversationUint16s(cfg.TwoHopReceiverIDs)
	}
	if cfg.UncoveredReceiverIDs == nil {
		cfg.UncoveredReceiverIDs = nil
	} else {
		cfg.UncoveredReceiverIDs = sortedDelayedGroupConversationUint16s(cfg.UncoveredReceiverIDs)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = cloneDelayedGroupConversationProfiles(def.Profiles)
	} else {
		cfg.Profiles = sortedDelayedGroupConversationProfiles(cfg.Profiles)
	}
	if cfg.CodecClass == "" {
		cfg.CodecClass = def.CodecClass
	}
	if cfg.AudioBandwidthHz == 0 {
		cfg.AudioBandwidthHz = def.AudioBandwidthHz
	}
	if cfg.ChannelBitrateBps == 0 {
		cfg.ChannelBitrateBps = def.ChannelBitrateBps
	}
	if cfg.FramePayloadBytes == 0 {
		cfg.FramePayloadBytes = def.FramePayloadBytes
	}
	if cfg.WireHeaderBytes == 0 {
		cfg.WireHeaderBytes = def.WireHeaderBytes
	}
	if cfg.TurnaroundMs == 0 {
		cfg.TurnaroundMs = def.TurnaroundMs
	}
	if cfg.LinkDelayMs == 0 {
		cfg.LinkDelayMs = def.LinkDelayMs
	}
	if cfg.PreambleMs == 0 {
		cfg.PreambleMs = def.PreambleMs
	}
	if cfg.SyncMs == 0 {
		cfg.SyncMs = def.SyncMs
	}
	if cfg.InterFrameGapMs == 0 {
		cfg.InterFrameGapMs = def.InterFrameGapMs
	}
	if cfg.MaxFirstAudioLatencyMs == 0 {
		cfg.MaxFirstAudioLatencyMs = def.MaxFirstAudioLatencyMs
	}
	if cfg.MaxCompletionMs == 0 {
		cfg.MaxCompletionMs = def.MaxCompletionMs
	}
	return cfg
}

func delayedGroupConversationProfileRow(cfg DelayedGroupConversationConfig, profile DelayedGroupConversationProfile, expectedReceivers []uint16) (DelayedGroupConversationProfileRow, error) {
	row := DelayedGroupConversationProfileRow{
		ProfileID:              profile.ProfileID,
		ProfileClass:           profile.ProfileClass,
		BurstMs:                profile.BurstMs,
		ProtectionOverheadPct:  profile.ProtectionOverheadPct,
		JitterBufferMs:         profile.JitterBufferMs,
		TurnCount:              len(cfg.TalkTurns),
		SpeakerCount:           len(cfg.SpeakerIDs),
		ExpectedReceiverCount:  len(expectedReceivers),
		AccountedReceiverCount: len(cfg.DirectReceiverIDs) + len(cfg.OneHopReceiverIDs) + len(cfg.TwoHopReceiverIDs),
		TurnReports:            make([]DelayedGroupConversationTurnReport, 0, len(cfg.TalkTurns)),
	}
	latencies, err := delayedGroupConversationLatencies(cfg, profile)
	if err != nil {
		return DelayedGroupConversationProfileRow{}, err
	}
	row.ReceiverGroups = delayedGroupConversationInitialGroupSummaries(cfg, latencies)
	row.MissingUncoveredReceiverCount = len(cfg.UncoveredReceiverIDs)
	for _, group := range row.ReceiverGroups {
		if group.FirstAudioLatencyMs > row.MaxFirstAudioLatencyMs {
			row.MaxFirstAudioLatencyMs = group.FirstAudioLatencyMs
		}
		if group.CompletionLatencyMs > row.MaxCompletionLatencyMs {
			row.MaxCompletionLatencyMs = group.CompletionLatencyMs
		}
	}
	for i, turn := range cfg.TalkTurns {
		turnReport := DelayedGroupConversationTurnReport{
			TurnID:         turn.TurnID,
			SpeakerID:      turn.SpeakerID,
			StartMs:        turn.StartMs,
			BurstMs:        int64(profile.BurstMs),
			NextTurnStart:  -1,
			ReceiverGroups: delayedGroupConversationInitialGroupSummaries(cfg, latencies),
		}
		if i+1 < len(cfg.TalkTurns) {
			turnReport.NextTurnStart = cfg.TalkTurns[i+1].StartMs
		}
		for groupIndex := range turnReport.ReceiverGroups {
			group := &turnReport.ReceiverGroups[groupIndex]
			stale, overlap, err := delayedGroupConversationPlaybackConflicts(cfg.TalkTurns, i, turn.StartMs, int64(profile.BurstMs), group.FirstAudioLatencyMs, group.CompletionLatencyMs)
			if err != nil {
				return DelayedGroupConversationProfileRow{}, err
			}
			group.StalePlayoutCount = stale * group.ReceiverCount
			group.OverlapCount = overlap * group.ReceiverCount
			row.StalePlayoutCount += group.StalePlayoutCount
			row.OverlapCount += group.OverlapCount
			row.ReceiverGroups[groupIndex].StalePlayoutCount += group.StalePlayoutCount
			row.ReceiverGroups[groupIndex].OverlapCount += group.OverlapCount
		}
		row.TurnReports = append(row.TurnReports, turnReport)
	}
	row.DegradeReasons = delayedGroupConversationRowReasons(cfg, row)
	row.Outcome = delayedGroupConversationRowOutcome(cfg, row)
	return row, nil
}

func delayedGroupConversationInitialGroupSummaries(cfg DelayedGroupConversationConfig, latencies map[int]delayedGroupConversationLatency) []DelayedGroupConversationReceiverGroup {
	return []DelayedGroupConversationReceiverGroup{
		delayedGroupConversationGroupSummary("direct", 0, cfg.DirectReceiverIDs, latencies[0], false),
		delayedGroupConversationGroupSummary("one_hop", 1, cfg.OneHopReceiverIDs, latencies[1], false),
		delayedGroupConversationGroupSummary("two_hop", 2, cfg.TwoHopReceiverIDs, latencies[2], false),
		delayedGroupConversationGroupSummary("uncovered", -1, cfg.UncoveredReceiverIDs, delayedGroupConversationLatency{}, true),
	}
}

func delayedGroupConversationGroupSummary(label string, depth int, ids []uint16, latency delayedGroupConversationLatency, uncovered bool) DelayedGroupConversationReceiverGroup {
	accounted := len(ids)
	missing := 0
	if uncovered {
		accounted = 0
		missing = len(ids)
	}
	return DelayedGroupConversationReceiverGroup{
		DepthLabel:                    label,
		RelayDepth:                    depth,
		ReceiverIDs:                   cloneDelayedGroupConversationUint16s(ids),
		ReceiverCount:                 len(ids),
		FirstAudioLatencyMs:           latency.FirstAudioMs,
		CompletionLatencyMs:           latency.CompletionMs,
		MissingUncoveredReceiverCount: missing,
		AccountedReceiverCount:        accounted,
	}
}

func delayedGroupConversationRowReasons(cfg DelayedGroupConversationConfig, row DelayedGroupConversationProfileRow) []string {
	reasons := make([]string, 0, 5)
	if row.AccountedReceiverCount != row.ExpectedReceiverCount {
		reasons = append(reasons, "not_all_expected_receivers_accounted")
	}
	if row.MissingUncoveredReceiverCount > 0 {
		reasons = append(reasons, "missing_uncovered_receivers")
	}
	if row.MaxFirstAudioLatencyMs > delayedGroupConversationFirstLatencyCap(cfg) {
		reasons = append(reasons, "first_audio_latency_budget_exceeded")
	}
	if row.MaxCompletionLatencyMs > delayedGroupConversationCompletionCap(cfg) {
		reasons = append(reasons, "completion_latency_budget_exceeded")
	}
	if row.StalePlayoutCount > 0 {
		reasons = append(reasons, "stale_playout_over_next_turn")
	}
	if row.OverlapCount > 0 {
		reasons = append(reasons, "delayed_playback_overlaps_later_talk_turn")
	}
	return reasons
}

func delayedGroupConversationRowOutcome(cfg DelayedGroupConversationConfig, row DelayedGroupConversationProfileRow) string {
	if row.MaxCompletionLatencyMs > delayedGroupConversationCompletionCap(cfg) {
		return DelayedGroupConversationOutcomeFail
	}
	if len(row.DegradeReasons) > 0 {
		return DelayedGroupConversationOutcomeDegraded
	}
	return DelayedGroupConversationOutcomePass
}

func delayedGroupConversationReportOutcome(report DelayedGroupConversationReport) (string, []string) {
	if report.RowCount > 0 && report.PassRows == report.RowCount {
		return DelayedGroupConversationOutcomePass, nil
	}
	if report.RowCount > 0 && report.FailRows == report.RowCount {
		return DelayedGroupConversationOutcomeFail, []string{"all_profile_rows_failed"}
	}
	return DelayedGroupConversationOutcomeDegraded, []string{"matrix_contains_degraded_or_failed_rows"}
}

func delayedGroupConversationExpectedReceivers(cfg DelayedGroupConversationConfig) []uint16 {
	out := make([]uint16, 0, len(cfg.DirectReceiverIDs)+len(cfg.OneHopReceiverIDs)+len(cfg.TwoHopReceiverIDs)+len(cfg.UncoveredReceiverIDs))
	out = append(out, cfg.DirectReceiverIDs...)
	out = append(out, cfg.OneHopReceiverIDs...)
	out = append(out, cfg.TwoHopReceiverIDs...)
	out = append(out, cfg.UncoveredReceiverIDs...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func delayedGroupConversationGroupConfigs(cfg DelayedGroupConversationConfig) []DelayedGroupConversationGroupConfig {
	return []DelayedGroupConversationGroupConfig{
		{DepthLabel: "direct", RelayDepth: 0, ReceiverIDs: cloneDelayedGroupConversationUint16s(cfg.DirectReceiverIDs)},
		{DepthLabel: "one_hop", RelayDepth: 1, ReceiverIDs: cloneDelayedGroupConversationUint16s(cfg.OneHopReceiverIDs)},
		{DepthLabel: "two_hop", RelayDepth: 2, ReceiverIDs: cloneDelayedGroupConversationUint16s(cfg.TwoHopReceiverIDs)},
		{DepthLabel: "uncovered", RelayDepth: -1, ReceiverIDs: cloneDelayedGroupConversationUint16s(cfg.UncoveredReceiverIDs)},
	}
}

func cloneDelayedGroupConversationUint16s(values []uint16) []uint16 {
	return append([]uint16(nil), values...)
}

func sortedDelayedGroupConversationUint16s(values []uint16) []uint16 {
	out := cloneDelayedGroupConversationUint16s(values)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func cloneDelayedGroupConversationTurns(values []DelayedGroupConversationTurnConfig) []DelayedGroupConversationTurnConfig {
	return append([]DelayedGroupConversationTurnConfig(nil), values...)
}

func cloneDelayedGroupConversationProfiles(values []DelayedGroupConversationProfile) []DelayedGroupConversationProfile {
	return append([]DelayedGroupConversationProfile(nil), values...)
}

func sortedDelayedGroupConversationProfiles(values []DelayedGroupConversationProfile) []DelayedGroupConversationProfile {
	out := cloneDelayedGroupConversationProfiles(values)
	sort.Slice(out, func(i, j int) bool {
		if out[i].ProfileClass != out[j].ProfileClass {
			return out[i].ProfileClass < out[j].ProfileClass
		}
		return out[i].ProfileID < out[j].ProfileID
	})
	return out
}

func cloneDelayedGroupConversationStrings(values []string) []string {
	return append([]string(nil), values...)
}
