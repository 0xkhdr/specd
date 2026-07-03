# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-07-03

First public release of `specd`, a spec-driven coding harness (stdlib-only Go, no
external dependencies). The binary enforces; the agent reasons — repo perception and
steering authoring live in the agent-facing skill pack, not in the CLI.

### Added

- Core CLI with a unified command registry and consistent exit-code handling.
- `init` command: idempotent, marker-based `AGENTS.md` scaffolding and merge, and
  scaffolds a skill pack under `.specd/skills/` (`specd-foundations`,
  `specd-steering`, `specd-requirements`, `specd-design`, `specd-tasks`,
  `specd-execute`). The agent reads `specd-steering` to inspect the repo and author
  `product/structure/tech.md` and set `config.defaultVerify` itself.
- Spec lifecycle: spec files, state with schema versioning, and CAS-guarded writes.
- Goroutine-safe spec locking with lock assertions and hardened slug validation.
- DAG engine for spec dependencies with cached regexes, preallocated slices, and benchmarks.
- Modular check gates with blocker utilities.
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
- **Supersedes the 2026-06-14 pre-release build carrying the same `v0.1.0` tag.**
  That early build shipped `boot`, `enrich`, `dispatch`, `uninstall`, and a
  self-`update` command; all were removed before this final cut. `boot`/`enrich`
  performed repo perception and steering authoring inside the binary, violating the
  Foundational Split — that work now lives in the skill pack above. `dispatch` moved
  to `specd next --dispatch`; the self-`update` and `uninstall` commands and
  `scripts/uninstall.sh` were dropped (reinstall via `scripts/install.sh --force`).

[Unreleased]: https://github.com/0xkhdr/specd/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/0xkhdr/specd/releases/tag/v0.1.0
