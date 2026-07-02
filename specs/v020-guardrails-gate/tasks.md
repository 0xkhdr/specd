# V2 Tasks — Executable Guardrails Gate

Plan coverage: P1.4. Dependencies: none. Dependents: V5, V11.

## Wave 1 — Config parser + gate core

- [ ] `internal/core/guardrails.go`: JSON config load/validate (unknown fields
  rejected, RE2 compile check, size cap, actionable errors with context).
- [ ] Gate implementation: forbiddenImports / forbiddenPatterns /
  forbiddenPaths / forbiddenCommands evaluation as pure functions over file
  contents; findings sorted (path, rule).
- [ ] Changed-file scoping via stdlib exec `git diff --name-only`; `--all`
  override; non-git fallback = full scan + warning.
- **Validation:** `go test ./internal/core/... -run Guardrail -race -count=1`

## Wave 2 — Pipeline + scaffold + surfaces (depends on Wave 1)

- [ ] Register `guardrails` as first gate in `specd check` pipeline; absent
  config = pass (compat).
- [ ] `specd init --guardrails` secure-default template (marker-merge
  idempotent, like `agents.go` pattern).
- [ ] `specd status`: surface guardrails-file digest change (tamper
  visibility).
- **Validation:** `go test ./internal/cmd/... -run 'Check|Init|Status' -race`

## Wave 3 — Hostile input + performance (depends on Wave 2)

- [ ] Adversarial tests: malformed JSON, catastrophic-looking patterns,
  path traversal in forbiddenPaths, oversize config (per P4.4 cadence —
  adversarial tests land in the same PR).
- [ ] Fuzz the config parser (Go native fuzzing).
- [ ] Bench on large-repo fixture; record in `docs/agent-harness-baselines.md`.
- **Validation:** `go test ./internal/core/... -run 'Guardrail|Fuzz' -count=2`

## Rollout & cleanup

- [ ] Docs: `docs/validation-gates.md` (new gate, ordering, scoping),
  command-reference (`--guardrails`, `--all`), CHANGELOG; registry/help/docs
  parity tests pass.
- **Rollback:** remove config file; gate no-ops.
- **Completion evidence:** `make ci` green; determinism + hostile tests
  committed; zero findings on this repo with the shipped template.
