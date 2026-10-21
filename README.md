# Ad Hoc Radio

Research into **ad-hoc voice relay over handheld radios**: if station A cannot
hear station B, a third station that hears both receives A's digital voice and
retransmits it, so the group keeps talking without a fixed repeater. The idea is
close to single-frequency "relay-through-the-group" systems such as the Russian
*Lira* family, built from open firmware and commodity handhelds.

**Status: research paused before the physical-layer measurement gate (G1).**
The protocol core and a deterministic simulator work. The radio link that the
work assumed does not exist on the chosen hardware. Nothing here is a product,
and nothing here authorizes transmission.

- What was proven, what failed, and where it stopped: [docs/results.md](docs/results.md)
- Every experiment with hypothesis, expectation and result: [docs/experiments.md](docs/experiments.md)
- How to continue: [Next steps](#next-steps)

## Concept

```text
   A  ---- x ----  B          A cannot reach B directly
    \             /
     \--> R1 ----/            R1 hears both and relays A's voice frames
```

- **Universal peers.** Every radio can talk, listen and relay; there is no fixed
  repeater role.
- **Per-PTT bounded relay tree.** When someone presses PTT, a deterministic
  planner picks a tree rooted at the speaker that reaches the expected group
  with at most three relay retransmissions, ranked by coverage, depth, number
  of transmissions, link risk and battery cost
  ([ADR-0015](docs/decisions.md#adr-0015-bounded-per-ptt-scheduled-relay-tree)).
- **Scheduled relay timing.** Relays retransmit in planned opportunities on one
  shared half-duplex channel; duplicate suppression and a jitter buffer keep
  playback ordered ([frame contract](docs/protocol/frame-contract.md),
  [concept](docs/concept.md#3-how-it-works)).
- **Separate voice and control planes**; the codec payload is opaque to the
  protocol ([ADR-0006](docs/decisions.md#adr-0006-separate-voice-plane-and-control-plane)).
- **Analog FM stays usable.** Firmware experiments keep a conventional FM mode
  next to the experimental ADHR mode.
- **Not DMR.** ADHR is a project-native protocol and claims no DMR
  compatibility ([ADR-0001](docs/decisions.md#adr-0001-custom-protocol-instead-of-dmr)).

The target user requirement: speech quality first; 1.5-2 s from end of speech
to end of playback through one relay is acceptable (the proposed G3 gate allows
2.5 s at three relays).

## Main results

| Area | Result |
| --- | --- |
| Protocol and simulator | Relay election, per-PTT tree planning, scheduling, duplicate suppression and metrics run deterministically (same seed, same bytes) in a Go simulator with a regression pack. |
| Capacity | The stock TYT MD-UV380 paths offer about **3.4-7.7 kbit/s** net (3.6 kbit/s raw DMR slot payload). Opus 8k over three relays on one shared channel needs about **50 kbit/s** gross (75 with FEC 2/3). Earlier go/no-go results were computed at an *assumed* 19.2 kbit/s that this hardware does not provide. |
| Codecs | Codec2 2400/3200 was rejected by listening for field use (one listener, two clips); Opus 8k was intelligible (one listener, two clips) but does not fit the radio channel; Google Lyra does not fit the radio MCU (models exceed internal flash). |
| Firmware | Reproducible receive-only builds of OpenRTX and OpenGD77 with ADHR hooks for the MD-UV380, flashed and rolled back to stock firmware (settings verified byte for byte on the 10 W unit). The HR-C6000 baseband offers no accepted ADHR transport. |
| Timing | RX-to-TX turnaround was never measured; the models assumed 1 ms, proxies suggest 20-58 ms. |

Details and evidence links: [docs/results.md](docs/results.md).

## Where it stopped

On one shared channel every relay repeats the airtime, so load grows about
`(n + 1)`-fold with `n` relays. With a few kbit/s of real payload, one live
relay works only for codecs up to about 2.4-3 kbit/s, and no codec that passed
listening fits three relays. Receiving a custom 4FSK stream on the MD-UV380 also
needs a hardware modification. Details: [results](docs/results.md#4-where-it-stopped).

## Next steps

A full review on 2026-10-02 replaced the plan with measurement gates
([next steps](docs/next-steps.md)). No new simulator code until G1.

| Gate | Question | Pass criterion (proposed) |
| --- | --- | --- |
| G0 quality | Is an Opus-class codec needed at all? Blind A/B/C: stock DMR vs Opus 8k vs FM, digits and phrases, with noise | word and digit accuracy >= 95% in silence, >= 90% in noise |
| G1 PHY truth | What does a real link deliver? | TYT: turnaround p95 <= 5 ms and >= 25 kbit/s payload; or a CC1101-class link: PER <= 5% at >= 100 kBd through one floor and two walls |
| G2 airtime | Does the measured rate carry the relay tree? | channel load <= 0.8 at one relay, <= 1.0 at three; PTT setup <= 0.3 s |
| G3 end to end | Three real nodes | p95 end of speech to end of playback <= 2.0 s at one relay, <= 2.5 s at three |

A measurement plan for G1 with low-cost hardware (TYT, a CC1101 module,
dummy loads, attenuators):
[G1 measurement plan](docs/next-steps.md#2-g1-measurement-plan).
Candidate faster radios: CC1101 (433 MHz, up to 250-500 kBd) and SX1262 boards.

## Repository map

| Path | Contents |
| --- | --- |
| `cmd/adhr-sim/`, `internal/` | Go simulator and protocol packages (no external dependencies) |
| `configs/` | scenarios, regression pack, firmware contracts, audio and bench fixtures |
| `scripts/audio/` | offline codec comparison (Codec2, Opus, Lyra are downloaded, not vendored); listening-page labels are in Russian, matching the Russian test speech |
| `scripts/review/` | capacity recomputation and review tools |
| `firmware/` | ADHR overlays and build scripts for OpenRTX and OpenGD77 on the MD-UV380 |
| `tools/` | isolated OpenRTX build container, XMODEM receiver |
| `docs/` | results, experiment log, concept, decision and research digests, hardware notes, protocol contracts, MD-UV380 runbooks ([index](docs/README.md)) |

## Quick start

Requires Go 1.24 or newer.

```bash
go build ./... && go test -count=1 ./...
go run ./cmd/adhr-sim run configs/scenarios/one-hop.json --metrics /tmp/one-hop.json
rm -rf /tmp/adhr-regression   # regression-pack needs an empty output directory
go run ./cmd/adhr-sim regression-pack configs/regression/core-cli-regression.json --out /tmp/adhr-regression
go run ./cmd/adhr-sim --help
```

`one-hop.json` disables the direct A-to-B link and shows 100 source frames
delivered through the relay. See [CONTRIBUTING.md](CONTRIBUTING.md) for all checks.

## Safety and regulation

The repository contains no transmit-capable ADHR firmware; the firmware
experiments are receive-only. Any RF work must start with dummy loads,
attenuators, shielding or receive-only tests. Transmitting needs the right to
use the band, power and emission type in your country
([regulatory notes](docs/research.md#4-regulatory-constraints-high-level-not-legal-advice)).
Flashing a radio can brick it; back up the codeplug and SPI flash first and
have a tested rollback (see the [MD-UV380 runbooks](docs/hardware/)).

## License

- Own code: [Apache-2.0](LICENSE).
- Documentation: [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/),
  except quoted third-party material.
- `configs/audio/opus-transport-trace-20260906.json` contains Opus packets
  derived from two Wikimedia Commons recordings and is CC BY-SA 4.0 (see
  [NOTICE](NOTICE)).
- `firmware/openrtx-*`: GPL-3.0-or-later, because it patches and links into
  [OpenRTX](https://github.com/OpenRTX/OpenRTX).
- `firmware/opengd77-*` contains only this project's own `adhr_*` sources,
  host tests and a host-side USB bench tool. They build only on top of
  [OpenGD77](https://www.opengd77.com/), whose license forbids commercial use;
  a firmware image built that way falls under the OpenGD77 license.

See [NOTICE](NOTICE) for third-party components and data. The project was
developed with AI coding agents using the BMAD method.
