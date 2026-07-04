# Tasks W0 — Restore Truth

> Blocks all other review-specs waves. Exempt from the dogfood rule (the loop cannot
> close before W1); evidence = the verify commands below passing literally.

## Wave 1 — audit & reset

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| P0.1a | auditor | `specs/progress.md` | — | `sh scripts/audit-progress.sh` (runs every progress.md verify literally, exits non-zero if any ✅ task's command fails) | audit log lists pass/fail per task; no judgment-based status |
| P0.1b | craftsman | `specs/progress.md`, `scripts/audit-progress.sh` | P0.1a | `sh scripts/audit-progress.sh` | every remaining ✅ has a passing command; false ✅ (≥ T1.1,T2.4,T2.6,T5.4,T8.6,T12.3) flipped ⬜; `files:` paths all exist |
| P0.2 | craftsman | `.specd/roles/`, `.specd/specs/demo/` (delete) | — | `test "$(ls .specd/roles | sort | tr '\n' ' ')" = "auditor.md craftsman.md scout.md validator.md " && test ! -e .specd/specs/demo` | exactly 4 roles, auditor present, scribe gone, demo junk deleted |

## Wave 2 — missing docs

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| P0.3a | craftsman | `docs/charter.md` | — | `grep -q 'harness component' docs/charter.md` | every registered verb mapped to one component + one principle; verb list matches registry |
| P0.3b | craftsman | `docs/context.md` | — | `test -s docs/context.md && grep -q 'read-full' docs/context.md` | four item modes, budget env, `ceil(len/4)` heuristic documented |
| P0.3c | craftsman | `docs/deferred-flywheel.md` | — | `test -s docs/deferred-flywheel.md && grep -q 'DeployApproval' docs/deferred-flywheel.md` | deferred evidence shapes + two re-entry seams documented |

## Traceability (task → requirement → finding)
- P0.1a/P0.1b → R0.1 → F1, F13 · P0.2 → R0.2 → F12 · P0.3a–c → R0.3 → F1
