# V5 Tasks — Eval Framework

Plan coverage: P1.3 (+ prototype lifecycle from §4 requirements row).
Dependencies: V1, V2, V3. Dependents: V8, V9, V11, V12.

## Wave 1 — Rubric schema + deterministic checks

- [ ] `internal/core/eval.go`: rubric JSON parse/validate (line-context
  errors, unknown fields rejected, points, minScore).
- [ ] Check kinds `artifact_present`, `file_pattern` (RE2, changed-file scope
  shared with V2 helper).
- [ ] `trajectory` check kind: predicates over V3 ledger (max retries,
  forbidden tools, verify-before-complete ordering).
- **Validation:** `go test ./internal/core/... -run Eval -race -count=1`

## Wave 2 — Command checks + scoring + CLI (depends on Wave 1)

- [ ] `command` check kind through the shared custom-gate sandboxed exec path
  (env scrub, timeout, bwrap); stdout digest recorded.
- [ ] `specd eval <spec> [--suite] [--trajectory]` (`internal/cmd/eval.go`):
  score → `state.json.evals` + `.specd/specs/<slug>/evals/<suite>-<seq>.json`;
  exit 1 below minScore.
- [ ] Adversarial suite: hostile rubric fields (injection attempts, traversal
  in artifact paths, oversize) rejected; sandbox assertions.
- [ ] Rubric parser fuzz test.
- **Validation:** `go test ./internal/core/... ./internal/cmd/... -run 'Eval|Fuzz' -race`

## Wave 3 — Compile, trend, gate (depends on Wave 2)

- [ ] `specd eval init <spec>`: EARS requirement IDs → rubric stubs
  (deterministic transform); `specd-eval-author` skill shipped same PR.
- [ ] `specd eval trend`: score deltas + failure clustering by deterministic
  keys over result history; fixtures-based tests.
- [ ] `eval` gate: completion requires ≥1 recorded run
  (`config.eval.required`: on for new inits, off for migrated repos).
- **Validation:** `go test ./internal/core/... -run 'EvalInit|Trend|Gate' -count=2`

## Wave 4 — Prototype lifecycle (depends on Wave 3)

- [ ] `specd new --prototype`: flag in state; design/tasks gates skipped;
  `complete` transition hard-blocked.
- [ ] `specd promote <spec>`: converts to full spec; normal approve ratchet
  applies; e2e test.
- **Validation:** `go test ./internal/cmd/... -run 'Prototype|Promote' -race`

## Rollout & cleanup

- [ ] Docs: command-reference (eval/eval init/eval trend/promote),
  validation-gates (eval gate), mcp-guide if exposed, AGENTS.md template eval
  discipline, CHANGELOG; registry/help/docs parity green.
- **Rollback:** disable eval gate config; rubric files inert.
- **Completion evidence:** `make ci` green; purity, adversarial, and
  round-trip tests committed.
