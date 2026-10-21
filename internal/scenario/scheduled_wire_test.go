package scenario

import (
	"github.com/rmsff/ad-hoc-radio/internal/peer"
	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
	"github.com/rmsff/ad-hoc-radio/internal/transport"
	"testing"
)

func candidateVoiceProfile() timing.VoiceBudgetProfile {
	return timing.VoiceBudgetProfile{ID: "codec2-class-1600-80ms-size-fixture", CodecBitrateBPS: 1600, PacketPeriodMS: 80, GrossBitrateBPS: 19200, FECNumerator: 2, FECDenominator: 3, FramingBits: 48, RXToTXUS: 1000, TXToRXUS: 1000, ProcessingUS: 1000, UncertaintyUS: 1000}
}
func TestScheduledWireRejectsSyntheticTenMSWindow(t *testing.T) {
	v := scheduledTestVersion(t, []routing.RelayTreeEdge{{ParentStationID: 1, ChildStationID: 2}}, []uint16{2}, nil, 0, 1, 0, 2)
	cfg := scheduledTestRuntimeConfig(v, 3)
	p := candidateVoiceProfile()
	cfg.VoiceProfile = &p
	if _, err := RunScheduledRuntime(cfg); err == nil {
		t.Fatal("10 ms window accepted for real 30-byte protected packet")
	}
}

func TestScheduledEveryPeerCanSpeakListenAndRelay(t *testing.T) {
	states := []peer.State{}
	links := []transport.Link{}
	directed := scheduledMetricsLinks([2]uint16{1, 2}, [2]uint16{2, 1}, [2]uint16{2, 3}, [2]uint16{3, 2})
	for i := range directed {
		directed[i].Loss = 0
	}
	for _, id := range []uint16{1, 2, 3} {
		states = append(states, peer.State{ID: id, Mode: peer.ADHR, RelayEnabled: true, BatteryFraction: 1, Clock: peer.Clock{MaxUncertaintyMS: 4}})
	}
	for _, l := range directed {
		links = append(links, transport.Link{From: transport.StationID(l.FromStationID), To: transport.StationID(l.ToStationID), Enabled: true, FixedDelayMs: 1})
	}
	fleet, err := peer.NewFleet(states)
	if err != nil {
		t.Fatal(err)
	}
	for turn, source := range []uint16{1, 2, 3} {
		members := []uint16{}
		relays := []routing.RelayState{}
		for _, state := range states {
			if state.ID != source {
				members = append(members, state.ID)
				relays = append(relays, state.RelayState())
			}
		}
		cfg := scheduledMetricsScenarioConfig(ScheduledRelayTreeOneRelayScenarioID, members, directed, relays)
		cfg.SourceStationID = source
		cfg.ScheduleAuthorityID = source
		cfg.TimingAuthorityID = source
		cfg.CallID += uint32(turn)
		cfg.TimingEpochID = cfg.CallID
		cfg.StartMS = 20000 + int64(turn)*40000
		p := candidateVoiceProfile()
		cfg.VoiceProfile = &p
		control := peer.Config{Fleet: fleet, Peers: states, Links: links, StartMS: cfg.StartMS - 20000, RetryMS: 5000, Attempts: 2, Physical: timing.VoiceBudgetProfile{ID: "control-unlimited-simulator", CodecBitrateBPS: 1600, PacketPeriodMS: 80, GrossBitrateBPS: 19200, FECNumerator: 1, FECDenominator: 1}}
		cfg.Control = &control
		result, err := RunScheduledRelayTreeScenario(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if result.OutcomeOverride != "pass" {
			t.Fatalf("source %d not usable: %+v", source, result.ScheduledRelayTree.Audio)
		}
		for _, m := range result.ScheduledMemberDelivery {
			if m.FramesPlayed != int(cfg.FrameCount) {
				t.Fatalf("source=%d listener=%+v", source, m)
			}
		}
		if source != 2 && result.RelayForwarded == 0 {
			t.Fatal("intermediate group member did not relay")
		}
	}
}
