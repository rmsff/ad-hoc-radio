package scenario

import (
	"fmt"
	"strings"

	"github.com/rmsff/ad-hoc-radio/internal/transport"
)

// TwoHopLiveScenarioMetrics converts bounded two-hop live evidence into
// the public ScenarioMetrics report shape used by adhr-sim run.
func TwoHopLiveScenarioMetrics(report TwoHopLiveScenarioReport) (ScenarioMetrics, error) {
	if len(report.RelayPath) != 2 {
		return ScenarioMetrics{}, fmt.Errorf("two-hop live metrics: relay path length=%d, want 2", len(report.RelayPath))
	}

	linkCounters := transport.DeliveryCounters{
		Delivered:  make(map[string]int),
		Lost:       make(map[string]int),
		Corrupted:  make(map[string]int),
		Duplicated: make(map[string]int),
	}
	linkDelaySumsMs := make(map[string]int64)
	linkDelayCounts := make(map[string]int)

	sourceFramesSent := 0
	relayFramesReceived := 0
	firstHopDelayMs := int64(0)
	firstHopDelaySamples := 0
	maxRxAtMs := report.StartMs

	for _, row := range report.ForwardingTrace {
		if row.HopIndex == 0 {
			sourceFramesSent++
		}
		if row.ReceivedByNode && row.HopIndex < 2 {
			relayFramesReceived++
		}
		if !row.ReceivedByNode && !row.AcceptedByNode {
			continue
		}

		key := linkReportKey(row.FromID, row.ToID)
		linkCounters.Delivered[key]++
		delayMs := row.RxAtMs - row.TxAtMs
		if delayMs < 0 {
			delayMs = 0
		}
		linkDelaySumsMs[key] += delayMs
		linkDelayCounts[key]++
		if row.HopIndex == 0 {
			firstHopDelayMs += delayMs
			firstHopDelaySamples++
		}
		if row.RxAtMs > maxRxAtMs {
			maxRxAtMs = row.RxAtMs
		}
	}

	if firstHopDelaySamples > 0 {
		firstHopDelayMs /= int64(firstHopDelaySamples)
	}
	simDurationMs := maxRxAtMs - report.StartMs
	if simDurationMs < 0 {
		return ScenarioMetrics{}, fmt.Errorf("two-hop live metrics: max_rx_at_ms=%d before start_ms=%d", maxRxAtMs, report.StartMs)
	}
	relayPathDelayMs := report.EndToEndLatencyMs - firstHopDelayMs
	if relayPathDelayMs < 0 {
		relayPathDelayMs = 0
	}

	latencyCount := report.ReceiverCounters.FramesAccepted
	playoutLatencySamplesMs := repeatInt64(report.EndToEndLatencyMs, latencyCount)

	return ScenarioMetrics{
		SourceFramesCreated:       report.SourceFramesCreated,
		SourceFramesSent:          sourceFramesSent,
		RelayFramesReceived:       relayFramesReceived,
		RelayForwarded:            report.Relay1FramesForwarded + report.Relay2FramesForwarded,
		RelayDroppedByReason:      nil,
		RelaySeenCacheEntries:     report.Relay1FramesForwarded + report.Relay2FramesForwarded,
		RelaySeenCacheEvictions:   0,
		ReceiverCounters:          report.ReceiverCounters,
		LinkCounters:              linkCounters,
		LinkDelaySumsMs:           linkDelaySumsMs,
		LinkDelayCounts:           linkDelayCounts,
		EventCount:                report.EventCount,
		EventHash:                 report.EventHash,
		TwoHopLive:                twoHopLiveMetricsSection(report),
		OutcomeOverride:           twoHopLiveMetricsOutcome(report.Decision),
		Notes:                     twoHopLiveMetricsNotes(report.FailReasons),
		ScenarioID:                report.ScenarioID,
		Seed:                      report.Seed,
		SimDurationMs:             simDurationMs,
		SourceID:                  report.SourceID,
		RelayID:                   report.RelayPath[0],
		ReceiverID:                report.ReceiverID,
		StartMs:                   report.StartMs,
		FramePeriodMs:             report.VoicePeriodMs,
		LinkDelayMs:               report.LinkDelayMs,
		ReceiverJitterBufferMs:    0,
		PlayoutLatencySamplesMs:   playoutLatencySamplesMs,
		SourceToRelaySamplesMs:    repeatInt64(firstHopDelayMs, latencyCount),
		RelayToReceiverSamplesMs:  repeatInt64(relayPathDelayMs, latencyCount),
		PlayoutWaitSamplesMs:      repeatInt64(0, latencyCount),
		SlotModeEnabled:           true,
		SlotDurationMs:            report.SourceSlotMs,
		OutOfSlotTxCount:          report.SlotMissCount,
		RelaySlotWaitCount:        0,
		RelayForwardQueueMaxDepth: 1,
		RelayForwardQueueDrops:    0,
		Transmissions:             nil,
		LinkReportDefaults: []LinkEndpoint{
			{From: report.SourceID, To: report.RelayPath[0]},
			{From: report.RelayPath[0], To: report.RelayPath[1]},
			{From: report.RelayPath[1], To: report.ReceiverID},
		},
	}, nil
}

func twoHopLiveMetricsOutcome(decision string) string {
	switch decision {
	case TwoHopLiveOutcomePass:
		return "pass"
	case TwoHopLiveOutcomeFail:
		return "fail"
	default:
		return "inconclusive"
	}
}

func twoHopLiveMetricsNotes(reasons []string) string {
	if len(reasons) == 0 {
		return ""
	}
	return "two_hop_live_fail_reasons=" + strings.Join(reasons, ",")
}

func twoHopLiveMetricsSection(report TwoHopLiveScenarioReport) *TwoHopLiveReport {
	return &TwoHopLiveReport{
		RelayPath:             cloneUint16s(report.RelayPath),
		SourceWireMode:        report.SourceWireMode,
		ChannelBitrateBps:     report.ChannelBitrateBps,
		SourceFrameBytes:      report.SourceFrameBytes,
		RelayFrameBytes:       report.RelayFrameBytes,
		SourceSlotMs:          report.SourceSlotMs,
		RelaySlotMs:           report.RelaySlotMs,
		GuardTimeMs:           report.GuardTimeMs,
		LinkDelayMs:           report.LinkDelayMs,
		SourceAirtimeMs:       report.SourceMeasurement.AirtimeMs,
		RelayAirtimeMs:        report.RelayMeasurement.AirtimeMs,
		EndToEndLatencyMs:     report.EndToEndLatencyMs,
		CycleOccupiedMs:       report.CycleOccupiedMs,
		CycleSpareMs:          report.CycleSpareMs,
		SlotMissCount:         report.SlotMissCount,
		RelayReadinessMisses:  report.RelayReadinessMissCount,
		Relay1FramesForwarded: report.Relay1FramesForwarded,
		Relay2FramesForwarded: report.Relay2FramesForwarded,
	}
}

func repeatInt64(value int64, count int) []int64 {
	if count <= 0 {
		return nil
	}
	out := make([]int64, count)
	for i := range out {
		out[i] = value
	}
	return out
}

func cloneUint16s(in []uint16) []uint16 {
	if in == nil {
		return nil
	}
	out := make([]uint16, len(in))
	copy(out, in)
	return out
}
