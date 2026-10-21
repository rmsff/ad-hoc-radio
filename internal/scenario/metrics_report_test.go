package scenario

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/node"
)

// TestIT007_MetricsReportValidatesSchema proves Story 3.3 AC-1 and
// AC-2: completed scenario metrics export as JSON that validates
// against docs/protocol/metrics-schema.json and includes latency
// min/avg/p50/p95/max.
//
// Traceability: IT-007 (FR-5, FR-7, FR-11, SM-1, SM-3, SM-4).
func TestIT007_MetricsReportValidatesSchema(t *testing.T) {
	metrics := runScenario(t)
	raw, err := metrics.MarshalReportJSON()
	if err != nil {
		t.Fatalf("MarshalReportJSON failed: %v", err)
	}
	validateMetricsJSONAgainstSchema(t, raw)

	var report MetricsReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("unmarshal report failed: %v", err)
	}

	if report.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", report.SchemaVersion)
	}
	if report.ScenarioID != ScenarioID {
		t.Errorf("scenario_id = %q, want %q", report.ScenarioID, ScenarioID)
	}
	if report.SourceFramesCreated != DefaultFrameCount {
		t.Errorf("source_frames_created = %d, want %d", report.SourceFramesCreated, DefaultFrameCount)
	}
	if report.SourceFramesSent != DefaultFrameCount {
		t.Errorf("source_frames_sent = %d, want %d", report.SourceFramesSent, DefaultFrameCount)
	}
	if report.Relay.FramesReceived != DefaultFrameCount+2 {
		t.Errorf("relay.frames_received = %d, want %d (call_start + voice + call_end)", report.Relay.FramesReceived, DefaultFrameCount+2)
	}
	if report.Relay.FramesForwarded != DefaultFrameCount {
		t.Errorf("relay.frames_forwarded = %d, want %d", report.Relay.FramesForwarded, DefaultFrameCount)
	}
	if report.Relay.SeenCacheEntries != DefaultFrameCount {
		t.Errorf("relay.seen_cache_entries = %d, want %d", report.Relay.SeenCacheEntries, DefaultFrameCount)
	}
	if report.Receiver.FramesAccepted != DefaultFrameCount {
		t.Errorf("receiver.frames_accepted = %d, want %d", report.Receiver.FramesAccepted, DefaultFrameCount)
	}
	if len(metrics.PlayoutLatencySamplesMs) != DefaultFrameCount {
		t.Errorf("playout latency samples = %d, want %d", len(metrics.PlayoutLatencySamplesMs), DefaultFrameCount)
	}
	if report.LatencyMs.Min != nodeDefaultJitterBufferMs() ||
		report.LatencyMs.Avg != nodeDefaultJitterBufferMs() ||
		report.LatencyMs.P50 != nodeDefaultJitterBufferMs() ||
		report.LatencyMs.P95 != nodeDefaultJitterBufferMs() ||
		report.LatencyMs.Max != nodeDefaultJitterBufferMs() {
		t.Errorf("latency summary = %+v, want all primary fields %.0f", report.LatencyMs, nodeDefaultJitterBufferMs())
	}
	if report.LatencyMs.P95 >= 250 {
		t.Errorf("SM-3: latency_ms.p95 = %v, want < 250", report.LatencyMs.P95)
	}
	if report.Outcome != "pass" {
		t.Errorf("outcome = %q, want pass", report.Outcome)
	}
	assertLinkReport(t, report, "1", "2", DefaultFrameCount+2)
	assertLinkReport(t, report, "2", "3", DefaultFrameCount)
	assertLinkReport(t, report, "1", "3", 0)
	assertLinkAvgDelay(t, report, "1", "2", DefaultLinkDelayMs)
	assertLinkAvgDelay(t, report, "2", "3", DefaultLinkDelayMs)
}

// TestIT007_TwoSlotMetricsReportValidatesSchema proves the same
// schema-shaped metrics export works for the Story 3.1 two-slot SFR
// scenario and carries slot-specific counters.
func TestIT007_TwoSlotMetricsReportValidatesSchema(t *testing.T) {
	metrics := runTwoSlotScenario(t)
	raw, err := metrics.MarshalReportJSON()
	if err != nil {
		t.Fatalf("MarshalReportJSON failed: %v", err)
	}
	validateMetricsJSONAgainstSchema(t, raw)

	report := metrics.Report()
	if report.ScenarioID != TwoSlotScenarioID {
		t.Errorf("scenario_id = %q, want %q", report.ScenarioID, TwoSlotScenarioID)
	}
	if report.OutOfSlotTxCount != 0 {
		t.Errorf("out_of_slot_tx_count = %d, want 0", report.OutOfSlotTxCount)
	}
	if report.Relay.ForwardQueueMaxDepth != 1 {
		t.Errorf("relay.forward_queue_max_depth = %d, want 1", report.Relay.ForwardQueueMaxDepth)
	}
	if report.LatencyMs.P95 != nodeDefaultJitterBufferMs() {
		t.Errorf("latency_ms.p95 = %v, want %.0f", report.LatencyMs.P95, nodeDefaultJitterBufferMs())
	}
}

// TestIT007_FixedSeedReportHashDeterministic proves Story 3.3 AC-3:
// repeated fixed-seed runs export identical determinism.event_hash.
func TestIT007_FixedSeedReportHashDeterministic(t *testing.T) {
	m1 := runScenario(t)
	m2 := runScenario(t)

	r1 := m1.Report()
	r2 := m2.Report()
	if r1.Determinism.EventHash != r2.Determinism.EventHash {
		t.Errorf("determinism.event_hash mismatch:\n  run1=%s\n  run2=%s",
			r1.Determinism.EventHash, r2.Determinism.EventHash)
	}
	if r1.Determinism.EventCount != r2.Determinism.EventCount {
		t.Errorf("determinism.event_count mismatch: %d vs %d",
			r1.Determinism.EventCount, r2.Determinism.EventCount)
	}

	raw1, err := m1.MarshalReportJSON()
	if err != nil {
		t.Fatalf("run1 MarshalReportJSON failed: %v", err)
	}
	raw2, err := m2.MarshalReportJSON()
	if err != nil {
		t.Fatalf("run2 MarshalReportJSON failed: %v", err)
	}
	if !bytes.Equal(raw1, raw2) {
		t.Errorf("fixed-seed metrics JSON differs:\nrun1=%s\nrun2=%s", raw1, raw2)
	}
}

func TestMetricsReport_OutcomeFailsOnOverAccept(t *testing.T) {
	metrics := runScenario(t)
	metrics.ReceiverCounters.FramesAccepted = metrics.SourceFramesCreated + 1
	metrics.ReceiverCounters.PlayoutFramesEmitted = metrics.SourceFramesCreated + 1

	report := metrics.Report()
	if report.Outcome != "fail" {
		t.Errorf("outcome = %q, want fail when receiver over-accepts source frames", report.Outcome)
	}
}

func TestMetricsReport_ZeroValueIsSchemaValidInconclusive(t *testing.T) {
	raw, err := (ScenarioMetrics{}).MarshalReportJSON()
	if err != nil {
		t.Fatalf("MarshalReportJSON failed: %v", err)
	}
	validateMetricsJSONAgainstSchema(t, raw)

	report := (ScenarioMetrics{}).Report()
	if report.ScenarioID != unknownScenarioID {
		t.Errorf("scenario_id = %q, want %q", report.ScenarioID, unknownScenarioID)
	}
	if report.Determinism.EventHash != emptyEventHash {
		t.Errorf("event_hash = %q, want %q", report.Determinism.EventHash, emptyEventHash)
	}
	if report.Outcome != "inconclusive" {
		t.Errorf("outcome = %q, want inconclusive", report.Outcome)
	}
}

func TestMetricsReport_StackVersionIsSourceControlled(t *testing.T) {
	report := runScenario(t).Report()
	if report.Stack.Version != metricsStackVersion {
		t.Errorf("stack.version = %q, want %q", report.Stack.Version, metricsStackVersion)
	}
}

func TestMetricsReport_SourceFramesSentExplicitZero(t *testing.T) {
	metrics := runScenario(t)
	metrics.SourceFramesSent = 0

	report := metrics.Report()
	if report.SourceFramesSent != 0 {
		t.Errorf("source_frames_sent = %d, want explicit zero", report.SourceFramesSent)
	}
}

func TestMetricsReport_DoesNotFabricateLatencySamples(t *testing.T) {
	metrics := runScenario(t)
	metrics.PlayoutLatencySamplesMs = nil
	metrics.SourceToRelaySamplesMs = nil
	metrics.RelayToReceiverSamplesMs = nil
	metrics.PlayoutWaitSamplesMs = nil

	report := metrics.Report()
	if report.LatencyMs.Min != 0 ||
		report.LatencyMs.Avg != 0 ||
		report.LatencyMs.P50 != 0 ||
		report.LatencyMs.P95 != 0 ||
		report.LatencyMs.Max != 0 {
		t.Errorf("latency summary = %+v, want zero without measured samples", report.LatencyMs)
	}
}

func TestMetricsReport_AllZeroLatencySamplesAreAccepted(t *testing.T) {
	metrics := runScenario(t)
	metrics.PlayoutLatencySamplesMs = []int64{0, 0, 0}
	metrics.SourceToRelaySamplesMs = []int64{0, 0, 0}
	metrics.RelayToReceiverSamplesMs = []int64{0, 0, 0}
	metrics.PlayoutWaitSamplesMs = []int64{0, 0, 0}

	report := metrics.Report()
	if report.LatencyMs.Min != 0 ||
		report.LatencyMs.Avg != 0 ||
		report.LatencyMs.P50 != 0 ||
		report.LatencyMs.P95 != 0 ||
		report.LatencyMs.Max != 0 {
		t.Errorf("latency summary = %+v, want zero for all-zero non-nil samples", report.LatencyMs)
	}
}

func TestMetricsReport_LatencySamplesClampedToSchemaSafeRange(t *testing.T) {
	metrics := runScenario(t)
	metrics.PlayoutLatencySamplesMs = []int64{-10, maxExactFloat64IntMs + 1}
	metrics.SourceToRelaySamplesMs = []int64{-1, 1}
	metrics.RelayToReceiverSamplesMs = []int64{1, maxExactFloat64IntMs + 1}
	metrics.PlayoutWaitSamplesMs = []int64{-5, 5}

	report := metrics.Report()
	if report.LatencyMs.Min != 0 {
		t.Errorf("latency_ms.min = %v, want 0 after negative clamp", report.LatencyMs.Min)
	}
	if report.LatencyMs.Max != float64(maxExactFloat64IntMs) {
		t.Errorf("latency_ms.max = %v, want %d after exact-float cap", report.LatencyMs.Max, maxExactFloat64IntMs)
	}
	if report.LatencyMs.SourceToRelayAvg < 0 ||
		report.LatencyMs.RelayToReceiverAvg < 0 ||
		report.LatencyMs.PlayoutAvg < 0 {
		t.Errorf("latency component averages must be non-negative: %+v", report.LatencyMs)
	}
}

func TestMetricsReport_LatencyUsesReceiverArrivalWhenLaterThanJitterTick(t *testing.T) {
	cfg := DefaultScenarioConfig()
	cfg.FrameCount = 3
	cfg.FramePeriodMs = 250
	cfg.LinkDelayMs = 125

	runner := NewScenarioRunner(cfg)
	metrics, err := runner.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	report := metrics.Report()
	if report.LatencyMs.P95 != 250 {
		t.Errorf("latency_ms.p95 = %v, want 250 (receiver arrival later than jitter tick)", report.LatencyMs.P95)
	}
	if report.LatencyMs.PlayoutAvg != 0 {
		t.Errorf("latency_ms.playout_avg = %v, want 0 when playout waits for receiver arrival", report.LatencyMs.PlayoutAvg)
	}
}

func TestMetricsReport_LatencyRequiresCompleteComponentSamples(t *testing.T) {
	metrics := runScenario(t)
	metrics.SourceToRelaySamplesMs = metrics.SourceToRelaySamplesMs[:len(metrics.SourceToRelaySamplesMs)-1]

	report := metrics.Report()
	if report.LatencyMs.Min != 0 ||
		report.LatencyMs.Avg != 0 ||
		report.LatencyMs.P50 != 0 ||
		report.LatencyMs.P95 != 0 ||
		report.LatencyMs.Max != 0 ||
		report.LatencyMs.SourceToRelayAvg != 0 ||
		report.LatencyMs.RelayToReceiverAvg != 0 ||
		report.LatencyMs.PlayoutAvg != 0 {
		t.Errorf("latency summary = %+v, want zero when component sample lengths mismatch", report.LatencyMs)
	}
}

func TestMetricsReportSchemaValidatorRejectsTrailingTokens(t *testing.T) {
	metrics := runScenario(t)
	raw, err := metrics.MarshalReportJSON()
	if err != nil {
		t.Fatalf("MarshalReportJSON failed: %v", err)
	}
	withTrailing := append(append([]byte(nil), raw...), []byte("\n{}")...)
	if err := validateMetricsJSONValue(withTrailing, loadMetricsSchema(t)); err == nil {
		t.Fatal("schema validator accepted metrics JSON with trailing token")
	}
}

func TestMetricsReportSchemaValidatorAcceptsHighUint64Seed(t *testing.T) {
	metrics := runScenario(t)
	report := metrics.Report()
	report.Seed = ^uint64(0)
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report failed: %v", err)
	}
	if err := validateMetricsJSONValue(raw, loadMetricsSchema(t)); err != nil {
		t.Fatalf("schema validator rejected high uint64 seed: %v", err)
	}
}

func assertLinkReport(t *testing.T, report MetricsReport, from, to string, delivered int) {
	t.Helper()
	for _, link := range report.Links {
		if link.From == from && link.To == to {
			if link.Delivered != delivered {
				t.Errorf("link %s->%s delivered = %d, want %d", from, to, link.Delivered, delivered)
			}
			return
		}
	}
	t.Fatalf("link %s->%s not found in report: %+v", from, to, report.Links)
}

func assertLinkAvgDelay(t *testing.T, report MetricsReport, from, to string, want int64) {
	t.Helper()
	for _, link := range report.Links {
		if link.From == from && link.To == to {
			if link.AvgDelayMs == nil {
				t.Fatalf("link %s->%s avg_delay_ms missing", from, to)
			}
			if *link.AvgDelayMs != float64(want) {
				t.Errorf("link %s->%s avg_delay_ms = %v, want %d", from, to, *link.AvgDelayMs, want)
			}
			return
		}
	}
	t.Fatalf("link %s->%s not found in report: %+v", from, to, report.Links)
}

func nodeDefaultJitterBufferMs() float64 {
	return float64(node.DefaultReceiverJitterBufferMs)
}
