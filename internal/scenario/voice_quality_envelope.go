package scenario

import (
	"fmt"

	"github.com/rmsff/ad-hoc-radio/internal/channel"
)

const VoiceQualityEnvelopeScenarioID = "voice_quality_envelope_simulation"

const (
	VoiceQualityOutcomePass     = "fm_like_pass"
	VoiceQualityOutcomeDegraded = "degraded"
	VoiceQualityOutcomeFail     = "fail"
)

const (
	VoiceQualityTargetCommunications = "communications_quality"
	VoiceQualityTargetFMLike         = "fm_like_proxy"
)

const (
	voiceQualityReasonNotFMLikeProxy = "not_fm_like_quality_proxy"
	voiceQualityReasonLatencyBudget  = "quality_latency_budget_exceeded"
	voiceQualityReasonDrainSlower    = "burst_drain_slower_than_speech"
	voiceQualityMaxRows              = 64
)

type VoiceQualityEnvelopeConfig struct {
	ScenarioID             string
	Seed                   uint64
	TurnaroundMs           int64
	LinkDelayMs            int64
	PreambleMs             int64
	SyncMs                 int64
	InterFrameGapMs        int64
	RelayDepthCandidates   []int
	LatencyBudgetMs        int64
	LiveTurnaroundBudgetMs int64
	Profiles               []VoiceQualityProfile
}

type VoiceQualityProfile struct {
	ProfileID                  string
	CodecClass                 string
	QualityTarget              string
	CodecBitrateBps            int64
	AudioBandwidthHz           int
	ChannelBitrateBps          int64
	BurstDurationMs            int64
	FramePayloadBytes          int
	WireHeaderBytes            int
	ProtectionOverheadPermille int64
	QualityScore               int
	FMLikeProxy                bool
	Notes                      string
}

type VoiceQualityEnvelopeReport struct {
	ScenarioID                 string                    `json:"scenario_id"`
	Seed                       uint64                    `json:"seed"`
	TurnaroundMs               int64                     `json:"turnaround_ms"`
	LatencyBudgetMs            int64                     `json:"latency_budget_ms"`
	LiveTurnaroundBudgetMs     int64                     `json:"live_turnaround_budget_ms"`
	LiveRelayViable            bool                      `json:"live_relay_viable"`
	AssumptionOnly             bool                      `json:"assumption_only"`
	Profiles                   []VoiceQualityProfile     `json:"profiles"`
	Rows                       []VoiceQualityEnvelopeRow `json:"rows"`
	RowCount                   int                       `json:"row_count"`
	FMLikeRows                 int                       `json:"fm_like_rows"`
	FMLikePassRows             int                       `json:"fm_like_pass_rows"`
	FMLikeFailRows             int                       `json:"fm_like_fail_rows"`
	BestFMLikeTwoRelayRow      VoiceQualityEnvelopeRow   `json:"best_fm_like_two_relay_row"`
	SelectedGoodQualityProfile string                    `json:"selected_good_quality_profile"`
	GoodQualitySupported       bool                      `json:"good_quality_supported"`
	EventHash                  string                    `json:"event_hash"`
}

type VoiceQualityEnvelopeRow struct {
	RowID                      string              `json:"row_id"`
	ProfileID                  string              `json:"profile_id"`
	CodecClass                 string              `json:"codec_class"`
	QualityTarget              string              `json:"quality_target"`
	CodecBitrateBps            int64               `json:"codec_bitrate_bps"`
	AudioBandwidthHz           int                 `json:"audio_bandwidth_hz"`
	ChannelBitrateBps          int64               `json:"channel_bitrate_bps"`
	BurstDurationMs            int64               `json:"burst_duration_ms"`
	RelayDepth                 int                 `json:"relay_depth"`
	FramePayloadBytes          int                 `json:"frame_payload_bytes"`
	WireHeaderBytes            int                 `json:"wire_header_bytes"`
	MaxWireFrameBytes          int                 `json:"max_wire_frame_bytes"`
	ProtectionOverheadPermille int64               `json:"protection_overhead_permille"`
	EncodedVoiceBytes          int64               `json:"encoded_voice_bytes"`
	ProtectedVoiceBytes        int64               `json:"protected_voice_bytes"`
	FrameCount                 int                 `json:"frame_count"`
	AirtimeMs                  int64               `json:"airtime_ms"`
	FrameTxDurationMs          int64               `json:"frame_tx_duration_ms"`
	BurstTxDurationMs          int64               `json:"burst_tx_duration_ms"`
	SourceAvailableAtMs        int64               `json:"source_available_at_ms"`
	FirstAudioLatencyMs        int64               `json:"first_audio_latency_ms"`
	CompletionMs               int64               `json:"completion_ms"`
	LastFrameLatencyMs         int64               `json:"last_frame_latency_ms"`
	MaxUserPerceivedLatencyMs  int64               `json:"max_user_perceived_latency_ms"`
	DrainRatioPermille         int64               `json:"drain_ratio_permille"`
	BurstDrainFasterThanSpeech bool                `json:"burst_drain_faster_than_speech"`
	LiveRelayViable            bool                `json:"live_relay_viable"`
	StoreForwardRequired       bool                `json:"store_forward_required"`
	QualityScore               int                 `json:"quality_score"`
	FMLikeProxy                bool                `json:"fm_like_proxy"`
	Outcome                    string              `json:"outcome"`
	FailReasons                []string            `json:"fail_reasons"`
	Measurement                channel.Measurement `json:"measurement"`
	MeasuredHardwareCapability bool                `json:"measured_hardware_capability"`
	Notes                      string              `json:"notes"`
}

func DefaultVoiceQualityEnvelopeConfig() VoiceQualityEnvelopeConfig {
	return VoiceQualityEnvelopeConfig{
		ScenarioID:             VoiceQualityEnvelopeScenarioID,
		Seed:                   DefaultScenarioSeed,
		TurnaroundMs:           200,
		LinkDelayMs:            1,
		PreambleMs:             1,
		SyncMs:                 1,
		InterFrameGapMs:        1,
		RelayDepthCandidates:   []int{1, 2, 3},
		LatencyBudgetMs:        2000,
		LiveTurnaroundBudgetMs: 10,
		Profiles: []VoiceQualityProfile{
			{
				ProfileID:                  "codec2_3200_reference",
				CodecClass:                 "codec2_class_reference",
				QualityTarget:              VoiceQualityTargetCommunications,
				CodecBitrateBps:            3200,
				AudioBandwidthHz:           3400,
				ChannelBitrateBps:          19200,
				BurstDurationMs:            500,
				FramePayloadBytes:          32,
				WireHeaderBytes:            14,
				ProtectionOverheadPermille: 200,
				QualityScore:               55,
				FMLikeProxy:                false,
				Notes:                      "low-bitrate communications-quality reference, not the good-voice target",
			},
			{
				ProfileID:                  "fm_like_wideband_12k_primary",
				CodecClass:                 "opus_class_wideband_speech_proxy",
				QualityTarget:              VoiceQualityTargetFMLike,
				CodecBitrateBps:            12000,
				AudioBandwidthHz:           8000,
				ChannelBitrateBps:          19200,
				BurstDurationMs:            500,
				FramePayloadBytes:          96,
				WireHeaderBytes:            14,
				ProtectionOverheadPermille: 150,
				QualityScore:               82,
				FMLikeProxy:                true,
				Notes:                      "primary good-voice proxy: 12 kbps wideband speech plus 15 percent protection budget",
			},
			{
				ProfileID:                  "fm_like_wideband_12k_robust_1s",
				CodecClass:                 "opus_class_wideband_speech_proxy",
				QualityTarget:              VoiceQualityTargetFMLike,
				CodecBitrateBps:            12000,
				AudioBandwidthHz:           8000,
				ChannelBitrateBps:          19200,
				BurstDurationMs:            1000,
				FramePayloadBytes:          96,
				WireHeaderBytes:            14,
				ProtectionOverheadPermille: 150,
				QualityScore:               82,
				FMLikeProxy:                true,
				Notes:                      "same quality target with a longer burst for loss/FEC experiments",
			},
			{
				ProfileID:                  "fm_like_high_16k_large_packet",
				CodecClass:                 "opus_class_high_speech_proxy",
				QualityTarget:              VoiceQualityTargetFMLike,
				CodecBitrateBps:            16000,
				AudioBandwidthHz:           8000,
				ChannelBitrateBps:          19200,
				BurstDurationMs:            500,
				FramePayloadBytes:          256,
				WireHeaderBytes:            14,
				ProtectionOverheadPermille: 50,
				QualityScore:               88,
				FMLikeProxy:                true,
				Notes:                      "higher-bitrate speech proxy; relies on larger packets to keep burst drain under speech duration",
			},
		},
	}
}

func RunVoiceQualityEnvelopeSimulation(cfg VoiceQualityEnvelopeConfig) (VoiceQualityEnvelopeReport, error) {
	cfg = normalizeVoiceQualityEnvelopeConfig(cfg)
	if err := validateVoiceQualityEnvelopeConfig(cfg); err != nil {
		return VoiceQualityEnvelopeReport{}, err
	}
	report := VoiceQualityEnvelopeReport{
		ScenarioID:             cfg.ScenarioID,
		Seed:                   cfg.Seed,
		TurnaroundMs:           cfg.TurnaroundMs,
		LatencyBudgetMs:        cfg.LatencyBudgetMs,
		LiveTurnaroundBudgetMs: cfg.LiveTurnaroundBudgetMs,
		LiveRelayViable:        cfg.TurnaroundMs <= cfg.LiveTurnaroundBudgetMs,
		AssumptionOnly:         true,
		Profiles:               cloneVoiceQualityProfiles(cfg.Profiles),
		Rows:                   make([]VoiceQualityEnvelopeRow, 0, voiceQualityEnvelopeRowCount(cfg)),
	}

	for _, profile := range cfg.Profiles {
		for _, relayDepth := range cfg.RelayDepthCandidates {
			row, err := voiceQualityEnvelopeRow(cfg, profile, relayDepth)
			if err != nil {
				return VoiceQualityEnvelopeReport{}, err
			}
			report.Rows = append(report.Rows, row)
			if row.FMLikeProxy {
				report.FMLikeRows++
				if row.Outcome == VoiceQualityOutcomePass {
					report.FMLikePassRows++
				}
				if row.Outcome == VoiceQualityOutcomeFail {
					report.FMLikeFailRows++
				}
			}
		}
	}
	report.RowCount = len(report.Rows)
	report.BestFMLikeTwoRelayRow = bestVoiceQualityEnvelopeRow(report.Rows, 2)
	report.SelectedGoodQualityProfile = report.BestFMLikeTwoRelayRow.ProfileID
	report.GoodQualitySupported = report.BestFMLikeTwoRelayRow.Outcome == VoiceQualityOutcomePass

	hash, err := voiceQualityEnvelopeHash(report)
	if err != nil {
		return VoiceQualityEnvelopeReport{}, err
	}
	report.EventHash = hash
	return report, nil
}

func voiceQualityEnvelopeRow(cfg VoiceQualityEnvelopeConfig, profile VoiceQualityProfile, relayDepth int) (VoiceQualityEnvelopeRow, error) {
	encodedBytes, protectedBytes, err := voiceQualityPayloadBytes(profile)
	if err != nil {
		return VoiceQualityEnvelopeRow{}, err
	}
	frameCount, err := voiceQualityFrameCount(protectedBytes, profile.FramePayloadBytes)
	if err != nil {
		return VoiceQualityEnvelopeRow{}, err
	}
	maxWireFrameBytes := profile.FramePayloadBytes + profile.WireHeaderBytes
	measurement, err := channel.MeasureFrameBytes(maxWireFrameBytes, channel.AirtimeConfig{
		ChannelBitrateBps: profile.ChannelBitrateBps,
		SlotDurationMs:    profile.BurstDurationMs,
	})
	if err != nil {
		return VoiceQualityEnvelopeRow{}, err
	}
	frameTxDuration, err := checkedLimitedMultiHopAdd(measurement.AirtimeMs, cfg.PreambleMs)
	if err != nil {
		return VoiceQualityEnvelopeRow{}, err
	}
	frameTxDuration, err = checkedLimitedMultiHopAdd(frameTxDuration, cfg.SyncMs)
	if err != nil {
		return VoiceQualityEnvelopeRow{}, err
	}
	burstTxDuration, err := delayedRelayVoiceBurstTxDuration(frameCount, frameTxDuration, cfg.InterFrameGapMs)
	if err != nil {
		return VoiceQualityEnvelopeRow{}, err
	}
	sourceAvailable := profile.BurstDurationMs
	if burstTxDuration > sourceAvailable {
		sourceAvailable = burstTxDuration
	}
	sourceAvailable, err = checkedLimitedMultiHopAdd(sourceAvailable, cfg.LinkDelayMs)
	if err != nil {
		return VoiceQualityEnvelopeRow{}, err
	}
	firstAudioLatency, completion, err := delayedRelayVoiceRelayTimeline(sourceAvailable, relayDepth, cfg.TurnaroundMs, burstTxDuration, frameTxDuration, cfg.LinkDelayMs)
	if err != nil {
		return VoiceQualityEnvelopeRow{}, err
	}
	lastFrameLatency := completion - profile.BurstDurationMs
	maxLatency := firstAudioLatency
	if lastFrameLatency > maxLatency {
		maxLatency = lastFrameLatency
	}

	row := VoiceQualityEnvelopeRow{
		RowID:                      voiceQualityEnvelopeRowID(profile.ProfileID, relayDepth),
		ProfileID:                  profile.ProfileID,
		CodecClass:                 profile.CodecClass,
		QualityTarget:              profile.QualityTarget,
		CodecBitrateBps:            profile.CodecBitrateBps,
		AudioBandwidthHz:           profile.AudioBandwidthHz,
		ChannelBitrateBps:          profile.ChannelBitrateBps,
		BurstDurationMs:            profile.BurstDurationMs,
		RelayDepth:                 relayDepth,
		FramePayloadBytes:          profile.FramePayloadBytes,
		WireHeaderBytes:            profile.WireHeaderBytes,
		MaxWireFrameBytes:          maxWireFrameBytes,
		ProtectionOverheadPermille: profile.ProtectionOverheadPermille,
		EncodedVoiceBytes:          encodedBytes,
		ProtectedVoiceBytes:        protectedBytes,
		FrameCount:                 frameCount,
		AirtimeMs:                  measurement.AirtimeMs,
		FrameTxDurationMs:          frameTxDuration,
		BurstTxDurationMs:          burstTxDuration,
		SourceAvailableAtMs:        sourceAvailable,
		FirstAudioLatencyMs:        firstAudioLatency,
		CompletionMs:               completion,
		LastFrameLatencyMs:         lastFrameLatency,
		MaxUserPerceivedLatencyMs:  maxLatency,
		DrainRatioPermille:         voiceQualityPermille(burstTxDuration, profile.BurstDurationMs),
		BurstDrainFasterThanSpeech: burstTxDuration <= profile.BurstDurationMs,
		LiveRelayViable:            cfg.TurnaroundMs <= cfg.LiveTurnaroundBudgetMs,
		StoreForwardRequired:       cfg.TurnaroundMs > cfg.LiveTurnaroundBudgetMs,
		QualityScore:               profile.QualityScore,
		FMLikeProxy:                profile.FMLikeProxy,
		Measurement:                measurement,
		MeasuredHardwareCapability: false,
		Notes:                      profile.Notes,
	}
	row.FailReasons = voiceQualityFailReasons(cfg, row)
	row.Outcome = voiceQualityOutcome(row.FailReasons)
	return row, nil
}

func voiceQualityEnvelopeRowID(profileID string, relayDepth int) string {
	return fmt.Sprintf("%s_relays_%d", profileID, relayDepth)
}
