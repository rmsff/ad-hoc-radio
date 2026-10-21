package transport

import (
	"fmt"
	"math"
	"math/rand/v2"
)

// SamplePacket applies Link impairments after serialization. It does not choose
// recipients or charge airtime: the caller owns the selected edge and TX window.
// A per-attempt seed makes draws independent of unrelated map/event insertion.
func SamplePacket(link Link, raw []byte, serializedAtMS int64, seed uint64) ([]Delivery, error) {
	if err := (Topology{Links: []Link{link}}).Validate(); err != nil {
		return nil, err
	}
	if serializedAtMS < 0 || link.JitterMs >= math.MaxInt64 {
		return nil, fmt.Errorf("transport: invalid packet time/jitter")
	}
	if !link.Enabled {
		return nil, nil
	}
	sampler := LocalTransport{rng: rand.New(rand.NewPCG(seed, seed))}
	if sampler.drawPct(link.LossPct) {
		return nil, nil
	}
	jitter := sampler.drawJitter(link.JitterMs)
	if link.FixedDelayMs > math.MaxInt64-serializedAtMS || jitter > math.MaxInt64-serializedAtMS-link.FixedDelayMs {
		return nil, fmt.Errorf("transport: delivery time overflows int64")
	}
	at := serializedAtMS + link.FixedDelayMs + jitter
	corrupted := len(raw) > 0 && sampler.drawPct(link.CorruptionPct)
	copyRaw := append([]byte(nil), raw...)
	if corrupted {
		copyRaw[len(copyRaw)-1] ^= 1
	}
	result := []Delivery{{From: link.From, To: link.To, Raw: copyRaw, ReceivedAt: at, Corrupted: corrupted}}
	if sampler.drawPct(link.DuplicatePct) {
		if at > math.MaxInt64-deterministicDuplicateDelayMs {
			return nil, fmt.Errorf("transport: duplicate time overflows int64")
		}
		result = append(result, Delivery{From: link.From, To: link.To, Raw: append([]byte(nil), copyRaw...), ReceivedAt: at + deterministicDuplicateDelayMs, Corrupted: corrupted, IsDuplicate: true})
	}
	return result, nil
}
