# Bench Session Dry-Run Fixtures

These files exercise the station-required bench session path before a TYT
MD-UV380 / MD-UV3x0-class station is physically available.

They intentionally use:

```text
status = dry_run_fixture
hardware_claim_allowed = false
field_tx_ready = false
```

`dry_run_fixture` is accepted by `adhr-sim convert-bench-log`, but it converts
channel and turnaround logs into canonical `fixture_only` inputs. The final
hardware-readiness result must remain blocked.

Run:

```bash
rm -rf out/bench-dry-run
mkdir -p out/bench-dry-run/inputs out/bench-dry-run/reports

go run ./cmd/adhr-sim convert-bench-log \
  configs/bench/dry-run/channel-dry-run-log.json \
  --out out/bench-dry-run/inputs/channel.json

go run ./cmd/adhr-sim convert-bench-log \
  configs/bench/dry-run/turnaround-dry-run-log.json \
  --out out/bench-dry-run/inputs/turnaround.json

go run ./cmd/adhr-sim convert-bench-log \
  configs/bench/dry-run/audio-dry-run-log.json \
  --out out/bench-dry-run/inputs/audio.json

go run ./cmd/adhr-sim calibrate-channel \
  out/bench-dry-run/inputs/channel.json \
  --report out/bench-dry-run/reports/channel-report.json

go run ./cmd/adhr-sim calibrate-tyt \
  out/bench-dry-run/inputs/turnaround.json \
  --report out/bench-dry-run/reports/turnaround-report.json

go run ./cmd/adhr-sim score-audio \
  out/bench-dry-run/inputs/audio.json \
  --report out/bench-dry-run/reports/audio-report.json

go run ./cmd/adhr-sim hardware-readiness \
  --channel out/bench-dry-run/reports/channel-report.json \
  --turnaround out/bench-dry-run/reports/turnaround-report.json \
  --audio out/bench-dry-run/reports/audio-report.json \
  --report out/bench-dry-run/reports/hardware-readiness.json
```

Expected final result:

```text
outcome = hardware_readiness_blocked
bench_experiment_ready = false
hardware_claim_allowed = false
field_tx_ready = false
```
