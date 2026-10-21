package scenario

import (
	"errors"
	"fmt"

	"github.com/rmsff/ad-hoc-radio/internal/election"
)

const RelayElectionControlPlaneScenarioID = "relay_election_control_plane"

const (
	maxRelayElectionControlCandidatesPerSnapshot = 128
	maxRelayElectionControlEvents                = 200_000
)

const (
	RelayElectionControlEventBeacon      = "beacon"
	RelayElectionControlEventRelayOffer  = "relay_offer"
	RelayElectionControlEventRelayAssign = "relay_assign"
	RelayElectionControlEventReelection  = "reelection"
)

type RelayElectionControlPlaneScenarioConfig struct {
	ScenarioID string
	Seed       uint64
	SourceID   uint16
	ReceiverID uint16
	FrameCount int
	Policy     election.Policy
	Coverage   RelayElectionCoverageConfig
	Snapshots  []RelayElectionSnapshot
}

type RelayElectionControlPlaneReport struct {
	ScenarioID                       string
	Seed                             uint64
	SourceID                         uint16
	ReceiverID                       uint16
	FrameCount                       int
	ControlEvents                    []RelayElectionControlEvent
	Assignments                      []RelayElectionAssignment
	VoiceForwardingTrace             []RelayElectionControlVoiceForward
	CoveragePredictions              []RelayElectionCoveragePrediction
	EventCounts                      RelayElectionControlEventCounts
	HandoverCount                    int
	UnassignedRelayForwardCount      int
	UnselectedRelayVoiceForwardCount int
	EventHash                        string
}

type RelayElectionControlEvent struct {
	Index         int
	SnapshotIndex int
	Seq           uint32
	Type          string
	FromID        uint16
	ToID          uint16
	RelayID       uint16
	Selected      bool
	Score         float64
}

type RelayElectionAssignment struct {
	SnapshotIndex    int
	StartSeq         uint32
	EndSeqExclusive  uint32
	RelayID          uint16
	PreviousRelayID  uint16
	Score            float64
	EligibleCount    int
	BeaconCount      int
	OfferCount       int
	ActiveRelayCount int
	RelayRequired    bool
	VoiceMode        string
	Handover         bool
}

type RelayElectionControlVoiceForward struct {
	Seq                         uint32
	AssignedRelayID             uint16
	ForwardingRelayID           uint16
	SnapshotIndex               int
	ActiveRelayCount            int
	UnselectedRelayForwardCount int
	VoiceMode                   string
}

type RelayElectionControlEventCounts struct {
	Beacon      int
	RelayOffer  int
	RelayAssign int
	Reelection  int
}

type relayElectionOffer struct {
	RelayID uint16
	Score   float64
}

func DefaultRelayElectionControlPlaneScenarioConfig() RelayElectionControlPlaneScenarioConfig {
	base := DefaultRelayElectionScenarioConfig()
	return RelayElectionControlPlaneScenarioConfig{
		ScenarioID: RelayElectionControlPlaneScenarioID,
		Seed:       base.Seed,
		SourceID:   base.SourceID,
		ReceiverID: base.ReceiverID,
		FrameCount: base.FrameCount,
		Policy:     base.Policy,
		Snapshots:  base.Snapshots,
	}
}

func RunRelayElectionControlPlaneScenario(cfg RelayElectionControlPlaneScenarioConfig) (RelayElectionControlPlaneReport, error) {
	cfg = normalizeRelayElectionControlPlaneScenarioConfig(cfg)
	if err := validateRelayElectionControlPlaneScenarioConfig(cfg); err != nil {
		return RelayElectionControlPlaneReport{}, err
	}

	report := RelayElectionControlPlaneReport{
		ScenarioID:           cfg.ScenarioID,
		Seed:                 cfg.Seed,
		SourceID:             cfg.SourceID,
		ReceiverID:           cfg.ReceiverID,
		FrameCount:           cfg.FrameCount,
		ControlEvents:        make([]RelayElectionControlEvent, 0, len(cfg.Snapshots)*3),
		Assignments:          make([]RelayElectionAssignment, 0, len(cfg.Snapshots)),
		VoiceForwardingTrace: make([]RelayElectionControlVoiceForward, 0, cfg.FrameCount),
		CoveragePredictions:  make([]RelayElectionCoveragePrediction, 0, len(cfg.Snapshots)),
	}

	var previousRelayID uint16
	for i, snapshot := range cfg.Snapshots {
		if relayElectionClosedGroupCoverageEnabled(cfg) {
			endSeq := relayElectionControlSnapshotEndSeq(cfg, i)
			directPrediction := newRelayElectionCoveragePrediction(cfg, snapshot, i, 0, RelayElectionVoiceModeDirectOnly)
			appendRelayElectionControlBeacons(&report, cfg, snapshot, i)

			if !directPrediction.RelayRequired {
				report.CoveragePredictions = append(report.CoveragePredictions, directPrediction)
				report.Assignments = append(report.Assignments, RelayElectionAssignment{
					SnapshotIndex:    i,
					StartSeq:         snapshot.StartSeq,
					EndSeqExclusive:  endSeq,
					RelayID:          0,
					PreviousRelayID:  previousRelayID,
					BeaconCount:      len(snapshot.Candidates),
					OfferCount:       0,
					ActiveRelayCount: 0,
					RelayRequired:    false,
					VoiceMode:        RelayElectionVoiceModeDirectOnly,
				})
				appendRelayElectionControlVoiceTrace(&report, i, snapshot.StartSeq, endSeq, 0, 0, RelayElectionVoiceModeDirectOnly)
				previousRelayID = 0
				continue
			}

			usefulCandidates := usefulRelayCandidates(snapshot, snapshot.Candidates, directPrediction.CoveredDirect, directPrediction.ExpectedRecipients)
			selection, err := election.SelectRelay(usefulCandidates, cfg.Policy)
			if errors.Is(err, election.ErrNoEligibleRelay) {
				degradedPrediction := newRelayElectionCoveragePrediction(cfg, snapshot, i, 0, RelayElectionVoiceModeDirectOnlyDegraded)
				report.CoveragePredictions = append(report.CoveragePredictions, degradedPrediction)
				report.Assignments = append(report.Assignments, RelayElectionAssignment{
					SnapshotIndex:    i,
					StartSeq:         snapshot.StartSeq,
					EndSeqExclusive:  endSeq,
					RelayID:          0,
					PreviousRelayID:  previousRelayID,
					BeaconCount:      len(snapshot.Candidates),
					OfferCount:       0,
					ActiveRelayCount: 0,
					RelayRequired:    true,
					VoiceMode:        RelayElectionVoiceModeDirectOnlyDegraded,
				})
				appendRelayElectionControlVoiceTrace(&report, i, snapshot.StartSeq, endSeq, 0, 0, RelayElectionVoiceModeDirectOnlyDegraded)
				previousRelayID = 0
				continue
			}
			if err != nil {
				return RelayElectionControlPlaneReport{}, fmt.Errorf("snapshot %d start_seq=%d: %w", i, snapshot.StartSeq, err)
			}

			if i > 0 {
				appendRelayElectionReelectionEvent(&report, cfg, snapshot, i)
			}
			offers, err := relayElectionControlOffers(usefulCandidates, cfg.Policy)
			if err != nil {
				return RelayElectionControlPlaneReport{}, fmt.Errorf("snapshot %d start_seq=%d: %w", i, snapshot.StartSeq, err)
			}
			for _, offer := range offers {
				report.appendControlEvent(RelayElectionControlEvent{
					SnapshotIndex: i,
					Seq:           snapshot.StartSeq,
					Type:          RelayElectionControlEventRelayOffer,
					FromID:        offer.RelayID,
					ToID:          cfg.SourceID,
					RelayID:       offer.RelayID,
					Selected:      offer.RelayID == selection.RelayID,
					Score:         offer.Score,
				})
			}

			handover := previousRelayID != 0 && previousRelayID != selection.RelayID
			if handover {
				report.HandoverCount++
			}
			prediction := newRelayElectionCoveragePrediction(cfg, snapshot, i, selection.RelayID, RelayElectionVoiceModeRelayed)
			report.CoveragePredictions = append(report.CoveragePredictions, prediction)
			report.appendControlEvent(RelayElectionControlEvent{
				SnapshotIndex: i,
				Seq:           snapshot.StartSeq,
				Type:          RelayElectionControlEventRelayAssign,
				FromID:        cfg.SourceID,
				ToID:          selection.RelayID,
				RelayID:       selection.RelayID,
				Selected:      true,
				Score:         selection.Score,
			})
			report.Assignments = append(report.Assignments, RelayElectionAssignment{
				SnapshotIndex:    i,
				StartSeq:         snapshot.StartSeq,
				EndSeqExclusive:  endSeq,
				RelayID:          selection.RelayID,
				PreviousRelayID:  previousRelayID,
				Score:            selection.Score,
				EligibleCount:    selection.EligibleCount,
				BeaconCount:      len(snapshot.Candidates),
				OfferCount:       len(offers),
				ActiveRelayCount: 1,
				RelayRequired:    true,
				VoiceMode:        RelayElectionVoiceModeRelayed,
				Handover:         handover,
			})
			appendRelayElectionControlVoiceTrace(&report, i, snapshot.StartSeq, endSeq, selection.RelayID, 1, RelayElectionVoiceModeRelayed)
			previousRelayID = selection.RelayID
			continue
		}

		if i > 0 {
			appendRelayElectionReelectionEvent(&report, cfg, snapshot, i)
		}

		selection, err := election.SelectRelay(snapshot.Candidates, cfg.Policy)
		if err != nil {
			return RelayElectionControlPlaneReport{}, fmt.Errorf("snapshot %d start_seq=%d: %w", i, snapshot.StartSeq, err)
		}

		offers, err := relayElectionControlOffers(snapshot.Candidates, cfg.Policy)
		if err != nil {
			return RelayElectionControlPlaneReport{}, fmt.Errorf("snapshot %d start_seq=%d: %w", i, snapshot.StartSeq, err)
		}
		appendRelayElectionControlBeacons(&report, cfg, snapshot, i)
		for _, offer := range offers {
			report.appendControlEvent(RelayElectionControlEvent{
				SnapshotIndex: i,
				Seq:           snapshot.StartSeq,
				Type:          RelayElectionControlEventRelayOffer,
				FromID:        offer.RelayID,
				ToID:          cfg.SourceID,
				RelayID:       offer.RelayID,
				Selected:      offer.RelayID == selection.RelayID,
				Score:         offer.Score,
			})
		}

		endSeq := relayElectionControlSnapshotEndSeq(cfg, i)
		handover := previousRelayID != 0 && previousRelayID != selection.RelayID
		if handover {
			report.HandoverCount++
		}

		report.appendControlEvent(RelayElectionControlEvent{
			SnapshotIndex: i,
			Seq:           snapshot.StartSeq,
			Type:          RelayElectionControlEventRelayAssign,
			FromID:        cfg.SourceID,
			ToID:          selection.RelayID,
			RelayID:       selection.RelayID,
			Selected:      true,
			Score:         selection.Score,
		})
		report.Assignments = append(report.Assignments, RelayElectionAssignment{
			SnapshotIndex:    i,
			StartSeq:         snapshot.StartSeq,
			EndSeqExclusive:  endSeq,
			RelayID:          selection.RelayID,
			PreviousRelayID:  previousRelayID,
			Score:            selection.Score,
			EligibleCount:    selection.EligibleCount,
			BeaconCount:      len(snapshot.Candidates),
			OfferCount:       len(offers),
			ActiveRelayCount: 1,
			RelayRequired:    true,
			VoiceMode:        RelayElectionVoiceModeRelayed,
			Handover:         handover,
		})

		appendRelayElectionControlVoiceTrace(&report, i, snapshot.StartSeq, endSeq, selection.RelayID, 1, RelayElectionVoiceModeRelayed)

		previousRelayID = selection.RelayID
	}

	report.EventCounts = countRelayElectionControlEvents(report.ControlEvents)
	report.UnassignedRelayForwardCount = countUnassignedControlForwards(report.Assignments, report.VoiceForwardingTrace)
	report.UnselectedRelayVoiceForwardCount = countUnselectedRelayVoiceForwards(report.VoiceForwardingTrace)

	eventHash, err := relayElectionControlPlaneReportHash(report)
	if err != nil {
		return RelayElectionControlPlaneReport{}, err
	}
	report.EventHash = eventHash
	return report, nil
}

func (r *RelayElectionControlPlaneReport) appendControlEvent(event RelayElectionControlEvent) {
	event.Index = len(r.ControlEvents)
	r.ControlEvents = append(r.ControlEvents, event)
}

func normalizeRelayElectionControlPlaneScenarioConfig(cfg RelayElectionControlPlaneScenarioConfig) RelayElectionControlPlaneScenarioConfig {
	if cfg.ScenarioID == "" {
		cfg.ScenarioID = RelayElectionControlPlaneScenarioID
	}
	if cfg.Seed == 0 {
		cfg.Seed = DefaultScenarioSeed
	}
	cfg.Coverage = normalizeRelayElectionCoverageConfig(cfg.Coverage)
	return cfg
}

func validateRelayElectionControlPlaneScenarioConfig(cfg RelayElectionControlPlaneScenarioConfig) error {
	for i, snapshot := range cfg.Snapshots {
		if len(snapshot.Candidates) > maxRelayElectionControlCandidatesPerSnapshot {
			return fmt.Errorf("relay election control-plane scenario: snapshot %d candidates=%d exceeds cap %d",
				i, len(snapshot.Candidates), maxRelayElectionControlCandidatesPerSnapshot)
		}
	}
	if upperBoundRelayElectionControlEventCount(cfg.Snapshots) > maxRelayElectionControlEvents {
		return fmt.Errorf("relay election control-plane scenario: control event upper bound exceeds cap %d", maxRelayElectionControlEvents)
	}
	if err := validateRelayElectionControlCandidateValues(cfg); err != nil {
		return err
	}
	if err := validateRelayElectionCoverageConfig(cfg); err != nil {
		return err
	}

	base := RelayElectionScenarioConfig{
		ScenarioID: cfg.ScenarioID,
		Seed:       cfg.Seed,
		SourceID:   cfg.SourceID,
		ReceiverID: cfg.ReceiverID,
		FrameCount: cfg.FrameCount,
		Policy:     cfg.Policy,
		Snapshots:  cfg.Snapshots,
	}
	if err := validateRelayElectionScenarioConfig(base); err != nil {
		return err
	}
	return nil
}

func validateRelayElectionControlCandidateValues(cfg RelayElectionControlPlaneScenarioConfig) error {
	for i, snapshot := range cfg.Snapshots {
		_, err := election.SelectRelay(snapshot.Candidates, cfg.Policy)
		if err == nil || errors.Is(err, election.ErrNoEligibleRelay) {
			continue
		}
		return fmt.Errorf("snapshot %d start_seq=%d: %w", i, snapshot.StartSeq, err)
	}
	return nil
}

func upperBoundRelayElectionControlEventCount(snapshots []RelayElectionSnapshot) int {
	count := 0
	for i, snapshot := range snapshots {
		if i > 0 {
			count++
		}
		count += len(snapshot.Candidates) * 2
		count++
	}
	return count
}
