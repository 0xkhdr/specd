---
name: specd-pinky
description: Execute a Pinky worker mission from a host coding agent. Load after receiving a mission or before running `specd pinky claim|heartbeat|progress|query|report|block|release|inbox`. Covers authority, context loading, verification, queries/directives, cancellation, and telemetry trust labels.
---

# specd pinky

Pinky is a host-executed worker contract. specd validates authority and state transitions; host performs assigned work.

## Claim and context

```
specd pinky claim --mission <mission.json>
specd context <spec>
```

Load the mission's `contextManifest` in order. Required items are the role contract, Pinky skill, one phase-scoped skill, `specd context <spec>`, and scoped files. Optional source artifacts stay collapsed unless needed.

The manifest is budgeted. `estimatedTokens` sums the required items; `budget` is the effective ceiling for this mission. Load every required item, then expand `reference-if-needed`/optional items only when the contract demands it, and stop before `budget`.

Each item carries a `mode`:

- `read-full` — load the whole artifact.
- `read-targeted` — the `path` resolves to a **slice**, not the whole file (the task's row in `tasks.md`, only the covered requirement lines, the named design section). Read just that slice; do not re-expand to the full file unless you genuinely need more.
- `run-command` — run the `command` (e.g. `specd context <spec>`) and read its output.
- `reference-if-needed` — keep collapsed; open only if the contract requires it.

`tokenHint` is the measured estimate for each item (the slice when targeted), so the running total is real, not a guess.

## Authority

- One mission per worker invocation.
- Builder may edit only declared files/scope.
- Investigator, reviewer, and verifier are read-only except ACP progress/blocker/report messages.
- Never flip `tasks.md` checkboxes. Never edit `state.json`. Never forge evidence refs.

## Progress, queries, blockers, cancellation

- Heartbeat before lease expiry.
- Report meaningful progress with `specd pinky progress`.
- For a bounded clarification that may let work continue, send `specd pinky query --text "..."`, then poll `specd pinky inbox` for a Brain directive. Obey `continue`, `retry`, `cancel`, `reassign`, or `escalate` exactly.
- If no bounded answer can unblock you, use `specd pinky block` with exact blocker and stop after one retry.
- On cancel directive, stop at next safe point and acknowledge; specd does not promise process termination.

## Verification and completion

- Run proof through `specd verify`; do not treat host stdout as trusted proof.
- Terminal report must include changed files, git head, verify record ref, stdout/stderr tails, and duration.
- Completion happens only after specd reconciles the report with existing verification and task integrity gates.

## Telemetry trust

Token counts, cost, duration, stdout, stderr, and changed files are `hostReported`. They help operations, never correctness.
