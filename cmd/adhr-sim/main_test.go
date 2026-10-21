package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/node"
	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

func TestIT010_RunCommandWritesMetrics(t *testing.T) {
	metricsPath := filepath.Join(t.TempDir(), "out", "metrics.json")

	if err := run([]string{"run", scenarioConfigPath(t), "--metrics", metricsPath}); err != nil {
		t.Fatalf("run command failed: %v", err)
	}

	report := readMetricsReport(t, metricsPath)
	if report.ScenarioID != scenario.ScenarioID {
		t.Errorf("scenario_id = %q, want %q", report.ScenarioID, scenario.ScenarioID)
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

func TestIT010_RunCommandDeterministicHash(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.json")
	second := filepath.Join(dir, "nested", "second.json")

	if err := run([]string{"run", scenarioConfigPath(t), "--metrics", first}); err != nil {
		t.Fatalf("first run command failed: %v", err)
	}
	if err := run([]string{"run", scenarioConfigPath(t), "--metrics", second}); err != nil {
		t.Fatalf("second run command failed: %v", err)
	}

	report1 := readMetricsReport(t, first)
	report2 := readMetricsReport(t, second)
	if report1.Determinism.EventHash != report2.Determinism.EventHash {
		t.Errorf("determinism.event_hash mismatch:\n  first=%s\n  second=%s",
			report1.Determinism.EventHash, report2.Determinism.EventHash)
	}
	if report1.ScenarioID != report2.ScenarioID {
		t.Errorf("scenario_id mismatch: %q vs %q", report1.ScenarioID, report2.ScenarioID)
	}
}

func TestIT046_RunCommandBackpressureScenarioWritesMetrics(t *testing.T) {
	metricsPath := filepath.Join(t.TempDir(), "out", "backpressure.json")

	if err := run([]string{"run", backpressureScenarioConfigPath(t), "--metrics", metricsPath}); err != nil {
		t.Fatalf("run command failed: %v", err)
	}

	report := readMetricsReport(t, metricsPath)
	if report.ScenarioID != scenario.RelayForwardQueueBackpressureScenarioID {
		t.Errorf("scenario_id = %q, want %q", report.ScenarioID, scenario.RelayForwardQueueBackpressureScenarioID)
	}
	if report.Outcome != "fail" {
		t.Errorf("outcome = %q, want fail under intentional queue stress", report.Outcome)
	}
	if report.SourceFramesCreated != 6 || report.SourceFramesSent != 6 {
		t.Errorf("source frames = created %d sent %d, want 6/6", report.SourceFramesCreated, report.SourceFramesSent)
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
	if report.Receiver.FramesAccepted != 2 || report.Receiver.PlayoutFramesEmitted != 2 {
		t.Errorf("receiver accepted/emitted = %d/%d, want 2/2",
			report.Receiver.FramesAccepted, report.Receiver.PlayoutFramesEmitted)
	}
	if report.Receiver.MissingSequences == 0 {
		t.Error("receiver.missing_sequences = 0, want missing sequences under intentional queue stress")
	}
	if report.Determinism.EventHash != "af9a4905f97d898abe50299db30d58a492c4ee291cbe1e75977593b99fc996c4" {
		t.Errorf("determinism.event_hash = %q", report.Determinism.EventHash)
	}
}

func TestIT046_RunCommandBackpressureScenarioDeterministicHash(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.json")
	second := filepath.Join(dir, "second.json")

	if err := run([]string{"run", backpressureScenarioConfigPath(t), "--metrics", first}); err != nil {
		t.Fatalf("first run command failed: %v", err)
	}
	if err := run([]string{"run", backpressureScenarioConfigPath(t), "--metrics", second}); err != nil {
		t.Fatalf("second run command failed: %v", err)
	}

	report1 := readMetricsReport(t, first)
	report2 := readMetricsReport(t, second)
	if report1.Determinism.EventHash != report2.Determinism.EventHash {
		t.Errorf("determinism.event_hash mismatch:\n  first=%s\n  second=%s",
			report1.Determinism.EventHash, report2.Determinism.EventHash)
	}
}

func TestLoadScenarioConfigMapsOneHopImpairmentFields(t *testing.T) {
	configPath := writeScenarioConfig(t, `{
  "scenario_id": "one_hop_nominal",
  "source_to_receiver_enabled": true,
  "source_to_receiver_loss_pct": 5,
  "source_to_relay_duplicate_pct": 10,
  "relay_to_receiver_duplicate_pct": 15,
  "source_to_receiver_duplicate_pct": 20,
  "source_to_relay_corruption_pct": 25,
  "relay_to_receiver_corruption_pct": 30,
  "source_to_receiver_corruption_pct": 35,
  "source_to_receiver_jitter_ms": 7,
  "relay_seen_cache_max_entries": 3,
  "relay_forward_queue_max_depth": 4,
  "allow_slot_timing_stress": true,
  "voice_group_id_override": 200
}`)

	_, runnable, err := loadScenarioConfig(configPath)
	if err != nil {
		t.Fatalf("loadScenarioConfig failed: %v", err)
	}
	if runnable.OneHop == nil {
		t.Fatal("OneHop config is nil")
	}
	cfg := *runnable.OneHop
	if !cfg.SourceToReceiverEnabled {
		t.Fatal("SourceToReceiverEnabled = false, want true")
	}
	if cfg.SourceToReceiverLossPct != 5 ||
		cfg.SourceToRelayDuplicatePct != 10 ||
		cfg.RelayToReceiverDuplicatePct != 15 ||
		cfg.SourceToReceiverDuplicatePct != 20 ||
		cfg.SourceToRelayCorruptionPct != 25 ||
		cfg.RelayToReceiverCorruptionPct != 30 ||
		cfg.SourceToReceiverCorruptionPct != 35 ||
		cfg.SourceToReceiverJitterMs != 7 ||
		cfg.RelaySeenCacheMaxEntries != 3 ||
		cfg.RelayForwardQueueMaxDepth != 4 ||
		!cfg.AllowSlotTimingStress ||
		cfg.VoiceGroupIDOverride != 200 {
		t.Fatalf("mapped impairment config mismatch: %+v", cfg)
	}
}

func TestRunCommandRejectsBackpressureScenarioWithoutSlotMode(t *testing.T) {
	configPath := writeScenarioConfig(t, `{
  "scenario_id": "relay_forward_queue_backpressure",
  "slot_mode_enabled": false,
  "allow_slot_timing_stress": true
}`)
	metricsPath := filepath.Join(t.TempDir(), "metrics.json")

	err := run([]string{"run", configPath, "--metrics", metricsPath})
	if err == nil {
		t.Fatal("run command accepted backpressure scenario without slot mode")
	}
	if !strings.Contains(err.Error(), "requires slot_mode_enabled") {
		t.Fatalf("error = %q, want slot-mode requirement context", err.Error())
	}
	assertFileMissing(t, metricsPath)
}

func TestRunCommandRejectsBackpressureScenarioWithoutTimingStress(t *testing.T) {
	configPath := writeScenarioConfig(t, `{
  "scenario_id": "relay_forward_queue_backpressure",
  "frame_period_ms": 20,
  "slot_mode_enabled": true,
  "slot_duration_ms": 40
}`)
	metricsPath := filepath.Join(t.TempDir(), "metrics.json")

	err := run([]string{"run", configPath, "--metrics", metricsPath})
	if err == nil {
		t.Fatal("run command accepted backpressure scenario without explicit timing stress")
	}
	if !strings.Contains(err.Error(), "requires allow_slot_timing_stress") {
		t.Fatalf("error = %q, want timing-stress requirement context", err.Error())
	}
	assertFileMissing(t, metricsPath)
}

func TestRunCommandRejectsBackpressureScenarioWithoutQueueStress(t *testing.T) {
	configPath := writeScenarioConfig(t, `{
  "scenario_id": "relay_forward_queue_backpressure",
  "frame_period_ms": 80,
  "slot_mode_enabled": true,
  "slot_duration_ms": 40,
  "allow_slot_timing_stress": true,
  "relay_forward_queue_max_depth": 1
}`)
	metricsPath := filepath.Join(t.TempDir(), "metrics.json")

	err := run([]string{"run", configPath, "--metrics", metricsPath})
	if err == nil {
		t.Fatal("run command accepted backpressure scenario without queue stress")
	}
	if !strings.Contains(err.Error(), "requires frame_period_ms < 2*slot_duration_ms") {
		t.Fatalf("error = %q, want queue-stress requirement context", err.Error())
	}
	assertFileMissing(t, metricsPath)
}

func TestRunCommandRejectsUnknownJSONField(t *testing.T) {
	configPath := writeScenarioConfig(t, `{
  "scenario_id": "one_hop_nominal",
  "seed": 42,
  "frame_count": 100,
  "frame_period_ms": 80,
  "start_ms": 1000,
  "link_delay_ms": 10,
  "source_id": 1,
  "relay_id": 2,
  "receiver_id": 3,
  "net_id": 1,
  "group_id": 100,
  "call_id": 9001,
  "ttl": 1,
  "slot_mode_enabled": false,
  "slot_duration_ms": 0,
  "unexpected": true
}`)
	metricsPath := filepath.Join(t.TempDir(), "metrics.json")

	err := run([]string{"run", configPath, "--metrics", metricsPath})
	if err == nil {
		t.Fatal("run command accepted unknown JSON field")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("error = %q, want unknown field context", err.Error())
	}
	assertFileMissing(t, metricsPath)
}

func TestRunCommandRejectsMalformedJSON(t *testing.T) {
	configPath := writeScenarioConfig(t, `{"scenario_id": "one_hop_nominal"`)
	metricsPath := filepath.Join(t.TempDir(), "metrics.json")

	err := run([]string{"run", configPath, "--metrics", metricsPath})
	if err == nil {
		t.Fatal("run command accepted malformed JSON")
	}
	if !strings.Contains(err.Error(), "decode scenario config") {
		t.Fatalf("error = %q, want decode scenario config context", err.Error())
	}
	assertFileMissing(t, metricsPath)
}

func TestRunCommandRejectsMissingScenarioFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing-scenario.json")
	metricsPath := filepath.Join(t.TempDir(), "metrics.json")

	err := run([]string{"run", configPath, "--metrics", metricsPath})
	if err == nil {
		t.Fatal("run command accepted missing scenario file")
	}
	if !strings.Contains(err.Error(), "read scenario config") {
		t.Fatalf("error = %q, want read scenario config context", err.Error())
	}
	assertFileMissing(t, metricsPath)
}

func TestRunCommandConvertsScenarioPanicToError(t *testing.T) {
	configPath := writeScenarioConfig(t, `{
  "scenario_id": "one_hop_nominal",
  "seed": 42,
  "frame_count": 1,
  "frame_period_ms": 80,
  "start_ms": 9223372036854775800,
  "link_delay_ms": 10,
  "source_id": 1,
  "relay_id": 2,
  "receiver_id": 3,
  "net_id": 1,
  "group_id": 100,
  "call_id": 9001,
  "ttl": 1,
  "slot_mode_enabled": false,
  "slot_duration_ms": 0
}`)
	metricsPath := filepath.Join(t.TempDir(), "metrics.json")

	err := run([]string{"run", configPath, "--metrics", metricsPath})
	if err == nil {
		t.Fatal("run command accepted panic-inducing scenario config")
	}
	if !strings.Contains(err.Error(), "scenario validation failed") {
		t.Fatalf("error = %q, want scenario validation failed context", err.Error())
	}
	assertFileMissing(t, metricsPath)
}

func TestRunCommandRejectsUnsupportedScenarioID(t *testing.T) {
	configPath := writeScenarioConfig(t, `{
  "scenario_id": "not_supported",
  "seed": 42,
  "frame_count": 100,
  "frame_period_ms": 80,
  "start_ms": 1000,
  "link_delay_ms": 10,
  "source_id": 1,
  "relay_id": 2,
  "receiver_id": 3,
  "net_id": 1,
  "group_id": 100,
  "call_id": 9001,
  "ttl": 1,
  "slot_mode_enabled": false,
  "slot_duration_ms": 0
}`)

	err := run([]string{"run", configPath, "--metrics", filepath.Join(t.TempDir(), "metrics.json")})
	if err == nil {
		t.Fatal("run command accepted unsupported scenario_id")
	}
	if !strings.Contains(err.Error(), "unsupported scenario_id") {
		t.Fatalf("error = %q, want unsupported scenario_id context", err.Error())
	}
}

func TestRunCommandDoesNotLeavePartialMetricsOnWriteError(t *testing.T) {
	parentFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parentFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("write parent file failed: %v", err)
	}
	metricsPath := filepath.Join(parentFile, "metrics.json")

	err := run([]string{"run", scenarioConfigPath(t), "--metrics", metricsPath})
	if err == nil {
		t.Fatal("run command succeeded with unwritable metrics path")
	}
	assertFileMissing(t, metricsPath)
}

func TestRunCommandRejectsFlagLikeMetricsPath(t *testing.T) {
	err := run([]string{"run", scenarioConfigPath(t), "--metrics", "--metrics"})
	if err == nil {
		t.Fatal("run command accepted flag-like metrics path")
	}
	if !strings.Contains(err.Error(), "flag-like") {
		t.Fatalf("error = %q, want flag-like metrics path context", err.Error())
	}
}

func TestRunCommandRejectsHelpAsMetricsPath(t *testing.T) {
	err := run([]string{"run", scenarioConfigPath(t), "--metrics", "--help"})
	if err == nil {
		t.Fatal("run command accepted --help as metrics path")
	}
	if !strings.Contains(err.Error(), "flag-like") {
		t.Fatalf("error = %q, want flag-like metrics path context", err.Error())
	}
}

func TestRunCommandRejectsSlotModeForOneHopScenario(t *testing.T) {
	configPath := writeScenarioConfig(t, `{
  "scenario_id": "one_hop_nominal",
  "seed": 42,
  "frame_count": 100,
  "frame_period_ms": 80,
  "start_ms": 0,
  "link_delay_ms": 10,
  "source_id": 1,
  "relay_id": 2,
  "receiver_id": 3,
  "net_id": 1,
  "group_id": 100,
  "call_id": 9001,
  "ttl": 1,
  "slot_mode_enabled": true,
  "slot_duration_ms": 40
}`)

	err := run([]string{"run", configPath, "--metrics", filepath.Join(t.TempDir(), "metrics.json")})
	if err == nil {
		t.Fatal("run command accepted slot mode for one-hop scenario_id")
	}
	if !strings.Contains(err.Error(), "does not support slot_mode_enabled") {
		t.Fatalf("error = %q, want slot mode rejection context", err.Error())
	}
}

func TestRunCommandRejectsDirectoryShapedMetricsPath(t *testing.T) {
	metricsPath := filepath.Join(t.TempDir(), "out") + string(os.PathSeparator)

	err := run([]string{"run", scenarioConfigPath(t), "--metrics", metricsPath})
	if err == nil {
		t.Fatal("run command accepted directory-shaped metrics path")
	}
	if !strings.Contains(err.Error(), "must name a file") {
		t.Fatalf("error = %q, want file path context", err.Error())
	}
	if _, err := os.Stat(metricsPath); !os.IsNotExist(err) {
		t.Fatalf("directory-shaped metrics target exists or returned unexpected error: %v", err)
	}
}

func TestWriteMetricsFileCleansTempFileOnRenameFailure(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "metrics.json")
	if err := os.Mkdir(targetDir, 0o755); err != nil {
		t.Fatalf("mkdir target dir failed: %v", err)
	}

	err := writeMetricsFile(targetDir, []byte("{}"))
	if err == nil {
		t.Fatal("writeMetricsFile succeeded when target is an existing directory")
	}

	matches, err := filepath.Glob(filepath.Join(dir, ".metrics.json.tmp-*"))
	if err != nil {
		t.Fatalf("glob temp files failed: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary metrics files remain after rename failure: %v", matches)
	}
}

func TestRunCommandHelpDoesNotRequireScenarioPath(t *testing.T) {
	if err := run([]string{"run", "--help"}); err != nil {
		t.Fatalf("run --help returned error: %v", err)
	}
}

func TestMainReportsErrorToStderrAndExitsNonZero(t *testing.T) {
	configPath := writeScenarioConfig(t, `{"scenario_id": "one_hop_nominal"`)
	metricsPath := filepath.Join(t.TempDir(), "metrics.json")
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcessCLIError")
	cmd.Env = append(os.Environ(),
		"GO_WANT_HELPER_PROCESS=1",
		"ADHR_HELPER_ARGS=run\n"+configPath+"\n--metrics\n"+metricsPath,
	)

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("helper process succeeded, want non-zero exit")
	}
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() == 0 {
		t.Fatalf("helper process err = %v, want non-zero exit", err)
	}
	if !strings.Contains(string(output), "decode scenario config") {
		t.Fatalf("stderr/stdout = %q, want actionable decode error", output)
	}
	assertFileMissing(t, metricsPath)
}

func TestHelperProcessCLIError(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := strings.Split(os.Getenv("ADHR_HELPER_ARGS"), "\n")
	os.Args = append([]string{os.Args[0]}, args...)
	main()
}

func readMetricsReport(t *testing.T, path string) scenario.MetricsReport {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read metrics file failed: %v", err)
	}
	var report scenario.MetricsReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("unmarshal metrics report failed: %v\n%s", err, raw)
	}
	return report
}

func scenarioConfigPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "configs", "scenarios", "one-hop.json")
}

func backpressureScenarioConfigPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "configs", "scenarios", "relay-forward-queue-backpressure.json")
}

func writeScenarioConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "scenario.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write scenario config failed: %v", err)
	}
	return path
}

func assertFileMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !(os.IsNotExist(err) || strings.Contains(err.Error(), "not a directory")) {
		t.Fatalf("metrics path %s exists or returned unexpected error: %v", path, err)
	}
}
