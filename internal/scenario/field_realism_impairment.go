package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/rmsff/ad-hoc-radio/internal/channel"
	"github.com/rmsff/ad-hoc-radio/internal/election"
)

const FieldRealismImpairmentScenarioID = "field_realism_impairment_sweep"

const (
	FieldRealismOutcomePass     = "pass"
	FieldRealismOutcomeDegraded = "degraded"
	FieldRealismOutcomeFail     = "fail"
)

const (
	FieldRealismReasonDirectedLinkDegraded  = "directed_link_degraded"
	FieldRealismReasonNoFreshRelayCandidate = "no_fresh_relay_candidate"
	FieldRealismReasonGuardExhausted        = "guard_exhausted"
	FieldRealismReasonExternalInterference  = "external_interference"
	FieldRealismReasonHiddenNodeCollision   = "hidden_node_collision"
	FieldRealismReasonLocalBusy             = "local_busy"
	FieldRealismReasonBeaconStale           = "beacon_stale"
	FieldRealismReasonBeaconDead            = "beacon_dead"
)

const (
	FieldRealismBusyActionDefer = "defer"
	FieldRealismBusyActionDrop  = "drop"
)

const (
	maxFieldRealismImpairmentRows = 10_000
	maxFieldRealismBeaconDueCount = 1_000_000
)

type FieldRealismImpairmentConfig struct {
	ScenarioID                  string
	Seed                        uint64
	SourceID                    uint16
	ReceiverID                  uint16
	Nodes                       []FieldRealismNode
	DirectedLinks               []FieldRealismDirectedLinkTimeline
	Policy                      election.Policy
	ClockDriftPpmCandidates     []int64
	SlotOffsetMsCandidates      []int64
	BeaconLossPctCandidates     []float64
	BeaconJitterMsCandidates    []int64
	CallStartMs                 int64
	VoicePeriodMs               int64
	FrameCount                  int
	SourceSlotMs                int64
	RelaySlotMs                 int64
	GuardTimeMs                 int64
	LinkDelayMs                 int64
	SourceFrameBytes            int
	RelayFrameBytes             int
	BeaconFrameBytes            int
	ChannelBitrateBps           int64
	PreambleMs                  int64
	SyncMs                      int64
	TxRxSwitchMs                int64
	BeaconIntervalMs            int64
	NeighborStaleMs             int64
	NeighborDeadMs              int64
	ExternalInterferenceWindows []FieldRealismWindow
	HiddenNodeTxWindows         []FieldRealismHiddenNodeWindow
	LocalBusyWindows            []FieldRealismLocalBusyWindow
}

type FieldRealismNode struct {
	NodeID         uint16
	BeaconOffsetMs int64
	RelayCapable   bool
	RelayEnabled   bool
	Busy           bool
	Battery        float64
	Stability      float64
	HealthTimeline []FieldRealismNodeHealthPoint
}

type FieldRealismNodeHealthPoint struct {
	AtMs         int64
	RelayEnabled bool
	Busy         bool
	Battery      float64
	Stability    float64
}

type FieldRealismDirectedLinkTimeline struct {
	FromID uint16
	ToID   uint16
	Points []FieldRealismQualityPoint
}

type FieldRealismQualityPoint struct {
	AtMs    int64
	Quality float64
}

type FieldRealismWindow struct {
	StartMs    int64
	DurationMs int64
}

type FieldRealismHiddenNodeWindow struct {
	StartMs      int64
	DurationMs   int64
	ReceiverID   uint16
	InterfererID uint16
}

type FieldRealismLocalBusyWindow struct {
	NodeID     uint16
	StartMs    int64
	DurationMs int64
	Action     string
}

type FieldRealismImpairmentReport struct {
	ScenarioID   string                      `json:"scenario_id"`
	Seed         uint64                      `json:"seed"`
	Inventory    FieldRealismInventory       `json:"inventory"`
	Aggregates   FieldRealismAggregate       `json:"aggregates"`
	RowCount     int                         `json:"row_count"`
	PassRows     int                         `json:"pass_rows"`
	DegradedRows int                         `json:"degraded_rows"`
	FailRows     int                         `json:"fail_rows"`
	Rows         []FieldRealismImpairmentRow `json:"rows"`
	EventHash    string                      `json:"event_hash"`
}

type FieldRealismInventory struct {
	ModeledNow           []string `json:"modeled_now"`
	SimulatedInThisStory []string `json:"simulated_in_this_story"`
	ExplicitlyDeferred   []string `json:"explicitly_deferred"`
}

type FieldRealismAggregate struct {
	ExternalInterferenceCount int `json:"external_interference_count"`
	HiddenNodeCollisionCount  int `json:"hidden_node_collision_count"`
	LocalBusyDeferCount       int `json:"local_busy_defer_count"`
	LocalBusyDropCount        int `json:"local_busy_drop_count"`
	DriftCollisionRiskCount   int `json:"drift_collision_risk_count"`
	OutOfSlotTxCount          int `json:"out_of_slot_tx_count"`
	BeaconLostCount           int `json:"beacon_lost_count"`
	BeaconJitteredCount       int `json:"beacon_jittered_count"`
}

type FieldRealismImpairmentRow struct {
	RowID                     string                     `json:"row_id"`
	ClockDriftPpm             int64                      `json:"clock_drift_ppm"`
	SlotOffsetMs              int64                      `json:"slot_offset_ms"`
	SlotOffsetMsEffective     int64                      `json:"slot_offset_ms_effective"`
	GuardTimeMs               int64                      `json:"guard_time_ms"`
	GuardRemainingMs          int64                      `json:"guard_remaining_ms"`
	BeaconLossPct             float64                    `json:"beacon_loss_pct"`
	BeaconJitterMs            int64                      `json:"beacon_jitter_ms"`
	SourceFrameBytes          int                        `json:"source_frame_bytes"`
	RelayFrameBytes           int                        `json:"relay_frame_bytes"`
	BeaconFrameBytes          int                        `json:"beacon_frame_bytes"`
	SourceAirtimeMs           int64                      `json:"source_airtime_ms"`
	RelayAirtimeMs            int64                      `json:"relay_airtime_ms"`
	BeaconAirtimeMs           int64                      `json:"beacon_airtime_ms"`
	SelectedRelayID           uint16                     `json:"selected_relay_id"`
	SelectedRelayScore        float64                    `json:"selected_relay_score"`
	FreshNeighborCount        int                        `json:"fresh_neighbor_count"`
	StaleNeighborCount        int                        `json:"stale_neighbor_count"`
	DeadNeighborCount         int                        `json:"dead_neighbor_count"`
	NeverSeenNeighborCount    int                        `json:"never_seen_neighbor_count"`
	FreshRelayCandidateCount  int                        `json:"fresh_relay_candidate_count"`
	BeaconDueCount            int                        `json:"beacon_due_count"`
	BeaconTxCount             int                        `json:"beacon_tx_count"`
	BeaconLostCount           int                        `json:"beacon_lost_count"`
	BeaconJitteredCount       int                        `json:"beacon_jittered_count"`
	DriftCollisionRiskCount   int                        `json:"drift_collision_risk_count"`
	OutOfSlotTxCount          int                        `json:"out_of_slot_tx_count"`
	ExternalInterferenceCount int                        `json:"external_interference_count"`
	HiddenNodeCollisionCount  int                        `json:"hidden_node_collision_count"`
	LocalBusyDeferCount       int                        `json:"local_busy_defer_count"`
	LocalBusyDropCount        int                        `json:"local_busy_drop_count"`
	Candidates                []FieldRealismCandidateRow `json:"candidates"`
	Outcome                   string                     `json:"outcome"`
	FailReasons               []string                   `json:"fail_reasons"`
}

type FieldRealismCandidateRow struct {
	NodeID            uint16  `json:"node_id"`
	LastSeenMs        int64   `json:"last_seen_ms"`
	AgeAtCallMs       int64   `json:"age_at_call_ms"`
	Status            string  `json:"status"`
	RelayCapable      bool    `json:"relay_capable"`
	RelayEnabled      bool    `json:"relay_enabled"`
	Busy              bool    `json:"busy"`
	QualityFromSource float64 `json:"quality_from_source"`
	GroupCoverage     float64 `json:"group_coverage"`
	ReverseQuality    float64 `json:"reverse_quality"`
	Battery           float64 `json:"battery"`
	Stability         float64 `json:"stability"`
	EligibleForRelay  bool    `json:"eligible_for_relay"`
}

func DefaultFieldRealismImpairmentConfig() FieldRealismImpairmentConfig {
	return FieldRealismImpairmentConfig{
		ScenarioID:               FieldRealismImpairmentScenarioID,
		Seed:                     DefaultScenarioSeed,
		SourceID:                 DefaultSourceID,
		ReceiverID:               defaultLimitedMultiHopReceiverID,
		Nodes:                    defaultFieldRealismNodes(),
		DirectedLinks:            defaultFieldRealismDirectedLinks(),
		Policy:                   election.DefaultPolicy(),
		ClockDriftPpmCandidates:  []int64{0, 20, 50, 100},
		SlotOffsetMsCandidates:   []int64{0, 1, 2, 5},
		BeaconLossPctCandidates:  []float64{0, 10, 50},
		BeaconJitterMsCandidates: []int64{0, 100, 1000},
		CallStartMs:              defaultPeriodicBeaconVoiceStartMs,
		VoicePeriodMs:            defaultTwoHopLiveVoicePeriodMs,
		FrameCount:               8,
		SourceSlotMs:             12,
		RelaySlotMs:              12,
		GuardTimeMs:              1,
		LinkDelayMs:              1,
		SourceFrameBytes:         defaultTwoHopLiveShortSourceFrameBytes,
		RelayFrameBytes:          defaultTwoHopLiveRelayFrameBytes,
		BeaconFrameBytes:         defaultPeriodicBeaconFrameBytes,
		ChannelBitrateBps:        19200,
		PreambleMs:               1,
		SyncMs:                   1,
		TxRxSwitchMs:             2,
		BeaconIntervalMs:         defaultPeriodicBeaconIntervalMs,
		NeighborStaleMs:          defaultPeriodicBeaconStaleMs,
		NeighborDeadMs:           defaultPeriodicBeaconDeadMs,
	}
}

func RunFieldRealismImpairmentSweep(cfg FieldRealismImpairmentConfig) (FieldRealismImpairmentReport, error) {
	cfg = normalizeFieldRealismImpairmentConfig(cfg)
	if err := validateFieldRealismImpairmentConfig(cfg); err != nil {
		return FieldRealismImpairmentReport{}, err
	}

	report := FieldRealismImpairmentReport{
		ScenarioID: cfg.ScenarioID,
		Seed:       cfg.Seed,
		Inventory:  defaultFieldRealismInventory(),
		Rows:       make([]FieldRealismImpairmentRow, 0, fieldRealismRowCount(cfg)),
	}

	for _, driftPpm := range cfg.ClockDriftPpmCandidates {
		for _, slotOffsetMs := range cfg.SlotOffsetMsCandidates {
			for _, beaconLossPct := range cfg.BeaconLossPctCandidates {
				for _, beaconJitterMs := range cfg.BeaconJitterMsCandidates {
					row, err := fieldRealismRow(cfg, driftPpm, slotOffsetMs, beaconLossPct, beaconJitterMs)
					if err != nil {
						return FieldRealismImpairmentReport{}, err
					}
					report.Rows = append(report.Rows, row)
					report.Aggregates.add(row)
					switch row.Outcome {
					case FieldRealismOutcomePass:
						report.PassRows++
					case FieldRealismOutcomeDegraded:
						report.DegradedRows++
					case FieldRealismOutcomeFail:
						report.FailRows++
					default:
						return FieldRealismImpairmentReport{}, fmt.Errorf("field realism impairment: unknown outcome %q", row.Outcome)
					}
				}
			}
		}
	}
	report.RowCount = len(report.Rows)

	hash, err := fieldRealismReportHash(report)
	if err != nil {
		return FieldRealismImpairmentReport{}, err
	}
	report.EventHash = hash
	return report, nil
}

func fieldRealismRow(
	cfg FieldRealismImpairmentConfig,
	driftPpm int64,
	slotOffsetMs int64,
	beaconLossPct float64,
	beaconJitterMs int64,
) (FieldRealismImpairmentRow, error) {
	sourceMeasurement, err := channel.MeasureFrameBytes(cfg.SourceFrameBytes, channel.AirtimeConfig{
		ChannelBitrateBps: cfg.ChannelBitrateBps,
		SlotDurationMs:    cfg.SourceSlotMs,
	})
	if err != nil {
		return FieldRealismImpairmentRow{}, err
	}
	relayMeasurement, err := channel.MeasureFrameBytes(cfg.RelayFrameBytes, channel.AirtimeConfig{
		ChannelBitrateBps: cfg.ChannelBitrateBps,
		SlotDurationMs:    cfg.RelaySlotMs,
	})
	if err != nil {
		return FieldRealismImpairmentRow{}, err
	}
	beaconMeasurement, err := channel.MeasureFrameBytes(cfg.BeaconFrameBytes, channel.AirtimeConfig{
		ChannelBitrateBps: cfg.ChannelBitrateBps,
		SlotDurationMs:    cfg.BeaconIntervalMs,
	})
	if err != nil {
		return FieldRealismImpairmentRow{}, err
	}

	candidates, beaconCounts, beaconWindows, err := fieldRealismCandidateRows(cfg, beaconMeasurement.AirtimeMs, beaconLossPct, beaconJitterMs)
	if err != nil {
		return FieldRealismImpairmentRow{}, err
	}
	freshCandidates := fieldRealismFreshElectionCandidates(candidates)
	result, err := election.SelectRelay(freshCandidates, cfg.Policy)
	selectedRelayID := uint16(0)
	selectedScore := 0.0
	reasons := make([]string, 0, 6)
	if err != nil {
		if !errors.Is(err, election.ErrNoEligibleRelay) {
			return FieldRealismImpairmentRow{}, err
		}
		reasons = append(reasons, FieldRealismReasonNoFreshRelayCandidate)
	} else {
		selectedRelayID = result.RelayID
		selectedScore = result.Score
	}

	slotOffsetEffective, guardRemaining, driftRiskCount, err := fieldRealismGuardBudget(cfg, driftPpm, slotOffsetMs)
	if err != nil {
		return FieldRealismImpairmentRow{}, err
	}
	if driftRiskCount > 0 {
		reasons = append(reasons, FieldRealismReasonGuardExhausted)
	}

	sourceTxDuration, err := checkedLimitedMultiHopAdd(sourceMeasurement.AirtimeMs, cfg.PreambleMs)
	if err != nil {
		return FieldRealismImpairmentRow{}, err
	}
	sourceTxDuration, err = checkedLimitedMultiHopAdd(sourceTxDuration, cfg.SyncMs)
	if err != nil {
		return FieldRealismImpairmentRow{}, err
	}
	relayTxDuration, err := checkedLimitedMultiHopAdd(relayMeasurement.AirtimeMs, cfg.PreambleMs)
	if err != nil {
		return FieldRealismImpairmentRow{}, err
	}
	relayTxDuration, err = checkedLimitedMultiHopAdd(relayTxDuration, cfg.SyncMs)
	if err != nil {
		return FieldRealismImpairmentRow{}, err
	}
	txWindows, err := fieldRealismVoiceTxWindows(cfg, sourceTxDuration, relayTxDuration, selectedRelayID)
	if err != nil {
		return FieldRealismImpairmentRow{}, err
	}
	txWindows = append(txWindows, beaconWindows...)
	occupancy, err := fieldRealismOccupancyCounts(cfg, txWindows)
	if err != nil {
		return FieldRealismImpairmentRow{}, err
	}
	occupancy.localBusyDefer += beaconCounts.localBusyDefer
	occupancy.localBusyDrop += beaconCounts.localBusyDrop
	reasons = fieldRealismAppendOccupancyReasons(reasons, occupancy)
	reasons = fieldRealismAppendCandidateReasons(reasons, candidates)

	row := FieldRealismImpairmentRow{
		RowID:                     fieldRealismRowID(driftPpm, slotOffsetMs, beaconLossPct, beaconJitterMs),
		ClockDriftPpm:             driftPpm,
		SlotOffsetMs:              slotOffsetMs,
		SlotOffsetMsEffective:     slotOffsetEffective,
		GuardTimeMs:               cfg.GuardTimeMs,
		GuardRemainingMs:          guardRemaining,
		BeaconLossPct:             beaconLossPct,
		BeaconJitterMs:            beaconJitterMs,
		SourceFrameBytes:          cfg.SourceFrameBytes,
		RelayFrameBytes:           cfg.RelayFrameBytes,
		BeaconFrameBytes:          cfg.BeaconFrameBytes,
		SourceAirtimeMs:           sourceMeasurement.AirtimeMs,
		RelayAirtimeMs:            relayMeasurement.AirtimeMs,
		BeaconAirtimeMs:           beaconMeasurement.AirtimeMs,
		SelectedRelayID:           selectedRelayID,
		SelectedRelayScore:        selectedScore,
		FreshNeighborCount:        beaconCounts.freshNeighbors,
		StaleNeighborCount:        beaconCounts.staleNeighbors,
		DeadNeighborCount:         beaconCounts.deadNeighbors,
		NeverSeenNeighborCount:    beaconCounts.neverSeenNeighbors,
		FreshRelayCandidateCount:  len(freshCandidates),
		BeaconDueCount:            beaconCounts.due,
		BeaconTxCount:             beaconCounts.tx,
		BeaconLostCount:           beaconCounts.lost,
		BeaconJitteredCount:       beaconCounts.jittered,
		DriftCollisionRiskCount:   driftRiskCount,
		OutOfSlotTxCount:          driftRiskCount,
		ExternalInterferenceCount: occupancy.externalInterference,
		HiddenNodeCollisionCount:  occupancy.hiddenNodeCollision,
		LocalBusyDeferCount:       occupancy.localBusyDefer,
		LocalBusyDropCount:        occupancy.localBusyDrop,
		Candidates:                candidates,
		Outcome:                   fieldRealismOutcome(reasons, occupancy),
		FailReasons:               reasons,
	}
	return row, nil
}

func defaultFieldRealismInventory() FieldRealismInventory {
	return FieldRealismInventory{
		ModeledNow: []string{
			"voice_frame_airtime",
			"relay_election_scoring",
			"periodic_beacon_neighbor_freshness",
		},
		SimulatedInThisStory: []string{
			"directed_asymmetric_link_quality",
			"clock_drift_ppm",
			"slot_offset_ms",
			"external_interference_windows",
			"hidden_node_tx_windows",
			"local_busy_windows",
			"beacon_loss_pct",
			"beacon_jitter_ms",
			"dynamic_relay_health_timeline",
		},
		ExplicitlyDeferred: []string{
			"real_rf_synchronization",
			"capture_effect",
			"codec_quality_degradation",
			"hardware_measured_tx_rx_switch_time",
			"receiver_ack_delivery_receipts",
		},
	}
}

func (a *FieldRealismAggregate) add(row FieldRealismImpairmentRow) {
	a.ExternalInterferenceCount += row.ExternalInterferenceCount
	a.HiddenNodeCollisionCount += row.HiddenNodeCollisionCount
	a.LocalBusyDeferCount += row.LocalBusyDeferCount
	a.LocalBusyDropCount += row.LocalBusyDropCount
	a.DriftCollisionRiskCount += row.DriftCollisionRiskCount
	a.OutOfSlotTxCount += row.OutOfSlotTxCount
	a.BeaconLostCount += row.BeaconLostCount
	a.BeaconJitteredCount += row.BeaconJitteredCount
}

func fieldRealismReportHash(report FieldRealismImpairmentReport) (string, error) {
	hashable := report
	hashable.EventHash = ""
	b, err := json.Marshal(hashable)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
