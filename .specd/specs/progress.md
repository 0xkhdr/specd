# Spec Suite Progress — Command Palette Optimization

## Overall Status
- Total Specs: 5
- Total Waves: 5 (suite-level integration waves; each spec carries 3 internal waves)
- Tasks Complete: 10 / 27
- Current Phase: cmd-mcp-sync complete

> Surface target: 33 commands → **20 survivors** (16 daily workflow + 4 meta-hidden). 13 commands killed.

## Spec Registry
| Spec | Status | Current Wave | Blockers |
|------|--------|--------------|----------|
| cmd-audit | verifying | complete | none |
| cmd-merge | planned | — | depends on cmd-audit |
| cmd-deprecate | planned | — | depends on cmd-audit |
| cmd-mcp-sync | complete | complete | none |
| cmd-docs | planned | — | depends on cmd-mcp-sync |

## Wave Schedule
### Wave 1: Audit & Analysis
- [x] cmd-audit: T1–T5
### Wave 2: Merge & Deprecate
- [ ] cmd-merge: T1–T6
- [ ] cmd-deprecate: T1–T5
### Wave 3: MCP Sync
- [x] cmd-mcp-sync: T1–T5
### Wave 4: Documentation
- [ ] cmd-docs: T1–T6
### Wave 5: Integration Verify
- [ ] All specs: final verify + approve
  - specd check cmd-audit && specd check cmd-merge && specd check cmd-deprecate && specd check cmd-mcp-sync && specd check cmd-docs
  - go test ./internal/core/ -run 'TestNoDuplicateCommands|TestFlagSingleOwner|TestPaletteCeiling'
  - go test ./internal/mcp/ -run TestCLIMCPParity
  - bash scripts/docs-lint.sh

## Cross-Spec Dependencies
- `cmd-merge` depends on `cmd-audit`
- `cmd-deprecate` depends on `cmd-audit`
- `cmd-mcp-sync` depends on `cmd-merge` and `cmd-deprecate`
- `cmd-docs` depends on `cmd-mcp-sync`

## Survivor Ledger (target)
**Daily workflow (16):** init, new, status, context, check, approve, next, verify, task, report, decision, midreq, memory, waves, brain, pinky
**Meta-hidden (4):** version, help, mcp, fusion
**Killed (13):** doctor, migrate, mode, dispatch, validate, schema, serve, replay, diff, watch, program, update, uninstall
