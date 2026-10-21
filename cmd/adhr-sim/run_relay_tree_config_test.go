package main

import (
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

func TestIT064_LoadScheduledRelayTreeConfigAndExecute(t *testing.T) {
	path := writeScenarioConfig(t, scheduledRelayTreeDirectConfigJSON())
	dto, runnable, err := loadScenarioConfig(path)
	if err != nil {
		t.Fatalf("loadScenarioConfig failed: %v", err)
	}
	if dto.ScenarioID != scenario.ScheduledRelayTreeDirectScenarioID {
		t.Fatalf("scenario_id = %q", dto.ScenarioID)
	}
	if runnable.ScheduledRelayTree == nil || runnable.OneHop != nil || runnable.TwoHopLive != nil {
		t.Fatalf("runnable config = %+v", runnable)
	}
	metrics, err := executeScenario(runnable)
	if err != nil {
		t.Fatalf("executeScenario failed: %v", err)
	}
	if metrics.ScheduledRelayTree == nil || metrics.TwoHopLive != nil {
		t.Fatalf("extension evidence = scheduled %v legacy %v", metrics.ScheduledRelayTree, metrics.TwoHopLive)
	}
}

func TestIT064_ScheduledRelayTreeConfigRejectsUnknownTrailingAndHybridFields(t *testing.T) {
	valid := scheduledRelayTreeDirectConfigJSON()
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "unknown nested field",
			body: strings.Replace(valid, `"schedule_id": 100`, `"schedule_id": 100, "unexpected": true`, 1),
			want: "unknown field",
		},
		{
			name: "trailing token",
			body: valid + ` {}`,
			want: "trailing JSON token",
		},
		{
			name: "legacy hybrid field",
			body: strings.Replace(valid, `"seed": 42`, `"seed": 42, "source_slot_ms": 12`, 1),
			want: "unknown field",
		},
		{
			name: "missing nested plan",
			body: strings.Replace(valid, `"initial_plan": {`, `"ignored_plan": {`, 1),
			want: "unknown field",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := writeScenarioConfig(t, tc.body)
			_, _, err := loadScenarioConfig(path)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func scheduledRelayTreeDirectConfigJSON() string {
	return `{
  "scenario_id": "scheduled_relay_tree_direct",
  "seed": 42,
  "frame_count": 3,
  "start_ms": 1000,
  "net_id": 1,
  "group_id": 100,
  "source_station_id": 1,
  "call_id": 9001,
  "schedule": {
    "schedule_id": 100,
    "schedule_authority_id": 1,
    "timing_strategy": "source_led",
    "timing_epoch_id": 9001,
    "timing_authority_id": 1
  },
  "initial_plan": {
    "snapshot": {
      "id": 9,
      "observed_at_ms": 900,
      "freshness_ms": 1000,
      "member_ids": [2, 3]
    },
    "graph": {
      "epoch": 7,
      "freshness_ms": 1000,
      "links": [
        {"from_station_id": 1, "to_station_id": 2, "enabled": true, "quality": 0.9, "loss": 0.01, "delay_ms": 1, "observed_at_ms": 900},
        {"from_station_id": 1, "to_station_id": 3, "enabled": true, "quality": 0.9, "loss": 0.01, "delay_ms": 1, "observed_at_ms": 900}
      ]
    },
    "constraints": {
      "planning_at_ms": 1000,
      "max_relay_depth": 3,
      "max_playout_latency_ms": 250,
      "max_voice_airtime_pct": 100,
      "require_all_expected_members": true,
      "min_relay_battery_fraction": 0,
      "min_link_quality": 0,
      "max_link_loss": 1
    },
    "cost_model": {
      "source_opportunity_ms": 10,
      "relay_opportunity_ms": 10,
      "guard_after_ms": 1,
      "frame_period_ms": 80
    },
    "search_limits": {
      "max_paths_per_member": 20,
      "max_tree_candidates": 200,
      "max_feasible_trees": 200
    },
    "relay_states": []
  },
  "runtime": {
    "opportunity_profile": {
      "source_duration_ms": 10,
      "relay_duration_ms": 10,
      "guard_after_ms": 1,
      "max_cycle_ms": 80
    },
    "forward_queue_max_depth": 64,
    "missed_parent_threshold": 2,
    "allow_cycle_timing_stress": false
  },
  "replans": []
}`
}
