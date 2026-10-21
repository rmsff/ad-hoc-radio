package scenario

import (
	"reflect"
	"strings"
	"testing"
)

func TestIT023_GroupVoiceProfileSelectsRelaySafeForWholeGroup(t *testing.T) {
	report, err := RunGroupVoiceProfileScenario(DefaultGroupVoiceProfileScenarioConfig())
	if err != nil {
		t.Fatalf("RunGroupVoiceProfileScenario failed: %v", err)
	}

	if report.ScenarioID != GroupVoiceProfileScenarioID {
		t.Fatalf("ScenarioID = %q, want %q", report.ScenarioID, GroupVoiceProfileScenarioID)
	}
	if report.Outcome != GroupVoiceOutcomePass {
		t.Fatalf("Outcome = %q, want %q reason=%q", report.Outcome, GroupVoiceOutcomePass, report.Reason)
	}
	if report.SelectedVoiceProfile != GroupVoiceProfileRelaySafe {
		t.Fatalf("SelectedVoiceProfile = %q, want %q", report.SelectedVoiceProfile, GroupVoiceProfileRelaySafe)
	}
	if report.RequiredRelayDepth != 2 {
		t.Fatalf("RequiredRelayDepth = %d, want 2", report.RequiredRelayDepth)
	}
	assertUint16Slice(t, "ExpectedRecipients", report.ExpectedRecipients, []uint16{2, 3, 4, 5, 6, 7, 8, 9, 10})
	assertUint16Slice(t, "CoveredDirect", report.CoveredDirect, []uint16{8, 9, 10})
	assertUint16Slice(t, "CoveredViaRelay", report.CoveredViaRelay, []uint16{2, 3, 4, 5, 6, 7})
	assertCoverageDepthRows(t, report.CoverageByDepth, []GroupVoiceCoverageDepth{
		{Depth: 1, Members: []uint16{6, 7}},
		{Depth: 2, Members: []uint16{2, 3, 4, 5}},
	})
	assertUint16Slice(t, "UncoveredMembers", report.UncoveredMembers, nil)
	assertUint16Slice(t, "UnknownMembers", report.UnknownMembers, nil)
	assertUint16Slice(t, "ProfileAppliesToMembers", report.ProfileAppliesToMembers, report.ExpectedRecipients)
	if !report.CoveragePass || report.CoverageFraction != 1 {
		t.Fatalf("coverage pass/fraction = %v/%g, want true/1", report.CoveragePass, report.CoverageFraction)
	}

	if len(report.ProfileEvaluations) != 3 {
		t.Fatalf("ProfileEvaluations = %d, want 3", len(report.ProfileEvaluations))
	}
	assertProfileRejected(t, report.ProfileEvaluations[0], GroupVoiceProfileHigh, "relay_depth_exceeded")
	assertProfileRejected(t, report.ProfileEvaluations[1], GroupVoiceProfileBalanced, "relay_depth_exceeded")
	relaySafe := report.ProfileEvaluations[2]
	if !relaySafe.Selected {
		t.Fatal("relay_safe evaluation not marked selected")
	}
	if relaySafe.RequiredRelayDepth != 2 {
		t.Fatalf("relay_safe RequiredRelayDepth = %d, want 2", relaySafe.RequiredRelayDepth)
	}
	if relaySafe.CycleSpareMs != 45 || relaySafe.EndToEndLatencyMs != 35 {
		t.Fatalf("relay_safe timing spare/latency = %d/%d, want 45/35", relaySafe.CycleSpareMs, relaySafe.EndToEndLatencyMs)
	}
	if relaySafe.SourceFrameBytes != 22 || relaySafe.RelayFrameBytes != 22 {
		t.Fatalf("relay_safe bytes source/relay = %d/%d, want 22/22", relaySafe.SourceFrameBytes, relaySafe.RelayFrameBytes)
	}
	if report.EventHash == "" {
		t.Fatal("EventHash is empty")
	}

	t.Logf("group voice profile: selected=%s depth=%d coverage=%g latency=%dms spare=%dms hash=%s",
		report.SelectedVoiceProfile, report.RequiredRelayDepth, report.CoverageFraction,
		relaySafe.EndToEndLatencyMs, relaySafe.CycleSpareMs, report.EventHash)
}

func TestIT023_GroupVoiceProfileKeepsHighWhenDirectCoversGroup(t *testing.T) {
	cfg := DefaultGroupVoiceProfileScenarioConfig()
	cfg.SourceDirectReachable = []uint16{2, 3, 4, 5, 6, 7, 8, 9, 10}
	cfg.RelayCoverage = nil

	report, err := RunGroupVoiceProfileScenario(cfg)
	if err != nil {
		t.Fatalf("RunGroupVoiceProfileScenario failed: %v", err)
	}

	if report.SelectedVoiceProfile != GroupVoiceProfileHigh {
		t.Fatalf("SelectedVoiceProfile = %q, want %q", report.SelectedVoiceProfile, GroupVoiceProfileHigh)
	}
	if report.RequiredRelayDepth != 0 {
		t.Fatalf("RequiredRelayDepth = %d, want 0", report.RequiredRelayDepth)
	}
	assertUint16Slice(t, "CoveredDirect", report.CoveredDirect, []uint16{2, 3, 4, 5, 6, 7, 8, 9, 10})
	assertUint16Slice(t, "CoveredViaRelay", report.CoveredViaRelay, nil)
	if report.ProfileEvaluations[0].ProfileName != GroupVoiceProfileHigh || !report.ProfileEvaluations[0].Selected {
		t.Fatalf("first evaluation = %+v, want selected high", report.ProfileEvaluations[0])
	}
	if report.ProfileEvaluations[0].EndToEndLatencyMs != 25 {
		t.Fatalf("high direct latency = %d, want 25", report.ProfileEvaluations[0].EndToEndLatencyMs)
	}
}

func TestIT023_GroupVoiceProfileReportsUncoveredAndUnknownMembers(t *testing.T) {
	cfg := DefaultGroupVoiceProfileScenarioConfig()
	cfg.ExpectedMembers = []uint16{1, 2, 3, 4, 5}
	cfg.SourceDirectReachable = []uint16{2}
	cfg.RelayCoverage = []GroupVoiceRelayCoverage{{RelayPath: []uint16{20}, Members: []uint16{3}}}
	cfg.UnreachableMembers = []uint16{4}

	report, err := RunGroupVoiceProfileScenario(cfg)
	if err != nil {
		t.Fatalf("RunGroupVoiceProfileScenario failed: %v", err)
	}

	if report.Outcome != GroupVoiceOutcomeDegraded {
		t.Fatalf("Outcome = %q, want %q", report.Outcome, GroupVoiceOutcomeDegraded)
	}
	if report.Reason != groupVoiceReasonNotAllMembersCovered {
		t.Fatalf("Reason = %q, want %q", report.Reason, groupVoiceReasonNotAllMembersCovered)
	}
	if report.SelectedVoiceProfile != "" {
		t.Fatalf("SelectedVoiceProfile = %q, want empty when group is not fully covered", report.SelectedVoiceProfile)
	}
	if report.CoveragePass {
		t.Fatal("CoveragePass = true, want false")
	}
	assertUint16Slice(t, "CoveredDirect", report.CoveredDirect, []uint16{2})
	assertUint16Slice(t, "CoveredViaRelay", report.CoveredViaRelay, []uint16{3})
	assertUint16Slice(t, "UncoveredMembers", report.UncoveredMembers, []uint16{4})
	assertUint16Slice(t, "UnknownMembers", report.UnknownMembers, []uint16{5})
}

func TestIT023_GroupVoiceProfileDeterministicReplay(t *testing.T) {
	cfg := DefaultGroupVoiceProfileScenarioConfig()
	first, err := RunGroupVoiceProfileScenario(cfg)
	if err != nil {
		t.Fatalf("first RunGroupVoiceProfileScenario failed: %v", err)
	}
	second, err := RunGroupVoiceProfileScenario(cfg)
	if err != nil {
		t.Fatalf("second RunGroupVoiceProfileScenario failed: %v", err)
	}

	if first.EventHash != second.EventHash {
		t.Fatalf("EventHash mismatch:\nfirst=%s\nsecond=%s", first.EventHash, second.EventHash)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("same config produced different reports:\nfirst=%+v\nsecond=%+v", first, second)
	}
}

func TestIT023_GroupVoiceProfileRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  GroupVoiceProfileScenarioConfig
		want string
	}{
		{name: "source missing from expected members", cfg: func() GroupVoiceProfileScenarioConfig {
			cfg := DefaultGroupVoiceProfileScenarioConfig()
			cfg.ExpectedMembers = []uint16{2, 3, 4}
			return cfg
		}(), want: "source_id"},
		{name: "coverage fraction out of range", cfg: func() GroupVoiceProfileScenarioConfig {
			cfg := DefaultGroupVoiceProfileScenarioConfig()
			cfg.RequiredGroupCoverage = 1.1
			return cfg
		}(), want: "required_group_coverage"},
		{name: "relay path empty", cfg: func() GroupVoiceProfileScenarioConfig {
			cfg := DefaultGroupVoiceProfileScenarioConfig()
			cfg.RelayCoverage[0].RelayPath = nil
			return cfg
		}(), want: "relay_path"},
		{name: "relay-covered and unreachable conflict", cfg: func() GroupVoiceProfileScenarioConfig {
			cfg := DefaultGroupVoiceProfileScenarioConfig()
			cfg.UnreachableMembers = []uint16{6}
			return cfg
		}(), want: "both relay-covered and unreachable"},
		{name: "duplicate profile", cfg: func() GroupVoiceProfileScenarioConfig {
			cfg := DefaultGroupVoiceProfileScenarioConfig()
			cfg.Profiles[1].Name = cfg.Profiles[0].Name
			return cfg
		}(), want: "duplicate profile"},
		{name: "negative guard", cfg: func() GroupVoiceProfileScenarioConfig {
			cfg := DefaultGroupVoiceProfileScenarioConfig()
			cfg.Profiles[0].GuardTimeMs = -1
			return cfg
		}(), want: "guard"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RunGroupVoiceProfileScenario(tc.cfg)
			if err == nil {
				t.Fatal("RunGroupVoiceProfileScenario accepted invalid config")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want context %q", err.Error(), tc.want)
			}
		})
	}
}

func assertProfileRejected(t *testing.T, evaluation GroupVoiceProfileEvaluation, profileName, reason string) {
	t.Helper()
	if evaluation.ProfileName != profileName {
		t.Fatalf("ProfileName = %q, want %q", evaluation.ProfileName, profileName)
	}
	if evaluation.Selected {
		t.Fatalf("%s selected unexpectedly", profileName)
	}
	for _, got := range evaluation.RejectReasons {
		if got == reason {
			return
		}
	}
	t.Fatalf("%s RejectReasons = %v, want %q", profileName, evaluation.RejectReasons, reason)
}

func assertCoverageDepthRows(t *testing.T, got, want []GroupVoiceCoverageDepth) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CoverageByDepth = %+v, want %+v", got, want)
	}
}
