# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Removed (breaking)

- **`boot` and `enrich` commands removed**, along with the two repo-global
  freshness gates (`specd check --boot` / `--enrich`), `boot.json`, and
  `enrich.json`. These performed repo *perception* and steering *authoring* inside
  the binary, violating the Foundational Split (the agent reasons; the harness
  enforces). `specd boot` and `specd enrich` are now unknown commands (exit 2).

### Added

- **`init` scaffolds a skill pack** under `.specd/skills/`: `specd-foundations`,
  `specd-steering`, `specd-requirements`, `specd-design`, `specd-tasks`, and
  `specd-execute`. The agent reads `specd-steering` to inspect the repo and author
  `product/structure/tech.md` + set `config.defaultVerify` itself — replacing
  `boot`/`enrich` with progressive-disclosure agent knowledge.

## [0.1.0] - 2026-06-14

First public release of `specd`, a spec-driven coding harness (stdlib-only Go, no
external dependencies).

### Added

- Core CLI with a unified command registry and consistent exit-code handling.
- `init` command: idempotent, marker-based `AGENTS.md` scaffolding and merge.
- `boot` command with boot-freshness validation gate.
- `enrich` command with `plan`, `apply`, and `status` sub-verbs.
- `dispatch` command plus verification records and an acceptance gate.
- `uninstall` command.
- Spec lifecycle: spec files, state with schema versioning, and CAS-guarded writes.
- Goroutine-safe spec locking with lock assertions and hardened slug validation.
- DAG engine for spec dependencies with cached regexes, preallocated slices, and benchmarks.
- Modular check gates with blocker utilities.
- Verified self-update flow with checksum (`SHA256SUMS`) verification.
- Security model documentation and hardening review.
- Install guide with fallback to build from source when no release binary is available.
- `goreleaser` release pipeline (linux/darwin/windows, amd64/arm64) with version
  injected via ldflags.
- Comprehensive test harness: deterministic `FakeClock`, spec builder, assertions,
  and end-to-end lifecycle tests.
- Hardened CI/testing pipeline.

### Notes

- Migrated from the original TypeScript implementation to Go.
- Renamed the `SPECd_JSON` environment variable to `SPECD_JSON`.

[0.1.0]: https://github.com/0xkhdr/specd/releases/tag/v0.1.0
