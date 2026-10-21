package main

import (
	"os"
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

func TestIT068_TimingComparisonLoadsAllRelayTreeFixturesWithoutSchemaChanges(t *testing.T) {
	files := []string{
		"direct.json",
		"one-relay.json",
		"two-relay.json",
		"three-relay-exception.json",
		"branch-narrow.json",
		"branch-wide.json",
		"partition-recovery.json",
	}
	fixtures := make([]scenario.TimingProfileComparisonFixture, 0, len(files))
	for _, file := range files {
		raw, err := os.ReadFile(scheduledRelayTreeFixturePath(t, file))
		if err != nil {
			t.Fatalf("read %s failed: %v", file, err)
		}
		cfg, err := decodeScheduledRelayTreeScenarioConfig(raw)
		if err != nil {
			t.Fatalf("decode unchanged fixture %s failed: %v", file, err)
		}
		fixtures = append(fixtures, scenario.TimingProfileComparisonFixture{
			FixtureID:     strings.TrimSuffix(file, ".json"),
			BaseConfig:    cfg,
			TimingOverlay: timingComparisonFixtureOverlay(strings.TrimSuffix(file, ".json")),
		})
	}

	report, err := scenario.RunTimingProfileComparison(fixtures)
	if err != nil {
		t.Fatalf("28-row comparison failed: %v", err)
	}
	if len(report.Rows) != 28 {
		t.Fatalf("row count = %d, want 28", len(report.Rows))
	}
	wantSuffixes := []string{
		"/source_led/project_native_variable",
		"/source_led/dmr_inspired_30ms",
		"/group_leader/project_native_variable",
		"/group_leader/dmr_inspired_30ms",
	}
	for i, row := range report.Rows {
		if !strings.HasSuffix(row.RowID, wantSuffixes[i%len(wantSuffixes)]) {
			t.Fatalf("row[%d]=%q does not match canonical suffix %q", i, row.RowID, wantSuffixes[i%len(wantSuffixes)])
		}
	}
	partitionRows := 0
	fitRows := 0
	dmrFitRows := 0
	for _, row := range report.Rows {
		if row.FixtureID == "partition-recovery" {
			partitionRows++
		}
		if row.ProfileFit {
			fitRows++
			if row.Profile == scenario.TimingProfileDMRInspired30MS {
				dmrFitRows++
			}
		}
	}
	if partitionRows != 4 {
		t.Fatalf("partition-recovery rows = %d, want 4", partitionRows)
	}
	if fitRows != 22 || dmrFitRows != 8 {
		t.Fatalf("fit rows total/DMR = %d/%d, want 22/8", fitRows, dmrFitRows)
	}
	if report.Recommendation != "insufficient_evidence" || report.Evidence.Outcome != "degraded" {
		t.Fatalf("recommendation/outcome = %q/%q, want insufficient_evidence/degraded", report.Recommendation, report.Evidence.Outcome)
	}
}

func timingComparisonFixtureOverlay(fixtureID string) scenario.TimingProfileComparisonOverlay {
	peerIDs := map[string][]uint16{
		"direct":                {1, 2, 3},
		"one-relay":             {1, 2, 10},
		"two-relay":             {1, 2, 10, 11},
		"three-relay-exception": {1, 2, 10, 11, 12},
		"branch-narrow":         {1, 2, 3, 10},
		"branch-wide":           {1, 2, 3, 10, 11},
		"partition-recovery":    {1, 2},
	}[fixtureID]
	peers := make([]scenario.TimingAuthorityPeer, 0, len(peerIDs))
	for _, stationID := range peerIDs {
		peers = append(peers, scenario.TimingAuthorityPeer{
			StationID:     stationID,
			TimingCapable: true,
			LastSeenMS:    1_000,
			FreshnessMS:   10_000,
			BatteryPct:    90,
		})
	}
	return scenario.TimingProfileComparisonOverlay{
		Peers:               peers,
		MinTimingBatteryPct: 20,
		SyncToleranceMS:     1,
	}
}
