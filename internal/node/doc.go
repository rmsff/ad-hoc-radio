// Package node implements station role state machines: source, relay,
// and receiver.
//
// Boundary: this package owns per-station behavior — frame emission
// (source), forwarding with seen-cache and TTL handling (relay), and
// duplicate suppression / jitter interaction (receiver). It depends on
// protocol and transport abstractions, never on RF/baseband details.
//
// Relay must forward frame payload bytes without decoding or
// re-encoding audio (AD-3).
//
// Status: call lifecycle state exists for relay and receiver (Story
// 2.0). Forwarding, duplicate suppression, and source emission land
// in later Epic 2 stories.
package node
