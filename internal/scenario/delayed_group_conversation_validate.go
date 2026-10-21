package scenario

import (
	"fmt"
	"strings"
)

func validateDelayedGroupConversationConfig(cfg DelayedGroupConversationConfig) error {
	if cfg.ScenarioID != DelayedGroupConversationScenarioID {
		return fmt.Errorf("delayed group conversation: unsupported scenario_id %q", cfg.ScenarioID)
	}
	if err := validateDelayedGroupConversationIDs("speaker_ids", cfg.SpeakerIDs, 3); err != nil {
		return err
	}
	if err := validateDelayedGroupConversationTurns(cfg); err != nil {
		return err
	}
	if err := validateDelayedGroupConversationIDs("direct_receiver_ids", cfg.DirectReceiverIDs, 1); err != nil {
		return err
	}
	if err := validateDelayedGroupConversationIDs("one_hop_receiver_ids", cfg.OneHopReceiverIDs, 1); err != nil {
		return err
	}
	if err := validateDelayedGroupConversationIDs("two_hop_receiver_ids", cfg.TwoHopReceiverIDs, 1); err != nil {
		return err
	}
	if err := validateDelayedGroupConversationReceiverUniqueness(cfg); err != nil {
		return err
	}
	if len(cfg.Profiles) == 0 || len(cfg.Profiles) > delayedGroupConversationMaxProfiles {
		return fmt.Errorf("delayed group conversation: profile count must be in [1,%d]", delayedGroupConversationMaxProfiles)
	}
	if err := validateDelayedGroupConversationProfiles(cfg.Profiles); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.CodecClass) == "" {
		return fmt.Errorf("delayed group conversation: codec_class must not be empty")
	}
	if cfg.AudioBandwidthHz <= 0 || cfg.ChannelBitrateBps <= 0 || cfg.FramePayloadBytes <= 0 || cfg.WireHeaderBytes <= 0 {
		return fmt.Errorf("delayed group conversation: audio/channel/frame values must be > 0")
	}
	if cfg.TurnaroundMs < 0 || cfg.LinkDelayMs < 0 || cfg.PreambleMs < 0 || cfg.SyncMs < 0 || cfg.InterFrameGapMs < 0 {
		return fmt.Errorf("delayed group conversation: timing values must be >= 0")
	}
	if cfg.MaxFirstAudioLatencyMs <= 0 || cfg.MaxCompletionMs <= 0 {
		return fmt.Errorf("delayed group conversation: latency caps must be > 0")
	}
	return nil
}

func validateDelayedGroupConversationIDs(name string, ids []uint16, minCount int) error {
	if len(ids) < minCount {
		return fmt.Errorf("delayed group conversation: %s must contain at least %d ids", name, minCount)
	}
	seen := make(map[uint16]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			return fmt.Errorf("delayed group conversation: %s contains zero id", name)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("delayed group conversation: %s contains duplicate id %d", name, id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func validateDelayedGroupConversationTurns(cfg DelayedGroupConversationConfig) error {
	if len(cfg.TalkTurns) < 5 || len(cfg.TalkTurns) > delayedGroupConversationMaxTurns {
		return fmt.Errorf("delayed group conversation: talk turn count must be in [5,%d]", delayedGroupConversationMaxTurns)
	}
	speakers := make(map[uint16]struct{}, len(cfg.SpeakerIDs))
	for _, speakerID := range cfg.SpeakerIDs {
		speakers[speakerID] = struct{}{}
	}
	seen := make(map[string]struct{}, len(cfg.TalkTurns))
	usedSpeakers := make(map[uint16]struct{}, len(cfg.SpeakerIDs))
	var previousStart int64 = -1
	for i, turn := range cfg.TalkTurns {
		if strings.TrimSpace(turn.TurnID) == "" {
			return fmt.Errorf("delayed group conversation: talk_turns[%d].turn_id must not be empty", i)
		}
		if _, ok := seen[turn.TurnID]; ok {
			return fmt.Errorf("delayed group conversation: duplicate turn_id %q", turn.TurnID)
		}
		seen[turn.TurnID] = struct{}{}
		if _, ok := speakers[turn.SpeakerID]; !ok {
			return fmt.Errorf("delayed group conversation: talk_turns[%d].speaker_id=%d is not in speaker_ids", i, turn.SpeakerID)
		}
		usedSpeakers[turn.SpeakerID] = struct{}{}
		if turn.StartMs < 0 || turn.StartMs <= previousStart {
			return fmt.Errorf("delayed group conversation: talk_turns must have strictly increasing non-negative start_ms")
		}
		previousStart = turn.StartMs
	}
	if len(usedSpeakers) < 3 {
		return fmt.Errorf("delayed group conversation: talk turns must include at least 3 distinct speakers")
	}
	for _, speakerID := range cfg.SpeakerIDs {
		if _, ok := usedSpeakers[speakerID]; !ok {
			return fmt.Errorf("delayed group conversation: speaker_id %d is declared but never used in talk_turns", speakerID)
		}
	}
	return nil
}

func validateDelayedGroupConversationReceiverUniqueness(cfg DelayedGroupConversationConfig) error {
	seen := make(map[uint16]string)
	groups := []DelayedGroupConversationGroupConfig{
		{DepthLabel: "direct", ReceiverIDs: cfg.DirectReceiverIDs},
		{DepthLabel: "one_hop", ReceiverIDs: cfg.OneHopReceiverIDs},
		{DepthLabel: "two_hop", ReceiverIDs: cfg.TwoHopReceiverIDs},
		{DepthLabel: "uncovered", ReceiverIDs: cfg.UncoveredReceiverIDs},
	}
	for _, group := range groups {
		for _, id := range group.ReceiverIDs {
			if id == 0 {
				return fmt.Errorf("delayed group conversation: %s receiver id must not be zero", group.DepthLabel)
			}
			if previous, ok := seen[id]; ok {
				return fmt.Errorf("delayed group conversation: receiver id %d appears in both %s and %s", id, previous, group.DepthLabel)
			}
			seen[id] = group.DepthLabel
		}
	}
	return nil
}

func validateDelayedGroupConversationProfiles(profiles []DelayedGroupConversationProfile) error {
	seen := make(map[string]struct{}, len(profiles))
	for _, profile := range profiles {
		if strings.TrimSpace(profile.ProfileID) == "" || strings.TrimSpace(profile.ProfileClass) == "" {
			return fmt.Errorf("delayed group conversation: profile id and class must not be empty")
		}
		switch profile.ProfileClass {
		case VoiceRobustnessProfileNormal, VoiceRobustnessProfileProtected, VoiceRobustnessProfileRobustDelayed:
		default:
			return fmt.Errorf("delayed group conversation: profile %q has unsupported class %q", profile.ProfileID, profile.ProfileClass)
		}
		if _, ok := seen[profile.ProfileID]; ok {
			return fmt.Errorf("delayed group conversation: duplicate profile_id %q", profile.ProfileID)
		}
		seen[profile.ProfileID] = struct{}{}
		if profile.BurstMs <= 0 {
			return fmt.Errorf("delayed group conversation: profile %q burst_ms must be > 0", profile.ProfileID)
		}
		if profile.ProtectionOverheadPct < 0 || profile.ProtectionOverheadPct > 100 {
			return fmt.Errorf("delayed group conversation: profile %q protection_overhead_pct must be in [0,100]", profile.ProfileID)
		}
		if profile.JitterBufferMs < 0 || profile.JitterBufferMs > 5000 {
			return fmt.Errorf("delayed group conversation: profile %q jitter_buffer_ms must be in [0,5000]", profile.ProfileID)
		}
	}
	return nil
}
