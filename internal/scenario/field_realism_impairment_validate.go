package scenario

import (
	"fmt"
	"math"
)

func validateFieldRealismImpairmentConfig(cfg FieldRealismImpairmentConfig) error {
	if cfg.ScenarioID == "" {
		return fmt.Errorf("field realism impairment: scenario_id must not be empty")
	}
	if cfg.SourceID == 0 {
		return fmt.Errorf("field realism impairment: source_id must be > 0")
	}
	if cfg.ReceiverID == 0 {
		return fmt.Errorf("field realism impairment: receiver_id must be > 0")
	}
	if cfg.SourceID == cfg.ReceiverID {
		return fmt.Errorf("field realism impairment: source_id and receiver_id must be distinct")
	}
	if err := validateFieldRealismNodes(cfg); err != nil {
		return err
	}
	if err := validateFieldRealismDirectedLinks(cfg); err != nil {
		return err
	}
	if err := validateFieldRealismNonNegativeInt64s("clock_drift_ppm_candidates", cfg.ClockDriftPpmCandidates); err != nil {
		return err
	}
	if err := validateFieldRealismNonNegativeInt64s("slot_offset_ms_candidates", cfg.SlotOffsetMsCandidates); err != nil {
		return err
	}
	if err := validateFieldRealismPercentages("beacon_loss_pct_candidates", cfg.BeaconLossPctCandidates); err != nil {
		return err
	}
	if err := validateFieldRealismNonNegativeInt64s("beacon_jitter_ms_candidates", cfg.BeaconJitterMsCandidates); err != nil {
		return err
	}
	if cfg.CallStartMs < 0 {
		return fmt.Errorf("field realism impairment: call_start_ms=%d must be >= 0", cfg.CallStartMs)
	}
	if cfg.VoicePeriodMs <= 0 {
		return fmt.Errorf("field realism impairment: voice_period_ms=%d must be > 0", cfg.VoicePeriodMs)
	}
	if cfg.FrameCount <= 0 {
		return fmt.Errorf("field realism impairment: frame_count=%d must be > 0", cfg.FrameCount)
	}
	if cfg.FrameCount > maxRelayElectionTraceFrames {
		return fmt.Errorf("field realism impairment: frame_count=%d exceeds cap %d", cfg.FrameCount, maxRelayElectionTraceFrames)
	}
	if cfg.SourceSlotMs <= 0 {
		return fmt.Errorf("field realism impairment: source_slot_ms=%d must be > 0", cfg.SourceSlotMs)
	}
	if cfg.RelaySlotMs <= 0 {
		return fmt.Errorf("field realism impairment: relay_slot_ms=%d must be > 0", cfg.RelaySlotMs)
	}
	if cfg.GuardTimeMs < 0 {
		return fmt.Errorf("field realism impairment: guard_time_ms=%d must be >= 0", cfg.GuardTimeMs)
	}
	if cfg.LinkDelayMs < 0 {
		return fmt.Errorf("field realism impairment: link_delay_ms=%d must be >= 0", cfg.LinkDelayMs)
	}
	if cfg.SourceFrameBytes <= 0 {
		return fmt.Errorf("field realism impairment: source_frame_bytes=%d must be > 0", cfg.SourceFrameBytes)
	}
	if cfg.RelayFrameBytes <= 0 {
		return fmt.Errorf("field realism impairment: relay_frame_bytes=%d must be > 0", cfg.RelayFrameBytes)
	}
	if cfg.BeaconFrameBytes <= 0 {
		return fmt.Errorf("field realism impairment: beacon_frame_bytes=%d must be > 0", cfg.BeaconFrameBytes)
	}
	if cfg.ChannelBitrateBps <= 0 {
		return fmt.Errorf("field realism impairment: channel_bitrate_bps=%d must be > 0", cfg.ChannelBitrateBps)
	}
	if cfg.PreambleMs < 0 || cfg.SyncMs < 0 || cfg.TxRxSwitchMs < 0 {
		return fmt.Errorf("field realism impairment: preamble_ms, sync_ms, and tx_rx_switch_ms must be >= 0")
	}
	if cfg.BeaconIntervalMs <= 0 {
		return fmt.Errorf("field realism impairment: beacon_interval_ms=%d must be > 0", cfg.BeaconIntervalMs)
	}
	if cfg.NeighborStaleMs <= 0 {
		return fmt.Errorf("field realism impairment: neighbor_stale_ms=%d must be > 0", cfg.NeighborStaleMs)
	}
	if cfg.NeighborDeadMs <= cfg.NeighborStaleMs {
		return fmt.Errorf("field realism impairment: neighbor_dead_ms=%d must be > neighbor_stale_ms=%d", cfg.NeighborDeadMs, cfg.NeighborStaleMs)
	}
	if err := validateFieldRealismBeaconDueCount(cfg); err != nil {
		return err
	}
	if err := validateFieldRealismWindows("external_interference_windows", cfg.ExternalInterferenceWindows); err != nil {
		return err
	}
	if err := validateFieldRealismHiddenWindows(cfg.HiddenNodeTxWindows); err != nil {
		return err
	}
	if err := validateFieldRealismBusyWindows(cfg.LocalBusyWindows); err != nil {
		return err
	}
	rows := fieldRealismRowCount(cfg)
	if rows <= 0 {
		return fmt.Errorf("field realism impairment: row count must be > 0")
	}
	if rows > maxFieldRealismImpairmentRows {
		return fmt.Errorf("field realism impairment: row count %d exceeds cap %d", rows, maxFieldRealismImpairmentRows)
	}
	return nil
}

func validateFieldRealismNodes(cfg FieldRealismImpairmentConfig) error {
	if len(cfg.Nodes) == 0 {
		return fmt.Errorf("field realism impairment: nodes must not be empty")
	}
	seen := make(map[uint16]struct{}, len(cfg.Nodes))
	for i, node := range cfg.Nodes {
		if node.NodeID == 0 {
			return fmt.Errorf("field realism impairment: node %d node_id must be > 0", i)
		}
		if node.NodeID == cfg.SourceID || node.NodeID == cfg.ReceiverID {
			return fmt.Errorf("field realism impairment: node %d node_id=%d must be relay-candidate only", i, node.NodeID)
		}
		if _, ok := seen[node.NodeID]; ok {
			return fmt.Errorf("field realism impairment: duplicate node_id %d", node.NodeID)
		}
		seen[node.NodeID] = struct{}{}
		if node.BeaconOffsetMs < 0 {
			return fmt.Errorf("field realism impairment: node %d beacon_offset_ms=%d must be >= 0", i, node.BeaconOffsetMs)
		}
		if err := validateFieldRealismUnit("node battery", node.Battery); err != nil {
			return fmt.Errorf("field realism impairment: node %d: %w", i, err)
		}
		if err := validateFieldRealismUnit("node stability", node.Stability); err != nil {
			return fmt.Errorf("field realism impairment: node %d: %w", i, err)
		}
		for pointIndex, point := range node.HealthTimeline {
			if point.AtMs < 0 {
				return fmt.Errorf("field realism impairment: node %d health_timeline %d at_ms=%d must be >= 0", i, pointIndex, point.AtMs)
			}
			if err := validateFieldRealismUnit("health battery", point.Battery); err != nil {
				return fmt.Errorf("field realism impairment: node %d health_timeline %d: %w", i, pointIndex, err)
			}
			if err := validateFieldRealismUnit("health stability", point.Stability); err != nil {
				return fmt.Errorf("field realism impairment: node %d health_timeline %d: %w", i, pointIndex, err)
			}
		}
	}
	return nil
}

func validateFieldRealismDirectedLinks(cfg FieldRealismImpairmentConfig) error {
	if len(cfg.DirectedLinks) == 0 {
		return fmt.Errorf("field realism impairment: directed_links must not be empty")
	}
	nodeIDs := map[uint16]struct{}{
		cfg.SourceID:   {},
		cfg.ReceiverID: {},
	}
	for _, node := range cfg.Nodes {
		nodeIDs[node.NodeID] = struct{}{}
	}
	seenLinks := make(map[[2]uint16]struct{}, len(cfg.DirectedLinks))
	for i, link := range cfg.DirectedLinks {
		if link.FromID == 0 || link.ToID == 0 {
			return fmt.Errorf("field realism impairment: directed_links %d endpoints must be > 0", i)
		}
		if link.FromID == link.ToID {
			return fmt.Errorf("field realism impairment: directed_links %d self-loop is not allowed", i)
		}
		key := [2]uint16{link.FromID, link.ToID}
		if _, ok := seenLinks[key]; ok {
			return fmt.Errorf("field realism impairment: directed_links %d duplicate from_id=%d to_id=%d", i, link.FromID, link.ToID)
		}
		seenLinks[key] = struct{}{}
		if _, ok := nodeIDs[link.FromID]; !ok {
			return fmt.Errorf("field realism impairment: directed_links %d from_id=%d is unknown", i, link.FromID)
		}
		if _, ok := nodeIDs[link.ToID]; !ok {
			return fmt.Errorf("field realism impairment: directed_links %d to_id=%d is unknown", i, link.ToID)
		}
		if len(link.Points) == 0 {
			return fmt.Errorf("field realism impairment: directed_links %d points must not be empty", i)
		}
		seenPoints := make(map[int64]struct{}, len(link.Points))
		for pointIndex, point := range link.Points {
			if point.AtMs < 0 {
				return fmt.Errorf("field realism impairment: directed_links %d point %d at_ms=%d must be >= 0", i, pointIndex, point.AtMs)
			}
			if _, ok := seenPoints[point.AtMs]; ok {
				return fmt.Errorf("field realism impairment: directed_links %d duplicate point at_ms=%d", i, point.AtMs)
			}
			seenPoints[point.AtMs] = struct{}{}
			if err := validateFieldRealismUnit("quality", point.Quality); err != nil {
				return fmt.Errorf("field realism impairment: directed_links %d point %d: %w", i, pointIndex, err)
			}
		}
	}
	return nil
}

func validateFieldRealismBeaconDueCount(cfg FieldRealismImpairmentConfig) error {
	total := int64(0)
	for _, node := range cfg.Nodes {
		if node.BeaconOffsetMs > cfg.CallStartMs {
			continue
		}
		dueCount := (cfg.CallStartMs-node.BeaconOffsetMs)/cfg.BeaconIntervalMs + 1
		if dueCount > maxFieldRealismBeaconDueCount-total {
			return fmt.Errorf("field realism impairment: beacon due count exceeds cap %d", maxFieldRealismBeaconDueCount)
		}
		total += dueCount
	}
	return nil
}

func validateFieldRealismNonNegativeInt64s(name string, values []int64) error {
	if len(values) == 0 {
		return fmt.Errorf("field realism impairment: %s must not be empty", name)
	}
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if value < 0 {
			return fmt.Errorf("field realism impairment: %s contains %d, want >= 0", name, value)
		}
		if _, ok := seen[value]; ok {
			return fmt.Errorf("field realism impairment: %s contains duplicate %d", name, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateFieldRealismPercentages(name string, values []float64) error {
	if len(values) == 0 {
		return fmt.Errorf("field realism impairment: %s must not be empty", name)
	}
	seen := make(map[float64]struct{}, len(values))
	for _, value := range values {
		if math.IsNaN(value) {
			return fmt.Errorf("field realism impairment: %s contains NaN", name)
		}
		if math.IsInf(value, 0) {
			return fmt.Errorf("field realism impairment: %s contains Inf", name)
		}
		if value < 0 || value > 100 {
			return fmt.Errorf("field realism impairment: %s contains %g, want [0, 100]", name, value)
		}
		if _, ok := seen[value]; ok {
			return fmt.Errorf("field realism impairment: %s contains duplicate %g", name, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateFieldRealismUnit(name string, value float64) error {
	if math.IsNaN(value) {
		return fmt.Errorf("%s is NaN", name)
	}
	if math.IsInf(value, 0) {
		return fmt.Errorf("%s is Inf", name)
	}
	if value < 0 || value > 1 {
		return fmt.Errorf("%s=%g must be in [0, 1]", name, value)
	}
	return nil
}

func validateFieldRealismWindows(name string, windows []FieldRealismWindow) error {
	for i, window := range windows {
		if window.StartMs < 0 {
			return fmt.Errorf("field realism impairment: %s %d start_ms=%d must be >= 0", name, i, window.StartMs)
		}
		if window.DurationMs <= 0 {
			return fmt.Errorf("field realism impairment: %s %d duration_ms=%d must be > 0", name, i, window.DurationMs)
		}
		if _, err := checkedLimitedMultiHopAdd(window.StartMs, window.DurationMs); err != nil {
			return fmt.Errorf("field realism impairment: %s %d end time: %w", name, i, err)
		}
	}
	return nil
}

func validateFieldRealismHiddenWindows(windows []FieldRealismHiddenNodeWindow) error {
	for i, window := range windows {
		if window.StartMs < 0 {
			return fmt.Errorf("field realism impairment: hidden_node_tx_windows %d start_ms=%d must be >= 0", i, window.StartMs)
		}
		if window.DurationMs <= 0 {
			return fmt.Errorf("field realism impairment: hidden_node_tx_windows %d duration_ms=%d must be > 0", i, window.DurationMs)
		}
		if window.ReceiverID == 0 || window.InterfererID == 0 {
			return fmt.Errorf("field realism impairment: hidden_node_tx_windows %d receiver_id and interferer_id must be > 0", i)
		}
		if _, err := checkedLimitedMultiHopAdd(window.StartMs, window.DurationMs); err != nil {
			return fmt.Errorf("field realism impairment: hidden_node_tx_windows %d end time: %w", i, err)
		}
	}
	return nil
}

func validateFieldRealismBusyWindows(windows []FieldRealismLocalBusyWindow) error {
	for i, window := range windows {
		if window.NodeID == 0 {
			return fmt.Errorf("field realism impairment: local_busy_windows %d node_id must be > 0", i)
		}
		if window.StartMs < 0 {
			return fmt.Errorf("field realism impairment: local_busy_windows %d start_ms=%d must be >= 0", i, window.StartMs)
		}
		if window.DurationMs <= 0 {
			return fmt.Errorf("field realism impairment: local_busy_windows %d duration_ms=%d must be > 0", i, window.DurationMs)
		}
		switch window.Action {
		case FieldRealismBusyActionDefer, FieldRealismBusyActionDrop:
		default:
			return fmt.Errorf("field realism impairment: local_busy_windows %d action=%q is unsupported", i, window.Action)
		}
		if _, err := checkedLimitedMultiHopAdd(window.StartMs, window.DurationMs); err != nil {
			return fmt.Errorf("field realism impairment: local_busy_windows %d end time: %w", i, err)
		}
	}
	return nil
}
