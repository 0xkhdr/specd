# Tasks — Domain 03 Driver DAG

`[ ]` pending. Execute wave only after dependencies pass. Files declared scope; record deviation
before edit. Cross-domain prerequisites remain README program links, not local task IDs.

## W0 — contract baseline

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T01 | scout | docs/google-sdlc-alignment/README.md; docs/google-sdlc-alignment/03-agent-tool-driving-and-native-guidance.md; specs/03-agent-tool-driving-and-native-guidance | | printf ok | map R1-R8 to current command/context/MCP surfaces and Domain 01/02/04/06 boundaries |
| [x] T02 | craftsman | internal/core/driver.go; internal/core/driver_test.go; internal/core/commands.go; internal/core/handshake.go; internal/core/handshake_test.go | T01 | `go test ./internal/core -run 'Test(Driver|Handshake|Command)'` | versioned Bootstrap/Guide/Finding model, canonical ordering/digest R1 |
| [x] T03 | craftsman | internal/cmd/integration_polish_test.go; internal/cmd/lifecycle_test.go; internal/mcp/handshake_test.go; internal/mcp/parity_test.go | T01 | `go test ./internal/cmd ./internal/mcp -run 'Test(Integration|Lifecycle|Handshake|Parity)'` | failing fresh-fixture path/example/pin/ambiguity/handoff baseline R3-R8 |

> **W0 deviations.** T01 inventory maps R1→core driver, R2→Domain 02 manifest,
> R3→scaffold, R4→dispatch/MCP config, R5→doctor, R6→guidance/palette,
> R7→handshake, and R8→CLI/MCP/orchestration. T02 reused existing command and handshake
> contracts, so `commands.go` and `handshake.go` needed no W0 edit. T03 baselines fit in
> `integration_polish_test.go`, `mcp/handshake_test.go`, and `mcp/parity_test.go`; declared
> lifecycle test files were not needed until later implementation waves.

## W1 — truthful paths, scaffold, resolution, doctor

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T04 | craftsman | internal/context/manifest.go; internal/context/manifest_test.go; internal/context/steering_manifest_test.go | T02,T03 | `go test ./internal/context -run 'Test(BuildManifest|SteeringInManifest)'` | `.specd/specs` canonical emitted paths; unresolved required item fails R2.1,R2.3 |
| [x] T05 | craftsman | internal/context/manifest.go; internal/context/manifest_test.go; internal/core/tasksparser.go; internal/core/tasksparser_test.go | T04 | `go test ./internal/context ./internal/core -run 'Test(Manifest|Tasks)'` | design/task/declared-file guidance fields consume Domain 02-compatible metadata R2.2 |
| [x] T06 | craftsman | internal/core/embed_templates/AGENTS.md; internal/core/embed_templates/roles; internal/core/scaffold.go; internal/core/scaffold_test.go; internal/cmd/init_scaffold_test.go | T03 | `go test ./internal/core ./internal/cmd -run 'Test(Scaffold|InitScaffold)'` | every generated unmarked command runnable or explicit placeholder R3.1,R3.2 |
| [x] T07 | craftsman | internal/core/specresolver.go; internal/core/specresolver_test.go; internal/cmd/dispatch.go; internal/cmd/dispatch_test.go; internal/core/mcpconfig.go; internal/core/mcpconfig_test.go | T02,T03 | `go test ./internal/core ./internal/cmd -run 'Test(SpecResolver|Dispatch|MCPConfig)'` | explicit/pinned/single resolution; ambiguity refusal; no inert `SPECD_SPEC` R4 |
| [x] T08 | craftsman | internal/core/doctor.go; internal/core/doctor_test.go; internal/cmd/registry.go; internal/cmd/registry_test.go; internal/cmd/integration_polish_test.go | T04,T06,T07 | `go test ./internal/core ./internal/cmd -run 'Test(Doctor|Registry|Integration)'` | read-only agent doctor codes/fixes; fresh bad fixtures never false-pass R5 |

> **W1 deviations.** T08 also required `internal/core/commands.go`,
> `docs/command-reference.md`, and `docs/CHEATSHEET.md`: exposing `agents doctor` changes the
> existing command usage contract and docs-lint requires both operator references synchronized.
> T05 reused Domain 02 `DeclaredFiles`, so no parser edit was needed. T06 only changed the
> defective memory command placeholder and its scaffold test; other declared scaffold/role files
> already conformed. T07 removed inert MCP pin emission and added the pure resolver; dispatch
> remains explicit-operand-only, so its declared files needed no edit. V1 path fields remain
> additive-compatible; strict missing-required refusal stays on authoritative V2.

## W2 — driver projection

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T09 | craftsman | internal/core/driver.go; internal/core/driver_test.go; internal/core/commands.go; internal/core/commandmeta_test.go; internal/core/gates/registry.go | T02,T07 | `go test ./internal/core ./internal/core/gates -run 'Test(Driver|Command|Registry)'` | phase/frontier/blocker projection; parser-valid actions R6.1,R6.2 |
| [x] T10 | craftsman | internal/cmd/registry.go; internal/cmd/registry_test.go; internal/cmd/lifecycle.go; internal/cmd/lifecycle_test.go; internal/cmd/dispatch.go | T08,T09 | `go test ./internal/cmd -run 'Test(Registry|Lifecycle|Dispatch)'` | guide/doctor CLI JSON; human-only action never agent-authorized R5,R6.3 |
| [x] T11 | craftsman | internal/mcp/server.go; internal/mcp/tools_core.go; internal/mcp/parity_test.go; docs/mcp-guide.md | T10 | `go test ./internal/mcp -run 'Test(Parity|Initialize)'` | MCP guide/doctor parity and documented route R6,R8.1 |

> **W2 deviations.** T09 reused command and gate metadata without editing
> `commandmeta_test.go` or `gates/registry.go`. T10 added `internal/core/commands.go` plus synced
> command docs for the additive `agents guide` usage; existing lifecycle/dispatch code needed no
> change. T11 also updated `internal/mcp/handshake_test.go` to pin advertised driver protocol;
> canonical `tools_core.go` already projected the `agents` positional route.

## W3 — drift, context metadata, handoff

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T12 | craftsman | internal/core/handshake.go; internal/core/handshake_test.go; internal/core/scaffold.go; internal/core/scaffold_test.go; internal/context/manifest.go | T05,T06,T09 | `go test ./internal/core ./internal/context -run 'Test(Handshake|Scaffold|Manifest)'` | isolated guidance/context-schema digests; managed drift detected R3.3,R7.1 |
| [x] T13 | craftsman | internal/context/manifest.go; internal/context/manifest_test.go; internal/context/hud.go; internal/context/hud_test.go | T05,T12 | `go test ./internal/context -run 'Test(Manifest|HUD)'` | path/reason/priority/digest/required status byte-stable; missing required fail R2,R7 |
| [x] T14 | craftsman | internal/mcp/server.go; internal/mcp/policy_test.go; internal/mcp/parity_test.go; internal/cmd/dispatch.go | T11,T12 | `go test ./internal/mcp ./internal/cmd -run 'Test(Policy|Parity|Dispatch)'` | typed `MCP_HANDOFF_REQUIRED`; exact actor/CLI route; no side effect R7.2,R7.3 |

## W4 — host conformance and capabilities

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T15 | craftsman | internal/integration/driver_conformance_test.go; internal/integration/registry.go; internal/integration/snippet.go; internal/cmd/e2e_test.go; internal/mcp/parity_test.go | T10,T11,T14 | `go test ./internal/integration ./internal/cmd ./internal/mcp -run 'Test(DriverConformance|LifecycleE2E|Parity)'` | CLI/MCP/future-host lifecycle fixture equivalent R8.1 |
| [x] T16 | craftsman | internal/core/capabilities.go; internal/core/capabilities_test.go; internal/mcp/server.go; internal/mcp/policy_test.go; docs/mcp-guide.md | T15 | `go test ./internal/core ./internal/mcp -run 'Test(Capabilities|Policy)'` | deterministic supported/downgrade/refusal for host declarations R8.2 |
| [x] T17 | craftsman | docs/command-reference.md; docs/CHEATSHEET.md; docs/mcp-guide.md; docs/contributor-guide.md; internal/core/embed_templates/AGENTS.md | T13,T16 | `./scripts/docs-lint.sh && go test ./internal/cmd -run 'Test(Registry|InitScaffold)'` | operator/host migration and bootstrap docs synchronized |

## W5 — remote envelope, release proof

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T18 | craftsman | internal/orchestration/dispatch_envelope.go; internal/orchestration/dispatch_envelope_test.go; internal/core/driver.go; internal/core/driver_test.go | T15,T16 | `go test ./internal/orchestration ./internal/core -run 'Test(DispatchEnvelope|Driver)'` | pinned task/role/files/context/config/palette/HEAD envelope R8.3 |
| [x] T19 | craftsman | internal/orchestration/acp.go; internal/orchestration/acp_test.go; internal/orchestration/lease.go; internal/orchestration/lease_test.go | T18 | `go test ./internal/orchestration -run 'Test(ACP|Lease|DispatchEnvelope)'` | stale changed envelope claim/report rejected; no completion authority added R8.3 |
| [x] T20 | craftsman | scripts/regress-domains.sh; scripts/regress-lint.sh; internal/cmd/e2e_test.go; internal/mcp/parity_test.go; internal/integration/driver_conformance_test.go | T17,T19 | `go test ./internal/cmd ./internal/mcp ./internal/integration -run 'Test(LifecycleE2E|Parity|DriverConformance)' && ./scripts/regress-domains.sh && ./scripts/regress-lint.sh` | fresh/multi-spec/stale/missing/forbidden conformance release proof |
| [x] T21 | validator | specs/03-agent-tool-driving-and-native-guidance; internal/core; internal/context; internal/cmd; internal/mcp; internal/integration; internal/orchestration | T20 | go test ./... -race -count=1 && go vet ./... && ./scripts/test-lint.sh && ./scripts/docs-lint.sh && ./scripts/regress-all.sh && ./scripts/regress-domains.sh | full Domain 03 evidence |

## Cross-wave rules

- Test public contract failure first. Keep plain output compatibility until migration test passes.
- Domain 02 owns context selection/budget; Domain 03 may not weaken required-context failure.
- Do not declare remote dispatch complete before Domain 05/06 worker/authority contracts evidence.
- Keep `reference/` untouched; `gofmt -l .` empty before release.
