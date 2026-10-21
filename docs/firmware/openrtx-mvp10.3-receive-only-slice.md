# OpenRTX MVP-10.3 Receive-Only Firmware Slice

> Historical working record from the project's experiments, kept as reference.
> Current state: [docs/results.md](../results.md). Notes marked "not published"
> belong to the project's internal record.

> **Correct-course note (2026-07-12):** This completed package remains
> reference/fallback evidence. ADR-0017 initiated the OpenGD77 evaluation;
> MVP-10.4..10.6 are now complete and ADR-0018 limits OpenGD77 to
> prototype-only with no selected product backend. The first physical
> receive-only gate remains MVP-10.7 and is blocked by recovery/calibration/
> rollback evidence; this package is not the intended product firmware base.

Status: build verified, not flashed

Date: 2026-07-12

## Purpose

MVP-10.3 builds the selected OpenRTX backend with a bounded receive-side host
adapter while keeping RF transmission disabled by construction. The slice is a
firmware integration artifact for the next receive-only bench story. It is not
an OTA protocol implementation, a radio-frame receive proof, or a hardware
measurement result.

The complete checked-in evidence is:

- `firmware/openrtx-mvp10.3/` - source overlay, existing-file patch, dependency
  lock, build helpers, host-native tests, and compact checked-in evidence;
- `configs/firmware-contract/openrtx-mduv3x0-adhr-rxonly-mvp10.3.json` - strict
  build evidence;
- `configs/firmware-contract/openrtx-mduv3x0-adhr-rxonly-rollback.json` - dry-run
  rollback package;
- `internal/firmwareevidence/` - IT-087 strict and negative validation.

Large build outputs are intentionally ignored under
local artifact (not published) and are identified by the checked-in hashes.

## Exact Inputs

```text
OpenRTX tag: v0.4.4
OpenRTX commit: 801fc8dd313eb3ac6f83fd4d3b63ed895effdd56
target: mduv3x0
cross file: cross_cm4.txt
container source path: /src
builder image digest:
  sha256:6a25272b232219b3b7201696125c32a54e0c6d018bca823f3a44a87a8f70a93e
Meson: 1.3.2
Miosix GCC: 9.2.0
Binutils: 2.32
```

The build runs with `--network none` and Meson `--wrap-mode=nodownload`.
OpenRTX's floating `radio_tool` wrap is therefore replaced at build time by an
explicit offline dependency lock:

| Dependency | Commit |
|---|---|
| Codec2 | `a298f3789d7ef9e829049ebe14da8845d97ba8c1` |
| XPowersLib | `b0ed896c17d49a1b823ba0a6323cafd89b3a7d91` |
| radio_tool | `6fe15235814a69a0b43f840935000dd5642aa2f4` |

Each dependency also has a content hash after the OpenRTX package overlay is
applied. See `firmware/openrtx-mvp10.3/dependency-lock.json`.

## Host Boundary

The adapter is a versioned, line-delimited, fixed-buffer protocol. It carries
opaque frame bytes and observations supplied by the host test path.

```text
ADHR1 PING
ADHR1 RESET
ADHR1 RX <rssi_dbm> <hex-frame>
ADHR1 LINK <from> <to> <rssi_dbm> <loss_milli_pct> <delay_ms>
ADHR1 TIMING <authority> <epoch> <acquisition_ms> <drift_ppm>
ADHR1 SCHEDULED_TX ...
```

The operation mapping is:

| Command | Contract-v1 shape | Result |
|---|---|---|
| `RX` | `receive_delivery` | emits `frame_bytes`, `monotonic_ms`, and `rssi_dbm` |
| `LINK` | `link_measurement` | emits the five frozen station/time/loss/delay fields |
| `TIMING` | `timing_observation` | emits authority/epoch/acquisition/drift and monotonic time |
| `RESET` | host reset | restarts observation sequence and increments reset generation |
| `SCHEDULED_TX` | `scheduled_transmit` | always `unsupported_operation` |
| unknown/malformed | none | deterministic bounded error |

Limits:

```text
line buffer: 384 bytes
response buffer: 512 bytes
opaque frame: 128 bytes maximum
partial command timeout: 1000 ms from first byte
USB reads per RTX poll: at most 8 x 32 bytes
USB output queue: 4 x 512-byte fixed slots
USB writes started per RTX poll: at most 1
output overflow policy: drop_newest with aggregated error
```

The USB CDC adapter calls the existing non-blocking `vcom_readBlock`, uses
backend-local `getTick()` milliseconds, and creates no thread. The pure parser
uses static/fixed buffers, no heap, no exceptions, and no RTTI.

Responses are copied into a four-slot static FIFO. `vcom_tryWriteBlock()` only
starts one transfer when the endpoint is ready; it never waits for completion.
When the host stops reading, new responses are dropped after the queue fills.
After space returns, one bounded diagnostic reports the aggregate loss:

```text
ADHR1 ERROR code=output_backpressure dropped=<count>
```

The host-native adapter test holds the endpoint busy, verifies that polling
returns without waiting, then releases it and checks FIFO order, `drop_newest`,
the aggregate drop count, and at most one transfer start per poll. This is a
software integration test, not physical USB throughput evidence.

These host-injected events do not prove that OpenRTX has received canonical ADHR
frames over RF. Physical receive/control behavior remains MVP-10.4 work.

## TX Disabled By Construction

The target has three independent guards:

1. `rtxStatus.txDisable=1` at initialization.
2. Every accepted runtime configuration is forced back to `txDisable=1`.
3. `radio_enableTx()` for `mduv3x0` returns immediately before PA/RF setup.

The linked disassembly is:

```text
080184f8 <radio_enableTx>:
 80184f8: 4770  bx lr
```

The host boundary has no transmit operation. `SCHEDULED_TX` is represented only
to return `unsupported_operation`. No Docker command exposed USB devices, and
no station write, reset, DFU access, or RF test occurred in this story.

## Codec Boundary

The new ADHR source files contain no Codec2, M17, DMR, or AMBE imports or calls.
Undefined symbols in the three new objects are limited to libc helpers, the
pure parser/queue, `getTick`, and existing USB VCOM functions.

This is a narrow source-isolation result only. The baseline OpenRTX `mduv3x0`
target still links Codec2 and M17 code. Therefore the broad candidate
`codec_coupling` gate remains `deferred`; MVP-10.3 does not claim a codec-free
firmware image or backend pass.

## Reproducible Builds

Two independent clean checkouts were built with the same source, cache, image,
container path, environment, and command:

```text
firmware/openrtx-mvp10.3/prepare-dependencies.sh default
firmware/openrtx-mvp10.3/build-isolated.sh build-a
firmware/openrtx-mvp10.3/build-isolated.sh build-b
```

Only validated identifiers are accepted. Destructive scratch operations are
confined to canonical `/tmp/adhr-mvp10-openrtx/dependency-caches/<id>` and
`/tmp/adhr-mvp10-openrtx/runs/<id>` roots (resolved as `/private/tmp` on macOS).

The following outputs were byte-identical:

| Output | Bytes | SHA-256 |
|---|---:|---|
| ELF | 4,327,876 | `57d6263e977d9c3f9291b784aa966c5e133a9c205b3a36a0cfbc4b2687c65ae1` |
| bin | 237,744 | `d1cf3c92afa1bec4c32d9407fb9a149155e8ded85f9d96a9c5d57a2784c2c4aa` |
| wrap | 238,256 | `31399b78bf65016b9a2a6186dd6cac471c9855a84b252dde84cd1ee7120ffc4e` |
| map | 899,576 | `5015233cbf2ba76792f8db99ff504d263c72319ba9c7b1b9bef457b2035ac585` |

Size/section reports, symbol tables, full disassembly, three-object imports, TX
guard and USB try-write disassembly, protocol strings, toolchain, applied source
patch, bundle source hashes, and source isolation were also identical. Build
logs differed because parallel Ninja progress lines appeared in a different
order; logs are informational and are not build inputs or firmware outputs.

`firmware/openrtx-mvp10.3/evidence/` keeps the compact canonical subset. IT-087
parses `bundle-source-hashes.txt` as an exact 15-file inventory (14 bundle files
plus the dependency lock), verifies full overlay/upstream patch coverage, and
binds every checked-in evidence file to the manifest. Large local artifacts are
checked by a separate test that reports an explicit skip when absent.

## Resource Budget

| Resource | Baseline | MVP-10.3 | Delta | Planning limit | Result |
|---|---:|---:|---:|---:|---|
| text | 220,596 | 236,024 | +15,428 | +65,536 flash | pass |
| data | 1,712 | 1,720 | +8 | included below | pass |
| bss | 53,160 | 56,160 | +3,000 | included below | pass |
| small RAM static | 13,912 | 16,920 | +3,008 | +8,192 | pass |
| larger RAM static | 40,960 | 40,960 | 0 | +24,576 dynamic planning budget | pass |
| firmware bin | 222,312 | 237,744 | +15,432 | +65,536 | pass |
| wrapped firmware | 222,824 | 238,256 | +15,432 | +65,536 | pass |

Remaining measured regions:

```text
flash: 630608 bytes
small RAM: 48616 bytes
larger RAM heap/stack region: 89600 bytes
```

These values prove build headroom only. They do not prove runtime stack margin,
USB throughput, RF receive behavior, or physical station stability.

## Rollback Truth

The rollback package binds the exact MVP-10.3 bin/wrap hashes to
`md-uv380-rollback-inputs-20260712.json` and the existing recovery runbooks.
It records:

- intake identity `D219.043` / codeplug `V02.37`;
- a same-family `MD-UV380-AES255(REC)-D219.74` recovery image and prior write
  evidence;
- codeplug readback availability;
- TYT firmware update/DFU recovery mode and procedure references.

It does not prove:

- availability of the exact D219.043 MCU image;
- an exact intake firmware round trip;
- calibration round-trip preservation;
- end-to-end rollback after this exact build.

For that reason `firmware_write_allowed=false`. If this reference package is
ever revisited, it still requires its own install/observe/reset/rollback proof.
The active OpenGD77 sequence prepares rollback evidence in MVP-10.4 and executes
the first physical receive-only install/rollback gate in MVP-10.7.

## Verification

Host and evidence tests:

```bash
firmware/openrtx-mvp10.3/run-host-tests.sh
go test ./internal/firmwareevidence -count=1
```

IT-087 validates:

- strict JSON and canonical hashes;
- exact Scope 9 handoff, MVP-10.1 selection, and MVP-10.2 measurement bindings;
- patch/bundle/dependency/rollback input hashes;
- exact frozen receive/link/timing field names and units;
- parser boundaries, reset, malformed input, timeout, overflow, and TX reject;
- busy-host queue overflow, deterministic recovery, and backpressure reporting;
- both local build outputs when `.artifacts` is present;
- TX guard/non-blocking USB/source isolation evidence and unsafe-claim rejection.

## Handoff To MVP-10.4

MVP-10.4 is the first story that may require a physical station and explicit
operator intervention. It must remain receive-only and collect hashed evidence
for write, first boot, firmware identity, host connection, parser operations,
reset/reconnect, and return to the known-good stock/configuration state.

No transmit-capable or uncontrolled over-the-air test is authorized by this
document.
