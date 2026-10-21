package scenario

import "fmt"

const (
	fieldRealismTxKindBeacon      = "beacon"
	fieldRealismTxKindSourceVoice = "source_voice"
	fieldRealismTxKindRelayVoice  = "relay_voice"
)

type fieldRealismTxWindow struct {
	nodeID     uint16
	receiverID uint16
	startMs    int64
	durationMs int64
	kind       string
}

type fieldRealismOccupancy struct {
	externalInterference int
	hiddenNodeCollision  int
	localBusyDefer       int
	localBusyDrop        int
}

func fieldRealismVoiceTxWindows(
	cfg FieldRealismImpairmentConfig,
	sourceTxDuration int64,
	relayTxDuration int64,
	selectedRelayID uint16,
) ([]fieldRealismTxWindow, error) {
	windows := make([]fieldRealismTxWindow, 0, cfg.FrameCount*2)
	for seq := 0; seq < cfg.FrameCount; seq++ {
		seqOffset, err := checkedLimitedMultiHopMul(int64(seq), cfg.VoicePeriodMs)
		if err != nil {
			return nil, err
		}
		sourceStart, err := checkedLimitedMultiHopAdd(cfg.CallStartMs, seqOffset)
		if err != nil {
			return nil, err
		}
		windows = append(windows, fieldRealismTxWindow{
			nodeID:     cfg.SourceID,
			receiverID: selectedRelayID,
			startMs:    sourceStart,
			durationMs: sourceTxDuration,
			kind:       fieldRealismTxKindSourceVoice,
		})
		if selectedRelayID == 0 {
			continue
		}
		relayStart, err := checkedLimitedMultiHopAdd(sourceStart, cfg.SourceSlotMs)
		if err != nil {
			return nil, err
		}
		windows = append(windows, fieldRealismTxWindow{
			nodeID:     selectedRelayID,
			receiverID: cfg.ReceiverID,
			startMs:    relayStart,
			durationMs: relayTxDuration,
			kind:       fieldRealismTxKindRelayVoice,
		})
	}
	return windows, nil
}

func fieldRealismOccupancyCounts(cfg FieldRealismImpairmentConfig, txWindows []fieldRealismTxWindow) (fieldRealismOccupancy, error) {
	counts := fieldRealismOccupancy{}
	for _, interference := range cfg.ExternalInterferenceWindows {
		overlaps, err := fieldRealismWindowOverlapsAny(interference.StartMs, interference.DurationMs, txWindows)
		if err != nil {
			return fieldRealismOccupancy{}, err
		}
		if overlaps {
			counts.externalInterference++
		}
	}
	for _, hidden := range cfg.HiddenNodeTxWindows {
		overlaps, err := fieldRealismWindowOverlapsReceiver(hidden.StartMs, hidden.DurationMs, hidden.ReceiverID, txWindows)
		if err != nil {
			return fieldRealismOccupancy{}, err
		}
		if overlaps {
			counts.hiddenNodeCollision++
		}
	}
	for _, busy := range cfg.LocalBusyWindows {
		overlaps, err := fieldRealismWindowOverlapsNode(busy.StartMs, busy.DurationMs, busy.NodeID, txWindows)
		if err != nil {
			return fieldRealismOccupancy{}, err
		}
		if !overlaps {
			continue
		}
		switch busy.Action {
		case FieldRealismBusyActionDefer:
			counts.localBusyDefer++
		case FieldRealismBusyActionDrop:
			counts.localBusyDrop++
		}
	}
	return counts, nil
}

func fieldRealismBeaconTxStart(
	cfg FieldRealismImpairmentConfig,
	nodeID uint16,
	dueMs int64,
	durationMs int64,
) (int64, bool, int, int, error) {
	txAtMs := dueMs
	deferCount := 0
	for attempts := 0; attempts <= len(cfg.LocalBusyWindows); attempts++ {
		deferred := false
		for _, busy := range cfg.LocalBusyWindows {
			if busy.NodeID != nodeID {
				continue
			}
			overlaps, err := fieldRealismWindowsOverlap(txAtMs, durationMs, busy.StartMs, busy.DurationMs)
			if err != nil {
				return 0, false, 0, 0, err
			}
			if !overlaps {
				continue
			}
			if busy.Action == FieldRealismBusyActionDrop {
				return txAtMs, true, deferCount, 1, nil
			}
			busyEnd, err := checkedLimitedMultiHopAdd(busy.StartMs, busy.DurationMs)
			if err != nil {
				return 0, false, 0, 0, err
			}
			if busyEnd > txAtMs {
				txAtMs = busyEnd
			}
			deferCount++
			deferred = true
		}
		if !deferred {
			return txAtMs, false, deferCount, 0, nil
		}
	}
	return 0, false, 0, 0, fmt.Errorf("field realism impairment: local busy beacon defer loop exceeded")
}

func fieldRealismWindowOverlapsAny(startMs, durationMs int64, windows []fieldRealismTxWindow) (bool, error) {
	for _, window := range windows {
		overlaps, err := fieldRealismWindowsOverlap(startMs, durationMs, window.startMs, window.durationMs)
		if err != nil {
			return false, err
		}
		if overlaps {
			return true, nil
		}
	}
	return false, nil
}

func fieldRealismWindowOverlapsReceiver(startMs, durationMs int64, receiverID uint16, windows []fieldRealismTxWindow) (bool, error) {
	for _, window := range windows {
		if window.receiverID != receiverID {
			continue
		}
		overlaps, err := fieldRealismWindowsOverlap(startMs, durationMs, window.startMs, window.durationMs)
		if err != nil {
			return false, err
		}
		if overlaps {
			return true, nil
		}
	}
	return false, nil
}

func fieldRealismWindowOverlapsNode(startMs, durationMs int64, nodeID uint16, windows []fieldRealismTxWindow) (bool, error) {
	for _, window := range windows {
		if window.nodeID != nodeID {
			continue
		}
		overlaps, err := fieldRealismWindowsOverlap(startMs, durationMs, window.startMs, window.durationMs)
		if err != nil {
			return false, err
		}
		if overlaps {
			return true, nil
		}
	}
	return false, nil
}

func fieldRealismWindowsOverlap(leftStart, leftDuration, rightStart, rightDuration int64) (bool, error) {
	leftEnd, err := checkedLimitedMultiHopAdd(leftStart, leftDuration)
	if err != nil {
		return false, err
	}
	rightEnd, err := checkedLimitedMultiHopAdd(rightStart, rightDuration)
	if err != nil {
		return false, err
	}
	return leftStart < rightEnd && leftEnd > rightStart, nil
}

func fieldRealismAppendOccupancyReasons(reasons []string, counts fieldRealismOccupancy) []string {
	if counts.externalInterference > 0 {
		reasons = append(reasons, FieldRealismReasonExternalInterference)
	}
	if counts.hiddenNodeCollision > 0 {
		reasons = append(reasons, FieldRealismReasonHiddenNodeCollision)
	}
	if counts.localBusyDefer > 0 || counts.localBusyDrop > 0 {
		reasons = append(reasons, FieldRealismReasonLocalBusy)
	}
	return reasons
}

func fieldRealismAppendCandidateReasons(reasons []string, candidates []FieldRealismCandidateRow) []string {
	linkDegraded := false
	hasStale := false
	hasDead := false
	for _, candidate := range candidates {
		if candidate.QualityFromSource < 1 || candidate.GroupCoverage < 1 || candidate.ReverseQuality < 1 {
			linkDegraded = true
		}
		if candidate.Status == PeriodicBeaconNeighborStale {
			hasStale = true
		}
		if candidate.Status == PeriodicBeaconNeighborDead {
			hasDead = true
		}
	}
	if linkDegraded {
		reasons = append(reasons, FieldRealismReasonDirectedLinkDegraded)
	}
	if hasStale {
		reasons = append(reasons, FieldRealismReasonBeaconStale)
	}
	if hasDead {
		reasons = append(reasons, FieldRealismReasonBeaconDead)
	}
	return reasons
}

func fieldRealismOutcome(reasons []string, counts fieldRealismOccupancy) string {
	if counts.localBusyDrop > 0 {
		return FieldRealismOutcomeFail
	}
	if len(reasons) == 0 {
		return FieldRealismOutcomePass
	}
	return FieldRealismOutcomeDegraded
}
