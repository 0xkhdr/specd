# Spec Suite Progress — Command Palette Optimization

## Overall Status
- Total Specs: 5
- Total Waves: 5 (suite-level integration waves; each spec carries 3 internal waves)
- Tasks Complete: 27 / 27
- Current Phase: Wave 5 integration verify complete; all specs complete; optimization plan Phases 1–5 applied (sunset guard + ledger reconcile + re-verify)

> Surface target: 33 commands → **20 survivors** (16 daily workflow + 4 meta-hidden). 13 commands retired (hidden), of which 10 ship as functional aliases-with-sunset (removal v0.2.0) and 3 as removed-stubs.

## Spec Registry
| Spec | Status | Current Wave | Blockers |
|------|--------|--------------|----------|
| cmd-audit | complete | complete | none |
| cmd-merge | complete | complete | none |
| cmd-deprecate | complete | complete | none |
| cmd-mcp-sync | complete | complete | none |
| cmd-docs | complete | complete | none |

## Wave Schedule
### Wave 1: Audit & Analysis
- [x] cmd-audit: T1–T5
### Wave 2: Merge & Deprecate
- [x] cmd-merge: T1–T6
- [x] cmd-deprecate: T1–T5
### Wave 3: MCP Sync
- [x] cmd-mcp-sync: T1–T5
### Wave 4: Documentation
- [x] cmd-docs: T1–T6
### Wave 5: Integration Verify
- [x] All specs: final verify + approve
  - specd check cmd-audit && specd check cmd-merge && specd check cmd-deprecate && specd check cmd-mcp-sync && specd check cmd-docs
  - go test ./internal/core/ -run 'TestNoDuplicateCommands|TestFlagSingleOwner|TestPaletteCeiling'
  - go test ./internal/mcp/ -run TestCLIMCPParity
  - go test ./internal/cmd/ -run 'TestLegacyAliasSunset'   # alias-sunset guard: every legacyAlias warns + has removal version
  - bash scripts/docs-lint.sh

## Cross-Spec Dependencies
- `cmd-merge` depends on `cmd-audit`
- `cmd-deprecate` depends on `cmd-audit`
- `cmd-mcp-sync` depends on `cmd-merge` and `cmd-deprecate`
- `cmd-docs` depends on `cmd-mcp-sync`

## Survivor Ledger (target)
**Daily workflow (16):** init, new, status, context, check, approve, next, verify, task, report, decision, midreq, memory, waves, brain, pinky
**Meta-hidden (4):** version, help, mcp, fusion
**Retired (13)** — all `Hidden: true`, `DeprecatedIn: v0.2.0`:
- *Aliased-with-sunset (10)* — merged into a survivor flag; legacy name still functional during grace, emits stderr deprecation warning, removed at **v0.2.0**:
  - `doctor` → `init --repair` · `mode` → `status --set-mode`/`new --orchestrated` · `dispatch` → `next --dispatch` · `validate` → `check --schema` · `schema` → `check --schema-only` · `serve` → `report --serve` · `watch` → `report --watch` · `replay` → `report --history` · `diff` → `report --diff` · `program` → `status --program`
- *Removed-stub (3)* — no survivor; legacy name prints migration message + non-zero exit:
  - `migrate` → `init --migrate` · `update` · `uninstall`

Guard: `TestLegacyAliasSunset` asserts every legacyAlias emits a deprecation warning and carries a recorded removal version (closes the prior "gates measure the menu, not the kitchen" gap).
