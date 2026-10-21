# Research Digest

This page condenses the project's desk research into one readable document. The original
research notes were working artifacts written while the project was still deciding what to
build; they are not published. What follows keeps their questions, the concrete numbers they
found, the verdict each one reached, and the external sources behind it. Where a later review
corrected an earlier conclusion, the digest says so.

Every finding carries an evidence grade:

- **sourced** — taken from a datasheet, standard, paper or source code;
- **simulated** — produced by the deterministic simulator or an offline computation;
- **measured** — observed on real hardware or by a human listener;
- **assumption** — a working value that was never confirmed (most often the 19.2 kbit/s
  channel rate and the 1 ms RX-to-TX turnaround).

The most authoritative overall assessment is the full solution review of 2026-10-02,
summarized in [results.md](results.md#4-where-it-stopped). Its corrections (mainly that the
assumed 19.2 kbit/s channel rate and 1 ms turnaround do not hold) are noted in each section;
the sources behind the capacity numbers are in section 6.

---

## 1. Concept and prior art

**Question.** Can portable radios relay live group voice for each other without
infrastructure, so that station A reaches station B through a third station C when A and B
have no direct RF contact? Does anything like this already exist?

**Findings.**

- A commercial reference exists (sourced). The Lira DP-3800 family describes a portable DMR
  radio with a single-frequency repeater function: one frequency, receive and transmit
  separated by time slots, direct-channel fallback when the repeater leaves coverage. The
  DP-3800M "Ad Hoc" variant describes up to 32 stations where each may act as end subscriber,
  portable repeater or command node, with ordinary DMR radios as end subscribers. The vendor
  leaflet lists AMBE+2 as the family codec. The public pages disclose no election, routing,
  synchronization or frame details, so nothing was inferred about the internals.
- DMR Tier II (sourced) is two-slot TDMA in a 12.5 kHz channel with 4FSK; one 60 ms frame
  holds two 30 ms slots. It is a useful timing reference, but full air-interface
  compatibility (AMBE+2 ecosystem, color codes, signaling, interoperability testing) is far
  too heavy for a research MVP.
- M17 (sourced) is an open digital-voice protocol: 4FSK at 4800 symbol/s, 9600 bit/s, 9 kHz
  occupied bandwidth, 40 ms frames of 384 bits, Codec2 for speech. It is FDMA and solves none
  of the single-frequency relay timing problems, but it is a credible open reference for
  framing and a transmit path on supported radios.
- Analog FM carries no source id, call id, sequence number, hop count or duplicate
  identity, and a half-duplex handheld cannot receive and transmit at the same instant on one
  channel. A live single-frequency relay therefore needs digital frames plus time separation,
  store-and-forward, or two radios.
- LoRa meshes (Meshtastic) and Reticulum are text/telemetry systems with high latency
  tolerance, not live-voice transports; their lessons go to the control plane only
  (section 3). APRS/AX.25 digipeating, TETRA Direct Mode, P25 Phase II, NXDN, D-STAR and
  dPMR were mapped as comparison points; none is a compatibility target.

**Verdict.** Build a project-native protocol ("ADHR") with explicit frame identity,
duplicate suppression, hop limits and slot-based relay rules; use DMR and M17 as references,
not targets; prove a fixed one-hop single-frequency relay in a simulator first. This verdict
still stands: the 2026-10-02 review judged the protocol core (bounded relay-tree planning,
duplicate suppression, scheduling) sound; what failed was the physical premise (section 6).
See [protocol/frame-contract.md](protocol/frame-contract.md).

**Key sources.**

- Lira DP-3800: https://www.lira-radio.ru/catalog/portativnye_ratsii/8431/
- Lira DP-3800M (Ad Hoc): https://www.lira-radio.ru/catalog/portativnye_ratsii/8440/
- Lira DP-3800 series leaflet (AMBE+2): https://www.lira-radio.ru/upload/iblock/6e6/ut5dhwjk7h134yw3hhmeonxysnhjdnp3/Listovka-LiRA-DP_3800-series.pdf
- DMR Association, DMR standards: https://www.dmrassociation.org/dmr-standards.html
- ETSI TS 102 361-1 (DMR air interface): https://www.etsi.org/deliver/etsi_ts/102300_102399/10236101/02.06.01_60/ts_10236101v020601p.pdf
- M17 specification: https://spec.m17project.org/files/M17_spec.pdf
- Codec2: https://github.com/drowe67/codec2

---

## 2. Synchronization and multi-hop TDMA scheduling

**Question.** In an ad-hoc network with no fixed infrastructure, who defines slot timing, and
how far can a relay chain `A -> R1 -> R2 -> B` stretch on one shared half-duplex channel?

**Findings.**

- Three candidate timing models were recorded: source-led (the talker's first burst defines
  the slot epoch), relay- or group-leader-led (a selected station beacons timing), and
  distributed contention with guard times. DMR itself (sourced) specifies an elected
  direct-mode timing leader, propagation of timing by other mobile stations, and a
  single-frequency base station that repeats inbound voice in the alternate slot, either
  next-slot or delayed by three bursts (90 ms).
- On one frequency hops are serialized, not parallel (simulated). For `h` relays the
  airtime per delivered frame and the end-to-end latency both grow as O(h); a pipelined chain
  still delivers one frame per superframe, but the superframe is `(h + 1)` slots long.
- First-order airtime at the then-assumed rates (assumption): a 56-byte canonical frame takes
  47 ms at 9600 bit/s and 24 ms at 19 200 bit/s; a 22-byte compressed relay frame takes 19 ms
  and 10 ms. The 56-byte frame cannot fit a 40 ms slot at 9600 bit/s; two relays with a strict
  40 ms voice cadence do not fit on one frequency at any tested size.
- Header compression on relay hops (simulated): a relay that has seen the call start can
  drop call-scoped fields; a 14-byte compressed header turns a 56-byte frame into 22 bytes
  (about 2.5x less airtime at Codec2-class payloads).
- Ranked mitigations: coverage-aware election that minimizes the worst-case hop count
  (cheapest), header compression (implemented), DTX/VAD (about 2-2.5x average airtime
  saving, not worst-case), lower codec bitrate, wider channel, frame aggregation, on-demand
  slots, spatial slot reuse (research-grade) and two-frequency relay (breaks the
  single-frequency premise).
- Scheduling literature confirms that live voice is hard-deadline traffic: queue stability is
  not enough, the link schedule itself must keep every packet inside its deadline. A 2013
  IEEE paper on TDMA voice relay for small groups shows the design class is plausible.
- Final integrated profile (simulated, assumption-dependent): 80 ms frame period, 36-byte
  protected packet, FEC rate 2/3, 29 ms reserved per transmitter opportunity at 19.2 kbit/s,
  giving 29/58/87/116 ms per cycle for 0/1/2/3 relays and a design ceiling of
  `max_relay_depth = 3`. At 4800 bit/s one packet costs 100 ms and cannot sustain 80 ms.

**Verdict.** Accepted for the simulator: an ideal deterministic clock, explicit
transmitter-opportunity schedules, bounded relay trees with depth up to 3, and source-led
versus group-leader timing as comparable modes. No over-the-air synchronization algorithm was
ever accepted or measured. **Superseded in part:** every cycle budget above assumes
19.2 kbit/s and 1 ms turnaround; on the about 3.4-7.7 kbit/s net the review recomputed for the
TYT, even one live relay works only for codecs up to about 2.4-3 kbit/s and no accepted codec
passes three relays. A 30 ms slot grid fits at most one relay inside one 80 ms period (section 6).

**Key sources.**

- ETSI TS 102 361-2 (DMR voice and generic services): https://www.etsi.org/deliver/etsi_ts/102300_102399/10236102/02.05.01_60/ts_10236102v020501p.pdf
- Scheduling in multi-hop wireless networks with deadlines: https://arxiv.org/abs/2604.17493
- Joint routing, scheduling and power control with hard deadlines: https://arxiv.org/abs/1702.04749
- A TDMA protocol design to relay voice communications (IEEE PACRIM 2013): https://ieeexplore.ieee.org/document/6625482/

---

## 3. Relay election and control plane

**Question.** Which station relays a given call, how is it chosen without a coordinator, and
what can packet meshes teach about the control plane?

**Findings.**

- Core rule: one call has zero or one selected relay. Flooding is rejected for voice because
  every redundant rebroadcast burns airtime, collides and adds jitter. Relay is a capability
  plus runtime state of an ordinary portable node, never fixed infrastructure.
- Neighbor discovery by periodic beacons (simulated, assumption): with nine nodes, 32-byte
  beacons and 19.2 kbit/s, a 60 s interval costs about 0.21% of airtime, a 2 s interval about
  6.3%; voice always pre-empts beacons.
- A direct-coverage gate runs first: if the talker already covers the expected group, no
  relay is appointed. Coverage is directional and is reported as a prediction ("covered"),
  never as a delivery receipt.
- Candidate score (hypothesis, tuned only in simulation):
  `40 * quality_from_source + 35 * group_coverage + 15 * battery + 10 * stability - 30 * busy - 50 * low_battery`,
  where `group_coverage` penalizes deep reachability so a central relay that reaches the
  group in one or two hops beats a peripheral one. Election timing is delay-based: the best
  candidate answers first; a candidate that overhears a better offer cancels its own. Leases
  are temporary and re-election runs on degradation.
- Lessons kept from Meshtastic and Reticulum (sourced): duplicate caches are mandatory but
  need separate keys per plane; hop limits are plane-specific (voice TTL 1, control TTL 1-3);
  overhearing cancels pending rebroadcasts; forwarding is capability-based; announces must be
  rate-limited; no node needs the global topology. Nothing is copied into the voice path.
- The per-call single relay later generalized into a per-PTT bounded relay tree planned
  deterministically. The review found that schedule negotiation costs 715/1608/2934 bytes for
  1/2/3 relays before the first word (0.3/0.7/1.2 s of airtime at 19.2 kbit/s) and proposed
  overhear-based forwarding without a per-PTT handshake as an alternative worth testing.

**Verdict.** Voice plane and control plane are separate: directed voice through the assigned
relay with strict slots; bounded managed broadcast with duplicate suppression and an airtime
budget for control. Election is testable without hardware and remains valid; the control cost
per PTT is an open problem.

**Key sources.**

- Meshtastic mesh algorithm: https://meshtastic.org/docs/overview/mesh-algo/
- Meshtastic device roles: https://meshtastic.org/docs/configuration/radio/device/
- Meshtastic firmware mesh sources: https://github.com/meshtastic/firmware/tree/develop/src/mesh
- Reticulum manual: https://reticulum.network/manual/understanding.html

---

## 4. Regulatory constraints (high level, not legal advice)

**Question.** Is there a realistic license-free or simple-license band where live relayed
voice is both allowed and technically workable?

**Findings** (sourced; each is a CEPT or national position that must be checked per country).

- PMR446 / dPMR446 / DMR Tier I: 0.5 W, fixed antennas, no repeaters; the digital modes are
  existing standards, not free protocol space. Weak fit for a custom relay.
- 433 MHz SRD (CEPT ERC/REC 70-03, 433.05-434.79 MHz): 10 mW e.r.p. with a duty cycle of at
  most 10%, or 1 mW with no duty-cycle requirement; one entry in 434.04-434.79 MHz allows
  10 mW with no duty cycle at 25 kHz bandwidth. A Russian regulatory decision lists
  433.075-434.750 MHz at up to 10 mW. One relay roughly doubles channel occupancy and
  three relays quadruple it, so a duty-cycle-limited band is a poor fit for relayed voice.
- Amateur 70 cm / 2 m: plausible for licensed experiments, with identification rules and no
  consumer-product route.
- Business/private land-mobile license: the plausible practical path for a deployed system,
  outside research scope.
- A conducted, shielded or dummy-load bench setup is the right place for protocol and
  baseband work before any field assumption.

**Verdict.** Not proven that a license-free band supports this product. The project rule
became: no transmit-capable or field-oriented work without a regulatory profile and
lab-safety criteria; transmission off by default; frequency, power and bandwidth stay
configurable research parameters (12.5 and 25 kHz candidates). Regulatory analysis is outside
this research; measurements are limited to conducted or shielded setups or licensed operation.

**Key sources.**

- CEPT ERC/REC 70-03: https://docdb.cept.org/download/4635
- ETSI EN 300 113-1 (land mobile transmitter limits): https://www.etsi.org/deliver/etsi_en/300100_300199/30011301/01.06.02_60/en_30011301v010602p.pdf

---

## 5. DMR as the protocol substrate (re-evaluation, July 2026)

**Question.** Should the project drop its custom protocol and build on DMR, OpenGD77 or a
hybrid for multi-floor and basement search-and-rescue voice without pre-installed repeaters?

**Findings** (sourced).

- DMR supplies 4FSK, two 30 ms slots per 60 ms frame, an elected direct-mode timing leader
  and a standardized single-frequency base-station repeat path, but no automatic multi-hop
  voice relay between ordinary mobile stations.
- In the official OpenGD77 MD-UV380 source, `DMR_MODE_SFR` exists only as an enum placeholder
  with no implementation; the implemented modes are DMO/RMO-centric and DMR voice needs
  manufacturer donor sections for AMBE.
- The OpenGD77 hotspot path already receives link control plus 27 bytes of encoded audio per
  burst and emits MMDVM-style frames over USB, so a relay controller could forward encoded
  voice without transcoding; one MD-UV380 is still half-duplex.
- ETSI reserves manufacturer-private feature sets in CSBK control messages, so relay-capable
  nodes could carry a private control plane while stock radios stay leaf endpoints.
- Six integration patterns were compared. Those needing a separate relay appliance, two RF
  chains or a packet backhaul were rejected because the deployment invariant is "every
  deployed node is the same carried radio".

**Verdict.** Accepted on 2026-07-10: identical carried radios, any of which may
be endpoint, relay or timing leader; voice through a selected scheduled relay tree, not
flooding; `max_relay_depth = 3` as a ceiling with depth 0 or 1 as the normal case; DMR
timing, burst framing and the HR-C6000 baseband as candidate lower-layer building blocks
behind a narrow backend boundary, with OpenRTX/M17 as the open fallback. This is an
architecture direction, not measured evidence.

**Key sources.**

- ETSI TS 102 361-1 V2.6.1: https://www.etsi.org/deliver/etsi_ts/102300_102399/10236101/02.06.01_60/ts_10236101v020601p.pdf
- MMDVM firmware: https://github.com/g4klx/MMDVM
- M17 reference implementations: https://github.com/M17-Project/M17_Implementations

---

## 6. TYT MD-UV380: channel rate, turnaround and baseband

**Question.** Can the TYT MD-UV380 (STM32F405VG MCU, AT1846S transceiver, HR-C6000 DMR
baseband) carry 22-byte short voice frames at 19.2 kbit/s in a 25 kHz channel with the 1-2 ms
RX/TX turnaround the two-hop schedule needs?

**Findings.**

- Identity (sourced): dual-band DMR/FM handheld, FCC grants 136-174 MHz and 406.1-480 MHz
  (the radio label says 400-480 MHz),
  up to about 5 W measured on UHF, ETSI TS 102 361-1/-2/-3 compliance. The AT1846S datasheet
  and OpenRTX driver expose 12.5 and 25 kHz bandwidth tables; bandwidth is not symbol rate.
- Channel rate (sourced, corrects the assumption): the HR-C6000 manual states "4.8 kbps and
  9.6 kbps data transmission". A DMR time slot carries 27 bytes (216 payload bits) per 30 ms
  burst, one burst every 60 ms: 3.6 kbit/s raw per slot (7.2 only if one radio may use both
  slots, unproven). M17 packet mode on OpenRTX tops out around 4.5 kbit/s. The review's
  recomputation ([capacity_20261002.py](../scripts/review/capacity_20261002.py)) gives about
  3.38 kbit/s net for DMR Layer-2 (2.26 with FEC 2/3), 7.67 kbit/s without FEC or 5.11 with
  FEC for raw 4FSK via the MCU (M17-class), and about 1.35-2.0 kbit/s for an FM voice-band
  soft modem. On one shared channel the load grows about `(n + 1)`-fold with `n` relays, so
  Opus 8k over three relays needs about 50 kbit/s gross (75 with FEC 2/3) and Opus 6k about
  37.5 (56.3). **19.2 kbit/s has no path on this hardware.**
- Receive path (sourced): OpenRTX transmits M17/4FSK on the MD-UV380 without a hardware
  change, but receiving a raw 4FSK stream through the MCU needs a soldered RF-to-MCU wire. A
  stock radio cannot receive a custom stream.
- Turnaround, never measured. The AT1846S datasheet gives no PLL lock or switching times. The
  OpenRTX `radio_enableTx()` for this target sleeps 50 ms before switching the AT1846S to TX
  (a documented workaround for a clean carrier that truncates the M17 preamble). Public
  proxies (sourced): ETSI EN 300 113 attack/release limits 25/20 ms; a purpose-built
  half-duplex data module 15 ms typical, 20 ms max; P25 attack budgets up to 100 ms;
  same-class DMR/FM handhelds in lab reviews show RX-to-TX delay of 20-58 ms (one 221 ms
  outlier) and TX-to-RX recovery to audio of 116-420 ms; one forum report of about 500 ms
  PTT-to-key-up on an MD-UV380.
- Simulator thresholds the proxies had to meet (simulated): with 22 bytes at 19.2 kbit/s a
  16 ms slot tolerates 2 ms of switching, a 20 ms slot 6 ms, 24 ms 10 ms, 30 ms 16 ms. Every
  public proxy lands in the "no live two-hop tuple" band.
- HR-C6000 transport (sourced plus host fixture): the Layer-2 path exposes a 27-byte buffer
  per 30 ms burst (one burst per time slot every 60 ms), read in the slot interrupt before the
  codec call; a 22-byte ADHR short frame fits in it
  with a 5-byte envelope (discriminator, version, length, reserved, CRC-8). The Layer-1 path
  documents 36-byte demodulated 4FSK frames, but its handler in the pinned OpenGD77 source is
  empty and usable payload, coding and sync are unknown. On a 30 ms slot grid the serialized
  lower bound is 30/60/90/120 ms for 0/1/2/3 relays, so only direct and one-relay fit an
  80 ms period. The 36-byte integrated voice packet does not fit the 27-byte envelope.

**Verdict.** The TYT MD-UV380 stays a bench research candidate and a
microphone/speaker/user-interface platform. As the voice link for live multi-hop relay it
fails on documents alone: rate 2-5x below the assumed 19.2 kbit/s, the 25 kbit/s payload
threshold of gate G1 out of reach, turnaround unmeasured and probably tens of milliseconds.
**Supersedes** the earlier "19.2 kbit/s at 25 kHz remains the research target" stance.
Measuring TYT turnaround is now optional and only informs a "one relay, low bitrate" fallback.

**Key sources.**

- HR-C6000 translated manual: https://www.opengd77.com/downloads/datasheets/C6000/HR_C6000_translated.pdf
- AT1846S datasheet: https://www.opengd77.com/downloads/datasheets/AT1846S/AT1846S-Datasheet-0.3.pdf
- OpenRTX MD-UV3x0 radio driver (50 ms sleep): https://github.com/OpenRTX/OpenRTX/blob/master/platform/drivers/baseband/radio_UV3x0.cpp
- OpenRTX AT1846S UV3x0 bandwidth tables: https://github.com/OpenRTX/OpenRTX/blob/master/platform/drivers/baseband/AT1846S_UV3x0.cpp
- OpenRTX MD-UV380 M17 hardware mods: https://github.com/OpenRTX/openrtx.github.io/blob/master/M17/mduv380_mods.md
- OpenRTX FOSDEM 2025 slides (M17 packet rate): https://archive.fosdem.org/2025/events/attachments/fosdem-2025-5723-m17-and-openrtx-one-year-later/slides/237888/OpenRTXM_8BzFKaa.pdf
- TYT MD-UV380 product page: https://www.tyt888.com/pro_info147.html
- FCC record POD-MDUV380: https://fcc.report/FCC-ID/POD-MDUV380
- STM32F405VG: https://www.st.com/en/microcontrollers-microprocessors/stm32f405vg.html
- Handheld lab reviews used as proxies: https://www.va2pv.com/wp-content/uploads/2018/12/QST_AnyTone_AT-D868UV_review_VA2PV_November2018.pdf, https://www.arrl.org/files/file/ProductReviewsForDeb/2024/pr102023.pdf, https://www.radiomanual.info/schemi/Vari/Alinco_DJ-MD5TGP_review_QST_2019.pdf, https://www.radioamatore.info/attachments/article/709/QST-June-2011-TYT-Product-Review.pdf

---

## 7. Firmware bases: OpenRTX versus OpenGD77

**Question.** Which open firmware is the better base for an ADHR slice on the MD-UV380:
OpenRTX (cleaner research platform) or OpenGD77 (more complete handheld product)?

**Findings** (sourced, measured builds; build and flashing history in
[hardware.md](hardware.md#firmware-bases)).

- OpenRTX v0.4.4 (`mduv3x0`, GPL-3.0-or-later) builds reproducibly in an isolated container.
  The 222,312-byte image uses 25.6% of the 848 KiB (868,352-byte) application flash region,
  21% of the 64 KiB static RAM region and 31% of the 127.5 KiB main RAM region
  ([research/019](research/019-openrtx-isolated-build-and-resource-budget.md)).
- OpenGD77 (license forbids commercial use; DMR voice needs AMBE from donor firmware) builds
  byte-identically with an empty codec section. On the 10 W Plus target only about 1.9 KiB
  of main RAM and 4.6 KiB of CCM are statically free.
- Neither firmware was ever observed receiving an ADHR frame over RF; all receive evidence is
  host-fixture only.

**Verdict.** OpenRTX remains the completed reference and fallback. OpenGD77 is fixed as
`opengd77_prototype_only`: a non-commercial research vehicle with the 27-byte Layer-2 path as
the only justified receive-only measurement transport and **no accepted product transport**
(decision steps in [research/016](research/016-opengd77-feasibility-spike.md),
[research/017](research/017-opengd77-source-build-intake.md) and
[research/020](research/020-backend-candidate-selection.md)). Because stock reception of a
custom stream needs a solder mod, neither base turns the TYT into a fast link.

**Key sources.**

- OpenRTX: https://github.com/OpenRTX/OpenRTX (tag https://github.com/OpenRTX/OpenRTX/releases/tag/v0.4.4)
- OpenRTX compiling guide: https://github.com/OpenRTX/openrtx.github.io/blob/master/compiling.md
- OpenGD77 FAQ (license, sources): https://opengd77.org/viewtopic.php?t=7
- OpenGD77 user guide: https://github.com/LibreDMR/OpenGD77_UserGuide/blob/master/OpenGD77_User_Guide.md
- OpenGD77 MD-UV380 release and source index: https://www.opengd77.com/downloads/releases/MDUV380_DM1701/R20260131/sources_and_tools/

---

## 8. Codecs: Codec2, Opus, Lyra

**Question.** Which speech codec gives acceptable field intelligibility at a bitrate that
survives one to three relays, and does it fit the MD-UV380 MCU?

**Findings.**

- Codec2 (sourced, measured listening): open LGPL 2.1 codec at 700-3200 bit/s; the 3200 mode
  is the one actively maintained for M17. The simulator deferred any binding behind a codec
  interface so relays forward opaque bytes. At the assumed 19.2 kbit/s, Codec2 2400 fits one
  relay with 80 ms grouping and only 2 ms of cycle margin; 3200 needs 160 ms grouping. In
  listening tests the listener found both clean 2400 and 3200 samples decipherable only with
  concentrated effort and **rejected them for field use**. Evidence base: one listener, one
  speaker, two clips of literary speech with background hum, no noise, no digits.
- Opus (sourced, measured listening, simulated): libopus 1.5.2 at 6 and 8 kbit/s narrowband,
  20 ms packets, 6.5 ms lookahead. All delivered variants were intelligible, including a clip
  with 29.5% random packet loss under native concealment. Transport cost at the assumed
  19.2 kbit/s: the mean 8k stream is about 7.4 kbit/s of codec data, but the peak 20 ms
  packet (29 codec bytes, 49 wire bytes) needs 38 ms of reservation; no relayed shape fits at
  any cadence and only direct 160 ms grouping is admitted. For two relays even the ideal
  *average* rate (codec bodies and FEC only, no headers or guards) is 22.0-22.4 kbit/s for 6k
  and 31.2-31.6 kbit/s for 8k. The *peak* rate with native packing and the full header at
  160 ms grouping, before guards, is 26.7 / 41.4 / 56.3 kbit/s (6k) and 33.8 / 52.4 /
  71.3 kbit/s (8k) for one / two / three relays. The review's estimate for Opus 8k over three
  relays is about 50 kbit/s gross (75 with FEC 2/3).
- Google Lyra V2 at 3200 bit/s (measured build, sourced audit): built natively on an arm64
  Mac from the pinned v1.3.2 tree, 8 bytes per 20 ms, faster than real time on the host,
  Apache-2.0. Its three TFLite models total 3 583 374 bytes against 1 MiB of internal flash;
  encoder filters alone are 1.6 MB and the largest single filter (327 680 bytes) exceeds the
  MCU's combined 192 KiB SRAM. Persistent streaming state is 55,232 / 51,648 bytes
  (53.9 / 50.4 KiB) for encoder / decoder, against about 1.9 KiB statically free. External SPI flash (16 MiB, blocking HAL
  reads at about 4.5 Mbit/s) would need seconds per frame if filters were streamed. The
  quantizer uses an operator absent from the TFLite Micro default resolver, and no training
  pipeline is published.
- Google Lyra is unrelated to the Lira radios; the Lira family codec is AMBE+2.
- The offline pipeline (five profiles, 74 audio files, 1440 transport rows, reproduced twice)
  lives in [scripts/audio](../scripts/audio/).

**Verdict.** Codec2 rejected on quality; Opus 8k is the only intelligible candidate but does
not fit one relay at the assumed rate, let alone the real 3.4-7.7 kbit/s net; Lyra is a no-go on
the MD-UV380 unchanged. The review concluded that no accepted codec passes three relays on this
hardware and that the quality evidence is too thin; it proposed a blind A/B/C test (stock DMR
AMBE+2 versus Opus 8k versus FM) with digits and noise as gate G0.

**Key sources.**

- Codec2: https://github.com/drowe67/codec2
- Opus: https://opus-codec.org/ (examples: https://opus-codec.org/examples/)
- Lyra v1.3.2: https://github.com/google/lyra/tree/47698dadf0010abff6a848e02642f55f806d4842

---

## 9. G1 measurement plan with low-cost hardware

**Question.** With one TYT MD-UV380, a CC1101 board (ESP32-S3 plus CC1101 at 433 MHz), an
ESP32-C6 board and a laptop, and no oscilloscope, logic analyzer or SDR, which G1
measurements are possible and what must be bought?

**Findings.**

- The TYT fails the G1 payload threshold on paper (9.6 kbit/s gross, 3.6 kbit/s per slot),
  and unmodified OpenRTX waits 50 ms before TX; TYT RF measurement is optional.
- CC1101 passes timing by a wide margin (about 30 us direct RX/TX switch, up to 250-500 kBd);
  the open risks are loss through floors at 433 MHz, the missing sensitivity figure at
  100 kBd, and license-free power and duty-cycle limits for relayed voice.
- Opus figures for ESP32-S3 exist only for 48 kHz stereo; mono 8/16 kHz load must be
  measured. ESP-NOW is a stand-in link for protocol debugging only.
- Almost all of G1 can be done without emission; a second CC1101 node and 2 W attenuators
  are the only purchases on the critical path.

**Verdict.** Start G1 with CC1101, not the TYT. The step-by-step plan, thresholds and
shopping list are in [next steps](next-steps.md#2-g1-measurement-plan).

**Key sources.**

- TI CC1101 datasheet: https://www.ti.com/lit/ds/symlink/cc1101.pdf
- LILYGO T-Embed CC1101: https://github.com/Xinyuan-LilyGO/T-Embed-CC1101
- Espressif esp_audio_codec (Opus on ESP32-S3): https://components.espressif.com/components/espressif/esp_audio_codec
- ESP-NOW measurements: https://developer.espressif.com/blog/esp-now-for-outdoor-applications/

---

## What remains unknown

- **Any measured radio link.** No RX-to-TX turnaround, usable gross bitrate, preamble/sync
  overhead or receiver-valid-after-TX time has been measured on the MD-UV380 under any
  firmware; the 1 ms model value and the 19.2 kbit/s rate are both unconfirmed, and the review
  treats the rate as refuted.
- **Over-the-air synchronization.** Source-led versus group-leader timing, late entry, drift
  and guard sizing were compared only in simulation with an ideal clock.
- **Whether the HR-C6000 delivers arbitrary payloads on target**, how its Layer-2 FEC and
  30 ms timeline map to project framing, and what the Layer-1 path can carry.
- **Speech quality under field conditions.** One listener, one speaker, two clips, no noise,
  no digits or call signs; Codec2 and Lyra algorithmic delay uncalibrated; no measurement of
  Opus cycles, RAM or stack on any MCU.
- **CC1101 at 100 kBd**: sensitivity, PER through a floor and two walls, effective range at
  10 mW, and the real duty-cycle picture for relayed voice.
- **Opus on the ESP32-S3** in mono 8/16 kHz at 6-16 kbit/s.
- **Control-plane cost per PTT** (715-2934 bytes before the first word) and whether
  overhear-based forwarding without a handshake removes it.
- **Node definition**: stock handheld only, or handheld plus a fast-link module; this is the
  open decision that G1 is designed to settle.
- **Regulatory path** for any field use; nothing here is legal advice.
