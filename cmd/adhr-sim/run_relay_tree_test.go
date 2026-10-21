package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

func TestIT058_ScheduledRelayTreeCLI_DirectAndChains(t *testing.T) {
	tests := []struct {
		name        string
		file        string
		wantDepth   uint8
		wantOpps    int
		wantCycleMS int64
	}{
		{name: "direct", file: "direct.json", wantDepth: 0, wantOpps: 1, wantCycleMS: 11},
		{name: "one relay", file: "one-relay.json", wantDepth: 1, wantOpps: 2, wantCycleMS: 22},
		{name: "two relay", file: "two-relay.json", wantDepth: 2, wantOpps: 3, wantCycleMS: 33},
		{name: "three relay exception", file: "three-relay-exception.json", wantDepth: 3, wantOpps: 4, wantCycleMS: 44},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			report := runScheduledRelayTreeFixture(t, tc.file)
			got := report.ScheduledRelayTree
			if got == nil || report.TwoHopLive != nil {
				t.Fatalf("extension evidence = scheduled %v legacy %v", got, report.TwoHopLive)
			}
			if got.SelectedRelayDepth != tc.wantDepth || got.TxOpportunityCount != tc.wantOpps || got.ScheduleCycleDurationMS != tc.wantCycleMS {
				t.Fatalf("depth/opps/cycle = %d/%d/%d, want %d/%d/%d",
					got.SelectedRelayDepth, got.TxOpportunityCount, got.ScheduleCycleDurationMS,
					tc.wantDepth, tc.wantOpps, tc.wantCycleMS)
			}
			if report.Outcome != "pass" || got.DuplicatePlayoutCount != 0 || got.OutOfScheduleTxCount != 0 {
				t.Fatalf("nominal outcome/counters = %q/%+v", report.Outcome, got)
			}
		})
	}
}

func TestIT059_ScheduledRelayTreeCLI_BranchesSerializeOwners(t *testing.T) {
	narrow := runScheduledRelayTreeFixture(t, "branch-narrow.json").ScheduledRelayTree
	wide := runScheduledRelayTreeFixture(t, "branch-wide.json").ScheduledRelayTree
	if narrow.SelectedRelayDepth != 1 || narrow.TxOpportunityCount != 2 || len(narrow.SelectedTreeEdges) != 3 {
		t.Fatalf("narrow branch evidence = %+v", narrow)
	}
	if wide.SelectedRelayDepth != 1 || wide.TxOpportunityCount != 3 || len(wide.SelectedTreeEdges) != 4 {
		t.Fatalf("wide branch evidence = %+v", wide)
	}
	if wide.ScheduleCycleDurationMS <= narrow.ScheduleCycleDurationMS {
		t.Fatalf("wide cycle=%d must exceed narrow=%d", wide.ScheduleCycleDurationMS, narrow.ScheduleCycleDurationMS)
	}
}

func TestIT060_ScheduledRelayTreeCLI_PartitionRecoveryUsesFutureVersion(t *testing.T) {
	report := runScheduledRelayTreeFixture(t, "partition-recovery.json")
	got := report.ScheduledRelayTree
	if got.ScheduleVersion != 2 || got.RouteReplanCount != 2 || got.VersionActivationCount != 1 {
		t.Fatalf("recovery version/replans/activations = %d/%d/%d, want 2/2/1", got.ScheduleVersion, got.RouteReplanCount, got.VersionActivationCount)
	}
	if got.ScheduleExpiredDropCount != 1 || got.BlockedFromSeq != nil {
		t.Fatalf("recovery suppression/final block = %d/%v, want 1/nil", got.ScheduleExpiredDropCount, got.BlockedFromSeq)
	}
	if report.SourceFramesCreated != 4 || report.SourceFramesSent != 3 || report.Outcome != "pass" {
		t.Fatalf("source/outcome = %d/%d/%s, want 4/3/pass", report.SourceFramesCreated, report.SourceFramesSent, report.Outcome)
	}
}

func TestIT061_ScheduledRelayTreeCLI_FixedInputsProduceIdenticalMetrics(t *testing.T) {
	for _, file := range []string{
		"direct.json", "one-relay.json", "two-relay.json", "three-relay-exception.json",
		"branch-narrow.json", "branch-wide.json", "partition-recovery.json",
	} {
		t.Run(file, func(t *testing.T) {
			first := runScheduledRelayTreeFixtureBytes(t, file)
			second := runScheduledRelayTreeFixtureBytes(t, file)
			if !bytes.Equal(first, second) {
				t.Fatalf("fixed-input metrics differ for %s", file)
			}
		})
	}
}

func TestIT061_ThreeRelayExceptionAddsCoverageBeyondDepthTwo(t *testing.T) {
	path := scheduledRelayTreeFixturePath(t, "three-relay-exception.json")
	_, runnable, err := loadScenarioConfig(path)
	if err != nil {
		t.Fatalf("load config failed: %v", err)
	}
	cfg := *runnable.ScheduledRelayTree
	cfg.InitialPlan.Constraints.MaxRelayDepth = 2
	_, err = scenario.RunScheduledRelayTreeScenario(cfg)
	if !errors.Is(err, routing.ErrNoFeasibleRelayTree) {
		t.Fatalf("max depth 2 error = %v, want ErrNoFeasibleRelayTree", err)
	}
}

func runScheduledRelayTreeFixture(t *testing.T, file string) scenario.MetricsReport {
	t.Helper()
	raw := runScheduledRelayTreeFixtureBytes(t, file)
	var report scenario.MetricsReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("decode metrics failed: %v", err)
	}
	return report
}

func runScheduledRelayTreeFixtureBytes(t *testing.T, file string) []byte {
	t.Helper()
	metricsPath := filepath.Join(t.TempDir(), "metrics.json")
	if err := run([]string{"run", scheduledRelayTreeFixturePath(t, file), "--metrics", metricsPath}); err != nil {
		t.Fatalf("run %s failed: %v", file, err)
	}
	raw, err := os.ReadFile(metricsPath)
	if err != nil {
		t.Fatalf("read metrics %s failed: %v", metricsPath, err)
	}
	return raw
}

func scheduledRelayTreeFixturePath(t *testing.T, file string) string {
	t.Helper()
	path := filepath.Join("..", "..", "configs", "scenarios", "relay-tree", file)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("fixture %s unavailable: %v", path, err)
	}
	return path
}
