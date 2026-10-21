package backendmeasurement

import (
	"fmt"
	"sort"
)

func NearestRank(values []uint64, percentile uint8) (uint64, error) {
	if len(values) == 0 {
		return 0, fmt.Errorf("nearest rank: values are required")
	}
	if percentile == 0 || percentile > 100 {
		return 0, fmt.Errorf("nearest rank: percentile=%d, want 1..100", percentile)
	}
	sorted := append([]uint64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	rank := (uint64(percentile)*uint64(len(sorted)) + 99) / 100
	return sorted[rank-1], nil
}

func SummarizeTrials(report MeasurementReport) (MeasurementSummary, error) {
	summary := MeasurementSummary{FailureReasons: []FailureReasonCount{}}
	durations := make([]uint64, 0, len(report.Trials))
	failures := make(map[string]uint32)

	if report.Definition.Kind == KindChannelCapacity {
		summary.Channel = &ChannelSummary{}
	}
	if report.Definition.Kind == KindTimingAcquisition {
		summary.Timing = &TimingSummary{}
	}

	for _, trial := range report.Trials {
		if trial.Warmup {
			summary.WarmupCount++
			continue
		}
		summary.AttemptedSampleCount++
		if trial.Channel != nil {
			if err := addChannelSummary(summary.Channel, trial.Channel); err != nil {
				return MeasurementSummary{}, err
			}
		}
		if trial.Timing != nil {
			if err := addTimingSummary(summary.Timing, trial.Timing); err != nil {
				return MeasurementSummary{}, err
			}
		}
		if trial.Status == TrialFailed {
			summary.FailedSampleCount++
			failures[trial.FailureReason]++
			continue
		}
		summary.SuccessfulSampleCount++
		durations = append(durations, trial.DurationUS)
	}

	if len(durations) > 0 {
		var err error
		summary.P50US, err = NearestRank(durations, 50)
		if err != nil {
			return MeasurementSummary{}, err
		}
		summary.P95US, err = NearestRank(durations, 95)
		if err != nil {
			return MeasurementSummary{}, err
		}
		summary.MaxUS, err = NearestRank(durations, 100)
		if err != nil {
			return MeasurementSummary{}, err
		}
	}

	reasons := make([]string, 0, len(failures))
	for reason := range failures {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	for _, reason := range reasons {
		summary.FailureReasons = append(summary.FailureReasons, FailureReasonCount{Reason: reason, Count: failures[reason]})
	}
	return summary, nil
}

func addChannelSummary(summary *ChannelSummary, observation *ChannelObservation) error {
	if summary == nil {
		return fmt.Errorf("backend measurement: channel observation without channel summary")
	}
	var ok bool
	if summary.PayloadBytes, ok = addUint64(summary.PayloadBytes, observation.PayloadBytes); !ok {
		return fmt.Errorf("backend measurement: channel payload total overflow")
	}
	if summary.OverheadBytes, ok = addUint64(summary.OverheadBytes, observation.OverheadBytes); !ok {
		return fmt.Errorf("backend measurement: channel overhead total overflow")
	}
	if summary.BitErrors, ok = addUint64(summary.BitErrors, observation.BitErrors); !ok {
		return fmt.Errorf("backend measurement: channel bit-error total overflow")
	}
	if summary.FrameErrors, ok = addUint64(summary.FrameErrors, observation.FrameErrors); !ok {
		return fmt.Errorf("backend measurement: channel frame-error total overflow")
	}
	if summary.FramesLost, ok = addUint64(summary.FramesLost, observation.FramesLost); !ok {
		return fmt.Errorf("backend measurement: channel lost-frame total overflow")
	}
	return nil
}

func addTimingSummary(summary *TimingSummary, observation *TimingObservation) error {
	if summary == nil {
		return fmt.Errorf("backend measurement: timing observation without timing summary")
	}
	summary.MaxAbsBoundaryErrorUS = max(summary.MaxAbsBoundaryErrorUS, absInt64(observation.BoundaryErrorUS))
	summary.MaxAbsDriftMilliPPM = max(summary.MaxAbsDriftMilliPPM, absInt64(observation.DriftMilliPPM))
	summary.MaxGuardUsedUS = max(summary.MaxGuardUsedUS, observation.GuardUsedUS)
	var ok bool
	if summary.ResyncCount, ok = addUint64(summary.ResyncCount, observation.ResyncCount); !ok {
		return fmt.Errorf("backend measurement: timing resync total overflow")
	}
	return nil
}

func addUint64(a, b uint64) (uint64, bool) {
	result := a + b
	return result, result >= a
}

func absInt64(value int64) uint64 {
	if value >= 0 {
		return uint64(value)
	}
	return uint64(-(value + 1)) + 1
}
