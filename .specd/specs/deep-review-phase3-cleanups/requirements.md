# Requirements — deep-review-phase3-cleanups

> Source: DEEP-REVIEW.md §2 findings #6, #7, #8; §4 Phase 3.

## R1 — Replace hand-rolled helpers with stdlib

- owner: 0xkhdr
- priority: should
- risk: low

- R1.1: When membership checks are needed, the system shall use `slices.Contains` instead of the hand-rolled `contains` (internal/core/config_validate.go, internal/cmd/dispatch.go), `containsString` (internal/core/authority.go), and `containsPhase` (internal/core/driver.go) helpers.
- R1.2: When sorted map keys are needed, the system shall use `slices.Sorted(maps.Keys(m))` instead of the three hand-rolled `sortedKeys` copies (internal/cmd/registry.go, internal/core/prometheus.go, internal/core/gates/intake.go).
- edge: If a helper carries semantics beyond plain membership (custom equality or type), the system shall keep it and record why in the design.

## R2 — Honest handler layout in internal/cmd

- owner: 0xkhdr
- priority: should
- risk: medium

- R2.1: When a verb handler is looked up, the system shall find it in that verb's own file under `internal/cmd/`, with `registry.go` holding only the verb→handler map, dispatch plumbing, and shared helpers.
- R2.2: When the split lands, the system shall show no behavior change: the diff shall be pure moves plus import adjustments, with the full test suite green.
- edge: If a helper is shared by several handlers, the system shall keep it in registry.go (or a shared helpers file) rather than duplicate it.

## Non-goals

- No handler logic changes, renames, or signature changes.
- No wholesale `sort` → `slices` migration where call sites are not drop-in.
