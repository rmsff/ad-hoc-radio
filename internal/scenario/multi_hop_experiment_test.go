package scenario

import (
	"reflect"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/protocol"
)

func TestIT015_LimitedMultiHopExperimentSchedulesTwoRelayChain(t *testing.T) {
	report, err := RunLimitedMultiHopExperiment(DefaultLimitedMultiHopExperimentConfig())
	if err != nil {
		t.Fatalf("RunLimitedMultiHopExperiment failed: %v", err)
	}

	if report.ScenarioID != LimitedMultiHopScenarioID {
		t.Fatalf("ScenarioID = %q, want %q", report.ScenarioID, LimitedMultiHopScenarioID)
	}
	if report.HopCount != 2 {
		t.Fatalf("HopCount = %d, want 2 relay hops", report.HopCount)
	}
	if report.SlotsPerSuperframe != 3 {
		t.Fatalf("SlotsPerSuperframe = %d, want 3", report.SlotsPerSuperframe)
	}
	if report.SuperframeDurationMs != 120 {
		t.Fatalf("SuperframeDurationMs = %d, want 120", report.SuperframeDurationMs)
	}
	if len(report.ForwardingTrace) != report.FrameCount*report.SlotsPerSuperframe {
		t.Fatalf("ForwardingTrace rows = %d, want %d", len(report.ForwardingTrace), report.FrameCount*report.SlotsPerSuperframe)
	}

	wantFirst := []LimitedMultiHopForward{
		{Seq: 0, HopIndex: 0, FromID: 1, ToID: 2, SlotIndex: 0, SlotName: "source_slot", TxAtMs: 1000, RxAtMs: 1057, AirtimeMs: 47, FitsSlot: false},
		{Seq: 0, HopIndex: 1, FromID: 2, ToID: 3, SlotIndex: 1, SlotName: "relay1_slot", TxAtMs: 1040, RxAtMs: 1097, AirtimeMs: 47, FitsSlot: false},
		{Seq: 0, HopIndex: 2, FromID: 3, ToID: 4, SlotIndex: 2, SlotName: "relay2_slot", TxAtMs: 1080, RxAtMs: 1137, AirtimeMs: 47, FitsSlot: false},
	}
	if !reflect.DeepEqual(report.ForwardingTrace[:3], wantFirst) {
		t.Fatalf("first frame trace = %+v, want %+v", report.ForwardingTrace[:3], wantFirst)
	}
	if report.EndToEndLatencyMs != 137 {
		t.Fatalf("EndToEndLatencyMs = %d, want 137", report.EndToEndLatencyMs)
	}
	if report.LatencyP95Ms != 137 {
		t.Fatalf("LatencyP95Ms = %d, want 137", report.LatencyP95Ms)
	}
	if report.SlotMissCount != report.FrameCount*report.SlotsPerSuperframe {
		t.Fatalf("SlotMissCount = %d, want one slot miss per transmission", report.SlotMissCount)
	}
	if report.RelayReadinessMissCount != report.FrameCount*2 {
		t.Fatalf("RelayReadinessMissCount = %d, want one miss per relay receive", report.RelayReadinessMissCount)
	}
	if report.TimingBudgetPass {
		t.Fatal("TimingBudgetPass = true, want false because canonical 56-byte frame misses 40 ms slot at 9600 bit/s")
	}
	if report.Decision != LimitedMultiHopDecisionNoGo {
		t.Fatalf("Decision = %q, want %q", report.Decision, LimitedMultiHopDecisionNoGo)
	}
	wantReasons := []string{LimitedMultiHopFailSlotMiss, LimitedMultiHopFailRelayReadinessMiss, LimitedMultiHopFailFramePeriodExceeded}
	if !reflect.DeepEqual(report.FailReasons, wantReasons) {
		t.Fatalf("FailReasons = %v, want %v", report.FailReasons, wantReasons)
	}
}

func TestIT015_LimitedMultiHopExperimentPassesWithCompressedCandidate(t *testing.T) {
	cfg := DefaultLimitedMultiHopExperimentConfig()
	cfg.FrameBytes = 22
	cfg.SlotDurationMs = 20
	cfg.LinkDelayMs = 1

	report, err := RunLimitedMultiHopExperiment(cfg)
	if err != nil {
		t.Fatalf("RunLimitedMultiHopExperiment failed: %v", err)
	}

	if report.Measurement.AirtimeMs != 19 {
		t.Fatalf("AirtimeMs = %d, want 19", report.Measurement.AirtimeMs)
	}
	if report.SourceFramePeriodMs != 60 {
		t.Fatalf("SourceFramePeriodMs = %d, want 60", report.SourceFramePeriodMs)
	}
	if report.EndToEndLatencyMs != 60 {
		t.Fatalf("EndToEndLatencyMs = %d, want 60", report.EndToEndLatencyMs)
	}
	if !report.Measurement.FitsSlot {
		t.Fatal("FitsSlot = false, want true")
	}
	if report.SlotMissCount != 0 {
		t.Fatalf("SlotMissCount = %d, want 0", report.SlotMissCount)
	}
	if report.RelayReadinessMissCount != 0 {
		t.Fatalf("RelayReadinessMissCount = %d, want 0", report.RelayReadinessMissCount)
	}
	if !report.TimingBudgetPass {
		t.Fatal("TimingBudgetPass = false, want true for 22-byte candidate under default latency budget")
	}
	if report.Decision != LimitedMultiHopDecisionGo {
		t.Fatalf("Decision = %q, want %q", report.Decision, LimitedMultiHopDecisionGo)
	}
	if len(report.FailReasons) != 0 {
		t.Fatalf("FailReasons = %v, want empty", report.FailReasons)
	}
}

func TestIT015_LimitedMultiHopExperimentMixedCanonicalSourceCompressedRelay(t *testing.T) {
	compressedRelayBytes, err := protocol.CompressedRelayHopWireSize(8)
	if err != nil {
		t.Fatalf("CompressedRelayHopWireSize failed: %v", err)
	}

	cfg := DefaultLimitedMultiHopExperimentConfig()
	cfg.ChannelBitrateBps = 19200
	cfg.SlotDurationMs = 40
	cfg.LinkDelayMs = 1
	cfg.SourceFrameBytes = 56
	cfg.RelayFrameBytes = compressedRelayBytes

	report, err := RunLimitedMultiHopExperiment(cfg)
	if err != nil {
		t.Fatalf("RunLimitedMultiHopExperiment failed: %v", err)
	}

	if report.SourceFrameBytes != 56 {
		t.Fatalf("SourceFrameBytes = %d, want 56", report.SourceFrameBytes)
	}
	if report.RelayFrameBytes != 22 {
		t.Fatalf("RelayFrameBytes = %d, want 22", report.RelayFrameBytes)
	}
	if report.SourceMeasurement.AirtimeMs != 24 {
		t.Fatalf("SourceMeasurement.AirtimeMs = %d, want 24", report.SourceMeasurement.AirtimeMs)
	}
	if report.RelayMeasurement.AirtimeMs != 10 {
		t.Fatalf("RelayMeasurement.AirtimeMs = %d, want 10", report.RelayMeasurement.AirtimeMs)
	}

	wantFirst := []LimitedMultiHopForward{
		{Seq: 0, HopIndex: 0, FromID: 1, ToID: 2, SlotIndex: 0, SlotName: "source_slot", TxAtMs: 1000, RxAtMs: 1025, AirtimeMs: 24, FitsSlot: true},
		{Seq: 0, HopIndex: 1, FromID: 2, ToID: 3, SlotIndex: 1, SlotName: "relay1_slot", TxAtMs: 1040, RxAtMs: 1051, AirtimeMs: 10, FitsSlot: true},
		{Seq: 0, HopIndex: 2, FromID: 3, ToID: 4, SlotIndex: 2, SlotName: "relay2_slot", TxAtMs: 1080, RxAtMs: 1091, AirtimeMs: 10, FitsSlot: true},
	}
	if !reflect.DeepEqual(report.ForwardingTrace[:3], wantFirst) {
		t.Fatalf("first mixed frame trace = %+v, want %+v", report.ForwardingTrace[:3], wantFirst)
	}
	if report.SlotMissCount != 0 {
		t.Fatalf("SlotMissCount = %d, want 0", report.SlotMissCount)
	}
	if report.RelayReadinessMissCount != 0 {
		t.Fatalf("RelayReadinessMissCount = %d, want 0", report.RelayReadinessMissCount)
	}
	if report.EndToEndLatencyMs != 91 {
		t.Fatalf("EndToEndLatencyMs = %d, want 91", report.EndToEndLatencyMs)
	}
	if report.TimingBudgetPass {
		t.Fatal("TimingBudgetPass = true, want false under strict 80 ms source-frame cadence")
	}
	if !reflect.DeepEqual(report.FailReasons, []string{LimitedMultiHopFailFramePeriodExceeded}) {
		t.Fatalf("FailReasons = %v, want [%s]", report.FailReasons, LimitedMultiHopFailFramePeriodExceeded)
	}
}

func TestIT015_LimitedMultiHopExperimentFailsLatencyBudget(t *testing.T) {
	cfg := DefaultLimitedMultiHopExperimentConfig()
	cfg.FrameBytes = 22
	cfg.SlotDurationMs = 20
	cfg.LinkDelayMs = 1
	cfg.LatencyBudgetMs = 40

	report, err := RunLimitedMultiHopExperiment(cfg)
	if err != nil {
		t.Fatalf("RunLimitedMultiHopExperiment failed: %v", err)
	}

	if report.SlotMissCount != 0 {
		t.Fatalf("SlotMissCount = %d, want 0 so this test isolates latency budget", report.SlotMissCount)
	}
	if report.TimingBudgetPass {
		t.Fatal("TimingBudgetPass = true, want false")
	}
	if report.Decision != LimitedMultiHopDecisionNoGo {
		t.Fatalf("Decision = %q, want %q", report.Decision, LimitedMultiHopDecisionNoGo)
	}
	if !reflect.DeepEqual(report.FailReasons, []string{LimitedMultiHopFailLatencyBudgetExceeded}) {
		t.Fatalf("FailReasons = %v, want [%s]", report.FailReasons, LimitedMultiHopFailLatencyBudgetExceeded)
	}
}

func TestIT015_LimitedMultiHopExperimentDeterministicReplay(t *testing.T) {
	cfg := DefaultLimitedMultiHopExperimentConfig()
	cfg.FrameBytes = 22
	cfg.SlotDurationMs = 20
	cfg.LinkDelayMs = 1

	first, err := RunLimitedMultiHopExperiment(cfg)
	if err != nil {
		t.Fatalf("first RunLimitedMultiHopExperiment failed: %v", err)
	}
	second, err := RunLimitedMultiHopExperiment(cfg)
	if err != nil {
		t.Fatalf("second RunLimitedMultiHopExperiment failed: %v", err)
	}

	if first.EventHash == "" {
		t.Fatal("EventHash is empty")
	}
	if first.EventHash != second.EventHash {
		t.Fatalf("EventHash mismatch:\nfirst=%s\nsecond=%s", first.EventHash, second.EventHash)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("same config produced different reports:\nfirst=%+v\nsecond=%+v", first, second)
	}
}

func TestIT015_LimitedMultiHopExperimentRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  LimitedMultiHopExperimentConfig
	}{
		{name: "zero frame count", cfg: func() LimitedMultiHopExperimentConfig {
			cfg := DefaultLimitedMultiHopExperimentConfig()
			cfg.FrameCount = 0
			return cfg
		}()},
		{name: "zero frame bytes", cfg: func() LimitedMultiHopExperimentConfig {
			cfg := DefaultLimitedMultiHopExperimentConfig()
			cfg.FrameBytes = 0
			return cfg
		}()},
		{name: "frame bytes exceed cap", cfg: func() LimitedMultiHopExperimentConfig {
			cfg := DefaultLimitedMultiHopExperimentConfig()
			cfg.FrameBytes = maxLimitedMultiHopFrameBytes + 1
			return cfg
		}()},
		{name: "zero relay1 id", cfg: func() LimitedMultiHopExperimentConfig {
			cfg := DefaultLimitedMultiHopExperimentConfig()
			cfg.Relay1ID = 0
			return cfg
		}()},
		{name: "duplicate relay ids", cfg: func() LimitedMultiHopExperimentConfig {
			cfg := DefaultLimitedMultiHopExperimentConfig()
			cfg.Relay2ID = cfg.Relay1ID
			return cfg
		}()},
		{name: "negative start", cfg: func() LimitedMultiHopExperimentConfig {
			cfg := DefaultLimitedMultiHopExperimentConfig()
			cfg.StartMs = -1
			return cfg
		}()},
		{name: "link delay exceeds slot", cfg: func() LimitedMultiHopExperimentConfig {
			cfg := DefaultLimitedMultiHopExperimentConfig()
			cfg.LinkDelayMs = cfg.SlotDurationMs + 1
			return cfg
		}()},
		{name: "slot duration overflow", cfg: func() LimitedMultiHopExperimentConfig {
			cfg := DefaultLimitedMultiHopExperimentConfig()
			cfg.SlotDurationMs = 1 << 62
			return cfg
		}()},
		{name: "zero latency budget", cfg: func() LimitedMultiHopExperimentConfig {
			cfg := DefaultLimitedMultiHopExperimentConfig()
			cfg.LatencyBudgetMs = 0
			return cfg
		}()},
		{name: "zero frame period budget", cfg: func() LimitedMultiHopExperimentConfig {
			cfg := DefaultLimitedMultiHopExperimentConfig()
			cfg.FramePeriodBudgetMs = 0
			return cfg
		}()},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := RunLimitedMultiHopExperiment(tc.cfg); err == nil {
				t.Fatal("RunLimitedMultiHopExperiment accepted invalid config")
			}
		})
	}
}
