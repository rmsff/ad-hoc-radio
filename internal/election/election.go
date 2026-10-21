package election

import (
	"errors"
	"fmt"
	"math"
)

const (
	scoreScale          = 1000
	qualityWeight       = 40.0
	groupCoverageWeight = 35.0
	batteryWeight       = 15.0
	stabilityWeight     = 10.0
)

var ErrNoEligibleRelay = errors.New("election: no eligible relay")

type Candidate struct {
	NodeID            uint16
	RelayCapable      bool
	RelayEnabled      bool
	Busy              bool
	QualityFromSource float64
	GroupCoverage     float64
	Battery           float64
	Stability         float64
}

type Policy struct {
	LowBatteryThreshold float64
	MinSourceQuality    float64
}

type Result struct {
	RelayID       uint16
	Score         float64
	EligibleCount int
}

func DefaultPolicy() Policy {
	return Policy{
		LowBatteryThreshold: 0.10,
		MinSourceQuality:    0.20,
	}
}

func SelectRelay(candidates []Candidate, policy Policy) (Result, error) {
	if err := validatePolicy(policy); err != nil {
		return Result{}, err
	}

	var selected Result
	seenNodeIDs := make(map[uint16]struct{}, len(candidates))
	selectedScoreUnits := 0
	for i, c := range candidates {
		if err := validateCandidate(c); err != nil {
			return Result{}, fmt.Errorf("candidate %d: %w", i, err)
		}
		if _, exists := seenNodeIDs[c.NodeID]; exists {
			return Result{}, fmt.Errorf("candidate %d: duplicate node_id %d", i, c.NodeID)
		}
		seenNodeIDs[c.NodeID] = struct{}{}
		if !eligible(c, policy) {
			continue
		}

		scoreUnits := scoreCandidateUnits(c)
		if selected.EligibleCount == 0 || scoreUnits > selectedScoreUnits || (scoreUnits == selectedScoreUnits && c.NodeID < selected.RelayID) {
			selected.RelayID = c.NodeID
			selected.Score = float64(scoreUnits) / scoreScale
			selectedScoreUnits = scoreUnits
		}
		selected.EligibleCount++
	}

	if selected.EligibleCount == 0 {
		return Result{}, ErrNoEligibleRelay
	}
	return selected, nil
}

func scoreCandidateUnits(c Candidate) int {
	score := qualityWeight*c.QualityFromSource +
		groupCoverageWeight*c.GroupCoverage +
		batteryWeight*c.Battery +
		stabilityWeight*c.Stability
	return int(math.Round(score * scoreScale))
}

func eligible(c Candidate, policy Policy) bool {
	return c.RelayCapable &&
		c.RelayEnabled &&
		!c.Busy &&
		c.Battery >= policy.LowBatteryThreshold &&
		c.QualityFromSource >= policy.MinSourceQuality &&
		c.GroupCoverage > 0
}

func validatePolicy(policy Policy) error {
	if err := validateUnitInterval("low_battery_threshold", policy.LowBatteryThreshold); err != nil {
		return err
	}
	if err := validateUnitInterval("min_source_quality", policy.MinSourceQuality); err != nil {
		return err
	}
	return nil
}

func validateCandidate(c Candidate) error {
	if c.NodeID == 0 {
		return errors.New("node_id must be > 0")
	}
	if err := validateUnitInterval("quality_from_source", c.QualityFromSource); err != nil {
		return err
	}
	if err := validateUnitInterval("group_coverage", c.GroupCoverage); err != nil {
		return err
	}
	if err := validateUnitInterval("battery", c.Battery); err != nil {
		return err
	}
	if err := validateUnitInterval("stability", c.Stability); err != nil {
		return err
	}
	return nil
}

func validateUnitInterval(name string, v float64) error {
	if math.IsNaN(v) {
		return fmt.Errorf("%s is NaN", name)
	}
	if math.IsInf(v, 0) {
		return fmt.Errorf("%s is Inf", name)
	}
	if v < 0 || v > 1 {
		return fmt.Errorf("%s=%g must be in [0, 1]", name, v)
	}
	return nil
}
