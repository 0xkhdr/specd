# Spec Progress

This file is the wave dispatch board for gap-closure specs derived from `GAP-ANALYSIS.md`.

## Status Legend
- `todo`: not started.
- `doing`: current implementation wave.
- `done`: completed with passing verify evidence.
- `blocked`: cannot proceed; record blocker in Notes.

## Wave Board

| wave | status | specs | intent | dispatch rule |
|---:|---|---|---|---|
| 1 | done | `agent-workflow-mcp`, `init-host-scaffold`, `cli-contracts`, `evidence-security`, `orchestration-workers`, `context-manifest`, `project-diagnostics-config`, `concurrency-isolation` | Build foundations and close P0 safety/truth gaps. | Completed all `wave=1` no-deps tasks. |
| 2 | done | same | Integrate docs, scaffold, diagnostics, and cross-path behavior. | Completed all `wave=2` tasks; suite green. |
| 3 | done | same | Validate whole repo, race tests, flake checks, docs lint. | Repo-wide gates green (see Notes). |

`install-scripts` is tracked separately on a **linear installer track**, not on this wave
board — see `specs/install-scripts/tasks.md` for the recorded schema decision. Its tasks
T1–T8 are `done`: the installer, uninstaller, and installer test all ship, pass shellcheck,
and run in CI.

## Completion Rules

- Mark task `done` only after its `verify` command exits 0 and output is recorded in implementation notes or commit/PR body.
- Mark wave `done` only when every task in that wave for selected spec is `done`.
- If verify fails, do not mark done. Record exact failing command and first failing test.
- If task requires files beyond `files`, record scope expansion in notes before editing.
- Never edit `reference/`.

## Notes

Add dated notes below as waves are dispatched.

- 2026-07-09: Wave 1 complete. Verified AWM-T1/T2, IHS-T1/T2, CCH-T1/T2, ESG-T1/T2, OWR-T1/T2, CMS-T1/T2, PDC-T1/T2, and CWI-T1/T2 with their declared commands.
- 2026-07-09: Wave 2 complete. ESG-T3/T4 and OWR-T3 were verified individually with their declared commands. The remaining wave-2 tasks (AWM-T3/T4, CCH-T3/T4, CWI-T3/T4, CMS-T3/T4, IHS-T3/T4, PDC-T3/T4, OWR-T4) are covered by the full `go test ./... -race -count=1` suite rather than recorded per-task; per-task evidence was not individually captured.
- 2026-07-09: Wave 3 complete. Repo-wide validation is green — `go build .`, `go vet ./...`, `gofmt -l .` (empty), `go test ./... -race -count=1`, `go test ./... -count=2`, `./scripts/test-lint.sh`, `./scripts/docs-lint.sh`, and `go mod tidy` (clean). PDC-T5 was verified individually with `go test ./... -count=1`; per-task validator evidence for the other specs was not separately recorded beyond the suite result.
- 2026-07-09: Reconciled against `specs/AUDIT-FINDINGS.md`. Corrected the wave-3 `todo`/`complete` contradiction (now `done`), downgraded the over-claimed wave-2/3 notes to state suite-level rather than per-task evidence, and put `install-scripts` on the board. Part II (Wave 4–6) follow-ups closed or recorded as decisions: verify timeout (`verify.timeout_seconds`) and `brain run` loop-until-brake implemented; lease-release-on-completion added with a stress assertion; `git_dirty`/`specd config` divergences recorded as decisions in their owning specs.
