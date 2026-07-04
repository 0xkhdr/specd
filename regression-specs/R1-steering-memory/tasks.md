# Tasks R1 — Steering Memory & Promotion

> **Build wave:** R-A. See `regression-specs/progress.md`.
> **Depends on domains:** 10 (config/paths/clock/lock), 02 (spec dirs, `new`), 08 (steering).
> **Unblocks:** 12 (flywheel promotion tier).

## Wave 1 — pure core + primitives

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| TR1.1 | craftsman | `internal/core/memory.go` | — | `go test ./internal/core -run TestMemoryBlock` | `ExtractMemBlock` reads a `## key` block to the next heading; `RenderMemBlock` is byte-stable; `--related` → `[[a]], [[b]]`, absent → `—` |
| TR1.2 | craftsman | `internal/core/config_loader.go`, `internal/core/config_validate.go` | — | `go test ./internal/core -run TestPromotionThreshold` | `PromotionThreshold` defaults to 3, validates `>= 1`, overridable via config cascade |
| TR1.3 | craftsman | `internal/core/paths.go` | — | `go test ./internal/core -run 'TestSpecMemoryPath\|TestListSpecs'` | `SpecMemoryPath`/`SteeringMemoryPath` resolve under `.specd/`; `ListSpecs` enumerates `.specd/specs/*/` |

## Wave 2 — command + scaffold

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| TR1.4 | craftsman | `internal/cmd/memory.go`, `internal/core/commands.go`, `internal/cmd/registry.go` | TR1.1, TR1.2, TR1.3 | `go run . new demo && go run . memory demo add --key k --pattern p --body b --source s --criticality minor && grep -q '## k' .specd/specs/demo/memory.md` | `memory` registered with a non-nil handler; `add` appends under lock; missing flag / bad criticality exits non-zero; parity guard stays green |
| TR1.5 | craftsman | `internal/cmd/lifecycle.go` | — | `go run . new demo && test -f .specd/specs/demo/memory.md` | `new` scaffolds an empty `memory.md` artifact |

## Wave 3 — flywheel + guard

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| TR1.6 | validator | `internal/cmd/memory_test.go` | TR1.4, TR1.5 | `go test ./internal/cmd -run TestMemoryPromoteFlywheel` | below threshold `promote` refuses with observed-count + threshold; at/above threshold (or `--force`) appends the block + deterministic provenance to `steering/memory.md`; missing key fails loud (RM.5); `Clock` injected so output is byte-stable |

## Traceability (task → requirement)
- TR1.1 → RM.1, RM.6 · TR1.2 → RM.3 · TR1.3 → RM.3 · TR1.4 → RM.1, RM.2, RM.8 · TR1.5 → RM.9
- TR1.6 → RM.3, RM.4, RM.5, RM.7 · RM.8 also covered by the existing `TestEveryCommandHasHandler`.
