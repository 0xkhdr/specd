# Tasks — Regression: CLI + Command Surface (args, lifecycle, JSON contracts)

## Wave 1
- [x] T1 — Enumerate subcommands, flags, and json/exit-code coverage ✓ complete · evidence: Investigator task (verify N/A). Inventory in memory.md: 23 dispatchable cmds, 9 --json cmds, exit taxonomy 0/1/2/3, 4 scoped gaps mapped to T2/T3/T4. No code edited. · 2026-06-17T17:13:54.41881886Z
  - why: the contract can't be frozen until it's fully listed (R1-R4)
  - role: investigator
  - files: internal/cmd/registry.go, internal/cli/args.go
  - contract: list every subcommand, its flags, whether it supports --json, and its test; mark gaps; do NOT edit
  - acceptance: table {cmd -> flags, --json?, parse-test?, json-test?, exit-code-test?}
  - verify: N/A
  - depends: —
  - requirements: 1, 2, 4

## Wave 2
- [x] T2 — Argument parse + help golden tests for every subcommand ✓ complete · evidence: go test ./internal/cli/... ./internal/cmd/ -run 'Args|Registry|Help' exit 0. Added TestDocumentedFlagsParse + TestDocumentedBooleanFlagsRegistered + TestUnknownFlagIsTolerated (args_test.go, R1.1/freeze ADR-001) and TestEveryRegisteredCommandHasHelp (registry_test.go, R1.3 help integrity for all 23 cmds). · 2026-06-17T17:17:44.654270114Z
  - why: R1 predictability across the whole surface
  - role: builder
  - files: internal/cli/args_test.go, internal/cmd/registry_test.go
  - contract: table-test parse for each cmd incl unknown-flag non-zero and --help zero; do NOT change parsing behavior
  - acceptance: R1.1-R1.3 pass for every subcommand
  - verify: go test ./internal/cli/... ./internal/cmd/ -run 'Args|Registry|Help'
  - depends: T1
  - requirements: 1

- [x] T3 — JSON-contract stability tests ✓ complete · evidence: go test ./internal/cmd/ -run JSON exit 0. Added check/waves/approve schema subtests to TestJSONContracts, plus TestJSONErrorPath (R2.3: failing check emits ok:false+violations under --json, ExitGate), TestJSONNoANSI (R2.2: 7 --json cmds, zero ESC bytes), TestJSONUninstall (uninstall --json on temp HOME). · 2026-06-17T17:25:42.805358164Z
  - why: R2 — agents must not break on format drift
  - role: builder
  - files: internal/cmd/json_contract_test.go
  - contract: for each --json command assert valid JSON, expected top-level keys, zero ANSI; assert error path still emits JSON
  - acceptance: R2.1-R2.3 pass
  - verify: go test ./internal/cmd/ -run JSON
  - depends: T1
  - requirements: 2

- [x] T4 — Lifecycle E2E + exit-code golden ✓ complete · evidence: go test ./internal/cmd/ ./internal/core/ -run 'Lifecycle|Exit' exit 0. Added core/exit_test.go TestExitCodeTaxonomyGolden (R4.1/R4.3 pinned distinct codes 0/1/2/3) and lifecycle_test.go: TestLifecycleReportReflectsFinalState (R3.3 report reflects complete), TestLifecycleOutOfOrderBlocked (R3.2), TestExitCodeTaxonomyCLI (R4.1/R4.2 golden table: success→0, validation→1, gate→1, notfound→3, usage→2). · 2026-06-17T17:31:39.970604843Z
  - why: R3, R4 — the full author workflow and stable exit taxonomy
  - role: builder
  - files: internal/cmd/lifecycle_test.go, internal/core/exit_test.go
  - contract: drive new->check->approve->task->report in temp repo; golden-table exit codes for success/validation/gate-block
  - acceptance: R3.1-R3.3 and R4.1-R4.3 pass
  - verify: go test ./internal/cmd/ ./internal/core/ -run 'Lifecycle|Exit'
  - depends: T1
  - requirements: 3, 4

## Wave 3
- [x] T5 — Review CLI contract for brittleness and undocumented exits ✓ complete · evidence: go test ./internal/cli/... ./internal/cmd/ -count=2 exit 0 (stable, no flakes). Review clean: no byte-equality goldens, ANSI leak structurally impossible (PrintJSON colorless + NO_COLOR), exit codes documented per-cmd + Registry parity (no UNMAPPED), R1.2 deviation recorded as ADR-001. No blocking defects. Findings in memory.md cli-regression-review. · 2026-06-17T17:33:06.278036466Z
  - why: over-strict goldens flake; undocumented exit codes break scripts
  - role: reviewer
  - files: internal/cli, internal/cmd
  - contract: review T2-T4 for byte-equality brittleness, ANSI leak into --json, undocumented exit codes; flag only
  - acceptance: every subcommand covered; exit codes documented; no UNMAPPED cmd
  - verify: go test ./internal/cli/... ./internal/cmd/ -count=2
  - depends: T2, T3, T4
  - requirements: 1, 2, 3, 4
