---
title: TYT MD-UV380 Suitability Checkpoint
status: draft
type: hardware-suitability-checkpoint
updated: 2026-07-07
---

# TYT MD-UV380 Suitability Checkpoint

> Historical working record from the project's experiments, kept as reference.
> Current state: [docs/results.md](../results.md). Notes marked "not published"
> belong to the project's internal record.

2026-09-05 software qualification: the physical observations below describe the
original device and remain historical. Current OpenGD77 revision
`mvp9.9-e-20260905-r2` has separate original/Plus10W reproducible RX-only builds;
see MVP-9.9 acceptance.
No device was read, flashed, restored or transmitted during that software work.
The old OpenRTX/OpenGD77 candidate labels below do not override ADR-0017/0018;
the current integrated live-voice verdict remains no-go under ADR-0021.

This checkpoint records what the connected TYT MD-UV380 has proven for the
ADHR hardware path and what remains blocked. It is not a hardware-readiness
claim and does not authorize RF transmission.

## Verdict

```text
physical_model = TYT MD-UV380
power_class = 5W
frequency_label = 136-174MHz / 400-480MHz
stock_firmware = D219.043
codeplug_version = V02.37
restore_family_candidate = MD-UV380-AES255(REC)-D219.74.bin

restore_baseline = partial_but_practical_for_current_stage
stock_usb_live_control = not_observed
stock_firmware_adhr_suitability = low
openrtx_derived_path = primary_bench_candidate
opengd77_derived_path = deferred_future_candidate
hardware_claim_allowed = false
field_tx_ready = false
```

The station is useful for ADHR as a bench firmware target, primarily through an
OpenRTX-derived path. The stock TYT firmware is useful as an intake, backup,
restore, and reference baseline; it is not a good platform for custom ADHR
voice frames or live USB control.

## Confirmed Identity

Physical label:

```text
DMR Digital Transceiver
Model: MD-UV380
Power: 5W
Frequency: 136-174MHz / 400-480MHz
FCC ID: POD-MDUV380
```

Radio UI:

```text
Firmware: D219.043
CP ver: V02.37
```

Interpretation:

```text
D = REC firmware family
219 = AES256 255 groups family
043 = currently installed firmware revision
CP ver V02.37 = CPS/codeplug version, not firmware family
```

The same-family official restore candidate is:

```text
MD-UV380-AES255(REC)-D219.74.bin
```

Do not use `MD-UV390*`, `MD-UV380-10W*`, `V219` CSV, or non-AES
`D19`/`V19` files as the same-family restore image for this physical station.

## Backup State

Read-only `dmrconfig` backups succeeded through NanoPi/FriendlyWrt:

```text
radio = TYT MD-UV380
last_programmed = 2025-05-30 14:37:56
cps_software_version = V02.37
radio_id = 1234
radio_name = -
```

Known local artifact sets:

```text
local artifact (not published)
```

The human-readable `dmrconfig` configuration is the preferred comparison
artifact for codeplug content. Raw `device.img` should be kept, but not treated
as a stable byte-for-byte diff source without repeated reads: the latest live
check produced two matching `.conf` files and two raw images that differed only
in the first 1024-byte block.

DFU upload produced additional images:

```text
SPI flash readback:
  md-uv380-5w-spi-flash-0x00000000-0x100000.bin
  size = 1048576 bytes
  sha256 = 7812b06835a44c9389c16819042916d30020c589540c97cc941ad2129842cb03

Internal flash upload:
  md-uv380-5w-internal-flash-0x0800c000-0x0f4000.bin
  size = 999424 bytes
  sha256 = eed5b294e79942d2b793c0a283cf7b3290a158dfd666913b2519deeb38ed8199
```

The internal upload is deterministic, but it does not look like a normal STM32
vector-table firmware image. It must not be described as a proven full stock
firmware restore image.

## USB Behavior

Previously observed programming/DFU-style USB identity:

```text
VID:PID = 0483:df11
product = Digital Radio in USB mode
interfaces = DFU-style, no CDC serial, no USB audio endpoint
```

This is enough for read-only codeplug and DFU-style backup operations. It is
not evidence of live audio, live PTT, or custom frame transport over stock USB.

If the radio is expected to be visible for the next check, it should be
connected in normal mode:

```text
radio powered on normally
normal screen visible
USB cable connected
PTT not pressed
not in red/green DFU or firmware mode
```

## Suitability Questions

| Question | Current answer | Evidence |
|---|---|---|
| Can stock firmware be used as the ADHR protocol platform? | No practical evidence. | It exposes DMR/analog behavior, not custom ADHR voice frames. |
| Can stock USB provide live PTT/audio/frame control? | Not observed. | USB appears as programming/DFU style, not serial/audio. |
| Can we fully restore stock state? | Partial but practical for this stage. | Same-family official firmware candidate plus codeplug/SPI/internal uploads exist; MCU restore dump remains unproven. |
| Is the hardware still useful? | Yes, as a bench target. | OpenRTX supports the MD-UV3x0 family and local source/build intake passed. |
| Is OpenRTX the right next firmware path? | Yes, for bench experiments. | Project docs keep OpenRTX-derived work as the primary path. |
| Is OpenGD77 the right next firmware path? | Not now. | OpenGD77 remains a deferred future candidate with unresolved gates. |
| Can we measure TX/RX turnaround now on stock firmware? | Not enough. | No live USB/PTT control path is proven; RF TX remains blocked without lab safety. |

## Next Correct Live Step

For the immediate connected-radio check:

1. Connect the radio in normal mode, not DFU/firmware mode.
2. Verify USB visibility from NanoPi with `lsusb`.
3. If visible, run read-only codeplug detection with `dmrconfig`.
4. Do not write codeplug, do not flash firmware, and do not transmit.

Expected positive result:

```text
USB device 0483:df11 is visible
dmrconfig read-only detection reports TYT MD-UV380
```

Expected negative result:

```text
no 0483:df11 device
no /dev/ttyUSB* or /dev/ttyACM*
dfu-util lists no DFU device
```

Live check history on NanoPi:

```text
date = 2026-07-07, first check
lsusb = host controllers only
dfu-util -l = no DFU devices listed
/dev/ttyUSB* = none
/dev/ttyACM* = none
radio_visible = false

date = 2026-07-07, after reconnect
lsusb = 0483:df11 AnyRoad Technology Digital Radio in USB mode
dfu-util -l = alt0 Internal Flash, alt1 1G Flash, alt2 SPI Flash
/dev/ttyUSB* = none
/dev/ttyACM* = none
dmrconfig_readonly = OK, Connect to TYT MD-UV380
latest_artifact_dir = local artifact (not published)
latest_tar_sha256 = e840c069ef5b98cea5817aaf25c46bb798d0f42f79874dcb1142c510542e2070
latest_conf_sha256 = f1405e5296b282fffd4998f4360c2327f3af40e98a5390b1e52142c5b7caacd1
latest_conf_pair_same = true
latest_raw_img_a_sha256 = 12b1124aca29d09bae6507d5744f57be468077e148d5a6cee0a45da5db75b40f
latest_raw_img_b_sha256 = 370b86ebef1b325d71e3a354ae85a3ca0f2533c2c9e8b7cb9277753134c0a4bd
latest_raw_pair_diff = 1024 bytes, range 0x000000..0x0003ff
usb_evidence_sha256 = e3e621908d945cb9e1e594db620fd55f19139981bfe9a118a550c803f93deddd
radio_visible = true
```

If the negative result persists, the next physical checks are cable seating,
radio power state, and using the cable that already worked for `dmrconfig`.

## Firmware Path Implication

For ADHR, do not plan around the stock TYT firmware. The useful path is:

1. Keep stock artifacts as a restore/reference baseline.
2. Continue with OpenRTX-derived bench firmware, receive-only by default.
3. Use dummy load, shielding, or attenuated lab setup before any transmit-capable
   experiment.
4. Keep OpenGD77 as a later product-oriented spike only after its gates are
   resolved.

This preserves the project boundary: custom ADHR voice frames are not DMR, and
stock MD-UV380 DMR behavior is not an ADHR compatibility proof.
