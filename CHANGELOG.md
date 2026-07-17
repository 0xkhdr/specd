# Changelog

All notable changes to `specd` are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). See
[docs/versioning-policy.md](docs/versioning-policy.md) for how releases are cut.

## [Unreleased]

## [1.0.0] - 2026-07-17

First public release. Every on-disk contract ships at schema version 1 with no
earlier versions to migrate from: unknown or mismatched schema versions fail
closed everywhere.

- The spec-driven pipeline: requirements → design → tasks → evidence-gated
  execution, with deterministic validation gates and no LLM in any gate, DAG,
  or report path.
- Evidence integrity: a task completes only against a passing verify record
  (exit 0 pinned to a resolvable git HEAD); no bypass flag exists.
- Base and orchestrated execution models, including the opt-in deterministic
  Brain controller (leases, decisions, ACP ledger) and wave-based frontiers.
- Typed machine context manifest (`kind: context_manifest`,
  `schema_version: "1"`) alongside the default human-readable renderer;
  bounded, cited context with receipts.
- Canonical v1 telemetry envelope required on every persisted record; run
  ledger, provenance, program, state, rollback, and release-candidate schemas
  all pinned to v1 and fail-closed.
- MCP server exposing the command palette; roles (scout, craftsman, validator,
  auditor) and steering scaffolding via `specd init`.
- CI/CD pipeline: `gofmt`/`vet`/`go mod tidy` gates, `-race` + `-count=2` test
  legs across `{ubuntu, macos} × {go 1.26.x, stable}`, `golangci-lint`, pinned
  `govulncheck`, cross-process stress and crash-fault jobs, an enforced
  coverage floor, and reproducible release packaging via `.goreleaser.yml`
  (static build, checksums, SBOM).
- Zero runtime dependencies: standard library only, single static binary.

[Unreleased]: https://github.com/0xkhdr/specd/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/0xkhdr/specd/releases/tag/v1.0.0
