package scenario

import (
	"github.com/rmsff/ad-hoc-radio/internal/channel"
	"github.com/rmsff/ad-hoc-radio/internal/election"
)

const PeriodicBeaconNeighborScenarioID = "periodic_beacon_neighbor_table"

const (
	BeaconVoicePolicyDeferAfterVoice = "defer_after_voice"
)

const (
	PeriodicBeaconEventBeaconTx   = "beacon_tx"
	PeriodicBeaconEventVoiceStart = "voice_start"
	PeriodicBeaconEventVoiceEnd   = "voice_end"
)

const (
	PeriodicBeaconNeighborFresh     = "fresh"
	PeriodicBeaconNeighborStale     = "stale"
	PeriodicBeaconNeighborDead      = "dead"
	PeriodicBeaconNeighborNeverSeen = "never_seen"
)

const (
	PeriodicBeaconOutcomeRelayed          = "relayed"
	PeriodicBeaconOutcomeNoFreshCandidate = "no_fresh_candidate"
)

const (
	defaultPeriodicBeaconIntervalMs      int64 = 60_000
	defaultPeriodicBeaconStaleMs         int64 = 180_000
	defaultPeriodicBeaconDeadMs          int64 = 300_000
	defaultPeriodicBeaconFrameBytes            = 32
	defaultPeriodicBeaconSimulationMs    int64 = 180_000
	defaultPeriodicBeaconVoiceStartMs    int64 = 120_000
	defaultPeriodicBeaconVoiceDurationMs int64 = 3_200
)

type PeriodicBeaconNeighborScenarioConfig struct {
	ScenarioID        string
	Seed              uint64
	SourceID          uint16
	Policy            election.Policy
	Nodes             []PeriodicBeaconNode
	BeaconIntervalMs  int64
	BeaconFrameBytes  int
	ChannelBitrateBps int64
	BeaconGuardMs     int64
	NeighborStaleMs   int64
	NeighborDeadMs    int64
	SimulationEndMs   int64
	VoicePolicy       string
	VoiceWindows      []PeriodicBeaconVoiceWindow
}

type PeriodicBeaconNode struct {
	NodeID            uint16
	BeaconOffsetMs    int64
	Disabled          bool
	DisabledAtMs      int64
	RelayCapable      bool
	RelayEnabled      bool
	Busy              bool
	QualityFromSource float64
	GroupCoverage     float64
	Battery           float64
	Stability         float64
}

type PeriodicBeaconVoiceWindow struct {
	StartMs    int64
	DurationMs int64
}

type PeriodicBeaconNeighborReport struct {
	ScenarioID                 string
	Seed                       uint64
	SourceID                   uint16
	BeaconIntervalMs           int64
	BeaconFrameBytes           int
	BeaconAirtimeMs            int64
	ChannelBitrateBps          int64
	BeaconGuardMs              int64
	NeighborStaleMs            int64
	NeighborDeadMs             int64
	SimulationEndMs            int64
	VoicePolicy                string
	VoiceWindows               []PeriodicBeaconVoiceWindow
	Events                     []PeriodicBeaconEvent
	NeighborRows               []PeriodicBeaconNeighborRow
	CallStartMs                int64
	TableConvergedAtMs         int64
	BeaconDueCount             int
	BeaconTxCount              int
	DeferredBeaconCount        int
	BeaconDuringVoiceCount     int
	TotalBeaconAirtimeMs       int64
	ControlChannelOccupancyPct float64
	FreshNeighborCount         int
	StaleNeighborCount         int
	DeadNeighborCount          int
	NeverSeenNeighborCount     int
	FreshRelayCandidateCount   int
	SelectedRelayID            uint16
	SelectedRelayScore         float64
	Outcome                    string
	EventHash                  string
}

type PeriodicBeaconEvent struct {
	Index          int
	TimeMs         int64
	Type           string
	NodeID         uint16
	OriginalDueMs  int64
	TxDurationMs   int64
	RxCompleteMs   int64
	Deferred       bool
	DeferredReason string
}

type PeriodicBeaconNeighborRow struct {
	NodeID            uint16
	LastSeenMs        int64
	AgeAtCallMs       int64
	Status            string
	RelayCapable      bool
	RelayEnabled      bool
	QualityFromSource float64
	GroupCoverage     float64
	Battery           float64
	Stability         float64
	EligibleForRelay  bool
}

func DefaultPeriodicBeaconNeighborScenarioConfig() PeriodicBeaconNeighborScenarioConfig {
	intervalMs := defaultPeriodicBeaconIntervalMs
	nodeIDs := []uint16{2, 3, 4, 5, 6, 7, 8, 9, 10}
	nodes := make([]PeriodicBeaconNode, 0, len(nodeIDs))
	for i, nodeID := range nodeIDs {
		node := PeriodicBeaconNode{
			NodeID:         nodeID,
			BeaconOffsetMs: int64(i) * intervalMs / int64(len(nodeIDs)),
			RelayEnabled:   false,
			Battery:        0.8,
			Stability:      0.8,
		}
		switch nodeID {
		case 2:
			node.RelayCapable = true
			node.RelayEnabled = true
			node.QualityFromSource = 1.0
			node.GroupCoverage = 0.95
			node.Battery = 0.9
			node.Stability = 0.9
		case 3:
			node.RelayCapable = true
			node.RelayEnabled = true
			node.QualityFromSource = 0.8
			node.GroupCoverage = 0.8
			node.Battery = 0.9
			node.Stability = 0.8
		}
		nodes = append(nodes, node)
	}
	return PeriodicBeaconNeighborScenarioConfig{
		ScenarioID:        PeriodicBeaconNeighborScenarioID,
		Seed:              DefaultScenarioSeed,
		SourceID:          DefaultSourceID,
		Policy:            election.DefaultPolicy(),
		Nodes:             nodes,
		BeaconIntervalMs:  intervalMs,
		BeaconFrameBytes:  defaultPeriodicBeaconFrameBytes,
		ChannelBitrateBps: 19200,
		BeaconGuardMs:     1,
		NeighborStaleMs:   defaultPeriodicBeaconStaleMs,
		NeighborDeadMs:    defaultPeriodicBeaconDeadMs,
		SimulationEndMs:   defaultPeriodicBeaconSimulationMs,
		VoicePolicy:       BeaconVoicePolicyDeferAfterVoice,
		VoiceWindows: []PeriodicBeaconVoiceWindow{
			{StartMs: defaultPeriodicBeaconVoiceStartMs, DurationMs: defaultPeriodicBeaconVoiceDurationMs},
		},
	}
}

func RunPeriodicBeaconNeighborScenario(cfg PeriodicBeaconNeighborScenarioConfig) (PeriodicBeaconNeighborReport, error) {
	cfg = normalizePeriodicBeaconNeighborScenarioConfig(cfg)
	if err := validatePeriodicBeaconNeighborScenarioConfig(cfg); err != nil {
		return PeriodicBeaconNeighborReport{}, err
	}

	measurement, err := channel.MeasureFrameBytes(cfg.BeaconFrameBytes, channel.AirtimeConfig{
		ChannelBitrateBps: cfg.ChannelBitrateBps,
		SlotDurationMs:    cfg.BeaconIntervalMs,
	})
	if err != nil {
		return PeriodicBeaconNeighborReport{}, err
	}

	dueBeacons, err := periodicBeaconDueList(cfg)
	if err != nil {
		return PeriodicBeaconNeighborReport{}, err
	}
	report := PeriodicBeaconNeighborReport{
		ScenarioID:        cfg.ScenarioID,
		Seed:              cfg.Seed,
		SourceID:          cfg.SourceID,
		BeaconIntervalMs:  cfg.BeaconIntervalMs,
		BeaconFrameBytes:  cfg.BeaconFrameBytes,
		BeaconAirtimeMs:   measurement.AirtimeMs,
		ChannelBitrateBps: cfg.ChannelBitrateBps,
		BeaconGuardMs:     cfg.BeaconGuardMs,
		NeighborStaleMs:   cfg.NeighborStaleMs,
		NeighborDeadMs:    cfg.NeighborDeadMs,
		SimulationEndMs:   cfg.SimulationEndMs,
		VoicePolicy:       cfg.VoicePolicy,
		VoiceWindows:      clonePeriodicBeaconVoiceWindows(cfg.VoiceWindows),
		Events:            make([]PeriodicBeaconEvent, 0, len(dueBeacons)+len(cfg.VoiceWindows)*2),
		NeighborRows:      make([]PeriodicBeaconNeighborRow, 0, len(cfg.Nodes)),
		CallStartMs:       cfg.VoiceWindows[0].StartMs,
		BeaconDueCount:    len(dueBeacons),
	}

	for _, window := range cfg.VoiceWindows {
		endMs, err := periodicBeaconWindowEnd(window)
		if err != nil {
			return PeriodicBeaconNeighborReport{}, err
		}
		report.Events = append(report.Events,
			PeriodicBeaconEvent{TimeMs: window.StartMs, Type: PeriodicBeaconEventVoiceStart},
			PeriodicBeaconEvent{TimeMs: endMs, Type: PeriodicBeaconEventVoiceEnd},
		)
	}

	lastSeenByNode := make(map[uint16][]int64, len(cfg.Nodes))
	channelFreeAtMs := int64(0)
	for _, due := range dueBeacons {
		txAtMs, deferred, reason, duringVoice, err := periodicBeaconTxStart(due.due, measurement.AirtimeMs, channelFreeAtMs, cfg)
		if err != nil {
			return PeriodicBeaconNeighborReport{}, err
		}
		if txAtMs > cfg.SimulationEndMs {
			continue
		}
		rxCompleteMs, err := checkedLimitedMultiHopAdd(txAtMs, measurement.AirtimeMs)
		if err != nil {
			return PeriodicBeaconNeighborReport{}, err
		}
		if rxCompleteMs > cfg.SimulationEndMs {
			continue
		}
		channelFreeAtMs, err = checkedLimitedMultiHopAdd(rxCompleteMs, cfg.BeaconGuardMs)
		if err != nil {
			return PeriodicBeaconNeighborReport{}, err
		}
		lastSeenByNode[due.node.NodeID] = append(lastSeenByNode[due.node.NodeID], rxCompleteMs)
		report.Events = append(report.Events, PeriodicBeaconEvent{
			TimeMs:         txAtMs,
			Type:           PeriodicBeaconEventBeaconTx,
			NodeID:         due.node.NodeID,
			OriginalDueMs:  due.due,
			TxDurationMs:   measurement.AirtimeMs,
			RxCompleteMs:   rxCompleteMs,
			Deferred:       deferred,
			DeferredReason: reason,
		})
		report.BeaconTxCount++
		report.TotalBeaconAirtimeMs += measurement.AirtimeMs
		if deferred {
			report.DeferredBeaconCount++
		}
		if duringVoice {
			report.BeaconDuringVoiceCount++
		}
	}

	sortPeriodicBeaconEvents(report.Events)
	for i := range report.Events {
		report.Events[i].Index = i
	}

	report.NeighborRows = periodicBeaconNeighborRows(cfg, lastSeenByNode, report.CallStartMs)
	report.TableConvergedAtMs = periodicBeaconTableConvergedAt(report.NeighborRows)
	report.applyNeighborCountsAndElection(cfg)
	if report.SimulationEndMs > 0 {
		report.ControlChannelOccupancyPct = float64(report.TotalBeaconAirtimeMs) * 100 / float64(report.SimulationEndMs)
	}

	hash, err := periodicBeaconNeighborReportHash(report)
	if err != nil {
		return PeriodicBeaconNeighborReport{}, err
	}
	report.EventHash = hash
	return report, nil
}
