package scenario

import (
	"fmt"
	"math"
	"sort"

	"github.com/rmsff/ad-hoc-radio/internal/election"
)

const (
	RelayElectionCoverageModeClosedGroup = "closed_group"
	RelayElectionCoverageModeOpenGroup   = "open_group"
)

const (
	RelayElectionVoiceModeRelayed            = "relayed"
	RelayElectionVoiceModeDirectOnly         = "direct_only"
	RelayElectionVoiceModeDirectOnlyDegraded = "direct_only_degraded"
)

const RelayElectionCoverageConfidenceBeaconEstimate = "beacon_estimate"

type RelayElectionCoverageConfig struct {
	Mode                  string
	ExpectedMembers       []uint16
	RequiredGroupCoverage float64
}

type RelayElectionSnapshotCoverage struct {
	SourceDirectReachable []uint16
	RelayReachable        []RelayElectionRelayReachability
	Unreachable           []uint16
}

type RelayElectionRelayReachability struct {
	RelayID uint16
	Members []uint16
}

type RelayElectionCoveragePrediction struct {
	SnapshotIndex         int
	Mode                  string
	ExpectedMembers       []uint16
	ExpectedRecipients    []uint16
	RequiredGroupCoverage float64
	SourceDirectCoverage  float64
	RelayRequired         bool
	SelectedRelayID       uint16
	VoiceMode             string
	CoveredDirect         []uint16
	CoveredViaRelay       []uint16
	UncoveredMembers      []uint16
	UnknownMembers        []uint16
	Confidence            string
}

func normalizeRelayElectionCoverageConfig(cfg RelayElectionCoverageConfig) RelayElectionCoverageConfig {
	if cfg.Mode == RelayElectionCoverageModeClosedGroup && cfg.RequiredGroupCoverage == 0 {
		cfg.RequiredGroupCoverage = 1
	}
	return cfg
}

func validateRelayElectionCoverageConfig(cfg RelayElectionControlPlaneScenarioConfig) error {
	switch cfg.Coverage.Mode {
	case "", RelayElectionCoverageModeClosedGroup, RelayElectionCoverageModeOpenGroup:
	default:
		return fmt.Errorf("relay election control-plane scenario: coverage mode %q is invalid", cfg.Coverage.Mode)
	}

	if err := validateCoverageIDList("expected_members", cfg.Coverage.ExpectedMembers); err != nil {
		return err
	}
	if math.IsNaN(cfg.Coverage.RequiredGroupCoverage) {
		return fmt.Errorf("relay election control-plane scenario: required_group_coverage is NaN")
	}
	if math.IsInf(cfg.Coverage.RequiredGroupCoverage, 0) {
		return fmt.Errorf("relay election control-plane scenario: required_group_coverage is Inf")
	}
	if cfg.Coverage.RequiredGroupCoverage < 0 || cfg.Coverage.RequiredGroupCoverage > 1 {
		return fmt.Errorf("relay election control-plane scenario: required_group_coverage=%g must be in [0, 1]", cfg.Coverage.RequiredGroupCoverage)
	}

	recipients := expectedCoverageRecipients(cfg)
	if cfg.Coverage.Mode != RelayElectionCoverageModeClosedGroup || len(recipients) == 0 {
		return nil
	}
	recipientSet := uint16Set(recipients)
	for i, snapshot := range cfg.Snapshots {
		candidateSet := make(map[uint16]struct{}, len(snapshot.Candidates))
		for _, candidate := range snapshot.Candidates {
			candidateSet[candidate.NodeID] = struct{}{}
		}

		if err := validateCoverageMembers("snapshot source_direct_reachable", snapshot.Coverage.SourceDirectReachable, recipientSet); err != nil {
			return fmt.Errorf("snapshot %d: %w", i, err)
		}
		if err := validateCoverageMembers("snapshot unreachable", snapshot.Coverage.Unreachable, recipientSet); err != nil {
			return fmt.Errorf("snapshot %d: %w", i, err)
		}
		sourceReachableSet := uint16Set(snapshot.Coverage.SourceDirectReachable)
		unreachableSet := uint16Set(snapshot.Coverage.Unreachable)
		for _, memberID := range snapshot.Coverage.Unreachable {
			if _, ok := sourceReachableSet[memberID]; ok {
				return fmt.Errorf("snapshot %d: member_id=%d is both source-direct reachable and unreachable", i, memberID)
			}
		}
		seenRelayIDs := make(map[uint16]struct{}, len(snapshot.Coverage.RelayReachable))
		for reachabilityIndex, reachability := range snapshot.Coverage.RelayReachable {
			if reachability.RelayID == 0 {
				return fmt.Errorf("snapshot %d relay_reachable %d: relay_id must be > 0", i, reachabilityIndex)
			}
			if _, ok := seenRelayIDs[reachability.RelayID]; ok {
				return fmt.Errorf("snapshot %d relay_reachable %d: duplicate relay_id=%d", i, reachabilityIndex, reachability.RelayID)
			}
			seenRelayIDs[reachability.RelayID] = struct{}{}
			if _, ok := candidateSet[reachability.RelayID]; !ok {
				return fmt.Errorf("snapshot %d relay_reachable %d: relay_id=%d is not a candidate", i, reachabilityIndex, reachability.RelayID)
			}
			if err := validateCoverageMembers("relay_reachable members", reachability.Members, recipientSet); err != nil {
				return fmt.Errorf("snapshot %d relay_reachable %d: %w", i, reachabilityIndex, err)
			}
			for _, memberID := range reachability.Members {
				if _, ok := unreachableSet[memberID]; ok {
					return fmt.Errorf("snapshot %d relay_reachable %d: member_id=%d is both relay-reachable and unreachable", i, reachabilityIndex, memberID)
				}
			}
		}
	}
	return nil
}

func relayElectionClosedGroupCoverageEnabled(cfg RelayElectionControlPlaneScenarioConfig) bool {
	return cfg.Coverage.Mode == RelayElectionCoverageModeClosedGroup && len(expectedCoverageRecipients(cfg)) > 0
}

func expectedCoverageRecipients(cfg RelayElectionControlPlaneScenarioConfig) []uint16 {
	recipients := make([]uint16, 0, len(cfg.Coverage.ExpectedMembers))
	seen := make(map[uint16]struct{}, len(cfg.Coverage.ExpectedMembers))
	for _, memberID := range cfg.Coverage.ExpectedMembers {
		if memberID == 0 || memberID == cfg.SourceID {
			continue
		}
		if _, ok := seen[memberID]; ok {
			continue
		}
		seen[memberID] = struct{}{}
		recipients = append(recipients, memberID)
	}
	sort.Slice(recipients, func(i, j int) bool { return recipients[i] < recipients[j] })
	return recipients
}

func newRelayElectionCoveragePrediction(
	cfg RelayElectionControlPlaneScenarioConfig,
	snapshot RelayElectionSnapshot,
	snapshotIndex int,
	selectedRelayID uint16,
	voiceMode string,
) RelayElectionCoveragePrediction {
	expectedMembers := sortedUniqueUint16s(cfg.Coverage.ExpectedMembers)
	recipients := expectedCoverageRecipients(cfg)
	sourceReachableSet := uint16Set(snapshot.Coverage.SourceDirectReachable)
	selectedRelayReachableSet := relayReachableSet(snapshot.Coverage, selectedRelayID)
	unreachableSet := uint16Set(snapshot.Coverage.Unreachable)

	prediction := RelayElectionCoveragePrediction{
		SnapshotIndex:         snapshotIndex,
		Mode:                  cfg.Coverage.Mode,
		ExpectedMembers:       expectedMembers,
		ExpectedRecipients:    recipients,
		RequiredGroupCoverage: cfg.Coverage.RequiredGroupCoverage,
		SelectedRelayID:       selectedRelayID,
		VoiceMode:             voiceMode,
		Confidence:            RelayElectionCoverageConfidenceBeaconEstimate,
	}

	for _, memberID := range recipients {
		if _, ok := sourceReachableSet[memberID]; ok {
			prediction.CoveredDirect = append(prediction.CoveredDirect, memberID)
			continue
		}
		if selectedRelayID != 0 {
			if _, ok := selectedRelayReachableSet[memberID]; ok {
				prediction.CoveredViaRelay = append(prediction.CoveredViaRelay, memberID)
				continue
			}
		}
		if _, ok := unreachableSet[memberID]; ok {
			prediction.UncoveredMembers = append(prediction.UncoveredMembers, memberID)
			continue
		}
		prediction.UnknownMembers = append(prediction.UnknownMembers, memberID)
	}

	if len(recipients) > 0 {
		prediction.SourceDirectCoverage = float64(len(prediction.CoveredDirect)) / float64(len(recipients))
	}
	prediction.RelayRequired = prediction.SourceDirectCoverage < cfg.Coverage.RequiredGroupCoverage
	return prediction
}

func usefulRelayCandidates(snapshot RelayElectionSnapshot, candidates []election.Candidate, directCovered []uint16, recipients []uint16) []election.Candidate {
	directSet := uint16Set(directCovered)
	useful := make([]election.Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		relayReachable := relayReachableSet(snapshot.Coverage, candidate.NodeID)
		for _, memberID := range recipients {
			if _, direct := directSet[memberID]; direct {
				continue
			}
			if _, relayCovers := relayReachable[memberID]; relayCovers {
				useful = append(useful, candidate)
				break
			}
		}
	}
	return useful
}

func relayReachableSet(coverage RelayElectionSnapshotCoverage, relayID uint16) map[uint16]struct{} {
	result := make(map[uint16]struct{})
	if relayID == 0 {
		return result
	}
	for _, reachability := range coverage.RelayReachable {
		if reachability.RelayID != relayID {
			continue
		}
		for _, memberID := range reachability.Members {
			result[memberID] = struct{}{}
		}
	}
	return result
}

func sortedUniqueUint16s(ids []uint16) []uint16 {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[uint16]struct{}, len(ids))
	out := make([]uint16, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func uint16Set(ids []uint16) map[uint16]struct{} {
	set := make(map[uint16]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}

func validateCoverageIDList(name string, ids []uint16) error {
	seen := make(map[uint16]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			return fmt.Errorf("relay election control-plane scenario: %s contains zero member id", name)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("relay election control-plane scenario: %s contains duplicate member_id=%d", name, id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func validateCoverageMembers(name string, ids []uint16, expected map[uint16]struct{}) error {
	seen := make(map[uint16]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			return fmt.Errorf("%s contains zero member id", name)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("%s contains duplicate member_id=%d", name, id)
		}
		seen[id] = struct{}{}
		if len(expected) > 0 {
			if _, ok := expected[id]; !ok {
				return fmt.Errorf("%s member_id=%d is not an expected recipient", name, id)
			}
		}
	}
	return nil
}
