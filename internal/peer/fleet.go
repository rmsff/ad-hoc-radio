package peer

import (
	"fmt"

	"github.com/rmsff/ad-hoc-radio/internal/transport"
)

// Fleet shares physical state on one deterministic event-loop. It is not a
// goroutine-safe service; concurrent calls must advance the same timeline.
type Fleet struct {
	locals map[uint16]*localState
	air    []*AirTransmission
}
type AirTransmission struct {
	links      []transport.Link
	owner      uint16
	start, end int64
	collided   bool
}

func NewFleet(states []State) (*Fleet, error) {
	if len(states) == 0 || len(states) > MaxPeers {
		return nil, fmt.Errorf("peer: fleet requires 1..64 peers")
	}
	f := &Fleet{locals: map[uint16]*localState{}}
	for _, s := range states {
		if err := s.Validate(); err != nil {
			return nil, err
		}
		if f.locals[s.ID] != nil {
			return nil, fmt.Errorf("peer: duplicate fleet id")
		}
		f.locals[s.ID] = &localState{State: s, Pending: map[string]*Permit{}, Closed: map[string]int64{}}
	}
	return f, nil
}

// SetState changes local availability/clock, never grants a permit.
func (f *Fleet) SetState(s State) error {
	if err := s.Validate(); err != nil {
		return err
	}
	l := f.locals[s.ID]
	if l == nil {
		return fmt.Errorf("peer: unknown fleet id")
	}
	l.State = s
	return nil
}

// ReserveAir records actual TX, not permission. Shared-peer half-duplex and
// overlapping audible transmitters collide; disjoint partitions may coexist.
func (c *Controller) ReserveAir(owner uint16, start, end int64) (*AirTransmission, error) {
	if !validTime(start) || !validTime(end) || end <= start {
		return nil, fmt.Errorf("peer: invalid airtime interval")
	}
	if len(c.fleet.air) >= MaxControlEvents {
		return nil, fmt.Errorf("peer: fleet airtime bound")
	}
	next := &AirTransmission{owner: owner, start: start, end: end, links: c.cfg.Links}
	for _, prior := range c.fleet.air {
		if start < prior.end && prior.start < end && interferesOn(c.cfg.Links, prior.links, owner, prior.owner) {
			prior.collided = true
			next.collided = true
		}
	}
	c.fleet.air = append(c.fleet.air, next)
	return next, nil
}
func interferesOn(first, second []transport.Link, a, b uint16) bool {
	if a == b {
		return true
	}
	receivers := map[uint16]bool{a: true}
	for _, links := range [2][]transport.Link{first, second} {
		for _, l := range links {
			if l.Enabled && uint16(l.From) == a {
				receivers[uint16(l.To)] = true
			}
		}
	}
	if receivers[b] {
		return true
	}
	for _, links := range [2][]transport.Link{first, second} {
		for _, l := range links {
			if l.Enabled && uint16(l.From) == b && receivers[uint16(l.To)] {
				return true
			}
		}
	}
	return false
}
func (c *Controller) AirCollided(tx *AirTransmission) bool { return tx != nil && tx.collided }
func activationMS(p Permit) int64 {
	return p.EpochMS + int64(p.Schedule.Binding.EffectiveSeq)*p.PeriodMS
}
