package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/scenario"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

type scheduledRelayTreeScenarioDTO struct {
	Control                *scheduledPeerControlDTO    `json:"control,omitempty"`
	VoiceLinks             []scheduledTransportLinkDTO `json:"voice_links,omitempty"`
	ExpectedServiceOutcome string                      `json:"expected_service_outcome,omitempty"`
	VoiceProfile           *timing.VoiceBudgetProfile  `json:"voice_profile,omitempty"`
	ScenarioID             string                      `json:"scenario_id"`
	Seed                   *uint64                     `json:"seed"`
	FrameCount             *uint32                     `json:"frame_count"`
	StartMS                *int64                      `json:"start_ms"`
	NetID                  *uint16                     `json:"net_id"`
	GroupID                *uint16                     `json:"group_id"`
	SourceStationID        *uint16                     `json:"source_station_id"`
	CallID                 *uint32                     `json:"call_id"`
	Schedule               *scheduledRelayScheduleDTO  `json:"schedule"`
	InitialPlan            *scheduledRelayPlanDTO      `json:"initial_plan"`
	Runtime                *scheduledRelayRuntimeDTO   `json:"runtime"`
	Replans                []scheduledRelayReplanDTO   `json:"replans"`
}

type scheduledRelayScheduleDTO struct {
	ScheduleID          uint32 `json:"schedule_id"`
	ScheduleAuthorityID uint16 `json:"schedule_authority_id"`
	TimingStrategy      string `json:"timing_strategy"`
	TimingEpochID       uint32 `json:"timing_epoch_id"`
	TimingAuthorityID   uint16 `json:"timing_authority_id"`
}

type scheduledRelayPlanDTO struct {
	Snapshot     scheduledRelaySnapshotDTO     `json:"snapshot"`
	Graph        scheduledRelayGraphDTO        `json:"graph"`
	Constraints  scheduledRelayConstraintsDTO  `json:"constraints"`
	CostModel    scheduledRelayCostModelDTO    `json:"cost_model"`
	SearchLimits scheduledRelaySearchLimitsDTO `json:"search_limits"`
	RelayStates  []scheduledRelayStateDTO      `json:"relay_states"`
}

type scheduledRelaySnapshotDTO struct {
	ID           uint32   `json:"id"`
	ObservedAtMS int64    `json:"observed_at_ms"`
	FreshnessMS  uint32   `json:"freshness_ms"`
	MemberIDs    []uint16 `json:"member_ids"`
}

type scheduledRelayGraphDTO struct {
	Epoch       uint32                  `json:"epoch"`
	FreshnessMS uint32                  `json:"freshness_ms"`
	Links       []scheduledRelayLinkDTO `json:"links"`
}

type scheduledRelayLinkDTO struct {
	FromStationID uint16  `json:"from_station_id"`
	ToStationID   uint16  `json:"to_station_id"`
	Enabled       bool    `json:"enabled"`
	Quality       float64 `json:"quality"`
	Loss          float64 `json:"loss"`
	DelayMS       int64   `json:"delay_ms"`
	ObservedAtMS  int64   `json:"observed_at_ms"`
}

type scheduledRelayConstraintsDTO struct {
	PlanningAtMS              int64   `json:"planning_at_ms"`
	MaxRelayDepth             uint8   `json:"max_relay_depth"`
	MaxPlayoutLatencyMS       int64   `json:"max_playout_latency_ms"`
	MaxVoiceAirtimePct        float64 `json:"max_voice_airtime_pct"`
	RequireAllExpectedMembers bool    `json:"require_all_expected_members"`
	MinRelayBatteryFraction   float64 `json:"min_relay_battery_fraction"`
	MinLinkQuality            float64 `json:"min_link_quality"`
	MaxLinkLoss               float64 `json:"max_link_loss"`
}

type scheduledRelayCostModelDTO struct {
	SourceOpportunityMS int64 `json:"source_opportunity_ms"`
	RelayOpportunityMS  int64 `json:"relay_opportunity_ms"`
	GuardAfterMS        int64 `json:"guard_after_ms"`
	FramePeriodMS       int64 `json:"frame_period_ms"`
}

type scheduledRelaySearchLimitsDTO struct {
	MaxSearchSteps    int `json:"max_search_steps,omitempty"`
	MaxPathsPerMember int `json:"max_paths_per_member"`
	MaxTreeCandidates int `json:"max_tree_candidates"`
	MaxFeasibleTrees  int `json:"max_feasible_trees"`
}

type scheduledRelayStateDTO struct {
	StationID       uint16  `json:"station_id"`
	RelayCapable    bool    `json:"relay_capable"`
	RelayEnabled    bool    `json:"relay_enabled"`
	Busy            bool    `json:"busy"`
	BatteryFraction float64 `json:"battery_fraction"`
}

type scheduledRelayRuntimeDTO struct {
	OpportunityProfile     scheduledRelayOpportunityProfileDTO `json:"opportunity_profile"`
	ForwardQueueMaxDepth   int                                 `json:"forward_queue_max_depth"`
	MissedParentThreshold  int                                 `json:"missed_parent_threshold"`
	AllowCycleTimingStress bool                                `json:"allow_cycle_timing_stress"`
}

type scheduledRelayOpportunityProfileDTO struct {
	SourceDurationMS uint16 `json:"source_duration_ms"`
	RelayDurationMS  uint16 `json:"relay_duration_ms"`
	GuardAfterMS     uint16 `json:"guard_after_ms"`
	MaxCycleMS       uint32 `json:"max_cycle_ms"`
}

type scheduledRelayReplanDTO struct {
	AtMS   int64                          `json:"at_ms"`
	Reason scenario.ScheduledReplanReason `json:"reason"`
	Plan   *scheduledRelayPlanDTO         `json:"plan"`
}

func scheduledRelayTreeScenarioID(raw []byte) (string, bool) {
	var envelope struct {
		ScenarioID string `json:"scenario_id"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&envelope); err != nil {
		return "", false
	}
	return envelope.ScenarioID, scenario.IsScheduledRelayTreeScenarioID(envelope.ScenarioID)
}

func decodeScheduledRelayTreeScenarioConfig(raw []byte) (scenario.ScheduledRelayTreeScenarioConfig, error) {
	var dto scheduledRelayTreeScenarioDTO
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&dto); err != nil {
		return scenario.ScheduledRelayTreeScenarioConfig{}, err
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return scenario.ScheduledRelayTreeScenarioConfig{}, fmt.Errorf("trailing JSON token")
		}
		return scenario.ScheduledRelayTreeScenarioConfig{}, err
	}
	return dto.toConfig()
}

func (d scheduledRelayTreeScenarioDTO) toConfig() (scenario.ScheduledRelayTreeScenarioConfig, error) {
	if d.Seed == nil || d.FrameCount == nil || d.StartMS == nil || d.NetID == nil || d.GroupID == nil ||
		d.SourceStationID == nil || d.CallID == nil || d.Schedule == nil || d.InitialPlan == nil || d.Runtime == nil {
		return scenario.ScheduledRelayTreeScenarioConfig{}, fmt.Errorf("scheduled relay-tree config requires all identity, schedule, initial_plan, and runtime fields")
	}
	replans := make([]scenario.ScheduledRelayTreeReplanConfig, 0, len(d.Replans))
	for i, replan := range d.Replans {
		if replan.Plan == nil {
			return scenario.ScheduledRelayTreeScenarioConfig{}, fmt.Errorf("replans[%d].plan is required", i)
		}
		replans = append(replans, scenario.ScheduledRelayTreeReplanConfig{
			AtMS: replan.AtMS, Reason: replan.Reason, Plan: replan.Plan.toConfig(),
		})
	}
	cfg := scenario.ScheduledRelayTreeScenarioConfig{
		VoiceLinks:             decodeScheduledTransportLinks(d.VoiceLinks),
		ExpectedServiceOutcome: d.ExpectedServiceOutcome,
		VoiceProfile:           d.VoiceProfile,
		ScenarioID:             d.ScenarioID,
		Seed:                   *d.Seed,
		FrameCount:             *d.FrameCount,
		StartMS:                *d.StartMS,
		NetID:                  *d.NetID,
		GroupID:                *d.GroupID,
		SourceStationID:        *d.SourceStationID,
		CallID:                 *d.CallID,
		ScheduleID:             d.Schedule.ScheduleID,
		ScheduleAuthorityID:    d.Schedule.ScheduleAuthorityID,
		TimingStrategy:         timing.TimingStrategy(d.Schedule.TimingStrategy),
		TimingEpochID:          d.Schedule.TimingEpochID,
		TimingAuthorityID:      d.Schedule.TimingAuthorityID,
		InitialPlan:            d.InitialPlan.toConfig(),
		Replans:                replans,
		OpportunityProfile: timing.OpportunityProfile{
			SourceDurationMS: d.Runtime.OpportunityProfile.SourceDurationMS,
			RelayDurationMS:  d.Runtime.OpportunityProfile.RelayDurationMS,
			GuardAfterMS:     d.Runtime.OpportunityProfile.GuardAfterMS,
			MaxCycleMS:       d.Runtime.OpportunityProfile.MaxCycleMS,
		},
		ForwardQueueMaxDepth:   d.Runtime.ForwardQueueMaxDepth,
		MissedParentThreshold:  d.Runtime.MissedParentThreshold,
		AllowCycleTimingStress: d.Runtime.AllowCycleTimingStress,
	}
	if d.Control != nil {
		cfg.Control = d.Control.toConfig()
	}
	return cfg, nil
}

func (d scheduledRelayPlanDTO) toConfig() scenario.ScheduledRelayTreePlanConfig {
	links := make([]routing.DirectedLink, len(d.Graph.Links))
	for i, link := range d.Graph.Links {
		links[i] = routing.DirectedLink{
			FromStationID: link.FromStationID, ToStationID: link.ToStationID,
			Enabled: link.Enabled, Quality: link.Quality, Loss: link.Loss,
			DelayMS: link.DelayMS, ObservedAtMS: link.ObservedAtMS,
		}
	}
	states := make([]routing.RelayState, len(d.RelayStates))
	for i, state := range d.RelayStates {
		states[i] = routing.RelayState{
			StationID: state.StationID, RelayCapable: state.RelayCapable,
			RelayEnabled: state.RelayEnabled, Busy: state.Busy,
			BatteryFraction: state.BatteryFraction,
		}
	}
	return scenario.ScheduledRelayTreePlanConfig{
		Snapshot: routing.ExpectedGroupSnapshot{
			ID: d.Snapshot.ID, ObservedAtMS: d.Snapshot.ObservedAtMS,
			FreshnessMS: d.Snapshot.FreshnessMS,
			MemberIDs:   append([]uint16(nil), d.Snapshot.MemberIDs...),
		},
		Graph: routing.NeighborGraph{
			Epoch: d.Graph.Epoch, FreshnessMS: d.Graph.FreshnessMS, Links: links,
		},
		Constraints: routing.PlanningConstraints{
			PlanningAtMS:              d.Constraints.PlanningAtMS,
			MaxRelayDepth:             d.Constraints.MaxRelayDepth,
			MaxPlayoutLatencyMS:       d.Constraints.MaxPlayoutLatencyMS,
			MaxVoiceAirtimePct:        d.Constraints.MaxVoiceAirtimePct,
			RequireAllExpectedMembers: d.Constraints.RequireAllExpectedMembers,
			MinRelayBatteryFraction:   d.Constraints.MinRelayBatteryFraction,
			MinLinkQuality:            d.Constraints.MinLinkQuality,
			MaxLinkLoss:               d.Constraints.MaxLinkLoss,
		},
		CostModel: routing.PlanningCostModel{
			SourceOpportunityMS: d.CostModel.SourceOpportunityMS,
			RelayOpportunityMS:  d.CostModel.RelayOpportunityMS,
			GuardAfterMS:        d.CostModel.GuardAfterMS,
			FramePeriodMS:       d.CostModel.FramePeriodMS,
		},
		SearchLimits: routing.PlanningSearchLimits{
			MaxSearchSteps:    d.SearchLimits.MaxSearchSteps,
			MaxPathsPerMember: d.SearchLimits.MaxPathsPerMember,
			MaxTreeCandidates: d.SearchLimits.MaxTreeCandidates,
			MaxFeasibleTrees:  d.SearchLimits.MaxFeasibleTrees,
		},
		RelayStates: states,
	}
}
