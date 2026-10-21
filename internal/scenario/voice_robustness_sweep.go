package scenario

import (
	"fmt"

	"github.com/rmsff/ad-hoc-radio/internal/channel"
)

const VoiceRobustnessSweepScenarioID = "voice_robustness_parameter_sweep"
const VoiceRobustnessSweepStoryID = "MVP-8.8.1"
const VoiceRobustnessSweepEvidenceType = "voice_robustness_parameter_sweep"

const (
	VoiceRobustnessProfileNormal        = "normal"
	VoiceRobustnessProfileProtected     = "protected"
	VoiceRobustnessProfileRobustDelayed = "robust_delayed"
)

const (
	VoiceRobustnessOutcomeGoodVoicePass = AudioIntelligibilityOutcomeGoodVoicePass
	VoiceRobustnessOutcomeDegraded      = AudioIntelligibilityOutcomeDegraded
	VoiceRobustnessOutcomeFail          = AudioIntelligibilityOutcomeFail
)

const (
	VoiceRobustnessScoringMethod          = "deterministic_robustness_proxy_v1"
	VoiceRobustnessDefaultThreshold       = AudioIntelligibilityDefaultThreshold
	voiceRobustnessDefaultLatencyBudgetMs = 2000
	voiceRobustnessDefaultCompletionMs    = 2000
	voiceRobustnessMaxRows                = 2000
)

type VoiceRobustnessSweepConfig struct {
	ScenarioID             string
	Seed                   uint64
	Fixture                AudioIntelligibilityFixture
	NoiseProfiles          []AudioIntelligibilityNoiseProfile
	BurstMsValues          []int
	ProtectionOverheadPct  []int
	JitterBufferMsValues   []int
	PacketLossPctValues    []int
	BurstLossMsValues      []int
	CodecClass             string
	AudioBandwidthHz       int
	ChannelBitrateBps      int64
	FramePayloadBytes      int
	WireHeaderBytes        int
	TurnaroundMs           int64
	LinkDelayMs            int64
	PreambleMs             int64
	SyncMs                 int64
	InterFrameGapMs        int64
	RelayDepth             int
	GoodVoiceThreshold     int
	LatencyBudgetMs        int64
	CompletionLatencyMaxMs int64
	ScoringMethod          string
}

type VoiceRobustnessSweepReport struct {
	ScenarioID                string                              `json:"scenario_id"`
	Seed                      uint64                              `json:"seed"`
	Evidence                  ScenarioEvidence                    `json:"evidence"`
	Rows                      []VoiceRobustnessSweepRow           `json:"rows"`
	RowCount                  int                                 `json:"row_count"`
	BaselineGoodVoicePassRows int                                 `json:"baseline_good_voice_pass_rows"`
	BaselineDegradedRows      int                                 `json:"baseline_degraded_rows"`
	BaselineFailRows          int                                 `json:"baseline_fail_rows"`
	GoodVoicePassRows         int                                 `json:"good_voice_pass_rows"`
	DegradedRows              int                                 `json:"degraded_rows"`
	FailRows                  int                                 `json:"fail_rows"`
	ImprovedRows              int                                 `json:"improved_rows"`
	RecommendedProfiles       []VoiceRobustnessRecommendedProfile `json:"recommended_profiles"`
	GoodVoiceThreshold        int                                 `json:"good_voice_threshold"`
	LatencyBudgetMs           int64                               `json:"latency_budget_ms"`
	CompletionLatencyMaxMs    int64                               `json:"completion_latency_max_ms"`
	ScoringMethod             string                              `json:"scoring_method"`
	EventHash                 string                              `json:"event_hash"`
}

type VoiceRobustnessSweepRow struct {
	RowID                             string   `json:"row_id"`
	ProfileID                         string   `json:"profile_id"`
	ProfileClass                      string   `json:"profile_class"`
	NoiseProfileID                    string   `json:"noise_profile_id"`
	NoiseSNRDb                        int      `json:"noise_snr_db"`
	PacketLossPct                     int      `json:"packet_loss_pct"`
	BurstLossMs                       int      `json:"burst_loss_ms"`
	BurstMs                           int      `json:"burst_ms"`
	ProtectionOverheadPct             int      `json:"protection_overhead_pct"`
	JitterBufferMs                    int      `json:"jitter_buffer_ms"`
	AudioBandwidthHz                  int      `json:"audio_bandwidth_hz"`
	CodecClass                        string   `json:"codec_class"`
	BaselineIntelligibilityProxyScore int      `json:"baseline_intelligibility_proxy_score"`
	BaselineOutcome                   string   `json:"baseline_outcome"`
	AdjustedIntelligibilityProxyScore int      `json:"adjusted_intelligibility_proxy_score"`
	Outcome                           string   `json:"outcome"`
	ImprovedFromBaseline              bool     `json:"improved_from_baseline"`
	FirstAudioLatencyMs               int64    `json:"first_audio_latency_ms"`
	CompletionLatencyMs               int64    `json:"completion_latency_ms"`
	LatencyBudgetMs                   int64    `json:"latency_budget_ms"`
	EffectivePacketLossPct            int      `json:"effective_packet_loss_pct"`
	EffectiveBurstLossMs              int      `json:"effective_burst_loss_ms"`
	PacketLossPenaltyReductionPct     int      `json:"packet_loss_penalty_reduction_pct"`
	BurstLossJitterAbsorptionMs       int      `json:"burst_loss_jitter_absorption_ms"`
	ProtectionAssumption              string   `json:"protection_assumption"`
	DegradeReasons                    []string `json:"degrade_reasons"`
	ResidualRisk                      []string `json:"residual_risk"`
}

type VoiceRobustnessRecommendedProfile struct {
	ProfileClass                      string `json:"profile_class"`
	ProfileID                         string `json:"profile_id"`
	ExemplarRowID                     string `json:"exemplar_row_id"`
	Outcome                           string `json:"outcome"`
	GoodVoicePassRows                 int    `json:"good_voice_pass_rows"`
	DegradedRows                      int    `json:"degraded_rows"`
	FailRows                          int    `json:"fail_rows"`
	ImprovedRows                      int    `json:"improved_rows"`
	WorstAdjustedIntelligibilityScore int    `json:"worst_adjusted_intelligibility_proxy_score"`
	FirstAudioLatencyMs               int64  `json:"first_audio_latency_ms"`
	CompletionLatencyMs               int64  `json:"completion_latency_ms"`
	ProtectionOverheadPct             int    `json:"protection_overhead_pct"`
	JitterBufferMs                    int    `json:"jitter_buffer_ms"`
	BurstMs                           int    `json:"burst_ms"`
	ResidualDegradationStillPossible  bool   `json:"residual_degradation_still_possible"`
	Recommendation                    string `json:"recommendation"`
}

func DefaultVoiceRobustnessSweepConfig() VoiceRobustnessSweepConfig {
	audio := DefaultAudioIntelligibilityConfig()
	return VoiceRobustnessSweepConfig{
		ScenarioID:             VoiceRobustnessSweepScenarioID,
		Seed:                   DefaultScenarioSeed,
		Fixture:                audio.Fixture,
		NoiseProfiles:          cloneAudioIntelligibilityNoiseProfiles(audio.NoiseProfiles),
		BurstMsValues:          []int{250, 320, 500},
		ProtectionOverheadPct:  []int{15, 25, 35},
		JitterBufferMsValues:   []int{160, 250, 400},
		PacketLossPctValues:    []int{0, 5, 10},
		BurstLossMsValues:      []int{0, 80, 120},
		CodecClass:             audio.CodecClass,
		AudioBandwidthHz:       audio.AudioBandwidthHz,
		ChannelBitrateBps:      19200,
		FramePayloadBytes:      96,
		WireHeaderBytes:        14,
		TurnaroundMs:           200,
		LinkDelayMs:            1,
		PreambleMs:             1,
		SyncMs:                 1,
		InterFrameGapMs:        1,
		RelayDepth:             2,
		GoodVoiceThreshold:     VoiceRobustnessDefaultThreshold,
		LatencyBudgetMs:        voiceRobustnessDefaultLatencyBudgetMs,
		CompletionLatencyMaxMs: voiceRobustnessDefaultCompletionMs,
		ScoringMethod:          VoiceRobustnessScoringMethod,
	}
}

func RunVoiceRobustnessSweep(cfg VoiceRobustnessSweepConfig) (VoiceRobustnessSweepReport, error) {
	cfg = normalizeVoiceRobustnessSweepConfig(cfg)
	if err := validateVoiceRobustnessSweepConfig(cfg); err != nil {
		return VoiceRobustnessSweepReport{}, err
	}
	report := VoiceRobustnessSweepReport{
		ScenarioID:             cfg.ScenarioID,
		Seed:                   cfg.Seed,
		Rows:                   make([]VoiceRobustnessSweepRow, 0, voiceRobustnessRowCount(cfg)),
		GoodVoiceThreshold:     cfg.GoodVoiceThreshold,
		LatencyBudgetMs:        cfg.LatencyBudgetMs,
		CompletionLatencyMaxMs: cfg.CompletionLatencyMaxMs,
		ScoringMethod:          cfg.ScoringMethod,
	}
	for _, burstMs := range cfg.BurstMsValues {
		for _, protectionPct := range cfg.ProtectionOverheadPct {
			for _, jitterMs := range cfg.JitterBufferMsValues {
				for _, packetLossPct := range cfg.PacketLossPctValues {
					for _, burstLossMs := range cfg.BurstLossMsValues {
						for _, noise := range cfg.NoiseProfiles {
							row, err := voiceRobustnessRow(cfg, burstMs, protectionPct, jitterMs, packetLossPct, burstLossMs, noise)
							if err != nil {
								return VoiceRobustnessSweepReport{}, err
							}
							report.Rows = append(report.Rows, row)
							voiceRobustnessAccumulate(&report, row)
						}
					}
				}
			}
		}
	}
	report.RowCount = len(report.Rows)
	report.RecommendedProfiles = voiceRobustnessRecommendedProfiles(report.Rows, cfg)
	report.Evidence = voiceRobustnessEvidence(cfg, report, "")
	hash, err := voiceRobustnessHash(report)
	if err != nil {
		return VoiceRobustnessSweepReport{}, err
	}
	report.EventHash = hash
	report.Evidence = voiceRobustnessEvidence(cfg, report, hash)
	if err := ValidateScenarioEvidence(report.Evidence); err != nil {
		return VoiceRobustnessSweepReport{}, err
	}
	return report, nil
}

func voiceRobustnessRow(cfg VoiceRobustnessSweepConfig, burstMs int, protectionPct int, jitterMs int, packetLossPct int, burstLossMs int, noise AudioIntelligibilityNoiseProfile) (VoiceRobustnessSweepRow, error) {
	rawLoss := AudioIntelligibilityLossProfile{
		LossProfileID: fmt.Sprintf("loss_%d_burst_%d", packetLossPct, burstLossMs),
		PacketLossPct: packetLossPct,
		BurstLossMs:   burstLossMs,
	}
	baselineScore := audioIntelligibilityScore(noise, rawLoss)
	baselineReasons := audioIntelligibilityDegradeReasons(cfg.Fixture, baselineScore, cfg.GoodVoiceThreshold)
	baselineOutcome := audioIntelligibilityOutcome(baselineScore, cfg.Fixture, cfg.GoodVoiceThreshold, baselineReasons)

	effectivePacketLoss := voiceRobustnessEffectivePacketLoss(packetLossPct, protectionPct)
	jitterAbsorption := voiceRobustnessJitterAbsorption(jitterMs)
	effectiveBurstLoss := voiceRobustnessEffectiveBurstLoss(burstLossMs, protectionPct, jitterAbsorption)
	adjustedLoss := AudioIntelligibilityLossProfile{
		LossProfileID: fmt.Sprintf("effective_loss_%d_burst_%d", effectivePacketLoss, effectiveBurstLoss),
		PacketLossPct: effectivePacketLoss,
		BurstLossMs:   effectiveBurstLoss,
	}
	adjustedScore := audioIntelligibilityScore(noise, adjustedLoss)
	firstAudioLatencyMs, completionLatencyMs, err := voiceRobustnessLatency(cfg, burstMs, protectionPct, jitterMs)
	if err != nil {
		return VoiceRobustnessSweepRow{}, err
	}
	degradeReasons := voiceRobustnessDegradeReasons(cfg, noise, adjustedScore, firstAudioLatencyMs, completionLatencyMs)
	outcome := voiceRobustnessOutcome(adjustedScore, cfg.Fixture, cfg.GoodVoiceThreshold, firstAudioLatencyMs, completionLatencyMs, cfg, degradeReasons)
	residualRisk := voiceRobustnessResidualRisk(noise, packetLossPct, burstLossMs, effectivePacketLoss, effectiveBurstLoss)
	profileClass := voiceRobustnessProfileClass(burstMs, protectionPct, jitterMs)
	profileID := voiceRobustnessProfileID(burstMs, protectionPct, jitterMs)
	return VoiceRobustnessSweepRow{
		RowID:                             voiceRobustnessRowID(profileID, noise.NoiseProfileID, packetLossPct, burstLossMs),
		ProfileID:                         profileID,
		ProfileClass:                      profileClass,
		NoiseProfileID:                    noise.NoiseProfileID,
		NoiseSNRDb:                        noise.NoiseSNRDb,
		PacketLossPct:                     packetLossPct,
		BurstLossMs:                       burstLossMs,
		BurstMs:                           burstMs,
		ProtectionOverheadPct:             protectionPct,
		JitterBufferMs:                    jitterMs,
		AudioBandwidthHz:                  cfg.AudioBandwidthHz,
		CodecClass:                        cfg.CodecClass,
		BaselineIntelligibilityProxyScore: baselineScore,
		BaselineOutcome:                   baselineOutcome,
		AdjustedIntelligibilityProxyScore: adjustedScore,
		Outcome:                           outcome,
		ImprovedFromBaseline:              voiceRobustnessImproved(baselineOutcome, outcome, baselineScore, adjustedScore),
		FirstAudioLatencyMs:               firstAudioLatencyMs,
		CompletionLatencyMs:               completionLatencyMs,
		LatencyBudgetMs:                   cfg.LatencyBudgetMs,
		EffectivePacketLossPct:            effectivePacketLoss,
		EffectiveBurstLossMs:              effectiveBurstLoss,
		PacketLossPenaltyReductionPct:     voiceRobustnessProtectionReductionPct(protectionPct),
		BurstLossJitterAbsorptionMs:       jitterAbsorption,
		ProtectionAssumption:              "proxy_reduces_packet_and_burst_loss_penalties_not_real_fec",
		DegradeReasons:                    degradeReasons,
		ResidualRisk:                      residualRisk,
	}, nil
}

func voiceRobustnessLatency(cfg VoiceRobustnessSweepConfig, burstMs int, protectionPct int, jitterMs int) (int64, int64, error) {
	profile := VoiceQualityProfile{
		ProfileID:                  "voice_robustness_latency_proxy",
		CodecClass:                 cfg.CodecClass,
		QualityTarget:              VoiceQualityTargetFMLike,
		CodecBitrateBps:            12000,
		AudioBandwidthHz:           cfg.AudioBandwidthHz,
		ChannelBitrateBps:          cfg.ChannelBitrateBps,
		BurstDurationMs:            int64(burstMs),
		FramePayloadBytes:          cfg.FramePayloadBytes,
		WireHeaderBytes:            cfg.WireHeaderBytes,
		ProtectionOverheadPermille: int64(protectionPct * 10),
		QualityScore:               82,
		FMLikeProxy:                true,
	}
	_, protectedBytes, err := voiceQualityPayloadBytes(profile)
	if err != nil {
		return 0, 0, err
	}
	frameCount, err := voiceQualityFrameCount(protectedBytes, profile.FramePayloadBytes)
	if err != nil {
		return 0, 0, err
	}
	maxWireFrameBytes := profile.FramePayloadBytes + profile.WireHeaderBytes
	measurement, err := measureVoiceQualityFrame(maxWireFrameBytes, profile.ChannelBitrateBps, profile.BurstDurationMs)
	if err != nil {
		return 0, 0, err
	}
	frameTxDuration, err := checkedLimitedMultiHopAdd(measurement, cfg.PreambleMs)
	if err != nil {
		return 0, 0, err
	}
	frameTxDuration, err = checkedLimitedMultiHopAdd(frameTxDuration, cfg.SyncMs)
	if err != nil {
		return 0, 0, err
	}
	burstTxDuration, err := delayedRelayVoiceBurstTxDuration(frameCount, frameTxDuration, cfg.InterFrameGapMs)
	if err != nil {
		return 0, 0, err
	}
	sourceAvailable := int64(burstMs)
	if burstTxDuration > sourceAvailable {
		sourceAvailable = burstTxDuration
	}
	sourceAvailable, err = checkedLimitedMultiHopAdd(sourceAvailable, cfg.LinkDelayMs)
	if err != nil {
		return 0, 0, err
	}
	firstAudio, completion, err := delayedRelayVoiceRelayTimeline(sourceAvailable, cfg.RelayDepth, cfg.TurnaroundMs, burstTxDuration, frameTxDuration, cfg.LinkDelayMs)
	if err != nil {
		return 0, 0, err
	}
	firstAudio, err = checkedLimitedMultiHopAdd(firstAudio, int64(jitterMs))
	if err != nil {
		return 0, 0, err
	}
	completion, err = checkedLimitedMultiHopAdd(completion, int64(jitterMs))
	if err != nil {
		return 0, 0, err
	}
	return firstAudio, completion, nil
}

func measureVoiceQualityFrame(frameBytes int, bitrateBps int64, slotMs int64) (int64, error) {
	measurement, err := channel.MeasureFrameBytes(frameBytes, channel.AirtimeConfig{
		ChannelBitrateBps: bitrateBps,
		SlotDurationMs:    slotMs,
	})
	if err != nil {
		return 0, err
	}
	return measurement.AirtimeMs, nil
}
