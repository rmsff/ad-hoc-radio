package sim

import "math"

// SimulatedClock is the ideal monotonic clock for MVP simulation per
// ARCHITECTURE-SPINE.md AD-7 and algorithm-and-experiments.md §4.3.
//
// Time unit: milliseconds. The clock only advances when explicitly
// driven; it has no wall-clock dependency, so tests are deterministic
// (NFR-1). Real RF slot synchronization is deferred per AD-7.
type SimulatedClock struct {
	nowMs int64
}

// NewSimulatedClock returns a clock starting at t=0.
func NewSimulatedClock() *SimulatedClock {
	return &SimulatedClock{}
}

// Now returns the current simulated time in milliseconds.
func (c *SimulatedClock) Now() int64 {
	return c.nowMs
}

// Advance moves the clock forward by ms milliseconds. Negative values
// and overflows of the int64 field are rejected: simulated time is
// monotonic and bounded. A panic surfaces programmer/configuration
// errors loudly rather than producing a wrapped, non-monotonic clock.
func (c *SimulatedClock) Advance(ms int64) {
	if ms < 0 {
		panic("SimulatedClock.Advance: negative advance not allowed")
	}
	if c.nowMs > math.MaxInt64-ms {
		panic("SimulatedClock.Advance: int64 overflow")
	}
	c.nowMs += ms
}

// Set moves the clock to an absolute time. Useful for tests that want
// to pin a start time. t must not be less than the current time.
func (c *SimulatedClock) Set(t int64) {
	if t < c.nowMs {
		panic("SimulatedClock.Set: cannot move backwards")
	}
	c.nowMs = t
}

// AddMs returns a + b in simulated-millisecond space, panicking on
// int64 overflow or negative result. It is the shared overflow-safe
// addition used by the transport for delay and arrival-time
// arithmetic so that a misconfigured link cannot produce a wrapped or
// negative simulated time.
func AddMs(a, b int64) int64 {
	if b < 0 || a < 0 {
		panic("sim.AddMs: negative operand not allowed")
	}
	if a > math.MaxInt64-b {
		panic("sim.AddMs: int64 overflow")
	}
	return a + b
}
