package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	readinessDashboardSchemaVersion = 1
	readinessDashboardScenarioID    = "evidence_readiness_dashboard"
	readinessDashboardHashAlgorithm = "sha256"
	readinessDashboardSerialization = "readiness-dashboard-v1"
)

const (
	readinessDashboardOutcomePlanningReadyStationRequired = "planning_ready_station_required"
	readinessDashboardOutcomeFirmwareBenchReady           = "firmware_bench_ready"
	readinessDashboardOutcomeUnexpectedRegression         = "unexpected_regression"
)

const (
	readinessDashboardClassSimulationProven = "simulation_proven"
	readinessDashboardClassExpectedStress   = "expected_stress"
	readinessDashboardClassFixtureProxy     = "fixture_proxy"
	readinessDashboardClassHardwareBlocked  = "hardware_blocked"
	readinessDashboardClassHardwareBench    = "hardware_bench_ready"
	readinessDashboardClassUnexpected       = "unexpected"
	readinessDashboardClassUnknown          = "unknown"
)

type readinessDashboardArgs struct {
	summaryPath string
	reportPath  string
}

type readinessDashboardReport struct {
	SchemaVersion         int                         `json:"schema_version"`
	ScenarioID            string                      `json:"scenario_id"`
	SourcePackID          string                      `json:"source_pack_id"`
	SourcePackEventHash   string                      `json:"source_pack_event_hash"`
	RegressionOutcome     string                      `json:"regression_outcome"`
	CaseCount             int                         `json:"case_count"`
	SimulationProvenCount int                         `json:"simulation_proven_count"`
	ExpectedStressCount   int                         `json:"expected_stress_count"`
	FixtureProxyCount     int                         `json:"fixture_proxy_count"`
	HardwareBlockedCount  int                         `json:"hardware_blocked_count"`
	HardwareBenchReady    bool                        `json:"hardware_bench_ready"`
	UnexpectedCount       int                         `json:"unexpected_count"`
	PlanningReady         bool                        `json:"planning_ready"`
	StationRequired       bool                        `json:"station_required"`
	FirmwareEntryReady    bool                        `json:"firmware_entry_ready"`
	HardwareClaimAllowed  bool                        `json:"hardware_claim_allowed"`
	FieldTXReady          bool                        `json:"field_tx_ready"`
	Outcome               string                      `json:"outcome"`
	Blockers              []string                    `json:"blockers"`
	NextActions           []string                    `json:"next_actions"`
	Rows                  []readinessDashboardCaseRow `json:"rows"`
	Determinism           regressionPackDeterminism   `json:"determinism"`
}

type readinessDashboardCaseRow struct {
	CaseID        string `json:"case_id"`
	Command       string `json:"command"`
	ScenarioID    string `json:"scenario_id"`
	Outcome       string `json:"outcome"`
	Status        string `json:"status"`
	EvidenceClass string `json:"evidence_class"`
	Readiness     string `json:"readiness"`
	Reason        string `json:"reason"`
	EventHash     string `json:"event_hash"`
}

func runReadinessDashboardCommand(args []string) error {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		printReadinessDashboardUsage(os.Stdout)
		return nil
	}
	parsed, err := parseReadinessDashboardArgs(args)
	if err != nil {
		return err
	}
	summary, err := loadReadinessDashboardSummary(parsed.summaryPath)
	if err != nil {
		return err
	}
	report, err := buildReadinessDashboardReport(summary)
	if err != nil {
		return err
	}
	raw, err := marshalReadinessDashboardReportJSON(report)
	if err != nil {
		return fmt.Errorf("marshal readiness dashboard report: %w", err)
	}
	return writeReportFile(parsed.reportPath, raw)
}

func parseReadinessDashboardArgs(args []string) (readinessDashboardArgs, error) {
	var parsed readinessDashboardArgs
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--report":
			if i+1 >= len(args) {
				return readinessDashboardArgs{}, fmt.Errorf("readiness-dashboard: --report requires a path")
			}
			if strings.HasPrefix(args[i+1], "-") {
				return readinessDashboardArgs{}, fmt.Errorf("readiness-dashboard: --report requires a path, got flag-like value %q", args[i+1])
			}
			if parsed.reportPath != "" {
				return readinessDashboardArgs{}, fmt.Errorf("readiness-dashboard: --report specified more than once")
			}
			parsed.reportPath = args[i+1]
			i++
		case strings.HasPrefix(arg, "-"):
			return readinessDashboardArgs{}, fmt.Errorf("readiness-dashboard: unknown flag %q", arg)
		default:
			if parsed.summaryPath != "" {
				return readinessDashboardArgs{}, fmt.Errorf("readiness-dashboard: unexpected argument %q", arg)
			}
			parsed.summaryPath = arg
		}
	}
	if parsed.summaryPath == "" {
		return readinessDashboardArgs{}, fmt.Errorf("readiness-dashboard: regression summary path is required")
	}
	if parsed.reportPath == "" {
		return readinessDashboardArgs{}, fmt.Errorf("readiness-dashboard: --report path is required")
	}
	return parsed, nil
}

func printReadinessDashboardUsage(w *os.File) {
	fmt.Fprintln(w, "usage: adhr-sim readiness-dashboard <regression-summary.json> --report <path>")
}

func loadReadinessDashboardSummary(path string) (regressionPackSummary, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return regressionPackSummary{}, fmt.Errorf("read regression summary %q: %w", path, err)
	}
	var summary regressionPackSummary
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&summary); err != nil {
		return regressionPackSummary{}, fmt.Errorf("decode regression summary %q: %w", path, err)
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return regressionPackSummary{}, fmt.Errorf("decode regression summary %q: trailing JSON token", path)
		}
		return regressionPackSummary{}, fmt.Errorf("decode regression summary %q: %w", path, err)
	}
	if err := validateReadinessDashboardSummary(summary); err != nil {
		return regressionPackSummary{}, fmt.Errorf("regression summary %q: %w", path, err)
	}
	return summary, nil
}

func validateReadinessDashboardSummary(summary regressionPackSummary) error {
	if summary.SchemaVersion != regressionPackSchemaVersion {
		return fmt.Errorf("schema_version=%d unsupported", summary.SchemaVersion)
	}
	if strings.TrimSpace(summary.PackID) == "" {
		return fmt.Errorf("pack_id is required")
	}
	if summary.CaseCount != len(summary.Cases) {
		return fmt.Errorf("case_count=%d does not match cases length %d", summary.CaseCount, len(summary.Cases))
	}
	if summary.Outcome == "" {
		return fmt.Errorf("outcome is required")
	}
	if summary.Determinism.EventHash == "" {
		return fmt.Errorf("determinism.event_hash is required")
	}
	if summary.Determinism.HashAlgorithm != regressionPackHashAlgorithm {
		return fmt.Errorf("determinism.hash_algorithm=%q unsupported", summary.Determinism.HashAlgorithm)
	}
	if summary.Determinism.Serialization != regressionPackSerialization {
		return fmt.Errorf("determinism.serialization=%q unsupported", summary.Determinism.Serialization)
	}
	passCount := 0
	failCount := 0
	for i, row := range summary.Cases {
		if strings.TrimSpace(row.CaseID) == "" {
			return fmt.Errorf("cases[%d].case_id is required", i)
		}
		if strings.TrimSpace(row.Command) == "" {
			return fmt.Errorf("cases[%d].command is required", i)
		}
		if strings.TrimSpace(row.ScenarioID) == "" {
			return fmt.Errorf("cases[%d].scenario_id is required", i)
		}
		if strings.TrimSpace(row.Outcome) == "" {
			return fmt.Errorf("cases[%d].outcome is required", i)
		}
		if strings.TrimSpace(row.ExpectedOutcome) == "" {
			return fmt.Errorf("cases[%d].expected_outcome is required", i)
		}
		if strings.TrimSpace(row.EventHash) == "" {
			return fmt.Errorf("cases[%d].event_hash is required", i)
		}
		switch row.Status {
		case regressionPackCasePass:
			passCount++
		case regressionPackCaseFail:
			failCount++
		default:
			return fmt.Errorf("cases[%d].status=%q unsupported", i, row.Status)
		}
		expectedStatus := regressionPackCaseFail
		if row.Outcome == row.ExpectedOutcome {
			expectedStatus = regressionPackCasePass
		}
		if row.Status != expectedStatus {
			return fmt.Errorf("cases[%d].status=%q does not match outcome/expected_outcome", i, row.Status)
		}
	}
	if summary.ExpectedPassCount != passCount {
		return fmt.Errorf("expected_pass_count=%d does not match pass rows %d", summary.ExpectedPassCount, passCount)
	}
	if summary.UnexpectedCount != failCount {
		return fmt.Errorf("unexpected_count=%d does not match fail rows %d", summary.UnexpectedCount, failCount)
	}
	expectedHash, err := regressionPackSummaryHash(summary)
	if err != nil {
		return err
	}
	if summary.Determinism.EventHash != expectedHash {
		return fmt.Errorf("determinism.event_hash mismatch")
	}
	return nil
}

func buildReadinessDashboardReport(summary regressionPackSummary) (readinessDashboardReport, error) {
	report := readinessDashboardReport{
		SchemaVersion:        readinessDashboardSchemaVersion,
		ScenarioID:           readinessDashboardScenarioID,
		SourcePackID:         summary.PackID,
		SourcePackEventHash:  summary.Determinism.EventHash,
		RegressionOutcome:    summary.Outcome,
		CaseCount:            summary.CaseCount,
		StationRequired:      true,
		HardwareClaimAllowed: false,
		FieldTXReady:         false,
		Rows:                 make([]readinessDashboardCaseRow, 0, len(summary.Cases)),
	}
	for _, row := range summary.Cases {
		dashboardRow := readinessDashboardRow(row)
		report.Rows = append(report.Rows, dashboardRow)
		readinessDashboardAccumulate(&report, dashboardRow)
	}
	report.PlanningReady = summary.Outcome == regressionPackOutcomePass && report.UnexpectedCount == 0
	report.FirmwareEntryReady = report.PlanningReady &&
		report.HardwareBenchReady &&
		report.HardwareBlockedCount == 0
	report.Blockers = readinessDashboardBlockers(report)
	report.NextActions = readinessDashboardNextActions(report)
	report.Outcome = readinessDashboardOutcome(report)
	hash, err := readinessDashboardHash(report)
	if err != nil {
		return readinessDashboardReport{}, err
	}
	report.Determinism = regressionPackDeterminism{
		EventHash:     hash,
		HashAlgorithm: readinessDashboardHashAlgorithm,
		Serialization: readinessDashboardSerialization,
	}
	return report, nil
}

func readinessDashboardRow(row regressionPackCaseSummary) readinessDashboardCaseRow {
	out := readinessDashboardCaseRow{
		CaseID:     row.CaseID,
		Command:    row.Command,
		ScenarioID: row.ScenarioID,
		Outcome:    row.Outcome,
		Status:     row.Status,
		EventHash:  row.EventHash,
	}
	if row.Status != regressionPackCasePass {
		out.EvidenceClass = readinessDashboardClassUnexpected
		out.Readiness = "investigate_regression"
		out.Reason = "actual outcome did not match expected_outcome"
		return out
	}
	switch row.Command {
	case "run":
		if row.Outcome == regressionPackOutcomePass {
			out.EvidenceClass = readinessDashboardClassSimulationProven
			out.Readiness = "ready_for_simulation_planning"
			out.Reason = "checked-in simulator scenario still passes"
		} else {
			out.EvidenceClass = readinessDashboardClassExpectedStress
			out.Readiness = "expected_stress_behavior"
			out.Reason = "checked-in stress scenario still produces its expected non-pass outcome"
		}
	case "ptt-contention":
		out.EvidenceClass = readinessDashboardClassExpectedStress
		out.Readiness = "expected_contention_behavior"
		out.Reason = "PTT contention policy remains explicit and deterministic"
	case "calibrate-channel", "calibrate-tyt", "score-audio":
		out.EvidenceClass = readinessDashboardClassFixtureProxy
		out.Readiness = "requires_real_bench_artifact"
		out.Reason = "fixture/proxy evidence is useful for planning but cannot become a hardware claim"
	case "hardware-readiness":
		readinessDashboardHardwareRow(&out)
	default:
		out.EvidenceClass = readinessDashboardClassUnknown
		out.Readiness = "review_required"
		out.Reason = "dashboard has no classifier for this command"
	}
	return out
}

func readinessDashboardHardwareRow(row *readinessDashboardCaseRow) {
	switch row.Outcome {
	case "hardware_bench_ready":
		row.EvidenceClass = readinessDashboardClassHardwareBench
		row.Readiness = "bench_ready"
		row.Reason = "readiness gate has complete bench evidence, but field TX remains separately gated"
	case "hardware_readiness_blocked":
		row.EvidenceClass = readinessDashboardClassHardwareBlocked
		row.Readiness = "blocked_until_station_bench"
		row.Reason = "fixture-only reports do not satisfy real channel/turnaround/audio evidence gates"
	case "field_tx_blocked":
		row.EvidenceClass = readinessDashboardClassHardwareBlocked
		row.Readiness = "blocked_until_lab_and_regulatory_gate"
		row.Reason = "transmit-capable work requested without lab/regulatory preconditions"
	default:
		row.EvidenceClass = readinessDashboardClassUnknown
		row.Readiness = "review_required"
		row.Reason = "unknown hardware-readiness outcome"
	}
}

func readinessDashboardAccumulate(report *readinessDashboardReport, row readinessDashboardCaseRow) {
	switch row.EvidenceClass {
	case readinessDashboardClassSimulationProven:
		report.SimulationProvenCount++
	case readinessDashboardClassExpectedStress:
		report.ExpectedStressCount++
	case readinessDashboardClassFixtureProxy:
		report.FixtureProxyCount++
	case readinessDashboardClassHardwareBlocked:
		report.HardwareBlockedCount++
	case readinessDashboardClassHardwareBench:
		report.HardwareBenchReady = true
	case readinessDashboardClassUnexpected:
		report.UnexpectedCount++
	case readinessDashboardClassUnknown:
		report.UnexpectedCount++
	}
}

func readinessDashboardBlockers(report readinessDashboardReport) []string {
	if report.UnexpectedCount > 0 {
		return []string{"unexpected_regression_pack_case"}
	}
	if report.HardwareBenchReady && report.HardwareBlockedCount == 0 {
		return nil
	}
	return []string{
		"real_channel_bench_artifact_required",
		"real_tyt_turnaround_measurement_required",
		"real_audio_or_codec_bench_artifact_required",
		"physical_station_required_for_bench_phase",
	}
}

func readinessDashboardNextActions(report readinessDashboardReport) []string {
	if report.UnexpectedCount > 0 {
		return []string{"fix_unexpected_regression_pack_case_before_hardware_planning"}
	}
	if report.HardwareBenchReady && report.HardwareBlockedCount == 0 {
		return []string{
			"prepare_receive_only_or_dummy_load_firmware_experiment",
			"keep_field_tx_blocked_until_separate_go_no_go",
		}
	}
	return []string{
		"obtain_tyt_md_uv380_or_equivalent_bench_station",
		"collect_real_channel_turnaround_and_audio_artifacts",
		"rerun_regression_pack_and_readiness_dashboard_with_real_bench_inputs",
		"keep_transmit_capable_firmware_work_blocked_until_readiness_changes",
	}
}

func readinessDashboardOutcome(report readinessDashboardReport) string {
	if report.UnexpectedCount > 0 || report.RegressionOutcome != regressionPackOutcomePass {
		return readinessDashboardOutcomeUnexpectedRegression
	}
	if report.HardwareBenchReady && report.HardwareBlockedCount == 0 {
		return readinessDashboardOutcomeFirmwareBenchReady
	}
	return readinessDashboardOutcomePlanningReadyStationRequired
}

func readinessDashboardHash(report readinessDashboardReport) (string, error) {
	hashInput := struct {
		SchemaVersion         int                         `json:"schema_version"`
		ScenarioID            string                      `json:"scenario_id"`
		SourcePackID          string                      `json:"source_pack_id"`
		SourcePackEventHash   string                      `json:"source_pack_event_hash"`
		RegressionOutcome     string                      `json:"regression_outcome"`
		CaseCount             int                         `json:"case_count"`
		SimulationProvenCount int                         `json:"simulation_proven_count"`
		ExpectedStressCount   int                         `json:"expected_stress_count"`
		FixtureProxyCount     int                         `json:"fixture_proxy_count"`
		HardwareBlockedCount  int                         `json:"hardware_blocked_count"`
		HardwareBenchReady    bool                        `json:"hardware_bench_ready"`
		UnexpectedCount       int                         `json:"unexpected_count"`
		PlanningReady         bool                        `json:"planning_ready"`
		StationRequired       bool                        `json:"station_required"`
		FirmwareEntryReady    bool                        `json:"firmware_entry_ready"`
		HardwareClaimAllowed  bool                        `json:"hardware_claim_allowed"`
		FieldTXReady          bool                        `json:"field_tx_ready"`
		Outcome               string                      `json:"outcome"`
		Blockers              []string                    `json:"blockers"`
		NextActions           []string                    `json:"next_actions"`
		Rows                  []readinessDashboardCaseRow `json:"rows"`
	}{
		SchemaVersion:         report.SchemaVersion,
		ScenarioID:            report.ScenarioID,
		SourcePackID:          report.SourcePackID,
		SourcePackEventHash:   report.SourcePackEventHash,
		RegressionOutcome:     report.RegressionOutcome,
		CaseCount:             report.CaseCount,
		SimulationProvenCount: report.SimulationProvenCount,
		ExpectedStressCount:   report.ExpectedStressCount,
		FixtureProxyCount:     report.FixtureProxyCount,
		HardwareBlockedCount:  report.HardwareBlockedCount,
		HardwareBenchReady:    report.HardwareBenchReady,
		UnexpectedCount:       report.UnexpectedCount,
		PlanningReady:         report.PlanningReady,
		StationRequired:       report.StationRequired,
		FirmwareEntryReady:    report.FirmwareEntryReady,
		HardwareClaimAllowed:  report.HardwareClaimAllowed,
		FieldTXReady:          report.FieldTXReady,
		Outcome:               report.Outcome,
		Blockers:              report.Blockers,
		NextActions:           report.NextActions,
		Rows:                  report.Rows,
	}
	raw, err := json.Marshal(hashInput)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func marshalReadinessDashboardReportJSON(report readinessDashboardReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}
