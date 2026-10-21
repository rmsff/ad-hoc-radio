# Hardware and Firmware

This page explains the radio side of the project: which handheld was used,
what its hardware can and cannot carry, which open firmware bases were tried,
what was built and flashed, and what went wrong with backups. It is written so
that someone with the same radio can repeat the work without bricking it or
transmitting by accident. The detailed step-by-step records live in
`docs/hardware/` and `docs/firmware/`; this page links to them.

Short version: the protocol core in the simulator is sound, but the stock TYT
radio link is too slow for live voice over several hops. Its RX-to-TX
turnaround has never been measured. The full verdict is in
[results.md](results.md) and the plan forward in [next-steps.md](next-steps.md).

## The radio

The test radio is a **TYT MD-UV380** (dual-band DMR and FM handheld,
136-174 / 400-480 MHz on the label; the FCC grant lists 406.1-480 MHz on UHF; the MD-UV3x0 family also covers the MD-UV390 and RT3S-class
clones). Two units were used: a 5 W unit and a 10 W "Plus" unit. Each needs its
own firmware target and its own stock image.

| Block | Part | Role |
| --- | --- | --- |
| MCU | STM32F405VG (Cortex-M4F) | Runs the firmware. 1 MiB internal flash, 192 KiB SRAM (128 KiB main + 64 KiB CCM) |
| RF transceiver | AT1846S | FM radio-on-chip: synthesizer, RX/TX switching, 12.5 / 25 kHz filters |
| Baseband | HR-C6000 | DMR 4FSK modem and Layer-2 framing (30 ms slots, 60 ms cycle) |
| External flash | SPI NOR | Codeplug, fonts and vendor resources; the DFU layout exposes 1 MiB of it |
| Bootloader | TYT, at `0x08000000..0x0800C000` | USB DFU (`0483:df11`) for firmware writes; untouched by every writer used here |

**Flash and RAM limits.** The application lives at `0x0800C000`. In the
OpenRTX linker layout that leaves 868,352 bytes of flash for the application,
64 KiB of CCM for static data and about 127.5 KiB of main RAM, of which the
40 KiB framebuffer takes the biggest single piece. A stock OpenRTX build uses
about a quarter of the flash, so flash is not the problem. RAM is. The OpenGD77
build for the 10 W Plus target leaves only 1,896 bytes of main RAM and 4,616
bytes of CCM statically free. That is why an unchanged neural codec such as Lyra V2
(about 3.6 MB of models, over 50 KB of encoder state) cannot fit. See
[research.md](research.md) for the codec audit.

**What the stock path can carry.** All figures are arithmetic from datasheets
and standards, not measurements:

| Path | Payload rate | Notes |
| --- | --- | --- |
| HR-C6000 DMR Layer-2, 27-byte buffer | about 3.4 kbit/s net (3.6 kbit/s raw slot payload) | 27 bytes (216 payload bits) per 30 ms burst, one burst per time slot every 60 ms (ETSI TS 102 361-1). Using both slots from one radio is unproven |
| HR-C6000 data modes | 4.8 or 9.6 kbit/s gross | The two rates the chip manual documents |
| Raw 4FSK 9.6k with an MCU modem (M17-style) | about 7.7 kbit/s net, 5.1 with FEC 2/3 | Transmit works on stock hardware. **Receive needs a soldered wire** (`ADC_IVINP` to `PC3`, per the OpenRTX MD-UV380 mod guide) |
| FM voice-band soft modem | about 2.4 kbit/s gross, 1.35-2.0 net | FreeDV-2400B class |
| 19.2 kbit/s | no path | Was a planning assumption; channel width is not symbol rate |

On one shared half-duplex channel every relay repeats the airtime, so the load
grows about `(relays + 1)`-fold. At these rates one live relay works only for
codecs of about 2.4-3 kbit/s, and no accepted codec passes three relays
([results](results.md#4-where-it-stopped)). Even on the
DMR slot grid, a depth-2 relay needs 90 ms before any turnaround and so misses
an 80 ms voice period.

**Turnaround is unknown.** The simulator assumed 1 ms RX-to-TX. Public proxies
for comparable handhelds give 20-58 ms (one outlier at 221 ms), and OpenRTX's
`radio_enableTx()` for this target sleeps 50 ms before keying the AT1846S (a
documented workaround for an unmodulated carrier at the start of TX). Neither
number is a TYT measurement. The proxy study is summarized in
[research.md](research.md); the measurement ideas are below.

## Firmware bases

| | OpenRTX | OpenGD77 |
| --- | --- | --- |
| Pinned source | `v0.4.4`, commit `801fc8dd`, target `mduv3x0` | `OpenGD77_MDUV380_DM1701_20260130.zip` (SHA-256 `97764b2c...`), targets `MDUV380_FW` and `MDUV380_10W_PLUS_FW` |
| License | GPL-3.0-or-later for the changed files (REUSE-managed; bundled parts carry their own licenses) | Extended BSD 3-Clause **with a non-commercial clause**; only stable-release sources are published, beta sources are not |
| Toolchain | Miosix GCC 9.2.0, Meson | Arm GNU 13.3.Rel1 (the archive's metadata still names GCC 7, which no longer builds it) |
| DMR voice | Not supported on this target; links Codec2 and M17 | Needs the AMBE codec copied from an official TYT "donor" firmware |
| Maturity as a handheld | Bench-friendly, fewer user features | Close to a complete user firmware |

**Why OpenGD77 is research-only.** Its license forbids commercial use, and its
digital path depends on donor firmware and DMR/AMBE voice. The project avoided
the donor entirely by building with an empty codec section (all `0xFF`, no
donor input) and stubbing every reachable codec call. That proves a clean
build, not a product base. The disposition is "prototype only": OpenGD77 may
carry bounded, non-commercial receive-only measurements, but neither of its
HR-C6000 transport paths is accepted as the product transport. OpenRTX remains
the cleaner reference for build, telemetry and rollback evidence. Background:
[research/016 OpenGD77 feasibility](research/016-opengd77-feasibility-spike.md),
[research/017 OpenGD77 intake](research/017-opengd77-source-build-intake.md),
[research/018 OpenRTX intake](research/018-openrtx-source-build-intake.md),
[research/020 backend selection](research/020-backend-candidate-selection.md),
and the decision summary in [decisions.md](decisions.md).

## What was built

Every build is **receive-only**: transmit is disabled in code, not just in a
menu. Nothing here demonstrates an ADHR frame received over the air.

**OpenRTX spikes on the 5 W unit (written to the radio).** An early RX-only
OpenRTX build was flashed with `radio_tool`, booted, and showed USB CDC
enumeration and the OpenRTX UI. Follow-up builds added text telemetry over
USB CDC (`ADHR PING`, `ADHR START`, `ADHR RX ...` lines with frequency, mode,
RSSI, squelch and `txdis=1`), commands that force an analog receive profile
(`ADHR SETNFM <hz>`), and point-in-time snapshots (`ADHR SNAP`). Analog NFM
reception from a second handheld was confirmed by ear on VHF and UHF. The stock
firmware offers no live USB control at all: in normal mode it exposes no serial,
audio or command interface.

**OpenRTX MVP-10.3 (built twice, not flashed).** A reproducible contract-v1
slice with a line-based host adapter (`ADHR1 PING / RESET / RX / LINK / TIMING`),
fixed buffers, no heap, and a four-slot non-blocking USB output queue that drops
the newest response under backpressure. `SCHEDULED_TX` always answers
`unsupported_operation`. It adds about 15 KB of flash and 3 KB of static RAM.
Details: [OpenRTX MVP-10.3 slice](firmware/openrtx-mvp10.3-receive-only-slice.md),
sources in [`firmware/openrtx-mvp10.3/`](../firmware/openrtx-mvp10.3/).

**OpenGD77 MVP-10.4..10.7.**

- **10.4, ADHR mode.** Empty-codec build with explicit `Analog FM` and `ADHR`
  application states. Boot defaults to Analog FM; the operator's digital
  selection maps to `ADHR` and shows no DMR labels. One central transition
  function rejects invalid mode changes. Package:
  [`firmware/opengd77-mvp10.4/`](../firmware/opengd77-mvp10.4/).
- **10.5, 27-byte Layer-2 envelope.** A receive hook sits right after the
  HR-C6000's 27-byte SPI read and before codec processing, active only in ADHR
  mode. The envelope is a 5-byte header (`0xA7`, version, length 22, reserved,
  CRC-8/ATM) plus one unchanged 22-byte ADHR short voice frame. A fixed
  four-event queue, no heap; about 432 bytes of code. Proven on host fixtures
  only. Format: [OpenGD77 L2 envelope](protocol/opengd77-l2-envelope-v1.md);
  sources: [`firmware/opengd77-mvp10.5/`](../firmware/opengd77-mvp10.5/).
- **10.6, transport verdict.** Layer-2 is the only justified receive-only
  measurement path. Layer-1 (36-byte physical frames) was not pursued: the
  pinned OpenGD77 handler is empty, and the MCU would have to own all channel
  coding. The source evidence embeds OpenGD77 code and is not published
  ([why](../firmware/README.md)).
- **10.7, USB bench.** A small binary protocol over USB CDC (magic `A`,
  version 1, requests up to 31 bytes, responses up to 39) with `identity`,
  `status`, `set-mode`, `inject` (a 27-byte L2 candidate), `pop`, `lifecycle`
  and `reset`. Dispatch runs in the firmware's communication task, and any TX
  attempt answers `tx_guard`. A dependency-free host client is in
  [`adhr_usb_bench.py`](../firmware/opengd77-mvp10.7/tools/adhr_usb_bench.py).
  Builds exist for both the original and the 10 W Plus targets; their build
  evidence is not published ([why](../firmware/README.md)). The
  Plus image was written to the 10 W unit and ran. No receive, mode or bench
  results were recorded from it, so this step counts as inconclusive (see
  [experiments.md](experiments.md)).

**TX guards.** OpenRTX: `txDisable=1` at init, every runtime config forced back
to `txDisable=1`, and `radio_enableTx()` linked as a single `bx lr` (checked
in the disassembly). OpenGD77 (build flag `ADHR_RX_ONLY=1`): application PTT
dispatch plus seven central and low-level TX functions are guarded. The
low-level guards also reset the PA enable pins and DAC, and the L2 adapter's
`adhrL2TXAllowed()` always returns false.

**Reproducible isolated builds.** Every firmware is built twice from one
read-only input snapshot. Each run extracts source and toolchain independently
into a container with `--network none`, fixed paths and pinned image digests.
ELF, binary, wrap, map, symbols and disassembly must come out byte-identical
before an image counts. Floating upstream dependencies (OpenRTX's
`radio_tool` wrap, Codec2) are replaced by a dependency lock. Helpers:
[`tools/openrtx-build/`](../tools/openrtx-build/README.md) and the
`build-isolated.sh` script in each `firmware/` package. Upstream sources and
compiled images are never committed; only hashes and compact evidence are.
Hashes are indexed in
[`configs/firmware-contract/`](../configs/firmware-contract/README.md).

## Backup and rollback lessons

These lessons cost the most time. The full records are the
[readback runbook](hardware/tyt-md-uv380-readback-runbook.md), the
[suitability checkpoint](hardware/tyt-md-uv380-suitability-checkpoint.md) and
the [10 W stock restore runbook](hardware/tyt-md-uv380-10w-stock-restore-runbook.md).

1. **Identify the exact variant first.** On the radio, open Menu > Utilities >
   Radio Info > Versions. In a name like `D219.043`, the letter is TYT's firmware family
   label (`D` = REC, `V` = CSV); `019`, `119` and `219` mean no AES, AES with 16 key groups and AES with
   255 key groups. Separately, 5 W and 10 W units take different images.
   `CP ver` is the programming-software version, not the firmware family.
   Never flash an image from another family.
2. **Codeplug backup is real.** Use `dmrconfig -r` (OpenRTX fork). Two
   independent reads should match. Raw `device.img` can differ in its first
   1 KiB block between reads, so compare `device.conf` too. Note that
   `device.conf` always differs in its generation-date line.
3. **SPI flash backup is real.** In DFU mode, a vendor-mode upload of the full
   1 MiB external flash gave two identical reads, and its prefix matches the
   codeplug. It works from the bootloader, so it can be repeated before and
   after a write.
4. **The DFU "internal flash dump" is not a backup.** `dfu-util -U` on alt 0
   returns 999,424 bytes that are one constant 1 KiB block repeated 976 times:
   the TYT bootloader answers uploads with a fixed block. The two reads matched
   only because the answer never changes. So no copy of the factory firmware
   exists, and a DFU readback cannot verify a write.
5. **`dfu-util -D` did not change the application**, even though it reported
   success. Do not use it to write firmware.
6. **What does write.** Use `radio_tool --wrap --radio UV3X0` on the decrypted
   application segment (`0x0800C000`) of the official image, then
   `radio_tool -d <index> -f -i <wrap>`. Only proceed when `radio_tool --list`
   shows exactly one TYT device. The OpenGD77 Python loader (`-m MD-UV380`)
   speaks the same bootloader protocol and is the fallback writer. Both write
   only from `0x0800C000` upward, so the bootloader and DFU mode stay
   reachable.
7. **The stock target is D219.74.** The factory D219.043 image cannot be
   recovered, so the official same-family `D219.74` is the restore target. Its
   resource segment is byte-identical to the backed-up SPI region.
8. **Verify by screen and codeplug readback.** After the write, check the
   version screen. Then read the codeplug in normal mode and compare it with
   the backup. In the 10 W restore, OpenGD77 had left stray group-list entries,
   so 1,592 bytes differed. After writing the backup back with `dmrconfig -w`
   the difference was 0 bytes, and the full SPI image matched the July baseline.
   A separate calibration round trip was never demonstrated on its own.
9. **Small tools can destroy backups.** The XMODEM receiver
   ([`tools/xmodem-rx/`](../tools/xmodem-rx/README.md)) opens its output file
   for overwrite, so always give it a new path.

Both units are back on stock D219.74 (the on-screen version check of the
10 W unit is still pending). The restore inputs are pinned
by hash in
[`md-uv380-rollback-inputs-20260712.json`](../configs/firmware-contract/md-uv380-rollback-inputs-20260712.json).

## Measuring without lab gear

The project had no oscilloscope, logic analyzer or SDR. The review proposed three
instrument-free methods for the missing numbers (RX-to-TX turnaround and chain
delay). None has been run yet; the measurement contract they must satisfy is in
[backend measurement contract](firmware/backend-measurement-contract.md).

- **VOX + Mac.** Play a marked tone from the Mac into one radio in VOX mode and
  record the other radio's speaker with the same Mac. One clock measures the
  whole chain: TX attack, squelch opening and audio path. This gives an
  end-to-end delay, not chip timing, and it does transmit, so use a dummy load
  or a permitted frequency.
- **CC1101 as an RSSI envelope detector.** Tune a CC1101 board to 433 MHz,
  poll its RSSI, and wire the radio's PTT to the board's GPIO as a shared time
  mark. Calibrate on a known DMR burst (27.5 ms of every 60 ms). Expected
  resolution is about 10-200 us. This detects carrier energy, not modulation
  readiness. The radio puts out up to about 5 W while the CC1101 input limit is
  +10 dBm, so only do this conductively, through a dummy load and attenuators.
- **Firmware-only timing with the PA off.** Stamp a cycle counter around
  `radio_enableTx/Rx` and the AT1846S mode writes, with the PA forced off so
  nothing radiates. This bounds the software path (including OpenRTX's 50 ms
  sleep) but not RF settling, because the drivers do not wait for PLL lock.

Gate G1 asks for turnaround p95 of at most 5 ms on both sides and at least
25 kbit/s of payload. The TYT already fails the rate threshold on paper, so the
first two methods are optional for deciding the radio link.

## Candidate faster radios

The likely node is a handheld used as microphone and speaker, plus a small
fast-link module. These are options, not decisions; also see
[suitable radio platforms](hardware/suitable-radio-platforms.md) for the
original handheld survey.

**CC1101 (TI, sub-GHz, here 433 MHz).** Datasheet figures:

- GFSK up to 250 kBd, 2-FSK up to 500 kBd, 4-FSK up to 300 kBd
- About +10 dBm output; sensitivity -104 dBm at 38.4 kBd and -95 dBm at
  250 kBd (1% PER, 20-byte packets)
- 64-byte RX and TX FIFOs; packets up to 255 bytes
- Direct RX/TX switch in about 30 us, roughly 150 times faster than the G1
  threshold; going through IDLE with calibration takes up to about 0.8 ms

A 20 ms Opus frame at 16 kbit/s is 40 bytes and fits one FIFO load. With about
10 bytes of header over three relays (four transmissions), Opus 8k would use about 48% of a 100 kBd
channel, Opus 16k about 80%, and Opus 16k at 250 kBd about 32%. On this chip
the risk moves from timing to propagation: there is no datasheet sensitivity
at 100 kBd, wall and floor loss at 433 MHz is unmeasured, and the band has
power and duty-cycle limits (typically 10 mW and 10% duty in the EU). A second
CC1101 node is needed for any real link. Two CC1101s on a cable need 40-60 dB
of attenuation between them. ESP-NOW between two ESP32 boards can stand in for
protocol and store-and-forward tests, but it says nothing about 433 MHz range.

**SX1262 boards.** Semtech's sub-GHz transceiver, common on ESP32 boards, does
LoRa and (G)FSK up to about 300 kbit/s at up to +22 dBm. The project has not
evaluated it. Before choosing it, check its FSK sensitivity at the chosen rate,
its TX/RX switching time and the local band rules against the same G1
thresholds.

## Safety

- **No TX by default.** Every project firmware is receive-only by construction,
  and the guards are checked in the disassembly, not just in the source. Once
  stock firmware is back, PTT transmits again: do not press it except on a
  permitted frequency or into a dummy load.
- **Dummy load and attenuator.** Any experiment that keys the transmitter, or
  that puts a receiver next to it, uses a 50-ohm dummy load or a conductive
  path with enough attenuation. Never connect a 5 W output to a +10 dBm input.
- **Flash carefully.** Flash only in the mode you have checked: DFU (LED
  red/green) for firmware, normal mode for codeplug reads. Start with a
  charged battery, and never touch the cable or power during a write. Back up
  the codeplug and SPI flash first, and keep the backups outside the working
  tree with checksums.
- **Never redistribute vendor firmware.** Official TYT images, decrypted
  segments, UV3X0 wraps, donor codec blobs and OpenGD77 binaries stay local.
  This repository ships only scripts, overlays, patches and hashes. Fetch
  vendor files from the vendor yourself.
- **Keep station data private.** Codeplug dumps contain radio IDs, contacts
  and frequencies. Do not commit `device.img`, `device.conf` or raw logs.
- **Regulation.** This page is not legal advice. Transmitting on amateur or
  ISM bands has power, duty-cycle and licensing rules that differ by country.

See also: [project overview](../README.md), [concept](concept.md),
[frame contract](protocol/frame-contract.md),
[simulator-to-firmware contract](firmware/simulator-to-firmware-contract.md),
[firmware slice design](firmware/adhr-firmware-slice-design.md),
[OpenRTX bench entry plan](firmware/openrtx-md-uv380-bench-entry-plan.md),
[research/019 OpenRTX build and resource budget](research/019-openrtx-isolated-build-and-resource-budget.md).
