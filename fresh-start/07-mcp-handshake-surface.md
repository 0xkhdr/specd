# Domain: MCP & Handshake Surface

## 1. Purpose & value mapping
- **Principles served:** P5 (Agent-Agnostic by Design), P1 (harness exposes enforcement,
  not authorship, as tools).
- **Paper concept realized:** *tools* — "the developer defines the tools the agent will
  have access to" (p.28). MCP is how specd's deterministic capabilities become callable
  tools inside any MCP-speaking host.
- **Core use case:** an MCP host (e.g. Claude Code) connects over stdio JSON-RPC and calls
  a small, parity-tested set of specd tools (`check`, `next`, `verify`, `status`, `task`,
  context) plus a handshake that hands the host its policy digest. No vendor lock-in; the
  same tools everywhere.
- **If none → CUT:** N/A for MCP (it is the primary tool transport); but the surface is
  heavily trimmed.

## 2. Current-state analysis (from specd)
- **Reference files read:** `internal/mcp/*`, `internal/cmd/mcp.go`,
  `internal/cmd/handshake.go`, `internal/core/handshake.go`, `docs/mcp-guide.md`,
  `docs/agent-integration.md` (MCP tools section), `internal/core/manifest_tools.go`.
- **What exists today; key contracts/invariants:**
  - `mcp.go` (85) starts a native stdio JSON-RPC 2.0 server (`internal/mcp`), stdlib-only.
  - **Six intent-level MCP tools** wrap orchestration verbs (`brain_orchestrate`,
    `brain_status`, `brain_approve`, `brain_pause`/`brain_cancel`, `brain_resume`) — "they
    add no new core authority." Two **raw passthrough tools** (`specd_brain`,
    `specd_pinky`) also exist.
  - `handshake.go` (66) dispatches `bootstrap`; `core/handshake.go` defines
    `HandshakeBootstrapVersion` — the host bootstrap/policy schema. `manifest_tools.go`
    provides a per-spec MCP tool policy (`required/optional/forbidden`) from `manifest.json`,
    read-only and deterministic (degrades to empty policy on malformed file).
- **Redundancy / complexity / drift found (evidence):**
  - Both intent-level tools **and** raw passthroughs exist for Brain/Pinky — two ways to do
    the same thing, doubling the tested surface. The doc itself notes passthroughs "add no
    new core authority," which is an argument to cut them.
  - The MCP tool set skews toward orchestration (six brain tools) before the core lifecycle
    tools are even enumerated — inverted priority for an MVP whose orchestration tier is
    opt-in.

## 3. Fresh-start decision
- **Verdict per capability:**
  - stdio JSON-RPC 2.0 server — **KEEP** (stdlib-only, the portable floor).
  - HTTP JSON-RPC — **DEFER** (stdio covers the MVP hosts; HTTP is a later transport).
  - Core lifecycle tools (`check`, `next`, `verify`, `task`, `status`, `context`,
    `handshake`) — **KEEP** as the parity-tested set.
  - Six brain intent tools — **SIMPLIFY to three** (`brain_orchestrate`, `brain_status`,
    `brain_approve`) and only register them when orchestration is enabled (domain 09).
  - Raw passthroughs (`specd_brain`, `specd_pinky`) — **CUT** (no added authority; double
    surface).
  - Handshake bootstrap + policy digest — **KEEP** (P5 on-ramp).
  - Per-spec tool policy (`manifest_tools.go`) — **KEEP** (lets a spec forbid/require tools;
    a real guardrail).
- **Minimal accurate surface:**
  - Commands: `mcp` (serve stdio), `handshake [bootstrap|policy]`.
  - MCP tools (v1): `specd_check`, `specd_next`, `specd_verify`, `specd_task`,
    `specd_status`, `specd_context`, `specd_handshake` (+ 3 brain tools when enabled).
  - Modules: `internal/mcp` (server), `core/handshake.go`, `manifest_tools.go`.
- **Architecture & flexibility improvements:**
  - **Parity test as a gate:** every CLI verb exposed as an MCP tool must have a
    `TestMCPParity` asserting the tool result equals the CLI result for the same input —
    the two surfaces can never drift (mirrors the registry↔help guard, domain 10).
  - **Tool registration is data-driven** from the same command registry (domain 10), so
    adding a core verb optionally exposes a tool without a second hand-maintained list.
  - **Policy-scoped tools:** the per-spec `manifest.json` `forbidden` list is enforced
    server-side, so a spec can lock a worker out of dangerous tools.

## 4. Requirements (EARS-shaped) — seed for requirements.md
1. When `mcp` is invoked, the system shall serve JSON-RPC 2.0 over stdio using only the Go
   standard library.
2. When a host calls a core specd tool, the system shall return a result byte-equal to the
   corresponding CLI command's JSON output for the same inputs.
3. When orchestration is disabled, the system shall not register any brain MCP tool.
4. When a spec's `manifest.json` lists a tool as `forbidden`, the system shall refuse that
   tool call for that spec.
5. When a host runs `handshake bootstrap`, the system shall return the current bootstrap
   schema version and the effective policy digest.
6. The system shall not expose raw passthrough tools that duplicate the intent-level tools.

## 5. Design notes — seed for design.md
- **Module boundaries:** `internal/mcp` (transport + dispatch), `core/handshake.go`
  (bootstrap/policy schema), `manifest_tools.go` (per-spec policy); tool handlers call the
  same core functions the CLI verbs call.
- **Key types:** `Tool{Name,Schema,Handler}`; `HandshakeBootstrap{Version,PolicyDigest}`;
  `ContextManifestTools{Required,Optional,Forbidden}`.
- **Data/on-disk contracts:** `.specd/specs/<slug>/manifest.json` (tool policy); no new
  state files.
- **Invariants to preserve:** stdlib-only server; CLI↔MCP parity; deterministic tool
  results; policy fail-safe (malformed manifest → empty policy, not open).
- **External interfaces:** JSON-RPC 2.0 method names; the `--config` snippet (domain 06)
  points hosts at `specd mcp`.

## 6. Proposed task DAG — seed for tasks.md

### Wave 1 — server & core tools
| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T7.1 | craftsman | `internal/mcp/server.go`, `internal/cmd/mcp.go` | — | `echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | go run . mcp` | stdio JSON-RPC responds |
| T7.2 | craftsman | `internal/mcp/tools_core.go` | T7.1 | `go test ./internal/mcp -run TestMCPParity` | tool result == CLI JSON |
| T7.3 | craftsman | `internal/core/manifest_tools.go` | T7.1 | `go test ./internal/core -run TestForbiddenTool` | forbidden tool refused |
### Wave 2 — handshake & policy
| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T7.4 | craftsman | `internal/cmd/handshake.go`, `internal/core/handshake.go` | — | `go run . handshake bootstrap --json | grep -q version` | bootstrap returns version + policy digest |
| T7.5 | craftsman | `internal/mcp/tools_brain.go` | T7.2 | `go test ./internal/mcp -run TestBrainToolsGatedByConfig` | brain tools absent when orchestration off |
| T7.6 | validator | `internal/mcp/parity_test.go` | T7.2 | `go test ./internal/mcp -run TestMCPParity` | every exposed verb has a parity test |

## 7. Risks, open questions, cross-domain dependencies
- **Risk:** parity drift as the CLI evolves. Mitigation: data-drive tool registration from
  the command registry so there is one source (domain 10).
- **Open question:** expose `report`/`decision`/`memory` as tools in v1? Proposed: no —
  keep the tool set to the enforcement/query core; authoring tools can come later.
- **Cross-domain deps:** domain 06 (snippet points here; shares role asset map), domain 09
  (brain tools gated on orchestration), domain 10 (registry drives tool list), domains
  02–05 (tools call the same core functions).
