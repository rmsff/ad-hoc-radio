# Bench Log Templates

These files are raw capture templates for the TYT MD-UV380 / MD-UV3x0 Phase A
pre-station preparation package.

They are not direct `adhr-sim` inputs. After real bench data exists, set
`status` to `captured`, replace every `TBD`, keep
`hardware_claim_allowed=false` and `field_tx_ready=false`, then convert the
filled logs into the strict input shapes consumed by:

```bash
adhr-sim convert-bench-log configs/bench/log-templates/channel-filled.json --out configs/bench/channel-<date>.json
adhr-sim convert-bench-log configs/bench/log-templates/turnaround-filled.json --out configs/measurements/tyt-md-uv380-<date>.json
adhr-sim convert-bench-log configs/bench/log-templates/audio-filled.json --out configs/bench/audio-<date>.json
```

Then run:

```bash
adhr-sim calibrate-channel configs/bench/channel-<date>.json --report out/bench/channel-report.json
adhr-sim calibrate-tyt configs/measurements/tyt-md-uv380-<date>.json --report out/bench/turnaround-report.json
adhr-sim score-audio configs/bench/audio-<date>.json --report out/bench/audio-report.json
```

Until converted from real bench captures, these templates must not be used to
claim measured TYT hardware capability.

For pre-station process checks, use the separate `configs/bench/dry-run/`
fixtures with `status=dry_run_fixture`. That status is accepted by
`convert-bench-log`, but channel and turnaround outputs remain canonical
`fixture_only` inputs rather than `bench_measured` evidence.

## Conversion Map

| Raw template | Canonical input fields to produce |
| --- | --- |
| `channel-bench-log-template.json` | `measurement_source`, `test_fixture`, `measurement_artifact_ref`, `measurement_artifact_sha256`, `channel_width_hz`, `gross_bitrate_bps`, `frames_sent`, `frames_decoded`, `crc_error_count`, `duration_ms` |
| `turnaround-bench-log-template.json` | `measurement_source`, `test_fixture`, `measurement_artifact_ref`, `measurement_artifact_sha256` kept in notes, `firmware_path`, `channel_width_hz`, `gross_bitrate_bps`, `rx_to_tx_samples_ms`, `tx_to_rx_samples_ms`, `rx_valid_after_tx_samples_ms` |
| `audio-bench-log-template.json` | `fixture.fixture_id`, `fixture.fixture_path`, `fixture.artifact_sha256`, `fixture.evidence_grade`, `noise_profiles`, `loss_profiles`, `codec_class`, `audio_bandwidth_hz`, `scoring_method`, `good_voice_threshold` |

For real bench data, `status=captured` is the only raw-log status that maps
channel and turnaround outputs to canonical `measurement_source=bench_measured`
and `test_fixture=false`. Use it only after the raw artifact path and hash are
available.
