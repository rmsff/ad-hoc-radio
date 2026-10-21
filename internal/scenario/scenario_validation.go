package scenario

import (
	"fmt"
	"math"

	"github.com/rmsff/ad-hoc-radio/internal/node"
	"github.com/rmsff/ad-hoc-radio/internal/protocol"
)

// validateScenarioConfig panics on invalid configuration. Config is
// programmer-supplied at construction time; an invalid one is a
// programmer error, surfaced loudly (same convention as
// SimulatedClock.Advance and Topology.Validate).
func validateScenarioConfig(cfg ScenarioConfig) {
	if cfg.FrameCount < 1 {
		panic(fmt.Sprintf("scenario: FrameCount=%d must be >= 1", cfg.FrameCount))
	}
	if cfg.FrameCount > math.MaxUint32 {
		panic(fmt.Sprintf("scenario: FrameCount=%d exceeds uint32 max %d (seq/last_seq truncation)", cfg.FrameCount, math.MaxUint32))
	}
	if cfg.FramePeriodMs <= 0 || cfg.FramePeriodMs > protocol.MaxFramePeriodMs {
		panic(fmt.Sprintf("scenario: FramePeriodMs=%d must be in 1..%d", cfg.FramePeriodMs, protocol.MaxFramePeriodMs))
	}
	if cfg.StartMs < 0 {
		panic(fmt.Sprintf("scenario: StartMs=%d must be >= 0", cfg.StartMs))
	}
	if cfg.LinkDelayMs < 0 {
		panic(fmt.Sprintf("scenario: LinkDelayMs=%d must be >= 0", cfg.LinkDelayMs))
	}
	validateScenarioPct("SourceToRelayLossPct", cfg.SourceToRelayLossPct)
	validateScenarioPct("RelayToReceiverLossPct", cfg.RelayToReceiverLossPct)
	validateScenarioPct("SourceToReceiverLossPct", cfg.SourceToReceiverLossPct)
	validateScenarioPct("SourceToRelayDuplicatePct", cfg.SourceToRelayDuplicatePct)
	validateScenarioPct("RelayToReceiverDuplicatePct", cfg.RelayToReceiverDuplicatePct)
	validateScenarioPct("SourceToReceiverDuplicatePct", cfg.SourceToReceiverDuplicatePct)
	validateScenarioPct("SourceToRelayCorruptionPct", cfg.SourceToRelayCorruptionPct)
	validateScenarioPct("RelayToReceiverCorruptionPct", cfg.RelayToReceiverCorruptionPct)
	validateScenarioPct("SourceToReceiverCorruptionPct", cfg.SourceToReceiverCorruptionPct)
	if cfg.SourceToRelayJitterMs < 0 {
		panic(fmt.Sprintf("scenario: SourceToRelayJitterMs=%d must be >= 0", cfg.SourceToRelayJitterMs))
	}
	if cfg.RelayToReceiverJitterMs < 0 {
		panic(fmt.Sprintf("scenario: RelayToReceiverJitterMs=%d must be >= 0", cfg.RelayToReceiverJitterMs))
	}
	if cfg.SourceToReceiverJitterMs < 0 {
		panic(fmt.Sprintf("scenario: SourceToReceiverJitterMs=%d must be >= 0", cfg.SourceToReceiverJitterMs))
	}
	if cfg.SourceToRelayJitterMs >= math.MaxInt64 {
		panic(fmt.Sprintf("scenario: SourceToRelayJitterMs=%d too large (>= MaxInt64)", cfg.SourceToRelayJitterMs))
	}
	if cfg.RelayToReceiverJitterMs >= math.MaxInt64 {
		panic(fmt.Sprintf("scenario: RelayToReceiverJitterMs=%d too large (>= MaxInt64)", cfg.RelayToReceiverJitterMs))
	}
	if cfg.SourceToReceiverJitterMs >= math.MaxInt64 {
		panic(fmt.Sprintf("scenario: SourceToReceiverJitterMs=%d too large (>= MaxInt64)", cfg.SourceToReceiverJitterMs))
	}
	if cfg.ReceiverJitterBufferMs < 0 {
		panic(fmt.Sprintf("scenario: ReceiverJitterBufferMs=%d must be >= 0", cfg.ReceiverJitterBufferMs))
	}
	if cfg.ReceiverLateThresholdMs < 0 {
		panic(fmt.Sprintf("scenario: ReceiverLateThresholdMs=%d must be >= 0", cfg.ReceiverLateThresholdMs))
	}
	if cfg.RelaySeenCacheMaxEntries < 0 {
		panic(fmt.Sprintf("scenario: RelaySeenCacheMaxEntries=%d must be >= 0", cfg.RelaySeenCacheMaxEntries))
	}
	if cfg.RelayForwardQueueMaxDepth < 0 {
		panic(fmt.Sprintf("scenario: RelayForwardQueueMaxDepth=%d must be >= 0", cfg.RelayForwardQueueMaxDepth))
	}
	validateScenarioTimeHorizon(cfg)
	validateRelayedFrameTiming(cfg)
	if cfg.TTL < 1 {
		panic(fmt.Sprintf("scenario: TTL=%d must be >= 1 for one-hop relay (§10)", cfg.TTL))
	}
	if cfg.NetID == 0 {
		panic(fmt.Sprintf("scenario: NetID=%d must be non-zero", cfg.NetID))
	}
	if cfg.GroupID == 0 {
		panic(fmt.Sprintf("scenario: GroupID=%d must be non-zero (§18: group_id 0 is reserved)", cfg.GroupID))
	}
	if cfg.VoiceGroupIDOverride != 0 && cfg.VoiceGroupIDOverride == cfg.GroupID {
		panic(fmt.Sprintf("scenario: VoiceGroupIDOverride=%d must differ from GroupID when explicitly set", cfg.VoiceGroupIDOverride))
	}
	if cfg.SourceID == 0 || cfg.RelayID == 0 || cfg.ReceiverID == 0 {
		panic(fmt.Sprintf("scenario: station IDs must be non-zero (§4.3); got source=%d relay=%d receiver=%d",
			cfg.SourceID, cfg.RelayID, cfg.ReceiverID))
	}
	if cfg.SourceID == cfg.RelayID || cfg.RelayID == cfg.ReceiverID || cfg.SourceID == cfg.ReceiverID {
		panic(fmt.Sprintf("scenario: station IDs must be distinct; got source=%d relay=%d receiver=%d",
			cfg.SourceID, cfg.RelayID, cfg.ReceiverID))
	}
	validateSlotConfig(cfg)
}

func validateRelayedFrameTiming(cfg ScenarioConfig) {
	if cfg.SlotModeEnabled && cfg.AllowSlotTimingStress {
		return
	}
	// Timing invariant: the last relayed voice frame must arrive at B
	// before the logical call_end time. The last voice frame has
	// created_at_ms = StartMs + (FrameCount-1)*FramePeriodMs. It goes
	// A->R (delay LinkDelayMs) then R->B (delay LinkDelayMs), so total
	// transit = 2*LinkDelayMs. The call_end has created_at_ms = StartMs
	// + FrameCount*FramePeriodMs. For the last frame to arrive before
	// call_end: (FrameCount-1)*FramePeriodMs + 2*LinkDelayMs <=
	// FrameCount*FramePeriodMs, i.e., 2*LinkDelayMs <= FramePeriodMs.
	//
	// Overflow-safe form: LinkDelayMs > FramePeriodMs/2 avoids int64
	// overflow. Link jitter is intentionally excluded: E-005 sweeps
	// jitter stress and records late/missing behavior rather than
	// rejecting those rows as invalid configs.
	if cfg.LinkDelayMs > cfg.FramePeriodMs/2 {
		panic(fmt.Sprintf("scenario: timing invariant violated: 2*LinkDelayMs > FramePeriodMs (LinkDelayMs=%d, FramePeriodMs=%d; relayed frames could arrive after call_end)",
			cfg.LinkDelayMs, cfg.FramePeriodMs))
	}
}

func receiverPolicyForConfig(cfg ScenarioConfig) node.ReceiverPolicy {
	policy := node.DefaultReceiverPolicy()
	policy.JitterBufferMs = receiverJitterBufferMsForConfig(cfg)
	policy.LateThresholdMs = receiverLateThresholdMsForConfig(cfg)
	return policy
}

func validateScenarioPct(name string, v float64) {
	if math.IsNaN(v) {
		panic(fmt.Sprintf("scenario: %s is NaN", name))
	}
	if math.IsInf(v, 0) {
		panic(fmt.Sprintf("scenario: %s is Inf", name))
	}
	if v < 0 || v > 100 {
		panic(fmt.Sprintf("scenario: %s=%g must be in [0, 100]", name, v))
	}
}
