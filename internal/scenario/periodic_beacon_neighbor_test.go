package scenario

import (
	"reflect"
	"strings"
	"testing"
)

func TestIT027_PeriodicBeaconMinuteIntervalKeepsNeighborTableFresh(t *testing.T) {
	report, err := RunPeriodicBeaconNeighborScenario(DefaultPeriodicBeaconNeighborScenarioConfig())
	if err != nil {
		t.Fatalf("RunPeriodicBeaconNeighborScenario failed: %v", err)
	}

	if report.ScenarioID != PeriodicBeaconNeighborScenarioID {
		t.Fatalf("ScenarioID = %q, want %q", report.ScenarioID, PeriodicBeaconNeighborScenarioID)
	}
	if report.BeaconIntervalMs != defaultPeriodicBeaconIntervalMs {
		t.Fatalf("BeaconIntervalMs = %d, want %d", report.BeaconIntervalMs, defaultPeriodicBeaconIntervalMs)
	}
	if report.NeighborStaleMs != defaultPeriodicBeaconStaleMs || report.NeighborDeadMs != defaultPeriodicBeaconDeadMs {
		t.Fatalf("stale/dead = %d/%d, want %d/%d",
			report.NeighborStaleMs, report.NeighborDeadMs, defaultPeriodicBeaconStaleMs, defaultPeriodicBeaconDeadMs)
	}
	if report.Outcome != PeriodicBeaconOutcomeRelayed {
		t.Fatalf("Outcome = %q, want %q", report.Outcome, PeriodicBeaconOutcomeRelayed)
	}
	if report.SelectedRelayID != 2 {
		t.Fatalf("SelectedRelayID = %d, want 2", report.SelectedRelayID)
	}
	if report.FreshRelayCandidateCount != 2 {
		t.Fatalf("FreshRelayCandidateCount = %d, want 2", report.FreshRelayCandidateCount)
	}
	if report.StaleNeighborCount != 0 || report.DeadNeighborCount != 0 || report.NeverSeenNeighborCount != 0 {
		t.Fatalf("stale/dead/never = %d/%d/%d, want all zero",
			report.StaleNeighborCount, report.DeadNeighborCount, report.NeverSeenNeighborCount)
	}
	if report.ControlChannelOccupancyPct >= 1 {
		t.Fatalf("ControlChannelOccupancyPct = %.3f, want < 1%% for minute beacon interval", report.ControlChannelOccupancyPct)
	}
	if report.EventHash == "" {
		t.Fatal("EventHash is empty")
	}
	t.Logf("periodic beacon default: interval=%dms beacon_airtime=%dms occupancy=%.3f%% selected_relay=%d fresh_relays=%d hash=%s",
		report.BeaconIntervalMs, report.BeaconAirtimeMs, report.ControlChannelOccupancyPct,
		report.SelectedRelayID, report.FreshRelayCandidateCount, report.EventHash)
}

func TestIT027_TwoSecondBeaconIntervalConsumesMuchMoreAirtime(t *testing.T) {
	fastCfg := DefaultPeriodicBeaconNeighborScenarioConfig()
	fastCfg.BeaconIntervalMs = 2_000
	fastCfg.SimulationEndMs = 60_000
	fastCfg.VoiceWindows = []PeriodicBeaconVoiceWindow{{StartMs: 30_000, DurationMs: 3_200}}
	fastCfg.Nodes = periodicBeaconNodesWithSpreadOffsets(fastCfg.Nodes, fastCfg.BeaconIntervalMs)

	fastReport, err := RunPeriodicBeaconNeighborScenario(fastCfg)
	if err != nil {
		t.Fatalf("fast RunPeriodicBeaconNeighborScenario failed: %v", err)
	}
	minuteReport, err := RunPeriodicBeaconNeighborScenario(DefaultPeriodicBeaconNeighborScenarioConfig())
	if err != nil {
		t.Fatalf("minute RunPeriodicBeaconNeighborScenario failed: %v", err)
	}

	if fastReport.ControlChannelOccupancyPct <= 5 {
		t.Fatalf("fast ControlChannelOccupancyPct = %.3f, want > 5%%", fastReport.ControlChannelOccupancyPct)
	}
	if fastReport.ControlChannelOccupancyPct <= minuteReport.ControlChannelOccupancyPct*10 {
		t.Fatalf("fast occupancy %.3f should be much higher than minute occupancy %.3f",
			fastReport.ControlChannelOccupancyPct, minuteReport.ControlChannelOccupancyPct)
	}
	t.Logf("periodic beacon occupancy: 2s=%.3f%% 60s=%.3f%%",
		fastReport.ControlChannelOccupancyPct, minuteReport.ControlChannelOccupancyPct)
}

func TestIT027_BeaconDuringVoiceIsDeferredAfterVoiceWindow(t *testing.T) {
	cfg := DefaultPeriodicBeaconNeighborScenarioConfig()
	cfg.Nodes = []PeriodicBeaconNode{
		{
			NodeID:            2,
			BeaconOffsetMs:    0,
			RelayCapable:      true,
			RelayEnabled:      true,
			QualityFromSource: 1,
			GroupCoverage:     1,
			Battery:           1,
			Stability:         1,
		},
	}
	cfg.BeaconIntervalMs = 1_000
	cfg.SimulationEndMs = 8_000
	cfg.VoiceWindows = []PeriodicBeaconVoiceWindow{{StartMs: 2_000, DurationMs: 3_200}}

	report, err := RunPeriodicBeaconNeighborScenario(cfg)
	if err != nil {
		t.Fatalf("RunPeriodicBeaconNeighborScenario failed: %v", err)
	}
	if report.DeferredBeaconCount == 0 || report.BeaconDuringVoiceCount == 0 {
		t.Fatalf("deferred/during voice = %d/%d, want non-zero", report.DeferredBeaconCount, report.BeaconDuringVoiceCount)
	}
	voiceStart := cfg.VoiceWindows[0].StartMs
	voiceEnd := voiceStart + cfg.VoiceWindows[0].DurationMs
	for _, event := range report.Events {
		if event.Type != PeriodicBeaconEventBeaconTx {
			continue
		}
		if event.TimeMs < voiceEnd && event.RxCompleteMs > voiceStart {
			t.Fatalf("beacon event overlaps voice window: %+v voice=%d..%d", event, voiceStart, voiceEnd)
		}
		if event.OriginalDueMs >= voiceStart && event.OriginalDueMs < voiceEnd && !event.Deferred {
			t.Fatalf("beacon due during voice was not deferred: %+v", event)
		}
	}
	t.Logf("periodic beacon voice conflict: deferred=%d due_during_voice=%d",
		report.DeferredBeaconCount, report.BeaconDuringVoiceCount)
}

func TestIT027_StaleRelayCandidateIsNotEligible(t *testing.T) {
	cfg := DefaultPeriodicBeaconNeighborScenarioConfig()
	cfg.SimulationEndMs = 260_000
	cfg.VoiceWindows = []PeriodicBeaconVoiceWindow{{StartMs: 250_000, DurationMs: 3_200}}
	for i := range cfg.Nodes {
		if cfg.Nodes[i].NodeID == 2 {
			cfg.Nodes[i].DisabledAtMs = 70_000
		}
	}

	report, err := RunPeriodicBeaconNeighborScenario(cfg)
	if err != nil {
		t.Fatalf("RunPeriodicBeaconNeighborScenario failed: %v", err)
	}

	row := requirePeriodicBeaconNeighborRow(t, report, 2)
	if row.Status != PeriodicBeaconNeighborStale {
		t.Fatalf("node 2 Status = %q, want %q age=%d last_seen=%d",
			row.Status, PeriodicBeaconNeighborStale, row.AgeAtCallMs, row.LastSeenMs)
	}
	if row.EligibleForRelay {
		t.Fatalf("node 2 EligibleForRelay = true, want false")
	}
	if report.SelectedRelayID != 3 {
		t.Fatalf("SelectedRelayID = %d, want 3 after node 2 becomes stale", report.SelectedRelayID)
	}
	t.Logf("periodic beacon stale relay: node2_age=%dms status=%s selected_relay=%d",
		row.AgeAtCallMs, row.Status, report.SelectedRelayID)
}

func TestIT027_DisabledAtStartNodeDoesNotBeaconOrBecomeFresh(t *testing.T) {
	cfg := DefaultPeriodicBeaconNeighborScenarioConfig()
	cfg.Nodes = []PeriodicBeaconNode{
		{
			NodeID:            2,
			Disabled:          true,
			DisabledAtMs:      0,
			RelayCapable:      true,
			RelayEnabled:      true,
			QualityFromSource: 1,
			GroupCoverage:     1,
			Battery:           1,
			Stability:         1,
		},
	}

	report, err := RunPeriodicBeaconNeighborScenario(cfg)
	if err != nil {
		t.Fatalf("RunPeriodicBeaconNeighborScenario failed: %v", err)
	}

	row := requirePeriodicBeaconNeighborRow(t, report, 2)
	if row.Status != PeriodicBeaconNeighborNeverSeen {
		t.Fatalf("node 2 Status = %q, want %q", row.Status, PeriodicBeaconNeighborNeverSeen)
	}
	if row.EligibleForRelay {
		t.Fatal("node 2 EligibleForRelay = true, want false")
	}
	if report.BeaconTxCount != 0 {
		t.Fatalf("BeaconTxCount = %d, want 0 for node disabled at start", report.BeaconTxCount)
	}
	if report.Outcome != PeriodicBeaconOutcomeNoFreshCandidate {
		t.Fatalf("Outcome = %q, want %q", report.Outcome, PeriodicBeaconOutcomeNoFreshCandidate)
	}
}

func TestIT027_VoiceEndSortsBeforeDeferredBeaconAtSameTimestamp(t *testing.T) {
	cfg := DefaultPeriodicBeaconNeighborScenarioConfig()
	cfg.BeaconGuardMs = 0
	cfg.BeaconIntervalMs = 1_000
	cfg.SimulationEndMs = 6_000
	cfg.Nodes = []PeriodicBeaconNode{
		{
			NodeID:            2,
			BeaconOffsetMs:    0,
			RelayCapable:      true,
			RelayEnabled:      true,
			QualityFromSource: 1,
			GroupCoverage:     1,
			Battery:           1,
			Stability:         1,
		},
	}
	cfg.VoiceWindows = []PeriodicBeaconVoiceWindow{{StartMs: 2_000, DurationMs: 3_200}}

	report, err := RunPeriodicBeaconNeighborScenario(cfg)
	if err != nil {
		t.Fatalf("RunPeriodicBeaconNeighborScenario failed: %v", err)
	}
	voiceEndMs := int64(5_200)
	voiceEndIndex := -1
	deferredBeaconIndex := -1
	for _, event := range report.Events {
		if event.TimeMs != voiceEndMs {
			continue
		}
		switch event.Type {
		case PeriodicBeaconEventVoiceEnd:
			voiceEndIndex = event.Index
		case PeriodicBeaconEventBeaconTx:
			if event.Deferred && event.DeferredReason == "voice_active" {
				deferredBeaconIndex = event.Index
			}
		}
	}
	if voiceEndIndex < 0 || deferredBeaconIndex < 0 {
		t.Fatalf("missing same-timestamp voice_end/deferred beacon: voice_end=%d beacon=%d events=%+v",
			voiceEndIndex, deferredBeaconIndex, report.Events)
	}
	if voiceEndIndex > deferredBeaconIndex {
		t.Fatalf("voice_end index %d should be before deferred beacon index %d", voiceEndIndex, deferredBeaconIndex)
	}
}

func TestIT027_PeriodicBeaconDeterministicReplay(t *testing.T) {
	cfg := DefaultPeriodicBeaconNeighborScenarioConfig()
	first, err := RunPeriodicBeaconNeighborScenario(cfg)
	if err != nil {
		t.Fatalf("first RunPeriodicBeaconNeighborScenario failed: %v", err)
	}
	second, err := RunPeriodicBeaconNeighborScenario(cfg)
	if err != nil {
		t.Fatalf("second RunPeriodicBeaconNeighborScenario failed: %v", err)
	}
	if first.EventHash != second.EventHash {
		t.Fatalf("EventHash mismatch:\nfirst=%s\nsecond=%s", first.EventHash, second.EventHash)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("same config produced different reports:\nfirst=%+v\nsecond=%+v", first, second)
	}
}

func TestPeriodicBeaconNeighborRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  PeriodicBeaconNeighborScenarioConfig
		want string
	}{
		{name: "zero interval", cfg: func() PeriodicBeaconNeighborScenarioConfig {
			cfg := DefaultPeriodicBeaconNeighborScenarioConfig()
			cfg.BeaconIntervalMs = -1
			return cfg
		}(), want: "beacon_interval_ms"},
		{name: "dead before stale", cfg: func() PeriodicBeaconNeighborScenarioConfig {
			cfg := DefaultPeriodicBeaconNeighborScenarioConfig()
			cfg.NeighborDeadMs = cfg.NeighborStaleMs
			return cfg
		}(), want: "neighbor_dead_ms"},
		{name: "bad offset", cfg: func() PeriodicBeaconNeighborScenarioConfig {
			cfg := DefaultPeriodicBeaconNeighborScenarioConfig()
			cfg.Nodes[0].BeaconOffsetMs = cfg.BeaconIntervalMs
			return cfg
		}(), want: "beacon_offset_ms"},
		{name: "overlapping voice", cfg: func() PeriodicBeaconNeighborScenarioConfig {
			cfg := DefaultPeriodicBeaconNeighborScenarioConfig()
			cfg.VoiceWindows = []PeriodicBeaconVoiceWindow{
				{StartMs: 1_000, DurationMs: 2_000},
				{StartMs: 2_000, DurationMs: 2_000},
			}
			return cfg
		}(), want: "overlaps"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RunPeriodicBeaconNeighborScenario(tc.cfg)
			if err == nil {
				t.Fatal("RunPeriodicBeaconNeighborScenario accepted invalid config")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want context %q", err.Error(), tc.want)
			}
		})
	}
}

func requirePeriodicBeaconNeighborRow(t *testing.T, report PeriodicBeaconNeighborReport, nodeID uint16) PeriodicBeaconNeighborRow {
	t.Helper()
	for _, row := range report.NeighborRows {
		if row.NodeID == nodeID {
			return row
		}
	}
	t.Fatalf("missing neighbor row for node %d", nodeID)
	return PeriodicBeaconNeighborRow{}
}

func periodicBeaconNodesWithSpreadOffsets(nodes []PeriodicBeaconNode, intervalMs int64) []PeriodicBeaconNode {
	out := clonePeriodicBeaconNodes(nodes)
	for i := range out {
		out[i].BeaconOffsetMs = int64(i) * intervalMs / int64(len(out))
	}
	return out
}
