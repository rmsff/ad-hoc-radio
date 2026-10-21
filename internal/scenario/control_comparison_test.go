package scenario

import (
	"bytes"
	"encoding/json"
	"github.com/rmsff/ad-hoc-radio/internal/peer"
	"testing"
)

func TestControlComparisonCausalityAndDenominators(t *testing.T) {
	r, err := RunControlComparison()
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Rows) != 190 || r.Verdict != "no_go_live_voice_hardware" {
		t.Fatalf("scope %+v", r)
	}
	for i := 0; i < len(r.Rows); i += 2 {
		a, b := r.Rows[i], r.Rows[i+1]
		if a.Format != peer.JSONCRCv0 || b.Format != peer.CompactV1 || a.ExpectedFrames != b.ExpectedFrames || a.SetupAllowanceMS != b.SetupAllowanceMS || a.Admission != b.Admission {
			t.Fatal("unmatched comparison")
		}
	}
	for _, row := range r.Rows {
		if row.PlayedFrames > row.ExpectedFrames || row.ExpectedFrames == 0 {
			t.Fatal("bad denominator")
		}
		if row.ServiceOutcome == "pass" && (row.AllLocalCommitsMS == nil || row.FirstPlayoutFromPTTMS == nil || row.PlayedFrames != row.ExpectedFrames) {
			t.Fatalf("false pass %+v", row)
		}
		if row.AllLocalCommitsMS != nil && (*row.AllLocalCommitsMS >= row.SetupAllowanceMS || len(row.LocalCommits) != row.ExpectedFrames/128+1) {
			t.Fatal("commit telemetry fabricated")
		}
		if row.Admission == "rejected" && (row.FirstPlayoutFromPTTMS != nil || row.SetupControl != nil || row.PlayedFrames != 0) {
			t.Fatal("rejection fabricated runtime")
		}
		if row.Case == "chain" && row.CadenceMS == 80 && row.Depth >= 2 && row.Admission != "rejected" {
			t.Fatal("voice capacity gate relaxed")
		}
		if row.Case == "chain" && row.Depth == 1 && row.GrossBPS == 19200 && row.SetupAllowanceMS == 500 {
			if row.Format == peer.CompactV1 {
				if row.ServiceOutcome != "pass" || *row.AllLocalCommitsMS != 494 || *row.FirstPlayoutFromPTTMS != 660 || row.SetupControl.Bytes != 715 {
					t.Fatalf("compact one-relay evidence %+v", row)
				}
			} else if row.ServiceOutcome != "fail" {
				t.Fatal("JSON completed impossible deadline")
			}
		}
		if row.Case == "proposal_lost" || row.Case == "ack_lost" || row.Case == "commit_lost" || row.Case == "late_commit" || row.Case == "corruption" || row.Case == "l2_limit" {
			if row.ServiceOutcome != "fail" || row.AllLocalCommitsMS != nil {
				t.Fatalf("failure hidden %+v", row)
			}
		}
	}
	again, err := RunControlComparison()
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(r)
	b, _ := json.Marshal(again)
	if !bytes.Equal(a, b) {
		t.Fatal("comparison not deterministic")
	}
}
