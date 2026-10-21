package scenario

import "encoding/json"

const PortableHardwareReadinessScenarioID = "portable_hardware_readiness_gate"
const PortableHardwareReadinessStoryID = "MVP-8.12"
const PortableHardwareReadinessEvidenceType = "portable_hardware_readiness_gate"

const (
	PortableHardwareGateStatusPass         = "pass"
	PortableHardwareGateStatusMissing      = "missing"
	PortableHardwareGateStatusInsufficient = "insufficient"
	PortableHardwareGateStatusDeferred     = "deferred"
)

const (
	PortableHardwareOutcomeBenchReady   = "hardware_bench_ready"
	PortableHardwareOutcomeBlocked      = "hardware_readiness_blocked"
	PortableHardwareOutcomeFieldBlocked = "field_tx_blocked"
)

const (
	portableHardwareGateCandidate  = "candidate_selected"
	portableHardwareGateFirmware   = "firmware_inspectable"
	portableHardwareGateChannel    = "channel_bitrate_bench_verified"
	portableHardwareGateTurnaround = "tx_rx_turnaround_bench_measured"
	portableHardwareGateVoice      = "voice_quality_fixture_artifact"
	portableHardwareGateTxLab      = "tx_lab_preconditions_documented"
)

type PortableHardwareReadinessConfig struct {
	ScenarioID                     string
	Seed                           uint64
	CandidateID                    string
	RadioFamily                    string
	FirmwarePath                   string
	FirmwareInspectable            bool
	ChannelWidthHz                 int
	GrossBitrateBps                int64
	ChannelBenchVerified           bool
	ChannelArtifactRef             string
	TurnaroundMeasurementSource    string
	TurnaroundArtifactRef          string
	MaxTurnaroundP95Ms             int64
	AudioFixtureArtifactRef        string
	AudioFixtureSHA256             string
	AudioGoodVoicePass             bool
	IntelligibilityProxyScore      int
	GoodVoiceThreshold             int
	LabSafetyPlanDocumented        bool
	RegulatoryProfileDocumented    bool
	TransmitCapableWorkRequested   bool
	TransmitRequiresSeparateGoNoGo bool
}

type PortableHardwareReadinessReport struct {
	ScenarioID                      string                             `json:"scenario_id"`
	Seed                            uint64                             `json:"seed"`
	Evidence                        ScenarioEvidence                   `json:"evidence"`
	CandidateID                     string                             `json:"candidate_id"`
	RadioFamily                     string                             `json:"radio_family"`
	FirmwarePath                    string                             `json:"firmware_path"`
	FirmwareInspectable             bool                               `json:"firmware_inspectable"`
	ChannelWidthHz                  int                                `json:"channel_width_hz"`
	GrossBitrateBps                 int64                              `json:"gross_bitrate_bps"`
	TurnaroundMeasurementSource     string                             `json:"turnaround_measurement_source"`
	MaxTurnaroundP95Ms              int64                              `json:"max_turnaround_p95_ms"`
	CalibrationDecision             string                             `json:"calibration_decision"`
	SelectedTupleSlotMs             int64                              `json:"selected_tuple_slot_ms"`
	TupleThresholdMs                int64                              `json:"tuple_threshold_ms"`
	IntelligibilityProxyScore       int                                `json:"intelligibility_proxy_score"`
	GoodVoiceThreshold              int                                `json:"good_voice_threshold"`
	Gates                           []PortableHardwareReadinessGateRow `json:"gates"`
	GateCount                       int                                `json:"gate_count"`
	PassedGateCount                 int                                `json:"passed_gate_count"`
	MissingEvidenceCount            int                                `json:"missing_evidence_count"`
	InsufficientEvidenceCount       int                                `json:"insufficient_evidence_count"`
	DeferredGateCount               int                                `json:"deferred_gate_count"`
	ReadinessBlockerCount           int                                `json:"readiness_blocker_count"`
	BenchExperimentReady            bool                               `json:"bench_experiment_ready"`
	TransmitCapableWorkRequested    bool                               `json:"transmit_capable_work_requested"`
	TransmitPreconditionsDocumented bool                               `json:"transmit_preconditions_documented"`
	TransmitRequiresSeparateGoNoGo  bool                               `json:"transmit_requires_separate_go_no_go"`
	FieldTXReady                    bool                               `json:"field_tx_ready"`
	HardwareClaimAllowed            bool                               `json:"hardware_claim_allowed"`
	Outcome                         string                             `json:"outcome"`
	DegradeReasons                  []string                           `json:"degrade_reasons"`
	EventHash                       string                             `json:"event_hash"`
}

type PortableHardwareReadinessGateRow struct {
	GateID      string `json:"gate_id"`
	Category    string `json:"category"`
	RequiredFor string `json:"required_for"`
	Status      string `json:"status"`
	Blocking    bool   `json:"blocking"`
	Observed    string `json:"observed"`
	Required    string `json:"required"`
	EvidenceRef string `json:"evidence_ref"`
	Reason      string `json:"reason"`
}

func DefaultPortableHardwareReadinessConfig() PortableHardwareReadinessConfig {
	return PortableHardwareReadinessConfig{
		ScenarioID:                     PortableHardwareReadinessScenarioID,
		Seed:                           DefaultScenarioSeed,
		CandidateID:                    "tyt_md_uv380_bench_candidate",
		RadioFamily:                    tytLikelyRadioFamily,
		FirmwarePath:                   "openrtx_derived_bench",
		FirmwareInspectable:            true,
		ChannelWidthHz:                 tytLikelyChannelWidthHz,
		GrossBitrateBps:                19200,
		ChannelBenchVerified:           false,
		TurnaroundMeasurementSource:    TYTBenchMeasurementSourceFixtureOnly,
		MaxTurnaroundP95Ms:             200,
		IntelligibilityProxyScore:      82,
		GoodVoiceThreshold:             80,
		TransmitRequiresSeparateGoNoGo: true,
	}
}

func RunPortableHardwareReadinessGate(cfg PortableHardwareReadinessConfig) (PortableHardwareReadinessReport, error) {
	cfg = normalizePortableHardwareReadinessConfig(cfg)
	if err := validatePortableHardwareReadinessConfig(cfg); err != nil {
		return PortableHardwareReadinessReport{}, err
	}
	decision, slotMs, thresholdMs := tytBenchCalibrationDecision(cfg.MaxTurnaroundP95Ms)
	report := PortableHardwareReadinessReport{
		ScenarioID:                      cfg.ScenarioID,
		Seed:                            cfg.Seed,
		CandidateID:                     cfg.CandidateID,
		RadioFamily:                     cfg.RadioFamily,
		FirmwarePath:                    cfg.FirmwarePath,
		FirmwareInspectable:             cfg.FirmwareInspectable,
		ChannelWidthHz:                  cfg.ChannelWidthHz,
		GrossBitrateBps:                 cfg.GrossBitrateBps,
		TurnaroundMeasurementSource:     cfg.TurnaroundMeasurementSource,
		MaxTurnaroundP95Ms:              cfg.MaxTurnaroundP95Ms,
		CalibrationDecision:             decision,
		SelectedTupleSlotMs:             slotMs,
		TupleThresholdMs:                thresholdMs,
		IntelligibilityProxyScore:       cfg.IntelligibilityProxyScore,
		GoodVoiceThreshold:              cfg.GoodVoiceThreshold,
		TransmitCapableWorkRequested:    cfg.TransmitCapableWorkRequested,
		TransmitPreconditionsDocumented: cfg.LabSafetyPlanDocumented && cfg.RegulatoryProfileDocumented,
		TransmitRequiresSeparateGoNoGo:  cfg.TransmitRequiresSeparateGoNoGo,
		FieldTXReady:                    false,
		HardwareClaimAllowed:            false,
	}
	report.Gates = portableHardwareReadinessGates(cfg, report)
	report.GateCount = len(report.Gates)
	for _, gate := range report.Gates {
		portableHardwareAccumulateGate(&report, gate)
	}
	report.BenchExperimentReady = report.ReadinessBlockerCount == 0 ||
		(report.ReadinessBlockerCount == 1 && portableHardwareOnlyTxLabBlocks(report.Gates))
	report.Outcome, report.DegradeReasons = portableHardwareReadinessOutcome(report)
	report.Evidence = portableHardwareEvidence(cfg, report, "")
	hash, err := portableHardwareHash(report)
	if err != nil {
		return PortableHardwareReadinessReport{}, err
	}
	report.EventHash = hash
	report.Evidence = portableHardwareEvidence(cfg, report, hash)
	if err := ValidateScenarioEvidence(report.Evidence); err != nil {
		return PortableHardwareReadinessReport{}, err
	}
	return report, nil
}

func (r PortableHardwareReadinessReport) MarshalReportJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
