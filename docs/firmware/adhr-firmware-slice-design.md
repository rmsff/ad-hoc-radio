---
title: ADHR Minimal Firmware Slice Design
created: 2026-07-05
updated: 2026-07-05
status: done
story_id: MVP-8.31
---

# ADHR Minimal Firmware Slice Design

> Historical working record from the project's experiments, kept as reference.
> Current state: [docs/results.md](../results.md). Notes marked "not published"
> belong to the project's internal record.

> **Historical design note (2026-07-12):** This MVP-8.31 OpenRTX slice design
> was implemented as reference evidence in MVP-10.3. ADR-0017 initiated the
> OpenGD77 evaluation and preserved explicit Analog FM + ADHR modes;
> MVP-10.4..10.6 are complete and ADR-0018 records
> `opengd77_prototype_only` with no selected product backend. The body below
> remains the historical OpenRTX design and must not be used as a current
> product-backend specification.

## 1. Purpose

Define the smallest firmware slice worth building after a TYT MD-UV380 /
MD-UV3x0-class station is available.

This is a pre-station design. It does not implement firmware, flash hardware, or
authorize transmission.

```text
slice_name = adhr_bench_slice_v0
source_base = OpenRTX-derived
default_mode = receive_only
tx_path_default = blocked
firmware_entry_ready = false
hardware_claim_allowed = false
field_tx_ready = false
```

## 2. Non-Goals

- No product UI.
- No OpenGD77 migration.
- No DMR, M17, or AMBE compatibility claim.
- No encryption feature.
- No field RF transmission.
- No hidden proprietary protocol behavior as a safety mechanism.

## 3. Slice Boundary

The first slice should be a bench-mode firmware branch or patch set with these
modules:

| Module | Responsibility | Must not do |
| --- | --- | --- |
| `adhr_frame_io` | Accept and emit ADHR frame bytes using the simulator's canonical/short wire formats. | Decode/encode audio or call external voice codecs. |
| `adhr_scheduler` | Execute a bounded slot/beacon schedule from a loaded bench profile. | Invent timing that the simulator does not model. |
| `adhr_radio_adapter` | Wrap OpenRTX radio hooks for RX/TX/off and mode/bandwidth setup. | Bypass safety gates or transmit by default. |
| `adhr_beacon` | Generate and consume low-rate neighbor-table beacon records. | Claim delivery acknowledgement. |
| `adhr_bench_log` | Emit deterministic diagnostic records over a serial/log channel. | Emit secrets, raw private keys, or unbounded logs. |
| `adhr_safety_gate` | Keep TX disabled unless a bench-safe profile explicitly allows it. | Treat simulator success as lab approval. |

## 4. State Machine

```text
boot
  -> receive_only_safe
  -> profile_loaded
  -> idle_rx
  -> beacon_due
  -> rx_frame
  -> tx_guard
  -> tx_frame
  -> tx_cooldown
  -> idle_rx

any_state -> error_receive_only
```

Required fail-safe behavior:

- invalid profile -> `error_receive_only`;
- missed timing deadline -> log `timing_violation`, return to `idle_rx`;
- radio hook error -> `error_receive_only`;
- safety gate failure -> no TX, log `safety_gate_blocked`.

## 5. OpenRTX Integration Leads

OpenRTX source intake identifies these leads for a later patch:

| ADHR need | OpenRTX lead |
| --- | --- |
| RX enable | `radio_enableRx()` |
| TX enable | `radio_enableTx()` |
| RTX off | `radio_disableRtx()` |
| mode/bandwidth setup | `radio_setOpmode()`, `radio_updateConfiguration()`, `AT1846S_BW` |
| target pins | `platform/targets/MD-UV3x0/pinmap.h` |
| PTT state | `platform_getPttStatus()` |
| scheduler/task loop | `rtx_threadFunc()` / `rtx_task()` path |

These are source leads, not accepted timing proof.

## 6. Bench Safety Defaults

The first slice must compile with:

```text
ADHR_TX_ENABLED_BY_DEFAULT = false
ADHR_PROFILE_REQUIRED_FOR_TX = true
ADHR_FIELD_TX_ALLOWED = false
ADHR_BENCH_LOG_REQUIRED = true
```

Allowed safety profiles for later station work:

| Profile | TX allowed? | Intended use |
| --- | --- | --- |
| `rx_only` | no | First boot, UI/serial/log validation. |
| `dummy_load` | possible only after explicit lab gate | Controlled bench TX into dummy load. |
| `shielded_low_power` | possible only after explicit lab gate | Controlled shielded test. |
| `field` | no for this slice | Out of scope. |

## 7. Firmware Inputs

The slice consumes only versioned inputs from the simulator-to-firmware contract:

- `sim_profile.json`;
- `tx_plan.jsonl` when TX is allowed by a later bench profile;
- optional static node identity for bench fixtures.

The slice emits:

- `bench_log.jsonl`;
- `bench_summary.json`;
- firmware build metadata.

## 8. Acceptance Criteria for Later Implementation

1. Boots into `receive_only_safe` with TX disabled.
2. Loads a simulator profile and rejects invalid/unsafe profiles.
3. Emits bench log records with monotonic timestamps.
4. Can execute beacon timing in receive-only dry-run mode.
5. Can exercise RX/TX hook transitions only behind a bench-safe gate.
6. Produces logs accepted by `adhr-sim convert-bench-log` or a future firmware
   log converter.
7. Does not change ADHR frame semantics without updating
   `docs/protocol/frame-contract.md`.
