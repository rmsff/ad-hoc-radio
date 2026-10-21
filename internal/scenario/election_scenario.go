package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/rmsff/ad-hoc-radio/internal/election"
)

const RelayElectionScenarioID = "relay_election_handover"

const (
	relayElectionReceiverID     uint16 = 4
	maxRelayElectionTraceFrames        = 1_000_000
	maxRelayElectionFrameCount         = int64(^uint32(0))
)

type RelayElectionScenarioConfig struct {
	ScenarioID string
	Seed       uint64
	SourceID   uint16
	ReceiverID uint16
	FrameCount int
	Policy     election.Policy
	Snapshots  []RelayElectionSnapshot
}

type RelayElectionSnapshot struct {
	StartSeq   uint32
	Candidates []election.Candidate
	Coverage   RelayElectionSnapshotCoverage
}

type RelayElectionScenarioReport struct {
	ScenarioID                  string
	Seed                        uint64
	SourceID                    uint16
	ReceiverID                  uint16
	FrameCount                  int
	Decisions                   []RelayElectionDecision
	ForwardingTrace             []RelayElectionForward
	HandoverCount               int
	UnassignedRelayForwardCount int
	EventHash                   string
}

type RelayElectionDecision struct {
	SnapshotIndex    int
	StartSeq         uint32
	EndSeqExclusive  uint32
	RelayID          uint16
	PreviousRelayID  uint16
	Score            float64
	EligibleCount    int
	ActiveRelayCount int
	Handover         bool
}

type RelayElectionForward struct {
	Seq              uint32
	RelayID          uint16
	SnapshotIndex    int
	ActiveRelayCount int
}

func DefaultRelayElectionScenarioConfig() RelayElectionScenarioConfig {
	return RelayElectionScenarioConfig{
		ScenarioID: RelayElectionScenarioID,
		Seed:       DefaultScenarioSeed,
		SourceID:   DefaultSourceID,
		ReceiverID: relayElectionReceiverID,
		FrameCount: 8,
		Policy:     election.DefaultPolicy(),
		Snapshots: []RelayElectionSnapshot{
			{
				StartSeq: 0,
				Candidates: []election.Candidate{
					{
						NodeID:            2,
						RelayCapable:      true,
						RelayEnabled:      true,
						QualityFromSource: 1.00,
						GroupCoverage:     0.95,
						Battery:           0.90,
						Stability:         0.90,
					},
					{
						NodeID:            3,
						RelayCapable:      true,
						RelayEnabled:      true,
						QualityFromSource: 0.70,
						GroupCoverage:     0.80,
						Battery:           0.90,
						Stability:         0.80,
					},
				},
			},
			{
				StartSeq: 4,
				Candidates: []election.Candidate{
					{
						NodeID:            2,
						RelayCapable:      true,
						RelayEnabled:      true,
						QualityFromSource: 0.30,
						GroupCoverage:     0.30,
						Battery:           0.90,
						Stability:         0.90,
					},
					{
						NodeID:            3,
						RelayCapable:      true,
						RelayEnabled:      true,
						QualityFromSource: 0.90,
						GroupCoverage:     0.95,
						Battery:           0.90,
						Stability:         0.80,
					},
				},
			},
		},
	}
}

func RunRelayElectionScenario(cfg RelayElectionScenarioConfig) (RelayElectionScenarioReport, error) {
	cfg = normalizeRelayElectionScenarioConfig(cfg)
	if err := validateRelayElectionScenarioConfig(cfg); err != nil {
		return RelayElectionScenarioReport{}, err
	}

	report := RelayElectionScenarioReport{
		ScenarioID:      cfg.ScenarioID,
		Seed:            cfg.Seed,
		SourceID:        cfg.SourceID,
		ReceiverID:      cfg.ReceiverID,
		FrameCount:      cfg.FrameCount,
		Decisions:       make([]RelayElectionDecision, 0, len(cfg.Snapshots)),
		ForwardingTrace: make([]RelayElectionForward, 0, cfg.FrameCount),
	}

	var previousRelayID uint16
	for i, snapshot := range cfg.Snapshots {
		result, err := election.SelectRelay(snapshot.Candidates, cfg.Policy)
		if err != nil {
			return RelayElectionScenarioReport{}, fmt.Errorf("snapshot %d start_seq=%d: %w", i, snapshot.StartSeq, err)
		}

		endSeq := uint32(cfg.FrameCount)
		if i+1 < len(cfg.Snapshots) {
			endSeq = cfg.Snapshots[i+1].StartSeq
		}
		handover := previousRelayID != 0 && previousRelayID != result.RelayID
		if handover {
			report.HandoverCount++
		}

		decision := RelayElectionDecision{
			SnapshotIndex:    i,
			StartSeq:         snapshot.StartSeq,
			EndSeqExclusive:  endSeq,
			RelayID:          result.RelayID,
			PreviousRelayID:  previousRelayID,
			Score:            result.Score,
			EligibleCount:    result.EligibleCount,
			ActiveRelayCount: 1,
			Handover:         handover,
		}
		report.Decisions = append(report.Decisions, decision)

		for seq := snapshot.StartSeq; seq < endSeq; seq++ {
			report.ForwardingTrace = append(report.ForwardingTrace, RelayElectionForward{
				Seq:              seq,
				RelayID:          result.RelayID,
				SnapshotIndex:    i,
				ActiveRelayCount: 1,
			})
		}

		previousRelayID = result.RelayID
	}

	report.UnassignedRelayForwardCount = countUnassignedRelayForwards(report.Decisions, report.ForwardingTrace)

	eventHash, err := relayElectionReportHash(report)
	if err != nil {
		return RelayElectionScenarioReport{}, err
	}
	report.EventHash = eventHash
	return report, nil
}

func normalizeRelayElectionScenarioConfig(cfg RelayElectionScenarioConfig) RelayElectionScenarioConfig {
	if cfg.ScenarioID == "" {
		cfg.ScenarioID = RelayElectionScenarioID
	}
	if cfg.Seed == 0 {
		cfg.Seed = DefaultScenarioSeed
	}
	return cfg
}

func validateRelayElectionScenarioConfig(cfg RelayElectionScenarioConfig) error {
	if cfg.FrameCount <= 0 {
		return fmt.Errorf("relay election scenario: frame_count=%d must be > 0", cfg.FrameCount)
	}
	if cfg.FrameCount > maxRelayElectionTraceFrames {
		return fmt.Errorf("relay election scenario: frame_count=%d exceeds trace cap %d", cfg.FrameCount, maxRelayElectionTraceFrames)
	}
	if int64(cfg.FrameCount) > maxRelayElectionFrameCount {
		return fmt.Errorf("relay election scenario: frame_count=%d must fit uint32 sequence space", cfg.FrameCount)
	}
	if cfg.SourceID == 0 {
		return fmt.Errorf("relay election scenario: source_id must be > 0")
	}
	if cfg.ReceiverID == 0 {
		return fmt.Errorf("relay election scenario: receiver_id must be > 0")
	}
	if cfg.SourceID == cfg.ReceiverID {
		return fmt.Errorf("relay election scenario: source_id and receiver_id must be distinct")
	}
	if len(cfg.Snapshots) == 0 {
		return fmt.Errorf("relay election scenario: at least one election snapshot is required")
	}
	if cfg.Snapshots[0].StartSeq != 0 {
		return fmt.Errorf("relay election scenario: first snapshot start_seq=%d, want 0", cfg.Snapshots[0].StartSeq)
	}
	for i, snapshot := range cfg.Snapshots {
		if len(snapshot.Candidates) == 0 {
			return fmt.Errorf("relay election scenario: snapshot %d has no candidates", i)
		}
		if uint64(snapshot.StartSeq) >= uint64(cfg.FrameCount) {
			return fmt.Errorf("relay election scenario: snapshot %d start_seq=%d must be < frame_count=%d", i, snapshot.StartSeq, cfg.FrameCount)
		}
		for candidateIndex, candidate := range snapshot.Candidates {
			if candidate.NodeID == cfg.SourceID {
				return fmt.Errorf("relay election scenario: snapshot %d candidate %d node_id=%d matches source_id", i, candidateIndex, candidate.NodeID)
			}
			if candidate.NodeID == cfg.ReceiverID {
				return fmt.Errorf("relay election scenario: snapshot %d candidate %d node_id=%d matches receiver_id", i, candidateIndex, candidate.NodeID)
			}
		}
		if i > 0 && snapshot.StartSeq <= cfg.Snapshots[i-1].StartSeq {
			return fmt.Errorf("relay election scenario: snapshot %d start_seq=%d must be greater than previous start_seq=%d",
				i, snapshot.StartSeq, cfg.Snapshots[i-1].StartSeq)
		}
	}
	return nil
}

func countUnassignedRelayForwards(decisions []RelayElectionDecision, trace []RelayElectionForward) int {
	assignedRelayBySnapshot := make(map[int]uint16, len(decisions))
	for _, decision := range decisions {
		assignedRelayBySnapshot[decision.SnapshotIndex] = decision.RelayID
	}

	count := 0
	for _, row := range trace {
		relayID, ok := assignedRelayBySnapshot[row.SnapshotIndex]
		if !ok || row.RelayID != relayID || row.ActiveRelayCount != 1 {
			count++
		}
	}
	return count
}

func relayElectionReportHash(report RelayElectionScenarioReport) (string, error) {
	hashable := report
	hashable.EventHash = ""
	b, err := json.Marshal(hashable)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
