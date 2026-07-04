# Tasks W7 — Regression & Acceptance

> Standing regression over W0–W6. Dogfooded like W1–W6: real spec under `.specd/specs/`, verified via
> `specd verify`, closed via `specd task complete`. Runs after W6's fixes land; re-runnable on every push.

## Wave 1 — cross-wave regression harness

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| P7.1 | craftsman | `scripts/regress-all.sh` | — | `sh scripts/regress-all.sh` | script runs every `verify:` in `review-specs/*/tasks.md` literally via `sh -c`, logs exit per task, exits non-zero iff any fails; verdict from the log, never judgment |
| P7.2 | auditor | `scripts/regress-lint.sh` | — | `sh scripts/regress-lint.sh` | flags any verify that targets authoring `specs/` where runtime reads `.specd/specs/`, any absent-path pass (G4 hollow-verify), any `files:`/verify path failing `test -e` (G3 stale target); exits non-zero if any smell present |

## Wave 2 — per-domain best-practice regression

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| P7.3 | auditor | `scripts/regress-domains.sh` | P7.1 | `sh scripts/regress-domains.sh` | runs the §3 per-domain matrix (W0 honesty … W6 release); each wave's owned invariant re-asserted against the built binary / tree; exits non-zero on first violation |
| P7.4 | validator | `.specd/specs/` (review ledger) | P7.1, P7.2, P7.3 | `./specd status --json \| grep -vq '"open"'` | no review-specs wave marked complete while its README findings row is open; `report`/`status` completion equals the evidence ledger (F1/G2 generalized) |

## Wave 3 — record the method

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| P7.5 | craftsman | `PROJECT.md`, `review-specs/README.md` | P7.4 | `grep -qi 'regression' PROJECT.md && grep -q '07-regression-acceptance' review-specs/README.md` | the standing-regression method recorded as an ADR note in PROJECT.md §4; W7 registered in the README wave DAG + finding matrix so no surface is unowned |

## Traceability (task → requirement → source)
- P7.1 → R7.1 → REGRESSION_REVIEW.md §1 (run every literal verify, distrust progress.md)
- P7.2 → R7.2, R7.3 → G3 (stale target), G4 (hollow verify)
- P7.3 → R7.5 → per-wave domain contracts (W0–W6 findings matrix)
- P7.4 → R7.4 → F1/G2 (falsified tracker), ADR-8
- P7.5 → G3 (unrecorded consolidation/method → live ADR home in PROJECT.md §4)
