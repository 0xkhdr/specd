# Tasks — Domain 04 Quality DAG

`[ ]` pending. Execute wave only after dependencies pass. Files declared scope; record deviation
before edit. Cross-domain links remain README program links, not local task ids.

## W0 — baseline and contract decision

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T01 | scout | docs/google-sdlc-alignment/README.md; docs/google-sdlc-alignment/04-verification-evals-and-quality.md; specs/04-verification-evals-and-quality | | printf ok | map R1-R7 to current evidence/criteria/complete/gates/ACP/context/report surfaces and Domain 01/02/05/06/07/09/10 boundaries |
| [ ] T02 | craftsman | internal/core/evidence_test.go; internal/core/task_complete_test.go; internal/core/tasksparser_test.go; internal/core/gates/core_test.go; internal/cmd/lifecycle_test.go | T01 | go test ./internal/core ./internal/core/gates ./internal/cmd -run 'Test(Evidence|CompleteTask|Tasks|Core|Lifecycle)' | failing legacy/stale/wrong-class/wrong-task/malformed baseline; old verify compatibility fixed |
| [ ] T03 | craftsman | internal/core/tasksparser.go; internal/core/tasksparser_test.go; docs/open-spec-format.md; docs/validation-gates.md | T01 | go test ./internal/core -run 'TestTasks' && ./scripts/docs-lint.sh | choose companion vs task-table quality declaration; byte-stable migration and schema documented R1,R2 |

## W1 — evidence envelope and declaration

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T04 | craftsman | internal/core/eval.go; internal/core/eval_test.go; internal/core/evidence.go; internal/core/evidence_test.go | T02,T03 | go test ./internal/core -run 'Test(Eval|Evidence)' | V1 envelope/classes/identity/ordered canonical digest; unknown input fails R1 |
| [ ] T05 | craftsman | internal/core/quality_contract.go; internal/core/quality_contract_test.go; internal/core/tasksparser.go; internal/core/tasksparser_test.go | T03,T04 | go test ./internal/core -run 'Test(QualityContract|Tasks|Eval)' | task required class/check refs; legacy task bytes unchanged; cross-class satisfaction refused R2 |
| [ ] T06 | craftsman | internal/core/paths.go; internal/core/paths_test.go; internal/core/eval_store.go; internal/core/eval_store_test.go; internal/core/io.go | T04 | go test ./internal/core -run 'Test(EvalStore|Path|Atomic)' | canonical eval/trace paths; append/atomic local storage and duplicate identity refusal R1,R3 |

## W2 — import, freshness, trajectory

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T07 | craftsman | internal/core/eval_import.go; internal/core/eval_import_test.go; internal/core/eval_store.go; internal/core/eval_store_test.go | T04,T06 | go test ./internal/core -run 'Test(EvalImport|EvalStore)' | JSON/JSONL schema/digest/task/check/provenance validation; stable findings; no network R3 |
| [ ] T08 | craftsman | internal/core/evidence_freshness.go; internal/core/evidence_freshness_test.go; internal/core/task_complete.go; internal/core/task_complete_test.go; internal/core/gates/core.go; internal/core/gates/core_test.go | T05,T07 | go test ./internal/core ./internal/core/gates -run 'Test(EvidenceFreshness|CompleteTask|Core)' | current HEAD/diff/output/dataset/rubric/trace policy; required test failure no bypass R3 |
| [ ] T09 | craftsman | internal/orchestration/trace.go; internal/orchestration/trace_test.go; internal/orchestration/acp.go; internal/orchestration/acp_test.go | T04 | go test ./internal/orchestration -run 'Test(Trace|ACP)' | normalized observable V1 events, monotonic sequence/unique ids/sanitized field rejection R4 |
| [ ] T10 | craftsman | internal/core/trajectory.go; internal/core/trajectory_test.go; internal/core/eval_import.go; internal/core/eval_import_test.go | T07,T09 | go test ./internal/core ./internal/orchestration -run 'Test(Trajectory|EvalImport|Trace)' | required/forbidden tool policy and trace digest validation; no hidden reasoning R4 |

## W3 — coverage and gate composition

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T11 | craftsman | internal/core/quality_policy.go; internal/core/quality_policy_test.go; internal/core/gates/quality.go; internal/core/gates/quality_test.go; internal/core/gates/registry.go | T05,T08,T10 | go test ./internal/core ./internal/core/gates -run 'Test(QualityPolicy|Quality|Registry)' | required evidence composition; stable offline gate order; stale/missing class blocks R3,R5 |
| [ ] T12 | craftsman | internal/core/criteria.go; internal/core/criteria_test.go; internal/core/quality_policy.go; internal/core/quality_policy_test.go; internal/core/gates/quality.go; internal/core/gates/quality_test.go | T11 | go test ./internal/core ./internal/core/gates -run 'Test(Criteria|QualityPolicy|Quality)' | critical acceptance→check mapping; unknown/uncovered/threshold-less refs fail R5 |
| [ ] T13 | craftsman | internal/core/gates/quality.go; internal/core/gates/quality_test.go; scripts/regress-lint.sh; scripts/regress-domains.sh | T12 | go test ./internal/core/gates -run TestQuality && ./scripts/regress-lint.sh && ./scripts/regress-domains.sh | production-risk trivial/compile-only verify lint; explicit read-only exception R5 |

## W4 — adapters, dataset/rubric governance

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T14 | craftsman | internal/cmd/eval.go; internal/cmd/eval_test.go; internal/cmd/registry.go; internal/cmd/registry_test.go; internal/core/eval_import.go | T07,T11 | go test ./internal/cmd ./internal/core -run 'Test(Eval|Registry|EvalImport)' | local `eval import/status` contract; adapter absence does not network/downgrade gate R3,R6 |
| [ ] T15 | craftsman | internal/core/eval_policy.go; internal/core/eval_policy_test.go; internal/core/eval_import.go; internal/core/eval_import_test.go | T11,T14 | go test ./internal/core -run 'Test(EvalPolicy|EvalImport)' | code/human/heuristic/LM metadata validation; fixed-run aggregation/critical-case/inadequate-sample deterministic R6 |
| [ ] T16 | craftsman | internal/core/evalset.go; internal/core/evalset_test.go; internal/core/eval_policy.go; internal/core/eval_policy_test.go; docs/open-spec-format.md | T15 | go test ./internal/core -run 'Test(Evalset|EvalPolicy)' | owner/version/digest/cases/rubric/redaction/review schema; edits invalidate old evidence R6 |

## W5 — quality packet, review, flywheel, release proof

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T17 | craftsman | internal/context/manifest.go; internal/context/manifest_test.go; internal/context/hud.go; internal/context/hud_test.go; internal/core/quality_contract.go | T05,T12,T16 | go test ./internal/context ./internal/core -run 'Test(Manifest|HUD|QualityContract)' | compact labelled quality contract with refs/digests/freshness; no raw dataset/trace R7 |
| [ ] T18 | craftsman | internal/core/review.go; internal/core/review_test.go; internal/core/embed_templates/roles/auditor.md; internal/context/manifest.go; internal/context/manifest_test.go | T12,T17 | go test ./internal/core ./internal/context -run 'Test(Review|Manifest)' | production review prompts/contracts hard-20% risks; review cannot bypass required test R7 |
| [ ] T19 | craftsman | internal/core/quality_ledger.go; internal/core/quality_ledger_test.go; internal/core/report.go; internal/core/report_test.go | T15,T16 | go test ./internal/core -run 'Test(QualityLedger|Report)' | redacted append-only failure/promotion ledger; report separates proof/gap/stale/score R7 |
| [ ] T20 | craftsman | internal/cmd/e2e_test.go; internal/core/gates/quality_test.go; internal/orchestration/trace_test.go; internal/context/manifest_test.go; scripts/regress-domains.sh | T13,T14,T17,T18,T19 | go test ./internal/cmd ./internal/core/gates ./internal/orchestration ./internal/context -run 'Test(LifecycleE2E|Quality|Trace|Manifest)' && ./scripts/regress-domains.sh | stale rubric, failed verify+high judge, missing trajectory, shallow verify, malformed adapter, outage black-box proof |
| [ ] T21 | craftsman | docs/open-spec-format.md; docs/validation-gates.md; docs/command-reference.md; docs/CHEATSHEET.md; docs/contributor-guide.md; docs/agent-integration.md | T20 | ./scripts/docs-lint.sh && go test ./internal/cmd -run 'Test(Eval|Registry)' | operator/adapter/migration docs synchronized |
| [ ] T22 | validator | specs/04-verification-evals-and-quality; internal/core; internal/core/gates; internal/cmd; internal/context; internal/orchestration | T21 | go test ./... -race -count=1 && go vet ./... && ./scripts/test-lint.sh && ./scripts/docs-lint.sh && ./scripts/regress-all.sh && ./scripts/regress-domains.sh | full Domain 04 evidence |

## Cross-wave rules

- Add failing public-contract test before change. Never mark a task complete through imported score
  alone; current `verify` remains non-bypass foundation.
- Domain 05 owns live worker mission/lease; Domain 06 owns trusted diff scope/profile authority;
  Domain 04 consumes validated identities only.
- Keep eval runner/model/network outside core. Missing external service reports policy outcome,
  never causes offline `specd check` to call network or silently pass.
- Preserve byte-stable tasks parser, atomic/CAS/lock behavior, zero runtime dependencies, and
  `reference/` museum boundary.
