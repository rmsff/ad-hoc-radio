# Regression Packs

Regression packs run checked-in CLI scenarios and summarize whether each actual
outcome still matches its expected outcome.

Run the core pack:

```bash
go run ./cmd/adhr-sim regression-pack configs/regression/core-cli-regression.json --out out/regression
go run ./cmd/adhr-sim readiness-dashboard out/regression/summary.json --report out/readiness-dashboard.json
```

`summary.json` reports pack-level `outcome`, per-case `status`, artifact paths,
and deterministic hashes. A case can have `outcome=fail` and still
`status=pass` when the failure is the expected stress behavior.

The readiness dashboard summarizes the pack into `planning_ready`,
`station_required`, `firmware_entry_ready`, and blocker fields.

Most cases use a single `input` path relative to the pack file. The
`hardware-readiness` case uses `inputs` instead, with `channel`, `turnaround`,
and `audio` paths relative to the pack output directory. That lets one pack run
`calibrate-channel`, `calibrate-tyt`, and `score-audio` first, then feed their
generated reports into the readiness gate.
