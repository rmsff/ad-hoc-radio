package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/rmsff/ad-hoc-radio/internal/node"
	"github.com/rmsff/ad-hoc-radio/internal/peer"
	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

func (r *scheduledRuntime) result() (ScheduledRuntimeResult, error) {
	latest := r.activeVersions[len(r.activeVersions)-1]
	treeHash, err := routing.RelayTreeHash(latest.Tree)
	if err != nil {
		return ScheduledRuntimeResult{}, fmt.Errorf("scheduled runtime: tree hash: %w", err)
	}
	scheduleHash, err := timing.OpportunityScheduleHash(latest.Schedule)
	if err != nil {
		return ScheduledRuntimeResult{}, fmt.Errorf("scheduled runtime: schedule hash: %w", err)
	}
	eventHash, err := r.queue.EventHash()
	if err != nil {
		return ScheduledRuntimeResult{}, fmt.Errorf("scheduled runtime: event hash: %w", err)
	}
	receiverCounters := make(map[uint16]node.ReceiverCounters, len(r.cfg.Receivers))
	delivered := make([]uint16, 0, len(latest.Schedule.CoveredMemberIDs))
	for _, id := range sortedScheduledReceiverIDs(r.cfg.Receivers) {
		counters := r.cfg.Receivers[id].Counters()
		receiverCounters[id] = counters
		if counters.PlayoutFramesEmitted > 0 && slicesContainsUint16(latest.Schedule.CoveredMemberIDs, id) {
			delivered = append(delivered, id)
		}
	}
	result := ScheduledRuntimeResult{
		PlayoutEvents:           append([]ScheduledPlayoutEvent(nil), r.playoutEvents...),
		MemberDelivery:          r.memberDelivery(),
		ExpectedMemberIDs:       append([]uint16(nil), latest.Schedule.Binding.ExpectedMemberIDs...),
		CoveredMemberIDs:        append([]uint16(nil), latest.Schedule.CoveredMemberIDs...),
		DeliveredMemberIDs:      delivered,
		UncoveredMemberIDs:      append([]uint16(nil), latest.Schedule.UncoveredMemberIDs...),
		Counters:                cloneScheduledCounters(r.counters),
		ReceiverCounters:        receiverCounters,
		RelaySeenCacheEntries:   r.scheduledRelaySeenCacheEntries(),
		RelaySeenCacheEvictions: r.scheduledRelaySeenCacheEvictions(),
		ActiveTree:              cloneScheduledTree(latest.Tree),
		ActiveSchedule:          cloneOpportunitySchedule(latest.Schedule),
		LinkObservations:        r.scheduledLinkObservations(),
		LatencyByRelayDepth:     r.scheduledLatencyObservations(latest.Tree.MaxRelayDepth),
		TreeHash:                treeHash,
		ScheduleHash:            scheduleHash,
		EventHash:               eventHash,
		EventCount:              len(r.queue.Emitted()),
		SimDurationMS:           r.nowMS,
		BlockedFromSeq:          cloneUint32Pointer(r.blockedFromSeq),
	}
	if r.control != nil {
		stats := r.control.Counters()
		result.ControlCounters = &stats
	}
	if r.cfg.VoiceProfile != nil {
		raw, err := r.encodeScheduledWire(r.cfg.Source.EmitVoiceFrame(0))
		if err != nil {
			return ScheduledRuntimeResult{}, err
		}
		if r.cfg.Control != nil {
			raw, err = peer.BindVoice(latest.Schedule.Binding.ScheduleVersion, raw)
			if err != nil {
				return ScheduledRuntimeResult{}, err
			}
		}
		budget, err := r.cfg.VoiceProfile.Measure(raw)
		if err != nil {
			return ScheduledRuntimeResult{}, err
		}
		result.Wire = &ScheduledWireEvidence{Profile: *r.cfg.VoiceProfile, Budget: budget, EvidenceGrade: "synthetic_size_fixture", AudioQualityVerified: false, HardwareMeasured: false}
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return ScheduledRuntimeResult{}, fmt.Errorf("scheduled runtime: result hash encode: %w", err)
	}
	digest := sha256.Sum256(encoded)
	result.ResultHash = hex.EncodeToString(digest[:])
	return result, nil
}

func (r *scheduledRuntime) scheduledRelaySeenCacheEntries() int {
	total := 0
	for _, id := range sortedScheduledRelayIDs(r.cfg.Relays) {
		total += r.cfg.Relays[id].Lifecycle().SeenEntryCount()
	}
	return total
}

func (r *scheduledRuntime) scheduledRelaySeenCacheEvictions() int {
	total := 0
	for _, id := range sortedScheduledRelayIDs(r.cfg.Relays) {
		total += r.cfg.Relays[id].Lifecycle().SeenEvictionCount()
	}
	return total
}

func (r *scheduledRuntime) scheduledLinkObservations() []ScheduledLinkObservation {
	delays := map[ScheduledLinkKey]int64{}
	for _, version := range r.activeVersions {
		for _, edge := range version.Tree.Edges {
			key := ScheduledLinkKey{FromStationID: edge.ParentStationID, ToStationID: edge.ChildStationID}
			delays[key] = version.LinkDelayMS[key]
		}
	}
	keys := make([]ScheduledLinkKey, 0, len(delays))
	for key := range delays {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].FromStationID != keys[j].FromStationID {
			return keys[i].FromStationID < keys[j].FromStationID
		}
		return keys[i].ToStationID < keys[j].ToStationID
	})
	observations := make([]ScheduledLinkObservation, 0, len(keys))
	for _, key := range keys {
		observations = append(observations, ScheduledLinkObservation{
			DelaySumMS: r.linkDelaySumMS[key],
			Lost:       r.linkLost[key], Corrupted: r.linkCorrupted[key], Duplicated: r.linkDuplicated[key],
			FromStationID: key.FromStationID,
			ToStationID:   key.ToStationID,
			DelayMS:       delays[key],
			Deliveries:    r.linkDeliveries[key],
		})
	}
	return observations
}

func (r *scheduledRuntime) scheduledLatencyObservations(maxDepth uint8) []ScheduledRelayDepthLatencySamples {
	for _, version := range r.activeVersions {
		if version.Tree.MaxRelayDepth > maxDepth {
			maxDepth = version.Tree.MaxRelayDepth
		}
	}
	observations := make([]ScheduledRelayDepthLatencySamples, 0, int(maxDepth)+1)
	for depth := uint8(0); depth <= maxDepth; depth++ {
		observations = append(observations, ScheduledRelayDepthLatencySamples{
			RelayDepth: depth,
			SamplesMS:  append([]int64(nil), r.latencyByDepth[depth]...),
		})
		if depth == ^uint8(0) {
			break
		}
	}
	return observations
}
