# OpenGD77 Layer-2 Envelope v1

Status: host/source fixture for MVP-10.5  
Identifier: `adhr-opengd77-l2-envelope-v1`

## Scope

This candidate adapter carries one unchanged 22-byte ADHR short source/direct
voice frame in the 27-byte buffer exposed by the investigated HR-C6000 Layer-2
voice path. It does not replace canonical `Frame` semantics, active call
context, identity keys, routing, scheduling, duplicate suppression, or metrics.

The adapter is not a DMR interoperability claim. Its current evidence grade is
`host_fixture_only`; no RF frame has been received.

## Wire Layout

All multi-byte fields inside the embedded short frame remain big-endian as
defined by `frame-contract.md` section 4.5. The five adapter bytes are:

| Offset | Size | Field | v1 rule |
|---:|---:|---|---|
| 0 | 1 | `discriminator` | exactly `0xA7`; every other value is reserved and rejected |
| 1 | 1 | `envelope_version` | exactly `0x01`; every other value is rejected |
| 2 | 1 | `payload_length` | exactly decimal `22` / `0x16` |
| 3 | 1 | `reserved` | exactly `0x00`; all bits are reserved and must be zero |
| 4 | 1 | `envelope_crc8` | CRC-8/ATM over offsets `0..3` followed by `5..26` |
| 5 | 22 | `short_voice` | unchanged adapter bytes from frame-contract section 4.5 |

Total size is exactly `27` bytes. There is no implicit padding and no extension
area in v1. Truncated and oversized buffers are rejected before field parsing.

## Integrity And FEC Ownership

`envelope_crc8` uses:

```text
polynomial = 0x07
initial = 0x00
xorout = 0x00
reflect_input = false
reflect_output = false
coverage = wire[0:4] || wire[5:27]
```

This CRC protects candidate adapter discrimination and transport corruption. It
does not weaken or replace the embedded short frame's CRC-32/IEEE payload and
context-binding checks. A later expansion to canonical `Frame` must still pass
`DecodeShortVoice` with the exact active call context.

For the Layer-2 candidate only, HR-C6000 owns lower-layer burst framing, FEC,
sync detection, and any DMR-derived physical cadence. The adapter does not claim
those mechanisms are suitable for ADHR; MVP-10.6 decides that question.

## Receive Results

Closed reason codes shared by C and Go fixtures:

| Code | Name | Meaning |
|---:|---|---|
| 0 | `accepted` | valid envelope, 22-byte payload copied into event |
| 1 | `wrong_mode` | active mode is not ADHR; parser state is untouched |
| 2 | `truncated` | input is shorter than 27 bytes or null |
| 3 | `oversize` | input is longer than 27 bytes |
| 4 | `wrong_discriminator` | byte 0 is not `0xA7` |
| 5 | `wrong_version` | byte 1 is not `0x01` |
| 6 | `wrong_length` | byte 2 is not `22` |
| 7 | `reserved` | byte 3 is non-zero |
| 8 | `integrity` | CRC-8/ATM mismatch |
| 9 | `timeout` | lifecycle reset caused by timeout |
| 10 | `queue_pressure` | fixed queue is full; existing events are not overwritten |
| 11 | `reset` | explicit receive-state reset |
| 12 | `disconnect` | receive source disconnected |

Malformed inputs in ADHR mode are consumed and cannot fall through to standard
voice decoding. The same bytes in Analog FM return `wrong_mode` without changing
sequence, queue, or parser state.

## Event ABI

The host/source spike uses a fixed-capacity queue of four events. Events have a
deterministic 34-byte serialization:

```text
event_version    uint8   // 1
event_kind       uint8   // 1=frame, 2=rejected, 3=lifecycle
reason           uint8
payload_length   uint8   // 22 only for accepted frame; otherwise 0
sequence         uint32  // big-endian
observed_tick    uint32  // big-endian candidate-local tick
payload          [22]byte
```

Queue pressure never overwrites a queued event. Timeout, reset, and disconnect
clear stale queue state and emit one lifecycle event. The adapter has no heap,
codec, TX, or RF dependency.

## Golden Vectors

Positive canonical fixture:

```text
short22 = 011000020314cbf0b66eb710f7eb1020304050607080
wire27  = a7011600fc011000020314cbf0b66eb710f7eb1020304050607080
crc8    = fc
```

The embedded frame expands with the fixture context:

```text
net_id=1 source_id=10 group_id=7 call_id=0x9001
base_seq=40 base_created_at_ms=1000000 frame_period_ms=80
result: seq=42 hop=0 ttl=3 payload=1020304050607080
```

Negative vectors mutate exactly one condition unless the size itself is under
test: discriminator `00`, version `02`, length `15`, reserved `01`, CRC `fd`,
26-byte truncation, and one-byte oversize. Wrong-mode and lifecycle/pressure
fixtures are state-machine cases rather than alternate wire encodings.

## No-Claim Boundary

```text
evidence_grade=host_fixture_only
rf_frame_received=false
dmr_interoperability=false
hardware_ready=false
firmware_write_allowed=false
field_tx_ready=false
```
