package main

import (
	"encoding/json"
	"github.com/rmsff/ad-hoc-radio/internal/scenario"
	"os"
	"path/filepath"
	"testing"
)

func TestControlComparisonCommand(t *testing.T) {
	file := filepath.Join(t.TempDir(), "report.json")
	if err := run([]string{"control-comparison", "--report", file}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var r scenario.ControlComparison
	if err = json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Rows) != 190 || len(r.SHA256) != 64 || r.Verdict != "no_go_live_voice_hardware" {
		t.Fatal("incorrect command report")
	}
	for _, args := range [][]string{{}, {"--report"}, {"--report", "--bad"}, {"--report", file, "extra"}} {
		if err := runControlComparisonCommand(args); err == nil {
			t.Fatal("invalid args accepted")
		}
	}
}
