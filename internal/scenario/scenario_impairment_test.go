package scenario

import (
	"math"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/node"
)

// TestIT002_DirectAndRelayDuplicateSuppression implements Scenario B:
// A->B direct and A->R->B are both available, so B must accept one
// copy per origin_key and suppress the later relayed duplicate.
func TestIT002_DirectAndRelayDuplicateSuppression(t *testing.T) {
	cfg := DefaultScenarioConfig()
	cfg.FrameCount = 12
	cfg.SourceToReceiverEnabled = true

	runner := NewScenarioRunner(cfg)
	metrics, err := runner.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if metrics.ReceiverCounters.FramesAccepted != cfg.FrameCount {
		t.Fatalf("frames_accepted = %d, want %d", metrics.ReceiverCounters.FramesAccepted, cfg.FrameCount)
	}
	if metrics.ReceiverCounters.DirectFramesAccepted != cfg.FrameCount {
		t.Fatalf("direct_frames_accepted = %d, want %d", metrics.ReceiverCounters.DirectFramesAccepted, cfg.FrameCount)
	}
	if metrics.ReceiverCounters.RelayedFramesAccepted != 0 {
		t.Fatalf("relayed_frames_accepted = %d, want 0 because direct copies arrive first", metrics.ReceiverCounters.RelayedFramesAccepted)
	}
	if metrics.ReceiverCounters.DuplicatesSuppressed != cfg.FrameCount {
		t.Fatalf("duplicates_suppressed = %d, want %d", metrics.ReceiverCounters.DuplicatesSuppressed, cfg.FrameCount)
	}
	if metrics.ReceiverCounters.PlayoutFramesEmitted != cfg.FrameCount {
		t.Fatalf("playout_frames_emitted = %d, want %d", metrics.ReceiverCounters.PlayoutFramesEmitted, cfg.FrameCount)
	}
	if metrics.RelayForwarded != cfg.FrameCount {
		t.Fatalf("relay_forwarded = %d, want %d", metrics.RelayForwarded, cfg.FrameCount)
	}
	if got := metrics.LinkCounters.Delivered["1->3"]; got != cfg.FrameCount+2 {
		t.Fatalf("direct 1->3 delivered = %d, want %d including call_start/call_end", got, cfg.FrameCount+2)
	}
	if got := metrics.LinkCounters.Delivered["2->3"]; got != cfg.FrameCount {
		t.Fatalf("relay 2->3 delivered = %d, want %d", got, cfg.FrameCount)
	}
}

// TestIT002_LinkDuplicatePctSuppressesDuplicates covers the explicit
// duplicate_pct transport impairment at scenario level.
func TestIT002_LinkDuplicatePctSuppressesDuplicates(t *testing.T) {
	cfg := DefaultScenarioConfig()
	cfg.FrameCount = 8
	cfg.RelayToReceiverDuplicatePct = 100

	runner := NewScenarioRunner(cfg)
	metrics, err := runner.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if metrics.ReceiverCounters.FramesAccepted != cfg.FrameCount {
		t.Fatalf("frames_accepted = %d, want %d", metrics.ReceiverCounters.FramesAccepted, cfg.FrameCount)
	}
	if metrics.ReceiverCounters.DuplicatesSuppressed != cfg.FrameCount {
		t.Fatalf("duplicates_suppressed = %d, want %d", metrics.ReceiverCounters.DuplicatesSuppressed, cfg.FrameCount)
	}
	if metrics.ReceiverCounters.PlayoutFramesEmitted != cfg.FrameCount {
		t.Fatalf("playout_frames_emitted = %d, want %d", metrics.ReceiverCounters.PlayoutFramesEmitted, cfg.FrameCount)
	}
	if got := metrics.LinkCounters.Duplicated["2->3"]; got != cfg.FrameCount {
		t.Fatalf("2->3 duplicated = %d, want %d", got, cfg.FrameCount)
	}
	if got := metrics.LinkCounters.Delivered["2->3"]; got != cfg.FrameCount*2 {
		t.Fatalf("2->3 delivered = %d, want %d including duplicates", got, cfg.FrameCount*2)
	}
}

// TestIT008_GroupMismatchDropsWrongGroup implements Scenario I:
// call_start opens group 100, but voice frames deliberately carry a
// different group_id. Relay and receiver must both drop the voice
// frames as wrong_group, and no wrong-group frame may reach playout.
func TestIT008_GroupMismatchDropsWrongGroup(t *testing.T) {
	cfg := DefaultScenarioConfig()
	cfg.FrameCount = 10
	cfg.SourceToReceiverEnabled = true
	cfg.VoiceGroupIDOverride = 200

	runner := NewScenarioRunner(cfg)
	metrics, err := runner.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if got := metrics.RelayDroppedByReason[node.DropReasonWrongGroup]; got != cfg.FrameCount {
		t.Fatalf("relay wrong_group drops = %d, want %d", got, cfg.FrameCount)
	}
	if got := metrics.ReceiverCounters.DroppedByReason[node.DropReasonWrongGroup]; got != cfg.FrameCount {
		t.Fatalf("receiver wrong_group drops = %d, want %d", got, cfg.FrameCount)
	}
	if metrics.RelayForwarded != 0 {
		t.Fatalf("relay_forwarded = %d, want 0", metrics.RelayForwarded)
	}
	if metrics.ReceiverCounters.FramesAccepted != 0 {
		t.Fatalf("frames_accepted = %d, want 0", metrics.ReceiverCounters.FramesAccepted)
	}
	if metrics.ReceiverCounters.PlayoutFramesEmitted != 0 {
		t.Fatalf("playout_frames_emitted = %d, want 0", metrics.ReceiverCounters.PlayoutFramesEmitted)
	}
	if got := metrics.LinkCounters.Delivered["2->3"]; got != 0 {
		t.Fatalf("relay 2->3 delivered = %d, want 0", got)
	}
}

// TestIT009_CorruptionDropsAndCounts implements Scenario H:
// corrupt R->B deliveries fail validation at the receiver, increment
// invalid drops, and remain visible in per-link metrics.
func TestIT009_CorruptionDropsAndCounts(t *testing.T) {
	cfg := DefaultScenarioConfig()
	cfg.FrameCount = 10
	cfg.RelayToReceiverCorruptionPct = 100

	runner := NewScenarioRunner(cfg)
	metrics, err := runner.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if metrics.RelayForwarded != cfg.FrameCount {
		t.Fatalf("relay_forwarded = %d, want %d", metrics.RelayForwarded, cfg.FrameCount)
	}
	if got := metrics.LinkCounters.Corrupted["2->3"]; got != cfg.FrameCount {
		t.Fatalf("2->3 corrupted = %d, want %d", got, cfg.FrameCount)
	}
	if got := metrics.ReceiverCounters.DroppedByReason[node.DropReasonInvalid]; got != cfg.FrameCount {
		t.Fatalf("receiver invalid drops = %d, want %d", got, cfg.FrameCount)
	}
	if metrics.ReceiverCounters.FramesAccepted != 0 {
		t.Fatalf("frames_accepted = %d, want 0", metrics.ReceiverCounters.FramesAccepted)
	}
	if metrics.ReceiverCounters.PlayoutFramesEmitted != 0 {
		t.Fatalf("playout_frames_emitted = %d, want 0", metrics.ReceiverCounters.PlayoutFramesEmitted)
	}
}

// TestIT012_SeenCacheOverflowEvictsOldest implements Scenario J:
// the relay seen-cache is bounded and evicts oldest entries
// deterministically while still forwarding every first-seen frame.
func TestIT012_SeenCacheOverflowEvictsOldest(t *testing.T) {
	cfg := DefaultScenarioConfig()
	cfg.FrameCount = 5
	cfg.RelaySeenCacheMaxEntries = 3

	runner := NewScenarioRunner(cfg)
	metrics, err := runner.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if metrics.RelayForwarded != cfg.FrameCount {
		t.Fatalf("relay_forwarded = %d, want %d", metrics.RelayForwarded, cfg.FrameCount)
	}
	if metrics.RelaySeenCacheEntries != cfg.RelaySeenCacheMaxEntries {
		t.Fatalf("relay_seen_cache_entries = %d, want %d", metrics.RelaySeenCacheEntries, cfg.RelaySeenCacheMaxEntries)
	}
	if metrics.RelaySeenCacheEvictions != cfg.FrameCount-cfg.RelaySeenCacheMaxEntries {
		t.Fatalf("relay_seen_cache_evictions = %d, want %d",
			metrics.RelaySeenCacheEvictions, cfg.FrameCount-cfg.RelaySeenCacheMaxEntries)
	}
	if metrics.ReceiverCounters.PlayoutFramesEmitted != cfg.FrameCount {
		t.Fatalf("playout_frames_emitted = %d, want %d", metrics.ReceiverCounters.PlayoutFramesEmitted, cfg.FrameCount)
	}
}

func TestScenario_ImpairmentConfigPanics(t *testing.T) {
	tests := []struct {
		name string
		cfg  ScenarioConfig
	}{
		{"SourceToRelayDuplicatePct=-1", func() ScenarioConfig { c := DefaultScenarioConfig(); c.SourceToRelayDuplicatePct = -1; return c }()},
		{"RelayToReceiverDuplicatePct>100", func() ScenarioConfig { c := DefaultScenarioConfig(); c.RelayToReceiverDuplicatePct = 101; return c }()},
		{"SourceToReceiverCorruptionPct=NaN", func() ScenarioConfig {
			c := DefaultScenarioConfig()
			c.SourceToReceiverCorruptionPct = math.NaN()
			return c
		}()},
		{"RelayToReceiverCorruptionPct=Inf", func() ScenarioConfig {
			c := DefaultScenarioConfig()
			c.RelayToReceiverCorruptionPct = math.Inf(1)
			return c
		}()},
		{"SourceToReceiverJitterMs=-1", func() ScenarioConfig { c := DefaultScenarioConfig(); c.SourceToReceiverJitterMs = -1; return c }()},
		{"SourceToReceiverJitterMs=MaxInt64", func() ScenarioConfig {
			c := DefaultScenarioConfig()
			c.SourceToReceiverJitterMs = math.MaxInt64
			return c
		}()},
		{"RelaySeenCacheMaxEntries=-1", func() ScenarioConfig { c := DefaultScenarioConfig(); c.RelaySeenCacheMaxEntries = -1; return c }()},
		{"VoiceGroupIDOverride==GroupID", func() ScenarioConfig { c := DefaultScenarioConfig(); c.VoiceGroupIDOverride = c.GroupID; return c }()},
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
