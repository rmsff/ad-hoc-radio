package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/rmsff/ad-hoc-radio/internal/scenario"
)

func decodeBenchLogEnvelope(raw []byte, dst *benchLogEnvelope) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("decode bench log: %w", err)
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode bench log: trailing JSON token")
		}
		return fmt.Errorf("decode bench log: %w", err)
	}
	return nil
}

func decodeStrictBenchLog(raw []byte, decodeInto func(*json.Decoder) error) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decodeInto(decoder); err != nil {
		return fmt.Errorf("decode bench log: %w", err)
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("decode bench log: trailing JSON token")
		}
		return fmt.Errorf("decode bench log: %w", err)
	}
	return nil
}

func validateBenchLogRequiredFields(raw []byte, required []string) error {
	fields, err := decodeBenchLogFields(raw)
	if err != nil {
		return err
	}
	for _, name := range required {
		if err := requireBenchLogField(fields, name); err != nil {
			return err
		}
	}
	return nil
}

func decodeBenchLogFields(raw []byte) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&fields); err != nil {
		return nil, fmt.Errorf("decode bench log: %w", err)
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("decode bench log: trailing JSON token")
		}
		return nil, fmt.Errorf("decode bench log: %w", err)
	}
	return fields, nil
}

func requireBenchLogField(fields map[string]json.RawMessage, name string) error {
	raw, ok := fields[name]
	if !ok {
		return fmt.Errorf("%s is required", name)
	}
	if isJSONNull(raw) {
		return fmt.Errorf("%s must not be null", name)
	}
	return nil
}

func validateBenchLogCommon(schemaVersion int, artifactType string, wantArtifactType string, status string, hardwareClaimAllowed bool, fieldTXReady bool) error {
	if schemaVersion != benchLogSchemaVersion {
		return fmt.Errorf("schema_version=%d unsupported", schemaVersion)
	}
	if artifactType != wantArtifactType {
		return fmt.Errorf("artifact_type=%q does not match %q", artifactType, wantArtifactType)
	}
	switch status {
	case benchLogStatusCaptured:
	case benchLogStatusDryRunFixture:
	case benchLogStatusTemplateOnly:
		return fmt.Errorf("status=%q cannot be converted; fill the log and set status=%q or %q", status, benchLogStatusCaptured, benchLogStatusDryRunFixture)
	default:
		return fmt.Errorf("status=%q unsupported", status)
	}
	if hardwareClaimAllowed {
		return fmt.Errorf("hardware_claim_allowed must remain false in raw bench logs")
	}
	if fieldTXReady {
		return fmt.Errorf("field_tx_ready must remain false in raw bench logs")
	}
	return nil
}

func channelBenchLogMeasurementFlags(status string) (string, bool) {
	if status == benchLogStatusDryRunFixture {
		return scenario.ChannelBenchMeasurementSourceFixtureOnly, true
	}
	return scenario.ChannelBenchMeasurementSourceBenchMeasured, false
}

func turnaroundBenchLogMeasurementFlags(status string) (string, bool) {
	if status == benchLogStatusDryRunFixture {
		return scenario.TYTBenchMeasurementSourceFixtureOnly, true
	}
	return scenario.TYTBenchMeasurementSourceBenchMeasured, false
}

func canonicalBenchLogArtifact(status string, ref string, sha256 string) (string, string) {
	if status == benchLogStatusDryRunFixture {
		return "", ""
	}
	return strings.TrimSpace(ref), strings.TrimSpace(sha256)
}

func dryRunBenchLogNotes(status string, artifactRef string, artifactSHA256 string, notes string) string {
	if status != benchLogStatusDryRunFixture {
		return notes
	}
	parts := []string{
		"dry_run_fixture=true",
		"dry_run_artifact_ref=" + strings.TrimSpace(artifactRef),
		"dry_run_artifact_sha256=" + strings.TrimSpace(artifactSHA256),
	}
	if strings.TrimSpace(notes) != "" {
		parts = append(parts, strings.TrimSpace(notes))
	}
	return strings.Join(parts, "; ")
}

func validateChannelBenchLogCounters(log channelBenchLog) error {
	if len(log.CandidateChannelWidthHz) == 0 {
		return fmt.Errorf("candidate_channel_width_hz must be non-empty")
	}
	if len(log.CandidateGrossBitrateBps) == 0 {
		return fmt.Errorf("candidate_gross_bitrate_bps must be non-empty")
	}
	if log.SelectedChannelWidthHz <= 0 {
		return fmt.Errorf("selected_channel_width_hz must be > 0")
	}
	if log.SelectedGrossBitrateBps <= 0 {
		return fmt.Errorf("selected_gross_bitrate_bps must be > 0")
	}
	if log.FramesSent <= 0 {
		return fmt.Errorf("frames_sent must be > 0")
	}
	if log.DurationMs <= 0 {
		return fmt.Errorf("duration_ms must be > 0")
	}
	if log.FramesDecoded < 0 {
		return fmt.Errorf("frames_decoded must be >= 0")
	}
	if log.CRCErrorCount < 0 {
		return fmt.Errorf("crc_error_count must be >= 0")
	}
	return nil
}

func validateBenchLogProvenance(operator string, capturedAtUTC string, labSetupRef string) error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "operator", value: operator},
		{name: "captured_at_utc", value: capturedAtUTC},
		{name: "lab_setup_ref", value: labSetupRef},
	} {
		if err := requireNoTBD(field.name, field.value); err != nil {
			return err
		}
	}
	return nil
}

func channelBenchLogRequiredFields() []string {
	return []string{
		"schema_version",
		"artifact_type",
		"status",
		"hardware_claim_allowed",
		"field_tx_ready",
		"target_radio_family",
		"operator",
		"captured_at_utc",
		"lab_setup_ref",
		"convert_to",
		"candidate_channel_width_hz",
		"candidate_gross_bitrate_bps",
		"selected_channel_width_hz",
		"selected_gross_bitrate_bps",
		"modulation_profile",
		"frames_sent",
		"frames_decoded",
		"crc_error_count",
		"duration_ms",
		"measurement_artifact_ref",
		"measurement_artifact_sha256",
	}
}

func turnaroundBenchLogRequiredFields() []string {
	return []string{
		"schema_version",
		"artifact_type",
		"status",
		"hardware_claim_allowed",
		"field_tx_ready",
		"target_radio_family",
		"operator",
		"captured_at_utc",
		"lab_setup_ref",
		"convert_to",
		"firmware_path",
		"control_path_under_test",
		"channel_width_hz",
		"gross_bitrate_bps",
		"preamble_ms",
		"sync_ms",
		"link_delay_ms",
		"rx_to_tx_samples_ms",
		"tx_to_rx_samples_ms",
		"rx_valid_after_tx_samples_ms",
		"measurement_artifact_ref",
		"measurement_artifact_sha256",
	}
}

func audioBenchLogRequiredFields() []string {
	return []string{
		"schema_version",
		"artifact_type",
		"status",
		"hardware_claim_allowed",
		"field_tx_ready",
		"target_radio_family",
		"operator",
		"captured_at_utc",
		"lab_setup_ref",
		"convert_to",
		"fixture_or_capture_id",
		"fixture_or_capture_path",
		"artifact_sha256",
		"evidence_grade",
		"codec_class",
		"audio_bandwidth_hz",
		"scoring_method",
		"good_voice_threshold",
		"noise_profiles",
		"loss_profiles",
	}
}

func requireNoTBD(name string, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("%s must not be empty", name)
	}
	if strings.EqualFold(trimmed, "TBD") {
		return fmt.Errorf("%s must not be TBD", name)
	}
	return nil
}

func benchLogNotes(operator string, capturedAtUTC string, labSetupRef string, notes string) string {
	parts := make([]string, 0, 4)
	if strings.TrimSpace(operator) != "" && !strings.EqualFold(strings.TrimSpace(operator), "TBD") {
		parts = append(parts, "operator="+strings.TrimSpace(operator))
	}
	if strings.TrimSpace(capturedAtUTC) != "" && !strings.EqualFold(strings.TrimSpace(capturedAtUTC), "TBD") {
		parts = append(parts, "captured_at_utc="+strings.TrimSpace(capturedAtUTC))
	}
	if strings.TrimSpace(labSetupRef) != "" && !strings.EqualFold(strings.TrimSpace(labSetupRef), "TBD") {
		parts = append(parts, "lab_setup_ref="+strings.TrimSpace(labSetupRef))
	}
	if strings.TrimSpace(notes) != "" {
		parts = append(parts, strings.TrimSpace(notes))
	}
	return strings.Join(parts, "; ")
}

func cloneInt64s(values []int64) []int64 {
	if len(values) == 0 {
		return nil
	}
	out := make([]int64, len(values))
	copy(out, values)
	return out
}

func cloneNoiseProfiles(values []scenario.AudioIntelligibilityNoiseProfile) []scenario.AudioIntelligibilityNoiseProfile {
	if len(values) == 0 {
		return nil
	}
	out := make([]scenario.AudioIntelligibilityNoiseProfile, len(values))
	copy(out, values)
	return out
}

func cloneLossProfiles(values []scenario.AudioIntelligibilityLossProfile) []scenario.AudioIntelligibilityLossProfile {
	if len(values) == 0 {
		return nil
	}
	out := make([]scenario.AudioIntelligibilityLossProfile, len(values))
	copy(out, values)
	return out
}

func marshalConvertedTurnaroundConfig(cfg scenario.TYTBenchMeasurementConfig) ([]byte, error) {
	out := struct {
		ScenarioID              string  `json:"scenario_id"`
		Seed                    uint64  `json:"seed"`
		MeasurementSource       string  `json:"measurement_source"`
		RadioModel              string  `json:"radio_model"`
		TestFixture             bool    `json:"test_fixture"`
		MeasurementArtifactRef  string  `json:"measurement_artifact_ref"`
		FirmwarePath            string  `json:"firmware_path"`
		ChannelWidthHz          int     `json:"channel_width_hz"`
		GrossBitrateBps         int64   `json:"gross_bitrate_bps"`
		PreambleMs              int64   `json:"preamble_ms"`
		SyncMs                  int64   `json:"sync_ms"`
		LinkDelayMs             int64   `json:"link_delay_ms"`
		RxToTxSamplesMs         []int64 `json:"rx_to_tx_samples_ms"`
		TxToRxSamplesMs         []int64 `json:"tx_to_rx_samples_ms"`
		RxValidAfterTxSamplesMs []int64 `json:"rx_valid_after_tx_samples_ms"`
		Notes                   string  `json:"notes"`
	}{
		ScenarioID:              cfg.ScenarioID,
		Seed:                    cfg.Seed,
		MeasurementSource:       cfg.MeasurementSource,
		RadioModel:              cfg.RadioModel,
		TestFixture:             cfg.TestFixture,
		MeasurementArtifactRef:  cfg.MeasurementArtifactRef,
		FirmwarePath:            cfg.FirmwarePath,
		ChannelWidthHz:          cfg.ChannelWidthHz,
		GrossBitrateBps:         cfg.GrossBitrateBps,
		PreambleMs:              cfg.PreambleMs,
		SyncMs:                  cfg.SyncMs,
		LinkDelayMs:             cfg.LinkDelayMs,
		RxToTxSamplesMs:         cloneInt64s(cfg.RxToTxSamplesMs),
		TxToRxSamplesMs:         cloneInt64s(cfg.TxToRxSamplesMs),
		RxValidAfterTxSamplesMs: cloneInt64s(cfg.RxValidAfterTxSamplesMs),
		Notes:                   cfg.Notes,
	}
	return json.MarshalIndent(out, "", "  ")
}
