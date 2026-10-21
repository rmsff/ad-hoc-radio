package peer

import (
	"fmt"
	"maps"
	"slices"
)

func (c *Controller) processLocal(p pending, at int64) (EventResult, error) {
	tx := c.transactions[p.Message.Permit.Digest]
	l := c.locals[p.Message.To]
	switch p.Local {
	case "launch":
		if c.ended || !c.acceptProposal(l, tx.permit, at) {
			c.stats.Rejected++
			return EventResult{}, nil
		}
		tx.acked[c.source] = true
		for _, id := range tx.targets {
			if err := c.propose(tx, id, at, 0); err != nil {
				return EventResult{}, err
			}
		}
		return EventResult{}, c.tryCommit(tx, at)
	case "retry_proposal":
		if tx != nil && !c.ended && !tx.acked[p.Message.To] {
			return EventResult{}, c.propose(tx, p.Message.To, at, p.Attempt)
		}
	case "retry_commit":
		if tx != nil && !c.ended && at < activationMS(tx.permit) {
			return EventResult{}, c.sendCommit(tx, p.Message.To, at, p.Attempt)
		}
	case "activate":
		if l == nil {
			return EventResult{}, nil
		}
		permit := l.Pending[p.Message.Permit.Digest]
		if permit == nil || !samePermitReference(*permit, p.Message.Permit) || !permit.Committed || !available(l, at) {
			return EventResult{}, nil
		}
		if l.Active != nil && at < l.Active.ExpiresMS {
			if !sameCall(*l.Active, *permit) || l.Active.Schedule.Binding.ScheduleVersion >= permit.Schedule.Binding.ScheduleVersion {
				return EventResult{}, nil
			}
		}
		l.Active = permit
		delete(l.Pending, permit.Digest)
		return EventResult{Activated: l.ID, Version: permit.Schedule.Binding.ScheduleVersion, Digest: permit.Digest}, nil
	case "expire":
		return c.release(l, p.Message.Permit, at, true), nil
	}
	return EventResult{}, nil
}

// expiredPendingDigests returns the digests of expired pending permits in
// ascending order, the order in which acceptProposal releases them. The order
// is defensive: release may add an expired digest to Closed, and the sweep in
// acceptProposal removes it right after (its expiry is <= at), so map order
// could matter only in the corner case of a full Closed set.
func expiredPendingDigests(l *localState, at int64) []string {
	var expired []string
	for _, digest := range slices.Sorted(maps.Keys(l.Pending)) {
		if at >= l.Pending[digest].ExpiresMS {
			expired = append(expired, digest)
		}
	}
	return expired
}

func (c *Controller) propose(tx *transaction, id uint16, at int64, attempt int) error {
	if attempt >= c.cfg.Attempts || at >= activationMS(tx.permit) || c.ended {
		return nil
	}
	m := message{Kind: Proposal, From: c.source, To: id, Permit: tx.permit}
	if err := c.send(m, at, attempt); err != nil {
		return err
	}
	retryAt := at + c.cfg.RetryMS*(1+int64(c.source%MaxPeers))
	if attempt+1 < c.cfg.Attempts && retryAt < activationMS(tx.permit) {
		return c.push(retryAt, pending{Local: "retry_proposal", Message: m, Attempt: attempt + 1}, id)
	}
	return nil
}
func (c *Controller) acceptProposal(l *localState, p Permit, at int64) bool {
	if !available(l, at) || at+l.Clock.AcquisitionMS >= activationMS(p) {
		return false
	}
	checked, err := makePermit(p.Schedule, p.EpochMS, p.PeriodMS)
	if err != nil || checked.Digest != p.Digest || checked.ExpiresMS != p.ExpiresMS {
		return false
	}
	// A Fleet may outlive the queue that originally received a tentative.
	// Reclaim only expired pending digests; never clear a foreign active call.
	for _, digest := range expiredPendingDigests(l, at) {
		c.release(l, *l.Pending[digest], at, true)
	}
	for digest, until := range l.Closed {
		if at >= until {
			delete(l.Closed, digest)
		}
	}
	if _, closed := l.Closed[p.Digest]; closed {
		return false
	}
	if (l.Pending[p.Digest] == nil && len(l.Pending) >= MaxTransactions) || len(l.Closed) >= MaxTransactions {
		return false
	}
	if l.Active != nil && at < l.Active.ExpiresMS {
		if !sameCall(*l.Active, p) || p.Schedule.Binding.ScheduleVersion < l.Active.Schedule.Binding.ScheduleVersion {
			return false
		}
		if !sameTimeline(*l.Active, p) || p.Schedule.Binding.ScheduleVersion == l.Active.Schedule.Binding.ScheduleVersion && !samePermitReference(*l.Active, p) {
			return false
		}
	}
	for _, pending := range l.Pending {
		if pending.Committed && at < pending.ExpiresMS && !sameCall(*pending, p) {
			return false
		}
	}
	if l.Tentative != nil && at < activationMS(*l.Tentative) && !samePermitReference(*l.Tentative, p) {
		if sameCall(*l.Tentative, p) {
			if !sameTimeline(*l.Tentative, p) {
				return false
			}
			if p.Schedule.Binding.ScheduleVersion <= l.Tentative.Schedule.Binding.ScheduleVersion {
				return false
			}
		} else if precedes(*l.Tentative, p) {
			return false
		}
	}
	for _, op := range p.Schedule.Opportunities {
		if op.OwnerStationID == l.ID && l.ID != p.Schedule.Binding.SourceStationID && (!l.RelayEnabled || l.BatteryFraction < c.cfg.MinBattery) {
			return false
		}
	}
	if prior := l.Pending[p.Digest]; prior != nil {
		if !samePermitReference(*prior, p) {
			return false
		}
		l.Tentative = prior
		return true
	}
	c.acquire(l, &p, at)
	if err := c.push(p.ExpiresMS, pending{Local: "expire", Message: message{Permit: p, To: l.ID}}, l.ID); err != nil {
		return false
	}
	l.Tentative = &p
	l.Pending[p.Digest] = &p
	return true
}
func (c *Controller) tryCommit(tx *transaction, at int64) error {
	if tx.committed || c.ended || at >= activationMS(tx.permit) {
		return nil
	}
	for id := range tx.required {
		if !tx.acked[id] {
			return nil
		}
	}
	source := c.locals[c.source]
	if !available(source, at) || source.Tentative == nil || !samePermitReference(*source.Tentative, tx.permit) {
		c.stats.Rejected++
		return nil
	}
	if err := c.installCommit(source, tx.permit, at); err != nil {
		return err
	}
	if p := source.Pending[tx.permit.Digest]; p == nil || !p.Committed {
		return nil
	}
	tx.committed = true
	for _, id := range tx.targets {
		if err := c.sendCommit(tx, id, at, 0); err != nil {
			return err
		}
	}
	return nil
}
func (c *Controller) sendCommit(tx *transaction, id uint16, at int64, attempt int) error {
	if attempt >= c.cfg.Attempts || at >= activationMS(tx.permit) {
		return nil
	}
	m := message{Kind: Commit, From: c.source, To: id, Permit: tx.permit}
	if err := c.send(m, at, attempt); err != nil {
		return err
	}
	if attempt+1 < c.cfg.Attempts && at+c.cfg.RetryMS < activationMS(tx.permit) {
		return c.push(at+c.cfg.RetryMS, pending{Local: "retry_commit", Message: m, Attempt: attempt + 1}, id)
	}
	return nil
}
func (c *Controller) installCommit(l *localState, p Permit, at int64) error {
	if !available(l, at) || at >= activationMS(p) || l.Tentative == nil || !samePermitReference(*l.Tentative, p) {
		c.stats.Rejected++
		return nil
	}
	local := l.Pending[p.Digest]
	if local == nil || !samePermitReference(*local, p) {
		return nil
	}
	if local.Committed {
		return nil
	}
	if l.Active != nil && at < l.Active.ExpiresMS && !sameCall(*l.Active, p) {
		c.stats.Rejected++
		return nil
	}
	local.Committed = true
	c.stats.Commits++
	c.receipts = append(c.receipts, CommitReceipt{StationID: l.ID, ScheduleVersion: p.Schedule.Binding.ScheduleVersion, Digest: p.Digest, ReceivedAtMS: at})
	m := message{Kind: Commit, From: c.source, To: l.ID, Permit: p}
	if err := c.push(activationMS(p), pending{Local: "activate", Message: m}, l.ID); err != nil {
		return err
	}
	// Local clock expiry is conservative: never retain TX beyond source expiry.
	expiry := c.mappedTime(l, local, p.ExpiresMS)
	if expiry > p.ExpiresMS {
		expiry = p.ExpiresMS
	}
	if expiry < at {
		expiry = at
	}
	return c.push(expiry, pending{Local: "expire", Message: m}, l.ID)
}
func (c *Controller) release(l *localState, p Permit, at int64, expired bool) EventResult {
	if l == nil {
		return EventResult{}
	}
	matched := false
	if prior := l.Pending[p.Digest]; prior != nil && samePermitReference(*prior, p) {
		delete(l.Pending, p.Digest)
		matched = true
	}
	if l.Tentative != nil && samePermitReference(*l.Tentative, p) {
		l.Tentative = nil
		matched = true
	}
	activeMatches := l.Active != nil && samePermitReference(*l.Active, p)
	if (matched || activeMatches) && len(l.Closed) < MaxTransactions {
		l.Closed[p.Digest] = p.ExpiresMS
	}
	if !activeMatches {
		return EventResult{}
	}
	l.Active = nil
	if expired {
		c.stats.Expirations++
	}
	return EventResult{Ended: l.ID, Version: p.Schedule.Binding.ScheduleVersion, Digest: p.Digest, Expired: expired}
}
func (c *Controller) End(at int64) error {
	if !validTime(at) || c.source == 0 {
		return fmt.Errorf("peer: invalid End")
	}
	if c.ended {
		return nil
	}
	c.ended = true
	// Each End is digest-bound; an old controller cannot clear a new call.
	for _, tx := range c.orderedTransactions() {
		c.release(c.locals[c.source], tx.permit, at, false)
		for _, id := range tx.targets {
			if err := c.send(message{Kind: End, From: c.source, To: id, Permit: tx.permit}, at, 0); err != nil {
				return err
			}
		}
	}
	return nil
}
func (c *Controller) Reacquire(id uint16, at int64) error {
	if !validTime(at) || c.ended {
		return fmt.Errorf("peer: invalid reacquisition")
	}
	source := c.locals[c.source]
	if !c.Matches(c.source) {
		return fmt.Errorf("peer: no source permit for reacquisition")
	}
	tx := c.transactions[source.Active.Digest]
	if tx == nil || !tx.committed || !samePermitReference(tx.permit, *source.Active) {
		return fmt.Errorf("peer: no matching active transaction for reacquisition")
	}
	for attempt := 0; attempt < c.cfg.Attempts; attempt++ {
		when := at + int64(attempt)*c.cfg.RetryMS
		if when >= source.Active.ExpiresMS {
			break
		}
		if err := c.send(message{Kind: Sync, From: c.source, To: id, Permit: tx.permit}, when, attempt); err != nil {
			return err
		}
	}
	return nil
}
