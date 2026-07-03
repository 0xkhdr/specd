# Tasks 10 — CLI Architecture & Foundations

> **Build waves:** A (T10.1–T10.4), B (T10.5–T10.7). See `specs/progress.md`.
> **Depends on domains:** 01. **Unblocks:** 02, 07, 09 (all).

## Wave 1 — primitives

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T10.1 | craftsman | `internal/core/io.go` | — | `go test ./internal/core -run TestAtomicWrite` | temp→fsync→rename; append fsyncs |
| T10.2 | craftsman | `internal/core/lock.go` | — | `go test ./internal/core -run TestReentrantLock` | reentrant; stale reclaim; timeout |
| T10.3 | craftsman | `internal/core/paths.go`, `internal/core/slug.go` | — | `go test ./internal/core -run 'TestFindRoot\|TestSlug'` | walk-up NotFound(3); slug grammar `^[a-z0-9][a-z0-9-]*$` |

## Wave 2 — parser, registry, config

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T10.4 | craftsman | `internal/cli/args.go`, `main.go` | — | `go test ./internal/cli -run TestArgs` | zero-dep parser; usage on error |
| T10.5 | craftsman | `internal/cmd/registry.go`, `internal/core/commands.go` | T10.4 | `go test ./internal/core -run TestRegistryMatchesHelp` | dispatch↔help parity |
| T10.6 | craftsman | `internal/core/config_loader.go`, `internal/core/config_validate.go` | — | `go test ./internal/core -run TestConfigCascade` | global→project→env; fail-loud; scrub |
| T10.7 | validator | `internal/core/config_test.go` | T10.6 | `go test ./internal/core -run TestConfigNoLegacyJSON` | legacy config.json not parsed at runtime |

## Traceability (task → requirement)
- T10.1 → R10.3 · T10.2 → R10.4 · T10.3 → R10.5 · T10.4 → R10.1 · T10.5 → R10.2 · T10.6 → R10.6, R10.7 · T10.7 → R10.6 (no legacy)
