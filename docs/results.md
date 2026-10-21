---
title: Results — Ad Hoc Radio
created: 2026-10-21
status: current
---

# Results

One-page summary of what the research established, what it rejected, and where
it stopped. Every row links to its evidence. The per-experiment record with
hypotheses and expectations is [experiments.md](experiments.md). The most
authoritative assessment is the project's full review of 2026-10-02; where it
corrects an earlier claim, this page follows the review.

Evidence grades used below:

- **measured** — observed on real hardware;
- **simulated** — deterministic simulator or offline computation;
- **assumption-based** — simulated, but the outcome depends on a parameter
  that was assumed, not measured (most often the 19.2 kbit/s channel rate);
- **sourced** — taken from a datasheet, standard or source code.

## 1. What works

| Result | Grade | Evidence |
| --- | --- | --- |
| Per-PTT bounded relay tree: deterministic planning of zero to three relays, ranked by coverage, depth, transmissions, link risk and battery | simulated | [ADR-0015](decisions.md#adr-0015-bounded-per-ptt-scheduled-relay-tree), [concept](concept.md#5-simulator) |
| `A -> relay -> B` with the direct link disabled; duplicate suppression, jitter buffer, metrics; same seed gives the same bytes | simulated | `configs/scenarios/one-hop.json`, `configs/regression/` |
| Relay election, group floor and PTT contention policies | simulated | [ADR-0009](decisions.md#adr-0009-controlled-single-relay-election), [ADR-0013](decisions.md#adr-0013-group-floor-busy-and-collision-policy) |
| Compact peer control (schedule permits) and actual wire cost per hop | simulated | [ADR-0022](decisions.md#adr-0022-compact-control-and-offline-listening-gate), [compact control](protocol/compact-control-v1.md) |
| Reproducible receive-only firmware builds for TYT MD-UV380: OpenRTX with ADHR hooks and TX guards, OpenGD77 with an ADHR mode, a 27-byte Layer-2 envelope and a USB bench interface; Analog FM preserved | measured (build reproducibility, flashing) | [OpenRTX slice](firmware/openrtx-mvp10.3-receive-only-slice.md), `firmware/openrtx-mvp10.3/evidence/`, [firmware research](research.md#7-firmware-bases-openrtx-versus-opengd77); OpenGD77 build evidence is not published |
| Return to stock firmware: MD-UV380 5W back to stock D219.74 (stock UI confirmed on screen); MD-UV380 10W back to D219.74 with codeplug readback identical and SPI flash identical to the backup; the on-screen version check of the 10W unit is still pending | measured | [10W restore runbook](hardware/tyt-md-uv380-10w-stock-restore-runbook.md), [readback runbook](hardware/tyt-md-uv380-readback-runbook.md) |
| Offline codec pipeline: five codec profiles, 74 audio files, 1440 transport rows, reproduced twice | simulated | [codec research](research.md#8-codecs-codec2-opus-lyra) |

## 2. Negative results (worth keeping)

| Result | Grade | Evidence |
| --- | --- | --- |
| No DMR compatibility path: ADHR is project-native | decision | [ADR-0001](decisions.md#adr-0001-custom-protocol-instead-of-dmr) |
| HR-C6000 baseband: Layer-2 (27 bytes per 30 ms burst, one burst per time slot every 60 ms) is usable only as a receive-only measurement path; Layer-1 (36-byte frames) has an empty handler in OpenGD77 and unknown payload/coding; no accepted product transport | sourced | [TYT channel research](research.md#6-tyt-md-uv380-channel-rate-turnaround-and-baseband), [ADR-0018](decisions.md#adr-0018-opengd77-prototype-only-no-hr-c6000-product-transport) |
| OpenGD77 is non-commercial by license: research vehicle only | sourced | [ADR-0018](decisions.md#adr-0018-opengd77-prototype-only-no-hr-c6000-product-transport) |
| Google Lyra does not fit the MD-UV380 MCU unchanged: models exceed internal flash, working memory exceeds spare RAM | sourced | [codec research](research.md#8-codecs-codec2-opus-lyra) |
| Codec2 2400/3200 rejected for field use by listening (one listener, two clips) | measured (subjective) | [codec research](research.md#8-codecs-codec2-opus-lyra) |
| Relayed Opus 6-8k does not fit one relay at an assumed 19.2 kbit/s, even with zero ADHR headers | assumption-based | [codec research](research.md#8-codecs-codec2-opus-lyra) |

## 3. Results that rest on an assumption

| Result | Why it is not proof |
| --- | --- |
| Store-and-forward through two relays: first audio 1391 ms, completion 1783 ms for one 0.5 s burst of a 12 kbit/s FM-like proxy | computed for a single burst at an assumed 19.2 kbit/s channel and 200 ms turnaround; no codec, no radio. Continuous 12 kbit/s speech through two relays needs about 36 kbit/s of airtime and would not fit even 19.2 kbit/s ([experiments](experiments.md#timing--capacity)) |
| Codec2 2400 fits one relay with 80 ms grouping and 2 ms cycle margin | same assumed 19.2 kbit/s ([codec research](research.md#8-codecs-codec2-opus-lyra)) |
| Integrated profile: 29/58/87/116 ms per cycle for 0-3 relays | assumes 19.2 kbit/s and 1 ms for every RX/TX switch ([ADR-0021](decisions.md#adr-0021-byte-derived-voice-profile-and-live-voice-no-go)) |

About 15 completed work items are tools and dry runs; no bench-measured link
artifact exists ([experiments](experiments.md)).

## 4. Where it stopped

| Blocker | Detail | Evidence |
| --- | --- | --- |
| Channel rate | The HR-C6000 manual states 4.8 and 9.6 kbit/s data; a DMR time slot carries 27 bytes per 30 ms burst every 60 ms, 3.6 kbit/s raw. Realistic TYT paths give about 3.4-7.7 kbit/s net (3.38 for DMR Layer-2, 7.67 for raw 4FSK without FEC). 19.2 kbit/s has no path on this hardware. | [TYT channel research](research.md#6-tyt-md-uv380-channel-rate-turnaround-and-baseband) |
| Airtime grows with relays | One shared half-duplex channel repeats the airtime for every relay, so load grows about `(n + 1)`-fold with `n` relays. Opus 8k over three relays needs about 50 kbit/s gross (75 with FEC 2/3); Opus 6k needs 37.5 (56.3). | same; `scripts/review/capacity_20261002.py` |
| Turnaround unmeasured | Models assume 1 ms; proxies give 20-58 ms (one 221 ms outlier). | [TYT channel research](research.md#6-tyt-md-uv380-channel-rate-turnaround-and-baseband) |
| Custom receive needs a hardware mod | OpenRTX can transmit 4FSK on the MD-UV380, but receiving a raw stream needs a soldered wire. | [TYT channel research](research.md#6-tyt-md-uv380-channel-rate-turnaround-and-baseband) |
| Control cost | Per-PTT schedule negotiation costs 715/1608/2934 bytes for 1/2/3 relays before the first word. | same |
| Quality evidence is thin | One listener, two clips, no noise or digits. | [codec research](research.md#8-codecs-codec2-opus-lyra) |

Conclusion: the protocol core is sound and reusable; the physical premise
(live voice, up to three relays, one shared channel, stock TYT as the link) is
not viable.

## 5. How to continue

1. **G0, quality.** Blind A/B/C listening (stock DMR, Opus 8k, FM) with digits
   and noise decides whether an Opus-class codec is needed.
2. **G1, PHY truth.** Measure turnaround and payload on the TYT, or measure a
   faster link such as a CC1101 module at 433 MHz (PER at >= 100 kBd through a
   floor and two walls). [G1 measurement plan](next-steps.md#2-g1-measurement-plan).
3. **G2, airtime** and **G3, end to end** on three real nodes, only after G1.

Alternatives ranked by value per effort, including analog store-and-forward
and "one live relay, degrade beyond": [next steps](next-steps.md#3-alternatives-worth-testing).
