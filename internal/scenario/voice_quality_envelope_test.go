package scenario

import (
	"reflect"
	"testing"
)

func TestIT033_VoiceQualityEnvelopeDefaultReportDeterministic(t *testing.T) {
	first, err := RunVoiceQualityEnvelopeSimulation(DefaultVoiceQualityEnvelopeConfig())
	if err != nil {
		t.Fatalf("RunVoiceQualityEnvelopeSimulation failed: %v", err)
	}
	second, err := RunVoiceQualityEnvelopeSimulation(DefaultVoiceQualityEnvelopeConfig())
	if err != nil {
		t.Fatalf("second RunVoiceQualityEnvelopeSimulation failed: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("default voice quality envelope report is not deterministic")
	}
	if first.ScenarioID != VoiceQualityEnvelopeScenarioID {
		t.Fatalf("ScenarioID = %q, want %q", first.ScenarioID, VoiceQualityEnvelopeScenarioID)
	}
	if first.LiveRelayViable {
		t.Fatal("LiveRelayViable = true, want false for 200 ms turnaround")
	}
	if !first.AssumptionOnly {
		t.Fatal("AssumptionOnly = false, want true")
	}
	if first.RowCount != 12 || len(first.Rows) != 12 {
		t.Fatalf("row count = %d/%d, want 12", first.RowCount, len(first.Rows))
	}
	if first.FMLikeRows != 9 || first.FMLikePassRows != 3 || first.FMLikeFailRows != 6 {
		t.Fatalf("fm-like rows/pass/fail = %d/%d/%d, want 9/3/6",
			first.FMLikeRows, first.FMLikePassRows, first.FMLikeFailRows)
	}
	if !first.GoodQualitySupported {
		t.Fatal("GoodQualitySupported = false, want true")
	}
	if first.SelectedGoodQualityProfile != "fm_like_wideband_12k_primary" {
		t.Fatalf("SelectedGoodQualityProfile = %q, want fm_like_wideband_12k_primary", first.SelectedGoodQualityProfile)
	}
	if first.EventHash == "" {
		t.Fatal("EventHash is empty")
	}
}

func TestIT033_PrimaryFMLikeTwoRelayProfileFitsAt200msTurnaround(t *testing.T) {
	report, err := RunVoiceQualityEnvelopeSimulation(DefaultVoiceQualityEnvelopeConfig())
	if err != nil {
		t.Fatalf("RunVoiceQualityEnvelopeSimulation failed: %v", err)
	}
	row := mustFindVoiceQualityEnvelopeRow(t, report.Rows, "fm_like_wideband_12k_primary_relays_2")
	if row.Outcome != VoiceQualityOutcomePass {
		t.Fatalf("outcome = %q, want %q; reasons=%v", row.Outcome, VoiceQualityOutcomePass, row.FailReasons)
	}
	if row.CodecBitrateBps != 12000 || row.AudioBandwidthHz != 8000 || row.QualityScore != 82 {
		t.Fatalf("quality profile bitrate/bandwidth/score = %d/%d/%d, want 12000/8000/82",
			row.CodecBitrateBps, row.AudioBandwidthHz, row.QualityScore)
	}
	if row.EncodedVoiceBytes != 750 || row.ProtectedVoiceBytes != 863 || row.FrameCount != 9 {
		t.Fatalf("payload bytes protected/frame count = %d/%d/%d, want 750/863/9",
			row.EncodedVoiceBytes, row.ProtectedVoiceBytes, row.FrameCount)
	}
	if row.MaxWireFrameBytes != 110 || row.AirtimeMs != 46 || row.BurstTxDurationMs != 440 {
		t.Fatalf("wire/airtime/burst tx = %d/%d/%d, want 110/46/440",
			row.MaxWireFrameBytes, row.AirtimeMs, row.BurstTxDurationMs)
	}
	if row.FirstAudioLatencyMs != 1391 || row.CompletionMs != 1783 || row.MaxUserPerceivedLatencyMs != 1391 {
		t.Fatalf("latency first/completion/max = %d/%d/%d, want 1391/1783/1391",
			row.FirstAudioLatencyMs, row.CompletionMs, row.MaxUserPerceivedLatencyMs)
	}
	if !row.BurstDrainFasterThanSpeech || row.DrainRatioPermille != 880 {
		t.Fatalf("drain faster/ratio = %v/%d, want true/880", row.BurstDrainFasterThanSpeech, row.DrainRatioPermille)
	}
	if !row.StoreForwardRequired {
		t.Fatal("StoreForwardRequired = false, want true")
	}
	if row.MeasuredHardwareCapability {
		t.Fatal("MeasuredHardwareCapability = true, want false")
	}
}

func TestIT033_Codec2ReferenceIsNotTreatedAsFMLike(t *testing.T) {
	report, err := RunVoiceQualityEnvelopeSimulation(DefaultVoiceQualityEnvelopeConfig())
	if err != nil {
		t.Fatalf("RunVoiceQualityEnvelopeSimulation failed: %v", err)
	}
	row := mustFindVoiceQualityEnvelopeRow(t, report.Rows, "codec2_3200_reference_relays_2")
	if row.FMLikeProxy {
		t.Fatal("Codec2 reference FMLikeProxy = true, want false")
	}
	if row.Outcome != VoiceQualityOutcomeDegraded {
		t.Fatalf("Codec2 reference outcome = %q, want %q", row.Outcome, VoiceQualityOutcomeDegraded)
	}
	if !reflect.DeepEqual(row.FailReasons, []string{voiceQualityReasonNotFMLikeProxy}) {
		t.Fatalf("Codec2 reference fail reasons = %v, want [%s]", row.FailReasons, voiceQualityReasonNotFMLikeProxy)
	}
}

func TestVoiceQualityEnvelopeRejectsInvalidConfig(t *testing.T) {
	valid := func() VoiceQualityEnvelopeConfig {
		return DefaultVoiceQualityEnvelopeConfig()
	}
	tests := []struct {
		name string
		cfg  VoiceQualityEnvelopeConfig
	}{
		{name: "unsupported scenario", cfg: func() VoiceQualityEnvelopeConfig {
			cfg := valid()
			cfg.ScenarioID = "other"
			return cfg
		}()},
		{name: "duplicate relay depth", cfg: func() VoiceQualityEnvelopeConfig {
			cfg := valid()
			cfg.RelayDepthCandidates = []int{1, 1}
			return cfg
		}()},
		{name: "invalid quality target", cfg: func() VoiceQualityEnvelopeConfig {
			cfg := valid()
			cfg.Profiles[0].QualityTarget = "unknown"
			return cfg
		}()},
		{name: "fm flag mismatch", cfg: func() VoiceQualityEnvelopeConfig {
			cfg := valid()
			cfg.Profiles[0].FMLikeProxy = true
			return cfg
		}()},
		{name: "row cap", cfg: func() VoiceQualityEnvelopeConfig {
			cfg := valid()
			cfg.RelayDepthCandidates = []int{1, 2, 3, 4, 5, 6, 7, 8}
			cfg.Profiles = append(cfg.Profiles, cfg.Profiles...)
			cfg.Profiles = append(cfg.Profiles, cfg.Profiles...)
			for i := range cfg.Profiles {
				cfg.Profiles[i].ProfileID = cfg.Profiles[i].ProfileID + "_" + string(rune('a'+i))
			}
			return cfg
		}()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := RunVoiceQualityEnvelopeSimulation(tc.cfg); err == nil {
				t.Fatal("RunVoiceQualityEnvelopeSimulation accepted invalid config")
			}
		})
	}
}

func mustFindVoiceQualityEnvelopeRow(t *testing.T, rows []VoiceQualityEnvelopeRow, id string) VoiceQualityEnvelopeRow {
	t.Helper()
	for _, row := range rows {
		if row.RowID == id {
			return row
		}
	}
	t.Fatalf("missing voice quality envelope row %q", id)
	return VoiceQualityEnvelopeRow{}
}
