package scenario

import (
	"fmt"
	"math"

	"github.com/rmsff/ad-hoc-radio/internal/node"
)

// DefaultScenarioSeed is the canonical seed for Scenario A per
// algorithm-and-experiments.md §5.
const DefaultScenarioSeed uint64 = 42

// DefaultFrameCount is the canonical frame count for Scenario A per
// AC-1 ("frames 1..100").
const DefaultFrameCount int = 100

// DefaultFramePeriodMs is the MVP source frame period per
// algorithm-and-experiments.md §4.3.
const DefaultFramePeriodMs int64 = 80

// DefaultLinkDelayMs is the canonical per-link delay for Scenario A.
const DefaultLinkDelayMs int64 = 10

// DefaultRelayForwardQueueMaxDepth is the MVP bounded relay queue
// policy from frame-contract.md §14.
const DefaultRelayForwardQueueMaxDepth = 32

// DefaultStartMs is the canonical call start time.
const DefaultStartMs int64 = 1_000

// Canonical station IDs per algorithm-and-experiments.md §5.
const (
	DefaultSourceID   uint16 = 1
	DefaultRelayID    uint16 = 2
	DefaultReceiverID uint16 = 3
)

// Canonical call parameters per algorithm-and-experiments.md §5.
const (
	DefaultNetID   uint16 = 1
	DefaultGroupID uint16 = 100
	DefaultCallID  uint32 = 9001
	DefaultTTL     uint8  = 1
)

// ScenarioID is the canonical scenario identifier, registered in
// docs/traceability.md.
const ScenarioID = "one_hop_nominal"

// ScenarioConfig configures the A → R → B scenario.
type ScenarioConfig struct {
	ScenarioID                    string
	Seed                          uint64
	FrameCount                    int
	FramePeriodMs                 int64
	StartMs                       int64
	LinkDelayMs                   int64
	SourceToRelayLossPct          float64
	RelayToReceiverLossPct        float64
	SourceToReceiverEnabled       bool
	SourceToReceiverLossPct       float64
	SourceToRelayDuplicatePct     float64
	RelayToReceiverDuplicatePct   float64
	SourceToReceiverDuplicatePct  float64
	SourceToRelayCorruptionPct    float64
	RelayToReceiverCorruptionPct  float64
	SourceToReceiverCorruptionPct float64
	SourceToRelayJitterMs         int64
	RelayToReceiverJitterMs       int64
	SourceToReceiverJitterMs      int64
	ReceiverJitterBufferMs        int64
	ReceiverLateThresholdMs       int64
	RelaySeenCacheMaxEntries      int
	RelayForwardQueueMaxDepth     int
	SourceID                      uint16
	RelayID                       uint16
	ReceiverID                    uint16
	NetID                         uint16
	GroupID                       uint16
	VoiceGroupIDOverride          uint16
	CallID                        uint32
	TTL                           uint8
	SlotModeEnabled               bool
	SlotDurationMs                int64
	AllowSlotTimingStress         bool
}

// DefaultScenarioConfig returns the canonical Scenario A configuration.
func DefaultScenarioConfig() ScenarioConfig {
	return ScenarioConfig{
		Seed:          DefaultScenarioSeed,
		FrameCount:    DefaultFrameCount,
		FramePeriodMs: DefaultFramePeriodMs,
		StartMs:       DefaultStartMs,
		LinkDelayMs:   DefaultLinkDelayMs,
		SourceID:      DefaultSourceID,
		RelayID:       DefaultRelayID,
		ReceiverID:    DefaultReceiverID,
		NetID:         DefaultNetID,
		GroupID:       DefaultGroupID,
		CallID:        DefaultCallID,
		TTL:           DefaultTTL,
	}
}

func validateScenarioTimeHorizon(cfg ScenarioConfig) {
	frames := int64(cfg.FrameCount)
	if frames > (math.MaxInt64-cfg.StartMs)/cfg.FramePeriodMs {
		panic(fmt.Sprintf("scenario: time horizon overflows int64: StartMs=%d FrameCount=%d FramePeriodMs=%d",
			cfg.StartMs, cfg.FrameCount, cfg.FramePeriodMs))
	}
	// Account for delivery delays and configured jitter. This is an
	// overflow guard only: E-005 intentionally sweeps jitter values
	// that may stress late/missing behavior, so this must not reject a
	// row merely because max jitter could exceed the logical
	// frame-period timing budget.
	endMs := cfg.StartMs + frames*cfg.FramePeriodMs
	maxDeliveryDelayMs := maxScenarioDeliveryDelayMs(cfg)
	if maxDeliveryDelayMs > math.MaxInt64-endMs {
		panic(fmt.Sprintf("scenario: delivery time horizon overflows int64: endMs=%d MaxDeliveryDelayMs=%d",
			endMs, maxDeliveryDelayMs))
	}
	jitterBufferMs := receiverJitterBufferMsForConfig(cfg)
	if jitterBufferMs > math.MaxInt64-endMs {
		panic(fmt.Sprintf("scenario: jitter evaluation horizon overflows int64: endMs=%d JitterBufferMs=%d",
			endMs, jitterBufferMs))
	}
	lateThresholdMs := receiverLateThresholdMsForConfig(cfg)
	if lateThresholdMs > math.MaxInt64-endMs {
		panic(fmt.Sprintf("scenario: late-threshold evaluation horizon overflows int64: endMs=%d LateThresholdMs=%d",
			endMs, lateThresholdMs))
	}
}

func receiverJitterBufferMsForConfig(cfg ScenarioConfig) int64 {
	if cfg.ReceiverJitterBufferMs == 0 {
		return node.DefaultReceiverJitterBufferMs
	}
	return cfg.ReceiverJitterBufferMs
}

func receiverLateThresholdMsForConfig(cfg ScenarioConfig) int64 {
	if cfg.ReceiverLateThresholdMs == 0 {
		return node.DefaultReceiverLateThresholdMs
	}
	return cfg.ReceiverLateThresholdMs
}

func voiceGroupIDForConfig(cfg ScenarioConfig) uint16 {
	if cfg.VoiceGroupIDOverride == 0 {
		return cfg.GroupID
	}
	return cfg.VoiceGroupIDOverride
}

func relayLifecyclePolicyForConfig(cfg ScenarioConfig) node.LifecyclePolicy {
	policy := node.DefaultLifecyclePolicy()
	if cfg.RelaySeenCacheMaxEntries != 0 {
		policy.RelaySeenCacheMaxEntries = cfg.RelaySeenCacheMaxEntries
	}
	return policy
}

func relayForwardQueueMaxDepthForConfig(cfg ScenarioConfig) int {
	if cfg.RelayForwardQueueMaxDepth == 0 {
		return DefaultRelayForwardQueueMaxDepth
	}
	return cfg.RelayForwardQueueMaxDepth
}

func maxScenarioDeliveryDelayMs(cfg ScenarioConfig) int64 {
	sourceDelay := addScenarioMs(cfg.LinkDelayMs, cfg.SourceToRelayJitterMs, "source-to-relay delivery delay")
	relayDelay := addScenarioMs(cfg.LinkDelayMs, cfg.RelayToReceiverJitterMs, "relay-to-receiver delivery delay")
	relayedDelay := addScenarioMs(sourceDelay, relayDelay, "relayed delivery delay")
	if !cfg.SourceToReceiverEnabled {
		return relayedDelay
	}
	directDelay := addScenarioMs(cfg.LinkDelayMs, cfg.SourceToReceiverJitterMs, "source-to-receiver delivery delay")
	if directDelay > relayedDelay {
		return directDelay
	}
	return relayedDelay
}

func addScenarioMs(a, b int64, label string) int64 {
	if a < 0 || b < 0 {
		panic(fmt.Sprintf("scenario: %s has negative operand (%d, %d)", label, a, b))
	}
	if b > math.MaxInt64-a {
		panic(fmt.Sprintf("scenario: %s overflows int64 (%d + %d)", label, a, b))
	}
	return a + b
}
