# Implement Wave Prompt

Use this prompt when dispatching a coding agent for one wave.

```text
You are implementing one specd gap-closure wave.

Repository: /var/www/html/rai/up/specd
Read first:
- AGENTS.md
- GAP-ANALYSIS.md
- specs/progress.md
- specs/<spec-slug>/spec.md
- specs/<spec-slug>/tasks.md

Assignment:
- Spec slug: <spec-slug>
- Wave: <wave-number>
- Tasks: <task-ids>

Rules:
- Prefix shell commands with `rtk` unless debugging raw output is required.
- Touch only files listed in each task `files` column. If more files are required, stop and record scope expansion before editing.
- Never touch `reference/`.
- Preserve specd invariants: deterministic gates, no LLM in gate/DAG/report paths, evidence requires passing verify pinned to real git HEAD, no bypass flag, atomic writes/CAS/locks, zero runtime dependencies.
- If CLI verbs or flags change, update `docs/command-reference.md` and `docs/CHEATSHEET.md` together.
- Use tests before claims. Mark no task done without passing its verify command.
- On test failure, identify root cause. Add or update a regression/invariant test before retry when failure exposes missing coverage.
- Avoid broad rewrites. Prefer canonical metadata, typed helpers, table-driven tests, and deterministic rendering.

Execution:
1. Read assigned spec and tasks.
2. Inspect current code paths named in `Required Knowledge`.
3. Implement only assigned wave tasks.
4. Run each task verify command.
5. Run any broader verify listed by spec when practical.
6. Update `specs/progress.md` with task status, verify command, result, and notes.
7. Report:
   - tasks completed
   - files changed
   - verify commands and results
   - blockers or scope deviations

Stop conditions:
- Required file is outside task scope and scope expansion is not approved.
- Verify fails twice for same root cause.
- Dirty worktree contains conflicting user edits.
- Task would weaken evidence, determinism, or safety invariants.
```

## Wave Selection Guidance

- Prefer one spec wave per agent invocation.
- Do not mix wave 1 and wave 2 for same spec in one dispatch unless wave 1 is already verified done.
- Run wave 3 as validation-only unless earlier wave tasks explicitly require small fixes.
- For high-risk specs, dispatch reviewer after implementation:
  - `evidence-security`
  - `orchestration-workers`
  - `concurrency-isolation`

