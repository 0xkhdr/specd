# Tasks — specd Init and Agent Discovery

Source: `spec.md`  
Execution rule: complete waves in dependency order. Tasks inside same wave may run
in parallel when file ownership does not overlap.

## Wave 1 — Correctness and contracts

- [x] T1 — Add failing init-write regression coverage
  - why: Current init can print write errors then return success; lock expected behavior before refactor.
  - role: builder
  - files: internal/cmd/init_test.go, internal/cmd/commands_test.go, internal/testharness/
  - contract: Tests inject required scaffold write failures and assert exit 1, failed path reporting, and no ready claim.
  - acceptance: Tests fail against current implementation and cover human plus JSON modes.
  - verify: go test ./internal/cmd/... -run 'Init.*(Failure|Write|JSON)' -count=1
  - depends: —
  - requirements: R1.1, R5.1

- [x] T2 — Define versioned init result and plan models
  - why: Reliable dry-run, JSON output, doctor, and host setup need one deterministic contract.
  - role: builder
  - files: internal/core/initplan.go, internal/core/initplan_test.go
  - contract: Add InitOptions, InitAction, InitPlan, InitResult, file result, agent result, verification result, warning, and next-action types with non-null sorted slices.
  - acceptance: Marshaled result is deterministic, schemaVersion is 1, arrays never encode null, and planner performs no writes.
  - verify: go test ./internal/core/... -run 'InitPlan|InitResult' -count=2
  - depends: —
  - requirements: R1.2, R5.2, R5.4

- [x] T3 — Replace init file arrays with scaffold manifest
  - why: One manifest must drive init, repair, refresh, doctor, and template parity.
  - role: builder
  - files: internal/core/scaffold.go, internal/core/scaffold_test.go, internal/cmd/init.go
  - contract: Manifest declares template, target, policy, and required state for every steering, role, skill, config, and AGENTS.md asset.
  - acceptance: Existing default scaffold remains byte-compatible; missing template or duplicate target fails tests.
  - verify: go test ./internal/core/... ./internal/cmd/... -run 'Scaffold|InitSkillTemplates|InitDefaultRegression' -count=1
  - depends: T2
  - requirements: R1.3, R1.4, R1.5

- [x] T4 — Implement fail-closed init executor
  - why: Init must never report readiness after partial required failure.
  - role: builder
  - files: internal/core/initplan.go, internal/cmd/init.go, internal/cmd/init_test.go
  - contract: Preflight templates/targets first, execute deterministic actions, collect failures, return exit 1 on any required failure, and emit one InitResult.
  - acceptance: Fresh init succeeds; injected failure is non-zero; healthy rerun is byte-stable; MCP invocation receives valid JSON.
  - verify: go test ./internal/cmd/... ./internal/core/... -run 'Init' -count=2
  - depends: T1, T2, T3
  - requirements: R1.1, R1.2, R1.3, R5.1, R5.2

## Wave 2 — Safe lifecycle modes

- [x] T5 — Add dry-run, repair, and refresh modes
  - why: Users need safe preview and recovery without destructive broad force.
  - role: builder
  - files: internal/cli/args.go, internal/core/commands.go, internal/core/initplan.go, internal/cmd/init.go, internal/cmd/init_test.go
  - contract: `--dry-run` writes nothing; `--repair` restores missing assets only; `--refresh` updates managed assets/marker sections only; conflicts return usage exit 2.
  - acceptance: User-authored steering and AGENTS.md outside markers survive repair/refresh; dry-run lists exact actions.
  - verify: go test ./internal/cli/... ./internal/cmd/... ./internal/core/... -run 'Init.*(DryRun|Repair|Refresh|Conflict)' -count=1
  - depends: T4
  - requirements: R1.4, R1.5, R5.4

- [x] T6 — Harden AGENTS.md merge and force semantics
  - why: Existing `--force` resets full AGENTS.md and can destroy user content.
  - role: builder
  - files: internal/core/agents.go, internal/core/agents_test.go, internal/cmd/init.go
  - contract: Refresh replaces managed marker body only; malformed/duplicate markers fail safely; destructive full reset requires explicit force path and warning.
  - acceptance: Preamble/postamble preserved by normal, repair, and refresh modes; malformed markers never produce duplicated managed instructions silently.
  - verify: go test ./internal/core/... ./internal/cmd/... -run 'MergeAgents|Init.*Agents' -count=1
  - depends: T5
  - requirements: R1.4, R1.5

- [x] T7 — Add first-init staging and rollback behavior
  - why: Per-file atomic writes do not make full initialization atomic.
  - role: builder
  - files: internal/core/initplan.go, internal/core/io.go, internal/core/initplan_test.go, internal/cmd/init_test.go
  - contract: Fresh `.specd/` is staged in sibling temp tree and renamed into place; preexisting project files are never deleted during rollback; residual partial state is explicitly reported.
  - acceptance: Failure before commit leaves no `.specd/`; failure during external merge preserves original file and reports backup/remediation.
  - verify: go test ./internal/core/... ./internal/cmd/... -run 'Init.*(Transaction|Rollback|Stage)' -count=1
  - depends: T4, T6
  - requirements: R1.2

## Wave 3 — MCP discovery quality

- [x] T8 — Implement MCP protocol version negotiation
  - why: Fixed `2024-11-05` response ignores current MCP lifecycle negotiation contract.
  - role: builder
  - files: internal/mcp/server.go, internal/mcp/server_test.go, internal/mcp/integration_test.go
  - contract: Parse initialize params, maintain ordered supported revisions, echo supported requested revision, otherwise return latest supported revision, and preserve old-client compatibility.
  - acceptance: Tests cover newest, legacy, missing, and unsupported requested versions with deterministic responses.
  - verify: go test ./internal/mcp/... -run 'Initialize|ProtocolVersion|EndToEnd' -count=2
  - depends: —
  - requirements: R4.3, R6.4

- [x] T9 — Add concise MCP server instructions
  - why: Hosts can discover tools but lack server-wide workflow guidance.
  - role: builder
  - files: internal/mcp/server.go, internal/mcp/server_test.go, internal/core/embed_templates/AGENTS.md
  - contract: Initialize response includes instructions telling agent to orient first, avoid direct state edits, check gates, and verify evidence; first 512 characters are self-contained.
  - acceptance: Instructions exist, remain under agreed byte budget, and align with AGENTS.md non-negotiable rules.
  - verify: go test ./internal/mcp/... ./internal/cmd/... -run 'Instructions|AgentBudget' -count=1
  - depends: T8
  - requirements: R4.4, R6.3

- [x] T10 — Build reusable in-process MCP health probe
  - why: Init and doctor need proof that server negotiates and exposes baseline tools.
  - role: builder
  - files: internal/mcp/probe.go, internal/mcp/probe_test.go
  - contract: Probe performs initialize, initialized notification, tools/list, baseline tool assertion, protocol/tool-count capture, timeout, and latency measurement without shell execution.
  - acceptance: Healthy server passes; missing baseline tool, malformed response, timeout, and protocol mismatch produce typed failures.
  - verify: go test ./internal/mcp/... -run 'Probe' -count=2
  - depends: T8, T9
  - requirements: R4.1, R4.2, R4.3, R4.4

## Wave 4 — Host adapter framework

- [x] T11 — Define host adapter registry and conformance suite
  - why: Snippet-only map cannot support detection, install, inspect, verify, or repair.
  - role: builder
  - files: internal/integration/adapter.go, internal/integration/registry.go, internal/integration/conformance_test.go, internal/mcp/hosts.go
  - contract: Typed adapters expose name, scopes, detection, plan, install, inspect, and verify; registry order is deterministic; legacy HostConfig remains compatible during migration.
  - acceptance: Fake adapter passes shared idempotency, no-shell, no-secret, deterministic-plan, and project-scope tests.
  - verify: go test ./internal/integration/... ./internal/mcp/... -run 'Adapter|Registry|HostConfig' -count=2
  - depends: T2
  - requirements: R2.1, R2.2, R3.3, R6.4

- [x] T12 — Implement host detection engine
  - why: Init needs evidence-backed supported-agent discovery without guessing.
  - role: builder
  - files: internal/integration/detect.go, internal/integration/detect_test.go
  - contract: Detect executable via LookPath, project config markers, supported scopes, confidence, and reason; no writes or host process execution during detection.
  - acceptance: Fake PATH and config fixtures produce stable results; ambiguous non-interactive selection returns suggestions without mutation.
  - verify: go test ./internal/integration/... -run 'Detect|Selection' -count=2
  - depends: T11
  - requirements: R2.1, R2.2, R2.3, R2.4

- [x] T13 — Add integration ownership manifest
  - why: Repair and removal require proof of which host entries specd owns.
  - role: builder
  - files: internal/integration/manifest.go, internal/integration/manifest_test.go, internal/core/paths.go
  - contract: `.specd/integrations.json` records schema version, host, scope, server name, root strategy, method, target, and SHA-256 fingerprint; stores no environment values or secrets.
  - acceptance: Atomic load/save, migration guard, stable ordering, fingerprint mismatch refusal, and portability policy are tested.
  - verify: go test ./internal/integration/... -run 'Manifest|Fingerprint|Ownership' -count=2
  - depends: T11
  - requirements: R3.4, R3.5

- [x] T14 — Add safe JSON project-config merge utility
  - why: Cursor, VS Code, Gemini, and Claude project configs may contain unrelated user settings.
  - role: builder
  - files: internal/integration/jsonmerge.go, internal/integration/jsonmerge_test.go
  - contract: Parse existing JSON, mutate only named nested MCP server key, preserve unrelated semantic values, reject invalid JSON, backup before write, and emit deterministic formatting.
  - acceptance: Multi-server fixtures survive; invalid JSON writes nothing; duplicate owned entry updates idempotently; symlink escape policy is enforced.
  - verify: go test ./internal/integration/... -run 'JSONMerge|Backup|Symlink' -count=2
  - depends: T11, T13
  - requirements: R3.2, R3.5, R6.2

## Wave 5 — CLI coding-agent integrations

- [x] T15 — Implement Codex project adapter
  - why: Codex supports CLI-managed MCP and shared CLI/IDE project configuration.
  - role: builder
  - files: internal/integration/codex.go, internal/integration/codex_test.go, internal/mcp/embed_hosts/codex.toml
  - contract: Detect `codex`; prefer official `codex mcp add` argv without shell; inspect project registration; fall back to manual snippet if project scope cannot be safely automated.
  - acceptance: Fake Codex records exact argv including `specd mcp --root`; rerun is idempotent; unavailable CLI yields actionable manual result.
  - verify: go test ./internal/integration/... -run 'Codex' -count=2
  - depends: T10, T11, T12, T13
  - requirements: R2.1, R3.1, R3.3, R3.5, R4.5

- [x] T16 — Implement Claude Code project adapter
  - why: Existing Claude Desktop snippet does not serve main CLI coding-agent path.
  - role: builder
  - files: internal/integration/claude.go, internal/integration/claude_test.go, internal/mcp/embed_hosts/
  - contract: Detect `claude`; use official project-scoped MCP registration when available; inspect `.mcp.json`; preserve existing servers; retain Claude Desktop as separate manual adapter.
  - acceptance: Fake Claude argv and `.mcp.json` fixtures pass conformance; no global config changes occur by default.
  - verify: go test ./internal/integration/... -run 'Claude' -count=2
  - depends: T10, T11, T12, T13, T14
  - requirements: R2.1, R3.1, R3.2, R3.3, R3.5

- [x] T17 — Implement Gemini CLI project adapter
  - why: Gemini CLI is a major CLI agent and supports project/user MCP scopes.
  - role: builder
  - files: internal/integration/gemini.go, internal/integration/gemini_test.go, internal/mcp/embed_hosts/gemini.json
  - contract: Detect `gemini`; prefer `gemini mcp add` project scope; inspect `.gemini/settings.json`; preserve trust/allow settings and unrelated servers.
  - acceptance: Fake Gemini CLI and JSON fixtures pass adapter conformance and idempotency tests.
  - verify: go test ./internal/integration/... ./internal/mcp/... -run 'Gemini|HostConfig' -count=2
  - depends: T10, T11, T12, T13, T14
  - requirements: R2.1, R3.1, R3.2, R3.3, R3.5

## Wave 6 — Init orchestration and doctor

- [x] T18 — Integrate agent selection and consent into init
  - why: Scaffold and adapters must form one smooth, safe first-run flow.
  - role: builder
  - files: internal/cmd/init.go, internal/core/commands.go, internal/cli/args.go, internal/cmd/init_test.go
  - contract: Add `--agent`, `--scope`, `--yes`, `--non-interactive`, `--verbose`; interactive TTY may prompt; non-TTY ambiguous auto-detection mutates no host config; global scope requires explicit consent.
  - acceptance: Fresh interactive, explicit non-interactive, ambiguous, skip, all, unavailable host, and global-scope cases produce specified outcomes.
  - verify: go test ./internal/cmd/... ./internal/cli/... -run 'Init.*(Agent|Interactive|Consent|Scope)' -count=2
  - depends: T5, T7, T10, T12, T15, T16, T17
  - requirements: R2.3, R2.4, R3.1, R5.1, R5.3

- [x] T19 — Add `specd doctor`
  - why: Users need one command to diagnose scaffold, MCP server, and host registration.
  - role: builder
  - files: internal/cmd/doctor.go, internal/cmd/doctor_test.go, internal/cmd/registry.go, internal/core/commands.go, main.go
  - contract: Doctor reports binary/root/scaffold integrity, MCP probe, host detection, registration state, and exact remediation; `--fix` only repairs safe project-scoped owned state.
  - acceptance: Registry/help parity passes; healthy, missing scaffold, broken MCP, missing registration, invalid config, and ownership mismatch fixtures return correct codes.
  - verify: go test ./internal/cmd/... ./internal/core/... ./internal/mcp/... -run 'Doctor|RegistryMatchesHelp' -count=2
  - depends: T10, T11, T13, T18
  - requirements: R4.5, R5.2, R5.3

- [x] T20 — Finalize concise human and JSON onboarding receipts
  - why: First-run output must communicate readiness and one next action, not only file inventory.
  - role: builder
  - files: internal/cmd/init.go, internal/cmd/doctor.go, internal/core/output.go, internal/cmd/json_contract_test.go, internal/cmd/agent_budget_test.go
  - contract: Human output leads with status, configured agents, verification, and next action; JSON emits one InitResult; verbose mode carries path detail.
  - acceptance: No ANSI in JSON; arrays non-null; output deterministic; next action appears within first 12 human lines.
  - verify: go test ./internal/cmd/... -run 'Init.*(Output|JSON|Budget)|Doctor.*(Output|JSON)' -count=2
  - depends: T18, T19
  - requirements: R5.1, R5.2, R5.3

## Wave 7 — IDE adapters and compatibility

- [x] T21 — Implement Cursor workspace adapter
  - why: Cursor is a major coding-agent host and currently requires manual snippet merge.
  - role: builder
  - files: internal/integration/cursor.go, internal/integration/cursor_test.go, internal/mcp/embed_hosts/cursor.json
  - contract: Detect Cursor/project config, safely merge `specd` server under official workspace schema, preserve unrelated servers, and return reload guidance.
  - acceptance: Current official schema fixture parses; setup is idempotent; invalid or unknown schema fails to manual instructions without destructive write.
  - verify: go test ./internal/integration/... ./internal/mcp/... -run 'Cursor|HostConfig' -count=2
  - depends: T11, T12, T13, T14, T20
  - requirements: R3.1, R3.2, R3.5, R6.4

- [x] T22 — Implement VS Code workspace adapter
  - why: VS Code supports workspace MCP configuration and guided host management.
  - role: builder
  - files: internal/integration/vscode.go, internal/integration/vscode_test.go, internal/mcp/embed_hosts/vscode.json
  - contract: Detect `code`/workspace config, use stable official workspace schema or native command when available, preserve unrelated settings, and return reload/trust guidance.
  - acceptance: Current official schema fixture and fallback-manual path pass; no user settings mutation by default.
  - verify: go test ./internal/integration/... ./internal/mcp/... -run 'VSCode|HostConfig' -count=2
  - depends: T11, T12, T13, T14, T20
  - requirements: R3.1, R3.2, R3.5, R6.4

- [x] T23 — Expand host compatibility matrix and drift guards
  - why: Worldwide harness claims must be test-backed and honest as host schemas evolve.
  - role: builder
  - files: internal/integration/conformance_test.go, internal/mcp/host_compat_test.go, docs/agent-harness-compat.md
  - contract: Matrix lists detection, project install, global install, stdio, HTTP, verification depth, and known limits for every adapter; registry and docs sets must match.
  - acceptance: CI fails when adapter, docs, templates, or fixtures drift; unsupported cells say unsupported/manual, never implied supported.
  - verify: go test ./internal/integration/... ./internal/mcp/... -run 'Compatibility|Conformance|Host' -count=2
  - depends: T15, T16, T17, T21, T22
  - requirements: R6.3, R6.4

## Wave 8 — Installer, documentation, and release proof

- [x] T24 — Improve installer handoff
  - why: Installation must lead directly to agent-aware project onboarding without initializing arbitrary directories.
  - role: builder
  - files: scripts/install.sh, scripts/install_test.sh, README.md, docs/user-guide.md
  - contract: Installer verifies installed binary/version, prints binary path, handles shell PATH guidance accurately, and recommends `cd <project> && specd init --agent auto`.
  - acceptance: Shell tests cover existing install, PATH absent/present, source fallback, and no project mutation.
  - verify: shellcheck scripts/*.sh && sh scripts/install_test.sh
  - depends: T18, T20
  - requirements: R5.3, R6.2

- [x] T25 — Rewrite golden-path onboarding docs
  - why: Market-ready onboarding should begin with user outcome and agent prompt.
  - role: builder
  - files: README.md, docs/user-guide.md, docs/mcp-guide.md, docs/command-reference.md, docs/troubleshooting.md
  - contract: Document one-command auto path, explicit/manual path, air-gapped path, host matrix, consent/scope rules, doctor remediation, and trust boundaries.
  - acceptance: All flags/hosts match command metadata and adapter registry; examples use project scope by default.
  - verify: go test ./internal/cmd/... ./internal/mcp/... ./internal/integration/... -run 'Registry|Help|Compatibility|Docs' -count=1
  - depends: T19, T20, T23, T24
  - requirements: R3.1, R4.5, R5.3, R6.3

- [x] T26 — Add onboarding performance and deterministic-output gates
  - why: Smooth onboarding needs measurable latency and stable machine contracts.
  - role: builder
  - files: internal/cmd/init_benchmark_test.go, internal/mcp/probe_test.go, docs/agent-harness-baselines.md, Makefile, scripts/coverage-check.sh
  - contract: Benchmark fresh init, rerun, detection, and MCP probe; add deterministic output assertions and documented threshold policy without flaky wall-clock gates.
  - acceptance: Baselines recorded; p95 target documented; deterministic byte checks run in CI; performance regression policy is reviewable.
  - verify: go test ./internal/cmd/... ./internal/mcp/... -run 'Deterministic|BenchmarkContract' -count=2
  - depends: T20, T23
  - requirements: R1.3, R4.1, R5.2

- [x] T27 — Run full production release gate
  - why: Init changes touch file safety, MCP protocol, host config, docs, and cross-platform behavior.
  - role: verifier
  - files: Makefile, TESTING.md, docs/agent-harness-compat.md
  - contract: Run formatting, vet, shellcheck, race suite, order-dependence suite, coverage floor, stress harness, and cross-platform build; record supported/unsupported host evidence.
  - acceptance: `make ci` passes; no new dependency; working tree contains no generated temp/backup artifacts; compatibility matrix matches evidence.
  - verify: make ci
  - depends: T25, T26
  - requirements: R1.1, R1.3, R3.2, R4.1, R5.1, R6.1, R6.2, R6.4
