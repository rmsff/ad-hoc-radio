# Architecture Decisions

A digest of the 22 architecture decision records (ADR-0001 to ADR-0022,
June to September 2026), with each decision's reason, consequences and standing
after the full solution review of 2026-10-02. The records themselves are not
published.

The review's verdict: **the protocol core is sound; the physical premise
(live voice, up to three relays, one shared channel, a stock TYT MD-UV380 as
the link) is not viable**, because several ADRs rest on unmeasured values
(19.2 kbit/s, 1 ms turnaround, 80 ms frame, fixed 10 ms opportunities); see
[results.md](results.md#4-where-it-stopped) and the gates in
[next-steps.md](next-steps.md).

Status legend:

| Status | Meaning |
| --- | --- |
| `current` | The decision stands as written. A note in parentheses marks a later narrowing or generalization. |
| `superseded by ADR-xxxx` | A later decision replaced it; the older one is history. |
| `current, rests on an unmeasured assumption` | The decision stands, but its numbers or verdicts depend on a parameter that was assumed. Treat them as planning bounds until G1 measures the real link. |

## Summary

| ADR | Decision in one line | Status |
| --- | --- | --- |
| 0001 | Use a custom protocol (ADHR), not DMR compatibility | `current` |
| 0002 | Build a fixed single-frequency one-relay slot relay first | `superseded by ADR-0015` |
| 0003 | Write the simulator and protocol core in Go, stdlib first | `current` |
| 0004 | Technical documents and identifiers in English | `current` |
| 0005 | Relay is a capability and runtime state of a portable node, not infrastructure | `current` (narrowed by ADR-0019) |
| 0006 | Separate voice plane (scheduled, never flooded) from control plane | `current` |
| 0007 | Keep the codec behind a boundary; defer Codec2, no cgo | `current` |
| 0008 | Model the channel as an abstract bit pipe with configurable rate | `current, rests on an unmeasured assumption` |
| 0009 | Elect exactly one relay per call by a deterministic score | `current` (generalized by ADR-0015) |
| 0010 | Skip relaying when the source already covers the closed group | `current` |
| 0011 | Compress relayed voice frames to a 14-byte context-bound header | `current` |
| 0012 | Apply the same short header to source and direct voice hops | `current` |
| 0013 | Group floor: first arrival wins, losers get explicit busy or collision | `current` |
| 0014 | Firmware: OpenRTX now, OpenGD77 as a later candidate | `superseded by ADR-0017` |
| 0015 | Bounded per-PTT scheduled relay tree, zero to three relays | `current` |
| 0016 | Source-led, 80 ms, depth 3, width 4 as the backend measurement envelope | `current, rests on an unmeasured assumption` |
| 0017 | OpenGD77 as next candidate; Analog FM and ADHR as separate modes | `superseded by ADR-0018` (mode separation still current) |
| 0018 | OpenGD77 is a prototype only; no HR-C6000 path is the product transport | `current` |
| 0019 | Every product handheld can relay; evidence boundaries tightened | `current` |
| 0020 | Peer-local, source-led schedule control with fail-closed permits | `current` |
| 0021 | Byte-derived voice profile; live-voice hardware verdict is no-go | `current, rests on an unmeasured assumption` |
| 0022 | Compact binary control encoding; offline listening gate for codecs | `current` |

## ADR-0001: Custom protocol instead of DMR

The goal was to prove relay mechanics, not to interoperate, so ADHR is a
custom protocol with frame identity, source/group/call ids, sequence numbers,
hop/TTL loop control, duplicate suppression and an opaque codec payload. DMR
compatibility would have pulled in the whole air interface and AMBE voice; the
cost is no interoperability with DMR handhelds. The review confirms the
choice: DMR lower layers are only backend inputs, and stock DMR voice returns
only as a listening reference in G0.

## ADR-0002: Single-frequency slot relay first

The first mode was one fixed relay on one frequency with two slots: the source
sends frame N, the relay forwards it in the next slot. It was the smallest
case of the A, relay, B scenario and avoided the collisions, duplicates and
loops of naive flooding. ADR-0015 replaced the fixed relay and schedule with
the general tree but kept one hop as the first acceptance slice; the one-hop
scenarios in [`configs/scenarios/`](../configs/scenarios/) remain fixtures.

## ADR-0003: Go for the simulator

A deterministic protocol simulator needed fast iteration, reproducible tests,
a single CLI binary and a stdlib with JSON, CRC32 and testing, so Go was chosen
over Rust, TypeScript and C++; Python stays for analysis
([`scripts/audio/`](../scripts/audio/)). Go is not the firmware language:
embedded code stays in C behind a protocol boundary
([`firmware/`](../firmware/)). The review keeps the core
([`internal/`](../internal/), [`cmd/adhr-sim/`](../cmd/adhr-sim/)) at any
outcome.

## ADR-0004: Documentation language

Specifications, contracts, schemas and decision records are in English, and
identifiers, field names and code comments are never translated, to stop drift
between documents and code. No protocol consequence; the review leaves it.

## ADR-0005: Portable relay capability model

There is no fixed repeater: every participant is a portable station, and
relaying is a capability plus runtime state (`relay_capable`, `relay_enabled`,
`relay_eligible`, `assigned_relay`). Candidates are filtered before scoring
(capable, enabled, not busy, battery and source link good enough), so busy or
low-battery nodes are skipped. ADR-0019 removed the endpoint-only allowance for
product handhelds; it remains for simulator cases. The review keeps the model;
what the node physically is stays open (last section).

## ADR-0006: Separate voice plane and control plane

Meshtastic- and Reticulum-style flooding would destroy airtime and latency for
live voice. Voice is forwarded only by the source or a relay named in the
active schedule, never flooded, and deduplicated by `origin_key`. Control
frames (beacons, call start, offers, schedule updates) may use bounded
broadcast with duplicate suppression, their own hop limit and an airtime
budget. The review keeps the split and questions only the control volume per
call (ADR-0020).

## ADR-0007: Codec boundary, Codec2 deferred

The simulator owns a codec interface ([`internal/codec/`](../internal/codec/))
with a deterministic dummy codec, and the relay never decodes or re-encodes
audio. Codec2 (LGPL 2.1) was not linked: no cgo or host tool in the default
tests; a real codec enters first as an optional external process after
license, build, latency and size review. That path later fed offline listening
(ADR-0022), where Codec2 2400/3200 was rejected. The boundary stands; the codec
is unselected.

## ADR-0008: Abstract bit-pipe airtime model

Airtime is `ceil(frame_bits * 1000 / channel_bitrate_bps)` over a
configurable rate, with deterministic loss, delay, jitter and corruption
([`internal/channel/`](../internal/channel/)); the 9600 bit/s default was an
anchor, not an M17 or DMR claim. Preamble, sync, FEC, guards and turnaround
were left out, so cost is underestimated, and later runs used 19.2 kbit/s,
which no TYT path supports. The model is only as good as the measured rate fed
into it.

## ADR-0009: Controlled single-relay election

Each call elects exactly one relay by a deterministic weighted score (source
link, group coverage, battery, stability) after the ADR-0005 filter, lower node
id breaking ties ([`internal/election/`](../internal/election/)); handover is
a new election on a later snapshot. Weights were never tuned on real links.
ADR-0015 generalized this to a zero-to-three-relay tree; the review keeps the
package and proposes "one live relay, degrade beyond" as a fallback.

## ADR-0010: Direct coverage gate

If the talker already reaches every expected member of a closed group, no relay
is assigned. Coverage is directional (`source -> member`) and estimated from
beacons and neighbor tables, not per-frame ACKs; each member is covered
directly, via relay, uncovered or unknown, and reports say "probably covered",
never "delivered". The gate became the first ranking rule of the relay tree
and is unaffected by the review.

## ADR-0011: Compressed relay-hop wire format

Sweeps showed the 48-byte canonical header dwarfed an 8-byte Codec2-class
payload, so relayed voice uses a 14-byte context-bound header (22 bytes with
that payload) ([`internal/protocol/`](../internal/protocol/)). The compressed
form needs an active call context and expands back to the canonical frame
before duplicate suppression; missing or stale context drops the frame, and
late entry needs context. The format is valid on any link; the review changes
only the budget: one 22-byte frame in each 27-byte DMR Layer-2 burst (one
burst per time slot every 60 ms) leaves 8 bytes of codec data, about
1.07 kbit/s.

## ADR-0012: Short source and direct voice format

The 14-byte header was extended to source and direct hops, so every scheduled
voice hop is 22 bytes with an 8-byte payload
([protocol/frame-contract.md](protocol/frame-contract.md)). In the simulator
two-relay live voice dropped from 61 ms to 35 ms end to end in an 80 ms cycle.
The format is current; the timing assumed 19.2 kbit/s and 1 ms guards and link
delay, so it is not hardware evidence.

## ADR-0013: Group floor busy and collision policy

Never two successful speakers at once: the first PTT wins, later overlapping
attempts are denied as busy, exactly simultaneous ones get one deterministic
winner plus explicit collisions, and losers send no voice. Counters make it
visible; queueing, priority and carrier sensing are out of scope. The rule
stands after the review.

## ADR-0014: OpenRTX now, OpenGD77 later

First bench work on the TYT MD-UV380 used OpenRTX-based firmware for its
exposed radio control entry points; OpenGD77 waited for a license and DMR/AMBE
review, stock firmware gave too little control, and a new firmware was too
costly. OpenRTX yielded reproducible receive-only builds, rollback and USB
telemetry
([firmware/openrtx-mvp10.3-receive-only-slice.md](firmware/openrtx-mvp10.3-receive-only-slice.md),
[research/018](research/018-openrtx-source-build-intake.md),
[research/019](research/019-openrtx-isolated-build-and-resource-budget.md)).
ADR-0017 replaced the sequencing when its handheld UI proved unsuitable. The
review adds that receiving custom 4FSK under OpenRTX here needs a soldered wire.

## ADR-0015: Bounded per-PTT scheduled relay tree

The central decision. From a freshness-bounded directed neighbor graph and a
membership snapshot, a deterministic planner
([`internal/routing/`](../internal/routing/)) picks, per PTT, a tree rooted at
the speaker with zero to three relays. Trees are filtered by depth, freshness,
eligibility, latency and airtime, then ranked by coverage, depth, transmission
count, link risk and battery, with a numeric tie-break (a 2026-10-02 erratum
fixed a text comparison that put relay 10 before 9). Opportunities are
serialized with one owner each, schedules are monotonic leases
(`effective_seq..valid_through_seq`), and radio specifics sit behind
`RadioBackend` ([`internal/radiobackend/`](../internal/radiobackend/)). The
review calls the core sound on any link, but on one shared half-duplex channel
airtime grows about `(n + 1)`-fold, so depth three with good voice is out of
reach on the TYT.

## ADR-0016: Simulator-selected backend measurement envelope

A 30-case stress matrix passed project-native variable slots at depth 3 and
width 4; a DMR-inspired profile (30 ms bursts, one per time slot every
60 ms) relayed nothing in that model.
Source-led timing became the only initial hardware profile (80 ms frame,
depth 3, width 4, no spatial reuse, hardware claims false). The core owns
routing and schedules; a backend only executes an authorized transmit inside a
monotonic time window, under a versioned measurement contract
([firmware/backend-measurement-contract.md](firmware/backend-measurement-contract.md),
[firmware/simulator-to-firmware-contract.md](firmware/simulator-to-firmware-contract.md)).
Ownership rules stand; the envelope does not: the 80 ms frame and 10 ms
opportunities are assumptions, and the "go" used two dummy payload bytes, so
it is a scheduling result, not voice capacity.

## ADR-0017: OpenGD77 candidate and Analog FM coexistence

OpenGD77 became the next candidate because the handheld must stay a usable
conventional radio. Modes are explicit and visible: Analog FM (no ADHR framing
or retransmission), ADHR, and optional DMR outside scope; switching isolates
receive, transmit, codec and call state, with no hidden fallback. Plan: an
empty-codec build without donor AMBE, a receive-only hook on the 27-byte
HR-C6000 Layer-2 payload, then a 36-byte Layer-1 comparison
([research/016](research/016-opengd77-feasibility-spike.md),
[017](research/017-opengd77-source-build-intake.md),
[020](research/020-backend-candidate-selection.md)). ADR-0018 closed the
evaluation; the mode separation remains a requirement.

## ADR-0018: OpenGD77 prototype only, no HR-C6000 product transport

Two identical empty-codec builds and a receive-only Layer-2 hook were
produced: a 5-byte envelope header around one 22-byte ADHR short frame fills
the 27-byte Layer-2 buffer
([protocol/opengd77-l2-envelope-v1.md](protocol/opengd77-l2-envelope-v1.md),
[`firmware/`](../firmware/)). The hook was tested only with host fixtures: no
project bytes crossed the chip or the air. Layer-2's 30 ms grid needs 90 ms for two relays
and 120 ms for three before turnaround, beyond an 80 ms period; Layer-1 has an
empty receive handler and unknown payload. Verdict `opengd77_prototype_only`:
a non-commercial research vehicle (per its license), no HR-C6000 product
transport, backend unselected. The review strengthens this: the documented chip
rates leave no route to 19.2 kbit/s.

## ADR-0019: Universal peer relay and evidence boundaries

Every product handheld acts as source, receiver and relay in the same firmware;
no relay-class device is needed. Capability is not assignment: disabled
relaying, Analog FM mode, busy or low battery make a peer ineligible with a
visible reason, and a selected relay still plays the call without decoding in
the forwarding path. ADR-0016 became a conditional envelope, and any live-voice
claim now needs one causal runtime (control, impairments, real wire sizes,
local clocks, playout). The review keeps this; whether a handheld is a stock
radio or a radio plus a fast-link module is now the main open question.

## ADR-0020: Peer-local schedule control with fail-closed permits

Forward voice reachability does not prove reverse control reachability, so
control is a bounded source-led transaction over directed links
([`internal/peer/`](../internal/peer/)): proposal, availability ACK from every
selected transmitter, then commit; a peer installs an expiring local permit
only on a matching commit. No silence-as-ACK, bounded retries, clock
uncertainty beyond budget inhibits TX, and partial activation is reported, not
hidden. The review prices it: 715/1608/2934 bytes of setup for one/two/three
relays, 0.3-1.2 s of airtime before the first word even at 19.2 kbit/s, and
proposes testing overhear-based forwarding without a per-PTT handshake.

## ADR-0021: Byte-derived voice profile and live-voice no-go

The runtime was rebuilt with real encoders: a 1600 bit/s, 80 ms size profile
gives a 36-byte packet, 480 bits on air with FEC 2/3 and framing, so at
19.2 kbit/s and 1 ms per switching step each opportunity takes 29 ms
(29/58/87/116 ms for zero to three relays). At 4.8 kbit/s one packet already
misses the cadence, and the 27-byte Layer-2 envelope cannot carry it; codec
data is never truncated. Verdict: `no_go_live_voice_hardware`; store-and-forward
not implemented. The review confirms the no-go and finds it optimistic:
19.2 kbit/s has no hardware path and 1 ms turnaround was never measured.

## ADR-0022: Compact control and offline listening gate

The transaction keeps its rules but defaults to the binary `compact_v1`
encoding ([protocol/compact-control-v1.md](protocol/compact-control-v1.md)):
only the proposal carries the schedule, other messages carry a 64-byte
reference with the full digest. One-relay setup fell from 6548 bytes (JSON) to
715, and unknown formats or too-small backend payloads fail closed. An
optional offline listening harness for real codecs
([`scripts/audio/`](../scripts/audio/)) keeps codecs out of the build and the
relay. Its verdicts rest on one listener and two clips without noise or
digits; the review asks for a blind test (G0) before choosing a codec.

## Decisions still open

No accepted decision exists for these; most are to be settled by measurement,
not simulation (gates in [next-steps.md](next-steps.md)).

- **Course correction.** Working rule since 2026-10-02: no new simulator
  features until G1; fixes, tests, intake of real measurements and parameter
  updates are allowed. Its formal decision record is not yet written.
- **Product radio link and backend.** Unselected (ADR-0018). The stock TYT
  gives about 3.4 kbit/s net over DMR Layer-2 (3.6 kbit/s raw slot payload),
  or 5.1-7.7 kbit/s over 4FSK with a receive hardware mod; a CC1101-class module is far faster at lower
  power. G1 decides on measured turnaround, payload rate and packet errors.
- **Node definition.** Stock handheld (fewer relays or lower quality), handheld
  plus a small fast-link module (the handheld stays microphone and speaker), or
  a dedicated fast-radio device. Decided by G1.
- **Codec.** Unselected. Codec2 was rejected by listening, Opus 8k is
  intelligible but does not fit one relay even at 19.2 kbit/s, and Lyra does
  not fit the TYT microcontroller. G0 (blind listening of digits and phrases,
  quiet and noisy, against stock DMR voice and FM) decides whether an
  Opus-class codec is needed.
- **Delivery mode.** Live frames or block store-and-forward within the
  1.5-2 s tolerance (end of speech to end of playback). A simulator run reached
  two relays in 1783 ms for one 500 ms burst at the assumed 19.2 kbit/s, but
  store-and-forward was never implemented; "one live relay, degrade beyond" is the other candidate.
- **Live relay depth.** Three relays stay only if G2 shows channel load at most
  1.0 on the measured rate and PTT setup within 0.3 s; else `live_max_depth = 1`.
- **Control and timing.** Per-PTT handshake (ADR-0020) or overhear-based
  forwarding; over-the-air schedule distribution and late-entry refresh are
  undefined. Source-led timing is a hypothesis, group-leader a comparison;
  spatial reuse stays off until conflicts can be measured.
- **Floor extensions and licensing.** Queueing and priority (ADR-0013) are
  deferred; OpenGD77-derived work is non-commercial, so a product needs a
  separate license decision.

Safety rules hold under every decision above: backup and tested rollback
before flashing, transmission off by default, and emission during measurements
only under the conditions in [hardware.md](hardware.md).
