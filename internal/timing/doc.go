// Package timing implements the simulator timing model.
//
// Boundary: this package owns SimulatedClock, slot/superframe
// scheduling, and the jitter buffer. The MVP uses an idealized
// SimulatedClock; real RF slot synchronization, guard time, drift, and
// late entry are deferred until a timing ADR based on
// docs/research/002-ad-hoc-sfr-synchronization.md (AD-7).
//
// Status: slot/superframe scheduler lands in Story 3.1; jitter buffer in
// Story 3.2.
package timing
