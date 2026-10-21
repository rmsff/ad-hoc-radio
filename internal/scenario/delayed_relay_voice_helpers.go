package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

func normalizeDelayedRelayVoiceConfig(cfg DelayedRelayVoiceConfig) DelayedRelayVoiceConfig {
	def := DefaultDelayedRelayVoiceConfig()
	if cfg.ScenarioID == "" {
		cfg.ScenarioID = def.ScenarioID
	}
	if cfg.Seed == 0 {
		cfg.Seed = def.Seed
	}
	if cfg.ChannelBitrateBpsCandidates == nil {
		cfg.ChannelBitrateBpsCandidates = cloneMultiHopTimingInt64s(def.ChannelBitrateBpsCandidates)
	} else {
		cfg.ChannelBitrateBpsCandidates = sortedMultiHopTimingInt64s(cfg.ChannelBitrateBpsCandidates)
	}
	if cfg.FrameBytesCandidates == nil {
		cfg.FrameBytesCandidates = []int{22, 56}
	} else {
		cfg.FrameBytesCandidates = sortedDelayedRelayVoiceInts(cfg.FrameBytesCandidates)
	}
	if cfg.BurstDurationMsCandidates == nil {
		cfg.BurstDurationMsCandidates = cloneMultiHopTimingInt64s(def.BurstDurationMsCandidates)
	} else {
		cfg.BurstDurationMsCandidates = sortedMultiHopTimingInt64s(cfg.BurstDurationMsCandidates)
	}
	if cfg.RelayDepthCandidates == nil {
		cfg.RelayDepthCandidates = []int{1, 2, 3}
	} else {
		cfg.RelayDepthCandidates = sortedDelayedRelayVoiceInts(cfg.RelayDepthCandidates)
	}
	if cfg.TurnaroundMs == 0 {
		cfg.TurnaroundMs = def.TurnaroundMs
	}
	if cfg.PreambleMs == 0 {
		cfg.PreambleMs = def.PreambleMs
	}
	if cfg.SyncMs == 0 {
		cfg.SyncMs = def.SyncMs
	}
	if cfg.LinkDelayMs == 0 {
		cfg.LinkDelayMs = def.LinkDelayMs
	}
	if cfg.InterFrameGapMs == 0 {
		cfg.InterFrameGapMs = def.InterFrameGapMs
	}
	if cfg.VoicePeriodMs == 0 {
		cfg.VoicePeriodMs = def.VoicePeriodMs
	}
	if cfg.LatencyBudgetMs == 0 {
		cfg.LatencyBudgetMs = def.LatencyBudgetMs
	}
	if cfg.LiveTurnaroundBudgetMs == 0 {
		cfg.LiveTurnaroundBudgetMs = def.LiveTurnaroundBudgetMs
	}
	return cfg
}

func validateDelayedRelayVoiceConfig(cfg DelayedRelayVoiceConfig) error {
	if cfg.ScenarioID == "" {
		return fmt.Errorf("delayed relay voice: scenario_id must not be empty")
	}
	if cfg.ScenarioID != DelayedRelayVoiceScenarioID {
		return fmt.Errorf("delayed relay voice: unsupported scenario_id %q", cfg.ScenarioID)
	}
	if err := validatePositiveDelayedRelayVoiceInt64s("channel_bitrate_bps_candidates", cfg.ChannelBitrateBpsCandidates); err != nil {
		return err
	}
	if err := validatePositiveDelayedRelayVoiceInts("frame_bytes_candidates", cfg.FrameBytesCandidates); err != nil {
		return err
	}
	if err := validatePositiveDelayedRelayVoiceInt64s("burst_duration_ms_candidates", cfg.BurstDurationMsCandidates); err != nil {
		return err
	}
	if err := validatePositiveDelayedRelayVoiceInts("relay_depth_candidates", cfg.RelayDepthCandidates); err != nil {
		return err
	}
	if cfg.TurnaroundMs < 0 {
		return fmt.Errorf("delayed relay voice: turnaround_ms=%d must be >= 0", cfg.TurnaroundMs)
	}
	if cfg.PreambleMs < 0 {
		return fmt.Errorf("delayed relay voice: preamble_ms=%d must be >= 0", cfg.PreambleMs)
	}
	if cfg.SyncMs < 0 {
		return fmt.Errorf("delayed relay voice: sync_ms=%d must be >= 0", cfg.SyncMs)
	}
	if cfg.LinkDelayMs < 0 {
		return fmt.Errorf("delayed relay voice: link_delay_ms=%d must be >= 0", cfg.LinkDelayMs)
	}
	if cfg.InterFrameGapMs < 0 {
		return fmt.Errorf("delayed relay voice: inter_frame_gap_ms=%d must be >= 0", cfg.InterFrameGapMs)
	}
	if cfg.VoicePeriodMs <= 0 {
		return fmt.Errorf("delayed relay voice: voice_period_ms=%d must be > 0", cfg.VoicePeriodMs)
	}
	if cfg.LatencyBudgetMs <= 0 {
		return fmt.Errorf("delayed relay voice: latency_budget_ms=%d must be > 0", cfg.LatencyBudgetMs)
	}
	if cfg.LiveTurnaroundBudgetMs < 0 {
		return fmt.Errorf("delayed relay voice: live_turnaround_budget_ms=%d must be >= 0", cfg.LiveTurnaroundBudgetMs)
	}
	if delayedRelayVoiceRowCount(cfg) > delayedRelayVoiceMaxRows {
		return fmt.Errorf("delayed relay voice: row count would exceed cap %d", delayedRelayVoiceMaxRows)
	}
	return nil
}

func delayedRelayVoiceRelayTimeline(sourceCaptureComplete int64, relayDepth int, turnaroundMs, burstTxDuration, frameTxDuration, linkDelayMs int64) (int64, int64, error) {
	fullAvailableAt := sourceCaptureComplete
	var firstAudioLatency int64
	var completion int64
	for hop := 0; hop < relayDepth; hop++ {
		txStart, err := checkedLimitedMultiHopAdd(fullAvailableAt, turnaroundMs)
		if err != nil {
			return 0, 0, err
		}
		firstRx, err := checkedLimitedMultiHopAdd(txStart, frameTxDuration)
		if err != nil {
			return 0, 0, err
		}
		firstRx, err = checkedLimitedMultiHopAdd(firstRx, linkDelayMs)
		if err != nil {
			return 0, 0, err
		}
		fullRx, err := checkedLimitedMultiHopAdd(txStart, burstTxDuration)
		if err != nil {
			return 0, 0, err
		}
		fullRx, err = checkedLimitedMultiHopAdd(fullRx, linkDelayMs)
		if err != nil {
			return 0, 0, err
		}
		firstAudioLatency = firstRx
		completion = fullRx
		fullAvailableAt = fullRx
	}
	return firstAudioLatency, completion, nil
}

func delayedRelayVoiceFrameCount(burstMs, voicePeriodMs int64) (int, error) {
	if burstMs <= 0 || voicePeriodMs <= 0 {
		return 0, fmt.Errorf("delayed relay voice: burst and voice period must be > 0")
	}
	count := burstMs / voicePeriodMs
	if burstMs%voicePeriodMs != 0 {
		count++
	}
	if count <= 0 || count > int64(delayedRelayVoiceMaxRows*DefaultFrameCount) {
		return 0, fmt.Errorf("delayed relay voice: derived frame_count=%d is out of range", count)
	}
	return int(count), nil
}

func delayedRelayVoiceBurstTxDuration(frameCount int, frameTxDuration, interFrameGapMs int64) (int64, error) {
	framesDuration, err := checkedLimitedMultiHopMul(int64(frameCount), frameTxDuration)
	if err != nil {
		return 0, err
	}
	if frameCount <= 1 {
		return framesDuration, nil
	}
	gapsDuration, err := checkedLimitedMultiHopMul(int64(frameCount-1), interFrameGapMs)
	if err != nil {
		return 0, err
	}
	return checkedLimitedMultiHopAdd(framesDuration, gapsDuration)
}

func delayedRelayVoiceChannelOccupiedMs(frameCount int, frameTxDuration int64, relayDepth int, burstTxDuration int64) (int64, error) {
	sourceOccupied, err := checkedLimitedMultiHopMul(int64(frameCount), frameTxDuration)
	if err != nil {
		return 0, err
	}
	relayOccupied, err := checkedLimitedMultiHopMul(int64(relayDepth), burstTxDuration)
	if err != nil {
		return 0, err
	}
	return checkedLimitedMultiHopAdd(sourceOccupied, relayOccupied)
}

func delayedRelayVoiceFailReasons(cfg DelayedRelayVoiceConfig, row DelayedRelayVoiceRow) []string {
	reasons := []string{delayedRelayVoiceReasonNotLive}
	if !row.LiveTwoHopViable {
		reasons = append(reasons, delayedRelayVoiceReasonLiveTurnaroundBad)
	}
	if !row.BurstDrainFasterThanSpeech {
		reasons = append(reasons, delayedRelayVoiceReasonDrainSlower)
	}
	if row.MaxUserPerceivedLatencyMs > cfg.LatencyBudgetMs {
		reasons = append(reasons, delayedRelayVoiceReasonLatencyBudget)
	}
	return reasons
}

func delayedRelayVoiceOutcome(reasons []string, storeForwardRequired bool) string {
	for _, reason := range reasons {
		if reason == delayedRelayVoiceReasonLatencyBudget || reason == delayedRelayVoiceReasonDrainSlower {
			return DelayedRelayVoiceOutcomeFail
		}
	}
	if storeForwardRequired {
		return DelayedRelayVoiceOutcomeDegraded
	}
	return DelayedRelayVoiceOutcomePass
}

func bestDelayedRelayVoiceRow(rows []DelayedRelayVoiceRow, relayDepth int) DelayedRelayVoiceRow {
	var best DelayedRelayVoiceRow
	for _, row := range rows {
		if row.RelayDepth != relayDepth || row.Outcome == DelayedRelayVoiceOutcomeFail {
			continue
		}
		if best.RowID == "" ||
			row.MaxUserPerceivedLatencyMs < best.MaxUserPerceivedLatencyMs ||
			(row.MaxUserPerceivedLatencyMs == best.MaxUserPerceivedLatencyMs && row.BurstDurationMs < best.BurstDurationMs) ||
			(row.MaxUserPerceivedLatencyMs == best.MaxUserPerceivedLatencyMs && row.BurstDurationMs == best.BurstDurationMs && row.FrameBytes < best.FrameBytes) ||
			(row.MaxUserPerceivedLatencyMs == best.MaxUserPerceivedLatencyMs && row.BurstDurationMs == best.BurstDurationMs && row.FrameBytes == best.FrameBytes && row.ChannelBitrateBps > best.ChannelBitrateBps) {
			best = row
		}
	}
	return best
}

func delayedRelayVoicePermille(numerator, denominator int64) int64 {
	if denominator <= 0 {
		return 0
	}
	return (numerator*1000 + denominator/2) / denominator
}

func delayedRelayVoiceRowID(bitrate int64, frameBytes int, burstMs int64, relayDepth int) string {
	return fmt.Sprintf("bitrate_%d_frame_%dB_burst_%dms_relays_%d", bitrate, frameBytes, burstMs, relayDepth)
}

func validatePositiveDelayedRelayVoiceInt64s(name string, values []int64) error {
	if len(values) == 0 {
		return fmt.Errorf("delayed relay voice: %s must not be empty", name)
	}
	seen := make(map[int64]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			return fmt.Errorf("delayed relay voice: %s contains %d, want > 0", name, value)
		}
		if _, ok := seen[value]; ok {
			return fmt.Errorf("delayed relay voice: %s contains duplicate %d", name, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validatePositiveDelayedRelayVoiceInts(name string, values []int) error {
	if len(values) == 0 {
		return fmt.Errorf("delayed relay voice: %s must not be empty", name)
	}
	seen := make(map[int]struct{}, len(values))
	for _, value := range values {
		if value <= 0 {
			return fmt.Errorf("delayed relay voice: %s contains %d, want > 0", name, value)
		}
		if _, ok := seen[value]; ok {
			return fmt.Errorf("delayed relay voice: %s contains duplicate %d", name, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func sortedDelayedRelayVoiceInts(values []int) []int {
	out := append([]int(nil), values...)
	sort.Ints(out)
	return out
}

func delayedRelayVoiceRowCount(cfg DelayedRelayVoiceConfig) int {
	return len(cfg.ChannelBitrateBpsCandidates) *
		len(cfg.FrameBytesCandidates) *
		len(cfg.BurstDurationMsCandidates) *
		len(cfg.RelayDepthCandidates)
}

func delayedRelayVoiceHash(report DelayedRelayVoiceReport) (string, error) {
	hashable := report
	hashable.EventHash = ""
	b, err := json.Marshal(hashable)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
