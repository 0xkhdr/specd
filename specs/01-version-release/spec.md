# 01-version-release — `specd version` verb and release pipeline

Wave 0. FINDINGS refs: C.3, B.13, D-tier0 item 2.

## Problem

The binary cannot identify itself in the field: no `version` verb, no
ldflags version injection, no release pipeline. Any bug report against a
deployed binary is unattributable. v1 had all three (verb with `--json`,
ldflags-injected values, `.goreleaser.yml` under `reference/`) — this is a
pure regression with trivial cost, flagged in FINDINGS as "port
immediately".

## Requirements (EARS)

- R1: WHEN a user runs `specd version`, THE SYSTEM SHALL print version,
  git commit, and build date on one human-readable line.
- R2: WHEN a user runs `specd version --json`, THE SYSTEM SHALL emit a
  stable JSON object `{"version":..., "commit":..., "date":...}`.
- R3: WHEN the binary is built without ldflags (plain `go build`), THE
  SYSTEM SHALL report `dev` for version and best-effort values from
  `debug.ReadBuildInfo()` for commit/date rather than empty strings.
- R4: THE SYSTEM SHALL provide a release pipeline (goreleaser config at
  repo root) that injects version/commit/date via `-ldflags -X` and builds
  static binaries for at least linux/amd64, linux/arm64, darwin/arm64.
- R5: THE `version` verb SHALL be registered like every other verb
  (declared in `internal/core/commands.go`, handler in `internal/cmd/`,
  wired in `registry.go`) and documented in both
  `docs/command-reference.md` and `docs/CHEATSHEET.md`.

## Design notes / best practice

- Version vars: package-level `var version = "dev"`, `commit`, `date` in a
  small `internal/version` package (or main) targeted by
  `-X github.com/0xkhdr/specd/internal/version.Version=...`.
- Fallback: `runtime/debug.ReadBuildInfo()` gives `vcs.revision` and
  `vcs.time` on stdlib alone — no dependency added; zero-runtime-deps
  invariant holds.
- Goreleaser: start from `reference/.goreleaser.yml` as *design input only*
  (never copy blindly, never import reference code); `CGO_ENABLED=0`,
  `-trimpath` for reproducibility.
- CI: a workflow step runs `goreleaser check` (or `release --snapshot
  --clean` in dry-run) so config rot is caught.
- Exit codes: 0 on success; malformed extra args follow the existing
  fail-closed exit-2 convention.

## Out of scope

- Auto-update, version-check-against-remote, changelog generation.
- Signing/notarization (record as follow-up if distribution demands it).

## Acceptance

- `go build -o specd . && ./specd version` prints a sane dev-mode line.
- ldflags build prints injected values; `--json` output round-trips through
  `jq`.
- `goreleaser check` passes; docs updated in both files (docs-lint green).
