package scenario

func voiceRobustnessRecommendedProfiles(rows []VoiceRobustnessSweepRow, cfg VoiceRobustnessSweepConfig) []VoiceRobustnessRecommendedProfile {
	classes := []string{VoiceRobustnessProfileNormal, VoiceRobustnessProfileProtected, VoiceRobustnessProfileRobustDelayed}
	firstAudioCap := cfg.LatencyBudgetMs
	if firstAudioCap > voiceRobustnessDefaultLatencyBudgetMs {
		firstAudioCap = voiceRobustnessDefaultLatencyBudgetMs
	}
	completionCap := cfg.CompletionLatencyMaxMs
	if completionCap > voiceRobustnessDefaultCompletionMs {
		completionCap = voiceRobustnessDefaultCompletionMs
	}
	out := make([]VoiceRobustnessRecommendedProfile, 0, len(classes))
	for _, class := range classes {
		summaries := make(map[string]voiceRobustnessProfileSummary)
		for _, row := range rows {
			if row.ProfileClass != class ||
				!voiceRobustnessRecommendationEligible(row, class) ||
				row.FirstAudioLatencyMs > firstAudioCap ||
				row.CompletionLatencyMs > completionCap {
				continue
			}
			summary := summaries[row.ProfileID]
			summary = voiceRobustnessAccumulateRecommendationSummary(summary, row)
			summaries[row.ProfileID] = summary
		}
		var best voiceRobustnessProfileSummary
		for _, summary := range summaries {
			if best.ProfileID == "" || voiceRobustnessBetterProfileRecommendation(summary, best) {
				best = summary
			}
		}
		if best.ProfileID == "" {
			continue
		}
		out = append(out, VoiceRobustnessRecommendedProfile{
			ProfileClass:                      best.ProfileClass,
			ProfileID:                         best.ProfileID,
			ExemplarRowID:                     best.Exemplar.RowID,
			Outcome:                           best.Outcome,
			GoodVoicePassRows:                 best.GoodVoicePassRows,
			DegradedRows:                      best.DegradedRows,
			FailRows:                          best.FailRows,
			ImprovedRows:                      best.ImprovedRows,
			WorstAdjustedIntelligibilityScore: best.WorstAdjustedIntelligibilityScore,
			FirstAudioLatencyMs:               best.FirstAudioLatencyMs,
			CompletionLatencyMs:               best.CompletionLatencyMs,
			ProtectionOverheadPct:             best.ProtectionOverheadPct,
			JitterBufferMs:                    best.JitterBufferMs,
			BurstMs:                           best.BurstMs,
			ResidualDegradationStillPossible:  best.Outcome != VoiceRobustnessOutcomeGoodVoicePass,
			Recommendation:                    voiceRobustnessRecommendationText(best.Exemplar),
		})
	}
	return out
}

type voiceRobustnessProfileSummary struct {
	ProfileClass                      string
	ProfileID                         string
	Outcome                           string
	GoodVoicePassRows                 int
	DegradedRows                      int
	FailRows                          int
	ImprovedRows                      int
	WorstAdjustedIntelligibilityScore int
	FirstAudioLatencyMs               int64
	CompletionLatencyMs               int64
	ProtectionOverheadPct             int
	JitterBufferMs                    int
	BurstMs                           int
	Exemplar                          VoiceRobustnessSweepRow
}

func voiceRobustnessAccumulateRecommendationSummary(summary voiceRobustnessProfileSummary, row VoiceRobustnessSweepRow) voiceRobustnessProfileSummary {
	if summary.ProfileID == "" {
		summary.ProfileClass = row.ProfileClass
		summary.ProfileID = row.ProfileID
		summary.WorstAdjustedIntelligibilityScore = row.AdjustedIntelligibilityProxyScore
		summary.FirstAudioLatencyMs = row.FirstAudioLatencyMs
		summary.CompletionLatencyMs = row.CompletionLatencyMs
		summary.ProtectionOverheadPct = row.ProtectionOverheadPct
		summary.JitterBufferMs = row.JitterBufferMs
		summary.BurstMs = row.BurstMs
		summary.Exemplar = row
	}
	switch row.Outcome {
	case VoiceRobustnessOutcomeGoodVoicePass:
		summary.GoodVoicePassRows++
	case VoiceRobustnessOutcomeDegraded:
		summary.DegradedRows++
	case VoiceRobustnessOutcomeFail:
		summary.FailRows++
	}
	if row.ImprovedFromBaseline {
		summary.ImprovedRows++
	}
	if row.AdjustedIntelligibilityProxyScore < summary.WorstAdjustedIntelligibilityScore {
		summary.WorstAdjustedIntelligibilityScore = row.AdjustedIntelligibilityProxyScore
	}
	if row.FirstAudioLatencyMs > summary.FirstAudioLatencyMs {
		summary.FirstAudioLatencyMs = row.FirstAudioLatencyMs
	}
	if row.CompletionLatencyMs > summary.CompletionLatencyMs {
		summary.CompletionLatencyMs = row.CompletionLatencyMs
	}
	if voiceRobustnessBetterRecommendation(row, summary.Exemplar) {
		summary.Exemplar = row
	}
	summary.Outcome = voiceRobustnessSummaryOutcome(summary)
	return summary
}

func voiceRobustnessSummaryOutcome(summary voiceRobustnessProfileSummary) string {
	total := summary.GoodVoicePassRows + summary.DegradedRows + summary.FailRows
	if total > 0 && summary.GoodVoicePassRows == total {
		return VoiceRobustnessOutcomeGoodVoicePass
	}
	if total > 0 && summary.FailRows == total {
		return VoiceRobustnessOutcomeFail
	}
	return VoiceRobustnessOutcomeDegraded
}

func voiceRobustnessBetterProfileRecommendation(candidate voiceRobustnessProfileSummary, current voiceRobustnessProfileSummary) bool {
	if candidate.GoodVoicePassRows != current.GoodVoicePassRows {
		return candidate.GoodVoicePassRows > current.GoodVoicePassRows
	}
	if candidate.FailRows != current.FailRows {
		return candidate.FailRows < current.FailRows
	}
	if candidate.DegradedRows != current.DegradedRows {
		return candidate.DegradedRows < current.DegradedRows
	}
	if candidate.ImprovedRows != current.ImprovedRows {
		return candidate.ImprovedRows > current.ImprovedRows
	}
	if candidate.WorstAdjustedIntelligibilityScore != current.WorstAdjustedIntelligibilityScore {
		return candidate.WorstAdjustedIntelligibilityScore > current.WorstAdjustedIntelligibilityScore
	}
	if candidate.FirstAudioLatencyMs != current.FirstAudioLatencyMs {
		return candidate.FirstAudioLatencyMs < current.FirstAudioLatencyMs
	}
	if candidate.CompletionLatencyMs != current.CompletionLatencyMs {
		return candidate.CompletionLatencyMs < current.CompletionLatencyMs
	}
	return candidate.ProfileID < current.ProfileID
}

func voiceRobustnessRecommendationEligible(row VoiceRobustnessSweepRow, class string) bool {
	switch class {
	case VoiceRobustnessProfileNormal:
		return row.ProtectionOverheadPct <= 15 && row.JitterBufferMs <= 160
	case VoiceRobustnessProfileProtected:
		return row.ProtectionOverheadPct >= 25 && row.ProtectionOverheadPct < 35 && row.JitterBufferMs >= 250 && row.JitterBufferMs < 400
	case VoiceRobustnessProfileRobustDelayed:
		return row.ProtectionOverheadPct >= 35 && row.JitterBufferMs >= 400
	default:
		return false
	}
}

func voiceRobustnessBetterRecommendation(candidate VoiceRobustnessSweepRow, current VoiceRobustnessSweepRow) bool {
	if voiceRobustnessOutcomeRank(candidate.Outcome) != voiceRobustnessOutcomeRank(current.Outcome) {
		return voiceRobustnessOutcomeRank(candidate.Outcome) > voiceRobustnessOutcomeRank(current.Outcome)
	}
	if candidate.ImprovedFromBaseline != current.ImprovedFromBaseline {
		return candidate.ImprovedFromBaseline
	}
	if candidate.AdjustedIntelligibilityProxyScore != current.AdjustedIntelligibilityProxyScore {
		return candidate.AdjustedIntelligibilityProxyScore > current.AdjustedIntelligibilityProxyScore
	}
	if candidate.FirstAudioLatencyMs != current.FirstAudioLatencyMs {
		return candidate.FirstAudioLatencyMs < current.FirstAudioLatencyMs
	}
	if candidate.CompletionLatencyMs != current.CompletionLatencyMs {
		return candidate.CompletionLatencyMs < current.CompletionLatencyMs
	}
	return candidate.RowID < current.RowID
}

func voiceRobustnessRecommendationText(row VoiceRobustnessSweepRow) string {
	switch row.ProfileClass {
	case VoiceRobustnessProfileNormal:
		return "use_when_channel_is_clean_or_lightly_impaired"
	case VoiceRobustnessProfileProtected:
		return "use_when_packet_loss_or_short_burst_loss_is_visible"
	default:
		return "use_for_bad_link_delayed_voice_when_latency_budget_allows"
	}
}
