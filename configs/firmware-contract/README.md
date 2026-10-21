# Firmware contract files

Machine-readable contracts and build evidence between the simulator and the
firmware experiments on the TYT MD-UV380. Several are hash-pinned and checked by
Go tests (`internal/firmwareevidence`, `internal/radiobackend`,
`internal/backendmeasurement`); do not edit them by hand.

None of these files is a hardware, RF or transmit claim:

```text
hardware_claim_allowed = false
field_tx_ready = false
```

| File | Contents |
| --- | --- |
| `mvp9-radio-backend-handoff.json` | Simulator-to-radio-backend handoff: ownership boundary, operations, safety gates, OpenRTX and OpenGD77 candidate evaluation |
| `mvp10-backend-selection.json` | Selects OpenRTX for the first receive-only slice; pins the research notes `docs/research/016`-`020` by sha256 |
| `mvp10-backend-measurement-contract.json` | Synthetic measurement contract: endpoints, matrix, clock and failure rules; not evaluated on hardware |
| `md-uv380-rollback-inputs-20260712.json` | Hashed inventory of codeplug backups and stock D219.74 recovery inputs (files themselves are local only) |
| `openrtx-mduv3x0-resource-budget.json` | Baseline OpenRTX `mduv3x0` build and resource budget |
| `openrtx-mduv3x0-adhr-rxonly-mvp10.3.json` | Two clean offline OpenRTX receive-only builds: sources, dependencies, applied patch, receive-field mapping, TX guards, resources |
| `openrtx-mduv3x0-adhr-rxonly-build.json` | Receive-only OpenRTX patch build evidence |
| `openrtx-mduv3x0-adhr-rxonly-rollback.json` | Dry-run rollback package for the MVP-10.3 build |
| `openrtx-mduv3x0-adhr-rxonly-flash-20260708.json` | First receive-only OpenRTX write to an MD-UV380, normal-mode USB and first-boot evidence |
| `openrtx-mduv3x0-adhr-telemetry-build-20260708.json`, `-setfm-`, `-snap-` | Follow-up receive-only builds with USB CDC telemetry, forced analog receive and point-in-time snapshots |
| `prestation-sim-profile.json`, `prestation-bench-log.jsonl` | Pre-station simulator profile and bench-log fixture |
| `fixtures/` | Test fixtures for the contracts above |

Binary build outputs and radio backups referenced by hash are kept outside the
repository and are not distributed. Background: [docs/hardware.md](../../docs/hardware.md).
