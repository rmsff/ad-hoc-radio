package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const AudioIntelligibilityScenarioID = "audio_intelligibility_fixture_envelope"
const AudioIntelligibilityStoryID = "MVP-8.8"
const AudioIntelligibilityEvidenceType = "audio_intelligibility_fixture"

const (
	AudioIntelligibilityOutcomeGoodVoicePass = "good_voice_pass"
	AudioIntelligibilityOutcomeDegraded      = "degraded"
	AudioIntelligibilityOutcomeFail          = "fail"
)

const (
	AudioIntelligibilityScoringMethod    = "deterministic_fixture_proxy_v1"
	AudioIntelligibilityDefaultThreshold = 75
	audioIntelligibilityMaxRows          = 64
)

const audioIntelligibilityFixtureText = "ad-hoc-radio MVP-8.8 synthetic clean voice fixture v1\npurpose: deterministic artifact for audio intelligibility envelope tests\nnot a recorded human voice and not a subjective MOS fixture\n"

type AudioIntelligibilityConfig struct {
	ScenarioID         string                             `json:"scenario_id"`
	Seed               uint64                             `json:"seed"`
	Fixture            AudioIntelligibilityFixture        `json:"fixture"`
	NoiseProfiles      []AudioIntelligibilityNoiseProfile `json:"noise_profiles"`
	LossProfiles       []AudioIntelligibilityLossProfile  `json:"loss_profiles"`
	CodecClass         string                             `json:"codec_class"`
	AudioBandwidthHz   int                                `json:"audio_bandwidth_hz"`
	ScoringMethod      string                             `json:"scoring_method"`
	GoodVoiceThreshold int                                `json:"good_voice_threshold"`
}

type AudioIntelligibilityFixture struct {
	FixtureID      string `json:"fixture_id"`
	FixturePath    string `json:"fixture_path"`
	ArtifactSHA256 string `json:"artifact_sha256"`
	EvidenceGrade  string `json:"evidence_grade"`
}

type AudioIntelligibilityNoiseProfile struct {
	NoiseProfileID string `json:"noise_profile_id"`
	NoiseSNRDb     int    `json:"noise_snr_db"`
}

type AudioIntelligibilityLossProfile struct {
	LossProfileID string `json:"loss_profile_id"`
	PacketLossPct int    `json:"packet_loss_pct"`
	BurstLossMs   int    `json:"burst_loss_ms"`
}

type AudioIntelligibilityReport struct {
	ScenarioID         string                    `json:"scenario_id"`
	Seed               uint64                    `json:"seed"`
	Evidence           ScenarioEvidence          `json:"evidence"`
	Rows               []AudioIntelligibilityRow `json:"rows"`
	RowCount           int                       `json:"row_count"`
	GoodVoicePassRows  int                       `json:"good_voice_pass_rows"`
	DegradedRows       int                       `json:"degraded_rows"`
	FailRows           int                       `json:"fail_rows"`
	GoodVoiceThreshold int                       `json:"good_voice_threshold"`
	ScoringMethod      string                    `json:"scoring_method"`
	EventHash          string                    `json:"event_hash"`
}

type AudioIntelligibilityRow struct {
	RowID                     string   `json:"row_id"`
	FixtureID                 string   `json:"fixture_id"`
	FixturePath               string   `json:"fixture_path"`
	ArtifactSHA256            string   `json:"artifact_sha256"`
	EvidenceGrade             string   `json:"evidence_grade"`
	NoiseProfileID            string   `json:"noise_profile_id"`
	NoiseSNRDb                int      `json:"noise_snr_db"`
	LossProfileID             string   `json:"loss_profile_id"`
	PacketLossPct             int      `json:"packet_loss_pct"`
	BurstLossMs               int      `json:"burst_loss_ms"`
	CodecClass                string   `json:"codec_class"`
	AudioBandwidthHz          int      `json:"audio_bandwidth_hz"`
	ScoringMethod             string   `json:"scoring_method"`
	IntelligibilityProxyScore int      `json:"intelligibility_proxy_score"`
	GoodVoiceThreshold        int      `json:"good_voice_threshold"`
	Outcome                   string   `json:"outcome"`
	DegradeReasons            []string `json:"degrade_reasons"`
}

func DefaultAudioIntelligibilityConfig() AudioIntelligibilityConfig {
	return AudioIntelligibilityConfig{
		ScenarioID: AudioIntelligibilityScenarioID,
		Seed:       DefaultScenarioSeed,
		Fixture: AudioIntelligibilityFixture{
			FixtureID:      "fm_like_clean_voice_v1",
			FixturePath:    "configs/fixtures/audio/fm-like-clean-voice-v1.txt",
			ArtifactSHA256: audioIntelligibilityDefaultFixtureSHA256(),
			EvidenceGrade:  EvidenceGradeFixtureArtifact,
		},
		NoiseProfiles: []AudioIntelligibilityNoiseProfile{
			{NoiseProfileID: "clean", NoiseSNRDb: 40},
			{NoiseProfileID: "light_noise", NoiseSNRDb: 24},
			{NoiseProfileID: "field_noise", NoiseSNRDb: 16},
			{NoiseProfileID: "severe_noise", NoiseSNRDb: 8},
		},
		LossProfiles: []AudioIntelligibilityLossProfile{
			{LossProfileID: "no_loss", PacketLossPct: 0, BurstLossMs: 0},
			{LossProfileID: "packet_loss_5pct", PacketLossPct: 5, BurstLossMs: 0},
			{LossProfileID: "burst_loss_120ms", PacketLossPct: 8, BurstLossMs: 120},
		},
		CodecClass:         "opus_class_wideband_speech_proxy",
		AudioBandwidthHz:   8000,
		ScoringMethod:      AudioIntelligibilityScoringMethod,
		GoodVoiceThreshold: AudioIntelligibilityDefaultThreshold,
	}
}

func RunAudioIntelligibilitySimulation(cfg AudioIntelligibilityConfig) (AudioIntelligibilityReport, error) {
	cfg = normalizeAudioIntelligibilityConfig(cfg)
	if err := validateAudioIntelligibilityConfig(cfg); err != nil {
		return AudioIntelligibilityReport{}, err
	}
	report := AudioIntelligibilityReport{
		ScenarioID:         cfg.ScenarioID,
		Seed:               cfg.Seed,
		Rows:               make([]AudioIntelligibilityRow, 0, audioIntelligibilityRowCount(cfg)),
		GoodVoiceThreshold: cfg.GoodVoiceThreshold,
		ScoringMethod:      cfg.ScoringMethod,
	}
	for _, noise := range cfg.NoiseProfiles {
		for _, loss := range cfg.LossProfiles {
			row := audioIntelligibilityRow(cfg, noise, loss)
			report.Rows = append(report.Rows, row)
			switch row.Outcome {
			case AudioIntelligibilityOutcomeGoodVoicePass:
				report.GoodVoicePassRows++
			case AudioIntelligibilityOutcomeDegraded:
				report.DegradedRows++
			case AudioIntelligibilityOutcomeFail:
				report.FailRows++
			}
		}
	}
	report.RowCount = len(report.Rows)
	report.Evidence = audioIntelligibilityEvidence(cfg, report, "")

	hash, err := audioIntelligibilityHash(report)
	if err != nil {
		return AudioIntelligibilityReport{}, err
	}
	report.EventHash = hash
	report.Evidence = audioIntelligibilityEvidence(cfg, report, hash)
	if err := ValidateScenarioEvidence(report.Evidence); err != nil {
		return AudioIntelligibilityReport{}, err
	}
	return report, nil
}

func (r AudioIntelligibilityReport) MarshalReportJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

func audioIntelligibilityRow(cfg AudioIntelligibilityConfig, noise AudioIntelligibilityNoiseProfile, loss AudioIntelligibilityLossProfile) AudioIntelligibilityRow {
	score := audioIntelligibilityScore(noise, loss)
	reasons := audioIntelligibilityDegradeReasons(cfg.Fixture, score, cfg.GoodVoiceThreshold)
	outcome := audioIntelligibilityOutcome(score, cfg.Fixture, cfg.GoodVoiceThreshold, reasons)
	return AudioIntelligibilityRow{
		RowID:                     fmt.Sprintf("%s_%s", noise.NoiseProfileID, loss.LossProfileID),
		FixtureID:                 cfg.Fixture.FixtureID,
		FixturePath:               cfg.Fixture.FixturePath,
		ArtifactSHA256:            cfg.Fixture.ArtifactSHA256,
		EvidenceGrade:             cfg.Fixture.EvidenceGrade,
		NoiseProfileID:            noise.NoiseProfileID,
		NoiseSNRDb:                noise.NoiseSNRDb,
		LossProfileID:             loss.LossProfileID,
		PacketLossPct:             loss.PacketLossPct,
		BurstLossMs:               loss.BurstLossMs,
		CodecClass:                cfg.CodecClass,
		AudioBandwidthHz:          cfg.AudioBandwidthHz,
		ScoringMethod:             cfg.ScoringMethod,
		IntelligibilityProxyScore: score,
		GoodVoiceThreshold:        cfg.GoodVoiceThreshold,
		Outcome:                   outcome,
		DegradeReasons:            reasons,
	}
}

func audioIntelligibilityScore(noise AudioIntelligibilityNoiseProfile, loss AudioIntelligibilityLossProfile) int {
	score := 95
	if noise.NoiseSNRDb < 30 {
		score -= (30 - noise.NoiseSNRDb) * 2
	}
	score -= loss.PacketLossPct * 2
	score -= loss.BurstLossMs / 10
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func audioIntelligibilityOutcome(score int, fixture AudioIntelligibilityFixture, threshold int, reasons []string) string {
	if score >= threshold && audioIntelligibilityHasStrongEvidence(fixture) {
		return AudioIntelligibilityOutcomeGoodVoicePass
	}
	for _, reason := range reasons {
		if reason == "intelligibility_score_below_fail_floor" {
			return AudioIntelligibilityOutcomeFail
		}
	}
	return AudioIntelligibilityOutcomeDegraded
}

func audioIntelligibilityDegradeReasons(fixture AudioIntelligibilityFixture, score int, threshold int) []string {
	reasons := make([]string, 0, 3)
	if !audioIntelligibilityHasStrongEvidence(fixture) {
		reasons = append(reasons, "insufficient_fixture_artifact")
	}
	if score < threshold {
		reasons = append(reasons, "intelligibility_score_below_threshold")
	}
	if score < 45 {
		reasons = append(reasons, "intelligibility_score_below_fail_floor")
	}
	return reasons
}

func audioIntelligibilityHasStrongEvidence(fixture AudioIntelligibilityFixture) bool {
	switch fixture.EvidenceGrade {
	case EvidenceGradeFixtureArtifact, EvidenceGradeBenchMeasured:
		return fixture.ArtifactSHA256 != ""
	default:
		return false
	}
}

func audioIntelligibilityEvidence(cfg AudioIntelligibilityConfig, report AudioIntelligibilityReport, eventHash string) ScenarioEvidence {
	outcome := AudioIntelligibilityOutcomeDegraded
	reasons := []string{"matrix_contains_degraded_or_failed_rows"}
	if report.RowCount > 0 && report.GoodVoicePassRows == report.RowCount {
		outcome = AudioIntelligibilityOutcomeGoodVoicePass
		reasons = nil
	} else if report.GoodVoicePassRows == 0 && report.DegradedRows == 0 && report.FailRows > 0 {
		outcome = AudioIntelligibilityOutcomeFail
		reasons = []string{"all_rows_failed"}
	} else if report.GoodVoicePassRows == 0 {
		reasons = []string{"no_good_voice_pass_rows"}
	}
	return ScenarioEvidence{
		SchemaVersion: ScenarioEvidenceSchemaVersion,
		StoryID:       AudioIntelligibilityStoryID,
		ScenarioID:    cfg.ScenarioID,
		EvidenceType:  AudioIntelligibilityEvidenceType,
		EvidenceGrade: cfg.Fixture.EvidenceGrade,
		Seed:          cfg.Seed,
		Determinism: ScenarioEvidenceDeterminism{
			EventHash:     eventHash,
			HashAlgorithm: ScenarioEvidenceHashAlgorithm,
			Serialization: ScenarioEvidenceSerialization,
		},
		ArtifactRefs: []ScenarioEvidenceArtifact{
			{
				ArtifactID: cfg.Fixture.FixtureID,
				Path:       cfg.Fixture.FixturePath,
				SHA256:     cfg.Fixture.ArtifactSHA256,
				Role:       "audio_fixture",
			},
		},
		AssumptionFlags:      []string{"deterministic_proxy_score", "not_subjective_mos", "not_real_codec"},
		HardwareClaimAllowed: false,
		Outcome:              outcome,
		DegradeReasons:       reasons,
	}
}

func normalizeAudioIntelligibilityConfig(cfg AudioIntelligibilityConfig) AudioIntelligibilityConfig {
	def := DefaultAudioIntelligibilityConfig()
	if cfg.ScenarioID == "" {
		cfg.ScenarioID = def.ScenarioID
	}
	if cfg.Seed == 0 {
		cfg.Seed = def.Seed
	}
	if cfg.Fixture.FixtureID == "" && cfg.Fixture.FixturePath == "" && cfg.Fixture.ArtifactSHA256 == "" && cfg.Fixture.EvidenceGrade == "" {
		cfg.Fixture = def.Fixture
	}
	if cfg.NoiseProfiles == nil {
		cfg.NoiseProfiles = cloneAudioIntelligibilityNoiseProfiles(def.NoiseProfiles)
	} else {
		cfg.NoiseProfiles = sortedAudioIntelligibilityNoiseProfiles(cfg.NoiseProfiles)
	}
	if cfg.LossProfiles == nil {
		cfg.LossProfiles = cloneAudioIntelligibilityLossProfiles(def.LossProfiles)
	} else {
		cfg.LossProfiles = sortedAudioIntelligibilityLossProfiles(cfg.LossProfiles)
	}
	if cfg.CodecClass == "" {
		cfg.CodecClass = def.CodecClass
	}
	if cfg.AudioBandwidthHz == 0 {
		cfg.AudioBandwidthHz = def.AudioBandwidthHz
	}
	if cfg.ScoringMethod == "" {
		cfg.ScoringMethod = def.ScoringMethod
	}
	if cfg.GoodVoiceThreshold == 0 {
		cfg.GoodVoiceThreshold = def.GoodVoiceThreshold
	}
	return cfg
}

func validateAudioIntelligibilityConfig(cfg AudioIntelligibilityConfig) error {
	if cfg.ScenarioID != AudioIntelligibilityScenarioID {
		return fmt.Errorf("audio intelligibility: unsupported scenario_id %q", cfg.ScenarioID)
	}
	if err := validateAudioIntelligibilityFixture(cfg.Fixture); err != nil {
		return err
	}
	if len(cfg.NoiseProfiles) == 0 || len(cfg.LossProfiles) == 0 {
		return fmt.Errorf("audio intelligibility: noise and loss profiles must not be empty")
	}
	if audioIntelligibilityRowCount(cfg) > audioIntelligibilityMaxRows {
		return fmt.Errorf("audio intelligibility: row count would exceed cap %d", audioIntelligibilityMaxRows)
	}
	if strings.TrimSpace(cfg.CodecClass) == "" {
		return fmt.Errorf("audio intelligibility: codec_class must not be empty")
	}
	if cfg.AudioBandwidthHz <= 0 {
		return fmt.Errorf("audio intelligibility: audio_bandwidth_hz must be > 0")
	}
	if cfg.ScoringMethod != AudioIntelligibilityScoringMethod {
		return fmt.Errorf("audio intelligibility: unsupported scoring_method %q", cfg.ScoringMethod)
	}
	if cfg.GoodVoiceThreshold < 1 || cfg.GoodVoiceThreshold > 100 {
		return fmt.Errorf("audio intelligibility: good_voice_threshold=%d must be in [1,100]", cfg.GoodVoiceThreshold)
	}
	if err := validateAudioIntelligibilityNoiseProfiles(cfg.NoiseProfiles); err != nil {
		return err
	}
	return validateAudioIntelligibilityLossProfiles(cfg.LossProfiles)
}

func validateAudioIntelligibilityFixture(fixture AudioIntelligibilityFixture) error {
	if strings.TrimSpace(fixture.FixtureID) == "" {
		return fmt.Errorf("audio intelligibility: fixture_id must not be empty")
	}
	if strings.TrimSpace(fixture.FixturePath) == "" {
		return fmt.Errorf("audio intelligibility: fixture_path must not be empty")
	}
	if !validScenarioEvidenceGrade(fixture.EvidenceGrade) {
		return fmt.Errorf("audio intelligibility: unsupported evidence_grade %q", fixture.EvidenceGrade)
	}
	if fixture.ArtifactSHA256 != "" && !isLowerHexSHA256(fixture.ArtifactSHA256) {
		return fmt.Errorf("audio intelligibility: artifact_sha256 must be a lowercase sha256 hex digest")
	}
	return nil
}

func validateAudioIntelligibilityNoiseProfiles(profiles []AudioIntelligibilityNoiseProfile) error {
	seen := make(map[string]struct{}, len(profiles))
	for _, profile := range profiles {
		if strings.TrimSpace(profile.NoiseProfileID) == "" {
			return fmt.Errorf("audio intelligibility: noise_profile_id must not be empty")
		}
		if profile.NoiseSNRDb < -20 || profile.NoiseSNRDb > 80 {
			return fmt.Errorf("audio intelligibility: noise_snr_db=%d must be in [-20,80]", profile.NoiseSNRDb)
		}
		if _, ok := seen[profile.NoiseProfileID]; ok {
			return fmt.Errorf("audio intelligibility: duplicate noise_profile_id %q", profile.NoiseProfileID)
		}
		seen[profile.NoiseProfileID] = struct{}{}
	}
	return nil
}

func validateAudioIntelligibilityLossProfiles(profiles []AudioIntelligibilityLossProfile) error {
	seen := make(map[string]struct{}, len(profiles))
	for _, profile := range profiles {
		if strings.TrimSpace(profile.LossProfileID) == "" {
			return fmt.Errorf("audio intelligibility: loss_profile_id must not be empty")
		}
		if profile.PacketLossPct < 0 || profile.PacketLossPct > 100 {
			return fmt.Errorf("audio intelligibility: packet_loss_pct=%d must be in [0,100]", profile.PacketLossPct)
		}
		if profile.BurstLossMs < 0 {
			return fmt.Errorf("audio intelligibility: burst_loss_ms=%d must be >= 0", profile.BurstLossMs)
		}
		if _, ok := seen[profile.LossProfileID]; ok {
			return fmt.Errorf("audio intelligibility: duplicate loss_profile_id %q", profile.LossProfileID)
		}
		seen[profile.LossProfileID] = struct{}{}
	}
	return nil
}

func audioIntelligibilityRowCount(cfg AudioIntelligibilityConfig) int {
	return len(cfg.NoiseProfiles) * len(cfg.LossProfiles)
}

func audioIntelligibilityDefaultFixtureSHA256() string {
	sum := sha256.Sum256([]byte(audioIntelligibilityFixtureText))
	return hex.EncodeToString(sum[:])
}

func audioIntelligibilityHash(report AudioIntelligibilityReport) (string, error) {
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

func cloneAudioIntelligibilityNoiseProfiles(profiles []AudioIntelligibilityNoiseProfile) []AudioIntelligibilityNoiseProfile {
	return append([]AudioIntelligibilityNoiseProfile(nil), profiles...)
}

func sortedAudioIntelligibilityNoiseProfiles(profiles []AudioIntelligibilityNoiseProfile) []AudioIntelligibilityNoiseProfile {
	out := cloneAudioIntelligibilityNoiseProfiles(profiles)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].NoiseProfileID < out[j].NoiseProfileID
	})
	return out
}

func cloneAudioIntelligibilityLossProfiles(profiles []AudioIntelligibilityLossProfile) []AudioIntelligibilityLossProfile {
	return append([]AudioIntelligibilityLossProfile(nil), profiles...)
}

func sortedAudioIntelligibilityLossProfiles(profiles []AudioIntelligibilityLossProfile) []AudioIntelligibilityLossProfile {
	out := cloneAudioIntelligibilityLossProfiles(profiles)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].LossProfileID < out[j].LossProfileID
	})
	return out
}
