package scenario

import (
	"math"
	"reflect"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/node"
)

// runScenario runs the default scenario and returns metrics. Helper to
// avoid repetition in test cases.
func runScenario(t *testing.T) ScenarioMetrics {
	t.Helper()
	runner := NewScenarioRunner(DefaultScenarioConfig())
	metrics, err := runner.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	return metrics
}

// TestIT001_NominalARBSOneHopRelay is the canonical Scenario A
// integration test per algorithm-and-experiments.md §10.1.
//
// AC-1: B receives frames only through Relay (direct accepted == 0).
// AC-2: 0% loss → B accepts at least 95 frames.
// AC-3: fixed seed → event hash reproducible.
//
// Traceability: IT-001 (FR-1, FR-3, FR-4, FR-5, FR-10, FR-11, SM-1, SM-2).
func TestIT001_NominalARBSOneHopRelay(t *testing.T) {
	metrics := runScenario(t)

	// Source created 100 voice frames.
	if metrics.SourceFramesCreated != 100 {
		t.Errorf("source_frames_created = %d, want 100", metrics.SourceFramesCreated)
	}

	// AC-2: B accepts at least 95 valid frames under 0% loss.
	if metrics.ReceiverCounters.FramesAccepted < 95 {
		t.Errorf("AC-2: frames_accepted = %d, want >= 95", metrics.ReceiverCounters.FramesAccepted)
	}

	// AC-1 + AC-2: direct accepted frames == 0 (A→B disabled).
	if metrics.ReceiverCounters.DirectFramesAccepted != 0 {
		t.Errorf("AC-1: direct_frames_accepted = %d, want 0 (A→B disabled)", metrics.ReceiverCounters.DirectFramesAccepted)
	}

	// AC-1: all accepted frames are relayed (relay_id = R).
	if metrics.ReceiverCounters.RelayedFramesAccepted != metrics.ReceiverCounters.FramesAccepted {
		t.Errorf("AC-1: relayed_frames_accepted = %d, want == frames_accepted = %d",
			metrics.ReceiverCounters.RelayedFramesAccepted, metrics.ReceiverCounters.FramesAccepted)
	}

	// SM-2: duplicate playback zero.
	if metrics.ReceiverCounters.DuplicatesSuppressed != 0 {
		t.Errorf("SM-2: duplicates_suppressed = %d, want 0 (no duplicates in nominal)", metrics.ReceiverCounters.DuplicatesSuppressed)
	}
	if metrics.ReceiverCounters.LateFrames != 0 {
		t.Errorf("Story 3.2 AC-3: late_frames = %d, want 0", metrics.ReceiverCounters.LateFrames)
	}
	if metrics.ReceiverCounters.MissingSequences != 0 {
		t.Errorf("Story 3.2 AC-3: missing_sequences = %d, want 0", metrics.ReceiverCounters.MissingSequences)
	}
	if metrics.ReceiverCounters.PlayoutFramesEmitted != metrics.ReceiverCounters.FramesAccepted {
		t.Errorf("playout_frames_emitted = %d, want frames_accepted %d",
			metrics.ReceiverCounters.PlayoutFramesEmitted, metrics.ReceiverCounters.FramesAccepted)
	}

	// Relay forwarded all 100 voice frames (0% loss, no drops).
	if metrics.RelayForwarded != 100 {
		t.Errorf("relay_forwarded = %d, want 100", metrics.RelayForwarded)
	}

	// No relay drops in nominal scenario.
	for reason, count := range metrics.RelayDroppedByReason {
		if count != 0 {
			t.Errorf("relay drop reason %s = %d, want 0 in nominal scenario", reason, count)
		}
	}

	// Event hash is non-empty (64 hex chars for SHA-256).
	if len(metrics.EventHash) != 64 {
		t.Errorf("event_hash length = %d, want 64 (SHA-256 hex)", len(metrics.EventHash))
	}

	// Event count: call_start (A→R) + 100 voice (A→R) + call_end (A→R)
	// + 100 voice forwarded (R→B) = 202 frame_delivery events.
	const expectedEventCount = 202
	if metrics.EventCount != expectedEventCount {
		t.Errorf("event_count = %d, want %d (102 A→R + 100 R→B)", metrics.EventCount, expectedEventCount)
	}
}

// TestIT001_AC3_Determinism proves that running the scenario twice
// with the same seed produces identical metrics and event hash.
//
// Traceability: IT-001 (AC-3, SM-5, NFR-1).
func TestIT001_AC3_Determinism(t *testing.T) {
	m1 := runScenario(t)
	m2 := runScenario(t)

	// AC-3: event hash reproducible.
	if m1.EventHash != m2.EventHash {
		t.Errorf("AC-3: event hash mismatch:\n  run1=%s\n  run2=%s", m1.EventHash, m2.EventHash)
	}

	// AC-3: event count reproducible.
	if m1.EventCount != m2.EventCount {
		t.Errorf("AC-3: event_count mismatch: %d vs %d", m1.EventCount, m2.EventCount)
	}

	// AC-3: receiver counters reproducible.
	if !reflect.DeepEqual(m1.ReceiverCounters, m2.ReceiverCounters) {
		t.Errorf("AC-3: receiver counters mismatch:\n  %+v\n  %+v", m1.ReceiverCounters, m2.ReceiverCounters)
	}

	// AC-3: full metrics reproducibility (all fields).
	if !reflect.DeepEqual(m1, m2) {
		t.Errorf("AC-3: metrics mismatch:\n  run1=%+v\n  run2=%+v", m1, m2)
	}
}

// TestIT001_NoDirectADelivery proves that the transport delivered zero
// frames directly from A to B (A→B link disabled).
//
// Traceability: IT-001 (AC-1, FR-5).
func TestIT001_NoDirectADelivery(t *testing.T) {
	runner := NewScenarioRunner(DefaultScenarioConfig())
	_, err := runner.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	counters := runner.Transport().Counters()
	directKey := "1->3" // SourceID=1, ReceiverID=3

	if delivered, ok := counters.Delivered[directKey]; ok && delivered > 0 {
		t.Errorf("AC-1: A→B delivered %d frames, want 0 (link disabled)", delivered)
	}
	if lost, ok := counters.Lost[directKey]; ok && lost > 0 {
		t.Errorf("AC-1: A→B lost %d frames, want 0 (link disabled, no sends)", lost)
	}
}

// TestIT001_RelayPathDelivery proves that the transport delivered
// frames on A→R and R→B links.
//
// Traceability: IT-001 (FR-1, FR-3).
func TestIT001_RelayPathDelivery(t *testing.T) {
	runner := NewScenarioRunner(DefaultScenarioConfig())
	_, err := runner.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	counters := runner.Transport().Counters()

	// A→R: call_start + 100 voice + call_end = 102 deliveries.
	arDelivered := counters.Delivered["1->2"]
	if arDelivered < 102 {
		t.Errorf("A→R delivered = %d, want >= 102", arDelivered)
	}

	// R→B: 100 forwarded voice frames.
	rbDelivered := counters.Delivered["2->3"]
	if rbDelivered != 100 {
		t.Errorf("R→B delivered = %d, want 100", rbDelivered)
	}
}

// TestIT001_AllAcceptedFramesHaveRelayID proves that all frames B
// accepted have relay_id = R (not direct from A).
//
// Traceability: IT-001 (AC-1, §10.1 expected: "All B-accepted voice
// frames have relay_id = R").
func TestIT001_AllAcceptedFramesHaveRelayID(t *testing.T) {
	runner := NewScenarioRunner(DefaultScenarioConfig())
	_, err := runner.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Check receiver counters: all accepted are relayed.
	c := runner.Receiver().Counters()
	if c.DirectFramesAccepted != 0 {
		t.Errorf("direct_frames_accepted = %d, want 0", c.DirectFramesAccepted)
	}
	if c.RelayedFramesAccepted != c.FramesAccepted {
		t.Errorf("relayed_frames_accepted = %d, want == frames_accepted = %d",
			c.RelayedFramesAccepted, c.FramesAccepted)
	}
}

// TestIT001_CallLifecycleProved proves that call_start and call_end
// were processed by relay and receiver.
//
// Traceability: IT-001 (FR-10).
func TestIT001_CallLifecycleProved(t *testing.T) {
	runner := NewScenarioRunner(DefaultScenarioConfig())
	_, err := runner.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Relay should have call state (from call_start via transport).
	key := node.CallKey{NetID: DefaultNetID, SourceID: DefaultSourceID, CallID: DefaultCallID}
	state, ok := runner.Relay().Lifecycle().Call(key)
	if !ok {
		t.Fatal("relay call state missing after scenario")
	}
	if state.Active {
		t.Error("relay call still active after call_end")
	}

	// Receiver should have call state (from direct ProcessCallStart).
	state, ok = runner.Receiver().Lifecycle().Call(key)
	if !ok {
		t.Fatal("receiver call state missing after scenario")
	}
	if state.Active {
		t.Error("receiver call still active after call_end")
	}
}

// TestIT001_CustomConfig proves that a non-default config works.
func TestIT001_CustomConfig(t *testing.T) {
	cfg := DefaultScenarioConfig()
	cfg.FrameCount = 10
	cfg.Seed = 999

	runner := NewScenarioRunner(cfg)
	metrics, err := runner.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if metrics.SourceFramesCreated != 10 {
		t.Errorf("source_frames_created = %d, want 10", metrics.SourceFramesCreated)
	}
	if metrics.ReceiverCounters.FramesAccepted != 10 {
		t.Errorf("frames_accepted = %d, want 10", metrics.ReceiverCounters.FramesAccepted)
	}
	if metrics.Seed != 999 {
		t.Errorf("seed = %d, want 999", metrics.Seed)
	}
}

// TestIT001_SameEventCountAcrossSeeds proves that different seeds
// produce the same event count under 0% impairment (impairment draws
// are not used when loss/jitter/duplication are 0%). Hash equality is
// NOT asserted because 0% impairment means the seed has no effect on
// delivery — the hash is identical, not different.
func TestIT001_SameEventCountAcrossSeeds(t *testing.T) {
	cfg1 := DefaultScenarioConfig()
	cfg1.Seed = 42
	runner1 := NewScenarioRunner(cfg1)
	m1, err := runner1.Run()
	if err != nil {
		t.Fatalf("Run1 failed: %v", err)
	}

	cfg2 := DefaultScenarioConfig()
	cfg2.Seed = 999
	runner2 := NewScenarioRunner(cfg2)
	m2, err := runner2.Run()
	if err != nil {
		t.Fatalf("Run2 failed: %v", err)
	}

	// Different seeds may produce same hash for 0% loss/no jitter
	// (deterministic impairment draws are not used). But event count
	// should be the same. This test is a sanity check, not a hard
	// assertion — if impairment is 0%, seed doesn't affect delivery.
	if m1.EventCount != m2.EventCount {
		t.Errorf("event_count differs across seeds: %d vs %d (expected same for 0%% impairment)",
			m1.EventCount, m2.EventCount)
	}
}

// TestScenario_InvalidConfigPanics proves that invalid config values
// panic at construction time (config-time programmer errors).
func TestScenario_InvalidConfigPanics(t *testing.T) {
	tests := []struct {
		name string
		cfg  ScenarioConfig
	}{
		{"FrameCount=0", func() ScenarioConfig { c := DefaultScenarioConfig(); c.FrameCount = 0; return c }()},
		{"FrameCount=-1", func() ScenarioConfig { c := DefaultScenarioConfig(); c.FrameCount = -1; return c }()},
		{"FramePeriodMs=0", func() ScenarioConfig { c := DefaultScenarioConfig(); c.FramePeriodMs = 0; return c }()},
		{"FramePeriodMs=-1", func() ScenarioConfig { c := DefaultScenarioConfig(); c.FramePeriodMs = -1; return c }()},
		{"StartMs=-1", func() ScenarioConfig { c := DefaultScenarioConfig(); c.StartMs = -1; return c }()},
		{"LinkDelayMs=-1", func() ScenarioConfig { c := DefaultScenarioConfig(); c.LinkDelayMs = -1; return c }()},
		{"SourceToRelayLossPct=-1", func() ScenarioConfig { c := DefaultScenarioConfig(); c.SourceToRelayLossPct = -1; return c }()},
		{"SourceToRelayLossPct>100", func() ScenarioConfig { c := DefaultScenarioConfig(); c.SourceToRelayLossPct = 101; return c }()},
		{"SourceToRelayLossPct=NaN", func() ScenarioConfig { c := DefaultScenarioConfig(); c.SourceToRelayLossPct = math.NaN(); return c }()},
		{"SourceToRelayLossPct=Inf", func() ScenarioConfig { c := DefaultScenarioConfig(); c.SourceToRelayLossPct = math.Inf(1); return c }()},
		{"RelayToReceiverLossPct=-1", func() ScenarioConfig { c := DefaultScenarioConfig(); c.RelayToReceiverLossPct = -1; return c }()},
		{"RelayToReceiverLossPct>100", func() ScenarioConfig { c := DefaultScenarioConfig(); c.RelayToReceiverLossPct = 101; return c }()},
		{"RelayToReceiverLossPct=NaN", func() ScenarioConfig { c := DefaultScenarioConfig(); c.RelayToReceiverLossPct = math.NaN(); return c }()},
		{"RelayToReceiverLossPct=Inf", func() ScenarioConfig { c := DefaultScenarioConfig(); c.RelayToReceiverLossPct = math.Inf(1); return c }()},
		{"SourceToRelayJitterMs=-1", func() ScenarioConfig { c := DefaultScenarioConfig(); c.SourceToRelayJitterMs = -1; return c }()},
		{"RelayToReceiverJitterMs=-1", func() ScenarioConfig { c := DefaultScenarioConfig(); c.RelayToReceiverJitterMs = -1; return c }()},
		{"SourceToRelayJitterMs=MaxInt64", func() ScenarioConfig { c := DefaultScenarioConfig(); c.SourceToRelayJitterMs = math.MaxInt64; return c }()},
		{"RelayToReceiverJitterMs=MaxInt64", func() ScenarioConfig {
			c := DefaultScenarioConfig()
			c.RelayToReceiverJitterMs = math.MaxInt64
			return c
		}()},
		{"ReceiverJitterBufferMs=-1", func() ScenarioConfig { c := DefaultScenarioConfig(); c.ReceiverJitterBufferMs = -1; return c }()},
		{"ReceiverLateThresholdMs=-1", func() ScenarioConfig { c := DefaultScenarioConfig(); c.ReceiverLateThresholdMs = -1; return c }()},
		{"SourceID=0", func() ScenarioConfig { c := DefaultScenarioConfig(); c.SourceID = 0; return c }()},
		{"RelayID=0", func() ScenarioConfig { c := DefaultScenarioConfig(); c.RelayID = 0; return c }()},
		{"ReceiverID=0", func() ScenarioConfig { c := DefaultScenarioConfig(); c.ReceiverID = 0; return c }()},
		{"SourceID==RelayID", func() ScenarioConfig { c := DefaultScenarioConfig(); c.SourceID = c.RelayID; return c }()},
		{"RelayID==ReceiverID", func() ScenarioConfig { c := DefaultScenarioConfig(); c.RelayID = c.ReceiverID; return c }()},
		{"SourceID==ReceiverID", func() ScenarioConfig { c := DefaultScenarioConfig(); c.SourceID = c.ReceiverID; return c }()},
		{"TTL=0", func() ScenarioConfig { c := DefaultScenarioConfig(); c.TTL = 0; return c }()},
		{"NetID=0", func() ScenarioConfig { c := DefaultScenarioConfig(); c.NetID = 0; return c }()},
		{"GroupID=0", func() ScenarioConfig { c := DefaultScenarioConfig(); c.GroupID = 0; return c }()},
		{"FrameCount>MaxUint32", func() ScenarioConfig { c := DefaultScenarioConfig(); c.FrameCount = math.MaxUint32 + 1; return c }()},
		{"time horizon overflow", func() ScenarioConfig { c := DefaultScenarioConfig(); c.StartMs = math.MaxInt64 - 10; return c }()},
		{"delivery time horizon overflow", func() ScenarioConfig {
			c := DefaultScenarioConfig()
			c.FrameCount = 1
			c.FramePeriodMs = 10
			c.StartMs = math.MaxInt64 - c.FramePeriodMs
			c.LinkDelayMs = 1
			return c
		}()},
		{"jitter evaluation horizon overflow", func() ScenarioConfig {
			c := DefaultScenarioConfig()
			c.FrameCount = 1
			c.FramePeriodMs = 1
			c.StartMs = math.MaxInt64 - c.FramePeriodMs
			c.LinkDelayMs = 0
			return c
		}()},
		{"late-threshold evaluation horizon overflow", func() ScenarioConfig {
			c := DefaultScenarioConfig()
			c.FrameCount = 1
			c.FramePeriodMs = 1
			c.StartMs = math.MaxInt64 - c.FramePeriodMs
			c.LinkDelayMs = 0
			c.ReceiverJitterBufferMs = 1
			c.ReceiverLateThresholdMs = 2
			return c
		}()},
		{"2*LinkDelay>FramePeriod", func() ScenarioConfig { c := DefaultScenarioConfig(); c.LinkDelayMs = c.FramePeriodMs; return c }()},
		{"LinkDelayMs=MaxInt64 (overflow guard)", func() ScenarioConfig { c := DefaultScenarioConfig(); c.LinkDelayMs = math.MaxInt64; return c }()},
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

// TestScenario_SingleUse proves that calling Run() twice on the same
// runner returns an error without side effects (no mutated queue or
// transport state).
func TestScenario_SingleUse(t *testing.T) {
	runner := NewScenarioRunner(DefaultScenarioConfig())
	_, err := runner.Run()
	if err != nil {
		t.Fatalf("first Run failed: %v", err)
	}

	eventCountAfterFirstRun := len(runner.Queue().Emitted())

	// Second Run should return error immediately (single-use guard)
	// without registering callbacks or sending frames.
	_, err = runner.Run()
	if err == nil {
		t.Fatal("second Run on same runner succeeded; expected error (single-use)")
	}

	// Verify no side effects: event count unchanged.
	eventCountAfterSecondRun := len(runner.Queue().Emitted())
	if eventCountAfterSecondRun != eventCountAfterFirstRun {
		t.Errorf("second Run mutated queue: events %d → %d (expected no change)",
			eventCountAfterFirstRun, eventCountAfterSecondRun)
	}
}

// TestScenario_ScenarioIDConst proves that the ScenarioID returned by
// Run matches the exported const and the traceability registration.
func TestScenario_ScenarioIDConst(t *testing.T) {
	metrics := runScenario(t)
	if metrics.ScenarioID != ScenarioID {
		t.Errorf("ScenarioID = %q, want %q (const)", metrics.ScenarioID, ScenarioID)
	}
	if metrics.ScenarioID != "one_hop_nominal" {
		t.Errorf("ScenarioID = %q, want %q (traceability.md)", metrics.ScenarioID, "one_hop_nominal")
	}
}
