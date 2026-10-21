package scenario

import (
	"reflect"
	"testing"
)

const testPortableHardwareSHA = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestIT039_PortableHardwareReadinessDefaultReportDeterministic(t *testing.T) {
	first, err := RunPortableHardwareReadinessGate(DefaultPortableHardwareReadinessConfig())
	if err != nil {
		t.Fatalf("RunPortableHardwareReadinessGate failed: %v", err)
	}
	second, err := RunPortableHardwareReadinessGate(DefaultPortableHardwareReadinessConfig())
	if err != nil {
		t.Fatalf("second RunPortableHardwareReadinessGate failed: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("default portable hardware readiness report is not deterministic")
	}
	if first.ScenarioID != PortableHardwareReadinessScenarioID {
		t.Fatalf("ScenarioID = %q, want %q", first.ScenarioID, PortableHardwareReadinessScenarioID)
	}
	if first.GateCount != 6 || len(first.Gates) != 6 {
		t.Fatalf("gate count = %d/%d, want 6", first.GateCount, len(first.Gates))
	}
	if first.BenchExperimentReady {
		t.Fatal("default report unexpectedly says bench experiment is ready")
	}
	if first.HardwareClaimAllowed || first.FieldTXReady {
		t.Fatalf("claim gates = hardware:%v field_tx:%v, want false/false", first.HardwareClaimAllowed, first.FieldTXReady)
	}
	if first.PassedGateCount != 2 || first.MissingEvidenceCount != 3 || first.DeferredGateCount != 1 || first.ReadinessBlockerCount != 3 {
		t.Fatalf("gate counts pass/missing/deferred/blockers = %d/%d/%d/%d, want 2/3/1/3",
			first.PassedGateCount, first.MissingEvidenceCount, first.DeferredGateCount, first.ReadinessBlockerCount)
	}
	if first.CalibrationDecision != TYTBenchDecisionNoLiveTwoHop || first.SelectedTupleSlotMs != 0 {
		t.Fatalf("calibration decision = %q slot=%d, want no_live/0", first.CalibrationDecision, first.SelectedTupleSlotMs)
	}
	if first.Outcome != PortableHardwareOutcomeBlocked {
		t.Fatalf("Outcome = %q, want %q; reasons=%v", first.Outcome, PortableHardwareOutcomeBlocked, first.DegradeReasons)
	}
	if !containsString(first.DegradeReasons, portableHardwareGateChannel) ||
		!containsString(first.DegradeReasons, portableHardwareGateTurnaround) ||
		!containsString(first.DegradeReasons, portableHardwareGateVoice) {
		t.Fatalf("DegradeReasons = %v, want missing channel, turnaround, voice gates", first.DegradeReasons)
	}
	if first.EventHash == "" || first.EventHash != first.Evidence.Determinism.EventHash {
		t.Fatalf("event hash mismatch: report=%q evidence=%q", first.EventHash, first.Evidence.Determinism.EventHash)
	}
	if first.EventHash != "0654b19579207f8b5ea3a79659590736e09c42a311dc45b26641969d37f9b792" {
		t.Fatalf("EventHash = %q, want pinned default hash", first.EventHash)
	}
	if err := ValidateScenarioEvidence(first.Evidence); err != nil {
		t.Fatalf("ValidateScenarioEvidence failed: %v", err)
	}
}

func TestIT039_BenchEvidenceCanMakeBenchReadyWithoutFieldClaim(t *testing.T) {
	cfg := portableHardwareBenchReadyConfig()
	report, err := RunPortableHardwareReadinessGate(cfg)
	if err != nil {
		t.Fatalf("RunPortableHardwareReadinessGate failed: %v", err)
	}
	if !report.BenchExperimentReady || report.Outcome != PortableHardwareOutcomeBenchReady {
		t.Fatalf("bench ready/outcome = %v/%q, want true/%q; reasons=%v",
			report.BenchExperimentReady, report.Outcome, PortableHardwareOutcomeBenchReady, report.DegradeReasons)
	}
	if report.HardwareClaimAllowed || report.FieldTXReady {
		t.Fatalf("claim gates = hardware:%v field_tx:%v, want false/false", report.HardwareClaimAllowed, report.FieldTXReady)
	}
	if report.ReadinessBlockerCount != 0 || report.MissingEvidenceCount != 0 || report.InsufficientEvidenceCount != 0 {
		t.Fatalf("blockers/missing/insufficient = %d/%d/%d, want 0/0/0",
			report.ReadinessBlockerCount, report.MissingEvidenceCount, report.InsufficientEvidenceCount)
	}
	if report.CalibrationDecision != TYTBenchDecisionPrimary24ms || report.SelectedTupleSlotMs != 24 || report.TupleThresholdMs != 10 {
		t.Fatalf("calibration decision/slot/threshold = %q/%d/%d, want primary/24/10",
			report.CalibrationDecision, report.SelectedTupleSlotMs, report.TupleThresholdMs)
	}
}

func TestIT039_TransmitRequestedWithoutLabGateBlocksFieldWork(t *testing.T) {
	cfg := portableHardwareBenchReadyConfig()
	cfg.TransmitCapableWorkRequested = true
	report, err := RunPortableHardwareReadinessGate(cfg)
	if err != nil {
		t.Fatalf("RunPortableHardwareReadinessGate failed: %v", err)
	}
	if report.Outcome != PortableHardwareOutcomeFieldBlocked {
		t.Fatalf("Outcome = %q, want field blocked; reasons=%v", report.Outcome, report.DegradeReasons)
	}
	if report.ReadinessBlockerCount != 1 || !containsString(report.DegradeReasons, portableHardwareGateTxLab) {
		t.Fatalf("blockers/reasons = %d/%v, want tx lab blocker", report.ReadinessBlockerCount, report.DegradeReasons)
	}
	if report.FieldTXReady || report.HardwareClaimAllowed {
		t.Fatalf("field/hardware claim = %v/%v, want false/false", report.FieldTXReady, report.HardwareClaimAllowed)
	}
}

func TestPortableHardwareReadinessRejectsInvalidConfig(t *testing.T) {
	valid := func() PortableHardwareReadinessConfig {
		return portableHardwareBenchReadyConfig()
	}
	tests := []struct {
		name string
		cfg  PortableHardwareReadinessConfig
	}{
		{name: "unsupported scenario", cfg: func() PortableHardwareReadinessConfig {
			cfg := valid()
			cfg.ScenarioID = "other"
			return cfg
		}()},
		{name: "empty candidate", cfg: func() PortableHardwareReadinessConfig {
			cfg := valid()
			cfg.CandidateID = " "
			return cfg
		}()},
		{name: "unsupported channel width", cfg: func() PortableHardwareReadinessConfig {
			cfg := valid()
			cfg.ChannelWidthHz = 50000
			return cfg
		}()},
		{name: "unsupported bitrate", cfg: func() PortableHardwareReadinessConfig {
			cfg := valid()
			cfg.GrossBitrateBps = 38400
			return cfg
		}()},
		{name: "bench turnaround without artifact", cfg: func() PortableHardwareReadinessConfig {
			cfg := valid()
			cfg.TurnaroundArtifactRef = ""
			return cfg
		}()},
		{name: "verified channel without artifact", cfg: func() PortableHardwareReadinessConfig {
			cfg := valid()
			cfg.ChannelArtifactRef = ""
			return cfg
		}()},
		{name: "audio artifact without sha", cfg: func() PortableHardwareReadinessConfig {
			cfg := valid()
			cfg.AudioFixtureSHA256 = ""
			return cfg
		}()},
		{name: "bad sha", cfg: func() PortableHardwareReadinessConfig {
			cfg := valid()
			cfg.AudioFixtureSHA256 = "abc"
			return cfg
		}()},
		{name: "negative turnaround", cfg: func() PortableHardwareReadinessConfig {
			cfg := valid()
			cfg.MaxTurnaroundP95Ms = -1
			return cfg
		}()},
		{name: "bad score", cfg: func() PortableHardwareReadinessConfig {
			cfg := valid()
			cfg.IntelligibilityProxyScore = 101
			return cfg
		}()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := RunPortableHardwareReadinessGate(tc.cfg); err == nil {
				t.Fatal("RunPortableHardwareReadinessGate accepted invalid config")
			}
		})
	}
}

func portableHardwareBenchReadyConfig() PortableHardwareReadinessConfig {
	cfg := DefaultPortableHardwareReadinessConfig()
	cfg.ChannelBenchVerified = true
	cfg.ChannelArtifactRef = "bench/channel-19200-25khz.json"
	cfg.TurnaroundMeasurementSource = TYTBenchMeasurementSourceBenchMeasured
	cfg.TurnaroundArtifactRef = "bench/tyt-turnaround.json"
	cfg.MaxTurnaroundP95Ms = 10
	cfg.AudioFixtureArtifactRef = "fixtures/audio/fm-like-good-voice.json"
	cfg.AudioFixtureSHA256 = testPortableHardwareSHA
	cfg.AudioGoodVoicePass = true
	cfg.IntelligibilityProxyScore = 82
	return cfg
}
