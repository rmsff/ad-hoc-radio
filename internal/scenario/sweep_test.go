package scenario

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

func TestIT011_ExperimentSweepsProduceSchemaValidMetrics(t *testing.T) {
	result, err := RunExperimentSweeps()
	if err != nil {
		t.Fatalf("RunExperimentSweeps failed: %v", err)
	}

	if result.Summary.SchemaVersion != 1 {
		t.Errorf("schema_version = %d, want 1", result.Summary.SchemaVersion)
	}
	if result.Summary.SummaryType != "experiment_sweeps" {
		t.Errorf("summary_type = %q, want experiment_sweeps", result.Summary.SummaryType)
	}
	if len(result.Summary.Rows) != 118 {
		t.Fatalf("summary rows = %d, want 118", len(result.Summary.Rows))
	}
	if len(result.Runs) != len(result.Summary.Rows) {
		t.Fatalf("runs = %d, summary rows = %d", len(result.Runs), len(result.Summary.Rows))
	}
	if !sweepRowsSorted(result.Summary.Rows) {
		t.Fatal("summary rows are not sorted by experiment_id and row_id")
	}

	seenPaths := make(map[string]bool, len(result.Runs))
	for i, run := range result.Runs {
		raw, err := run.Metrics.MarshalReportJSON()
		if err != nil {
			t.Fatalf("row %s MarshalReportJSON failed: %v", run.Row.RowID, err)
		}
		validateMetricsJSONAgainstSchema(t, raw)

		report := run.Metrics.Report()
		if result.Summary.Rows[i] != run.Row {
			t.Fatalf("summary row %d does not match run row:\nsummary=%+v\nrun=%+v", i, result.Summary.Rows[i], run.Row)
		}
		if run.Row.EventHash != report.Determinism.EventHash {
			t.Errorf("row %s event_hash = %s, want metrics hash %s",
				run.Row.RowID, run.Row.EventHash, report.Determinism.EventHash)
		}
		if run.Row.MetricsPath == "" {
			t.Errorf("row %s has empty metrics_path", run.Row.RowID)
		}
		if seenPaths[run.Row.MetricsPath] {
			t.Errorf("duplicate metrics_path %q", run.Row.MetricsPath)
		}
		seenPaths[run.Row.MetricsPath] = true
	}
}

func TestIT011_ExperimentSweepRepeatability(t *testing.T) {
	first, err := RunExperimentSweeps()
	if err != nil {
		t.Fatalf("first RunExperimentSweeps failed: %v", err)
	}
	second, err := RunExperimentSweeps()
	if err != nil {
		t.Fatalf("second RunExperimentSweeps failed: %v", err)
	}

	if !reflect.DeepEqual(first.Summary, second.Summary) {
		firstJSON, _ := json.MarshalIndent(first.Summary, "", "  ")
		secondJSON, _ := json.MarshalIndent(second.Summary, "", "  ")
		t.Fatalf("sweep summary is not repeatable:\nfirst=%s\nsecond=%s", firstJSON, secondJSON)
	}
	for i := range first.Runs {
		if first.Runs[i].Row.EventHash != second.Runs[i].Row.EventHash {
			t.Errorf("row %d event hash mismatch: %s vs %s",
				i, first.Runs[i].Row.EventHash, second.Runs[i].Row.EventHash)
		}
	}
}

func TestIT011_E003SummaryRecordsLossBehavior(t *testing.T) {
	result, err := RunExperimentSweeps()
	if err != nil {
		t.Fatalf("RunExperimentSweeps failed: %v", err)
	}

	rows := rowsForExperiment(result.Summary.Rows, "E-003")
	if len(rows) != 15 {
		t.Fatalf("E-003 rows = %d, want 15", len(rows))
	}

	var nominalFound bool
	var lossyFound bool
	for _, row := range rows {
		if row.Parameters.LossPct == nil || row.Parameters.LinkSet == "" {
			t.Fatalf("E-003 row %s missing loss_pct/link_set parameters: %+v", row.RowID, row.Parameters)
		}
		if *row.Parameters.LossPct == 0 && row.Parameters.LinkSet == "both" {
			nominalFound = true
			if row.LinkLost != 0 || row.ReceiverMissingSequences != 0 || row.ReceiverFramesAccepted != DefaultFrameCount {
				t.Errorf("E-003 nominal both row = %+v, want no loss/missing and %d accepted frames", row, DefaultFrameCount)
			}
		}
		if *row.Parameters.LossPct == 20 && row.LinkLost > 0 && row.ReceiverMissingSequences > 0 {
			lossyFound = true
		}
		if row.ReceiverMissingSequences > 0 && row.TimingBudgetPass {
			t.Errorf("E-003 row %s timing_budget_pass=true with missing_sequences=%d",
				row.RowID, row.ReceiverMissingSequences)
		}
	}
	if !nominalFound {
		t.Fatal("E-003 nominal loss_pct=0 link_set=both row not found")
	}
	if !lossyFound {
		t.Fatal("E-003 has no 20% loss row with link_lost > 0 and missing_sequences > 0")
	}
}

func TestIT011_E004SummaryRecordsSlotTiming(t *testing.T) {
	result, err := RunExperimentSweeps()
	if err != nil {
		t.Fatalf("RunExperimentSweeps failed: %v", err)
	}

	rows := rowsForExperiment(result.Summary.Rows, "E-004")
	if len(rows) != 3 {
		t.Fatalf("E-004 rows = %d, want 3 valid tuples", len(rows))
	}
	for _, row := range rows {
		if row.Parameters.SlotDurationMs == nil || row.Parameters.FramePeriodMs == nil {
			t.Fatalf("E-004 row %s missing slot/frame parameters: %+v", row.RowID, row.Parameters)
		}
		if *row.Parameters.FramePeriodMs != 2*(*row.Parameters.SlotDurationMs) {
			t.Errorf("E-004 row %s has invalid tuple frame_period_ms=%d slot_duration_ms=%d",
				row.RowID, *row.Parameters.FramePeriodMs, *row.Parameters.SlotDurationMs)
		}
		if row.InvalidConfig {
			t.Errorf("E-004 row %s unexpectedly marked invalid_config", row.RowID)
		}
		if row.OutOfSlotTxCount != 0 {
			t.Errorf("E-004 row %s out_of_slot_tx_count = %d, want 0", row.RowID, row.OutOfSlotTxCount)
		}
		if row.LatencyP95Ms <= 0 {
			t.Errorf("E-004 row %s latency_p95_ms = %v, want > 0", row.RowID, row.LatencyP95Ms)
		}
		if !row.TimingBudgetPass {
			t.Errorf("E-004 row %s timing_budget_pass = false, want true for generated valid tuples", row.RowID)
		}
	}
}

func TestIT011_E005SummaryRecordsJitterTradeoffs(t *testing.T) {
	result, err := RunExperimentSweeps()
	if err != nil {
		t.Fatalf("RunExperimentSweeps failed: %v", err)
	}

	rows := rowsForExperiment(result.Summary.Rows, "E-005")
	if len(rows) != 100 {
		t.Fatalf("E-005 rows = %d, want 100", len(rows))
	}

	var lowBufferP95 float64
	var highBufferP95 float64
	var lowFound bool
	var highFound bool
	for _, row := range rows {
		if row.Parameters.ReceiverJitterBufferMs == nil ||
			row.Parameters.ReceiverLateThresholdMs == nil ||
			row.Parameters.LinkJitterMs == nil {
			t.Fatalf("E-005 row %s missing jitter parameters: %+v", row.RowID, row.Parameters)
		}
		if row.ReceiverPlayoutFramesEmitted < 0 || row.ReceiverLateFrames < 0 || row.ReceiverMissingSequences < 0 {
			t.Errorf("E-005 row %s has negative receiver counters: %+v", row.RowID, row)
		}
		if *row.Parameters.LinkJitterMs == 0 &&
			*row.Parameters.ReceiverLateThresholdMs == 250 &&
			*row.Parameters.ReceiverJitterBufferMs == 80 {
			lowBufferP95 = row.LatencyP95Ms
			lowFound = true
		}
		if *row.Parameters.LinkJitterMs == 0 &&
			*row.Parameters.ReceiverLateThresholdMs == 250 &&
			*row.Parameters.ReceiverJitterBufferMs == 320 {
			highBufferP95 = row.LatencyP95Ms
			highFound = true
		}
	}
	if !lowFound || !highFound {
		t.Fatalf("E-005 comparison rows missing: lowFound=%v highFound=%v", lowFound, highFound)
	}
	if !(lowBufferP95 < highBufferP95) {
		t.Fatalf("E-005 latency tradeoff not visible: jitter_buffer=80 p95=%v, jitter_buffer=320 p95=%v",
			lowBufferP95, highBufferP95)
	}
}

func TestScenario_LinkJitterLatencyUsesObservedDeliveryDelays(t *testing.T) {
	cfg := DefaultScenarioConfig()
	cfg.SourceToRelayJitterMs = 60
	cfg.RelayToReceiverJitterMs = 60
	cfg.ReceiverJitterBufferMs = 80

	runner := NewScenarioRunner(cfg)
	metrics, err := runner.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	report := metrics.Report()
	relayLinkAvg := linkAvgDelay(t, report, "2", "3")
	if report.LatencyMs.RelayToReceiverAvg != relayLinkAvg {
		t.Errorf("relay_to_receiver_avg = %v, want observed link avg %v", report.LatencyMs.RelayToReceiverAvg, relayLinkAvg)
	}
	if report.LatencyMs.SourceToRelayAvg <= float64(DefaultLinkDelayMs) {
		t.Errorf("source_to_relay_avg = %v, want > nominal link delay %d under jitter",
			report.LatencyMs.SourceToRelayAvg, DefaultLinkDelayMs)
	}
	if report.LatencyMs.RelayToReceiverAvg <= float64(DefaultLinkDelayMs) {
		t.Errorf("relay_to_receiver_avg = %v, want > nominal link delay %d under jitter",
			report.LatencyMs.RelayToReceiverAvg, DefaultLinkDelayMs)
	}
	if report.LatencyMs.P95 <= float64(receiverJitterBufferMsForConfig(cfg)) {
		t.Errorf("latency_ms.p95 = %v, want > jitter_buffer_ms %d when observed receiver arrivals exceed playout deadline",
			report.LatencyMs.P95, receiverJitterBufferMsForConfig(cfg))
	}
}

func linkAvgDelay(t *testing.T, report MetricsReport, from, to string) float64 {
	t.Helper()
	for _, link := range report.Links {
		if link.From == from && link.To == to {
			if link.AvgDelayMs == nil {
				t.Fatalf("link %s->%s avg_delay_ms missing", from, to)
			}
			return *link.AvgDelayMs
		}
	}
	t.Fatalf("link %s->%s not found in report: %+v", from, to, report.Links)
	return 0
}

func rowsForExperiment(rows []SweepSummaryRow, experimentID string) []SweepSummaryRow {
	var out []SweepSummaryRow
	for _, row := range rows {
		if row.ExperimentID == experimentID {
			out = append(out, row)
		}
	}
	return out
}

func sweepRowsSorted(rows []SweepSummaryRow) bool {
	return sort.SliceIsSorted(rows, func(i, j int) bool {
		if rows[i].ExperimentID != rows[j].ExperimentID {
			return rows[i].ExperimentID < rows[j].ExperimentID
		}
		return rows[i].RowID < rows[j].RowID
	})
}
