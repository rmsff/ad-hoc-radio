---
title: TYT MD-UV380 Phase A Pre-Station Preparation
created: 2026-07-04
updated: 2026-07-05
status: draft
type: hardware-preparation
---

# TYT MD-UV380 Phase A Pre-Station Preparation

> Historical working record from the project's experiments, kept as reference.
> Current state: [docs/results.md](../results.md). Notes marked "not published"
> belong to the project's internal record.

> **Correct-course note (2026-07-12):** The OpenRTX entry points below remain
> completed reference evidence. ADR-0017 initiated the OpenGD77 evaluation and
> requires explicit Analog FM + ADHR modes. MVP-10.4..10.6 are complete;
> ADR-0018 limits OpenGD77 to prototype-only, and MVP-10.7 remains the first
> physical gate but is blocked by exact recovery/calibration/rollback evidence.

## 1. Purpose

This document records what is ready before a TYT MD-UV380 / MD-UV3x0-class
station is physically available.

Phase A is preparation only. It creates templates, command paths, and source
entry-point notes that make the later bench session reproducible. It does not
claim measured TYT channel bitrate, measured TX/RX turnaround, measured voice
quality, or field transmit readiness.

## 2. Prepared Artifacts

| Area | Prepared artifact | Later canonical input |
| --- | --- | --- |
| Channel bitrate / bandwidth | `configs/bench/log-templates/channel-bench-log-template.json` | `configs/bench/channel-*.json` for `adhr-sim calibrate-channel` |
| TX/RX turnaround | `configs/bench/log-templates/turnaround-bench-log-template.json` | `configs/measurements/tyt-md-uv380-*.json` for `adhr-sim calibrate-tyt` |
| Audio / codec fixture | `configs/bench/log-templates/audio-bench-log-template.json` | `configs/bench/audio-*.json` for `adhr-sim score-audio` |
| Command package | internal note (not published) | Fixture-safe chain, then bench-measured replacement artifacts |
| Dry-run package | `configs/bench/dry-run/` | Filled pre-station raw logs that convert to fixture-only canonical inputs |
| Firmware gate | `docs/hardware/tyt-md-uv380-firmware-entry-plan.md` | Phase B station-required execution plan |

The log templates are intentionally not accepted directly by the CLI commands.
They are raw capture templates that must be converted into the existing strict
JSON input shapes after real bench data exists.

MVP-8.26 adds the conversion step:

```bash
adhr-sim convert-bench-log <raw-log.json> --out <canonical.json>
```

Filled real bench logs must use `status=captured`; `status=template_only` is
rejected. Pre-station dry-run logs use `status=dry_run_fixture`, which validates
the same conversion path but maps channel and turnaround outputs to
`fixture_only` / `test_fixture=true`.

## 3. Current Verification Commands

Run these before a bench session and again after replacing fixture inputs:

```bash
go test -count=1 ./...
go vet ./...
gofmt -l .
git diff --check
go list -m all
go run ./cmd/adhr-sim regression-pack configs/regression/core-cli-regression.json --out out/regression-phase-a
go run ./cmd/adhr-sim readiness-dashboard out/regression-phase-a/summary.json --report out/readiness-dashboard-phase-a.json
go run ./cmd/adhr-sim convert-bench-log <filled-raw-log.json> --out <canonical-input.json>
```

MVP-8.27 adds a checked-in process rehearsal under `configs/bench/dry-run/`.
Run the commands in that directory's README before the station arrives and
again before replacing fixture logs with real captures.

The expected fixture-safe readiness conclusion before the station exists is:

```text
planning_ready = true
station_required = true
firmware_entry_ready = false
hardware_claim_allowed = false
field_tx_ready = false
```

## 4. OpenRTX Source Entry Points Reviewed

These upstream paths were reviewed as entry-point references on 2026-07-04.
They are not vendored into this repository and do not create a project
compatibility claim.

MVP-8.30 extends this with a pinned source/build intake:

```text
candidate_source_pin = OpenRTX v0.4.4
candidate_source_commit = 801fc8dd313eb3ac6f83fd4d3b63ed895effdd56
target = mduv3x0
build_status = isolated_docker_build_pass
firmware_bin_bytes = 222312
wrapped_firmware_bytes = 222824
```

MVP-8.33 adds the isolated build helper and first resource budget:

- `tools/openrtx-build/`;
- `docs/research/019-openrtx-isolated-build-and-resource-budget.md`;
- `configs/firmware-contract/openrtx-mduv3x0-resource-budget.json`.

| Question | OpenRTX source path | Why it matters |
| --- | --- | --- |
| MD-UV3x0 target path | `platform/targets/MD-UV3x0/` | Board-specific config, platform setup, and pin map live here. |
| Radio interface boundary | `openrtx/include/interfaces/radio.h` | Defines `radio_init`, `radio_enableRx`, `radio_enableTx`, `radio_disableRtx`, and configuration update calls. |
| UV3x0 radio driver | `platform/drivers/baseband/radio_UV3x0.cpp` | Owns AT1846S/HR_C6000 driver instances and RTX GPIO setup for the target. |
| AT1846S bandwidth controls | `platform/drivers/baseband/AT1846S.h`, `platform/drivers/baseband/AT1846S_UV3x0.cpp` | Exposes `AT1846S_BW::_12P5` / `_25` and `setBandwidth`, so 12.5 kHz and 25 kHz are the visible source-level bandwidth choices. |
| RX/TX control points | `AT1846S_FuncMode::{RX,TX,OFF}` and `radio_enableRx` / `radio_enableTx` interface | The later bench hook must measure the same control path intended for the future mode, not a slow user-interface proxy. |
| Digital/baseband companion | `platform/drivers/baseband/HR_C6000_UV3x0.cpp` | Contains target-specific digital/baseband initialization paths that may affect future receive/bench-loopback exploration. |

Reference URLs:

- <https://github.com/OpenRTX/OpenRTX>
- <https://github.com/OpenRTX/OpenRTX/tree/master/platform/targets/MD-UV3x0>
- <https://github.com/OpenRTX/OpenRTX/blob/master/openrtx/include/interfaces/radio.h>
- <https://github.com/OpenRTX/OpenRTX/blob/master/platform/drivers/baseband/radio_UV3x0.cpp>
- <https://github.com/OpenRTX/OpenRTX/blob/master/platform/drivers/baseband/AT1846S.h>
- <https://github.com/OpenRTX/OpenRTX/blob/master/platform/drivers/baseband/AT1846S_UV3x0.cpp>
- <https://github.com/OpenRTX/OpenRTX/blob/master/platform/drivers/baseband/HR_C6000_UV3x0.cpp>
- <https://github.com/OpenRTX/openrtx.github.io/blob/master/at1846_registers.md>

Pre-station firmware design/contract references:

- `docs/research/018-openrtx-source-build-intake.md`
- `docs/research/019-openrtx-isolated-build-and-resource-budget.md`
- `docs/firmware/adhr-firmware-slice-design.md`
- `docs/firmware/simulator-to-firmware-contract.md`
- `configs/firmware-contract/prestation-sim-profile.json`
- `configs/firmware-contract/prestation-bench-log.jsonl`
- `configs/firmware-contract/openrtx-mduv3x0-resource-budget.json`

## 5. Phase B Handoff Checklist

When the station is available, collect these before changing any simulator
assumption:

1. Exact model, board revision, and current firmware path.
2. Channel log converted into `configs/bench/channel-<date>.json`.
3. Turnaround log converted into
   `configs/measurements/tyt-md-uv380-<date>.json`.
4. Audio/codec log converted into `configs/bench/audio-<date>.json`.
5. SHA-256 and artifact references for every bench-measured input.
6. Updated readiness reports from:

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

## 6. Physical Station Intake Checkpoint

On 2026-07-07 a physical TYT MD-UV380 was connected to NanoPi/FriendlyWrt and
read through OpenRTX `dmrconfig` in read-only mode.

Verified facts:

```text
USB device: 0483:df11 AnyRoad Technology Digital Radio in USB mode
physical label: MD-UV380, 5W, 136-174MHz / 400-480MHz, FCC ID POD-MDUV380
operation: dmrconfig -r -t
result: two independent readbacks matched byte-for-byte
write_operations: none
```

Runbook and artifacts:

- `docs/hardware/tyt-md-uv380-readback-runbook.md`
- `docs/hardware/tyt-md-uv380-suitability-checkpoint.md`

The local backup directory (not published) is local-only and gitignored. This checkpoint
proves cable/tooling/readback access and preserves the official TYT firmware
restore archive as a candidate, but it does not prove a complete current-stock
firmware dump and does not measure channel bitrate, TX/RX turnaround, audio
quality, or field readiness.

The label-confirmed restore target is standard `MD-UV380 5W`, so `MD-UV390*`
and `MD-UV380-10W*` firmware files are excluded for this physical station.
The radio UI later resolved the firmware family as `D219.043`, which maps to
the `REC` / `AES255` branch.

Additional read-only DFU backup was captured on 2026-07-07:

```text
internal_flash_upload_sha256 = eed5b294e79942d2b793c0a283cf7b3290a158dfd666913b2519deeb38ed8199
spi_flash_upload_sha256 = 7812b06835a44c9389c16819042916d30020c589540c97cc941ad2129842cb03
```

The SPI upload is useful as a broader backup baseline. The internal upload is
not yet accepted as a proven restore-capable MCU firmware dump.

The radio UI later resolved the installed firmware family:

```text
Firmware = D219.043
CP ver = V02.37
matching official restore candidate = MD-UV380-AES255(REC)-D219.74.bin
```

This identifies the stock restore family but does not by itself prove hardware
readiness for the ADHR firmware path.

The suitability checkpoint preserves stock TYT firmware as a restore/reference
baseline. The historical OpenRTX experiment remains reference evidence; current
ADHR work follows the OpenGD77 sequence in ADR-0017.

## 7. Boundaries

Phase A does not authorize over-the-air transmission. The allowed next work is
planning, fixture-safe CLI regression, receive-only inspection, file/loopback,
or a documented lab-safe bench setup after the device exists.
