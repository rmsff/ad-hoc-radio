package scenario

import (
	"fmt"
	"slices"
	"strings"

	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

func validateRelayTreeStressMatrixConfig(cfg RelayTreeStressMatrixConfig) error {
	if len(cfg.Cases) == 0 {
		return fmt.Errorf("relay-tree stress matrix: at least one case is required")
	}
	seenCases := make(map[string]struct{}, len(cfg.Cases))
	for i, testCase := range cfg.Cases {
		if err := validateRelayTreeStressCase(testCase); err != nil {
			return fmt.Errorf("relay-tree stress matrix: case[%d]: %w", i, err)
		}
		if _, ok := seenCases[testCase.CaseID]; ok {
			return fmt.Errorf("relay-tree stress matrix: duplicate case_id=%q", testCase.CaseID)
		}
		seenCases[testCase.CaseID] = struct{}{}
	}
	return validateRelayTreeBackendAssessments(cfg.BackendAssessments)
}

func validateRelayTreeStressCase(testCase RelayTreeStressCase) error {
	if testCase.CaseID == "" || testCase.CaseID != strings.TrimSpace(testCase.CaseID) || strings.Contains(testCase.CaseID, "/") {
		return fmt.Errorf("case_id=%q is invalid", testCase.CaseID)
	}
	if !validRelayTreeStressClass(testCase.Class) {
		return fmt.Errorf("case_id=%q case_class=%q is invalid", testCase.CaseID, testCase.Class)
	}
	if !validRelayTreeStressRole(testCase.Role) {
		return fmt.Errorf("case_id=%q case_role=%q is invalid", testCase.CaseID, testCase.Role)
	}
	if !validRelayTreeStressGroupSize(testCase.ExpectedGroupSize) {
		return fmt.Errorf("case_id=%q expected_group_size=%d is invalid", testCase.CaseID, testCase.ExpectedGroupSize)
	}
	if testCase.RequestedRelayDepth > 3 {
		return fmt.Errorf("case_id=%q requested_relay_depth=%d exceeds 3", testCase.CaseID, testCase.RequestedRelayDepth)
	}
	if testCase.Class == RelayTreeStressClassBranch {
		if testCase.BranchWidth < 2 {
			return fmt.Errorf("case_id=%q branch_width=%d must be >= 2", testCase.CaseID, testCase.BranchWidth)
		}
	} else if testCase.BranchWidth != 0 {
		return fmt.Errorf("case_id=%q branch_width is valid only for branch cases", testCase.CaseID)
	}
	if len(testCase.BaseConfig.InitialPlan.Snapshot.MemberIDs) != testCase.ExpectedGroupSize {
		return fmt.Errorf(
			"case_id=%q expected_group_size=%d does not match snapshot members=%d",
			testCase.CaseID,
			testCase.ExpectedGroupSize,
			len(testCase.BaseConfig.InitialPlan.Snapshot.MemberIDs),
		)
	}
	if err := validateRelayTreeStressTags(testCase.StressTags); err != nil {
		return fmt.Errorf("case_id=%q: %w", testCase.CaseID, err)
	}
	if testCase.Role == RelayTreeStressRoleCore {
		if testCase.Class != RelayTreeStressClassNominal || !testCase.Expectation.RequireAllDelivered || testCase.Expectation.AllowProfileRejection {
			return fmt.Errorf("case_id=%q core case must be nominal, require all delivery, and forbid rejection", testCase.CaseID)
		}
	}
	if testCase.Expectation.MinRouteReplans < 0 || testCase.Expectation.MinVersionActivations < 0 {
		return fmt.Errorf("case_id=%q replan expectations must be >= 0", testCase.CaseID)
	}
	if !slices.IsSorted(testCase.Expectation.ExpectedUncoveredMemberIDs) || hasDuplicateUint16(testCase.Expectation.ExpectedUncoveredMemberIDs) {
		return fmt.Errorf("case_id=%q expected uncovered member IDs must be sorted and unique", testCase.CaseID)
	}
	for _, id := range testCase.Expectation.ExpectedUncoveredMemberIDs {
		if !slices.Contains(testCase.BaseConfig.InitialPlan.Snapshot.MemberIDs, id) {
			return fmt.Errorf("case_id=%q expected uncovered member_id=%d is not expected", testCase.CaseID, id)
		}
	}
	if err := validateScheduledRelayTreeScenarioConfig(testCase.BaseConfig); err != nil {
		return fmt.Errorf("case_id=%q base config: %w", testCase.CaseID, err)
	}
	if err := validateTimingProfileOverlay(testCase.BaseConfig, testCase.TimingOverlay); err != nil {
		return fmt.Errorf("case_id=%q timing overlay: %w", testCase.CaseID, err)
	}
	return nil
}

func validateRelayTreeStressTags(tags []RelayTreeStressTag) error {
	if len(tags) == 0 {
		return fmt.Errorf("stress_tags must not be empty")
	}
	if !slices.IsSorted(tags) {
		return fmt.Errorf("stress_tags must be sorted")
	}
	for i, tag := range tags {
		if !validRelayTreeStressTag(tag) {
			return fmt.Errorf("stress_tags[%d]=%q is invalid", i, tag)
		}
		if i > 0 && tag == tags[i-1] {
			return fmt.Errorf("stress_tags contain duplicate %q", tag)
		}
	}
	return nil
}

func validateRelayTreeBackendAssessments(assessments []RelayTreeBackendAssessment) error {
	seen := make(map[string]struct{}, len(assessments))
	for i, assessment := range assessments {
		if !validStressTimingStrategy(assessment.TimingStrategy) || !validStressTimingProfile(assessment.Profile) {
			return fmt.Errorf("relay-tree stress matrix: backend_assessments[%d] profile is invalid", i)
		}
		profileID := relayTreeStressProfileID(assessment.TimingStrategy, assessment.Profile)
		if _, ok := seen[profileID]; ok {
			return fmt.Errorf("relay-tree stress matrix: duplicate backend assessment for %s", profileID)
		}
		seen[profileID] = struct{}{}
		switch assessment.Status {
		case RelayTreeBackendNotEvaluated:
			if assessment.Evidence != nil || strings.TrimSpace(assessment.Reason) != "" {
				return fmt.Errorf("relay-tree stress matrix: backend %s not_evaluated cannot carry evidence or reason", profileID)
			}
		case RelayTreeBackendFailed:
			if strings.TrimSpace(assessment.Reason) == "" {
				return fmt.Errorf("relay-tree stress matrix: backend %s failed status requires reason", profileID)
			}
			if assessment.Reason != strings.TrimSpace(assessment.Reason) {
				return fmt.Errorf("relay-tree stress matrix: backend %s reason must be trimmed", profileID)
			}
		case RelayTreeBackendPassed:
			if assessment.Evidence == nil {
				return fmt.Errorf("relay-tree stress matrix: backend %s passed status requires bench evidence", profileID)
			}
			if err := ValidateScenarioEvidence(*assessment.Evidence); err != nil {
				return fmt.Errorf("relay-tree stress matrix: backend %s evidence: %w", profileID, err)
			}
			if assessment.Evidence.EvidenceGrade != EvidenceGradeBenchMeasured || !assessment.Evidence.HardwareClaimAllowed {
				return fmt.Errorf("relay-tree stress matrix: backend %s passed status requires hardware-claimable bench evidence", profileID)
			}
		default:
			return fmt.Errorf("relay-tree stress matrix: backend %s status=%q is invalid", profileID, assessment.Status)
		}
	}
	return nil
}

func validRelayTreeStressClass(class RelayTreeStressCaseClass) bool {
	switch class {
	case RelayTreeStressClassNominal,
		RelayTreeStressClassBranch,
		RelayTreeStressClassAsymmetric,
		RelayTreeStressClassPartition,
		RelayTreeStressClassMembership,
		RelayTreeStressClassLink,
		RelayTreeStressClassRelay,
		RelayTreeStressClassCombined:
		return true
	default:
		return false
	}
}

func validRelayTreeStressRole(role RelayTreeStressCaseRole) bool {
	return role == RelayTreeStressRoleCore || role == RelayTreeStressRoleResilience || role == RelayTreeStressRoleBoundary
}

func validRelayTreeStressGroupSize(size int) bool {
	return size == 2 || size == 4 || size == 8 || size == 16
}

func validRelayTreeStressTag(tag RelayTreeStressTag) bool {
	switch tag {
	case RelayTreeStressTagNominal,
		RelayTreeStressTagDirect,
		RelayTreeStressTagChain,
		RelayTreeStressTagBranch,
		RelayTreeStressTagAsymmetricLink,
		RelayTreeStressTagPartition,
		RelayTreeStressTagRecovery,
		RelayTreeStressTagStaleMembership,
		RelayTreeStressTagMissingMember,
		RelayTreeStressTagLateJoin,
		RelayTreeStressTagLinkLoss,
		RelayTreeStressTagLinkDelay,
		RelayTreeStressTagClockDrift,
		RelayTreeStressTagRelayBusy,
		RelayTreeStressTagRelayLowBattery,
		RelayTreeStressTagRelayLoss,
		RelayTreeStressTagLeaderLoss:
		return true
	default:
		return false
	}
}

func validStressTimingStrategy(strategy timing.TimingStrategy) bool {
	return strategy == timing.TimingStrategySourceLed || strategy == timing.TimingStrategyGroupLeader
}

func validStressTimingProfile(profile TimingProfile) bool {
	return profile == TimingProfileProjectNative || profile == TimingProfileDMRInspired30MS
}

func hasDuplicateUint16(values []uint16) bool {
	for i := 1; i < len(values); i++ {
		if values[i] == values[i-1] {
			return true
		}
	}
	return false
}
