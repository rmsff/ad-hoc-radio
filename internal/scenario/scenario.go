// Package scenario implements the end-to-end simulation scenarios for
// the Ad Hoc Radio MVP per algorithm-and-experiments.md §10.
//
// Story 2.3 implements Scenario A — nominal one-hop relay (A → R → B)
// with no slots, no loss, and deterministic event ordering. Later
// report-only scenario evidence covers relay election, direct coverage,
// and limited multi-hop experiments without changing the primary
// ScenarioMetrics contract.
//
// The scenario package is the integration layer: it imports sim,
// transport, node, and protocol. Lower layers must not import scenario
// (no import cycles).
package scenario

import (
	"fmt"

	"github.com/rmsff/ad-hoc-radio/internal/node"
	"github.com/rmsff/ad-hoc-radio/internal/protocol"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
	"github.com/rmsff/ad-hoc-radio/internal/transport"
)

// ScenarioMetrics collects results from a scenario run. Fields align
// with docs/protocol/metrics-schema.json where applicable; report
// export lives in metrics_report.go.
type ScenarioMetrics struct {
	SourceFramesCreated       int
	SourceFramesSent          int
	RelayFramesReceived       int
	RelayForwarded            int
	RelayDroppedByReason      map[node.DropReason]int
	RelaySeenCacheEntries     int
	RelaySeenCacheEvictions   int
	ReceiverCounters          node.ReceiverCounters
	LinkCounters              transport.DeliveryCounters
	LinkDelaySumsMs           map[string]int64
	LinkDelayCounts           map[string]int
	EventCount                int
	EventHash                 string
	TwoHopLive                *TwoHopLiveReport
	ScheduledRelayTree        *ScheduledRelayTreeReport
	ScheduledMemberDelivery   []ScheduledMemberDeliveryCount
	OutcomeOverride           string
	Notes                     string
	ScenarioID                string
	Seed                      uint64
	SimDurationMs             int64
	SourceID                  uint16
	RelayID                   uint16
	ReceiverID                uint16
	StartMs                   int64
	FramePeriodMs             int64
	LinkDelayMs               int64
	ReceiverJitterBufferMs    int64
	PlayoutLatencySamplesMs   []int64
	SourceToRelaySamplesMs    []int64
	RelayToReceiverSamplesMs  []int64
	PlayoutWaitSamplesMs      []int64
	SlotModeEnabled           bool
	SlotDurationMs            int64
	OutOfSlotTxCount          int
	RelaySlotWaitCount        int
	RelayForwardQueueMaxDepth int
	RelayForwardQueueDrops    int
	Transmissions             []SlotTransmission
	LinkReportDefaults        []LinkEndpoint
}

// LinkEndpoint is a metrics-report default link row. Runtime counters
// can add more links, but scenario-specific defaults keep zero-count
// topology rows intentional instead of hard-coded to one-hop A -> R -> B.
type LinkEndpoint struct {
	From uint16
	To   uint16
}

// ScenarioRunner orchestrates the A → R → B simulation per
// algorithm-and-experiments.md §10.1.
//
// The runner is NOT safe for concurrent use. The MVP simulator is
// single-threaded (SimulatedClock, event-driven). Run() is single-use:
// a second call returns an error without mutating any state.
type ScenarioRunner struct {
	config       ScenarioConfig
	clock        *sim.SimulatedClock
	queue        *sim.EventQueue
	transport    *transport.LocalTransport
	relay        *node.RelayStation
	receiver     *node.ReceiverStation
	source       *SourceStation
	ran          bool
	scenarioID   string
	slotSchedule *timing.Schedule

	// Collected metrics.
	sourceFramesCreated       int
	relayForwardedCount       int
	relayFramesReceived       int
	outOfSlotTxCount          int
	relaySlotWaitCount        int
	relayForwardQueueMaxDepth int
	relayForwardQueueDrops    int
	pendingRelayTx            []int64
	relayScenarioDropCounters map[node.DropReason]int
	linkDelaySumsMs           map[string]int64
	linkDelayCounts           map[string]int
	playoutLatencySamplesMs   []int64
	sourceToRelaySamplesMs    []int64
	relayToReceiverSamplesMs  []int64
	playoutWaitSamplesMs      []int64
	sourceToRelayDelayByKey   map[transmissionKey]int64
	relayToReceiverDelayByKey map[transmissionKey]int64
	receiverArrivalByKey      map[transmissionKey]int64
	transmissions             []SlotTransmission
}

// NewScenarioRunner creates a runner with the canonical Scenario A
// topology: A→R enabled, R→B enabled, A→B disabled.
//
// Panics on invalid config: FrameCount < 1, FramePeriodMs <= 0,
// StartMs < 0, LinkDelayMs < 0, or station IDs that are zero or
// non-distinct. These are configuration-time programmer errors, not
// runtime conditions.
func NewScenarioRunner(cfg ScenarioConfig) *ScenarioRunner {
	validateScenarioConfig(cfg)
	clock := sim.NewSimulatedClock()
	queue := sim.NewEventQueue()
	top := transport.Topology{
		Links: []transport.Link{
			{
				From:          transport.StationID(cfg.SourceID),
				To:            transport.StationID(cfg.RelayID),
				Enabled:       true,
				LossPct:       cfg.SourceToRelayLossPct,
				FixedDelayMs:  cfg.LinkDelayMs,
				JitterMs:      cfg.SourceToRelayJitterMs,
				DuplicatePct:  cfg.SourceToRelayDuplicatePct,
				CorruptionPct: cfg.SourceToRelayCorruptionPct,
			},
			{
				From:          transport.StationID(cfg.RelayID),
				To:            transport.StationID(cfg.ReceiverID),
				Enabled:       true,
				LossPct:       cfg.RelayToReceiverLossPct,
				FixedDelayMs:  cfg.LinkDelayMs,
				JitterMs:      cfg.RelayToReceiverJitterMs,
				DuplicatePct:  cfg.RelayToReceiverDuplicatePct,
				CorruptionPct: cfg.RelayToReceiverCorruptionPct,
			},
			{
				From:          transport.StationID(cfg.SourceID),
				To:            transport.StationID(cfg.ReceiverID),
				Enabled:       cfg.SourceToReceiverEnabled,
				LossPct:       cfg.SourceToReceiverLossPct,
				FixedDelayMs:  cfg.LinkDelayMs,
				JitterMs:      cfg.SourceToReceiverJitterMs,
				DuplicatePct:  cfg.SourceToReceiverDuplicatePct,
				CorruptionPct: cfg.SourceToReceiverCorruptionPct,
			},
		},
	}
	tr := transport.NewLocalTransport(top, queue, cfg.Seed)

	relayLife := node.NewRelayLifecycle(relayLifecyclePolicyForConfig(cfg))
	relay := node.NewRelayStation(cfg.RelayID, relayLife, node.DefaultRelayPolicy())

	recvLife := node.NewReceiverLifecycle(node.DefaultLifecyclePolicy())
	receiver := node.NewReceiverStation(cfg.ReceiverID, recvLife, receiverPolicyForConfig(cfg))

	source := NewSourceStation(cfg)

	return &ScenarioRunner{
		config:       cfg,
		clock:        clock,
		queue:        queue,
		transport:    tr,
		relay:        relay,
		receiver:     receiver,
		source:       source,
		scenarioID:   scenarioIDForConfig(cfg),
		slotSchedule: newSlotScheduleForConfig(cfg),
	}
}

// Run executes the scenario and returns metrics.
//
// The simulation loop:
//  1. Emit call_start from source, send via transport. Also directly
//     initialize receiver call state (§8.1: "MVP may allow scenario
//     metadata to create call state without transmitting an explicit
//     call_start frame" — the relay does not forward control frames in
//     Story 2.1, so the receiver cannot get call_start via transport).
//  2. Emit voice frames 0..N-1, send via transport.
//  3. Emit call_end, send via transport. Also directly finalize
//     receiver call state.
//  4. Drain the event queue: transport.Run() processes frame_delivery
//     events; the relay callback forwards accepted voice frames back
//     through transport.Send, adding new delivery events. The loop
//     continues until the queue is empty.
func (r *ScenarioRunner) Run() (ScenarioMetrics, error) {
	// Single-use guard: prevent any side effects on a second call.
	// Without this, a second Run() would register callbacks and send
	// call_start before hitting the error on receiver.ProcessCallStart,
	// leaving mutated state in the queue and transport.
	if r.ran {
		return ScenarioMetrics{}, fmt.Errorf("scenario: Run() already called; ScenarioRunner is single-use")
	}
	r.ran = true

	// Register receivers on the transport.
	r.transport.Register(transport.StationID(r.config.RelayID), r.relayCallback)
	r.transport.Register(transport.StationID(r.config.ReceiverID), r.receiverCallback)

	// 1. Call start.
	startFrame, err := r.source.EmitCallStart()
	if err != nil {
		return ScenarioMetrics{}, fmt.Errorf("scenario: emit call_start: %w", err)
	}
	// Encode the call_start frame so transport.Send can broadcast it.
	r.recordTransmission(TransmissionRoleSource, startFrame, r.config.StartMs)
	r.transport.Send(transport.StationID(r.config.SourceID), startFrame, r.config.StartMs)

	// Directly initialize receiver call state (§8.1 MVP allowance:
	// relay does not forward control frames in Story 2.1).
	if err := r.receiver.ProcessCallStart(startFrame, r.config.StartMs); err != nil {
		return ScenarioMetrics{}, fmt.Errorf("scenario: receiver ProcessCallStart: %w", err)
	}

	// 2. Voice frames.
	for seq := 0; seq < r.config.FrameCount; seq++ {
		vf := r.source.EmitVoiceFrame(uint32(seq))
		r.sourceFramesCreated++
		r.recordTransmission(TransmissionRoleSource, vf, vf.SentAtMs)
		r.transport.Send(transport.StationID(r.config.SourceID), vf, vf.SentAtMs)
	}

	// 3. Call end.
	endFrame, err := r.source.EmitCallEnd()
	if err != nil {
		return ScenarioMetrics{}, fmt.Errorf("scenario: emit call_end: %w", err)
	}
	endMs := endFrame.SentAtMs
	r.recordTransmission(TransmissionRoleSource, endFrame, endMs)
	r.transport.Send(transport.StationID(r.config.SourceID), endFrame, endMs)

	// 4. Drain the event queue.
	for r.queue.Len() > 0 {
		r.transport.Run()
		// If a non-frame event is next, the scenario runner (as
		// simulator driver) must handle it. The nominal scenario
		// produces no non-frame events; if one appears, it's a
		// programmer error (e.g., slot scheduling wired without a
		// handler). Surface it loudly rather than silently discarding
		// (transport.Run contract: "Run must never skip or discard
		// non-frame events").
		if ev, ok := r.queue.Peek(); ok {
			if ev.EventType != "frame_delivery" {
				panic(fmt.Sprintf("scenario: unhandled non-frame event type=%q at t=%d; scenario driver does not process non-frame events",
					ev.EventType, ev.TimeMs))
			}
		}
	}

	// Evaluate receiver jitter after all nominal deliveries have arrived,
	// before synthetic call_end flushes lifecycle jitter state. Story 3.2
	// keeps this as direct scenario-driver evaluation rather than adding
	// non-frame jitter_tick events to the shared transport queue. Story
	// 3.3 records playout latency from these deterministic ticks so
	// report latency and receiver playout counters share one source of
	// truth.
	r.evaluateReceiverJitterTimeline()

	// Directly finalize receiver call state AFTER the simulation loop
	// (relay does not forward control frames in Story 2.1, so B cannot
	// get call_end via transport). Calling ProcessCallEnd before the
	// loop would mark the call inactive and cause all voice frames to
	// be dropped as call_not_active.
	if err := r.receiver.ProcessCallEnd(endFrame, endMs); err != nil {
		return ScenarioMetrics{}, fmt.Errorf("scenario: receiver ProcessCallEnd: %w", err)
	}

	// 5. Collect metrics.
	hash, err := r.queue.EventHash()
	if err != nil {
		return ScenarioMetrics{}, fmt.Errorf("scenario: event hash: %w", err)
	}

	relayForwarded := r.relayForwardedCount
	relayDropCounters := mergeDropCounters(r.relay.DropCounters(), r.relayScenarioDropCounters)
	linkCounters := r.transport.Counters()

	recvCounters := r.receiver.Counters()
	emitted := r.queue.Emitted()
	simDurationMs := sim.AddMs(endMs, receiverJitterBufferMsForConfig(r.config))
	for _, ev := range emitted {
		if ev.TimeMs > simDurationMs {
			simDurationMs = ev.TimeMs
		}
	}

	return ScenarioMetrics{
		SourceFramesCreated:       r.sourceFramesCreated,
		SourceFramesSent:          r.sourceFramesCreated,
		RelayFramesReceived:       r.relayFramesReceived,
		RelayForwarded:            relayForwarded,
		RelayDroppedByReason:      relayDropCounters,
		RelaySeenCacheEntries:     r.relay.Lifecycle().SeenEntryCount(),
		RelaySeenCacheEvictions:   r.relay.Lifecycle().SeenEvictionCount(),
		ReceiverCounters:          recvCounters,
		LinkCounters:              *linkCounters,
		LinkDelaySumsMs:           cloneStringInt64Map(r.linkDelaySumsMs),
		LinkDelayCounts:           cloneStringIntMap(r.linkDelayCounts),
		EventCount:                len(r.queue.Emitted()),
		EventHash:                 hash,
		ScenarioID:                r.scenarioID,
		Seed:                      r.config.Seed,
		SimDurationMs:             simDurationMs,
		SourceID:                  r.config.SourceID,
		RelayID:                   r.config.RelayID,
		ReceiverID:                r.config.ReceiverID,
		StartMs:                   r.config.StartMs,
		FramePeriodMs:             r.config.FramePeriodMs,
		LinkDelayMs:               r.config.LinkDelayMs,
		ReceiverJitterBufferMs:    receiverJitterBufferMsForConfig(r.config),
		PlayoutLatencySamplesMs:   cloneInt64s(r.playoutLatencySamplesMs),
		SourceToRelaySamplesMs:    cloneInt64s(r.sourceToRelaySamplesMs),
		RelayToReceiverSamplesMs:  cloneInt64s(r.relayToReceiverSamplesMs),
		PlayoutWaitSamplesMs:      cloneInt64s(r.playoutWaitSamplesMs),
		SlotModeEnabled:           r.config.SlotModeEnabled,
		SlotDurationMs:            r.config.SlotDurationMs,
		OutOfSlotTxCount:          r.outOfSlotTxCount,
		RelaySlotWaitCount:        r.relaySlotWaitCount,
		RelayForwardQueueMaxDepth: r.relayForwardQueueMaxDepth,
		RelayForwardQueueDrops:    r.relayForwardQueueDrops,
		Transmissions:             cloneTransmissions(r.transmissions),
	}, nil
}

// relayCallback is the transport receiver for the relay station. It
// dispatches by frame type: call_start → ProcessCallStart, voice →
// ProcessVoiceFrame (forward if accepted), call_end → ProcessCallEnd.
func (r *ScenarioRunner) relayCallback(d transport.Delivery) {
	nowMs := d.ReceivedAt
	r.relayFramesReceived++
	r.recordObservedLinkDelay(d)

	// Corrupted frame: Decode failed, d.Frame is zero value. Record
	// as invalid via ProcessVoiceFrame (which calls VerifyIntegrity).
	if d.Corrupted {
		r.relay.ProcessVoiceFrame(d.Frame, nowMs)
		return
	}
	r.recordObservedVoiceDelivery(d)

	switch d.Frame.FrameType {
	case protocol.FrameTypeCallStart:
		if err := r.relay.ProcessCallStart(d.Frame, nowMs); err != nil {
			// Log drop; full logging is Story 3.3.
			return
		}
	case protocol.FrameTypeCallEnd:
		if err := r.relay.ProcessCallEnd(d.Frame, nowMs); err != nil {
			return
		}
	case protocol.FrameTypeVoice:
		result := r.relay.ProcessVoiceFrame(d.Frame, nowMs)
		if result.Forwarded != nil {
			r.scheduleRelayForward(*result.Forwarded, nowMs)
		}
	default:
		// Unknown frame type: drop via ProcessVoiceFrame (unsupported).
		r.relay.ProcessVoiceFrame(d.Frame, nowMs)
	}
}

// receiverCallback is the transport receiver for the receiver station.
// It dispatches by frame type: call_start → ProcessCallStart, voice →
// ProcessVoiceFrame (accept/suppress), call_end → ProcessCallEnd.
func (r *ScenarioRunner) receiverCallback(d transport.Delivery) {
	nowMs := d.ReceivedAt
	r.recordObservedLinkDelay(d)

	// Corrupted frame: record as invalid.
	if d.Corrupted {
		r.receiver.ProcessVoiceFrame(d.Frame, nowMs)
		return
	}
	r.recordObservedVoiceDelivery(d)

	switch d.Frame.FrameType {
	case protocol.FrameTypeCallStart:
		if err := r.receiver.ProcessCallStart(d.Frame, nowMs); err != nil {
			return
		}
	case protocol.FrameTypeCallEnd:
		if err := r.receiver.ProcessCallEnd(d.Frame, nowMs); err != nil {
			return
		}
	case protocol.FrameTypeVoice:
		r.receiver.ProcessVoiceFrame(d.Frame, nowMs)
	default:
		r.receiver.ProcessVoiceFrame(d.Frame, nowMs)
	}
}

// Clock returns the simulated clock (for test inspection).
func (r *ScenarioRunner) Clock() *sim.SimulatedClock { return r.clock }

// Queue returns the event queue (for test inspection).
func (r *ScenarioRunner) Queue() *sim.EventQueue { return r.queue }

// Transport returns the transport (for test inspection).
func (r *ScenarioRunner) Transport() *transport.LocalTransport { return r.transport }

// Relay returns the relay station (for test inspection).
func (r *ScenarioRunner) Relay() *node.RelayStation { return r.relay }

// Receiver returns the receiver station (for test inspection).
func (r *ScenarioRunner) Receiver() *node.ReceiverStation { return r.receiver }
