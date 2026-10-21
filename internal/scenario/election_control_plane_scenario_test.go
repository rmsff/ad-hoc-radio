package scenario

import (
	"reflect"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/election"
)

func TestIT013_ControlPlaneScenarioEmitsBoundedEvents(t *testing.T) {
	cfg := defaultRelayElectionControlPlaneScenarioConfig()
	report, err := RunRelayElectionControlPlaneScenario(cfg)
	if err != nil {
		t.Fatalf("RunRelayElectionControlPlaneScenario failed: %v", err)
	}

	if report.EventCounts.Beacon != 4 {
		t.Fatalf("Beacon count = %d, want 4", report.EventCounts.Beacon)
	}
	if report.EventCounts.RelayOffer != 4 {
		t.Fatalf("RelayOffer count = %d, want 4", report.EventCounts.RelayOffer)
	}
	if report.EventCounts.RelayAssign != 2 {
		t.Fatalf("RelayAssign count = %d, want 2", report.EventCounts.RelayAssign)
	}
	if report.EventCounts.Reelection != 1 {
		t.Fatalf("Reelection count = %d, want 1", report.EventCounts.Reelection)
	}
	if len(report.Assignments) != 2 {
		t.Fatalf("Assignments = %d, want 2", len(report.Assignments))
	}
	for _, assignment := range report.Assignments {
		if assignment.ActiveRelayCount != 1 {
			t.Fatalf("snapshot %d ActiveRelayCount = %d, want 1", assignment.SnapshotIndex, assignment.ActiveRelayCount)
		}
		if assignment.BeaconCount > len(cfg.Snapshots[assignment.SnapshotIndex].Candidates) {
			t.Fatalf("snapshot %d BeaconCount = %d exceeds candidates", assignment.SnapshotIndex, assignment.BeaconCount)
		}
		if assignment.OfferCount > assignment.BeaconCount {
			t.Fatalf("snapshot %d OfferCount = %d exceeds BeaconCount = %d", assignment.SnapshotIndex, assignment.OfferCount, assignment.BeaconCount)
		}
	}
	if report.Assignments[0].RelayID != 2 {
		t.Fatalf("initial assignment RelayID = %d, want 2", report.Assignments[0].RelayID)
	}
	if report.Assignments[1].RelayID != 3 {
		t.Fatalf("handover assignment RelayID = %d, want 3", report.Assignments[1].RelayID)
	}
}

func TestIT013_ControlPlaneScenarioSelectedOfferScoreMatchesAssignment(t *testing.T) {
	report, err := RunRelayElectionControlPlaneScenario(defaultRelayElectionControlPlaneScenarioConfig())
	if err != nil {
		t.Fatalf("RunRelayElectionControlPlaneScenario failed: %v", err)
	}

	assignmentsBySnapshot := make(map[int]RelayElectionAssignment, len(report.Assignments))
	for _, assignment := range report.Assignments {
		assignmentsBySnapshot[assignment.SnapshotIndex] = assignment
	}
	for _, event := range report.ControlEvents {
		if event.Type != RelayElectionControlEventRelayOffer || !event.Selected {
			continue
		}
		assignment, ok := assignmentsBySnapshot[event.SnapshotIndex]
		if !ok {
			t.Fatalf("selected relay_offer for snapshot %d has no assignment", event.SnapshotIndex)
		}
		if event.RelayID != assignment.RelayID {
			t.Fatalf("snapshot %d selected offer relay_id=%d, assignment relay_id=%d", event.SnapshotIndex, event.RelayID, assignment.RelayID)
		}
		if event.Score != assignment.Score {
			t.Fatalf("snapshot %d selected offer score=%v, assignment score=%v", event.SnapshotIndex, event.Score, assignment.Score)
		}
	}
}

func TestIT013_ControlPlaneScenarioVoiceTraceUsesOnlyAssignedRelay(t *testing.T) {
	report, err := RunRelayElectionControlPlaneScenario(defaultRelayElectionControlPlaneScenarioConfig())
	if err != nil {
		t.Fatalf("RunRelayElectionControlPlaneScenario failed: %v", err)
	}

	if len(report.VoiceForwardingTrace) != 8 {
		t.Fatalf("VoiceForwardingTrace rows = %d, want 8", len(report.VoiceForwardingTrace))
	}
	if report.UnassignedRelayForwardCount != 0 {
		t.Fatalf("UnassignedRelayForwardCount = %d, want 0", report.UnassignedRelayForwardCount)
	}
	if report.UnselectedRelayVoiceForwardCount != 0 {
		t.Fatalf("UnselectedRelayVoiceForwardCount = %d, want 0", report.UnselectedRelayVoiceForwardCount)
	}
	for _, row := range report.VoiceForwardingTrace {
		if row.ActiveRelayCount != 1 {
			t.Fatalf("seq %d ActiveRelayCount = %d, want 1", row.Seq, row.ActiveRelayCount)
		}
		if row.AssignedRelayID != row.ForwardingRelayID {
			t.Fatalf("seq %d AssignedRelayID = %d, ForwardingRelayID = %d", row.Seq, row.AssignedRelayID, row.ForwardingRelayID)
		}
		if row.Seq < 4 && row.ForwardingRelayID != 2 {
			t.Fatalf("seq %d ForwardingRelayID = %d, want 2 before handover", row.Seq, row.ForwardingRelayID)
		}
		if row.Seq >= 4 && row.ForwardingRelayID != 3 {
			t.Fatalf("seq %d ForwardingRelayID = %d, want 3 after handover", row.Seq, row.ForwardingRelayID)
		}
	}
}

func TestIT013_ControlPlaneScenarioReelectionCausesHandoverAssignment(t *testing.T) {
	report, err := RunRelayElectionControlPlaneScenario(defaultRelayElectionControlPlaneScenarioConfig())
	if err != nil {
		t.Fatalf("RunRelayElectionControlPlaneScenario failed: %v", err)
	}

	if report.HandoverCount != 1 {
		t.Fatalf("HandoverCount = %d, want 1", report.HandoverCount)
	}
	secondAssignIndex := -1
	reelectionIndex := -1
	for _, event := range report.ControlEvents {
		if event.SnapshotIndex != 1 {
			continue
		}
		switch event.Type {
		case RelayElectionControlEventReelection:
			reelectionIndex = event.Index
		case RelayElectionControlEventRelayAssign:
			secondAssignIndex = event.Index
			if event.RelayID != 3 {
				t.Fatalf("second relay_assign RelayID = %d, want 3", event.RelayID)
			}
		}
	}
	if reelectionIndex < 0 {
		t.Fatal("missing reelection event for second snapshot")
	}
	if secondAssignIndex < 0 {
		t.Fatal("missing relay_assign event for second snapshot")
	}
	if reelectionIndex >= secondAssignIndex {
		t.Fatalf("reelection index = %d, relay_assign index = %d, want reelection before assign", reelectionIndex, secondAssignIndex)
	}
	if !report.Assignments[1].Handover {
		t.Fatal("second assignment Handover = false, want true")
	}
	if report.Assignments[1].PreviousRelayID != 2 {
		t.Fatalf("second assignment PreviousRelayID = %d, want 2", report.Assignments[1].PreviousRelayID)
	}
}

func TestIT013_ControlPlaneScenarioOrdersControlEvents(t *testing.T) {
	report, err := RunRelayElectionControlPlaneScenario(defaultRelayElectionControlPlaneScenarioConfig())
	if err != nil {
		t.Fatalf("RunRelayElectionControlPlaneScenario failed: %v", err)
	}

	got := make([]string, 0, len(report.ControlEvents))
	for _, event := range report.ControlEvents {
		if event.SnapshotIndex == 1 {
			got = append(got, event.Type)
		}
	}
	want := []string{
		RelayElectionControlEventReelection,
		RelayElectionControlEventBeacon,
		RelayElectionControlEventBeacon,
		RelayElectionControlEventRelayOffer,
		RelayElectionControlEventRelayOffer,
		RelayElectionControlEventRelayAssign,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot 1 event order = %v, want %v", got, want)
	}
}

func TestIT013_ControlPlaneScenarioSupportsOneSnapshotOneFrame(t *testing.T) {
	cfg := RelayElectionControlPlaneScenarioConfig{
		ScenarioID: RelayElectionControlPlaneScenarioID,
		Seed:       DefaultScenarioSeed,
		SourceID:   DefaultSourceID,
		ReceiverID: relayElectionReceiverID,
		FrameCount: 1,
		Policy:     election.DefaultPolicy(),
		Snapshots: []RelayElectionSnapshot{
			{
				StartSeq: 0,
				Candidates: []election.Candidate{
					{
						NodeID:            DefaultRelayID,
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

	report, err := RunRelayElectionControlPlaneScenario(cfg)
	if err != nil {
		t.Fatalf("RunRelayElectionControlPlaneScenario failed: %v", err)
	}
	if report.EventCounts.Reelection != 0 {
		t.Fatalf("Reelection count = %d, want 0", report.EventCounts.Reelection)
	}
	if report.EventCounts.RelayAssign != 1 {
		t.Fatalf("RelayAssign count = %d, want 1", report.EventCounts.RelayAssign)
	}
	if len(report.Assignments) != 1 {
		t.Fatalf("Assignments = %d, want 1", len(report.Assignments))
	}
	if report.Assignments[0].EndSeqExclusive != 1 {
		t.Fatalf("EndSeqExclusive = %d, want 1", report.Assignments[0].EndSeqExclusive)
	}
	if len(report.VoiceForwardingTrace) != 1 {
		t.Fatalf("VoiceForwardingTrace rows = %d, want 1", len(report.VoiceForwardingTrace))
	}
	if report.VoiceForwardingTrace[0].ForwardingRelayID != DefaultRelayID {
		t.Fatalf("ForwardingRelayID = %d, want %d", report.VoiceForwardingTrace[0].ForwardingRelayID, DefaultRelayID)
	}
}

func TestIT013_ControlPlaneScenarioDeterministicReplay(t *testing.T) {
	cfg := defaultRelayElectionControlPlaneScenarioConfig()
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

func TestIT013_ControlPlaneScenarioRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  RelayElectionControlPlaneScenarioConfig
	}{
		{name: "too many candidates", cfg: func() RelayElectionControlPlaneScenarioConfig {
			cfg := defaultRelayElectionControlPlaneScenarioConfig()
			cfg.Snapshots[0].Candidates = make([]election.Candidate, maxRelayElectionControlCandidatesPerSnapshot+1)
			for i := range cfg.Snapshots[0].Candidates {
				cfg.Snapshots[0].Candidates[i] = election.Candidate{
					NodeID:            uint16(i + 10),
					RelayCapable:      true,
					RelayEnabled:      true,
					QualityFromSource: 1,
					GroupCoverage:     1,
					Battery:           1,
					Stability:         1,
				}
			}
			return cfg
		}()},
		{name: "control event upper bound exceeds cap", cfg: func() RelayElectionControlPlaneScenarioConfig {
			cfg := defaultRelayElectionControlPlaneScenarioConfig()
			cfg.FrameCount = maxRelayElectionControlEvents
			cfg.Snapshots = make([]RelayElectionSnapshot, maxRelayElectionControlEvents/3+1)
			for i := range cfg.Snapshots {
				cfg.Snapshots[i] = RelayElectionSnapshot{
					StartSeq: uint32(i),
					Candidates: []election.Candidate{
						{
							NodeID:            DefaultRelayID,
							RelayCapable:      true,
							RelayEnabled:      true,
							QualityFromSource: 1,
							GroupCoverage:     1,
							Battery:           1,
							Stability:         1,
						},
					},
				}
			}
			return cfg
		}()},
		{name: "zero source id", cfg: func() RelayElectionControlPlaneScenarioConfig {
			cfg := defaultRelayElectionControlPlaneScenarioConfig()
			cfg.SourceID = 0
			return cfg
		}()},
		{name: "empty snapshot candidates", cfg: func() RelayElectionControlPlaneScenarioConfig {
			cfg := defaultRelayElectionControlPlaneScenarioConfig()
			cfg.Snapshots[0].Candidates = nil
			return cfg
		}()},
		{name: "receiver as relay candidate", cfg: func() RelayElectionControlPlaneScenarioConfig {
			cfg := defaultRelayElectionControlPlaneScenarioConfig()
			cfg.Snapshots[0].Candidates[0].NodeID = cfg.ReceiverID
			return cfg
		}()},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := RunRelayElectionControlPlaneScenario(tc.cfg); err == nil {
				t.Fatal("RunRelayElectionControlPlaneScenario accepted invalid config")
			}
		})
	}
}

func TestIT013_ControlPlaneCounterHelpersDetectSyntheticViolations(t *testing.T) {
	assignments := []RelayElectionAssignment{
		{SnapshotIndex: 0, RelayID: 2},
	}
	trace := []RelayElectionControlVoiceForward{
		{SnapshotIndex: 0, AssignedRelayID: 2, ForwardingRelayID: 2, ActiveRelayCount: 1},
		{SnapshotIndex: 0, AssignedRelayID: 2, ForwardingRelayID: 3, ActiveRelayCount: 1},
		{SnapshotIndex: 1, AssignedRelayID: 4, ForwardingRelayID: 4, ActiveRelayCount: 1},
		{SnapshotIndex: 2, AssignedRelayID: 0, ForwardingRelayID: 0, ActiveRelayCount: 0},
	}
	if got := countUnassignedControlForwards(assignments, trace); got != 2 {
		t.Fatalf("countUnassignedControlForwards = %d, want 2", got)
	}

	unselectedTrace := []RelayElectionControlVoiceForward{
		{UnselectedRelayForwardCount: 2},
		{UnselectedRelayForwardCount: 3},
	}
	if got := countUnselectedRelayVoiceForwards(unselectedTrace); got != 5 {
		t.Fatalf("countUnselectedRelayVoiceForwards = %d, want 5", got)
	}
}

func defaultRelayElectionControlPlaneScenarioConfig() RelayElectionControlPlaneScenarioConfig {
	return DefaultRelayElectionControlPlaneScenarioConfig()
}
