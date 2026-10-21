package scenario

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/rmsff/ad-hoc-radio/internal/peer"
	"github.com/rmsff/ad-hoc-radio/internal/protocol"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

type OpusPayloadStats struct {
	Count         int     `json:"count"`
	MinBytes      int     `json:"min_bytes"`
	P95Bytes      int     `json:"p95_bytes"`
	MaxBytes      int     `json:"max_bytes"`
	MeanBytes     float64 `json:"mean_bytes"`
	TotalBytes    int     `json:"total_bytes"`
	WireMinBytes  int     `json:"wire_min_bytes"`
	WireMaxBytes  int     `json:"wire_max_bytes"`
	ProtectedBits int64   `json:"total_protected_bits"`
}

type OpusBudgetRow struct {
	SampleID            string                   `json:"sample_id"`
	PacketCount         int                      `json:"original_packet_count"`
	GroupCount          int                      `json:"group_count"`
	ModelDurationMS     int64                    `json:"model_duration_ms"`
	SourceSamples       int                      `json:"source_samples_8khz"`
	ExtraBatchWaitMS    int64                    `json:"extra_batch_wait_ms"`
	RateClass           string                   `json:"rate_class"`
	Payload             OpusPayloadStats         `json:"payload"`
	PeakBudget          timing.WireBudget        `json:"peak_budget"`
	Opportunities       int                      `json:"serialized_opportunities"`
	ReservedCycleMS     int64                    `json:"reserved_cycle_ms"`
	PeakUtilizationPct  float64                  `json:"peak_cycle_utilization_pct"`
	SerializationMinBPS int64                    `json:"serialization_only_min_bps"`
	PeakReservationBPS  *int64                   `json:"peak_reservation_min_bps"`
	CaptureAllowanceUS  int64                    `json:"oldest_sample_capture_allowance_us"`
	CreationToPlayoutMS *int64                   `json:"first_creation_to_playout_ms"`
	AllMemberPlayout    *RelayDepthLatencyReport `json:"played_member_playout_latency_ms"`
	LatencyComplete     bool                     `json:"latency_covers_all_expected_frames"`
	CapturePlusP95US    *float64                 `json:"capture_plus_model_playout_p95_estimate_us"`
	CaptureToPlayoutUS  *int64                   `json:"capture_plus_first_playout_estimate_us"`
	SerialPTTEstimateUS *int64                   `json:"serial_setup_capture_first_playout_estimate_us"`
	Trial               ControlComparisonRow     `json:"trial"`
}

type OpusBudgetReport struct {
	Version              int             `json:"version"`
	InputSHA256          string          `json:"input_sha256"`
	SourceManifestSHA256 string          `json:"source_manifest_sha256"`
	EvidenceGrade        string          `json:"evidence_grade"`
	Verdict              string          `json:"verdict"`
	Rows                 []OpusBudgetRow `json:"rows"`
	Limits               []string        `json:"limits"`
	SHA256               string          `json:"sha256"`
}

func RunOpusBudget(raw []byte) (OpusBudgetReport, error) {
	t, err := DecodeOpusTrace(raw)
	if err != nil {
		return OpusBudgetReport{}, err
	}
	r := OpusBudgetReport{Version: 1, InputSHA256: opusTraceHash(raw), SourceManifestSHA256: t.SourceManifestSHA256,
		EvidenceGrade: "real_packet_sizes_conservative_synthetic_runtime_envelope", Verdict: "no_go_live_voice_hardware",
		Limits: []string{
			"No codec, waveform, firmware or physical-radio compatibility is selected",
			"4800/19200 are existing assumptions; 38400/57600/76800/115200 are unsupported sensitivity hypotheses",
			"Opus target is 8000 CVBR, but runtime sends synthetic maximum-sized envelopes, NOT Opus audio",
			"Groups 2/4/8 add hypothetical uint16 lengths; format/quality/PLC under grouped losses are not accepted",
			"Whole trace including flush tail is counted; final partial group occupies a complete configured period",
			"2/3 external protection, 48 framing bits and four 1ms guard components are unchanged assumptions",
			"One transmitter opportunity per source/selected relay; no spatial reuse or free branch forwarding",
			"Configured setup500/2000ms is not observed Commit latency; runtime includes later control/End",
			"No discovery MAC, RF bit errors, clock drift, MCU codec cost or variable-payload audio playout measured",
			"Capture estimates add packet duration and 6.5ms lookahead; serial PTT estimate assumes capture after setup",
			"Latency estimates are not measured microphone-to-speaker timing or a p95 user-quality verdict",
			"Default planner160ms budget and receiver playout policy retained; failed rows remain in report",
			"Maximum means observed trace maximum, not an enforced size ceiling for future Opus streams",
			"Latency samples cover played frames only; failed recipients stay in expected-frame counters",
			"Peak reservation bitrate is an arithmetic necessary bound, not sufficient setup/link/backend feasibility",
		}}
	for _, s := range t.Samples {
		for _, group := range []int{1, 2, 4, 8} {
			for _, rate := range []int64{4800, 19200, 38400, 57600, 76800, 115200} {
				for _, shape := range [][2]int{{0, 1}, {1, 1}, {2, 1}, {3, 1}, {1, 2}, {1, 4}} {
					for _, setup := range []int64{500, 2000} {
						row, err := runOpusBudgetCase(t, s, group, uint8(shape[0]), shape[1], rate, setup, false)
						if err != nil {
							return r, err
						}
						r.Rows = append(r.Rows, row)
					}
				}
			}
		}
		row, err := runOpusBudgetCase(t, s, 1, 0, 1, 76800, 2000, true)
		if err != nil {
			return r, err
		}
		r.Rows = append(r.Rows, row)
	}
	encoded, err := json.Marshal(r)
	if err != nil {
		return r, err
	}
	r.SHA256 = opusTraceHash(encoded)
	return r, nil
}

func opusPayloadStats(sizes []int, p timing.VoiceBudgetProfile) (OpusPayloadStats, timing.WireBudget, error) {
	ordered := append([]int(nil), sizes...)
	sort.Ints(ordered)
	out := OpusPayloadStats{Count: len(sizes), MinBytes: ordered[0], MaxBytes: ordered[len(ordered)-1], P95Bytes: ordered[(95*len(ordered)+99)/100-1]}
	var peak timing.WireBudget
	for _, n := range sizes {
		size, err := protocol.ShortVoiceWireSize(n)
		if err != nil {
			return out, peak, err
		}
		b, err := p.Measure(make([]byte, size+peer.VoiceBindingOverhead))
		if err != nil {
			return out, peak, err
		}
		out.TotalBytes += n
		out.ProtectedBits += b.ProtectedBits
		if b.WireBytes > peak.WireBytes {
			peak = b
		}
		if out.WireMinBytes == 0 || b.WireBytes < out.WireMinBytes {
			out.WireMinBytes = b.WireBytes
		}
	}
	out.MeanBytes, out.WireMaxBytes = float64(out.TotalBytes)/float64(len(sizes)), peak.WireBytes
	return out, peak, nil
}

func runOpusBudgetCase(t OpusTrace, s OpusTraceSample, group int, depth uint8, width int, rate, setup int64, l2 bool) (OpusBudgetRow, error) {
	return runPacketEnvelopeCase(t, s, group, depth, width, rate, setup, l2, "opus-real-trace-maximum-size-envelope-not-codec")
}

// Packet costs are codec-neutral; the caller validates provenance and delay semantics.
func runPacketEnvelopeCase(t OpusTrace, s OpusTraceSample, group int, depth uint8, width int, rate, setup int64, l2 bool, profileID string) (OpusBudgetRow, error) {
	if depth > 3 || width < 1 || width > 4 {
		return OpusBudgetRow{}, fmt.Errorf("opus budget: shape outside experiment")
	}
	sizes, err := opusBundlePayloads(s, group)
	if err != nil || len(sizes) == 0 {
		return OpusBudgetRow{}, fmt.Errorf("opus budget: empty/invalid grouping")
	}
	period := int64(group) * t.PacketMS
	maximum := 0
	for _, n := range sizes {
		if n > maximum {
			maximum = n
		}
	}
	cfg := peerAcceptanceConfig(depth, width, rate, 42)
	cfg.StartMS, cfg.FrameCount = setup, uint32(len(sizes))
	cfg.Control.WireFormat = peer.CompactV1
	cfg.VoiceProfile.ID = profileID
	cfg.VoiceProfile.CodecBitrateBPS = int64(maximum) * 8000 / period
	cfg.VoiceProfile.PacketPeriodMS = period
	name := "chain"
	if width > 1 {
		name = "branches"
	}
	if l2 {
		name = "l2_27byte_limit"
		cfg.VoiceProfile.MaxPayloadBytes, cfg.Control.Physical.MaxPayloadBytes = 27, 27
	}
	stats, peak, err := opusPayloadStats(sizes, *cfg.VoiceProfile)
	if err != nil {
		return OpusBudgetRow{}, err
	}
	opportunities := 1 + int(depth)*width
	cycle := int64(opportunities) * int64(peak.DurationMS+peak.GuardMS)
	row := OpusBudgetRow{SampleID: s.ID, PacketCount: len(s.PacketsHex), GroupCount: group, ModelDurationMS: int64(len(sizes)) * period,
		SourceSamples: s.InputSamples, ExtraBatchWaitMS: period - t.PacketMS, RateClass: "existing_assumption",
		Payload: stats, PeakBudget: peak, Opportunities: opportunities, ReservedCycleMS: cycle,
		PeakUtilizationPct: float64(cycle) * 100 / float64(period), CaptureAllowanceUS: period*1000 + t.LookaheadUS,
		SerializationMinBPS: opusCeil(stats.ProtectedBits*int64(opportunities)*1000, int64(len(sizes))*period)}
	if rate > 19200 {
		row.RateClass = "unverified_higher_rate_hypothesis"
	}
	available := period/int64(opportunities) - int64(peak.GuardMS)
	if available > 0 {
		v := opusCeil(peak.ProtectedBits*1000, available)
		row.PeakReservationBPS = &v
	}
	row.Trial, err = compareControlCase(name, cfg, depth, width)
	if err != nil {
		return row, err
	}
	if row.Trial.Admission == "admitted" {
		metrics, err := RunScheduledRelayTreeScenario(cfg)
		if err != nil {
			return row, err
		}
		row.AllMemberPlayout = metrics.ScheduledRelayTree.Audio.PlayoutLatency
		row.LatencyComplete = row.Trial.PlayedFrames == row.Trial.ExpectedFrames && row.Trial.ExpectedFrames > 0
		if row.AllMemberPlayout != nil {
			v := row.AllMemberPlayout.P95*1000 + float64(row.CaptureAllowanceUS)
			row.CapturePlusP95US = &v
		}
	}
	if row.Trial.FirstPlayoutFromPTTMS != nil {
		creation := *row.Trial.FirstPlayoutFromPTTMS - setup
		row.CreationToPlayoutMS = &creation
		capture := creation*1000 + row.CaptureAllowanceUS
		row.CaptureToPlayoutUS = &capture
		serial := *row.Trial.FirstPlayoutFromPTTMS*1000 + row.CaptureAllowanceUS
		row.SerialPTTEstimateUS = &serial
	}
	return row, nil
}

func opusCeil(n, d int64) int64 {
	return (n + d - 1) / d
}
