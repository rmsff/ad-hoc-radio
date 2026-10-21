package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

const (
	benchLogArtifactChannel     = "channel_bench_log_template"
	benchLogArtifactTurnaround  = "turnaround_bench_log_template"
	benchLogArtifactAudio       = "audio_bench_log_template"
	benchLogStatusTemplateOnly  = "template_only"
	benchLogStatusCaptured      = "captured"
	benchLogStatusDryRunFixture = "dry_run_fixture"
	benchLogSchemaVersion       = 1
)

type benchLogEnvelope struct {
	SchemaVersion int    `json:"schema_version"`
	ArtifactType  string `json:"artifact_type"`
	Status        string `json:"status"`
}

type channelBenchLog struct {
	SchemaVersion             int     `json:"schema_version"`
	ArtifactType              string  `json:"artifact_type"`
	Status                    string  `json:"status"`
	HardwareClaimAllowed      bool    `json:"hardware_claim_allowed"`
	FieldTXReady              bool    `json:"field_tx_ready"`
	TargetRadioFamily         string  `json:"target_radio_family"`
	Operator                  string  `json:"operator"`
	CapturedAtUTC             string  `json:"captured_at_utc"`
	LabSetupRef               string  `json:"lab_setup_ref"`
	ConvertTo                 string  `json:"convert_to"`
	CandidateChannelWidthHz   []int   `json:"candidate_channel_width_hz"`
	CandidateGrossBitrateBps  []int64 `json:"candidate_gross_bitrate_bps"`
	SelectedChannelWidthHz    int     `json:"selected_channel_width_hz"`
	SelectedGrossBitrateBps   int64   `json:"selected_gross_bitrate_bps"`
	ModulationProfile         string  `json:"modulation_profile"`
	FramesSent                int64   `json:"frames_sent"`
	FramesDecoded             int64   `json:"frames_decoded"`
	CRCErrorCount             int64   `json:"crc_error_count"`
	DurationMs                int64   `json:"duration_ms"`
	MeasurementArtifactRef    string  `json:"measurement_artifact_ref"`
	MeasurementArtifactSHA256 string  `json:"measurement_artifact_sha256"`
	Notes                     string  `json:"notes"`
}

type turnaroundBenchLog struct {
	SchemaVersion             int     `json:"schema_version"`
	ArtifactType              string  `json:"artifact_type"`
	Status                    string  `json:"status"`
	HardwareClaimAllowed      bool    `json:"hardware_claim_allowed"`
	FieldTXReady              bool    `json:"field_tx_ready"`
	TargetRadioFamily         string  `json:"target_radio_family"`
	Operator                  string  `json:"operator"`
	CapturedAtUTC             string  `json:"captured_at_utc"`
	LabSetupRef               string  `json:"lab_setup_ref"`
	ConvertTo                 string  `json:"convert_to"`
	FirmwarePath              string  `json:"firmware_path"`
	ControlPathUnderTest      string  `json:"control_path_under_test"`
	ChannelWidthHz            int     `json:"channel_width_hz"`
	GrossBitrateBps           int64   `json:"gross_bitrate_bps"`
	PreambleMs                int64   `json:"preamble_ms"`
	SyncMs                    int64   `json:"sync_ms"`
	LinkDelayMs               int64   `json:"link_delay_ms"`
	RxToTxSamplesMs           []int64 `json:"rx_to_tx_samples_ms"`
	TxToRxSamplesMs           []int64 `json:"tx_to_rx_samples_ms"`
	RxValidAfterTxSamplesMs   []int64 `json:"rx_valid_after_tx_samples_ms"`
	MeasurementArtifactRef    string  `json:"measurement_artifact_ref"`
	MeasurementArtifactSHA256 string  `json:"measurement_artifact_sha256"`
	Notes                     string  `json:"notes"`
}

type audioBenchLog struct {
	SchemaVersion        int                                         `json:"schema_version"`
	ArtifactType         string                                      `json:"artifact_type"`
	Status               string                                      `json:"status"`
	HardwareClaimAllowed bool                                        `json:"hardware_claim_allowed"`
	FieldTXReady         bool                                        `json:"field_tx_ready"`
	TargetRadioFamily    string                                      `json:"target_radio_family"`
	Operator             string                                      `json:"operator"`
	CapturedAtUTC        string                                      `json:"captured_at_utc"`
	LabSetupRef          string                                      `json:"lab_setup_ref"`
	ConvertTo            string                                      `json:"convert_to"`
	FixtureOrCaptureID   string                                      `json:"fixture_or_capture_id"`
	FixtureOrCapturePath string                                      `json:"fixture_or_capture_path"`
	ArtifactSHA256       string                                      `json:"artifact_sha256"`
	EvidenceGrade        string                                      `json:"evidence_grade"`
	CodecClass           string                                      `json:"codec_class"`
	AudioBandwidthHz     int                                         `json:"audio_bandwidth_hz"`
	ScoringMethod        string                                      `json:"scoring_method"`
	GoodVoiceThreshold   int                                         `json:"good_voice_threshold"`
	NoiseProfiles        []scenario.AudioIntelligibilityNoiseProfile `json:"noise_profiles"`
	LossProfiles         []scenario.AudioIntelligibilityLossProfile  `json:"loss_profiles"`
	Notes                string                                      `json:"notes"`
}

func runConvertBenchLogCommand(args []string) error {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		printConvertBenchLogUsage(os.Stdout)
		return nil
	}
	inputPath, outPath, err := parseConvertBenchLogArgs(args)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read bench log %q: %w", inputPath, err)
	}
	out, err := convertBenchLog(raw)
	if err != nil {
		return fmt.Errorf("convert-bench-log: %w", err)
	}
	return writeAtomicJSONFile("converted bench log", outPath, out)
}

func parseConvertBenchLogArgs(args []string) (string, string, error) {
	var inputPath string
	var outPath string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--out":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("convert-bench-log: --out requires a path")
			}
			if strings.HasPrefix(args[i+1], "-") {
				return "", "", fmt.Errorf("convert-bench-log: --out requires a path, got flag-like value %q", args[i+1])
			}
			if outPath != "" {
				return "", "", fmt.Errorf("convert-bench-log: --out specified more than once")
			}
			outPath = args[i+1]
			i++
		case strings.HasPrefix(arg, "-"):
			return "", "", fmt.Errorf("convert-bench-log: unknown flag %q", arg)
		default:
			if inputPath != "" {
				return "", "", fmt.Errorf("convert-bench-log: unexpected argument %q", arg)
			}
			inputPath = arg
		}
	}
	if inputPath == "" {
		return "", "", fmt.Errorf("convert-bench-log: raw log path is required")
	}
	if outPath == "" {
		return "", "", fmt.Errorf("convert-bench-log: --out path is required")
	}
	return inputPath, outPath, nil
}

func printConvertBenchLogUsage(w *os.File) {
	fmt.Fprintln(w, "usage: adhr-sim convert-bench-log <raw-log.json> --out <canonical.json>")
}

func convertBenchLog(raw []byte) ([]byte, error) {
	var envelope benchLogEnvelope
	if err := decodeBenchLogEnvelope(raw, &envelope); err != nil {
		return nil, err
	}
	switch envelope.ArtifactType {
	case benchLogArtifactChannel:
		return convertChannelBenchLog(raw)
	case benchLogArtifactTurnaround:
		return convertTurnaroundBenchLog(raw)
	case benchLogArtifactAudio:
		return convertAudioBenchLog(raw)
	default:
		return nil, fmt.Errorf("unsupported artifact_type %q", envelope.ArtifactType)
	}
}

func convertChannelBenchLog(raw []byte) ([]byte, error) {
	if err := validateBenchLogRequiredFields(raw, channelBenchLogRequiredFields()); err != nil {
		return nil, err
	}
	var log channelBenchLog
	if err := decodeStrictBenchLog(raw, func(d *json.Decoder) error { return d.Decode(&log) }); err != nil {
		return nil, err
	}
	if err := validateBenchLogCommon(log.SchemaVersion, log.ArtifactType, benchLogArtifactChannel, log.Status, log.HardwareClaimAllowed, log.FieldTXReady); err != nil {
		return nil, err
	}
	if err := validateBenchLogProvenance(log.Operator, log.CapturedAtUTC, log.LabSetupRef); err != nil {
		return nil, err
	}
	if err := requireNoTBD("convert_to", log.ConvertTo); err != nil {
		return nil, err
	}
	if err := requireNoTBD("target_radio_family", log.TargetRadioFamily); err != nil {
		return nil, err
	}
	if err := requireNoTBD("modulation_profile", log.ModulationProfile); err != nil {
		return nil, err
	}
	if err := requireNoTBD("measurement_artifact_ref", log.MeasurementArtifactRef); err != nil {
		return nil, err
	}
	if err := requireNoTBD("measurement_artifact_sha256", log.MeasurementArtifactSHA256); err != nil {
		return nil, err
	}
	if err := validateChannelBenchLogCounters(log); err != nil {
		return nil, err
	}
	measurementSource, testFixture := channelBenchLogMeasurementFlags(log.Status)
	artifactRef, artifactSHA := canonicalBenchLogArtifact(log.Status, log.MeasurementArtifactRef, log.MeasurementArtifactSHA256)
	cfg := scenario.ChannelBenchArtifactConfig{
		ScenarioID:                scenario.ChannelBenchArtifactScenarioID,
		Seed:                      scenario.DefaultScenarioSeed,
		MeasurementSource:         measurementSource,
		RadioFamily:               strings.TrimSpace(log.TargetRadioFamily),
		TestFixture:               testFixture,
		MeasurementArtifactRef:    artifactRef,
		MeasurementArtifactSHA256: artifactSHA,
		ChannelWidthHz:            log.SelectedChannelWidthHz,
		GrossBitrateBps:           log.SelectedGrossBitrateBps,
		ModulationProfile:         strings.TrimSpace(log.ModulationProfile),
		FramesSent:                log.FramesSent,
		FramesDecoded:             log.FramesDecoded,
		CRCErrorCount:             log.CRCErrorCount,
		DurationMs:                log.DurationMs,
		Notes:                     benchLogNotes(log.Operator, log.CapturedAtUTC, log.LabSetupRef, dryRunBenchLogNotes(log.Status, log.MeasurementArtifactRef, log.MeasurementArtifactSHA256, log.Notes)),
	}
	if _, err := scenario.RunChannelBenchArtifactValidation(cfg); err != nil {
		return nil, err
	}
	return json.MarshalIndent(cfg, "", "  ")
}

func convertTurnaroundBenchLog(raw []byte) ([]byte, error) {
	if err := validateBenchLogRequiredFields(raw, turnaroundBenchLogRequiredFields()); err != nil {
		return nil, err
	}
	var log turnaroundBenchLog
	if err := decodeStrictBenchLog(raw, func(d *json.Decoder) error { return d.Decode(&log) }); err != nil {
		return nil, err
	}
	if err := validateBenchLogCommon(log.SchemaVersion, log.ArtifactType, benchLogArtifactTurnaround, log.Status, log.HardwareClaimAllowed, log.FieldTXReady); err != nil {
		return nil, err
	}
	if err := validateBenchLogProvenance(log.Operator, log.CapturedAtUTC, log.LabSetupRef); err != nil {
		return nil, err
	}
	if err := requireNoTBD("convert_to", log.ConvertTo); err != nil {
		return nil, err
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "target_radio_family", value: log.TargetRadioFamily},
		{name: "firmware_path", value: log.FirmwarePath},
		{name: "control_path_under_test", value: log.ControlPathUnderTest},
		{name: "measurement_artifact_ref", value: log.MeasurementArtifactRef},
		{name: "measurement_artifact_sha256", value: log.MeasurementArtifactSHA256},
	} {
		if err := requireNoTBD(field.name, field.value); err != nil {
			return nil, err
		}
	}
	measurementSource, testFixture := turnaroundBenchLogMeasurementFlags(log.Status)
	artifactRef, _ := canonicalBenchLogArtifact(log.Status, log.MeasurementArtifactRef, log.MeasurementArtifactSHA256)
	notes := "control_path_under_test=" + strings.TrimSpace(log.ControlPathUnderTest) + "; " + log.Notes
	if log.Status == benchLogStatusCaptured {
		notes = "control_path_under_test=" + strings.TrimSpace(log.ControlPathUnderTest) + "; measurement_artifact_sha256=" + strings.TrimSpace(log.MeasurementArtifactSHA256) + "; " + log.Notes
	}
	cfg := scenario.TYTBenchMeasurementConfig{
		ScenarioID:              scenario.TYTBenchMeasurementScenarioID,
		Seed:                    scenario.DefaultScenarioSeed,
		MeasurementSource:       measurementSource,
		RadioModel:              strings.TrimSpace(log.TargetRadioFamily),
		TestFixture:             testFixture,
		MeasurementArtifactRef:  artifactRef,
		FirmwarePath:            strings.TrimSpace(log.FirmwarePath),
		ChannelWidthHz:          log.ChannelWidthHz,
		GrossBitrateBps:         log.GrossBitrateBps,
		PreambleMs:              log.PreambleMs,
		SyncMs:                  log.SyncMs,
		LinkDelayMs:             log.LinkDelayMs,
		RxToTxSamplesMs:         cloneInt64s(log.RxToTxSamplesMs),
		TxToRxSamplesMs:         cloneInt64s(log.TxToRxSamplesMs),
		RxValidAfterTxSamplesMs: cloneInt64s(log.RxValidAfterTxSamplesMs),
		Notes:                   benchLogNotes(log.Operator, log.CapturedAtUTC, log.LabSetupRef, dryRunBenchLogNotes(log.Status, log.MeasurementArtifactRef, log.MeasurementArtifactSHA256, notes)),
	}
	if _, err := scenario.RunTYTBenchMeasurementCalibration(cfg); err != nil {
		return nil, err
	}
	return marshalConvertedTurnaroundConfig(cfg)
}

func convertAudioBenchLog(raw []byte) ([]byte, error) {
	if err := validateBenchLogRequiredFields(raw, audioBenchLogRequiredFields()); err != nil {
		return nil, err
	}
	var log audioBenchLog
	if err := decodeStrictBenchLog(raw, func(d *json.Decoder) error { return d.Decode(&log) }); err != nil {
		return nil, err
	}
	if err := validateBenchLogCommon(log.SchemaVersion, log.ArtifactType, benchLogArtifactAudio, log.Status, log.HardwareClaimAllowed, log.FieldTXReady); err != nil {
		return nil, err
	}
	if err := validateBenchLogProvenance(log.Operator, log.CapturedAtUTC, log.LabSetupRef); err != nil {
		return nil, err
	}
	if err := requireNoTBD("convert_to", log.ConvertTo); err != nil {
		return nil, err
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "fixture_or_capture_id", value: log.FixtureOrCaptureID},
		{name: "fixture_or_capture_path", value: log.FixtureOrCapturePath},
		{name: "artifact_sha256", value: log.ArtifactSHA256},
		{name: "evidence_grade", value: log.EvidenceGrade},
		{name: "codec_class", value: log.CodecClass},
		{name: "scoring_method", value: log.ScoringMethod},
	} {
		if err := requireNoTBD(field.name, field.value); err != nil {
			return nil, err
		}
	}
	if len(log.NoiseProfiles) == 0 {
		return nil, fmt.Errorf("noise_profiles must be non-empty")
	}
	if len(log.LossProfiles) == 0 {
		return nil, fmt.Errorf("loss_profiles must be non-empty")
	}
	if log.Status == benchLogStatusDryRunFixture && strings.TrimSpace(log.EvidenceGrade) == scenario.EvidenceGradeBenchMeasured {
		return nil, fmt.Errorf("dry_run_fixture audio evidence_grade must not be bench_measured")
	}
	if log.AudioBandwidthHz <= 0 {
		return nil, fmt.Errorf("audio_bandwidth_hz must be > 0")
	}
	if log.GoodVoiceThreshold <= 0 {
		return nil, fmt.Errorf("good_voice_threshold must be > 0")
	}
	cfg := scenario.AudioIntelligibilityConfig{
		ScenarioID: scenario.AudioIntelligibilityScenarioID,
		Seed:       scenario.DefaultScenarioSeed,
		Fixture: scenario.AudioIntelligibilityFixture{
			FixtureID:      strings.TrimSpace(log.FixtureOrCaptureID),
			FixturePath:    strings.TrimSpace(log.FixtureOrCapturePath),
			ArtifactSHA256: strings.TrimSpace(log.ArtifactSHA256),
			EvidenceGrade:  strings.TrimSpace(log.EvidenceGrade),
		},
		NoiseProfiles:      cloneNoiseProfiles(log.NoiseProfiles),
		LossProfiles:       cloneLossProfiles(log.LossProfiles),
		CodecClass:         strings.TrimSpace(log.CodecClass),
		AudioBandwidthHz:   log.AudioBandwidthHz,
		ScoringMethod:      strings.TrimSpace(log.ScoringMethod),
		GoodVoiceThreshold: log.GoodVoiceThreshold,
	}
	if _, err := scenario.RunAudioIntelligibilitySimulation(cfg); err != nil {
		return nil, err
	}
	return json.MarshalIndent(cfg, "", "  ")
}
