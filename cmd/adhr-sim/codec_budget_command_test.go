package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCodecBudgetCommandRejectsInvalidInput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "bad.json")
	report := filepath.Join(dir, "report.json")
	if err := os.WriteFile(input, []byte(`{"codec":"invented"}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{nil, {"--input", input}, {"--input", input, "--report", report}} {
		if err := runCodecBudgetCommand(args); err == nil {
			t.Fatal("accepted bad command")
		}
	}
	if _, err := os.Stat(report); !os.IsNotExist(err) {
		t.Fatal("invalid input produced report", err)
	}
}
