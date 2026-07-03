# Build Wave Prompt

Use this prompt when handing one implementation wave to a coding agent.

```markdown
You are rebuilding `specd` from scratch in `/var/www/html/rai/up/specd`.

Implement exactly this wave:

- Wave: `<WAVE_ID>`
- Tasks: `<PASTE_TASK_ROWS_FROM_specs/progress.md_OR_tasks.md>`

## Required Reading

Before editing code, read these files in order:

1. `AGENTS.md`
2. `fresh-start/00-decisions.md`
3. `fresh-start/00-roadmap.md`
4. `specs/progress.md`
5. For every task in this wave:
   - `specs/<domain>/spec.md`
   - `specs/<domain>/tasks.md`
   - the `fresh-start/<domain>.md` file named in that spec header
   - any `reference/...` files named in that spec header or verdict table

Treat `reference/` as read-only evidence. Do not import it, copy it wholesale, or build from it.

## Mission

Build only the tasks listed above. Follow each task's `files:` scope, `depends-on:`,
`verify:`, and `acceptance:` exactly. If a task needs a file not listed in `files:`, stop and
update the task/spec first rather than silently widening scope.

## SDLC Guardrails

- Preserve `Agent = Model + Harness`: code implements deterministic harness behavior, not model judgment.
- No LLM/network call may sit inside gates, DAG computation, reports, Brain `Decide`, or any other decision path.
- Keep zero runtime Go dependencies: Go stdlib only, static binary target.
- Preserve atomic writes: temp file + fsync + rename.
- Preserve reentrant per-spec advisory lock, stale reclaim, and CAS on `revision`.
- Preserve `ParseTasksMd` byte round-trip.
- Use embedded templates via `go:embed`; no runtime template dependency.
- Evidence integrity is mandatory: no task is complete without a passing verify record containing exit code and git HEAD.
- Keep subtractive bias: no old command/features return unless the current spec says KEEP/SIMPLIFY.
- Keep context lean: read targeted source, avoid broad reference-tree copying.

## Implementation Rules

- Use existing repo patterns once created; otherwise choose minimal clear Go packages named by the specs.
- Prefer small pure functions for gates, DAG, context manifests, and Brain decisions.
- Keep CLI thin: parse/dispatch in CLI layer, behavior in core packages.
- Keep on-disk contracts stable and documented in tests.
- Add tests matching each task acceptance line.
- Do not mark progress complete until the task verify command passes.

## Verification Flow

For each task:

1. Confirm dependencies are complete in `specs/progress.md`.
2. Implement only declared files.
3. Run that task's `verify:` command.
4. If verify fails, diagnose root cause, fix, and rerun.
5. When verify passes, write the required evidence record with exit code and git HEAD.
6. Update `specs/progress.md`: mark task done, update wave counts, update totals.

At wave end, run the narrowest aggregate check that covers changed packages. Report:

- tasks completed
- verify commands run
- evidence records written
- files changed
- any spec/task gaps discovered

If a requirement is unclear or conflicts with a higher-priority source, stop and report the
conflict with exact file references. Do not guess.
```
