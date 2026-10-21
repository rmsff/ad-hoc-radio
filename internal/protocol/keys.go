package protocol

// OriginKey is the canonical duplicate-suppression and relay seen-cache
// key per frame-contract.md §7.1.
//
// Direct and relayed copies of the same source frame share one
// OriginKey. It is a value type and is comparable, so it can be used as
// a map key by relay seen-cache (Story 2.1) and receiver duplicate
// suppression (Story 2.2).
type OriginKey struct {
	NetID    uint16
	SourceID uint16
	CallID   uint32
	Seq      uint32
}

// CopyKey distinguishes direct and relayed copies of the same source
// frame for debugging and per-path metrics per frame-contract.md §7.2.
//
// Direct and relayed copies share an OriginKey but have different
// CopyKey values (different relay_id and/or hop).
type CopyKey struct {
	NetID    uint16
	SourceID uint16
	RelayID  uint16
	CallID   uint32
	Seq      uint32
	Hop      uint8
}

// OriginKey returns the canonical duplicate-suppression key for f per
// frame-contract.md §7.1: (net_id, source_id, call_id, seq).
//
// AC-3 of Story 1.2: direct and relayed copies of the same source
// frame produce identical OriginKey values.
func (f Frame) OriginKey() OriginKey {
	return OriginKey{
		NetID:    f.NetID,
		SourceID: f.SourceID,
		CallID:   f.CallID,
		Seq:      f.Seq,
	}
}

// CopyKey returns the per-copy debugging key for f per frame-contract.md
// §7.2: (net_id, source_id, relay_id, call_id, seq, hop).
func (f Frame) CopyKey() CopyKey {
	return CopyKey{
		NetID:    f.NetID,
		SourceID: f.SourceID,
		RelayID:  f.RelayID,
		CallID:   f.CallID,
		Seq:      f.Seq,
		Hop:      f.Hop,
	}
}
