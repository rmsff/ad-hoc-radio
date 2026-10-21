# TYT Bench Package Fixtures

These files are deterministic bench-package templates for local simulator checks.
They do not claim measured TYT MD-UV380 hardware behavior.

Run:

```bash
go run ./cmd/adhr-sim calibrate-channel configs/bench/channel-fixture.json --report out/bench/channel-report.json
go run ./cmd/adhr-sim calibrate-tyt configs/measurements/tyt-md-uv380-fixture.json --report out/bench/turnaround-report.json
go run ./cmd/adhr-sim score-audio configs/bench/audio-fixture.json --report out/bench/audio-report.json
go run ./cmd/adhr-sim hardware-readiness --channel out/bench/channel-report.json --turnaround out/bench/turnaround-report.json --audio out/bench/audio-report.json --report out/bench/hardware-readiness.json
```

Expected fixture-only result: the final hardware-readiness report is blocked
because channel and turnaround evidence are not bench-measured.

Phase A pre-station raw capture templates live in `log-templates/`. They are
not direct `adhr-sim` inputs and must be converted into the strict JSON shapes
above with `adhr-sim convert-bench-log <raw-log.json> --out <canonical.json>`
after real bench data exists.

Pre-station dry-run raw logs live in `dry-run/`. They use
`status=dry_run_fixture`, exercise the same conversion and readiness command
shape, and must still finish with blocked/no-claim hardware readiness.
