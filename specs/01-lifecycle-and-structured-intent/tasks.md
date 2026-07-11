# Tasks — Domain 01 DAG

`[ ]` pending. Waves execute only after dependency evidence passes. Files are declared implementation scope; adjust only through recorded decision.

## W0 — Contract and baseline

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T01 | craftsman | internal/core/requirements.go; internal/core/requirements_test.go | | go test ./internal/core -run 'TestRequirements' | R1.1,R1.3 parser/model |
| [x] T02 | craftsman | internal/core/requirements.go; internal/core/requirements_test.go; internal/core/gates/ears.go; internal/core/gates/ears_test.go | T01 | go test ./internal/core ./internal/core/gates -run 'TestRequirements\|TestEARS' | R1.2 exact requirement findings |
| [x] T03 | craftsman | internal/core/state.go; internal/core/state_test.go; internal/core/io.go | T01 | go test ./internal/core -run 'TestState' | schema migration, CAS-safe records |
| [x] T04 | craftsman | internal/core/gates/core.go; internal/core/gates/core_test.go; internal/core/roles.go | | go test ./internal/core ./internal/core/gates -run 'TestRoles' | R4.1 reject unknown role |
| [x] T05 | craftsman | internal/core/config_loader.go; internal/core/config_validate.go; internal/core/gates/core.go; internal/core/gates/core_test.go | T04 | go test ./internal/core ./internal/core/gates -run 'TestVerify' | R4.2 role-aware trivial verify policy |

**W0 cross-wave deviations (files edited beyond the declared lists, recorded per prompt.md §2):**
- `internal/cmd/registry.go` (T05): wired `CheckCtx.TrivialVerify` from config so R4.2 enforces in the live `check`/`approve` path.
- `internal/cmd/lifecycle.go` (T05): scaffold placeholder task `craftsman → scout` — a fresh placeholder is read-only until authored (craftsman + `printf ok` violated R4.2).
- Test-fixture migrations (T04/T05): `builder → craftsman` in 9 test files (builder is not a canonical role); craftsman + trivial verify → `scout` in `task_complete_test.go`, `e2e_test.go`, `criteria_test.go`, `link_test.go`, `lifecycle_test.go`; `brain_run_test.go` verify `printf ok → printf done`; `lifecycle_test.go` role assertion `craftsman → scout`; `config_test.go` `== → reflect.DeepEqual` (VerifyConfig gained a slice field).

## W1 — Design and planning guidance

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T06 | craftsman | internal/core/design.go; internal/core/design_test.go; internal/core/gates/approval.go; internal/core/gates/approval_gate_test.go | T02,T03 | go test ./internal/core ./internal/core/gates -run 'TestDesign' | R2.1 design contract/digest |
| [x] T07 | craftsman | internal/core/gates/approval.go; internal/core/gates/approval_gate_test.go; internal/cmd/lifecycle.go; internal/cmd/lifecycle_test.go | T06 | go test ./internal/core/gates ./internal/cmd -run 'TestDesign' | R2.2 approval refusal |
| [x] T08 | craftsman | internal/core/commands.go; internal/core/commandmeta_test.go; internal/cmd/registry.go; internal/cmd/registry_test.go | T04 | go test ./internal/core ./internal/cmd -run 'Test.*Guide\|TestCommand' | R6.1 legal action model |
| [x] T09 | craftsman | internal/cmd/status.go; internal/cmd/status_test.go; internal/mcp; internal/core/commands.go | T08 | go test ./internal/cmd ./internal/mcp -run 'Test.*Guide\|Test.*Status' | R6.1 JSON CLI/MCP guidance |
| [x] T10 | craftsman | internal/core/embed_templates/AGENTS.md; internal/core/scaffold.go; internal/cmd/init_scaffold_test.go; docs/command-reference.md; docs/CHEATSHEET.md | T09 | go test ./internal/cmd -run TestInitScaffold && ./scripts/docs-lint.sh | R6.2 scaffold command parity |

**W1 cross-wave deviations (files edited beyond the declared lists, recorded per prompt.md §2):**
- `internal/core/gates/core.go` (T06): added `CheckCtx.DesignContractRequired` field to arm the production design-contract check (R2.1) while keeping the default profile backward compatible (R7.1). CheckCtx lives in core.go; the design gate body stays in the declared approval.go.

## W2 — Task trace/risk contract

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T11 | craftsman | internal/core/tasksparser.go; internal/core/tasksparser_test.go; internal/core/tasksparser_fuzz_test.go | T02,T06 | go test ./internal/core -run 'TestTasks\|TestRewrite' | R3.1 backward-compatible task metadata |
| [x] T12 | craftsman | internal/core/tasksparser.go; internal/core/dag.go; internal/core/dag_test.go; internal/core/gates/core.go; internal/core/gates/core_test.go | T11 | go test ./internal/core ./internal/core/gates -run 'Test.*Trace\|TestDAG' | resolvable task refs/risk |
| [x] T13 | craftsman | internal/core/scaffold.go; internal/core/scaffold_test.go; docs/open-spec-format.md; docs/validation-gates.md | T12 | go test ./internal/core -run TestScaffold && ./scripts/docs-lint.sh | author format/docs migration guidance |

**W2 cross-wave deviations (files edited beyond the declared lists, recorded per prompt.md §2):**
- `internal/core/gates/registry_test.go` (T12): added `task-trace` to the pinned registry order — registering the 15th gate required updating this parity assertion.
- `README.md`, `docs/contributor-guide.md`, `docs/README.md` (T12): bumped the "14 core gates" claim to "15" for the new `task-trace` gate (docs-lint pins the count to the registry). `docs/validation-gates.md` (in T13's list) also updated.
- `internal/core/dag.go` / `dag_test.go` (T12 declared, not needed): the trace/risk check resolves refs against requirement ids and validates risk tiers without touching the DAG, so no DAG change was required (subtractive).
- `internal/core/embed_templates/steering/structure.md` (T13): the scaffolded authoring guidance lives in the structure steering template; `scaffold.go` materializes it unchanged, so the template — not `scaffold.go` — carried the new format guidance.

## W3 — Coverage gates

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T14 | craftsman | internal/core/coverage.go; internal/core/coverage_test.go; internal/core/gates/criteria.go; internal/core/gates/criteria_test.go | T12 | go test ./internal/core ./internal/core/gates -run 'TestCoverage\|TestCriteria' | R3.2 requirement/design/task graph |
| [ ] T15 | craftsman | internal/core/evidence_policy.go; internal/core/evidence_policy_test.go; internal/core/gates/registry.go; internal/core/gates/registry_test.go | T14,T05 | go test ./internal/core ./internal/core/gates -run 'TestEvidencePolicy\|TestRegistry' | R3.3 deterministic boundary policy |
| [ ] T16 | craftsman | internal/cmd/lifecycle.go; internal/cmd/lifecycle_test.go; internal/cmd/e2e_test.go | T14,T15 | go test ./internal/cmd -run 'TestLifecycle\|TestCoverage' | execution approval blocks coverage gap |

## W4 — Amendment staleness

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T17 | craftsman | internal/core/amendment.go; internal/core/amendment_test.go; internal/core/state.go; internal/core/state_test.go | T03,T14 | go test ./internal/core -run 'TestAmendment\|TestState' | R5.1 append-only impact record |
| [ ] T18 | craftsman | internal/core/freshness.go; internal/core/freshness_test.go; internal/core/gates/approval.go; internal/core/gates/approval_gate_test.go | T17,T15 | go test ./internal/core ./internal/core/gates -run 'TestFreshness\|TestApproval' | R5.2,R5.3 stale/current rules |
| [ ] T19 | craftsman | internal/cmd/lifecycle.go; internal/cmd/lifecycle_test.go; internal/cmd/dispatch.go; internal/cmd/dispatch_test.go | T18 | go test ./internal/cmd -run 'TestMidreq\|TestDispatch' | unsafe dispatch pause, no rewind |

## W5 — Production profile

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T20 | craftsman | internal/core/config_loader.go; internal/core/config_validate.go; internal/core/config_test.go; internal/core/handshake.go | T15,T18 | go test ./internal/core -run 'TestConfig\|TestHandshake' | R7.1,R7.2 profile/config digest |
| [ ] T21 | craftsman | internal/core/gates/criteria.go; internal/core/gates/review.go; internal/core/gates/criteria_test.go; internal/core/gates/review_test.go | T20 | go test ./internal/core/gates -run 'TestCriteria\|TestReview' | production current evidence/review |
| [ ] T22 | craftsman | docs/open-spec-format.md; docs/validation-gates.md; docs/command-reference.md; docs/CHEATSHEET.md; internal/core/embed_templates/project.yml | T20,T21 | ./scripts/docs-lint.sh && go test ./internal/core -run TestConfig | profile operator docs/template |

## W6 — Bounded spikes

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T23 | craftsman | internal/core/spike.go; internal/core/spike_test.go; internal/core/state.go; internal/cmd/lifecycle.go; internal/cmd/lifecycle_test.go | T17,T20 | go test ./internal/core ./internal/cmd -run 'TestSpike' | R7.3 no spike bypass |

## W7 — Conformance and reports

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T24 | craftsman | internal/cmd/e2e_test.go; internal/cmd/integration_polish_test.go; scripts/regress-domains.sh; scripts/regress-lint.sh | T10,T16,T19,T21,T23 | go test ./internal/cmd -run 'TestLifecycleE2E\|TestIntegration' && ./scripts/regress-domains.sh | R8.1 fresh/restart/negative suite |
| [ ] T25 | craftsman | internal/core/report.go; internal/core/report_test.go; internal/cmd/report.go; internal/cmd/report_history_test.go; docs/command-reference.md; docs/CHEATSHEET.md | T18,T21,T24 | go test ./internal/core ./internal/cmd -run 'TestReport\|TestHistory' && ./scripts/docs-lint.sh | R8.2 stable coverage/staleness report |
| [ ] T26 | validator | SPEC.md; specs/01-lifecycle-and-structured-intent | T25 | go test ./... -race -count=1 && go vet ./... && ./scripts/test-lint.sh && ./scripts/docs-lint.sh && ./scripts/regress-all.sh && ./scripts/regress-domains.sh | all R1-R8 release evidence |

## Cross-wave checks

- Add test before public contract code.
- Run `gofmt -w` only touched Go files; final `gofmt -l .` empty.
- Keep docs command reference + cheatsheet byte-identical.
- Never edit `reference/`.
