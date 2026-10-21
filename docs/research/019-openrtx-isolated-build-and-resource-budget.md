---
title: OpenRTX Isolated Build and Resource Budget
created: 2026-07-05
updated: 2026-07-05
status: done
type: technical-research
story_id: MVP-8.33
---

# OpenRTX Isolated Build and Resource Budget

## 1. Question

Can we build the selected OpenRTX `mduv3x0` target in an isolated environment
and estimate whether the TYT MD-UV380-class MCU has enough memory headroom for
the planned ADHR slice?

Short answer:

```text
outcome = isolated_build_pass_resource_budget_promising
source = OpenRTX v0.4.4
commit = 801fc8dd313eb3ac6f83fd4d3b63ed895effdd56
target = mduv3x0
isolation = Docker linux/amd64
firmware_entry_ready = false
hardware_claim_allowed = false
field_tx_ready = false
```

The OpenRTX target builds in an isolated Docker environment. Static memory
headroom is large enough for a conservative ADHR bench slice, provided the first
slice stays small, bounded, receive-only by default, and avoids large queues or
dynamic allocation in timing-critical paths.

## 2. Build Environment

Host:

```text
host_os = macOS arm64
docker = available
docker_daemon = running
```

Container:

```text
platform = linux/amd64
base_image = ubuntu:24.04@sha256:4fbb8e6a8395de5a7550b33509421a2bafbc0aab6c06ba2cef9ebffbc7092d90
toolchain = Miosix GCC 9.2.0-mp3.2
toolchain_installer_sha256 = d71fdac28a6fc4a4c0b41e90fb46c3722805b04e1ddb355d0da2f1b67e50ef3e
```

Helper files:

- `tools/openrtx-build/Dockerfile`;
- `tools/openrtx-build/build-mduv3x0.sh`.

The helper intentionally keeps the OpenRTX checkout and build artifacts outside
this repository by default.

The helper also verifies that tag `v0.4.4` resolves to commit
`801fc8dd313eb3ac6f83fd4d3b63ed895effdd56` before building. This prevents a
future upstream tag move from silently changing the resource baseline. The
Ubuntu base image is pinned by digest; apt package repository state is still not
a certification-grade reproducible snapshot, but it is sufficient for this
pre-station research budget.

## 3. Build Result

Commands executed in the isolated container:

```bash
meson setup --cross-file cross_cm4.txt build_cm4_adhr
meson compile -C build_cm4_adhr openrtx_mduv3x0
meson compile -C build_cm4_adhr openrtx_mduv3x0_bin
meson compile -C build_cm4_adhr openrtx_mduv3x0_wrap
```

Observed outputs:

| Artifact | Size |
| --- | ---: |
| ELF text | 220596 bytes |
| ELF data | 1712 bytes |
| ELF bss | 53160 bytes |
| ELF total `dec` | 275468 bytes |
| `openrtx_mduv3x0_bin` | 222312 bytes |
| `openrtx_mduv3x0_wrap` | 222824 bytes |

The wrapped artifact was produced for measurement only. It was not flashed.

OpenRTX's baseline build pulls its own upstream subprojects, including Codec2
and M17-related source files. This is an upstream baseline firmware fact only:
the ADHR simulator, frame protocol, and future bench slice do not gain a
Codec2 dependency or any M17 compatibility claim from this build.

## 4. Memory Model

OpenRTX's MDx linker script exposes these relevant memory regions:

| Region | Size | Notes |
| --- | ---: | --- |
| `flash` | 868352 bytes | `1M - 48K - 128K`, origin `0x0800C000`; bootloader/settings reserved. |
| `smallram` | 65536 bytes | CCM/static `.data` and `.bss`. |
| `largeram` | 130560 bytes | 128 KiB minus 512-byte main stack; framebuffer, heap, and thread stacks. |

Measured static use:

| Region | Used | Remaining | Use |
| --- | ---: | ---: | ---: |
| flash image | 222312 bytes | 646040 bytes | 25.6% |
| `smallram` static | 13912 bytes | 51624 bytes | 21.2% |
| `largeram` static `.bss2` | 40960 bytes | 89600 bytes | 31.4% |

Largest observed RAM object:

```text
framebuffer = 40960 bytes
```

This means the biggest practical risk is not flash size. It is dynamic RAM
pressure in `largeram`, because heap and thread stacks share the remaining
largeram after the framebuffer.

## 5. ADHR Resource Budget

Recommended first ADHR slice budget:

| Budget | Limit | Reason |
| --- | ---: | --- |
| Flash addition | <= 64 KiB | Keeps the first slice small relative to the 646 KiB free flash budget. |
| `smallram` static addition | <= 8 KiB | Avoids crowding the 64 KiB CCM/static region. |
| `largeram` dynamic addition | <= 24 KiB | Leaves stack/heap room after the framebuffer. |
| Average CPU load | <= 10% | Leaves OpenRTX UI/audio/RTX work headroom. |
| Slot-critical CPU load | <= 20% of the active slot window | Keeps beacon/frame scheduling from consuming slot timing budget. |

The current simulator design is compatible with that budget if firmware data
structures stay bounded:

- neighbor table: fixed-size, small node cap;
- relay candidate list: fixed-size, no unbounded maps;
- frame queues: bounded count, mostly 22-byte short voice frames;
- logs: ring-buffered or streamed, not retained unbounded in RAM;
- no heap allocation inside slot-critical RX/TX code.

## 6. Minimum Platform Requirements

Based on this build, the practical minimum for an OpenRTX-derived ADHR bench
slice should be:

```text
cpu = Cortex-M4-class or better
flash_available_for_app >= 512 KiB
ram_total >= 192 KiB split RAM acceptable
small_static_ram_free_after_base >= 32 KiB preferred
heap_stack_free_after_base >= 64 KiB preferred
hardware_timer_resolution <= 1 ms required
monotonic_log_clock = required
rx_tx_control_hooks = required
```

TYT MD-UV380-class OpenRTX target appears plausible on memory, but this does not
prove CPU timing, RF channel rate, audio quality, or TX/RX turnaround.

## 7. What This Does Not Prove

This build does not prove:

- real TYT MD-UV380 behavior;
- RF/baseband data throughput;
- TX/RX switch latency;
- audio quality;
- slot synchronization;
- field transmit readiness;
- OpenRTX compatibility of ADHR;
- that the future ADHR slice will fit after implementation.

It only proves that the selected OpenRTX source target can be built in isolation
and that the base firmware has enough static memory headroom to continue with a
small, bounded ADHR bench-slice design.

## 8. Follow-Up

1. Keep generated OpenRTX artifacts outside this repository.
2. Add a firmware resource budget check to the future ADHR firmware branch.
3. When the slice exists, compare its `.text`, `.data`, `.bss`, `.bss2`, binary
   size, and top symbols against
   `configs/firmware-contract/openrtx-mduv3x0-resource-budget.json`.
4. Treat any dynamic allocation in slot-critical code as a review finding.
