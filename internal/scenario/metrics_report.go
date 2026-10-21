package scenario

import (
	"encoding/json"
	"math"
	"sort"
	"strings"

	"github.com/rmsff/ad-hoc-radio/internal/node"
)

const (
	metricsSchemaVersion = 1
	metricsHashAlgorithm = "sha256"
	metricsSerialization = "canonical-jsonl-v1"
	metricsStackVersion  = "go1.24"
	unknownScenarioID    = "unknown"
	emptyEventHash       = "0000000000000000000000000000000000000000000000000000000000000000"
	maxExactFloat64IntMs = int64(1 << 53)
)

// MetricsReport is the schema-shaped JSON report defined by
// docs/protocol/metrics-schema.json.
type MetricsReport struct {
	SchemaVersion       int                       `json:"schema_version"`
	ScenarioID          string                    `json:"scenario_id"`
	Seed                uint64                    `json:"seed"`
	SimDurationMs       int64                     `json:"sim_duration_ms"`
	Stack               StackReport               `json:"stack"`
	TwoHopLive          *TwoHopLiveReport         `json:"two_hop_live,omitempty"`
	ScheduledRelayTree  *ScheduledRelayTreeReport `json:"scheduled_relay_tree,omitempty"`
	SourceFramesCreated int                       `json:"source_frames_created"`
	SourceFramesSent    int                       `json:"source_frames_sent"`
	Links               []LinkReport              `json:"links"`
	Relay               RelayReport               `json:"relay"`
	Receiver            ReceiverReport            `json:"receiver"`
	LatencyMs           LatencyReport             `json:"latency_ms"`
	OutOfSlotTxCount    int                       `json:"out_of_slot_tx_count"`
	Determinism         DeterminismReport         `json:"determinism"`
	Outcome             string                    `json:"outcome"`
	Notes               string                    `json:"notes,omitempty"`
}

type StackReport struct {
	Language string `json:"language"`
	Version  string `json:"version"`
}

type TwoHopLiveReport struct {
	RelayPath             []uint16 `json:"relay_path"`
	SourceWireMode        string   `json:"source_wire_mode"`
	ChannelBitrateBps     int64    `json:"channel_bitrate_bps"`
	SourceFrameBytes      int      `json:"source_frame_bytes"`
	RelayFrameBytes       int      `json:"relay_frame_bytes"`
	SourceSlotMs          int64    `json:"source_slot_ms"`
	RelaySlotMs           int64    `json:"relay_slot_ms"`
	GuardTimeMs           int64    `json:"guard_time_ms"`
	LinkDelayMs           int64    `json:"link_delay_ms"`
	SourceAirtimeMs       int64    `json:"source_airtime_ms"`
	RelayAirtimeMs        int64    `json:"relay_airtime_ms"`
	EndToEndLatencyMs     int64    `json:"end_to_end_latency_ms"`
	CycleOccupiedMs       int64    `json:"cycle_occupied_ms"`
	CycleSpareMs          int64    `json:"cycle_spare_ms"`
	SlotMissCount         int      `json:"slot_miss_count"`
	RelayReadinessMisses  int      `json:"relay_readiness_miss_count"`
	Relay1FramesForwarded int      `json:"relay1_frames_forwarded"`
	Relay2FramesForwarded int      `json:"relay2_frames_forwarded"`
}

type LinkReport struct {
	From       string   `json:"from"`
	To         string   `json:"to"`
	Delivered  int      `json:"delivered"`
	Lost       int      `json:"lost"`
	Corrupted  int      `json:"corrupted"`
	Duplicated int      `json:"duplicated"`
	AvgDelayMs *float64 `json:"avg_delay_ms,omitempty"`
}

type RelayReport struct {
	FramesReceived       int                     `json:"frames_received"`
	FramesForwarded      int                     `json:"frames_forwarded"`
	DroppedByReason      map[node.DropReason]int `json:"dropped_by_reason"`
	SeenCacheEntries     int                     `json:"seen_cache_entries"`
	SeenCacheEvictions   int                     `json:"seen_cache_evictions"`
	ForwardQueueMaxDepth int                     `json:"forward_queue_max_depth"`
	ForwardQueueDrops    int                     `json:"forward_queue_drops"`
}

type ReceiverReport struct {
	FramesReceived        int                     `json:"frames_received"`
	FramesAccepted        int                     `json:"frames_accepted"`
	DuplicatesSuppressed  int                     `json:"duplicates_suppressed"`
	LateFrames            int                     `json:"late_frames"`
	MissingSequences      int                     `json:"missing_sequences"`
	PlayoutFramesEmitted  int                     `json:"playout_frames_emitted"`
	DirectFramesAccepted  int                     `json:"direct_frames_accepted"`
	RelayedFramesAccepted int                     `json:"relayed_frames_accepted"`
	DroppedByReason       map[node.DropReason]int `json:"dropped_by_reason"`
}

type LatencyReport struct {
	Min                float64 `json:"min"`
	Avg                float64 `json:"avg"`
	P50                float64 `json:"p50"`
	P95                float64 `json:"p95"`
	Max                float64 `json:"max"`
	SourceToRelayAvg   float64 `json:"source_to_relay_avg"`
	RelayToReceiverAvg float64 `json:"relay_to_receiver_avg"`
	PlayoutAvg         float64 `json:"playout_avg"`
}

type DeterminismReport struct {
	EventCount    int    `json:"event_count"`
	EventHash     string `json:"event_hash"`
	HashAlgorithm string `json:"hash_algorithm"`
	Serialization string `json:"serialization"`
}

// Report converts collected ScenarioMetrics into the canonical metrics
// JSON model. It does not use wall-clock time.
func (m ScenarioMetrics) Report() MetricsReport {
	return MetricsReport{
		SchemaVersion:       metricsSchemaVersion,
		ScenarioID:          reportScenarioID(m.ScenarioID),
		Seed:                m.Seed,
		SimDurationMs:       m.SimDurationMs,
		Stack:               StackReport{Language: "go", Version: metricsStackVersion},
		TwoHopLive:          cloneTwoHopLiveReport(m.TwoHopLive),
		ScheduledRelayTree:  cloneScheduledRelayTreeReport(m.ScheduledRelayTree),
		SourceFramesCreated: m.SourceFramesCreated,
		SourceFramesSent:    m.SourceFramesSent,
		Links:               m.linkReports(),
		Relay: RelayReport{
			FramesReceived:       m.RelayFramesReceived,
			FramesForwarded:      m.RelayForwarded,
			DroppedByReason:      cloneDropCounters(m.RelayDroppedByReason),
			SeenCacheEntries:     m.RelaySeenCacheEntries,
			SeenCacheEvictions:   m.RelaySeenCacheEvictions,
			ForwardQueueMaxDepth: m.RelayForwardQueueMaxDepth,
			ForwardQueueDrops:    m.RelayForwardQueueDrops,
		},
		Receiver: ReceiverReport{
			FramesReceived:        m.ReceiverCounters.FramesReceived,
			FramesAccepted:        m.ReceiverCounters.FramesAccepted,
			DuplicatesSuppressed:  m.ReceiverCounters.DuplicatesSuppressed,
			LateFrames:            m.ReceiverCounters.LateFrames,
			MissingSequences:      m.ReceiverCounters.MissingSequences,
			PlayoutFramesEmitted:  m.ReceiverCounters.PlayoutFramesEmitted,
			DirectFramesAccepted:  m.ReceiverCounters.DirectFramesAccepted,
			RelayedFramesAccepted: m.ReceiverCounters.RelayedFramesAccepted,
			DroppedByReason:       cloneDropCounters(m.ReceiverCounters.DroppedByReason),
		},
		LatencyMs:        m.latencyReport(),
		OutOfSlotTxCount: m.OutOfSlotTxCount,
		Determinism: DeterminismReport{
			EventCount:    m.EventCount,
			EventHash:     reportEventHash(m.EventHash),
			HashAlgorithm: metricsHashAlgorithm,
			Serialization: metricsSerialization,
		},
		Outcome: m.outcome(),
		Notes:   m.Notes,
	}
}

// MarshalReportJSON exports the schema-shaped metrics report as stable
// JSON. encoding/json emits map keys deterministically for string-keyed
// maps, including DropReason keys.
func (m ScenarioMetrics) MarshalReportJSON() ([]byte, error) {
	return json.MarshalIndent(m.Report(), "", "  ")
}

func reportScenarioID(id string) string {
	if id == "" {
		return unknownScenarioID
	}
	return id
}

func reportEventHash(hash string) string {
	if hash == "" {
		return emptyEventHash
	}
	return hash
}

func (m ScenarioMetrics) linkReports() []LinkReport {
	keys := m.defaultLinkReportKeys()
	for key := range m.LinkCounters.Delivered {
		keys[key] = struct{}{}
	}
	for key := range m.LinkCounters.Lost {
		keys[key] = struct{}{}
	}
	for key := range m.LinkCounters.Corrupted {
		keys[key] = struct{}{}
	}
	for key := range m.LinkCounters.Duplicated {
		keys[key] = struct{}{}
	}
	for key := range m.LinkDelayCounts {
		keys[key] = struct{}{}
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)

	out := make([]LinkReport, 0, len(ordered))
	for _, key := range ordered {
		report, ok := m.linkReport(key)
		if ok {
			out = append(out, report)
		}
	}
	return out
}

func (m ScenarioMetrics) defaultLinkReportKeys() map[string]struct{} {
	keys := make(map[string]struct{})
	if len(m.LinkReportDefaults) > 0 {
		for _, link := range m.LinkReportDefaults {
			if link.From == 0 || link.To == 0 {
				continue
			}
			keys[linkReportKey(link.From, link.To)] = struct{}{}
		}
		return keys
	}
	if m.SourceID != 0 && m.RelayID != 0 {
		keys[linkReportKey(m.SourceID, m.RelayID)] = struct{}{}
	}
	if m.RelayID != 0 && m.ReceiverID != 0 {
		keys[linkReportKey(m.RelayID, m.ReceiverID)] = struct{}{}
	}
	if m.SourceID != 0 && m.ReceiverID != 0 {
		keys[linkReportKey(m.SourceID, m.ReceiverID)] = struct{}{}
	}
	return keys
}

func (m ScenarioMetrics) linkReport(key string) (LinkReport, bool) {
	from, to, ok := splitLinkReportKey(key)
	if !ok {
		return LinkReport{}, false
	}
	report := LinkReport{
		From:       from,
		To:         to,
		Delivered:  m.LinkCounters.Delivered[key],
		Lost:       m.LinkCounters.Lost[key],
		Corrupted:  m.LinkCounters.Corrupted[key],
		Duplicated: m.LinkCounters.Duplicated[key],
	}
	if count := m.LinkDelayCounts[key]; count > 0 {
		avg := float64(m.LinkDelaySumsMs[key]) / float64(count)
		report.AvgDelayMs = &avg
	}
	return report, true
}

func splitLinkReportKey(key string) (string, string, bool) {
	parts := strings.Split(key, "->")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func (m ScenarioMetrics) latencyReport() LatencyReport {
	samples := m.latencySamples()
	primary := summarize(samples.playoutLatencyMs)
	return LatencyReport{
		Min:                primary.Min,
		Avg:                primary.Avg,
		P50:                primary.P50,
		P95:                primary.P95,
		Max:                primary.Max,
		SourceToRelayAvg:   average(samples.sourceToRelayMs),
		RelayToReceiverAvg: average(samples.relayToReceiverMs),
		PlayoutAvg:         average(samples.playoutWaitMs),
	}
}

type latencySamples struct {
	playoutLatencyMs  []float64
	sourceToRelayMs   []float64
	relayToReceiverMs []float64
	playoutWaitMs     []float64
}

func (m ScenarioMetrics) latencySamples() latencySamples {
	if len(m.PlayoutLatencySamplesMs) > 0 {
		if !sameLen(m.PlayoutLatencySamplesMs, m.SourceToRelaySamplesMs, m.RelayToReceiverSamplesMs, m.PlayoutWaitSamplesMs) {
			return latencySamples{}
		}
		return latencySamples{
			playoutLatencyMs:  nonNegativeInt64sToFloat64s(m.PlayoutLatencySamplesMs),
			sourceToRelayMs:   nonNegativeInt64sToFloat64s(m.SourceToRelaySamplesMs),
			relayToReceiverMs: nonNegativeInt64sToFloat64s(m.RelayToReceiverSamplesMs),
			playoutWaitMs:     nonNegativeInt64sToFloat64s(m.PlayoutWaitSamplesMs),
		}
	}
	return latencySamples{}
}

func sameLen(primary []int64, others ...[]int64) bool {
	for _, other := range others {
		if len(other) != len(primary) {
			return false
		}
	}
	return true
}

type latencySummary struct {
	Min float64
	Avg float64
	P50 float64
	P95 float64
	Max float64
}

func summarize(samples []float64) latencySummary {
	if len(samples) == 0 {
		return latencySummary{}
	}
	sorted := append([]float64(nil), samples...)
	sort.Float64s(sorted)
	return latencySummary{
		Min: sorted[0],
		Avg: average(sorted),
		P50: percentile(sorted, 50),
		P95: percentile(sorted, 95),
		Max: sorted[len(sorted)-1],
	}
}

func average(samples []float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, v := range samples {
		sum += v
	}
	return sum / float64(len(samples))
}

func nonNegativeInt64sToFloat64s(in []int64) []float64 {
	if in == nil {
		return nil
	}
	out := make([]float64, len(in))
	for i, v := range in {
		out[i] = float64(clampLatencyMs(v))
	}
	return out
}

func clampLatencyMs(v int64) int64 {
	if v < 0 {
		return 0
	}
	if v > maxExactFloat64IntMs {
		return maxExactFloat64IntMs
	}
	return v
}

func percentile(sorted []float64, pct float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil((pct/100)*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func (m ScenarioMetrics) outcome() string {
	switch m.OutcomeOverride {
	case "pass", "fail", "inconclusive":
		return m.OutcomeOverride
	}
	if m.SourceFramesCreated == 0 {
		return "inconclusive"
	}
	if m.ReceiverCounters.LateFrames == 0 &&
		m.ReceiverCounters.MissingSequences == 0 &&
		m.ReceiverCounters.FramesAccepted == m.SourceFramesCreated &&
		m.ReceiverCounters.PlayoutFramesEmitted == m.SourceFramesCreated {
		return "pass"
	}
	return "fail"
}

func cloneDropCounters(in map[node.DropReason]int) map[node.DropReason]int {
	out := make(map[node.DropReason]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneTwoHopLiveReport(in *TwoHopLiveReport) *TwoHopLiveReport {
	if in == nil {
		return nil
	}
	out := *in
	if in.RelayPath != nil {
		out.RelayPath = make([]uint16, len(in.RelayPath))
		copy(out.RelayPath, in.RelayPath)
	}
	return &out
}
