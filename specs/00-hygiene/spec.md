# 00-hygiene — Fix documentation/reality drift and record every deliberate skip

Wave 0. FINDINGS refs: C.1, C.2, C.7, D-tier0 items 1 and 4.

## Problem

1. **CLAUDE.md documents infrastructure that does not exist** (C.1). It
   instructs contributors to run `./scripts/test-lint.sh`,
   `./scripts/docs-lint.sh`, `./scripts/stress*.sh`, and asserts
   `docs/CHEATSHEET.md` mirrors `docs/command-reference.md`. None of those
   files exist — `scripts/` contains only `audit-progress.sh`,
   `regress-all.sh`, `regress-domains.sh`, `regress-lint.sh`,
   `verify-progress.sh`; `docs/` has no `CHEATSHEET.md`. The project's own
   "Docs sync" invariant is violated by its onboarding file, and a
   contributor "running the gates before pushing" runs nothing — silently
   weakened CI claims.
2. **`triage` is a permanent deferral with no decision record** (C.2). The
   subtractive-bias rule is "cut or defer *and record the decision*"; nothing
   is recorded for `triage` nor for any other deliberate v1 skip (conductor,
   dashboard, packs, harness sharing, ingest, deploy/observe, eval —
   FINDINGS B.8–B.12, B.17, B.24, B.25).
3. **Config surface is ahead of enforcement** (C.7). The loader accepts
   orchestration and severity blocks, but several knobs have no consumer
   (e.g. orchestration model selection). Unconsumed config is silent
   misconfiguration waiting to happen.

## Decision

Make reality match the documentation rather than deleting the claims: the
claimed gates (test lint, docs lint, cheatsheet mirror) are genuinely useful
and cheap. Where a claim is not worth honoring, edit CLAUDE.md instead —
but the default is build, not delete, because CI already references these
gates conceptually.

## Requirements (EARS)

- R1: WHEN a contributor runs `./scripts/test-lint.sh`, THE SYSTEM SHALL
  perform the structural test-suite lint CLAUDE.md describes (no banned test
  suffixes, no space-separated subtest names, no duplicated helpers) and
  exit non-zero on any violation.
- R2: WHEN a contributor runs `./scripts/docs-lint.sh`, THE SYSTEM SHALL
  verify `docs/CHEATSHEET.md` mirrors the command surface of
  `docs/command-reference.md` and exit non-zero on divergence.
- R3: THE SYSTEM SHALL provide `docs/CHEATSHEET.md`, generated or checked
  against `docs/command-reference.md`, covering every live verb and flag.
- R4: WHEN CLAUDE.md or README name a script, file, or gate, THE SYSTEM
  SHALL contain that artifact (drift check is a lint, not a convention).
- R5: THE SYSTEM SHALL carry a decision record (`docs/decisions/` ADRs) for
  every deliberate skip listed in FINDINGS: triage, conductor, dashboard,
  packs, harness sharing, ingest, deploy, observe, eval/prototype — each
  with reasoning and an explicit revisit trigger.
- R6: WHEN the config loader encounters an unknown key, THE SYSTEM SHALL
  reject the config with a diagnostic naming the key (fail closed), and
  every accepted key SHALL have at least one consumer or a tracking decision.
- R7: IF `stress*.sh` scripts are not built, THEN CLAUDE.md SHALL not
  reference them (either build minimal contention harnesses or edit the doc;
  record whichever way as part of R5's decision set).

## Design notes / best practice

- Lint scripts: POSIX shell, `set -euo pipefail`, zero dependencies beyond
  coreutils + grep — same constraint philosophy as the binary itself. Each
  prints violations as `file:line: message` and exits 1.
- `docs-lint.sh` mirror check: extract the verb/flag inventory from both
  files (fenced command lines) and diff; do not require byte-identical prose.
  Whatever rule is chosen must be written at the top of the script — the
  rule is the contract.
- ADRs: one file per decision, `docs/decisions/NNNN-slug.md`, with Status /
  Context / Decision / Revisit-trigger sections. Sequence continues from any
  existing ADR numbering in the repo.
- Config validation: strict decode (`json.Decoder.DisallowUnknownFields` or
  equivalent for the current format). Unknown key ⇒ exit 2 with the key name
  — matches the "unknown verbs fail closed" precedent.
- CI: wire both lint scripts into the CI workflow so the drift class cannot
  recur.

## Out of scope

- Implementing `triage` (record decision only).
- Building the `stress*.sh` suite beyond what R7 decides.

## Acceptance

- Fresh clone: every command CLAUDE.md tells a contributor to run exists and
  exits 0 on a clean tree.
- `docs/decisions/` contains one ADR per skipped v1 capability listed in R5.
- Config file with an unknown key fails closed with a named-key diagnostic.
