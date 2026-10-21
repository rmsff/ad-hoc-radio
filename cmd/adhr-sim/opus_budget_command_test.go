package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

func TestOpusBudgetCommandRejectsInvalidInput(t *testing.T) {
	for _, args := range [][]string{nil, {"--report", "out"}, {"--input", "x", "--bad", "y"}} {
		if err := runOpusBudgetCommand(args); err == nil || !strings.Contains(err.Error(), "usage:") {
			t.Fatalf("usage: %v", err)
		}
	}
	dir := t.TempDir()
	input, output := filepath.Join(dir, "bad.json"), filepath.Join(dir, "report.json")
	if err := os.WriteFile(input, []byte(`{"version":1,"unknown":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"opus-budget", "--input", input, "--report", output}); err == nil {
		t.Fatal("bad trace accepted")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("report written for malformed input")
	}
}

func TestOpusBudgetCommandReport(t *testing.T) {
	trace := scenario.OpusTrace{Version: 1, Codec: "opus-8000-nb-20ms-cvbr", SourceManifestSHA256: strings.Repeat("a", 64),
		LibrarySHA256: strings.Repeat("b", 64), PacketMS: 20, LookaheadUS: 6500, License: "CC-BY-SA-4.0",
		Attribution: "synthetic CLI test, not audio evidence", Samples: []scenario.OpusTraceSample{{ID: "test", InputSamples: 8000}}}
	for i := 0; i < 51; i++ {
		trace.Samples[0].PacketsHex = append(trace.Samples[0].PacketsHex, strings.Repeat("ab", 20))
	}
	dir := t.TempDir()
	input, output := filepath.Join(dir, "trace.json"), filepath.Join(dir, "report.json")
	raw, _ := json.Marshal(trace)
	if err := os.WriteFile(input, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"opus-budget", "--input", input, "--report", output}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var report scenario.OpusBudgetReport
	if err := json.Unmarshal(data, &report); err != nil || len(report.Rows) != 289 || len(report.SHA256) != 64 ||
		report.Verdict != "no_go_live_voice_hardware" {
		t.Fatalf("invalid report: %v", err)
	}
}
