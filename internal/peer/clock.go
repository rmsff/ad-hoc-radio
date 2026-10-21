package peer

import "sort"

func (c *Controller) acquire(l *localState, p *Permit, at int64) {
	p.ReceivedMS = at
	p.AcquiredMS = at + l.Clock.AcquisitionMS
	// Offset is observable only after delivered timing control. ResidualErrorMS
	// represents bounded one-way timing-estimation error, not perfect RF sync.
	p.ClockCorrectionMS = l.Clock.OffsetMS + at*l.Clock.DriftPPM/1000000 - l.Clock.ResidualErrorMS
}
func (c *Controller) mappedTime(l *localState, p *Permit, nominal int64) int64 {
	if !validTime(nominal) {
		return -1
	}
	value := (nominal + p.ClockCorrectionMS - l.Clock.OffsetMS) * 1000000 / (1000000 + l.Clock.DriftPPM)
	if !validTime(value) {
		return -1
	}
	return value
}
func (c *Controller) TransmitAt(id uint16, nominalMS int64) int64 {
	l := c.locals[id]
	if l == nil || l.Active == nil || !c.Matches(id) {
		return nominalMS
	}
	if id == c.source {
		return nominalMS
	}
	return c.mappedTime(l, l.Active, nominalMS)
}
func (c *Controller) Matches(id uint16) bool {
	l := c.locals[id]
	return l != nil && l.Active != nil && l.Active.Committed && sameCall(*l.Active, c.permit) && sameTimeline(*l.Active, c.permit)
}
func (c *Controller) Active(id uint16) bool { return c.Matches(id) }
func (c *Controller) CanTransmit(id uint16, seq uint32, at int64) bool {
	l := c.locals[id]
	if l == nil || l.Active == nil {
		return false
	}
	return c.CanTransmitVersion(id, l.Active.Schedule.Binding.ScheduleVersion, seq, at)
}
func (c *Controller) CanTransmitVersion(id, version uint16, seq uint32, at int64) bool {
	if !c.AcceptVoice(id, version, seq, at) {
		c.stats.ExpiredTX++
		return false
	}
	l := c.locals[id]
	for _, op := range l.Active.Schedule.Opportunities {
		if op.OwnerStationID == id {
			return id == c.source || l.RelayEnabled && l.BatteryFraction >= c.cfg.MinBattery
		}
	}
	c.stats.ExpiredTX++
	return false
}

// AcceptVoice binds the frame's version to this peer's received local permit.
// It never compares against a source-global "latest" schedule as authority.
func (c *Controller) AcceptVoice(id, version uint16, seq uint32, at int64) bool {
	l := c.locals[id]
	if !available(l, at) || !c.Matches(id) {
		return false
	}
	p := l.Active
	b := p.Schedule.Binding
	if b.ScheduleVersion != version || seq < b.EffectiveSeq || seq > b.ValidThroughSeq || at < p.AcquiredMS || at < activationMS(*p) || at >= p.ExpiresMS {
		return false
	}
	elapsed := at - p.ReceivedMS
	if elapsed < 0 {
		return false
	}
	uncertainty := abs(l.Clock.ResidualErrorMS) + (elapsed*l.Clock.DriftBoundPPM+999999)/1000000
	estimated := at + l.Clock.OffsetMS + at*l.Clock.DriftPPM/1000000 - p.ClockCorrectionMS
	return uncertainty <= l.Clock.MaxUncertaintyMS && estimated < p.ExpiresMS
}
func (c *Controller) orderedTransactions() []*transaction {
	txs := make([]*transaction, 0, len(c.transactions))
	for _, tx := range c.transactions {
		txs = append(txs, tx)
	}
	sort.Slice(txs, func(i, j int) bool {
		return txs[i].permit.Schedule.Binding.ScheduleVersion < txs[j].permit.Schedule.Binding.ScheduleVersion
	})
	return txs
}
