package scenario

import "fmt"

const DelayedGroupConversationScenarioID = "delayed_group_conversation"
const DelayedGroupConversationStoryID = "MVP-8.9"
const DelayedGroupConversationEvidenceType = "delayed_group_conversation"

const (
	DelayedGroupConversationOutcomePass     = "conversation_pass"
	DelayedGroupConversationOutcomeDegraded = "conversation_degraded"
	DelayedGroupConversationOutcomeFail     = "fail"
)

const (
	delayedGroupConversationMaxProfiles          = 16
	delayedGroupConversationMaxTurns             = 64
	delayedGroupConversationDefaultFirstLatency  = 2000
	delayedGroupConversationDefaultCompletionMax = 2000
)

type DelayedGroupConversationConfig struct {
	ScenarioID             string
	Seed                   uint64
	SpeakerIDs             []uint16
	TalkTurns              []DelayedGroupConversationTurnConfig
	DirectReceiverIDs      []uint16
	OneHopReceiverIDs      []uint16
	TwoHopReceiverIDs      []uint16
	UncoveredReceiverIDs   []uint16
	Profiles               []DelayedGroupConversationProfile
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
	MaxFirstAudioLatencyMs int64
	MaxCompletionMs        int64
}

type DelayedGroupConversationTurnConfig struct {
	TurnID    string `json:"turn_id"`
	SpeakerID uint16 `json:"speaker_id"`
	StartMs   int64  `json:"start_ms"`
}

type DelayedGroupConversationProfile struct {
	ProfileID             string `json:"profile_id"`
	ProfileClass          string `json:"profile_class"`
	BurstMs               int    `json:"burst_ms"`
	ProtectionOverheadPct int    `json:"protection_overhead_pct"`
	JitterBufferMs        int    `json:"jitter_buffer_ms"`
}

type DelayedGroupConversationReport struct {
	ScenarioID          string                                `json:"scenario_id"`
	Seed                uint64                                `json:"seed"`
	Evidence            ScenarioEvidence                      `json:"evidence"`
	Profiles            []DelayedGroupConversationProfile     `json:"profiles"`
	Rows                []DelayedGroupConversationProfileRow  `json:"rows"`
	SpeakerIDs          []uint16                              `json:"speaker_ids"`
	Turns               []DelayedGroupConversationTurnConfig  `json:"turns"`
	ExpectedReceiverIDs []uint16                              `json:"expected_receiver_ids"`
	ReceiverGroups      []DelayedGroupConversationGroupConfig `json:"receiver_groups"`
	RowCount            int                                   `json:"row_count"`
	PassRows            int                                   `json:"pass_rows"`
	DegradedRows        int                                   `json:"degraded_rows"`
	FailRows            int                                   `json:"fail_rows"`
	Outcome             string                                `json:"outcome"`
	DegradeReasons      []string                              `json:"degrade_reasons"`
	EventHash           string                                `json:"event_hash"`
}

type DelayedGroupConversationGroupConfig struct {
	DepthLabel  string   `json:"depth_label"`
	RelayDepth  int      `json:"relay_depth"`
	ReceiverIDs []uint16 `json:"receiver_ids"`
}

type DelayedGroupConversationProfileRow struct {
	ProfileID                     string                                  `json:"profile_id"`
	ProfileClass                  string                                  `json:"profile_class"`
	BurstMs                       int                                     `json:"burst_ms"`
	ProtectionOverheadPct         int                                     `json:"protection_overhead_pct"`
	JitterBufferMs                int                                     `json:"jitter_buffer_ms"`
	TurnCount                     int                                     `json:"turn_count"`
	SpeakerCount                  int                                     `json:"speaker_count"`
	ReceiverGroups                []DelayedGroupConversationReceiverGroup `json:"receiver_groups"`
	MaxFirstAudioLatencyMs        int64                                   `json:"max_first_audio_latency_ms"`
	MaxCompletionLatencyMs        int64                                   `json:"max_completion_latency_ms"`
	StalePlayoutCount             int                                     `json:"stale_playout_count"`
	OverlapCount                  int                                     `json:"overlap_count"`
	MissingUncoveredReceiverCount int                                     `json:"missing_uncovered_receiver_count"`
	AccountedReceiverCount        int                                     `json:"accounted_receiver_count"`
	ExpectedReceiverCount         int                                     `json:"expected_receiver_count"`
	Outcome                       string                                  `json:"outcome"`
	DegradeReasons                []string                                `json:"degrade_reasons"`
	TurnReports                   []DelayedGroupConversationTurnReport    `json:"turn_reports"`
}

type DelayedGroupConversationReceiverGroup struct {
	DepthLabel                    string   `json:"depth_label"`
	RelayDepth                    int      `json:"relay_depth"`
	ReceiverIDs                   []uint16 `json:"receiver_ids"`
	ReceiverCount                 int      `json:"receiver_count"`
	FirstAudioLatencyMs           int64    `json:"first_audio_latency_ms"`
	CompletionLatencyMs           int64    `json:"completion_latency_ms"`
	StalePlayoutCount             int      `json:"stale_playout_count"`
	OverlapCount                  int      `json:"overlap_count"`
	MissingUncoveredReceiverCount int      `json:"missing_uncovered_receiver_count"`
	AccountedReceiverCount        int      `json:"accounted_receiver_count"`
}

type DelayedGroupConversationTurnReport struct {
	TurnID         string                                  `json:"turn_id"`
	SpeakerID      uint16                                  `json:"speaker_id"`
	StartMs        int64                                   `json:"start_ms"`
	BurstMs        int64                                   `json:"burst_ms"`
	NextTurnStart  int64                                   `json:"next_turn_start_ms"`
	ReceiverGroups []DelayedGroupConversationReceiverGroup `json:"receiver_groups"`
}

type delayedGroupConversationLatency struct {
	FirstAudioMs int64
	CompletionMs int64
}

func DefaultDelayedGroupConversationConfig() DelayedGroupConversationConfig {
	robustness := DefaultVoiceRobustnessSweepConfig()
	return DelayedGroupConversationConfig{
		ScenarioID:             DelayedGroupConversationScenarioID,
		Seed:                   DefaultScenarioSeed,
		SpeakerIDs:             []uint16{1, 11, 12},
		TalkTurns:              defaultDelayedGroupConversationTurns(),
		DirectReceiverIDs:      []uint16{8, 9, 10},
		OneHopReceiverIDs:      []uint16{6, 7},
		TwoHopReceiverIDs:      []uint16{2, 3, 4, 5},
		Profiles:               defaultDelayedGroupConversationProfiles(),
		CodecClass:             robustness.CodecClass,
		AudioBandwidthHz:       robustness.AudioBandwidthHz,
		ChannelBitrateBps:      robustness.ChannelBitrateBps,
		FramePayloadBytes:      robustness.FramePayloadBytes,
		WireHeaderBytes:        robustness.WireHeaderBytes,
		TurnaroundMs:           robustness.TurnaroundMs,
		LinkDelayMs:            robustness.LinkDelayMs,
		PreambleMs:             robustness.PreambleMs,
		SyncMs:                 robustness.SyncMs,
		InterFrameGapMs:        robustness.InterFrameGapMs,
		MaxFirstAudioLatencyMs: delayedGroupConversationDefaultFirstLatency,
		MaxCompletionMs:        delayedGroupConversationDefaultCompletionMax,
	}
}

func RunDelayedGroupConversationScenario(cfg DelayedGroupConversationConfig) (DelayedGroupConversationReport, error) {
	cfg = normalizeDelayedGroupConversationConfig(cfg)
	if err := validateDelayedGroupConversationConfig(cfg); err != nil {
		return DelayedGroupConversationReport{}, err
	}
	report := DelayedGroupConversationReport{
		ScenarioID:          cfg.ScenarioID,
		Seed:                cfg.Seed,
		Profiles:            cloneDelayedGroupConversationProfiles(cfg.Profiles),
		Rows:                make([]DelayedGroupConversationProfileRow, 0, len(cfg.Profiles)),
		SpeakerIDs:          cloneDelayedGroupConversationUint16s(cfg.SpeakerIDs),
		Turns:               cloneDelayedGroupConversationTurns(cfg.TalkTurns),
		ExpectedReceiverIDs: delayedGroupConversationExpectedReceivers(cfg),
		ReceiverGroups:      delayedGroupConversationGroupConfigs(cfg),
	}
	for _, profile := range cfg.Profiles {
		row, err := delayedGroupConversationProfileRow(cfg, profile, report.ExpectedReceiverIDs)
		if err != nil {
			return DelayedGroupConversationReport{}, err
		}
		report.Rows = append(report.Rows, row)
		switch row.Outcome {
		case DelayedGroupConversationOutcomePass:
			report.PassRows++
		case DelayedGroupConversationOutcomeDegraded:
			report.DegradedRows++
		case DelayedGroupConversationOutcomeFail:
			report.FailRows++
		default:
			return DelayedGroupConversationReport{}, fmt.Errorf("delayed group conversation: unsupported outcome %q", row.Outcome)
		}
	}
	report.RowCount = len(report.Rows)
	report.Outcome, report.DegradeReasons = delayedGroupConversationReportOutcome(report)
	report.Evidence = delayedGroupConversationEvidence(cfg, report, "")
	hash, err := delayedGroupConversationHash(report)
	if err != nil {
		return DelayedGroupConversationReport{}, err
	}
	report.EventHash = hash
	report.Evidence = delayedGroupConversationEvidence(cfg, report, hash)
	if err := ValidateScenarioEvidence(report.Evidence); err != nil {
		return DelayedGroupConversationReport{}, err
	}
	return report, nil
}
