# Tasks — Domain 03 Driver DAG

`[ ]` pending. Execute wave only after dependencies pass. Files declared scope; record deviation
before edit. Cross-domain prerequisites remain README program links, not local task IDs.

## W0 — contract baseline

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T01 | scout | docs/google-sdlc-alignment/README.md; docs/google-sdlc-alignment/03-agent-tool-driving-and-native-guidance.md; specs/03-agent-tool-driving-and-native-guidance | | printf ok | map R1-R8 to current command/context/MCP surfaces and Domain 01/02/04/06 boundaries |
| [ ] T02 | craftsman | internal/core/driver.go; internal/core/driver_test.go; internal/core/commands.go; internal/core/handshake.go; internal/core/handshake_test.go | T01 | go test ./internal/core -run 'Test(Driver|Handshake|Command)' | versioned Bootstrap/Guide/Finding model, canonical ordering/digest R1 |
| [ ] T03 | craftsman | internal/cmd/integration_polish_test.go; internal/cmd/lifecycle_test.go; internal/mcp/handshake_test.go; internal/mcp/parity_test.go | T01 | go test ./internal/cmd ./internal/mcp -run 'Test(Integration|Lifecycle|Handshake|Parity)' | failing fresh-fixture path/example/pin/ambiguity/handoff baseline R3-R8 |

## W1 — truthful paths, scaffold, resolution, doctor

| id | role | files | depends-on | verify | acceptance |
| [ ] T04 | craftsman | internal/context/manifest.go; internal/context/manifest_test.go; internal/context/steering_manifest_test.go | T02,T03 | go test ./internal/context -run 'Test(BuildManifest|SteeringInManifest)' | `.specd/specs` canonical emitted paths; unresolved required item fails R2.1,R2.3 |
| [ ] T05 | craftsman | internal/context/manifest.go; internal/context/manifest_test.go; internal/core/tasksparser.go; internal/core/tasksparser_test.go | T04 | go test ./internal/context ./internal/core -run 'Test(Manifest|Tasks)' | design/task/declared-file guidance fields consume Domain 02-compatible metadata R2.2 |
| [ ] T06 | craftsman | internal/core/embed_templates/AGENTS.md; internal/core/embed_templates/roles; internal/core/scaffold.go; internal/core/scaffold_test.go; internal/cmd/init_scaffold_test.go | T03 | go test ./internal/core ./internal/cmd -run 'Test(Scaffold|InitScaffold)' | every generated unmarked command runnable or explicit placeholder R3.1,R3.2 |
| [ ] T07 | craftsman | internal/core/specresolver.go; internal/core/specresolver_test.go; internal/cmd/dispatch.go; internal/cmd/dispatch_test.go; internal/core/mcpconfig.go; internal/core/mcpconfig_test.go | T02,T03 | go test ./internal/core ./internal/cmd -run 'Test(SpecResolver|Dispatch|MCPConfig)' | explicit/pinned/single resolution; ambiguity refusal; no inert `SPECD_SPEC` R4 |
| [ ] T08 | craftsman | internal/core/doctor.go; internal/core/doctor_test.go; internal/cmd/registry.go; internal/cmd/registry_test.go; internal/cmd/integration_polish_test.go | T04,T06,T07 | go test ./internal/core ./internal/cmd -run 'Test(Doctor|Registry|Integration)' | read-only agent doctor codes/fixes; fresh bad fixtures never false-pass R5 |

## W2 — driver projection

| id | role | files | depends-on | verify | acceptance |
| [ ] T09 | craftsman | internal/core/driver.go; internal/core/driver_test.go; internal/core/commands.go; internal/core/commandmeta_test.go; internal/core/gates/registry.go | T02,T07 | go test ./internal/core ./internal/core/gates -run 'Test(Driver|Command|Registry)' | phase/frontier/blocker projection; parser-valid actions R6.1,R6.2 |
| [ ] T10 | craftsman | internal/cmd/registry.go; internal/cmd/registry_test.go; internal/cmd/lifecycle.go; internal/cmd/lifecycle_test.go; internal/cmd/dispatch.go | T08,T09 | go test ./internal/cmd -run 'Test(Registry|Lifecycle|Dispatch)' | guide/doctor CLI JSON; human-only action never agent-authorized R5,R6.3 |
| [ ] T11 | craftsman | internal/mcp/server.go; internal/mcp/tools_core.go; internal/mcp/parity_test.go; docs/mcp-guide.md | T10 | go test ./internal/mcp -run 'Test(Parity|Initialize)' | MCP guide/doctor parity and documented route R6,R8.1 |

## W3 — drift, context metadata, handoff

| id | role | files | depends-on | verify | acceptance |
| [ ] T12 | craftsman | internal/core/handshake.go; internal/core/handshake_test.go; internal/core/scaffold.go; internal/core/scaffold_test.go; internal/context/manifest.go | T05,T06,T09 | go test ./internal/core ./internal/context -run 'Test(Handshake|Scaffold|Manifest)' | isolated guidance/context-schema digests; managed drift detected R3.3,R7.1 |
| [ ] T13 | craftsman | internal/context/manifest.go; internal/context/manifest_test.go; internal/context/hud.go; internal/context/hud_test.go | T05,T12 | go test ./internal/context -run 'Test(Manifest|HUD)' | path/reason/priority/digest/required status byte-stable; missing required fail R2,R7 |
| [ ] T14 | craftsman | internal/mcp/server.go; internal/mcp/policy_test.go; internal/mcp/parity_test.go; internal/cmd/dispatch.go | T11,T12 | go test ./internal/mcp ./internal/cmd -run 'Test(Policy|Parity|Dispatch)' | typed `MCP_HANDOFF_REQUIRED`; exact actor/CLI route; no side effect R7.2,R7.3 |

## W4 — host conformance and capabilities

| id | role | files | depends-on | verify | acceptance |
| [ ] T15 | craftsman | internal/integration/driver_conformance_test.go; internal/integration/registry.go; internal/integration/snippet.go; internal/cmd/e2e_test.go; internal/mcp/parity_test.go | T10,T11,T14 | go test ./internal/integration ./internal/cmd ./internal/mcp -run 'Test(DriverConformance|LifecycleE2E|Parity)' | CLI/MCP/future-host lifecycle fixture equivalent R8.1 |
| [ ] T16 | craftsman | internal/core/capabilities.go; internal/core/capabilities_test.go; internal/mcp/server.go; internal/mcp/policy_test.go; docs/mcp-guide.md | T15 | go test ./internal/core ./internal/mcp -run 'Test(Capabilities|Policy)' | deterministic supported/downgrade/refusal for host declarations R8.2 |
| [ ] T17 | craftsman | docs/command-reference.md; docs/CHEATSHEET.md; docs/mcp-guide.md; docs/contributor-guide.md; internal/core/embed_templates/AGENTS.md | T13,T16 | ./scripts/docs-lint.sh && go test ./internal/cmd -run 'Test(Registry|InitScaffold)' | operator/host migration and bootstrap docs synchronized |

## W5 — remote envelope, release proof

| id | role | files | depends-on | verify | acceptance |
| [ ] T18 | craftsman | internal/orchestration/dispatch_envelope.go; internal/orchestration/dispatch_envelope_test.go; internal/core/driver.go; internal/core/driver_test.go | T15,T16 | go test ./internal/orchestration ./internal/core -run 'Test(DispatchEnvelope|Driver)' | pinned task/role/files/context/config/palette/HEAD envelope R8.3 |
| [ ] T19 | craftsman | internal/orchestration/acp.go; internal/orchestration/acp_test.go; internal/orchestration/lease.go; internal/orchestration/lease_test.go | T18 | go test ./internal/orchestration -run 'Test(ACP|Lease|DispatchEnvelope)' | stale changed envelope claim/report rejected; no completion authority added R8.3 |
| [ ] T20 | craftsman | scripts/regress-domains.sh; scripts/regress-lint.sh; internal/cmd/e2e_test.go; internal/mcp/parity_test.go; internal/integration/driver_conformance_test.go | T17,T19 | go test ./internal/cmd ./internal/mcp ./internal/integration -run 'Test(LifecycleE2E|Parity|DriverConformance)' && ./scripts/regress-domains.sh && ./scripts/regress-lint.sh | fresh/multi-spec/stale/missing/forbidden conformance release proof |
| [ ] T21 | validator | specs/03-agent-tool-driving-and-native-guidance; internal/core; internal/context; internal/cmd; internal/mcp; internal/integration; internal/orchestration | T20 | go test ./... -race -count=1 && go vet ./... && ./scripts/test-lint.sh && ./scripts/docs-lint.sh && ./scripts/regress-all.sh && ./scripts/regress-domains.sh | full Domain 03 evidence |

## Cross-wave rules

- Test public contract failure first. Keep plain output compatibility until migration test passes.
- Domain 02 owns context selection/budget; Domain 03 may not weaken required-context failure.
- Do not declare remote dispatch complete before Domain 05/06 worker/authority contracts evidence.
- Keep `reference/` untouched; `gofmt -l .` empty before release.
