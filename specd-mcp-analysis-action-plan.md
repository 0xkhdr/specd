# specd MCP Integration: Analysis & Action Plan

> **Date:** 2026-06-20  
> **Focus:** MCP server architecture, tool definition bloat, and production-grade selective loading  
> **Scope:** `internal/mcp/*`, `internal/cmd/mcp.go`, `internal/core/commands.go`

---

## 1. Executive Summary

`specd` exposes its entire CLI command surface (~25+ commands) as MCP tools via a thin JSON-RPC 2.0 transport layer. The current implementation in `internal/mcp/tools.go` generates **one tool per command** automatically at startup (`buildTools()`), with no filtering, grouping, or lazy-loading mechanism. This creates a **context bloat problem** for MCP hosts: every tool definition (name, description, input schema, annotations) is transmitted to the LLM on every `tools/list` call and remains in the host's tool registry for the session lifetime.

For production-grade MCP integrations, the industry consensus (and emerging MCP spec direction) is that **tool lists should be scoped, namespaced, and lazily discoverable** — not monolithic dumps. The current specd approach violates this by unconditionally surfacing all commands including meta/install commands (`update`, `uninstall`), orchestration internals (`brain`, `pinky`), and infrequent operations as first-class tools.

---

## 2. Current Architecture Deep Dive

### 2.1 Tool Generation Pipeline

```
internal/core/commands.go  →  CommandMeta[] (registry)
         │
         ▼
internal/mcp/tools.go      →  buildTools()
         │
    ┌────┴────┐
    │         │
commandToTool()  intentTools[]
    │         │
    └────┬────┘
         ▼
    toolDef[] (~30 tools)
         │
         ▼
internal/mcp/server.go     →  tools/list RPC response
```

**Key files:**
- `internal/mcp/server.go` — JSON-RPC 2.0 loop, `initialize`, `tools/list`, `tools/call`
- `internal/mcp/tools.go` — `buildTools()`, `commandToTool()`, intent-level wrappers
- `internal/mcp/transport.go` — Framing auto-detection (newline-delimited vs Content-Length)
- `internal/mcp/transport_http.go` — HTTP/SSE transport with `dispatchLocked` mutex
- `internal/cmd/mcp.go` — `RunMCP()`, `--config`, `--root`, `--http` flags

### 2.2 Tool Surface Area (Current)

| Category | Tools | Count |
|----------|-------|-------|
| **Lifecycle** | `specd_init`, `specd_doctor`, `specd_new`, `specd_approve` | 4 |
| **Execution** | `specd_next`, `specd_dispatch`, `specd_verify`, `specd_task` | 4 |
| **Inspection** | `specd_status`, `specd_check`, `specd_waves`, `specd_context`, `specd_report`, `specd_serve`, `specd_watch`, `specd_validate`, `specd_replay`, `specd_diff` | 10 |
| **Records** | `specd_decision`, `specd_midreq`, `specd_memory` | 3 |
| **Program** | `specd_program` | 1 |
| **Orchestration** | `specd_brain`, `specd_pinky` | 2 |
| **Meta/Destructive** | `specd_schema`, `specd_update`, `specd_uninstall` | 3 |
| **Intent-level** | `brain_orchestrate`, `brain_status`, `brain_approve`, `brain_pause`, `brain_resume`, `brain_cancel` | 6 |
| **TOTAL** | | **~33 tools** |

### 2.3 The Context Bloat Problem

**Why this matters:**

1. **Token consumption:** Every tool definition includes a name, description, and JSON Schema. 33 tools × ~200 tokens each ≈ **6,600+ tokens** consumed before any user message.
2. **LLM confusion:** Large tool lists degrade tool-selection accuracy. Models perform worse when choosing from 30+ options vs. 5-8 focused ones.
3. **Host limitations:** Some MCP hosts impose hard limits on tool count or total schema size.
4. **Noise ratio:** A user working on a single spec in the EXECUTE phase has zero need for `specd_uninstall`, `specd_update`, `specd_schema`, or `specd_replay` in their context.

### 2.4 Current Limitations (from `docs/mcp-guide.md`)

| Limitation | Impact on Context Bloat |
|------------|------------------------|
| `listChanged: false` | Host cannot dynamically prune tools; static list is loaded once |
| No resources or prompts | All context must flow through tools; no alternative discovery channel |
| Serial tool calls | Not directly related, but reinforces that tool design should be coarse-grained |
| Static tool list for process lifetime | Cannot adapt to phase/project state |

---

## 3. Production-Grade Best Practices for MCP Tool Management

### 3.1 Principle: Progressive Disclosure

> **Only expose tools the user can meaningfully invoke right now.**

This is the single most important principle. Tools should be:
- **Phase-scoped:** Different tools for PLAN vs EXECUTE vs VERIFY
- **Role-scoped:** Investigators need read-only; builders need write
- **State-scoped:** Don't show `specd_approve` if no spec is awaiting approval

### 3.2 Principle: Namespace Hierarchies

Use tool names as namespaces:
- `specd_read_*` — all read-only inspection tools
- `specd_write_*` — all state-mutating tools
- `specd_meta_*` — install/update/uninstall (rarely needed)
- `specd_orchestrate_*` — brain/pinky (only when orchestration enabled)

This lets hosts filter by prefix and lets users mentally model the surface.

### 3.3 Principle: Coarse over Fine

Prefer **fewer, more powerful tools** over many atomic ones:
- Instead of `specd_status`, `specd_waves`, `specd_context` as separate tools → one `specd_read_state` with a `--view` parameter
- Instead of `brain_orchestrate`, `brain_status`, `brain_pause` → one `specd_orchestrate` with `--action`

This is the **opposite** of the current approach where every CLI flag becomes a tool argument.

### 3.4 Principle: Lazy Discovery via Resources (Future-Proofing)

The MCP spec is evolving toward **Resources** and **Prompts** as first-class citizens. When specd implements these:
- `resources/specs/{slug}` → spec artifacts as readable resources (no tool needed)
- `prompts/phase/{phase}` → phase-specific system prompts
- Tools then become **action verbs** only: `create`, `update`, `verify`, `approve`

### 3.5 Principle: Host-Aware Contract

The server should negotiate with the host:
- `initialize` params could include `preferredToolCount` or `toolNamespaces`
- Server responds with a filtered subset
- This is speculative (not in current MCP spec) but aligns with protocol evolution

---

## 4. Recommended Action Plan

### Phase A: Immediate Mitigation (Low Risk, High Impact)

**Goal:** Reduce tool count from ~33 to ~8-12 without breaking existing integrations.

#### A1. Introduce Tool Categories & Selective Exposure

Add a `mcp` section to `.specd/config.json`:

```json
{
  "mcp": {
    "expose": "essential",
    "essentialTools": [
      "status", "context", "check", "next",
      "verify", "task", "approve", "report"
    ],
    "includeOrchestration": false,
    "includeMeta": false
  }
}
```

**Behavior:**
- `"expose": "all"` → current behavior (backward compatible)
- `"expose": "essential"` → only listed tools
- `"expose": "phase"` → dynamic based on active spec phase (advanced)

**Implementation:**
- `internal/mcp/tools.go`: `buildTools()` reads config before generating tool list
- Filter `core.Commands` against allowlist
- Intent tools follow same filtering (if `brain` is excluded, intent tools are too)

#### A2. Merge Read-Only Inspection Tools

Collapse the 10 inspection tools into **3 composite tools**:

| New Tool | Replaces | Parameters |
|----------|----------|------------|
| `specd_inspect` | `status`, `waves`, `context`, `check`, `validate`, `replay`, `diff` | `--view status\|waves\|context\|check\|validate\|replay\|diff` |
| `specd_read` | `report`, `serve`, `watch` | `--format md\|html`, `--once` |
| `specd_query` | `next`, `dispatch` | `--frontier`, `--all`, `--json` |

**Benefits:**
- Reduces inspection surface from 10 → 3 tools
- Single schema with enum parameter is smaller than 10 separate schemas
- LLM has one "read" affordance; `--view` guides it

#### A3. Hide Meta/Destructive by Default

`specd_update`, `specd_uninstall`, `specd_schema` should be **opt-in** via config:

```json
{
  "mcp": {
    "includeMeta": false
  }
}
```

These are:
- `update` — modifies the specd binary itself (rare, dangerous)
- `uninstall` — destructive to the harness
- `schema` — useful for spec pack authors only

#### A4. Namespace Orchestration Tools

When orchestration is **disabled** in config (`orchestration.enabled: false`), **do not expose** `specd_brain`, `specd_pinky`, or any intent-level brain tools.

When enabled, expose them under a clear prefix: `specd_orchestrate_*` instead of mixing `brain_` and `specd_brain`.

**Current mess:**
- `specd_brain` (raw passthrough)
- `brain_orchestrate` (intent)
- `brain_status` (intent)
- `specd_pinky` (raw)

**Clean:**
- `specd_orchestrate` (unified, `--action start\|step\|status\|pause\|resume\|cancel`)
- `specd_worker` (replaces `pinky`, `--action claim\|heartbeat\|progress\|query\|report\|block\|release`)

### Phase B: Structural Refactoring (Medium Risk, High Value)

#### B1. Implement MCP Resources (Spec-Artifact-as-Resource)

Add `resources/list` and `resources/read` handlers:

```
resource://specs/{slug}/requirements.md
resource://specs/{slug}/design.md
resource://specs/{slug}/tasks.md
resource://specs/{slug}/state.json
resource://steering/reasoning.md
resource://steering/workflow.md
```

**Impact:**
- `specd_context` tool can be deprecated in favor of `resources/read`
- `specd_status` can return lightweight metadata; full artifacts via resources
- Reduces tool definitions further (no need for artifact-reading tools)

#### B2. Implement MCP Prompts (Phase-Specific Prompts)

Add `prompts/list` and `prompts/get`:

```
prompt://phase/requirements
prompt://phase/design
prompt://phase/tasks
prompt://phase/execute
prompt://role/builder
prompt://role/investigator
```

**Impact:**
- Steering files and role prompts become native MCP prompts
- Agents load them via `prompts/get` instead of `specd_context` tool calls
- Further reduces tool surface

#### B3. Dynamic Tool List (`tools/listChanged: true`)

When the spec phase changes (requirements → design → tasks → executing), the server could:
1. Update its internal tool filter
2. Send `notifications/tools/list_changed` to the host
3. Host re-fetches `tools/list` and gets a phase-appropriate subset

**Requires:**
- `capabilities.tools.listChanged: true` in initialize response
- Background goroutine watching `.specd/` for phase transitions
- Thread-safe tool list updates

### Phase C: Advanced Optimization (Speculative, Future-Proof)

#### C1. Context-Manifest-Driven Tool Loading

Leverage specd's existing `contextManifest` concept (from `specd pinky brief`):

```json
{
  "contextManifest": {
    "requiredTools": ["specd_inspect", "specd_verify", "specd_task"],
    "optionalTools": ["specd_decision", "specd_memory"],
    "forbiddenTools": ["specd_approve"]
  }
}
```

The MCP server reads the active spec's manifest and filters tools accordingly.

#### C2. Host Capability Negotiation

Extend `initialize` params:

```json
{
  "protocolVersion": "2024-11-05",
  "clientInfo": { "name": "claude-code", "version": "1.2.3" },
  "capabilities": {
    "tools": { "listChanged": true },
    "specd": {
      "maxTools": 16,
      "preferredNamespaces": ["specd_read", "specd_write"]
    }
  }
}
```

Server respects `maxTools` and `preferredNamespaces` when building the tool list.

---

## 5. Implementation Priority Matrix

| Item | Effort | Risk | Impact | Phase |
|------|--------|------|--------|-------|
| A1: Config-based tool filtering | Low | Low | High | Phase A |
| A2: Merge inspection tools | Low | Medium | High | Phase A |
| A3: Hide meta by default | Low | Low | Medium | Phase A |
| A4: Conditional orchestration exposure | Low | Low | Medium | Phase A |
| B1: MCP Resources | Medium | Medium | Very High | Phase B |
| B2: MCP Prompts | Medium | Medium | High | Phase B |
| B3: Dynamic tool list changes | Medium | Medium | High | Phase B |
| C1: Context manifest tools | High | Medium | Medium | Phase C |
| C2: Host negotiation | High | High | Low | Phase C |

---

## 6. Code-Level Implementation Sketch

### 6.1 Config Schema Extension

```go
// internal/core/specfiles.go — add to Config struct
type MCPConfig struct {
    Expose               string   `json:"expose"`                // "all" | "essential" | "phase"
    EssentialTools       []string `json:"essentialTools"`
    IncludeOrchestration bool     `json:"includeOrchestration"`
    IncludeMeta          bool     `json:"includeMeta"`
}
```

### 6.2 Tool Filtering in `buildTools()`

```go
// internal/mcp/tools.go
func buildTools(cfg *core.Config) []toolDef {
    tools := make([]toolDef, 0)

    allowed := make(map[string]bool)
    for _, t := range cfg.MCP.EssentialTools {
        allowed[t] = true
    }

    for _, c := range core.Commands {
        if metaCommands[c.Command] {
            continue
        }
        if cfg.MCP.Expose == "essential" && !allowed[c.Command] {
            continue
        }
        if !cfg.MCP.IncludeMeta && (c.Command == "update" || c.Command == "uninstall") {
            continue
        }
        if !cfg.MCP.IncludeOrchestration && (c.Command == "brain" || c.Command == "pinky") {
            continue
        }
        tools = append(tools, commandToTool(c))
    }

    // Intent tools follow same filtering
    for _, it := range intentTools {
        if cfg.MCP.Expose == "essential" && !allowed[it.name] {
            continue
        }
        tools = append(tools, it.def())
    }

    return tools
}
```

### 6.3 Composite Inspection Tool

```go
// internal/cmd/inspect.go — new command
func RunInspect(args cli.Args) int {
    view := args.Str("view")
    switch view {
    case "status": return runStatus(args)
    case "waves": return runWaves(args)
    case "context": return runContext(args)
    case "check": return runCheck(args)
    // ... etc
    default:
        core.Error("unknown view: " + view)
        return core.ExitUsage
    }
}
```

Register in `internal/cmd/registry.go` and `internal/core/commands.go`.

### 6.4 Resource Handler Skeleton

```go
// internal/mcp/resources.go
func handleResourcesList() map[string]any {
    return map[string]any{
        "resources": []map[string]any{
            {"uri": "specd://specs/{slug}/requirements.md", "name": "Spec Requirements"},
            {"uri": "specd://specs/{slug}/design.md", "name": "Spec Design"},
            {"uri": "specd://specs/{slug}/tasks.md", "name": "Spec Tasks"},
            {"uri": "specd://specs/{slug}/state.json", "name": "Spec State"},
        },
    }
}

func handleResourceRead(uri string) (string, error) {
    // Parse URI, read from .specd/specs/{slug}/...
    // Return markdown or JSON content
}
```

Wire into `route()` in `server.go`.

---

## 7. Testing Strategy

1. **Unit tests:** `tools_test.go` asserting filtered tool lists per config variant
2. **Integration tests:** Start MCP server with different configs, call `tools/list`, assert count
3. **Round-trip tests:** Ensure composite tools (`specd_inspect`) produce identical output to their atomic predecessors
4. **Backward compatibility:** `expose: "all"` must produce the exact same tool list as today
5. **Concurrency:** HTTP transport with dynamic tool list updates must remain thread-safe

---

## 8. Migration Path for Existing Users

1. **Default behavior unchanged:** New config fields are optional; missing = `"expose": "all"`
2. **Opt-in optimization:** Users add `"mcp": { "expose": "essential" }` to `.specd/config.json`
3. **`specd doctor` enhancement:** Detects large tool lists and suggests config optimization
4. **Documentation:** Update `docs/mcp-guide.md` with new config options and best practices

---

## 9. Key Takeaways

| Problem | Root Cause | Solution |
|---------|-----------|----------|
| 33 tools in context | `buildTools()` emits all commands unconditionally | Config-based filtering + composite tools |
| Meta tools visible | No categorization of tool risk | `includeMeta: false` by default |
| Orchestration tools always visible | No conditional gating on `orchestration.enabled` | Gate tool exposure on config |
| No resource/prompt support | Only `tools` capability implemented | Phase B: Add resources + prompts |
| Static list | `listChanged: false` | Phase B: Dynamic updates on phase change |

**The golden rule:** *The agent reasons. The harness enforces. The MCP server should only expose what the agent needs to reason about right now.*
