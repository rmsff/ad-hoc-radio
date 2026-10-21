package scenario

import (
	"fmt"
	"math"
	"strconv"

	"github.com/rmsff/ad-hoc-radio/internal/node"
	"github.com/rmsff/ad-hoc-radio/internal/protocol"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
	"github.com/rmsff/ad-hoc-radio/internal/transport"
)

const (
	// TwoSlotScenarioID is Scenario F / IT-005 in traceability.
	TwoSlotScenarioID = "two_slot_sfr"

	// RelayForwardQueueBackpressureScenarioID is MVP-8.20 / IT-046.
	RelayForwardQueueBackpressureScenarioID = "relay_forward_queue_backpressure"

	// DefaultSlotDurationMs is the canonical Story 3.1 two-slot duration.
	DefaultSlotDurationMs int64 = 40
)

const (
	TransmissionRoleSource = timing.SlotOwnerSource
	TransmissionRoleRelay  = timing.SlotOwnerRelay
)

// SlotTransmission is a scenario-level transmit trace used by IT-005
// and metrics export. Transport events record arrival time; this
// records transmit time.
type SlotTransmission struct {
	TimeMs         int64
	StationID      uint16
	NetID          uint16
	SourceID       uint16
	CallID         uint32
	Role           timing.SlotOwner
	FrameType      uint8
	Seq            uint32
	SlotIndex      int
	SlotName       string
	InAssignedSlot bool
}

// DefaultTwoSlotScenarioConfig returns Scenario F: A -> R -> B with
// two-slot SFR timing under ideal simulated clock.
func DefaultTwoSlotScenarioConfig() ScenarioConfig {
	cfg := DefaultScenarioConfig()
	cfg.StartMs = 0
	cfg.FramePeriodMs = DefaultSlotDurationMs * 2
	cfg.LinkDelayMs = 10
	cfg.SlotModeEnabled = true
	cfg.SlotDurationMs = DefaultSlotDurationMs
	return cfg
}

func scenarioIDForConfig(cfg ScenarioConfig) string {
	if cfg.ScenarioID != "" {
		return cfg.ScenarioID
	}
	if cfg.SlotModeEnabled {
		return TwoSlotScenarioID
	}
	return ScenarioID
}

func newSlotScheduleForConfig(cfg ScenarioConfig) *timing.Schedule {
	if !cfg.SlotModeEnabled {
		return nil
	}
	s := timing.NewTwoSlotSFR(cfg.StartMs, cfg.SlotDurationMs)
	return &s
}

func validateSlotConfig(cfg ScenarioConfig) {
	if !cfg.SlotModeEnabled {
		if cfg.SlotDurationMs < 0 {
			panic(fmt.Sprintf("scenario: SlotDurationMs=%d must be >= 0 when slot mode is disabled", cfg.SlotDurationMs))
		}
		return
	}
	if cfg.SlotDurationMs <= 0 {
		panic(fmt.Sprintf("scenario: SlotDurationMs=%d must be > 0 when slot mode is enabled", cfg.SlotDurationMs))
	}
	if cfg.SlotDurationMs > math.MaxInt64/2 {
		panic(fmt.Sprintf("scenario: SlotDurationMs=%d makes two-slot superframe overflow int64", cfg.SlotDurationMs))
	}
	superframeMs := cfg.SlotDurationMs * 2
	if cfg.FramePeriodMs != superframeMs && !cfg.AllowSlotTimingStress {
		panic(fmt.Sprintf("scenario: slot mode requires FramePeriodMs=%d (2*SlotDurationMs), got %d", superframeMs, cfg.FramePeriodMs))
	}
	if cfg.LinkDelayMs > cfg.SlotDurationMs {
		panic(fmt.Sprintf("scenario: slot mode requires LinkDelayMs=%d <= SlotDurationMs=%d so relay can forward in the next relay slot", cfg.LinkDelayMs, cfg.SlotDurationMs))
	}
}

func (r *ScenarioRunner) relayTransmitTime(receivedAtMs int64) int64 {
	if !r.config.SlotModeEnabled {
		return receivedAtMs
	}
	return r.slotSchedule.NextOwnerSlotStart(receivedAtMs, timing.SlotOwnerRelay)
}

func (r *ScenarioRunner) scheduleRelayForward(f protocol.Frame, nowMs int64) {
	txAt := nowMs
	if r.config.SlotModeEnabled {
		r.pruneRelayForwardReservations(nowMs)
		if r.relayForwardQueueFull() {
			r.recordRelayScenarioDrop(node.DropReasonQueueFull)
			r.relayForwardQueueDrops++
			return
		}
		txAt = r.nextRelayForwardSlot(nowMs)
		if relayForwardStaleAt(f, txAt) {
			r.recordRelayScenarioDrop(node.DropReasonStaleFrame)
			return
		}
		r.reserveRelayForwardSlot(txAt)
		if txAt > nowMs {
			r.relaySlotWaitCount++
		}
	}
	f.SentAtMs = txAt
	f.HeaderCRC32 = 0
	f.PayloadCRC32 = 0
	r.recordTransmission(TransmissionRoleRelay, f, txAt)
	r.relayForwardedCount++
	r.transport.Send(transport.StationID(r.config.RelayID), f, txAt)
}

func (r *ScenarioRunner) relayForwardQueueFull() bool {
	maxDepth := relayForwardQueueMaxDepthForConfig(r.config)
	return len(r.pendingRelayTx) >= maxDepth
}

func (r *ScenarioRunner) nextRelayForwardSlot(nowMs int64) int64 {
	txAt := r.relayTransmitTime(nowMs)
	if len(r.pendingRelayTx) > 0 {
		nextAfterLast := r.nextRelaySlotAfter(r.latestPendingRelayTx())
		if nextAfterLast > txAt {
			txAt = nextAfterLast
		}
	}
	return txAt
}

func (r *ScenarioRunner) reserveRelayForwardSlot(txAt int64) {
	r.pendingRelayTx = append(r.pendingRelayTx, txAt)
	if len(r.pendingRelayTx) > r.relayForwardQueueMaxDepth {
		r.relayForwardQueueMaxDepth = len(r.pendingRelayTx)
	}
}

func (r *ScenarioRunner) pruneRelayForwardReservations(nowMs int64) {
	pending := r.pendingRelayTx[:0]
	for _, txAt := range r.pendingRelayTx {
		if r.relaySlotEndMs(txAt) > nowMs {
			pending = append(pending, txAt)
		}
	}
	r.pendingRelayTx = pending
}

func (r *ScenarioRunner) latestPendingRelayTx() int64 {
	latest := r.pendingRelayTx[0]
	for _, txAt := range r.pendingRelayTx[1:] {
		if txAt > latest {
			latest = txAt
		}
	}
	return latest
}

func (r *ScenarioRunner) relaySlotEndMs(txAt int64) int64 {
	return sim.AddMs(r.relaySlotStartMs(txAt), r.config.SlotDurationMs)
}

func (r *ScenarioRunner) nextRelaySlotAfter(txAt int64) int64 {
	return r.slotSchedule.NextOwnerSlotStart(r.relaySlotEndMs(txAt), timing.SlotOwnerRelay)
}

func (r *ScenarioRunner) relaySlotStartMs(timeMs int64) int64 {
	if timeMs < r.slotSchedule.OriginMs {
		panic(fmt.Sprintf("scenario: relay tx time %d before slot schedule origin %d", timeMs, r.slotSchedule.OriginMs))
	}
	elapsed := timeMs - r.slotSchedule.OriginMs
	slotNumber := elapsed / r.slotSchedule.SlotDurationMs
	slotOffset := mulScenarioMs(slotNumber, r.slotSchedule.SlotDurationMs, "relay slot start")
	return sim.AddMs(r.slotSchedule.OriginMs, slotOffset)
}

func relayForwardStaleAt(f protocol.Frame, txAtMs int64) bool {
	if txAtMs < f.CreatedAtMs {
		return true
	}
	return txAtMs-f.CreatedAtMs > node.DefaultRelayStaleThresholdMs
}

func (r *ScenarioRunner) recordRelayScenarioDrop(reason node.DropReason) {
	if r.relayScenarioDropCounters == nil {
		r.relayScenarioDropCounters = make(map[node.DropReason]int)
	}
	r.relayScenarioDropCounters[reason]++
}

func (r *ScenarioRunner) recordTransmission(role timing.SlotOwner, f protocol.Frame, timeMs int64) {
	if !r.config.SlotModeEnabled {
		stationID := r.config.SourceID
		if role == timing.SlotOwnerRelay {
			stationID = r.config.RelayID
		}
		r.transmissions = append(r.transmissions, SlotTransmission{
			TimeMs:         timeMs,
			StationID:      stationID,
			NetID:          f.NetID,
			SourceID:       f.SourceID,
			CallID:         f.CallID,
			Role:           role,
			FrameType:      f.FrameType,
			Seq:            f.Seq,
			SlotIndex:      -1,
			InAssignedSlot: true,
		})
		return
	}
	stationID := r.config.SourceID
	if role == timing.SlotOwnerRelay {
		stationID = r.config.RelayID
	}
	slot := r.slotSchedule.SlotAt(timeMs)
	inAssignedSlot := slot.Owner == role
	if !inAssignedSlot {
		r.outOfSlotTxCount++
	}
	r.transmissions = append(r.transmissions, SlotTransmission{
		TimeMs:         timeMs,
		StationID:      stationID,
		NetID:          f.NetID,
		SourceID:       f.SourceID,
		CallID:         f.CallID,
		Role:           role,
		FrameType:      f.FrameType,
		Seq:            f.Seq,
		SlotIndex:      r.slotSchedule.SlotIndexAt(timeMs),
		SlotName:       slot.Name,
		InAssignedSlot: inAssignedSlot,
	})
}

func (r *ScenarioRunner) evaluateReceiverJitterTimeline() {
	jitterBufferMs := receiverJitterBufferMsForConfig(r.config)
	for seq := 0; seq < r.config.FrameCount; seq++ {
		sourceCreatedMs := sim.AddMs(r.config.StartMs, int64(seq)*r.config.FramePeriodMs)
		playoutAtMs := sim.AddMs(sourceCreatedMs, jitterBufferMs)
		key := transmissionKey{
			NetID:    r.config.NetID,
			SourceID: r.config.SourceID,
			CallID:   r.config.CallID,
			Seq:      uint32(seq),
		}
		if receiverRxMs, ok := r.receiverArrivalByKey[key]; ok {
			if receiverRxMs > playoutAtMs {
				playoutAtMs = receiverRxMs
			}
		}
		eval := r.receiver.EvaluateJitter(playoutAtMs)
		r.recordPlayoutLatency(eval, playoutAtMs)
	}
}

func (r *ScenarioRunner) recordPlayoutLatency(eval node.JitterEvaluation, playoutAtMs int64) {
	for _, frame := range eval.Emitted {
		key := transmissionKeyFromFrame(frame)
		sourceToRelayMs, ok := r.sourceToRelayDelayByKey[key]
		if !ok {
			continue
		}
		relayToReceiverMs, ok := r.relayToReceiverDelayByKey[key]
		if !ok {
			continue
		}
		receiverRxMs, ok := r.receiverArrivalByKey[key]
		if !ok {
			continue
		}
		effectivePlayoutAtMs := playoutAtMs
		if receiverRxMs > effectivePlayoutAtMs {
			effectivePlayoutAtMs = receiverRxMs
		}
		r.playoutLatencySamplesMs = append(r.playoutLatencySamplesMs, effectivePlayoutAtMs-frame.CreatedAtMs)
		r.sourceToRelaySamplesMs = append(r.sourceToRelaySamplesMs, sourceToRelayMs)
		r.relayToReceiverSamplesMs = append(r.relayToReceiverSamplesMs, relayToReceiverMs)
		r.playoutWaitSamplesMs = append(r.playoutWaitSamplesMs, effectivePlayoutAtMs-receiverRxMs)
	}
}

func (r *ScenarioRunner) recordObservedLinkDelay(d transport.Delivery) {
	if d.Corrupted {
		return
	}
	if d.ReceivedAt < d.Frame.SentAtMs {
		return
	}
	if r.linkDelaySumsMs == nil {
		r.linkDelaySumsMs = make(map[string]int64)
	}
	if r.linkDelayCounts == nil {
		r.linkDelayCounts = make(map[string]int)
	}
	key := linkReportKey(uint16(d.From), uint16(d.To))
	r.linkDelaySumsMs[key] += d.ReceivedAt - d.Frame.SentAtMs
	r.linkDelayCounts[key]++
}

func (r *ScenarioRunner) recordObservedVoiceDelivery(d transport.Delivery) {
	if d.Frame.FrameType != protocol.FrameTypeVoice {
		return
	}
	if d.ReceivedAt < d.Frame.SentAtMs {
		return
	}
	key := transmissionKeyFromFrame(d.Frame)
	delayMs := d.ReceivedAt - d.Frame.SentAtMs

	switch {
	case uint16(d.From) == r.config.SourceID && uint16(d.To) == r.config.RelayID:
		if r.sourceToRelayDelayByKey == nil {
			r.sourceToRelayDelayByKey = make(map[transmissionKey]int64)
		}
		recordMinDelay(r.sourceToRelayDelayByKey, key, delayMs)
	case uint16(d.From) == r.config.RelayID && uint16(d.To) == r.config.ReceiverID:
		if r.relayToReceiverDelayByKey == nil {
			r.relayToReceiverDelayByKey = make(map[transmissionKey]int64)
		}
		if r.receiverArrivalByKey == nil {
			r.receiverArrivalByKey = make(map[transmissionKey]int64)
		}
		recordMinDelay(r.relayToReceiverDelayByKey, key, delayMs)
		recordMinDelay(r.receiverArrivalByKey, key, d.ReceivedAt)
	}
}

func recordMinDelay(delays map[transmissionKey]int64, key transmissionKey, value int64) {
	if existing, ok := delays[key]; !ok || value < existing {
		delays[key] = value
	}
}

type transmissionKey struct {
	NetID    uint16
	SourceID uint16
	CallID   uint32
	Seq      uint32
}

func transmissionKeyFromFrame(f protocol.Frame) transmissionKey {
	return transmissionKey{NetID: f.NetID, SourceID: f.SourceID, CallID: f.CallID, Seq: f.Seq}
}

func relayTxIndex(transmissions []SlotTransmission) map[transmissionKey]int64 {
	out := make(map[transmissionKey]int64)
	for _, tx := range transmissions {
		if tx.Role != timing.SlotOwnerRelay || tx.FrameType != protocol.FrameTypeVoice {
			continue
		}
		key := transmissionKey{NetID: tx.NetID, SourceID: tx.SourceID, CallID: tx.CallID, Seq: tx.Seq}
		if existing, ok := out[key]; !ok || tx.TimeMs < existing {
			out[key] = tx.TimeMs
		}
	}
	return out
}

func linkReportKey(from, to uint16) string {
	return strconv.FormatUint(uint64(from), 10) + "->" + strconv.FormatUint(uint64(to), 10)
}

func mergeDropCounters(primary map[node.DropReason]int, extra map[node.DropReason]int) map[node.DropReason]int {
	if len(extra) == 0 {
		return primary
	}
	out := make(map[node.DropReason]int, len(primary)+len(extra))
	for reason, count := range primary {
		out[reason] = count
	}
	for reason, count := range extra {
		out[reason] += count
	}
	return out
}

func mulScenarioMs(a, b int64, label string) int64 {
	if a < 0 || b < 0 {
		panic(fmt.Sprintf("scenario: %s has negative operand (%d, %d)", label, a, b))
	}
	if a != 0 && b > math.MaxInt64/a {
		panic(fmt.Sprintf("scenario: %s overflows int64 (%d * %d)", label, a, b))
	}
	return a * b
}

func cloneTransmissions(in []SlotTransmission) []SlotTransmission {
	if in == nil {
		return nil
	}
	out := make([]SlotTransmission, len(in))
	copy(out, in)
	return out
}

func cloneInt64s(in []int64) []int64 {
	if in == nil {
		return nil
	}
	out := make([]int64, len(in))
	copy(out, in)
	return out
}

func cloneStringInt64Map(in map[string]int64) map[string]int64 {
	if in == nil {
		return nil
	}
	out := make(map[string]int64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneStringIntMap(in map[string]int) map[string]int {
	if in == nil {
		return nil
	}
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
