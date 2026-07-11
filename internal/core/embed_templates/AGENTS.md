# specd — host integration guide

**Agent = Model + Harness.** You (the model) supply reasoning. `specd` (the harness)
makes the plan safely delegable: it owns state, gates, and evidence — deterministically,
with no LLM in its decision path. Read this file before acting on a specd project.

## The loop
1. `specd status <slug> --guide` — the machine guidance for the current phase: the
   legal commands, the required artifact, the blockers, and the human-only actions.
   Only run the commands it lists as legal. It never lists task context or task verify
   when there is no executable task, and **`approve` is always human-only** — you never
   self-approve.
2. `specd context <slug> <task> --json` — get typed context V2, including required
   task knowledge, tool routes, authority limits, and config/palette drift digests
   (only once a task is executable — the guide will say so).
3. Do the task under its **role** (below). Touch only the task's declared `files:`.
4. `specd verify` — record evidence (exit code + git HEAD). This, not your say-so, is
   what marks a task complete.
5. `specd check` — run the readiness gates. A **human** runs `specd approve` to advance
   the phase, and only if the gates pass.

## Roles (read `.specd/roles/<role>.md` before acting as one)
- 🔍 **scout** — read-only explore & report. Never bound to a write task.
- 🛠️ **craftsman** — write + verify. Exactly one atomic task per invocation.
- 🧪 **validator** — read-only; runs the verify line and reports the record.
- 🛡️ **auditor** — read-only; audits a diff/scope against acceptance.

A task's `role:` determines what it may do. Read-only roles never write and never
fabricate a passing check.

## Guardrails (non-negotiable)
- **Evidence integrity.** No task completes without a passing verify record (exit code 0
  pinned to a real git HEAD). A read-only task carries a verify line it can pass
  (e.g. `printf ok`); there is no flag that bypasses the evidence gate.
- **Determinism.** Gates, DAG, and reports are pure functions of on-disk `.specd/` state.
- **Scope.** Touch only a task's declared files. Record deviations via `specd decision`.
- **Blocked means stop.** Retry once, then report `blocked` with the exact blocker.

## On-disk surface
- `.specd/specs/<slug>/{requirements.md,design.md,tasks.md,state.json,.lock}`
- `.specd/roles/*.md`, `.specd/steering/*.md` — the role and steering constitutions.

Steering files (`.specd/steering/`) carry the project's reasoning, workflow, product,
tech, and structure rules. Load a steering file when its phase needs it.
