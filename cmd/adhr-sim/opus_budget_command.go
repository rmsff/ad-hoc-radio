package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

func runOpusBudgetCommand(args []string) error {
	if len(args) != 4 || args[0] != "--input" || args[2] != "--report" ||
		args[1] == "" || args[3] == "" || strings.HasPrefix(args[1], "-") || strings.HasPrefix(args[3], "-") {
		return fmt.Errorf("usage: adhr-sim opus-budget --input <trace.json> --report <path>")
	}
	f, err := os.Open(args[1])
	if err != nil {
		return err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, scenario.MaxOpusTraceBytes+1))
	if err != nil {
		return err
	}
	report, err := scenario.RunOpusBudget(raw)
	if err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return writeReportFile(args[3], encoded)
}
