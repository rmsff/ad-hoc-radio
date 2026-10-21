package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const ChannelBenchArtifactScenarioID = "channel_bench_artifact"
const ChannelBenchArtifactStoryID = "MVP-8.13"
const ChannelBenchArtifactEvidenceType = "channel_bench_artifact"

const (
	ChannelBenchMeasurementSourceFixtureOnly   = "fixture_only"
	ChannelBenchMeasurementSourceBenchMeasured = "bench_measured"
	ChannelBenchMeasurementSourceUnknown       = "unknown"
)

const (
	ChannelBenchOutcomeFixtureOnly = "channel_fixture_only"
	ChannelBenchOutcomeVerified    = "channel_bench_verified"
	ChannelBenchOutcomeFail        = "channel_bench_failed"
)

type ChannelBenchArtifactConfig struct {
	ScenarioID                string `json:"scenario_id"`
	Seed                      uint64 `json:"seed"`
	MeasurementSource         string `json:"measurement_source"`
	RadioFamily               string `json:"radio_family"`
	TestFixture               bool   `json:"test_fixture"`
	MeasurementArtifactRef    string `json:"measurement_artifact_ref"`
	MeasurementArtifactSHA256 string `json:"measurement_artifact_sha256"`
	ChannelWidthHz            int    `json:"channel_width_hz"`
	GrossBitrateBps           int64  `json:"gross_bitrate_bps"`
	ModulationProfile         string `json:"modulation_profile"`
	FramesSent                int64  `json:"frames_sent"`
	FramesDecoded             int64  `json:"frames_decoded"`
	CRCErrorCount             int64  `json:"crc_error_count"`
	DurationMs                int64  `json:"duration_ms"`
	Notes                     string `json:"notes"`
}

type ChannelBenchArtifactReport struct {
	ScenarioID                 string           `json:"scenario_id"`
	Seed                       uint64           `json:"seed"`
	Evidence                   ScenarioEvidence `json:"evidence"`
	MeasurementSource          string           `json:"measurement_source"`
	RadioFamily                string           `json:"radio_family"`
	TestFixture                bool             `json:"test_fixture"`
	MeasurementArtifactRef     string           `json:"measurement_artifact_ref"`
	MeasurementArtifactSHA256  string           `json:"measurement_artifact_sha256"`
	ChannelWidthHz             int              `json:"channel_width_hz"`
	GrossBitrateBps            int64            `json:"gross_bitrate_bps"`
	ModulationProfile          string           `json:"modulation_profile"`
	FramesSent                 int64            `json:"frames_sent"`
	FramesDecoded              int64            `json:"frames_decoded"`
	CRCErrorCount              int64            `json:"crc_error_count"`
	DurationMs                 int64            `json:"duration_ms"`
	DecodeSuccessPermille      int64            `json:"decode_success_permille"`
	CRCErrorPermille           int64            `json:"crc_error_permille"`
	ChannelBenchVerified       bool             `json:"channel_bench_verified"`
	MeasuredHardwareCapability bool             `json:"measured_hardware_capability"`
	HardwareClaimAllowed       bool             `json:"hardware_claim_allowed"`
	Outcome                    string           `json:"outcome"`
	DegradeReasons             []string         `json:"degrade_reasons"`
	Notes                      string           `json:"notes"`
	EventHash                  string           `json:"event_hash"`
}

func DefaultChannelBenchArtifactConfig() ChannelBenchArtifactConfig {
	return ChannelBenchArtifactConfig{
		ScenarioID:        ChannelBenchArtifactScenarioID,
		Seed:              DefaultScenarioSeed,
		MeasurementSource: ChannelBenchMeasurementSourceFixtureOnly,
		RadioFamily:       tytLikelyRadioFamily,
		TestFixture:       true,
		ChannelWidthHz:    tytLikelyChannelWidthHz,
		GrossBitrateBps:   19200,
		ModulationProfile: "custom_2fsk_25khz_fixture",
		FramesSent:        1000,
		FramesDecoded:     1000,
		CRCErrorCount:     0,
		DurationMs:        10000,
		Notes:             "fixture-only channel artifact format check, not a hardware claim",
	}
}

func RunChannelBenchArtifactValidation(cfg ChannelBenchArtifactConfig) (ChannelBenchArtifactReport, error) {
	cfg = normalizeChannelBenchArtifactConfig(cfg)
	if err := validateChannelBenchArtifactConfig(cfg); err != nil {
		return ChannelBenchArtifactReport{}, err
	}
	success := permille(cfg.FramesDecoded, cfg.FramesSent)
	crcErrors := permille(cfg.CRCErrorCount, cfg.FramesSent)
	hardwareClaimAllowed := cfg.MeasurementSource == ChannelBenchMeasurementSourceBenchMeasured &&
		!cfg.TestFixture &&
		cfg.MeasurementArtifactRef != ""
	report := ChannelBenchArtifactReport{
		ScenarioID:                 cfg.ScenarioID,
		Seed:                       cfg.Seed,
		MeasurementSource:          cfg.MeasurementSource,
		RadioFamily:                cfg.RadioFamily,
		TestFixture:                cfg.TestFixture,
		MeasurementArtifactRef:     cfg.MeasurementArtifactRef,
		MeasurementArtifactSHA256:  cfg.MeasurementArtifactSHA256,
		ChannelWidthHz:             cfg.ChannelWidthHz,
		GrossBitrateBps:            cfg.GrossBitrateBps,
		ModulationProfile:          cfg.ModulationProfile,
		FramesSent:                 cfg.FramesSent,
		FramesDecoded:              cfg.FramesDecoded,
		CRCErrorCount:              cfg.CRCErrorCount,
		DurationMs:                 cfg.DurationMs,
		DecodeSuccessPermille:      success,
		CRCErrorPermille:           crcErrors,
		MeasuredHardwareCapability: hardwareClaimAllowed,
		HardwareClaimAllowed:       hardwareClaimAllowed,
		Notes:                      cfg.Notes,
	}
	report.ChannelBenchVerified = hardwareClaimAllowed && success >= 990 && crcErrors <= 10
	report.Outcome, report.DegradeReasons = channelBenchOutcome(report)
	report.Evidence = channelBenchEvidence(cfg, report, "")
	hash, err := channelBenchHash(report)
	if err != nil {
		return ChannelBenchArtifactReport{}, err
	}
	report.EventHash = hash
	report.Evidence = channelBenchEvidence(cfg, report, hash)
	if err := ValidateScenarioEvidence(report.Evidence); err != nil {
		return ChannelBenchArtifactReport{}, err
	}
	return report, nil
}

func (r ChannelBenchArtifactReport) MarshalReportJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

func normalizeChannelBenchArtifactConfig(cfg ChannelBenchArtifactConfig) ChannelBenchArtifactConfig {
	def := DefaultChannelBenchArtifactConfig()
	if cfg.ScenarioID == "" {
		cfg.ScenarioID = def.ScenarioID
	}
	if cfg.Seed == 0 {
		cfg.Seed = def.Seed
	}
	if cfg.MeasurementSource == "" {
		cfg.MeasurementSource = def.MeasurementSource
	}
	if cfg.RadioFamily == "" {
		cfg.RadioFamily = def.RadioFamily
	}
	if cfg.ChannelWidthHz == 0 {
		cfg.ChannelWidthHz = def.ChannelWidthHz
	}
	if cfg.GrossBitrateBps == 0 {
		cfg.GrossBitrateBps = def.GrossBitrateBps
	}
	if cfg.ModulationProfile == "" {
		cfg.ModulationProfile = def.ModulationProfile
	}
	if cfg.FramesSent == 0 {
		cfg.FramesSent = def.FramesSent
	}
	if cfg.FramesDecoded == 0 {
		cfg.FramesDecoded = def.FramesDecoded
	}
	if cfg.DurationMs == 0 {
		cfg.DurationMs = def.DurationMs
	}
	cfg.MeasurementArtifactRef = strings.TrimSpace(cfg.MeasurementArtifactRef)
	cfg.MeasurementArtifactSHA256 = strings.TrimSpace(cfg.MeasurementArtifactSHA256)
	cfg.RadioFamily = strings.TrimSpace(cfg.RadioFamily)
	cfg.ModulationProfile = strings.TrimSpace(cfg.ModulationProfile)
	return cfg
}

func validateChannelBenchArtifactConfig(cfg ChannelBenchArtifactConfig) error {
	if cfg.ScenarioID != ChannelBenchArtifactScenarioID {
		return fmt.Errorf("channel bench artifact: unsupported scenario_id %q", cfg.ScenarioID)
	}
	switch cfg.MeasurementSource {
	case ChannelBenchMeasurementSourceFixtureOnly:
		if !cfg.TestFixture {
			return fmt.Errorf("channel bench artifact: fixture_only requires test_fixture=true")
		}
	case ChannelBenchMeasurementSourceBenchMeasured:
		if cfg.TestFixture {
			return fmt.Errorf("channel bench artifact: bench_measured requires test_fixture=false")
		}
		if cfg.MeasurementArtifactRef == "" {
			return fmt.Errorf("channel bench artifact: bench_measured requires measurement_artifact_ref")
		}
		if cfg.MeasurementArtifactSHA256 == "" {
			return fmt.Errorf("channel bench artifact: bench_measured requires measurement_artifact_sha256")
		}
	case ChannelBenchMeasurementSourceUnknown:
	default:
		return fmt.Errorf("channel bench artifact: unsupported measurement_source %q", cfg.MeasurementSource)
	}
	if cfg.RadioFamily == "" || cfg.ModulationProfile == "" {
		return fmt.Errorf("channel bench artifact: radio_family and modulation_profile are required")
	}
	if cfg.MeasurementArtifactSHA256 != "" && !isLowerHexSHA256(cfg.MeasurementArtifactSHA256) {
		return fmt.Errorf("channel bench artifact: measurement_artifact_sha256 must be lowercase sha256")
	}
	if cfg.ChannelWidthHz != 12500 && cfg.ChannelWidthHz != 25000 {
		return fmt.Errorf("channel bench artifact: channel_width_hz=%d unsupported", cfg.ChannelWidthHz)
	}
	if cfg.GrossBitrateBps != 9600 && cfg.GrossBitrateBps != 19200 {
		return fmt.Errorf("channel bench artifact: gross_bitrate_bps=%d unsupported", cfg.GrossBitrateBps)
	}
	if cfg.FramesSent <= 0 || cfg.FramesDecoded < 0 || cfg.CRCErrorCount < 0 || cfg.DurationMs <= 0 {
		return fmt.Errorf("channel bench artifact: frame counters and duration must be non-negative with frames_sent/duration > 0")
	}
	if cfg.FramesDecoded > cfg.FramesSent || cfg.CRCErrorCount > cfg.FramesSent {
		return fmt.Errorf("channel bench artifact: decoded/error counters must not exceed frames_sent")
	}
	return nil
}

func channelBenchOutcome(report ChannelBenchArtifactReport) (string, []string) {
	if report.ChannelBenchVerified {
		return ChannelBenchOutcomeVerified, nil
	}
	reasons := make([]string, 0, 3)
	if report.MeasurementSource != ChannelBenchMeasurementSourceBenchMeasured {
		reasons = append(reasons, "not_bench_measured")
	}
	if report.DecodeSuccessPermille < 990 {
		reasons = append(reasons, "decode_success_below_threshold")
	}
	if report.CRCErrorPermille > 10 {
		reasons = append(reasons, "crc_error_above_threshold")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "bench_evidence_not_accepted")
	}
	if report.MeasurementSource == ChannelBenchMeasurementSourceFixtureOnly {
		return ChannelBenchOutcomeFixtureOnly, reasons
	}
	return ChannelBenchOutcomeFail, reasons
}

func channelBenchEvidence(cfg ChannelBenchArtifactConfig, report ChannelBenchArtifactReport, eventHash string) ScenarioEvidence {
	grade := EvidenceGradeFixtureMetadata
	if report.MeasuredHardwareCapability {
		grade = EvidenceGradeBenchMeasured
	}
	return ScenarioEvidence{
		SchemaVersion: ScenarioEvidenceSchemaVersion,
		StoryID:       ChannelBenchArtifactStoryID,
		ScenarioID:    cfg.ScenarioID,
		EvidenceType:  ChannelBenchArtifactEvidenceType,
		EvidenceGrade: grade,
		Seed:          cfg.Seed,
		Determinism: ScenarioEvidenceDeterminism{
			EventHash:     eventHash,
			HashAlgorithm: ScenarioEvidenceHashAlgorithm,
			Serialization: ScenarioEvidenceSerialization,
		},
		ArtifactRefs: channelBenchArtifactRefs(report),
		AssumptionFlags: []string{
			"channel_artifact_gate",
			"no_rf_field_authorization",
			"not_voice_quality_evidence",
			"not_turnaround_evidence",
		},
		HardwareClaimAllowed: report.HardwareClaimAllowed,
		Outcome:              report.Outcome,
		DegradeReasons:       append([]string(nil), report.DegradeReasons...),
	}
}

func channelBenchArtifactRefs(report ChannelBenchArtifactReport) []ScenarioEvidenceArtifact {
	if report.MeasurementArtifactRef == "" {
		return nil
	}
	return []ScenarioEvidenceArtifact{
		{
			ArtifactID: "channel_bench_artifact",
			Path:       report.MeasurementArtifactRef,
			SHA256:     report.MeasurementArtifactSHA256,
			Role:       "channel_bitrate_bandwidth_evidence",
		},
	}
}

func channelBenchHash(report ChannelBenchArtifactReport) (string, error) {
	hashable := report
	hashable.EventHash = ""
	hashable.Evidence.Determinism.EventHash = ""
	b, err := json.Marshal(hashable)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func permille(numerator int64, denominator int64) int64 {
	if denominator == 0 {
		return 0
	}
	return numerator * 1000 / denominator
}
