package scenario

import (
	"github.com/rmsff/ad-hoc-radio/internal/peer"
	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
	"github.com/rmsff/ad-hoc-radio/internal/transport"
	"sort"
)

// This is an explicit size/timing hypothesis, never a codec or radio receipt.
func PeerAcceptanceVoiceProfile(grossBPS int64) timing.VoiceBudgetProfile {
	return timing.VoiceBudgetProfile{ID: "adhr-1600-80ms-size-v1", CodecBitrateBPS: 1600, PacketPeriodMS: 80,
		GrossBitrateBPS: grossBPS, FECNumerator: 2, FECDenominator: 3, FramingBits: 48,
		RXToTXUS: 1000, TXToRXUS: 1000, ProcessingUS: 1000, UncertaintyUS: 1000}
}

func peerAcceptanceConfig(depth uint8, width int, grossBPS int64, seed uint64) ScheduledRelayTreeScenarioConfig {
	p := PeerAcceptanceVoiceProfile(grossBPS)
	c := ScheduledRelayTreeScenarioConfig{ScenarioID: ScheduledRelayTreeBranchWideScenarioID, Seed: seed, FrameCount: 128,
		StartMS: 20000, NetID: 1, GroupID: 100, SourceStationID: 1, CallID: 9001, ScheduleID: 1,
		ScheduleAuthorityID: 1, TimingStrategy: timing.TimingStrategySourceLed, TimingEpochID: 9001, TimingAuthorityID: 1,
		VoiceProfile: &p, ForwardQueueMaxDepth: 8, MissedParentThreshold: 1000}
	controlPhysical := p
	controlPhysical.ID = "explicit-unfragmented-control-hypothesis"
	// Preserve the historical MVP-9.9 matrix as an explicit JSON baseline.
	control := peer.Config{WireFormat: peer.JSONCRCv0, StartMS: 0, RetryMS: 5000, Attempts: 2, Seed: seed, MinBattery: 0.2, Physical: controlPhysical}
	addPeer := func(id uint16) {
		s := peer.State{ID: id, Mode: peer.ADHR, RelayEnabled: true, BatteryFraction: 1, Clock: peer.Clock{MaxUncertaintyMS: 1}}
		control.Peers = append(control.Peers, s)
		if id != 1 {
			c.InitialPlan.Snapshot.MemberIDs = append(c.InitialPlan.Snapshot.MemberIDs, id)
			c.InitialPlan.RelayStates = append(c.InitialPlan.RelayStates, s.RelayState())
		}
	}
	addLink := func(from, to uint16) {
		c.InitialPlan.Graph.Links = append(c.InitialPlan.Graph.Links, routing.DirectedLink{FromStationID: from, ToStationID: to, Enabled: true, Quality: 1, DelayMS: 1, ObservedAtMS: 0})
		control.Links = append(control.Links, transport.Link{From: transport.StationID(from), To: transport.StationID(to), Enabled: true, FixedDelayMs: 1})
	}
	addPeer(1)
	next := uint16(2)
	for branch := 0; branch < width; branch++ {
		parent := uint16(1)
		for hop := uint8(0); hop <= depth; hop++ {
			addPeer(next)
			addLink(parent, next)
			addLink(next, parent)
			parent = next
			next++
		}
	}
	c.InitialPlan.Snapshot.ID = 1
	sort.Slice(c.InitialPlan.Graph.Links, func(i, j int) bool {
		a, b := c.InitialPlan.Graph.Links[i], c.InitialPlan.Graph.Links[j]
		if a.FromStationID != b.FromStationID {
			return a.FromStationID < b.FromStationID
		}
		return a.ToStationID < b.ToStationID
	})
	c.InitialPlan.Snapshot.FreshnessMS = 1000000
	c.InitialPlan.Graph.Epoch = 1
	c.InitialPlan.Graph.FreshnessMS = 1000000
	c.InitialPlan.Constraints = routing.PlanningConstraints{PlanningAtMS: 0, MaxRelayDepth: 3, MaxPlayoutLatencyMS: 160, MaxVoiceAirtimePct: 100, RequireAllExpectedMembers: true, MinRelayBatteryFraction: 0.2, MinLinkQuality: 0.1, MaxLinkLoss: 1}
	c.InitialPlan.SearchLimits = routing.PlanningSearchLimits{MaxPathsPerMember: 128, MaxTreeCandidates: 4096, MaxFeasibleTrees: 4096, MaxSearchSteps: 100000}
	c.Control = &control
	return c
}
