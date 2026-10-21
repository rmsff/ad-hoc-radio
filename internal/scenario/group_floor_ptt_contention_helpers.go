package scenario

import (
	"fmt"
	"sort"
	"strings"
)

func normalizeGroupFloorPTTContentionConfig(cfg GroupFloorPTTContentionConfig) GroupFloorPTTContentionConfig {
	def := DefaultGroupFloorPTTContentionConfig()
	if cfg.ScenarioID == "" {
		cfg.ScenarioID = def.ScenarioID
	}
	if cfg.Seed == 0 {
		cfg.Seed = def.Seed
	}
	if cfg.FloorPolicy == "" {
		cfg.FloorPolicy = def.FloorPolicy
	}
	if cfg.AccessDecisionMs == 0 {
		cfg.AccessDecisionMs = def.AccessDecisionMs
	}
	if cfg.FloorReleaseGuardMs == 0 {
		cfg.FloorReleaseGuardMs = def.FloorReleaseGuardMs
	}
	if cfg.FirstAudioWindowMs == 0 {
		cfg.FirstAudioWindowMs = def.FirstAudioWindowMs
	}
	if cfg.Attempts == nil {
		cfg.Attempts = cloneGroupFloorPTTAttempts(def.Attempts)
	} else {
		cfg.Attempts = cloneGroupFloorPTTAttempts(cfg.Attempts)
	}
	return cfg
}

func validateGroupFloorPTTContentionConfig(cfg GroupFloorPTTContentionConfig) error {
	if cfg.ScenarioID != GroupFloorPTTContentionScenarioID {
		return fmt.Errorf("group floor ptt contention: unsupported scenario_id %q", cfg.ScenarioID)
	}
	if cfg.FloorPolicy != GroupFloorPolicyFirstArrivalWins {
		return fmt.Errorf("group floor ptt contention: unsupported floor_policy %q", cfg.FloorPolicy)
	}
	if cfg.AccessDecisionMs <= 0 || cfg.FirstAudioWindowMs <= 0 {
		return fmt.Errorf("group floor ptt contention: access_decision_ms and first_audio_window_ms must be > 0")
	}
	if cfg.FloorReleaseGuardMs < 0 {
		return fmt.Errorf("group floor ptt contention: floor_release_guard_ms must be >= 0")
	}
	if len(cfg.Attempts) < 2 || len(cfg.Attempts) > groupFloorPTTMaxAttempts {
		return fmt.Errorf("group floor ptt contention: attempt count must be in [2,%d]", groupFloorPTTMaxAttempts)
	}
	seen := make(map[string]struct{}, len(cfg.Attempts))
	for i, attempt := range cfg.Attempts {
		if strings.TrimSpace(attempt.EventID) == "" {
			return fmt.Errorf("group floor ptt contention: attempts[%d].event_id must not be empty", i)
		}
		if _, ok := seen[attempt.EventID]; ok {
			return fmt.Errorf("group floor ptt contention: duplicate event_id %q", attempt.EventID)
		}
		seen[attempt.EventID] = struct{}{}
		if attempt.SourceID == 0 {
			return fmt.Errorf("group floor ptt contention: attempts[%d].source_id must not be zero", i)
		}
		if attempt.PTTStartMs < 0 {
			return fmt.Errorf("group floor ptt contention: attempts[%d].ptt_start_ms must be >= 0", i)
		}
		if attempt.TalkDurationMs <= 0 {
			return fmt.Errorf("group floor ptt contention: attempts[%d].talk_duration_ms must be > 0", i)
		}
	}
	return nil
}

func sortedGroupFloorPTTAttempts(attempts []GroupFloorPTTAttemptConfig) []GroupFloorPTTAttemptConfig {
	out := cloneGroupFloorPTTAttempts(attempts)
	sort.Slice(out, func(i, j int) bool {
		if out[i].PTTStartMs != out[j].PTTStartMs {
			return out[i].PTTStartMs < out[j].PTTStartMs
		}
		if out[i].SourceID != out[j].SourceID {
			return out[i].SourceID < out[j].SourceID
		}
		return out[i].EventID < out[j].EventID
	})
	return out
}

func groupFloorPTTBlockingFloor(attempt GroupFloorPTTAttemptConfig, activeFloors []groupFloorPTTActiveFloor) (groupFloorPTTActiveFloor, bool) {
	var blocking groupFloorPTTActiveFloor
	found := false
	for _, active := range activeFloors {
		if attempt.PTTStartMs >= active.StartMs && attempt.PTTStartMs < active.GuardEnd {
			if !found || active.StartMs > blocking.StartMs || (active.StartMs == blocking.StartMs && active.Sequence > blocking.Sequence) {
				blocking = active
				found = true
			}
		}
	}
	return blocking, found
}

func groupFloorPTTClippedFirstAudio(cfg GroupFloorPTTContentionConfig) int64 {
	if cfg.AccessDecisionMs < cfg.FirstAudioWindowMs {
		return cfg.AccessDecisionMs
	}
	return cfg.FirstAudioWindowMs
}

func groupFloorPTTNearestRank(samples []int64, percentile int) int64 {
	if len(samples) == 0 {
		return 0
	}
	ordered := append([]int64(nil), samples...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	index := (percentile*len(ordered) + 99) / 100
	if index < 1 {
		index = 1
	}
	if index > len(ordered) {
		index = len(ordered)
	}
	return ordered[index-1]
}

func groupFloorPTTDuplicatePlaybackCount(rows []GroupFloorPTTAttemptRow) int {
	count := 0
	for i := range rows {
		if !rows[i].SuccessfulSpeaker {
			continue
		}
		for j := i + 1; j < len(rows); j++ {
			if !rows[j].SuccessfulSpeaker {
				continue
			}
			if groupFloorPTTFloorsOverlap(rows[i], rows[j]) {
				count++
			}
		}
	}
	return count
}

func groupFloorPTTFloorsOverlap(a GroupFloorPTTAttemptRow, b GroupFloorPTTAttemptRow) bool {
	return a.FloorStartMs < b.FloorEndMs && b.FloorStartMs < a.FloorEndMs
}

func cloneGroupFloorPTTAttempts(values []GroupFloorPTTAttemptConfig) []GroupFloorPTTAttemptConfig {
	return append([]GroupFloorPTTAttemptConfig(nil), values...)
}

func cloneGroupFloorPTTStrings(values []string) []string {
	return append([]string(nil), values...)
}
