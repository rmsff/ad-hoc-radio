// Package codec defines the codec boundary for voice payloads.
//
// Boundary: a dummy codec provides deterministic payload bytes for
// protocol/relay testing without real audio compression. A codec spy
// proves relay does not call Decode or Encode (AD-3, NFR-3). Codec2
// integration is a later, isolated adapter and is not an MVP
// dependency (AR-3: no AMBE; Codec2 path decided in Story 4.2).
//
// Status: dummy codec and codec spy implemented in Story 4.1.
package codec
