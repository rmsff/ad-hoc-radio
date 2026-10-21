---
title: Simulator to Firmware Contract
created: 2026-07-05
updated: 2026-10-02
status: done
story_id: MVP-8.32
current_handoff_story_id: MVP-9.8
---

# Simulator to Firmware Contract

> Historical working record from the project's experiments, kept as reference.
> Current state: [docs/results.md](../results.md). Notes marked "not published"
> belong to the project's internal record.

## 1. Purpose

Define the pre-station contract between the Go simulator and a future
OpenRTX-derived ADHR bench firmware slice.

This contract is versioned documentation plus fixtures. MVP-8.32 established
the pre-station v0 profile/log contract. MVP-9.8 adds an independent v1
`RadioBackend` semantic handoff after the bounded relay-tree simulator gate.
Neither version changes the canonical frame contract or metrics schema.

```text
contract_version = adhr-sim-fw-contract-v0
hardware_claim_allowed = false
field_tx_ready = false
```

## 2. Artifact Set

| Artifact | Producer | Consumer | Purpose |
| --- | --- | --- | --- |
| `sim_profile.json` | simulator/planning docs | firmware slice | Static scenario, node, timing, beacon, channel, and safety settings. |
| `tx_plan.jsonl` | simulator | firmware slice | Optional future TX schedule; disabled unless bench safety gate allows TX. |
| `bench_log.jsonl` | firmware slice | simulator/readiness tools | Observed firmware events with monotonic timestamps. |
| `bench_summary.json` | firmware slice or converter | readiness tools | Compact summary of profile, build, hooks, and safety status. |

Current checked-in fixtures live under `configs/firmware-contract/`.

## 3. `sim_profile.json`

Required top-level fields:

```text
contract_version
profile_id
profile_status
radio_family
source_base
safety_profile
node
timing
channel
beacon
frame_formats
expected_outputs
```

Required invariants:

- `contract_version` must equal `adhr-sim-fw-contract-v0`;
- `profile_status` is `prestation_fixture` until real hardware exists;
- `safety_profile.field_tx_allowed` must be `false`;
- `safety_profile.tx_enabled_by_default` must be `false`;
- `timing` values are simulator-derived assumptions, not measurements;
- `frame_formats` must point back to `docs/protocol/frame-contract.md` and
  accepted short-frame ADRs.

## 4. `bench_log.jsonl`

Each line is one JSON object.

Required fields:

```text
contract_version
profile_id
seq
monotonic_ms
event
source
severity
details
```

Allowed initial events:

| Event | Meaning |
| --- | --- |
| `boot` | Firmware slice started. |
| `profile_loaded` | Profile accepted. |
| `rx_enable` | RX hook requested/enabled. |
| `rx_disable` | RX hook disabled. |
| `tx_blocked` | TX was requested but safety gate blocked it. |
| `beacon_due` | Beacon timer reached its scheduled point. |
| `beacon_emit_dry_run` | Beacon bytes would be emitted, but TX is disabled. |
| `frame_rx` | ADHR frame bytes received in a later bench run. |
| `crc_fail` | Frame rejected by integrity validation. |
| `timing_violation` | Firmware missed a schedule deadline. |
| `error_receive_only` | Fail-safe receive-only mode entered. |

Initial contract fixtures intentionally contain no real TX event.

## 5. Timing Rules

- Use a monotonic clock only.
- Do not use wall-clock time for schedule decisions.
- Preserve simulator units in milliseconds unless explicitly documented.
- Firmware logs must include enough timestamps to measure:
  - RX enable latency;
  - TX gate decision latency;
  - beacon timer drift;
  - profile load time;
  - future RX/TX turnaround once station-required bench work starts.

## 6. Safety Rules

The simulator may generate a TX plan, but firmware must ignore TX unless a later
bench-safe profile explicitly allows it.

```text
field_tx_allowed = false
unsafe_profile_behavior = reject_or_receive_only
```

If the firmware cannot prove a profile is safe, it must enter
`error_receive_only`.

## 7. Drift Control

This contract must be updated when any of these change:

- ADHR frame identity, flags, CRC, or binary layout;
- short voice frame layout;
- beacon interval/freshness rules;
- relay election state;
- readiness artifact schema;
- firmware bench log event vocabulary.

If a change touches frame semantics, update `docs/protocol/frame-contract.md`.
If a change touches CLI metrics JSON, update
`docs/protocol/metrics-schema.json`.

## 8. Fixtures

The current fixtures are intentionally dry-run only:

- `configs/firmware-contract/prestation-sim-profile.json`;
- `configs/firmware-contract/prestation-bench-log.jsonl`.

They prove the contract shape, not firmware behavior.

## 9. MVP-9 `RadioBackend` Handoff

The checked-in handoff bundle is:

```text
configs/firmware-contract/mvp9-radio-backend-handoff.json
contract_version = adhr-sim-fw-contract-v1
canonical_sha256 = d97a01b0160fe2c125574ffc1c1125a9da20c2d20e0508d35d565977d38ec39a
```

V1 is additive. It does not replace the v0 `sim_profile.json` and
`bench_log.jsonl` fixtures. It transfers only stable MVP-9 semantics to future
backend adapters and records the evidence still required before a candidate can
enter a receive-only lab slice.

### 9.1 Simulator gate

```text
profile_id = source_led/project_native_variable
architecture_verdict = accepted
profile_verdict = go
stress_evidence_hash = 7883ebade1df42ad1bd5c96302cabdf87e2fd6900b04b0b1507eb12f539aed29
frame_period_ms = 80
max_relay_depth = 3
max_serialized_branch_width = 4
serialized_single_frequency = true
spatial_reuse_enabled = false
```

These values are the MVP-9.7 simulator envelope. They are not channel,
turnaround, synchronization, audio, hardware, or RF measurements.

### 9.2 Ownership boundary

The network core continues to own:

- canonical frame identity and semantics;
- expected-group snapshots and directed topology input;
- relay-tree planning;
- schedule versioning and transmitter-opportunity order;
- duplicate suppression and metrics semantics.

The backend may own only:

- backend-specific framing, FEC, and modulation;
- radio control;
- channel-capacity, TX/RX-turnaround, and timing-acquisition measurements;
- link observations delivered to the core.

The two ownership sets are exact, sorted, and non-overlapping in contract v1.
Changing them requires a new contract version and architecture review.

### 9.3 Boundary operations

Contract v1 defines five API-neutral operation shapes:

| Operation | Direction | Meaning |
| --- | --- | --- |
| `discovery_observation` | backend -> core | Fresh station observation; it does not mutate group membership itself. |
| `link_measurement` | backend -> core | Directed measured link input; the core still owns graph and route selection. |
| `receive_delivery` | backend -> core | Received frame bytes plus monotonic/RSSI observation. |
| `scheduled_transmit` | core -> backend | Already-authorized call, schedule-version, opportunity, timing, and frame bytes. |
| `timing_observation` | backend -> core | Measured acquisition/drift input; the core still owns timing strategy semantics. |

This is a data contract, not a Go callback/threading design and not an OTA
schedule-control encoding. A future Epic 7 adapter may select concrete
callbacks, queues, or polling without changing these semantics.

The core alone validates the inclusive schedule lease
`effective_seq..valid_through_seq`. Lease fields do not cross this boundary.
Each `scheduled_transmit` is one exact execution command keyed by:

```text
(net_id, group_id, source_id, call_id, schedule_id, schedule_version,
 seq, opportunity_index)
```

The command carries `backend_clock_id`, absolute `not_before_monotonic_ms`, and
absolute `deadline_monotonic_ms` in one backend-local monotonic clock domain.
The backend rejects duplicate keys, early execution, and expired execution. It
does not infer lease validity, routes, relay ownership, or opportunity order.

### 9.4 Backend evaluation gates

Each candidate carries exactly these gates:

```text
license
codec_coupling
channel_capacity
tx_rx_turnaround
timing_acquisition
lab_safe_measurement
```

`channel_capacity`, `tx_rx_turnaround`, `timing_acquisition`, and
`lab_safe_measurement` can pass only with `bench_measured` evidence and artifact
hashes. Build/source evidence may keep a candidate under investigation but
cannot produce a hardware pass.

Epic 7 must define one strict versioned measurement-report contract before any
measured gate can pass. It must define semantic endpoints, units, candidate and
profile identity, firmware/binary hashes, clock domain and resolution, trial
matrix, minimum sample size, warm-up handling, thresholds, percentile method,
failed-sample accounting, raw artifact references, and deterministic canonical
output/hash. A missing field, threshold, sample, or raw artifact forces a
non-passing/no-claim result.

Current contract-v1 candidate status:

| Candidate | Verdict | Meaning |
| --- | --- | --- |
| `openrtx_mduv3x0` | `measurement_required` | Completed reference/fallback path; source/build, receive-only adapter, rollback package, and partial bench telemetry evidence exist. It is not the intended product firmware base. Channel/turnaround/timing/full lab gates are not passed. |
| `opengd77_mduv380` | `measurement_required` | Primary next candidate under ADR-0017. The target archive exposes a candidate empty-codec path only; MVP-10.4..10.6 must prove reproducibility, scoped no-donor/no-AMBE behavior, explicit Analog FM/ADHR modes, receive-only custom payload handling, and the HR-C6000 transport verdict. License, timing, turnaround, and lab gates remain unresolved. |

A failed gate rejects only that candidate. It does not rewrite ADR-0015 or the
MVP-9.7 simulator verdict.

### 9.5 Safety and claims

V1 requires:

```text
receive_only_default = true
tx_enabled_by_default = false
field_tx_allowed = false
hardware_claim_allowed = false
regulatory_profile_accepted = false
lab_safety_plan_accepted = false
dmr_compatible = false
compatibility_claim = false
ambe_dependency = false
rf_synchronization_claim = false
```

`eligible_for_receive_only_lab` would still not permit field transmission. The
mandatory order is receive-only evidence first, then accepted regulatory
profile plus shielded/attenuated/dummy-load safety plan, then transmit-capable
measurement inside that setup. Field work requires a later explicit ADR/story.

### 9.6 Relay-tree tie-break and `call_start` bounds

Any implementation that ranks relay trees, such as a future firmware-side
planner or a conformance fixture, applies the ADR-0015 objective and ends with
this numeric lexicographic order of the canonical tree fields:

1. `covered_member_ids`;
2. edges: the list in the tree's canonical edge order
   `(hop, parent_station_id, child_station_id)`, each element compared by
   `(child_station_id, hop, parent_station_id)`;
3. `max_relay_depth`;
4. `membership_snapshot_id`;
5. `source_station_id`;
6. `topology_epoch`;
7. `uncovered_member_ids`;
8. tree hash, as a safety net against drift: an implementation that compares
   all seven fields never reaches it and need not compute it for ordering.

Station ids compare as numbers, never as JSON text. Lists compare element by
element, and a list that is a prefix of another ranks first. Within one
planning request only `covered_member_ids` and the edges separate candidates;
the other fields make the order total between trees of different requests.

Two trees of equal cost that differ only in relay 9 versus relay 10 are
ordered with the tree through relay 9 first. This is not a general "lower relay
id first" rule. Covered members come first: a tree covering `[9]` through relay
20 ranks before a tree covering `[10]` through relay 3. The first edge decides
next: for members `{3, 4}` and links `1->3, 1->4, 1->9, 1->10, 9->3, 10->4`,
the tree `1->3, 1->10->4` ranks before `1->4, 1->9->3`, because its first
edge reaches child 3. The canonical JSON text feeds only the tree hash.

This order is the planner tie-break only. The window scheduler orders relay
opportunities by its own key `(hop, parent_station_id, owner_station_id)`
(frame-contract.md section 8.4); the two keys differ and share only the numeric
comparison of station ids.

The `call_start` bounds are defined in frame-contract.md section 8.1:
`frame_period_ms` in `1..60000` (compact_v1 and the peer layer: `1..10000`)
and `created_at_ms` in `0..MaxSafeCreatedAtMs`; section 11 gives the order of
the age checks and the overflow-free age comparison.
