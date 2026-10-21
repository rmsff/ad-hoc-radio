package backendmeasurement

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
)

func DecodeReport(reader io.Reader) (MeasurementReport, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var report MeasurementReport
	if err := decoder.Decode(&report); err != nil {
		return MeasurementReport{}, fmt.Errorf("decode backend measurement report: %w", err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return MeasurementReport{}, fmt.Errorf("decode backend measurement report: trailing JSON value")
		}
		return MeasurementReport{}, fmt.Errorf("decode backend measurement report: trailing JSON: %w", err)
	}
	if err := ValidateReport(report); err != nil {
		return MeasurementReport{}, err
	}
	return report, nil
}

func MarshalCanonical(report MeasurementReport) ([]byte, error) {
	if err := ValidateReport(report); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(CloneReport(report))
	if err != nil {
		return nil, fmt.Errorf("marshal backend measurement report: %w", err)
	}
	return raw, nil
}

func ReportHash(report MeasurementReport) (string, error) {
	raw, err := MarshalCanonical(report)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func CloneReport(in MeasurementReport) MeasurementReport {
	out := in
	out.TrialPlan.MatrixCases = make([]MatrixCase, len(in.TrialPlan.MatrixCases))
	for i, matrixCase := range in.TrialPlan.MatrixCases {
		out.TrialPlan.MatrixCases[i] = matrixCase
		if matrixCase.Channel != nil {
			channel := *matrixCase.Channel
			out.TrialPlan.MatrixCases[i].Channel = &channel
		}
	}
	if in.Thresholds != nil {
		thresholds := *in.Thresholds
		if in.Thresholds.Channel != nil {
			channel := *in.Thresholds.Channel
			thresholds.Channel = &channel
		}
		if in.Thresholds.Timing != nil {
			timing := *in.Thresholds.Timing
			thresholds.Timing = &timing
		}
		out.Thresholds = &thresholds
	}
	out.RawArtifacts = append([]ArtifactRef{}, in.RawArtifacts...)
	out.Trials = make([]Trial, len(in.Trials))
	for i, trial := range in.Trials {
		out.Trials[i] = trial
		if trial.Channel != nil {
			channel := *trial.Channel
			out.Trials[i].Channel = &channel
		}
		if trial.Timing != nil {
			timing := *trial.Timing
			out.Trials[i].Timing = &timing
		}
	}
	out.Summary.FailureReasons = append([]FailureReasonCount{}, in.Summary.FailureReasons...)
	if in.Summary.Channel != nil {
		channel := *in.Summary.Channel
		out.Summary.Channel = &channel
	}
	if in.Summary.Timing != nil {
		timing := *in.Summary.Timing
		out.Summary.Timing = &timing
	}
	out.NoPassReasons = append([]string{}, in.NoPassReasons...)
	return out
}
