package scenario

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/rmsff/ad-hoc-radio/internal/protocol"
	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
)

func TestScenarioConfigFramePeriodBound(t *testing.T) {
	accepted := DefaultScenarioConfig()
	accepted.FramePeriodMs = protocol.MaxFramePeriodMs
	NewScenarioRunner(accepted)

	rejected := DefaultScenarioConfig()
	rejected.FramePeriodMs = protocol.MaxFramePeriodMs + 1
	defer func() {
		recovered := recover()
		if recovered == nil || !strings.Contains(fmt.Sprint(recovered), "FramePeriodMs=60001 must be in 1..60000") {
			t.Fatalf("panic = %v, want frame period bound", recovered)
		}
	}()
	NewScenarioRunner(rejected)
}

func TestGroupVoiceProfileVoicePeriodBound(t *testing.T) {
	tests := []struct {
		period  int64
		wantErr string
	}{
		{80, ""},
		{protocol.MaxFramePeriodMs, ""},
		{protocol.MaxFramePeriodMs + 1, "voice_period_ms=60001 must be <= 60000"},
		{math.MaxInt64, "must be <= 60000"},
	}
	for _, tc := range tests {
		cfg := DefaultGroupVoiceProfileScenarioConfig()
		for i := range cfg.Profiles {
			cfg.Profiles[i].VoicePeriodMs = tc.period
		}
		err := validateGroupVoiceProfileScenarioConfig(cfg)
		if tc.wantErr == "" && err != nil || tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
			t.Fatalf("voice_period_ms=%d: error = %v, want %q", tc.period, err, tc.wantErr)
		}
	}
}

func TestScheduledRelayTreeProfileFramePeriodBound(t *testing.T) {
	profile := timing.OpportunityProfile{SourceDurationMS: 10, RelayDurationMS: 10, GuardAfterMS: 1, MaxCycleMS: 80}
	tests := []struct {
		period  int64
		wantErr bool
	}{
		{80, false},
		{protocol.MaxFramePeriodMs, false},
		{protocol.MaxFramePeriodMs + 1, true},
		{math.MaxUint32, true},
		{0, true},
	}
	for _, tc := range tests {
		cost := routing.PlanningCostModel{SourceOpportunityMS: 10, RelayOpportunityMS: 10, GuardAfterMS: 1, FramePeriodMS: tc.period}
		err := validateScheduledRelayTreeProfile(cost, profile)
		if (err != nil) != tc.wantErr {
			t.Fatalf("frame_period_ms=%d: error = %v, wantErr %v", tc.period, err, tc.wantErr)
		}
		if tc.wantErr && tc.period > 0 && !strings.Contains(err.Error(), "must be in 1..60000") {
			t.Fatalf("frame_period_ms=%d: error = %v, want period bound", tc.period, err)
		}
	}
}
