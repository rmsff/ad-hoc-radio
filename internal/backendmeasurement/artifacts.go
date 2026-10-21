package backendmeasurement

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
)

func ValidateRawArtifactContents(report MeasurementReport, root string) error {
	if err := ValidateReport(report); err != nil {
		return err
	}
	if root == "" {
		return fmt.Errorf("backend measurement: artifact root is required")
	}
	for _, artifact := range report.RawArtifacts {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(artifact.Path)))
		if err != nil {
			return fmt.Errorf("backend measurement: read raw artifact %q: %w", artifact.Path, err)
		}
		sum := sha256.Sum256(raw)
		if got := hex.EncodeToString(sum[:]); got != artifact.SHA256 {
			return fmt.Errorf("backend measurement: raw artifact %q SHA-256=%q, want %q", artifact.Path, got, artifact.SHA256)
		}
	}
	return nil
}

func ValidateGatePassEvidence(report MeasurementReport, root string) error {
	if report.Definition.Kind == KindRXToTXTurnaround || report.Definition.Kind == KindTXToRXTurnaround {
		return fmt.Errorf("backend measurement: tx_rx_turnaround gate requires both directional reports")
	}
	return validateSingleGatePassEvidence(report, root)
}

func ValidateTurnaroundGatePassEvidence(rxToTX, txToRX MeasurementReport, root string) error {
	if rxToTX.Definition.Kind != KindRXToTXTurnaround || txToRX.Definition.Kind != KindTXToRXTurnaround {
		return fmt.Errorf("backend measurement: turnaround pair requires RX-to-TX then TX-to-RX reports")
	}
	if rxToTX.Candidate != txToRX.Candidate || rxToTX.Clock != txToRX.Clock || rxToTX.Units != txToRX.Units {
		return fmt.Errorf("backend measurement: turnaround reports must share candidate, clock, and units")
	}
	if rxToTX.Thresholds == nil || txToRX.Thresholds == nil || !reflect.DeepEqual(rxToTX.Thresholds, txToRX.Thresholds) {
		return fmt.Errorf("backend measurement: turnaround reports must share thresholds")
	}
	if err := validateSingleGatePassEvidence(rxToTX, root); err != nil {
		return fmt.Errorf("backend measurement: RX-to-TX evidence: %w", err)
	}
	if err := validateSingleGatePassEvidence(txToRX, root); err != nil {
		return fmt.Errorf("backend measurement: TX-to-RX evidence: %w", err)
	}
	return nil
}

func validateSingleGatePassEvidence(report MeasurementReport, root string) error {
	if report.Verdict != VerdictPassed || !report.Safety.GatePassClaim {
		return fmt.Errorf("backend measurement: report is not a passing gate artifact")
	}
	return ValidateRawArtifactContents(report, root)
}
