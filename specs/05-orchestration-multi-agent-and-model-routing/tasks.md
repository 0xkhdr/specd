# Tasks — Domain 05 Orchestration DAG

`[ ]` pending. Execute wave after dependencies pass. Touch declared files only; record deviation.
Cross-domain prerequisites remain README links, not local task ids.

## W0 — inventory, wording, contract baseline

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T01 | scout | docs/google-sdlc-alignment/README.md; docs/google-sdlc-alignment/05-orchestration-multi-agent-and-model-routing.md; specs/05-orchestration-multi-agent-and-model-routing | | printf ok | map R1-R6 to Brain/ACP/session/lease/config/MCP/report surfaces and Domain 01/02/03/04/06/07/10 boundaries |
| [x] T02 | craftsman | internal/cmd/brain_run_test.go; internal/cmd/brain_lifecycle_test.go; internal/cmd/brain_worker_test.go; internal/orchestration/acp_rigor_test.go; internal/orchestration/recover_test.go | T01 | go test ./internal/cmd ./internal/orchestration -run 'Test(Brain|Worker|ACP|Recover)' | failing no-launch, controller-lease, missing-public-lifecycle, stale-pin, claim-race baseline R1-R4 |
| [x] T03 | craftsman | docs/command-reference.md; docs/CHEATSHEET.md; docs/google-sdlc-alignment/05-orchestration-multi-agent-and-model-routing.md | T01 | ./scripts/docs-lint.sh | docs say dispatch records pending mission, not worker/model launch; exact present limitations and migration route R1 |

> **W0 deviations.** T01 maps R1 Brain/ACP/session, R2 lease, R3 worker/report, R4 recovery,
> R5 config/routing, R6 MCP/report adapters, with 01/02/03/04/06/07/10 boundaries. Existing
> brain/ACP/recovery tests already pin no-launch, controller lease, stale/race limitations, so T02
> required no duplicate fixtures. Domain alignment doc already stated limitations; T03 tightened
> only synchronized command docs to remove ambiguous “dispatch” wording.

## W1 — mission and pending dispatch

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T04 | craftsman | internal/orchestration/mission.go; internal/orchestration/mission_test.go; internal/orchestration/acp.go; internal/orchestration/acp_test.go | T02 | go test ./internal/orchestration -run 'Test(Mission|ACP)' | canonical MissionV1/version/digest/pins/ordered serialization; unknown/missing/duplicate fail R1 |
| [x] T05 | craftsman | internal/orchestration/session.go; internal/orchestration/session_test.go; internal/orchestration/checkpoint.go; internal/orchestration/checkpoint_test.go; internal/orchestration/lease.go; internal/orchestration/lease_test.go | T04 | go test ./internal/orchestration -run 'Test(Session|Checkpoint|Lease)' | pending mission distinct from LeaseV1; unique lease id/state/revocation model R1,R2 |
| [x] T06 | craftsman | internal/cmd/brain_run.go; internal/cmd/brain_run_test.go; internal/cmd/brain_lifecycle_test.go; internal/cmd/report.go; internal/cmd/report_test.go | T03,T04,T05 | go test ./internal/cmd ./internal/orchestration -run 'Test(Brain|Report|Mission|Session)' | Brain writes checkpoint then pending ACP mission; never `worker_id=brain`; status/report names pending/no delivery R1 |
| [x] T07 | craftsman | internal/core/driver.go; internal/core/driver_test.go; internal/context/manifest.go; internal/context/manifest_test.go; internal/orchestration/mission.go | T04 | go test ./internal/core ./internal/context ./internal/orchestration -run 'Test(Driver|Manifest|Mission)' | consume Domain 03 dispatch + Domain 02 receipt pin; role/files/acceptance/verify/context/config/palette/authority/subject drift fails R1 |

> **W1 deviations.** Existing ACP duplicate-mission, checkpoint ordering/identity, and report
> projection contracts already supported pending missions, so `acp.go`, `acp_test.go`,
> `checkpoint.go`, `checkpoint_test.go`, `report.go`, and `report_test.go` needed no edits.

## W2 — worker lifecycle and normal completion

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T08 | craftsman | internal/orchestration/worker.go; internal/orchestration/worker_test.go; internal/orchestration/lease.go; internal/orchestration/lease_test.go; internal/orchestration/acp.go | T05,T07 | go test ./internal/orchestration -run 'Test(Worker|Lease|ACP)' | registered worker/capability claim validates role/pins; locked claim race grants one typed lease R2 |
| [x] T09 | craftsman | internal/cmd/brain.go; internal/cmd/brain_claim_test.go; internal/cmd/registry.go; internal/cmd/registry_test.go; internal/core/commands.go | T08 | go test ./internal/cmd ./internal/orchestration -run 'Test(BrainClaim|Registry|Worker|Lease)' | public versioned `brain claim` parser/output/error codes; no direct state edit R2 |
| [x] T10 | craftsman | internal/orchestration/heartbeat.go; internal/orchestration/heartbeat_test.go; internal/cmd/brain.go; internal/cmd/brain_heartbeat_test.go | T08,T09 | go test ./internal/cmd ./internal/orchestration -run 'Test(Heartbeat|BrainHeartbeat|Lease)' | matching live lease heartbeat/renewal bounded by policy; stale/wrong/revoked rejected R3 |
| [x] T11 | craftsman | internal/cmd/brain_worker.go; internal/cmd/brain_report_test.go; internal/cmd/brain.go; internal/orchestration/report.go; internal/orchestration/report_test.go | T09,T10 | go test ./internal/cmd ./internal/orchestration -run 'Test(BrainReport|WorkerReport|Report)' | report validates mission/lease/worker/role/pins/current evidence then normal completion path; no helper bypass R3 |
| [x] T12 | craftsman | internal/core/diff.go; internal/core/diff_test.go; internal/cmd/brain_worker.go; internal/cmd/brain_report_test.go; internal/core/gates/scope.go; internal/core/gates/scope_test.go | T11 | go test ./internal/core ./internal/core/gates ./internal/cmd -run 'Test(Diff|Scope|BrainReport)' | local HEAD/diff server-observed; claimed scope disagreement retained/refused; consume Domain 06 scope verdict R3 |

## W3 — recovery, cancellation, conformance

> **W2 deviations.** Repository has no `internal/cmd/brain.go`; public lifecycle parsing stays in
> `brain_run.go` with atomic helpers split into `brain_claim.go`, `brain_heartbeat.go`, and
> `brain_report.go`. Command metadata changed CLI guidance, requiring synchronized command docs.
> Existing `brain_worker.go` evidence checks were reused. Domain 06 scope files were introduced
> early because T12 requires the harness-derived verdict; worker-reported paths remain audit-only.

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T13 | craftsman | internal/orchestration/recover.go; internal/orchestration/recover_test.go; internal/orchestration/checkpoint.go; internal/orchestration/checkpoint_test.go; internal/cmd/brain_run.go | T06,T11 | go test ./internal/orchestration ./internal/cmd -run 'Test(Recover|Checkpoint|BrainResume)' | crash before/after mission/claim/report/CAS converges; no duplicate mission/completion R4 |
| [x] T14 | craftsman | internal/orchestration/cancel.go; internal/orchestration/cancel_test.go; internal/cmd/brain_run.go; internal/cmd/brain_cancel_test.go | T10,T13 | go test ./internal/orchestration ./internal/cmd -run 'Test(Cancel|BrainCancel|Lease)' | cancel/revoke/expire ack and retry/escalation policy; later report always refused R3,R4 |
| [x] T15 | craftsman | internal/orchestration/conflict.go; internal/orchestration/conflict_test.go; internal/core/frontier.go; internal/core/frontier_test.go; internal/orchestration/worker.go | T08,T14 | go test ./internal/orchestration ./internal/core -run 'Test(Conflict|Frontier|Worker)' | overlapping write scopes no parallel lease absent pinned coordination rule R4 |
| [x] T16 | craftsman | internal/integration/orchestration_conformance_test.go; internal/cmd/e2e_test.go; internal/cmd/brain_claim_test.go; internal/cmd/brain_report_test.go | T12,T13,T14,T15 | go test ./internal/integration ./internal/cmd -run 'Test(OrchestrationConformance|LifecycleE2E|Brain)' | fake host pending→claim→heartbeat→verify→report→complete/resume fixture; exactly-once mission, safe retry R1-R4 |

> **W3 deviations.** T14 changes cancellation from deleting leases to retaining typed revoked
> leases, so the pre-W3 lifecycle assertion in `internal/cmd/brain_lifecycle_test.go` must be
> updated to assert retained revocation; deletion would let a stale report lose its causal refusal.
> T15 must edit `internal/cmd/brain_claim.go` to enforce its conflict decision at the public claim
> boundary; a pure helper without this integration would leave overlapping leases possible.
> T16 conformance exposed harness-owned `.specd/` ledger/session writes in worker scope. It reuses
> T12's declared `internal/core/diff.go` and test to exclude harness metadata from subject changes.

## W4 — routing, limits, observation

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T17 | craftsman | internal/core/config.go; internal/core/config_validate.go; internal/core/config_test.go; internal/core/embed_templates/project.yml | T04 | go test ./internal/core -run 'TestConfig' | versioned routing/limits policy parse/validation; safe defaults preserve current behavior R5 |
| [ ] T18 | craftsman | internal/core/tasksparser.go; internal/core/tasksparser_test.go; internal/orchestration/routing.go; internal/orchestration/routing_test.go | T17 | go test ./internal/core ./internal/orchestration -run 'Test(Tasks|Routing)' | risk/complexity/capability metadata byte-stable; deterministic eligible class/reason/fallback R5 |
| [ ] T19 | craftsman | internal/orchestration/brakes.go; internal/orchestration/brakes_test.go; internal/orchestration/decide.go; internal/orchestration/decide_test.go; internal/cmd/brain_run.go | T17,T18 | go test ./internal/orchestration ./internal/cmd -run 'Test(Brakes|Decide|Brain)' | config budget/deadline/retry/unknown telemetry brakes before dispatch/escalates R5 |
| [ ] T20 | craftsman | internal/orchestration/telemetry.go; internal/orchestration/telemetry_test.go; internal/orchestration/acp.go; internal/orchestration/acp_test.go; internal/cmd/report.go | T11,T19 | go test ./internal/orchestration ./internal/cmd -run 'Test(Telemetry|ACP|Report)' | unit/source/knownness + provider/model route facts bounded/redacted; facts not proof R5,R6 |

## W5 — adapters and release proof

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T21 | craftsman | internal/orchestration/a2a.go; internal/orchestration/a2a_test.go; docs/mcp-guide.md; docs/command-reference.md; docs/CHEATSHEET.md | T16,T20 | go test ./internal/orchestration -run 'TestA2A' && ./scripts/docs-lint.sh | versioned redacted mission/claim/heartbeat/cancel/report mapping; round trip required pins; unknown version fails R6 |
| [ ] T22 | craftsman | internal/integration/orchestration_conformance_test.go; internal/mcp/parity_test.go; internal/orchestration/a2a_test.go; docs/mcp-guide.md | T21 | go test ./internal/integration ./internal/mcp ./internal/orchestration -run 'Test(OrchestrationConformance|Parity|A2A)' | local CLI/MCP/A2A same semantic ACP fixture; declared transport diff only R6 |
| [ ] T23 | craftsman | scripts/regress-domains.sh; scripts/regress-lint.sh; internal/cmd/e2e_test.go; internal/integration/orchestration_conformance_test.go | T03,T16,T20,T22 | go test ./internal/cmd ./internal/integration -run 'Test(LifecycleE2E|OrchestrationConformance)' && ./scripts/regress-domains.sh && ./scripts/regress-lint.sh | fresh/no-adapter/role-race/stale-pin/revoke/budget/parallel/A2A regression proof |
| [ ] T24 | validator | specs/05-orchestration-multi-agent-and-model-routing; internal/core; internal/cmd; internal/orchestration; internal/integration; internal/mcp | T23 | go test ./... -race -count=1 && go vet ./... && ./scripts/test-lint.sh && ./scripts/docs-lint.sh && ./scripts/regress-all.sh && ./scripts/regress-domains.sh | full Domain 05 evidence |

## Cross-wave rules

- Add failing public-contract fixture before each lifecycle semantic change.
- Domain 03 owns generic dispatch envelope; Domain 05 specializes worker state machine without
  duplicate command/manifest policy.
- Domain 04 evidence and Domain 06 authority/scope must be validated by contracts, never mocked
  green at report/completion boundary.
- Adapter/provider failure cannot change core state except validated transport result event.
- Keep `reference/` untouched; `gofmt -l .` empty before release.
