# V5 — Eval Framework (Rubrics, Scoring, Flywheel)

## 1. Purpose and requirement coverage

Deterministic eval core with a plugin judge (plan §5.4): JSON rubrics, pure
scoring, trajectory checks over V3's ledger, sandboxed `command` checks, rubric
skeleton compiled from approved requirements, and trend/regression reporting.
Covers plan tasks **P1.3** (P0) and the quality-flywheel scoring half of the
SDLC "specs as eval criteria" concept. Prototype lifecycle (`--prototype` /
`promote`) rides here as the requirements-phase companion.

## 2. Verified current state

- Sandboxed exec path to reuse: custom-gate runner
  (`internal/core/customgate.go`) — env scrub, timeout, bwrap when available.
  Single shared exec path is a stated risk mitigation (plan risk 3).
- Parser error style to copy: `internal/core/tasksparser.go` (actionable
  line-level errors).
- Requirements artifacts + EARS gate exist; `specd new` lifecycle in
  `internal/cmd/new.go`; approve ratchet in `approve.go`.
- V3 trajectory ledger is the input for `trajectory` checks.

## 3. Proposed design and end-to-end flow

- **Rubric** `.specd/evals/<suite>.json`: array of checks, kinds
  `artifact_present`, `file_pattern` (RE2 over changed files), `trajectory`
  (predicates over the ledger: max retries, forbidden tools,
  verify-before-complete ordering), `command` (sandboxed executable, exit code
  = pass/fail), each with points; suite `minScore`.
- `specd eval <spec> [--suite <name>] [--trajectory]`: runs checks, writes
  score to `state.json.evals` (V1) + full results
  `.specd/specs/<slug>/evals/<suite>-<seq>.json`; exit 1 below `minScore`.
- `specd eval init <spec>`: compiles approved `requirements.md` into a rubric
  skeleton — one `artifact_present`/`file_pattern` stub per EARS requirement ID
  (deterministic transform, zero interpretation); the agent refines via the new
  `specd-eval-author` skill (shipped same PR — mechanism 2).
- `specd eval trend`: score deltas + failure clustering by deterministic keys
  (gate name, task id, error class) over result-file history.
- **Prototype lifecycle:** `specd new --prototype` flags the spec; prototype
  specs skip design/tasks gates but **cannot** reach `complete`;
  `specd promote <spec>` converts to a full spec through the normal ratchet.
- **LM judge** = a `command` check pointing at a user script; disabled by
  default; exit code + stdout digest recorded as evidence.
- Optional `eval` gate: completion blocked without ≥1 recorded rubric run
  (config-on for new inits, off for migrated repos — gate-fatigue mitigation).

## 4. Interfaces, contracts, data, configuration, dependencies

- **New artifacts:** rubrics, result files (Appendix B shapes).
- **New commands:** `eval`, `eval init`, `eval trend`, `promote`,
  `new --prototype` (registry discipline, invariant 10).
- **Dependencies:** V1 (evals block), V3 (trajectory checks), V2 (guardrails
  screen rubric `command` lines). **Dependents:** V8 (gate suite ordering),
  V9 (deploy requires eval green), V11 (bundled suites), V12.

## 5. Invariants, security, errors, observability, compatibility, rollback

- Scoring pure: same inputs → same score (invariant 7); FakeClock for seq/time.
- Hostile rubrics: command-injection attempts in rubric fields
  rejected/escaped; `command` checks run only through the shared sandboxed
  exec path (env scrub, timeout, bwrap); adversarial tests land in this PR
  (P4.4 cadence).
- Determinism of the *record*, not the judge: plugin output stored as digest.
- Compat: no rubric → `eval` exits with a distinct "no suite" status; the
  eval gate is opt-in for migrated repos.
- **Rollback:** remove rubric files + disable gate flag.

## 6. Acceptance criteria and validation commands

- Rubric round-trip + fuzz; validation errors carry line context.
- Scoring purity test; exit 1 below `minScore`; result-file seq monotonic.
- Hostile-rubric adversarial suite; sandbox/env-scrub assertions on `command`.
- `eval init` transform: fixture requirements → expected stubs (no golden
  files; content assertions).
- Trend/clustering from fixtures deterministic.
- Prototype: cannot reach `complete`; `promote` e2e passes normal gates.
- `go test ./internal/core/... ./internal/cmd/... -run 'Eval|Promote' -race -count=2`

## 7. Open decisions and deviations

- Path deviation DV1. Skill `specd-eval-author` ships in the same PR as the
  engine (docs standard).
- Open: result-file retention (unbounded vs capped). Decision: unbounded,
  documented — they are evidence; users prune via VCS.
