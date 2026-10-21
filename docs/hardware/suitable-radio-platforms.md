---
title: Suitable Radio Platforms — Ad Hoc Radio
created: 2026-06-29
updated: 2026-07-01
status: draft
type: hardware-shortlist
---

# Suitable Radio Platforms

> Historical working record from the project's experiments, kept as reference.
> Current state: [docs/results.md](../results.md). Notes marked "not published"
> belong to the project's internal record.

## 0. Purpose

This document records radio platforms that are worth tracking for a future hardware path after the Go simulator proves the relay behavior.

It intentionally lists only suitable or plausibly suitable platforms. It does not list rejected radios.

The current accepted path is still:

```text
Go simulator
  -> protocol / relay / slot / metrics validation
  -> baseband and codec boundary experiments
  -> hardware platform work later
```

## 1. Selection criteria

A suitable radio platform should have:

- a credible open firmware or low-level developer path;
- a documented target in a relevant firmware project;
- enough community/tooling history to be practical;
- portable station-class form factor for the main product direction;
- a path toward digital voice / frame-level experimentation;
- alignment with ADR-0005: relay is a portable node capability, not fixed infrastructure;
- alignment with ADR-0006: voice-plane and control-plane have different forwarding rules.

Marketing range, output power, and stock features are less important than developer access.

## 2. First-choice handheld: TYT MD-UV380 / MD-UV3x0 class

This is the preferred first hardware family to investigate.

Why it fits:

- OpenRTX has a dedicated `mduv3x0` target.
- The OpenRTX build file labels that platform section as `TYT MD-UV380`.
- The OpenRTX README mentions TYT MD-UV3x0 in the supported tooling path.
- It is a portable station-class device, so it matches the no-fixed-infrastructure product direction.
- It is closer to the OpenRTX/M17 direction than ordinary analogue radios.

Project use:

```text
main handheld candidate for future OpenRTX/M17-based experiments
```

Caveats:

- OpenRTX is experimental.
- M17 support is experimental.
- The project must not claim DMR, M17, or Lira compatibility unless a later ADR accepts that target.

### 2.1 Preliminary feasibility notes: 25 kHz and TX/RX turnaround

This section answers the current hardware question narrowly:

```text
Can the TYT MD-UV380 / MD-UV3x0 class plausibly support:
1. a 25 kHz channel-width experiment;
2. fast transmit/receive switching for slot-based relay?
```

Current answer:

```text
25 kHz RF bandwidth: plausible on the AT1846S / analog-channel path, not yet proven for our custom digital mode.
Fast TX/RX switching: unknown; must be measured before live two-hop SFR can be accepted.
```

Evidence:

- OpenRTX has a dedicated `MD-UV3x0` target and marks the device as supporting
  `CONFIG_M17`.
- The OpenRTX `MD-UV3x0` target exposes an `HR_C6000` baseband driver and uses
  an `AT1846S` radio-on-chip driver.
- The OpenRTX `AT1846S` driver exposes bandwidth settings for `12.5 kHz` and
  `25 kHz`, and the UV3x0 implementation writes separate register tables for
  both modes.
- The `AT1846S` driver exposes functional modes `OFF`, `RX`, and `TX`.
- The `MD-UV3x0` pin map exposes RF-stage controls such as `VHF_LNA_EN`,
  `UHF_LNA_EN`, `TX_PA_EN`, `RF_APC_SW`, and `PA_SEL_SW`.
- OpenRTX/M17 support for these radios requires hardware modifications so the
  MCU has access to microphone and RF receive paths.

What this proves:

- `25 kHz` is a realistic **bench parameter** for the radio chip path.
- The platform is more suitable than an ordinary closed analogue handheld
  because the firmware path and low-level RF control path exist.
- The selected TYT family remains the best first hardware candidate.

What this does **not** prove:

- It does not prove that `25 kHz / 19200 bps` custom digital voice will work.
  Channel bandwidth, symbol rate, modem filtering, deviation, FEC/interleaving,
  and baseband path limits are separate questions.
- It does not prove that a source frame can be received, decoded/validated, and
  retransmitted in the next 20/40 ms slot.
- It does not prove the TX-to-RX or RX-to-TX settling time for this hardware.
  PLL/LO settling, PA ramp, LNA enable, AGC/squelch, baseband filters, and MCU
  scheduling must be measured.

Required future bench check before accepting this hardware for SFR timing:

```text
TYT-MDUV3X0-BENCH-001:
  - flash OpenRTX or a minimal OpenRTX-derived bench firmware;
  - configure AT1846S bandwidth = 12.5 kHz and 25 kHz;
  - toggle RX -> TX -> RX through the same control path the future mode would use;
  - measure:
      rx_to_tx_ready_ms
      tx_to_rx_ready_ms
      pa_ramp_ms
      rx_rssi_valid_ms
      baseband_settle_ms
  - run only with lab-safe conditions appropriate to the RF/lab gate.
```

Detailed measurement plan: internal note (not published).
Firmware entry plan: `docs/hardware/tyt-md-uv380-firmware-entry-plan.md`.

Until that test exists, simulator values like `25 kHz / 19200 bps` and guard
times `1/2/5 ms` remain research parameters, not hardware commitments.

### 2.2 MVP-8.1 feasibility decision

MVP-8.1 adds a narrower decision:

```text
candidate_status = bench_research_candidate
hardware_commitment = no
firmware_path = OpenRTX-derived path preferred for custom experiments
future_migration_candidate = OpenGD77-derived ADHR fork, spike required
stock_mode_path = not sufficient for project protocol
channel_width_hz candidates = [12500, 25000]
channel_bitrate_bps candidates = [9600, 19200]
```

The key finding is that `25 kHz` and `19200 bps` are separate questions.
Public evidence supports `25 kHz` as a plausible bench parameter, but not
`19200 bps` custom digital voice through a real MD-UV380 firmware/baseband
path.

MVP-6.7 gives the current simulator thresholds for a `22B @ 19200 bps` short
voice frame with `1 ms` preamble, `1 ms` sync, `1 ms` link delay, and `1 ms`
guard:

| Source/relay slot | Max TX/RX switch for readiness | Approx two-hop latency | Cycle spare |
| --- | ---: | ---: | ---: |
| 16 ms | 2 ms | 45 ms | 35 ms |
| 20 ms | 6 ms | 53 ms | 27 ms |
| 24 ms | 10 ms | 61 ms | 19 ms |
| 30 ms | 16 ms | 73 ms | 7 ms |

The 16 ms slot plan is therefore aggressive. If measured turnaround is above
roughly `2 ms`, the simulator should move to a larger slot tuple or reject the
handheld path for live two-hop SFR.

Detailed research: internal note (not published).

### 2.3 Firmware base and future migration candidate

> **Current decision (2026-07-12):** ADR-0017 started the AMBE-free
> `Analog FM + ADHR` OpenGD77 evaluation; MVP-10.4..10.6 are complete.
> ADR-0018 now limits OpenGD77 to prototype-only, selects no product backend,
> and keeps physical install blocked by recovery/calibration/rollback. OpenRTX
> remains completed reference/fallback evidence.

ADR-0014 keeps the first bench firmware path as **OpenRTX-derived**.

OpenGD77 is tracked as a future migration candidate because it appears more
product-ready as a handheld user firmware, but it is not accepted as the current
base. MVP-8.28 completed the first feasibility spike and kept the outcome at
`dual_track_research`: OpenRTX-derived remains primary, while OpenGD77-derived
requires a later source/build/license intake before any switch.

Historical decision before ADR-0017:

```text
primary_bench_firmware_path = OpenRTX-derived
future_migration_candidate = OpenGD77-derived ADHR fork
current_primary_switch_to_opengd77 = no
required_before_switch = all OpenGD77 G1-G12 gates pass plus superseding ADR
```

The completed feasibility and source/build intake do not satisfy that switch
gate. A future switch must still pin and checksum an official target source
archive, resolve license and distribution constraints, prove RX/TX control
hooks, custom ADHR frame path feasibility, slot scheduler integration, beacon
timing, relay integration, rollback/calibration backup, concrete advantage over
OpenRTX-derived work, and whether ADHR can remain separate from DMR/AMBE
compatibility. Any switch also requires a new ADR that supersedes ADR-0014.

MVP-8.28 result: `docs/research/016-opengd77-feasibility-spike.md`.
MVP-8.29 result: `docs/research/017-opengd77-source-build-intake.md`.

MVP-8.29 does not change the bench firmware path. It locates the official
OpenGD77 `MDUV380_DM1701/R20260131` source archive by URL/name/size, but leaves
OpenGD77 deferred because the archive was not fully downloaded/checksummed and
secondary mirror evidence is DMR/codec/build coupled with no MD-UV380 target
match.

## 3. First fallback handheld: TYT MD-380 / MD-390 / MD-3x0 family

This is the strongest older/reference handheld fallback.

Why it fits:

- OpenRTX has a dedicated `md3x0` target.
- The OpenRTX README mentions TYT MD-3x0 in the supported tooling path.
- The family has a long history in digital-radio firmware experimentation.
- It is a handheld platform and fits the portable relay capability model.

Project use:

```text
fallback handheld candidate if MD-UV3x0 hardware is unavailable or harder to reproduce
```

## 4. Lab candidate: TYT MD-9600

This is suitable as a bench/lab platform, not as the main portable group-radio target.

Why it fits:

- OpenRTX has a dedicated `md9600` target.
- It can be useful as a stable desk or vehicle-class development fixture.

Project use:

```text
lab / bench target for firmware and integration experiments
```

Important boundary:

```text
MD-9600 may help lab work, but it must not redefine the product as fixed infrastructure.
```

## 5. Secondary experimental handhelds

These are worth tracking as secondary candidates because OpenRTX has related targets for them.

### Radioddity GD-77

Project use:

```text
secondary handheld experiment target
```

Why it fits:

- OpenRTX includes a `gd77` target.
- It is a portable digital-radio family with open-firmware history.

### Baofeng DM-1801

Project use:

```text
secondary handheld experiment target
```

Why it fits:

- OpenRTX includes a `dm1801` target.
- It is a portable digital-radio platform relevant to the same research direction.

### Baofeng DM-1701

Project use:

```text
secondary handheld experiment target
```

Why it fits:

- OpenRTX includes a `dm1701` target.
- It is a portable target in the current OpenRTX build list.

## 6. Specialized experimental platforms

### Connect Systems CS7000 / CS7000-PLUS

Project use:

```text
specialized experimental platform if available
```

Why it fits:

- OpenRTX includes `cs7000` and `cs7000p` targets.
- These platforms sit close to the OpenRTX/M17 ecosystem.

## 7. Shortlist summary

| Priority | Platform | Best use |
| --- | --- | --- |
| 1 | TYT MD-UV380 / MD-UV3x0 class | Main handheld candidate |
| 2 | TYT MD-380 / MD-390 / MD-3x0 family | Older/reference handheld fallback |
| 3 | TYT MD-9600 | Lab/bench candidate |
| 4 | Radioddity GD-77 | Secondary handheld experiment |
| 5 | Baofeng DM-1801 | Secondary handheld experiment |
| 6 | Baofeng DM-1701 | Secondary handheld experiment |
| 7 | Connect Systems CS7000 / CS7000-PLUS | Specialized experimental platform |

## 8. Relationship to the simulator

Hardware selection must not lead the architecture. The simulator remains the source of truth until these pieces are stable:

- canonical frame model;
- one-hop fixed relay;
- duplicate suppression;
- slot/superframe timing;
- metrics schema;
- relay election and control-plane rules.

The future hardware path should adapt the proven simulator logic into a project-specific mode. Stock radio behavior alone is not the goal.

## 9. Source notes

Primary source family:

- OpenRTX README: open-source radio firmware, experimental status, M17 support note, TYT MD-3x0 / MD-UV3x0 tooling note.
- OpenRTX build file: confirms build targets including `md3x0`, `mduv3x0`, `md9600`, `gd77`, `dm1801`, `dm1701`, `cs7000`, and `cs7000p`.

This document is a project shortlist, not a vendor endorsement and not a regulatory approval.
