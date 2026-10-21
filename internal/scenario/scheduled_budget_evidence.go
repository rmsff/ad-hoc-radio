package scenario

import "github.com/rmsff/ad-hoc-radio/internal/timing"

type ScheduledWireEvidence struct {
	Profile              timing.VoiceBudgetProfile `json:"profile"`
	Budget               timing.WireBudget         `json:"budget"`
	EvidenceGrade        string                    `json:"evidence_grade"`
	AudioQualityVerified bool                      `json:"audio_quality_verified"`
	HardwareMeasured     bool                      `json:"hardware_measured"`
}

func scheduledExpectedOutcome(value string) string {
	if value == "" {
		return "pass"
	}
	return value
}
func scheduledControlAirtimePct(cfg ScheduledRelayTreeScenarioConfig, result ScheduledRuntimeResult) float64 {
	start := cfg.StartMS
	if cfg.Control != nil {
		start = cfg.Control.StartMS
	}
	elapsed := result.SimDurationMS - start
	if result.ControlCounters == nil || elapsed <= 0 {
		return 0
	}
	return float64(result.ControlCounters.AirtimeUS) / (float64(elapsed) * 1000) * 100
}
