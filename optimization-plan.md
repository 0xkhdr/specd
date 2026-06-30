# Spec Implementation Optimization Plan — Command Palette Suite

> **Date:** 2026-06-30
> **Branch:** `optimization`
> **Scope:** Review of `.specd/specs/progress.md` and the 5-spec command-palette suite
> (`cmd-audit`, `cmd-merge`, `cmd-deprecate`, `cmd-mcp-sync`, `cmd-docs`) against the
> actual `internal/` implementation.
> **Verdict:** Surface target met (16 daily + 4 meta-hidden = 20 visible). All claimed
> gate tests + docs-lint pass and `go build ./...` is green. But the *merge* layer was
> only half-finished: 10 "killed" commands still ship as silent live aliases, one
> capability is orphaned, and several spec artifacts are stale. None of these are caught
> by the existing gates — they pass *because* the gates only measure the visible palette.

---

## 1. What the specs claimed vs. what shipped

`progress.md` reports **27/27 tasks complete, all 5 specs complete**, surface reduced
33 → 20. That headline is **true at the visible-palette level** and verified:

| Gate | Result |
|------|--------|
| `go test ./internal/core -run 'TestNoDuplicateCommands\|TestFlagSingleOwner\|TestPaletteCeiling'` | PASS |
| `go test ./internal/mcp -run TestCLIMCPParity` | PASS |
| `bash scripts/docs-lint.sh` | PASS (exit 0) |
| `go build ./...` | PASS |

`TestPaletteCeiling` (`internal/core/commands_palette_test.go:50`) asserts `daily ≤ 16`
and `total ≤ 20`, **skipping every command with `DeprecatedIn != ""`**. The 10 merged
commands all carry `Hidden: true` + `DeprecatedIn: "palette-merge"`, so they are invisible
to the ceiling test. **The gates measure the menu, not the kitchen.** The runtime still
dispatches all 13 old names.

### How merge was actually implemented
- **Survivor flags are real and delegate correctly.** `report --history/--diff/--serve/--watch`,
  `next --dispatch`, `check --schema/--schema-only`, `init --repair/--migrate`,
  `status --program` exist in `core.Commands` and route through the original handlers — no
  logic duplication. This satisfies cmd-merge REQ-001/REQ-003. **Good.**
- **But the old top-level names also still work**, via `legacyAlias` in
  `internal/cmd/registry.go:60`:

```go
"doctor":   RunDoctor,      "mode":     RunMode,      "dispatch": RunDispatch,
"program":  RunProgram,     "validate": RunValidate,  "schema":   RunSchema,
"replay":   RunReplay,      "diff":     RunDiff,       "serve":    RunServe,
"watch":    RunWatch,
// only these three give a migration message + non-zero exit:
"migrate":  deprecatedRuntimeCommand(...),
"update":   deprecatedRuntimeCommand(...),
"uninstall":deprecatedRuntimeCommand(...),
```

So there are now **two reachable paths to every merged behavior**: the spec-intended
survivor flag *and* the original command name. The original name executes **silently at
exit 0 with no migration hint** — the opposite of what both specs require.

---

## 2. Gaps (ranked)

### GAP-1 — Merged commands have no sunset path (HIGH)
`doctor, mode, dispatch, program, validate, schema, serve, watch, replay, diff` dispatch
straight to their standalone handlers and succeed silently.

- **cmd-merge** design "Error handling": *"Usage of a removed top-level command name → exit 2
  with a one-line 'moved to `<survivor> --<flag>`' hint, not a silent failure."* — **violated.**
- **cmd-deprecate** REQ-1.2: grace-period commands SHALL print a deprecation warning to
  stderr. — **violated** for the merge set.

Effect: the 13 old names live forever as undocumented hidden aliases with zero nudge toward
the new surface, defeating the memorizable-palette goal. Nothing tests or bounds them.

### GAP-2 — `mode --set` capability is orphaned (HIGH)
The audit maps `mode → new --orchestrated` + `status` (read). But `internal/cmd/mode.go`
also implements `mode --set base|orchestrated`, which **mutates the mode of an existing spec**
(with brain-session safety guards). `new --orchestrated` is **create-only** (`new` errors if
the spec already exists); `status` is read-only. **No survivor can change an existing spec's
mode** — that capability only survives behind the `mode` legacy alias. This violates
cmd-merge REQ-001 ("no capability lost"). If the alias is later removed, the behavior
vanishes silently.

### GAP-3 — `serve` / `watch` classification conflict across specs (MEDIUM)
Three artifacts disagree:
- `cmd-audit/audit-summary.md` lists `serve, watch` under **Deprecations**.
- `cmd-merge` requirements list them under **merge** (`report --serve/--watch`).
- `cmd-deprecate` scope table **omits** them.

Implementation = merged **and** kept as live aliases. The audit ledger (`audit.csv`) is the
declared contract input for both downstream specs; this contradiction means the ledger is no
longer authoritative. Pick one disposition and reconcile all three.

### GAP-4 — Stale audit artifacts (MEDIUM)
`cmd-audit/audit-summary.md` "Documentation gaps" still claims `migrate` and `fusion` are
*"absent as first-class rows in docs/command-reference.md ... flagged undocumented in
registry.txt."* **Already fixed:** `docs/command-reference.md` now has a `fusion` row (line 60),
a `migrate` migration-appendix row (line 92), and `init --migrate` documented. The audit
summary + `registry.txt` were not refreshed after `cmd-docs` shipped, so they now lie.

### GAP-5 — No real removal-version tracking (MEDIUM)
cmd-deprecate REQ-1.3: *"document each deprecation's removal version."* The implementation
uses `DeprecatedIn: "palette-merge"` (a label) and the docs "When removed" column also reads
`palette-merge` — neither is a version. There is no target release at which the aliases are
deleted, so the grace period is unbounded.

### GAP-6 — Dead-weight public API surface (LOW)
`RunDoctor, RunMode, RunDispatch, RunProgram, RunValidate, RunSchema, RunReplay, RunDiff,
RunServe, RunWatch` remain **exported** even though they are no longer top-level commands.
Their files (`doctor.go, mode.go, serve.go, watch.go, dispatch.go, program.go, validate.go,
schema.go, replay.go, diff.go`) are still full standalone command implementations. Once the
survivor flags own these behaviors, the handlers should be unexported helpers (or folded into
the survivor files) so the package API reflects the actual surface.

### GAP-7 — `progress.md` overstates completion (LOW)
"Killed (13)" implies the names are gone. They are **hidden + aliased, not removed**. The
progress doc should distinguish *hidden/aliased-with-sunset* from *removed*, and the Wave 5
verify list should include a guard for the alias surface (see Action 1.3).

---

## 3. Action plan

### Phase 1 — Close the merge sunset (addresses GAP-1, GAP-7)
1.1 Replace each merge-set entry in `legacyAlias` (`internal/cmd/registry.go:60`) so the old
    name routes to the **survivor handler with the merged flag injected**, and emits a
    one-line stderr deprecation warning naming the new home (e.g.
    `specd doctor → use 'specd init --repair'`). Reuse the `deprecatedRuntimeCommand` pattern
    but keep behavior functional during the grace period (cmd-deprecate REQ-1.2).
1.2 Decide exit semantics: cmd-merge design says exit 2 for a removed name; cmd-deprecate REQ-1.2
    says keep functional + warn during grace. Resolve in a decision record — recommended:
    **functional + stderr warning + exit 0 during grace, flip to exit 2 at the removal version.**
1.3 Add `TestLegacyAliasSunset` in `internal/cmd`: assert every `legacyAlias` key (a) emits a
    deprecation warning and (b) has a recorded removal version. This is the missing guard that
    lets the gates see the kitchen, not just the menu.

### Phase 2 — Recover the orphaned capability (addresses GAP-2)
2.1 Decide the survivor home for "change an existing spec's mode." Recommended:
    `status <slug> --set-mode base|orchestrated` (status already owns mode reporting) **or**
    keep `mode` as a documented survivor. Record via `specd decision`.
2.2 Port `mode.go`'s set path (incl. brain-session guards) to the chosen survivor; add a
    parity smoke test. Only then retire the `mode` alias.

### Phase 3 — Reconcile the ledger (addresses GAP-3, GAP-4, GAP-5)
3.1 Settle `serve`/`watch` disposition (recommended: **merge**, since `report --serve/--watch`
    already exist and work) and update `audit.csv`, `audit-summary.md`, `cmd-deprecate` scope,
    and `registry.txt` to agree.
3.2 Refresh `audit-summary.md` "Documentation gaps" + `registry.txt` to reflect that
    `migrate`/`fusion` are now documented.
3.3 Replace `DeprecatedIn: "palette-merge"` labels with a concrete target version (e.g. the
    next minor) in `core.Commands` and the docs "When removed" column.

### Phase 4 — Trim dead surface (addresses GAP-6)
4.1 After Phases 1–2, unexport the orphaned `RunX` handlers (or fold them into the survivor
    files) and delete now-dead standalone wiring. Keep the behavior, shrink the package API.
4.2 Update/relocate the standalone command tests
    (`doctor_test.go, serve_test.go, watch_test.go, diff_test.go, mode_cmd_test.go,
    schema_validate_test.go, program_test.go, …`) to exercise the behavior through the survivor
    flag, so coverage tracks the real entry point.

### Phase 5 — Re-verify and update progress
5.1 Re-run the Wave 5 gate set **plus** the two new tests (1.3, 2.2).
5.2 Update `progress.md`: split "Killed" into *removed* vs *aliased-with-sunset(version)*, and
    add the alias-sunset gate to the Wave 5 verify list.

---

## 4. Suggested order & risk

| Phase | Risk | Why first / blocking |
|-------|------|----------------------|
| 1 (sunset + guard test) | low | Pure routing + new test; unblocks measuring the real surface |
| 2 (mode --set home) | medium | Capability recovery; must precede retiring the `mode` alias |
| 3 (ledger reconcile) | low | Docs/data only; no code risk |
| 4 (unexport/trim) | medium | Do last — depends on 1 & 2 retiring the aliases cleanly |
| 5 (re-verify) | low | Final gate |

**One-line summary:** the palette *looks* optimized but the merge is half-applied — 13 legacy
names still execute silently, `mode --set` has no survivor home, and the audit ledger contradicts
itself. Phases 1–2 are the substantive fixes; 3–5 are cleanup and truth-in-docs.
