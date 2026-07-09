# Project Diagnostics and Config Spec

## Purpose
Make project configuration loading, diagnostics, and drift reporting fail-safe and actionable.

## Source Gaps
- GAP-ANALYSIS.md domain 7: `project.yml` diagnostics and config digest gaps.
- Missing or stale project config may warn instead of fail where policy requires fail-closed.
- Config source precedence and digest visibility are unclear.

## Goals
- Centralize config loading with source precedence and diagnostics.
- Surface effective config digest in status/check reports.
- Fail closed for invalid policy-bearing config.
- Keep missing optional config non-fatal with clear diagnostic.

## Non-Goals
- Do not introduce a new config file format unless required.
- Do not make optional local developer overrides mandatory.
- Do not read secrets into reports.

## Required Knowledge
- Config loader: `internal/core/config_loader.go`.
- Commands: `internal/cmd/registry.go`.
- Gates: `internal/core/gates/`.
- Docs: `docs/command-reference.md`, `docs/agent-integration.md`.

## Functional Contract
- Config precedence is deterministic: defaults, project, environment/flags where supported.
- Diagnostics report source path, parse result, warnings, errors, and digest.
- Invalid config that affects safety, gates, security, orchestration, or evidence fails closed.
- Optional missing config reports warning only when behavior falls back to default.

## Acceptance Criteria
- Tests cover missing, malformed, stale, and valid config.
- `specd status` and `specd check` expose config diagnostics in text and JSON where supported.
- Docs define config precedence and fail-closed rules.
- Digest excludes volatile paths/secrets and remains stable across equivalent config.

## Invariants
- Diagnostics are deterministic.
- No secret values are printed.
- Safety policy parse errors cannot be ignored.

## Verification
- `go test ./internal/core ./internal/core/gates ./internal/cmd -run 'Test.*Config|Test.*Diagnostic|Test.*Status|Test.*Check' -count=1`
- `go test ./... -count=1`

## Decisions

- **`specd config` verb (GAP 7.2, was W6-T6).** GAP-ANALYSIS proposed a dedicated
  `specd config [--json]` verb. Not built. Effective-config diagnostics are exposed through
  `specd status` and `specd check` (and the `handshake bootstrap` config digest), which cover
  the operator need to see resolved configuration and catch bad keys. A standalone verb is
  deferred until a consumer needs machine-readable effective config on its own; adding it now
  would be a speculative surface. W6-T6 is superseded by this exposure.
- **Config location (GAP 7.3).** Settled: project configuration lives at top-level
  `project.yml` (the path every loader already reads via
  `core.LoadConfig(ConfigPaths{Project: <root>/project.yml})`), with `SPECD_*` environment
  overrides on top. (A machine-wide global layer was dropped — no CLI path populated it.)
  `.specd/config.yml` is **not**
  used — `.specd/` holds per-spec runtime state, not project config — so there is one
  unambiguous config path.

