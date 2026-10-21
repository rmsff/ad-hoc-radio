# Documentation

Start with the [project README](../README.md).

| Document | What it covers |
| --- | --- |
| [results.md](results.md) | What was proven, what failed, where the work stopped |
| [experiments.md](experiments.md) | Every experiment: question, expectation, result, verdict |
| [concept.md](concept.md) | Idea, requirements, protocol architecture, simulator |
| [decisions.md](decisions.md) | Digest of the 22 architecture decisions and their status |
| [research.md](research.md) | Desk research digest with external sources |
| [hardware.md](hardware.md) | TYT MD-UV380, firmware bases, backup and rollback, faster radios |
| [next-steps.md](next-steps.md) | Measurement gates G0-G3, G1 plan, alternatives |

Reference material used by the code and tests:

| Path | Contents |
| --- | --- |
| [protocol/](protocol/) | Frame contract, compact control, OpenGD77 Layer-2 envelope, metrics schema |
| [hardware/](hardware/) | MD-UV380 runbooks (backup, firmware entry, stock restore), suitability checkpoint, platform shortlist, pre-station preparation |
| [firmware/](firmware/) | Firmware slice design and simulator-to-firmware contracts |
| `research/016`-`020` | Firmware base intake and backend selection notes, pinned by `configs/firmware-contract/mvp10-backend-selection.json` |

The reference folders are historical working records; where they disagree with
the documents above, `results.md` and `next-steps.md` are current. The digests
summarize a larger internal set of research notes, reviews and work records
that is not published; references to it are marked "not published". Hash-pinned
files (`research/016`-`020`, some `configs/` contracts) are kept byte for byte
and may still name such notes.

## Glossary

| Term | Meaning |
| --- | --- |
| ADHR | Ad Hoc Radio, the project's own voice relay protocol (not DMR) |
| MVP-x.y | Internal stage or work-item number; matches the `firmware/` directory names |
| G0-G3 | Measurement gates: quality, PHY truth, airtime, end to end ([next steps](next-steps.md)) |
| hop, relay | `n` relays mean `n + 1` hops (transmissions of each frame). Legacy file and experiment names such as `one-hop` and `two-hop` count relays, not hops |
| opportunity | A scheduled transmit slot for one station on the shared channel |
| PTT | Push to talk; each PTT press gets its own relay tree |
| HR-C6000 L1 / L2 | Physical-frame and DMR Layer-2 interfaces of the TYT baseband chip |
| PER | Packet error rate |
| kBd | Kilobaud, symbols per second x 1000 |
| codeplug | The radio's channel and settings memory image |
| DFU | USB firmware-update mode of the radio's bootloader |
| S&F | Store-and-forward: receive a whole block or phrase, then retransmit |
| FEC, PLC | Forward error correction; packet-loss concealment in the decoder |
| CCM | Core-coupled memory: the MCU's small static RAM next to main RAM |
