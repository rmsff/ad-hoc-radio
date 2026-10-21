# Backend Measurement Contract

> Historical working record from the project's experiments, kept as reference.
> Current state: [docs/results.md](../results.md). Notes marked "not published"
> belong to the project's internal record.

`adhr-backend-measurement-v1` is the canonical evidence envelope for Epic 7
backend measurements. It is separate from simulator metrics and does not grant
permission to write firmware or transmit RF.

## Frozen Inputs

```text
handoff_contract_sha256 = d97a01b0160fe2c125574ffc1c1125a9da20c2d20e0508d35d565977d38ec39a
selection_report_sha256 = e83f4b00fa43e75b8483d6158fa30951c2ba956893fe9269e9f8cef3d81a630b
candidate_id = openrtx_mduv3x0
profile_id = source_led/project_native_variable
```

A report for another candidate or profile requires a new accepted selection;
it cannot silently reuse this fixture.

## Semantic Endpoints

| Measurement kind | Backend gate | Start | End |
| --- | --- | --- | --- |
| `channel_capacity` | `channel_capacity` | external test-frame start | valid receiver-frame completion |
| `rx_to_tx_turnaround` | `tx_rx_turnaround` | last valid external RX marker | first valid TX marker |
| `tx_to_rx_turnaround` | `tx_rx_turnaround` | last valid TX marker | first valid RX marker |
| `timing_acquisition` | `timing_acquisition` | source sync reference | backend epoch lock |

All duration values are integer microseconds measured in one named
backend-local monotonic clock. The clock is not UTC and values are not relative
offsets without a declared clock domain.

Channel observations include payload and overhead bytes, bit/frame errors and
lost frames. Timing-acquisition observations include boundary error,
drift in milli-ppm, guard use and resynchronization count.

Every report contains sorted matrix cases and every trial names one case.
Channel cases bind the tested profile, channel rate, payload/overhead, required
frame bytes and guard margin. A channel report cannot pass unless every case
has the minimum attempts, at least one successful sample, and tested capacity
at least `required_frame_bytes + guard_margin_bytes`.

Every microsecond measurement and time threshold must represent an exact whole
number of ticks at `clock.resolution_ns`; v1 does not silently round values.

## Sample And Percentile Rules

- Warm-up trials are explicitly marked and excluded from the measurement
  denominator and percentiles.
- Every non-warm-up trial is attempted evidence. Failed trials remain in
  `attempted_sample_count`, `failed_sample_count` and sorted failure reasons.
- p50, p95 and max use nearest rank over successful non-warm-up durations:
  `rank = ceil(percentile * n / 100)`, 1-based.
- A report must carry the exact recomputed summary; a mismatched declared
  summary is invalid.

## Non-Pass Rules

A report cannot pass with an unknown clock, missing thresholds, missing raw
artifact references, insufficient attempted samples, no successful samples,
non-bench provenance or an exceeded threshold. Missing prerequisites produce
`not_evaluated`; complete bench evidence that exceeds a threshold produces
`failed`.

Before a passing report is accepted as gate evidence, the caller must run
`ValidateGatePassEvidence(report, artifactRoot)`. It rejects a non-passing
report and verifies that every relative raw artifact exists and matches its
declared SHA-256. Structural JSON validation alone is not gate acceptance.

For `tx_rx_turnaround`, one directional report is never enough. The common gate
is accepted only through `ValidateTurnaroundGatePassEvidence(rxToTX, txToRX,
artifactRoot)`, which requires both directions to pass with identical
candidate, clock, units and threshold definitions.

The checked-in fixture intentionally has:

```text
evidence_grade = fixture
clock.status = unknown
verdict = not_evaluated
no_pass_reasons = [not_bench_measured, unknown_clock]
canonical_sha256 = 1f217ed60af3b8ac67588d8f0bb605c0fb66c955af3d86641214d6d08ff03227
```

Its raw NDJSON is synthetic. IT-086 recomputes its content SHA-256 and verifies
the frozen handoff and selection hashes.

## Safety Boundary

`hardware_claim_allowed`, `rf_synchronization_claim`, `field_tx_ready` and
`firmware_write_allowed` remain false. Only a structurally valid,
`bench_measured` report with complete evidence and satisfied thresholds may set
`gate_pass_claim=true`; this still does not authorize field or RF activity.

## Files

- Contract fixture:
  `configs/firmware-contract/mvp10-backend-measurement-contract.json`
- Raw synthetic fixture:
  `configs/firmware-contract/fixtures/mvp10.2-channel-fixture.ndjson`
- Go implementation: `internal/backendmeasurement/`
