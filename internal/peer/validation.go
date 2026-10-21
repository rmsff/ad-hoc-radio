package peer

import (
	"fmt"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
	"github.com/rmsff/ad-hoc-radio/internal/transport"
	"math"
)

func validTime(at int64) bool { return at >= 0 && at <= MaxTimeMS }
func validateConfig(cfg Config, q *sim.EventQueue) error {
	if !cfg.WireFormat.Valid() {
		return fmt.Errorf("peer: unsupported wire format")
	}
	if q == nil || !validTime(cfg.StartMS) || cfg.RetryMS <= 0 || cfg.RetryMS > 1000000 || cfg.Attempts < 1 || cfg.Attempts > 8 || cfg.ReplanLeadMS < 0 || cfg.ReplanLeadMS > 100000000 {
		return fmt.Errorf("peer: invalid control bounds")
	}
	if math.IsNaN(cfg.MinBattery) || cfg.MinBattery < 0 || cfg.MinBattery > 1 || len(cfg.Peers) > MaxPeers || len(cfg.Faults) > 256 {
		return fmt.Errorf("peer: invalid battery/inventory/fault bounds")
	}
	if err := cfg.Physical.Validate(); err != nil {
		return err
	}
	seen := map[uint16]bool{}
	for _, s := range cfg.Peers {
		if err := s.Validate(); err != nil {
			return err
		}
		if seen[s.ID] {
			return fmt.Errorf("peer: duplicate id")
		}
		seen[s.ID] = true
	}
	for _, f := range cfg.Faults {
		if f.Kind != Proposal && f.Kind != ACK && f.Kind != Commit && f.Kind != End && f.Kind != Sync {
			return fmt.Errorf("peer: invalid fault kind")
		}
		if math.IsNaN(f.LossPct) || f.LossPct < 0 || f.LossPct > 100 || f.DelayMS < 0 || f.DelayMS > 1000000 {
			return fmt.Errorf("peer: invalid control fault")
		}
	}
	return nil
}
func makePermit(s timing.OpportunitySchedule, epoch, period int64) (Permit, error) {
	if err := timing.ValidateOpportunitySchedule(s); err != nil {
		return Permit{}, err
	}
	if !validTime(epoch) || period <= 0 || period > 10000 || s.Binding.ValidThroughSeq > 65535 || len(s.Binding.ExpectedMemberIDs) > MaxPeers || len(s.Opportunities) > MaxPeers || uint64(period) < uint64(s.CycleDurationMS()) {
		return Permit{}, fmt.Errorf("peer: invalid bounded call horizon/cycle")
	}
	expires := epoch + (int64(s.Binding.ValidThroughSeq)+1)*period
	if !validTime(expires) {
		return Permit{}, fmt.Errorf("peer: expiry exceeds time bound")
	}
	if s.Binding.TimingStrategy != timing.TimingStrategySourceLed || s.Binding.ScheduleAuthorityID != s.Binding.SourceStationID || s.Binding.TimingAuthorityID != s.Binding.SourceStationID {
		return Permit{}, fmt.Errorf("peer: only source-led control implemented")
	}
	hash, err := timing.OpportunityScheduleHash(s)
	if err != nil {
		return Permit{}, err
	}
	return Permit{Schedule: cloneSchedule(s), Digest: hash, EpochMS: epoch, PeriodMS: period, ExpiresMS: expires}, nil
}
func cloneSchedule(s timing.OpportunitySchedule) timing.OpportunitySchedule {
	s.Binding.ExpectedMemberIDs = append([]uint16(nil), s.Binding.ExpectedMemberIDs...)
	s.CoveredMemberIDs = append([]uint16(nil), s.CoveredMemberIDs...)
	s.UncoveredMemberIDs = append([]uint16(nil), s.UncoveredMemberIDs...)
	s.Opportunities = append([]timing.TxOpportunity(nil), s.Opportunities...)
	return s
}

// UpdateLinks installs one local observed snapshot; it neither discovers peers
// nor refreshes old evidence. Zero freshness explicitly denotes a static fixture.
func (c *Controller) UpdateLinks(links []transport.Link, observed, freshness int64) error {
	if !validTime(observed) || freshness < 0 || freshness > MaxTimeMS || len(links) > MaxPeers*(MaxPeers-1) {
		return fmt.Errorf("peer: invalid link snapshot bounds")
	}
	seen := map[[2]uint16]bool{}
	for _, l := range links {
		k := [2]uint16{uint16(l.From), uint16(l.To)}
		if c.locals[k[0]] == nil || c.locals[k[1]] == nil || k[0] == k[1] || seen[k] || l.FixedDelayMs < 0 || l.FixedDelayMs > 1000000 || l.JitterMs < 0 || l.JitterMs > 1000000 {
			return fmt.Errorf("peer: invalid link endpoints/delay")
		}
		seen[k] = true
		check := l
		check.Enabled = true
		if err := (transport.Topology{Links: []transport.Link{check}}).Validate(); err != nil {
			return err
		}
	}
	c.cfg.Links = append([]transport.Link(nil), links...)
	c.cfg.LinksObservedAtMS = observed
	c.cfg.LinksFreshnessMS = freshness
	return nil
}
func sameCall(a, b Permit) bool {
	x, y := a.Schedule.Binding, b.Schedule.Binding
	return x.NetID == y.NetID && x.GroupID == y.GroupID && x.SourceStationID == y.SourceStationID && x.CallID == y.CallID && x.ScheduleID == y.ScheduleID
}

func sameTimeline(a, b Permit) bool { return a.EpochMS == b.EpochMS && a.PeriodMS == b.PeriodMS }

func samePermitReference(a, b Permit) bool {
	return a.Digest == b.Digest && a.Schedule.Binding.ScheduleVersion == b.Schedule.Binding.ScheduleVersion && sameTimeline(a, b)
}
func precedes(a, b Permit) bool {
	x, y := a.Schedule.Binding, b.Schedule.Binding
	if x.SourceStationID != y.SourceStationID {
		return x.SourceStationID < y.SourceStationID
	}
	if x.CallID != y.CallID {
		return x.CallID < y.CallID
	}
	if x.NetID != y.NetID {
		return x.NetID < y.NetID
	}
	if x.GroupID != y.GroupID {
		return x.GroupID < y.GroupID
	}
	return x.ScheduleID < y.ScheduleID
}
func available(l *localState, at int64) bool {
	return l != nil && validTime(at) && l.Mode == ADHR && !l.Busy && at >= l.AvailableAtMS
}
