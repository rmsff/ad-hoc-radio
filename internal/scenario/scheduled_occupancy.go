package scenario

import "fmt"

type ScheduledInterferenceWindow struct{ StartMS, EndMS int64 }
type scheduledTransmission struct {
	Owner          uint16
	StartMS, EndMS int64
	Collided       bool
}

func (r *scheduledRuntime) reserveTransmission(owner uint16, startMS int64, durationMS uint16) (int, error) {
	end, ok := checkedScheduledTime(startMS, uint64(durationMS))
	if !ok {
		return 0, fmt.Errorf("scheduled runtime: TX end overflows")
	}
	next := scheduledTransmission{Owner: owner, StartMS: startMS, EndMS: end}
	for i := range r.transmissions {
		prior := &r.transmissions[i]
		if startMS < prior.EndMS && prior.StartMS < end {
			prior.Collided = true
			next.Collided = true
		}
	}
	for _, window := range r.cfg.Interference {
		if startMS < window.EndMS && window.StartMS < end {
			next.Collided = true
		}
	}
	r.transmissions = append(r.transmissions, next)
	return len(r.transmissions) - 1, nil
}
