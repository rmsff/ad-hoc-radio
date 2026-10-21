package scenario

import (
	"reflect"
	"strings"
	"testing"
)

func TestIT019_TwoHopLiveScenarioDeliversThroughTwoRelays(t *testing.T) {
	report, err := RunTwoHopLiveScenario(DefaultTwoHopLiveScenarioConfig())
	if err != nil {
		t.Fatalf("RunTwoHopLiveScenario failed: %v", err)
	}

	if report.ScenarioID != TwoHopLiveScenarioID {
		t.Fatalf("ScenarioID = %q, want %q", report.ScenarioID, TwoHopLiveScenarioID)
	}
	if report.SourceMeasurement.AirtimeMs != 24 {
		t.Fatalf("SourceMeasurement.AirtimeMs = %d, want 24", report.SourceMeasurement.AirtimeMs)
	}
	if report.RelayMeasurement.AirtimeMs != 10 {
		t.Fatalf("RelayMeasurement.AirtimeMs = %d, want 10", report.RelayMeasurement.AirtimeMs)
	}
	if report.SourceFrameBytes != 56 {
		t.Fatalf("SourceFrameBytes = %d, want 56", report.SourceFrameBytes)
	}
	if report.RelayFrameBytes != 22 {
		t.Fatalf("RelayFrameBytes = %d, want 22", report.RelayFrameBytes)
	}
	if report.SourceFramesCreated != report.FrameCount {
		t.Fatalf("SourceFramesCreated = %d, want %d", report.SourceFramesCreated, report.FrameCount)
	}
	if report.Relay1FramesForwarded != report.FrameCount {
		t.Fatalf("Relay1FramesForwarded = %d, want %d", report.Relay1FramesForwarded, report.FrameCount)
	}
	if report.Relay2FramesForwarded != report.FrameCount {
		t.Fatalf("Relay2FramesForwarded = %d, want %d", report.Relay2FramesForwarded, report.FrameCount)
	}
	if report.ReceiverCounters.FramesAccepted != report.FrameCount {
		t.Fatalf("receiver frames accepted = %d, want %d", report.ReceiverCounters.FramesAccepted, report.FrameCount)
	}
	if report.ReceiverCounters.RelayedFramesAccepted != report.FrameCount {
		t.Fatalf("relayed frames accepted = %d, want %d", report.ReceiverCounters.RelayedFramesAccepted, report.FrameCount)
	}
	if report.ReceiverCounters.DirectFramesAccepted != 0 {
		t.Fatalf("direct frames accepted = %d, want 0", report.ReceiverCounters.DirectFramesAccepted)
	}
	if report.ReceiverCounters.PlayoutFramesEmitted != report.FrameCount {
		t.Fatalf("playout frames emitted = %d, want %d", report.ReceiverCounters.PlayoutFramesEmitted, report.FrameCount)
	}
	if report.ReceiverCounters.LateFrames != 0 || report.ReceiverCounters.MissingSequences != 0 {
		t.Fatalf("late/missing = %d/%d, want 0/0", report.ReceiverCounters.LateFrames, report.ReceiverCounters.MissingSequences)
	}
	if report.SlotMissCount != 0 {
		t.Fatalf("SlotMissCount = %d, want 0", report.SlotMissCount)
	}
	if report.RelayReadinessMissCount != 0 {
		t.Fatalf("RelayReadinessMissCount = %d, want 0", report.RelayReadinessMissCount)
	}
	if report.CycleOccupiedMs != 65 {
		t.Fatalf("CycleOccupiedMs = %d, want 65", report.CycleOccupiedMs)
	}
	if report.CycleSpareMs != 15 {
		t.Fatalf("CycleSpareMs = %d, want 15", report.CycleSpareMs)
	}
	if report.EndToEndLatencyMs != 61 {
		t.Fatalf("EndToEndLatencyMs = %d, want 61", report.EndToEndLatencyMs)
	}
	if report.LatencyP95Ms != 61 {
		t.Fatalf("LatencyP95Ms = %d, want 61", report.LatencyP95Ms)
	}
	if report.EventCount != report.FrameCount*3 {
		t.Fatalf("EventCount = %d, want %d", report.EventCount, report.FrameCount*3)
	}
	if report.EventHash == "" {
		t.Fatal("EventHash is empty")
	}
	if report.Decision != TwoHopLiveOutcomePass {
		t.Fatalf("Decision = %q, want %q; reasons=%v", report.Decision, TwoHopLiveOutcomePass, report.FailReasons)
	}
	if len(report.FailReasons) != 0 {
		t.Fatalf("FailReasons = %v, want empty", report.FailReasons)
	}

	t.Logf("two-hop full scenario: frames=%d event_count=%d latency=%dms spare=%dms hash=%s",
		report.FrameCount, report.EventCount, report.EndToEndLatencyMs, report.CycleSpareMs, report.EventHash)
}

func TestIT019_TwoHopLiveScenarioTraceUsesCanonicalThenCompressedBytes(t *testing.T) {
	report, err := RunTwoHopLiveScenario(DefaultTwoHopLiveScenarioConfig())
	if err != nil {
		t.Fatalf("RunTwoHopLiveScenario failed: %v", err)
	}

	got := firstSeqTrace(report.ForwardingTrace, 0)
	want := []TwoHopLiveForward{
		{
			Seq:            0,
			HopIndex:       0,
			FromID:         1,
			ToID:           2,
			TxAtMs:         1000,
			RxAtMs:         1025,
			WireBytes:      56,
			AirtimeMs:      24,
			CompressedWire: false,
			RelayReady:     true,
			ReceivedByNode: true,
		},
		{
			Seq:             0,
			HopIndex:        1,
			FromID:          2,
			ToID:            3,
			TxAtMs:          1030,
			RxAtMs:          1041,
			WireBytes:       22,
			AirtimeMs:       10,
			CompressedWire:  true,
			RelayReady:      true,
			ReceivedByNode:  true,
			ForwardedByNode: true,
		},
		{
			Seq:             0,
			HopIndex:        2,
			FromID:          3,
			ToID:            4,
			TxAtMs:          1050,
			RxAtMs:          1061,
			WireBytes:       22,
			AirtimeMs:       10,
			CompressedWire:  true,
			RelayReady:      true,
			ReceivedByNode:  true,
			ForwardedByNode: true,
			AcceptedByNode:  true,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("seq0 trace = %+v, want %+v", got, want)
	}
}

func TestIT019_TwoHopLiveScenarioDeterministicReplay(t *testing.T) {
	first, err := RunTwoHopLiveScenario(DefaultTwoHopLiveScenarioConfig())
	if err != nil {
		t.Fatalf("first RunTwoHopLiveScenario failed: %v", err)
	}
	second, err := RunTwoHopLiveScenario(DefaultTwoHopLiveScenarioConfig())
	if err != nil {
		t.Fatalf("second RunTwoHopLiveScenario failed: %v", err)
	}

	if first.EventHash != second.EventHash {
		t.Fatalf("EventHash mismatch:\nfirst=%s\nsecond=%s", first.EventHash, second.EventHash)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same config produced different reports")
	}
}

func TestIT019_TwoHopLiveScenarioRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  TwoHopLiveScenarioConfig
		want string
	}{
		{name: "ttl too small", cfg: func() TwoHopLiveScenarioConfig {
			cfg := DefaultTwoHopLiveScenarioConfig()
			cfg.TTL = 1
			return cfg
		}(), want: "ttl"},
		{name: "relay bytes mismatch", cfg: func() TwoHopLiveScenarioConfig {
			cfg := DefaultTwoHopLiveScenarioConfig()
			cfg.RelayFrameBytes = 23
			return cfg
		}(), want: "relay_frame_bytes"},
		{name: "duplicate station id", cfg: func() TwoHopLiveScenarioConfig {
			cfg := DefaultTwoHopLiveScenarioConfig()
			cfg.Relay2ID = cfg.Relay1ID
			return cfg
		}(), want: "duplicates"},
		{name: "negative guard", cfg: func() TwoHopLiveScenarioConfig {
			cfg := DefaultTwoHopLiveScenarioConfig()
			cfg.GuardTimeMs = -1
			return cfg
		}(), want: "guard_time_ms"},
		{name: "unsupported source wire mode", cfg: func() TwoHopLiveScenarioConfig {
			cfg := DefaultTwoHopLiveScenarioConfig()
			cfg.SourceWireMode = "unknown"
			return cfg
		}(), want: "source_wire_mode"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RunTwoHopLiveScenario(tc.cfg)
			if err == nil {
				t.Fatal("RunTwoHopLiveScenario accepted invalid config")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want context %q", err.Error(), tc.want)
			}
		})
	}
}

func TestIT021_TwoHopLiveShortSourceScenarioDeliversThroughTwoRelays(t *testing.T) {
	report, err := RunTwoHopLiveScenario(DefaultTwoHopLiveShortSourceScenarioConfig())
	if err != nil {
		t.Fatalf("RunTwoHopLiveScenario failed: %v", err)
	}

	if report.ScenarioID != TwoHopLiveShortSourceScenarioID {
		t.Fatalf("ScenarioID = %q, want %q", report.ScenarioID, TwoHopLiveShortSourceScenarioID)
	}
	if report.SourceWireMode != TwoHopLiveSourceWireShort {
		t.Fatalf("SourceWireMode = %q, want %q", report.SourceWireMode, TwoHopLiveSourceWireShort)
	}
	if report.SourceMeasurement.AirtimeMs != 10 {
		t.Fatalf("SourceMeasurement.AirtimeMs = %d, want 10", report.SourceMeasurement.AirtimeMs)
	}
	if report.RelayMeasurement.AirtimeMs != 10 {
		t.Fatalf("RelayMeasurement.AirtimeMs = %d, want 10", report.RelayMeasurement.AirtimeMs)
	}
	if report.SourceFrameBytes != 22 || report.RelayFrameBytes != 22 {
		t.Fatalf("wire bytes source/relay = %d/%d, want 22/22", report.SourceFrameBytes, report.RelayFrameBytes)
	}
	if report.SourceFramesCreated != report.FrameCount {
		t.Fatalf("SourceFramesCreated = %d, want %d", report.SourceFramesCreated, report.FrameCount)
	}
	if report.Relay1FramesForwarded != report.FrameCount {
		t.Fatalf("Relay1FramesForwarded = %d, want %d", report.Relay1FramesForwarded, report.FrameCount)
	}
	if report.Relay2FramesForwarded != report.FrameCount {
		t.Fatalf("Relay2FramesForwarded = %d, want %d", report.Relay2FramesForwarded, report.FrameCount)
	}
	if report.ReceiverCounters.FramesAccepted != report.FrameCount {
		t.Fatalf("receiver frames accepted = %d, want %d", report.ReceiverCounters.FramesAccepted, report.FrameCount)
	}
	if report.ReceiverCounters.PlayoutFramesEmitted != report.FrameCount {
		t.Fatalf("playout frames emitted = %d, want %d", report.ReceiverCounters.PlayoutFramesEmitted, report.FrameCount)
	}
	if report.ReceiverCounters.LateFrames != 0 || report.ReceiverCounters.MissingSequences != 0 {
		t.Fatalf("late/missing = %d/%d, want 0/0", report.ReceiverCounters.LateFrames, report.ReceiverCounters.MissingSequences)
	}
	if report.SlotMissCount != 0 {
		t.Fatalf("SlotMissCount = %d, want 0", report.SlotMissCount)
	}
	if report.RelayReadinessMissCount != 0 {
		t.Fatalf("RelayReadinessMissCount = %d, want 0", report.RelayReadinessMissCount)
	}
	if report.CycleOccupiedMs != 35 {
		t.Fatalf("CycleOccupiedMs = %d, want 35", report.CycleOccupiedMs)
	}
	if report.CycleSpareMs != 45 {
		t.Fatalf("CycleSpareMs = %d, want 45", report.CycleSpareMs)
	}
	if report.EndToEndLatencyMs != 35 {
		t.Fatalf("EndToEndLatencyMs = %d, want 35", report.EndToEndLatencyMs)
	}
	if report.LatencyP95Ms != 35 {
		t.Fatalf("LatencyP95Ms = %d, want 35", report.LatencyP95Ms)
	}
	if report.EventCount != report.FrameCount*3 {
		t.Fatalf("EventCount = %d, want %d", report.EventCount, report.FrameCount*3)
	}
	if report.EventHash == "" {
		t.Fatal("EventHash is empty")
	}
	if report.Decision != TwoHopLiveOutcomePass {
		t.Fatalf("Decision = %q, want %q; reasons=%v", report.Decision, TwoHopLiveOutcomePass, report.FailReasons)
	}
	if len(report.FailReasons) != 0 {
		t.Fatalf("FailReasons = %v, want empty", report.FailReasons)
	}

	t.Logf("two-hop short-source full scenario: frames=%d event_count=%d latency=%dms spare=%dms hash=%s",
		report.FrameCount, report.EventCount, report.EndToEndLatencyMs, report.CycleSpareMs, report.EventHash)
}

func TestIT021_TwoHopLiveShortSourceTraceUsesShortThenCompressedBytes(t *testing.T) {
	report, err := RunTwoHopLiveScenario(DefaultTwoHopLiveShortSourceScenarioConfig())
	if err != nil {
		t.Fatalf("RunTwoHopLiveScenario failed: %v", err)
	}

	got := firstSeqTrace(report.ForwardingTrace, 0)
	want := []TwoHopLiveForward{
		{
			Seq:            0,
			HopIndex:       0,
			FromID:         1,
			ToID:           2,
			TxAtMs:         1000,
			RxAtMs:         1011,
			WireBytes:      22,
			AirtimeMs:      10,
			ShortWire:      true,
			CompressedWire: false,
			RelayReady:     true,
			ReceivedByNode: true,
		},
		{
			Seq:             0,
			HopIndex:        1,
			FromID:          2,
			ToID:            3,
			TxAtMs:          1012,
			RxAtMs:          1023,
			WireBytes:       22,
			AirtimeMs:       10,
			CompressedWire:  true,
			RelayReady:      true,
			ReceivedByNode:  true,
			ForwardedByNode: true,
		},
		{
			Seq:             0,
			HopIndex:        2,
			FromID:          3,
			ToID:            4,
			TxAtMs:          1024,
			RxAtMs:          1035,
			WireBytes:       22,
			AirtimeMs:       10,
			CompressedWire:  true,
			RelayReady:      true,
			ReceivedByNode:  true,
			ForwardedByNode: true,
			AcceptedByNode:  true,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("seq0 trace = %+v, want %+v", got, want)
	}
}

func TestIT021_TwoHopLiveShortSourceScenarioDeterministicReplay(t *testing.T) {
	first, err := RunTwoHopLiveScenario(DefaultTwoHopLiveShortSourceScenarioConfig())
	if err != nil {
		t.Fatalf("first RunTwoHopLiveScenario failed: %v", err)
	}
	second, err := RunTwoHopLiveScenario(DefaultTwoHopLiveShortSourceScenarioConfig())
	if err != nil {
		t.Fatalf("second RunTwoHopLiveScenario failed: %v", err)
	}

	if first.EventHash != second.EventHash {
		t.Fatalf("EventHash mismatch:\nfirst=%s\nsecond=%s", first.EventHash, second.EventHash)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same config produced different reports")
	}
}

func TestIT021_TwoHopLiveMetricsDurationIsRelativeToStart(t *testing.T) {
	baseCfg := DefaultTwoHopLiveShortSourceScenarioConfig()
	shiftedCfg := baseCfg
	shiftedCfg.StartMs = baseCfg.StartMs + 5_000

	baseReport, err := RunTwoHopLiveScenario(baseCfg)
	if err != nil {
		t.Fatalf("base RunTwoHopLiveScenario failed: %v", err)
	}
	shiftedReport, err := RunTwoHopLiveScenario(shiftedCfg)
	if err != nil {
		t.Fatalf("shifted RunTwoHopLiveScenario failed: %v", err)
	}
	baseMetrics, err := TwoHopLiveScenarioMetrics(baseReport)
	if err != nil {
		t.Fatalf("base TwoHopLiveScenarioMetrics failed: %v", err)
	}
	shiftedMetrics, err := TwoHopLiveScenarioMetrics(shiftedReport)
	if err != nil {
		t.Fatalf("shifted TwoHopLiveScenarioMetrics failed: %v", err)
	}

	if baseMetrics.SimDurationMs != shiftedMetrics.SimDurationMs {
		t.Fatalf("SimDurationMs changed with start_ms: base=%d shifted=%d",
			baseMetrics.SimDurationMs, shiftedMetrics.SimDurationMs)
	}
	if shiftedMetrics.StartMs != shiftedCfg.StartMs {
		t.Fatalf("StartMs = %d, want %d", shiftedMetrics.StartMs, shiftedCfg.StartMs)
	}
}

func TestDecodeDeliveryRejectsAmbiguousWireMode(t *testing.T) {
	rt := &twoHopLiveRuntime{}
	_, err := rt.decodeDelivery(twoHopLivePendingDelivery{
		ShortWire:      true,
		CompressedWire: true,
	})
	if err == nil {
		t.Fatal("decodeDelivery accepted ambiguous short+compressed delivery")
	}
	if !strings.Contains(err.Error(), "both short and compressed") {
		t.Fatalf("error = %q, want ambiguous wire mode context", err.Error())
	}
}

func firstSeqTrace(trace []TwoHopLiveForward, seq uint32) []TwoHopLiveForward {
	out := make([]TwoHopLiveForward, 0, 3)
	for _, row := range trace {
		if row.Seq == seq {
			out = append(out, row)
		}
	}
	return out
}
