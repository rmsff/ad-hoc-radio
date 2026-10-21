package peer

import (
	"fmt"
	"github.com/rmsff/ad-hoc-radio/internal/sim"
	"github.com/rmsff/ad-hoc-radio/internal/transport"
	"sort"
)

func (c *Controller) route(from, to uint16) []uint16 {
	queue := [][]uint16{{from}}
	seen := map[uint16]bool{from: true}
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		last := path[len(path)-1]
		if last == to {
			return path
		}
		next := []uint16{}
		for _, l := range c.cfg.Links {
			if l.Enabled && uint16(l.From) == last && !seen[uint16(l.To)] {
				next = append(next, uint16(l.To))
			}
		}
		sort.Slice(next, func(i, j int) bool { return next[i] < next[j] })
		for _, id := range next {
			if !seen[id] {
				seen[id] = true
				queue = append(queue, append(append([]uint16(nil), path...), id))
			}
		}
	}
	return nil
}
func (c *Controller) send(m message, at int64, attempt int) error {
	if !validTime(at) || attempt < 0 || attempt >= c.cfg.Attempts {
		return fmt.Errorf("peer: invalid send time/attempt")
	}
	if c.messages >= MaxControlTransmissions {
		return fmt.Errorf("peer: control message bound")
	}
	route := c.route(m.From, m.To)
	if len(route) < 2 {
		c.stats.Unreachable++
		return nil
	}
	c.messages++
	m.Nonce = c.messages
	raw, err := encodeControl(m, c.cfg.WireFormat)
	if err != nil {
		return err
	}
	return c.sendHop(pending{Raw: raw, Message: m, Route: route, Attempt: attempt}, at)
}
func (c *Controller) sendHop(p pending, at int64) error {
	if p.Hop < 0 || p.Hop+1 >= len(p.Route) || len(p.Route) > MaxPeers || !validTime(at) {
		return fmt.Errorf("peer: invalid route/time")
	}
	from, to := p.Route[p.Hop], p.Route[p.Hop+1]
	if at < c.busFreeMS {
		at = c.busFreeMS
	}
	if !validTime(at) {
		return fmt.Errorf("peer: serialized time bound")
	}
	if p.Message.Kind != End && at >= p.Message.Permit.ExpiresMS {
		return nil
	}
	if (p.Message.Kind == Proposal || p.Message.Kind == ACK || p.Message.Kind == Commit) && at >= activationMS(p.Message.Permit) {
		return nil
	}
	if !available(c.locals[from], at) {
		c.stats.Rejected++
		return nil
	}
	if at < c.cfg.LinksObservedAtMS || (c.cfg.LinksFreshnessMS > 0 && at-c.cfg.LinksObservedAtMS > c.cfg.LinksFreshnessMS) {
		c.stats.Unreachable++
		return nil
	}
	link := (transport.Topology{Links: c.cfg.Links}).LinkFor(transport.StationID(from), transport.StationID(to))
	if link == nil || !link.Enabled {
		c.stats.Unreachable++
		return nil
	}
	copied := *link
	for _, f := range c.cfg.Faults {
		if f.Kind == p.Message.Kind && (f.Version == 0 || f.Version == p.Message.Permit.Schedule.Binding.ScheduleVersion) && (f.From == 0 || f.From == p.Message.From) && (f.To == 0 || f.To == p.Message.To) {
			copied.LossPct = f.LossPct
			copied.FixedDelayMs += f.DelayMS
		}
	}
	budget, err := c.cfg.Physical.Measure(p.Raw)
	if err != nil {
		return err
	}
	if !budget.FitsBackend {
		c.stats.Rejected++
		return nil
	}
	duration := (budget.OccupiedUS + 999) / 1000
	if duration < 1 {
		duration = 1
	}
	end := at + duration
	serialized := at + (budget.AirtimeUS+999)/1000
	if !validTime(end) || !validTime(serialized+copied.FixedDelayMs+copied.JitterMs+5) {
		return fmt.Errorf("peer: delivery exceeds time bound")
	}
	if c.stats.Attempts >= MaxControlTransmissions {
		return fmt.Errorf("peer: control transmission bound")
	}
	tx, err := c.ReserveAir(from, at, end)
	if err != nil {
		return err
	}
	c.busFreeMS = end
	c.stats.Attempts++
	c.stats.Bytes += len(p.Raw)
	c.stats.AirtimeUS += budget.OccupiedUS
	seed := c.cfg.Seed ^ uint64(from)<<48 ^ uint64(to)<<32 ^ uint64(p.Attempt)<<24 ^ p.Message.Nonce
	deliveries, err := transport.SamplePacket(copied, p.Raw, serialized, seed)
	if err != nil {
		return err
	}
	if len(deliveries) == 0 {
		c.stats.Lost++
	}
	for _, d := range deliveries {
		next := p
		next.Hop++
		next.Raw = d.Raw
		next.Transmission = tx
		if err := c.push(d.ReceivedAt, next, to); err != nil {
			return err
		}
	}
	return nil
}
func (c *Controller) push(at int64, p pending, destination uint16) error {
	if !validTime(at) || c.eventCount >= MaxControlEvents {
		return fmt.Errorf("peer: event time/count bound")
	}
	priority := sim.PriorityFrameDelivery
	if p.Local == "activate" || p.Local == "expire" || p.Local == "launch" {
		priority = sim.PriorityScheduleUpdate
	}
	c.queue.Push(sim.Event{TimeMs: at, EventPriority: priority, EventType: EventType, SourceStationID: p.Message.From, DestinationStationID: destination, Delivery: &sim.FrameDeliveryMeta{PayloadLen: len(p.Raw)}})
	c.events[c.queue.LastPushedID()] = p
	c.eventCount++
	return nil
}
