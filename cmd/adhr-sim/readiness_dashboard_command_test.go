package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIT049_ReadinessDashboardCommandWritesReport(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "regression")
	if err := run([]string{"regression-pack", regressionPackConfigPath(t), "--out", outDir}); err != nil {
		t.Fatalf("regression-pack command failed: %v", err)
	}
	reportPath := filepath.Join(t.TempDir(), "readiness-dashboard.json")

	if err := run([]string{
		"readiness-dashboard",
		filepath.Join(outDir, "summary.json"),
		"--report", reportPath,
	}); err != nil {
		t.Fatalf("readiness-dashboard command failed: %v", err)
	}

	report := readReadinessDashboardReport(t, reportPath)
	if report.ScenarioID != readinessDashboardScenarioID {
		t.Fatalf("scenario_id = %q, want %q", report.ScenarioID, readinessDashboardScenarioID)
	}
	if report.Outcome != readinessDashboardOutcomePlanningReadyStationRequired {
		t.Fatalf("outcome = %q, want %q; blockers=%v", report.Outcome, readinessDashboardOutcomePlanningReadyStationRequired, report.Blockers)
	}
	if report.CaseCount != 17 ||
		report.SimulationProvenCount != 10 ||
		report.ExpectedStressCount != 3 ||
		report.FixtureProxyCount != 3 ||
		report.HardwareBlockedCount != 1 ||
		report.UnexpectedCount != 0 {
		t.Fatalf("counts = case:%d sim:%d stress:%d fixture:%d blocked:%d unexpected:%d",
			report.CaseCount,
			report.SimulationProvenCount,
			report.ExpectedStressCount,
			report.FixtureProxyCount,
			report.HardwareBlockedCount,
			report.UnexpectedCount)
	}
	if !report.PlanningReady || !report.StationRequired || report.FirmwareEntryReady {
		t.Fatalf("readiness flags planning/station/firmware = %v/%v/%v, want true/true/false",
			report.PlanningReady, report.StationRequired, report.FirmwareEntryReady)
	}
	if report.HardwareClaimAllowed || report.FieldTXReady {
		t.Fatalf("claim flags hardware/field = %v/%v, want false/false", report.HardwareClaimAllowed, report.FieldTXReady)
	}
	if report.SourcePackEventHash != "67dcf00f8d52398d0b79cc3dc33c4f14ca9ac89dcfd0a9f7a8db40ce90868376" {
		t.Fatalf("source pack hash = %q", report.SourcePackEventHash)
	}
	if report.Determinism.EventHash != "ac9103409ae5afbb86dd6fae1b11438361a144d9c6831fa217a44d15b9a32adb" {
		t.Fatalf("dashboard hash = %q", report.Determinism.EventHash)
	}
	hardware := readinessDashboardRowsByCaseID(report.Rows)["hardware_readiness_fixture"]
	if hardware.EvidenceClass != readinessDashboardClassHardwareBlocked ||
		hardware.Readiness != "blocked_until_station_bench" ||
		hardware.Outcome != "hardware_readiness_blocked" {
		t.Fatalf("hardware row = %+v", hardware)
	}
	if !stringSliceContains(report.Blockers, "physical_station_required_for_bench_phase") {
		t.Fatalf("blockers = %v, want physical station blocker", report.Blockers)
	}
}

func TestIT049_ReadinessDashboardCommandDeterministicReport(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "regression")
	if err := run([]string{"regression-pack", regressionPackConfigPath(t), "--out", outDir}); err != nil {
		t.Fatalf("regression-pack command failed: %v", err)
	}

	dir := t.TempDir()
	first := filepath.Join(dir, "first.json")
	second := filepath.Join(dir, "second.json")
	args := []string{"readiness-dashboard", filepath.Join(outDir, "summary.json"), "--report"}
	if err := run(append(args, first)); err != nil {
		t.Fatalf("first readiness-dashboard failed: %v", err)
	}
	if err := run(append(args, second)); err != nil {
		t.Fatalf("second readiness-dashboard failed: %v", err)
	}
	firstRaw, err := os.ReadFile(first)
	if err != nil {
		t.Fatalf("read first dashboard failed: %v", err)
	}
	secondRaw, err := os.ReadFile(second)
	if err != nil {
		t.Fatalf("read second dashboard failed: %v", err)
	}
	if string(firstRaw) != string(secondRaw) {
		t.Fatalf("readiness dashboard reports differ:\nfirst=%s\nsecond=%s", firstRaw, secondRaw)
	}
}

func TestReadinessDashboard_BlockedHardwareRowPreventsFirmwareReady(t *testing.T) {
	summary := readinessDashboardSummaryFromRows(t, []regressionPackCaseSummary{
		{
			CaseID:          "hardware_readiness_fixture",
			Command:         "hardware-readiness",
			Output:          "reports/hardware-readiness-fixture.json",
			ScenarioID:      "portable_hardware_readiness_gate",
			Outcome:         "hardware_readiness_blocked",
			ExpectedOutcome: "hardware_readiness_blocked",
			Status:          regressionPackCasePass,
			EventHash:       "blocked_hash",
		},
		{
			CaseID:          "hardware_readiness_bench",
			Command:         "hardware-readiness",
			Output:          "reports/hardware-readiness-bench.json",
			ScenarioID:      "portable_hardware_readiness_gate",
			Outcome:         "hardware_bench_ready",
			ExpectedOutcome: "hardware_bench_ready",
			Status:          regressionPackCasePass,
			EventHash:       "bench_hash",
		},
	})

	report, err := buildReadinessDashboardReport(summary)
	if err != nil {
		t.Fatalf("build readiness dashboard failed: %v", err)
	}
	if report.FirmwareEntryReady || report.Outcome == readinessDashboardOutcomeFirmwareBenchReady {
		t.Fatalf("firmware readiness = %v outcome=%q, want blocked by fixture row", report.FirmwareEntryReady, report.Outcome)
	}
	if report.HardwareBlockedCount != 1 || !report.HardwareBenchReady {
		t.Fatalf("hardware blocked/bench = %d/%v, want 1/true", report.HardwareBlockedCount, report.HardwareBenchReady)
	}
}

func TestReadinessDashboardCommandRejectsInvalidArgsAndSummary(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing summary", args: []string{"readiness-dashboard"}, wantErr: "summary path"},
		{name: "missing report", args: []string{"readiness-dashboard", "summary.json"}, wantErr: "--report"},
		{name: "flag like report", args: []string{"readiness-dashboard", "summary.json", "--report", "--bad"}, wantErr: "flag-like"},
		{name: "duplicate report", args: []string{"readiness-dashboard", "summary.json", "--report", "a.json", "--report", "b.json"}, wantErr: "more than once"},
		{name: "unknown flag", args: []string{"readiness-dashboard", "--bad"}, wantErr: "unknown flag"},
		{name: "unexpected arg", args: []string{"readiness-dashboard", "a.json", "b.json", "--report", "out.json"}, wantErr: "unexpected argument"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := run(tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
		})
	}

	invalidSummaries := []struct {
		name    string
		body    string
		wantErr string
	}{
		{name: "unknown field", body: `{"schema_version":1,"unexpected":true}`, wantErr: "unknown field"},
		{name: "trailing token", body: readinessDashboardValidSummaryJSON(t) + `{}`, wantErr: "trailing JSON token"},
		{name: "case count mismatch", body: strings.Replace(readinessDashboardValidSummaryJSON(t), `"case_count":1`, `"case_count":2`, 1), wantErr: "does not match"},
		{name: "missing event hash", body: strings.Replace(readinessDashboardValidSummaryJSON(t), `"event_hash":"case_hash"`, `"event_hash":""`, 1), wantErr: "event_hash is required"},
		{name: "summary hash mismatch", body: readinessDashboardSummaryWithHash(t, "bad_hash"), wantErr: "event_hash mismatch"},
		{name: "status mismatch", body: readinessDashboardStatusMismatchSummaryJSON(t), wantErr: "does not match outcome"},
	}
	for _, tc := range invalidSummaries {
		t.Run(tc.name, func(t *testing.T) {
			summaryPath := filepath.Join(t.TempDir(), "summary.json")
			reportPath := filepath.Join(t.TempDir(), "dashboard.json")
			if err := os.WriteFile(summaryPath, []byte(tc.body), 0o644); err != nil {
				t.Fatalf("write summary failed: %v", err)
			}
			err := run([]string{"readiness-dashboard", summaryPath, "--report", reportPath})
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
			assertFileMissing(t, reportPath)
		})
	}
}

func readReadinessDashboardReport(t *testing.T, path string) readinessDashboardReport {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read readiness dashboard failed: %v", err)
	}
	var report readinessDashboardReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("unmarshal readiness dashboard failed: %v\n%s", err, raw)
	}
	return report
}

func readinessDashboardRowsByCaseID(rows []readinessDashboardCaseRow) map[string]readinessDashboardCaseRow {
	out := make(map[string]readinessDashboardCaseRow, len(rows))
	for _, row := range rows {
		out[row.CaseID] = row
	}
	return out
}

func readinessDashboardValidSummaryJSON(t *testing.T) string {
	t.Helper()
	summary := readinessDashboardSummaryFromRows(t, []regressionPackCaseSummary{
		{
			CaseID:          "one_hop_nominal",
			Command:         "run",
			Input:           "../scenarios/one-hop.json",
			Output:          "metrics/one-hop.json",
			ScenarioID:      "one_hop_nominal",
			Outcome:         regressionPackOutcomePass,
			ExpectedOutcome: regressionPackOutcomePass,
			Status:          regressionPackCasePass,
			EventHash:       "case_hash",
		},
	})
	raw, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal summary fixture failed: %v", err)
	}
	return string(raw)
}

func readinessDashboardSummaryFromRows(t *testing.T, rows []regressionPackCaseSummary) regressionPackSummary {
	t.Helper()
	summary := regressionPackSummary{
		SchemaVersion: regressionPackSchemaVersion,
		PackID:        "test_pack",
		Seed:          42,
		CaseCount:     len(rows),
		Cases:         rows,
		Determinism: regressionPackDeterminism{
			HashAlgorithm: regressionPackHashAlgorithm,
			Serialization: regressionPackSerialization,
		},
		Outcome: regressionPackOutcomePass,
	}
	for _, row := range rows {
		if row.Status == regressionPackCasePass {
			summary.ExpectedPassCount++
		} else {
			summary.UnexpectedCount++
			summary.Outcome = regressionPackOutcomeFail
		}
	}
	hash, err := regressionPackSummaryHash(summary)
	if err != nil {
		t.Fatalf("hash summary fixture failed: %v", err)
	}
	summary.Determinism.EventHash = hash
	return summary
}

func readinessDashboardSummaryWithHash(t *testing.T, eventHash string) string {
	t.Helper()
	var summary regressionPackSummary
	if err := json.Unmarshal([]byte(readinessDashboardValidSummaryJSON(t)), &summary); err != nil {
		t.Fatalf("unmarshal summary fixture failed: %v", err)
	}
	summary.Determinism.EventHash = eventHash
	raw, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal summary fixture failed: %v", err)
	}
	return string(raw)
}

func readinessDashboardStatusMismatchSummaryJSON(t *testing.T) string {
	t.Helper()
	var summary regressionPackSummary
	if err := json.Unmarshal([]byte(readinessDashboardValidSummaryJSON(t)), &summary); err != nil {
		t.Fatalf("unmarshal summary fixture failed: %v", err)
	}
	summary.Cases[0].Outcome = regressionPackOutcomeFail
	hash, err := regressionPackSummaryHash(summary)
	if err != nil {
		t.Fatalf("hash status mismatch fixture failed: %v", err)
	}
	summary.Determinism.EventHash = hash
	raw, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal status mismatch fixture failed: %v", err)
	}
	return string(raw)
}
