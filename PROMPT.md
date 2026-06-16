# Claude Code Opus 4.8 Prompt — Specd Multi-Feature Implementation

> **Agent:** Claude Code with Opus 4.8
> **Target Repository:** [specd](https://github.com/0xkhdr/specd) — Spec-Driven Coding Harness CLI
> **Date:** 2026-06-17
> **Mode:** Spec-driven, evidence-gated, multi-feature decomposition

---

## Mission Statement

Analyze the **specd** repository to build foundational knowledge of the project, with primary focus on the **MCP (Model Context Protocol) component**. Based on this analysis, create a structured implementation plan across three independent feature domains. For each feature, produce a dedicated spec directory containing `spec.md` and `tasks.md` that follows specd's own spec-driven workflow best practices.

The three feature domains are:

1. **MCP Deep-Dive & Real-World Integration** — Understand how specd's MCP server works and how to integrate it into real projects (Claude Desktop, Cursor, VS Code, etc.).
2. **MCP Expansion for External Tools** — Extend specd's MCP capabilities to support **antigravity CLI** and **codex** (OpenAI's CLI tool) as first-class MCP clients.
3. **Dashboard Decoupling from VS Code** — Remove the VS Code dashboard extension and make the dashboard accessible from any web browser for universal client availability.

---

## Phase 0: Repository Analysis & Knowledge Foundation

### 0.1 Repository Structure Discovery

Clone and analyze the specd repository structure. Focus on:

```
specd/
├── main.go                          # Entry point, arg router
├── internal/
│   ├── cli/args.go                  # Flag/positional parser
│   ├── cmd/                         # One file per CLI command
│   │   ├── init.go, new.go, status.go, context.go
│   │   ├── check.go, next.go, dispatch.go
│   │   ├── task.go, verify.go, approve.go
│   │   ├── report.go, waves.go, program.go
│   │   ├── update.go, mcp.go        # ← MCP command entry
│   │   └── registry.go              # Command dispatch table
│   ├── core/                        # Domain logic
│   │   ├── paths.go                 # .specd root locator
│   │   ├── state.go                 # state.json machine ledger
│   │   ├── phases.go                # Phase ↔ status mapping
│   │   ├── tasksparser.go           # tasks.md parser
│   │   ├── dag.go                   # Wave DAG, frontier
│   │   ├── ears.go                  # EARS requirements linter
│   │   ├── report.go                # md/html assembler
│   │   ├── specfiles.go             # Artifact accessors
│   │   ├── agents.go                # AGENTS.md marker merge
│   │   ├── commands.go              # CommandMeta registry
│   │   └── embed_templates/         # Shipped templates (go:embed)
│   │       ├── AGENTS.md
│   │       ├── config.json
│   │       ├── steering/            # Constitution files
│   │       ├── roles/               # Role persona prompts
│   │       ├── specStubs/           # Spec artifact stubs
│   │       └── skills/              # Companion skills
│   ├── mcp/                         # ← MCP JSON-RPC 2.0 stdio server
│   │   ├── server.go                # MCP stdio loop, request routing
│   │   ├── tools.go                 # Tool definitions from core.Commands
│   │   ├── resources.go             # MCP resources (spec files as resources)
│   │   └── prompts.go               # MCP prompts (role prompts)
│   └── testharness/                 # Deterministic test infra
├── editors/                         # VS Code extension (live dashboard webview)
│   └── vscode/                      # ← Target for removal/migration
├── .github/actions/specd-pr/        # GitHub Action
├── scripts/                         # install.sh, uninstall.sh
├── docs/                            # Full documentation
│   ├── concepts.md                  # Philosophy, eight principles
│   ├── agent-integration.md         # MCP, roles, subagent modes
│   ├── user-guide.md                # Lifecycle, artifacts
│   ├── command-reference.md         # Every command, flag, exit code
│   ├── validation-gates.md          # 7 core + opt-in gates
│   └── contributor-guide.md         # Architecture, concurrency
├── AGENTS.md                        # Dev guide for specd itself
├── TESTING.md                       # Deterministic test harness guide
└── Makefile / go.mod / LICENSE
```

### 0.2 MCP Component Deep Analysis

**Critical: Read and understand every file in `internal/mcp/` before proceeding.**

From the codebase analysis, specd's MCP implementation follows these patterns:

- **Protocol:** JSON-RPC 2.0 over stdio (protocol version `2024-11-05`)
- **Transport:** Stdio only (no HTTP/SSE yet)
- **Tools:** Every specd command (except `help`, `version`, `mcp`) is exposed as an MCP tool with `specd_` prefix
- **Tool Annotations:** `readOnlyHint` (status, waves, context, check, next, dispatch, report), `destructiveHint` (uninstall, update)
- **Tool Invocation:** Re-dispatches into existing CLI handlers with `SPECD_JSON=1` forced
- **Error Handling:** Panic recovery per call; malformed requests never tear down the loop
- **Exit Code Mapping:** Non-zero exit → `isError: true`; stderr diagnostics appended on failure
- **Structured Content:** JSON stdout is also attached as `structuredContent` in the result

**Key implementation details found in `internal/mcp/server.go`:**

```go
// The stdio loop reads framed JSON-RPC, dispatches to existing handlers,
// and writes framed responses. All diagnostics on stderr; r/w carry only protocol bytes.
func Serve(r io.Reader, w io.Writer, dispatch Dispatcher) error

// Tool calls re-dispatch into matching specd handler with SPECD_JSON semantics
func callTool(rawParams json.RawMessage, dispatch Dispatcher) (any, *rpcError)

// buildArgv turns tool arguments into CLI argv that round-trips through cli.ParseArgs
func buildArgv(arguments map[string]any) ([]string, error)

// capture redirects stdout/stderr through pipes, drains in goroutines,
// recovers panics so one bad call never crashes the server
func capture(fn func() int) (stdout, stderr string, code int)
```

**Key implementation details found in `internal/mcp/tools.go`:**

```go
const toolPrefix = "specd_"

var metaCommands = map[string]bool{"help": true, "version": true, "mcp": true}

var readOnlyCommands = map[string]bool{
    "status": true, "waves": true, "context": true, "check": true,
    "next": true, "dispatch": true, "report": true,
}

var destructiveCommands = map[string]bool{"uninstall": true, "update": true}

// buildTools generates tool list from core.Commands — new commands surface automatically
func buildTools() []toolDef
```

### 0.3 VS Code Dashboard Extension Analysis

Examine the `editors/vscode/` directory to understand:

- Extension manifest (`package.json`)
- Webview panel implementation (dashboard UI)
- Communication bridge between webview and specd CLI
- Current dependency on VS Code APIs (webview, commands, extension host)
- How `specd serve` currently provides the read-only dashboard

### 0.4 Documentation Review

Read these docs thoroughly before spec authoring:

1. `docs/concepts.md` — Eight principles, foundational split
2. `docs/agent-integration.md` — MCP wiring, roles, subagent modes, frontier dispatch
3. `docs/user-guide.md` — EARS requirements, design headers, task DAG, verify→complete flow
4. `docs/command-reference.md` — All commands, flags, exit codes
5. `docs/validation-gates.md` — Gate checks (EARS, design, task-schema, DAG, evidence, sync, traceability)
6. `docs/contributor-guide.md` — CLI architecture, concurrency model (advisory lock + CAS)

### 0.5 Build & Test Baseline

```bash
make build    # go build with ldflags
make test     # go test ./... -race -count=1
make ci       # full gate: lint + race + count=2 + coverage + stress
```

All tests must pass before any change. The test harness is deterministic (`internal/testharness`).

---

## Phase 1: Feature Spec Creation

Create **three independent spec directories** under `.specd/specs/` (or the equivalent in your working context). Each spec must contain:

```
.specd/specs/
├── mcp-integration/                 # Feature 1
│   ├── spec.md
│   └── tasks.md
├── mcp-expansion/                   # Feature 2
│   ├── spec.md
│   └── tasks.md
└── dashboard-web/                   # Feature 3
    ├── spec.md
    └── tasks.md
```

### Spec Format Requirements (per specd's own rules)

Each `spec.md` must follow the **EARS** (Easy Approach to Requirements Syntax) format:

```markdown
# Spec: <Feature Name>

## Requirements

### R1: <Requirement Title>
**When** <trigger>, **the** <system> **shall** <response>.
- **Acceptance:** <measurable criteria>
- **Gate:** <which validation gate validates this>

### R2: ...

## Design

### Architecture
### Data Flow
### Security Model
### Error Handling

## Decisions
<!-- ADRs go here -->

## Memory
<!-- Learnings promoted from other specs -->
```

Each `tasks.md` must define a **DAG of waves**:

```markdown
# Tasks: <Feature Name>

## Wave 1: Foundation
- [ ] T1: <Task> — `role: investigator` — `verify: <command>`
  - deps: []
- [ ] T2: <Task> — `role: builder` — `verify: <command>`
  - deps: [T1]

## Wave 2: Implementation
- [ ] T3: <Task> — `role: builder` — `verify: <command>`
  - deps: [T1, T2]

## Wave 3: Verification
- [ ] T4: <Task> — `role: verifier` — `verify: <command>`
  - deps: [T3]
```

---

## Feature 1: MCP Deep-Dive & Real-World Integration

### Domain: Protocol Integration / Developer Experience

### Objective

Transform specd's MCP from a "hidden" stdio server into a well-documented, easily integrable component that developers can connect to Claude Desktop, Cursor, VS Code MCP extensions, and any other MCP host with minimal friction.

### Key Questions to Answer in the Spec

1. **How does the current MCP server start?** (`specd mcp` command — examine `internal/cmd/mcp.go`)
2. **What is the exact stdio framing protocol?** (newline-delimited JSON-RPC 2.0)
3. **Which tools are exposed and what are their schemas?** (auto-generated from `core.Commands` via `buildTools()`)
4. **How does a host configure specd as an MCP server?** (Claude Desktop `mcpServers` config, Cursor MCP settings, etc.)
5. **What are the current limitations?** (stdio only, no SSE/HTTP, no resource subscriptions, no prompt templating)
6. **What is the real-world workflow for a developer using specd + MCP?** (init → write specs → drive via Claude Desktop/Cursor)

### Best Practices for This Domain (MCP/Protocol Integration)

- Follow the **official MCP specification** (2025-03-26 or latest stable)
- Use **JSON-RPC 2.0** with proper error codes (`-32700` parse, `-32600` invalid request, `-32601` method not found, `-32602` invalid params)
- Implement **graceful degradation** — malformed requests must never crash the server (already done; preserve this)
- Ensure **stdio hygiene** — only protocol bytes on stdout, diagnostics on stderr
- Support **capability negotiation** — declare `tools` capability with `listChanged: false` (current)
- Document **host configuration** with concrete examples for each target client
- Maintain **zero external dependencies** — specd uses Go stdlib only; any MCP changes must preserve this

### Spec Deliverables

- `spec.md`: Requirements (EARS), design architecture, security model, host configuration examples, gap analysis
- `tasks.md`: DAG waves covering analysis, documentation, configuration examples, integration tests

---

## Feature 2: MCP Expansion for External Tools (antigravity CLI + codex)

### Domain: Protocol Extension / Tool Ecosystem

### Objective

Extend specd's MCP server to support **antigravity CLI** and **OpenAI Codex CLI** as first-class MCP clients. This involves understanding each tool's architecture and creating the appropriate integration layer.

### Tool Analysis Requirements

#### antigravity CLI
- Research the antigravity CLI architecture and MCP client capabilities
- Determine if it supports stdio transport or requires HTTP/SSE
- Identify any custom capability requirements
- Design integration approach (direct stdio, wrapper script, or HTTP bridge)

#### OpenAI Codex CLI
- Research codex CLI's tool/plugin/extension mechanism
- Determine if codex supports MCP natively or requires an adapter
- Understand codex's context window and interaction patterns
- Design specd-to-codex bridge or native MCP support

### Best Practices for This Domain (Tool Ecosystem Integration)

- **Adapter Pattern**: Create thin adapters rather than modifying core specd logic
- **Transport Abstraction**: If a tool doesn't support stdio, implement an HTTP/SSE transport layer (this is a significant addition — consider as separate sub-spec)
- **Capability Discovery**: Tools must discover specd capabilities via `tools/list`; no hardcoded assumptions
- **Graceful Fallback**: If a tool doesn't support a specd feature, degrade gracefully
- **Configuration as Code**: Provide declarative configuration files for each tool integration
- **Test Isolation**: Use `internal/testharness` to create sandboxed integration tests
- **Documentation-First**: Each integration must have a dedicated setup guide with copy-paste configs

### Spec Deliverables

- `spec.md`: Requirements for each tool, transport architecture, adapter design, security boundaries, configuration schema
- `tasks.md`: DAG waves covering tool research, adapter implementation, transport layer (if needed), integration tests, documentation

---

## Feature 3: Dashboard Decoupling from VS Code

### Domain: Web Application / Cross-Platform UI

### Objective

Remove the VS Code-specific dashboard extension and make the dashboard a **standalone web application** accessible from any browser. The dashboard must remain read-only (as per specd's design) and communicate with the specd CLI via the existing `specd serve` mechanism or an enhanced API.

### Current State Analysis

- `specd serve` already provides a read-only dashboard (mentioned in README)
- `editors/vscode/` contains a VS Code extension with a webview-based dashboard
- The webview likely communicates with specd via some IPC or HTTP mechanism
- The goal is to **extract the dashboard UI** from the VS Code extension host and make it a generic web app

### Best Practices for This Domain (Web Dashboard / Cross-Platform UI)

- **Self-Contained HTML**: The existing dashboard is likely self-contained HTML (specd's reports are "self-contained HTML"); preserve this philosophy
- **Zero Build Step**: If possible, serve the dashboard directly from Go's `embed` or `http.FileServer` without a Node build step
- **Read-Only by Design**: The dashboard must NEVER mutate spec state — it reads from `state.json` and artifact files only
- **Real-Time Updates**: Use `specd watch` (NDJSON/SSE/webhook frontier events) for live updates without polling
- **No LLM Dependency**: Reports are generated programmatically from `state.json` — no LLM calls for rendering
- **Responsive Design**: Must work on desktop, tablet, and mobile browsers
- **Security**: If served over network, implement authentication/authorization or bind to localhost only
- **Backward Compatibility**: Existing `specd serve` behavior must be preserved or enhanced, not broken

### Architecture Direction

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Browser   │◄───►│  specd serve│◄───►│  state.json │
│  (any client)│ HTTP │  (Go http)  │ read │  + artifacts│
└─────────────┘     └─────────────┘     └─────────────┘
                            │
                            ▼
                     ┌─────────────┐
                     │ specd watch │
                     │ (SSE/NDJSON)│
                     └─────────────┘
```

### Spec Deliverables

- `spec.md`: Requirements for web dashboard, VS Code extension removal plan, architecture, security model, real-time update mechanism, browser compatibility matrix
- `tasks.md`: DAG waves covering VS Code extension audit, dashboard extraction, web server enhancement, real-time streaming, cross-browser testing, documentation

---

## Phase 2: Cross-Cutting Concerns

### Security Model

Apply specd's existing security model to all features:

- `verify:` commands execute via `sh -c` — treat as hostile input
- Child env is scrubbed to allowlist (`PATH`, `HOME`, `LANG`, `LC_ALL`, `TMPDIR`, `SPECD_*`)
- Spec slugs are path-validated (`^[a-z0-9][a-z0-9-]*$`)
- `state.json` is machine truth — never hand-edit
- Per-spec advisory lock (`WithSpecLock`) for concurrent mutation

### Testing Requirements

Every feature must include:

- Unit tests co-located with implementation (`*_test.go`)
- Deterministic test harness usage (`internal/testharness`)
- Race detector clean (`go test -race`)
- Coverage floor compliance (check existing coverage policy)
- End-to-end scenario tests (init → execute → report)

### Exit Code Contract

All new commands must follow specd's exit code contract:
- `0` = ok
- `1` = gate/validation failure
- `2` = usage error
- `3` = not found

### Documentation Requirements

Each feature must update or add:

- `docs/` entries (concepts, user-guide, command-reference as appropriate)
- `AGENTS.md` updates if agent workflow changes
- README.md updates for feature highlights
- Inline code comments citing design rationale

---

## Phase 3: Execution Protocol

### For Each Feature Spec:

1. **Analyze Phase** (investigator role)
   - Read all relevant source files
   - Document current behavior with exact file/line references
   - Identify integration points and boundaries

2. **Plan Phase** (builder role, read-only)
   - Author `requirements.md` in EARS format
   - Run `specd check <spec>` to validate
   - Human approval: `specd approve <spec>`

3. **Design Phase** (builder role)
   - Author `design.md` with all mandatory sections
   - Run `specd check <spec>`
   - Human approval: `specd approve <spec>`

4. **Tasks Phase** (builder role)
   - Author `tasks.md` with DAG waves
   - Run `specd check <spec>`
   - Human approval: `specd approve <spec>`

5. **Execute Phase** (builder role)
   - `specd next <spec>` to get runnable task
   - Implement task
   - `specd verify <spec> T<N>` — harness runs verify command, records exit code + git HEAD
   - `specd task <spec> T<N> --status complete` (only allowed if verify passed)
   - Repeat until all tasks complete

6. **Verify Phase** (verifier role)
   - `specd check <spec>` final validation
   - `specd approve <spec>` to close spec and generate reports

### Cross-Spec Coordination

If features have dependencies:

```bash
specd program link mcp-expansion --on mcp-integration
specd program link dashboard-web --on mcp-integration
specd program              # view program-level DAG
```

---

## Agent Discipline

### Thinking Protocol (Six-Phase Thinking)

Before any code change, follow the reasoning discipline from `.specd/steering/reasoning.md`:

1. **Comprehend** — What is the true intent? What are the hidden constraints?
2. **Decompose** — What are the atomic sub-problems?
3. **Hypothesize** — What are 3 possible approaches? What are their trade-offs?
4. **Select** — Which approach best satisfies constraints with least risk?
5. **Validate** — How will I know this is correct before I write code?
6. **Execute** — Write the minimal change that satisfies the hypothesis.

### Backpropagation Protocol

If a task reveals a flaw in an earlier phase (requirements, design, tasks):

1. Stop execution immediately
2. Document the flaw in `decisions.md` as an ADR
3. Propose a mid-requirement change via `specd midreq <spec>`
4. Do NOT proceed until human approves the change
5. Update affected downstream artifacts (design.md, tasks.md)

### Context Engineering

Use `specd context <spec>` to control what enters your context window:

```bash
specd context mcp-integration
```

This emits:
1. Phase briefing ("You are in PLAN phase. Do not edit code.")
2. Load list (minimal file list for context)
3. Signals (blockers, awaiting approval, uncovered requirements)

---

## Final Deliverables Checklist

- [ ] Three spec directories created with `spec.md` and `tasks.md`
- [ ] All specs follow EARS syntax and pass `specd check`
- [ ] All specs have human-approved phase transitions (`specd approve`)
- [ ] MCP component fully analyzed with file/line references
- [ ] VS Code extension audit complete with removal plan
- [ ] Dashboard web architecture designed with real-time updates
- [ ] antigravity CLI and codex integration paths identified
- [ ] All changes tested with `make test` (race detector clean)
- [ ] Documentation updated in `docs/` and `README.md`
- [ ] `AGENTS.md` updated if agent workflow changes
- [ ] Cross-spec dependencies declared in `.specd/program.json` if applicable

---

## Appendix: Reference Commands

```bash
# Repository setup
git clone https://github.com/0xkhdr/specd.git
cd specd
make build
make test

# Spec workflow (per feature)
specd init                          # if not already initialized
specd new <feature> --title "..."
specd check <feature>
specd approve <feature>
specd next <feature>
specd verify <feature> T<N>
specd task <feature> T<N> --status complete
specd report <feature>

# MCP testing
specd mcp                           # start MCP stdio server (for manual testing)
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | specd mcp

# Dashboard
specd serve                         # start read-only dashboard
specd watch                         # emit frontier events (NDJSON/SSE/webhook)

# Cross-spec program
specd program link <spec> --on <dep>
specd program status
```

---

*End of Prompt*
