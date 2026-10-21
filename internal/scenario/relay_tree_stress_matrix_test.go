package scenario

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

func TestIT071_DefaultStressMatrixCoversExactBoundedInventory(t *testing.T) {
	cases := BuildDefaultRelayTreeStressCases()
	if len(cases) != 30 {
		t.Fatalf("case count = %d, want 30", len(cases))
	}

	seenGrid := make(map[[2]int]bool)
	tags := make(map[string]bool)
	for _, testCase := range cases {
		for _, tag := range testCase.StressTags {
			tags[string(tag)] = true
		}
		if testCase.Role == RelayTreeStressRoleCore {
			seenGrid[[2]int{testCase.ExpectedGroupSize, int(testCase.RequestedRelayDepth)}] = true
		}
	}
	for _, size := range []int{2, 4, 8, 16} {
		for depth := 0; depth <= 3; depth++ {
			if !seenGrid[[2]int{size, depth}] {
				t.Fatalf("missing core grid size=%d depth=%d", size, depth)
			}
		}
	}
	for _, required := range []string{
		"branch", "asymmetric_link", "partition", "recovery", "stale_membership",
		"missing_member", "late_join", "link_loss", "link_delay", "clock_drift",
		"relay_busy", "relay_low_battery", "relay_loss", "leader_loss",
	} {
		if !tags[required] {
			t.Fatalf("missing stress tag %q", required)
		}
	}
}

func TestIT072_StressMatrixProducesCanonicalRowsAndProfileVerdicts(t *testing.T) {
	report, err := RunRelayTreeStressMatrix(RelayTreeStressMatrixConfig{
		Cases: BuildDefaultRelayTreeStressCases(),
	})
	if err != nil {
		t.Fatalf("RunRelayTreeStressMatrix failed: %v", err)
	}
	if report.CaseCount != 30 || report.RowCount != 120 || len(report.Rows) != 120 {
		t.Fatalf("report counts = cases:%d rows:%d/%d, want 30 and 120", report.CaseCount, report.RowCount, len(report.Rows))
	}
	for _, row := range report.Rows {
		if row.Classification == RelayTreeStressNoGo {
			t.Logf("no-go row %s reasons=%v comparison=%+v", row.RowID, row.ReasonCodes, row.Comparison)
		}
	}
	for _, summary := range report.ProfileSummaries {
		t.Logf("summary %+v", summary)
	}
	t.Logf("stress evidence hash %s", report.Evidence.Determinism.EventHash)
	if report.ArchitectureVerdict != RelayTreeArchitectureReviewRequired {
		t.Fatalf("impaired skeleton must require review, got %q", report.ArchitectureVerdict)
	}
	if report.Recommendation != "insufficient_evidence" {
		t.Fatalf("failed service must not recommend a profile: %q", report.Recommendation)
	}

	want := map[string]RelayTreeProfileVerdict{
		"source_led/project_native_variable":   RelayTreeProfileNoGo,
		"group_leader/project_native_variable": RelayTreeProfileNoGo,
		"source_led/dmr_inspired_30ms":         RelayTreeProfileNoGo,
		"group_leader/dmr_inspired_30ms":       RelayTreeProfileNoGo,
	}
	if len(report.ProfileSummaries) != len(want) {
		t.Fatalf("profile summaries = %d, want %d", len(report.ProfileSummaries), len(want))
	}
	for _, summary := range report.ProfileSummaries {
		if summary.Verdict != want[summary.ProfileID] {
			t.Fatalf("profile %s verdict = %q, want %q", summary.ProfileID, summary.Verdict, want[summary.ProfileID])
		}
		if summary.BackendStatus != RelayTreeBackendNotEvaluated {
			t.Fatalf("profile %s backend status = %q", summary.ProfileID, summary.BackendStatus)
		}
	}
	if report.DMRCompatible || report.CompatibilityClaim || report.AMBEDependency || report.RFSynchronizationClaim || report.Evidence.HardwareClaimAllowed {
		t.Fatalf("unexpected compatibility/hardware claim: %+v", report)
	}
}

func TestIT073_DepthThreeAndBranchWidthEvidenceIsExplicit(t *testing.T) {
	report, err := RunRelayTreeStressMatrix(RelayTreeStressMatrixConfig{Cases: BuildDefaultRelayTreeStressCases()})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range report.Rows {
		if row.RequestedRelayDepth != 3 {
			continue
		}
		if !row.CounterfactualEvaluated {
			t.Fatalf("depth-3 row %s did not evaluate the depth-2 counterfactual", row.RowID)
		}
		if row.Depth3AddsUsefulCoverage && (!row.Comparison.ProfileFit || row.Comparison.RuntimeOutcome != TimingRuntimePass || row.Classification != RelayTreeStressPass || row.Depth3CoverageGain <= 0) {
			t.Fatalf("depth-3 row %s claims non-executable utility: %+v", row.RowID, row)
		}
	}
	native := stressProfileSummary(t, report, timing.TimingStrategySourceLed, TimingProfileProjectNative)
	if native.Verdict != RelayTreeProfileNoGo {
		t.Fatalf("lossy native skeleton cannot prove live-voice envelope: %+v", native)
	}
	if native.MaxPassingBranchWidth > 4 || native.FirstImpracticalBranchWidth != 8 {
		t.Fatalf("native branch limits = %+v, want at most 4 first impractical 8", native)
	}
	dmr := stressProfileSummary(t, report, timing.TimingStrategySourceLed, TimingProfileDMRInspired30MS)
	if dmr.MaxSupportedRelayDepth != 0 || dmr.Depth3AddsUsefulCoverage {
		t.Fatalf("DMR-inspired depth evidence = %+v, want direct-only depth 0", dmr)
	}
	if dmr.MaxPassingBranchWidth != 0 || dmr.FirstImpracticalBranchWidth != 2 {
		t.Fatalf("DMR-inspired branch limits = %+v, want max 0 first impractical 2", dmr)
	}
}

func TestIT074_MembershipImpairmentAndRecoveryRemainExplicit(t *testing.T) {
	report, err := RunRelayTreeStressMatrix(RelayTreeStressMatrixConfig{Cases: BuildDefaultRelayTreeStressCases()})
	if err != nil {
		t.Fatal(err)
	}
	for _, caseID := range []string{"membership-stale", "membership-missing", "partition-partial"} {
		row := stressMatrixRow(t, report, caseID, timing.TimingStrategySourceLed, TimingProfileProjectNative)
		if row.Classification != RelayTreeStressNoGo || len(row.Comparison.UncoveredMemberIDs) == 0 {
			t.Fatalf("%s classification/uncovered = %q/%v", caseID, row.Classification, row.Comparison.UncoveredMemberIDs)
		}
	}
	for _, caseID := range []string{"partition-recovery", "relay-loss-replacement", "combined-leader-relay-loss"} {
		row := stressMatrixRow(t, report, caseID, timing.TimingStrategyGroupLeader, TimingProfileProjectNative)
		if row.Comparison.RouteReplanCount == 0 || row.Comparison.VersionActivationCount == 0 {
			t.Fatalf("%s recovery evidence = %+v", caseID, row)
		}
	}
	late := stressMatrixRow(t, report, "membership-late-join-drift", timing.TimingStrategyGroupLeader, TimingProfileProjectNative)
	if late.Comparison.LateEntryAcquisitionMS == 0 || late.Comparison.DriftCollisionCount == 0 || late.Comparison.ResyncCount == 0 {
		t.Fatalf("late join/drift evidence = %+v", late.Comparison)
	}
	accepted := stressMatrixRow(t, report, "link-boundary-accepted", timing.TimingStrategySourceLed, TimingProfileProjectNative)
	if !accepted.Comparison.ProfileFit || accepted.Classification != RelayTreeStressNoGo {
		t.Fatalf("planner eligibility is not lossless delivery: %+v", accepted)
	}
	rejected := stressMatrixRow(t, report, "link-boundary-rejected", timing.TimingStrategySourceLed, TimingProfileProjectNative)
	if rejected.Comparison.ProfileFit || rejected.Classification != RelayTreeStressDegraded || !rejected.InvariantPass {
		t.Fatalf("rejected link boundary = %+v, want expected degraded rejection", rejected)
	}
}

func TestIT075_CombinedReplansRequireDistinctSourceSequences(t *testing.T) {
	cases := BuildDefaultRelayTreeStressCases()
	combined := stressCaseByID(t, cases, "combined-leader-relay-loss")
	report, err := RunTimingProfileComparison([]TimingProfileComparisonFixture{{
		FixtureID:     combined.CaseID,
		BaseConfig:    combined.BaseConfig,
		TimingOverlay: combined.TimingOverlay,
	}})
	if err != nil {
		t.Fatalf("sequential combined fixture failed: %v", err)
	}
	row := timingComparisonRow(t, report, combined.CaseID+"/group_leader/project_native_variable")
	if row.RouteReplanCount != 2 || row.VersionActivationCount != 2 || row.ScheduleVersion != 3 || row.TimingAuthorityID != 3 {
		t.Fatalf("combined replan/version progression = %+v, want replans=2 activations=2 version=3 authority=3", row)
	}

	conflict := combined
	conflict.BaseConfig.Replans[0].AtMS = conflict.TimingOverlay.LeaderLoss.AtMS
	if _, err := RunTimingProfileComparison([]TimingProfileComparisonFixture{{
		FixtureID:     "combined-conflict",
		BaseConfig:    conflict.BaseConfig,
		TimingOverlay: conflict.TimingOverlay,
	}}); err == nil || !strings.Contains(err.Error(), "same effective source sequence") {
		t.Fatalf("same-sequence conflict error = %v", err)
	}

	existingConflict := combined
	existingConflict.BaseConfig = cloneTimingRelayTreeScenarioConfig(combined.BaseConfig)
	second := existingConflict.BaseConfig.Replans[0]
	second.AtMS++
	existingConflict.BaseConfig.Replans = append(existingConflict.BaseConfig.Replans, second)
	existingConflict.TimingOverlay.LeaderLoss = nil
	if _, err := RunTimingProfileComparison([]TimingProfileComparisonFixture{{
		FixtureID:     "existing-replans-conflict",
		BaseConfig:    existingConflict.BaseConfig,
		TimingOverlay: existingConflict.TimingOverlay,
	}}); err == nil || !strings.Contains(err.Error(), "same effective source sequence") {
		t.Fatalf("existing replan same-sequence conflict error = %v", err)
	}
}

func TestIT076_BackendFailureRejectsOnlyTargetProfile(t *testing.T) {
	report, err := RunRelayTreeStressMatrix(RelayTreeStressMatrixConfig{
		Cases: BuildDefaultRelayTreeStressCases(),
		BackendAssessments: []RelayTreeBackendAssessment{{
			TimingStrategy: timing.TimingStrategySourceLed,
			Profile:        TimingProfileProjectNative,
			Status:         RelayTreeBackendFailed,
			Reason:         "measured turnaround exceeds accepted envelope",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	failed := stressProfileSummary(t, report, timing.TimingStrategySourceLed, TimingProfileProjectNative)
	if failed.Verdict != RelayTreeProfileNoGo || failed.BackendStatus != RelayTreeBackendFailed {
		t.Fatalf("failed backend summary = %+v", failed)
	}
	other := stressProfileSummary(t, report, timing.TimingStrategyGroupLeader, TimingProfileProjectNative)
	if other.BackendStatus != RelayTreeBackendNotEvaluated || slices.Contains(other.ReasonCodes, "backend_failed") {
		t.Fatalf("backend failure contaminated unrelated profile: %+v", other)
	}
	if report.ArchitectureVerdict != RelayTreeArchitectureReviewRequired {
		t.Fatalf("backend failure rewrote architecture verdict: %q", report.ArchitectureVerdict)
	}
}

func TestIT077_ReportIsDeterministicUnderInputPermutation(t *testing.T) {
	cases := BuildDefaultRelayTreeStressCases()
	first, err := RunRelayTreeStressMatrix(RelayTreeStressMatrixConfig{Cases: cases})
	if err != nil {
		t.Fatal(err)
	}
	reversed := slices.Clone(cases)
	slices.Reverse(reversed)
	second, err := RunRelayTreeStressMatrix(RelayTreeStressMatrixConfig{Cases: reversed})
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, err := MarshalRelayTreeStressMatrixReport(first)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := MarshalRelayTreeStressMatrixReport(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatal("permuted stress cases changed canonical report bytes")
	}
	if first.Evidence.Determinism.EventHash == "" || first.Evidence.Determinism.EventHash != second.Evidence.Determinism.EventHash {
		t.Fatalf("determinism hashes differ: %q/%q", first.Evidence.Determinism.EventHash, second.Evidence.Determinism.EventHash)
	}
}

func TestIT078_DMRInspiredExactCycleFitStillFailsLiveRelayDelivery(t *testing.T) {
	report, err := RunRelayTreeStressMatrix(RelayTreeStressMatrixConfig{Cases: BuildDefaultRelayTreeStressCases()})
	if err != nil {
		t.Fatal(err)
	}
	row := stressMatrixRow(t, report, "core-g02-d1", timing.TimingStrategySourceLed, TimingProfileDMRInspired30MS)
	if !row.Comparison.ProfileFit || row.Comparison.ScheduleCycleDurationMS != 60 || row.Comparison.FramePeriodMS != 60 {
		t.Fatalf("DMR-inspired fit/cycle evidence = %+v", row.Comparison)
	}
	if row.Comparison.DeliveredFrameCount != 0 || len(row.Comparison.DeliveredMemberIDs) != 0 {
		t.Fatalf("DMR-inspired relay unexpectedly delivered frames: %+v", row.Comparison)
	}
	if row.Classification != RelayTreeStressNoGo || row.InvariantPass {
		t.Fatalf("DMR-inspired live-relay classification = %q invariant=%v, want no_go/false", row.Classification, row.InvariantPass)
	}
}

func TestIT079_StressMatrixEvidenceRemainsSyntheticAndNoClaim(t *testing.T) {
	report, err := RunRelayTreeStressMatrix(RelayTreeStressMatrixConfig{Cases: BuildDefaultRelayTreeStressCases()})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateScenarioEvidence(report.Evidence); err != nil {
		t.Fatalf("ValidateScenarioEvidence failed: %v", err)
	}
	if report.Evidence.EvidenceGrade != EvidenceGradeSyntheticProxy || report.Evidence.Outcome != "fail" || report.Evidence.HardwareClaimAllowed {
		t.Fatalf("evidence boundary = %+v", report.Evidence)
	}
	for _, row := range report.Rows {
		comparison := row.Comparison
		if comparison.DMRCompatible || comparison.CompatibilityClaim || comparison.AMBEDependency || comparison.RFSynchronizationClaim {
			t.Fatalf("row %s carries forbidden compatibility/RF claim", row.RowID)
		}
	}
}

func TestUT026_StressMatrixValidationRejectsMalformedInputs(t *testing.T) {
	cases := BuildDefaultRelayTreeStressCases()
	bad := slices.Clone(cases)
	bad[0].CaseID = " bad/id "
	if _, err := RunRelayTreeStressMatrix(RelayTreeStressMatrixConfig{Cases: bad}); err == nil || !strings.Contains(err.Error(), "case_id") {
		t.Fatalf("invalid case_id error = %v", err)
	}

	bad = slices.Clone(cases)
	bad[1].CaseID = bad[0].CaseID
	if _, err := RunRelayTreeStressMatrix(RelayTreeStressMatrixConfig{Cases: bad}); err == nil || !strings.Contains(err.Error(), "duplicate case_id") {
		t.Fatalf("duplicate case_id error = %v", err)
	}

	if _, err := RunRelayTreeStressMatrix(RelayTreeStressMatrixConfig{
		Cases: cases,
		BackendAssessments: []RelayTreeBackendAssessment{{
			TimingStrategy: timing.TimingStrategySourceLed,
			Profile:        TimingProfileProjectNative,
			Status:         RelayTreeBackendFailed,
		}},
	}); err == nil || !strings.Contains(err.Error(), "reason") {
		t.Fatalf("backend failure without reason error = %v", err)
	}

	synthetic := ScenarioEvidence{
		SchemaVersion: ScenarioEvidenceSchemaVersion,
		StoryID:       "test",
		ScenarioID:    "test",
		EvidenceType:  "test",
		EvidenceGrade: EvidenceGradeSyntheticProxy,
		Seed:          1,
		Determinism: ScenarioEvidenceDeterminism{
			EventHash:     strings.Repeat("0", 64),
			HashAlgorithm: ScenarioEvidenceHashAlgorithm,
			Serialization: ScenarioEvidenceSerialization,
		},
		ArtifactRefs:         []ScenarioEvidenceArtifact{},
		AssumptionFlags:      []string{},
		HardwareClaimAllowed: false,
		Outcome:              "pass",
		DegradeReasons:       []string{},
	}
	if _, err := RunRelayTreeStressMatrix(RelayTreeStressMatrixConfig{
		Cases: cases,
		BackendAssessments: []RelayTreeBackendAssessment{{
			TimingStrategy: timing.TimingStrategySourceLed,
			Profile:        TimingProfileProjectNative,
			Status:         RelayTreeBackendPassed,
			Evidence:       &synthetic,
		}},
	}); err == nil || !strings.Contains(err.Error(), "hardware-claimable bench evidence") {
		t.Fatalf("synthetic backend pass error = %v", err)
	}
}

func TestUT027_DefaultMatrixDoesNotMutateCallerInputs(t *testing.T) {
	t.Run("runner does not mutate caller inputs", func(t *testing.T) {
		cases := BuildDefaultRelayTreeStressCases()
		before := cloneRelayTreeStressCases(cases)
		if _, err := RunRelayTreeStressMatrix(RelayTreeStressMatrixConfig{Cases: cases}); err != nil {
			t.Fatal(err)
		}
		beforeJSON, _ := marshalStressCasesForTest(before)
		afterJSON, _ := marshalStressCasesForTest(cases)
		if !bytes.Equal(beforeJSON, afterJSON) {
			t.Fatal("stress runner mutated caller cases")
		}
	})

	t.Run("per-member threshold rejects aggregate-skewed delivery", func(t *testing.T) {
		members := stressMemberIDs(16)
		delivery := make([]ScheduledMemberDeliveryCount, 0, len(members))
		for _, memberID := range members {
			accepted := 100
			if memberID == members[len(members)-1] {
				accepted = 20
			}
			delivery = append(delivery, ScheduledMemberDeliveryCount{MemberID: memberID, FramesAccepted: accepted})
		}
		row := TimingProfileComparisonRow{
			SourceFramesCreated: 100,
			DeliveredFrameCount: 1_520,
			ExpectedMemberIDs:   append([]uint16(nil), members...),
			CoveredMemberIDs:    append([]uint16(nil), members...),
			DeliveredMemberIDs:  append([]uint16(nil), members...),
			UncoveredMemberIDs:  []uint16{},
			MemberDelivery:      delivery,
			SelectedRelayDepth:  0,
		}
		testCase := RelayTreeStressCase{
			Role:                RelayTreeStressRoleCore,
			RequestedRelayDepth: 0,
			Expectation: RelayTreeStressExpectation{
				RequireAllDelivered: true,
			},
		}
		aggregate := stressDeliveryRatio(row)
		minimum := stressMinMemberDeliveryPct(row)
		if aggregate != 95 || minimum != 20 {
			t.Fatalf("delivery ratios = aggregate %.2f minimum %.2f, want 95/20", aggregate, minimum)
		}
		reasons := relayTreeStressInvariantReasons(testCase, row, aggregate, minimum)
		if !slices.Contains(reasons, "member_delivery_below_95_pct") {
			t.Fatalf("skewed delivery reasons = %v", reasons)
		}
	})

	t.Run("covered member without delivery breaks delivered partition", func(t *testing.T) {
		row := TimingProfileComparisonRow{
			SourceFramesCreated: 100,
			ExpectedMemberIDs:   []uint16{2, 3},
			CoveredMemberIDs:    []uint16{2, 3},
			DeliveredMemberIDs:  []uint16{2},
			UncoveredMemberIDs:  []uint16{},
			MemberDelivery: []ScheduledMemberDeliveryCount{
				{MemberID: 2, FramesAccepted: 100},
				{MemberID: 3, FramesAccepted: 0},
			},
		}
		reasons := relayTreeStressInvariantReasons(
			RelayTreeStressCase{Role: RelayTreeStressRoleResilience},
			row,
			stressDeliveryRatio(row),
			stressMinMemberDeliveryPct(row),
		)
		if !slices.Contains(reasons, "delivered_member_partition_mismatch") {
			t.Fatalf("covered-undelivered reasons = %v", reasons)
		}
	})
}

func stressProfileSummary(t *testing.T, report RelayTreeStressMatrixReport, strategy timing.TimingStrategy, profile TimingProfile) RelayTreeProfileSummary {
	t.Helper()
	want := string(strategy) + "/" + string(profile)
	for _, summary := range report.ProfileSummaries {
		if summary.ProfileID == want {
			return summary
		}
	}
	t.Fatalf("missing profile summary %s", want)
	return RelayTreeProfileSummary{}
}

func stressMatrixRow(t *testing.T, report RelayTreeStressMatrixReport, caseID string, strategy timing.TimingStrategy, profile TimingProfile) RelayTreeStressMatrixRow {
	t.Helper()
	rowID := caseID + "/" + string(strategy) + "/" + string(profile)
	for _, row := range report.Rows {
		if row.RowID == rowID {
			return row
		}
	}
	t.Fatalf("missing row %s", rowID)
	return RelayTreeStressMatrixRow{}
}

func stressCaseByID(t *testing.T, cases []RelayTreeStressCase, id string) RelayTreeStressCase {
	t.Helper()
	for _, testCase := range cases {
		if testCase.CaseID == id {
			return testCase
		}
	}
	t.Fatalf("missing stress case %s", id)
	return RelayTreeStressCase{}
}

func marshalStressCasesForTest(cases []RelayTreeStressCase) ([]byte, error) {
	return marshalRelayTreeStressCases(cases)
}
