# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed (breaking)

- **`fusion` command renamed to `handshake`.** The hidden host-integration
  surface is now `specd handshake bootstrap|policy` and the MCP tool is
  `specd_handshake`. The name better reflects its responsibility: the
  session-start negotiation between host agent and scaffold (load list, command
  schema digest, binding policy, config-digest drift). Subcommands, flags,
  payload shape, and exit codes are unchanged. The context-manifest item kind
  `fusion-policy` is renamed to `handshake-policy`.

### Removed (breaking)

- **`boot` and `enrich` commands removed**, along with the two repo-global
  freshness gates (`specd check --boot` / `--enrich`), `boot.json`, and
  `enrich.json`. These performed repo *perception* and steering *authoring* inside
  the binary, violating the Foundational Split (the agent reasons; the harness
  enforces). `specd boot` and `specd enrich` are now unknown commands (exit 2).

- **13 deprecated legacy command aliases removed.** Each is now an unknown
  command (exit 2); the surviving flag-based home is listed where one exists:
  - `doctor` — no replacement. `specd init --repair` covers scaffold/pack
    repair, but `doctor`'s diagnostics (sandbox/container availability, MCP and
    host-registration health checks) are **not** preserved. This is a real
    capability loss, not a rename — see `SECURITY.md` for the updated
    threat-model note.
  - `dispatch` → `specd next --dispatch`
  - `program` → `specd status --program`
  - `validate` → `specd check --schema-only`
  - `schema` → `specd check --schema`
  - `replay` → `specd report --history`
  - `diff` → `specd report --diff`
  - `serve` → `specd report --serve`
  - `watch` → `specd report --watch`
  - `mode` → `specd status <slug> --set-mode` / `--recommend`, `specd new --orchestrated`
  - `migrate` — removed along with `specd init --migrate` (see below)
  - `update` — removed (see below)
  - `uninstall` — removed (see below)

- **`specd migrate config` / `specd init --migrate` removed.** Legacy JSON
  config is still *read* automatically; it is just no longer convertible to the
  current format via a built-in command.

- **`scripts/uninstall.sh` removed.** See `README.md`'s Uninstall section for
  the manual removal steps (the installer only ever placed a plain binary in
  `~/.local/bin`, with no directory or symlink to clean up).

- **`specd update` self-update command removed.** Reinstall via
  `scripts/install.sh --force` or your package manager instead.

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
