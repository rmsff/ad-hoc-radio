package backendmeasurement

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/radiobackend"
)

const measurementFixturePath = "../../configs/firmware-contract/mvp10-backend-measurement-contract.json"
const expectedMeasurementFixtureHash = "1f217ed60af3b8ac67588d8f0bb605c0fb66c955af3d86641214d6d08ff03227"

func TestUT030NearestRankUsesDeterministicIntegerRanks(t *testing.T) {
	tests := []struct {
		name       string
		values     []uint64
		percentile uint8
		want       uint64
	}{
		{name: "single p50", values: []uint64{7}, percentile: 50, want: 7},
		{name: "single p95", values: []uint64{7}, percentile: 95, want: 7},
		{name: "odd p50", values: []uint64{50, 10, 30}, percentile: 50, want: 30},
		{name: "even p50 nearest rank", values: []uint64{40, 10, 30, 20}, percentile: 50, want: 20},
		{name: "p95 non exact", values: sequence(1, 19), percentile: 95, want: 19},
		{name: "p95 exact", values: sequence(1, 20), percentile: 95, want: 19},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			original := append([]uint64(nil), test.values...)
			got, err := NearestRank(test.values, test.percentile)
			if err != nil {
				t.Fatalf("NearestRank() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("NearestRank() = %d, want %d", got, test.want)
			}
			if !slices.Equal(test.values, original) {
				t.Fatal("NearestRank() mutated input")
			}
		})
	}

	if _, err := NearestRank(nil, 50); err == nil {
		t.Fatal("NearestRank(nil) expected error")
	}
	if _, err := NearestRank([]uint64{1}, 0); err == nil {
		t.Fatal("NearestRank(percentile=0) expected error")
	}
}

func TestUT031FailedTrialsRemainInDenominatorAndVerdict(t *testing.T) {
	report := turnaroundReport()
	report.Trials = []Trial{
		{TrialID: "trial-001", CaseID: "turnaround-warm-nominal", Status: TrialSucceeded, DurationUS: 10},
		{TrialID: "trial-002", CaseID: "turnaround-warm-nominal", Status: TrialFailed, FailureReason: "timeout"},
		{TrialID: "trial-003", CaseID: "turnaround-warm-nominal", Status: TrialSucceeded, DurationUS: 30},
		{TrialID: "warmup-001", CaseID: "turnaround-warm-nominal", Warmup: true, Status: TrialFailed, FailureReason: "warmup timeout"},
	}
	report.TrialPlan.PlannedTrialCount = 3
	report.TrialPlan.MinimumSampleCount = 3
	report.TrialPlan.WarmupCount = 1
	report.Thresholds.Duration.MaxFailedSamples = 0

	final, err := FinalizeReport(report)
	if err != nil {
		t.Fatalf("FinalizeReport() error = %v", err)
	}
	if final.Summary.AttemptedSampleCount != 3 || final.Summary.SuccessfulSampleCount != 2 || final.Summary.FailedSampleCount != 1 || final.Summary.WarmupCount != 1 {
		t.Fatalf("summary counts = %+v", final.Summary)
	}
	if final.Summary.P50US != 10 || final.Summary.P95US != 30 || final.Summary.MaxUS != 30 {
		t.Fatalf("summary percentiles = %+v", final.Summary)
	}
	if len(final.Summary.FailureReasons) != 1 || final.Summary.FailureReasons[0].Reason != "timeout" || final.Summary.FailureReasons[0].Count != 1 {
		t.Fatalf("failure reasons = %+v", final.Summary.FailureReasons)
	}
	if final.Verdict != VerdictFailed || !slices.Contains(final.NoPassReasons, ReasonFailedSamplesExceeded) {
		t.Fatalf("verdict/reasons = %q/%v", final.Verdict, final.NoPassReasons)
	}
	if final.Safety.GatePassClaim {
		t.Fatal("failed report cannot claim gate pass")
	}
}

func TestIT086CheckedInContractIsStrictDeterministicAndNonPassing(t *testing.T) {
	raw := readMeasurementFixture(t)
	report, err := DecodeReport(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("DecodeReport() error = %v", err)
	}
	if report.SchemaVersion != SchemaVersion || report.StoryID != StoryID {
		t.Fatalf("identity = %q/%q", report.SchemaVersion, report.StoryID)
	}
	if report.HandoffContractSHA256 != FrozenHandoffSHA256 || report.SelectionReportSHA256 != FrozenSelectionSHA256 {
		t.Fatalf("frozen bindings = %q/%q", report.HandoffContractSHA256, report.SelectionReportSHA256)
	}
	if report.Candidate.CandidateID != SelectedCandidateID || report.Candidate.ProfileID != SelectedProfileID {
		t.Fatalf("candidate/profile = %q/%q", report.Candidate.CandidateID, report.Candidate.ProfileID)
	}
	if report.Verdict != VerdictNotEvaluated || report.EvidenceGrade != EvidenceFixture {
		t.Fatalf("fixture verdict/evidence = %q/%q", report.Verdict, report.EvidenceGrade)
	}
	if report.Safety.GatePassClaim || report.Safety.HardwareClaimAllowed || report.Safety.RFSynchronizationClaim || report.Safety.FieldTXReady || report.Safety.FirmwareWriteAllowed {
		t.Fatalf("unsafe fixture claims = %+v", report.Safety)
	}

	canonicalA, err := MarshalCanonical(report)
	if err != nil {
		t.Fatalf("MarshalCanonical(first) error = %v", err)
	}
	canonicalB, err := MarshalCanonical(report)
	if err != nil {
		t.Fatalf("MarshalCanonical(second) error = %v", err)
	}
	if !bytes.Equal(canonicalA, canonicalB) {
		t.Fatal("canonical bytes differ")
	}
	hashA, err := ReportHash(report)
	if err != nil {
		t.Fatalf("ReportHash() error = %v", err)
	}
	hashB, err := ReportHash(CloneReport(report))
	if err != nil || hashA != hashB {
		t.Fatalf("report hashes = %q/%q, error=%v", hashA, hashB, err)
	}
	if hashA != expectedMeasurementFixtureHash {
		t.Fatalf("report hash = %q, want %q", hashA, expectedMeasurementFixtureHash)
	}
	if err := ValidateRawArtifactContents(report, filepath.Join("..", "..")); err != nil {
		t.Fatalf("ValidateRawArtifactContents() error = %v", err)
	}

	handoffRaw, err := os.ReadFile(filepath.Join("..", "..", "configs", "firmware-contract", "mvp9-radio-backend-handoff.json"))
	if err != nil {
		t.Fatal(err)
	}
	handoff, err := radiobackend.DecodeContract(bytes.NewReader(handoffRaw))
	if err != nil {
		t.Fatal(err)
	}
	handoffHash, err := radiobackend.ContractHash(handoff)
	if err != nil || handoffHash != FrozenHandoffSHA256 {
		t.Fatalf("handoff hash = %q, error=%v", handoffHash, err)
	}
	selectionRaw, err := os.ReadFile(filepath.Join("..", "..", "configs", "firmware-contract", "mvp10-backend-selection.json"))
	if err != nil {
		t.Fatal(err)
	}
	selection, err := radiobackend.DecodeSelectionReport(bytes.NewReader(selectionRaw))
	if err != nil {
		t.Fatal(err)
	}
	selectionHash, err := radiobackend.SelectionReportHash(selection)
	if err != nil || selectionHash != FrozenSelectionSHA256 {
		t.Fatalf("selection hash = %q, error=%v", selectionHash, err)
	}
}

func TestMeasurementEndpointsAndKindMappingAreExact(t *testing.T) {
	tests := []struct {
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

	for _, test := range tests {
		t.Run(string(test.kind), func(t *testing.T) {
			report := turnaroundReport()
			report.Definition.Kind = test.kind
			report.Definition.GateName = test.gate
			report.Definition.StartMarker = test.start
			report.Definition.EndMarker = test.end
			applyKindFields(&report)
			final, err := FinalizeReport(report)
			if err != nil {
				t.Fatalf("FinalizeReport() error = %v", err)
			}
			if err := ValidateReport(final); err != nil {
				t.Fatalf("ValidateReport() error = %v", err)
			}

			wrong := CloneReport(final)
			wrong.Definition.EndMarker = EndpointBackendEpochLock
			if test.end == EndpointBackendEpochLock {
				wrong.Definition.EndMarker = EndpointFirstValidRXMarker
			}
			if err := ValidateReport(wrong); err == nil || !strings.Contains(err.Error(), "endpoints") {
				t.Fatalf("wrong endpoint error = %v", err)
			}
		})
	}
}

func TestMissingEvidenceAndThresholdsCannotPass(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*MeasurementReport)
		reason string
	}{
		{name: "unknown clock", mutate: func(r *MeasurementReport) {
			r.Clock.Status = ClockUnknown
			r.Clock.BackendClockID = "unknown"
			r.Clock.ResolutionNS = 0
		}, reason: ReasonUnknownClock},
		{name: "missing raw", mutate: func(r *MeasurementReport) { r.RawArtifacts = nil }, reason: ReasonMissingRawArtifacts},
		{name: "missing thresholds", mutate: func(r *MeasurementReport) { r.Thresholds = nil }, reason: ReasonMissingThresholds},
		{name: "insufficient samples", mutate: func(r *MeasurementReport) { r.TrialPlan.PlannedTrialCount = 3; r.TrialPlan.MinimumSampleCount = 3 }, reason: ReasonInsufficientSamples},
		{name: "fixture evidence", mutate: func(r *MeasurementReport) { r.EvidenceGrade = EvidenceFixture }, reason: ReasonNotBenchMeasured},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := turnaroundReport()
			test.mutate(&report)
			final, err := FinalizeReport(report)
			if err != nil {
				t.Fatalf("FinalizeReport() error = %v", err)
			}
			if final.Verdict == VerdictPassed || final.Safety.GatePassClaim || !slices.Contains(final.NoPassReasons, test.reason) {
				t.Fatalf("verdict/reasons = %q/%v", final.Verdict, final.NoPassReasons)
			}
		})
	}
}

func TestDecodeIsStrictSummaryIsVerifiedAndCloneIsImmutable(t *testing.T) {
	raw := readMeasurementFixture(t)
	unknown := bytes.Replace(raw, []byte(`"story_id": "MVP-10.2"`), []byte(`"story_id": "MVP-10.2", "unexpected": true`), 1)
	if _, err := DecodeReport(bytes.NewReader(unknown)); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown field error = %v", err)
	}
	if _, err := DecodeReport(bytes.NewReader(append(raw, []byte(` {}`)...))); err == nil || !strings.Contains(err.Error(), "trailing JSON") {
		t.Fatalf("trailing JSON error = %v", err)
	}

	report := turnaroundReport()
	final, err := FinalizeReport(report)
	if err != nil {
		t.Fatal(err)
	}
	wrong := CloneReport(final)
	wrong.Summary.P95US++
	if err := ValidateReport(wrong); err == nil || !strings.Contains(err.Error(), "summary") {
		t.Fatalf("summary mismatch error = %v", err)
	}

	clone := CloneReport(final)
	clone.Trials[0].TrialID = "mutated"
	clone.RawArtifacts[0].SHA256 = strings.Repeat("f", 64)
	clone.NoPassReasons = append(clone.NoPassReasons, "mutated")
	if final.Trials[0].TrialID == "mutated" || final.RawArtifacts[0].SHA256 == strings.Repeat("f", 64) || slices.Contains(final.NoPassReasons, "mutated") {
		t.Fatal("CloneReport() aliases caller data")
	}
}

func TestMalformedOrderingAndKindInputsAreRejected(t *testing.T) {
	base, err := FinalizeReport(turnaroundReport())
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		mutate  func(*MeasurementReport)
		wantErr string
	}{
		{name: "malformed firmware hash", mutate: func(r *MeasurementReport) { r.Candidate.FirmwareSHA256 = "bad" }, wantErr: "SHA-256"},
		{name: "wrong source revision", mutate: func(r *MeasurementReport) { r.Candidate.SourceRevision = strings.Repeat("a", 40) }, wantErr: "accepted selection"},
		{name: "invalid evidence enum", mutate: func(r *MeasurementReport) { r.EvidenceGrade = "datasheet_proxy" }, wantErr: "evidence_grade"},
		{name: "unsorted trials", mutate: func(r *MeasurementReport) { r.Trials[0], r.Trials[1] = r.Trials[1], r.Trials[0] }, wantErr: "sorted"},
		{name: "duplicate trial", mutate: func(r *MeasurementReport) { r.Trials[1].TrialID = r.Trials[0].TrialID }, wantErr: "duplicate"},
		{name: "unsorted raw refs", mutate: func(r *MeasurementReport) {
			r.RawArtifacts = []ArtifactRef{
				{Path: "configs/z.ndjson", SHA256: strings.Repeat("d", 64)},
				{Path: "configs/a.ndjson", SHA256: strings.Repeat("e", 64)},
			}
		}, wantErr: "raw_artifacts must be sorted"},
		{name: "turnaround carries timing fields", mutate: func(r *MeasurementReport) { r.Trials[0].Timing = &TimingObservation{} }, wantErr: "incompatible fields"},
		{name: "unsafe hardware claim", mutate: func(r *MeasurementReport) { r.Safety.HardwareClaimAllowed = true }, wantErr: "claims must remain false"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := CloneReport(base)
			test.mutate(&report)
			if err := ValidateReport(report); err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("ValidateReport() error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}

func TestThresholdAndTimingMetricsDriveFailure(t *testing.T) {
	report := turnaroundReport()
	report.Definition = MeasurementDefinition{
		GateName:    GateTimingAcquisition,
		Kind:        KindTimingAcquisition,
		StartMarker: EndpointSourceSyncReference,
		EndMarker:   EndpointBackendEpochLock,
	}
	applyKindFields(&report)
	report.Thresholds.Timing = &TimingThreshold{
		MaxAbsBoundaryErrorUS: 5,
		MaxAbsDriftMilliPPM:   10,
		MaxGuardUsedUS:        3,
		MaxResyncCount:        0,
	}
	report.Trials[0].Timing = &TimingObservation{BoundaryErrorUS: -6, DriftMilliPPM: -11, GuardUsedUS: 4, ResyncCount: 1}
	report.Trials[1].Timing = &TimingObservation{BoundaryErrorUS: 1, DriftMilliPPM: 1}

	final, err := FinalizeReport(report)
	if err != nil {
		t.Fatal(err)
	}
	if final.Verdict != VerdictFailed || !slices.Contains(final.NoPassReasons, ReasonTimingThresholdExceeded) {
		t.Fatalf("verdict/reasons = %q/%v", final.Verdict, final.NoPassReasons)
	}
	if final.Summary.Timing.MaxAbsBoundaryErrorUS != 6 || final.Summary.Timing.MaxAbsDriftMilliPPM != 11 || final.Summary.Timing.MaxGuardUsedUS != 4 || final.Summary.Timing.ResyncCount != 1 {
		t.Fatalf("timing summary = %+v", final.Summary.Timing)
	}
}

func turnaroundReport() MeasurementReport {
	return MeasurementReport{
		SchemaVersion:         SchemaVersion,
		StoryID:               StoryID,
		HandoffContractSHA256: FrozenHandoffSHA256,
		SelectionReportSHA256: FrozenSelectionSHA256,
		Candidate: CandidateIdentity{
			CandidateID:    SelectedCandidateID,
			ProfileID:      SelectedProfileID,
			FirmwareBase:   "OpenRTX v0.4.4",
			SourceRef:      "https://github.com/OpenRTX/OpenRTX.git",
			SourceRevision: "801fc8dd313eb3ac6f83fd4d3b63ed895effdd56",
			Target:         "mduv3x0",
			FirmwareSHA256: strings.Repeat("a", 64),
			BinarySHA256:   strings.Repeat("b", 64),
		},
		Definition: MeasurementDefinition{
			GateName:    GateTXRXTurnaround,
			Kind:        KindRXToTXTurnaround,
			StartMarker: EndpointLastValidExternalRXMarker,
			EndMarker:   EndpointFirstValidTXMarker,
		},
		Clock: ClockDefinition{Status: ClockKnown, BackendClockID: "openrtx_systick_monotonic", ResolutionNS: 1_000},
		Units: CanonicalUnits(),
		TrialPlan: TrialPlan{
			PlannedTrialCount:     2,
			MinimumSampleCount:    2,
			MinimumSamplesPerCase: 2,
			MatrixCases: []MatrixCase{{
				CaseID:          "turnaround-warm-nominal",
				ProfileID:       SelectedProfileID,
				StateCondition:  "warm",
				PowerProfile:    "lab_fixture",
				ExpectedOutcome: "success",
			}},
		},
		Thresholds:    &ThresholdSet{Version: ThresholdVersion, Duration: DurationThreshold{P95MaxUS: 50, MaxUS: 60, MaxFailedSamples: 0}},
		EvidenceGrade: EvidenceBenchMeasured,
		RawArtifacts:  []ArtifactRef{{Path: "configs/firmware-contract/fixtures/mvp10.2-turnaround-fixture.ndjson", SHA256: strings.Repeat("c", 64)}},
		Trials: []Trial{
			{TrialID: "trial-001", CaseID: "turnaround-warm-nominal", Status: TrialSucceeded, DurationUS: 10},
			{TrialID: "trial-002", CaseID: "turnaround-warm-nominal", Status: TrialSucceeded, DurationUS: 20},
		},
	}
}

func applyKindFields(report *MeasurementReport) {
	report.Thresholds.Channel = nil
	report.Thresholds.Timing = nil
	for i := range report.Trials {
		report.Trials[i].Channel = nil
		report.Trials[i].Timing = nil
	}
	switch report.Definition.Kind {
	case KindChannelCapacity:
		report.TrialPlan.MatrixCases = []MatrixCase{{
			CaseID:          "channel-profile-001",
			ProfileID:       SelectedProfileID,
			StateCondition:  "nominal",
			PowerProfile:    "lab_fixture",
			ExpectedOutcome: "success",
			Channel: &ChannelMatrixCase{
				ChannelRateBPS:     9600,
				PayloadBytes:       16,
				OverheadBytes:      8,
				RequiredFrameBytes: 20,
				GuardMarginBytes:   4,
			},
		}}
		report.Thresholds.Channel = &ChannelThreshold{MaxBitErrors: 0, MaxFrameErrors: 0, MaxFramesLost: 0}
		for i := range report.Trials {
			report.Trials[i].CaseID = "channel-profile-001"
			report.Trials[i].Channel = &ChannelObservation{ProfileID: SelectedProfileID, ChannelRateBPS: 9600, PayloadBytes: 16, OverheadBytes: 8}
		}
	case KindTimingAcquisition:
		report.TrialPlan.MatrixCases = []MatrixCase{{
			CaseID:          "timing-cold-entry",
			ProfileID:       SelectedProfileID,
			StateCondition:  "cold_entry",
			PowerProfile:    "receive_only",
			ExpectedOutcome: "success",
		}}
		report.Thresholds.Timing = &TimingThreshold{MaxAbsBoundaryErrorUS: 50, MaxAbsDriftMilliPPM: 1000, MaxGuardUsedUS: 10, MaxResyncCount: 0}
		for i := range report.Trials {
			report.Trials[i].CaseID = "timing-cold-entry"
			report.Trials[i].Timing = &TimingObservation{BoundaryErrorUS: 1, DriftMilliPPM: 2, GuardUsedUS: 3}
		}
	default:
		report.TrialPlan.MatrixCases = []MatrixCase{{
			CaseID:          "turnaround-warm-nominal",
			ProfileID:       SelectedProfileID,
			StateCondition:  "warm",
			PowerProfile:    "lab_fixture",
			ExpectedOutcome: "success",
		}}
		for i := range report.Trials {
			report.Trials[i].CaseID = "turnaround-warm-nominal"
		}
	}
}

func sequence(first, last uint64) []uint64 {
	values := make([]uint64, 0, last-first+1)
	for value := first; value <= last; value++ {
		values = append(values, value)
	}
	return values
}

func readMeasurementFixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Clean(measurementFixturePath))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func rawSHA256(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
