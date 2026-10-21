package peer

import (
	"fmt"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
	"github.com/rmsff/ad-hoc-radio/internal/timing"
	"sort"
)

const EventType = "peer_control"
const MaxTimeMS int64 = 1_000_000_000_000
const MaxPeers = 64
const MaxTransactions = 64
const MaxControlTransmissions = 65536
const MaxControlEvents = 262144

type Kind string

const (
	Proposal Kind = "proposal"
	ACK      Kind = "ack"
	Commit   Kind = "commit"
	End      Kind = "end"
	Sync     Kind = "sync"
)

type message struct {
	Kind     Kind
	From, To uint16
	Permit   Permit
	Nonce    uint64
}
type pending struct {
	Raw          []byte
	Route        []uint16
	Hop, Attempt int
	Message      message
	Local        string
	Transmission *AirTransmission
}
type EventResult struct {
	Activated, Ended uint16
	Version          uint16
	Digest           string
	Expired          bool
}
type Counters struct {
	Attempts       int   `json:"attempts"`
	Bytes          int   `json:"bytes"`
	AirtimeUS      int64 `json:"airtime_us"`
	Lost           int   `json:"lost"`
	Invalid        int   `json:"invalid"`
	Unreachable    int   `json:"unreachable"`
	Rejected       int   `json:"rejected"`
	ExpiredTX      int   `json:"expired_tx"`
	Commits        int   `json:"commits"`
	Duplicates     int   `json:"duplicates"`
	Expirations    int   `json:"expirations"`
	Reacquisitions int   `json:"reacquisitions"`
	Collisions     int   `json:"collisions"`
}
type transaction struct {
	permit    Permit
	targets   []uint16
	required  map[uint16]bool
	acked     map[uint16]bool
	committed bool
}
type Controller struct {
	cfg          Config
	queue        *sim.EventQueue
	locals       map[uint16]*localState
	fleet        *Fleet
	events       map[uint64]pending
	transactions map[string]*transaction
	seen         map[string]bool
	permit       Permit
	source       uint16
	stats        Counters
	busFreeMS    int64
	eventCount   int
	messages     uint64
	ended        bool
	receipts     []CommitReceipt
}

func New(cfg Config, queue *sim.EventQueue) (*Controller, error) {
	if err := validateConfig(cfg, queue); err != nil {
		return nil, err
	}
	if cfg.ReplanLeadMS == 0 {
		cfg.ReplanLeadMS = cfg.RetryMS * int64(cfg.Attempts)
	}
	cfg.Peers = append([]State(nil), cfg.Peers...)
	cfg.Faults = append([]ControlFault(nil), cfg.Faults...)
	fleet := cfg.Fleet
	if fleet == nil {
		var err error
		fleet, err = NewFleet(cfg.Peers)
		if err != nil {
			return nil, err
		}
	}
	for _, s := range cfg.Peers {
		if fleet.locals[s.ID] == nil {
			return nil, fmt.Errorf("peer: missing fleet id=%d", s.ID)
		}
	}
	c := &Controller{cfg: cfg, queue: queue, fleet: fleet, locals: fleet.locals, events: map[uint64]pending{}, transactions: map[string]*transaction{}, seen: map[string]bool{}, busFreeMS: cfg.StartMS}
	if err := c.UpdateLinks(cfg.Links, cfg.LinksObservedAtMS, cfg.LinksFreshnessMS); err != nil {
		return nil, err
	}
	return c, nil
}
func (c *Controller) Begin(schedule timing.OpportunitySchedule, epochMS, periodMS int64) error {
	if c.source != 0 {
		return fmt.Errorf("peer: call already started")
	}
	if err := timing.ValidateInitialOpportunitySchedule(schedule); err != nil {
		return err
	}
	return c.beginTransaction(schedule, epochMS, periodMS, c.cfg.StartMS)
}

// Replan disseminates an immutable successor. Failure never grants its version.
func (c *Controller) Replan(schedule timing.OpportunitySchedule, atMS int64) error {
	if c.source == 0 || c.ended || !validTime(atMS) || schedule.Binding.EffectiveSeq == 0 {
		return fmt.Errorf("peer: cannot replan")
	}
	if err := timing.ValidateScheduleSuccessor(c.permit.Schedule, schedule, schedule.Binding.EffectiveSeq-1); err != nil {
		return err
	}
	return c.beginTransaction(schedule, c.permit.EpochMS, c.permit.PeriodMS, atMS)
}
func (c *Controller) beginTransaction(schedule timing.OpportunitySchedule, epoch, period, at int64) error {
	p, err := makePermit(schedule, epoch, period)
	if err != nil {
		return err
	}
	if !validTime(at) || activationMS(p) <= at || len(c.transactions) >= MaxTransactions {
		return fmt.Errorf("peer: transaction horizon/bound")
	}
	tx := &transaction{permit: p, required: map[uint16]bool{}, acked: map[uint16]bool{}}
	ids := map[uint16]bool{}
	for _, id := range p.Schedule.CoveredMemberIDs {
		ids[id] = true
	}
	for _, op := range p.Schedule.Opportunities {
		ids[op.OwnerStationID] = true
		tx.required[op.OwnerStationID] = true
	}
	source := p.Schedule.Binding.SourceStationID
	for id := range ids {
		if c.locals[id] == nil {
			return fmt.Errorf("peer: missing participant=%d", id)
		}
		if id != source {
			tx.targets = append(tx.targets, id)
		}
	}
	sort.Slice(tx.targets, func(i, j int) bool { return tx.targets[i] < tx.targets[j] })
	c.source = source
	c.permit = p
	c.transactions[p.Digest] = tx
	return c.push(at, pending{Local: "launch", Message: message{Kind: Proposal, From: source, To: source, Permit: p}}, source)
}
func (c *Controller) Process(event sim.Event) (EventResult, error) {
	p, ok := c.events[event.EventID]
	if !ok || !validTime(event.TimeMs) {
		return EventResult{}, fmt.Errorf("peer: unknown event or invalid time")
	}
	delete(c.events, event.EventID)
	if p.Local != "" {
		return c.processLocal(p, event.TimeMs)
	}
	if p.Transmission != nil && p.Transmission.collided {
		c.stats.Collisions++
		return EventResult{}, nil
	}
	m, err := decodeControl(p.Raw, c.cfg.WireFormat)
	if err != nil {
		c.stats.Invalid++
		return EventResult{}, nil
	}
	if m.Kind != p.Message.Kind || m.From != p.Message.From || m.To != p.Message.To || m.Permit.Digest != p.Message.Permit.Digest {
		c.stats.Invalid++
		return EventResult{}, nil
	}
	key := fmt.Sprintf("%d/%d", m.Nonce, p.Hop)
	if c.seen[key] {
		c.stats.Duplicates++
		return EventResult{}, nil
	}
	c.seen[key] = true
	if p.Hop < len(p.Route)-1 {
		return EventResult{}, c.sendHop(p, event.TimeMs)
	}
	l := c.locals[m.To]
	if c.cfg.WireFormat != JSONCRCv0 && m.Kind != Proposal {
		var ok bool
		m, ok = resolveReference(l, m)
		if !ok {
			c.stats.Rejected++
			return EventResult{}, nil
		}
	}
	if !available(l, event.TimeMs) {
		c.stats.Rejected++
		return EventResult{}, nil
	}
	switch m.Kind {
	case Proposal:
		if !c.acceptProposal(l, m.Permit, event.TimeMs) {
			c.stats.Rejected++
			return EventResult{}, nil
		}
		tx := c.transactions[m.Permit.Digest]
		if tx != nil && tx.required[m.To] {
			return EventResult{}, c.send(message{Kind: ACK, From: m.To, To: m.From, Permit: m.Permit}, l.Tentative.AcquiredMS, p.Attempt)
		}
	case ACK:
		tx := c.transactions[m.Permit.Digest]
		if tx == nil || !samePermitReference(m.Permit, tx.permit) || m.To != c.source || !tx.required[m.From] || event.TimeMs >= activationMS(tx.permit) {
			c.stats.Rejected++
			return EventResult{}, nil
		}
		tx.acked[m.From] = true
		return EventResult{}, c.tryCommit(tx, event.TimeMs)
	case Commit:
		return EventResult{}, c.installCommit(l, m.Permit, event.TimeMs)
	case End:
		return c.release(l, m.Permit, event.TimeMs, false), nil
	case Sync:
		if l.Active != nil && samePermitReference(*l.Active, m.Permit) && event.TimeMs < l.Active.ExpiresMS {
			c.acquire(l, l.Active, event.TimeMs)
			c.stats.Reacquisitions++
		}
	}
	return EventResult{}, nil
}
func (c *Controller) LatestSchedule() timing.OpportunitySchedule {
	return cloneSchedule(c.permit.Schedule)
}
func (c *Controller) ReplanLeadMS() int64     { return c.cfg.ReplanLeadMS }
func (c *Controller) HasEvent(id uint64) bool { _, ok := c.events[id]; return ok }
func (c *Controller) Counters() Counters      { return c.stats }
