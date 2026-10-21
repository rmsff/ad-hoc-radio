package scenario

import (
	"errors"
	"math"
	"sort"

	"github.com/rmsff/ad-hoc-radio/internal/election"
)

type fieldRealismBeaconCounts struct {
	due                int
	tx                 int
	lost               int
	jittered           int
	localBusyDefer     int
	localBusyDrop      int
	freshNeighbors     int
	staleNeighbors     int
	deadNeighbors      int
	neverSeenNeighbors int
}

func defaultFieldRealismNodes() []FieldRealismNode {
	return []FieldRealismNode{
		{
			NodeID:         2,
			BeaconOffsetMs: 0,
			RelayCapable:   true,
			RelayEnabled:   true,
			Battery:        0.90,
			Stability:      0.90,
		},
		{
			NodeID:         3,
			BeaconOffsetMs: defaultPeriodicBeaconIntervalMs / 2,
			RelayCapable:   true,
			RelayEnabled:   true,
			Battery:        0.90,
			Stability:      0.80,
		},
	}
}

func defaultFieldRealismDirectedLinks() []FieldRealismDirectedLinkTimeline {
	return []FieldRealismDirectedLinkTimeline{
		{FromID: DefaultSourceID, ToID: 2, Points: []FieldRealismQualityPoint{{AtMs: 0, Quality: 1.00}}},
		{FromID: 2, ToID: defaultLimitedMultiHopReceiverID, Points: []FieldRealismQualityPoint{{AtMs: 0, Quality: 0.95}}},
		{FromID: 2, ToID: DefaultSourceID, Points: []FieldRealismQualityPoint{{AtMs: 0, Quality: 0.90}}},
		{FromID: DefaultSourceID, ToID: 3, Points: []FieldRealismQualityPoint{{AtMs: 0, Quality: 0.80}}},
		{FromID: 3, ToID: defaultLimitedMultiHopReceiverID, Points: []FieldRealismQualityPoint{{AtMs: 0, Quality: 0.80}}},
		{FromID: 3, ToID: DefaultSourceID, Points: []FieldRealismQualityPoint{{AtMs: 0, Quality: 0.80}}},
	}
}

func normalizeFieldRealismImpairmentConfig(cfg FieldRealismImpairmentConfig) FieldRealismImpairmentConfig {
	def := DefaultFieldRealismImpairmentConfig()
	if cfg.ScenarioID == "" {
		cfg.ScenarioID = def.ScenarioID
	}
	if cfg.Seed == 0 {
		cfg.Seed = def.Seed
	}
	if cfg.SourceID == 0 {
		cfg.SourceID = def.SourceID
	}
	if cfg.ReceiverID == 0 {
		cfg.ReceiverID = def.ReceiverID
	}
	if cfg.Nodes == nil {
		cfg.Nodes = cloneFieldRealismNodes(def.Nodes)
	} else {
		cfg.Nodes = cloneFieldRealismNodes(cfg.Nodes)
	}
	if cfg.DirectedLinks == nil {
		cfg.DirectedLinks = cloneFieldRealismDirectedLinks(def.DirectedLinks)
	} else {
		cfg.DirectedLinks = cloneFieldRealismDirectedLinks(cfg.DirectedLinks)
	}
	if cfg.Policy == (election.Policy{}) {
		cfg.Policy = def.Policy
	}
	if cfg.ClockDriftPpmCandidates == nil {
		cfg.ClockDriftPpmCandidates = cloneMultiHopTimingInt64s(def.ClockDriftPpmCandidates)
	} else {
		cfg.ClockDriftPpmCandidates = sortedMultiHopTimingInt64s(cfg.ClockDriftPpmCandidates)
	}
	if cfg.SlotOffsetMsCandidates == nil {
		cfg.SlotOffsetMsCandidates = cloneMultiHopTimingInt64s(def.SlotOffsetMsCandidates)
	} else {
		cfg.SlotOffsetMsCandidates = sortedMultiHopTimingInt64s(cfg.SlotOffsetMsCandidates)
	}
	if cfg.BeaconLossPctCandidates == nil {
		cfg.BeaconLossPctCandidates = cloneFieldRealismFloat64s(def.BeaconLossPctCandidates)
	} else {
		cfg.BeaconLossPctCandidates = sortedFieldRealismFloat64s(cfg.BeaconLossPctCandidates)
	}
	if cfg.BeaconJitterMsCandidates == nil {
		cfg.BeaconJitterMsCandidates = cloneMultiHopTimingInt64s(def.BeaconJitterMsCandidates)
	} else {
		cfg.BeaconJitterMsCandidates = sortedMultiHopTimingInt64s(cfg.BeaconJitterMsCandidates)
	}
	if cfg.CallStartMs == 0 {
		cfg.CallStartMs = def.CallStartMs
	}
	if cfg.VoicePeriodMs == 0 {
		cfg.VoicePeriodMs = def.VoicePeriodMs
	}
	if cfg.FrameCount == 0 {
		cfg.FrameCount = def.FrameCount
	}
	if cfg.SourceSlotMs == 0 {
		cfg.SourceSlotMs = def.SourceSlotMs
	}
	if cfg.RelaySlotMs == 0 {
		cfg.RelaySlotMs = def.RelaySlotMs
	}
	if cfg.GuardTimeMs == 0 {
		cfg.GuardTimeMs = def.GuardTimeMs
	}
	if cfg.LinkDelayMs == 0 {
		cfg.LinkDelayMs = def.LinkDelayMs
	}
	if cfg.SourceFrameBytes == 0 {
		cfg.SourceFrameBytes = def.SourceFrameBytes
	}
	if cfg.RelayFrameBytes == 0 {
		cfg.RelayFrameBytes = def.RelayFrameBytes
	}
	if cfg.BeaconFrameBytes == 0 {
		cfg.BeaconFrameBytes = def.BeaconFrameBytes
	}
	if cfg.ChannelBitrateBps == 0 {
		cfg.ChannelBitrateBps = def.ChannelBitrateBps
	}
	if cfg.PreambleMs == 0 {
		cfg.PreambleMs = def.PreambleMs
	}
	if cfg.SyncMs == 0 {
		cfg.SyncMs = def.SyncMs
	}
	if cfg.TxRxSwitchMs == 0 {
		cfg.TxRxSwitchMs = def.TxRxSwitchMs
	}
	if cfg.BeaconIntervalMs == 0 {
		cfg.BeaconIntervalMs = def.BeaconIntervalMs
	}
	if cfg.NeighborStaleMs == 0 {
		cfg.NeighborStaleMs = def.NeighborStaleMs
	}
	if cfg.NeighborDeadMs == 0 {
		cfg.NeighborDeadMs = def.NeighborDeadMs
	}
	cfg.ExternalInterferenceWindows = cloneFieldRealismWindows(cfg.ExternalInterferenceWindows)
	cfg.HiddenNodeTxWindows = cloneFieldRealismHiddenWindows(cfg.HiddenNodeTxWindows)
	cfg.LocalBusyWindows = cloneFieldRealismBusyWindows(cfg.LocalBusyWindows)
	return cfg
}

func fieldRealismCandidateRows(
	cfg FieldRealismImpairmentConfig,
	beaconAirtimeMs int64,
	beaconLossPct float64,
	beaconJitterMs int64,
) ([]FieldRealismCandidateRow, fieldRealismBeaconCounts, []fieldRealismTxWindow, error) {
	rows := make([]FieldRealismCandidateRow, 0, len(cfg.Nodes))
	beaconWindows := make([]fieldRealismTxWindow, 0)
	counts := fieldRealismBeaconCounts{}
	for _, node := range cfg.Nodes {
		effectiveNode := fieldRealismNodeAt(node, cfg.CallStartMs)
		lastSeenMs := int64(-1)
		beacons, err := fieldRealismBeaconDueTimes(node, cfg.CallStartMs, cfg.BeaconIntervalMs)
		if err != nil {
			return nil, fieldRealismBeaconCounts{}, nil, err
		}
		for seq, dueMs := range beacons {
			counts.due++
			if fieldRealismBeaconLost(cfg.Seed, node.NodeID, seq, beaconLossPct) {
				counts.lost++
				continue
			}
			txAtMs, dropped, deferCount, dropCount, err := fieldRealismBeaconTxStart(cfg, node.NodeID, dueMs, beaconAirtimeMs)
			if err != nil {
				return nil, fieldRealismBeaconCounts{}, nil, err
			}
			counts.localBusyDefer += deferCount
			counts.localBusyDrop += dropCount
			if dropped {
				continue
			}
			counts.tx++
			beaconWindows = append(beaconWindows, fieldRealismTxWindow{
				nodeID:     node.NodeID,
				receiverID: cfg.SourceID,
				startMs:    txAtMs,
				durationMs: beaconAirtimeMs,
				kind:       fieldRealismTxKindBeacon,
			})
			rxCompleteMs, err := fieldRealismBeaconRxComplete(txAtMs, beaconAirtimeMs, beaconJitterMs)
			if err != nil {
				return nil, fieldRealismBeaconCounts{}, nil, err
			}
			if beaconJitterMs > 0 {
				counts.jittered++
			}
			if rxCompleteMs <= cfg.CallStartMs && rxCompleteMs > lastSeenMs {
				lastSeenMs = rxCompleteMs
			}
		}
		status, ageAtCallMs := fieldRealismNeighborStatus(lastSeenMs, cfg.CallStartMs, cfg.NeighborStaleMs, cfg.NeighborDeadMs)
		switch status {
		case PeriodicBeaconNeighborFresh:
			counts.freshNeighbors++
		case PeriodicBeaconNeighborStale:
			counts.staleNeighbors++
		case PeriodicBeaconNeighborDead:
			counts.deadNeighbors++
		case PeriodicBeaconNeighborNeverSeen:
			counts.neverSeenNeighbors++
		}
		row := FieldRealismCandidateRow{
			NodeID:            effectiveNode.NodeID,
			LastSeenMs:        lastSeenMs,
			AgeAtCallMs:       ageAtCallMs,
			Status:            status,
			RelayCapable:      effectiveNode.RelayCapable,
			RelayEnabled:      effectiveNode.RelayEnabled,
			Busy:              effectiveNode.Busy,
			QualityFromSource: fieldRealismDirectedQuality(cfg.DirectedLinks, cfg.SourceID, effectiveNode.NodeID, cfg.CallStartMs),
			GroupCoverage:     fieldRealismDirectedQuality(cfg.DirectedLinks, effectiveNode.NodeID, cfg.ReceiverID, cfg.CallStartMs),
			ReverseQuality:    fieldRealismDirectedQuality(cfg.DirectedLinks, effectiveNode.NodeID, cfg.SourceID, cfg.CallStartMs),
			Battery:           effectiveNode.Battery,
			Stability:         effectiveNode.Stability,
		}
		eligible, err := fieldRealismCandidateEligible(row, cfg.Policy)
		if err != nil {
			return nil, fieldRealismBeaconCounts{}, nil, err
		}
		row.EligibleForRelay = row.Status == PeriodicBeaconNeighborFresh && eligible
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].NodeID < rows[j].NodeID })
	return rows, counts, beaconWindows, nil
}

func fieldRealismNodeAt(node FieldRealismNode, atMs int64) FieldRealismNode {
	effective := node
	points := cloneFieldRealismHealthPoints(node.HealthTimeline)
	sort.Slice(points, func(i, j int) bool { return points[i].AtMs < points[j].AtMs })
	for _, point := range points {
		if point.AtMs > atMs {
			continue
		}
		effective.RelayEnabled = point.RelayEnabled
		effective.Busy = point.Busy
		effective.Battery = point.Battery
		effective.Stability = point.Stability
	}
	return effective
}

func fieldRealismFreshElectionCandidates(rows []FieldRealismCandidateRow) []election.Candidate {
	candidates := make([]election.Candidate, 0, len(rows))
	for _, row := range rows {
		if !row.EligibleForRelay {
			continue
		}
		candidates = append(candidates, election.Candidate{
			NodeID:            row.NodeID,
			RelayCapable:      row.RelayCapable,
			RelayEnabled:      row.RelayEnabled,
			Busy:              row.Busy,
			QualityFromSource: row.QualityFromSource,
			GroupCoverage:     row.GroupCoverage,
			Battery:           row.Battery,
			Stability:         row.Stability,
		})
	}
	return candidates
}

func fieldRealismCandidateEligible(row FieldRealismCandidateRow, policy election.Policy) (bool, error) {
	if row.ReverseQuality < policy.MinSourceQuality {
		return false, nil
	}
	_, err := election.SelectRelay([]election.Candidate{
		{
			NodeID:            row.NodeID,
			RelayCapable:      row.RelayCapable,
			RelayEnabled:      row.RelayEnabled,
			Busy:              row.Busy,
			QualityFromSource: row.QualityFromSource,
			GroupCoverage:     row.GroupCoverage,
			Battery:           row.Battery,
			Stability:         row.Stability,
		},
	}, policy)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, election.ErrNoEligibleRelay) {
		return false, nil
	}
	return false, err
}

func fieldRealismBeaconDueTimes(node FieldRealismNode, callStartMs, intervalMs int64) ([]int64, error) {
	dueTimes := make([]int64, 0)
	for dueMs := node.BeaconOffsetMs; dueMs <= callStartMs; {
		dueTimes = append(dueTimes, dueMs)
		if dueMs == callStartMs {
			break
		}
		next, err := checkedLimitedMultiHopAdd(dueMs, intervalMs)
		if err != nil {
			return nil, err
		}
		dueMs = next
	}
	return dueTimes, nil
}

func fieldRealismBeaconRxComplete(dueMs, beaconAirtimeMs, beaconJitterMs int64) (int64, error) {
	withAirtime, err := checkedLimitedMultiHopAdd(dueMs, beaconAirtimeMs)
	if err != nil {
		return 0, err
	}
	return checkedLimitedMultiHopAdd(withAirtime, beaconJitterMs)
}

func fieldRealismBeaconLost(seed uint64, nodeID uint16, seq int, lossPct float64) bool {
	if lossPct <= 0 {
		return false
	}
	if lossPct >= 100 {
		return true
	}
	value := (seed + uint64(nodeID)*1103515245 + uint64(seq)*12345) % 10000
	threshold := uint64(math.Round(lossPct * 100))
	return value < threshold
}

func fieldRealismNeighborStatus(lastSeenMs, callStartMs, staleMs, deadMs int64) (string, int64) {
	if lastSeenMs < 0 {
		return PeriodicBeaconNeighborNeverSeen, -1
	}
	age := callStartMs - lastSeenMs
	switch {
	case age <= staleMs:
		return PeriodicBeaconNeighborFresh, age
	case age <= deadMs:
		return PeriodicBeaconNeighborStale, age
	default:
		return PeriodicBeaconNeighborDead, age
	}
}

func fieldRealismDirectedQuality(links []FieldRealismDirectedLinkTimeline, fromID, toID uint16, atMs int64) float64 {
	quality := 0.0
	for _, link := range links {
		if link.FromID != fromID || link.ToID != toID {
			continue
		}
		points := cloneFieldRealismQualityPoints(link.Points)
		sort.Slice(points, func(i, j int) bool { return points[i].AtMs < points[j].AtMs })
		for _, point := range points {
			if point.AtMs <= atMs {
				quality = point.Quality
				continue
			}
			break
		}
	}
	return quality
}

func fieldRealismGuardBudget(cfg FieldRealismImpairmentConfig, driftPpm, slotOffsetMs int64) (int64, int64, int, error) {
	frameCount := int64(cfg.FrameCount)
	callDurationMs, err := checkedLimitedMultiHopMul(frameCount, cfg.VoicePeriodMs)
	if err != nil {
		return 0, 0, 0, err
	}
	driftAbs := driftPpm
	if driftAbs < 0 {
		driftAbs = -driftAbs
	}
	driftProduct, err := checkedLimitedMultiHopMul(driftAbs, callDurationMs)
	if err != nil {
		return 0, 0, 0, err
	}
	driftMs := driftProduct / 1_000_000
	if driftProduct%1_000_000 != 0 {
		driftMs++
	}
	effective, err := checkedLimitedMultiHopAdd(slotOffsetMs, driftMs)
	if err != nil {
		return 0, 0, 0, err
	}
	guardRemaining := cfg.GuardTimeMs - effective
	if guardRemaining < 0 {
		return effective, guardRemaining, cfg.FrameCount, nil
	}
	return effective, guardRemaining, 0, nil
}
