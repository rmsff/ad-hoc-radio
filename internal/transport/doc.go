// Package transport provides the transport abstraction used by the
// simulator.
//
// Boundary: protocol and node logic run over an abstract transport.
// The initial implementation is local in-process delivery; UDP may be
// added later. Baseband and RF transports are deferred adapters and
// must not be referenced by protocol or node code (AD-5).
//
// Status: stub. Local transport and topology land in Story 1.3.
package transport
