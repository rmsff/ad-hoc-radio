package scenario

import (
	"reflect"
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

func TestUT023_ScheduledRelayTreeMetricsDirectAndOneRelay(t *testing.T) {
	tests := []struct {
		name              string
		scenarioID        string
		members           []uint16
		links             []routing.DirectedLink
		relayStates       []routing.RelayState
		wantDepth         uint8
		wantOpportunities int
		wantCycleMS       int64
		wantAirtimePct    float64
		wantLatencyMS     float64
	}{
		{
			name:              "direct",
			scenarioID:        ScheduledRelayTreeDirectScenarioID,
			members:           []uint16{2, 3},
			links:             scheduledMetricsLinks([2]uint16{1, 2}, [2]uint16{1, 3}),
			wantDepth:         0,
			wantOpportunities: 1,
			wantCycleMS:       11,
			wantAirtimePct:    12.5,
			wantLatencyMS:     11,
		},
		{
			name:              "one relay",
			scenarioID:        ScheduledRelayTreeOneRelayScenarioID,
			members:           []uint16{2},
			links:             scheduledMetricsLinks([2]uint16{1, 10}, [2]uint16{10, 2}),
			relayStates:       []routing.RelayState{scheduledMetricsRelayState(10)},
			wantDepth:         1,
			wantOpportunities: 2,
			wantCycleMS:       22,
			wantAirtimePct:    25,
			wantLatencyMS:     22,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := scheduledMetricsScenarioConfig(tc.scenarioID, tc.members, tc.links, tc.relayStates)
			metrics, err := RunScheduledRelayTreeScenario(cfg)
			if err != nil {
				t.Fatalf("RunScheduledRelayTreeScenario failed: %v", err)
			}
			raw, err := metrics.MarshalReportJSON()
			if err != nil {
				t.Fatalf("MarshalReportJSON failed: %v", err)
			}
			validateMetricsJSONAgainstSchema(t, raw)

			report := metrics.Report()
			if report.TwoHopLive != nil {
				t.Fatal("scheduled relay-tree report must omit two_hop_live")
			}
			got := report.ScheduledRelayTree
			if got == nil {
				t.Fatal("scheduled_relay_tree section missing")
			}
			if got.ScheduleID != cfg.ScheduleID || got.ScheduleVersion != 1 {
				t.Fatalf("schedule identity = %d/%d, want %d/1", got.ScheduleID, got.ScheduleVersion, cfg.ScheduleID)
			}
			if got.TimingStrategy != string(timing.TimingStrategySourceLed) ||
				got.TimingAuthorityID != cfg.SourceStationID || got.TimingEpochID != cfg.CallID {
				t.Fatalf("timing evidence = %+v", got)
			}
			if got.SelectedRelayDepth != tc.wantDepth || got.TxOpportunityCount != tc.wantOpportunities {
				t.Fatalf("depth/opportunities = %d/%d, want %d/%d", got.SelectedRelayDepth, got.TxOpportunityCount, tc.wantDepth, tc.wantOpportunities)
			}
			if got.ScheduleCycleDurationMS != tc.wantCycleMS || got.VoiceAirtimePct != tc.wantAirtimePct || got.ControlAirtimePct != 0 {
				t.Fatalf("cycle/airtime = %d/%v/%v, want %d/%v/0", got.ScheduleCycleDurationMS, got.VoiceAirtimePct, got.ControlAirtimePct, tc.wantCycleMS, tc.wantAirtimePct)
			}
			if !reflect.DeepEqual(got.ExpectedMemberIDs, tc.members) ||
				!reflect.DeepEqual(got.CoveredMemberIDs, tc.members) ||
				!reflect.DeepEqual(got.DeliveredMemberIDs, tc.members) || len(got.UncoveredMemberIDs) != 0 {
				t.Fatalf("member evidence = expected %v covered %v delivered %v uncovered %v",
					got.ExpectedMemberIDs, got.CoveredMemberIDs, got.DeliveredMemberIDs, got.UncoveredMemberIDs)
			}
			if len(got.LatencyByRelayDepth) != int(tc.wantDepth)+1 {
				t.Fatalf("latency buckets = %d, want %d", len(got.LatencyByRelayDepth), int(tc.wantDepth)+1)
			}
			bucket := got.LatencyByRelayDepth[tc.wantDepth]
			if bucket.RelayDepth != tc.wantDepth || bucket.SampleCount != len(tc.members)*int(cfg.FrameCount) ||
				bucket.Min != tc.wantLatencyMS || bucket.Avg != tc.wantLatencyMS ||
				bucket.P50 != tc.wantLatencyMS || bucket.P95 != tc.wantLatencyMS || bucket.Max != tc.wantLatencyMS {
				t.Fatalf("latency bucket = %+v, want depth=%d count=%d latency=%v",
					bucket, tc.wantDepth, len(tc.members)*int(cfg.FrameCount), tc.wantLatencyMS)
			}
			if got.ScheduleHash == "" || got.TreeHash == "" || got.ResultHash == "" || report.Determinism.EventHash == "" {
				t.Fatalf("deterministic hashes missing: %+v", got)
			}
			if got.DuplicatePlayoutCount != 0 || got.OutOfScheduleTxCount != 0 {
				t.Fatalf("nominal schedule counters = %+v", got)
			}
		})
	}
}

func TestUT023_ScheduledRelayTreeRejectsReplanTimingModelDrift(t *testing.T) {
	base := scheduledMetricsScenarioConfig(
		ScheduledRelayTreeDirectScenarioID,
		[]uint16{2},
		scheduledMetricsLinks([2]uint16{1, 2}),
		nil,
	)
	tests := []struct {
		name string
		edit func(*ScheduledRelayTreePlanConfig)
		want string
	}{
		{
			name: "opportunity duration drift",
			edit: func(plan *ScheduledRelayTreePlanConfig) { plan.CostModel.SourceOpportunityMS++ },
			want: "opportunity_profile",
		},
		{
			name: "frame period drift",
			edit: func(plan *ScheduledRelayTreePlanConfig) { plan.CostModel.FramePeriodMS += 20 },
			want: "frame_period_ms",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base
			plan := cfg.InitialPlan
			plan.Snapshot.MemberIDs = append([]uint16(nil), plan.Snapshot.MemberIDs...)
			plan.Graph.Links = append([]routing.DirectedLink(nil), plan.Graph.Links...)
			plan.RelayStates = append([]routing.RelayState(nil), plan.RelayStates...)
			tc.edit(&plan)
			cfg.Replans = []ScheduledRelayTreeReplanConfig{{
				AtMS: 1_080, Reason: ScheduledReplanTopologyEpochChanged, Plan: plan,
			}}
			_, err := RunScheduledRelayTreeScenario(cfg)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestUT023_ScheduledRelayTreeMetricsAreDeterministicAndIndependent(t *testing.T) {
	cfg := scheduledMetricsScenarioConfig(
		ScheduledRelayTreeOneRelayScenarioID,
		[]uint16{2},
		scheduledMetricsLinks([2]uint16{1, 10}, [2]uint16{10, 2}),
		[]routing.RelayState{scheduledMetricsRelayState(10)},
	)
	first, err := RunScheduledRelayTreeScenario(cfg)
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	second, err := RunScheduledRelayTreeScenario(cfg)
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	firstJSON, err := first.MarshalReportJSON()
	if err != nil {
		t.Fatalf("marshal first report failed: %v", err)
	}
	secondJSON, err := second.MarshalReportJSON()
	if err != nil {
		t.Fatalf("marshal second report failed: %v", err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatalf("fixed-input metrics differ:\n%s\n%s", firstJSON, secondJSON)
	}

	report := first.Report()
	report.ScheduledRelayTree.ExpectedMemberIDs[0] = 999
	if first.Report().ScheduledRelayTree.ExpectedMemberIDs[0] != 2 {
		t.Fatal("Report returned aliased scheduled relay-tree member slices")
	}
}

func scheduledMetricsScenarioConfig(
	scenarioID string,
	members []uint16,
	links []routing.DirectedLink,
	states []routing.RelayState,
) ScheduledRelayTreeScenarioConfig {
	return ScheduledRelayTreeScenarioConfig{
		ScenarioID:          scenarioID,
		Seed:                42,
		FrameCount:          3,
		StartMS:             1_000,
		NetID:               1,
		GroupID:             100,
		SourceStationID:     1,
		CallID:              9_001,
		ScheduleID:          100,
		ScheduleAuthorityID: 1,
		TimingStrategy:      timing.TimingStrategySourceLed,
		TimingEpochID:       9_001,
		TimingAuthorityID:   1,
		InitialPlan: ScheduledRelayTreePlanConfig{
			Snapshot: routing.ExpectedGroupSnapshot{
				ID: 9, ObservedAtMS: 900, FreshnessMS: 1_000,
				MemberIDs: append([]uint16(nil), members...),
			},
			Graph: routing.NeighborGraph{
				Epoch: 7, FreshnessMS: 1_000,
				Links: append([]routing.DirectedLink(nil), links...),
			},
			Constraints: routing.PlanningConstraints{
				PlanningAtMS: 1_000, MaxRelayDepth: 3,
				MaxPlayoutLatencyMS: 250, MaxVoiceAirtimePct: 100,
				RequireAllExpectedMembers: true,
				MinRelayBatteryFraction:   0, MinLinkQuality: 0, MaxLinkLoss: 1,
			},
			CostModel: routing.PlanningCostModel{
				SourceOpportunityMS: 10, RelayOpportunityMS: 10,
				GuardAfterMS: 1, FramePeriodMS: 80,
			},
			SearchLimits: routing.PlanningSearchLimits{
				MaxPathsPerMember: 20, MaxTreeCandidates: 200, MaxFeasibleTrees: 200,
			},
			RelayStates: append([]routing.RelayState(nil), states...),
		},
		OpportunityProfile: timing.OpportunityProfile{
			SourceDurationMS: 10, RelayDurationMS: 10, GuardAfterMS: 1, MaxCycleMS: 80,
		},
		ForwardQueueMaxDepth:  DefaultRelayForwardQueueMaxDepth,
		MissedParentThreshold: DefaultScheduledMissedParentThreshold,
	}
}

func scheduledMetricsLinks(pairs ...[2]uint16) []routing.DirectedLink {
	links := make([]routing.DirectedLink, 0, len(pairs))
	for _, pair := range pairs {
		links = append(links, routing.DirectedLink{
			FromStationID: pair[0], ToStationID: pair[1], Enabled: true,
			Quality: 0.9, Loss: 0.01, DelayMS: 1, ObservedAtMS: 900,
		})
	}
	return links
}

func scheduledMetricsRelayState(id uint16) routing.RelayState {
	return routing.RelayState{
		StationID: id, RelayCapable: true, RelayEnabled: true,
		BatteryFraction: 0.9,
	}
}
