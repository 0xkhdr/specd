# Tasks — cmd-mcp-sync

## Wave 1
- [x] T1 — Audit current MCP tool generation against registry ✓ complete · evidence: Read internal/mcp/tools.go and internal/mcp/intent.go; generation path and intent alias source identified. · 2026-06-30T16:07:01.337763515Z
  - why: Must know how MCP derives tools before proving parity post-merge
  - role: investigator
  - files: internal/mcp/
  - contract: Document how CommandMeta maps to advertised MCP tools and where intent aliases are defined
  - acceptance: Generation path + intent-alias source located and noted
  - verify: N/A (read-only investigation; findings recorded in spec memory)
  - depends: —
  - requirements: 1, 2, 3

## Wave 2
- [x] T2 — Honor Hidden + removed entries in tool generation ✓ complete · evidence: specd verify cmd-mcp-sync T2 PASS · 2026-06-30T16:07:06.699540276Z
  - why: MCP must not advertise meta-hidden or retired commands
  - role: builder
  - files: internal/mcp/, internal/core/commands.go
  - contract: Generator skips Hidden and retired entries; default tool list == non-hidden survivors
  - acceptance: serve/watch/replay/diff/update/uninstall absent from default tool list
  - verify: go test ./internal/mcp/ -run TestHiddenExcluded
  - depends: T1
  - requirements: 1
- [x] T3 — Surface absorbed flags + remap intent tools ✓ complete · evidence: specd verify cmd-mcp-sync T3 PASS · 2026-06-30T16:07:07.047519876Z
  - why: Merged behavior must remain reachable through survivor tool schemas and intent aliases
  - role: builder
  - files: internal/mcp/
  - contract: Survivor input schemas gain absorbed-flag properties; brain_orchestrate→brain start --auto-step, brain_status→brain status --verbose/--ledger
  - acceptance: report tool schema exposes serve/watch/history/diff; intent aliases resolve to survivors
  - verify: go test ./internal/mcp/ -run TestIntentAliasResolve
  - depends: T2
  - requirements: 2

## Wave 3
- [x] T4 — Enforce CLI↔MCP parity test ✓ complete · evidence: specd verify cmd-mcp-sync T4 PASS · 2026-06-30T16:07:12.398152966Z
  - why: Automated parity is the only durable guard against surface drift
  - role: reviewer
  - files: internal/mcp/parity_test.go
  - contract: TestCLIMCPParity asserts set(MCP tools)==set(CLI survivors not Hidden); prints symmetric diff on failure
  - acceptance: Parity test passes against the optimized surface
  - verify: go test ./internal/mcp/ -run TestCLIMCPParity
  - depends: T3
  - requirements: 1, 3
- [x] T5 — Gate cmd-mcp-sync spec ✓ complete · evidence: specd verify cmd-mcp-sync T5 PASS · 2026-06-30T16:07:12.459317219Z
  - why: Must pass validation before docs document the final surface
  - role: verifier
  - files: .specd/specs/cmd-mcp-sync/
  - contract: `specd check cmd-mcp-sync` exits 0
  - acceptance: All core gates pass
  - verify: specd check cmd-mcp-sync
  - depends: T4
  - requirements: 3
