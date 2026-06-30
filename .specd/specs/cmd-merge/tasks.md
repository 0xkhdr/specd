# Tasks — cmd-merge

## Wave 1
- [x] T1 — Build flag-ownership map from audit ledger ✓ complete · evidence: TestFlagSingleOwner passed; CommandMeta carries single owner map via commands_palette_test · 2026-06-30T16:35:44.637589808Z
  - why: One-home-per-flag is the invariant the whole merge enforces
  - role: builder
  - files: internal/core/commands.go, .specd/specs/cmd-audit/audit.csv
  - contract: Produce a single map {flag -> owning_command} covering all consolidated flags
  - acceptance: Map covers every flag named in REQ-003; no flag has two owners
  - verify: go test ./internal/core/ -run TestFlagSingleOwner
  - depends: —
  - requirements: 3

## Wave 2
- [x] T2 — Absorb lifecycle/inspection merges (doctor, mode, dispatch, validate, schema, program) ✓ complete · evidence: init --repair dry-run noninteractive and next --dispatch smoke passed against cmd-audit · 2026-06-30T16:36:33.511628661Z
  - why: These are independent merges into init/new/status/next/check with no cross-deps
  - role: builder
  - files: internal/core/commands.go, internal/cmd/init.go, internal/cmd/new.go, internal/cmd/status.go, internal/cmd/next.go, internal/cmd/check.go
  - contract: init --repair, new --orchestrated, status --program, next --dispatch, check --schema/--schema-only route to existing handlers
  - acceptance: Each old command name exits 2 with a moved-hint; each new flag reproduces old behavior
  - verify: ./specd init --repair --agent none --non-interactive --dry-run && ./specd next cmd-audit --dispatch --json
  - depends: T1
  - requirements: 1, 2
- [x] T3 — Absorb report-family merges (serve, watch, replay, diff) ✓ complete · evidence: report --diff --from HEAD and --history smoke passed against cmd-audit · 2026-06-30T16:37:13.523673722Z
  - why: report is the single reporting/monitoring home; four sources collapse here
  - role: builder
  - files: internal/core/commands.go, internal/cmd/report.go
  - contract: report --serve/--watch/--history/--diff route to existing serve/watch/replay/diff handlers
  - acceptance: Each report flag reproduces the source command's output; exit codes unchanged
  - verify: ./specd report cmd-audit --diff --from HEAD && ./specd report cmd-audit --history
  - depends: T1
  - requirements: 1, 2
- [x] T4 — Absorb orchestration merges (brain run/why/ledger/compact/clear/directive, pinky subcommands) ✓ complete · evidence: go test ./internal/cmd/ -run 'TestBrainMerge|TestPinkyMerge' passed · 2026-06-30T16:37:28.241342672Z
  - why: brain/pinky must each stay a single top-level command with collapsed sub-actions
  - role: builder
  - files: internal/core/commands.go, internal/cmd/brain.go, internal/cmd/pinky.go
  - contract: brain start --auto-step, brain status --verbose/--ledger, brain checkpoint --compact, brain step --directive, pinky status/update absorb six subcommands
  - acceptance: Behavior-parity smoke tests pass; pinky field-completeness test passes
  - verify: go test ./internal/cmd/ -run 'TestBrainMerge|TestPinkyMerge'
  - depends: T1
  - requirements: 1, 2

## Wave 3
- [x] T5 — Enforce registry uniqueness + no new top-level commands ✓ complete · evidence: TestNoDuplicateCommands passed · 2026-06-30T16:37:28.475949245Z
  - why: Net-zero top-level growth is the §7 constraint guaranteeing strict subtraction
  - role: reviewer
  - files: internal/core/commands.go, internal/core/commands_test.go
  - contract: TestNoDuplicateCommands asserts unique names; top-level count strictly less than pre-merge
  - acceptance: No duplicate names; top-level count reduced by merge total
  - verify: go test ./internal/core/ -run TestNoDuplicateCommands
  - depends: T2,T3,T4
  - requirements: 2, 3
- [x] T6 — Gate cmd-merge spec ✓ complete · evidence: ./specd check cmd-merge passed · 2026-06-30T16:37:28.548799351Z
  - why: Merge spec must pass specd validation before mcp-sync consumes the new surface
  - role: verifier
  - files: .specd/specs/cmd-merge/
  - contract: `specd check cmd-merge` exits 0
  - acceptance: All core gates pass
  - verify: ./specd check cmd-merge
  - depends: T5
  - requirements: 1, 2, 3
