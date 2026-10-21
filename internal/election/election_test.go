package election

import (
	"errors"
	"math"
	"testing"
)

func TestUT016_SelectRelayChoosesHighestScore(t *testing.T) {
	result, err := SelectRelay([]Candidate{
		{
			NodeID:            2,
			RelayCapable:      true,
			RelayEnabled:      true,
			QualityFromSource: 1.0,
			GroupCoverage:     0.20,
			Battery:           0.90,
			Stability:         0.80,
		},
		{
			NodeID:            3,
			RelayCapable:      true,
			RelayEnabled:      true,
			QualityFromSource: 0.90,
			GroupCoverage:     0.95,
			Battery:           0.80,
			Stability:         0.90,
		},
	}, DefaultPolicy())
	if err != nil {
		t.Fatalf("SelectRelay failed: %v", err)
	}

	if result.RelayID != 3 {
		t.Fatalf("RelayID = %d, want 3", result.RelayID)
	}
	if result.Score != 90.25 {
		t.Fatalf("Score = %.2f, want 90.25", result.Score)
	}
	if result.EligibleCount != 2 {
		t.Fatalf("EligibleCount = %d, want 2", result.EligibleCount)
	}
}

func TestUT016_SelectRelayFiltersIneligibleCandidates(t *testing.T) {
	tests := []struct {
		name      string
		candidate Candidate
	}{
		{
			name: "not relay capable",
			candidate: Candidate{
				NodeID:            2,
				RelayEnabled:      true,
				QualityFromSource: 1,
				GroupCoverage:     1,
				Battery:           1,
			},
		},
		{
			name: "relay disabled",
			candidate: Candidate{
				NodeID:            2,
				RelayCapable:      true,
				QualityFromSource: 1,
				GroupCoverage:     1,
				Battery:           1,
			},
		},
		{
			name: "busy",
			candidate: Candidate{
				NodeID:            2,
				RelayCapable:      true,
				RelayEnabled:      true,
				Busy:              true,
				QualityFromSource: 1,
				GroupCoverage:     1,
				Battery:           1,
			},
		},
		{
			name: "low battery",
			candidate: Candidate{
				NodeID:            2,
				RelayCapable:      true,
				RelayEnabled:      true,
				QualityFromSource: 1,
				GroupCoverage:     1,
				Battery:           0.09,
			},
		},
		{
			name: "weak source link",
			candidate: Candidate{
				NodeID:            2,
				RelayCapable:      true,
				RelayEnabled:      true,
				QualityFromSource: 0.19,
				GroupCoverage:     1,
				Battery:           1,
			},
		},
		{
			name: "zero group coverage",
			candidate: Candidate{
				NodeID:            2,
				RelayCapable:      true,
				RelayEnabled:      true,
				QualityFromSource: 1,
				GroupCoverage:     0,
				Battery:           1,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := SelectRelay([]Candidate{tc.candidate}, DefaultPolicy())
			if !errors.Is(err, ErrNoEligibleRelay) {
				t.Fatalf("error = %v, want ErrNoEligibleRelay", err)
			}
		})
	}
}

func TestUT016_SelectRelayRejectsEmptyCandidateList(t *testing.T) {
	_, err := SelectRelay(nil, DefaultPolicy())
	if !errors.Is(err, ErrNoEligibleRelay) {
		t.Fatalf("error = %v, want ErrNoEligibleRelay", err)
	}
}

func TestUT016_SelectRelayTieBreaksByLowerNodeID(t *testing.T) {
	result, err := SelectRelay([]Candidate{
		{
			NodeID:            9,
			RelayCapable:      true,
			RelayEnabled:      true,
			QualityFromSource: 0.80,
			GroupCoverage:     0.80,
			Battery:           0.80,
			Stability:         0.80,
		},
		{
			NodeID:            4,
			RelayCapable:      true,
			RelayEnabled:      true,
			QualityFromSource: 0.80,
			GroupCoverage:     0.80,
			Battery:           0.80,
			Stability:         0.80,
		},
	}, DefaultPolicy())
	if err != nil {
		t.Fatalf("SelectRelay failed: %v", err)
	}

	if result.RelayID != 4 {
		t.Fatalf("RelayID = %d, want lower node id 4", result.RelayID)
	}
	if result.EligibleCount != 2 {
		t.Fatalf("EligibleCount = %d, want 2", result.EligibleCount)
	}
}

func TestUT016_SelectRelayTieBreaksAfterScoreQuantization(t *testing.T) {
	result, err := SelectRelay([]Candidate{
		{
			NodeID:            9,
			RelayCapable:      true,
			RelayEnabled:      true,
			QualityFromSource: 0.10150,
			GroupCoverage:     0.10,
			Battery:           0.80,
			Stability:         0.80,
		},
		{
			NodeID:            4,
			RelayCapable:      true,
			RelayEnabled:      true,
			QualityFromSource: 0.10149999999999999,
			GroupCoverage:     0.10,
			Battery:           0.80,
			Stability:         0.80,
		},
	}, Policy{LowBatteryThreshold: 0.10, MinSourceQuality: 0.10})
	if err != nil {
		t.Fatalf("SelectRelay failed: %v", err)
	}

	if result.RelayID != 4 {
		t.Fatalf("RelayID = %d, want lower node id 4 after score quantization", result.RelayID)
	}
}

func TestUT016_SelectRelayRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name       string
		candidates []Candidate
		policy     Policy
	}{
		{
			name: "zero node id",
			candidates: []Candidate{{
				RelayCapable:      true,
				RelayEnabled:      true,
				QualityFromSource: 1,
				GroupCoverage:     1,
				Battery:           1,
			}},
			policy: DefaultPolicy(),
		},
		{
			name: "quality over one",
			candidates: []Candidate{{
				NodeID:            2,
				RelayCapable:      true,
				RelayEnabled:      true,
				QualityFromSource: 1.1,
				GroupCoverage:     1,
				Battery:           1,
			}},
			policy: DefaultPolicy(),
		},
		{
			name: "coverage NaN",
			candidates: []Candidate{{
				NodeID:            2,
				RelayCapable:      true,
				RelayEnabled:      true,
				QualityFromSource: 1,
				GroupCoverage:     math.NaN(),
				Battery:           1,
			}},
			policy: DefaultPolicy(),
		},
		{
			name: "battery Inf",
			candidates: []Candidate{{
				NodeID:            2,
				RelayCapable:      true,
				RelayEnabled:      true,
				QualityFromSource: 1,
				GroupCoverage:     1,
				Battery:           math.Inf(1),
			}},
			policy: DefaultPolicy(),
		},
		{
			name: "duplicate node id",
			candidates: []Candidate{
				{
					NodeID:            2,
					RelayCapable:      true,
					RelayEnabled:      true,
					QualityFromSource: 1,
					GroupCoverage:     1,
					Battery:           1,
				},
				{
					NodeID:            2,
					RelayCapable:      true,
					RelayEnabled:      true,
					QualityFromSource: 1,
					GroupCoverage:     1,
					Battery:           1,
				},
			},
			policy: DefaultPolicy(),
		},
		{
			name: "invalid threshold",
			candidates: []Candidate{{
				NodeID:            2,
				RelayCapable:      true,
				RelayEnabled:      true,
				QualityFromSource: 1,
				GroupCoverage:     1,
				Battery:           1,
			}},
			policy: Policy{LowBatteryThreshold: -0.1, MinSourceQuality: 0.2},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := SelectRelay(tc.candidates, tc.policy); err == nil {
				t.Fatal("SelectRelay accepted invalid input")
			}
		})
	}
}

func TestUT016_SelectRelayProvesSnapshotHandover(t *testing.T) {
	first, err := SelectRelay([]Candidate{
		{
			NodeID:            2,
			RelayCapable:      true,
			RelayEnabled:      true,
			QualityFromSource: 1.00,
			GroupCoverage:     0.95,
			Battery:           0.90,
			Stability:         0.90,
		},
		{
			NodeID:            3,
			RelayCapable:      true,
			RelayEnabled:      true,
			QualityFromSource: 0.70,
			GroupCoverage:     0.80,
			Battery:           0.90,
			Stability:         0.80,
		},
	}, DefaultPolicy())
	if err != nil {
		t.Fatalf("first SelectRelay failed: %v", err)
	}

	second, err := SelectRelay([]Candidate{
		{
			NodeID:            2,
			RelayCapable:      true,
			RelayEnabled:      true,
			QualityFromSource: 0.30,
			GroupCoverage:     0.30,
			Battery:           0.90,
			Stability:         0.90,
		},
		{
			NodeID:            3,
			RelayCapable:      true,
			RelayEnabled:      true,
			QualityFromSource: 0.90,
			GroupCoverage:     0.95,
			Battery:           0.90,
			Stability:         0.80,
		},
	}, DefaultPolicy())
	if err != nil {
		t.Fatalf("second SelectRelay failed: %v", err)
	}

	if first.RelayID != 2 {
		t.Fatalf("first RelayID = %d, want 2", first.RelayID)
	}
	if second.RelayID != 3 {
		t.Fatalf("second RelayID = %d, want handover to 3", second.RelayID)
	}
}
