package main

import (
	"encoding/json"
	"github.com/rmsff/ad-hoc-radio/internal/scenario"
	"os"
	"path/filepath"
	"testing"
)

func TestPeerAcceptanceCommand(t *testing.T) {
	for _, args := range [][]string{nil, {"--report"}, {"--report", "--bad"}, {"--report", "x", "extra"}} {
		if err := runPeerAcceptanceCommand(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	p := filepath.Join(t.TempDir(), "matrix.json")
	if err := run([]string{"peer-acceptance", "--report", p}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var report scenario.PeerAcceptanceMatrix
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	if report.TrialCount != 144 || report.SHA256 == "" {
		t.Fatal("missing integrated evidence")
	}
}
