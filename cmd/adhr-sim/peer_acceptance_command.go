package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

func runPeerAcceptanceCommand(args []string) error {
	if len(args) != 2 || args[0] != "--report" || args[1] == "" || strings.HasPrefix(args[1], "-") {
		return fmt.Errorf("usage: adhr-sim peer-acceptance --report <path>")
	}
	r, err := scenario.RunPeerAcceptanceMatrix()
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return writeReportFile(args[1], raw)
}
