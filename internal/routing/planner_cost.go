package routing

import (
	"fmt"
	"math"
)

const maxInt64 = int64(1<<63 - 1)

func evaluateTreeCost(request PlanRequest, tree RelayTree, linkIndex map[[2]uint16]DirectedLink, relayIndex map[uint16]RelayState) (TreeCost, bool, error) {
	relayOwners := selectedRelayOwners(request.SourceStationID, tree.Edges)
	relayCount := int64(len(relayOwners))

	txOpportunityCount := 1 + len(relayOwners)
	txRelayAirtime, ok := checkedMulInt64(request.CostModel.RelayOpportunityMS, relayCount)
	if !ok {
		return TreeCost{}, false, fmt.Errorf("tree cost: relay airtime overflows int64")
	}
	txAirtimeMS, ok := checkedAddInt64(request.CostModel.SourceOpportunityMS, txRelayAirtime)
	if !ok {
		return TreeCost{}, false, fmt.Errorf("tree cost: voice airtime overflows int64")
	}
	voiceAirtimeMilliPct, ok := roundedMilliPercent(txAirtimeMS, request.CostModel.FramePeriodMS)
	if !ok {
		return TreeCost{}, false, fmt.Errorf("tree cost: voice airtime milli-percent overflows int64")
	}

	sourceCycle, ok := checkedAddInt64(request.CostModel.SourceOpportunityMS, request.CostModel.GuardAfterMS)
	if !ok {
		return TreeCost{}, false, fmt.Errorf("tree cost: source cycle overflows int64")
	}
	relayCycleUnit, ok := checkedAddInt64(request.CostModel.RelayOpportunityMS, request.CostModel.GuardAfterMS)
	if !ok {
		return TreeCost{}, false, fmt.Errorf("tree cost: relay cycle unit overflows int64")
	}
	relayCycle, ok := checkedMulInt64(relayCycleUnit, relayCount)
	if !ok {
		return TreeCost{}, false, fmt.Errorf("tree cost: relay cycle overflows int64")
	}
	estimatedCycleMS, ok := checkedAddInt64(sourceCycle, relayCycle)
	if !ok {
		return TreeCost{}, false, fmt.Errorf("tree cost: schedule cycle overflows int64")
	}
	if estimatedCycleMS > request.CostModel.FramePeriodMS {
		return TreeCost{}, false, nil
	}

	maxLinkDelayMS, linkRiskUnits, err := treeDelayAndRisk(request.SourceStationID, tree, linkIndex)
	if err != nil {
		return TreeCost{}, false, err
	}
	estimatedLatencyMS, ok := checkedAddInt64(estimatedCycleMS, maxLinkDelayMS)
	if !ok {
		return TreeCost{}, false, fmt.Errorf("tree cost: delivery latency overflows int64")
	}
	if estimatedLatencyMS > request.Constraints.MaxPlayoutLatencyMS {
		return TreeCost{}, false, nil
	}

	maxAirtimeMilliPct := int64(math.Round(request.Constraints.MaxVoiceAirtimePct * 1_000))
	if voiceAirtimeMilliPct > maxAirtimeMilliPct {
		return TreeCost{}, false, nil
	}

	batteryCostUnits, err := aggregateBatteryCostUnits(relayOwners, relayIndex)
	if err != nil {
		return TreeCost{}, false, err
	}

	return TreeCost{
		CoveredMemberCount:            len(tree.Coverage.CoveredMemberIDs),
		SelectedMaxRelayDepth:         tree.MaxRelayDepth,
		TxOpportunityCount:            txOpportunityCount,
		AggregateLinkRiskUnits:        linkRiskUnits,
		AggregateBatteryCostUnits:     batteryCostUnits,
		EstimatedScheduleCycleMS:      estimatedCycleMS,
		EstimatedMaxDeliveryLatencyMS: estimatedLatencyMS,
		VoiceAirtimeMilliPct:          voiceAirtimeMilliPct,
	}, true, nil
}

func selectedRelayOwners(sourceID uint16, edges []RelayTreeEdge) []uint16 {
	seen := make(map[uint16]struct{})
	owners := make([]uint16, 0)
	for _, edge := range edges {
		if edge.ParentStationID == sourceID {
			continue
		}
		if _, ok := seen[edge.ParentStationID]; ok {
			continue
		}
		seen[edge.ParentStationID] = struct{}{}
		owners = append(owners, edge.ParentStationID)
	}
	return owners
}

func treeDelayAndRisk(sourceID uint16, tree RelayTree, linkIndex map[[2]uint16]DirectedLink) (int64, int64, error) {
	delayByStation := map[uint16]int64{sourceID: 0}
	maxCoveredDelay := int64(0)
	linkRiskUnits := int64(0)
	covered := make(map[uint16]struct{}, len(tree.Coverage.CoveredMemberIDs))
	for _, memberID := range tree.Coverage.CoveredMemberIDs {
		covered[memberID] = struct{}{}
	}

	for _, edge := range tree.Edges {
		link, ok := linkIndex[[2]uint16{edge.ParentStationID, edge.ChildStationID}]
		if !ok {
			return 0, 0, fmt.Errorf("tree cost: missing link %d->%d", edge.ParentStationID, edge.ChildStationID)
		}
		parentDelay, ok := delayByStation[edge.ParentStationID]
		if !ok {
			return 0, 0, fmt.Errorf("tree cost: parent station_id=%d has no delay path", edge.ParentStationID)
		}
		childDelay, ok := checkedAddInt64(parentDelay, link.DelayMS)
		if !ok {
			return 0, 0, fmt.Errorf("tree cost: cumulative link delay overflows int64")
		}
		delayByStation[edge.ChildStationID] = childDelay
		if _, isCovered := covered[edge.ChildStationID]; isCovered && childDelay > maxCoveredDelay {
			maxCoveredDelay = childDelay
		}
		risk := int64(math.Round(((1 - link.Quality) + link.Loss) * 1_000_000))
		linkRiskUnits, ok = checkedAddInt64(linkRiskUnits, risk)
		if !ok {
			return 0, 0, fmt.Errorf("tree cost: aggregate link risk overflows int64")
		}
	}
	return maxCoveredDelay, linkRiskUnits, nil
}

func aggregateBatteryCostUnits(relayOwners []uint16, relayIndex map[uint16]RelayState) (int64, error) {
	total := int64(0)
	for _, ownerID := range relayOwners {
		state, ok := relayIndex[ownerID]
		if !ok {
			return 0, fmt.Errorf("tree cost: relay owner station_id=%d has no relay state", ownerID)
		}
		cost := int64(math.Round((1 - state.BatteryFraction) * 1_000_000))
		var addOK bool
		total, addOK = checkedAddInt64(total, cost)
		if !addOK {
			return 0, fmt.Errorf("tree cost: aggregate battery cost overflows int64")
		}
	}
	return total, nil
}

func roundedMilliPercent(numeratorMS int64, framePeriodMS int64) (int64, bool) {
	scaled, ok := checkedMulInt64(numeratorMS, 100_000)
	if !ok {
		return 0, false
	}
	roundedNumerator, ok := checkedAddInt64(scaled, framePeriodMS/2)
	if !ok {
		return 0, false
	}
	return roundedNumerator / framePeriodMS, true
}

func checkedAddInt64(a, b int64) (int64, bool) {
	if b > 0 && a > maxInt64-b {
		return 0, false
	}
	if b < 0 && a < -maxInt64-1-b {
		return 0, false
	}
	return a + b, true
}

func checkedMulInt64(a, b int64) (int64, bool) {
	if a < 0 || b < 0 {
		return 0, false
	}
	if a == 0 || b == 0 {
		return 0, true
	}
	if a > maxInt64/b {
		return 0, false
	}
	return a * b, true
}
