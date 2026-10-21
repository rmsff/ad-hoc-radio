---
title: Canonical Frame Contract — Ad Hoc Radio
created: 2026-06-25
updated: 2026-10-02
status: draft
type: protocol-contract
---

# Canonical Frame Contract

## 0. Status and Authority

This document is the canonical contract for simulator-level frames.

Other docs may summarize fields, but implementation must follow this file for:

- frame fields;
- frame identity keys;
- duplicate suppression keys;
- flag bit positions;
- check algorithms;
- field serialization order;
- sequence behavior;
- relay loop-control behavior;
- call lifecycle frame semantics;
- drop reason enum;
- deterministic event hashing.

This is not a final over-the-air binary format. It is the evolving simulator/protocol-core contract from MVP-0 through the accepted MVP-9 direction. ADR-0011 accepts the compressed relay-hop wire adapter implemented in MVP-7.2, but that adapter must expand back to this canonical contract before receiver, relay, metrics, or event-hash behavior. ADR-0012 accepts the short source/direct voice adapter implemented in MVP-7.6, with the same canonical-expansion rule. ADR-0015 adds simulator-level bounded relay-tree schedule semantics without accepting an OTA schedule encoding.

## 1. Design Principles

### MVP-9.9 Event And Evidence Qualification

For the scheduled runtime, each receiver has a deadline at
`source_created_at + jitter_buffer_ms`, evaluated before final call cleanup.
A missing sequence advances at its deadline; a later packet cannot become
audible retroactively. No actual concealment audio is generated. Long PTT must
emit playout events before CALL_END. Arrival and playout timestamps are distinct.

Metrics retain top-level schema version 1 for additive compatibility, with an
explicit `scheduled_relay_tree.audio.version = 2` semantic boundary. New
`latency_ms` samples are creation-to-actual-playout; `audio.arrival_latency_ms`
and `latency_by_relay_depth` describe accepted arrival instead. Optional audio
summaries are null when no samples exist; legacy zero-filled numeric summary
objects do not establish measured zero latency. Consumers must require audio
version 2 for renewed service acceptance. Golden hashes change; old receipts
are not relabelled.

The expected-member denominator is the union of membership snapshots activated
during the PTT, with the full frame count owed to each member. This deliberately
conservative policy counts pre-join/post-departure omissions rather than hiding
them; it is not a catch-up service. `expected_service_outcome=fail` can make a
negative test pass, but its `audio.service_outcome` remains fail.

Peer-local proposal/ACK/commit/end/expiry follows ADR-0020. ADR-0022 adds the
default [compact control v1](compact-control-v1.md) simulator encoding;
JSON+CRC remains an explicitly selected historical comparison, not an RF packet type.
CRC is corruption detection, not authentication. Voice uses the existing short
source/compressed relay adapters with deterministic size fixtures; ADR-0021
owns the current assumed FEC/framing/turnaround envelope. The optional offline
listening harness does not replace synthetic payloads in this runtime.

1. A Relay Station forwards payload bytes and metadata; it does not decode/re-encode audio.
2. Direct and relayed copies of the same source frame share one `origin_key`.
3. Relay copies must be distinguishable for debugging and metrics through `copy_key`.
4. Frames carry enough metadata to prevent loops and stale forwarding.
5. Frame checks detect accidental corruption in the simulator; they are not a final RF FEC/CRC design.
6. Control frames define call lifecycle so caches and jitter buffers can be bounded.
7. Serialization must be deterministic so tests and event hashes are stable.
8. Route and schedule identity are call context; voice forwarding is allowed only for transmitters assigned by that context.

## 2. Frame Type Enum

| Value | Name | Purpose | Required in MVP |
| --- | --- | --- | --- |
| `0x01` | `voice` | Carries dummy or encoded voice payload. | Yes |
| `0x02` | `call_start` | Announces call start and initializes receiver/relay call state. | Yes |
| `0x03` | `call_end` | Ends call and permits cache cleanup. | Yes |
| `0x04` | `beacon` | Future neighbor discovery / relay election input. | No |
| `0x05` | `relay_offer` | Future relay election candidate offer. | No |

## 3. Canonical Frame Fields

All frames share this envelope.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `version` | `uint8` | Yes | Protocol contract version. MVP value: `1`. |
| `frame_type` | enum/`uint8` | Yes | One of the frame types in §2. |
| `net_id` | `uint16` | Yes | Logical simulation network id. Prevents cross-network duplicate-key collisions. |
| `source_id` | `uint16` | Yes | Originating Station id. Relay must not replace this. |
| `group_id` | `uint16` | Yes | Group/talkgroup id. MVP uses group call semantics only. |
| `relay_id` | `uint16` | Yes | `0` for direct/source frames; Relay Station id for relayed copies. |
| `call_id` | `uint32` | Yes | One PTT/call session id. Stable across all frames in a Call. |
| `seq` | `uint32` | Yes | Monotonic frame sequence within a Call. Starts at `0`. |
| `hop` | `uint8` | Yes | Number of relay hops already applied. Source frames use `0`. |
| `ttl` | `uint8` | Yes | Remaining relay budget. Relay decrements before forwarding. |
| `flags` | `uint16` | Yes | Bitset defined in §4. |
| `created_at_ms` | `int64` | Yes | Source-created simulation timestamp. Relay must preserve this. |
| `sent_at_ms` | `int64` | Yes | Timestamp when current copy was sent. Relay updates this on forwarding. |
| `payload_len` | `uint16` | Yes | Payload length in bytes. May be `0` for control frames. |
| `payload` | `bytes` | Yes | Dummy voice payload, codec payload, or control payload. |
| `header_crc32` | `uint32` | Yes | CRC-32/IEEE over canonical header fields except `header_crc32` and `payload_crc32`. |
| `payload_crc32` | `uint32` | Yes | CRC-32/IEEE over payload bytes. |

## 4. Canonical Binary Serialization

MVP encode/decode tests use this deterministic binary layout.

### 4.1 Byte order

All integer fields use network byte order:

```text
endianness: big-endian
signed int64: two's-complement big-endian
```

### 4.2 Header field order

The canonical header serialization order is exactly:

```text
version            uint8
frame_type         uint8
net_id             uint16
source_id          uint16
group_id           uint16
relay_id           uint16
call_id            uint32
seq                uint32
hop                uint8
ttl                uint8
flags              uint16
created_at_ms      int64
sent_at_ms         int64
payload_len        uint16
header_crc32       uint32
payload_crc32      uint32
```

For `header_crc32`, serialize the same header order but with `header_crc32 = 0` and the actual `payload_crc32` value present.

`payload` bytes are appended after the header and are not part of `header_crc32`.

### 4.3 Numeric ID mapping

Scenario configs may use human-readable station aliases (`A`, `R`, `B`), but frames store numeric ids.

Rules:

```text
station_alias: string, used only in config/logs
station_id: uint16, used in frame fields
```

The scenario config must define a one-to-one mapping:

```yaml
nodes:
  - id: A
    numeric_id: 1
  - id: R
    numeric_id: 2
  - id: B
    numeric_id: 3
```

Rules:

- `numeric_id = 0` is reserved for `relay_id=0` direct/source frames and must not be assigned to a Station.
- Duplicate aliases are invalid.
- Duplicate numeric ids are invalid.
- Metrics may include aliases, but protocol keys use numeric ids.

### 4.4 Compressed relay-hop wire adapter

ADR-0011 accepts a context-bound compressed wire representation for relayed
`voice` frames. This does not replace the canonical header order in §4.2. It is
a relay-hop serialization adapter with these boundaries:

- Applies only to relayed `voice` frames inside an active call context.
- Does not apply to source/direct `voice`, `call_start`, `call_end`, `beacon`,
  `relay_offer`, or future `relay_assign`/re-election control frames.
- Receivers must expand compressed bytes to canonical frame semantics before
  duplicate suppression, jitter handling, metrics, or event hashing.
- `origin_key` and `copy_key` must be reconstructable exactly as defined in §7.
- Payload bytes remain opaque; relay logic must not decode or re-encode codec
  payload.
- Missing, stale, or ambiguous call context must reject/drop the compressed
  frame deterministically.
- Payload integrity must remain at least canonical `payload_crc32` strength
  until a later ADR accepts a different integrity model.

The implemented MVP-7.2 compressed relay-hop layout is:

```text
compressed_version  uint8   // value: 1
extra_flags         uint8   // only late_allowed/test_corrupted; relayed implied
seq_delta           uint16  // big-endian: seq - context.base_seq
hop_ttl             uint8   // high nibble = hop, low nibble = ttl
sent_delta_ms       uint8   // sent_at_ms - created_at_ms
payload_crc32       uint32  // CRC-32/IEEE over payload bytes
envelope_crc32      uint32  // CRC-32/IEEE over context binding, bytes 0..9, payload
payload             bytes
```

Fixed compressed header size:

```text
compressed_relay_hop_header_size = 14 bytes
```

For an 8-byte Codec2-class payload this produces a 22-byte relay-hop wire
frame. Source/direct frames remain canonical semantics, except where the
ADR-0012 short source/direct voice adapter in §4.5 applies; decoded bytes must
expand back to canonical `Frame` behavior before forwarding, receiver, metrics,
or event-hash behavior.

Required active context:

```text
net_id
source_id
group_id
relay_id
call_id
base_seq
base_created_at_ms
frame_period_ms
```

Expansion rules:

```text
seq           = base_seq + seq_delta
created_at_ms = base_created_at_ms + seq_delta * frame_period_ms
sent_at_ms    = created_at_ms + sent_delta_ms
flags         = relayed | extra_flags
```

Limits:

- `seq_delta` must fit `uint16`.
- `hop` and `ttl` must each fit 4 bits (`0..15`), and `hop` must be non-zero.
- `sent_delta_ms` must fit `uint8`.
- `created_at_ms` must match the context-derived timestamp exactly.
- Overflow, missing context, stale context, unsupported flags, or CRC mismatch
  rejects the compressed frame.

`envelope_crc32` context binding serializes the required active context fields
in deterministic big-endian order before the compressed header prefix and
payload:

```text
net_id, source_id, group_id, relay_id, call_id,
base_seq, base_created_at_ms, frame_period_ms
```

### 4.5 Short source/direct voice wire adapter

ADR-0012 accepts a context-bound short wire representation for source/direct
`voice` frames. This does not replace the canonical header order in §4.2. It is
a source/direct serialization adapter with these boundaries:

- Applies only to direct/source `voice` frames inside an active call context.
- Does not apply to relayed `voice`, `call_start`, `call_end`, `beacon`,
  `relay_offer`, or future `relay_assign`/re-election control frames.
- Receivers and relays must expand short bytes to canonical frame semantics
  before relay forwarding, duplicate suppression, jitter handling, metrics, or
  event hashing.
- `origin_key` and `copy_key` must be reconstructable exactly as defined in §7.
- Payload bytes remain opaque; relay logic must not decode or re-encode codec
  payload.
- Missing, stale, or ambiguous call context must reject/drop the short frame
  deterministically.

The implemented MVP-7.6 short source/direct layout is:

```text
short_version       uint8   // value: 1
extra_flags         uint8   // only late_allowed/test_corrupted; direct implied
seq_delta           uint16  // big-endian: seq - context.base_seq
hop_ttl             uint8   // high nibble = hop (must be 0), low nibble = ttl
sent_delta_ms       uint8   // sent_at_ms - created_at_ms
payload_crc32       uint32  // CRC-32/IEEE over payload bytes
envelope_crc32      uint32  // CRC-32/IEEE over context binding, bytes 0..9, payload
payload             bytes
```

Fixed short voice header size:

```text
short_voice_header_size = 14 bytes
```

For an 8-byte Codec2-class payload this produces a 22-byte source/direct voice
wire frame.

Required active context:

```text
net_id
source_id
group_id
call_id
base_seq
base_created_at_ms
frame_period_ms
```

Expansion rules:

```text
seq           = base_seq + seq_delta
created_at_ms = base_created_at_ms + seq_delta * frame_period_ms
sent_at_ms    = created_at_ms + sent_delta_ms
relay_id      = 0
hop           = 0
flags         = direct | extra_flags
```

Limits:

- `seq_delta` must fit `uint16`.
- `ttl` must fit 4 bits (`0..15`), and `hop` must be zero.
- `sent_delta_ms` must fit `uint8`.
- `created_at_ms` must match the context-derived timestamp exactly.
- Overflow, missing context, stale context, unsupported flags, or CRC mismatch
  rejects the short frame.

`envelope_crc32` context binding serializes the required active context fields
in deterministic big-endian order before the short header prefix and payload:

```text
net_id, source_id, group_id, call_id,
base_seq, base_created_at_ms, frame_period_ms
```

### 4.6 Candidate OpenGD77 Layer-2 transport envelope

MVP-10.5 defines `adhr-opengd77-l2-envelope-v1` as a candidate-specific
27-byte transport wrapper around one unchanged 22-byte short source/direct
voice frame. Its five adapter bytes, CRC-8/ATM, reserved rules, reason codes,
event ABI, golden vectors, mode isolation, and lower-layer ownership are defined
in `docs/protocol/opengd77-l2-envelope-v1.md`.

This wrapper does not change any canonical field, identity key, call-context
requirement, routing rule, schedule rule, duplicate-suppression rule, or metrics
schema in this contract. It has only host/source-fixture evidence until a later
physical gate, and MVP-10.6 may reject the Layer-2 transport without changing
the embedded short-frame semantics.

## 5. Canonical Flags Bitset

`flags` is a `uint16` bitset. Undefined bits must be zero in MVP.

| Bit | Hex | Name | Meaning |
| --- | --- | --- | --- |
| 0 | `0x0001` | `direct` | Frame was sent directly by Source Station. |
| 1 | `0x0002` | `relayed` | Frame was sent by Relay Station. |
| 2 | `0x0004` | `call_start` | Frame starts a Call lifecycle. Usually matches `frame_type=call_start`. |
| 3 | `0x0008` | `call_end` | Frame ends a Call lifecycle. Usually matches `frame_type=call_end`. |
| 4 | `0x0010` | `late_allowed` | Receiver may accept late arrival for metrics but not playout. Optional. |
| 5 | `0x0020` | `test_corrupted` | Link model intentionally corrupted this frame for tests. |

Rules:

- Source-created `voice` frames set `direct` and clear `relayed`.
- Relay-created forwarded `voice` frames clear `direct` and set `relayed`.
- A frame must not have both `direct` and `relayed` set in MVP.
- Control frame flags should match `frame_type` where applicable.

## 6. CRC / Check Algorithms

MVP uses CRC-32/IEEE, pinned independently of language implementation:

```text
name: CRC-32/IEEE
polynomial normal: 0x04C11DB7
polynomial reflected: 0xEDB88320
init: 0xFFFFFFFF
input reflected: true
result reflected: true
final xor: 0xFFFFFFFF
check value for ASCII "123456789": 0xCBF43926
```

This matches Go `hash/crc32.ChecksumIEEE`.

Simulator rules:

- `payload_crc32` is computed over raw payload bytes.
- `header_crc32` is computed over canonical serialized header fields excluding `header_crc32` itself by setting `header_crc32 = 0` during calculation.
- Encoding must use the field order in §4.2.
- If CRC fails, decode returns invalid frame. It must not both silently accept and mark invalid.

Future RF/baseband work may replace or augment these checks with FEC/interleaving/air-interface CRCs through a later ADR.

## 7. Canonical Identity Keys

### 7.1 Origin Key

Used for duplicate suppression and relay seen-cache.

```text
origin_key = (net_id, source_id, call_id, seq)
```

### 7.2 Copy Key

Used for debugging and per-path metrics.

```text
copy_key = (net_id, source_id, relay_id, call_id, seq, hop)
```

Direct and relayed copies share an `origin_key`, but have different `copy_key` values.

## 8. Call Lifecycle

A Call is one PTT-like session.

### 8.1 `call_start`

Purpose:

- initialize receiver call state;
- initialize relay seen-cache scope;
- announce frame period and payload mode if needed;
- allow deterministic simulation metrics.

Minimum control payload fields:

```text
frame_period_ms
payload_mode
expected_frame_count optional
slot_mode_enabled
slot_duration_ms optional
ttl_default
schedule_id optional
schedule_version optional
schedule_digest optional
timing_strategy optional: source_led | group_leader
```

`frame_period_ms` is an integer in `1..60000` ms (`MaxFramePeriodMs` in
`internal/protocol`). The value 60000 is a simulator sanity bound that keeps
timestamp arithmetic inside int64; it is not a radio parameter. `call_start`
payload validation rejects any other value. These scenario validators check
the configured period against the same bound before a run:

- one-hop scenarios: `validateScenarioConfig` (`FramePeriodMs`);
- scheduled relay-tree scenarios: `validateScheduledRelayTreeProfile`
  (planning cost model `frame_period_ms`);
- CLI `relay_forward_queue_backpressure`: `cmd/adhr-sim/run_config.go`;
- two-hop live scenarios: `validateTwoHopLiveScenarioConfig`
  (`voice_period_ms`);
- group voice profile scenarios: `validateGroupVoiceProfileScenarioConfig`
  (`voice_period_ms` of every profile).

The compact_v1 control wire and the peer layer carry the period in a u16 and
allow only `1..10000` (`docs/protocol/compact-control-v1.md`). A scheduled
relay-tree scenario with peer control therefore rejects a period in
`10001..60000` when the call starts, even though its config validator accepts
it.

`MaxSafeCreatedAtMs = 2^63 - 1 - (2^32 - 1) * 60000 = 9223114338817075807`.
A station accepts a frame only when its `created_at_ms` lies in
`0..MaxSafeCreatedAtMs`; the receiver and relay lifecycle APIs reject a
`call_start` outside that range as well. Section 11 gives the order of the age
checks: a negative `created_at_ms` within the stale threshold is dropped as
`invalid`, one beyond the threshold as `stale_frame`, and a value above
`MaxSafeCreatedAtMs` as `invalid`.

`0..MaxSafeCreatedAtMs` is the limit of the simulated clock domain: the whole
call, from `call_start` to `call_end`, must lie inside it. The bound guarantees
`created_at_ms + seq * frame_period_ms <= 2^63 - 1` for every 32-bit `seq`. It
does not cover later additions to that value, such as jitter-buffer or late
deadlines, when `seq` approaches `2^32`.

For MVP-9 scheduled calls, `schedule_id`, `schedule_version`, and
`schedule_digest` are required. Legacy one-hop fixtures may omit them only while
they remain explicitly marked as pre-MVP-9 compatibility scenarios.

MVP may allow scenario metadata to create call state without transmitting an explicit `call_start` frame in the earliest unit tests. Integration scenarios include an explicit `call_start`.

### 8.2 `voice`

Rules:

- `call_id` must reference an active Call or scenario-defined Call.
- `seq` starts at `0` for each Call and increases by one per source frame.
- Relay must preserve `created_at_ms`, `source_id`, `group_id`, `call_id`, `seq`, and `payload`.
- Relay must update `relay_id`, `hop`, `ttl`, `flags`, and `sent_at_ms`.

### 8.3 `call_end`

Purpose:

- mark source call completion;
- allow receiver to flush jitter buffer;
- allow relay/receiver to evict per-call caches after a grace period.

Minimum control payload fields:

```text
last_seq
reason: normal | timeout | aborted
```

### 8.4 Active relay-tree schedule context

ADR-0015 defines a semantic schedule context for every scheduled call. The
simulator contract is:

```text
schedule_id             uint32
schedule_version        uint16
net_id                   uint16
group_id                 uint16
source_id                uint16
call_id                  uint32
schedule_authority_id    uint16
topology_epoch          uint32
topology_observed_at_ms  int64
membership_snapshot_id   uint32
membership_observed_at_ms int64
membership_freshness_ms  uint32
timing_strategy         source_led | group_leader
timing_authority_id     uint16
max_relay_depth         uint8, range 0..3
effective_seq           uint32
valid_through_seq       uint32, inclusive
expected_member_ids     ordered unique uint16 list
covered_member_ids      ordered unique uint16 list
uncovered_member_ids    ordered unique uint16 list
opportunities           ordered list
```

Each transmitter opportunity contains:

```text
opportunity_index       uint16
owner_station_id        uint16
parent_station_id       uint16, 0 for source
hop                     uint8, range 0..3
offset_ms               uint32
duration_ms             uint16
guard_after_ms          uint16
```

Schedule validity rules:

- `schedule_key = (net_id, group_id, source_id, call_id, schedule_id)`;
- `schedule_id` is non-zero and stable for the call; `schedule_version` starts
  at `1`, increases monotonically, and must not wrap within a call;
- `schedule_authority_id` owns the canonical membership/topology snapshot used
  by this version; peers do not merge their local snapshots into it implicitly;
- `expected_member_ids` are subscribed recipients observed by the authority at
  `membership_observed_at_ms`, exclude `source_id`, and remain fixed for this
  version even when a member later becomes stale or unavailable;
- `effective_seq <= valid_through_seq`; this version can authorize only source
  sequences in that inclusive range;
- opportunity indices are contiguous and strictly ordered;
- opportunity `0` is owned by the source with `parent_station_id=0` and `hop=0`;
- every relay owner is relay-eligible and appears after an opportunity from its
  selected parent;
- `hop` equals the number of relay retransmissions from the source and never
  exceeds `max_relay_depth` or `3`;
- the safe baseline has one owner per opportunity;
- every expected member is present in exactly one of `covered_member_ids` or
  `uncovered_member_ids`;
- a schedule update increments `schedule_version`; stale versions cannot
  authorize voice transmission;
- a newer version takes effect only at its `effective_seq`, which must be greater
  than the highest source sequence already created when the update is accepted;
  the previous version cannot authorize that sequence or any later sequence;
- topology/membership changes never mutate an active version in place; a replan
  creates a new authority-owned snapshot and schedule version;
- if no active version covers a sequence, no relay may transmit it; direct-only
  fallback also requires an explicit schedule version;
- an already-started source sequence is not retried after relay failure; missed
  delivery is recorded and any replan applies to a future sequence;
- the deterministic schedule digest is lowercase hexadecimal SHA-256 over
  `canonical-json-v1` for all schedule fields above, excluding the digest itself;
  object keys are lexicographically sorted and arrays retain their stated order.

Safe-baseline opportunity order is canonical:

```text
1. source opportunity
2. relay owners sorted by:
   hop ascending
   parent_station_id ascending
   owner_station_id ascending
```

At the same simulated timestamp, an accepted schedule-version update uses event
priority `15`: after slot boundary priority `10`, before source tick priority
`20`. This makes activation for `effective_seq` deterministic.

Simulator baseline replan policy:

```text
replan triggers:
  topology_epoch_changed
  membership_snapshot_changed
  selected_relay_unavailable
  timing_authority_lost
  consecutive_missed_parent_threshold reached (default 2, configurable)

on trigger:
  run planner against the new authority snapshot
  if canonical tree/schedule changes:
    publish next monotonic version
    effective_seq = highest_source_seq_created + 1
  else if scheduled voice is explicitly blocked and the plan is feasible:
    publish next monotonic version as explicit recovery authorization
    effective_seq = highest_source_seq_created + 1
  else:
    retain current version

on no feasible result:
  no implicit forwarding or flooding
  an explicit direct-only version may be published if direct delivery is feasible
```

`topology_epoch_changed` includes partition recovery. A late join affects
planning only through `membership_snapshot_changed`. This policy is simulator
semantics; the on-air trigger transport remains deferred.

The consecutive-missed-parent threshold is scoped to each assigned relay, not
summed across all relays. A successful sibling does not reset another relay's
streak; cascading misses for one source sequence cannot count as multiple
missed source frames for one relay. Version activation resets the streaks, and
simultaneous threshold triggers for the same source sequence are coalesced into
one replan attempt. This local simulator rule is not distributed fault consensus.
If no future source frame remains, no replan is attempted; the call completes
with its recorded missed-delivery counters instead of creating an invalid lease.

This remains simulator/protocol-core context, not an accepted over-the-air MAC.
ADR-0020 implements directed local agreement; ADR-0022 defines its compact binary
encoding. Real radio routing/framing, fragmentation and synchronization remain
deferred. These boundaries do not change semantic fields or assigned-relay-only
voice forwarding.

## 9. Schedule Cycle / Frame Period Invariant

For a scheduled call:

```text
opportunity_end_ms = offset_ms + duration_ms + guard_after_ms
schedule_cycle_duration_ms = max(opportunity_end_ms)
frame_period_ms >= schedule_cycle_duration_ms
```

One original source Voice Frame is produced per schedule cycle. The source and
every selected relay get only their named transmitter opportunity for that
frame. Branch width may add opportunities beyond
`selected_tree.max_relay_depth + 1`.

The historical two-slot one-hop fixture remains valid:

```text
opportunity 0: source
opportunity 1: one assigned relay
schedule_cycle_duration_ms = slot_duration_ms * 2
```

Non-matching frame periods are allowed only in explicit stress experiments and must record queue depth, queue drops, and out-of-slot attempts.

## 10. TTL and Hop Rules

Source-created voice frames use:

```text
hop = 0
ttl = configured_ttl
```

Relay forwarding rule:

```text
if ttl <= 0: drop ttl_expired
else:
    forwarded.hop = original.hop + 1
    forwarded.ttl = original.ttl - 1
```

For a selected relay tree:

```text
0 <= configured_ttl <= 3
configured_ttl = selected_tree.max_relay_depth
```

TTL is a loop-control ceiling, not authorization to forward. A Station may
forward only when it is the owner of the matching opportunity in the active
schedule version. This permits direct through three-relay paths while preventing
an eligible but unassigned peer from flooding a voice copy.

## 11. Stale Frame Rule

Relay and receiver must reject frames older than the configured stale threshold.

Initial simulator defaults:

```text
relay_stale_threshold_ms = 500
receiver_stale_threshold_ms = 1000
```

Age check, in this order:

```text
if frame.created_at_ms > now_ms:
    drop invalid                      # future frame
if age(now_ms, frame.created_at_ms) > threshold:
    drop stale_frame
if frame.created_at_ms < 0 or frame.created_at_ms > MaxSafeCreatedAtMs:
    drop invalid                      # section 8.1
```

`age` must not overflow: `now_ms - created_at_ms` in signed 64-bit arithmetic
wraps when `created_at_ms` is near the int64 minimum, and an ancient frame
would then pass as fresh. Once `created_at_ms <= now_ms` is known, compute the
age as an unsigned 64-bit difference and compare it with the threshold as an
unsigned value. The stale check runs before the range check, so an ancient
frame stays `stale_frame`, while a negative `created_at_ms` within the
threshold is `invalid`. A `call_start` that fails any of these checks is
rejected and creates no call state.

The values are simulation defaults, not final RF values.

## 12. Sequence Rules

- `seq` is a `uint32` and starts at `0` for each Call.
- MVP scenarios must not generate enough frames to wrap `seq`.
- If wraparound is ever tested, it requires a dedicated ADR and modular comparison tests.
- Receiver missing-sequence logic is per `(net_id, source_id, call_id)`.

## 13. Seen-Cache and Eviction Rules

Relay seen-cache is keyed by `origin_key`.

MVP cache policy:

```text
max_entries: scenario-configurable, default 4096
eviction: oldest insertion first within call scope
call_end_cleanup_grace_ms: 2000
```

Rules:

- Relay must not forward the same `origin_key` twice.
- Cache overflow must be counted in metrics.
- Eviction must be deterministic.
- `call_end` schedules cleanup of per-call seen-cache entries after grace period.

## 14. Forward Queue Backpressure

Relay may need to queue frames when slot mode is enabled.

Initial MVP policy:

```text
forward_queue_max_depth: scenario-configurable, default 32
overflow_policy: drop_newest
```

Rules:

- If queue is full, Relay drops the new forward candidate with reason `queue_full`.
- Queue drops increment `relay.forward_queue_drops`.
- `relay.forward_queue_max_depth` records the peak depth observed during the scenario.
- The queue-full check applies only after the frame is otherwise a valid forward
  candidate. Earlier validation failures keep their normal reasons:
  `invalid`, `wrong_group`, `stale_frame`, `already_seen`, `ttl_expired`, or
  `call_not_active`.
- In scheduled simulation, the forward queue may be modeled as FIFO
  transmitter-opportunity reservations. One opportunity must carry at most one
  queued voice frame for its assigned owner in the safe baseline.
- If a queued/reserved candidate would be stale at its reserved transmit time,
  Relay drops it as `stale_frame` and does not schedule an R->B delivery.
- Future policies such as `drop_oldest` require an ADR because they change voice continuity behavior.

## 15. Drop Reason Enum

Runtime drop reasons must use this enum:

```text
invalid
wrong_group
stale_frame
already_seen
ttl_expired
queue_full
call_not_active
late_frame
unsupported_frame_type
schedule_mismatch
schedule_expired
not_scheduled
reserved
```

Metrics schema restricts `dropped_by_reason` keys to this enum.

MVP-9 meanings:

- `schedule_mismatch`: call key, schedule id/version, source sequence, or parent
  does not match the candidate's active schedule context;
- `schedule_expired`: no active schedule version leases the source sequence;
- `not_scheduled`: the Station is not the owner of an opportunity for this
  candidate.

A missing parent copy at opportunity time increments
`missed_opportunity_count`; no frame exists to classify as a dropped candidate.

## 16. Event Ordering Determinism

If multiple simulation events have the same `time_ms`, process them by deterministic tie-breakers:

```text
1. time_ms ascending
2. event_priority ascending
3. source_station_id ascending
4. destination_station_id ascending
5. monotonically increasing event_id ascending
```

Recommended event priorities:

| Priority | Event |
| --- | --- |
| `10` | slot boundary |
| `15` | schedule-version activation or explicit forwarding block |
| `20` | source tick |
| `30` | frame delivery |
| `35` | scheduled relay transmit / reserved-opportunity consumption |
| `40` | jitter buffer tick |
| `50` | metrics flush |

At equal `time_ms`, priority `30` delivery is therefore visible to the relay
before its priority `35` transmit opportunity. A frame arriving exactly at the
opportunity start is eligible; a later frame is not retried in that opportunity.

## 17. Event Hash Contract

Metrics determinism uses:

```text
hash_algorithm: sha256
serialization: canonical-jsonl-v1
```

Canonical JSONL event serialization rules:

- one event per line;
- UTF-8;
- LF line ending;
- JSON object keys sorted lexicographically;
- no insignificant whitespace;
- events emitted after deterministic event ordering;
- event fields must include at least `time_ms`, `event_priority`, `event_id`, `event_type`, `source_station_id`, `destination_station_id`, and relevant `origin_key`/`copy_key` where applicable.

`event_hash` is lowercase hex SHA-256 over the full canonical JSONL byte stream.

## 18. Group Semantics

MVP uses group-call semantics only.

Rules:

- A Station receives/relays only frames whose `group_id` is in its subscribed groups.
- Relay does not rewrite `group_id`.
- Scheduled calls bind a freshness-bounded expected-group snapshot. Every
  expected member is classified as covered or uncovered before delivery, and
  delivery metrics must not silently omit either set.
- Unicast and broadcast semantics are out of scope for MVP and require a later ADR.
- `group_id = 0` is reserved and invalid for MVP voice frames.

## 19. Backward Compatibility

No backward compatibility is guaranteed until implementation begins and the first simulator version is tagged.

Any later change to this frame contract must update:
- `docs/protocol/metrics-schema.json` if metrics change;
- tests and fixtures.
