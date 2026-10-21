package backendmeasurement

import "sort"

func evaluateNoPassReasons(report MeasurementReport) []string {
	reasons := make([]string, 0, 10)
	if report.Clock.Status != ClockKnown {
		reasons = append(reasons, ReasonUnknownClock)
	}
	if report.Thresholds == nil {
		reasons = append(reasons, ReasonMissingThresholds)
	}
	if len(report.RawArtifacts) == 0 {
		reasons = append(reasons, ReasonMissingRawArtifacts)
	}
	if report.EvidenceGrade != EvidenceBenchMeasured {
		reasons = append(reasons, ReasonNotBenchMeasured)
	}
	if report.Summary.AttemptedSampleCount < report.TrialPlan.MinimumSampleCount {
		reasons = append(reasons, ReasonInsufficientSamples)
	}
	if report.Summary.SuccessfulSampleCount == 0 {
		reasons = append(reasons, ReasonNoSuccessfulSamples)
	}
	if !matrixCoverageComplete(report) {
		reasons = append(reasons, ReasonMatrixCoverageIncomplete)
	}
	if report.Definition.Kind == KindChannelCapacity && !channelCapacitySufficient(report.TrialPlan.MatrixCases) {
		reasons = append(reasons, ReasonChannelCapacityInsufficient)
	}
	if report.Thresholds != nil {
		if report.Summary.FailedSampleCount > report.Thresholds.Duration.MaxFailedSamples {
			reasons = append(reasons, ReasonFailedSamplesExceeded)
		}
		if report.Summary.P95US > report.Thresholds.Duration.P95MaxUS || report.Summary.MaxUS > report.Thresholds.Duration.MaxUS {
			reasons = append(reasons, ReasonDurationThresholdExceeded)
		}
		if report.Summary.Channel != nil && channelExceeded(*report.Summary.Channel, report.Thresholds.Channel) {
			reasons = append(reasons, ReasonChannelThresholdExceeded)
		}
		if report.Summary.Timing != nil && timingExceeded(*report.Summary.Timing, report.Thresholds.Timing) {
			reasons = append(reasons, ReasonTimingThresholdExceeded)
		}
	}
	sort.Strings(reasons)
	return reasons
}

func prerequisitesComplete(report MeasurementReport) bool {
	return report.Clock.Status == ClockKnown && report.Thresholds != nil && len(report.RawArtifacts) > 0 && report.EvidenceGrade == EvidenceBenchMeasured && report.Summary.AttemptedSampleCount >= report.TrialPlan.MinimumSampleCount && report.Summary.SuccessfulSampleCount > 0 && matrixCoverageComplete(report)
}

func matrixCoverageComplete(report MeasurementReport) bool {
	type counts struct {
		attempted uint32
		succeeded uint32
	}
	byCase := make(map[string]counts, len(report.TrialPlan.MatrixCases))
	for _, trial := range report.Trials {
		if trial.Warmup {
			continue
		}
		current := byCase[trial.CaseID]
		current.attempted++
		if trial.Status == TrialSucceeded {
			current.succeeded++
		}
		byCase[trial.CaseID] = current
	}
	for _, matrixCase := range report.TrialPlan.MatrixCases {
		current := byCase[matrixCase.CaseID]
		if current.attempted < report.TrialPlan.MinimumSamplesPerCase || current.succeeded == 0 {
			return false
		}
	}
	return true
}

func channelCapacitySufficient(cases []MatrixCase) bool {
	for _, matrixCase := range cases {
		if matrixCase.Channel == nil {
			return false
		}
		tested, testedOK := addUint64(matrixCase.Channel.PayloadBytes, matrixCase.Channel.OverheadBytes)
		required, requiredOK := addUint64(matrixCase.Channel.RequiredFrameBytes, matrixCase.Channel.GuardMarginBytes)
		if !testedOK || !requiredOK || tested < required {
			return false
		}
	}
	return true
}

func channelExceeded(summary ChannelSummary, threshold *ChannelThreshold) bool {
	return threshold == nil || summary.BitErrors > threshold.MaxBitErrors || summary.FrameErrors > threshold.MaxFrameErrors || summary.FramesLost > threshold.MaxFramesLost
}

func timingExceeded(summary TimingSummary, threshold *TimingThreshold) bool {
	return threshold == nil || summary.MaxAbsBoundaryErrorUS > threshold.MaxAbsBoundaryErrorUS || summary.MaxAbsDriftMilliPPM > threshold.MaxAbsDriftMilliPPM || summary.MaxGuardUsedUS > threshold.MaxGuardUsedUS || summary.ResyncCount > threshold.MaxResyncCount
}
