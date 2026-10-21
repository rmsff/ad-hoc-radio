package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rmsff/ad-hoc-radio/internal/peer"
	"github.com/rmsff/ad-hoc-radio/internal/transport"
)

type PeerAcceptanceRow struct {
	CollisionDrops      int                       `json:"collision_drops"`
	ID                  string                    `json:"id"`
	Seed                uint64                    `json:"seed"`
	Depth               uint8                     `json:"relay_depth"`
	Width               int                       `json:"branch_width"`
	GrossBPS            int64                     `json:"gross_bitrate_bps"`
	Admission           string                    `json:"admission"`
	Reason              string                    `json:"reason,omitempty"`
	ServiceOutcome      string                    `json:"service_outcome"`
	ExpectedFrames      int                       `json:"expected_member_frames"`
	PlayedFrames        int                       `json:"played_member_frames"`
	MissingFrames       int                       `json:"missing_member_frames"`
	FirstAudioFromPTTMS *int64                    `json:"first_audio_from_ptt_ms"`
	ContinuousMembers   int                       `json:"continuous_members"`
	QueueMaxDepth       int                       `json:"queue_max_depth"`
	QueueDrops          int                       `json:"queue_drops"`
	Evidence            *ScheduledRelayTreeReport `json:"evidence,omitempty"`
}

type PeerAcceptanceMatrix struct {
	StressDiagnostics []PeerAcceptanceRow `json:"stress_diagnostics"`
	Version           int                 `json:"version"`
	EvidenceGrade     string              `json:"evidence_grade"`
	Rows              []PeerAcceptanceRow `json:"rows"`
	TrialCount        int                 `json:"trial_count"`
	FailedTrials      int                 `json:"failed_trials"`
	Verdict           string              `json:"verdict"`
	Limits            []string            `json:"limits"`
	SHA256            string              `json:"sha256"`
}

func RunPeerAcceptanceMatrix() (PeerAcceptanceMatrix, error) {
	r := PeerAcceptanceMatrix{Version: 1, EvidenceGrade: "integrated_software_size_fixture", Verdict: "no_go_live_voice_hardware",
		Limits: []string{
			"1600 bps/80 ms synthetic bytes: intelligibility and codec integration unverified",
			"19200/4800 bps and 1 ms turnarounds are assumptions, not radio measurements",
			"unfragmented JSON+CRC control is charged per directed hop on a conservative serialized control bus, not an RF MAC",
			"depth is relay count; width counts independent source-rooted chains; no spatial reuse",
			"control links use the same directed topology but carry separately configured impairments",
			"20 s scheduled setup is an explicit experiment parameter, not an acceptable PTT target",
			"three deterministic seeds are regression samples, not a confidence interval for field reliability",
			"admission-rejected trials retain the entire expected-member denominator",
			"HR-C6000 L2 27-byte payload cannot carry this 36-byte version-bound voice packet; L1 remains unverified",
			"forced-overload diagnostics are separately counted, omit peer control and use legacy 30-byte packets",
			"no delayed store-forward implementation or audio-quality claim",
		}}
	appendCase := func(id string, depth uint8, width int, bps int64, seed uint64, edit func(*ScheduledRelayTreeScenarioConfig)) error {
		cfg := peerAcceptanceConfig(depth, width, bps, seed)
		if edit != nil {
			edit(&cfg)
		}
		row := PeerAcceptanceRow{ID: id, Depth: depth, Width: width, GrossBPS: bps, Seed: seed, ExpectedFrames: len(cfg.InitialPlan.Snapshot.MemberIDs) * int(cfg.FrameCount), ServiceOutcome: "fail"}
		m, err := RunScheduledRelayTreeScenario(cfg)
		if err != nil {
			if !strings.Contains(err.Error(), "no feasible") && !strings.Contains(err.Error(), "backend payload/slot does not fit") {
				return fmt.Errorf("matrix %s: unexpected runtime/input error: %w", id, err)
			}
			row.Admission = "rejected"
			row.Reason = err.Error()
			row.MissingFrames = row.ExpectedFrames
		} else {
			row.Admission = "admitted"
			row.Evidence = m.ScheduledRelayTree
			row.ServiceOutcome = m.ScheduledRelayTree.Audio.ServiceOutcome
			row.CollisionDrops = m.ScheduledRelayTree.CollisionDropCount
			row.QueueMaxDepth = m.RelayForwardQueueMaxDepth
			row.QueueDrops = m.RelayForwardQueueDrops
			for _, member := range m.ScheduledMemberDelivery {
				row.PlayedFrames += member.FramesPlayed
				row.MissingFrames += member.FramesMissing
				if member.FramesPlayed == member.FramesExpected {
					row.ContinuousMembers++
				}
			}
			if m.ScheduledRelayTree.FirstPlayoutMS != nil {
				first := *m.ScheduledRelayTree.FirstPlayoutMS - cfg.Control.StartMS
				row.FirstAudioFromPTTMS = &first
			}
		}
		r.Rows = append(r.Rows, row)
		return nil
	}
	for _, bps := range []int64{4800, 19200} {
		for depth := uint8(0); depth <= 3; depth++ {
			for _, width := range []int{1, 2, 4, 8} {
				for _, seed := range []uint64{1, 42, 77} {
					if err := appendCase(fmt.Sprintf("budget-%d-depth%d-width%d", bps, depth, width), depth, width, bps, seed, nil); err != nil {
						return r, err
					}
				}
			}
		}
	}
	negatives := []struct {
		id   string
		edit func(*ScheduledRelayTreeScenarioConfig)
	}{
		{"voice_corruption_100", func(c *ScheduledRelayTreeScenarioConfig) {
			c.VoiceLinks = []transport.Link{{From: 1, To: 2, Enabled: true, FixedDelayMs: 1, CorruptionPct: 100}}
		}},
		{"voice_duplicate_100", func(c *ScheduledRelayTreeScenarioConfig) {
			c.VoiceLinks = []transport.Link{{From: 1, To: 2, Enabled: true, FixedDelayMs: 1, DuplicatePct: 100}}
		}},
		{"voice_jitter_10ms", func(c *ScheduledRelayTreeScenarioConfig) {
			c.VoiceLinks = []transport.Link{{From: 1, To: 2, Enabled: true, FixedDelayMs: 1, JitterMs: 10}}
		}},
		{"codec_3200_reference", func(c *ScheduledRelayTreeScenarioConfig) {
			c.VoiceProfile.CodecBitrateBPS = 3200
			c.VoiceProfile.ID = "3200-reference-size-only"
		}},
		{"wideband_12000_reference", func(c *ScheduledRelayTreeScenarioConfig) {
			c.VoiceProfile.CodecBitrateBPS = 12000
			c.VoiceProfile.ID = "12000-reference-size-only"
		}},
		{"voice_loss_100", func(c *ScheduledRelayTreeScenarioConfig) {
			for i := range c.InitialPlan.Graph.Links {
				c.InitialPlan.Graph.Links[i].Loss = 1
			}
		}},
		{"voice_loss_20", func(c *ScheduledRelayTreeScenarioConfig) {
			for i := range c.InitialPlan.Graph.Links {
				c.InitialPlan.Graph.Links[i].Loss = .2
			}
		}},
		{"control_loss_100", func(c *ScheduledRelayTreeScenarioConfig) {
			for i := range c.Control.Links {
				c.Control.Links[i].LossPct = 100
			}
		}},
		{"control_reverse_missing", func(c *ScheduledRelayTreeScenarioConfig) { c.Control.Links[1].Enabled = false }},
		{"control_duplicate", func(c *ScheduledRelayTreeScenarioConfig) {
			for i := range c.Control.Links {
				c.Control.Links[i].DuplicatePct = 100
			}
		}},
		{"control_corrupt", func(c *ScheduledRelayTreeScenarioConfig) {
			for i := range c.Control.Links {
				c.Control.Links[i].CorruptionPct = 100
			}
		}},
		{"control_commit_lost", func(c *ScheduledRelayTreeScenarioConfig) {
			c.Control.Faults = []peer.ControlFault{{Kind: peer.Commit, To: 2, LossPct: 100}}
		}},
		{"relay_clock_drift", func(c *ScheduledRelayTreeScenarioConfig) {
			c.Control.Peers[1].Clock.DriftPPM = 1000
			c.Control.Peers[1].Clock.DriftBoundPPM = 1000
		}},
		{"hr_c6000_l2_27_byte_limit", func(c *ScheduledRelayTreeScenarioConfig) { c.VoiceProfile.MaxPayloadBytes = 27 }},
	}
	for _, tc := range negatives {
		for _, seed := range []uint64{1, 42, 77} {
			if err := appendCase(tc.id, 1, 1, 19200, seed, tc.edit); err != nil {
				return r, err
			}
		}
	}
	for _, candidate := range []struct {
		id     string
		depth  uint8
		period int64
	}{{"candidate-1600-100ms", 2, 100}, {"candidate-1600-160ms", 3, 160}} {
		for _, seed := range []uint64{1, 42, 77} {
			if err := appendCase(candidate.id, candidate.depth, 1, 19200, seed, func(c *ScheduledRelayTreeScenarioConfig) {
				c.VoiceProfile.PacketPeriodMS = candidate.period
				c.VoiceProfile.ID = candidate.id
			}); err != nil {
				return r, err
			}
		}
	}
	for _, shape := range []struct {
		depth uint8
		width int
	}{{3, 1}, {1, 4}} {
		for _, seed := range []uint64{1, 42, 77} {
			row, err := runPeerAcceptanceStress(shape.depth, shape.width, seed)
			if err != nil {
				return r, err
			}
			r.StressDiagnostics = append(r.StressDiagnostics, row)
		}
	}
	r.TrialCount = len(r.Rows)
	for _, row := range r.Rows {
		if row.ServiceOutcome != "pass" {
			r.FailedTrials++
		}
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return r, err
	}
	sum := sha256.Sum256(raw)
	r.SHA256 = hex.EncodeToString(sum[:])
	return r, nil
}
