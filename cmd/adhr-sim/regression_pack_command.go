package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	regressionPackSchemaVersion = 1
	regressionPackHashAlgorithm = "sha256"
	regressionPackSerialization = "regression-pack-summary-v1"
	regressionPackOutcomePass   = "pass"
	regressionPackOutcomeFail   = "fail"
	regressionPackCasePass      = "pass"
	regressionPackCaseFail      = "fail"
)

type regressionPackConfig struct {
	PackID string                 `json:"pack_id"`
	Seed   uint64                 `json:"seed"`
	Cases  []regressionPackCaseIn `json:"cases"`
}

type regressionPackCaseIn struct {
	CaseID                      string                    `json:"case_id"`
	Command                     string                    `json:"command"`
	Input                       string                    `json:"input,omitempty"`
	Inputs                      *regressionPackCaseInputs `json:"inputs,omitempty"`
	Output                      string                    `json:"output"`
	ExpectedOutcome             string                    `json:"expected_outcome"`
	TxRequested                 bool                      `json:"tx_requested,omitempty"`
	LabSafetyDocumented         bool                      `json:"lab_safety_documented,omitempty"`
	RegulatoryProfileDocumented bool                      `json:"regulatory_profile_documented,omitempty"`
}

type regressionPackCaseInputs struct {
	Channel    string `json:"channel,omitempty"`
	Turnaround string `json:"turnaround,omitempty"`
	Audio      string `json:"audio,omitempty"`
}

type regressionPackSummary struct {
	SchemaVersion     int                         `json:"schema_version"`
	PackID            string                      `json:"pack_id"`
	Seed              uint64                      `json:"seed"`
	CaseCount         int                         `json:"case_count"`
	ExpectedPassCount int                         `json:"expected_pass_count"`
	UnexpectedCount   int                         `json:"unexpected_count"`
	Cases             []regressionPackCaseSummary `json:"cases"`
	Determinism       regressionPackDeterminism   `json:"determinism"`
	Outcome           string                      `json:"outcome"`
}

type regressionPackCaseSummary struct {
	CaseID          string                    `json:"case_id"`
	Command         string                    `json:"command"`
	Input           string                    `json:"input,omitempty"`
	Inputs          *regressionPackCaseInputs `json:"inputs,omitempty"`
	Output          string                    `json:"output"`
	ScenarioID      string                    `json:"scenario_id"`
	Outcome         string                    `json:"outcome"`
	ExpectedOutcome string                    `json:"expected_outcome"`
	Status          string                    `json:"status"`
	EventHash       string                    `json:"event_hash"`
}

type regressionPackDeterminism struct {
	EventHash     string `json:"event_hash"`
	HashAlgorithm string `json:"hash_algorithm"`
	Serialization string `json:"serialization"`
}

type regressionPackOutputProbe struct {
	ScenarioID          string                       `json:"scenario_id"`
	Outcome             string                       `json:"outcome"`
	CalibrationDecision string                       `json:"calibration_decision"`
	EventHash           string                       `json:"event_hash"`
	Determinism         regressionPackEventHashProbe `json:"determinism"`
	Evidence            regressionPackEvidenceProbe  `json:"evidence"`
	DegradeReasons      []string                     `json:"degrade_reasons"`
	Notes               string                       `json:"notes"`
	ScenarioOutcome     string                       `json:"scenario_outcome"`
}

type regressionPackEvidenceProbe struct {
	Determinism regressionPackEventHashProbe `json:"determinism"`
	Outcome     string                       `json:"outcome"`
}

type regressionPackEventHashProbe struct {
	EventHash string `json:"event_hash"`
}

func runRegressionPackCommand(args []string) error {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		printRegressionPackUsage(os.Stdout)
		return nil
	}

	packPath, outDir, err := parseRegressionPackArgs(args)
	if err != nil {
		return err
	}
	cfg, err := loadRegressionPackConfig(packPath)
	if err != nil {
		return err
	}
	if err := prepareRegressionPackOutputDir(outDir); err != nil {
		return err
	}
	summary, err := runRegressionPack(packPath, outDir, cfg)
	if err != nil {
		return err
	}
	raw, err := marshalRegressionPackSummaryJSON(summary)
	if err != nil {
		return fmt.Errorf("marshal regression pack summary: %w", err)
	}
	return writeReportFile(filepath.Join(outDir, "summary.json"), raw)
}

func parseRegressionPackArgs(args []string) (string, string, error) {
	var packPath string
	var outDir string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--out":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("regression-pack: --out requires a directory path")
			}
			if strings.HasPrefix(args[i+1], "-") {
				return "", "", fmt.Errorf("regression-pack: --out requires a directory path, got flag-like value %q", args[i+1])
			}
			if outDir != "" {
				return "", "", fmt.Errorf("regression-pack: --out specified more than once")
			}
			outDir = args[i+1]
			i++
		case strings.HasPrefix(arg, "-"):
			return "", "", fmt.Errorf("regression-pack: unknown flag %q", arg)
		default:
			if packPath != "" {
				return "", "", fmt.Errorf("regression-pack: unexpected argument %q", arg)
			}
			packPath = arg
		}
	}
	if packPath == "" {
		return "", "", fmt.Errorf("regression-pack: pack path is required")
	}
	if outDir == "" {
		return "", "", fmt.Errorf("regression-pack: --out directory path is required")
	}
	return packPath, outDir, nil
}

func printRegressionPackUsage(w *os.File) {
	fmt.Fprintln(w, "usage: adhr-sim regression-pack <pack.json> --out <dir>")
}

func loadRegressionPackConfig(path string) (regressionPackConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return regressionPackConfig{}, fmt.Errorf("read regression pack %q: %w", path, err)
	}
	var cfg regressionPackConfig
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return regressionPackConfig{}, fmt.Errorf("decode regression pack %q: %w", path, err)
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return regressionPackConfig{}, fmt.Errorf("decode regression pack %q: trailing JSON token", path)
		}
		return regressionPackConfig{}, fmt.Errorf("decode regression pack %q: %w", path, err)
	}
	if err := validateRegressionPackConfig(cfg); err != nil {
		return regressionPackConfig{}, fmt.Errorf("regression pack %q: %w", path, err)
	}
	return cfg, nil
}

func prepareRegressionPackOutputDir(outDir string) error {
	info, err := os.Stat(outDir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return fmt.Errorf("create regression-pack output directory %q: %w", outDir, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect regression-pack output directory %q: %w", outDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("regression-pack output path %q must be a directory", outDir)
	}
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return fmt.Errorf("read regression-pack output directory %q: %w", outDir, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("regression-pack output directory %q must be empty", outDir)
	}
	return nil
}

func runRegressionPack(packPath string, outDir string, cfg regressionPackConfig) (regressionPackSummary, error) {
	summary := regressionPackSummary{
		SchemaVersion: regressionPackSchemaVersion,
		PackID:        cfg.PackID,
		Seed:          cfg.Seed,
		CaseCount:     len(cfg.Cases),
		Cases:         make([]regressionPackCaseSummary, 0, len(cfg.Cases)),
	}
	for _, c := range cfg.Cases {
		inputPath := regressionPackCaseInputPath(packPath, c)
		outputRel := filepath.ToSlash(filepath.Clean(c.Output))
		outputPath := filepath.Join(outDir, filepath.FromSlash(outputRel))
		if err := runRegressionPackCase(c, inputPath, outDir, outputPath); err != nil {
			return regressionPackSummary{}, err
		}
		row, err := summarizeRegressionPackCase(c, outputRel, outputPath)
		if err != nil {
			return regressionPackSummary{}, err
		}
		if row.Status == regressionPackCasePass {
			summary.ExpectedPassCount++
		} else {
			summary.UnexpectedCount++
		}
		summary.Cases = append(summary.Cases, row)
	}
	if summary.UnexpectedCount == 0 {
		summary.Outcome = regressionPackOutcomePass
	} else {
		summary.Outcome = regressionPackOutcomeFail
	}
	hash, err := regressionPackSummaryHash(summary)
	if err != nil {
		return regressionPackSummary{}, err
	}
	summary.Determinism = regressionPackDeterminism{
		EventHash:     hash,
		HashAlgorithm: regressionPackHashAlgorithm,
		Serialization: regressionPackSerialization,
	}
	return summary, nil
}

func regressionPackCaseInputPath(packPath string, c regressionPackCaseIn) string {
	if strings.TrimSpace(c.Input) == "" {
		return ""
	}
	return filepath.Clean(filepath.Join(filepath.Dir(packPath), c.Input))
}

func runRegressionPackCase(c regressionPackCaseIn, inputPath string, outDir string, outputPath string) error {
	switch c.Command {
	case "run":
		return runScenarioCommand([]string{inputPath, "--metrics", outputPath})
	case "ptt-contention":
		return runPTTContentionCommand([]string{inputPath, "--report", outputPath})
	case "calibrate-channel":
		return runCalibrateChannelCommand([]string{inputPath, "--report", outputPath})
	case "calibrate-tyt":
		return runCalibrateTYTCommand([]string{inputPath, "--report", outputPath})
	case "score-audio":
		return runScoreAudioCommand([]string{inputPath, "--report", outputPath})
	case "hardware-readiness":
		return runHardwareReadinessCommand(regressionPackHardwareReadinessArgs(c, outDir, outputPath))
	default:
		return fmt.Errorf("regression-pack: unsupported command %q", c.Command)
	}
}

func regressionPackHardwareReadinessArgs(c regressionPackCaseIn, outDir string, outputPath string) []string {
	args := []string{
		"--channel", filepath.Join(outDir, filepath.FromSlash(c.Inputs.Channel)),
		"--turnaround", filepath.Join(outDir, filepath.FromSlash(c.Inputs.Turnaround)),
		"--audio", filepath.Join(outDir, filepath.FromSlash(c.Inputs.Audio)),
		"--report", outputPath,
	}
	if c.TxRequested {
		args = append(args, "--tx-requested")
	}
	if c.LabSafetyDocumented {
		args = append(args, "--lab-safety-documented")
	}
	if c.RegulatoryProfileDocumented {
		args = append(args, "--regulatory-profile-documented")
	}
	return args
}

func summarizeRegressionPackCase(c regressionPackCaseIn, outputRel string, outputPath string) (regressionPackCaseSummary, error) {
	probe, err := loadRegressionPackOutputProbe(outputPath)
	if err != nil {
		return regressionPackCaseSummary{}, err
	}
	eventHash := probe.EventHash
	if eventHash == "" {
		eventHash = probe.Determinism.EventHash
	}
	if eventHash == "" {
		eventHash = probe.Evidence.Determinism.EventHash
	}
	if probe.ScenarioID == "" {
		return regressionPackCaseSummary{}, fmt.Errorf("regression-pack case %q output missing scenario_id", c.CaseID)
	}
	outcome := regressionPackProbeOutcome(probe)
	if outcome == "" {
		return regressionPackCaseSummary{}, fmt.Errorf("regression-pack case %q output missing outcome", c.CaseID)
	}
	if eventHash == "" {
		return regressionPackCaseSummary{}, fmt.Errorf("regression-pack case %q output missing event hash", c.CaseID)
	}
	status := regressionPackCaseFail
	if outcome == c.ExpectedOutcome {
		status = regressionPackCasePass
	}
	return regressionPackCaseSummary{
		CaseID:          c.CaseID,
		Command:         c.Command,
		Input:           regressionPackSummaryInput(c),
		Inputs:          regressionPackSummaryInputs(c),
		Output:          outputRel,
		ScenarioID:      probe.ScenarioID,
		Outcome:         outcome,
		ExpectedOutcome: c.ExpectedOutcome,
		Status:          status,
		EventHash:       eventHash,
	}, nil
}

func regressionPackProbeOutcome(probe regressionPackOutputProbe) string {
	switch {
	case probe.Outcome != "":
		return probe.Outcome
	case probe.ScenarioOutcome != "":
		return probe.ScenarioOutcome
	case probe.CalibrationDecision != "":
		return probe.CalibrationDecision
	case probe.Evidence.Outcome != "":
		return probe.Evidence.Outcome
	default:
		return ""
	}
}

func regressionPackSummaryInput(c regressionPackCaseIn) string {
	if strings.TrimSpace(c.Input) == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(c.Input))
}

func regressionPackSummaryInputs(c regressionPackCaseIn) *regressionPackCaseInputs {
	if c.Inputs == nil {
		return nil
	}
	return &regressionPackCaseInputs{
		Channel:    filepath.ToSlash(filepath.Clean(c.Inputs.Channel)),
		Turnaround: filepath.ToSlash(filepath.Clean(c.Inputs.Turnaround)),
		Audio:      filepath.ToSlash(filepath.Clean(c.Inputs.Audio)),
	}
}

func loadRegressionPackOutputProbe(path string) (regressionPackOutputProbe, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return regressionPackOutputProbe{}, fmt.Errorf("read regression-pack output %q: %w", path, err)
	}
	var probe regressionPackOutputProbe
	if err := json.Unmarshal(raw, &probe); err != nil {
		return regressionPackOutputProbe{}, fmt.Errorf("decode regression-pack output %q: %w", path, err)
	}
	return probe, nil
}

func regressionPackSummaryHash(summary regressionPackSummary) (string, error) {
	hashInput := struct {
		SchemaVersion     int                         `json:"schema_version"`
		PackID            string                      `json:"pack_id"`
		Seed              uint64                      `json:"seed"`
		CaseCount         int                         `json:"case_count"`
		ExpectedPassCount int                         `json:"expected_pass_count"`
		UnexpectedCount   int                         `json:"unexpected_count"`
		Cases             []regressionPackCaseSummary `json:"cases"`
		Outcome           string                      `json:"outcome"`
	}{
		SchemaVersion:     summary.SchemaVersion,
		PackID:            summary.PackID,
		Seed:              summary.Seed,
		CaseCount:         summary.CaseCount,
		ExpectedPassCount: summary.ExpectedPassCount,
		UnexpectedCount:   summary.UnexpectedCount,
		Cases:             summary.Cases,
		Outcome:           summary.Outcome,
	}
	raw, err := json.Marshal(hashInput)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func marshalRegressionPackSummaryJSON(summary regressionPackSummary) ([]byte, error) {
	return json.MarshalIndent(summary, "", "  ")
}
