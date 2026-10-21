package scenario

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

const maxTimingDriftPPM int32 = 1_000_000

func validateTimingProfileComparisonFixtures(fixtures []TimingProfileComparisonFixture) error {
	if len(fixtures) == 0 {
		return fmt.Errorf("timing profile comparison: at least one fixture is required")
	}
	seen := make(map[string]struct{}, len(fixtures))
	for i := range fixtures {
		fixture := fixtures[i]
		if fixture.FixtureID == "" || strings.TrimSpace(fixture.FixtureID) != fixture.FixtureID || strings.Contains(fixture.FixtureID, "/") {
			return fmt.Errorf("timing profile comparison: fixture[%d] fixture_id=%q is invalid", i, fixture.FixtureID)
		}
		if _, ok := seen[fixture.FixtureID]; ok {
			return fmt.Errorf("timing profile comparison: duplicate fixture_id=%q", fixture.FixtureID)
		}
		seen[fixture.FixtureID] = struct{}{}
		if err := validateScheduledRelayTreeScenarioConfig(fixture.BaseConfig); err != nil {
			return fmt.Errorf("timing profile comparison: fixture %q base config: %w", fixture.FixtureID, err)
		}
		if err := validateTimingProfileOverlay(fixture.BaseConfig, fixture.TimingOverlay); err != nil {
			return fmt.Errorf("timing profile comparison: fixture %q: %w", fixture.FixtureID, err)
		}
	}
	return nil
}

func validateTimingProfileOverlay(cfg ScheduledRelayTreeScenarioConfig, overlay TimingProfileComparisonOverlay) error {
	if overlay.MinTimingBatteryPct > 100 {
		return fmt.Errorf("min_timing_battery_pct=%d must be <= 100", overlay.MinTimingBatteryPct)
	}
	if overlay.SyncToleranceMS < 0 {
		return fmt.Errorf("sync_tolerance_ms=%d must be >= 0", overlay.SyncToleranceMS)
	}
	if len(overlay.Peers) == 0 {
		return fmt.Errorf("timing peers must not be empty")
	}
	seen := make(map[uint16]struct{}, len(overlay.Peers))
	for i, peer := range overlay.Peers {
		if peer.StationID == 0 {
			return fmt.Errorf("timing peer[%d] station_id must be > 0", i)
		}
		if _, ok := seen[peer.StationID]; ok {
			return fmt.Errorf("timing peers contain duplicate station_id=%d", peer.StationID)
		}
		seen[peer.StationID] = struct{}{}
		if peer.LastSeenMS > cfg.StartMS {
			return fmt.Errorf("timing peer %d last_seen_ms=%d is after evaluation start_ms=%d", peer.StationID, peer.LastSeenMS, cfg.StartMS)
		}
		if peer.BatteryPct > 100 {
			return fmt.Errorf("timing peer %d battery_pct=%d must be <= 100", peer.StationID, peer.BatteryPct)
		}
		if peer.DriftPPM < -maxTimingDriftPPM || peer.DriftPPM > maxTimingDriftPPM {
			return fmt.Errorf("timing peer %d drift_ppm=%d is outside [%d,%d]", peer.StationID, peer.DriftPPM, -maxTimingDriftPPM, maxTimingDriftPPM)
		}
		if peer.JoinAtMS != nil && *peer.JoinAtMS < cfg.StartMS {
			return fmt.Errorf("timing peer %d join_at_ms=%d is before start_ms=%d", peer.StationID, *peer.JoinAtMS, cfg.StartMS)
		}
	}
	leader, ok := electTimingAuthority(overlay, cfg.StartMS, 0)
	if !ok {
		return fmt.Errorf("no eligible timing authority at start_ms=%d", cfg.StartMS)
	}
	if overlay.ConfiguredLeaderID != 0 && overlay.ConfiguredLeaderID != leader {
		return fmt.Errorf("configured_leader_id=%d does not match deterministic leader=%d", overlay.ConfiguredLeaderID, leader)
	}
	if overlay.LeaderLoss != nil {
		loss := overlay.LeaderLoss
		if loss.AtMS < cfg.StartMS {
			return fmt.Errorf("leader loss at_ms=%d is before start_ms=%d", loss.AtMS, cfg.StartMS)
		}
		if loss.LostStationID != leader {
			return fmt.Errorf("leader loss station_id=%d must equal initial leader=%d", loss.LostStationID, leader)
		}
		if loss.LeaderReacquireMS < 0 {
			return fmt.Errorf("leader_reacquire_ms=%d must be >= 0", loss.LeaderReacquireMS)
		}
		if loss.AtMS > math.MaxInt64-loss.LeaderReacquireMS {
			return fmt.Errorf("leader loss reacquire time overflows int64")
		}
	}
	return nil
}

func electTimingAuthority(overlay TimingProfileComparisonOverlay, atMS int64, excludedID uint16) (uint16, bool) {
	eligible := make([]uint16, 0, len(overlay.Peers))
	for _, peer := range overlay.Peers {
		if peer.StationID == excludedID || !timingPeerEligible(peer, overlay.MinTimingBatteryPct, atMS) {
			continue
		}
		eligible = append(eligible, peer.StationID)
	}
	if len(eligible) == 0 {
		return 0, false
	}
	sort.Slice(eligible, func(i, j int) bool { return eligible[i] < eligible[j] })
	return eligible[0], true
}

func timingPeerEligible(peer TimingAuthorityPeer, minBatteryPct uint8, atMS int64) bool {
	if !peer.TimingCapable || peer.Busy || peer.BatteryPct < minBatteryPct || atMS < peer.LastSeenMS {
		return false
	}
	if peer.JoinAtMS != nil && atMS < *peer.JoinAtMS {
		return false
	}
	return uint64(atMS-peer.LastSeenMS) <= uint64(peer.FreshnessMS)
}

func checkedTimingDriftMS(elapsedMS int64, driftPPM int32) (int64, error) {
	if elapsedMS < 0 {
		return 0, fmt.Errorf("elapsed_ms=%d must be >= 0", elapsedMS)
	}
	ppm := int64(driftPPM)
	if ppm < 0 {
		ppm = -ppm
	}
	if ppm == 0 || elapsedMS == 0 {
		return 0, nil
	}
	if elapsedMS > math.MaxInt64/ppm {
		return 0, fmt.Errorf("timing drift multiplication overflows int64")
	}
	return elapsedMS * ppm / 1_000_000, nil
}
