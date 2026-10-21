package scenario

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
)

const (
	sweepSchemaVersion = 1
	sweepSummaryType   = "experiment_sweeps"
	sweepMetricsDir    = "metrics"
	timingBudgetP95Ms  = 250
)

// SweepSummary is the compact machine-readable Story 3.5 output.
type SweepSummary struct {
	SchemaVersion int               `json:"schema_version"`
	SummaryType   string            `json:"summary_type"`
	Experiments   []string          `json:"experiments"`
	Rows          []SweepSummaryRow `json:"rows"`
}

// SweepSummaryRow records one deterministic experiment parameter tuple.
type SweepSummaryRow struct {
	ExperimentID                 string          `json:"experiment_id"`
	RowID                        string          `json:"row_id"`
	MetricsPath                  string          `json:"metrics_path"`
	ScenarioID                   string          `json:"scenario_id"`
	Seed                         uint64          `json:"seed"`
	Parameters                   SweepParameters `json:"parameters"`
	InvalidConfig                bool            `json:"invalid_config"`
	ConfigError                  string          `json:"config_error,omitempty"`
	Outcome                      string          `json:"outcome"`
	EventHash                    string          `json:"event_hash"`
	EventCount                   int             `json:"event_count"`
	SourceFramesCreated          int             `json:"source_frames_created"`
	LinkDelivered                int             `json:"link_delivered"`
	LinkLost                     int             `json:"link_lost"`
	SourceToRelayDelivered       int             `json:"source_to_relay_delivered"`
	SourceToRelayLost            int             `json:"source_to_relay_lost"`
	RelayToReceiverDelivered     int             `json:"relay_to_receiver_delivered"`
	RelayToReceiverLost          int             `json:"relay_to_receiver_lost"`
	ReceiverFramesAccepted       int             `json:"receiver_frames_accepted"`
	ReceiverLateFrames           int             `json:"receiver_late_frames"`
	ReceiverMissingSequences     int             `json:"receiver_missing_sequences"`
	ReceiverPlayoutFramesEmitted int             `json:"receiver_playout_frames_emitted"`
	OutOfSlotTxCount             int             `json:"out_of_slot_tx_count"`
	LatencyP95Ms                 float64         `json:"latency_p95_ms"`
	TimingBudgetPass             bool            `json:"timing_budget_pass"`
}

// SweepParameters is intentionally concrete: no map[string]interface{}
// and no broad experiment DSL are needed for Story 3.5.
type SweepParameters struct {
	LinkSet                 string   `json:"link_set,omitempty"`
	LossPct                 *float64 `json:"loss_pct,omitempty"`
	SlotDurationMs          *int64   `json:"slot_duration_ms,omitempty"`
	FramePeriodMs           *int64   `json:"frame_period_ms,omitempty"`
	ReceiverJitterBufferMs  *int64   `json:"receiver_jitter_buffer_ms,omitempty"`
	ReceiverLateThresholdMs *int64   `json:"receiver_late_threshold_ms,omitempty"`
	LinkJitterMs            *int64   `json:"link_jitter_ms,omitempty"`
}

// SweepRunResult carries the row plus the full metrics object that CLI
// code writes as the per-run schema-valid metrics JSON file.
type SweepRunResult struct {
	Row     SweepSummaryRow
	Metrics ScenarioMetrics
}

// SweepResult is the full in-memory Story 3.5 result.
type SweepResult struct {
	Summary SweepSummary
	Runs    []SweepRunResult
}

type sweepCase struct {
	ExperimentID string
	RowID        string
	Config       ScenarioConfig
	Parameters   SweepParameters
}

// RunExperimentSweeps executes E-003, E-004, and E-005 with fixed
// seeds. It uses the same ScenarioRunner and ScenarioMetrics export
// path as the CLI scenario run command; it does not use wall-clock time.
func RunExperimentSweeps() (SweepResult, error) {
	cases := experimentSweepCases()
	sort.Slice(cases, func(i, j int) bool {
		if cases[i].ExperimentID != cases[j].ExperimentID {
			return cases[i].ExperimentID < cases[j].ExperimentID
		}
		return cases[i].RowID < cases[j].RowID
	})

	result := SweepResult{
		Summary: SweepSummary{
			SchemaVersion: sweepSchemaVersion,
			SummaryType:   sweepSummaryType,
			Experiments:   []string{"E-003", "E-004", "E-005"},
			Rows:          make([]SweepSummaryRow, 0, len(cases)),
		},
		Runs: make([]SweepRunResult, 0, len(cases)),
	}

	for _, c := range cases {
		metrics, err := runSweepCase(c.Config)
		if err != nil {
			return SweepResult{}, fmt.Errorf("sweep row %s: %w", c.RowID, err)
		}
		row := sweepSummaryRow(c, metrics)
		result.Summary.Rows = append(result.Summary.Rows, row)
		result.Runs = append(result.Runs, SweepRunResult{Row: row, Metrics: metrics})
	}

	return result, nil
}

func MarshalSweepSummaryJSON(summary SweepSummary) ([]byte, error) {
	return json.MarshalIndent(summary, "", "  ")
}

func experimentSweepCases() []sweepCase {
	var cases []sweepCase
	cases = append(cases, e003SweepCases()...)
	cases = append(cases, e004SweepCases()...)
	cases = append(cases, e005SweepCases()...)
	return cases
}

func e003SweepCases() []sweepCase {
	lossValues := []float64{0, 1, 5, 10, 20}
	linkSets := []struct {
		id            string
		sourceToRelay bool
		relayToRx     bool
	}{
		{id: "a_to_r", sourceToRelay: true},
		{id: "both", sourceToRelay: true, relayToRx: true},
		{id: "r_to_b", relayToRx: true},
	}

	var cases []sweepCase
	for _, lossPct := range lossValues {
		for _, linkSet := range linkSets {
			cfg := DefaultScenarioConfig()
			if linkSet.sourceToRelay {
				cfg.SourceToRelayLossPct = lossPct
			}
			if linkSet.relayToRx {
				cfg.RelayToReceiverLossPct = lossPct
			}
			rowID := fmt.Sprintf("e003_loss_%03d_%s_seed_%d", int(lossPct), linkSet.id, cfg.Seed)
			cases = append(cases, sweepCase{
				ExperimentID: "E-003",
				RowID:        rowID,
				Config:       cfg,
				Parameters: SweepParameters{
					LinkSet: linkSet.id,
					LossPct: float64Ptr(lossPct),
				},
			})
		}
	}
	return cases
}

func e004SweepCases() []sweepCase {
	slotDurations := []int64{20, 40, 80}
	cases := make([]sweepCase, 0, len(slotDurations))
	for _, slotDurationMs := range slotDurations {
		cfg := DefaultTwoSlotScenarioConfig()
		cfg.SlotDurationMs = slotDurationMs
		cfg.FramePeriodMs = slotDurationMs * 2
		rowID := fmt.Sprintf("e004_slot_%03d_frame_%03d_seed_%d", slotDurationMs, cfg.FramePeriodMs, cfg.Seed)
		cases = append(cases, sweepCase{
			ExperimentID: "E-004",
			RowID:        rowID,
			Config:       cfg,
			Parameters: SweepParameters{
				SlotDurationMs: int64Ptr(slotDurationMs),
				FramePeriodMs:  int64Ptr(cfg.FramePeriodMs),
			},
		})
	}
	return cases
}

func e005SweepCases() []sweepCase {
	jitterBuffers := []int64{80, 120, 160, 240, 320}
	lateThresholds := []int64{160, 200, 240, 250, 320}
	linkJitters := []int64{0, 10, 30, 60}

	var cases []sweepCase
	for _, jitterBufferMs := range jitterBuffers {
		for _, lateThresholdMs := range lateThresholds {
			for _, linkJitterMs := range linkJitters {
				cfg := DefaultScenarioConfig()
				cfg.ReceiverJitterBufferMs = jitterBufferMs
				cfg.ReceiverLateThresholdMs = lateThresholdMs
				cfg.SourceToRelayJitterMs = linkJitterMs
				cfg.RelayToReceiverJitterMs = linkJitterMs
				rowID := fmt.Sprintf("e005_jbuf_%03d_late_%03d_lj_%03d_seed_%d",
					jitterBufferMs, lateThresholdMs, linkJitterMs, cfg.Seed)
				cases = append(cases, sweepCase{
					ExperimentID: "E-005",
					RowID:        rowID,
					Config:       cfg,
					Parameters: SweepParameters{
						ReceiverJitterBufferMs:  int64Ptr(jitterBufferMs),
						ReceiverLateThresholdMs: int64Ptr(lateThresholdMs),
						LinkJitterMs:            int64Ptr(linkJitterMs),
					},
				})
			}
		}
	}
	return cases
}

func runSweepCase(cfg ScenarioConfig) (metrics ScenarioMetrics, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("scenario validation failed: %v", recovered)
		}
	}()

	runner := NewScenarioRunner(cfg)
	return runner.Run()
}

func sweepSummaryRow(c sweepCase, metrics ScenarioMetrics) SweepSummaryRow {
	report := metrics.Report()
	sourceToRelayKey := linkReportKey(metrics.SourceID, metrics.RelayID)
	relayToReceiverKey := linkReportKey(metrics.RelayID, metrics.ReceiverID)
	return SweepSummaryRow{
		ExperimentID:                 c.ExperimentID,
		RowID:                        c.RowID,
		MetricsPath:                  path.Join(sweepMetricsDir, c.RowID+".json"),
		ScenarioID:                   report.ScenarioID,
		Seed:                         report.Seed,
		Parameters:                   c.Parameters,
		InvalidConfig:                false,
		Outcome:                      report.Outcome,
		EventHash:                    report.Determinism.EventHash,
		EventCount:                   report.Determinism.EventCount,
		SourceFramesCreated:          report.SourceFramesCreated,
		LinkDelivered:                sumIntMap(metrics.LinkCounters.Delivered),
		LinkLost:                     sumIntMap(metrics.LinkCounters.Lost),
		SourceToRelayDelivered:       metrics.LinkCounters.Delivered[sourceToRelayKey],
		SourceToRelayLost:            metrics.LinkCounters.Lost[sourceToRelayKey],
		RelayToReceiverDelivered:     metrics.LinkCounters.Delivered[relayToReceiverKey],
		RelayToReceiverLost:          metrics.LinkCounters.Lost[relayToReceiverKey],
		ReceiverFramesAccepted:       report.Receiver.FramesAccepted,
		ReceiverLateFrames:           report.Receiver.LateFrames,
		ReceiverMissingSequences:     report.Receiver.MissingSequences,
		ReceiverPlayoutFramesEmitted: report.Receiver.PlayoutFramesEmitted,
		OutOfSlotTxCount:             report.OutOfSlotTxCount,
		LatencyP95Ms:                 report.LatencyMs.P95,
		TimingBudgetPass:             timingBudgetPass(report),
	}
}

func timingBudgetPass(report MetricsReport) bool {
	return report.SourceFramesCreated > 0 &&
		report.LatencyMs.P95 < timingBudgetP95Ms &&
		report.OutOfSlotTxCount == 0 &&
		report.Receiver.LateFrames == 0 &&
		report.Receiver.MissingSequences == 0 &&
		report.Receiver.PlayoutFramesEmitted == report.SourceFramesCreated
}

func sumIntMap(in map[string]int) int {
	var total int
	for _, v := range in {
		total += v
	}
	return total
}

func int64Ptr(v int64) *int64 {
	return &v
}

func float64Ptr(v float64) *float64 {
	return &v
}
