package scenario

import (
	"reflect"
	"strings"
	"testing"
)

func TestIT024_GroupVoiceDeliveryDeliversToMixedGroup(t *testing.T) {
	report, err := RunGroupVoiceDeliveryScenario(DefaultGroupVoiceDeliveryScenarioConfig())
	if err != nil {
		t.Fatalf("RunGroupVoiceDeliveryScenario failed: %v", err)
	}

	if report.Outcome != GroupVoiceOutcomePass {
		t.Fatalf("Outcome = %q, want %q reasons=%v", report.Outcome, GroupVoiceOutcomePass, report.FailReasons)
	}
	if report.SelectedVoiceProfile != GroupVoiceProfileRelaySafe {
		t.Fatalf("SelectedVoiceProfile = %q, want %q", report.SelectedVoiceProfile, GroupVoiceProfileRelaySafe)
	}
	if report.ProfileReport.RequiredRelayDepth != 2 {
		t.Fatalf("RequiredRelayDepth = %d, want 2", report.ProfileReport.RequiredRelayDepth)
	}
	if report.SourceFramesCreated != report.FrameCount {
		t.Fatalf("SourceFramesCreated = %d, want %d", report.SourceFramesCreated, report.FrameCount)
	}
	if report.SourceTransmissions != report.FrameCount {
		t.Fatalf("SourceTransmissions = %d, want %d", report.SourceTransmissions, report.FrameCount)
	}
	if report.RelayTransmissionCount != report.FrameCount*2 {
		t.Fatalf("RelayTransmissionCount = %d, want %d", report.RelayTransmissionCount, report.FrameCount*2)
	}
	if report.EventCount != report.FrameCount*11 {
		t.Fatalf("EventCount = %d, want %d", report.EventCount, report.FrameCount*11)
	}
	if report.EventHash == "" {
		t.Fatal("EventHash is empty")
	}
	assertRelayTransmissionRows(t, report.RelayTransmissions, []GroupVoiceRelayTransmission{
		{RelayID: 20, FramesForwarded: report.FrameCount, RelayDepth: 1, SelectedVoiceProfile: GroupVoiceProfileRelaySafe},
		{RelayID: 21, FramesForwarded: report.FrameCount, RelayDepth: 2, SelectedVoiceProfile: GroupVoiceProfileRelaySafe},
	})
	if len(report.ReceiverReports) != len(report.ProfileReport.ExpectedRecipients) {
		t.Fatalf("ReceiverReports = %d, want %d", len(report.ReceiverReports), len(report.ProfileReport.ExpectedRecipients))
	}

	for _, receiverID := range []uint16{8, 9, 10} {
		row := requireReceiverReport(t, report, receiverID)
		assertReceiverCounters(t, row, report.FrameCount, report.FrameCount, 0)
		if row.CoverageDepth != 0 {
			t.Fatalf("receiver %d CoverageDepth = %d, want 0", receiverID, row.CoverageDepth)
		}
	}
	for _, receiverID := range []uint16{6, 7} {
		row := requireReceiverReport(t, report, receiverID)
		assertReceiverCounters(t, row, report.FrameCount, 0, report.FrameCount)
		if row.CoverageDepth != 1 || !reflect.DeepEqual(row.RelayPath, []uint16{20}) {
			t.Fatalf("receiver %d path depth/path = %d/%v, want 1/[20]", receiverID, row.CoverageDepth, row.RelayPath)
		}
	}
	for _, receiverID := range []uint16{2, 3, 4, 5} {
		row := requireReceiverReport(t, report, receiverID)
		assertReceiverCounters(t, row, report.FrameCount, 0, report.FrameCount)
		if row.CoverageDepth != 2 || !reflect.DeepEqual(row.RelayPath, []uint16{20, 21}) {
			t.Fatalf("receiver %d path depth/path = %d/%v, want 2/[20 21]", receiverID, row.CoverageDepth, row.RelayPath)
		}
	}

	t.Logf("group voice delivery: receivers=%d source_tx=%d relay_tx=%d events=%d hash=%s",
		len(report.ReceiverReports), report.SourceTransmissions, report.RelayTransmissionCount, report.EventCount, report.EventHash)
}

func TestIT024_GroupVoiceDeliveryDirectOnlyKeepsHighProfile(t *testing.T) {
	cfg := DefaultGroupVoiceDeliveryScenarioConfig()
	cfg.ProfileConfig.SourceDirectReachable = []uint16{2, 3, 4, 5, 6, 7, 8, 9, 10}
	cfg.ProfileConfig.RelayCoverage = nil

	report, err := RunGroupVoiceDeliveryScenario(cfg)
	if err != nil {
		t.Fatalf("RunGroupVoiceDeliveryScenario failed: %v", err)
	}

	if report.Outcome != GroupVoiceOutcomePass {
		t.Fatalf("Outcome = %q, want pass reasons=%v", report.Outcome, report.FailReasons)
	}
	if report.SelectedVoiceProfile != GroupVoiceProfileHigh {
		t.Fatalf("SelectedVoiceProfile = %q, want %q", report.SelectedVoiceProfile, GroupVoiceProfileHigh)
	}
	if report.RelayTransmissionCount != 0 || len(report.RelayTransmissions) != 0 {
		t.Fatalf("relay transmissions = %d/%v, want none", report.RelayTransmissionCount, report.RelayTransmissions)
	}
	if report.EventCount != report.FrameCount*9 {
		t.Fatalf("EventCount = %d, want %d", report.EventCount, report.FrameCount*9)
	}
	for _, row := range report.ReceiverReports {
		assertReceiverCounters(t, row, report.FrameCount, report.FrameCount, 0)
		if row.SelectedVoiceProfile != GroupVoiceProfileHigh {
			t.Fatalf("receiver %d SelectedVoiceProfile = %q, want high", row.ReceiverID, row.SelectedVoiceProfile)
		}
	}
}

func TestIT024_GroupVoiceDeliveryCoverageGapsStayDegraded(t *testing.T) {
	cfg := DefaultGroupVoiceDeliveryScenarioConfig()
	cfg.ProfileConfig.ExpectedMembers = []uint16{1, 2, 3, 4, 5}
	cfg.ProfileConfig.SourceDirectReachable = []uint16{2}
	cfg.ProfileConfig.RelayCoverage = []GroupVoiceRelayCoverage{{RelayPath: []uint16{20}, Members: []uint16{3}}}
	cfg.ProfileConfig.UnreachableMembers = []uint16{4}

	report, err := RunGroupVoiceDeliveryScenario(cfg)
	if err != nil {
		t.Fatalf("RunGroupVoiceDeliveryScenario failed: %v", err)
	}

	if report.Outcome != GroupVoiceOutcomeDegraded {
		t.Fatalf("Outcome = %q, want degraded", report.Outcome)
	}
	if report.EventCount != 0 || len(report.ReceiverReports) != 0 {
		t.Fatalf("events/receivers = %d/%d, want 0/0 for degraded no-delivery proof", report.EventCount, len(report.ReceiverReports))
	}
	assertUint16Slice(t, "UncoveredMembers", report.ProfileReport.UncoveredMembers, []uint16{4})
	assertUint16Slice(t, "UnknownMembers", report.ProfileReport.UnknownMembers, []uint16{5})
}

func TestIT024_GroupVoiceDeliveryDeterministicReplay(t *testing.T) {
	cfg := DefaultGroupVoiceDeliveryScenarioConfig()
	first, err := RunGroupVoiceDeliveryScenario(cfg)
	if err != nil {
		t.Fatalf("first RunGroupVoiceDeliveryScenario failed: %v", err)
	}
	second, err := RunGroupVoiceDeliveryScenario(cfg)
	if err != nil {
		t.Fatalf("second RunGroupVoiceDeliveryScenario failed: %v", err)
	}

	if first.EventHash != second.EventHash {
		t.Fatalf("EventHash mismatch:\nfirst=%s\nsecond=%s", first.EventHash, second.EventHash)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("same config produced different reports:\nfirst=%+v\nsecond=%+v", first, second)
	}
}

func TestIT024_GroupVoiceDeliveryRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  GroupVoiceDeliveryScenarioConfig
		want string
	}{
		{name: "ttl too small", cfg: func() GroupVoiceDeliveryScenarioConfig {
			cfg := DefaultGroupVoiceDeliveryScenarioConfig()
			cfg.TTL = 1
			return cfg
		}(), want: "ttl"},
		{name: "relay depth too deep", cfg: func() GroupVoiceDeliveryScenarioConfig {
			cfg := DefaultGroupVoiceDeliveryScenarioConfig()
			cfg.ProfileConfig.RelayCoverage = []GroupVoiceRelayCoverage{{RelayPath: []uint16{20, 21, 22}, Members: []uint16{2}}}
			cfg.ProfileConfig.SourceDirectReachable = []uint16{3, 4, 5, 6, 7, 8, 9, 10}
			return cfg
		}(), want: "max depth 2"},
		{name: "relay duplicates receiver", cfg: func() GroupVoiceDeliveryScenarioConfig {
			cfg := DefaultGroupVoiceDeliveryScenarioConfig()
			cfg.ProfileConfig.RelayCoverage[0].RelayPath = []uint16{6}
			return cfg
		}(), want: "duplicates receiver_id"},
		{name: "unsupported source wire mode", cfg: func() GroupVoiceDeliveryScenarioConfig {
			cfg := DefaultGroupVoiceDeliveryScenarioConfig()
			cfg.ProfileConfig.Profiles[2].SourceWireMode = "mystery"
			return cfg
		}(), want: "source_wire_mode"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RunGroupVoiceDeliveryScenario(tc.cfg)
			if err == nil {
				t.Fatal("RunGroupVoiceDeliveryScenario accepted invalid config")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want context %q", err.Error(), tc.want)
			}
		})
	}
}

func TestIT025_GroupVoiceDeliveryRealisticBudgetStillDelivers(t *testing.T) {
	cfg := DefaultGroupVoiceDeliveryScenarioConfig()
	cfg.ProfileConfig.Profiles = realisticBudgetProfiles(16, 16, 1, 1, 1, 1)

	report, err := RunGroupVoiceDeliveryScenario(cfg)
	if err != nil {
		t.Fatalf("RunGroupVoiceDeliveryScenario failed: %v", err)
	}

	if report.Outcome != GroupVoiceOutcomePass {
		t.Fatalf("Outcome = %q, want pass reasons=%v", report.Outcome, report.FailReasons)
	}
	budget := report.ChannelBudget
	if budget.SourceAirtimeMs != 10 || budget.RelayAirtimeMs != 10 {
		t.Fatalf("airtime source/relay = %d/%d, want 10/10", budget.SourceAirtimeMs, budget.RelayAirtimeMs)
	}
	if budget.SourceTxDurationMs != 12 || budget.RelayTxDurationMs != 12 {
		t.Fatalf("tx duration source/relay = %d/%d, want 12/12", budget.SourceTxDurationMs, budget.RelayTxDurationMs)
	}
	if budget.SourceSlotUsedMs != 13 || budget.RelaySlotUsedMs != 13 {
		t.Fatalf("slot used source/relay = %d/%d, want 13/13", budget.SourceSlotUsedMs, budget.RelaySlotUsedMs)
	}
	if budget.SourceSlotSpareMs != 3 || budget.RelaySlotSpareMs != 3 {
		t.Fatalf("slot spare source/relay = %d/%d, want 3/3", budget.SourceSlotSpareMs, budget.RelaySlotSpareMs)
	}
	if !budget.SourceFitsSlot || !budget.RelayFitsSlot || !budget.RelayReady || !budget.OverallFits {
		t.Fatalf("budget fit flags = source:%t relay:%t ready:%t overall:%t, want all true", budget.SourceFitsSlot, budget.RelayFitsSlot, budget.RelayReady, budget.OverallFits)
	}
	if budget.EndToEndLatencyMs != 45 || budget.CycleSpareMs != 35 {
		t.Fatalf("latency/spare = %d/%d, want 45/35", budget.EndToEndLatencyMs, budget.CycleSpareMs)
	}
	for _, row := range report.ReceiverReports {
		assertReceiverCounters(t, row, report.FrameCount, row.Counters.DirectFramesAccepted, row.Counters.RelayedFramesAccepted)
	}

	second, err := RunGroupVoiceDeliveryScenario(cfg)
	if err != nil {
		t.Fatalf("second RunGroupVoiceDeliveryScenario failed: %v", err)
	}
	if report.EventHash != second.EventHash || !reflect.DeepEqual(report, second) {
		t.Fatalf("realistic budget replay mismatch:\nfirst=%+v\nsecond=%+v", report, second)
	}
}

func TestIT025_GroupVoiceDeliveryTxRxSwitchMissesRelayReadiness(t *testing.T) {
	cfg := DefaultGroupVoiceDeliveryScenarioConfig()
	cfg.ProfileConfig.Profiles = realisticBudgetProfiles(13, 13, 1, 1, 1, 1)

	report, err := RunGroupVoiceDeliveryScenario(cfg)
	if err != nil {
		t.Fatalf("RunGroupVoiceDeliveryScenario failed: %v", err)
	}

	if report.Outcome == GroupVoiceOutcomePass {
		t.Fatalf("Outcome = pass, want non-pass")
	}
	evaluation := requireProfileEvaluation(t, report.ProfileReport, GroupVoiceProfileRelaySafe)
	if evaluation.ChannelBudget.RelayReadinessMissCount != 2 {
		t.Fatalf("RelayReadinessMissCount = %d, want 2", evaluation.ChannelBudget.RelayReadinessMissCount)
	}
	if !stringSliceContains(evaluation.TimingFailReasons, TwoHopLiveFailRelayReadinessMiss) {
		t.Fatalf("TimingFailReasons = %v, want %q", evaluation.TimingFailReasons, TwoHopLiveFailRelayReadinessMiss)
	}
	if !stringSliceContains(report.FailReasons, "profile_selection_not_pass") {
		t.Fatalf("FailReasons = %v, want profile_selection_not_pass", report.FailReasons)
	}
}

func requireReceiverReport(t *testing.T, report GroupVoiceDeliveryReport, receiverID uint16) GroupVoiceReceiverReport {
	t.Helper()
	for _, row := range report.ReceiverReports {
		if row.ReceiverID == receiverID {
			return row
		}
	}
	t.Fatalf("receiver %d report missing", receiverID)
	return GroupVoiceReceiverReport{}
}

func assertReceiverCounters(t *testing.T, row GroupVoiceReceiverReport, frames int, direct int, relayed int) {
	t.Helper()
	if row.Counters.FramesAccepted != frames {
		t.Fatalf("receiver %d FramesAccepted = %d, want %d", row.ReceiverID, row.Counters.FramesAccepted, frames)
	}
	if row.Counters.PlayoutFramesEmitted != frames {
		t.Fatalf("receiver %d PlayoutFramesEmitted = %d, want %d", row.ReceiverID, row.Counters.PlayoutFramesEmitted, frames)
	}
	if row.Counters.DirectFramesAccepted != direct {
		t.Fatalf("receiver %d DirectFramesAccepted = %d, want %d", row.ReceiverID, row.Counters.DirectFramesAccepted, direct)
	}
	if row.Counters.RelayedFramesAccepted != relayed {
		t.Fatalf("receiver %d RelayedFramesAccepted = %d, want %d", row.ReceiverID, row.Counters.RelayedFramesAccepted, relayed)
	}
	if row.Counters.LateFrames != 0 || row.Counters.MissingSequences != 0 {
		t.Fatalf("receiver %d late/missing = %d/%d, want 0/0", row.ReceiverID, row.Counters.LateFrames, row.Counters.MissingSequences)
	}
}

func assertRelayTransmissionRows(t *testing.T, got, want []GroupVoiceRelayTransmission) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RelayTransmissions = %+v, want %+v", got, want)
	}
}

func realisticBudgetProfiles(sourceSlotMs int64, relaySlotMs int64, guardMs int64, preambleMs int64, syncMs int64, txRxSwitchMs int64) []GroupVoiceProfile {
	profiles := DefaultGroupVoiceProfiles()
	for i := range profiles {
		profiles[i].PreambleMs = preambleMs
		profiles[i].SyncMs = syncMs
		profiles[i].TxRxSwitchMs = txRxSwitchMs
		profiles[i].GuardTimeMs = guardMs
	}
	profiles[2].SourceSlotMs = sourceSlotMs
	profiles[2].RelaySlotMs = relaySlotMs
	return profiles
}

func requireProfileEvaluation(t *testing.T, report GroupVoiceProfileReport, profileName string) GroupVoiceProfileEvaluation {
	t.Helper()
	for _, row := range report.ProfileEvaluations {
		if row.ProfileName == profileName {
			return row
		}
	}
	t.Fatalf("profile evaluation %q missing", profileName)
	return GroupVoiceProfileEvaluation{}
}

func stringSliceContains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
