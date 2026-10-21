---
title: MVP-10.1 Backend Candidate Selection
created: 2026-07-12
updated: 2026-07-12
status: done
type: technical-research
story_id: MVP-10.1
---

# MVP-10.1 Backend Candidate Selection

## Decision

```text
selection_scope = receive_only_firmware_slice
selected_candidate = openrtx_mduv3x0
selected_source = OpenRTX v0.4.4
selected_commit = 801fc8dd313eb3ac6f83fd4d3b63ed895effdd56
selected_target = mduv3x0
opengd77_mduv380 = deferred
permanent_architecture_coupling = false
firmware_write_allowed = false
hardware_claim_allowed = false
field_tx_ready = false
```

OpenRTX is selected only as the first implementation base for Story 7.3 /
MVP-10.3. It has the strongest current source, target, isolated-build,
receive-only control/telemetry, and pinned rollback-input inventory. This decision does not
select an RF timing authority, modulation, codec, field firmware, or permanent
product base.

OpenGD77 remains a future candidate. It is not selected for MVP-10.3 because
its target build is not reproduced, its license prohibits commercial use, its
visible digital path remains donor/DMR/codec-coupled, and no clean ADHR
receive/control integration path or project-specific rollback package has been
accepted.

## Frozen Inputs

The selection is an overlay on the immutable MVP-9.8 handoff:

```text
contract_version = adhr-sim-fw-contract-v1
canonical_sha256 = d97a01b0160fe2c125574ffc1c1125a9da20c2d20e0508d35d565977d38ec39a
profile = source_led/project_native_variable
frame_period_ms = 80
max_relay_depth = 3
max_serialized_branch_width = 4
spatial_reuse_enabled = false
```

Story 10.1 does not modify that fixture or its hash.

## OpenRTX Evidence

Official source was refreshed from
<https://github.com/OpenRTX/OpenRTX/tree/v0.4.4> on 2026-07-12.
`git ls-remote --tags` still resolved `v0.4.4` to the accepted commit and exposed
no newer stable tag.

```text
commit = 801fc8dd313eb3ac6f83fd4d3b63ed895effdd56
git_tree = b160ca5e5d399a66b89c5313bbad4a368e28ffd3
target = mduv3x0
```

Source evidence hashes:

| Upstream file at `v0.4.4` | SHA-256 | Meaning |
| --- | --- | --- |
| `REUSE.toml` | `0b12c5ac014a71d1c67624b638202c06c68e745946ef62fe07d3f8e243870c03` | SPDX/mixed-license map; default OpenRTX paths are GPL-3.0-or-later. |
| `openrtx/src/rtx/rtx.cpp` | `55d686670a66db0bf9f1e9e8406374119579162f45de2c2493985de156702965` | RTX state/config path used by receive-only guard. |
| `platform/drivers/baseband/radio_UV3x0.cpp` | `087fad650668bb72b34c59bada8bce12a384e8578e6f007b5a3605825daad79f` | MD-UV3x0 RX/TX radio hooks. |
| `platform/targets/MD-UV3x0/hwconfig.h` | `7b5dc736fb73f7a75a47c5923ca6a04c041f84e7d3ec73a68f959b14f2ca72a5` | Target capability/build flags. |
| `meson.build` | `88aa38df294cf6e68d928db4bf76cc20fd59147af04132e57734432c0392d07f` | Target source and Codec2 dependency graph. |

### License Scope

The OpenRTX paths changed by the receive-only patch carry
`SPDX-License-Identifier: GPL-3.0-or-later`. The repository is REUSE-managed and
also contains Miosix, ST, CMSIS, fonts, libraries, and subprojects under their
own licenses. For this project stage, source inspection, local modification,
and isolated non-field research builds are acceptable engineering inputs.

This is not a blanket distribution/commercialization approval. Any distributed
firmware/source package must satisfy GPL source obligations and every applicable
component license. A product/commercial distribution decision requires a
separate legal review.

### Build And Target

- `mduv3x0` is a first-class Meson target.
- Docker `linux/amd64`, Ubuntu 24.04 by pinned digest, and Miosix GCC
  `9.2.0-mp3.2` already produced the target.
- The existing receive-only patch compiles `radio_enableTx()` to immediate
  return and forces `txDisable=1` in RTX state/config updates.
- The telemetry build proved a dedicated USB CDC command/observation path with
  `ADHR PING`, `ADHR START`, `ADHR STOP`, and `ADHR RX ... txdis=1` lines.
- Existing evidence still says `adhr_frame_rx=not implemented`; Story 10.3 must
  implement the bounded contract-v1 receive-side host boundary rather than
  reinterpret analog telemetry as ADHR frame reception.

### Codec Coupling

The baseline `mduv3x0` target links Codec2 and includes M17 source. Therefore
the broad backend gate remains:

```text
codec_coupling = not_evaluated
```

Selection is nevertheless valid for a receive-only control/observation slice
because MVP-10.3 is required to implement that path without codec calls and
with scheduled transmit rejected. If source review or symbol/import checks in
MVP-10.3 cannot prove that isolation, the OpenRTX selection becomes `no_go` for
the slice and no firmware write is authorized.

### Rollback Inputs

Existing project evidence includes:

- two byte-identical intake SPI/codeplug readbacks;
- OpenRTX DFU mode and `radio_tool` write path;
- an official same-family `MD-UV380-AES255(REC)-D219.74` recovery image and a
  successful `radio_tool` write log;
- a post-recovery factory codeplug image and write log;
- checked-in manifest
  `configs/firmware-contract/md-uv380-rollback-inputs-20260712.json` with exact
  hashes and sizes for every local recovery input.

This does **not** prove an exact `D219.043` MCU image round trip: no exact
`D219.043` MCU image was captured. It also does not prove calibration round
trip or the end-to-end rollback gate. Those claims remain false until MVP-10.4.
The passed Story 10.1 check means only that pinned recovery inputs exist for
packaging; it is not a hardware or rollback-success verdict.

MVP-10.3 must package these inputs but does not write a station. MVP-10.4 owns
the next physical install/observe/rollback operation.

## OpenGD77 Evidence

Official source archive was downloaded again from the target-family release on
2026-07-12:

```text
release = MDUV380_DM1701/R20260131
archive = OpenGD77_MDUV380_DM1701_20260130.zip
archive_size_bytes = 2761913
archive_sha256 = 97764b2cfe6ade4e6ecbbadcfa6dc8b62209e391ac0d05748a807b5031354aa9
root_license_sha256 = 48521640a1a8ac8309b9364f3ccbbc1570e773585bc50d37bd1c54106dc89f28
```

The root license is BSD-like with an additional clause forbidding commercial
use, including use in a project that permits commercial use. This is sufficient
for the current non-commercial source review but not for a general project
allowed-use decision.

Target source includes explicit `dmr_codec` code, `codec_bin.S`, and
`codec_cleaner` tools. The archive still does not provide a reproduced build or
an accepted custom ADHR receive/control path independent of donor/DMR voice.
The `DMR_MODE_SFR` symbol remains an enum placeholder, not an implemented SFR
mode. OpenGD77 is therefore `deferred`, not silently accepted because its user
firmware is more product-complete.

Primary sources:

- <https://www.opengd77.com/downloads/releases/MDUV380_DM1701/R20260131/sources_and_tools/>
- <https://opengd77.org/viewtopic.php?t=7>

## Candidate Check Summary

| Check | OpenRTX | OpenGD77 |
| --- | --- | --- |
| Source pin | passed | passed |
| Target support | passed | passed |
| Reproducible build | passed | deferred |
| License for current scoped use | passed with distribution caveat | deferred due non-commercial restriction |
| Codec coupling | deferred for full ADHR voice; must be source-proven in MVP-10.3 | deferred; donor/DMR coupling unresolved |
| Custom ADHR receive path | passed as identified/partially proven USB path; frame delivery remains MVP-10.3 | deferred |
| Rollback inputs | passed for input availability only; exact D219.043/calibration round trip remains unproven | deferred for an OpenGD77 ADHR fork |
| Hardware evidence boundary | passed: no hardware/TX claim | passed: no hardware/TX claim |

## Selection Guardrails For MVP-10.3

1. Use only OpenRTX `v0.4.4` commit `801fc8dd...` and target `mduv3x0`.
2. Keep TX disabled at RTX configuration and radio-driver entry points.
3. Implement receive/observation/error/unsupported-operation boundary without
   calling Codec2, M17, DMR, AMBE, or donor codec behavior.
4. Build twice from clean worktrees in the pinned isolated environment.
5. Record source/toolchain/patch/binary/map hashes and explain any binary drift.
6. Produce backup/rollback package only; do not flash a radio in MVP-10.3.
7. Preserve `hardware_claim_allowed=false` and `field_tx_ready=false`.

## Verdict

```text
openrtx_mduv3x0 = selected_receive_only
opengd77_mduv380 = deferred
next_story = MVP-10.2
firmware_story_after_contract = MVP-10.3
```

Candidate sequencing is now explicit. No backend measurement gate has been
closed by this selection report.
