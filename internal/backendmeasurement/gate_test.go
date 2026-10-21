package backendmeasurement

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestTurnaroundGateRequiresBothDirectionsAndRawEvidence(t *testing.T) {
	root := t.TempDir()
	rxRaw := writeRawFixture(t, root, "raw/rx-to-tx.ndjson", []byte("rx-to-tx\n"))
	txRaw := writeRawFixture(t, root, "raw/tx-to-rx.ndjson", []byte("tx-to-rx\n"))

	rx := turnaroundReport()
	rx.RawArtifacts = []ArtifactRef{{Path: "raw/rx-to-tx.ndjson", SHA256: rawSHA256(rxRaw)}}
	rxFinal, err := FinalizeReport(rx)
	if err != nil {
		t.Fatal(err)
	}
	tx := turnaroundReport()
	tx.Definition.Kind = KindTXToRXTurnaround
	tx.Definition.StartMarker = EndpointLastValidTXMarker
	tx.Definition.EndMarker = EndpointFirstValidRXMarker
	tx.RawArtifacts = []ArtifactRef{{Path: "raw/tx-to-rx.ndjson", SHA256: rawSHA256(txRaw)}}
	txFinal, err := FinalizeReport(tx)
	if err != nil {
		t.Fatal(err)
	}

	if err := ValidateGatePassEvidence(rxFinal, root); err == nil || !strings.Contains(err.Error(), "both directional") {
		t.Fatalf("single turnaround report error = %v", err)
	}
	if err := ValidateTurnaroundGatePassEvidence(rxFinal, txFinal, root); err != nil {
		t.Fatalf("ValidateTurnaroundGatePassEvidence() error = %v", err)
	}

	stale := CloneReport(txFinal)
	stale.RawArtifacts[0].SHA256 = strings.Repeat("d", 64)
	if err := ValidateTurnaroundGatePassEvidence(rxFinal, stale, root); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("stale pair artifact error = %v", err)
	}
	if err := os.Remove(filepath.Join(root, "raw", "tx-to-rx.ndjson")); err != nil {
		t.Fatal(err)
	}
	if err := ValidateTurnaroundGatePassEvidence(rxFinal, txFinal, root); err == nil || !strings.Contains(err.Error(), "read raw artifact") {
		t.Fatalf("missing pair artifact error = %v", err)
	}
}

func TestClockResolutionQuantizationIsEnforced(t *testing.T) {
	report := turnaroundReport()
	report.Clock.ResolutionNS = 1_000_000
	if _, err := FinalizeReport(report); err == nil || !strings.Contains(err.Error(), "clock resolution") {
		t.Fatalf("coarse resolution error = %v", err)
	}

	report = turnaroundReport()
	report.Clock.ResolutionNS = 3_000
	if _, err := FinalizeReport(report); err == nil || !strings.Contains(err.Error(), "clock resolution") {
		t.Fatalf("off-tick duration error = %v", err)
	}

	report = turnaroundReport()
	report.Clock.ResolutionNS = 2_000
	if _, err := FinalizeReport(report); err != nil {
		t.Fatalf("whole-tick report error = %v", err)
	}
}

func TestChannelMatrixCapacityCoverageAndFailedLossAreEnforced(t *testing.T) {
	report := turnaroundReport()
	report.Definition = MeasurementDefinition{
		GateName:    GateChannelCapacity,
		Kind:        KindChannelCapacity,
		StartMarker: EndpointExternalTestFrameStart,
		EndMarker:   EndpointValidReceiverFrameCompletion,
	}
	applyKindFields(&report)
	final, err := FinalizeReport(report)
	if err != nil {
		t.Fatal(err)
	}
	if final.Verdict != VerdictPassed {
		t.Fatalf("channel fixture verdict = %q/%v", final.Verdict, final.NoPassReasons)
	}

	insufficient := CloneReport(report)
	insufficient.TrialPlan.MatrixCases[0].Channel.RequiredFrameBytes = 21
	insufficientFinal, err := FinalizeReport(insufficient)
	if err != nil {
		t.Fatal(err)
	}
	if insufficientFinal.Verdict != VerdictFailed || !slices.Contains(insufficientFinal.NoPassReasons, ReasonChannelCapacityInsufficient) {
		t.Fatalf("insufficient capacity verdict = %q/%v", insufficientFinal.Verdict, insufficientFinal.NoPassReasons)
	}

	incomplete := CloneReport(report)
	secondCase := incomplete.TrialPlan.MatrixCases[0]
	secondCase.CaseID = "channel-profile-002"
	incomplete.TrialPlan.MatrixCases = append(incomplete.TrialPlan.MatrixCases, secondCase)
	incomplete.TrialPlan.PlannedTrialCount = 4
	incomplete.TrialPlan.MinimumSampleCount = 4
	incompleteFinal, err := FinalizeReport(incomplete)
	if err != nil {
		t.Fatal(err)
	}
	if incompleteFinal.Verdict != VerdictNotEvaluated || !slices.Contains(incompleteFinal.NoPassReasons, ReasonMatrixCoverageIncomplete) {
		t.Fatalf("matrix coverage verdict = %q/%v", incompleteFinal.Verdict, incompleteFinal.NoPassReasons)
	}

	loss := CloneReport(report)
	loss.Trials[1].Status = TrialFailed
	loss.Trials[1].DurationUS = 0
	loss.Trials[1].FailureReason = "frame lost"
	loss.Trials[1].Channel.FramesLost = 1
	loss.Thresholds.Duration.MaxFailedSamples = 1
	lossFinal, err := FinalizeReport(loss)
	if err != nil {
		t.Fatal(err)
	}
	if lossFinal.Summary.Channel.FramesLost != 1 || lossFinal.Verdict != VerdictFailed || !slices.Contains(lossFinal.NoPassReasons, ReasonChannelThresholdExceeded) {
		t.Fatalf("failed loss summary/verdict = %+v/%q/%v", lossFinal.Summary.Channel, lossFinal.Verdict, lossFinal.NoPassReasons)
	}
}

func TestCanonicalHashNormalizesNilAndEmptyArrays(t *testing.T) {
	report := turnaroundReport()
	report.RawArtifacts = nil
	final, err := FinalizeReport(report)
	if err != nil {
		t.Fatal(err)
	}
	nilSlices := CloneReport(final)
	nilSlices.RawArtifacts = nil
	emptySlices := CloneReport(final)
	emptySlices.RawArtifacts = []ArtifactRef{}
	hashNil, err := ReportHash(nilSlices)
	if err != nil {
		t.Fatal(err)
	}
	hashEmpty, err := ReportHash(emptySlices)
	if err != nil {
		t.Fatal(err)
	}
	if hashNil != hashEmpty {
		t.Fatalf("nil/empty canonical hashes differ: %s != %s", hashNil, hashEmpty)
	}
}

func TestEveryMatrixCaseUsesAcceptedCandidateProfile(t *testing.T) {
	kinds := []struct {
		kind  MeasurementKind
		gate  GateName
		start EndpointMarker
		end   EndpointMarker
	}{
		{KindChannelCapacity, GateChannelCapacity, EndpointExternalTestFrameStart, EndpointValidReceiverFrameCompletion},
		{KindRXToTXTurnaround, GateTXRXTurnaround, EndpointLastValidExternalRXMarker, EndpointFirstValidTXMarker},
		{KindTXToRXTurnaround, GateTXRXTurnaround, EndpointLastValidTXMarker, EndpointFirstValidRXMarker},
		{KindTimingAcquisition, GateTimingAcquisition, EndpointSourceSyncReference, EndpointBackendEpochLock},
	}
	for _, test := range kinds {
		t.Run(string(test.kind), func(t *testing.T) {
			report := turnaroundReport()
			report.Definition = MeasurementDefinition{GateName: test.gate, Kind: test.kind, StartMarker: test.start, EndMarker: test.end}
			applyKindFields(&report)
			report.TrialPlan.MatrixCases[0].ProfileID = "another/profile"
			if report.Trials[0].Channel != nil {
				for i := range report.Trials {
					report.Trials[i].Channel.ProfileID = "another/profile"
				}
			}
			if _, err := FinalizeReport(report); err == nil || !strings.Contains(err.Error(), "accepted candidate profile") {
				t.Fatalf("profile substitution error = %v", err)
			}
		})
	}
}

func writeRawFixture(t *testing.T, root, relative string, raw []byte) []byte {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return raw
}
