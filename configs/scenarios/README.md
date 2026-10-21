# Scenario configs

Simulator scenarios in the CLI's snake_case JSON format. All use simulated
timing; none is RF timing or DMR compatibility evidence. Background:
[docs/concept.md](../../docs/concept.md#5-simulator).

| Scenario | What it shows | Command |
| --- | --- | --- |
| `one-hop.json` | A reaches B through one relay when the direct link is disabled | `run` |
| `one-hop-direct-duplicate.json` | duplicate suppression when direct and relayed copies both arrive | `run` |
| `one-hop-corruption.json` | corrupted frames are rejected | `run` |
| `one-hop-group-mismatch.json` | frames for another group are ignored | `run` |
| `one-hop-seen-cache-overflow.json` | bounded duplicate cache under overflow | `run` |
| `relay-forward-queue-backpressure.json` | relay forward queue under backpressure | `run` |
| `two-hop-short-source.json` | `A -> R1 -> R2 -> B` with the short source format | `run` |
| `group-floor-ptt-contention.json` | busy channel and PTT collision policy (report only) | `ptt-contention` |
| `relay-tree/direct.json`, `one-relay.json`, `two-relay.json`, `three-relay-exception.json` | per-PTT relay tree at depth 0-3 | `run` |
| `relay-tree/branch-narrow.json`, `branch-wide.json` | branch width and transmitter opportunities | `run` |
| `relay-tree/partition-recovery.json` | replanning after a partition | `run` |

```bash
go run ./cmd/adhr-sim run configs/scenarios/one-hop.json --metrics /tmp/one-hop.json
go run ./cmd/adhr-sim ptt-contention configs/scenarios/group-floor-ptt-contention.json --report /tmp/ptt.json
go run ./cmd/adhr-sim peer-acceptance --report /tmp/peer.json
```

`peer-acceptance` generates the integrated fixed-seed depth and width matrix.

Relay-tree scenarios may set `voice_profile`, `voice_links`, `control` and
`expected_service_outcome`. Omitted control fields keep the simpler legacy
semantics. `voice_links` override impairments on selected edges; combining them
with replans is rejected. The optional `control` object accepts
`replan_lead_ms`, `links_observed_at_ms`, `links_freshness_ms` and targeted
`faults` (`kind`, `version`, `from_station_id`, `to_station_id`, `loss_pct`,
`delay_ms`). Each CLI run starts with a fresh set of peers.
