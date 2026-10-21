package scenario

import (
	"reflect"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/election"
)

func TestIT014_DirectCoverageSkipsRelayElection(t *testing.T) {
	cfg := directCoverageScenarioConfig()
	cfg.Snapshots[0].Coverage = RelayElectionSnapshotCoverage{
		SourceDirectReachable: []uint16{2, 3, 4},
	}

	report, err := RunRelayElectionControlPlaneScenario(cfg)
	if err != nil {
		t.Fatalf("RunRelayElectionControlPlaneScenario failed: %v", err)
	}

	if report.EventCounts.RelayAssign != 0 {
		t.Fatalf("RelayAssign count = %d, want 0", report.EventCounts.RelayAssign)
	}
	if report.EventCounts.RelayOffer != 0 {
		t.Fatalf("RelayOffer count = %d, want 0", report.EventCounts.RelayOffer)
	}
	assertSingleCoveragePrediction(t, report, RelayElectionVoiceModeDirectOnly, false, 0)
	prediction := report.CoveragePredictions[0]
	assertUint16Slice(t, "CoveredDirect", prediction.CoveredDirect, []uint16{2, 3, 4})
	assertUint16Slice(t, "CoveredViaRelay", prediction.CoveredViaRelay, nil)
	assertUint16Slice(t, "UncoveredMembers", prediction.UncoveredMembers, nil)
	assertUint16Slice(t, "UnknownMembers", prediction.UnknownMembers, nil)
	assertDirectOnlyVoiceTrace(t, report)
}

func TestIT014_RelayCompletesCoverage(t *testing.T) {
	cfg := directCoverageScenarioConfig()
	cfg.Snapshots[0].Coverage = RelayElectionSnapshotCoverage{
		SourceDirectReachable: []uint16{2},
		RelayReachable: []RelayElectionRelayReachability{
			{RelayID: 5, Members: []uint16{3, 4}},
		},
	}

	report, err := RunRelayElectionControlPlaneScenario(cfg)
	if err != nil {
		t.Fatalf("RunRelayElectionControlPlaneScenario failed: %v", err)
	}

	if report.EventCounts.RelayAssign != 1 {
		t.Fatalf("RelayAssign count = %d, want 1", report.EventCounts.RelayAssign)
	}
	assertSingleCoveragePrediction(t, report, RelayElectionVoiceModeRelayed, true, 5)
	prediction := report.CoveragePredictions[0]
	assertUint16Slice(t, "CoveredDirect", prediction.CoveredDirect, []uint16{2})
	assertUint16Slice(t, "CoveredViaRelay", prediction.CoveredViaRelay, []uint16{3, 4})
	assertUint16Slice(t, "UncoveredMembers", prediction.UncoveredMembers, nil)
	assertUint16Slice(t, "UnknownMembers", prediction.UnknownMembers, nil)
	if report.Assignments[0].RelayID != 5 {
		t.Fatalf("RelayID = %d, want 5", report.Assignments[0].RelayID)
	}
}

func TestIT014_PartialCoverageReportsGaps(t *testing.T) {
	cfg := directCoverageScenarioConfig()
	cfg.Snapshots[0].Coverage = RelayElectionSnapshotCoverage{
		SourceDirectReachable: []uint16{2},
		RelayReachable: []RelayElectionRelayReachability{
			{RelayID: 5, Members: []uint16{3}},
		},
		Unreachable: []uint16{4},
	}

	report, err := RunRelayElectionControlPlaneScenario(cfg)
	if err != nil {
		t.Fatalf("RunRelayElectionControlPlaneScenario failed: %v", err)
	}

	assertSingleCoveragePrediction(t, report, RelayElectionVoiceModeRelayed, true, 5)
	prediction := report.CoveragePredictions[0]
	assertUint16Slice(t, "CoveredDirect", prediction.CoveredDirect, []uint16{2})
	assertUint16Slice(t, "CoveredViaRelay", prediction.CoveredViaRelay, []uint16{3})
	assertUint16Slice(t, "UncoveredMembers", prediction.UncoveredMembers, []uint16{4})
	assertUint16Slice(t, "UnknownMembers", prediction.UnknownMembers, nil)
}

func TestIT014_NoUsefulRelayStaysExplicit(t *testing.T) {
	cfg := directCoverageScenarioConfig()
	cfg.Snapshots[0].Coverage = RelayElectionSnapshotCoverage{
		SourceDirectReachable: []uint16{2},
		RelayReachable: []RelayElectionRelayReachability{
			{RelayID: 5, Members: []uint16{2}},
		},
		Unreachable: []uint16{3},
	}

	report, err := RunRelayElectionControlPlaneScenario(cfg)
	if err != nil {
		t.Fatalf("RunRelayElectionControlPlaneScenario failed: %v", err)
	}

	if report.EventCounts.RelayAssign != 0 {
		t.Fatalf("RelayAssign count = %d, want 0", report.EventCounts.RelayAssign)
	}
	if report.EventCounts.RelayOffer != 0 {
		t.Fatalf("RelayOffer count = %d, want 0", report.EventCounts.RelayOffer)
	}
	assertSingleCoveragePrediction(t, report, RelayElectionVoiceModeDirectOnlyDegraded, true, 0)
	prediction := report.CoveragePredictions[0]
	assertUint16Slice(t, "CoveredDirect", prediction.CoveredDirect, []uint16{2})
	assertUint16Slice(t, "CoveredViaRelay", prediction.CoveredViaRelay, nil)
	assertUint16Slice(t, "UncoveredMembers", prediction.UncoveredMembers, []uint16{3})
	assertUint16Slice(t, "UnknownMembers", prediction.UnknownMembers, []uint16{4})
	assertDirectOnlyVoiceTrace(t, report)
}

func TestIT014_OpenGroupPreservesExistingElectionBehavior(t *testing.T) {
	cfg := directCoverageScenarioConfig()
	cfg.Coverage = RelayElectionCoverageConfig{
		Mode:                  RelayElectionCoverageModeOpenGroup,
		ExpectedMembers:       []uint16{1, 2, 3, 4},
		RequiredGroupCoverage: 1,
	}
	cfg.Snapshots[0].Coverage = RelayElectionSnapshotCoverage{
		SourceDirectReachable: []uint16{2, 3, 4},
	}

	report, err := RunRelayElectionControlPlaneScenario(cfg)
	if err != nil {
		t.Fatalf("RunRelayElectionControlPlaneScenario failed: %v", err)
	}

	if report.EventCounts.RelayAssign != 1 {
		t.Fatalf("RelayAssign count = %d, want 1", report.EventCounts.RelayAssign)
	}
	if len(report.CoveragePredictions) != 0 {
		t.Fatalf("CoveragePredictions = %d, want 0 for open-group guard", len(report.CoveragePredictions))
	}
	if report.VoiceForwardingTrace[0].ForwardingRelayID != 5 {
		t.Fatalf("ForwardingRelayID = %d, want 5", report.VoiceForwardingTrace[0].ForwardingRelayID)
	}
}

func TestIT014_OpenGroupIgnoresCoverageEvidenceOutsideRoster(t *testing.T) {
	cfg := directCoverageScenarioConfig()
	cfg.Coverage = RelayElectionCoverageConfig{
		Mode:                  RelayElectionCoverageModeOpenGroup,
		ExpectedMembers:       []uint16{1},
		RequiredGroupCoverage: 1,
	}
	cfg.Snapshots[0].Coverage = RelayElectionSnapshotCoverage{
		SourceDirectReachable: []uint16{99},
		RelayReachable: []RelayElectionRelayReachability{
			{RelayID: 5, Members: []uint16{99}},
		},
		Unreachable: []uint16{100},
	}

	report, err := RunRelayElectionControlPlaneScenario(cfg)
	if err != nil {
		t.Fatalf("RunRelayElectionControlPlaneScenario failed: %v", err)
	}
	if report.EventCounts.RelayAssign != 1 {
		t.Fatalf("RelayAssign count = %d, want 1", report.EventCounts.RelayAssign)
	}
	if len(report.CoveragePredictions) != 0 {
		t.Fatalf("CoveragePredictions = %d, want 0 for open-group guard", len(report.CoveragePredictions))
	}
}

func TestIT014_DirectCoverageDeterministicReplay(t *testing.T) {
	cfg := directCoverageScenarioConfig()
	cfg.Snapshots[0].Coverage = RelayElectionSnapshotCoverage{
		SourceDirectReachable: []uint16{2},
		RelayReachable: []RelayElectionRelayReachability{
			{RelayID: 5, Members: []uint16{3}},
		},
		Unreachable: []uint16{4},
	}

	first, err := RunRelayElectionControlPlaneScenario(cfg)
	if err != nil {
		t.Fatalf("first RunRelayElectionControlPlaneScenario failed: %v", err)
	}
	second, err := RunRelayElectionControlPlaneScenario(cfg)
	if err != nil {
		t.Fatalf("second RunRelayElectionControlPlaneScenario failed: %v", err)
	}

	if first.EventHash == "" {
		t.Fatal("EventHash is empty")
	}
	if first.EventHash != second.EventHash {
		t.Fatalf("EventHash mismatch:\nfirst=%s\nsecond=%s", first.EventHash, second.EventHash)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("same config produced different reports:\nfirst=%+v\nsecond=%+v", first, second)
	}
}

func TestIT014_RejectsInvalidCoverageConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  RelayElectionControlPlaneScenarioConfig
	}{
		{name: "invalid coverage mode", cfg: func() RelayElectionControlPlaneScenarioConfig {
			cfg := directCoverageScenarioConfig()
			cfg.Coverage.Mode = "closed-ish"
			return cfg
		}()},
		{name: "duplicate expected member", cfg: func() RelayElectionControlPlaneScenarioConfig {
			cfg := directCoverageScenarioConfig()
			cfg.Coverage.ExpectedMembers = []uint16{DefaultSourceID, 2, 2, 3, relayElectionReceiverID}
			return cfg
		}()},
		{name: "source direct member outside roster", cfg: func() RelayElectionControlPlaneScenarioConfig {
			cfg := directCoverageScenarioConfig()
			cfg.Snapshots[0].Coverage.SourceDirectReachable = []uint16{99}
			return cfg
		}()},
		{name: "relay reachability for non-candidate", cfg: func() RelayElectionControlPlaneScenarioConfig {
			cfg := directCoverageScenarioConfig()
			cfg.Snapshots[0].Coverage.RelayReachable = []RelayElectionRelayReachability{
				{RelayID: 99, Members: []uint16{2}},
			}
			return cfg
		}()},
		{name: "member both direct reachable and unreachable", cfg: func() RelayElectionControlPlaneScenarioConfig {
			cfg := directCoverageScenarioConfig()
			cfg.Snapshots[0].Coverage.SourceDirectReachable = []uint16{2}
			cfg.Snapshots[0].Coverage.Unreachable = []uint16{2}
			return cfg
		}()},
		{name: "member both relay reachable and unreachable", cfg: func() RelayElectionControlPlaneScenarioConfig {
			cfg := directCoverageScenarioConfig()
			cfg.Snapshots[0].Coverage.RelayReachable = []RelayElectionRelayReachability{
				{RelayID: 5, Members: []uint16{2}},
			}
			cfg.Snapshots[0].Coverage.Unreachable = []uint16{2}
			return cfg
		}()},
		{name: "duplicate relay reachability relay id", cfg: func() RelayElectionControlPlaneScenarioConfig {
			cfg := directCoverageScenarioConfig()
			cfg.Snapshots[0].Coverage.RelayReachable = []RelayElectionRelayReachability{
				{RelayID: 5, Members: []uint16{2}},
				{RelayID: 5, Members: []uint16{3}},
			}
			return cfg
		}()},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := RunRelayElectionControlPlaneScenario(tc.cfg); err == nil {
				t.Fatal("RunRelayElectionControlPlaneScenario accepted invalid coverage config")
			}
		})
	}
}

func directCoverageScenarioConfig() RelayElectionControlPlaneScenarioConfig {
	return RelayElectionControlPlaneScenarioConfig{
		ScenarioID: RelayElectionControlPlaneScenarioID,
		Seed:       DefaultScenarioSeed,
		SourceID:   DefaultSourceID,
		ReceiverID: relayElectionReceiverID,
		FrameCount: 3,
		Policy:     election.DefaultPolicy(),
		Coverage: RelayElectionCoverageConfig{
			Mode:                  RelayElectionCoverageModeClosedGroup,
			ExpectedMembers:       []uint16{DefaultSourceID, 2, 3, relayElectionReceiverID},
			RequiredGroupCoverage: 1,
		},
		Snapshots: []RelayElectionSnapshot{
			{
				StartSeq: 0,
				Candidates: []election.Candidate{
					{
						NodeID:            5,
						RelayCapable:      true,
						RelayEnabled:      true,
						QualityFromSource: 1,
						GroupCoverage:     1,
						Battery:           1,
						Stability:         1,
					},
				},
			},
		},
	}
}

func assertSingleCoveragePrediction(t *testing.T, report RelayElectionControlPlaneReport, voiceMode string, relayRequired bool, selectedRelayID uint16) {
	t.Helper()
	if len(report.CoveragePredictions) != 1 {
		t.Fatalf("CoveragePredictions = %d, want 1", len(report.CoveragePredictions))
	}
	if len(report.Assignments) != 1 {
		t.Fatalf("Assignments = %d, want 1", len(report.Assignments))
	}
	prediction := report.CoveragePredictions[0]
	if prediction.VoiceMode != voiceMode {
		t.Fatalf("CoveragePrediction VoiceMode = %q, want %q", prediction.VoiceMode, voiceMode)
	}
	if prediction.RelayRequired != relayRequired {
		t.Fatalf("CoveragePrediction RelayRequired = %v, want %v", prediction.RelayRequired, relayRequired)
	}
	if prediction.SelectedRelayID != selectedRelayID {
		t.Fatalf("CoveragePrediction SelectedRelayID = %d, want %d", prediction.SelectedRelayID, selectedRelayID)
	}
	assignment := report.Assignments[0]
	if assignment.VoiceMode != voiceMode {
		t.Fatalf("Assignment VoiceMode = %q, want %q", assignment.VoiceMode, voiceMode)
	}
	if assignment.RelayRequired != relayRequired {
		t.Fatalf("Assignment RelayRequired = %v, want %v", assignment.RelayRequired, relayRequired)
	}
	if assignment.RelayID != selectedRelayID {
		t.Fatalf("Assignment RelayID = %d, want %d", assignment.RelayID, selectedRelayID)
	}
}

func assertDirectOnlyVoiceTrace(t *testing.T, report RelayElectionControlPlaneReport) {
	t.Helper()
	for _, row := range report.VoiceForwardingTrace {
		if row.AssignedRelayID != 0 || row.ForwardingRelayID != 0 || row.ActiveRelayCount != 0 {
			t.Fatalf("seq %d uses relay assignment %+v, want direct-only no relay", row.Seq, row)
		}
	}
	if report.UnassignedRelayForwardCount != 0 {
		t.Fatalf("UnassignedRelayForwardCount = %d, want 0 for direct-only voice", report.UnassignedRelayForwardCount)
	}
}

func assertUint16Slice(t *testing.T, name string, got, want []uint16) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}
