package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

func validateChannelReadinessReport(report scenario.ChannelBenchArtifactReport) error {
	if err := scenario.ValidateScenarioEvidence(report.Evidence); err != nil {
		return fmt.Errorf("hardware-readiness: invalid channel evidence: %w", err)
	}
	if report.FramesSent <= 0 || report.FramesDecoded < 0 ||
		report.CRCErrorCount < 0 || report.FramesDecoded > report.FramesSent ||
		report.CRCErrorCount > report.FramesSent {
		return fmt.Errorf("hardware-readiness: inconsistent channel frame counters")
	}
	expectedSuccess := permilleForReadiness(report.FramesDecoded, report.FramesSent)
	expectedCRC := permilleForReadiness(report.CRCErrorCount, report.FramesSent)
	if report.DecodeSuccessPermille != expectedSuccess || report.CRCErrorPermille != expectedCRC {
		return fmt.Errorf("hardware-readiness: inconsistent channel derived counters")
	}
	if report.Evidence.StoryID != scenario.ChannelBenchArtifactStoryID ||
		report.Evidence.ScenarioID != report.ScenarioID ||
		report.Evidence.EvidenceType != scenario.ChannelBenchArtifactEvidenceType ||
		report.Evidence.Outcome != report.Outcome ||
		report.Evidence.HardwareClaimAllowed != report.HardwareClaimAllowed {
		return fmt.Errorf("hardware-readiness: inconsistent channel evidence envelope")
	}
	expectedHardwareClaim := report.MeasurementSource == scenario.ChannelBenchMeasurementSourceBenchMeasured &&
		!report.TestFixture &&
		strings.TrimSpace(report.MeasurementArtifactRef) != "" &&
		isLowerHexSHA256Digest(report.MeasurementArtifactSHA256)
	if report.MeasuredHardwareCapability != expectedHardwareClaim ||
		report.HardwareClaimAllowed != expectedHardwareClaim {
		return fmt.Errorf("hardware-readiness: inconsistent channel hardware claim flags")
	}
	expectedVerified := expectedHardwareClaim && expectedSuccess >= 990 && expectedCRC <= 10
	if report.ChannelBenchVerified != expectedVerified {
		return fmt.Errorf("hardware-readiness: inconsistent channel verification flag")
	}
	expectedOutcome := expectedChannelReadinessOutcome(report.MeasurementSource, expectedVerified)
	if report.Outcome != expectedOutcome {
		return fmt.Errorf("hardware-readiness: inconsistent channel outcome")
	}
	switch report.MeasurementSource {
	case scenario.ChannelBenchMeasurementSourceFixtureOnly:
		if !report.TestFixture || report.ChannelBenchVerified ||
			report.MeasuredHardwareCapability || report.HardwareClaimAllowed ||
			report.Outcome != scenario.ChannelBenchOutcomeFixtureOnly {
			return fmt.Errorf("hardware-readiness: inconsistent channel fixture-only report")
		}
	case scenario.ChannelBenchMeasurementSourceBenchMeasured:
		if report.TestFixture || strings.TrimSpace(report.MeasurementArtifactRef) == "" ||
			!isLowerHexSHA256Digest(report.MeasurementArtifactSHA256) ||
			!report.MeasuredHardwareCapability || !report.HardwareClaimAllowed {
			return fmt.Errorf("hardware-readiness: inconsistent channel bench-measured report")
		}
		if report.ChannelBenchVerified && report.Outcome != scenario.ChannelBenchOutcomeVerified {
			return fmt.Errorf("hardware-readiness: inconsistent channel verified outcome")
		}
		if !report.ChannelBenchVerified && report.Outcome == scenario.ChannelBenchOutcomeVerified {
			return fmt.Errorf("hardware-readiness: inconsistent channel failed outcome")
		}
	case scenario.ChannelBenchMeasurementSourceUnknown:
		if report.ChannelBenchVerified || report.MeasuredHardwareCapability || report.HardwareClaimAllowed {
			return fmt.Errorf("hardware-readiness: inconsistent channel unknown-source report")
		}
	default:
		return fmt.Errorf("hardware-readiness: unsupported channel measurement_source %q", report.MeasurementSource)
	}
	return nil
}

func validateTurnaroundReadinessReport(report scenario.TYTBenchMeasurementReport) error {
	if err := validateTYTReadinessMetric(report.RxToTx, "rx_to_tx"); err != nil {
		return err
	}
	if err := validateTYTReadinessMetric(report.TxToRx, "tx_to_rx"); err != nil {
		return err
	}
	if err := validateTYTReadinessMetric(report.RxValidAfterTx, "rx_valid_after_tx"); err != nil {
		return err
	}
	expectedMax, expectedLimiting := expectedTYTReadinessMaxAndLimiting(report)
	if report.MaxRequiredTurnaroundP95Ms != expectedMax {
		return fmt.Errorf("hardware-readiness: inconsistent TYT turnaround max p95")
	}
	if !sameStringSet(report.LimitingComponents, expectedLimiting) {
		return fmt.Errorf("hardware-readiness: inconsistent TYT turnaround limiting components")
	}
	expectedDecision, expectedSlot, expectedThreshold := expectedTYTReadinessDecision(expectedMax)
	if report.CalibrationDecision != expectedDecision ||
		report.SelectedTupleSlotMs != expectedSlot ||
		report.TupleThresholdMs != expectedThreshold {
		return fmt.Errorf("hardware-readiness: inconsistent TYT turnaround calibration decision")
	}
	expectedHardwareClaim := report.MeasurementSource == scenario.TYTBenchMeasurementSourceBenchMeasured &&
		!report.TestFixture &&
		strings.TrimSpace(report.MeasurementArtifactRef) != ""
	if report.AssumptionOnly != !expectedHardwareClaim ||
		report.MeasuredHardwareCapability != expectedHardwareClaim ||
		report.HardwareClaimAllowed != expectedHardwareClaim {
		return fmt.Errorf("hardware-readiness: inconsistent TYT hardware claim flags")
	}
	switch report.MeasurementSource {
	case scenario.TYTBenchMeasurementSourceFixtureOnly:
		if !report.TestFixture || !report.AssumptionOnly ||
			report.MeasuredHardwareCapability || report.HardwareClaimAllowed {
			return fmt.Errorf("hardware-readiness: inconsistent TYT fixture-only report")
		}
	case scenario.TYTBenchMeasurementSourceBenchMeasured:
		if report.TestFixture || strings.TrimSpace(report.MeasurementArtifactRef) == "" ||
			report.AssumptionOnly || !report.MeasuredHardwareCapability || !report.HardwareClaimAllowed {
			return fmt.Errorf("hardware-readiness: inconsistent TYT bench-measured report")
		}
	case scenario.TYTBenchMeasurementSourceUnknown:
		if report.MeasuredHardwareCapability || report.HardwareClaimAllowed {
			return fmt.Errorf("hardware-readiness: inconsistent TYT unknown-source report")
		}
	default:
		return fmt.Errorf("hardware-readiness: unsupported TYT measurement_source %q", report.MeasurementSource)
	}
	return nil
}

func validateTYTReadinessMetric(metric scenario.TYTBenchMetricSummary, name string) error {
	if metric.Name != name {
		return fmt.Errorf("hardware-readiness: inconsistent TYT metric name")
	}
	if metric.P50Ms < 0 || metric.P95Ms < 0 || metric.P95Ms < metric.P50Ms {
		return fmt.Errorf("hardware-readiness: inconsistent TYT metric percentiles")
	}
	switch metric.SummarySource {
	case scenario.TYTBenchSummarySourceRawSamples:
		if metric.SampleCount <= 0 {
			return fmt.Errorf("hardware-readiness: inconsistent TYT raw metric sample count")
		}
	case scenario.TYTBenchSummarySourcePrecomputed:
		if metric.SampleCount != 0 {
			return fmt.Errorf("hardware-readiness: inconsistent TYT precomputed metric sample count")
		}
	default:
		return fmt.Errorf("hardware-readiness: unsupported TYT metric summary_source %q", metric.SummarySource)
	}
	return nil
}

func validateAudioReadinessReport(report scenario.AudioIntelligibilityReport) error {
	if err := scenario.ValidateScenarioEvidence(report.Evidence); err != nil {
		return fmt.Errorf("hardware-readiness: invalid audio evidence: %w", err)
	}
	if report.Evidence.StoryID != scenario.AudioIntelligibilityStoryID ||
		report.Evidence.ScenarioID != report.ScenarioID ||
		report.Evidence.EvidenceType != scenario.AudioIntelligibilityEvidenceType ||
		report.Evidence.Outcome == "" ||
		report.Evidence.HardwareClaimAllowed {
		return fmt.Errorf("hardware-readiness: inconsistent audio evidence envelope")
	}
	if report.RowCount != len(report.Rows) {
		return fmt.Errorf("hardware-readiness: inconsistent audio row_count")
	}
	var goodRows int
	for _, row := range report.Rows {
		if row.Outcome != scenario.AudioIntelligibilityOutcomeGoodVoicePass {
			if audioRowHasStrongEvidence(row) && row.IntelligibilityProxyScore >= row.GoodVoiceThreshold {
				return fmt.Errorf("hardware-readiness: inconsistent audio degraded row above threshold")
			}
			continue
		}
		goodRows++
		if !audioRowHasStrongEvidence(row) ||
			!isLowerHexSHA256Digest(row.ArtifactSHA256) ||
			row.IntelligibilityProxyScore < row.GoodVoiceThreshold {
			return fmt.Errorf("hardware-readiness: inconsistent audio good_voice_pass row")
		}
	}
	if goodRows != report.GoodVoicePassRows {
		return fmt.Errorf("hardware-readiness: inconsistent audio good_voice_pass row count")
	}
	return nil
}

func audioRowHasStrongEvidence(row scenario.AudioIntelligibilityRow) bool {
	switch row.EvidenceGrade {
	case scenario.EvidenceGradeFixtureArtifact, scenario.EvidenceGradeBenchMeasured:
		return row.ArtifactSHA256 != ""
	default:
		return false
	}
}

func expectedTYTReadinessDecision(maxP95Ms int64) (string, int64, int64) {
	switch {
	case maxP95Ms <= 2:
		return scenario.TYTBenchDecisionAggressive16ms, 16, 2
	case maxP95Ms <= 6:
		return scenario.TYTBenchDecisionIntermediate20ms, 20, 6
	case maxP95Ms <= 10:
		return scenario.TYTBenchDecisionPrimary24ms, 24, 10
	case maxP95Ms <= 16:
		return scenario.TYTBenchDecisionSafe30ms, 30, 16
	default:
		return scenario.TYTBenchDecisionNoLiveTwoHop, 0, 0
	}
}

func expectedTYTReadinessMaxAndLimiting(report scenario.TYTBenchMeasurementReport) (int64, []string) {
	metrics := []scenario.TYTBenchMetricSummary{report.RxToTx, report.TxToRx, report.RxValidAfterTx}
	maxP95 := metrics[0].P95Ms
	for _, metric := range metrics[1:] {
		if metric.P95Ms > maxP95 {
			maxP95 = metric.P95Ms
		}
	}
	limiting := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		if metric.P95Ms == maxP95 {
			limiting = append(limiting, metric.Name)
		}
	}
	sort.Strings(limiting)
	return maxP95, limiting
}

func sameStringSet(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	left := append([]string(nil), a...)
	right := append([]string(nil), b...)
	sort.Strings(left)
	sort.Strings(right)
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func expectedChannelReadinessOutcome(source string, verified bool) string {
	if verified {
		return scenario.ChannelBenchOutcomeVerified
	}
	if source == scenario.ChannelBenchMeasurementSourceFixtureOnly {
		return scenario.ChannelBenchOutcomeFixtureOnly
	}
	return scenario.ChannelBenchOutcomeFail
}

func permilleForReadiness(numerator int64, denominator int64) int64 {
	if denominator == 0 {
		return 0
	}
	return numerator * 1000 / denominator
}

func isLowerHexSHA256Digest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
