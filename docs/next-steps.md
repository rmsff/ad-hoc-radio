---
title: Next Steps — Ad Hoc Radio
created: 2026-10-21
status: current
---

# Next Steps

The work stopped because the radio link it assumed does not exist on the chosen
hardware ([results](results.md)). The protocol core is kept; what happens next
is decided by **measurements**, in order. No new simulator code until G1.

## 1. Gates

Thresholds are a proposal; whoever continues may adjust them before starting.

| Gate | Question | Pass | If it fails |
| --- | --- | --- | --- |
| **G0 quality** | Is an Opus-class codec needed at all? 20 phrases x 3 conditions, blind A/B/C: stock DMR (AMBE+2) between two radios, Opus 8k, plain FM; digits and short operational phrases, with noise | word and digit accuracy >= 95% in silence, >= 90% in noise | if only Opus >= 8k passes, three relays need a PHY of about 50 kbit/s gross (75 with FEC 2/3); if only FM passes, keep analog store-and-forward and text |
| **G1 PHY truth** | What does a real link deliver? | TYT: turnaround p95 <= 5 ms and >= 25 kbit/s payload; or a CC1101-class link: PER <= 5% at >= 100 kBd through one floor and two walls | if CC1101 has PER > 10% at 38.4 kBd and the TYT fails, stop voice mesh; keep text and short commands |
| **G2 airtime** | Does the measured rate carry the relay tree? | channel load <= 0.8 at one relay, <= 1.0 at three; PTT setup <= 0.3 s | limit live voice to one relay (`live_max_depth = 1`) |
| **G3 end to end** | Three real nodes | p95 end of speech to end of playback for a 5 s phrase <= 2.0 s at one relay, <= 2.5 s at three; digit accuracy >= 90% in noise; frame delivery >= 95% | — |

G1 also answers the open product question: is a node **a stock radio**, or
**a radio plus a fast data module**.

## 2. G1 measurement plan

What the documents already settle:

- **The TYT fails the payload threshold on paper.** The HR-C6000 modem runs at
  9.6 kbit/s gross and a DMR slot carries 3.6 kbit/s raw (about 3.4 net)
  ([ETSI TS 102 361-1 V2.6.1](https://www.etsi.org/deliver/etsi_ts/102300_102399/10236101/02.06.01_60/ts_10236101v020601p.pdf),
  [HR-C6000 manual translation](https://www.opengd77.com/downloads/datasheets/C6000/HR_C6000_translated.pdf));
  receiving raw 4FSK needs a solder modification
  ([OpenRTX MD-UV380 mods](https://github.com/OpenRTX/openrtx.github.io/blob/master/M17/mduv380_mods.md)).
  OpenRTX also waits 50 ms before entering TX on this radio as a workaround
  ([radio_UV3x0.cpp](https://raw.githubusercontent.com/OpenRTX/OpenRTX/master/platform/drivers/baseband/radio_UV3x0.cpp)).
  The AT1846S datasheet gives no switching or PLL lock times
  ([datasheet](https://www.opengd77.com/downloads/datasheets/AT1846S/AT1846S-Datasheet-0.3.pdf)).
  Measuring TYT turnaround only matters for a fallback "one relay, low bit
  rate" mode.
- **CC1101 passes timing by a wide margin.** Direct RX/TX switching is about
  30 us; GFSK up to 250 kBd and 2-FSK up to 500 kBd cover the about 50 kbit/s gross
  (75 with FEC 2/3) that Opus 8k over three relays needs; +10 dBm output; sensitivity -104 dBm at 38.4 kBd and
  -95 dBm at 250 kBd, no published figure at 100 kBd
  ([TI CC1101 datasheet](https://www.ti.com/lit/ds/symlink/cc1101.pdf)).
  The open risks are loss through floors and walls at 433 MHz and the
  license-free limits (EU: 10 mW e.r.p., duty cycle <= 10%,
  [ERC/REC 70-03](https://docdb.cept.org/download/4635)); a relayed voice
  stream repeats airtime for every relay.

Three-relay airtime on CC1101 (arithmetic, four transmissions, ~10-byte header, no ACK): Opus
16 kbit/s at 100 kBd uses about 80% of the channel, 8 kbit/s about 48%,
16 kbit/s at 250 kBd about 32%.

### Stage 0 — no purchases, no emission

1. Record the TYT payload verdict from the documents (above).
2. Optional: time the TYT switching commands in firmware (cycle counter around
   the RX/TX switch, PA disabled). This bounds the software path only; it does
   not prove the RF path is ready.

Steps 3-4 are supporting work, not G1 criteria: they prepare the codec and a
stand-in link for later gates.

3. Run Opus in real time on an ESP32-S3: SILK in fixed point, mono 16 kHz,
   8-16 kbit/s, 20 ms frames; pass with >= 30% CPU headroom
   ([esp_audio_codec](https://components.espressif.com/components/espressif/esp_audio_codec),
   [micro-opus](https://github.com/esphome-libs/micro-opus)).
4. Measure ESP-NOW between ESP32-S3 and ESP32-C6 as a stand-in link for
   store-and-forward blocks (~400 kbit/s at short range, < 20 ms latency,
   [Espressif](https://developer.espressif.com/blog/esp-now-for-outdoor-applications/)).
   This debugs the protocol; it says nothing about 433 MHz propagation.

### Stage 1 — second CC1101 node and 2 W attenuators

5. Conducted CC1101 pair through 40-60 dB of attenuation (never connect two
   CC1101 directly; input limit +10 dBm). Measure PER at 38.4 / 100 / 250 kBd
   with real packet sizes, and direct RX/TX switching time.
6. Through one floor and two walls, within license-free power and duty-cycle
   limits or with a documented right to transmit. Pass: PER <= 5% at
   >= 100 kBd. Reject the class if PER > 10% at 38.4 kBd.

### Stage 2 — only for a TYT fallback mode

7. TYT carrier envelope on a CC1101 RSSI detector through a dummy load and a
   5 W attenuator chain, with PTT wired to a GPIO as the time reference
   (resolution about 10-200 us). Decides whether "one relay, low bit rate" on
   the TYT is possible.

G1 is ready for a verdict when steps 1, 5 and 6 are done; steps 3-4 are
recommended before G2.

### Shopping list (prices seen 2026-10-02)

| Item | Purpose | Price |
| --- | --- | --- |
| CC1101 433 MHz module with SMA (e.g. EBYTE E07-M1101D-SMA) + ESP32 board, or a second LILYGO T-Embed CC1101 Plus | second CC1101 node | about $2.4-3.8 per module; about $55 for a T-Embed |
| 2-3 SMA attenuators, 30 dB, 2 W | conducted CC1101 pair | low cost |
| 50 Ohm dummy load, 10 W, SMA | TYT fallback only | $11-27 |
| first attenuator 30 dB, 5 W | TYT into CC1101, fallback only | $53-249 |

The TYT emits up to about 5 W on UHF; any reception of a TYT signal on a
CC1101 must go through a dummy load and attenuators.

## 3. Alternatives worth testing

Ranked by value per effort with modest hardware.

| Idea | What it decides |
| --- | --- |
| Blind listening test (G0) | whether an Opus-class codec is required |
| Instrument-free timing kit: VOX on the radios and a laptop microphone/speaker; CC1101 as an RSSI envelope detector; firmware timing with the PA off | real turnaround and chain delay |
| CC1101 + Opus: a recorded burst first, then streaming | the only path found to three relays with good voice |
| "One live relay, degrade beyond" policy | a realistic "one relay done well" |
| Analog store-and-forward (record on PTT release, replay) | an FM-like baseline; each relay adds one phrase length of latency |
| Overhear / smart flooding without a per-PTT handshake | removes the 715-2934 byte control cost |
| Fall back to speech-to-text or short codes | a safety net |

Rejected: a two-slot DMR pipeline (needs turnaround of at most 2.5 ms), M17 with
Codec2 3200 (solder mod, rejected codec, load above 2 at one relay), two
frequencies per hop (two radios per node). Some commercial DMR handhelds
already ship a store-and-forward "digital simplex repeater" that replays a call
after PTT release; it is a useful reference for the FM-like baseline.

## 4. Open questions

- CC1101 PER and sensitivity at 100 kBd.
- 433 MHz loss through floors and walls in real buildings.
- Opus CPU load on ESP32-S3 at mono 8/16 kHz.
- Duty-cycle rules for relayed voice at 433 MHz in the target country.
- HR-C6000 RF switching time.
