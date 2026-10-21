package scenario

import (
	"reflect"
	"strings"
	"testing"
)

func TestIT031_TYTProxyTurnaroundDefaultReportDeterministic(t *testing.T) {
	first, err := RunTYTProxyTurnaroundSimulation(DefaultTYTProxyTurnaroundConfig())
	if err != nil {
		t.Fatalf("RunTYTProxyTurnaroundSimulation failed: %v", err)
	}
	second, err := RunTYTProxyTurnaroundSimulation(DefaultTYTProxyTurnaroundConfig())
	if err != nil {
		t.Fatalf("second RunTYTProxyTurnaroundSimulation failed: %v", err)
	}

	if first.ScenarioID != TYTProxyTurnaroundScenarioID {
		t.Fatalf("ScenarioID = %q, want %q", first.ScenarioID, TYTProxyTurnaroundScenarioID)
	}
	if !first.AssumptionOnly || first.MeasuredHardwareCapability || first.HardwareClaimAllowed {
		t.Fatalf("claim gates = assumption:%v measured:%v allowed:%v, want true/false/false",
			first.AssumptionOnly, first.MeasuredHardwareCapability, first.HardwareClaimAllowed)
	}
	if first.RowCount != 10 || len(first.Rows) != 10 {
		t.Fatalf("row count = %d/%d, want 10", first.RowCount, len(first.Rows))
	}
	if first.PublicProxyRows != 9 || first.ViableRows != 1 || first.NoLiveTwoHopRows != 9 {
		t.Fatalf("counts public/viable/no-live = %d/%d/%d, want 9/1/9",
			first.PublicProxyRows, first.ViableRows, first.NoLiveTwoHopRows)
	}
	if first.EventHash == "" || first.EventHash != second.EventHash {
		t.Fatalf("event hash mismatch: %q vs %q", first.EventHash, second.EventHash)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("same config produced different TYT proxy turnaround reports")
	}

	primary := mustFindTYTProxyRow(t, first.Rows, "project_primary_threshold_10ms")
	if primary.CalibrationDecision != TYTBenchDecisionPrimary24ms || !primary.LiveTwoHopTupleViable {
		t.Fatalf("primary threshold decision = %q viable=%v, want %q/true",
			primary.CalibrationDecision, primary.LiveTwoHopTupleViable, TYTBenchDecisionPrimary24ms)
	}

	for _, id := range []string{
		"alinco_dj_md5tgp_dmr_handheld_qst",
		"anytone_at_d868uv_dmr_handheld_qst",
		"anytone_at_d878uv_dmr_handheld_qst",
		"anytone_at_d878uvii_plus_dmr_handheld_qst",
		"bridgecom_bch220_handheld_audio_turnaround",
		"etsi_en300113_lmr_attack_release_limit",
		"lmd401_module_max_switching_time",
		"p25_c4fm_voice_standard_limit",
		"tyt_th_uvf1_analog_handheld_qst",
	} {
		row := mustFindTYTProxyRow(t, first.Rows, id)
		if row.HardwareClaimAllowed || row.MeasuredTYTHardwareCapability {
			t.Fatalf("%s claim gates = allowed:%v measured:%v, want false/false",
				id, row.HardwareClaimAllowed, row.MeasuredTYTHardwareCapability)
		}
		if row.CalibrationDecision != TYTBenchDecisionNoLiveTwoHop || row.LiveTwoHopTupleViable {
			t.Fatalf("%s decision = %q viable=%v, want no_live_two_hop_tuple/false",
				id, row.CalibrationDecision, row.LiveTwoHopTupleViable)
		}
	}

	bridge := mustFindTYTProxyRow(t, first.Rows, "bridgecom_bch220_handheld_audio_turnaround")
	if bridge.MaxRequiredTurnaroundP95Ms != 196 {
		t.Fatalf("BridgeCom max p95 = %d, want 196", bridge.MaxRequiredTurnaroundP95Ms)
	}
	if !reflect.DeepEqual(bridge.LimitingComponents, []string{tytBenchMetricRxValidAfterTx, tytBenchMetricTxToRx}) {
		t.Fatalf("BridgeCom limiters = %v, want [rx_valid_after_tx tx_to_rx]", bridge.LimitingComponents)
	}

	anytonePlus := mustFindTYTProxyRow(t, first.Rows, "anytone_at_d878uvii_plus_dmr_handheld_qst")
	if anytonePlus.MaxRequiredTurnaroundP95Ms != 420 {
		t.Fatalf("AnyTone AT-D878UVII Plus max p95 = %d, want 420", anytonePlus.MaxRequiredTurnaroundP95Ms)
	}
	if anytonePlus.EvidenceGrade != TYTProxyEvidenceDMRHandheldLabReview {
		t.Fatalf("AnyTone AT-D878UVII Plus evidence grade = %q, want %q",
			anytonePlus.EvidenceGrade, TYTProxyEvidenceDMRHandheldLabReview)
	}
}

func TestIT031_TYTProxyTurnaroundUsesBenchCalibrationThresholds(t *testing.T) {
	cfg := TYTProxyTurnaroundConfig{
		ScenarioID: TYTProxyTurnaroundScenarioID,
		Seed:       DefaultScenarioSeed,
		Profiles: []TYTProxyTurnaroundProfile{
			testTYTProxyProfile("threshold_16", 16),
			testTYTProxyProfile("threshold_17", 17),
		},
	}
	report, err := RunTYTProxyTurnaroundSimulation(cfg)
	if err != nil {
		t.Fatalf("RunTYTProxyTurnaroundSimulation failed: %v", err)
	}
	safe := mustFindTYTProxyRow(t, report.Rows, "threshold_16")
	if safe.CalibrationDecision != TYTBenchDecisionSafe30ms || safe.SelectedTupleSlotMs != 30 || !safe.LiveTwoHopTupleViable {
		t.Fatalf("16ms row decision/slot/viable = %q/%d/%v, want %q/30/true",
			safe.CalibrationDecision, safe.SelectedTupleSlotMs, safe.LiveTwoHopTupleViable, TYTBenchDecisionSafe30ms)
	}
	noLive := mustFindTYTProxyRow(t, report.Rows, "threshold_17")
	if noLive.CalibrationDecision != TYTBenchDecisionNoLiveTwoHop || noLive.SelectedTupleSlotMs != 0 || noLive.LiveTwoHopTupleViable {
		t.Fatalf("17ms row decision/slot/viable = %q/%d/%v, want %q/0/false",
			noLive.CalibrationDecision, noLive.SelectedTupleSlotMs, noLive.LiveTwoHopTupleViable, TYTBenchDecisionNoLiveTwoHop)
	}
}

func TestTYTProxyTurnaroundRejectsInvalidProfiles(t *testing.T) {
	valid := testTYTProxyProfile("valid", 10)
	tests := []struct {
		name string
		cfg  TYTProxyTurnaroundConfig
		want string
	}{
		{name: "unsupported scenario", cfg: func() TYTProxyTurnaroundConfig {
			cfg := TYTProxyTurnaroundConfig{ScenarioID: "other", Seed: DefaultScenarioSeed, Profiles: []TYTProxyTurnaroundProfile{valid}}
			return cfg
		}(), want: "scenario_id"},
		{name: "empty profiles", cfg: TYTProxyTurnaroundConfig{
			ScenarioID: TYTProxyTurnaroundScenarioID,
			Seed:       DefaultScenarioSeed,
			Profiles:   []TYTProxyTurnaroundProfile{},
		}, want: "at least one"},
		{name: "duplicate profile", cfg: TYTProxyTurnaroundConfig{
			ScenarioID: TYTProxyTurnaroundScenarioID,
			Seed:       DefaultScenarioSeed,
			Profiles:   []TYTProxyTurnaroundProfile{valid, valid},
		}, want: "duplicate"},
		{name: "duplicate profile after trim", cfg: TYTProxyTurnaroundConfig{
			ScenarioID: TYTProxyTurnaroundScenarioID,
			Seed:       DefaultScenarioSeed,
			Profiles: []TYTProxyTurnaroundProfile{
				valid,
				func() TYTProxyTurnaroundProfile {
					profile := valid
					profile.ProfileID = " valid "
					return profile
				}(),
			},
		}, want: "duplicate"},
		{name: "unsupported evidence", cfg: func() TYTProxyTurnaroundConfig {
			profile := valid
			profile.EvidenceGrade = "forum_guess"
			return TYTProxyTurnaroundConfig{ScenarioID: TYTProxyTurnaroundScenarioID, Seed: DefaultScenarioSeed, Profiles: []TYTProxyTurnaroundProfile{profile}}
		}(), want: "evidence_grade"},
		{name: "public source needs URL", cfg: func() TYTProxyTurnaroundConfig {
			profile := valid
			profile.SourceURL = ""
			return TYTProxyTurnaroundConfig{ScenarioID: TYTProxyTurnaroundScenarioID, Seed: DefaultScenarioSeed, Profiles: []TYTProxyTurnaroundProfile{profile}}
		}(), want: "source_url"},
		{name: "public source rejects whitespace URL", cfg: func() TYTProxyTurnaroundConfig {
			profile := valid
			profile.SourceURL = "   "
			return TYTProxyTurnaroundConfig{ScenarioID: TYTProxyTurnaroundScenarioID, Seed: DefaultScenarioSeed, Profiles: []TYTProxyTurnaroundProfile{profile}}
		}(), want: "source_url"},
		{name: "public source needs absolute HTTP URL", cfg: func() TYTProxyTurnaroundConfig {
			profile := valid
			profile.SourceURL = "docs/research/source.pdf"
			return TYTProxyTurnaroundConfig{ScenarioID: TYTProxyTurnaroundScenarioID, Seed: DefaultScenarioSeed, Profiles: []TYTProxyTurnaroundProfile{profile}}
		}(), want: "absolute URL"},
		{name: "p50 above p95", cfg: func() TYTProxyTurnaroundConfig {
			profile := valid
			profile.TxToRxP50Ms = 11
			profile.TxToRxP95Ms = 10
			return TYTProxyTurnaroundConfig{ScenarioID: TYTProxyTurnaroundScenarioID, Seed: DefaultScenarioSeed, Profiles: []TYTProxyTurnaroundProfile{profile}}
		}(), want: "p50_ms"},
		{name: "unsupported envelope", cfg: func() TYTProxyTurnaroundConfig {
			profile := valid
			profile.GrossBitrateBps = 38400
			return TYTProxyTurnaroundConfig{ScenarioID: TYTProxyTurnaroundScenarioID, Seed: DefaultScenarioSeed, Profiles: []TYTProxyTurnaroundProfile{profile}}
		}(), want: "gross_bitrate_bps"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := RunTYTProxyTurnaroundSimulation(tc.cfg)
			if err == nil {
				t.Fatal("RunTYTProxyTurnaroundSimulation accepted invalid config")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want context %q", err.Error(), tc.want)
			}
		})
	}
}

func testTYTProxyProfile(id string, p95 int64) TYTProxyTurnaroundProfile {
	return TYTProxyTurnaroundProfile{
		ProfileID:           id,
		EvidenceGrade:       TYTProxyEvidenceStandardLimit,
		SourceName:          "test standard",
		SourceURL:           "https://example.test/standard.pdf",
		SourceMetric:        "test p95",
		ChannelWidthHz:      tytLikelyChannelWidthHz,
		GrossBitrateBps:     19200,
		PreambleMs:          1,
		SyncMs:              1,
		LinkDelayMs:         1,
		RxToTxP50Ms:         p95,
		RxToTxP95Ms:         p95,
		TxToRxP50Ms:         p95,
		TxToRxP95Ms:         p95,
		RxValidAfterTxP50Ms: p95,
		RxValidAfterTxP95Ms: p95,
		ApplicabilityToTYT:  "test only",
		Interpretation:      "test only",
		Notes:               "test only",
	}
}

func mustFindTYTProxyRow(t *testing.T, rows []TYTProxyTurnaroundRow, id string) TYTProxyTurnaroundRow {
	t.Helper()
	for _, row := range rows {
		if row.ProfileID == id {
			return row
		}
	}
	t.Fatalf("profile_id %q not found in rows", id)
	return TYTProxyTurnaroundRow{}
}
