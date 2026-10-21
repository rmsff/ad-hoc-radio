package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

const TYTProxyTurnaroundScenarioID = "tyt_proxy_turnaround_profile_simulation"

const (
	TYTProxyEvidenceProjectAssumption    = "project_assumption"
	TYTProxyEvidenceStandardLimit        = "standard_limit"
	TYTProxyEvidenceModuleDatasheet      = "module_datasheet"
	TYTProxyEvidenceMeasuredRadioReview  = "measured_radio_review"
	TYTProxyEvidenceDMRHandheldLabReview = "dmr_handheld_lab_review"
	TYTProxyEvidenceCommercialRadioSpec  = "commercial_radio_spec"
	tytProxyMaxTurnaroundProfileRowCount = 64
)

type TYTProxyTurnaroundConfig struct {
	ScenarioID string
	Seed       uint64
	Profiles   []TYTProxyTurnaroundProfile
}

type TYTProxyTurnaroundProfile struct {
	ProfileID           string
	EvidenceGrade       string
	SourceName          string
	SourceURL           string
	SourceMetric        string
	ChannelWidthHz      int
	GrossBitrateBps     int64
	PreambleMs          int64
	SyncMs              int64
	LinkDelayMs         int64
	RxToTxP50Ms         int64
	RxToTxP95Ms         int64
	TxToRxP50Ms         int64
	TxToRxP95Ms         int64
	RxValidAfterTxP50Ms int64
	RxValidAfterTxP95Ms int64
	ApplicabilityToTYT  string
	Interpretation      string
	Notes               string
}

type TYTProxyTurnaroundReport struct {
	ScenarioID                 string                  `json:"scenario_id"`
	Seed                       uint64                  `json:"seed"`
	RadioFamily                string                  `json:"radio_family"`
	AssumptionOnly             bool                    `json:"assumption_only"`
	MeasuredHardwareCapability bool                    `json:"measured_hardware_capability"`
	HardwareClaimAllowed       bool                    `json:"hardware_claim_allowed"`
	Rows                       []TYTProxyTurnaroundRow `json:"rows"`
	RowCount                   int                     `json:"row_count"`
	PublicProxyRows            int                     `json:"public_proxy_rows"`
	ViableRows                 int                     `json:"viable_rows"`
	NoLiveTwoHopRows           int                     `json:"no_live_two_hop_rows"`
	EventHash                  string                  `json:"event_hash"`
}

type TYTProxyTurnaroundRow struct {
	ProfileID                     string   `json:"profile_id"`
	EvidenceGrade                 string   `json:"evidence_grade"`
	SourceName                    string   `json:"source_name"`
	SourceURL                     string   `json:"source_url"`
	SourceMetric                  string   `json:"source_metric"`
	ChannelWidthHz                int      `json:"channel_width_hz"`
	GrossBitrateBps               int64    `json:"gross_bitrate_bps"`
	PreambleMs                    int64    `json:"preamble_ms"`
	SyncMs                        int64    `json:"sync_ms"`
	LinkDelayMs                   int64    `json:"link_delay_ms"`
	RxToTxP50Ms                   int64    `json:"rx_to_tx_p50_ms"`
	RxToTxP95Ms                   int64    `json:"rx_to_tx_p95_ms"`
	TxToRxP50Ms                   int64    `json:"tx_to_rx_p50_ms"`
	TxToRxP95Ms                   int64    `json:"tx_to_rx_p95_ms"`
	RxValidAfterTxP50Ms           int64    `json:"rx_valid_after_tx_p50_ms"`
	RxValidAfterTxP95Ms           int64    `json:"rx_valid_after_tx_p95_ms"`
	MaxRequiredTurnaroundP95Ms    int64    `json:"max_required_turnaround_p95_ms"`
	LimitingComponents            []string `json:"limiting_components"`
	CalibrationDecision           string   `json:"calibration_decision"`
	SelectedTupleSlotMs           int64    `json:"selected_tuple_slot_ms"`
	TupleThresholdMs              int64    `json:"tuple_threshold_ms"`
	LiveTwoHopTupleViable         bool     `json:"live_two_hop_tuple_viable"`
	ApplicabilityToTYT            string   `json:"applicability_to_tyt"`
	Interpretation                string   `json:"interpretation"`
	MeasuredTYTHardwareCapability bool     `json:"measured_tyt_hardware_capability"`
	HardwareClaimAllowed          bool     `json:"hardware_claim_allowed"`
	Notes                         string   `json:"notes"`
}

func DefaultTYTProxyTurnaroundConfig() TYTProxyTurnaroundConfig {
	profiles := []TYTProxyTurnaroundProfile{
		{
			ProfileID:           "project_primary_threshold_10ms",
			EvidenceGrade:       TYTProxyEvidenceProjectAssumption,
			SourceName:          "MVP-8.2/MVP-8.3 simulator threshold",
			SourceURL:           "docs/research/010-tyt-tx-rx-switch-measurement-plan.md",
			SourceMetric:        "primary tuple threshold",
			ChannelWidthHz:      tytLikelyChannelWidthHz,
			GrossBitrateBps:     19200,
			PreambleMs:          1,
			SyncMs:              1,
			LinkDelayMs:         1,
			RxToTxP50Ms:         10,
			RxToTxP95Ms:         10,
			TxToRxP50Ms:         10,
			TxToRxP95Ms:         10,
			RxValidAfterTxP50Ms: 10,
			RxValidAfterTxP95Ms: 10,
			ApplicabilityToTYT:  "target only, not evidence",
			Interpretation:      "current primary tuple survives only if worst relevant turnaround p95 is <=10 ms",
			Notes:               "local simulator threshold anchor",
		},
		{
			ProfileID:           "etsi_en300113_lmr_attack_release_limit",
			EvidenceGrade:       TYTProxyEvidenceStandardLimit,
			SourceName:          "ETSI EN 300 113-1 V1.6.2",
			SourceURL:           "https://www.etsi.org/deliver/etsi_en/300100_300199/30011301/01.06.02_60/en_30011301v010602p.pdf",
			SourceMetric:        "transmitter attack <=25 ms, release <=20 ms",
			ChannelWidthHz:      tytLikelyChannelWidthHz,
			GrossBitrateBps:     19200,
			PreambleMs:          1,
			SyncMs:              1,
			LinkDelayMs:         1,
			RxToTxP50Ms:         25,
			RxToTxP95Ms:         25,
			TxToRxP50Ms:         20,
			TxToRxP95Ms:         20,
			RxValidAfterTxP50Ms: 20,
			RxValidAfterTxP95Ms: 20,
			ApplicabilityToTYT:  "conservative land-mobile standard proxy",
			Interpretation:      "if TYT only behaves around generic LMR transient limits, current live two-hop tuple is not viable",
			Notes:               "standard transmitter transient limits, not a TYT receiver-demodulation measurement",
		},
		{
			ProfileID:           "lmd401_module_max_switching_time",
			EvidenceGrade:       TYTProxyEvidenceModuleDatasheet,
			SourceName:          "Circuit Design LMD-401 operation guide",
			SourceURL:           "https://www.cdt21.com/wp/wp-content/uploads/support/download/docs/OG_LMD-401.pdf",
			SourceMetric:        "TX/RX switching time 15 ms typ, 20 ms max",
			ChannelWidthHz:      12500,
			GrossBitrateBps:     9600,
			PreambleMs:          1,
			SyncMs:              1,
			LinkDelayMs:         1,
			RxToTxP50Ms:         15,
			RxToTxP95Ms:         20,
			TxToRxP50Ms:         15,
			TxToRxP95Ms:         20,
			RxValidAfterTxP50Ms: 15,
			RxValidAfterTxP95Ms: 20,
			ApplicabilityToTYT:  "proxy for a purpose-built half-duplex data module, not TYT",
			Interpretation:      "even a data-module max switching value exceeds the current 30 ms fallback threshold",
			Notes:               "datasheet also recommends preamble >20 ms, which is outside the MVP-8.2 1 ms envelope",
		},
		{
			ProfileID:           "p25_c4fm_voice_standard_limit",
			EvidenceGrade:       TYTProxyEvidenceStandardLimit,
			SourceName:          "TIA-102.CAAB-C P25 C4FM/CQPSK recommendations",
			SourceURL:           "https://www.qsl.net/kb9mwr/projects/dv/apco25/TIA-102.CAAB-C-2010.pdf",
			SourceMetric:        "power attack <=50 ms, encoder attack <=100 ms",
			ChannelWidthHz:      12500,
			GrossBitrateBps:     9600,
			PreambleMs:          1,
			SyncMs:              1,
			LinkDelayMs:         1,
			RxToTxP50Ms:         100,
			RxToTxP95Ms:         100,
			TxToRxP50Ms:         50,
			TxToRxP95Ms:         50,
			RxValidAfterTxP50Ms: 100,
			RxValidAfterTxP95Ms: 100,
			ApplicabilityToTYT:  "conservative digital-voice standard proxy",
			Interpretation:      "P25-class attack budgets are far slower than current live two-hop timing assumptions",
			Notes:               "not a custom TYT bit-pipe measurement",
		},
		{
			ProfileID:           "bridgecom_bch220_handheld_audio_turnaround",
			EvidenceGrade:       TYTProxyEvidenceMeasuredRadioReview,
			SourceName:          "BridgeCom BCH-220 QST product review excerpt",
			SourceURL:           "https://device.report/m/0a47900c4b9964e51af8086c80325f34b10c405abfa9bfee2846561516e1ab0d.pdf",
			SourceMetric:        "TX->RX 196 ms to 50% audio, RX->TX 40 ms",
			ChannelWidthHz:      25000,
			GrossBitrateBps:     9600,
			PreambleMs:          1,
			SyncMs:              1,
			LinkDelayMs:         1,
			RxToTxP50Ms:         40,
			RxToTxP95Ms:         40,
			TxToRxP50Ms:         196,
			TxToRxP95Ms:         196,
			RxValidAfterTxP50Ms: 196,
			RxValidAfterTxP95Ms: 196,
			ApplicabilityToTYT:  "lower-cost handheld audio-path proxy, not TYT",
			Interpretation:      "audio-path handheld turnaround is unusable for current live two-hop slot assumptions",
			Notes:               "analog audio product-review metric, included as a deliberately pessimistic proxy",
		},
	}
	profiles = append(profiles, defaultTYTDMRHandheldProxyProfiles()...)
	return TYTProxyTurnaroundConfig{
		ScenarioID: TYTProxyTurnaroundScenarioID,
		Seed:       DefaultScenarioSeed,
		Profiles:   profiles,
	}
}

func RunTYTProxyTurnaroundSimulation(cfg TYTProxyTurnaroundConfig) (TYTProxyTurnaroundReport, error) {
	cfg = normalizeTYTProxyTurnaroundConfig(cfg)
	if err := validateTYTProxyTurnaroundConfig(cfg); err != nil {
		return TYTProxyTurnaroundReport{}, err
	}

	report := TYTProxyTurnaroundReport{
		ScenarioID:                 cfg.ScenarioID,
		Seed:                       cfg.Seed,
		RadioFamily:                tytLikelyRadioFamily,
		AssumptionOnly:             true,
		MeasuredHardwareCapability: false,
		HardwareClaimAllowed:       false,
		Rows:                       make([]TYTProxyTurnaroundRow, 0, len(cfg.Profiles)),
	}
	for _, profile := range cfg.Profiles {
		row, err := tytProxyTurnaroundRow(profile)
		if err != nil {
			return TYTProxyTurnaroundReport{}, err
		}
		report.Rows = append(report.Rows, row)
		if profile.EvidenceGrade != TYTProxyEvidenceProjectAssumption {
			report.PublicProxyRows++
		}
		if row.LiveTwoHopTupleViable {
			report.ViableRows++
		} else {
			report.NoLiveTwoHopRows++
		}
	}
	report.RowCount = len(report.Rows)

	hash, err := tytProxyTurnaroundHash(report)
	if err != nil {
		return TYTProxyTurnaroundReport{}, err
	}
	report.EventHash = hash
	return report, nil
}

func (r TYTProxyTurnaroundReport) MarshalReportJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

func normalizeTYTProxyTurnaroundConfig(cfg TYTProxyTurnaroundConfig) TYTProxyTurnaroundConfig {
	def := DefaultTYTProxyTurnaroundConfig()
	if cfg.ScenarioID == "" {
		cfg.ScenarioID = def.ScenarioID
	}
	if cfg.Seed == 0 {
		cfg.Seed = def.Seed
	}
	if cfg.Profiles == nil {
		cfg.Profiles = append([]TYTProxyTurnaroundProfile(nil), def.Profiles...)
	} else {
		cfg.Profiles = append([]TYTProxyTurnaroundProfile(nil), cfg.Profiles...)
	}
	for i := range cfg.Profiles {
		cfg.Profiles[i] = normalizeTYTProxyTurnaroundProfile(cfg.Profiles[i])
	}
	sort.Slice(cfg.Profiles, func(i, j int) bool {
		return cfg.Profiles[i].ProfileID < cfg.Profiles[j].ProfileID
	})
	return cfg
}

func normalizeTYTProxyTurnaroundProfile(profile TYTProxyTurnaroundProfile) TYTProxyTurnaroundProfile {
	profile.ProfileID = strings.TrimSpace(profile.ProfileID)
	profile.EvidenceGrade = strings.TrimSpace(profile.EvidenceGrade)
	profile.SourceName = strings.TrimSpace(profile.SourceName)
	profile.SourceURL = strings.TrimSpace(profile.SourceURL)
	profile.SourceMetric = strings.TrimSpace(profile.SourceMetric)
	profile.ApplicabilityToTYT = strings.TrimSpace(profile.ApplicabilityToTYT)
	profile.Interpretation = strings.TrimSpace(profile.Interpretation)
	profile.Notes = strings.TrimSpace(profile.Notes)
	return profile
}

func validateTYTProxyTurnaroundConfig(cfg TYTProxyTurnaroundConfig) error {
	if cfg.ScenarioID != TYTProxyTurnaroundScenarioID {
		return fmt.Errorf("tyt proxy turnaround: unsupported scenario_id %q", cfg.ScenarioID)
	}
	if len(cfg.Profiles) == 0 {
		return fmt.Errorf("tyt proxy turnaround: at least one profile is required")
	}
	if len(cfg.Profiles) > tytProxyMaxTurnaroundProfileRowCount {
		return fmt.Errorf("tyt proxy turnaround: profile count %d exceeds cap %d", len(cfg.Profiles), tytProxyMaxTurnaroundProfileRowCount)
	}
	seen := make(map[string]struct{}, len(cfg.Profiles))
	for _, profile := range cfg.Profiles {
		if err := validateTYTProxyTurnaroundProfile(profile); err != nil {
			return err
		}
		if _, exists := seen[profile.ProfileID]; exists {
			return fmt.Errorf("tyt proxy turnaround: duplicate profile_id %q", profile.ProfileID)
		}
		seen[profile.ProfileID] = struct{}{}
	}
	return nil
}

func validateTYTProxyTurnaroundProfile(profile TYTProxyTurnaroundProfile) error {
	if profile.ProfileID == "" {
		return fmt.Errorf("tyt proxy turnaround: profile_id must not be empty")
	}
	switch profile.EvidenceGrade {
	case TYTProxyEvidenceProjectAssumption,
		TYTProxyEvidenceStandardLimit,
		TYTProxyEvidenceModuleDatasheet,
		TYTProxyEvidenceMeasuredRadioReview,
		TYTProxyEvidenceDMRHandheldLabReview,
		TYTProxyEvidenceCommercialRadioSpec:
	default:
		return fmt.Errorf("tyt proxy turnaround: unsupported evidence_grade %q for profile %q", profile.EvidenceGrade, profile.ProfileID)
	}
	if profile.SourceName == "" || profile.SourceMetric == "" {
		return fmt.Errorf("tyt proxy turnaround: source_name and source_metric must not be empty for profile %q", profile.ProfileID)
	}
	if profile.EvidenceGrade != TYTProxyEvidenceProjectAssumption {
		if err := validateTYTProxyPublicSourceURL(profile.ProfileID, profile.SourceURL); err != nil {
			return err
		}
	}
	if profile.ChannelWidthHz != 12500 && profile.ChannelWidthHz != 25000 {
		return fmt.Errorf("tyt proxy turnaround: unsupported channel_width_hz=%d for profile %q", profile.ChannelWidthHz, profile.ProfileID)
	}
	if profile.GrossBitrateBps != 9600 && profile.GrossBitrateBps != 19200 {
		return fmt.Errorf("tyt proxy turnaround: unsupported gross_bitrate_bps=%d for profile %q", profile.GrossBitrateBps, profile.ProfileID)
	}
	if profile.PreambleMs != 1 || profile.SyncMs != 1 || profile.LinkDelayMs != 1 {
		return fmt.Errorf("tyt proxy turnaround: profile %q must stay in MVP-8.2 envelope preamble=1 sync=1 link_delay=1", profile.ProfileID)
	}
	if err := validateTYTProxyMetricPair(profile.ProfileID, "rx_to_tx", profile.RxToTxP50Ms, profile.RxToTxP95Ms); err != nil {
		return err
	}
	if err := validateTYTProxyMetricPair(profile.ProfileID, "tx_to_rx", profile.TxToRxP50Ms, profile.TxToRxP95Ms); err != nil {
		return err
	}
	return validateTYTProxyMetricPair(profile.ProfileID, "rx_valid_after_tx", profile.RxValidAfterTxP50Ms, profile.RxValidAfterTxP95Ms)
}

func validateTYTProxyMetricPair(profileID, metric string, p50, p95 int64) error {
	if p50 < 0 || p95 < 0 {
		return fmt.Errorf("tyt proxy turnaround: %s values must be >= 0 for profile %q", metric, profileID)
	}
	if p50 > p95 {
		return fmt.Errorf("tyt proxy turnaround: %s p50_ms=%d exceeds p95_ms=%d for profile %q", metric, p50, p95, profileID)
	}
	return nil
}

func validateTYTProxyPublicSourceURL(profileID, raw string) error {
	if raw == "" {
		return fmt.Errorf("tyt proxy turnaround: source_url must not be empty for public proxy profile %q", profileID)
	}
	parsed, err := url.Parse(raw)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return fmt.Errorf("tyt proxy turnaround: source_url must be an absolute URL for public proxy profile %q", profileID)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return fmt.Errorf("tyt proxy turnaround: source_url scheme %q unsupported for public proxy profile %q", parsed.Scheme, profileID)
	}
	return nil
}

func tytProxyTurnaroundRow(profile TYTProxyTurnaroundProfile) (TYTProxyTurnaroundRow, error) {
	calibration, err := RunTYTBenchMeasurementCalibration(tytProxyBenchConfig(profile))
	if err != nil {
		return TYTProxyTurnaroundRow{}, err
	}
	return TYTProxyTurnaroundRow{
		ProfileID:                     profile.ProfileID,
		EvidenceGrade:                 profile.EvidenceGrade,
		SourceName:                    profile.SourceName,
		SourceURL:                     profile.SourceURL,
		SourceMetric:                  profile.SourceMetric,
		ChannelWidthHz:                profile.ChannelWidthHz,
		GrossBitrateBps:               profile.GrossBitrateBps,
		PreambleMs:                    profile.PreambleMs,
		SyncMs:                        profile.SyncMs,
		LinkDelayMs:                   profile.LinkDelayMs,
		RxToTxP50Ms:                   profile.RxToTxP50Ms,
		RxToTxP95Ms:                   profile.RxToTxP95Ms,
		TxToRxP50Ms:                   profile.TxToRxP50Ms,
		TxToRxP95Ms:                   profile.TxToRxP95Ms,
		RxValidAfterTxP50Ms:           profile.RxValidAfterTxP50Ms,
		RxValidAfterTxP95Ms:           profile.RxValidAfterTxP95Ms,
		MaxRequiredTurnaroundP95Ms:    calibration.MaxRequiredTurnaroundP95Ms,
		LimitingComponents:            append([]string(nil), calibration.LimitingComponents...),
		CalibrationDecision:           calibration.CalibrationDecision,
		SelectedTupleSlotMs:           calibration.SelectedTupleSlotMs,
		TupleThresholdMs:              calibration.TupleThresholdMs,
		LiveTwoHopTupleViable:         calibration.CalibrationDecision != TYTBenchDecisionNoLiveTwoHop,
		ApplicabilityToTYT:            profile.ApplicabilityToTYT,
		Interpretation:                profile.Interpretation,
		MeasuredTYTHardwareCapability: false,
		HardwareClaimAllowed:          false,
		Notes:                         profile.Notes,
	}, nil
}

func tytProxyBenchConfig(profile TYTProxyTurnaroundProfile) TYTBenchMeasurementConfig {
	return TYTBenchMeasurementConfig{
		ScenarioID:             TYTBenchMeasurementScenarioID,
		Seed:                   DefaultScenarioSeed,
		MeasurementSource:      TYTBenchMeasurementSourceUnknown,
		RadioModel:             tytLikelyRadioFamily,
		TestFixture:            false,
		MeasurementArtifactRef: profile.SourceURL,
		FirmwarePath:           "proxy_public_source",
		ChannelWidthHz:         profile.ChannelWidthHz,
		GrossBitrateBps:        profile.GrossBitrateBps,
		PreambleMs:             profile.PreambleMs,
		SyncMs:                 profile.SyncMs,
		LinkDelayMs:            profile.LinkDelayMs,
		RxToTxP50Ms:            int64Ptr(profile.RxToTxP50Ms),
		RxToTxP95Ms:            int64Ptr(profile.RxToTxP95Ms),
		TxToRxP50Ms:            int64Ptr(profile.TxToRxP50Ms),
		TxToRxP95Ms:            int64Ptr(profile.TxToRxP95Ms),
		RxValidAfterTxP50Ms:    int64Ptr(profile.RxValidAfterTxP50Ms),
		RxValidAfterTxP95Ms:    int64Ptr(profile.RxValidAfterTxP95Ms),
		Notes:                  profile.Notes,
	}
}

func tytProxyTurnaroundHash(report TYTProxyTurnaroundReport) (string, error) {
	hashable := report
	hashable.EventHash = ""
	b, err := json.Marshal(hashable)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
