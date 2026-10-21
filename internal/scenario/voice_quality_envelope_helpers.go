package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

func normalizeVoiceQualityEnvelopeConfig(cfg VoiceQualityEnvelopeConfig) VoiceQualityEnvelopeConfig {
	def := DefaultVoiceQualityEnvelopeConfig()
	if cfg.ScenarioID == "" {
		cfg.ScenarioID = def.ScenarioID
	}
	if cfg.Seed == 0 {
		cfg.Seed = def.Seed
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
	if cfg.RelayDepthCandidates == nil {
		cfg.RelayDepthCandidates = sortedVoiceQualityInts(def.RelayDepthCandidates)
	} else {
		cfg.RelayDepthCandidates = sortedVoiceQualityInts(cfg.RelayDepthCandidates)
	}
	if cfg.LatencyBudgetMs == 0 {
		cfg.LatencyBudgetMs = def.LatencyBudgetMs
	}
	if cfg.LiveTurnaroundBudgetMs == 0 {
		cfg.LiveTurnaroundBudgetMs = def.LiveTurnaroundBudgetMs
	}
	if cfg.Profiles == nil {
		cfg.Profiles = cloneVoiceQualityProfiles(def.Profiles)
	} else {
		cfg.Profiles = sortedVoiceQualityProfiles(cfg.Profiles)
	}
	return cfg
}

func validateVoiceQualityEnvelopeConfig(cfg VoiceQualityEnvelopeConfig) error {
	if cfg.ScenarioID == "" {
		return fmt.Errorf("voice quality envelope: scenario_id must not be empty")
	}
	if cfg.ScenarioID != VoiceQualityEnvelopeScenarioID {
		return fmt.Errorf("voice quality envelope: unsupported scenario_id %q", cfg.ScenarioID)
	}
	if cfg.TurnaroundMs < 0 {
		return fmt.Errorf("voice quality envelope: turnaround_ms=%d must be >= 0", cfg.TurnaroundMs)
	}
	if cfg.LinkDelayMs < 0 || cfg.PreambleMs < 0 || cfg.SyncMs < 0 || cfg.InterFrameGapMs < 0 {
		return fmt.Errorf("voice quality envelope: link/preamble/sync/gap values must be >= 0")
	}
	if cfg.LatencyBudgetMs <= 0 {
		return fmt.Errorf("voice quality envelope: latency_budget_ms=%d must be > 0", cfg.LatencyBudgetMs)
	}
	if cfg.LiveTurnaroundBudgetMs < 0 {
		return fmt.Errorf("voice quality envelope: live_turnaround_budget_ms=%d must be >= 0", cfg.LiveTurnaroundBudgetMs)
	}
	if err := validatePositiveVoiceQualityInts("relay_depth_candidates", cfg.RelayDepthCandidates); err != nil {
		return err
	}
	if len(cfg.Profiles) == 0 {
		return fmt.Errorf("voice quality envelope: profiles must not be empty")
	}
	if voiceQualityEnvelopeRowCount(cfg) > voiceQualityMaxRows {
		return fmt.Errorf("voice quality envelope: row count would exceed cap %d", voiceQualityMaxRows)
	}
	seen := make(map[string]struct{}, len(cfg.Profiles))
	for _, profile := range cfg.Profiles {
		if err := validateVoiceQualityProfile(profile); err != nil {
			return err
		}
		if _, ok := seen[profile.ProfileID]; ok {
			return fmt.Errorf("voice quality envelope: duplicate profile_id %q", profile.ProfileID)
		}
		seen[profile.ProfileID] = struct{}{}
	}
	return nil
}

func validateVoiceQualityProfile(profile VoiceQualityProfile) error {
	if profile.ProfileID == "" {
		return fmt.Errorf("voice quality envelope: profile_id must not be empty")
	}
	if profile.CodecClass == "" {
		return fmt.Errorf("voice quality envelope: profile %q codec_class must not be empty", profile.ProfileID)
	}
	switch profile.QualityTarget {
	case VoiceQualityTargetCommunications, VoiceQualityTargetFMLike:
	default:
		return fmt.Errorf("voice quality envelope: profile %q unsupported quality_target %q", profile.ProfileID, profile.QualityTarget)
	}
	if profile.CodecBitrateBps <= 0 || profile.ChannelBitrateBps <= 0 {
		return fmt.Errorf("voice quality envelope: profile %q bitrates must be > 0", profile.ProfileID)
	}
	if profile.AudioBandwidthHz <= 0 {
		return fmt.Errorf("voice quality envelope: profile %q audio_bandwidth_hz must be > 0", profile.ProfileID)
	}
	if profile.BurstDurationMs <= 0 {
		return fmt.Errorf("voice quality envelope: profile %q burst_duration_ms must be > 0", profile.ProfileID)
	}
	if profile.FramePayloadBytes <= 0 || profile.WireHeaderBytes <= 0 {
		return fmt.Errorf("voice quality envelope: profile %q frame payload and header bytes must be > 0", profile.ProfileID)
	}
	if profile.ProtectionOverheadPermille < 0 {
		return fmt.Errorf("voice quality envelope: profile %q protection_overhead_permille=%d must be >= 0", profile.ProfileID, profile.ProtectionOverheadPermille)
	}
	if profile.QualityScore < 0 || profile.QualityScore > 100 {
		return fmt.Errorf("voice quality envelope: profile %q quality_score=%d must be in [0,100]", profile.ProfileID, profile.QualityScore)
	}
	if profile.FMLikeProxy && profile.QualityTarget != VoiceQualityTargetFMLike {
		return fmt.Errorf("voice quality envelope: profile %q fm_like_proxy requires quality_target=%q", profile.ProfileID, VoiceQualityTargetFMLike)
	}
	return nil
}

func voiceQualityPayloadBytes(profile VoiceQualityProfile) (int64, int64, error) {
	encodedBits, err := checkedLimitedMultiHopMul(profile.CodecBitrateBps, profile.BurstDurationMs)
	if err != nil {
		return 0, 0, err
	}
	encodedBits = voiceQualityCeilDiv(encodedBits, 1000)
	protectedBits, err := checkedLimitedMultiHopMul(encodedBits, 1000+profile.ProtectionOverheadPermille)
	if err != nil {
		return 0, 0, err
	}
	protectedBits = voiceQualityCeilDiv(protectedBits, 1000)
	return voiceQualityCeilDiv(encodedBits, 8), voiceQualityCeilDiv(protectedBits, 8), nil
}

func voiceQualityFrameCount(protectedBytes int64, framePayloadBytes int) (int, error) {
	if protectedBytes <= 0 || framePayloadBytes <= 0 {
		return 0, fmt.Errorf("voice quality envelope: protected bytes and frame payload must be > 0")
	}
	frames := voiceQualityCeilDiv(protectedBytes, int64(framePayloadBytes))
	if frames > int64(voiceQualityMaxRows*DefaultFrameCount) {
		return 0, fmt.Errorf("voice quality envelope: frame_count=%d is out of range", frames)
	}
	return int(frames), nil
}

func voiceQualityFailReasons(cfg VoiceQualityEnvelopeConfig, row VoiceQualityEnvelopeRow) []string {
	reasons := make([]string, 0, 3)
	if !row.FMLikeProxy {
		reasons = append(reasons, voiceQualityReasonNotFMLikeProxy)
	}
	if !row.BurstDrainFasterThanSpeech {
		reasons = append(reasons, voiceQualityReasonDrainSlower)
	}
	if row.MaxUserPerceivedLatencyMs > cfg.LatencyBudgetMs {
		reasons = append(reasons, voiceQualityReasonLatencyBudget)
	}
	return reasons
}

func voiceQualityOutcome(reasons []string) string {
	for _, reason := range reasons {
		if reason == voiceQualityReasonLatencyBudget || reason == voiceQualityReasonDrainSlower {
			return VoiceQualityOutcomeFail
		}
	}
	for _, reason := range reasons {
		if reason == voiceQualityReasonNotFMLikeProxy {
			return VoiceQualityOutcomeDegraded
		}
	}
	return VoiceQualityOutcomePass
}

func bestVoiceQualityEnvelopeRow(rows []VoiceQualityEnvelopeRow, relayDepth int) VoiceQualityEnvelopeRow {
	var best VoiceQualityEnvelopeRow
	for _, row := range rows {
		if row.RelayDepth != relayDepth || row.Outcome != VoiceQualityOutcomePass || !row.FMLikeProxy {
			continue
		}
		if best.RowID == "" ||
			row.MaxUserPerceivedLatencyMs < best.MaxUserPerceivedLatencyMs ||
			(row.MaxUserPerceivedLatencyMs == best.MaxUserPerceivedLatencyMs && row.QualityScore > best.QualityScore) ||
			(row.MaxUserPerceivedLatencyMs == best.MaxUserPerceivedLatencyMs && row.QualityScore == best.QualityScore && row.CodecBitrateBps > best.CodecBitrateBps) {
			best = row
		}
	}
	return best
}

func voiceQualityCeilDiv(numerator, denominator int64) int64 {
	if denominator <= 0 {
		panic("voice quality envelope: denominator must be > 0")
	}
	return (numerator + denominator - 1) / denominator
}

func voiceQualityPermille(numerator, denominator int64) int64 {
	if denominator <= 0 {
		return 0
	}
	return (numerator*1000 + denominator/2) / denominator
}

func voiceQualityEnvelopeRowCount(cfg VoiceQualityEnvelopeConfig) int {
	return len(cfg.Profiles) * len(cfg.RelayDepthCandidates)
}

func cloneVoiceQualityProfiles(profiles []VoiceQualityProfile) []VoiceQualityProfile {
	return append([]VoiceQualityProfile(nil), profiles...)
}

func sortedVoiceQualityProfiles(profiles []VoiceQualityProfile) []VoiceQualityProfile {
	out := cloneVoiceQualityProfiles(profiles)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].ProfileID < out[j].ProfileID
	})
	return out
}

func sortedVoiceQualityInts(values []int) []int {
	out := append([]int(nil), values...)
	sort.Ints(out)
	return out
}

func validatePositiveVoiceQualityInts(name string, values []int) error {
	if len(values) == 0 {
		return fmt.Errorf("voice quality envelope: %s must not be empty", name)
	}
	seen := make(map[int]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			return fmt.Errorf("voice quality envelope: %s contains %d, want > 0", name, value)
		}
		if _, ok := seen[value]; ok {
			return fmt.Errorf("voice quality envelope: %s contains duplicate %d", name, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func voiceQualityEnvelopeHash(report VoiceQualityEnvelopeReport) (string, error) {
	hashable := report
	hashable.EventHash = ""
	b, err := json.Marshal(hashable)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
