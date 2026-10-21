package scenario

import (
	"strings"
	"testing"
)

func TestPeerAcceptanceMatrixDenominatorsAndCausality(t *testing.T) {
	a, err := RunPeerAcceptanceMatrix()
	if err != nil {
		t.Fatal(err)
	}
	b, err := RunPeerAcceptanceMatrix()
	if err != nil {
		t.Fatal(err)
	}
	if a.SHA256 != b.SHA256 || a.SHA256 == "" {
		t.Fatal("matrix not reproducible")
	}
	if a.TrialCount != 144 || a.FailedTrials == 0 || a.Verdict != "no_go_live_voice_hardware" {
		t.Fatalf("false acceptance: %+v", a)
	}
	for _, r := range a.Rows {
		if r.Evidence != nil && r.CollisionDrops != r.Evidence.CollisionDropCount {
			t.Fatalf("collision summary diverged: %+v", r)
		}
		if r.ExpectedFrames <= 0 || r.PlayedFrames+r.MissingFrames != r.ExpectedFrames {
			t.Fatalf("denominator: %+v", r)
		}
		if r.ServiceOutcome == "pass" && r.PlayedFrames != r.ExpectedFrames {
			t.Fatalf("partial pass: %+v", r)
		}
		switch r.ID {
		case "voice_loss_100", "voice_corruption_100", "control_loss_100", "control_corrupt":
			if r.PlayedFrames != 0 || r.FirstAudioFromPTTMS != nil {
				t.Fatalf("false playout: %+v", r)
			}
		case "voice_duplicate_100", "control_duplicate":
			if r.ServiceOutcome != "pass" || r.Evidence.DuplicatePlayoutCount != 0 {
				t.Fatalf("duplicate handling: %+v", r)
			}
		case "hr_c6000_l2_27_byte_limit":
			if r.Admission != "rejected" {
				t.Fatal("27 byte limit ignored")
			}
		}
		if strings.HasPrefix(r.ID, "budget-4800") && r.Admission != "rejected" {
			t.Fatal("impossible protected 80 ms packet admitted")
		}
	}
	if len(a.StressDiagnostics) != 6 {
		t.Fatal("stress denominator")
	}
	for _, r := range a.StressDiagnostics {
		if r.ServiceOutcome != "fail" || r.MissingFrames == 0 || r.CollisionDrops == 0 {
			t.Fatalf("overload hidden: %+v", r)
		}
	}
}

func TestPeerAcceptanceNegativeTestIsNotServiceSuccess(t *testing.T) {
	c := peerAcceptanceConfig(1, 1, 19200, 42)
	c.ExpectedServiceOutcome = "fail"
	for i := range c.InitialPlan.Graph.Links {
		c.InitialPlan.Graph.Links[i].Loss = 1
	}
	r, err := RunScheduledRelayTreeScenario(c)
	if err != nil {
		t.Fatal(err)
	}
	if r.OutcomeOverride != "pass" || r.ScheduledRelayTree.Audio.ServiceOutcome != "fail" {
		t.Fatal("test and service outcomes conflated")
	}
}
