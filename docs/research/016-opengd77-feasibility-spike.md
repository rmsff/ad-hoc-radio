---
title: OpenGD77 Feasibility Spike for ADHR Firmware Path
created: 2026-07-05
updated: 2026-07-05
status: done
type: technical-research
story_id: MVP-8.28
---

# OpenGD77 Feasibility Spike for ADHR Firmware Path

## 1. Question

Can an OpenGD77-derived fork become a better future firmware base than the
current OpenRTX-derived bench path for ADHR on TYT MD-UV380 / RT3S-class radios?

Short answer:

```text
outcome = dual_track_research
primary_bench_path = OpenRTX-derived
future_product_candidate = OpenGD77-derived ADHR fork
switch_primary_now = no
station_required_for_final_decision = yes
source_intake_required_before_switch = yes
hardware_claim_allowed = false
field_tx_ready = false
```

OpenGD77 is worth keeping as a serious future migration candidate because it is
closer to a practical handheld user firmware. It should not replace OpenRTX as
the current bench path until the project proves source availability, license
acceptability, ADHR custom-mode integration, and independence from DMR/AMBE
voice paths.

## 2. Scope

This spike is a docs-only feasibility check. It does not:

- flash a radio;
- add OpenGD77 source to this repository;
- add an RF/baseband dependency;
- claim DMR, OpenGD77, OpenRTX, or M17 compatibility;
- claim hardware capability or field transmit readiness.

## 3. Source Evidence

Checked on 2026-07-05.

| Evidence | What it means for ADHR |
| --- | --- |
| OpenGD77 FAQ says commercial use is forbidden, stable release sources are available, beta sources are not released, and the code is under an extended BSD 3-Clause license with a non-commercial clause. | License and source-availability are hard gates before choosing OpenGD77 as the project base. |
| OpenGD77 user guide lists TYT MD-UV380 / Retevis RT3S / Baofeng DM-1701 / Retevis RT-84 as install targets and requires OpenGD77 CPS for flashing/codeplug work. | OpenGD77 is a realistic handheld user-firmware candidate for the target class. |
| OpenGD77 MD-UV380 install notes say normal MD-UV380 / RT3S codeplug is not supported and recommend backing up official firmware/calibration data before install. | The user-product path is mature enough to matter, but device backup and codeplug migration are mandatory. |
| OpenGD77 MD-UV380 install notes require an official MD9600 firmware donor for the MD-UV380 path and explain that the donor is used because the codec is identical enough for that target. | ADHR must not depend on donor-firmware DMR/AMBE voice behavior. Any OpenGD77-derived ADHR mode must prove codec independence. |
| The OpenGD77 downloads release index exposes release/source directories including current release paths. | Source intake is plausible, but must be pinned to a concrete release archive before a migration decision. |
| OpenRTX user guide exposes stable/nightly source-oriented firmware paths, flashing notes, FM mode, IF filter bandwidth 12.5/20/25 kHz, and modulation mode selection. | OpenRTX remains the cleaner current bench path for radio-control experiments, even if it is less product-complete. |

### 3.1 Evidence Matrix

| Area | Current evidence | Current status | Required before switch |
| --- | --- | --- | --- |
| License/distribution | OpenGD77 FAQ reports non-commercial license restriction. | blocker until reviewed | Written allowed-use decision. |
| Source availability | Stable release sources are available; beta/development sources are not. Release index showed 2026 release directories, but this spike did not pin or import an archive. | incomplete | Pin exact stable source archive and checksum. |
| Target support | OpenGD77 docs list MD-UV380 / RT3S-class install targets. | promising | Confirm exact purchased model/board revision. |
| Donor firmware / codec | MD-UV380 install notes require official donor firmware for the OpenGD77 path. | blocker for ADHR dependency | Prove ADHR mode does not depend on donor AMBE/DMR voice. |
| Custom frame path | No ADHR path identified in OpenGD77 source during this docs-only spike. | unknown | Identify where `call_start`, voice frames, and `call_end` can flow. |
| RX/TX control hooks | Not inspected in pinned OpenGD77 source. | unknown | Identify hooks and bench timing points. |
| Beacon / slot / relay scheduler | Not inspected in pinned OpenGD77 source. | unknown | Prove scheduler integration path without breaking user-radio workflow. |
| OpenRTX delta | OpenGD77 appears more product-ready, but ADHR-specific advantage is not proven. | unknown | Show concrete maintainability, UX, timing, or integration advantage over OpenRTX. |

Primary source URLs:

- OpenGD77 FAQ:
  <https://opengd77.org/viewtopic.php?t=7>
- OpenGD77 user guide:
  <https://github.com/LibreDMR/OpenGD77_UserGuide/blob/master/OpenGD77_User_Guide.md>
- OpenGD77 MD-UV380 / RT3S installation notes:
  <https://www.opengd77.com/archive/viewtopic6560.html?t=2380>
- OpenGD77 release/source index:
  <https://www.opengd77.com/downloads/releases/>
- OpenRTX user guide:
  <https://github.com/OpenRTX/openrtx.github.io/blob/master/user_guide.md>

## 4. Feasibility Assessment

### 4.1 Positive Signals

- OpenGD77 is closer to a complete handheld user product than OpenRTX.
- It supports the target class at the user-firmware level.
- It has a working CPS, codeplug workflow, flashing workflow, and a larger
  practical user base.
- It may provide a faster path to a polished handheld UX if ADHR can be added
  cleanly.

### 4.2 Blocking Risks

| Risk | Severity | Why it matters |
| --- | --- | --- |
| Non-commercial license clause | P1 | The project cannot pick a primary base until allowed use and distribution are clear. |
| Stable-only source availability | P1 | The project may not be able to inspect or fork the same code that current users run. |
| Donor firmware / codec path | P1 | ADHR MVP must not depend on AMBE/AMBE+2 or stock DMR voice behavior. |
| DMR/FM-centric architecture | P2 | ADHR needs custom frames, beacon timing, slot scheduling, relay control, and packet identity. These may not fit OpenGD77 cleanly. |
| Hardware variants and backup requirements | P2 | Wrong model/revision or missing calibration backup can make bench work unsafe or hard to revert. |
| Unknown RX/TX control timing | P2 | Product maturity does not prove fast enough turnaround for ADHR live relay timing. |

Explicit no-go rule:

```text
no_go_if_ambe_or_dmr_required = true
no_go_if_donor_firmware_required_for_adhr_voice = true
no_go_if_license_blocks_project_use = true
```

## 5. Hard Gates Before Switching to OpenGD77

OpenGD77 cannot become the primary ADHR firmware path until all gates below are
answered positively.

```text
G1 license_allowed_for_project_use = true
G2 exact_stable_source_release_available = true
G3 source_build_reproducible_for_target = true
G4 no_adhr_dependency_on_donor_ambe_or_dmr_voice = true
G5 custom_adhr_frame_path_identified = true
G6 rx_tx_control_hooks_identified = true
G7 slot_scheduler_integration_feasible = true
G8 beacon_timing_integration_feasible = true
G9 relay_state_machine_integration_feasible = true
G10 bench_safe_measurement_hooks_feasible = true
G11 rollback_and_calibration_backup_plan_exists = true
G12 openrtx_delta_proven = true
```

If any P1 gate fails, keep OpenRTX-derived as primary and treat OpenGD77 only as
a user-product reference.

`G12` exists because "OpenGD77 is more product-ready" is not enough by itself.
The project should switch only if OpenGD77 also gives a concrete ADHR advantage:
better maintainability, safer target support, usable UI/product ergonomics,
clearer bench hooks, lower integration cost, or measured timing advantage.

## 6. Outcome Vocabulary

Future reports should use one of these outcomes:

| Outcome | Meaning |
| --- | --- |
| `keep_openrtx_primary` | OpenGD77 is not viable enough to change the path. |
| `dual_track_research` | OpenRTX remains primary; OpenGD77 remains a candidate. |
| `switch_to_opengd77` | OpenGD77 passes source/license/design checks, proves ADHR-specific advantage, and becomes the preferred next bench path, pending device measurement. Requires a new ADR that supersedes ADR-0014. |
| `reject_opengd77` | OpenGD77 conflicts with ADHR constraints and should not be pursued. |
| `defer_opengd77` | Evidence is insufficient; do not treat this as a soft pass. |

Current outcome:

```text
dual_track_research
```

## 7. Decision

Do not migrate now.

OpenRTX-derived remains the current bench path because it is the cleaner
research route for low-level radio-control experiments already mapped in the
project docs.

OpenGD77-derived remains a serious future migration candidate because it is
more product-ready as a handheld firmware. The project should run a later
source-intake/build spike before any primary-path switch.

## 8. Follow-Up Story

Recommended follow-up after a physical station is available, or earlier if the
project wants to spend time on source intake before hardware:

```text
MVP-8.29 - OpenGD77 source/build intake
```

Suggested acceptance criteria:

1. Pin one OpenGD77 stable source release and record archive checksum.
2. Confirm license terms and allowed project use in `docs/research/013`.
3. Identify source paths for:
   - target hardware abstraction;
   - RX/TX control;
   - FM/DMR mode switching;
   - scheduler/task loop;
   - codec/donor-firmware boundary;
   - flashing and rollback path.
4. Produce a go/no-go table against gates G1-G12 above.
5. Compare OpenGD77 against OpenRTX on ADHR-specific hooks and integration cost.
6. Keep `hardware_claim_allowed=false` unless real bench artifacts exist.
