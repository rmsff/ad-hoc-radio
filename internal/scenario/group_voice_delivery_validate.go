package scenario

import "fmt"

func normalizeGroupVoiceDeliveryScenarioConfig(cfg GroupVoiceDeliveryScenarioConfig) GroupVoiceDeliveryScenarioConfig {
	def := DefaultGroupVoiceDeliveryScenarioConfig()
	if cfg.ScenarioID == "" {
		cfg.ScenarioID = def.ScenarioID
	}
	if cfg.Seed == 0 {
		cfg.Seed = def.Seed
	}
	if cfg.FrameCount == 0 {
		cfg.FrameCount = def.FrameCount
	}
	if cfg.StartMs == 0 {
		cfg.StartMs = def.StartMs
	}
	if cfg.NetID == 0 {
		cfg.NetID = def.NetID
	}
	if cfg.CallID == 0 {
		cfg.CallID = def.CallID
	}
	if cfg.TTL == 0 {
		cfg.TTL = def.TTL
	}
	if cfg.ReceiverJitterBufferMs == 0 {
		cfg.ReceiverJitterBufferMs = def.ReceiverJitterBufferMs
	}
	if cfg.ReceiverLateThresholdMs == 0 {
		cfg.ReceiverLateThresholdMs = def.ReceiverLateThresholdMs
	}
	if groupVoiceProfileConfigEmpty(cfg.ProfileConfig) {
		cfg.ProfileConfig = def.ProfileConfig
	} else {
		cfg.ProfileConfig = normalizeGroupVoiceProfileScenarioConfig(cfg.ProfileConfig)
	}
	return cfg
}

func groupVoiceProfileConfigEmpty(cfg GroupVoiceProfileScenarioConfig) bool {
	return cfg.ScenarioID == "" &&
		cfg.Seed == 0 &&
		cfg.SourceID == 0 &&
		cfg.GroupID == 0 &&
		cfg.CoverageMode == "" &&
		len(cfg.ExpectedMembers) == 0 &&
		cfg.RequiredGroupCoverage == 0 &&
		len(cfg.SourceDirectReachable) == 0 &&
		len(cfg.RelayCoverage) == 0 &&
		len(cfg.UnreachableMembers) == 0 &&
		len(cfg.Profiles) == 0
}

func validateGroupVoiceDeliveryScenarioConfig(cfg GroupVoiceDeliveryScenarioConfig) error {
	if cfg.ScenarioID == "" {
		return fmt.Errorf("group voice delivery scenario: scenario_id must not be empty")
	}
	if cfg.FrameCount <= 0 {
		return fmt.Errorf("group voice delivery scenario: frame_count=%d must be > 0", cfg.FrameCount)
	}
	if uint64(cfg.FrameCount) > uint64(^uint32(0)) {
		return fmt.Errorf("group voice delivery scenario: frame_count=%d must fit uint32 sequence space", cfg.FrameCount)
	}
	if cfg.StartMs < 0 {
		return fmt.Errorf("group voice delivery scenario: start_ms=%d must be >= 0", cfg.StartMs)
	}
	if cfg.NetID == 0 {
		return fmt.Errorf("group voice delivery scenario: net_id must be > 0")
	}
	if cfg.CallID == 0 {
		return fmt.Errorf("group voice delivery scenario: call_id must be > 0")
	}
	if cfg.TTL < 2 {
		return fmt.Errorf("group voice delivery scenario: ttl=%d must be >= 2 for bounded two-hop delivery", cfg.TTL)
	}
	if cfg.ReceiverJitterBufferMs < 0 {
		return fmt.Errorf("group voice delivery scenario: receiver_jitter_buffer_ms=%d must be >= 0", cfg.ReceiverJitterBufferMs)
	}
	if cfg.ReceiverLateThresholdMs < 0 {
		return fmt.Errorf("group voice delivery scenario: receiver_late_threshold_ms=%d must be >= 0", cfg.ReceiverLateThresholdMs)
	}
	profileCfg := normalizeGroupVoiceProfileScenarioConfig(cfg.ProfileConfig)
	if err := validateGroupVoiceProfileScenarioConfig(profileCfg); err != nil {
		return err
	}
	if len(groupVoiceExpectedRecipients(profileCfg)) == 0 {
		return fmt.Errorf("group voice delivery scenario: expected_members must include at least one receiver")
	}
	if err := validateGroupVoiceDeliveryDepth(profileCfg); err != nil {
		return err
	}
	return validateGroupVoiceDeliveryIDs(profileCfg)
}

func validateGroupVoiceDeliveryDepth(profileCfg GroupVoiceProfileScenarioConfig) error {
	for i, coverage := range profileCfg.RelayCoverage {
		if unsupportedGroupVoiceDepth(coverage.RelayPath) {
			return fmt.Errorf("group voice delivery scenario: relay_coverage %d relay_path depth=%d exceeds bounded max depth 2", i, len(coverage.RelayPath))
		}
	}
	return nil
}

func validateGroupVoiceDeliveryIDs(profileCfg GroupVoiceProfileScenarioConfig) error {
	endpoints := map[uint16]string{
		profileCfg.SourceID: "source_id",
	}
	for _, receiverID := range groupVoiceExpectedRecipients(profileCfg) {
		if previous, ok := endpoints[receiverID]; ok {
			return fmt.Errorf("group voice delivery scenario: receiver_id=%d duplicates %s", receiverID, previous)
		}
		endpoints[receiverID] = "receiver_id"
	}
	for _, coverage := range profileCfg.RelayCoverage {
		for _, relayID := range coverage.RelayPath {
			if previous, ok := endpoints[relayID]; ok {
				return fmt.Errorf("group voice delivery scenario: relay_id=%d duplicates %s", relayID, previous)
			}
		}
	}
	return nil
}
