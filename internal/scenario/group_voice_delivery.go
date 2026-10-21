package scenario

import (
	"sort"

	"github.com/rmsff/ad-hoc-radio/internal/channel"
	"github.com/rmsff/ad-hoc-radio/internal/node"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
)

const GroupVoiceDeliveryScenarioID = "group_voice_delivery"

type GroupVoiceDeliveryScenarioConfig struct {
	ScenarioID              string
	Seed                    uint64
	FrameCount              int
	StartMs                 int64
	NetID                   uint16
	CallID                  uint32
	TTL                     uint8
	ReceiverJitterBufferMs  int64
	ReceiverLateThresholdMs int64
	ProfileConfig           GroupVoiceProfileScenarioConfig
}

type GroupVoiceDeliveryReport struct {
	ScenarioID              string
	Seed                    uint64
	FrameCount              int
	GroupID                 uint16
	SourceID                uint16
	SelectedVoiceProfile    string
	ProfileReport           GroupVoiceProfileReport
	SourceFramesCreated     int
	SourceTransmissions     int
	RelayTransmissionCount  int
	RelayTransmissions      []GroupVoiceRelayTransmission
	ReceiverReports         []GroupVoiceReceiverReport
	SlotMissCount           int
	RelayReadinessMissCount int
	ChannelBudget           GroupVoiceChannelBudgetReport
	EventCount              int
	EventHash               string
	Outcome                 string
	FailReasons             []string
}

type GroupVoiceRelayTransmission struct {
	RelayID              uint16
	FramesForwarded      int
	RelayDepth           int
	SelectedVoiceProfile string
}

type GroupVoiceReceiverReport struct {
	ReceiverID           uint16
	CoverageDepth        int
	RelayPath            []uint16
	SelectedVoiceProfile string
	Counters             node.ReceiverCounters
	LastReceivedAtMs     int64
}

type groupVoiceDeliveryRuntime struct {
	cfg                 GroupVoiceDeliveryScenarioConfig
	profile             GroupVoiceProfile
	twoHopCfg           TwoHopLiveScenarioConfig
	queue               *sim.EventQueue
	pending             map[uint64]twoHopLivePendingDelivery
	relays              map[uint16]*node.RelayStation
	receivers           map[uint16]*node.ReceiverStation
	receiverPaths       map[uint16][]uint16
	relayDepths         map[uint16]int
	relayTxCounts       map[uint16]int
	sourceMeasurement   channel.Measurement
	relayMeasurement    channel.Measurement
	channelBudget       GroupVoiceChannelBudgetReport
	lastReceiverRxBySeq map[uint16]map[uint32]int64
	report              GroupVoiceDeliveryReport
}

func DefaultGroupVoiceDeliveryScenarioConfig() GroupVoiceDeliveryScenarioConfig {
	return GroupVoiceDeliveryScenarioConfig{
		ScenarioID:              GroupVoiceDeliveryScenarioID,
		Seed:                    DefaultScenarioSeed,
		FrameCount:              defaultLimitedMultiHopFrameCount,
		StartMs:                 DefaultStartMs,
		NetID:                   DefaultNetID,
		CallID:                  DefaultCallID,
		TTL:                     defaultTwoHopLiveTTL,
		ReceiverJitterBufferMs:  node.DefaultReceiverJitterBufferMs,
		ReceiverLateThresholdMs: node.DefaultReceiverLateThresholdMs,
		ProfileConfig:           DefaultGroupVoiceProfileScenarioConfig(),
	}
}

func RunGroupVoiceDeliveryScenario(cfg GroupVoiceDeliveryScenarioConfig) (GroupVoiceDeliveryReport, error) {
	cfg = normalizeGroupVoiceDeliveryScenarioConfig(cfg)
	if err := validateGroupVoiceDeliveryScenarioConfig(cfg); err != nil {
		return GroupVoiceDeliveryReport{}, err
	}

	profileReport, err := RunGroupVoiceProfileScenario(cfg.ProfileConfig)
	if err != nil {
		return GroupVoiceDeliveryReport{}, err
	}
	selectedProfile, ok := selectedGroupVoiceProfile(cfg.ProfileConfig.Profiles, profileReport.SelectedVoiceProfile)
	if !ok {
		return degradedGroupVoiceDeliveryReport(cfg, profileReport), nil
	}
	twoHopCfg, err := groupVoiceDeliveryTwoHopConfig(cfg, selectedProfile)
	if err != nil {
		return GroupVoiceDeliveryReport{}, err
	}
	sourceMeasurement, relayMeasurement, err := twoHopLiveScenarioMeasurements(twoHopCfg)
	if err != nil {
		return GroupVoiceDeliveryReport{}, err
	}
	channelBudget, err := groupVoiceChannelBudget(selectedProfile, profileReport.RequiredRelayDepth)
	if err != nil {
		return GroupVoiceDeliveryReport{}, err
	}
	receiverPaths := groupVoiceDeliveryReceiverPaths(cfg.ProfileConfig)
	relays, relayDepths := groupVoiceDeliveryRelays(receiverPaths)

	rt := &groupVoiceDeliveryRuntime{
		cfg:                 cfg,
		profile:             selectedProfile,
		twoHopCfg:           twoHopCfg,
		queue:               sim.NewEventQueue(),
		pending:             make(map[uint64]twoHopLivePendingDelivery),
		relays:              relays,
		receivers:           make(map[uint16]*node.ReceiverStation, len(profileReport.ExpectedRecipients)),
		receiverPaths:       receiverPaths,
		relayDepths:         relayDepths,
		relayTxCounts:       make(map[uint16]int, len(relays)),
		sourceMeasurement:   sourceMeasurement,
		relayMeasurement:    relayMeasurement,
		channelBudget:       channelBudget,
		lastReceiverRxBySeq: make(map[uint16]map[uint32]int64, len(profileReport.ExpectedRecipients)),
		report: GroupVoiceDeliveryReport{
			ScenarioID:           cfg.ScenarioID,
			Seed:                 cfg.Seed,
			FrameCount:           cfg.FrameCount,
			GroupID:              cfg.ProfileConfig.GroupID,
			SourceID:             cfg.ProfileConfig.SourceID,
			SelectedVoiceProfile: profileReport.SelectedVoiceProfile,
			ProfileReport:        profileReport,
			ChannelBudget:        channelBudget,
			Outcome:              GroupVoiceOutcomeFail,
		},
	}
	for _, receiverID := range profileReport.ExpectedRecipients {
		rt.receivers[receiverID] = node.NewReceiverStation(receiverID, node.NewReceiverLifecycle(node.DefaultLifecyclePolicy()), groupVoiceDeliveryReceiverPolicy(cfg))
		rt.lastReceiverRxBySeq[receiverID] = make(map[uint32]int64)
	}

	if err := rt.run(); err != nil {
		return GroupVoiceDeliveryReport{}, err
	}
	return rt.report, nil
}

func degradedGroupVoiceDeliveryReport(cfg GroupVoiceDeliveryScenarioConfig, profileReport GroupVoiceProfileReport) GroupVoiceDeliveryReport {
	return GroupVoiceDeliveryReport{
		ScenarioID:    cfg.ScenarioID,
		Seed:          cfg.Seed,
		FrameCount:    cfg.FrameCount,
		GroupID:       cfg.ProfileConfig.GroupID,
		SourceID:      cfg.ProfileConfig.SourceID,
		ProfileReport: profileReport,
		Outcome:       GroupVoiceOutcomeDegraded,
		FailReasons:   []string{"profile_selection_not_pass"},
	}
}

func selectedGroupVoiceProfile(profiles []GroupVoiceProfile, name string) (GroupVoiceProfile, bool) {
	for _, profile := range profiles {
		if profile.Name == name {
			return profile, true
		}
	}
	return GroupVoiceProfile{}, false
}

func groupVoiceDeliveryReceiverPolicy(cfg GroupVoiceDeliveryScenarioConfig) node.ReceiverPolicy {
	return node.ReceiverPolicy{
		StaleThresholdMs: node.DefaultReceiverStaleThresholdMs,
		JitterBufferMs:   cfg.ReceiverJitterBufferMs,
		LateThresholdMs:  cfg.ReceiverLateThresholdMs,
	}
}

func groupVoiceDeliveryRelays(receiverPaths map[uint16][]uint16) (map[uint16]*node.RelayStation, map[uint16]int) {
	relays := make(map[uint16]*node.RelayStation)
	depths := make(map[uint16]int)
	for _, path := range receiverPaths {
		for i, relayID := range path {
			if _, ok := relays[relayID]; !ok {
				relays[relayID] = node.NewRelayStation(relayID, node.NewRelayLifecycle(node.DefaultLifecyclePolicy()), node.DefaultRelayPolicy())
			}
			depth := i + 1
			if existing, ok := depths[relayID]; !ok || depth < existing {
				depths[relayID] = depth
			}
		}
	}
	return relays, depths
}

func groupVoiceDeliveryReceiverPaths(profileCfg GroupVoiceProfileScenarioConfig) map[uint16][]uint16 {
	paths := make(map[uint16][]uint16)
	for _, receiverID := range profileCfg.SourceDirectReachable {
		paths[receiverID] = nil
	}
	for _, coverage := range profileCfg.RelayCoverage {
		for _, receiverID := range coverage.Members {
			existing, ok := paths[receiverID]
			if ok && len(existing) <= len(coverage.RelayPath) {
				continue
			}
			paths[receiverID] = cloneGroupVoiceUint16s(coverage.RelayPath)
		}
	}
	return paths
}

func groupVoiceDeliveryTwoHopConfig(cfg GroupVoiceDeliveryScenarioConfig, profile GroupVoiceProfile) (TwoHopLiveScenarioConfig, error) {
	twoHopCfg := TwoHopLiveScenarioConfig{
		ScenarioID:              cfg.ScenarioID,
		Seed:                    cfg.Seed,
		FrameCount:              cfg.FrameCount,
		StartMs:                 cfg.StartMs,
		VoicePeriodMs:           profile.VoicePeriodMs,
		SourceSlotMs:            profile.SourceSlotMs,
		RelaySlotMs:             profile.RelaySlotMs,
		GuardTimeMs:             profile.GuardTimeMs,
		LinkDelayMs:             profile.LinkDelayMs,
		ChannelBitrateBps:       profile.ChannelBitrateBps,
		SourceFrameBytes:        profile.SourceFrameBytes,
		RelayFrameBytes:         profile.RelayFrameBytes,
		SourceWireMode:          profile.SourceWireMode,
		LatencyBudgetMs:         profile.LatencyBudgetMs,
		SourceID:                cfg.ProfileConfig.SourceID,
		Relay1ID:                1,
		Relay2ID:                2,
		ReceiverID:              3,
		NetID:                   cfg.NetID,
		GroupID:                 cfg.ProfileConfig.GroupID,
		CallID:                  cfg.CallID,
		TTL:                     cfg.TTL,
		ReceiverJitterBufferMs:  cfg.ReceiverJitterBufferMs,
		ReceiverLateThresholdMs: cfg.ReceiverLateThresholdMs,
	}
	if _, err := twoHopLiveSourcePayloadBytes(twoHopCfg); err != nil {
		return TwoHopLiveScenarioConfig{}, err
	}
	return twoHopCfg, nil
}

func sortedGroupVoiceDeliveryReceiverIDs(receivers map[uint16]*node.ReceiverStation) []uint16 {
	ids := make([]uint16, 0, len(receivers))
	for id := range receivers {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func sortedGroupVoiceRelayIDs(relays map[uint16]*node.RelayStation) []uint16 {
	ids := make([]uint16, 0, len(relays))
	for id := range relays {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func groupVoiceDeliveryFailReasons(report GroupVoiceDeliveryReport) []string {
	reasons := make([]string, 0, 5)
	if report.ProfileReport.Outcome != GroupVoiceOutcomePass {
		reasons = append(reasons, "profile_selection_not_pass")
	}
	if report.ChannelBudget.ChannelBitrateBps > 0 && !report.ChannelBudget.OverallFits {
		reasons = append(reasons, "slot_budget_exceeded")
	}
	if report.SlotMissCount > 0 {
		reasons = append(reasons, "slot_miss")
	}
	if report.RelayReadinessMissCount > 0 {
		reasons = append(reasons, "relay_readiness_miss")
	}
	for _, receiver := range report.ReceiverReports {
		if receiver.Counters.FramesAccepted != report.FrameCount {
			reasons = append(reasons, "receiver_acceptance_incomplete")
			break
		}
		if receiver.Counters.PlayoutFramesEmitted != report.FrameCount {
			reasons = append(reasons, "playout_incomplete")
			break
		}
	}
	return reasons
}

func unsupportedGroupVoiceDepth(path []uint16) bool {
	return len(path) > 2
}

func groupVoiceReceiverPathDepth(path []uint16) int {
	return len(path)
}
