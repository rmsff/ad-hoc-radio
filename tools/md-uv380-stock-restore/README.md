# MD-UV380 stock restore kit

Scripts used to return a TYT MD-UV380 10W from custom firmware to official
TYT D219.74 and the original codeplug. The procedure, pass conditions and
fallbacks are in the
[stock restore runbook](../../docs/hardware/tyt-md-uv380-10w-stock-restore-runbook.md).

| File | Purpose |
| --- | --- |
| `extract_official_segments.py` | Decrypts the segments of an official TYT `OutSecurityBin` image. The 1024-byte XOR key is read at run time from the OpenGD77 STM32 loader; no key is stored here. |
| `restore.sh` | Steps `check`, `spi-snapshot`, `flash`, `flash-gd77`, `settings-read`, `settings-write`; each step writes a log and exits non-zero on failure. |
| `spi_compare.py` | Compares an SPI flash snapshot with a baseline by region and 64 KiB block. |

Use these scripts only with firmware you are entitled to use and on a radio you
own. Decrypting vendor firmware may be restricted in some jurisdictions.

Not included and never distributed: official TYT firmware images, the decrypted
segments, radio backups, `radio_tool`, `dmrconfig`, the OpenGD77 loader, md380tools' `md380_read_spi.py`
and pyusb. `restore.sh` expects them under `ROOT` (`tools/`, `fw/`, `baseline/`)
with the sha256 values from the runbook; edit the `ROOT` line at the top of
`restore.sh` for your host.
