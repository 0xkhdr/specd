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
- Config precedence is deterministic: defaults, global, project, environment/flags where supported.
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

