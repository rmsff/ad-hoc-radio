package radiobackend

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
)

func DecodeSelectionReport(reader io.Reader) (SelectionReport, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var report SelectionReport
	if err := decoder.Decode(&report); err != nil {
		return SelectionReport{}, fmt.Errorf("decode backend selection report: %w", err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return SelectionReport{}, fmt.Errorf("decode backend selection report: trailing JSON value")
		}
		return SelectionReport{}, fmt.Errorf("decode backend selection report: trailing JSON: %w", err)
	}
	if err := ValidateSelectionReport(report); err != nil {
		return SelectionReport{}, err
	}
	return report, nil
}

func MarshalSelectionCanonical(report SelectionReport) ([]byte, error) {
	if err := ValidateSelectionReport(report); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(CloneSelectionReport(report))
	if err != nil {
		return nil, fmt.Errorf("marshal backend selection report: %w", err)
	}
	return raw, nil
}

func SelectionReportHash(report SelectionReport) (string, error) {
	raw, err := MarshalSelectionCanonical(report)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func CloneSelectionReport(in SelectionReport) SelectionReport {
	out := in
	out.Candidates = make([]SelectionCandidate, len(in.Candidates))
	for i, candidate := range in.Candidates {
		out.Candidates[i] = candidate
		out.Candidates[i].Checks = make([]SelectionCheck, len(candidate.Checks))
		for j, check := range candidate.Checks {
			out.Candidates[i].Checks[j] = check
			out.Candidates[i].Checks[j].ArtifactRefs = append([]ArtifactRef(nil), check.ArtifactRefs...)
		}
	}
	return out
}
