# Changelog

All notable changes to `specd` are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html). See
[docs/versioning-policy.md](docs/versioning-policy.md) for how releases are cut.

## [Unreleased]

Production-readiness hardening initiative (no behaviour change to the harness contract:
determinism, evidence integrity, and zero runtime dependencies are preserved throughout).

### Added
- CI/CD pipeline: `gofmt`/`vet`/`go mod tidy` gates, `-race` + `-count=2` test legs across
  `{ubuntu, macos} × {go 1.26.x, stable}`, `golangci-lint` v2, pinned `govulncheck`, cross-process
  stress and crash-fault jobs, and an enforced coverage floor (`scripts/coverage-check.sh`).
- Reproducible release packaging via `.goreleaser.yml` (static build, checksums, SBOM).
- `SECURITY.md`, `TESTING.md`, `docs/observability.md`, `docs/scale-envelope.md`,
  `docs/versioning-policy.md`, and this changelog.
- Observability test coverage: Prometheus exposition validity, history ordering + JSON schema
  stability, HUD render, and an exit-code/error-message drift guard against `troubleshooting.md`.
- MCP tool-call marshaling contract tests and a documented-example runnability check.
- `docs-lint.sh` drift guard: the gate count and the Go-version floor are now lint-enforced from
  their single authoritative sources (`internal/core/gates/core.go`, `go.mod`).

### Changed
- Coverage floor ratcheted 74.0% → 75.0% (policy target; measured 75.7%).
- Go floor documented consistently as 1.26+ (matches the `go` directive in `go.mod`).
- Gate count normalised to 14 everywhere.

### Removed
- Orphan scripts `stress-brain.sh` and `verify-progress.sh` (redundant with wired CI jobs / the
  Go suite; see `scripts/README.md` for the per-script decision).

## [0.2.0]

- Iteration on the harness surface and orchestration.

## [0.1.0]

- Initial public release: the spec-driven pipeline (requirements → design → tasks →
  evidence-gated execution), the validation gates, and the base + orchestrated execution models.

[Unreleased]: https://github.com/0xkhdr/specd/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/0xkhdr/specd/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/0xkhdr/specd/releases/tag/v0.1.0
