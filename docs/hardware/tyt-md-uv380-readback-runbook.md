---
title: TYT MD-UV380 Read-Only Codeplug Backup Runbook
created: 2026-07-07
updated: 2026-07-07
status: draft
type: hardware-runbook
---

# TYT MD-UV380 Read-Only Codeplug Backup Runbook

> Historical working record from the project's experiments, kept as reference.
> Current state: [docs/results.md](../results.md). Notes marked "not published"
> belong to the project's internal record.

## 1. Purpose

This runbook records the first successful read-only interaction with the
physical TYT MD-UV380 station through NanoPi/FriendlyWrt.

This is a codeplug/backup procedure only. It does not flash firmware, does not
write radio settings, does not measure RF TX/RX turnaround, and does not
authorize over-the-air transmission.

Important boundary: the verified `device.img` is an 832 KiB
codeplug/configuration image read by `dmrconfig` (`MEMSZ=0xd0000` in
`uv380.c`). It is not proven to be a complete MCU firmware dump and must not be
described as a full stock firmware backup.

## 2. Verified State

Date: `2026-07-07`

```text
radio_model = TYT MD-UV380
physical_label_power = 5W
physical_label_frequency = 136-174MHz / 400-480MHz
physical_label_fcc_id = POD-MDUV380
connection_host = NanoPi / FriendlyWrt 23.05.5
tool = OpenRTX/dmrconfig, locally built on NanoPi
operation = read-only codeplug backup
write_operations = none
result = two independent readbacks matched byte-for-byte
```

The working cable/radio mode enumerated as:

```text
0483:df11 AnyRoad Technology Digital Radio in USB mode
```

The earlier PL2303 serial cable enumerated as:

```text
067b:23a3 Prolific USB-Serial Controller
/dev/ttyUSB0
```

That serial cable proves USB-UART connectivity, but it was not the mode used by
`dmrconfig` for the successful MD-UV380 readback. For the readback path,
`dmrconfig` expected the `0483:df11` USB device.

Physical label check from a label photo (not published) on 2026-07-07 narrowed the restore
target:

```text
model = MD-UV380
power = 5W
frequency = 136-174MHz / 400-480MHz
fcc_id = POD-MDUV380
```

Therefore this station must not use `MD-UV390` or `MD-UV380-10W*` firmware
images. The label does not resolve `CSV` vs `REC` or `AES255` vs non-AES.

## 3. Tooling

`dmrconfig` upstream:

- <https://github.com/OpenRTX/dmrconfig>

The upstream README lists `TYT MD-UV380` as a supported radio and documents the
read command:

```text
dmrconfig -r [-t]
```

On NanoPi, `dmrconfig` was built temporarily under:

```text
/tmp/adhr-dmrconfig/dmrconfig
```

Supporting local `libusb` build was temporary:

```text
/tmp/adhr-libusb-build/
```

These paths are under `/tmp`; they may disappear after reboot or cleanup. If
they are missing, rebuild the tooling before repeating this runbook.

## 4. Read-Only Procedure

Prerequisites:

1. Radio is connected with the cable/mode that enumerates as `0483:df11`.
2. NanoPi sees the USB device in `lsusb`.
3. No `ser2net` bridge is holding the device.
4. `dmrconfig` exists at `/tmp/adhr-dmrconfig/dmrconfig`.

USB check:

```bash
lsusb
```

Expected device:

```text
0483:df11 AnyRoad Technology Digital Radio in USB mode
```

Read-only backup:

```bash
RUN_DIR="/tmp/adhr-md-uv380-readonly-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$RUN_DIR"
cd "$RUN_DIR"
timeout 120s /tmp/adhr-dmrconfig/dmrconfig -r -t > stdout.log 2> stderr.log
echo "$?" > rc
```

Expected success indicators:

```text
Connect to TYT MD-UV380.
Write codeplug to file 'device.img'.
Print configuration to file 'device.conf'.
rc = 0
```

Output files:

```text
device.img
device.conf
stdout.log
stderr.log
rc
```

## 5. Verified Artifacts

First readback:

```text
NanoPi: /tmp/adhr-md-uv380-readonly-20260707-192133
Local:  local artifact (not published)
```

Second independent readback:

```text
NanoPi: /tmp/adhr-md-uv380-readonly-20260707-192705-verify
Local:  local artifact (not published)
```

SHA-256:

```text
15d22b36b3ccefb8aa200d2e42c268dfdaf64f616e500c4ad9054de537d09210  device.img
128bee77931204e4d0549b3d50b3714d89195b16340271a0427fa1c298ae21cd  device.conf
```

Verification:

```text
device.img cmp=0
device.conf cmp=0
```

This proves the readback is stable across two independent reads.

The local backup directory (not published) is intentionally gitignored because codeplug dumps
can contain local radio configuration, IDs, contacts, frequencies, and other
station-specific data. Do not commit `device.img`, `device.conf`, or protocol
logs unless a later explicit sanitization decision says otherwise.

## 6. Readback Summary

Initial `device.conf` reported:

```text
Radio: TYT MD-UV380
Last Programmed Date: 2025-05-30 14:37:56
CPS Software Version: V02.37
ID: 1234
Name: -
Zone1: channels 1-5
ScanList1: channel 1
GroupList1: contact 1
```

Later live readback on 2026-07-07 produced a fuller `device.conf` with
digital/analog channel tables and contacts:

```text
Digital channels:
  1 Channel1 431.125 MHz
  2 Channel2 432.225 MHz
  3 Channel3 433.325 MHz
  4 Channel4 434.425 MHz
  6 Channel6 141.125 MHz
  7 Channel7 142.225 MHz
  8 Channel8 143.325 MHz
  9 Channel9 144.425 MHz

Analog channels:
  5 Channel5 435.525 MHz, CTCSS 82.5, 12.5 kHz
  10 Channel10 145.500 MHz, CTCSS 82.5, 12.5 kHz

Contact:
  1 Contact1 Group 1
```

The latest repeated readback confirms:

```text
artifact_dir = local artifact (not published)
conf_pair_same = true
conf_sha256 = f1405e5296b282fffd4998f4360c2327f3af40e98a5390b1e52142c5b7caacd1
raw_img_a_sha256 = 12b1124aca29d09bae6507d5744f57be468077e148d5a6cee0a45da5db75b40f
raw_img_b_sha256 = 370b86ebef1b325d71e3a354ae85a3ca0f2533c2c9e8b7cb9277753134c0a4bd
raw_img_pair_diff = first 1024 bytes only, 0x000000..0x0003ff
```

Use `device.conf` as the stable codeplug-content comparison artifact. Keep raw
`device.img` files, but do not treat a single raw image hash mismatch as a
codeplug change until repeated reads and the first-block instability are
accounted for.

## 7. Firmware Restore Package Checkpoint

The official TYT restore candidate downloaded on 2026-07-07:

```text
source_url = https://www.tyt888.com/uploads/file/20260318/20260318113300_1985.zip
local_path = local artifact (not published)
sha256 = 857e66cf2f6975829422ef4d9d172d331b5377e4d5826b052a7730880d46075a
```

The archive is titled `MD-UV380 and MD-UV390 latest firmware 0312` and contains
CPS installers plus multiple firmware images:

```text
CPS MD-UV380 Setup v2.45
CPS MD-UV390 Setup v2.45
MD-UV380(CSV)-V19.74.bin
MD-UV380(REC)-D19.74.bin
MD-UV380-10W(CSV)-V19.74.bin
MD-UV380-10W(REC)-D19.74.bin
MD-UV380-AES255(CSV)-V219.74.bin
MD-UV380-AES255(REC)-D219.74.bin
MD-UV380-10W-AES255(CSV)-V219.74.bin
MD-UV380-10W-AES255(REC)-D219.74.bin
```

Do not flash any of these until the exact radio variant is identified:

```text
MD-UV380 vs MD-UV390 = resolved for this station: MD-UV380
standard vs 10W = resolved for this station: standard 5W, not 10W
CSV vs REC = unresolved
AES255 vs non-AES = unresolved
current firmware family/version
```

For this physical station, exclude:

```text
MD-UV390*.bin
MD-UV380-10W*.bin
MD-UV390-10W*.bin
```

The official package improves restore posture, but it is not the same as a
complete readback of the currently installed stock firmware.

## 7.1 2026-07-07 Identity Recheck

After the physical label check, the same station class was reconnected and read
again through NanoPi:

```text
USB device = 0483:df11 AnyRoad Technology Digital Radio in USB mode
read_dir = local artifact (not published)
verify_remote_dir = /tmp/adhr-md-uv380-ident-20260707-204248-verify/
device_conf_sha256 = 128bee77931204e4d0549b3d50b3714d89195b16340271a0427fa1c298ae21cd
device_img_sha256 = 353478f683adb46153389fb77159c5255a79f3dbc560c089ed94fd699f60abed
```

The new `device.conf` is byte-identical to the original intake backup. The new
`device.img` differs from the original intake backup by exactly one byte
(`1-based offset 8323`, `0xfa -> 0xda`), while two fresh reads matched each
other. Treat `device.conf` as stable and keep both binary images until the
meaning of that byte is understood.

## 7.2 DFU Read-Only Backup

The radio also exposes a DfuSe layout through the same USB device:

```text
Found DFU: [0483:df11], alt=0, name="@Internal Flash   /0x0800C000/01*016Kg,01*064Kg,07*128Kg"
Found DFU: [0483:df11], alt=1, name="@SPI Flash Memory /0x00000000/16*064Kg"
```

`dfu-util` was installed on NanoPi and used only in upload/readback mode
(`-U`). No download/write command (`-D`) was used.

Local artifact:

```text
local artifact (not published)
tar_sha256 = 3176889fbec0ed5c4025249d3b9f6498ebb354bbd1a6951ebb7b03363a93dde8
```

Readback files:

```text
md-uv380-5w-internal-flash-0x0800c000-0x0f4000.bin
size = 999424
sha256 = eed5b294e79942d2b793c0a283cf7b3290a158dfd666913b2519deeb38ed8199

md-uv380-5w-spi-flash-0x00000000-0x100000.bin
size = 1048576
sha256 = 7812b06835a44c9389c16819042916d30020c589540c97cc941ad2129842cb03
```

Interpretation:

- The SPI flash upload is a useful full SPI/codeplug-side baseline.
- The internal flash upload is deterministic but does not look like a normal
  STM32 vector-table firmware image and must not yet be treated as a proven
  restore-capable MCU firmware dump.
- The official TYT `.bin` files are `OutSecurityBin` packaged images and do not
  directly match the raw DFU upload sizes.

## 7.3 TYT Firmware Variant Rules

`Operation.doc` in the official TYT archive states:

```text
Firmware starts from 019 = without AES256
Firmware starts from 119 = AES256 16 groups
Firmware starts from 219 = AES256 255 groups
P = CSV-GPS, GPS version, 100000 contacts
S = GPS-REC, GPS version, 10000 contacts and recording
D = REC, common/no-GPS version
V = CSV, common/no-GPS version
```

For the connected physical station, label and USB evidence resolve:

```text
model = MD-UV380
power = 5W
gps = no GPS evidence; treat as common/no-GPS MD-UV380, not MD-UV390/GPS
```

Remaining unresolved by read-only tooling:

```text
CSV vs REC
AES255 vs non-AES
current installed firmware family/version
```

Do not choose a restore `.bin` until those remaining fields are known from the
radio UI, official CPS, service tooling, or a verified firmware identification
method.

Fastest practical identification method on the radio:

```text
Menu -> Utilities -> Radio Info -> Versions
```

Record the exact firmware version string shown there. Interpret it as:

```text
V019.xxx or V19.xx   = CSV, common/no-GPS, no AES256
D019.xxx or D19.xx   = REC, common/no-GPS, no AES256
V219.xxx or V219.xx  = CSV, common/no-GPS, AES256 255 groups
D219.xxx or D219.xx  = REC, common/no-GPS, AES256 255 groups
```

For this physical `MD-UV380 5W` station, that version string directly narrows
the restore file:

```text
V019/V19  -> MD-UV380(CSV)-V19.74.bin
D019/D19  -> MD-UV380(REC)-D19.74.bin
V219      -> MD-UV380-AES255(CSV)-V219.74.bin
D219      -> MD-UV380-AES255(REC)-D219.74.bin
```

Do not infer `CSV/REC/AES` from `dmrconfig` `CPS Software Version`; that field
is the CPS/codeplug programming software version, not the radio firmware
variant.

Resolved from the radio UI on 2026-07-07:

```text
Firmware = D219.043
CP ver = V02.37
```

Interpretation:

```text
D = REC firmware family
219 = AES256 255 groups firmware family
043 = currently installed firmware revision
CP ver V02.37 = CPS/codeplug programming software version, not firmware family
```

For this physical station, the matching official restore candidate from the
2026-03-18 TYT archive is:

```text
MD-UV380-AES255(REC)-D219.74.bin
```

Do not use the `V219` CSV file unless intentionally converting this station
from REC to CSV and accepting the feature tradeoff. Do not use `D19/V19`
non-AES files as a same-family restore for this currently installed firmware.

## 8. Safety Gates

Allowed without further approval:

```text
lsusb
read-only dmrconfig -r
copying backup artifacts to ignored local storage
hashing and comparing readbacks
offline inspection of copied backup files
```

Blocked without separate explicit approval:

```text
dmrconfig -w
dmrconfig -c against the radio
dmrconfig -u against the radio
firmware flashing
OpenGD77/OpenRTX firmware replacement
any transmit-capable RF test
```

Before any write or firmware experiment:

1. Re-read the current codeplug.
2. Compare against the saved baseline.
3. Write a concrete change plan.
4. Define rollback artifacts.
5. Get explicit approval for the specific write/flash action.

## 9. Next Work

Good next steps:

1. Keep this baseline as the current MD-UV380 intake checkpoint.
2. Identify `CSV` vs `REC`, `AES255` vs non-AES, and current firmware family
   before choosing any official firmware image.
3. Investigate whether calibration/service data can be backed up separately via
   official CPS/service tooling.
4. Investigate why `dmrconfig` emits only a minimal `device.conf` for this
   codeplug.
5. Use official TYT CPS or a verified parser if a fuller channel/contact view
   is needed.
6. Keep RF timing work separate: this readback does not measure TX/RX
   turnaround or channel bitrate.
7. Update bench plans only after a real measurement artifact exists.
