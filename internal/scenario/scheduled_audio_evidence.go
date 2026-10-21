package scenario

import "sort"

// Version 2 corrects arrival-as-playout without relabeling historical evidence.
type ScheduledAudioEvidence struct {
	Version        int                            `json:"version"`
	ServiceOutcome string                         `json:"service_outcome"`
	Members        []ScheduledMemberDeliveryCount `json:"members"`
	ArrivalLatency *RelayDepthLatencyReport       `json:"arrival_latency_ms"`
	PlayoutLatency *RelayDepthLatencyReport       `json:"playout_latency_ms"`
}

func (r *scheduledRuntime) memberDelivery() []ScheduledMemberDeliveryCount {
	ids := map[uint16]bool{}
	for _, v := range r.activeVersions {
		for _, id := range v.Schedule.Binding.ExpectedMemberIDs {
			ids[id] = true
		}
	}
	ordered := make([]uint16, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	rows := make([]ScheduledMemberDeliveryCount, 0, len(ids))
	for _, id := range ordered {
		row := ScheduledMemberDeliveryCount{MemberID: id, FramesExpected: int(r.cfg.FrameCount), FramesLate: r.lateByMember[id]}
		if receiver := r.cfg.Receivers[id]; receiver != nil {
			counters := receiver.Counters()
			row.FramesAccepted = counters.FramesAccepted
			row.FramesPlayed = counters.PlayoutFramesEmitted
		}
		row.FramesOnTime = row.FramesAccepted - row.FramesLate
		row.FramesMissing = row.FramesExpected - row.FramesPlayed
		rows = append(rows, row)
	}
	return rows
}

func scheduledPlayoutSamples(events []ScheduledPlayoutEvent) []int64 {
	samples := make([]int64, 0, len(events))
	for _, event := range events {
		samples = append(samples, event.LatencyMS)
	}
	return samples
}

func scheduledFirstPlayout(events []ScheduledPlayoutEvent) *int64 {
	if len(events) == 0 {
		return nil
	}
	first := events[0].AtMS
	for _, event := range events {
		if event.AtMS < first {
			first = event.AtMS
		}
	}
	return &first
}

func scheduledAudioEvidence(result ScheduledRuntimeResult) ScheduledAudioEvidence {
	out := ScheduledAudioEvidence{Version: 2, ServiceOutcome: "pass", Members: append([]ScheduledMemberDeliveryCount(nil), result.MemberDelivery...)}
	if len(out.Members) == 0 {
		out.ServiceOutcome = "fail"
	}
	for _, m := range out.Members {
		if m.FramesPlayed == 0 || m.FramesPlayed != m.FramesExpected {
			out.ServiceOutcome = "fail"
		}
	}
	out.ArrivalLatency = scheduledOptionalLatency(scheduledRelayTreeFlattenedLatency(result.LatencyByRelayDepth))
	out.PlayoutLatency = scheduledOptionalLatency(scheduledPlayoutSamples(result.PlayoutEvents))
	return out
}

func scheduledOptionalLatency(samples []int64) *RelayDepthLatencyReport {
	if len(samples) == 0 {
		return nil
	}
	rows := scheduledRelayTreeLatencyReport([]ScheduledRelayDepthLatencySamples{{SamplesMS: samples}})
	return &rows[0]
}
