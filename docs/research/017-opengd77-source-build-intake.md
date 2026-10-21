---
title: OpenGD77 Source and Build Intake
created: 2026-07-05
updated: 2026-07-12
status: done
type: technical-research
story_id: MVP-8.29
---

# OpenGD77 Source and Build Intake

## 1. Question

Can the project pin, inspect, and treat OpenGD77 as a buildable source base for
ADHR on the TYT MD-UV380 / RT3S-class path?

Short answer:

```text
outcome = defer_opengd77
operational_posture = keep_openrtx_primary
official_release = MDUV380_DM1701/R20260131
official_source_archive = OpenGD77_MDUV380_DM1701_20260130.zip
official_source_archive_size_bytes = 2761913
official_source_archive_sha256 = 97764b2cfe6ade4e6ecbbadcfa6dc8b62209e391ac0d05748a807b5031354aa9
official_source_download = complete_on_2026-07-10_follow_up
official_source_inspected = yes_on_2026-07-10_follow_up
mirror_inspected = open-ham/OpenGD77@b843c1f050ebcfdb7aea7fad9182af13bc5ef3e6
mirror_target_match = no
firmware_entry_ready = false
hardware_claim_allowed = false
field_tx_ready = false
```

The initial 2026-07-05 intake could not complete and checksum the official
MD-UV380 / DM-1701 archive. A 2026-07-10 follow-up downloaded the full archive,
recorded the SHA-256 above, and inspected the target-family source. This closes
the source-availability/checksum gap only. It does not prove a successful build,
working SFR, ADHR integration, hardware behavior, or a safe migration path.

## 2. Scope

This is a source/build intake gate. It does not:

- flash a radio;
- add OpenGD77 source or binaries to this repository;
- build OpenGD77;
- claim OpenGD77, DMR, M17, AMBE, or OpenRTX compatibility;
- claim hardware capability or field transmit readiness;
- change the current OpenRTX-derived bench path.

## 3. Source Intake Evidence

Checked on 2026-07-05.

### 3.1 Official Release Index

The official OpenGD77 download index exposed this current target-family path:

```text
https://www.opengd77.com/downloads/releases/MDUV380_DM1701/R20260131/sources_and_tools/
```

Observed files:

| File | Observed size | Project meaning |
| --- | ---: | --- |
| `OpenGD77_20260131_linux_udev.zip` | 2343 bytes | Host setup helper, not firmware source. |
| `OpenGD77_20260131_python_tools.zip` | 69953 bytes | Host tools, not the firmware source archive. |
| `OpenGD77_MDUV380_DM1701_20260130.zip` | 2761913 bytes | Target-family source/tools archive candidate. |

Initial 2026-07-05 download status:

```text
full_archive_download = incomplete_in_current_environment
range_download_supported = yes
largest_contiguous_partial_download_bytes = 262144
checksum_recorded = no
```

This is enough to prove that a stable source archive is published for the
target-family release path. It is not enough to pin the build input or make a
source-level migration decision.

2026-07-10 follow-up:

```text
full_archive_download = complete
archive_path = /tmp/OpenGD77_MDUV380_DM1701_20260130.zip
archive_size_bytes = 2761913
sha256 = 97764b2cfe6ade4e6ecbbadcfa6dc8b62209e391ac0d05748a807b5031354aa9
extracted_source_inspected = yes
build_verified = no
```

Target-source inspection found `DMR_MODE_SFR` only in the `DMR_MODE` enum in
`MDUV380_firmware/application/include/functions/trx.h`; no other source
reference implements that mode. This supports the conclusion that SFR is a
placeholder in this release, not a working feature.

### 3.2 GitHub Mirror Inspection

Secondary evidence inspected:

```text
repository = https://github.com/open-ham/OpenGD77.git
commit = b843c1f050ebcfdb7aea7fad9182af13bc5ef3e6
confidence = secondary_old_mirror_not_target_current
```

Relevant mirror facts:

| Area | Evidence | ADHR implication |
| --- | --- | --- |
| Target support | `firmware/Makefile` supports `GD77`, `GD77s`, `DM1801`, and `RD5R`; no MD-UV380 / RT3S target in this mirror. | The mirror cannot prove MD-UV380 source/build readiness. |
| Build system | `BUILD.md` requires MCUXpresso IDE and NXP MK22 SDK. | Build reproducibility is non-trivial and not verified for target hardware. |
| Codec blobs | `BUILD.md`, `firmware/linkerdata/readme.txt`, and `firmware/source/dmr_codec/codec_bin.S` require external codec binary sections. | ADHR must not depend on this donor/DMR codec path. |
| DMR stack coupling | `firmware/include/hardware/HR-C6000.h` includes `dmr_codec/codec.h` and exposes DMR frame/slot states. | The visible digital path is DMR-centric, not an ADHR custom frame path. |
| RX/TX hooks | `firmware/include/functions/trx.h` and `firmware/source/functions/trx.c` expose `trxSetRX`, `trxSetTX`, `trxActivateRx`, `trxActivateTx`, `trxEnableTransmission`, and `trxDisableTransmission`. | Useful low-level hook names exist in the mirror, but target-current source still must be inspected. |
| Timing hooks | `firmware/source/interfaces/pit.c` uses a 100 us PIT period; `firmware/source/functions/ticks.c` exposes `fw_millis()`. | Potential timing primitives exist, but scheduler fit for ADHR beacon/slot/relay timing is unproven. |
| Hotspot path | `firmware/source/hotspot/uiHotspot.c` contains host-driven DMR/MMDVM-style behavior and TRX transitions. | It may inspire bench hooks, but it is not a ready ADHR packet path. |

## 4. Gate Table

Gate vocabulary:

```text
pass = enough evidence for this gate
incomplete = promising evidence exists, but not enough to accept
blocked = current evidence conflicts with the gate
not_applicable = out of scope for this intake
```

| Gate | Status | Evidence and next requirement |
| --- | --- | --- |
| G1 `license_allowed_for_project_use` | blocked | MVP-8.28 already found license/allowed-use risk. This intake did not produce a written allowed-use decision. |
| G2 `exact_stable_source_release_available` | pass | Official target-family source archive was downloaded and pinned at SHA-256 `97764b2cfe6ade4e6ecbbadcfa6dc8b62209e391ac0d05748a807b5031354aa9`. |
| G3 `source_build_reproducible_for_target` | blocked | Target-current official source is available and inspected, but its build has not been reproduced; toolchain requirements and donor codec sections remain unresolved. |
| G4 `no_adhr_dependency_on_donor_ambe_or_dmr_voice` | blocked | Mirror evidence shows codec binary sections and DMR codec integration. ADHR needs an explicit bypass/custom path before this can pass. |
| G5 `custom_adhr_frame_path_identified` | incomplete | No `call_start`/voice/`call_end` custom ADHR frame path was identified in target-current source. Mirror paths are DMR/FM/hotspot-centric. |
| G6 `rx_tx_control_hooks_identified` | incomplete | Target-current source is inspected and mirror TRX hooks provide leads, but those hooks have not been mapped to an accepted ADHR control path. |
| G7 `slot_scheduler_integration_feasible` | incomplete | Mirror timer/task primitives exist, but ADHR slot scheduling was not mapped into a safe firmware task path. |
| G8 `beacon_timing_integration_feasible` | incomplete | No OpenGD77 target-current beacon timer/injection path was identified. |
| G9 `relay_state_machine_integration_feasible` | incomplete | No ADHR relay state integration point was identified. |
| G10 `bench_safe_measurement_hooks_feasible` | incomplete | TRX hooks suggest measurement points, but no bench-safe instrumentation plan exists for OpenGD77. |
| G11 `rollback_and_calibration_backup_plan_exists` | incomplete | Existing project docs require backup/rollback before station work; no OpenGD77-specific verified plan was produced here. |
| G12 `openrtx_delta_proven` | blocked | OpenGD77 product maturity does not yet prove a better ADHR custom-frame, timing, or bench-integration path than OpenRTX-derived work. |

## 5. Decision

Do not migrate now.

```text
decision = defer_opengd77
operational_posture = keep_openrtx_primary
```

OpenGD77 remains a future migration candidate, not the current engineering base.
The official target archive is downloaded, checksummed, and inspected, so the
source-availability gate is closed. Build reproducibility, allowed-use terms,
codec independence, a clean custom ADHR path, timing acquisition, turnaround,
and lab-safe measurements remain open. The old mirror remains useful as a
warning: it shows visible low-level radio hooks, but also DMR/codec/build
coupling that conflicts with ADHR MVP constraints unless a clean custom mode is
proven.

## 6. Required Follow-Up Before Any Switch

1. Preserve the pinned `OpenGD77_MDUV380_DM1701_20260130.zip` archive and SHA-256;
   repeat source intake only when the selected release changes.
2. Reproduce the target build and record exact toolchain and binary hashes.
3. Resolve allowed-use/license terms in writing before any fork/migration.
4. Prove a custom ADHR frame path that bypasses donor AMBE/DMR voice behavior.
5. Map RX/TX control, slot scheduling, periodic beacon timing, relay state, and
   bench-safe instrumentation in target-current source.
6. Compare that mapped path against the already documented OpenRTX-derived bench
   path.
7. Keep `firmware_entry_ready=false`, `hardware_claim_allowed=false`, and
   `field_tx_ready=false` until real station artifacts and safety gates exist.

## 7. Sources

- Official target-family source index:
  <https://www.opengd77.com/downloads/releases/MDUV380_DM1701/R20260131/sources_and_tools/>
- Official target-family release directory:
  <https://www.opengd77.com/downloads/releases/MDUV380_DM1701/R20260131/>
- Official release root:
  <https://www.opengd77.com/downloads/releases/>
- Secondary GitHub mirror inspected for architecture signals:
  <https://github.com/open-ham/OpenGD77>
- Previous ADHR feasibility spike:
  `docs/research/016-opengd77-feasibility-spike.md`
