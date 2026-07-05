# 08-submit — Terminal `submit` verb, gate-enforced

Wave 2. FINDINGS refs: B.7, D-tier2 item 10.

## Problem

The pipeline currently ends at "complete": `report --pr` produces a PR
summary but nothing consumes it under gate enforcement — the pipeline ends
in a report nobody consumes instead of an enforced artifact (FINDINGS E,
loss 5). v1's `submit` validated all gates green, generated a
deterministic PR summary, and streamed it to an operator-configured
`config.submit.command` via sandboxed exec. It embeds no git/GitHub logic;
the operator supplies the command. Verdict: **port** — natural terminal
verb.

## Requirements (EARS)

- R1: WHEN a user runs `specd submit`, THE SYSTEM SHALL first run the full
  gate registry (plus opt-in gates the config enables) and refuse (exit 1,
  listing failing gates) unless every gate passes and every task is
  complete.
- R2: WHEN gates are green, THE SYSTEM SHALL generate the deterministic PR
  summary (same generator as `report --pr` — one implementation) and
  stream it on stdin to the command configured at `submit.command`,
  executed through the existing sandboxed exec path with a timeout.
- R3: IF `submit.command` is not configured, THEN `submit` SHALL print the
  summary to stdout and exit 0 (dry-run by default; explicitly documented).
- R4: WHEN the configured command exits non-zero, THE SYSTEM SHALL exit 1
  and record the failure; WHEN it succeeds, THE SYSTEM SHALL append a
  submission record {git HEAD, summary hash, command exit, timestamp} to
  the spec ledger.
- R5: WHEN `submit` runs on a spec already submitted at the same git HEAD,
  THE SYSTEM SHALL refuse (exit 1) unless `--resubmit` is given —
  idempotence guard against double-fires from orchestration.
- R6: THE verb SHALL declare phase compatibility (spec 03): valid only in
  the execution/complete phase.

## Design notes / best practice

- No git/GitHub logic in the binary — the operator command owns transport
  (`gh pr create --fill -F -`, a curl, a mail pipe). This keeps
  zero-dependency and Foundational Split invariants.
- Sandboxed exec: reuse `verify/exec.go`'s path (same env scrubbing,
  timeout, output capture) — do not grow a second exec implementation.
- Determinism: summary is a pure function of state.json + artifacts;
  summary hash in the ledger lets `report --history` (spec 13) show what
  was submitted.
- Config validation per spec 00: `submit.command` is a string argv-vector
  or shell line — pick one, document it, reject the other shape.
- Depends on spec 03 for phase metadata; can land before it with the check
  hand-rolled, but prefer ordering after.

## Out of scope

- Deploy/observe (recorded as deferred in spec 00 ADRs).
- Retry/backoff of the operator command.

## Acceptance

- Incomplete spec: `submit` exits 1 listing gates/tasks. Complete demo
  spec with `submit.command: cat`: summary streamed, ledger record with
  HEAD + hash; second run refuses without `--resubmit`; unset command
  prints to stdout exit 0. Full suite green.
