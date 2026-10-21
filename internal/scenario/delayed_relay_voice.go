package scenario

import (
	"fmt"

	"github.com/rmsff/ad-hoc-radio/internal/channel"
)

const DelayedRelayVoiceScenarioID = "delayed_relay_voice_turnaround_simulation"

const (
	DelayedRelayVoiceOutcomePass     = "delayed_pass"
	DelayedRelayVoiceOutcomeDegraded = "delayed_degraded"
	DelayedRelayVoiceOutcomeFail     = "fail"
)

const (
	delayedRelayVoiceMaxRows                 = 128
	delayedRelayVoiceReasonLatencyBudget     = "latency_budget_exceeded"
	delayedRelayVoiceReasonDrainSlower       = "burst_drain_slower_than_speech"
	delayedRelayVoiceReasonNotLive           = "not_live_voice"
	delayedRelayVoiceReasonLiveTurnaroundBad = "live_two_hop_turnaround_too_slow"
)

type DelayedRelayVoiceConfig struct {
	ScenarioID                  string
	Seed                        uint64
	ChannelBitrateBpsCandidates []int64
	FrameBytesCandidates        []int
	BurstDurationMsCandidates   []int64
	RelayDepthCandidates        []int
	TurnaroundMs                int64
	PreambleMs                  int64
	SyncMs                      int64
	LinkDelayMs                 int64
	InterFrameGapMs             int64
	VoicePeriodMs               int64
	LatencyBudgetMs             int64
	LiveTurnaroundBudgetMs      int64
}

type DelayedRelayVoiceReport struct {
	ScenarioID             string                 `json:"scenario_id"`
	Seed                   uint64                 `json:"seed"`
	TurnaroundMs           int64                  `json:"turnaround_ms"`
	VoicePeriodMs          int64                  `json:"voice_period_ms"`
	LatencyBudgetMs        int64                  `json:"latency_budget_ms"`
	LiveTurnaroundBudgetMs int64                  `json:"live_turnaround_budget_ms"`
	LiveTwoHopViable       bool                   `json:"live_two_hop_viable"`
	AssumptionOnly         bool                   `json:"assumption_only"`
	Rows                   []DelayedRelayVoiceRow `json:"rows"`
	RowCount               int                    `json:"row_count"`
	PassRows               int                    `json:"pass_rows"`
	DegradedRows           int                    `json:"degraded_rows"`
	FailRows               int                    `json:"fail_rows"`
	BestTwoRelayRow        DelayedRelayVoiceRow   `json:"best_two_relay_row"`
	EventHash              string                 `json:"event_hash"`
}

type DelayedRelayVoiceRow struct {
	RowID                      string              `json:"row_id"`
	ChannelBitrateBps          int64               `json:"channel_bitrate_bps"`
	FrameBytes                 int                 `json:"frame_bytes"`
	BurstDurationMs            int64               `json:"burst_duration_ms"`
	FrameCount                 int                 `json:"frame_count"`
	RelayDepth                 int                 `json:"relay_depth"`
	TurnaroundMs               int64               `json:"turnaround_ms"`
	PreambleMs                 int64               `json:"preamble_ms"`
	SyncMs                     int64               `json:"sync_ms"`
	LinkDelayMs                int64               `json:"link_delay_ms"`
	InterFrameGapMs            int64               `json:"inter_frame_gap_ms"`
	VoicePeriodMs              int64               `json:"voice_period_ms"`
	AirtimeMs                  int64               `json:"airtime_ms"`
	FrameTxDurationMs          int64               `json:"frame_tx_duration_ms"`
	SourceTalkSpanMs           int64               `json:"source_talk_span_ms"`
	SourceCaptureCompleteMs    int64               `json:"source_capture_complete_ms"`
	BurstTxDurationMs          int64               `json:"burst_tx_duration_ms"`
	ChannelOccupiedMs          int64               `json:"channel_occupied_ms"`
	CompletionMs               int64               `json:"completion_ms"`
	FirstAudioLatencyMs        int64               `json:"first_audio_latency_ms"`
	LastFrameLatencyMs         int64               `json:"last_frame_latency_ms"`
	MaxUserPerceivedLatencyMs  int64               `json:"max_user_perceived_latency_ms"`
	BurstDrainFasterThanSpeech bool                `json:"burst_drain_faster_than_speech"`
	ChannelOccupancyPermille   int64               `json:"channel_occupancy_permille"`
	Outcome                    string              `json:"outcome"`
	FailReasons                []string            `json:"fail_reasons"`
	LiveTwoHopViable           bool                `json:"live_two_hop_viable"`
	StoreForwardRequired       bool                `json:"store_forward_required"`
	Measurement                channel.Measurement `json:"measurement"`
	MeasuredHardwareCapability bool                `json:"measured_hardware_capability"`
}

func DefaultDelayedRelayVoiceConfig() DelayedRelayVoiceConfig {
	return DelayedRelayVoiceConfig{
		ScenarioID:                  DelayedRelayVoiceScenarioID,
		Seed:                        DefaultScenarioSeed,
		ChannelBitrateBpsCandidates: []int64{9600, 19200},
		FrameBytesCandidates:        []int{22, 56},
		BurstDurationMsCandidates:   []int64{500, 1000, 2000},
		RelayDepthCandidates:        []int{1, 2, 3},
		TurnaroundMs:                200,
		PreambleMs:                  1,
		SyncMs:                      1,
		LinkDelayMs:                 1,
		InterFrameGapMs:             1,
		VoicePeriodMs:               DefaultFramePeriodMs,
		LatencyBudgetMs:             2000,
		LiveTurnaroundBudgetMs:      10,
	}
}

func RunDelayedRelayVoiceSimulation(cfg DelayedRelayVoiceConfig) (DelayedRelayVoiceReport, error) {
	cfg = normalizeDelayedRelayVoiceConfig(cfg)
	if err := validateDelayedRelayVoiceConfig(cfg); err != nil {
		return DelayedRelayVoiceReport{}, err
	}

	report := DelayedRelayVoiceReport{
		ScenarioID:             cfg.ScenarioID,
		Seed:                   cfg.Seed,
		TurnaroundMs:           cfg.TurnaroundMs,
		VoicePeriodMs:          cfg.VoicePeriodMs,
		LatencyBudgetMs:        cfg.LatencyBudgetMs,
		LiveTurnaroundBudgetMs: cfg.LiveTurnaroundBudgetMs,
		LiveTwoHopViable:       cfg.TurnaroundMs <= cfg.LiveTurnaroundBudgetMs,
		AssumptionOnly:         true,
		Rows:                   make([]DelayedRelayVoiceRow, 0, delayedRelayVoiceRowCount(cfg)),
	}

	for _, bitrate := range cfg.ChannelBitrateBpsCandidates {
		for _, frameBytes := range cfg.FrameBytesCandidates {
			for _, burstMs := range cfg.BurstDurationMsCandidates {
				for _, relayDepth := range cfg.RelayDepthCandidates {
					row, err := delayedRelayVoiceRow(cfg, bitrate, frameBytes, burstMs, relayDepth)
					if err != nil {
						return DelayedRelayVoiceReport{}, err
					}
					report.Rows = append(report.Rows, row)
					switch row.Outcome {
					case DelayedRelayVoiceOutcomePass:
						report.PassRows++
					case DelayedRelayVoiceOutcomeDegraded:
						report.DegradedRows++
					case DelayedRelayVoiceOutcomeFail:
						report.FailRows++
					default:
						return DelayedRelayVoiceReport{}, fmt.Errorf("delayed relay voice: unsupported outcome %q", row.Outcome)
					}
				}
			}
		}
	}
	report.RowCount = len(report.Rows)
	report.BestTwoRelayRow = bestDelayedRelayVoiceRow(report.Rows, 2)

	hash, err := delayedRelayVoiceHash(report)
	if err != nil {
		return DelayedRelayVoiceReport{}, err
	}
	report.EventHash = hash
	return report, nil
}

func delayedRelayVoiceRow(cfg DelayedRelayVoiceConfig, bitrate int64, frameBytes int, burstMs int64, relayDepth int) (DelayedRelayVoiceRow, error) {
	frameCount, err := delayedRelayVoiceFrameCount(burstMs, cfg.VoicePeriodMs)
	if err != nil {
		return DelayedRelayVoiceRow{}, err
	}
	measurement, err := channel.MeasureFrameBytes(frameBytes, channel.AirtimeConfig{
		ChannelBitrateBps: bitrate,
		SlotDurationMs:    burstMs,
	})
	if err != nil {
		return DelayedRelayVoiceRow{}, err
	}
	frameTxDuration, err := checkedLimitedMultiHopAdd(measurement.AirtimeMs, cfg.PreambleMs)
	if err != nil {
		return DelayedRelayVoiceRow{}, err
	}
	frameTxDuration, err = checkedLimitedMultiHopAdd(frameTxDuration, cfg.SyncMs)
	if err != nil {
		return DelayedRelayVoiceRow{}, err
	}
	sourceTalkSpan, err := checkedLimitedMultiHopMul(int64(frameCount-1), cfg.VoicePeriodMs)
	if err != nil {
		return DelayedRelayVoiceRow{}, err
	}
	sourceCaptureComplete, err := checkedLimitedMultiHopAdd(sourceTalkSpan, frameTxDuration)
	if err != nil {
		return DelayedRelayVoiceRow{}, err
	}
	sourceCaptureComplete, err = checkedLimitedMultiHopAdd(sourceCaptureComplete, cfg.LinkDelayMs)
	if err != nil {
		return DelayedRelayVoiceRow{}, err
	}
	burstTxDuration, err := delayedRelayVoiceBurstTxDuration(frameCount, frameTxDuration, cfg.InterFrameGapMs)
	if err != nil {
		return DelayedRelayVoiceRow{}, err
	}
	firstAudioLatency, completion, err := delayedRelayVoiceRelayTimeline(sourceCaptureComplete, relayDepth, cfg.TurnaroundMs, burstTxDuration, frameTxDuration, cfg.LinkDelayMs)
	if err != nil {
		return DelayedRelayVoiceRow{}, err
	}
	lastFrameLatency := completion - sourceTalkSpan
	maxLatency := firstAudioLatency
	if lastFrameLatency > maxLatency {
		maxLatency = lastFrameLatency
	}
	channelOccupied, err := delayedRelayVoiceChannelOccupiedMs(frameCount, frameTxDuration, relayDepth, burstTxDuration)
	if err != nil {
		return DelayedRelayVoiceRow{}, err
	}

	row := DelayedRelayVoiceRow{
		RowID:                      delayedRelayVoiceRowID(bitrate, frameBytes, burstMs, relayDepth),
		ChannelBitrateBps:          bitrate,
		FrameBytes:                 frameBytes,
		BurstDurationMs:            burstMs,
		FrameCount:                 frameCount,
		RelayDepth:                 relayDepth,
		TurnaroundMs:               cfg.TurnaroundMs,
		PreambleMs:                 cfg.PreambleMs,
		SyncMs:                     cfg.SyncMs,
		LinkDelayMs:                cfg.LinkDelayMs,
		InterFrameGapMs:            cfg.InterFrameGapMs,
		VoicePeriodMs:              cfg.VoicePeriodMs,
		AirtimeMs:                  measurement.AirtimeMs,
		FrameTxDurationMs:          frameTxDuration,
		SourceTalkSpanMs:           sourceTalkSpan,
		SourceCaptureCompleteMs:    sourceCaptureComplete,
		BurstTxDurationMs:          burstTxDuration,
		ChannelOccupiedMs:          channelOccupied,
		CompletionMs:               completion,
		FirstAudioLatencyMs:        firstAudioLatency,
		LastFrameLatencyMs:         lastFrameLatency,
		MaxUserPerceivedLatencyMs:  maxLatency,
		BurstDrainFasterThanSpeech: burstTxDuration <= burstMs,
		ChannelOccupancyPermille:   delayedRelayVoicePermille(channelOccupied, completion),
		LiveTwoHopViable:           cfg.TurnaroundMs <= cfg.LiveTurnaroundBudgetMs,
		StoreForwardRequired:       cfg.TurnaroundMs > cfg.LiveTurnaroundBudgetMs,
		Measurement:                measurement,
		MeasuredHardwareCapability: false,
	}
	row.FailReasons = delayedRelayVoiceFailReasons(cfg, row)
	row.Outcome = delayedRelayVoiceOutcome(row.FailReasons, row.StoreForwardRequired)
	return row, nil
}
