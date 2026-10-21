package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rmsff/ad-hoc-radio/internal/peer"
	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

type ControlComparisonRow struct {
	Case                  string               `json:"case"`
	Format                peer.WireFormat      `json:"wire_format"`
	Depth                 uint8                `json:"relay_depth"`
	Width                 int                  `json:"branch_width"`
	GrossBPS              int64                `json:"gross_bitrate_bps"`
	CadenceMS             int64                `json:"voice_cadence_ms"`
	SetupAllowanceMS      int64                `json:"setup_allowance_ms"`
	Admission             string               `json:"admission"`
	Reason                string               `json:"reason,omitempty"`
	ExpectedFrames        int                  `json:"expected_member_frames"`
	PlayedFrames          int                  `json:"played_member_frames"`
	ServiceOutcome        string               `json:"service_outcome"`
	FirstPlayoutFromPTTMS *int64               `json:"first_playout_from_ptt_ms"`
	AllLocalCommitsMS     *int64               `json:"all_local_commits_ms"`
	SetupControl          *peer.Counters       `json:"setup_control,omitempty"`
	LocalCommits          []peer.CommitReceipt `json:"local_commits"`
	RuntimeControl        *peer.Counters       `json:"runtime_control,omitempty"`
}

type ControlComparison struct {
	Version       int                    `json:"version"`
	EvidenceGrade string                 `json:"evidence_grade"`
	Verdict       string                 `json:"verdict"`
	Rows          []ControlComparisonRow `json:"rows"`
	Limits        []string               `json:"limits"`
	SHA256        string                 `json:"sha256"`
}

func RunControlComparison() (ControlComparison, error) {
	r := ControlComparison{Version: 1, EvidenceGrade: "integrated_control_size_fixture", Verdict: "no_go_live_voice_hardware", Limits: []string{
		"Fixed seed 42; identical directed links, retry policy, FEC and framing for each encoding pair",
		"19200/4800 bps and 1 ms component guards are assumptions, not radio measurements",
		"all_local_commits_ms observes receipt at every participant; the source has no oracle for it",
		"setup_allowance_ms is configured, not discovered or negotiated; first playout retains that allowance",
		"setup_control stops at activation; runtime_control includes later control and End",
		"100/160/200 ms cadence rows are explicit size-only tradeoffs, not an accepted codec packetization",
		"No fragmentation, broadcast proposal optimization, discovery MAC, real codec playout or RF readiness",
		"64-byte compact references and 36-byte voice packets still exceed the 27-byte L2 boundary",
	}}
	appendCase := func(name string, depth uint8, width int, bps, cadence, setup int64, edit func(*ScheduledRelayTreeScenarioConfig)) error {
		for _, format := range []peer.WireFormat{peer.JSONCRCv0, peer.CompactV1} {
			cfg := peerAcceptanceConfig(depth, width, bps, 42)
			cfg.StartMS = setup
			cfg.VoiceProfile.PacketPeriodMS = cadence
			cfg.Control.WireFormat = format
			if edit != nil {
				edit(&cfg)
			}
			row, err := compareControlCase(name, cfg, depth, width)
			if err != nil {
				return err
			}
			r.Rows = append(r.Rows, row)
		}
		return nil
	}
	for _, bps := range []int64{4800, 19200} {
		for _, setup := range []int64{250, 500, 1000, 2000, 5000, 20000} {
			for depth := uint8(0); depth <= 3; depth++ {
				if err := appendCase("chain", depth, 1, bps, 80, setup, nil); err != nil {
					return r, err
				}
			}
			for _, width := range []int{2, 4} {
				if err := appendCase("branches", 1, width, bps, 80, setup, nil); err != nil {
					return r, err
				}
			}
		}
	}
	for _, shape := range []struct {
		depth   uint8
		width   int
		cadence int64
	}{{2, 1, 100}, {3, 1, 160}, {1, 2, 160}, {1, 4, 200}} {
		for _, setup := range []int64{1000, 2000, 5000, 20000} {
			if err := appendCase("cadence_comparison", shape.depth, shape.width, 19200, shape.cadence, setup, nil); err != nil {
				return r, err
			}
		}
	}
	for _, fault := range []string{"proposal_lost", "ack_lost", "commit_lost", "late_commit", "duplicates", "corruption", "l2_limit"} {
		err := appendCase(fault, 1, 1, 19200, 80, 2000, func(c *ScheduledRelayTreeScenarioConfig) {
			switch fault {
			case "proposal_lost":
				c.Control.Faults = []peer.ControlFault{{Kind: peer.Proposal, To: 2, LossPct: 100}}
			case "ack_lost":
				c.Control.Faults = []peer.ControlFault{{Kind: peer.ACK, From: 2, LossPct: 100}}
			case "commit_lost":
				c.Control.Faults = []peer.ControlFault{{Kind: peer.Commit, To: 2, LossPct: 100}}
			case "late_commit":
				c.Control.Faults = []peer.ControlFault{{Kind: peer.Commit, To: 2, DelayMS: 3000}}
			case "duplicates":
				for i := range c.Control.Links {
					c.Control.Links[i].DuplicatePct = 100
				}
			case "corruption":
				for i := range c.Control.Links {
					c.Control.Links[i].CorruptionPct = 100
				}
			case "l2_limit":
				c.Control.Physical.MaxPayloadBytes = 27
			}
		})
		if err != nil {
			return r, err
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

func compareControlCase(name string, cfg ScheduledRelayTreeScenarioConfig, depth uint8, width int) (ControlComparisonRow, error) {
	r := ControlComparisonRow{Case: name, Format: cfg.Control.WireFormat, Depth: depth, Width: width, GrossBPS: cfg.VoiceProfile.GrossBitrateBPS, CadenceMS: cfg.VoiceProfile.PacketPeriodMS, SetupAllowanceMS: cfg.StartMS, ServiceOutcome: "fail", ExpectedFrames: len(cfg.InitialPlan.Snapshot.MemberIDs) * int(cfg.FrameCount), LocalCommits: []peer.CommitReceipt{}}
	m, err := RunScheduledRelayTreeScenario(cfg)
	if err != nil {
		if !strings.Contains(err.Error(), "no feasible") && !strings.Contains(err.Error(), "backend payload/slot does not fit") {
			return r, fmt.Errorf("control comparison %s: %w", name, err)
		}
		r.Admission = "rejected"
		r.Reason = err.Error()
		return r, nil
	}
	r.Admission = "admitted"
	r.ServiceOutcome = m.ScheduledRelayTree.Audio.ServiceOutcome
	r.RuntimeControl = m.ScheduledRelayTree.Control
	if m.ScheduledRelayTree.FirstPlayoutMS != nil {
		v := *m.ScheduledRelayTree.FirstPlayoutMS - cfg.Control.StartMS
		r.FirstPlayoutFromPTTMS = &v
	}
	for _, member := range m.ScheduledRelayTree.Audio.Members {
		r.PlayedFrames += member.FramesPlayed
	}
	// Reuse the scenario's exact schedule construction. This second execution
	// isolates setup through activation, before voice and End affect counters.
	profile, err := scheduledVoiceOpportunityProfileWithControl(*cfg.VoiceProfile, true)
	if err != nil {
		return r, err
	}
	cfg.InitialPlan.CostModel = routing.PlanningCostModel{SourceOpportunityMS: int64(profile.SourceDurationMS), RelayOpportunityMS: int64(profile.RelayDurationMS), GuardAfterMS: int64(profile.GuardAfterMS), FramePeriodMS: cfg.VoiceProfile.PacketPeriodMS}
	plan, err := routing.PlanRelayTree(scheduledRelayTreePlanRequest(cfg.SourceStationID, cfg.InitialPlan))
	if err != nil {
		return r, err
	}
	s, err := timing.BuildOpportunitySchedule(scheduledRelayTreeBinding(cfg, cfg.InitialPlan, 1, 0), plan.Selected.Tree, profile)
	if err != nil {
		return r, err
	}
	q := sim.NewEventQueue()
	c, err := peer.New(*cfg.Control, q)
	if err != nil {
		return r, err
	}
	if err = c.Begin(s, cfg.StartMS, cfg.VoiceProfile.PacketPeriodMS); err != nil {
		return r, err
	}
	for {
		e, ok := q.Peek()
		if !ok || e.TimeMs >= cfg.StartMS {
			break
		}
		q.Pop()
		if _, err = c.Process(e); err != nil {
			return r, err
		}
	}
	stats := c.Counters()
	r.SetupControl = &stats
	r.LocalCommits = append(r.LocalCommits, c.CommitReceipts()...)
	if len(r.LocalCommits) == len(cfg.Control.Peers) {
		var last int64
		for _, receipt := range r.LocalCommits {
			if receipt.ReceivedAtMS > last {
				last = receipt.ReceivedAtMS
			}
		}
		last -= cfg.Control.StartMS
		r.AllLocalCommitsMS = &last
	}
	return r, nil
}
