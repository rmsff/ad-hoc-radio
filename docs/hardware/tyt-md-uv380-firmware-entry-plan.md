---
title: TYT MD-UV380 Firmware Entry Plan
created: 2026-07-04
updated: 2026-07-04
status: draft
type: firmware-entry-plan
---

# TYT MD-UV380 Firmware Entry Plan

> Historical working record from the project's experiments, kept as reference.
> Current state: [docs/results.md](../results.md). Notes marked "not published"
> belong to the project's internal record.

> **Correct-course note (2026-07-12):** This plan records the historical
> OpenRTX entry sequence. ADR-0017 initiated the OpenGD77 evaluation and
> requires explicit Analog FM + ADHR modes. MVP-10.4..10.6 are complete;
> ADR-0018 limits OpenGD77 to prototype-only, no product backend is selected,
> and MVP-10.7 remains blocked before any new station write. OpenRTX artifacts
> remain reference/fallback evidence.

## 1. Purpose

This plan defines how the project should enter the firmware phase for the
TYT MD-UV380 / MD-UV3x0 class without confusing simulator readiness with
hardware readiness.

The plan is intentionally split into two phases:

```text
Phase A: prepare now, before the station is available.
Phase B: continue only after a physical station is available.
```

## 2. Current State

The simulator and CLI evidence are ready for planning:

- relay behavior, duplicate suppression, timing, metrics, two-hop evidence, and
  stress cases are covered by checked-in tests and regression-pack cases;
- the fixture-safe bench chain exists:
  `calibrate-channel -> calibrate-tyt -> score-audio -> hardware-readiness`;
- `readiness-dashboard` turns that chain into one readiness report;
- fixture-only readiness stays blocked and does not make a hardware claim.

Current default conclusion:

```text
planning_ready = true
station_required = true
firmware_entry_ready = false
```

## 3. Phase A — Work Allowed Before Station Arrival

These tasks can be prepared without having the radio on hand:

1. Keep the Go simulator regression pack green.
2. Keep the readiness dashboard green with `planning_ready=true`.
3. Prepare bench artifact templates for:
   - channel bitrate / channel-width evidence;
   - RX->TX / TX->RX / RX-valid-after-TX turnaround evidence;
   - audio quality or codec fixture evidence.
4. Review OpenRTX-derived source entry points for:
   - MD-UV3x0 target build path;
   - AT1846S bandwidth selection;
   - RX/TX mode switching path;
   - safe receive-only or dummy-load bench hooks.
5. Preserve the historical ADR-0014/OpenRTX evidence, while current execution
   follows ADR-0017 and the OpenGD77 MVP-10.4..10.6 gates.
6. Prepare a bench log format that can later be converted into:
   - `configs/bench/channel-*.json`;
   - `configs/measurements/tyt-md-uv380-*.json`;
   - `configs/bench/audio-*.json`.
7. Pin the OpenRTX source/build candidate and keep the local toolchain isolated
   from the host system.
8. Define the minimal ADHR firmware slice and simulator-to-firmware contract.
9. Build the pinned OpenRTX `mduv3x0` target in an isolated container and record
   a conservative flash/RAM budget for the first ADHR bench slice.

Phase A preparation artifacts are captured in
`docs/hardware/tyt-md-uv380-phase-a-prestation-prep.md`. Raw capture templates
live under `configs/bench/log-templates/`; they are not direct CLI inputs until
converted from real bench data with `adhr-sim convert-bench-log`.

Additional pre-station artifacts:

- `docs/research/018-openrtx-source-build-intake.md`;
- `docs/research/019-openrtx-isolated-build-and-resource-budget.md`;
- `docs/firmware/adhr-firmware-slice-design.md`;
- `docs/firmware/simulator-to-firmware-contract.md`;
- `configs/firmware-contract/`.

Phase A must not claim:

```text
measured TYT turnaround
measured custom channel bitrate
measured voice quality on the radio
field TX readiness
```

## 4. Phase B — Requires Physical Station

Continue here only when a TYT MD-UV380 / MD-UV3x0-class station or equivalent
bench device is physically available.

Physical station intake started on 2026-07-07 with a read-only MD-UV380
codeplug backup through NanoPi/FriendlyWrt and OpenRTX `dmrconfig`.
See:

- `docs/hardware/tyt-md-uv380-readback-runbook.md`
- `docs/hardware/tyt-md-uv380-suitability-checkpoint.md`

Physical label evidence from a label photo (not published) narrowed the current connected
station to:

```text
model = MD-UV380
power = 5W
frequency = 136-174MHz / 400-480MHz
fcc_id = POD-MDUV380
```

Do not select `MD-UV390*` or `MD-UV380-10W*` firmware images for this station.
The firmware image choice was initially blocked on `CSV` vs `REC`, `AES255` vs
non-AES, and the current firmware family; the radio UI later resolved this
gate as `D219.043`.

Official TYT `Operation.doc` rules for the restore archive:

```text
V = CSV common/no-GPS
D = REC common/no-GPS
P = CSV-GPS
S = GPS-REC
019 = without AES256
119 = AES256 16 groups
219 = AES256 255 groups
```

For this station, only common/no-GPS MD-UV380 5W files remain candidates, but
read-only `dmrconfig`/DFU evidence did not resolve `V` vs `D` or non-AES vs
AES255.

Required final identification step before selecting a `.bin`:

```text
Menu -> Utilities -> Radio Info -> Versions
```

Use the firmware prefix from the radio screen:

```text
V019/V19 = MD-UV380(CSV)-V19.74.bin
D019/D19 = MD-UV380(REC)-D19.74.bin
V219 = MD-UV380-AES255(CSV)-V219.74.bin
D219 = MD-UV380-AES255(REC)-D219.74.bin
```

Do not use `CPS Software Version: V02.37` from `dmrconfig` as firmware variant
evidence.

Resolved current station state:

```text
Firmware = D219.043
CP ver = V02.37
restore_candidate = MD-UV380-AES255(REC)-D219.74.bin
```

This resolves the firmware-family gate for stock restore planning. It does not
authorize flashing; firmware write still requires an explicit action request
and a lab-safe rollback plan.

The suitability checkpoint also records the current ADHR conclusion: stock TYT
firmware is useful as a restore/reference baseline, but not as the custom ADHR
voice-frame platform. The OpenRTX-derived path remains the primary bench
candidate; OpenGD77 remains deferred.

The verified readback means USB/tooling access exists:

```text
USB device = 0483:df11 AnyRoad Technology Digital Radio in USB mode
tool = dmrconfig -r -t
result = two independent readbacks matched byte-for-byte
write_operations = none
```

This does not change the firmware gate. It is an intake checkpoint only, not a
hardware-readiness claim and not approval to write settings or flash firmware.

Required bench tasks:

1. Identify exact model, board revision, and firmware path.
2. Confirm the firmware can be built and flashed or otherwise inspected safely.
3. Measure channel capability under lab-safe conditions:
   - configured channel width;
   - gross bitrate target;
   - frame count;
   - decode success / CRC error counters.
4. Measure turnaround under the same control path intended for the future mode:
   - `rx_to_tx_ready_ms`;
   - `tx_to_rx_ready_ms`;
   - `rx_valid_after_tx_ms`;
   - p50 / p95 summaries or raw sample arrays.
5. Collect audio/codec evidence:
   - fixture-backed score first;
   - real codec or recorded bench artifact later if available.
6. Replace fixture-only inputs with bench-measured JSON artifacts.
7. Run:

```bash
adhr-sim calibrate-channel <channel-bench.json> --report out/bench/channel-report.json
adhr-sim calibrate-tyt <turnaround-bench.json> --report out/bench/turnaround-report.json
adhr-sim score-audio <audio-bench-or-fixture.json> --report out/bench/audio-report.json
adhr-sim hardware-readiness \
  --channel out/bench/channel-report.json \
  --turnaround out/bench/turnaround-report.json \
  --audio out/bench/audio-report.json \
  --report out/bench/hardware-readiness.json
```

8. Run regression and dashboard again:

```bash
adhr-sim regression-pack configs/regression/core-cli-regression.json --out out/regression
adhr-sim readiness-dashboard out/regression/summary.json --report out/readiness-dashboard.json
```

If OpenRTX-derived bench work shows that the platform path is too incomplete
for a usable handheld product, run the ADR-0014 OpenGD77 feasibility spike
before changing the primary firmware path. Do not switch the base only because
OpenGD77 is more product-ready; the spike must prove ADHR-specific control over
custom frames, slot scheduling, beacon timing, relay behavior, RX/TX turnaround,
and codec independence from DMR/AMBE voice paths.

## 5. Firmware Entry Gate

Firmware work can move beyond planning only when all are true:

```text
hardware-readiness outcome is hardware_bench_ready
readiness-dashboard outcome is firmware_bench_ready
firmware_entry_ready = true
hardware_claim_allowed remains false unless explicitly changed by a later ADR
field_tx_ready remains false unless separately approved
```

Transmit-capable firmware work remains blocked unless a separate lab/regulatory
go/no-go exists. The project can still do receive-only, file/loopback, dummy
load, attenuated, or shielded lab work when that specific setup is documented.

## 6. First Firmware Slice After Gate

The first firmware slice should be deliberately small:

```text
receive or bench-loopback first
no product mesh behavior
no encryption
no compatibility claim
no field transmit
```

Candidate first tasks:

1. Build and flash a known baseline firmware image.
2. Add a bench-only diagnostic mode that reports RX/TX mode transition timing.
3. Emit a minimal debug record compatible with the existing
   `calibrate-tyt` input shape.
4. Feed measured JSON into the Go CLI readiness chain.
5. Decide whether live two-hop, delayed relay voice, or a fallback timing model
   is realistic for this hardware.

## 7. Stop Conditions

Stop firmware escalation if any of these happen:

- the measured p95 turnaround maps to `no_live_two_hop_tuple`;
- channel evidence cannot support the selected bitrate / frame budget;
- audio evidence cannot meet the current good-voice threshold;
- flashing or firmware inspection is not reproducible;
- lab-safe conditions are not documented for the specific bench step.

In that case, continue in simulator/bench planning and pick a fallback:

```text
larger slots
delayed relay voice
one-hop only
different hardware candidate
```
