# Tasks — cmd-merge

## Wave 1
- [ ] T1 — Build flag-ownership map from audit ledger
  - why: One-home-per-flag is the invariant the whole merge enforces
  - role: builder
  - files: internal/core/commands.go, .specd/specs/cmd-audit/audit.csv
  - contract: Produce a single map {flag -> owning_command} covering all consolidated flags
  - acceptance: Map covers every flag named in REQ-003; no flag has two owners
  - verify: go test ./internal/core/ -run TestFlagSingleOwner
  - depends: —

## Wave 2
- [ ] T2 — Absorb lifecycle/inspection merges (doctor, mode, dispatch, validate, schema, program)
  - why: These are independent merges into init/new/status/next/check with no cross-deps
  - role: builder
  - files: internal/core/commands.go, internal/cmd/init.go, internal/cmd/new.go, internal/cmd/status.go, internal/cmd/next.go, internal/cmd/check.go
  - contract: init --repair, new --orchestrated, status --program, next --dispatch, check --schema/--schema-only route to existing handlers
  - acceptance: Each old command name exits 2 with a moved-hint; each new flag reproduces old behavior
  - verify: specd init --repair && specd next cmd-merge --dispatch --json
  - depends: T1
- [ ] T3 — Absorb report-family merges (serve, watch, replay, diff)
  - why: report is the single reporting/monitoring home; four sources collapse here
  - role: builder
  - files: internal/core/commands.go, internal/cmd/report.go
  - contract: report --serve/--watch/--history/--diff route to existing serve/watch/replay/diff handlers
  - acceptance: Each report flag reproduces the source command's output; exit codes unchanged
  - verify: specd report cmd-merge --diff && specd report cmd-merge --history
  - depends: T1
- [ ] T4 — Absorb orchestration merges (brain run/why/ledger/compact/clear/directive, pinky subcommands)
  - why: brain/pinky must each stay a single top-level command with collapsed sub-actions
  - role: builder
  - files: internal/core/commands.go, internal/cmd/brain.go, internal/cmd/pinky.go
  - contract: brain start --auto-step, brain status --verbose/--ledger, brain checkpoint --compact, brain step --directive, pinky status/update absorb six subcommands
  - acceptance: Behavior-parity smoke tests pass; pinky field-completeness test passes
  - verify: go test ./internal/cmd/ -run 'TestBrainMerge|TestPinkyMerge'
  - depends: T1

## Wave 3
- [ ] T5 — Enforce registry uniqueness + no new top-level commands
  - why: Net-zero top-level growth is the §7 constraint guaranteeing strict subtraction
  - role: reviewer
  - files: internal/core/commands.go, internal/core/commands_test.go
  - contract: TestNoDuplicateCommands asserts unique names; top-level count strictly less than pre-merge
  - acceptance: No duplicate names; top-level count reduced by merge total
  - verify: go test ./internal/core/ -run TestNoDuplicateCommands
  - depends: T2,T3,T4
- [ ] T6 — Gate cmd-merge spec
  - why: Merge spec must pass specd validation before mcp-sync consumes the new surface
  - role: verifier
  - files: .specd/specs/cmd-merge/
  - contract: `specd check cmd-merge` exits 0
  - acceptance: All core gates pass
  - verify: specd check cmd-merge
  - depends: T5
