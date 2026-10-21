---
title: TYT MD-UV380 10W Stock Firmware Restore Runbook
created: 2026-10-07
updated: 2026-10-21
status: executed (operator screen check pending)
type: hardware-runbook
---

# TYT MD-UV380 10W Stock Firmware Restore Runbook

> Historical working record from the project's experiments, kept as reference.
> Current state: [docs/results.md](../results.md). Notes marked "not published"
> belong to the project's internal record.

## 1. Purpose

Return the MD-UV380 10W station from the custom OpenGD77/ADHR RX-only firmware
(MVP-10.7, flashed 2026-07-14 and 2026-07-16) to official TYT firmware and the
operator's original settings.

```text
radio = TYT MD-UV380 10W, serial <serial>
family = REC, AES256, 255 key groups (factory firmware D219.043, CP V02.37)
current firmware = OpenGD77 ADHR RX-only (OpenMDUV380_10W_PLUS.bin)
target firmware = official MD-UV380-10W-AES255(REC)-D219.74, application segment only
host = NanoPi / FriendlyWrt, root@<nanopi-host>
```

"AES255" in the official file name is TYT's label for the AES-256 build with
255 key groups.

Never use 5W, CSV, non-AES, MD-UV390 or GPS images for this station.

## 2. Backup Reality (corrected 2026-10-07)

| Backup (2026-07-14) | Status |
|---|---|
| Codeplug `device.img` / `device.conf`, two identical `dmrconfig` reads | real; restore source for settings |
| External SPI flash 1 MiB, two identical vendor-mode reads, prefix matched `dmrconfig` | real; diagnostic baseline |
| "Internal application flash" 999424 B from `dfu-util -U` alt 0 (local intake `md-uv380-10w-preexperiment-restore-20260714`) | **not a firmware backup** |

The internal-flash dumps of both stations (5W 2026-07-07, 10W 2026-07-14) are a
single 1 KiB block repeated 976 times: the TYT bootloader answers DFU uploads
with a fixed block. Two reads matched because the answer is constant. There is
no copy of the exact factory D219.043 image, so the official D219.74 is the
only stock target. For the same reason a DFU readback cannot verify a write;
verification is the radio screen plus a normal-mode codeplug read.

The resource segment `0x00060000` of the official 10W file is byte-identical to
the 2026-07-14 SPI backup at `0x60000..0xBD400`, and identical to the 5W file.

## 3. Proven Path

On 2026-07-09 the MD-UV380 5W returned from OpenRTX to stock D219.74 this way
(inputs pinned in `configs/firmware-contract/md-uv380-rollback-inputs-20260712.json`):

1. Decrypt the `0x0800C000` segment of the official `OutSecurityBin` file.
2. `radio_tool --wrap --radio UV3X0 --segment 0x0800C000:<raw> -o <wrap>`.
3. Radio in DFU: `radio_tool -d 0 -f -i <wrap>` -> `Done!`.
4. Power-cycle in normal mode: stock TYT UI confirmed by the operator.

`dfu-util -D` reported success on that station but did not change the
application; do not use it. The OpenGD77 loader that wrote the current firmware
uses the same TYT bootloader protocol and the same 1024-byte MD-UV380 key, so it
is the fallback writer for the decrypted image.

## 4. Prepared Artifacts

Local copy: not published (gitignored, `SHA256SUMS`).
NanoPi: `/root/adhr-radio/md-uv380-10w-restore/` (`tools/`, `fw/`, `baseline/`,
`logs/`, `snapshots/`, `restore.sh`, `spi_compare.py`). The scripts are in
[`tools/md-uv380-stock-restore/`](../../tools/md-uv380-stock-restore/).

| Artifact | sha256 |
|---|---|
| official `MD-UV380-10W-AES255(REC)-D219.74.bin` | `7b7ed3075a7735cde969402eda163f96bf2931ae55cbcc4fbf62625bdaf04855` |
| decrypted segment `0x0800C000`, len `0xEE800` (SP `0x20002960`, reset `0x080F6525`) | `bcc1821207cb09c4592526dbed2749d4a017c0932800e96b2bcab1b3ed8eb925` |
| resource segment `0x00060000`, len `0x5D400` | `c3cc831f637cab63e8b1e64bb6177285206891160ec2bee9f3b3bb1671b7f508` |
| UV3X0 wrap, 977408 B (fw-info: MD-9600 (UV3X0), 1 segment) | `551d1a037f321b9b1a469a30b9df323c7efdd569d1dd8138ea19702a7dbc0e93` |
| `radio_tool`, aarch64 build 2026-07-08 | `51f0d24f5cbaede37db4e741044168738f05ef3ec438361d24605bc63c9a199c` |
| `dmrconfig`, aarch64 `f9bc00c` (made the 07-14 backup) | `0cddcb2699dc184264c3819a97b19ebf430cf7a1aa5773f4d730cdc39164ab07` |
| codeplug backup `device.img` (2026-07-14) | `ea72cdb13e9866a09c84e260916b82cc2b58c15f23f39090d90de8ee613120c3` |
| SPI backup 1 MiB (2026-07-14) | `13d5390d5c56dee5bb45083686c27cd30dcb04843de518ce5c9b7eb76fd3fa2b` |
| MVP-10.7 bundle `OpenMDUV380_10W_PLUS.bin` (rollback target) | `515839fd99ee87df05b332b1aef9bcdb8d52011ffe701b5a4b7928d07009fcab` |

Key check: the extractor (`extract_official_segments.py`, key from the OpenGD77
loader) reproduces the 2026-07-09 5W segment `a40402dc...`; the NanoPi
`radio_tool` reproduces the 2026-07-09 5W wrap `433f3db6...` byte-for-byte.

## 5. Procedure

All commands run on NanoPi in `/root/adhr-radio/md-uv380-10w-restore`; every
step writes `logs/<time>-<step>.log` and exits non-zero on failure.

| # | Radio mode (operator says) | Command | Writes to radio | Pass condition |
|---|---|---|---|---|
| 1 | DFU (LED red/green) | `sh restore.sh check` | no | one `0483:df11`, `radio_tool --list` sees it, SHA ok |
| 2 | DFU | `sh restore.sh spi-snapshot pre` | no | snapshot saved; report only (read works in bootloader mode) |
| 3 | DFU | `sh restore.sh flash` | yes, app flash | `Done!` |
| 4 | normal | operator: Menu > Utilities > Radio Info > Versions | no | stock UI, D219.74 |
| 5 | normal | `sh restore.sh settings-read` | no | `device.img differing bytes: 0` |
| 6 | normal, only if step 5 > 0 bytes and the operator agrees | `sh restore.sh settings-write`, then step 5 again | yes, codeplug | `device.img differing bytes: 0` |
| 7 | normal | `sh restore.sh spi-snapshot post` | no | report only |

`device.conf` always differs from the backup in line 2 (generation date), so it
is not the pass criterion; the binary `device.img` comparison is.

Fallbacks:

- `radio_tool --list` does not see the radio, or step 4 still shows OpenGD77:
  `sh restore.sh flash-gd77` (OpenGD77 loader, `-m MD-UV380`, decrypted image).
- Radio does not boot: both writers only touch `0x0800C000` and up, the
  bootloader stays intact, so DFU mode remains available; repeat step 3 or
  reflash the MVP-10.7 bundle with the OpenGD77 loader:
  `PYTHONPATH=tools/pyusb/pyusb-1.3.1 python3 tools/opengd77_stm32_firmware_loader.py -m MD-UV380 -f <OpenMDUV380_10W_PLUS.bin>`
  (sha256 in §4).
- Step 7 shows changes in `D219.74 resources` or outside the codeplug and the
  stock UI misbehaves: stop; an SPI write path is not validated.

## 6. Execution 2026-10-20

Logs and snapshots: local artifact (not published).

| Step | Result |
|---|---|
| 1 check (DFU) | one `0483:df11`, `radio_tool --info` = MD-UV380, SHA ok |
| 2 SPI pre (DFU) | read works in bootloader mode; resources identical; codeplug area differs (137 + 431 B in blocks 0x00000, 0x10000) |
| 3 flash | `radio_tool` erased 0x0800C000..0x08100000 (whole sectors), wrote ..0x080FA800, `Done!` |
| 4 normal boot | observed in the session, not logged: re-enumerated with stock DFU layout (`@Internal Flash /0x08000000`, `@1G Flash`, `@SPI Flash` alt 2), valid RTC. Operator screen check of D219.74: **pending** |
| 5 settings read | channels intact; OpenGD77 left garbage group lists 54-59; 1592 B differ |
| 6 settings write | `dmrconfig -w` backup, readback `device.img` sha256 `ea72cdb1...`, 0 bytes differ |
| 7 SPI post | full 1 MiB SPI identical to the 2026-07-14 baseline (`13d5390d...`) |

## 7. Safety

- Flash only in the mode the operator reported; charged battery before step 3; do
  not touch cable or power during step 3.
- Codeplug write (step 6) only with the operator's consent.
- Stock firmware transmits on PTT again; no PTT unless on a permitted frequency or into a dummy load.
- NanoPi credentials are not stored in git.

## 8. Related

- `docs/hardware/tyt-md-uv380-readback-runbook.md` — the 2026-07-14 backups used here.
- `docs/hardware/tyt-md-uv380-suitability-checkpoint.md` — stock firmware and restore baseline.
- `configs/firmware-contract/md-uv380-rollback-inputs-20260712.json` — pinned 5W rollback inputs.
- Earlier planning notes described MVP-10.7 rollback as blocked on exact D219.043
  recovery; this runbook shows that target does not exist and D219.74 is the
  stock restore.
