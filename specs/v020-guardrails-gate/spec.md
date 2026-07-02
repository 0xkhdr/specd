# V2 — Executable Guardrails Gate

## 1. Purpose and requirement coverage

Ship "guardrails as code": `.specd/guardrails.json` enforced by a new built-in
`guardrails` gate that runs **first** in the `specd check` pipeline —
deterministic, RE2-only, before any agent work. Covers plan task **P1.4** (P1)
and the SDLC "configure the harness" phase.

## 2. Verified current state

- Gate pipeline: `internal/core/gates.go` (built-in gates), custom-gates engine
  `internal/core/customgate.go` + `docs/custom-gates.md` (sandboxed exec path:
  env scrub, timeout, bwrap).
- `specd check` command: `internal/cmd/check.go`.
- `specd init` scaffolding: `internal/cmd/init.go` (marker-merge idempotency
  pattern).
- `specd status` surface for tamper visibility: `internal/cmd/status.go`.

## 3. Proposed design and end-to-end flow

`.specd/guardrails.json` (stdlib JSON, validated with line-context errors):
`forbiddenImports`, `forbiddenPatterns` (RE2 via `regexp`), `forbiddenPaths`,
`forbiddenCommands` (matched against `verify:` lines and, later, eval rubric
`command` checks from V5).

New built-in gate `guardrails` in the check pipeline, ordered first. Default
scan scope: tracked/changed files via stdlib exec of
`git diff --name-only <base>`; `--all` flag for full scans. Pure function over
file contents — same inputs, same findings. `specd init --guardrails` scaffolds
a secure-default template (no `crypto/md5`, `crypto/des`, `math/rand` for
tokens; language-aware sets as template comments). Changes to
`guardrails.json` surfaced in `specd status` (tamper visibility, since the file
is agent-visible).

## 4. Interfaces, contracts, data, configuration, dependencies

- **New artifact:** `.specd/guardrails.json` (human-authored constitution).
- **Stable:** existing gate order and semantics unchanged when file absent
  (gate is a no-op pass); check exit-code contract unchanged.
- **Dependencies:** none (Wave 1). **Dependents:** V5 (rubric `command`
  screening), V11 (harness bundle carries guardrails).

## 5. Invariants, security, errors, observability, compatibility, rollback

- RE2-only (Go `regexp`) — no backtracking DoS by construction; reject
  patterns that fail to compile with file/line context.
- Guardrails file is hostile-adjacent input: size cap, UTF-8 validation,
  unknown-field rejection.
- Deterministic (invariant 7): findings sorted by path then rule.
- Absent file = pass (backward compat, invariant 9); migrated repos opt in.
- **Rollback:** delete `guardrails.json`; gate self-disables.

## 6. Acceptance criteria and validation commands

- Table-driven gate tests: each rule class pass/fail + hostile config
  (bad regex, oversize file, unknown fields).
- Determinism: same repo state → identical findings, `-count=2` safe.
- `specd init --guardrails` idempotent; template compiles all shipped patterns.
- `specd status` shows guardrails-file mtime/digest change.
- `go test ./internal/core/... -run Guardrail -race -count=2` green; benchmark
  on a large-repo fixture (existing bench pattern) with no pipeline regression.

## 7. Open decisions and deviations

- Gate lives in `internal/core/gates.go` pipeline (plan says
  `internal/spec/gates.go` — path deviation DV1).
- Open: base ref for `git diff` (default `HEAD`; configurable
  `guardrails.json.baseRef`). Fallback when not a git repo or exec fails:
  full-scan with a warning — fail toward more scanning, never less.
