# specd rebuild — progress & wave manager

> **Purpose.** Single source of truth for *where the rebuild is*. Stage 2 (spec authoring) is
> **complete** — all 12 domain specs exist under `specs/<NN-domain>/{spec.md,tasks.md}`. Stage
> 3 (implementation) runs the build waves **A–H** below. Check a task off only when its
> `verify:` command passes and a record is written (ADR-8 evidence integrity).
>
> **Sources:** `AGENTS.md` (pipeline), `fresh-start/00-roadmap.md` (DAG + waves),
> `fresh-start/00-decisions.md` (ADRs). **Guardrails:** determinism first; evidence integrity
> absolute; ADR-8 invariants preserved; subtractive bias.

---

## Stage tracker

| Stage | State | Notes |
|---|---|---|
| 1 — Domain analysis | ✅ complete | 12 `fresh-start/*.md` + roadmap/ADRs/triage |
| 2 — Spec authoring | ✅ complete | 12 × `spec.md` + `tasks.md` (this directory) |
| 3 — Implementation | ✅ complete | Build waves A–H complete |

### Authoring completeness (Stage 2 DoD)
All 12 meet the contract: (a) EARS-shaped testable requirements; (b) design names module
boundaries + on-disk contracts + preserved invariants; (c) `tasks.md` DAG with
`id/role/files/depends-on/verify/acceptance` grouped into waves; (d) verdicts cite a
`reference/` file + KEEP/SIMPLIFY/REDESIGN/CUT/DEFER.

| Spec | Title | spec.md | tasks.md | Author order |
|---|---|---|---|---|
| 01 | Product & Philosophy Core | ✅ | ✅ | 1 |
| 10 | CLI Architecture & Foundations | ✅ | ✅ | 2 |
| 02 | Spec Lifecycle & State Model | ✅ | ✅ | 3 |
| 04 | Task DAG & Wave Execution | ✅ | ✅ | 4 |
| 05 | Evidence & Verification | ✅ | ✅ | 5 |
| 03 | Validation Gates Engine | ✅ | ✅ | 6 |
| 08 | Context Engineering | ✅ | ✅ | 7 |
| 06 | Agent-Agnostic Integration | ✅ | ✅ | 8 |
| 07 | MCP & Handshake Surface | ✅ | ✅ | 9 |
| 09 | Orchestration (Brain/Pinky) | ✅ | ✅ | 10 |
| 11 | Reporting & Observability | ✅ | ✅ | 11 |
| 12 | Flywheel (triage tier) | ✅ | ✅ | 12 |

---

## Critical path
`01 → 10 → 02 → 05 → 03 → 08 → 09`. Orchestration (09) is last because it composes the most;
everything it needs must be green first. Reporting (11) and flywheel (12) can slip a wave
without blocking the core loop.

---

## Build waves (implementation)

Legend: ⬜ pending · 🟡 in progress · ✅ done (verify passed + record written). Waves gate on
each other only where a task's `depends-on` crosses waves; independent tasks within/across
domains run in parallel.

### Wave A — foundations (parallel) ✅
| task | domain | files | verify |
|---|---|---|---|
| ✅ T10.1 | 10 | `internal/core/io.go` | `go test ./internal/core -run TestAtomicWrite` |
| ✅ T10.2 | 10 | `internal/core/lock.go` | `go test ./internal/core -run TestReentrantLock` |
| ✅ T10.3 | 10 | `internal/core/{paths,slug}.go` | `go test ./internal/core -run 'TestFindRoot\|TestSlug'` |
| ✅ T10.4 | 10 | `internal/cli/args.go`, `main.go` | `go test ./internal/cli -run TestArgs` |
| ✅ T1.1 | 01 | `docs/charter.md` | `grep -q 'harness component' docs/charter.md` |
| ✅ T1.2 | 01 | `go.mod` | zero `require` deps |

### Wave B — state & primitives close-out ✅
| task | domain | files | verify |
|---|---|---|---|
| ✅ T10.5 | 10 | `registry.go`, `commands.go` | `go test ./internal/core -run TestRegistryMatchesHelp` |
| ✅ T10.6 | 10 | `config_loader.go`, `config_validate.go` | `go test ./internal/core -run TestConfigCascade` |
| ✅ T10.7 | 10 | `config_test.go` | `go test ./internal/core -run TestConfigNoLegacyJSON` |
| ✅ T2.1 | 02 | `state.go` | `go test ./internal/core -run TestStateCAS` |
| ✅ T2.2 | 02 | `io.go` | `go test ./internal/core -run TestAtomicWrite` |
| ✅ T2.3 | 02 | `phases.go` | `go test ./internal/core -run TestPhaseRatchet` |
| ✅ T1.3 | 01 | `main.go`, `args.go` | bare invocation lists 16 verbs |
| ✅ T1.4 | 01 | `commands_test.go` | `go test ./internal/core -run TestRegistryMatchesHelp` |

### Wave C — lifecycle & parser (parallel) ✅
| task | domain | files | verify |
|---|---|---|---|
| ✅ T2.4 | 02 | `new.go` | `go run . new demo && test -f .specd/specs/demo/state.json` |
| ✅ T2.5 | 02 | `approve.go`, `task_complete.go` | `go test ./internal/cmd -run TestApproveGates` |
| ✅ T2.6 | 02 | `status.go` | `go run . status demo --json \| grep '"mode":"simple"'` |
| ✅ T2.7 | 02 | `state_lock_test.go` | `go test ./internal/core -run TestSaveStateRequiresLock` |
| ✅ T4.1 | 04 | `tasksparser.go`, `md.go` | `go test ./internal/core -run TestTasksRoundTrip` |
| ✅ T4.2 | 04 | `tasksparser.go` | `go test ./internal/core -run TestSingleLineRewrite` |
| ✅ T4.3 | 04 | `dag.go` | `go test ./internal/core -run TestDAG` |
| ✅ T5.1 | 05 | `verify/exec.go`, `customgate.go` | `go test ./internal/core/verify -run TestScrubbedEnv` |
| ✅ T5.2 | 05 | `evidence/ledger.go` | `go test ./internal/core/evidence -run TestAppendOnly` |
| ✅ T5.3 | 05 | `verify/capture.go` | `go test ./internal/core/verify -run TestChangedFiles` |

### Wave D — gates, evidence integrity, dispatch ✅
| task | domain | files | verify |
|---|---|---|---|
| ✅ T3.1 | 03 | `gates/registry.go` | `go test ./internal/core/gates -run TestRegistryOrder` |
| ✅ T3.2 | 03 | `gates/core.go`, `ears.go`, `dag.go` | `go test ./internal/core/gates -run TestCoreGates` |
| ✅ T3.3 | 03 | `check.go` | `go run . check demo` |
| ✅ T5.4 | 05 | `task_complete.go` | `go test ./internal/core -run TestCompleteRequiresEvidence` |
| ✅ T5.5 | 05 | `verify.go` | `go run . verify demo T1` |
| ✅ T5.6 | 05 | `verify.go` | `go test ./internal/cmd -run TestRevertOnFail` |
| ✅ T5.7 | 05 | `verify/sandbox_test.go` | `go test ./internal/core/verify -run TestSandboxFailClosed` |
| ✅ T4.4 | 04 | `next.go`, `frontier.go` | `go run . next demo --json` |
| ✅ T4.5 | 04 | `next.go` | `go run . next demo --waves` |
| ✅ T4.6 | 04 | `tasksparser_fuzz_test.go` | `go test ./internal/core -run FuzzTasks -fuzztime=30s` |

### Wave E — context & integration (parallel) ✅
| task | domain | files | verify |
|---|---|---|---|
| ✅ T8.1 | 08 | `context/manifest.go` | `go test ./internal/context -run TestBuildManifest` |
| ✅ T8.2 | 08 | `context/estimate.go` | `go test ./internal/context -run TestEstimateNoLLM` |
| ✅ T8.3 | 08 | `core/pinky_context.go` | `go build ./... && go vet ./...` |
| ✅ T8.4 | 08 | `cmd/context.go`, `dispatch.go` | surfaces share the engine (diff) |
| ✅ T6.1 | 06 | `embed_templates/roles/*`, `scaffold.go` | `go run . init && [ $(ls .specd/roles\|wc -l) -eq 4 ]` |
| ✅ T6.2 | 06 | `embed_templates/steering/*` | `test -f .specd/steering/workflow.md` |
| ✅ T6.3 | 06 | `agents.go` | `go test ./internal/core -run TestAgentsMergePreservesUser` |
| ✅ T6.4 | 06 | `integration/registry.go` | `go test ./internal/integration -run TestSnippetFallback` |
| ✅ T6.5 | 06 | `integration/<host>.go` | `go test ./internal/integration -run TestAdapterConformance` |
| ✅ T6.6 | 06 | role-injection wiring | `go test ./internal/core -run TestRolePromptDedup` |
| ✅ T6.7 | 06 | `integration/conformance_test.go` | `go test ./internal/integration -run TestAdapterConformance` |
| ✅ T3.4 | 03 | `gates/security/*` | `go run . check demo --security` |
| ✅ T3.5 | 03 | `gates/contextbudget.go` | `go test ./internal/core/gates -run TestContextBudgetGate` |
| ✅ T3.6 | 03 | `gates/parity_test.go` | `go test ./internal/core/gates -run TestByteIdenticalWhenOptInsOff` |

### Wave F — surfaces ✅
| task | domain | files | verify |
|---|---|---|---|
| ✅ T7.1 | 07 | `mcp/server.go`, `cmd/mcp.go` | `echo '{...tools/list}' \| go run . mcp` |
| ✅ T7.2 | 07 | `mcp/tools_core.go` | `go test ./internal/mcp -run TestMCPParity` |
| ✅ T7.3 | 07 | `manifest_tools.go` | `go test ./internal/core -run TestForbiddenTool` |
| ✅ T7.4 | 07 | `handshake.go` (cmd+core) | `go run . handshake bootstrap --json \| grep version` |
| ✅ T7.5 | 07 | `mcp/tools_brain.go` | `go test ./internal/mcp -run TestBrainToolsGatedByConfig` |
| ✅ T7.6 | 07 | `mcp/parity_test.go` | `go test ./internal/mcp -run TestMCPParity` |
| ✅ T8.5 | 08 | `context/budget.go`, `gates/contextbudget.go` | `SPECD_MAX_CONTEXT_TOKENS=10 go run . check demo` |
| ✅ T8.6 | 08 | `docs/context.md` | `grep -q read-targeted docs/context.md` |
| ✅ T8.7 | 08 | `context/manifest_test.go` | `go test ./internal/context -run TestManifestValidate` |
| ✅ T11.1 | 11 | `core/report.go` | `go test ./internal/core -run TestReportModel` |
| ✅ T11.2 | 11 | `cmd/status.go` | `go run . status demo --json` |
| ✅ T11.3 | 11 | `prsummary.go`, `commitlink.go` | `go test ./internal/core -run TestPRSummaryGolden` |
| ✅ T11.4 | 11 | `report_metrics.go` | `go test ./internal/core -run TestMetricsGolden` |
| ✅ T11.5 | 11 | `report_purity_test.go` | `go test ./internal/core -run TestNoLLMInRender` |

### Wave G — orchestration tier ✅
| task | domain | files | verify |
|---|---|---|---|
| ✅ T9.1 | 09 | `orchestration/decide.go` | `go test ./internal/orchestration -run TestDecidePure` |
| ✅ T9.2 | 09 | `orchestration/sense.go` | `go test ./internal/orchestration -run TestSense` |
| ✅ T9.3 | 09 | `orchestration/brakes.go` | `go test ./internal/orchestration -run TestBrakes` |
| ✅ T9.4 | 09 | `orchestration/acp.go` | `go test ./internal/orchestration -run TestACPRoundtrip` |
| ✅ T9.5 | 09 | `orchestration/lease.go` | `go test ./internal/orchestration -run TestLeaseReclaim` |
| ✅ T9.6 | 09 | `orchestration/session.go` | `go test ./internal/orchestration -run TestSessionCAS` |
| ✅ T9.7 | 09 | `cmd/brain.go`, `orchestration/driver.go` | `go test ./internal/orchestration -run TestBrainDriverDispatchesFrontier` |
| ✅ T9.8 | 09 | `cmd/pinky.go`, `brain_worker.go` | `go test ./internal/cmd -run TestReportRequiresVerify` |
| ✅ T9.9 | 09 | orchestration authority config | `go test ./internal/orchestration -run TestFailClosedAuthority` |
| ✅ T9.10 | 09 | `orchestration/decide_test.go` | `go test ./internal/orchestration -run TestNoLLM` |

### Wave H — flywheel (minimal) ✅
| task | domain | files | verify |
|---|---|---|---|
| ✅ T12.1 | 12 | `gates/security/scanners.go` | `go test ./internal/core/gates/security -run TestScanners` |
| ✅ T12.2 | 12 | `gates/security/allow.go` | `go test ./... -run TestAllowlistReasonRequired` |
| ✅ T12.3 | 12 | `docs/deferred-flywheel.md` | `grep -q DeployApproval docs/deferred-flywheel.md` |
| ✅ T12.4 | 12 | `gates/security/off_test.go` | `go test -run TestSecurityOffByDefault` |

---

## Progress rollup

| Wave | Tasks | Done | State |
|---|---|---|---|
| A | 6 | 6 | ✅ |
| B | 8 | 8 | ✅ |
| C | 10 | 10 | ✅ |
| D | 10 | 10 | ✅ |
| E | 14 | 14 | ✅ |
| F | 14 | 14 | ✅ |
| G | 10 | 10 | ✅ |
| H | 4 | 4 | ✅ |
| **Total** | **76** | **76** | **100%** |

---

## Definition of done (Stage 3, per task — ADR-8)
- [ ] Its `verify:` command passes and the record is written (exit code + git HEAD).
- [ ] It touches only the `files:` its task declares.
- [ ] Guardrails hold: determinism (no LLM in decision/gate/render path), zero deps, ADR-8
      invariants (atomic write, CAS, reentrant lock, `ParseTasks` round-trip, `go:embed`,
      evidence integrity).

## How to update this file
When a task's verify passes: flip its ⬜ to ✅ in its wave table, bump the wave's Done count and
the rollup total. A wave is ✅ when all its tasks are ✅ **and** no cross-wave `depends-on` is
still ⬜. Keep this file the projection of reality — never mark ahead of evidence.
