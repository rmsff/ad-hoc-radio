# Compact Peer Control v1

Status: implemented simulator codec under ADR-0022. All multi-byte fields use
big endian, with no alignment padding. Maximum input is 2048 bytes. CRC32/IEEE
is appended as a big-endian u32 over every preceding byte. CRC and the digest
detect corruption/conflicts; neither authenticates a peer.

## Common Header

| Offset | Bytes | Field |
|---:|---:|---|
| 0 | 2 | Magic ASCII `AC` |
| 2 | 1 | Encoding version, exactly 1 |
| 3 | 1 | Kind: 1 Proposal, 2 ACK, 3 Commit, 4 End, 5 Sync |
| 4 | 2 | Logical source of this control message, nonzero |
| 6 | 2 | Logical destination, nonzero and different from source |
| 8 | 8 | Nonzero message nonce; duplicate key retains nonce and route hop |
| 16 | 32 | Full binary SHA-256 from `OpportunityScheduleHash` |
| 48 | 2 | Schedule version, nonzero |
| 50 | 8 | Epoch in simulated ms, 0 through 1e12 |
| 58 | 2 | Period in ms, 1 through 10000 |
| 60 | variable | Proposal body only; empty for references |
| final 4 | 4 | CRC32/IEEE |

Every non-Proposal is exactly 64 bytes. The digest includes call/network/group,
schedule identity, version, membership, timing metadata and opportunities via
the unchanged canonical schedule contract. Epoch and period additionally bind
the reference. `ReceivedMS`, `AcquiredMS`, `Committed`, clock correction and
other local-only Permit state are not transmitted. Expiry is derived from the
received schedule horizon, epoch and period with checked bounds.

## Proposal Body

Fields in wire order:

```text
schedule_id u32, net_id u16, group_id u16, source_station_id u16,
call_id u32, topology_epoch u32, topology_observed_at_ms i64,
membership_snapshot_id u32, membership_observed_at_ms i64,
membership_freshness_ms u32, effective_seq u16, valid_through_seq u16,
max_relay_depth u8,
expected_count u8, expected_ids[expected_count] u16,
covered_count u8, covered_ids[covered_count] u16,
uncovered_count u8, uncovered_ids[uncovered_count] u16,
opportunity_count u8, opportunities[opportunity_count]
```

Each opportunity is 15 bytes:

```text
index u16, owner_station_id u16, parent_station_id u16, hop u8,
offset_ms u32, duration_ms u16, guard_after_ms u16
```

Counts are individually bounded by 64 before allocation; schedule validation
additionally enforces canonical order, coverage, uniqueness, dependency order,
depth, timing and source authority. Sequence horizon is at most 65535, as in
the existing peer controller. Source-led strategy and both authority IDs equal
to the source are implicit; unsupported strategies cannot be encoded. A
Proposal sender must equal the schedule source. The reconstructed canonical
schedule must reproduce the entire transmitted SHA-256 digest. Unexpected
suffixes, malformed fields, truncation and unknown versions/kinds are rejected.

Size is `115 + 2*(expected+covered+uncovered) + 15*opportunities`, including
header and CRC. A direct two-peer Proposal is 134 bytes; a three-peer,
one-relay Proposal is 153 bytes. No inventory truncation or fragmentation occurs.

## Local State Requirement

Forwarding is opaque over the existing bounded directed route model. At the
logical destination a reference resolves only against its own pending or active
permit, matching digest/version/epoch/period. ACK targets the schedule source;
other references originate there. Existing transaction and eligibility checks
then apply. Controller-global permits and event-attached source metadata must
never reconstruct a missing destination permit.

Unknown references are rejected. Delayed Commit cannot backdate authority;
old End cannot release a different call; lost Sync cannot refresh a clock.
Local activation/expiry/End also match the full reference, including epoch and
period even when the schedule digest is equal. Reacquisition must find the
source's active committed transaction with that exact reference; a stale
controller or missing transaction returns an error without sending Sync.
References do not establish authentication or a general on-air routing header.
The model still supplies observed directed paths; discovery and radio routing
framing/fragmentation remain unimplemented and must be budgeted before RF.

## CLI And Evidence

`control.wire_format` accepts `compact_v1`, `json_crc_v0`, or omission (compact).
The simulation metrics schema shape remains unchanged: `control.bytes` and
`control.airtime_us` charge the selected encoding per actual directed hop.
Use the paired `control-comparison` report's explicit `wire_format` field when
comparing encodings. `peer-acceptance` deliberately retains the historical JSON
configuration; it is not the new compact acceptance report.

Tests: `TestCompactRoundTripAndSizes`, `TestCompactHeaderGolden`,
`TestCompactMalformed`, `TestCompactReferenceRequiresLocalExactPermit`,
`TestCompactMaximumInventory`, `FuzzCompactControl`, existing peer fault/clock/
replan tests, and the JSON-to-CLI byte-accounting regression.
Timeline/regression coverage is in `control_timeline_test.go`.
