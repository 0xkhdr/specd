# Tasks — Regression: CLI + Command Surface (args, lifecycle, JSON contracts)

## Wave 1
- [ ] T1 — Enumerate subcommands, flags, and json/exit-code coverage
  - why: the contract can't be frozen until it's fully listed (R1-R4)
  - role: investigator
  - files: internal/cmd/registry.go, internal/cli/args.go
  - contract: list every subcommand, its flags, whether it supports --json, and its test; mark gaps; do NOT edit
  - acceptance: table {cmd -> flags, --json?, parse-test?, json-test?, exit-code-test?}
  - verify: N/A
  - depends: —
  - requirements: 1, 2, 4

## Wave 2
- [ ] T2 — Argument parse + help golden tests for every subcommand
  - why: R1 predictability across the whole surface
  - role: builder
  - files: internal/cli/args_test.go, internal/cmd/registry_test.go
  - contract: table-test parse for each cmd incl unknown-flag non-zero and --help zero; do NOT change parsing behavior
  - acceptance: R1.1-R1.3 pass for every subcommand
  - verify: go test ./internal/cli/... ./internal/cmd/ -run 'Args|Registry|Help'
  - depends: T1
  - requirements: 1

- [ ] T3 — JSON-contract stability tests
  - why: R2 — agents must not break on format drift
  - role: builder
  - files: internal/cmd/json_contract_test.go
  - contract: for each --json command assert valid JSON, expected top-level keys, zero ANSI; assert error path still emits JSON
  - acceptance: R2.1-R2.3 pass
  - verify: go test ./internal/cmd/ -run JSON
  - depends: T1
  - requirements: 2

- [ ] T4 — Lifecycle E2E + exit-code golden
  - why: R3, R4 — the full author workflow and stable exit taxonomy
  - role: builder
  - files: internal/cmd/lifecycle_test.go, internal/core/exit_test.go
  - contract: drive new->check->approve->task->report in temp repo; golden-table exit codes for success/validation/gate-block
  - acceptance: R3.1-R3.3 and R4.1-R4.3 pass
  - verify: go test ./internal/cmd/ ./internal/core/ -run 'Lifecycle|Exit'
  - depends: T1
  - requirements: 3, 4

## Wave 3
- [ ] T5 — Review CLI contract for brittleness and undocumented exits
  - why: over-strict goldens flake; undocumented exit codes break scripts
  - role: reviewer
  - files: internal/cli, internal/cmd
  - contract: review T2-T4 for byte-equality brittleness, ANSI leak into --json, undocumented exit codes; flag only
  - acceptance: every subcommand covered; exit codes documented; no UNMAPPED cmd
  - verify: go test ./internal/cli/... ./internal/cmd/ -count=2
  - depends: T2, T3, T4
  - requirements: 1, 2, 3, 4
