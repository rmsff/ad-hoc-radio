# Agent Instructions

Rules for AI coding agents and contributors working in this repository.

## Project purpose

Research into ad-hoc voice relay over handheld radios: voice frames, timing,
relay logic, packet identity, and eventually a controlled RF implementation. It
is not a LoRa telemetry project and not a generic analogue FM project.

## Read first

1. `README.md`
2. `docs/results.md` — current state; it follows the 2026-10-02 full review
3. `docs/next-steps.md` — measurement gates G0-G3
4. `docs/concept.md` — requirements and protocol architecture
5. `docs/decisions.md` — architecture decisions and their status
6. `docs/protocol/frame-contract.md`, `docs/protocol/metrics-schema.json`
7. `docs/research.md`, `docs/hardware.md`, `docs/experiments.md` as needed

When documents disagree, prefer `docs/results.md` and `docs/next-steps.md`, then
`docs/decisions.md`, then the protocol contracts in `docs/protocol/`.

## Hard constraints

- Do not treat this as a LoRa project.
- Do not claim DMR, Lira, or M17 compatibility unless explicitly implemented and tested.
- Do not introduce an AMBE/AMBE+2 dependency.
- Do not add encryption as a radio feature without a documented legal and regulatory decision.
- Do not provide instructions for unauthorized over-the-air transmission.
- Any RF work starts with receive-only, dummy load, attenuators, shielding, or other lab-safe conditions.
- Document frequency, power, bandwidth, duty cycle, emission class, antenna and certification assumptions before any transmit-capable work.
- No new simulator code until gate G1 (a measured radio link) passes; new decisions must cite a measured or sourced parameter.

## Code rules

- Go: no external dependencies; no `any` or `interface{}` in production code;
  results must not depend on `map` iteration order; time in integer
  milliseconds with overflow checks.
- Keep files under about 500 lines; keep it simple, no speculative features.
- Run the checks in `CONTRIBUTING.md` before proposing a change.

## Commit hygiene

- Keep documentation and code changes in separate commits where practical.
- Update `docs/protocol/frame-contract.md` when frame fields, flags, keys, call
  lifecycle or event ordering change; update `docs/protocol/metrics-schema.json`
  when metrics output changes.
- Record decisions that change protocol shape, RF assumptions, codec, timing,
  stack or hardware direction in `docs/decisions.md`.

## Security and secrets

- Never commit secrets, passwords, private keys, tokens, credentials, captures
  containing secrets, or `.env` files.
- Never commit vendor firmware, vendor manuals, radio backups, codeplugs or
  serial numbers.
- If a secret is discovered, stop and ask how to rotate or remove it safely.
