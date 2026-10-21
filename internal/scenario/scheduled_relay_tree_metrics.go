package scenario

import (
	"math"
	"sort"

	"github.com/rmsff/ad-hoc-radio/internal/node"
	"github.com/rmsff/ad-hoc-radio/internal/peer"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
	"github.com/rmsff/ad-hoc-radio/internal/transport"
)

type ScheduledRelayTreeEdgeReport struct {
	ParentStationID uint16 `json:"parent_station_id"`
	ChildStationID  uint16 `json:"child_station_id"`
	Hop             uint8  `json:"hop"`
}

type RelayDepthLatencyReport struct {
	RelayDepth  uint8   `json:"relay_depth"`
	SampleCount int     `json:"sample_count"`
	Min         float64 `json:"min"`
	Avg         float64 `json:"avg"`
	P50         float64 `json:"p50"`
	P95         float64 `json:"p95"`
	Max         float64 `json:"max"`
}

// ScheduledMemberDeliveryCount keeps every expected recipient in the denominator.
type ScheduledMemberDeliveryCount struct {
	MemberID       uint16 `json:"member_id"`
	FramesAccepted int    `json:"frames_accepted"`
	FramesExpected int    `json:"frames_expected"`
	FramesOnTime   int    `json:"frames_on_time"`
	FramesLate     int    `json:"frames_late"`
	FramesMissing  int    `json:"frames_missing"`
	FramesPlayed   int    `json:"frames_played"`
}

type ScheduledRelayTreeReport struct {
	CollisionDropCount        int                            `json:"collision_drop_count"`
	FirstPlayoutMS            *int64                         `json:"first_playout_ms,omitempty"`
	Control                   *peer.Counters                 `json:"control,omitempty"`
	Wire                      *ScheduledWireEvidence         `json:"wire,omitempty"`
	ExpectedServiceOutcome    string                         `json:"expected_service_outcome"`
	Audio                     ScheduledAudioEvidence         `json:"audio"`
	ScheduleID                uint32                         `json:"schedule_id"`
	ScheduleVersion           uint16                         `json:"schedule_version"`
	ScheduleHash              string                         `json:"schedule_hash"`
	TreeHash                  string                         `json:"tree_hash"`
	ResultHash                string                         `json:"result_hash"`
	TimingStrategy            string                         `json:"timing_strategy"`
	TimingEpochID             uint32                         `json:"timing_epoch_id"`
	TimingAuthorityID         uint16                         `json:"timing_authority_id"`
	ExpectedMemberIDs         []uint16                       `json:"expected_member_ids"`
	CoveredMemberIDs          []uint16                       `json:"covered_member_ids"`
	DeliveredMemberIDs        []uint16                       `json:"delivered_member_ids"`
	UncoveredMemberIDs        []uint16                       `json:"uncovered_member_ids"`
	SelectedTreeEdges         []ScheduledRelayTreeEdgeReport `json:"selected_tree_edges"`
	SelectedRelayDepth        uint8                          `json:"selected_relay_depth"`
	TxOpportunityCount        int                            `json:"tx_opportunity_count"`
	ScheduleCycleDurationMS   int64                          `json:"schedule_cycle_duration_ms"`
	VoiceAirtimePct           float64                        `json:"voice_airtime_pct"`
	ControlAirtimePct         float64                        `json:"control_airtime_pct"`
	OutOfScheduleTxCount      int                            `json:"out_of_schedule_tx_count"`
	MissedOpportunityCount    int                            `json:"missed_opportunity_count"`
	ScheduleMismatchDropCount int                            `json:"schedule_mismatch_drop_count"`
	ScheduleExpiredDropCount  int                            `json:"schedule_expired_drop_count"`
	NotScheduledDropCount     int                            `json:"not_scheduled_drop_count"`
	LatencyByRelayDepth       []RelayDepthLatencyReport      `json:"latency_by_relay_depth"`
	DuplicatePlayoutCount     int                            `json:"duplicate_playout_count"`
	RouteReplanCount          int                            `json:"route_replan_count"`
	VersionActivationCount    int                            `json:"version_activation_count"`
	BlockedFromSeq            *uint32                        `json:"blocked_from_seq,omitempty"`
}

func scheduledRelayTreeScenarioMetrics(
	cfg ScheduledRelayTreeScenarioConfig,
	result ScheduledRuntimeResult,
) ScenarioMetrics {
	receiverCounters := aggregateScheduledReceiverCounters(result.ReceiverCounters)
	relayDrops := scheduledRelayDropCounters(result.Counters.DroppedByReason, receiverCounters.DroppedByReason)
	linkCounters, delaySums, delayCounts, defaults := scheduledRelayTreeLinkMetrics(result.LinkObservations)
	latencySamples := scheduledPlayoutSamples(result.PlayoutEvents)

	outcome := scheduledAudioEvidence(result).ServiceOutcome
	if result.Counters.OutOfScheduleTxCount != 0 || result.Counters.DuplicatePlayoutCount != 0 {
		outcome = "fail"
	}
	for _, member := range result.MemberDelivery {
		if member.FramesPlayed != member.FramesExpected || member.FramesPlayed == 0 {
			outcome = "fail"
		}
	}
	if cfg.ExpectedServiceOutcome == "fail" {
		if outcome == "fail" && result.Counters.OutOfScheduleTxCount == 0 && result.Counters.DuplicatePlayoutCount == 0 {
			outcome = "pass"
		} else {
			outcome = "fail"
		}
	}
	return ScenarioMetrics{
		SourceFramesCreated:       result.Counters.SourceFramesCreated,
		SourceFramesSent:          result.Counters.SourceTransmissions,
		RelayFramesReceived:       result.Counters.RelayFramesReceived,
		RelayForwarded:            result.Counters.RelayFramesForwarded,
		RelayDroppedByReason:      relayDrops,
		RelaySeenCacheEntries:     result.RelaySeenCacheEntries,
		RelaySeenCacheEvictions:   result.RelaySeenCacheEvictions,
		ReceiverCounters:          receiverCounters,
		LinkCounters:              linkCounters,
		LinkDelaySumsMs:           delaySums,
		LinkDelayCounts:           delayCounts,
		EventCount:                result.EventCount,
		EventHash:                 result.EventHash,
		ScheduledRelayTree:        scheduledRelayTreeReport(cfg, result),
		ScheduledMemberDelivery:   append([]ScheduledMemberDeliveryCount(nil), result.MemberDelivery...),
		OutcomeOverride:           outcome,
		Notes:                     scheduledEvidenceNotes(result),
		ScenarioID:                cfg.ScenarioID,
		Seed:                      cfg.Seed,
		SimDurationMs:             result.SimDurationMS,
		SourceID:                  cfg.SourceStationID,
		StartMs:                   cfg.StartMS,
		FramePeriodMs:             cfg.InitialPlan.CostModel.FramePeriodMS,
		PlayoutLatencySamplesMs:   latencySamples,
		OutOfSlotTxCount:          result.Counters.OutOfScheduleTxCount,
		RelayForwardQueueMaxDepth: result.Counters.ForwardQueueMaxDepth,
		RelayForwardQueueDrops:    result.Counters.ForwardQueueDrops,
		LinkReportDefaults:        defaults,
	}
}

func scheduledRelayTreeMemberDelivery(counters map[uint16]node.ReceiverCounters) []ScheduledMemberDeliveryCount {
	ids := make([]int, 0, len(counters))
	for id := range counters {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)
	rows := make([]ScheduledMemberDeliveryCount, 0, len(ids))
	for _, rawID := range ids {
		id := uint16(rawID)
		rows = append(rows, ScheduledMemberDeliveryCount{
			MemberID:       id,
			FramesAccepted: counters[id].FramesAccepted,
		})
	}
	return rows
}

func scheduledRelayTreeReport(cfg ScheduledRelayTreeScenarioConfig, result ScheduledRuntimeResult) *ScheduledRelayTreeReport {
	schedule := result.ActiveSchedule
	edges := make([]ScheduledRelayTreeEdgeReport, len(result.ActiveTree.Edges))
	for i, edge := range result.ActiveTree.Edges {
		edges[i] = ScheduledRelayTreeEdgeReport{
			ParentStationID: edge.ParentStationID,
			ChildStationID:  edge.ChildStationID,
			Hop:             edge.Hop,
		}
	}
	return &ScheduledRelayTreeReport{
		CollisionDropCount:        result.Counters.CollisionDropCount,
		FirstPlayoutMS:            scheduledFirstPlayout(result.PlayoutEvents),
		Control:                   result.ControlCounters,
		Wire:                      result.Wire,
		ExpectedServiceOutcome:    scheduledExpectedOutcome(cfg.ExpectedServiceOutcome),
		Audio:                     scheduledAudioEvidence(result),
		ScheduleID:                schedule.Binding.ScheduleID,
		ScheduleVersion:           schedule.Binding.ScheduleVersion,
		ScheduleHash:              result.ScheduleHash,
		TreeHash:                  result.TreeHash,
		ResultHash:                result.ResultHash,
		TimingStrategy:            string(schedule.Binding.TimingStrategy),
		TimingEpochID:             cfg.TimingEpochID,
		TimingAuthorityID:         schedule.Binding.TimingAuthorityID,
		ExpectedMemberIDs:         scheduledRelayTreeMemberIDs(result.ExpectedMemberIDs),
		CoveredMemberIDs:          scheduledRelayTreeMemberIDs(result.CoveredMemberIDs),
		DeliveredMemberIDs:        scheduledRelayTreeMemberIDs(result.DeliveredMemberIDs),
		UncoveredMemberIDs:        scheduledRelayTreeMemberIDs(result.UncoveredMemberIDs),
		SelectedTreeEdges:         edges,
		SelectedRelayDepth:        result.ActiveTree.MaxRelayDepth,
		TxOpportunityCount:        len(schedule.Opportunities),
		ScheduleCycleDurationMS:   int64(schedule.CycleDurationMS()),
		VoiceAirtimePct:           scheduledRelayTreeVoiceAirtimePct(schedule.Opportunities, cfg.InitialPlan.CostModel.FramePeriodMS),
		ControlAirtimePct:         scheduledControlAirtimePct(cfg, result),
		OutOfScheduleTxCount:      result.Counters.OutOfScheduleTxCount,
		MissedOpportunityCount:    result.Counters.MissedOpportunityCount,
		ScheduleMismatchDropCount: result.Counters.ScheduleMismatchDropCount,
		ScheduleExpiredDropCount:  result.Counters.ScheduleExpiredDropCount,
		NotScheduledDropCount:     result.Counters.NotScheduledDropCount,
		LatencyByRelayDepth:       scheduledRelayTreeLatencyReport(result.LatencyByRelayDepth),
		DuplicatePlayoutCount:     result.Counters.DuplicatePlayoutCount,
		RouteReplanCount:          result.Counters.RouteReplanCount,
		VersionActivationCount:    result.Counters.VersionActivationCount,
		BlockedFromSeq:            cloneUint32Pointer(result.BlockedFromSeq),
	}
}

func cloneScheduledRelayTreeReport(in *ScheduledRelayTreeReport) *ScheduledRelayTreeReport {
	if in == nil {
		return nil
	}
	out := *in
	if in.FirstPlayoutMS != nil {
		v := *in.FirstPlayoutMS
		out.FirstPlayoutMS = &v
	}
	out.Audio.Members = append([]ScheduledMemberDeliveryCount(nil), in.Audio.Members...)
	if in.Audio.ArrivalLatency != nil {
		v := *in.Audio.ArrivalLatency
		out.Audio.ArrivalLatency = &v
	}
	if in.Audio.PlayoutLatency != nil {
		v := *in.Audio.PlayoutLatency
		out.Audio.PlayoutLatency = &v
	}
	if in.Wire != nil {
		v := *in.Wire
		out.Wire = &v
	}
	if in.Control != nil {
		v := *in.Control
		out.Control = &v
	}
	out.ExpectedMemberIDs = scheduledRelayTreeMemberIDs(in.ExpectedMemberIDs)
	out.CoveredMemberIDs = scheduledRelayTreeMemberIDs(in.CoveredMemberIDs)
	out.DeliveredMemberIDs = scheduledRelayTreeMemberIDs(in.DeliveredMemberIDs)
	out.UncoveredMemberIDs = scheduledRelayTreeMemberIDs(in.UncoveredMemberIDs)
	out.SelectedTreeEdges = append([]ScheduledRelayTreeEdgeReport(nil), in.SelectedTreeEdges...)
	out.LatencyByRelayDepth = append([]RelayDepthLatencyReport(nil), in.LatencyByRelayDepth...)
	out.BlockedFromSeq = cloneUint32Pointer(in.BlockedFromSeq)
	return &out
}

func scheduledRelayTreeMemberIDs(in []uint16) []uint16 {
	if len(in) == 0 {
		return []uint16{}
	}
	return append([]uint16(nil), in...)
}

func scheduledRelayTreeLatencyReport(in []ScheduledRelayDepthLatencySamples) []RelayDepthLatencyReport {
	out := make([]RelayDepthLatencyReport, len(in))
	for i, bucket := range in {
		summary := summarize(nonNegativeInt64sToFloat64s(bucket.SamplesMS))
		out[i] = RelayDepthLatencyReport{
			RelayDepth:  bucket.RelayDepth,
			SampleCount: len(bucket.SamplesMS),
			Min:         summary.Min,
			Avg:         summary.Avg,
			P50:         summary.P50,
			P95:         summary.P95,
			Max:         summary.Max,
		}
	}
	return out
}

func scheduledRelayTreeVoiceAirtimePct(opportunities []timing.TxOpportunity, framePeriodMS int64) float64 {
	var durationMS int64
	for _, opportunity := range opportunities {
		durationMS += int64(opportunity.DurationMS)
	}
	if framePeriodMS <= 0 {
		return 0
	}
	return math.Round((float64(durationMS)*100/float64(framePeriodMS))*1_000) / 1_000
}

func aggregateScheduledReceiverCounters(counters map[uint16]node.ReceiverCounters) node.ReceiverCounters {
	out := node.ReceiverCounters{DroppedByReason: make(map[node.DropReason]int)}
	ids := make([]int, 0, len(counters))
	for id := range counters {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)
	for _, rawID := range ids {
		counter := counters[uint16(rawID)]
		out.FramesReceived += counter.FramesReceived
		out.FramesAccepted += counter.FramesAccepted
		out.DuplicatesSuppressed += counter.DuplicatesSuppressed
		out.DirectFramesAccepted += counter.DirectFramesAccepted
		out.RelayedFramesAccepted += counter.RelayedFramesAccepted
		out.LateFrames += counter.LateFrames
		out.MissingSequences += counter.MissingSequences
		out.PlayoutFramesEmitted += counter.PlayoutFramesEmitted
		for reason, count := range counter.DroppedByReason {
			out.DroppedByReason[reason] += count
		}
	}
	return out
}

func scheduledRelayDropCounters(all, receiver map[node.DropReason]int) map[node.DropReason]int {
	out := cloneDropCounters(all)
	for reason, count := range receiver {
		out[reason] -= count
		if out[reason] <= 0 {
			delete(out, reason)
		}
	}
	return out
}

func scheduledRelayTreeLinkMetrics(observations []ScheduledLinkObservation) (
	transport.DeliveryCounters,
	map[string]int64,
	map[string]int,
	[]LinkEndpoint,
) {
	counters := transport.DeliveryCounters{
		Delivered: make(map[string]int), Lost: make(map[string]int),
		Corrupted: make(map[string]int), Duplicated: make(map[string]int),
	}
	delaySums := make(map[string]int64, len(observations))
	delayCounts := make(map[string]int, len(observations))
	defaults := make([]LinkEndpoint, 0, len(observations))
	for _, observation := range observations {
		key := linkReportKey(observation.FromStationID, observation.ToStationID)
		counters.Delivered[key] = observation.Deliveries
		counters.Lost[key] = observation.Lost
		counters.Corrupted[key] = observation.Corrupted
		counters.Duplicated[key] = observation.Duplicated
		delaySums[key] = observation.DelaySumMS
		delayCounts[key] = observation.Deliveries
		defaults = append(defaults, LinkEndpoint{From: observation.FromStationID, To: observation.ToStationID})
	}
	return counters, delaySums, delayCounts, defaults
}

func scheduledRelayTreeFlattenedLatency(buckets []ScheduledRelayDepthLatencySamples) []int64 {
	var out []int64
	for _, bucket := range buckets {
		out = append(out, bucket.SamplesMS...)
	}
	return out
}
