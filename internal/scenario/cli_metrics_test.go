package scenario

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/node"
)

// TestIT010_CLIOneHopMetricsValidatesSchema proves Story 3.4 AC-1
// through the actual CLI command while reusing the Story 3.3
// stdlib-only metrics schema validator.
func TestIT010_CLIOneHopMetricsValidatesSchema(t *testing.T) {
	metricsPath := filepath.Join(t.TempDir(), "out", "metrics.json")

	cmd := exec.Command("go", "run", "../../cmd/adhr-sim", "run", "../../configs/scenarios/one-hop.json", "--metrics", metricsPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI run failed: %v\n%s", err, output)
	}

	raw, err := os.ReadFile(metricsPath)
	if err != nil {
		t.Fatalf("read CLI metrics failed: %v", err)
	}
	validateMetricsJSONAgainstSchema(t, raw)

	var report MetricsReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("unmarshal CLI metrics failed: %v", err)
	}
	if report.ScenarioID != ScenarioID {
		t.Errorf("scenario_id = %q, want %q", report.ScenarioID, ScenarioID)
	}
	if report.Outcome != "pass" {
		t.Errorf("outcome = %q, want pass", report.Outcome)
	}
	if report.Receiver.RelayedFramesAccepted < 95 {
		t.Errorf("receiver.relayed_frames_accepted = %d, want >= 95", report.Receiver.RelayedFramesAccepted)
	}
	if report.Receiver.DirectFramesAccepted != 0 {
		t.Errorf("receiver.direct_frames_accepted = %d, want 0", report.Receiver.DirectFramesAccepted)
	}
}

// TestIT022_CLITwoHopShortSourceMetricsValidatesSchema proves MVP-7.8:
// the short-source two-hop full scenario is runnable through adhr-sim
// and emits the same schema-shaped metrics JSON as the one-hop CLI path.
func TestIT022_CLITwoHopShortSourceMetricsValidatesSchema(t *testing.T) {
	metricsPath := filepath.Join(t.TempDir(), "out", "two-hop.json")

	cmd := exec.Command("go", "run", "../../cmd/adhr-sim", "run", "../../configs/scenarios/two-hop-short-source.json", "--metrics", metricsPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI run failed: %v\n%s", err, output)
	}

	raw, err := os.ReadFile(metricsPath)
	if err != nil {
		t.Fatalf("read CLI metrics failed: %v", err)
	}
	validateMetricsJSONAgainstSchema(t, raw)

	var report MetricsReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("unmarshal CLI metrics failed: %v", err)
	}
	if report.ScenarioID != TwoHopLiveShortSourceScenarioID {
		t.Errorf("scenario_id = %q, want %q", report.ScenarioID, TwoHopLiveShortSourceScenarioID)
	}
	if report.Outcome != "pass" {
		t.Errorf("outcome = %q, want pass", report.Outcome)
	}
	if report.SourceFramesCreated != 8 || report.SourceFramesSent != 8 {
		t.Errorf("source frames = created %d sent %d, want 8/8", report.SourceFramesCreated, report.SourceFramesSent)
	}
	if report.Relay.FramesForwarded != 16 {
		t.Errorf("relay.frames_forwarded = %d, want 16", report.Relay.FramesForwarded)
	}
	if report.Receiver.FramesAccepted != 8 || report.Receiver.PlayoutFramesEmitted != 8 {
		t.Errorf("receiver accepted/emitted = %d/%d, want 8/8",
			report.Receiver.FramesAccepted, report.Receiver.PlayoutFramesEmitted)
	}
	if report.LatencyMs.P95 != 35 {
		t.Errorf("latency_ms.p95 = %v, want 35", report.LatencyMs.P95)
	}
	if report.TwoHopLive == nil {
		t.Fatal("two_hop_live section missing")
	}
	if report.TwoHopLive.EndToEndLatencyMs != 35 {
		t.Errorf("two_hop_live.end_to_end_latency_ms = %d, want 35", report.TwoHopLive.EndToEndLatencyMs)
	}
	if report.TwoHopLive.CycleSpareMs != 45 {
		t.Errorf("two_hop_live.cycle_spare_ms = %d, want 45", report.TwoHopLive.CycleSpareMs)
	}
	if report.Determinism.EventHash != "57c8429aaa0f44a079a42440b7de14b296b51048f4da3a5010150fbc92115d73" {
		t.Errorf("determinism.event_hash = %q", report.Determinism.EventHash)
	}
}

func TestIT022_TwoHopFailingMetricsValidateSchema(t *testing.T) {
	cfg := DefaultTwoHopLiveShortSourceScenarioConfig()
	cfg.VoicePeriodMs = 20

	twoHopReport, err := RunTwoHopLiveScenario(cfg)
	if err != nil {
		t.Fatalf("RunTwoHopLiveScenario failed: %v", err)
	}
	if twoHopReport.Decision != TwoHopLiveOutcomeFail {
		t.Fatalf("Decision = %q, want fail; reasons=%v", twoHopReport.Decision, twoHopReport.FailReasons)
	}

	metrics, err := TwoHopLiveScenarioMetrics(twoHopReport)
	if err != nil {
		t.Fatalf("TwoHopLiveScenarioMetrics failed: %v", err)
	}
	raw, err := metrics.MarshalReportJSON()
	if err != nil {
		t.Fatalf("MarshalReportJSON failed: %v", err)
	}
	validateMetricsJSONAgainstSchema(t, raw)

	report := metrics.Report()
	if report.Outcome != "fail" {
		t.Errorf("outcome = %q, want fail", report.Outcome)
	}
	if report.Notes == "" {
		t.Error("notes empty, want fail reasons")
	}
	if report.TwoHopLive == nil {
		t.Fatal("two_hop_live section missing")
	}
	if report.TwoHopLive.CycleSpareMs >= 0 {
		t.Errorf("two_hop_live.cycle_spare_ms = %d, want negative cycle overrun", report.TwoHopLive.CycleSpareMs)
	}
}

// TestIT046_CLIBackpressureMetricsValidateSchema proves MVP-8.20:
// the bounded relay forward-queue stress fixture is runnable through
// adhr-sim run and emits schema-valid ScenarioMetrics JSON with
// explicit queue_full counters.
func TestIT046_CLIBackpressureMetricsValidateSchema(t *testing.T) {
	metricsPath := filepath.Join(t.TempDir(), "out", "relay-forward-queue-backpressure.json")

	cmd := exec.Command("go", "run", "../../cmd/adhr-sim", "run", "../../configs/scenarios/relay-forward-queue-backpressure.json", "--metrics", metricsPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CLI run failed: %v\n%s", err, output)
	}

	raw, err := os.ReadFile(metricsPath)
	if err != nil {
		t.Fatalf("read CLI metrics failed: %v", err)
	}
	validateMetricsJSONAgainstSchema(t, raw)

	var report MetricsReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("unmarshal CLI metrics failed: %v", err)
	}
	if report.ScenarioID != RelayForwardQueueBackpressureScenarioID {
		t.Errorf("scenario_id = %q, want %q", report.ScenarioID, RelayForwardQueueBackpressureScenarioID)
	}
	if report.Outcome != "fail" {
		t.Errorf("outcome = %q, want fail under intentional queue stress", report.Outcome)
	}
	if report.Relay.ForwardQueueMaxDepth != 1 {
		t.Errorf("relay.forward_queue_max_depth = %d, want 1", report.Relay.ForwardQueueMaxDepth)
	}
	if report.Relay.ForwardQueueDrops != 4 {
		t.Errorf("relay.forward_queue_drops = %d, want 4", report.Relay.ForwardQueueDrops)
	}
	if got := report.Relay.DroppedByReason[node.DropReasonQueueFull]; got != report.Relay.ForwardQueueDrops {
		t.Errorf("relay.dropped_by_reason.queue_full = %d, want forward_queue_drops %d", got, report.Relay.ForwardQueueDrops)
	}
	if report.Relay.FramesForwarded != 2 {
		t.Errorf("relay.frames_forwarded = %d, want 2", report.Relay.FramesForwarded)
	}
	if report.Receiver.FramesAccepted != 2 || report.Receiver.MissingSequences == 0 {
		t.Errorf("receiver accepted/missing = %d/%d, want 2/nonzero",
			report.Receiver.FramesAccepted, report.Receiver.MissingSequences)
	}
	if report.Determinism.EventHash != "af9a4905f97d898abe50299db30d58a492c4ee291cbe1e75977593b99fc996c4" {
		t.Errorf("determinism.event_hash = %q", report.Determinism.EventHash)
	}
}
