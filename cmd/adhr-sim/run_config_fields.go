package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

func decodeScenarioConfigKeys(raw []byte) (map[string]struct{}, error) {
	var fields map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&fields); err != nil {
		return nil, err
	}
	keys := make(map[string]struct{}, len(fields))
	for key := range fields {
		keys[key] = struct{}{}
	}
	return keys, nil
}

func validateScenarioConfigFields(scenarioID string, keys map[string]struct{}) error {
	if scenarioID == "" {
		return nil
	}
	allowed, ok := scenarioFieldAllowlist(scenarioID)
	if !ok {
		return nil
	}
	extra := make([]string, 0)
	for key := range keys {
		if !allowed[key] {
			extra = append(extra, key)
		}
	}
	if len(extra) == 0 {
		return nil
	}
	sort.Strings(extra)
	return fmt.Errorf("scenario_id %q does not support field %q", scenarioID, extra[0])
}

func scenarioFieldAllowlist(scenarioID string) (map[string]bool, bool) {
	switch scenarioID {
	case scenario.ScenarioID:
		return oneHopScenarioFields(), true
	case scenario.RelayForwardQueueBackpressureScenarioID:
		return oneHopScenarioFields(), true
	case scenario.TwoHopLiveShortSourceScenarioID:
		return twoHopLiveScenarioFields(), true
	default:
		return nil, false
	}
}

func oneHopScenarioFields() map[string]bool {
	return map[string]bool{
		"scenario_id":                       true,
		"seed":                              true,
		"frame_count":                       true,
		"frame_period_ms":                   true,
		"start_ms":                          true,
		"link_delay_ms":                     true,
		"source_to_relay_loss_pct":          true,
		"relay_to_receiver_loss_pct":        true,
		"source_to_receiver_enabled":        true,
		"source_to_receiver_loss_pct":       true,
		"source_to_relay_duplicate_pct":     true,
		"relay_to_receiver_duplicate_pct":   true,
		"source_to_receiver_duplicate_pct":  true,
		"source_to_relay_corruption_pct":    true,
		"relay_to_receiver_corruption_pct":  true,
		"source_to_receiver_corruption_pct": true,
		"source_to_relay_jitter_ms":         true,
		"relay_to_receiver_jitter_ms":       true,
		"source_to_receiver_jitter_ms":      true,
		"receiver_jitter_buffer_ms":         true,
		"receiver_late_threshold_ms":        true,
		"relay_seen_cache_max_entries":      true,
		"relay_forward_queue_max_depth":     true,
		"source_id":                         true,
		"relay_id":                          true,
		"receiver_id":                       true,
		"net_id":                            true,
		"group_id":                          true,
		"voice_group_id_override":           true,
		"call_id":                           true,
		"ttl":                               true,
		"slot_mode_enabled":                 true,
		"slot_duration_ms":                  true,
		"allow_slot_timing_stress":          true,
	}
}

func twoHopLiveScenarioFields() map[string]bool {
	return map[string]bool{
		"scenario_id":                true,
		"seed":                       true,
		"frame_count":                true,
		"voice_period_ms":            true,
		"start_ms":                   true,
		"source_slot_ms":             true,
		"relay_slot_ms":              true,
		"guard_time_ms":              true,
		"link_delay_ms":              true,
		"channel_bitrate_bps":        true,
		"source_frame_bytes":         true,
		"relay_frame_bytes":          true,
		"source_wire_mode":           true,
		"latency_budget_ms":          true,
		"receiver_jitter_buffer_ms":  true,
		"receiver_late_threshold_ms": true,
		"source_id":                  true,
		"relay1_id":                  true,
		"relay2_id":                  true,
		"receiver_id":                true,
		"net_id":                     true,
		"group_id":                   true,
		"call_id":                    true,
		"ttl":                        true,
	}
}
