package scenario

import (
	"errors"
	"fmt"

	"github.com/rmsff/ad-hoc-radio/internal/election"
)

func normalizePeriodicBeaconNeighborScenarioConfig(cfg PeriodicBeaconNeighborScenarioConfig) PeriodicBeaconNeighborScenarioConfig {
	def := DefaultPeriodicBeaconNeighborScenarioConfig()
	if cfg.ScenarioID == "" {
		cfg.ScenarioID = def.ScenarioID
	}
	if cfg.Seed == 0 {
		cfg.Seed = def.Seed
	}
	if cfg.SourceID == 0 {
		cfg.SourceID = def.SourceID
	}
	if cfg.Policy == (election.Policy{}) {
		cfg.Policy = def.Policy
	}
	if cfg.Nodes == nil {
		cfg.Nodes = clonePeriodicBeaconNodes(def.Nodes)
	}
	if cfg.BeaconIntervalMs == 0 {
		cfg.BeaconIntervalMs = def.BeaconIntervalMs
	}
	if cfg.BeaconFrameBytes == 0 {
		cfg.BeaconFrameBytes = def.BeaconFrameBytes
	}
	if cfg.ChannelBitrateBps == 0 {
		cfg.ChannelBitrateBps = def.ChannelBitrateBps
	}
	if cfg.NeighborStaleMs == 0 {
		cfg.NeighborStaleMs = def.NeighborStaleMs
	}
	if cfg.NeighborDeadMs == 0 {
		cfg.NeighborDeadMs = def.NeighborDeadMs
	}
	if cfg.SimulationEndMs == 0 {
		cfg.SimulationEndMs = def.SimulationEndMs
	}
	if cfg.VoicePolicy == "" {
		cfg.VoicePolicy = def.VoicePolicy
	}
	if cfg.VoiceWindows == nil {
		cfg.VoiceWindows = clonePeriodicBeaconVoiceWindows(def.VoiceWindows)
	}
	return cfg
}

func validatePeriodicBeaconNeighborScenarioConfig(cfg PeriodicBeaconNeighborScenarioConfig) error {
	if cfg.SourceID == 0 {
		return fmt.Errorf("periodic beacon scenario: source_id must be > 0")
	}
	if cfg.BeaconIntervalMs <= 0 {
		return fmt.Errorf("periodic beacon scenario: beacon_interval_ms=%d must be > 0", cfg.BeaconIntervalMs)
	}
	if cfg.BeaconFrameBytes <= 0 {
		return fmt.Errorf("periodic beacon scenario: beacon_frame_bytes=%d must be > 0", cfg.BeaconFrameBytes)
	}
	if cfg.ChannelBitrateBps <= 0 {
		return fmt.Errorf("periodic beacon scenario: channel_bitrate_bps=%d must be > 0", cfg.ChannelBitrateBps)
	}
	if cfg.BeaconGuardMs < 0 {
		return fmt.Errorf("periodic beacon scenario: beacon_guard_ms=%d must be >= 0", cfg.BeaconGuardMs)
	}
	if cfg.NeighborStaleMs <= 0 {
		return fmt.Errorf("periodic beacon scenario: neighbor_stale_ms=%d must be > 0", cfg.NeighborStaleMs)
	}
	if cfg.NeighborDeadMs <= cfg.NeighborStaleMs {
		return fmt.Errorf("periodic beacon scenario: neighbor_dead_ms=%d must be > neighbor_stale_ms=%d", cfg.NeighborDeadMs, cfg.NeighborStaleMs)
	}
	if cfg.SimulationEndMs <= 0 {
		return fmt.Errorf("periodic beacon scenario: simulation_end_ms=%d must be > 0", cfg.SimulationEndMs)
	}
	if cfg.VoicePolicy != BeaconVoicePolicyDeferAfterVoice {
		return fmt.Errorf("periodic beacon scenario: unsupported voice_policy %q", cfg.VoicePolicy)
	}
	if len(cfg.VoiceWindows) == 0 {
		return fmt.Errorf("periodic beacon scenario: at least one voice window is required")
	}
	if len(cfg.Nodes) == 0 {
		return fmt.Errorf("periodic beacon scenario: at least one beacon node is required")
	}
	if err := validatePeriodicBeaconVoiceWindows(cfg.VoiceWindows, cfg.SimulationEndMs); err != nil {
		return err
	}
	seen := make(map[uint16]struct{}, len(cfg.Nodes))
	for i, node := range cfg.Nodes {
		if node.NodeID == 0 {
			return fmt.Errorf("periodic beacon scenario: node %d node_id must be > 0", i)
		}
		if node.NodeID == cfg.SourceID {
			return fmt.Errorf("periodic beacon scenario: node %d node_id=%d matches source_id", i, node.NodeID)
		}
		if _, ok := seen[node.NodeID]; ok {
			return fmt.Errorf("periodic beacon scenario: duplicate node_id %d", node.NodeID)
		}
		seen[node.NodeID] = struct{}{}
		if node.BeaconOffsetMs < 0 || node.BeaconOffsetMs >= cfg.BeaconIntervalMs {
			return fmt.Errorf("periodic beacon scenario: node %d beacon_offset_ms=%d must be in [0, beacon_interval_ms)", i, node.BeaconOffsetMs)
		}
		if node.DisabledAtMs < 0 {
			return fmt.Errorf("periodic beacon scenario: node %d disabled_at_ms=%d must be >= 0", i, node.DisabledAtMs)
		}
		if _, err := election.SelectRelay([]election.Candidate{periodicBeaconCandidate(node)}, cfg.Policy); err != nil && !errors.Is(err, election.ErrNoEligibleRelay) {
			return fmt.Errorf("periodic beacon scenario: node %d: %w", i, err)
		}
	}
	return nil
}

func validatePeriodicBeaconVoiceWindows(windows []PeriodicBeaconVoiceWindow, simulationEndMs int64) error {
	previousEnd := int64(-1)
	for i, window := range windows {
		if window.StartMs < 0 {
			return fmt.Errorf("periodic beacon scenario: voice window %d start_ms=%d must be >= 0", i, window.StartMs)
		}
		if window.DurationMs <= 0 {
			return fmt.Errorf("periodic beacon scenario: voice window %d duration_ms=%d must be > 0", i, window.DurationMs)
		}
		endMs, err := periodicBeaconWindowEnd(window)
		if err != nil {
			return fmt.Errorf("periodic beacon scenario: voice window %d: %w", i, err)
		}
		if endMs > simulationEndMs {
			return fmt.Errorf("periodic beacon scenario: voice window %d ends at %d after simulation_end_ms=%d", i, endMs, simulationEndMs)
		}
		if window.StartMs < previousEnd {
			return fmt.Errorf("periodic beacon scenario: voice window %d overlaps previous window", i)
		}
		previousEnd = endMs
	}
	return nil
}
