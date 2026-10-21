package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"sort"

	"github.com/rmsff/ad-hoc-radio/internal/election"
)

type periodicBeaconDue struct {
	node PeriodicBeaconNode
	due  int64
}

func periodicBeaconDueList(cfg PeriodicBeaconNeighborScenarioConfig) ([]periodicBeaconDue, error) {
	due := make([]periodicBeaconDue, 0)
	for _, node := range cfg.Nodes {
		for dueMs := node.BeaconOffsetMs; dueMs <= cfg.SimulationEndMs; {
			if !periodicBeaconNodeDisabledAt(node, dueMs) {
				due = append(due, periodicBeaconDue{node: node, due: dueMs})
			}
			next, err := checkedLimitedMultiHopAdd(dueMs, cfg.BeaconIntervalMs)
			if err != nil {
				return nil, err
			}
			dueMs = next
		}
	}
	sort.Slice(due, func(i, j int) bool {
		if due[i].due != due[j].due {
			return due[i].due < due[j].due
		}
		return due[i].node.NodeID < due[j].node.NodeID
	})
	return due, nil
}

func periodicBeaconNodeDisabledAt(node PeriodicBeaconNode, timeMs int64) bool {
	if node.Disabled {
		return timeMs >= node.DisabledAtMs
	}
	return node.DisabledAtMs > 0 && timeMs >= node.DisabledAtMs
}

func periodicBeaconTxStart(dueMs, airtimeMs, channelFreeAtMs int64, cfg PeriodicBeaconNeighborScenarioConfig) (int64, bool, string, bool, error) {
	txAtMs := dueMs
	deferred := false
	reason := ""
	duringVoice := false
	for {
		if txAtMs < channelFreeAtMs {
			txAtMs = channelFreeAtMs
			deferred = true
			if reason == "" {
				reason = "channel_busy"
			}
			continue
		}
		window, ok, err := periodicBeaconOverlappingVoiceWindow(txAtMs, airtimeMs, cfg.VoiceWindows)
		if err != nil {
			return 0, false, "", false, err
		}
		if ok {
			endMs, err := periodicBeaconWindowEnd(window)
			if err != nil {
				return 0, false, "", false, err
			}
			txAtMs, err = checkedLimitedMultiHopAdd(endMs, cfg.BeaconGuardMs)
			if err != nil {
				return 0, false, "", false, err
			}
			deferred = true
			duringVoice = true
			reason = "voice_active"
			continue
		}
		return txAtMs, deferred, reason, duringVoice, nil
	}
}

func periodicBeaconOverlappingVoiceWindow(txAtMs, airtimeMs int64, windows []PeriodicBeaconVoiceWindow) (PeriodicBeaconVoiceWindow, bool, error) {
	txEndMs, err := checkedLimitedMultiHopAdd(txAtMs, airtimeMs)
	if err != nil {
		return PeriodicBeaconVoiceWindow{}, false, err
	}
	for _, window := range windows {
		endMs, err := periodicBeaconWindowEnd(window)
		if err != nil {
			return PeriodicBeaconVoiceWindow{}, false, err
		}
		if txAtMs < endMs && txEndMs > window.StartMs {
			return window, true, nil
		}
	}
	return PeriodicBeaconVoiceWindow{}, false, nil
}

func periodicBeaconNeighborRows(cfg PeriodicBeaconNeighborScenarioConfig, lastSeenByNode map[uint16][]int64, callStartMs int64) []PeriodicBeaconNeighborRow {
	rows := make([]PeriodicBeaconNeighborRow, 0, len(cfg.Nodes))
	for _, node := range cfg.Nodes {
		lastSeenMs := int64(-1)
		for _, seenAtMs := range lastSeenByNode[node.NodeID] {
			if seenAtMs <= callStartMs && seenAtMs > lastSeenMs {
				lastSeenMs = seenAtMs
			}
		}
		status := PeriodicBeaconNeighborNeverSeen
		ageAtCallMs := int64(-1)
		if lastSeenMs >= 0 {
			ageAtCallMs = callStartMs - lastSeenMs
			switch {
			case ageAtCallMs <= cfg.NeighborStaleMs:
				status = PeriodicBeaconNeighborFresh
			case ageAtCallMs <= cfg.NeighborDeadMs:
				status = PeriodicBeaconNeighborStale
			default:
				status = PeriodicBeaconNeighborDead
			}
		}
		row := PeriodicBeaconNeighborRow{
			NodeID:            node.NodeID,
			LastSeenMs:        lastSeenMs,
			AgeAtCallMs:       ageAtCallMs,
			Status:            status,
			RelayCapable:      node.RelayCapable,
			RelayEnabled:      node.RelayEnabled,
			QualityFromSource: node.QualityFromSource,
			GroupCoverage:     node.GroupCoverage,
			Battery:           node.Battery,
			Stability:         node.Stability,
		}
		row.EligibleForRelay = status == PeriodicBeaconNeighborFresh && periodicBeaconNodeEligible(node, cfg.Policy)
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].NodeID < rows[j].NodeID })
	return rows
}

func (r *PeriodicBeaconNeighborReport) applyNeighborCountsAndElection(cfg PeriodicBeaconNeighborScenarioConfig) {
	freshCandidates := make([]election.Candidate, 0, len(r.NeighborRows))
	nodeByID := make(map[uint16]PeriodicBeaconNode, len(cfg.Nodes))
	for _, node := range cfg.Nodes {
		nodeByID[node.NodeID] = node
	}
	for _, row := range r.NeighborRows {
		switch row.Status {
		case PeriodicBeaconNeighborFresh:
			r.FreshNeighborCount++
		case PeriodicBeaconNeighborStale:
			r.StaleNeighborCount++
		case PeriodicBeaconNeighborDead:
			r.DeadNeighborCount++
		case PeriodicBeaconNeighborNeverSeen:
			r.NeverSeenNeighborCount++
		}
		if !row.EligibleForRelay {
			continue
		}
		freshCandidates = append(freshCandidates, periodicBeaconCandidate(nodeByID[row.NodeID]))
	}
	r.FreshRelayCandidateCount = len(freshCandidates)
	result, err := election.SelectRelay(freshCandidates, cfg.Policy)
	if err != nil {
		r.Outcome = PeriodicBeaconOutcomeNoFreshCandidate
		return
	}
	r.SelectedRelayID = result.RelayID
	r.SelectedRelayScore = result.Score
	r.Outcome = PeriodicBeaconOutcomeRelayed
}

func periodicBeaconTableConvergedAt(rows []PeriodicBeaconNeighborRow) int64 {
	convergedAtMs := int64(0)
	for _, row := range rows {
		if row.LastSeenMs < 0 {
			return -1
		}
		if row.LastSeenMs > convergedAtMs {
			convergedAtMs = row.LastSeenMs
		}
	}
	return convergedAtMs
}

func periodicBeaconCandidate(node PeriodicBeaconNode) election.Candidate {
	return election.Candidate{
		NodeID:            node.NodeID,
		RelayCapable:      node.RelayCapable,
		RelayEnabled:      node.RelayEnabled,
		Busy:              node.Busy,
		QualityFromSource: node.QualityFromSource,
		GroupCoverage:     node.GroupCoverage,
		Battery:           node.Battery,
		Stability:         node.Stability,
	}
}

func periodicBeaconNodeEligible(node PeriodicBeaconNode, policy election.Policy) bool {
	_, err := election.SelectRelay([]election.Candidate{periodicBeaconCandidate(node)}, policy)
	return err == nil
}

func periodicBeaconWindowEnd(window PeriodicBeaconVoiceWindow) (int64, error) {
	return checkedLimitedMultiHopAdd(window.StartMs, window.DurationMs)
}

func sortPeriodicBeaconEvents(events []PeriodicBeaconEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].TimeMs != events[j].TimeMs {
			return events[i].TimeMs < events[j].TimeMs
		}
		if periodicBeaconEventPriority(events[i].Type) != periodicBeaconEventPriority(events[j].Type) {
			return periodicBeaconEventPriority(events[i].Type) < periodicBeaconEventPriority(events[j].Type)
		}
		if events[i].NodeID != events[j].NodeID {
			return events[i].NodeID < events[j].NodeID
		}
		return events[i].OriginalDueMs < events[j].OriginalDueMs
	})
}

func periodicBeaconEventPriority(eventType string) int {
	switch eventType {
	case PeriodicBeaconEventVoiceStart:
		return 0
	case PeriodicBeaconEventVoiceEnd:
		return 1
	case PeriodicBeaconEventBeaconTx:
		return 2
	default:
		return math.MaxInt
	}
}

func periodicBeaconNeighborReportHash(report PeriodicBeaconNeighborReport) (string, error) {
	hashable := report
	hashable.EventHash = ""
	b, err := json.Marshal(hashable)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func clonePeriodicBeaconNodes(in []PeriodicBeaconNode) []PeriodicBeaconNode {
	out := make([]PeriodicBeaconNode, len(in))
	copy(out, in)
	return out
}

func clonePeriodicBeaconVoiceWindows(in []PeriodicBeaconVoiceWindow) []PeriodicBeaconVoiceWindow {
	out := make([]PeriodicBeaconVoiceWindow, len(in))
	copy(out, in)
	return out
}
