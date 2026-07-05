# 06-escalation-ratchet — Repeated verify failure forces a human checkpoint

Wave 1. FINDINGS refs: B.6, D-tier1 item 8.

## Problem

Nothing today stops an agent from burning attempts on a failing task
forever. v1's auto-escalation engine (`specd orchestrate status/resume
--override`) turned repeated failure into a mandatory human checkpoint via
deterministic rules over countable facts (`verify-fail`,
`retry-exhausted`, `blocker`, `cost-over-budget`) with human-only clearing.
FINDINGS verdict: **adapt** — the full engine is heavy, but the core
ratchet is small and high-value: after N failed verifies on a task, block
further attempts until a human records an override with a reason.

## Requirements (EARS)

- R1: WHEN a task accumulates N consecutive failed verify records (N from
  config `escalation.maxVerifyFails`, default 3) since its last pass or
  override, THE SYSTEM SHALL mark the task escalated in `state.json`.
- R2: WHILE a task is escalated, THE SYSTEM SHALL refuse `verify` and
  `task` completion attempts against it (exit 1, message naming the count
  and the override path), and the frontier SHALL exclude it so brain/next
  never dispatch it.
- R3: WHEN a human runs `specd task <id> --override --reason <text>`, THE
  SYSTEM SHALL clear the escalation, append an override record {task,
  reason, actor, timestamp, prior fail count} to the spec's ledger, and
  reset the consecutive-fail counter.
- R4: WHEN `--override` is given without a non-empty `--reason`, THE
  SYSTEM SHALL fail closed (exit 2).
- R5: THE escalation state SHALL be derived from countable facts already on
  disk (verify records + override records) — a pure function, recomputable,
  never a free-floating boolean that can drift.
- R6: WHEN `escalation.maxVerifyFails` is 0, THE SYSTEM SHALL disable the
  ratchet (documented escape hatch for CI-style loops), and `status` SHALL
  show escalated tasks prominently either way.

## Design notes / best practice

- Derivation (R5): count verify-fail records for the task newer than the
  latest pass/override record; escalated ⇔ count ≥ N. Store the *records*,
  derive the *state* — same doctrine as evidence ("countable facts only").
- Enforcement points: verify executor (`verify/exec.go` entry),
  `task_complete.go`, and frontier computation (`frontier.go`) — the third
  matters most, it is what keeps the brain from re-dispatching.
- The override is NOT an evidence bypass: an overridden task still needs a
  passing verify to complete. Override only re-opens the attempt budget.
  State this in code comment and docs — guards the no-bypass invariant.
- Ledger: reuse the existing append-only pattern (ACP ledger precedent);
  atomic write under the per-spec lock.
- `--override` on a non-escalated task: reject (exit 2) — overrides exist
  only as answers to escalations, keeping the ledger meaningful.
- Brain integration: `decide.go` treats escalated tasks as non-dispatchable
  facts; add a decision-path test proving a run halts (or moves on) rather
  than spinning on an escalated task.

## Out of scope

- Cost-based escalation triggers (needs spec 10 telemetry first; note
  forward hook in code comment).
- Notification/webhook on escalation.

## Acceptance

- Demo spec: 3 failed verifies ⇒ 4th attempt refused, task out of
  `next --waves` frontier, brain run does not dispatch it; `--override`
  without reason exits 2; with reason re-opens attempts and ledger holds
  the record; task still requires passing verify to complete. Full suite
  green, `-count=2` stable.
