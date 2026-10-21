// Package sim implements the simulation substrate: topology, link
// loss/delay/duplication models, the deterministic event queue, and
// metrics emission.
//
// Boundary: this package wires stations together via transport, drives
// them with the timing model, and produces schema-valid metrics JSON
// per docs/protocol/metrics-schema.json (AD-8). Tests in this package
// and downstream must be deterministic with fixed seeds (NFR-1).
//
// Status: stub. Topology and loss/delay simulation land in Story 1.3;
// metrics JSON in Story 3.3.
package sim
