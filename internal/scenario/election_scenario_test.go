package scenario

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/election"
)

func TestIT012_RelayElectionScenarioSelectsOneRelayPerSnapshot(t *testing.T) {
	report, err := RunRelayElectionScenario(defaultRelayElectionScenarioConfig())
	if err != nil {
		t.Fatalf("RunRelayElectionScenario failed: %v", err)
	}

	if len(report.Decisions) != 2 {
		t.Fatalf("decisions = %d, want 2", len(report.Decisions))
	}
	if report.Decisions[0].RelayID != 2 {
		t.Fatalf("initial RelayID = %d, want 2", report.Decisions[0].RelayID)
	}
	if report.Decisions[1].RelayID != 3 {
		t.Fatalf("handover RelayID = %d, want 3", report.Decisions[1].RelayID)
	}
	for _, decision := range report.Decisions {
		if decision.ActiveRelayCount != 1 {
			t.Fatalf("decision at seq %d ActiveRelayCount = %d, want 1", decision.StartSeq, decision.ActiveRelayCount)
		}
	}
}

func TestIT012_RelayElectionScenarioTraceUsesOnlyAssignedRelay(t *testing.T) {
	report, err := RunRelayElectionScenario(defaultRelayElectionScenarioConfig())
	if err != nil {
		t.Fatalf("RunRelayElectionScenario failed: %v", err)
	}

	if len(report.ForwardingTrace) != 8 {
		t.Fatalf("forwarding trace rows = %d, want 8", len(report.ForwardingTrace))
	}
	if report.UnassignedRelayForwardCount != 0 {
		t.Fatalf("UnassignedRelayForwardCount = %d, want 0", report.UnassignedRelayForwardCount)
	}
	for _, row := range report.ForwardingTrace {
		if row.ActiveRelayCount != 1 {
			t.Fatalf("seq %d ActiveRelayCount = %d, want 1", row.Seq, row.ActiveRelayCount)
		}
		switch {
		case row.Seq < 4 && row.RelayID != 2:
			t.Fatalf("seq %d RelayID = %d, want 2 before handover", row.Seq, row.RelayID)
		case row.Seq >= 4 && row.RelayID != 3:
			t.Fatalf("seq %d RelayID = %d, want 3 after handover", row.Seq, row.RelayID)
		}
	}
}

func TestIT012_RelayElectionScenarioCountsHandover(t *testing.T) {
	report, err := RunRelayElectionScenario(defaultRelayElectionScenarioConfig())
	if err != nil {
		t.Fatalf("RunRelayElectionScenario failed: %v", err)
	}

	if report.HandoverCount != 1 {
		t.Fatalf("HandoverCount = %d, want 1", report.HandoverCount)
	}
	if report.Decisions[1].PreviousRelayID != 2 {
		t.Fatalf("PreviousRelayID = %d, want 2", report.Decisions[1].PreviousRelayID)
	}
}

func TestIT012_RelayElectionScenarioDeterministicReplay(t *testing.T) {
	cfg := defaultRelayElectionScenarioConfig()
	first, err := RunRelayElectionScenario(cfg)
	if err != nil {
		t.Fatalf("first RunRelayElectionScenario failed: %v", err)
	}
	second, err := RunRelayElectionScenario(cfg)
	if err != nil {
		t.Fatalf("second RunRelayElectionScenario failed: %v", err)
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

func TestIT012_RelayElectionScenarioUsesExplicitZeroPolicy(t *testing.T) {
	cfg := RelayElectionScenarioConfig{
		ScenarioID: RelayElectionScenarioID,
		Seed:       DefaultScenarioSeed,
		SourceID:   DefaultSourceID,
		ReceiverID: relayElectionReceiverID,
		FrameCount: 1,
		Policy:     election.Policy{},
		Snapshots: []RelayElectionSnapshot{
			{
				StartSeq: 0,
				Candidates: []election.Candidate{
					{
						NodeID:            DefaultRelayID,
						RelayCapable:      true,
						RelayEnabled:      true,
						QualityFromSource: 0.10,
						GroupCoverage:     1,
						Battery:           1,
						Stability:         1,
					},
				},
			},
		},
	}

	report, err := RunRelayElectionScenario(cfg)
	if err != nil {
		t.Fatalf("RunRelayElectionScenario rejected explicit zero policy: %v", err)
	}
	if report.Decisions[0].RelayID != DefaultRelayID {
		t.Fatalf("RelayID = %d, want %d", report.Decisions[0].RelayID, DefaultRelayID)
	}
}

func TestIT012_RelayElectionScenarioRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  RelayElectionScenarioConfig
	}{
		{name: "zero frame count", cfg: func() RelayElectionScenarioConfig {
			cfg := defaultRelayElectionScenarioConfig()
			cfg.FrameCount = 0
			return cfg
		}()},
		{name: "zero source id", cfg: func() RelayElectionScenarioConfig {
			cfg := defaultRelayElectionScenarioConfig()
			cfg.SourceID = 0
			return cfg
		}()},
		{name: "zero receiver id", cfg: func() RelayElectionScenarioConfig {
			cfg := defaultRelayElectionScenarioConfig()
			cfg.ReceiverID = 0
			return cfg
		}()},
		{name: "frame count exceeds trace cap", cfg: func() RelayElectionScenarioConfig {
			cfg := defaultRelayElectionScenarioConfig()
			cfg.FrameCount = maxRelayElectionTraceFrames + 1
			return cfg
		}()},
		{name: "first snapshot not zero", cfg: func() RelayElectionScenarioConfig {
			cfg := defaultRelayElectionScenarioConfig()
			cfg.Snapshots[0].StartSeq = 1
			return cfg
		}()},
		{name: "unordered snapshots", cfg: func() RelayElectionScenarioConfig {
			cfg := defaultRelayElectionScenarioConfig()
			cfg.Snapshots[1].StartSeq = cfg.Snapshots[0].StartSeq
			return cfg
		}()},
		{name: "snapshot beyond frame count", cfg: func() RelayElectionScenarioConfig {
			cfg := defaultRelayElectionScenarioConfig()
			cfg.Snapshots[1].StartSeq = uint32(cfg.FrameCount)
			return cfg
		}()},
		{name: "duplicate candidate node id", cfg: func() RelayElectionScenarioConfig {
			cfg := defaultRelayElectionScenarioConfig()
			cfg.Snapshots[0].Candidates[1].NodeID = cfg.Snapshots[0].Candidates[0].NodeID
			return cfg
		}()},
		{name: "source as relay candidate", cfg: func() RelayElectionScenarioConfig {
			cfg := defaultRelayElectionScenarioConfig()
			cfg.Snapshots[0].Candidates[0].NodeID = cfg.SourceID
			return cfg
		}()},
		{name: "receiver as relay candidate", cfg: func() RelayElectionScenarioConfig {
			cfg := defaultRelayElectionScenarioConfig()
			cfg.Snapshots[0].Candidates[0].NodeID = cfg.ReceiverID
			return cfg
		}()},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := RunRelayElectionScenario(tc.cfg); err == nil {
				t.Fatal("RunRelayElectionScenario accepted invalid config")
			}
		})
	}
}

func TestIT012_RelayElectionScenarioRejectsFrameCountBeyondSequenceSpace(t *testing.T) {
	if strconv.IntSize < 64 {
		t.Skip("frame_count overflow case requires 64-bit int")
	}

	cfg := defaultRelayElectionScenarioConfig()
	cfg.FrameCount = int(maxRelayElectionFrameCount + 1)
	if _, err := RunRelayElectionScenario(cfg); err == nil {
		t.Fatal("RunRelayElectionScenario accepted frame_count beyond uint32 sequence space")
	}
}

func defaultRelayElectionScenarioConfig() RelayElectionScenarioConfig {
	return DefaultRelayElectionScenarioConfig()
}
