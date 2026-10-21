package scenario

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestIT028_FieldRealismDefaultReportIsDeterministicAndInventoried(t *testing.T) {
	first, err := RunFieldRealismImpairmentSweep(DefaultFieldRealismImpairmentConfig())
	if err != nil {
		t.Fatalf("RunFieldRealismImpairmentSweep failed: %v", err)
	}
	second, err := RunFieldRealismImpairmentSweep(DefaultFieldRealismImpairmentConfig())
	if err != nil {
		t.Fatalf("second RunFieldRealismImpairmentSweep failed: %v", err)
	}

	if first.ScenarioID != FieldRealismImpairmentScenarioID {
		t.Fatalf("ScenarioID = %q, want %q", first.ScenarioID, FieldRealismImpairmentScenarioID)
	}
	if len(first.Inventory.ModeledNow) == 0 || len(first.Inventory.SimulatedInThisStory) == 0 || len(first.Inventory.ExplicitlyDeferred) == 0 {
		t.Fatalf("inventory is incomplete: %+v", first.Inventory)
	}
	if first.RowCount == 0 || len(first.Rows) != first.RowCount {
		t.Fatalf("row count mismatch: row_count=%d len=%d", first.RowCount, len(first.Rows))
	}
	if first.EventHash == "" {
		t.Fatal("EventHash is empty")
	}
	if first.EventHash != second.EventHash {
		t.Fatalf("EventHash mismatch:\nfirst=%s\nsecond=%s", first.EventHash, second.EventHash)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same config produced different field-realism reports")
	}

	zero, err := RunFieldRealismImpairmentSweep(FieldRealismImpairmentConfig{})
	if err != nil {
		t.Fatalf("zero config RunFieldRealismImpairmentSweep failed: %v", err)
	}
	if !reflect.DeepEqual(first, zero) {
		t.Fatal("zero config did not normalize to DefaultFieldRealismImpairmentConfig")
	}
}

func TestIT028_DirectedAsymmetricLinkTimelineChangesRelaySelection(t *testing.T) {
	cfg := fieldRealismSingleRowConfig()

	nominal, err := RunFieldRealismImpairmentSweep(cfg)
	if err != nil {
		t.Fatalf("nominal RunFieldRealismImpairmentSweep failed: %v", err)
	}
	if got := nominal.Rows[0].SelectedRelayID; got != 2 {
		t.Fatalf("nominal selected_relay_id = %d, want 2", got)
	}

	cfg.DirectedLinks = []FieldRealismDirectedLinkTimeline{
		fieldRealismLink(cfg.SourceID, 2, 1.0),
		{
			FromID: 2,
			ToID:   cfg.ReceiverID,
			Points: []FieldRealismQualityPoint{
				{AtMs: 0, Quality: 0.95},
				{AtMs: cfg.CallStartMs, Quality: 0.0},
			},
		},
		fieldRealismLink(2, cfg.SourceID, 0.1),
		fieldRealismLink(cfg.SourceID, 3, 0.8),
		fieldRealismLink(3, cfg.ReceiverID, 0.8),
		fieldRealismLink(3, cfg.SourceID, 0.8),
	}
	degraded, err := RunFieldRealismImpairmentSweep(cfg)
	if err != nil {
		t.Fatalf("degraded RunFieldRealismImpairmentSweep failed: %v", err)
	}
	row := degraded.Rows[0]
	if row.SelectedRelayID != 3 {
		t.Fatalf("selected_relay_id = %d, want 3 after R2->group degrades", row.SelectedRelayID)
	}
	candidate := requireFieldRealismCandidate(t, row, 2)
	if candidate.QualityFromSource != 1.0 || candidate.GroupCoverage != 0 {
		t.Fatalf("candidate 2 directed qualities = source %.3f group %.3f, want 1.0/0.0",
			candidate.QualityFromSource, candidate.GroupCoverage)
	}
	if candidate.ReverseQuality != 0.1 {
		t.Fatalf("candidate 2 reverse quality = %.3f, want 0.1", candidate.ReverseQuality)
	}
}

func TestIT028_ReverseLinkDegradationExcludesRelayCandidate(t *testing.T) {
	cfg := fieldRealismSingleRowConfig()
	cfg.DirectedLinks = []FieldRealismDirectedLinkTimeline{
		fieldRealismLink(cfg.SourceID, 2, 1.0),
		fieldRealismLink(2, cfg.ReceiverID, 0.95),
		fieldRealismLink(2, cfg.SourceID, 0.0),
		fieldRealismLink(cfg.SourceID, 3, 0.8),
		fieldRealismLink(3, cfg.ReceiverID, 0.8),
		fieldRealismLink(3, cfg.SourceID, 0.8),
	}

	report, err := RunFieldRealismImpairmentSweep(cfg)
	if err != nil {
		t.Fatalf("RunFieldRealismImpairmentSweep failed: %v", err)
	}
	row := report.Rows[0]
	if row.SelectedRelayID != 3 {
		t.Fatalf("selected_relay_id = %d, want 3 after R2 reverse link degrades", row.SelectedRelayID)
	}
	candidate := requireFieldRealismCandidate(t, row, 2)
	if candidate.ReverseQuality != 0 || candidate.EligibleForRelay {
		t.Fatalf("candidate 2 reverse/eligible = %.3f/%v, want 0.0/false", candidate.ReverseQuality, candidate.EligibleForRelay)
	}
	if !fieldRealismHasReason(row.FailReasons, FieldRealismReasonDirectedLinkDegraded) {
		t.Fatalf("fail reasons = %v, want %q", row.FailReasons, FieldRealismReasonDirectedLinkDegraded)
	}
}

func TestIT028_DriftAndOffsetConsumeGuardBudget(t *testing.T) {
	cfg := fieldRealismSingleRowConfig()
	cfg.GuardTimeMs = 1
	cfg.ClockDriftPpmCandidates = []int64{100}
	cfg.SlotOffsetMsCandidates = []int64{5}

	report, err := RunFieldRealismImpairmentSweep(cfg)
	if err != nil {
		t.Fatalf("RunFieldRealismImpairmentSweep failed: %v", err)
	}
	row := report.Rows[0]
	if row.GuardRemainingMs >= 0 {
		t.Fatalf("GuardRemainingMs = %d, want negative guard after offset/drift", row.GuardRemainingMs)
	}
	if row.DriftCollisionRiskCount == 0 || row.OutOfSlotTxCount == 0 {
		t.Fatalf("drift/out_of_slot = %d/%d, want non-zero", row.DriftCollisionRiskCount, row.OutOfSlotTxCount)
	}
	if !fieldRealismHasReason(row.FailReasons, FieldRealismReasonGuardExhausted) {
		t.Fatalf("fail reasons = %v, want %q", row.FailReasons, FieldRealismReasonGuardExhausted)
	}
}

func TestIT028_ChannelOccupancyStressWindowsAreCounted(t *testing.T) {
	cfg := fieldRealismSingleRowConfig()
	cfg.ExternalInterferenceWindows = []FieldRealismWindow{{StartMs: cfg.CallStartMs, DurationMs: 20}}
	cfg.HiddenNodeTxWindows = []FieldRealismHiddenNodeWindow{{StartMs: cfg.CallStartMs + cfg.SourceSlotMs, DurationMs: 20, ReceiverID: cfg.ReceiverID, InterfererID: 99}}
	cfg.LocalBusyWindows = []FieldRealismLocalBusyWindow{
		{NodeID: 2, StartMs: cfg.CallStartMs + cfg.SourceSlotMs, DurationMs: 8, Action: FieldRealismBusyActionDefer},
		{NodeID: 2, StartMs: cfg.CallStartMs + cfg.SourceSlotMs + cfg.VoicePeriodMs, DurationMs: 8, Action: FieldRealismBusyActionDrop},
	}

	report, err := RunFieldRealismImpairmentSweep(cfg)
	if err != nil {
		t.Fatalf("RunFieldRealismImpairmentSweep failed: %v", err)
	}
	row := report.Rows[0]
	if row.ExternalInterferenceCount == 0 || row.HiddenNodeCollisionCount == 0 {
		t.Fatalf("external/hidden counts = %d/%d, want non-zero",
			row.ExternalInterferenceCount, row.HiddenNodeCollisionCount)
	}
	if row.LocalBusyDeferCount == 0 || row.LocalBusyDropCount == 0 {
		t.Fatalf("local busy defer/drop = %d/%d, want non-zero", row.LocalBusyDeferCount, row.LocalBusyDropCount)
	}
	if !fieldRealismHasReason(row.FailReasons, FieldRealismReasonLocalBusy) {
		t.Fatalf("fail reasons = %v, want %q", row.FailReasons, FieldRealismReasonLocalBusy)
	}
}

func TestIT028_BeaconOccupancyStressWindowsAreCounted(t *testing.T) {
	cfg := fieldRealismSingleRowConfig()
	cfg.ExternalInterferenceWindows = []FieldRealismWindow{{StartMs: cfg.Nodes[1].BeaconOffsetMs, DurationMs: 20}}
	cfg.LocalBusyWindows = []FieldRealismLocalBusyWindow{{NodeID: 2, StartMs: 0, DurationMs: 20, Action: FieldRealismBusyActionDefer}}

	report, err := RunFieldRealismImpairmentSweep(cfg)
	if err != nil {
		t.Fatalf("RunFieldRealismImpairmentSweep failed: %v", err)
	}
	row := report.Rows[0]
	if row.ExternalInterferenceCount == 0 {
		t.Fatalf("ExternalInterferenceCount = 0, want beacon overlap counted")
	}
	if row.LocalBusyDeferCount == 0 {
		t.Fatalf("LocalBusyDeferCount = 0, want beacon local-busy defer counted")
	}
}

func TestIT028_HiddenNodeCollisionAtSelectedRelayIsCounted(t *testing.T) {
	cfg := fieldRealismSingleRowConfig()
	cfg.HiddenNodeTxWindows = []FieldRealismHiddenNodeWindow{{StartMs: cfg.CallStartMs, DurationMs: 20, ReceiverID: 2, InterfererID: 99}}

	report, err := RunFieldRealismImpairmentSweep(cfg)
	if err != nil {
		t.Fatalf("RunFieldRealismImpairmentSweep failed: %v", err)
	}
	row := report.Rows[0]
	if row.HiddenNodeCollisionCount == 0 {
		t.Fatalf("HiddenNodeCollisionCount = 0, want source->selected-relay hidden collision counted")
	}
}

func TestIT028_BeaconJitterChangesNeighborFreshness(t *testing.T) {
	cfg := fieldRealismSingleRowConfig()
	cfg.CallStartMs = 1_000
	cfg.BeaconIntervalMs = 500
	cfg.NeighborStaleMs = 100
	cfg.NeighborDeadMs = 300
	cfg.BeaconJitterMsCandidates = []int64{800}

	report, err := RunFieldRealismImpairmentSweep(cfg)
	if err != nil {
		t.Fatalf("RunFieldRealismImpairmentSweep failed: %v", err)
	}
	row := report.Rows[0]
	if row.BeaconJitteredCount == 0 {
		t.Fatal("BeaconJitteredCount = 0, want jittered beacons")
	}
	if row.StaleNeighborCount == 0 {
		t.Fatalf("StaleNeighborCount = 0, want stale neighbors from delayed beacons; row=%+v", row)
	}
	if row.FreshRelayCandidateCount != 0 {
		t.Fatalf("FreshRelayCandidateCount = %d, want 0 after all relays become stale", row.FreshRelayCandidateCount)
	}
}

func TestIT028_BeaconLossExcludesRelayCandidates(t *testing.T) {
	cfg := fieldRealismSingleRowConfig()
	cfg.BeaconLossPctCandidates = []float64{100}

	report, err := RunFieldRealismImpairmentSweep(cfg)
	if err != nil {
		t.Fatalf("RunFieldRealismImpairmentSweep failed: %v", err)
	}
	row := report.Rows[0]
	if row.BeaconLostCount == 0 || row.BeaconTxCount != 0 {
		t.Fatalf("beacon lost/tx = %d/%d, want all due beacons lost", row.BeaconLostCount, row.BeaconTxCount)
	}
	if row.FreshRelayCandidateCount != 0 || row.SelectedRelayID != 0 {
		t.Fatalf("fresh candidates/selected = %d/%d, want no relay after beacon loss",
			row.FreshRelayCandidateCount, row.SelectedRelayID)
	}
	if row.NeverSeenNeighborCount == 0 {
		t.Fatalf("NeverSeenNeighborCount = 0, want lost beacons to prevent neighbor freshness")
	}
}

func TestIT028_DynamicRelayHealthExcludesBusyCandidate(t *testing.T) {
	cfg := fieldRealismSingleRowConfig()
	cfg.Nodes[0].HealthTimeline = []FieldRealismNodeHealthPoint{
		{AtMs: cfg.CallStartMs, RelayEnabled: true, Busy: true, Battery: 0.9, Stability: 0.9},
	}

	report, err := RunFieldRealismImpairmentSweep(cfg)
	if err != nil {
		t.Fatalf("RunFieldRealismImpairmentSweep failed: %v", err)
	}
	row := report.Rows[0]
	if row.SelectedRelayID != 3 {
		t.Fatalf("selected_relay_id = %d, want 3 after R2 becomes busy", row.SelectedRelayID)
	}
	candidate := requireFieldRealismCandidate(t, row, 2)
	if !candidate.Busy || candidate.EligibleForRelay {
		t.Fatalf("candidate 2 busy/eligible = %v/%v, want busy and ineligible", candidate.Busy, candidate.EligibleForRelay)
	}
}

func TestIT028_FieldRealismRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  FieldRealismImpairmentConfig
		want string
	}{
		{name: "negative timing", cfg: func() FieldRealismImpairmentConfig {
			cfg := fieldRealismSingleRowConfig()
			cfg.GuardTimeMs = -1
			return cfg
		}(), want: "guard_time_ms"},
		{name: "bad percentage", cfg: func() FieldRealismImpairmentConfig {
			cfg := fieldRealismSingleRowConfig()
			cfg.BeaconLossPctCandidates = []float64{101}
			return cfg
		}(), want: "beacon_loss_pct"},
		{name: "duplicate node", cfg: func() FieldRealismImpairmentConfig {
			cfg := fieldRealismSingleRowConfig()
			cfg.Nodes[1].NodeID = cfg.Nodes[0].NodeID
			return cfg
		}(), want: "duplicate node_id"},
		{name: "bad window", cfg: func() FieldRealismImpairmentConfig {
			cfg := fieldRealismSingleRowConfig()
			cfg.ExternalInterferenceWindows = []FieldRealismWindow{{StartMs: -1, DurationMs: 1}}
			return cfg
		}(), want: "external_interference_windows"},
		{name: "duplicate directed link", cfg: func() FieldRealismImpairmentConfig {
			cfg := fieldRealismSingleRowConfig()
			cfg.DirectedLinks = append(cfg.DirectedLinks, fieldRealismLink(cfg.SourceID, 2, 0.5))
			return cfg
		}(), want: "duplicate from_id"},
		{name: "duplicate link point", cfg: func() FieldRealismImpairmentConfig {
			cfg := fieldRealismSingleRowConfig()
			cfg.DirectedLinks[0].Points = append(cfg.DirectedLinks[0].Points, FieldRealismQualityPoint{AtMs: 0, Quality: 0.5})
			return cfg
		}(), want: "duplicate point"},
		{name: "invalid policy", cfg: func() FieldRealismImpairmentConfig {
			cfg := fieldRealismSingleRowConfig()
			cfg.Policy.MinSourceQuality = math.NaN()
			return cfg
		}(), want: "min_source_quality"},
		{name: "too many beacons", cfg: func() FieldRealismImpairmentConfig {
			cfg := fieldRealismSingleRowConfig()
			cfg.BeaconIntervalMs = 1
			cfg.CallStartMs = maxFieldRealismBeaconDueCount
			return cfg
		}(), want: "beacon due count"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RunFieldRealismImpairmentSweep(tc.cfg)
			if err == nil {
				t.Fatal("RunFieldRealismImpairmentSweep accepted invalid config")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want context %q", err.Error(), tc.want)
			}
		})
	}
}

func fieldRealismSingleRowConfig() FieldRealismImpairmentConfig {
	cfg := DefaultFieldRealismImpairmentConfig()
	cfg.ClockDriftPpmCandidates = []int64{0}
	cfg.SlotOffsetMsCandidates = []int64{0}
	cfg.BeaconLossPctCandidates = []float64{0}
	cfg.BeaconJitterMsCandidates = []int64{0}
	cfg.ExternalInterferenceWindows = nil
	cfg.HiddenNodeTxWindows = nil
	cfg.LocalBusyWindows = nil
	return cfg
}

func fieldRealismLink(fromID, toID uint16, quality float64) FieldRealismDirectedLinkTimeline {
	return FieldRealismDirectedLinkTimeline{
		FromID: fromID,
		ToID:   toID,
		Points: []FieldRealismQualityPoint{{AtMs: 0, Quality: quality}},
	}
}

func requireFieldRealismCandidate(t *testing.T, row FieldRealismImpairmentRow, nodeID uint16) FieldRealismCandidateRow {
	t.Helper()
	for _, candidate := range row.Candidates {
		if candidate.NodeID == nodeID {
			return candidate
		}
	}
	t.Fatalf("missing candidate row for node %d in %+v", nodeID, row.Candidates)
	return FieldRealismCandidateRow{}
}

func fieldRealismHasReason(reasons []string, want string) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}
