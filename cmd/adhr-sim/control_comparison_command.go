package main

import (
	"encoding/json"
	"fmt"
	"github.com/rmsff/ad-hoc-radio/internal/scenario"
	"strings"
)

func runControlComparisonCommand(args []string) error {
	if len(args) != 2 || args[0] != "--report" || args[1] == "" || strings.HasPrefix(args[1], "-") {
		return fmt.Errorf("usage: adhr-sim control-comparison --report <path>")
	}
	r, err := scenario.RunControlComparison()
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return writeReportFile(args[1], raw)
}
