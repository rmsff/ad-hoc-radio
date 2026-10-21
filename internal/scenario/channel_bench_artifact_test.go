package scenario

import (
	"reflect"
	"testing"
)

func TestIT040_ChannelBenchArtifactDefaultReportDeterministic(t *testing.T) {
	first, err := RunChannelBenchArtifactValidation(DefaultChannelBenchArtifactConfig())
	if err != nil {
		t.Fatalf("RunChannelBenchArtifactValidation failed: %v", err)
	}
	second, err := RunChannelBenchArtifactValidation(DefaultChannelBenchArtifactConfig())
	if err != nil {
		t.Fatalf("second RunChannelBenchArtifactValidation failed: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("default channel bench report is not deterministic")
	}
	if first.Outcome != ChannelBenchOutcomeFixtureOnly || first.ChannelBenchVerified {
		t.Fatalf("outcome/verified = %q/%v, want fixture_only/false", first.Outcome, first.ChannelBenchVerified)
	}
	if first.HardwareClaimAllowed || first.MeasuredHardwareCapability {
		t.Fatalf("claim gates = %v/%v, want false/false", first.HardwareClaimAllowed, first.MeasuredHardwareCapability)
	}
	if first.DecodeSuccessPermille != 1000 || first.CRCErrorPermille != 0 {
		t.Fatalf("success/crc permille = %d/%d, want 1000/0", first.DecodeSuccessPermille, first.CRCErrorPermille)
	}
	if first.EventHash == "" || first.EventHash != first.Evidence.Determinism.EventHash {
		t.Fatalf("event hash mismatch: report=%q evidence=%q", first.EventHash, first.Evidence.Determinism.EventHash)
	}
	if first.EventHash != "567876064246976ae52bed269b9a684ce6a26525859014c8b25bf7bea3a0c9b1" {
		t.Fatalf("EventHash = %q, want pinned default hash", first.EventHash)
	}
	if err := ValidateScenarioEvidence(first.Evidence); err != nil {
		t.Fatalf("ValidateScenarioEvidence failed: %v", err)
	}
}

func TestIT040_ChannelBenchMeasuredArtifactCanVerifyChannel(t *testing.T) {
	cfg := DefaultChannelBenchArtifactConfig()
	cfg.MeasurementSource = ChannelBenchMeasurementSourceBenchMeasured
	cfg.TestFixture = false
	cfg.MeasurementArtifactRef = "bench/channel/tyt-19200-25khz.json"
	cfg.MeasurementArtifactSHA256 = testPortableHardwareSHA
	report, err := RunChannelBenchArtifactValidation(cfg)
	if err != nil {
		t.Fatalf("RunChannelBenchArtifactValidation failed: %v", err)
	}
	if !report.ChannelBenchVerified || report.Outcome != ChannelBenchOutcomeVerified {
		t.Fatalf("verified/outcome = %v/%q, want true/%q", report.ChannelBenchVerified, report.Outcome, ChannelBenchOutcomeVerified)
	}
	if !report.HardwareClaimAllowed || !report.MeasuredHardwareCapability {
		t.Fatalf("claim gates = %v/%v, want true/true", report.HardwareClaimAllowed, report.MeasuredHardwareCapability)
	}
}

func TestIT040_ChannelBenchMeasuredArtifactCanFailQualityThreshold(t *testing.T) {
	cfg := DefaultChannelBenchArtifactConfig()
	cfg.MeasurementSource = ChannelBenchMeasurementSourceBenchMeasured
	cfg.TestFixture = false
	cfg.MeasurementArtifactRef = "bench/channel/bad.json"
	cfg.MeasurementArtifactSHA256 = testPortableHardwareSHA
	cfg.FramesDecoded = 900
	report, err := RunChannelBenchArtifactValidation(cfg)
	if err != nil {
		t.Fatalf("RunChannelBenchArtifactValidation failed: %v", err)
	}
	if report.ChannelBenchVerified || report.Outcome != ChannelBenchOutcomeFail {
		t.Fatalf("verified/outcome = %v/%q, want false/fail", report.ChannelBenchVerified, report.Outcome)
	}
	if !containsString(report.DegradeReasons, "decode_success_below_threshold") {
		t.Fatalf("DegradeReasons = %v, want decode threshold reason", report.DegradeReasons)
	}
}

func TestChannelBenchArtifactRejectsInvalidConfig(t *testing.T) {
	valid := func() ChannelBenchArtifactConfig {
		cfg := DefaultChannelBenchArtifactConfig()
		cfg.MeasurementSource = ChannelBenchMeasurementSourceBenchMeasured
		cfg.TestFixture = false
		cfg.MeasurementArtifactRef = "bench/channel/good.json"
		cfg.MeasurementArtifactSHA256 = testPortableHardwareSHA
		return cfg
	}
	tests := []struct {
		name string
		cfg  ChannelBenchArtifactConfig
	}{
		{name: "unsupported scenario", cfg: func() ChannelBenchArtifactConfig {
			cfg := valid()
			cfg.ScenarioID = "other"
			return cfg
		}()},
		{name: "fixture false", cfg: func() ChannelBenchArtifactConfig {
			cfg := valid()
			cfg.MeasurementSource = ChannelBenchMeasurementSourceFixtureOnly
			cfg.TestFixture = false
			return cfg
		}()},
		{name: "bench fixture true", cfg: func() ChannelBenchArtifactConfig {
			cfg := valid()
			cfg.TestFixture = true
			return cfg
		}()},
		{name: "bench without artifact", cfg: func() ChannelBenchArtifactConfig {
			cfg := valid()
			cfg.MeasurementArtifactRef = ""
			return cfg
		}()},
		{name: "bench without artifact sha", cfg: func() ChannelBenchArtifactConfig {
			cfg := valid()
			cfg.MeasurementArtifactSHA256 = ""
			return cfg
		}()},
		{name: "bad artifact sha", cfg: func() ChannelBenchArtifactConfig {
			cfg := valid()
			cfg.MeasurementArtifactSHA256 = "abc"
			return cfg
		}()},
		{name: "bad channel width", cfg: func() ChannelBenchArtifactConfig {
			cfg := valid()
			cfg.ChannelWidthHz = 50000
			return cfg
		}()},
		{name: "bad bitrate", cfg: func() ChannelBenchArtifactConfig {
			cfg := valid()
			cfg.GrossBitrateBps = 38400
			return cfg
		}()},
		{name: "decoded over sent", cfg: func() ChannelBenchArtifactConfig {
			cfg := valid()
			cfg.FramesDecoded = cfg.FramesSent + 1
			return cfg
		}()},
		{name: "negative crc", cfg: func() ChannelBenchArtifactConfig {
			cfg := valid()
			cfg.CRCErrorCount = -1
			return cfg
		}()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := RunChannelBenchArtifactValidation(tc.cfg); err == nil {
				t.Fatal("RunChannelBenchArtifactValidation accepted invalid config")
			}
		})
	}
}
