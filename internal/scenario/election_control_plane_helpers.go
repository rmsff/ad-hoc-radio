package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/rmsff/ad-hoc-radio/internal/election"
)

func appendRelayElectionReelectionEvent(report *RelayElectionControlPlaneReport, cfg RelayElectionControlPlaneScenarioConfig, snapshot RelayElectionSnapshot, snapshotIndex int) {
	report.appendControlEvent(RelayElectionControlEvent{
		SnapshotIndex: snapshotIndex,
		Seq:           snapshot.StartSeq,
		Type:          RelayElectionControlEventReelection,
		FromID:        cfg.SourceID,
		ToID:          0,
	})
}

func appendRelayElectionControlBeacons(report *RelayElectionControlPlaneReport, cfg RelayElectionControlPlaneScenarioConfig, snapshot RelayElectionSnapshot, snapshotIndex int) {
	for _, candidate := range snapshot.Candidates {
		report.appendControlEvent(RelayElectionControlEvent{
			SnapshotIndex: snapshotIndex,
			Seq:           snapshot.StartSeq,
			Type:          RelayElectionControlEventBeacon,
			FromID:        candidate.NodeID,
			ToID:          cfg.SourceID,
			RelayID:       candidate.NodeID,
		})
	}
}

func appendRelayElectionControlVoiceTrace(report *RelayElectionControlPlaneReport, snapshotIndex int, startSeq, endSeq uint32, relayID uint16, activeRelayCount int, voiceMode string) {
	for seq := startSeq; seq < endSeq; seq++ {
		report.VoiceForwardingTrace = append(report.VoiceForwardingTrace, RelayElectionControlVoiceForward{
			Seq:                         seq,
			AssignedRelayID:             relayID,
			ForwardingRelayID:           relayID,
			SnapshotIndex:               snapshotIndex,
			ActiveRelayCount:            activeRelayCount,
			UnselectedRelayForwardCount: 0,
			VoiceMode:                   voiceMode,
		})
	}
}

func relayElectionControlSnapshotEndSeq(cfg RelayElectionControlPlaneScenarioConfig, snapshotIndex int) uint32 {
	if snapshotIndex+1 < len(cfg.Snapshots) {
		return cfg.Snapshots[snapshotIndex+1].StartSeq
	}
	return uint32(cfg.FrameCount)
}

func relayElectionControlOffers(candidates []election.Candidate, policy election.Policy) ([]relayElectionOffer, error) {
	offers := make([]relayElectionOffer, 0, len(candidates))
	for i, candidate := range candidates {
		result, err := election.SelectRelay([]election.Candidate{candidate}, policy)
		switch {
		case err == nil:
			offers = append(offers, relayElectionOffer{RelayID: result.RelayID, Score: result.Score})
		case errors.Is(err, election.ErrNoEligibleRelay):
			continue
		default:
			return nil, fmt.Errorf("candidate %d: %w", i, err)
		}
	}
	return offers, nil
}

func countRelayElectionControlEvents(events []RelayElectionControlEvent) RelayElectionControlEventCounts {
	var counts RelayElectionControlEventCounts
	for _, event := range events {
		switch event.Type {
		case RelayElectionControlEventBeacon:
			counts.Beacon++
		case RelayElectionControlEventRelayOffer:
			counts.RelayOffer++
		case RelayElectionControlEventRelayAssign:
			counts.RelayAssign++
		case RelayElectionControlEventReelection:
			counts.Reelection++
		}
	}
	return counts
}

func countUnassignedControlForwards(assignments []RelayElectionAssignment, trace []RelayElectionControlVoiceForward) int {
	assignedRelayBySnapshot := make(map[int]uint16, len(assignments))
	for _, assignment := range assignments {
		assignedRelayBySnapshot[assignment.SnapshotIndex] = assignment.RelayID
	}

	count := 0
	for _, row := range trace {
		if row.AssignedRelayID == 0 && row.ForwardingRelayID == 0 && row.ActiveRelayCount == 0 {
			continue
		}
		relayID, ok := assignedRelayBySnapshot[row.SnapshotIndex]
		if !ok || row.AssignedRelayID != relayID || row.ForwardingRelayID != relayID || row.ActiveRelayCount != 1 {
			count++
		}
	}
	return count
}

func countUnselectedRelayVoiceForwards(trace []RelayElectionControlVoiceForward) int {
	count := 0
	for _, row := range trace {
		count += row.UnselectedRelayForwardCount
	}
	return count
}

func relayElectionControlPlaneReportHash(report RelayElectionControlPlaneReport) (string, error) {
	hashable := report
	hashable.EventHash = ""
	b, err := json.Marshal(hashable)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
