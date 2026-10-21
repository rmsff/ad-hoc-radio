# Concept and Architecture

This document covers the idea, the requirements and the structure of the
protocol and simulator. Results are in [results.md](results.md), the log in
[experiments.md](experiments.md), the plan in [next-steps.md](next-steps.md).

## 1. Problem and idea

Handhelds in a search group often lose each other behind a floor, a ridge or
a building. Station A cannot hear station B, but station C, somewhere between
them, hears both. The idea: C relays A's digital voice automatically, on the
same frequency, with no deployed repeater, and any radio can be C for a call.

```text
        A  - - - - - x - - - - -  B      A and B cannot hear each other
         \____  C (relay) _____/        C hears both and retransmits A
```

Russian "Lira" radios already do this: the DP-3800 is a portable
single-frequency repeater with time-slot RX/TX, and in the Ad Hoc models any
station can be an endpoint, repeater or command node. Their protocols are
closed; they are only a reference for user expectations, with no
compatibility claim ([research.md](research.md)). ADHR is a small open
protocol for this behavior, tested first in a deterministic simulator, then on
the TYT MD-UV380 with open firmware ([hardware.md](hardware.md)).

## 2. Requirements

### 2.1 Current requirements

The first two rows are the requirements added by the 2026-10-02 review; the
rest carry over from the original requirements.

| Area | Requirement |
| --- | --- |
| Speech quality | First priority. Digits and short search-and-rescue phrases must be intelligible. Quality may drop only on deeper hops. |
| Delay | Up to 1.5-2 s from end of speech to end of playback (P95) through one relay is acceptable. |
| Coverage | Reach every member that a chain of at most three relays can reach. Report the members it cannot reach. |
| Peers | Every supported radio can originate, receive and relay. No fixed or bulky infrastructure. |
| Forwarding | Relays forward encoded bytes, never decoding audio. Each original frame plays at most once, even if direct and relayed copies both arrive. |
| Handheld use | The radio keeps a normal analog FM mode, separate and explicit. |
| Evidence | Same simulator input and seed, same output bytes. |

Proposed field gates, not yet accepted, for a 5 s phrase: P95 end of speech
to end of playback at most 2.0 s with up to one relay and at most 2.5 s with
three; digit accuracy at least 90% in noise; frame delivery at least 95%.

### 2.2 Original requirements (superseded)

The original requirements targeted **live** voice: a one-hop playout latency
under 250 ms (p95) in the simulator, with audio quality out of MVP scope. That
target fits an 80 ms frame pipeline with a 160 ms jitter buffer, but it does
not describe what users need. Following it, the project dropped block
store-and-forward, although one 0.5 s burst through two relays completed in
1783 ms at the assumed 19.2 kbit/s (computed, not measured;
[experiments](experiments.md#timing--capacity)). The 250 ms figure now
applies only to the simulator's live-frame profile.

## 3. How it works

### 3.1 Universal peers

Every radio runs the same software. Source, relay and receiver are roles a
station takes for one call (one PTT press), not separate device types.
Runtime state can still exclude a station from relaying: relaying disabled,
busy, low battery, weak link to the source, or Analog FM mode. An assigned
relay keeps listening to the call it forwards.

### 3.2 Discovery and the link graph

Each station keeps a directed neighbor graph (who reaches whom, with link
quality, loss, delay and observation time); A hearing C does not mean C
hears A. Link observations and the expected-group snapshot (who should hear
the call) have freshness bounds; expired ones are dropped before planning.
In the simulator the graph comes from the scenario, and one scenario family
models periodic beacons. No on-air discovery protocol is specified yet.

### 3.3 Per-PTT bounded relay tree

At PTT the source plans a relay tree rooted at itself over the fresh graph,
at most three relays deep (`Source -> R1 -> R2 -> R3`). Candidates failing the
depth, eligibility, freshness, link, latency or airtime gates are dropped.
The rest are ranked by covered expected members (more first), then maximum
relay depth, transmitter opportunities, aggregate link risk and aggregate
battery cost (each fewer first), then canonical tree order with station ids
compared as numbers. Depth 3 wins only if it reaches a member that no tree
of depth 0-2 can reach. Every plan lists expected, covered and uncovered
members; an uncovered member never counts as delivered. A topology change
creates a new schedule version effective at a future sequence number; a
running version is never changed in place.

```text
expected group {B, D, E}         selected tree         opportunities
  S reaches R1, R2                    S                 0: S
  R1 reaches B, R3                  /   \               1: R1 (parent S)
  R2 reaches D                    R1     R2             2: R2 (parent S)
  R3 reaches E                   /  \      \            3: R3 (parent R1)
                                B    R3     D
                                      \
                                       E
```

### 3.4 Scheduled opportunities on one half-duplex channel

All stations share one frequency, and a radio cannot transmit and receive at
once. Voice runs in a repeating cycle of **transmitter opportunities**, one
cycle per source frame, each with exactly one owner, an offset, a duration
and a guard. The source goes first; relays follow in (hop, parent id, owner
id) order, so a relay always follows its parent. Relays at the same depth
take turns: slot sharing would need a conflict graph, which is not
implemented. The whole cycle must fit in the frame period:

```text
one 80 ms frame period, direct + 3-relay chain at an assumed 29 ms each
|-- S --|-- R1 --|-- R2 --|-- R3 --|
0      29       58       87      116 ms   -> does not fit in 80 ms
```

A relay transmits only in an opportunity it owns in the active version, for
the matching call and sequence; hearing a frame or having TTL left never
grants permission. A missing parent copy is logged as a missed opportunity,
not flooded. Timing follows the source (`source_led`); an elected group
leader also passed in the model, but needed one more replan on failure.

### 3.5 Duplicate suppression

Each frame has an origin key `(net_id, source_id, call_id, seq)` shared by
the direct and relayed copies. Receivers accept the first valid copy and
count the rest as suppressed; relays keep a bounded seen-cache by the same key
and forward each origin frame at most once. TTL and hop counters stop loops,
and stale frames are dropped (defaults: 500 ms at relays, 1000 ms at receivers).

### 3.6 Jitter buffer and playout

A receiver plays frame `seq` at its deadline, source creation time plus the
jitter-buffer delay. A frame missing at the deadline is counted as lost and
playout moves on; a later copy is never played, and no concealment audio is
synthesized. Service is measured at timed playout, not at packet arrival.

### 3.7 Voice plane and control plane

Voice is sent only by the source and the relays named in the active schedule,
with strict timing, and is never flooded. Control messages may use bounded
managed broadcast with duplicate suppression, their own hop limit and an
airtime budget. Control runs locally on each peer: the source sends a
Proposal with the schedule along directed routes, each selected relay answers
with an ACK over its own reverse route, the source sends a Commit, and each
peer activates only the permit it actually received. Voice then flows; End
or local lease expiry closes the call (Sync re-acquires timing). There is no
global atomic commit: a lost Commit gives partial delivery, reported as such.

### 3.8 Compact control

A Proposal carries the full schedule. Every other control message is a fixed
64-byte reference: the schedule's SHA-256 digest plus version, epoch and
frame period. A station acts on a reference only if it holds a matching
permit. CRC32 and the digest detect corruption; they do **not** authenticate.

### 3.9 Analog FM coexistence

The handheld has two explicit, visible modes: `Analog FM` (a normal FM/NFM
radio) and `ADHR`. Analog FM never joins the relay tree, and analog audio is
never taken for an ADHR frame. Switching needs no reflash and resets codec,
squelch, timing and call state. ADHR needs neither DMR voice nor AMBE. All
firmware work so far is receive-only ([hardware.md](hardware.md), [firmware/](firmware/)).

## 4. Frame and control formats

The specifications are in [protocol/](protocol/):

- **Canonical frame** ([frame-contract.md](protocol/frame-contract.md)): a
  48-byte big-endian header (identity, call, sequence, hop, TTL, flags,
  timestamps, length, two CRC32) plus payload; an internal model, not on-air.
- **Short and compressed adapters** (same document): 14-byte headers bound
  to the active call context, so an 8-byte payload gives a 22-byte frame.
  Receivers expand them to canonical frames, or reject them without context.
- **OpenGD77 Layer-2 envelope** ([opengd77-l2-envelope-v1.md](protocol/opengd77-l2-envelope-v1.md)):
  a candidate 27-byte wrapper (one DMR data burst) around one 22-byte short
  frame, checked only against host fixtures.
- **Compact control v1** ([compact-control-v1.md](protocol/compact-control-v1.md)):
  a 60-byte common header, an optional Proposal body and a CRC32. A Proposal
  is `115 + 2 * (expected + covered + uncovered) + 15 * opportunities` bytes,
  counting expected, covered and uncovered member ids: 134 for a direct
  two-peer call, 153 for three peers with one relay.
- **Metrics** ([metrics-schema.json](protocol/metrics-schema.json)): the report schema.

## 5. Simulator

The Go simulator ([cmd/adhr-sim](../cmd/adhr-sim/), no external dependencies)
runs many logical stations in one process over an abstract transport. It
models directed links with loss, delay, duplication and corruption; a
bit-pipe channel that turns bytes into airtime at a set bitrate; the planner
and scheduler; peer-local control with clock offset and drift; relay queues
and collisions on the shared channel; the jitter buffer and timed playout.
It does not model modulation, real FEC decoding, RF propagation or real
synchronization. Payloads are fixed-size placeholder bytes, not speech. Codec
listening tests run separately, offline, in [scripts/audio](../scripts/audio/).

**Determinism.** Events are ordered by time, then fixed priority (slot
boundary 10, schedule update 15, source tick 20, delivery 30, scheduled
transmit 35, jitter tick 40, metrics flush 50), then event id. The seed is
the only randomness, times are integer milliseconds with overflow checks, and
no result depends on map order. Repeated runs give byte-identical reports.

### 5.1 CLI

`go run ./cmd/adhr-sim --help` lists:

```text
run <scenario.json> --metrics <path>      sweep --out <dir>
regression-pack <pack.json> --out <dir>   readiness-dashboard <summary.json> --report <path>
peer-acceptance --report <path>           control-comparison --report <path>
ptt-contention <scenario.json> --report <path>
opus-budget --input <trace.json> --report <path>
codec-budget --input <trace.json> --report <path>
calibrate-tyt <measurement.json> --report <path>
calibrate-channel <channel.json> --report <path>
score-audio <audio.json> --report <path>
convert-bench-log <raw-log.json> --out <canonical.json>
hardware-readiness --channel <r> --turnaround <r> --audio <r> --report <path>
version
```

### 5.2 Scenarios and the regression pack

[configs/scenarios](../configs/scenarios/) has one-hop cases (nominal,
direct plus duplicate, corruption, group mismatch, seen-cache overflow, queue
backpressure), a two-relay short-frame case (`two-hop-short-source.json`), a PTT contention case, and seven
cases in [relay-tree](../configs/scenarios/relay-tree/): direct, one, two and
three relays, narrow and wide branches, partition and recovery.

[core-cli-regression.json](../configs/regression/core-cli-regression.json)
(17 cases, run with `adhr-sim regression-pack <pack> --out <dir>`) passes a
case when its outcome matches the expected one, so intended failures
(corruption, backpressure) and fixture-only hardware gates count as healthy.

### 5.3 Module map

| Package | Role |
| --- | --- |
| [internal/protocol](../internal/protocol/) | Canonical frame, short and compressed adapters, identity keys, CRC, control payloads |
| [internal/routing](../internal/routing/) | Group snapshot, directed graph, bounded relay-tree planner and ranking |
| [internal/election](../internal/election/) | Earlier single-relay scoring with an eligibility filter, kept for comparison |
| [internal/timing](../internal/timing/) | Opportunity scheduler, schedule hash and validation, two-slot fixture, voice budget |
| [internal/node](../internal/node/) | Relay and receiver state machines, call lifecycle, seen-cache, jitter buffer, playout deadlines |
| [internal/peer](../internal/peer/) | Peer-local control, clocks, permits, air reservation and collisions |
| [internal/sim](../internal/sim/) | Simulated clock, deterministic event queue, canonical event JSON and hash |
| [internal/transport](../internal/transport/) | Local transport over a directed topology with seeded impairments |
| [internal/channel](../internal/channel/) | Bit-pipe airtime from bytes and bitrate, loopback impairment |
| [internal/codec](../internal/codec/) | Codec boundary: dummy codec and a spy that proves relays never decode |
| [internal/scenario](../internal/scenario/) | Scenario runners and report-only experiment families |
| [internal/radiobackend](../internal/radiobackend/) | Simulator-to-firmware handoff contract (`adhr-sim-fw-contract-v1`) |

Two more packages under [internal/](../internal/), `backendmeasurement` and
`firmwareevidence`, check bench measurement and firmware evidence files. The
network core does not depend on any radio backend, and firmware details stay
below the `RadioBackend` boundary.

## 6. What the model assumes and why it matters

The protocol logic is tested; the physical numbers behind it are assumed:

| Parameter | In the model | What is known |
| --- | --- | --- |
| Gross channel rate | 19.2 kbit/s | The TYT's HR-C6000 chip documents 4.8 and 9.6 kbit/s; one DMR slot carries 3.6 kbit/s of payload |
| RX-to-TX turnaround | 1 ms | Not measured; public proxies for similar handhelds give 20-58 ms |
| Links | clean; FEC 2/3 counted as overhead only | Error correction is not simulated |
| Opportunity length | fixed | Real bursts also need preamble, sync and ramp time |

Airtime grows with every relay. On one shared half-duplex channel each relay
retransmits the frame, so a chain of `n` relays needs about `(n + 1)` times
the airtime of a direct call. The 36-byte voice packet is 288 bits (15 ms at
19.2 kbit/s); with FEC 2/3 and framing it becomes 480 bits (25 ms), and the
model adds 4 x 1 ms of assumed switching, processing and uncertainty, so each
transmitter reserves 29 ms
([ADR-0021](decisions.md#adr-0021-byte-derived-voice-profile-and-live-voice-no-go))
and a three-relay chain needs 116 ms out of every 80 ms frame period. Opus 8k
over three relays would need about 50 kbit/s gross (75 with FEC 2/3), while
the TYT paths give about 3.4-7.7 kbit/s net; full numbers are in
[results](results.md#4-where-it-stopped).

Control has a cost too. Negotiating the schedule at each PTT takes 715, 1608
and 2934 bytes for one, two and three relays. At 19.2 kbit/s that is about
0.3, 0.7 and 1.2 s of airtime before the first word (0.45, 1.0 and 1.8 s
with FEC 2/3), which eats most of the 1.5-2 s budget. Consequences:

- Negative results at 19.2 kbit/s still stand, since a real channel is
  slower; positive ones transfer only after rate and turnaround are measured.
- On a stock TYT one live relay works only with codecs up to about
  2.4-3 kbit/s, and no accepted codec survives three live relays. With 1.5-2 s
  acceptable, block store-and-forward is a candidate delivery mode only if
  `(relays + 1) x codec rate` stays below the net rate; for Opus 8k on the
  TYT paths (no FEC, one to three relays) that load is about 2-9 times the
  net rate, so it does not help there.
- The protocol layer does not depend on the radio. Still open: is a node a
  stock handheld (fewer relays or lower quality), a handheld with a small fast
  data-link module, or a dedicated fast-radio device? The G1 measurements are
  meant to decide; no new simulator features until then
  ([next-steps.md](next-steps.md), [decisions.md](decisions.md)).

## 7. Non-goals

- **Not DMR-compatible.** ADHR identity, routing, scheduling and duplicate
  suppression are custom; DMR-style timing below the backend boundary implies
  no compatibility. No Lira, M17 or PMR support, and no AMBE dependency.
- **Not a LoRa messenger.** The target is group voice, live or delayed by a
  few seconds, not text over a slow long-range link.
- **No transmit authorization.** Nothing here permits transmitting on any
  frequency or at any power, and all firmware work so far is receive-only.
  Before transmitting, the operator needs a lawful profile for band, power
  and emission, and a lab-safe setup.
- No flooding, no unbounded depth, no fixed repeaters, no encryption, no
  peer authentication. Entry point: [README.md](../README.md).
