# Contributing

The project is paused before the physical-layer measurement gate G1 (see
[README](README.md#next-steps)). The most useful contributions are
**measurements of a real radio link**, not more simulator features.

## Before you change anything

- Read [AGENTS.md](AGENTS.md): project rules, reading order and hard limits.
- Current state and gates: [docs/results.md](docs/results.md),
  [docs/next-steps.md](docs/next-steps.md).
- Until G1 passes, do not add new simulator code; new features or decisions
  should cite the measured or sourced parameter they rely on.

## Checks

All must pass before a pull request.

```bash
go build ./... && go vet ./...
gofmt -l cmd internal firmware        # must print nothing
go test -count=1 ./...
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s scripts/audio -p 'test_*.py'
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s scripts/review -p 'test_*.py'
```

Eight native audio tests (the libopus and listening-pack cases in
`scripts/audio/test_opus.py` and `test_opus_trace.py`) skip when the pinned
local libopus build or the local-only listening pack is absent; that is
expected in a fresh clone.

Go code rules: no external dependencies, no `any` or `interface{}` in
production code, results must not depend on `map` iteration order, time is in
integer milliseconds with overflow checks. Keep files under about 500 lines.

## Radio safety

- Transmission is off by default. Any RF test uses a dummy load, attenuators,
  shielding or receive-only mode, unless you hold the right to use that band,
  power and emission type in your country.
- Before flashing a radio, back up the codeplug and SPI flash and have a tested
  rollback; see the [MD-UV380 runbooks](docs/hardware/).
- Never commit vendor firmware images, vendor manuals, radio backups, serial
  numbers or credentials. Local artifacts belong in `.artifacts/` or `out/`
  (gitignored).

## Commits

Short, imperative, with a scope prefix, for example
`docs/g1: add CC1101 turnaround measurement`. Keep code and documentation in
separate commits where practical.
