---
title: OpenRTX Source and Build Intake
created: 2026-07-05
updated: 2026-07-05
status: done
type: technical-research
story_id: MVP-8.30
---

# OpenRTX Source and Build Intake

## 1. Question

Can the current OpenRTX-derived path be pinned and mapped far enough to continue
pre-station firmware planning for TYT MD-UV380 / MD-UV3x0-class radios?

Short answer:

```text
outcome = source_intake_ready_build_blocked_by_local_toolchain
operational_posture = keep_openrtx_primary
candidate_source_pin = OpenRTX v0.4.4
candidate_source_commit = 801fc8dd313eb3ac6f83fd4d3b63ed895effdd56
current_master_observed = ea89d8d7e723a92601bbdb0cce89c28eaa30a5fc
target = mduv3x0
cross_file = cross_cm4.txt
firmware_entry_ready = false
hardware_claim_allowed = false
field_tx_ready = false
```

OpenRTX remains the operational bench-firmware path. The repository has a
stable tag, an explicit `mduv3x0` build target, and visible radio-control entry
points. The local machine does not currently have the required OpenRTX build
toolchain installed, so build reproducibility is not proven in this intake.

Superseding note: MVP-8.33 later closed the build blocker by using an isolated
Docker `linux/amd64` build helper with the Miosix GCC 9.2.0-mp3.2 toolchain.
See `docs/research/019-openrtx-isolated-build-and-resource-budget.md`. This
does not change the hardware, timing, RF, or field-TX gates in this intake.

## 2. Scope

This intake does not:

- vendor OpenRTX source into this repository;
- build, wrap, flash, or run OpenRTX firmware;
- claim OpenRTX compatibility;
- claim DMR, M17, AMBE, or field RF readiness;
- change ADR-0014.

## 3. Source Pinning

Checked on 2026-07-05.

| Item | Result |
| --- | --- |
| Upstream repository | `https://github.com/OpenRTX/OpenRTX.git` |
| Stable candidate tag | `v0.4.4` |
| Stable candidate commit | `801fc8dd313eb3ac6f83fd4d3b63ed895effdd56` |
| Current master observed | `ea89d8d7e723a92601bbdb0cce89c28eaa30a5fc` |
| Project posture | Use the stable tag for reproducible source intake; monitor master separately. |

The stable tag is the candidate pin for future pre-station build work. The
current master was inspected only to check whether the source shape had drifted
in a way that changes the next planning steps.

## 4. Build Intake

OpenRTX's build documentation says the radio firmware path uses Meson, Ninja,
the Miosix GCC toolchain, and target-specific cross files. For MD-UV380 /
MD-UV390 / RT3s-class devices the documented tuple is:

```text
cross_file = cross_cm4.txt
target = mduv3x0
build_dir = build_cm4
setup = meson setup --cross-file cross_cm4.txt build_cm4
compile = meson compile -C build_cm4 openrtx_mduv3x0
wrap = meson compile -C build_cm4 openrtx_mduv3x0_wrap
flash = station_required_and_blocked
```

Local tool availability observed during this intake:

| Tool | Local status | Gate result |
| --- | --- | --- |
| `ninja` | present | pass |
| host `gcc` | present | pass for host-only checks |
| `meson` | missing | blocked |
| `arm-miosix-eabi-gcc` | missing | blocked |
| `radio_tool` | missing | blocked for wrapping/flashing |

No tool installation was performed. Installing the Miosix toolchain and
`radio_tool` is a machine setup task, not a firmware-readiness proof.

## 5. Source Entry Points

The stable tag exposes the expected MD-UV3x0 source shape.

| Need | OpenRTX source path | Intake result |
| --- | --- | --- |
| Target selection | `meson.build` `mduv3x0_src`, `mduv3x0_opts`, target `openrtx_mduv3x0` | pass |
| Target hardware config | `platform/targets/MD-UV3x0/hwconfig.h`, `pinmap.h`, `platform.c` | pass |
| Radio interface | `openrtx/include/interfaces/radio.h` | pass |
| RX/TX controls | `radio_enableRx`, `radio_enableTx`, `radio_disableRtx` | pass as source hooks, not timing proof |
| Bandwidth/mode controls | `AT1846S_BW::_12P5`, `AT1846S_BW::_25`, `AT1846S_FuncMode::{OFF,RX,TX}` | pass as source hooks, not channel proof |
| UV3x0 radio driver | `platform/drivers/baseband/radio_UV3x0.cpp` | pass |
| HR_C6000 companion | `platform/drivers/baseband/HR_C6000_UV3x0.cpp` | source lead only |
| Scheduler/task loop | `openrtx/src/core/threads.c`, `rtx_threadFunc`, `rtx_task()` | source lead only |
| Platform input | `platform_getPttStatus()` in `platform/targets/MD-UV3x0/platform.c` | source lead only |

Important source observation: `radio_enableTx()` includes a short sleep before
AT1846S TX functional mode is enabled. This makes bench timing instrumentation
mandatory; source inspection alone cannot prove TX/RX switching latency.

## 6. Gate Table

| Gate | Status | Evidence |
| --- | --- | --- |
| `source_pin_recorded` | pass | Stable tag `v0.4.4` and commit `801fc8dd...` recorded. |
| `target_mduv3x0_present` | pass | Meson target `mduv3x0` exists. |
| `build_command_identified` | pass | `meson setup --cross-file cross_cm4.txt build_cm4`; `meson compile -C build_cm4 openrtx_mduv3x0`. |
| `local_build_reproducible` | blocked | `meson`, `arm-miosix-eabi-gcc`, and `radio_tool` missing locally. |
| `rx_tx_hooks_identified` | pass | Radio interface and UV3x0 driver hooks identified. |
| `timing_measured` | blocked | Requires physical station or equivalent bench setup. |
| `firmware_entry_ready` | blocked | No station, no build proof, no bench evidence. |
| `field_tx_ready` | blocked | RF/legal/lab gates remain closed. |

## 7. Decision

Continue with OpenRTX-derived pre-station planning.

```text
decision = continue_openrtx_pre_station_path
next_docs = MVP-8.31 ADHR firmware slice design
next_contract = MVP-8.32 simulator-to-firmware contract
```

The next useful work without radios is not flashing. It is defining the minimal
ADHR firmware slice and the contract between the simulator and future firmware
bench logs.

## 8. Sources

- OpenRTX repository:
  <https://github.com/OpenRTX/OpenRTX>
- OpenRTX compiling guide:
  <https://github.com/OpenRTX/openrtx.github.io/blob/master/compiling.md>
- OpenRTX user guide:
  <https://github.com/OpenRTX/openrtx.github.io/blob/master/user_guide.md>
- Existing project Phase A prep:
  `docs/hardware/tyt-md-uv380-phase-a-prestation-prep.md`
