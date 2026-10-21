// Package peer models local ADHR participation, not a dedicated relay device.
package peer

import (
	"fmt"
	"math"

	"github.com/rmsff/ad-hoc-radio/internal/routing"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
	"github.com/rmsff/ad-hoc-radio/internal/transport"
)

type Mode string

const (
	ADHR     Mode = "adhr"
	AnalogFM Mode = "analog_fm"
)

type State struct {
	ID              uint16  `json:"id"`
	Mode            Mode    `json:"mode"`
	RelayEnabled    bool    `json:"relay_enabled"`
	Busy            bool    `json:"busy"`
	BatteryFraction float64 `json:"battery_fraction"`
	AvailableAtMS   int64   `json:"available_at_ms"`
	Clock           Clock   `json:"clock"`
}
type Clock struct {
	OffsetMS         int64 `json:"offset_ms"`
	DriftPPM         int64 `json:"drift_ppm"`
	ResidualErrorMS  int64 `json:"residual_error_ms"`
	DriftBoundPPM    int64 `json:"drift_bound_ppm"`
	AcquisitionMS    int64 `json:"acquisition_ms"`
	MaxUncertaintyMS int64 `json:"max_uncertainty_ms"`
}

func (s State) RelayState() routing.RelayState {
	return routing.RelayState{StationID: s.ID, RelayCapable: true, RelayEnabled: s.RelayEnabled && s.Mode == ADHR, Busy: s.Busy, BatteryFraction: s.BatteryFraction}
}
func (s State) Validate() error {
	if s.ID == 0 || (s.Mode != ADHR && s.Mode != AnalogFM) || math.IsNaN(s.BatteryFraction) || s.BatteryFraction < 0 || s.BatteryFraction > 1 || !validTime(s.AvailableAtMS) {
		return fmt.Errorf("peer: invalid state")
	}
	c := s.Clock
	if c.OffsetMS < -1000000 || c.OffsetMS > 1000000 || c.ResidualErrorMS < -1000000 || c.ResidualErrorMS > 1000000 || c.DriftPPM < -100000 || c.DriftPPM > 100000 || c.DriftBoundPPM < abs(c.DriftPPM) || c.DriftBoundPPM > 100000 || c.AcquisitionMS < 0 || c.AcquisitionMS > 1000000 || c.MaxUncertaintyMS < 0 || c.MaxUncertaintyMS > 1000000 {
		return fmt.Errorf("peer: invalid clock bounds")
	}
	return nil
}

type Config struct {
	WireFormat        WireFormat
	ReplanLeadMS      int64
	LinksObservedAtMS int64
	LinksFreshnessMS  int64
	Faults            []ControlFault
	Fleet             *Fleet
	Peers             []State
	Links             []transport.Link
	StartMS           int64
	RetryMS           int64
	Attempts          int
	Seed              uint64
	MinBattery        float64
	Physical          timing.VoiceBudgetProfile
}

type ControlFault struct {
	Version uint16
	Kind    Kind
	From    uint16
	To      uint16
	LossPct float64
	DelayMS int64
}
type Permit struct {
	Schedule          timing.OpportunitySchedule
	Digest            string
	EpochMS           int64
	PeriodMS          int64
	ReceivedMS        int64
	AcquiredMS        int64
	ExpiresMS         int64
	Committed         bool
	ClockCorrectionMS int64
}
type localState struct {
	State
	Tentative *Permit
	Active    *Permit
	Pending   map[string]*Permit
	Closed    map[string]int64
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
