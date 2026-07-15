# Tasks — Workflow coherence

`[ ]` pending. Implement exactly one wave per turn. Tests first. Mark only after wave verification.

## W0 — Baseline and contract

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T01 | scout | docs/sdlc-paper-current-reference-analysis.md; specs/11-workflow-coherence | - | printf ok | map F1-F9 to R1-R8 and current code/tests |
| [ ] T02 | craftsman | internal/cmd/workflow_coherence_test.go; internal/core/phases_test.go; internal/core/manifest_tools_test.go; internal/cmd/init_scaffold_test.go | T01 | `go test ./internal/core ./internal/cmd -run 'TestWorkflowCoherenceBaseline|TestPhaseRatchet|TestManifestTool'` | characterize skip, wrong effect, incomplete agent loop, missing skills/stub gaps |
| [ ] T03 | auditor | specs/11-workflow-coherence/requirements.md; specs/11-workflow-coherence/design.md; specs/11-workflow-coherence/tasks.md | T02 | printf ok | confirm every report finding and release scenario has task coverage |

## W1 — Exact lifecycle and approval

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T04 | craftsman | internal/core/phases.go; internal/core/phases_test.go | T02 | `go test ./internal/core -run 'TestPhase|TestAdvance'` | R1 exact successor matrix; same/skip/backward/unknown fail |
| [ ] T05 | craftsman | internal/core/commands.go; internal/cmd/lifecycle.go; internal/cmd/lifecycle_test.go; internal/cmd/registry.go | T04 | `go test ./internal/core ./internal/cmd -run 'TestApprove|TestCommand'` | R2 simple one-step approval; separate mode/exception operations |
| [ ] T06 | craftsman | internal/mcp; internal/core/driver.go; internal/core/driver_test.go; internal/cmd/integration_polish_test.go | T05 | `go test ./internal/core ./internal/cmd ./internal/mcp -run 'TestApprove|TestDriver|TestIntegration'` | R2 human handoff and next action parity |

## W2 — Canonical operation effects

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T07 | craftsman | internal/core/commands.go; internal/core/commandmeta_test.go; internal/core/manifest_tools.go; internal/core/manifest_tools_test.go | T06 | `go test ./internal/core -run 'TestCommand|TestOperation|TestManifestTool'` | R3 versioned per-operation actor/effect/authority schema |
| [ ] T08 | craftsman | internal/cmd/registry.go; internal/cmd/registry_test.go; internal/mcp; internal/core/handshake.go; internal/core/handshake_test.go | T07 | `go test ./internal/core ./internal/cmd ./internal/mcp -run 'TestOperation|TestRegistry|TestHandshake|TestParity'` | R3 all renderers derive same operations; mutation never read |
| [ ] T09 | validator | internal/core; internal/cmd; internal/mcp | T08 | `go test ./internal/core ./internal/cmd ./internal/mcp -run 'Test.*Operation|Test.*Effect|Test.*Parity' -count=2` | mixed subcommands and forbidden/human operations fail closed |

## W3 — Executable task completion loop

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T10 | craftsman | internal/core/commands.go; internal/core/authority.go; internal/core/authority_test.go; internal/core/driver.go; internal/core/driver_test.go | T08 | `go test ./internal/core -run 'TestAuthority|TestDriver|TestComplete'` | R4 narrow complete-task operation authorized after verify |
| [ ] T11 | craftsman | internal/cmd/lifecycle.go; internal/cmd/lifecycle_test.go; internal/cmd/registry.go; internal/cmd/e2e_test.go | T10 | `go test ./internal/cmd -run 'TestTaskComplete|TestLifecycle|TestWorkflow'` | R4 verify records only; completion enforces all current gates and CAS |
| [ ] T12 | craftsman | internal/mcp; internal/orchestration; internal/integration/driver_conformance_test.go | T11 | `go test ./internal/mcp ./internal/orchestration ./internal/integration -run 'Test.*Complete|TestDriverConformance'` | R4 CLI/MCP/orchestrated completion semantics equivalent |

## W4 — Progressive skills and templates

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T13 | craftsman | internal/core/embed_templates/skills; internal/core/embed_templates/templates.go; internal/core/managed.go; internal/core/scaffold.go; internal/core/scaffold_test.go; internal/context/skills_test.go | T12 | `go test ./internal/core ./internal/context -run 'TestScaffold|TestSkills|TestManaged'` | R5 shipped current-schema lazy skill pack |
| [ ] T14 | craftsman | internal/cmd/lifecycle.go; internal/cmd/lifecycle_test.go; internal/core/embed_templates/steering; internal/core/scaffold_test.go | T13 | `go test ./internal/core ./internal/cmd -run 'TestScaffold|TestStub|TestLifecycle'` | R6 production-shaped requirements/design/tasks; no fake task |
| [ ] T15 | craftsman | internal/core/embed_templates/AGENTS.md; internal/core/embed_templates/roles; internal/cmd/init_scaffold_test.go; internal/context/manifest_test.go | T14 | `go test ./internal/context ./internal/cmd -run 'TestManifest|TestInitScaffold|TestManagedCommand'` | R5-R6 small static guide; applicable skill selected; commands executable |

## W5 — Documentation, diagnostics, and rollup truth

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T16 | craftsman | internal/core/doctor.go; internal/core/doctor_test.go; internal/cmd/agents.go; internal/cmd/agents_test.go | T15 | `go test ./internal/core ./internal/cmd -run 'TestDoctor|TestAgents'` | R7 typed healthy empty result and defective findings |
| [ ] T17 | craftsman | scripts/regress-domains.sh; scripts/regress-lint.sh; specs/progress.md; specs/08-deployment-and-production-assurance/tasks.md; specs/09-maintenance-modernization-and-operating-model/tasks.md | T16 | `./scripts/regress-domains.sh && ./scripts/regress-lint.sh` | R7 rollup/task equality; existing W0 drift repaired |
| [ ] T18 | craftsman | README.md; docs/README.md; docs/user-guide.md; docs/concepts.md; docs/agent-integration.md; docs/command-reference.md; docs/CHEATSHEET.md; docs/google-sdlc-alignment; sdlc-with-vibe-coding.md | T17 | `./scripts/docs-lint.sh && go test ./internal/cmd -run 'Test.*Example|TestIntegration'` | R2,R7 normative commands correct; historical analyses labeled |

## W6 — Fresh-project release proof

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T19 | craftsman | internal/cmd/e2e_test.go; internal/integration/driver_conformance_test.go; scripts/regress-domains.sh | T18 | `go test ./internal/cmd ./internal/integration -run 'TestWorkflowCoherenceDefault|TestDriverConformance'` | R8 fresh default workflow driven only by generated surfaces |
| [ ] T20 | craftsman | internal/integration/production_smoke_test.go; internal/integration/security_conformance_test.go; internal/integration/orchestration_conformance_test.go | T19 | `go test ./internal/integration -run 'TestWorkflowCoherenceProduction|Test.*Conformance'` | R8 production authority/scope/sandbox/security/quality/review proof |
| [ ] T21 | validator | SPEC.md; specs/11-workflow-coherence; internal; docs; scripts | T20 | `go test ./... -race -count=1 && go test ./... -count=2 && go vet ./... && ./scripts/test-lint.sh && ./scripts/docs-lint.sh && ./scripts/regress-lint.sh && ./scripts/regress-domains.sh && ./scripts/regress-all.sh` | R1-R8 full release evidence; zero runtime deps; reference untouched |

## Cross-wave rules

- If required file is absent from row, record deviation here before editing.
- CLI/flag change updates command reference and CHEATSHEET together.
- New public contract starts RED and ends with black-box/conformance proof.
- One wave only per turn; do not start next wave after current turns green.
