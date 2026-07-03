# Spec 07 — MCP & Handshake Surface

> **Authoring order:** 9 / 12 · **Critical path:** feeds 09
> **Sources:** `fresh-start/07-mcp-handshake-surface.md`, paper p.28
> **ADRs:** ADR-3 (brain tools gated), ADR-8
> **Reference:** `reference/internal/mcp/*`, `reference/internal/cmd/{mcp,handshake}.go`, `reference/internal/core/{handshake,manifest_tools}.go`, `reference/docs/mcp-guide.md`

MCP is how specd's deterministic capabilities become callable tools inside any MCP-speaking
host. A small, parity-tested tool set plus a handshake that hands the host its policy digest.

---

## 1. Purpose & principles
- **Principles owned:** P5 (Agent-Agnostic by Design), P1 (harness exposes enforcement, not
  authorship, as tools).
- **Paper concept:** *tools* — "the developer defines the tools the agent will have access to"
  (p.28).

## 2. Verdicts (with citations)

| Capability | Verdict | Why / reference |
|---|---|---|
| stdio JSON-RPC 2.0 server | **KEEP** | stdlib-only portable floor. `reference/internal/mcp/*` |
| HTTP JSON-RPC | **DEFER** | stdio covers MVP hosts |
| Core lifecycle tools (`check,next,verify,task,status,context,handshake`) | **KEEP** as the parity-tested set | |
| Six brain intent tools | **SIMPLIFY to 3** (`brain_orchestrate,brain_status,brain_approve`), registered only when orchestration enabled | ADR-3 |
| Raw passthroughs (`specd_brain`, `specd_pinky`) | **CUT** | No added authority; double surface |
| Handshake bootstrap + policy digest | **KEEP** | P5 on-ramp. `reference/internal/core/handshake.go` |
| Per-spec tool policy (`manifest_tools.go`) | **KEEP** | Real guardrail (forbid/require tools) |

**MCP tools (v1):** `specd_check`, `specd_next`, `specd_verify`, `specd_task`, `specd_status`,
`specd_context`, `specd_handshake` (+ 3 brain tools when enabled). **Commands:** `mcp` (serve
stdio), `handshake [bootstrap|policy]`.

## 3. Requirements (EARS)
- **R7.1** When `mcp` is invoked, the system shall serve JSON-RPC 2.0 over stdio using only the
  Go standard library.
- **R7.2** When a host calls a core specd tool, the system shall return a result byte-equal to
  the corresponding CLI command's JSON output for the same inputs.
- **R7.3** When orchestration is disabled, the system shall not register any brain MCP tool.
- **R7.4** When a spec's `manifest.json` lists a tool as `forbidden`, the system shall refuse
  that tool call for that spec (malformed manifest → empty policy, fail-safe, not open).
- **R7.5** When a host runs `handshake bootstrap`, the system shall return the current
  bootstrap schema version and the effective policy digest.
- **R7.6** The system shall not expose raw passthrough tools that duplicate the intent-level
  tools.

## 4. Design

### Module boundaries
- `internal/mcp` — transport + dispatch. `core/handshake.go` — bootstrap/policy schema.
- `manifest_tools.go` — per-spec policy. Tool handlers call the **same core functions the CLI
  verbs call**; tool registration is data-driven from the command registry (Spec 10).

### Key types
- `Tool{Name, Schema, Handler}`; `HandshakeBootstrap{Version, PolicyDigest}`;
  `ContextManifestTools{Required, Optional, Forbidden}`.

### On-disk contracts
- `.specd/specs/<slug>/manifest.json` (tool policy); no new state files.

### External interfaces
- JSON-RPC 2.0 method names; the `--config` snippet (Spec 06) points hosts at `specd mcp`.

## 5. Invariants preserved (ADR-8)
stdlib-only server; **CLI↔MCP parity** (mirrors the registry↔help guard); deterministic tool
results; policy fail-safe (malformed manifest → empty policy, not open).

## 6. Cross-domain dependencies
- Depends on: Spec 06 (snippet + asset map), Spec 10 (registry drives tool list), Specs 02–05
  (tools call the same core functions), Spec 08 (`specd_context`).
- Feeds: Spec 09 (brain tools gated on orchestration).

## 7. Risks & open questions
- **Risk:** parity drift as the CLI evolves. → data-drive tool registration from the command
  registry so there is one source (Spec 10).
- **Open:** expose `report`/`decision`/`memory` as tools in v1? **Proposed:** no — keep the
  tool set to the enforcement/query core; authoring tools can come later.
