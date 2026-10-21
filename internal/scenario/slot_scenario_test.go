package scenario

import (
	"math"
	"reflect"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/node"
	"github.com/rmsff/ad-hoc-radio/internal/protocol"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

func runTwoSlotScenario(t *testing.T) ScenarioMetrics {
	t.Helper()
	runner := NewScenarioRunner(DefaultTwoSlotScenarioConfig())
	metrics, err := runner.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	return metrics
}

// TestIT005_TwoSlotSFRTiming is Scenario F / IT-005: Source transmits
// only in source_slot, Relay transmits only in relay_slot, and no
// out-of-slot transmissions occur.
//
// Traceability: IT-005 (FR-6, FR-7, FR-11).
func TestIT005_TwoSlotSFRTiming(t *testing.T) {
	metrics := runTwoSlotScenario(t)

	if metrics.ScenarioID != TwoSlotScenarioID {
		t.Errorf("ScenarioID = %q, want %q", metrics.ScenarioID, TwoSlotScenarioID)
	}
	if !metrics.SlotModeEnabled {
		t.Fatal("SlotModeEnabled = false, want true")
	}
	if metrics.SlotDurationMs != DefaultSlotDurationMs {
		t.Errorf("SlotDurationMs = %d, want %d", metrics.SlotDurationMs, DefaultSlotDurationMs)
	}
	if metrics.OutOfSlotTxCount != 0 {
		t.Errorf("out_of_slot_tx_count = %d, want 0", metrics.OutOfSlotTxCount)
	}
	// RelaySlotWaitCount == DefaultFrameCount is specific to the default
	// config where LinkDelayMs (10) < SlotDurationMs (40): the relay
	// always receives inside the source slot and must wait. With
	// LinkDelayMs == SlotDurationMs the relay receives exactly at the
	// relay slot boundary and does not wait (RelaySlotWaitCount == 0).
	if metrics.RelaySlotWaitCount != DefaultFrameCount {
		t.Errorf("relay_slot_wait_count = %d, want %d", metrics.RelaySlotWaitCount, DefaultFrameCount)
	}
	if metrics.ReceiverCounters.FramesAccepted != DefaultFrameCount {
		t.Errorf("frames_accepted = %d, want %d", metrics.ReceiverCounters.FramesAccepted, DefaultFrameCount)
	}
	if metrics.RelayForwarded != DefaultFrameCount {
		t.Errorf("relay_forwarded = %d, want %d", metrics.RelayForwarded, DefaultFrameCount)
	}
	if metrics.ReceiverCounters.LateFrames != 0 {
		t.Errorf("Story 3.2 AC-3: late_frames = %d, want 0", metrics.ReceiverCounters.LateFrames)
	}
	if metrics.ReceiverCounters.MissingSequences != 0 {
		t.Errorf("Story 3.2 AC-3: missing_sequences = %d, want 0", metrics.ReceiverCounters.MissingSequences)
	}
	if metrics.ReceiverCounters.PlayoutFramesEmitted != DefaultFrameCount {
		t.Errorf("playout_frames_emitted = %d, want %d", metrics.ReceiverCounters.PlayoutFramesEmitted, DefaultFrameCount)
	}

	var sourceVoice, relayVoice int
	for _, tx := range metrics.Transmissions {
		if !tx.InAssignedSlot {
			t.Errorf("out-of-slot tx: %+v", tx)
		}
		if tx.FrameType != protocol.FrameTypeVoice {
			continue
		}
		switch tx.Role {
		case timing.SlotOwnerSource:
			sourceVoice++
			assertVoiceTxSlot(t, tx, 0, timing.SourceSlotName, timing.SlotOwnerSource)
		case timing.SlotOwnerRelay:
			relayVoice++
			assertVoiceTxSlot(t, tx, 1, timing.RelaySlotName, timing.SlotOwnerRelay)
		default:
			t.Errorf("unexpected voice tx role %q", tx.Role)
		}
	}
	if sourceVoice != DefaultFrameCount {
		t.Errorf("source voice transmissions = %d, want %d", sourceVoice, DefaultFrameCount)
	}
	if relayVoice != DefaultFrameCount {
		t.Errorf("relay voice transmissions = %d, want %d", relayVoice, DefaultFrameCount)
	}
}

func TestIT005_RelayWaitsUntilRelaySlot(t *testing.T) {
	metrics := runTwoSlotScenario(t)

	sourceSeq0, ok := findVoiceTx(metrics.Transmissions, timing.SlotOwnerSource, 0)
	if !ok {
		t.Fatal("source voice seq=0 transmission not found")
	}
	if sourceSeq0.TimeMs != 0 {
		t.Errorf("source seq=0 tx time = %d, want 0", sourceSeq0.TimeMs)
	}

	relaySeq0, ok := findVoiceTx(metrics.Transmissions, timing.SlotOwnerRelay, 0)
	if !ok {
		t.Fatal("relay voice seq=0 transmission not found")
	}
	if relaySeq0.TimeMs != DefaultSlotDurationMs {
		t.Errorf("relay seq=0 tx time = %d, want %d", relaySeq0.TimeMs, DefaultSlotDurationMs)
	}
	if relaySeq0.TimeMs <= DefaultTwoSlotScenarioConfig().LinkDelayMs {
		t.Errorf("relay seq=0 did not wait after receive: tx=%d link_delay=%d", relaySeq0.TimeMs, DefaultTwoSlotScenarioConfig().LinkDelayMs)
	}
}

func TestIT005_TwoSlotDeterminism(t *testing.T) {
	m1 := runTwoSlotScenario(t)
	m2 := runTwoSlotScenario(t)

	if m1.EventHash != m2.EventHash {
		t.Errorf("event hash mismatch:\n  run1=%s\n  run2=%s", m1.EventHash, m2.EventHash)
	}
	if !reflect.DeepEqual(m1, m2) {
		t.Errorf("two-slot metrics mismatch:\n  run1=%+v\n  run2=%+v", m1, m2)
	}
}

func TestSlotScenario_InvalidConfigPanics(t *testing.T) {
	tests := []struct {
		name string
		cfg  ScenarioConfig
	}{
		{"slot enabled duration zero", func() ScenarioConfig { c := DefaultTwoSlotScenarioConfig(); c.SlotDurationMs = 0; return c }()},
		{"frame period mismatch", func() ScenarioConfig {
			c := DefaultTwoSlotScenarioConfig()
			c.FramePeriodMs = c.SlotDurationMs
			return c
		}()},
		{"link delay exceeds slot duration", func() ScenarioConfig {
			c := DefaultTwoSlotScenarioConfig()
			c.LinkDelayMs = c.SlotDurationMs + 1
			return c
		}()},
		{"slot duration overflow", func() ScenarioConfig {
			c := DefaultTwoSlotScenarioConfig()
			c.SlotDurationMs = math.MaxInt64
			c.FramePeriodMs = math.MaxInt64
			return c
		}()},
		{"negative disabled slot duration", func() ScenarioConfig { c := DefaultScenarioConfig(); c.SlotDurationMs = -1; return c }()},
		{"negative relay forward queue max depth", func() ScenarioConfig {
			c := DefaultTwoSlotScenarioConfig()
			c.RelayForwardQueueMaxDepth = -1
			return c
		}()},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("NewScenarioRunner with %s did not panic", tc.name)
				}
			}()
			NewScenarioRunner(tc.cfg)
		})
	}
}

func TestSlotScenario_AllowsExplicitTimingStress(t *testing.T) {
	cfg := DefaultTwoSlotScenarioConfig()
	cfg.FrameCount = 2
	cfg.FramePeriodMs = cfg.SlotDurationMs
	cfg.AllowSlotTimingStress = true

	runner := NewScenarioRunner(cfg)
	if _, err := runner.Run(); err != nil {
		t.Fatalf("Run failed with explicit timing stress: %v", err)
	}
}

// TestIT005_RelayNoWaitAtSlotBoundary proves that when LinkDelayMs ==
// SlotDurationMs, the relay receives exactly at the relay slot start
// and does not wait (RelaySlotWaitCount == 0). This documents the
// config-specific nature of the RelaySlotWaitCount assertion in
// TestIT005_TwoSlotSFRTiming.
func TestIT005_RelayNoWaitAtSlotBoundary(t *testing.T) {
	cfg := DefaultTwoSlotScenarioConfig()
	cfg.LinkDelayMs = cfg.SlotDurationMs // relay receives exactly at relay slot start

	runner := NewScenarioRunner(cfg)
	metrics, err := runner.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if metrics.RelaySlotWaitCount != 0 {
		t.Errorf("relay_slot_wait_count = %d, want 0 (relay receives at relay slot boundary, no wait)", metrics.RelaySlotWaitCount)
	}
	if metrics.OutOfSlotTxCount != 0 {
		t.Errorf("out_of_slot_tx_count = %d, want 0", metrics.OutOfSlotTxCount)
	}
	if metrics.ReceiverCounters.FramesAccepted != DefaultFrameCount {
		t.Errorf("frames_accepted = %d, want %d", metrics.ReceiverCounters.FramesAccepted, DefaultFrameCount)
	}
}

// TestIT045_RelayForwardQueueDropNewest proves the MVP-8.19 bounded
// relay queue policy: under explicit slot stress, a full relay queue
// drops the newest valid forward candidate as queue_full and preserves
// one relay transmission per relay slot.
func TestIT045_RelayForwardQueueDropNewest(t *testing.T) {
	cfg := DefaultTwoSlotScenarioConfig()
	cfg.FrameCount = 6
	cfg.FramePeriodMs = 20
	cfg.AllowSlotTimingStress = true
	cfg.RelayForwardQueueMaxDepth = 1

	runner := NewScenarioRunner(cfg)
	metrics, err := runner.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if metrics.RelayForwardQueueDrops != 4 {
		t.Fatalf("relay.forward_queue_drops = %d, want 4", metrics.RelayForwardQueueDrops)
	}
	if got := metrics.RelayDroppedByReason[node.DropReasonQueueFull]; got != metrics.RelayForwardQueueDrops {
		t.Fatalf("dropped_by_reason.queue_full = %d, want forward_queue_drops %d", got, metrics.RelayForwardQueueDrops)
	}
	if metrics.RelayForwarded != 2 {
		t.Fatalf("relay_forwarded = %d, want 2", metrics.RelayForwarded)
	}
	if metrics.ReceiverCounters.FramesAccepted != 2 {
		t.Fatalf("receiver.frames_accepted = %d, want 2", metrics.ReceiverCounters.FramesAccepted)
	}
	if metrics.RelayForwardQueueMaxDepth != 1 {
		t.Fatalf("relay.forward_queue_max_depth = %d, want 1", metrics.RelayForwardQueueMaxDepth)
	}
	relayVoiceByTime := make(map[int64]int)
	for _, tx := range metrics.Transmissions {
		if tx.Role == timing.SlotOwnerRelay && tx.FrameType == protocol.FrameTypeVoice {
			if !tx.InAssignedSlot {
				t.Fatalf("relay voice transmission out of slot: %+v", tx)
			}
			relayVoiceByTime[tx.TimeMs]++
		}
	}
	for timeMs, count := range relayVoiceByTime {
		if count != 1 {
			t.Fatalf("relay voice transmissions at t=%d = %d, want one per relay slot", timeMs, count)
		}
	}
}

func TestIT045_DefaultRelayForwardQueueDepthAllowsStressBurst(t *testing.T) {
	cfg := DefaultTwoSlotScenarioConfig()
	cfg.FrameCount = 6
	cfg.FramePeriodMs = 20
	cfg.AllowSlotTimingStress = true

	runner := NewScenarioRunner(cfg)
	metrics, err := runner.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if metrics.RelayForwardQueueDrops != 0 {
		t.Fatalf("relay.forward_queue_drops = %d, want 0 with default depth", metrics.RelayForwardQueueDrops)
	}
	if got := metrics.RelayDroppedByReason[node.DropReasonQueueFull]; got != 0 {
		t.Fatalf("dropped_by_reason.queue_full = %d, want 0", got)
	}
	if metrics.RelayForwarded != cfg.FrameCount {
		t.Fatalf("relay_forwarded = %d, want %d", metrics.RelayForwarded, cfg.FrameCount)
	}
	if metrics.RelayForwardQueueMaxDepth < 2 {
		t.Fatalf("relay.forward_queue_max_depth = %d, want stress depth >= 2", metrics.RelayForwardQueueMaxDepth)
	}
}

func TestIT045_QueuedFrameStaleAtReservedTransmitDoesNotHoldSlot(t *testing.T) {
	cfg := DefaultTwoSlotScenarioConfig()
	cfg.FrameCount = 10
	cfg.FramePeriodMs = 20
	cfg.AllowSlotTimingStress = true

	runner := NewScenarioRunner(cfg)
	metrics, err := runner.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if got := metrics.RelayDroppedByReason[node.DropReasonStaleFrame]; got != 1 {
		t.Fatalf("dropped_by_reason.stale_frame = %d, want 1", got)
	}
	if got := metrics.RelayDroppedByReason[node.DropReasonQueueFull]; got != 0 {
		t.Fatalf("dropped_by_reason.queue_full = %d, want 0", got)
	}
	if metrics.RelayForwardQueueDrops != 0 {
		t.Fatalf("relay.forward_queue_drops = %d, want 0", metrics.RelayForwardQueueDrops)
	}
	if metrics.RelayForwarded != cfg.FrameCount-1 {
		t.Fatalf("relay_forwarded = %d, want %d", metrics.RelayForwarded, cfg.FrameCount-1)
	}
}

func assertVoiceTxSlot(t *testing.T, tx SlotTransmission, wantIndex int, wantName string, wantOwner timing.SlotOwner) {
	t.Helper()
	if tx.SlotIndex != wantIndex || tx.SlotName != wantName {
		t.Errorf("voice tx seq=%d role=%s slot = (%d,%q), want (%d,%q)",
			tx.Seq, tx.Role, tx.SlotIndex, tx.SlotName, wantIndex, wantName)
	}
	if tx.Role != wantOwner {
		t.Errorf("voice tx seq=%d role = %q, want %q", tx.Seq, tx.Role, wantOwner)
	}
}

func findVoiceTx(transmissions []SlotTransmission, role timing.SlotOwner, seq uint32) (SlotTransmission, bool) {
	for _, tx := range transmissions {
		if tx.FrameType == protocol.FrameTypeVoice && tx.Role == role && tx.Seq == seq {
			return tx, true
		}
	}
	return SlotTransmission{}, false
}
