package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func normalizeVoiceRobustnessSweepConfig(cfg VoiceRobustnessSweepConfig) VoiceRobustnessSweepConfig {
	def := DefaultVoiceRobustnessSweepConfig()
	if cfg.ScenarioID == "" {
		cfg.ScenarioID = def.ScenarioID
	}
	if cfg.Seed == 0 {
		cfg.Seed = def.Seed
	}
	if cfg.Fixture.FixtureID == "" && cfg.Fixture.FixturePath == "" && cfg.Fixture.ArtifactSHA256 == "" && cfg.Fixture.EvidenceGrade == "" {
		cfg.Fixture = def.Fixture
	}
	if cfg.NoiseProfiles == nil {
		cfg.NoiseProfiles = cloneAudioIntelligibilityNoiseProfiles(def.NoiseProfiles)
	} else {
		cfg.NoiseProfiles = sortedAudioIntelligibilityNoiseProfiles(cfg.NoiseProfiles)
	}
	if cfg.BurstMsValues == nil {
		cfg.BurstMsValues = sortedVoiceRobustnessInts(def.BurstMsValues)
	} else {
		cfg.BurstMsValues = sortedVoiceRobustnessInts(cfg.BurstMsValues)
	}
	if cfg.ProtectionOverheadPct == nil {
		cfg.ProtectionOverheadPct = sortedVoiceRobustnessInts(def.ProtectionOverheadPct)
	} else {
		cfg.ProtectionOverheadPct = sortedVoiceRobustnessInts(cfg.ProtectionOverheadPct)
	}
	if cfg.JitterBufferMsValues == nil {
		cfg.JitterBufferMsValues = sortedVoiceRobustnessInts(def.JitterBufferMsValues)
	} else {
		cfg.JitterBufferMsValues = sortedVoiceRobustnessInts(cfg.JitterBufferMsValues)
	}
	if cfg.PacketLossPctValues == nil {
		cfg.PacketLossPctValues = sortedVoiceRobustnessInts(def.PacketLossPctValues)
	} else {
		cfg.PacketLossPctValues = sortedVoiceRobustnessInts(cfg.PacketLossPctValues)
	}
	if cfg.BurstLossMsValues == nil {
		cfg.BurstLossMsValues = sortedVoiceRobustnessInts(def.BurstLossMsValues)
	} else {
		cfg.BurstLossMsValues = sortedVoiceRobustnessInts(cfg.BurstLossMsValues)
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
	if cfg.RelayDepth == 0 {
		cfg.RelayDepth = def.RelayDepth
	}
	if cfg.GoodVoiceThreshold == 0 {
		cfg.GoodVoiceThreshold = def.GoodVoiceThreshold
	}
	if cfg.LatencyBudgetMs == 0 {
		cfg.LatencyBudgetMs = def.LatencyBudgetMs
	}
	if cfg.CompletionLatencyMaxMs == 0 {
		cfg.CompletionLatencyMaxMs = def.CompletionLatencyMaxMs
	}
	if cfg.ScoringMethod == "" {
		cfg.ScoringMethod = def.ScoringMethod
	}
	return cfg
}

func validateVoiceRobustnessSweepConfig(cfg VoiceRobustnessSweepConfig) error {
	if cfg.ScenarioID != VoiceRobustnessSweepScenarioID {
		return fmt.Errorf("voice robustness sweep: unsupported scenario_id %q", cfg.ScenarioID)
	}
	if err := validateAudioIntelligibilityFixture(cfg.Fixture); err != nil {
		return err
	}
	if len(cfg.NoiseProfiles) == 0 {
		return fmt.Errorf("voice robustness sweep: noise profiles must not be empty")
	}
	if err := validateAudioIntelligibilityNoiseProfiles(cfg.NoiseProfiles); err != nil {
		return err
	}
	if err := validateVoiceRobustnessPositiveInts("burst_ms", cfg.BurstMsValues); err != nil {
		return err
	}
	if err := validateVoiceRobustnessRangeInts("protection_overhead_pct", cfg.ProtectionOverheadPct, 0, 100); err != nil {
		return err
	}
	if err := validateVoiceRobustnessRangeInts("jitter_buffer_ms", cfg.JitterBufferMsValues, 0, 5000); err != nil {
		return err
	}
	if err := validateVoiceRobustnessRangeInts("packet_loss_pct", cfg.PacketLossPctValues, 0, 100); err != nil {
		return err
	}
	if err := validateVoiceRobustnessRangeInts("burst_loss_ms", cfg.BurstLossMsValues, 0, 10000); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.CodecClass) == "" {
		return fmt.Errorf("voice robustness sweep: codec_class must not be empty")
	}
	if cfg.AudioBandwidthHz <= 0 {
		return fmt.Errorf("voice robustness sweep: audio_bandwidth_hz=%d must be > 0", cfg.AudioBandwidthHz)
	}
	if cfg.ChannelBitrateBps <= 0 {
		return fmt.Errorf("voice robustness sweep: channel_bitrate_bps=%d must be > 0", cfg.ChannelBitrateBps)
	}
	if cfg.FramePayloadBytes <= 0 || cfg.WireHeaderBytes <= 0 {
		return fmt.Errorf("voice robustness sweep: frame payload and wire header bytes must be > 0")
	}
	if cfg.TurnaroundMs < 0 || cfg.LinkDelayMs < 0 || cfg.PreambleMs < 0 || cfg.SyncMs < 0 || cfg.InterFrameGapMs < 0 {
		return fmt.Errorf("voice robustness sweep: timing values must be >= 0")
	}
	if cfg.RelayDepth <= 0 {
		return fmt.Errorf("voice robustness sweep: relay_depth=%d must be > 0", cfg.RelayDepth)
	}
	if cfg.GoodVoiceThreshold < 1 || cfg.GoodVoiceThreshold > 100 {
		return fmt.Errorf("voice robustness sweep: good_voice_threshold=%d must be in [1,100]", cfg.GoodVoiceThreshold)
	}
	if cfg.LatencyBudgetMs <= 0 || cfg.CompletionLatencyMaxMs <= 0 {
		return fmt.Errorf("voice robustness sweep: latency budgets must be > 0")
	}
	if cfg.ScoringMethod != VoiceRobustnessScoringMethod {
		return fmt.Errorf("voice robustness sweep: unsupported scoring_method %q", cfg.ScoringMethod)
	}
	if voiceRobustnessRowCount(cfg) > voiceRobustnessMaxRows {
		return fmt.Errorf("voice robustness sweep: row count would exceed cap %d", voiceRobustnessMaxRows)
	}
	return nil
}

func voiceRobustnessAccumulate(report *VoiceRobustnessSweepReport, row VoiceRobustnessSweepRow) {
	switch row.BaselineOutcome {
	case VoiceRobustnessOutcomeGoodVoicePass:
		report.BaselineGoodVoicePassRows++
	case VoiceRobustnessOutcomeDegraded:
		report.BaselineDegradedRows++
	case VoiceRobustnessOutcomeFail:
		report.BaselineFailRows++
	}
	switch row.Outcome {
	case VoiceRobustnessOutcomeGoodVoicePass:
		report.GoodVoicePassRows++
	case VoiceRobustnessOutcomeDegraded:
		report.DegradedRows++
	case VoiceRobustnessOutcomeFail:
		report.FailRows++
	}
	if row.ImprovedFromBaseline {
		report.ImprovedRows++
	}
}

func voiceRobustnessEffectivePacketLoss(packetLossPct int, protectionPct int) int {
	reduction := voiceRobustnessProtectionReductionPct(protectionPct)
	return voiceRobustnessCeilDivInt(packetLossPct*(100-reduction), 100)
}

func voiceRobustnessEffectiveBurstLoss(burstLossMs int, protectionPct int, jitterAbsorptionMs int) int {
	afterJitter := burstLossMs - jitterAbsorptionMs
	if afterJitter < 0 {
		afterJitter = 0
	}
	reduction := voiceRobustnessProtectionReductionPct(protectionPct) / 2
	return voiceRobustnessCeilDivInt(afterJitter*(100-reduction), 100)
}

func voiceRobustnessProtectionReductionPct(protectionPct int) int {
	if protectionPct <= 15 {
		return 0
	}
	reduction := (protectionPct - 15) * 3
	if reduction > 60 {
		return 60
	}
	return reduction
}

func voiceRobustnessJitterAbsorption(jitterMs int) int {
	if jitterMs <= 160 {
		return 0
	}
	absorption := (jitterMs - 160) / 2
	if absorption > 120 {
		return 120
	}
	return absorption
}

func voiceRobustnessOutcome(score int, fixture AudioIntelligibilityFixture, threshold int, firstAudioLatencyMs int64, completionLatencyMs int64, cfg VoiceRobustnessSweepConfig, reasons []string) string {
	if completionLatencyMs > cfg.CompletionLatencyMaxMs {
		return VoiceRobustnessOutcomeFail
	}
	if score < 45 {
		return VoiceRobustnessOutcomeFail
	}
	if score >= threshold && firstAudioLatencyMs <= cfg.LatencyBudgetMs {
		return audioIntelligibilityOutcome(score, fixture, threshold, reasons)
	}
	return VoiceRobustnessOutcomeDegraded
}

func voiceRobustnessDegradeReasons(cfg VoiceRobustnessSweepConfig, noise AudioIntelligibilityNoiseProfile, score int, firstAudioLatencyMs int64, completionLatencyMs int64) []string {
	reasons := make([]string, 0, 4)
	if !audioIntelligibilityHasStrongEvidence(cfg.Fixture) {
		reasons = append(reasons, "insufficient_fixture_artifact")
	}
	if score < cfg.GoodVoiceThreshold {
		reasons = append(reasons, "intelligibility_score_below_threshold")
	}
	if score < 45 {
		reasons = append(reasons, "intelligibility_score_below_fail_floor")
	}
	if firstAudioLatencyMs > cfg.LatencyBudgetMs {
		reasons = append(reasons, "first_audio_latency_budget_exceeded")
	}
	if completionLatencyMs > cfg.CompletionLatencyMaxMs {
		reasons = append(reasons, "completion_latency_budget_exceeded")
	}
	if noise.NoiseSNRDb < 12 {
		reasons = append(reasons, "snr_residual_risk")
	}
	return reasons
}

func voiceRobustnessResidualRisk(noise AudioIntelligibilityNoiseProfile, packetLossPct int, burstLossMs int, effectivePacketLossPct int, effectiveBurstLossMs int) []string {
	risks := make([]string, 0, 3)
	if noise.NoiseSNRDb < 12 {
		risks = append(risks, "severe_snr_not_solved_by_packet_protection")
	}
	if packetLossPct > 0 && effectivePacketLossPct > 0 {
		risks = append(risks, "residual_packet_loss")
	}
	if burstLossMs > 0 && effectiveBurstLossMs > 0 {
		risks = append(risks, "residual_burst_loss")
	}
	return risks
}

func voiceRobustnessImproved(baselineOutcome string, outcome string, baselineScore int, adjustedScore int) bool {
	if voiceRobustnessOutcomeRank(outcome) > voiceRobustnessOutcomeRank(baselineOutcome) {
		return true
	}
	return adjustedScore > baselineScore
}

func voiceRobustnessOutcomeRank(outcome string) int {
	switch outcome {
	case VoiceRobustnessOutcomeGoodVoicePass:
		return 3
	case VoiceRobustnessOutcomeDegraded:
		return 2
	default:
		return 1
	}
}

func voiceRobustnessProfileClass(burstMs int, protectionPct int, jitterMs int) string {
	if protectionPct <= 15 && jitterMs <= 160 {
		return VoiceRobustnessProfileNormal
	}
	if protectionPct >= 35 && jitterMs >= 400 {
		return VoiceRobustnessProfileRobustDelayed
	}
	return VoiceRobustnessProfileProtected
}

func voiceRobustnessProfileID(burstMs int, protectionPct int, jitterMs int) string {
	return fmt.Sprintf("burst_%d_protect_%d_jitter_%d", burstMs, protectionPct, jitterMs)
}

func voiceRobustnessRowID(profileID string, noiseID string, packetLossPct int, burstLossMs int) string {
	return fmt.Sprintf("%s_%s_loss_%d_burst_%d", profileID, noiseID, packetLossPct, burstLossMs)
}

func voiceRobustnessEvidence(cfg VoiceRobustnessSweepConfig, report VoiceRobustnessSweepReport, eventHash string) ScenarioEvidence {
	outcome := VoiceRobustnessOutcomeDegraded
	reasons := []string{"matrix_contains_degraded_or_failed_rows"}
	if report.RowCount > 0 && report.GoodVoicePassRows == report.RowCount {
		outcome = VoiceRobustnessOutcomeGoodVoicePass
		reasons = nil
	} else if report.GoodVoicePassRows == 0 && report.DegradedRows == 0 && report.FailRows > 0 {
		outcome = VoiceRobustnessOutcomeFail
		reasons = []string{"all_rows_failed"}
	} else if report.GoodVoicePassRows == 0 {
		reasons = []string{"no_good_voice_pass_rows"}
	}
	return ScenarioEvidence{
		SchemaVersion: ScenarioEvidenceSchemaVersion,
		StoryID:       VoiceRobustnessSweepStoryID,
		ScenarioID:    cfg.ScenarioID,
		EvidenceType:  VoiceRobustnessSweepEvidenceType,
		EvidenceGrade: cfg.Fixture.EvidenceGrade,
		Seed:          cfg.Seed,
		Determinism: ScenarioEvidenceDeterminism{
			EventHash:     eventHash,
			HashAlgorithm: ScenarioEvidenceHashAlgorithm,
			Serialization: ScenarioEvidenceSerialization,
		},
		ArtifactRefs: []ScenarioEvidenceArtifact{
			{
				ArtifactID: cfg.Fixture.FixtureID,
				Path:       cfg.Fixture.FixturePath,
				SHA256:     cfg.Fixture.ArtifactSHA256,
				Role:       "audio_fixture",
			},
		},
		AssumptionFlags: []string{
			"deterministic_proxy_score",
			"not_subjective_mos",
			"not_real_codec",
			"not_real_fec",
			"report_only_robustness_sweep",
		},
		HardwareClaimAllowed: false,
		Outcome:              outcome,
		DegradeReasons:       reasons,
	}
}

func voiceRobustnessHash(report VoiceRobustnessSweepReport) (string, error) {
	hashable := report
	hashable.EventHash = ""
	hashable.Evidence.Determinism.EventHash = ""
	b, err := json.Marshal(hashable)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func voiceRobustnessRowCount(cfg VoiceRobustnessSweepConfig) int {
	return len(cfg.BurstMsValues) *
		len(cfg.ProtectionOverheadPct) *
		len(cfg.JitterBufferMsValues) *
		len(cfg.PacketLossPctValues) *
		len(cfg.BurstLossMsValues) *
		len(cfg.NoiseProfiles)
}

func sortedVoiceRobustnessInts(values []int) []int {
	out := append([]int(nil), values...)
	sort.Ints(out)
	return out
}

func validateVoiceRobustnessPositiveInts(name string, values []int) error {
	return validateVoiceRobustnessRangeInts(name, values, 1, 10000)
}

func validateVoiceRobustnessRangeInts(name string, values []int, minValue int, maxValue int) error {
	if len(values) == 0 {
		return fmt.Errorf("voice robustness sweep: %s must not be empty", name)
	}
	seen := make(map[int]struct{}, len(values))
	for _, value := range values {
		if value < minValue || value > maxValue {
			return fmt.Errorf("voice robustness sweep: %s contains %d, want [%d,%d]", name, value, minValue, maxValue)
		}
		if _, ok := seen[value]; ok {
			return fmt.Errorf("voice robustness sweep: %s contains duplicate %d", name, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func voiceRobustnessCeilDivInt(numerator int, denominator int) int {
	if denominator <= 0 {
		panic("voice robustness sweep: denominator must be > 0")
	}
	return (numerator + denominator - 1) / denominator
}
