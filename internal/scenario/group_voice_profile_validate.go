package scenario

import (
	"fmt"

	"github.com/rmsff/ad-hoc-radio/internal/protocol"
)

func validateGroupVoiceProfileScenarioConfig(cfg GroupVoiceProfileScenarioConfig) error {
	if cfg.ScenarioID == "" {
		return fmt.Errorf("group voice profile scenario: scenario_id must not be empty")
	}
	if cfg.SourceID == 0 {
		return fmt.Errorf("group voice profile scenario: source_id must be > 0")
	}
	if cfg.GroupID == 0 {
		return fmt.Errorf("group voice profile scenario: group_id must be > 0")
	}
	if cfg.CoverageMode != RelayElectionCoverageModeClosedGroup {
		return fmt.Errorf("group voice profile scenario: coverage_mode=%q must be closed_group", cfg.CoverageMode)
	}
	if err := validateCoverageIDList("group voice profile scenario: expected_members", cfg.ExpectedMembers); err != nil {
		return err
	}
	expectedSet := uint16Set(cfg.ExpectedMembers)
	if _, ok := expectedSet[cfg.SourceID]; !ok {
		return fmt.Errorf("group voice profile scenario: expected_members must include source_id=%d", cfg.SourceID)
	}
	if cfg.RequiredGroupCoverage < 0 || cfg.RequiredGroupCoverage > 1 {
		return fmt.Errorf("group voice profile scenario: required_group_coverage=%g must be in [0, 1]", cfg.RequiredGroupCoverage)
	}
	recipients := groupVoiceExpectedRecipients(cfg)
	recipientSet := uint16Set(recipients)
	if err := validateCoverageMembers("group voice profile scenario: source_direct_reachable", cfg.SourceDirectReachable, recipientSet); err != nil {
		return err
	}
	if err := validateCoverageMembers("group voice profile scenario: unreachable_members", cfg.UnreachableMembers, recipientSet); err != nil {
		return err
	}

	unreachableSet := uint16Set(cfg.UnreachableMembers)
	for _, memberID := range cfg.SourceDirectReachable {
		if _, ok := unreachableSet[memberID]; ok {
			return fmt.Errorf("group voice profile scenario: member_id=%d is both source-direct reachable and unreachable", memberID)
		}
	}
	for i, coverage := range cfg.RelayCoverage {
		if len(coverage.RelayPath) == 0 {
			return fmt.Errorf("group voice profile scenario: relay_coverage %d relay_path must not be empty", i)
		}
		if err := validateGroupVoiceRelayPath(i, coverage.RelayPath, cfg.SourceID); err != nil {
			return err
		}
		if err := validateCoverageMembers("group voice profile scenario: relay_coverage members", coverage.Members, recipientSet); err != nil {
			return fmt.Errorf("group voice profile scenario: relay_coverage %d: %w", i, err)
		}
		for _, memberID := range coverage.Members {
			if _, ok := unreachableSet[memberID]; ok {
				return fmt.Errorf("group voice profile scenario: relay_coverage %d member_id=%d is both relay-covered and unreachable", i, memberID)
			}
		}
	}
	return validateGroupVoiceProfiles(cfg.Profiles)
}

func validateGroupVoiceRelayPath(index int, path []uint16, sourceID uint16) error {
	seen := make(map[uint16]struct{}, len(path))
	for _, relayID := range path {
		if relayID == 0 {
			return fmt.Errorf("group voice profile scenario: relay_coverage %d relay_path contains zero relay id", index)
		}
		if relayID == sourceID {
			return fmt.Errorf("group voice profile scenario: relay_coverage %d relay_path contains source_id=%d", index, sourceID)
		}
		if _, ok := seen[relayID]; ok {
			return fmt.Errorf("group voice profile scenario: relay_coverage %d relay_path contains duplicate relay_id=%d", index, relayID)
		}
		seen[relayID] = struct{}{}
	}
	return nil
}

func validateGroupVoiceProfiles(profiles []GroupVoiceProfile) error {
	seenNames := make(map[string]struct{}, len(profiles))
	for i, profile := range profiles {
		if profile.Name == "" {
			return fmt.Errorf("group voice profile scenario: profiles %d name must not be empty", i)
		}
		if _, ok := seenNames[profile.Name]; ok {
			return fmt.Errorf("group voice profile scenario: duplicate profile name %q", profile.Name)
		}
		seenNames[profile.Name] = struct{}{}
		if profile.QualityRank < 0 {
			return fmt.Errorf("group voice profile scenario: profile %q quality_rank must be >= 0", profile.Name)
		}
		if profile.MaxRelayDepth < 0 {
			return fmt.Errorf("group voice profile scenario: profile %q max_relay_depth must be >= 0", profile.Name)
		}
		if profile.ChannelBitrateBps <= 0 || profile.SourceFrameBytes <= 0 || profile.RelayFrameBytes <= 0 {
			return fmt.Errorf("group voice profile scenario: profile %q bitrate and frame sizes must be > 0", profile.Name)
		}
		switch profile.SourceWireMode {
		case TwoHopLiveSourceWireCanonical, TwoHopLiveSourceWireShort:
		default:
			return fmt.Errorf("group voice profile scenario: profile %q source_wire_mode=%q is not supported", profile.Name, profile.SourceWireMode)
		}
		if err := validateGroupVoiceProfileWireSizes(profile); err != nil {
			return err
		}
		if profile.SourceSlotMs <= 0 || profile.RelaySlotMs <= 0 || profile.VoicePeriodMs <= 0 {
			return fmt.Errorf("group voice profile scenario: profile %q slot and period values must be > 0", profile.Name)
		}
		if profile.VoicePeriodMs > protocol.MaxFramePeriodMs {
			return fmt.Errorf("group voice profile scenario: profile %q voice_period_ms=%d must be <= %d", profile.Name, profile.VoicePeriodMs, protocol.MaxFramePeriodMs)
		}
		if profile.PreambleMs < 0 || profile.SyncMs < 0 || profile.TxRxSwitchMs < 0 {
			return fmt.Errorf("group voice profile scenario: profile %q preamble/sync/tx_rx_switch must be >= 0", profile.Name)
		}
		if profile.GuardTimeMs < 0 || profile.LinkDelayMs < 0 {
			return fmt.Errorf("group voice profile scenario: profile %q guard/link delay must be >= 0", profile.Name)
		}
		if profile.LatencyBudgetMs <= 0 || profile.FramePeriodBudgetMs <= 0 {
			return fmt.Errorf("group voice profile scenario: profile %q budgets must be > 0", profile.Name)
		}
	}
	return nil
}

func validateGroupVoiceProfileWireSizes(profile GroupVoiceProfile) error {
	cfg := TwoHopLiveScenarioConfig{
		SourceFrameBytes: profile.SourceFrameBytes,
		SourceWireMode:   profile.SourceWireMode,
	}
	payloadBytes, err := twoHopLiveSourcePayloadBytes(cfg)
	if err != nil {
		return fmt.Errorf("group voice profile scenario: profile %q: %w", profile.Name, err)
	}
	if profile.MaxRelayDepth == 0 {
		return nil
	}
	relayBytes, err := protocol.CompressedRelayHopWireSize(payloadBytes)
	if err != nil {
		return fmt.Errorf("group voice profile scenario: profile %q: %w", profile.Name, err)
	}
	if profile.RelayFrameBytes != relayBytes {
		return fmt.Errorf("group voice profile scenario: profile %q relay_frame_bytes=%d must equal compressed wire size %d for source payload %d", profile.Name, profile.RelayFrameBytes, relayBytes, payloadBytes)
	}
	return nil
}
