package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/peer"
	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

func TestPeerControlJSONToCLIUsesBoundWireAndActualControl(t *testing.T) {
	bytesByFormat := map[peer.WireFormat]int{}
	for _, format := range []peer.WireFormat{"", peer.CompactV1, peer.JSONCRCv0} {
		for _, loss := range []float64{0, 100} {
			name := "healthy"
			if loss == 100 {
				name = "all_control_lost"
			}
			t.Run(string(format)+"/"+name, func(t *testing.T) {
				raw, err := os.ReadFile(scheduledRelayTreeFixturePath(t, "one-relay.json"))
				if err != nil {
					t.Fatal(err)
				}
				var d scheduledRelayTreeScenarioDTO
				if err := json.Unmarshal(raw, &d); err != nil {
					t.Fatal(err)
				}
				*d.StartMS, *d.FrameCount = 20000, 8
				d.InitialPlan.Snapshot.ObservedAtMS = 0
				d.InitialPlan.Snapshot.FreshnessMS = 1000000
				d.InitialPlan.Snapshot.MemberIDs = []uint16{2, 10}
				d.InitialPlan.Graph.FreshnessMS = 1000000
				d.InitialPlan.Constraints.PlanningAtMS = 0
				d.Runtime.MissedParentThreshold = 1000
				for i := range d.InitialPlan.Graph.Links {
					d.InitialPlan.Graph.Links[i].ObservedAtMS = 0
					d.InitialPlan.Graph.Links[i].Loss = 0
				}
				p := scenario.PeerAcceptanceVoiceProfile(19200)
				d.VoiceProfile = &p
				d.Control = &scheduledPeerControlDTO{Physical: p, RetryMS: 5000, Attempts: 2, Seed: 42}
				d.Control.WireFormat = format
				for _, id := range []uint16{1, 2, 10} {
					d.Control.Peers = append(d.Control.Peers, peer.State{ID: id, Mode: peer.ADHR,
						RelayEnabled: true, BatteryFraction: 1, Clock: peer.Clock{MaxUncertaintyMS: 1}})
				}
				for _, edge := range [][2]uint16{{1, 10}, {10, 1}, {10, 2}, {2, 10}} {
					d.Control.Links = append(d.Control.Links, scheduledTransportLinkDTO{
						From: edge[0], To: edge[1], Enabled: true, FixedDelayMS: 1, LossPct: loss})
				}
				raw, err = json.Marshal(d)
				if err != nil {
					t.Fatal(err)
				}
				dir := t.TempDir()
				input, output := filepath.Join(dir, "scenario.json"), filepath.Join(dir, "metrics.json")
				if err := os.WriteFile(input, raw, 0600); err != nil {
					t.Fatal(err)
				}
				if err := run([]string{"run", input, "--metrics", output}); err != nil {
					t.Fatal(err)
				}
				raw, err = os.ReadFile(output)
				if err != nil {
					t.Fatal(err)
				}
				var report scenario.MetricsReport
				if err := json.Unmarshal(raw, &report); err != nil {
					t.Fatal(err)
				}
				got := report.ScheduledRelayTree
				if got == nil || got.Control == nil || got.Control.Attempts == 0 || got.Wire == nil || got.Wire.Budget.WireBytes != 36 {
					t.Fatalf("CLI dropped control/profile: %+v", got)
				}
				if len(got.Audio.Members) != 2 {
					t.Fatalf("member denominator: %+v", got.Audio)
				}
				for _, m := range got.Audio.Members {
					want := 8
					if loss == 100 {
						want = 0
					}
					if m.FramesExpected != 8 || m.FramesPlayed != want || m.FramesMissing != 8-want {
						t.Fatalf("loss=%v wrong delivery: %+v", loss, m)
					}
				}
				if loss == 100 {
					if report.SourceFramesSent != 0 || got.Audio.ServiceOutcome != "fail" || got.FirstPlayoutMS != nil {
						t.Fatal("lost control silently bypassed")
					}
				} else if report.Outcome != "pass" || got.Audio.ServiceOutcome != "pass" {
					t.Fatalf("healthy CLI service failed: %+v", got)
				}
				if loss == 0 {
					bytesByFormat[format] = got.Control.Bytes
				}
			})
		}
	}
	if bytesByFormat[""] == 0 || bytesByFormat[""] != bytesByFormat[peer.CompactV1] || bytesByFormat[peer.JSONCRCv0] < 3*bytesByFormat[peer.CompactV1] {
		t.Fatalf("CLI encoding ignored: %+v", bytesByFormat)
	}
}
