# Spec: MCP Config-Based Tool Filtering

> Plan items: **A1** (tool categories & selective exposure), **A3** (hide meta by
> default), **A4** (conditional orchestration exposure). Wave 1 — foundation.

## 1. Overview

`buildTools()` (`internal/mcp/tools.go:91`) currently emits one tool per non-meta
`core.Commands` entry plus all six `intentTools`, unconditionally — ~33 tools on
every `tools/list`. This spec adds an `mcp` block to `.specd/config.json` and
threads the loaded `*core.Config` into `buildTools` so the emitted set can be
filtered by exposure mode, meta gating, and orchestration gating.

This is the foundation: it changes the `buildTools` signature that every later
wave consumes. It introduces **no new tools** — only filtering of existing ones.

## 2. Goals / Non-goals

**Goals**
- Add `MCPConfig` to the `Config` struct with safe zero-value defaults.
- Filter the tool list by `expose` (`all` | `essential`), `includeMeta`, `includeOrchestration`.
- Default (absent config) is byte-identical to today's output.

**Non-goals**
- `expose: "phase"` dynamic mode (deferred to [mcp-dynamic-tool-list](../mcp-dynamic-tool-list/spec.md)).
- Composite tool merging (deferred to [mcp-composite-tools](../mcp-composite-tools/spec.md)).
- Resources/prompts.

## 3. Foundational facts (verified)

- `Config` lives in `internal/core/specfiles.go:13`; merged by `LoadConfig(root)` (line 130) via partial overlay over `DefaultConfig`. New nested structs follow the existing `OrchestrationCfg` pattern.
- `buildTools()` takes no args today; called from `route()` `tools/list` case (`internal/mcp/server.go:90`).
- `metaCommands = {help, version, mcp}` are already excluded (tools.go:12).
- `destructiveCommands = {uninstall, update}` (tools.go:24); `schema` is *not* currently flagged.
- Orchestration commands are `brain` + `pinky` (category `orchestration`, commands.go); intent tools all prefixed `brain_` (intent.go).
- `core.Config.Orchestration.Enabled` already exists (specfiles.go:25).

## 4. Requirements (EARS)

- **R1** WHEN `tools/list` is served AND no `mcp` block exists in config, THE SYSTEM SHALL emit the exact tool set produced today (full backward compatibility).
- **R2** WHEN `mcp.expose` is `"all"` (explicit or default), THE SYSTEM SHALL emit every non-meta command-mirror tool and all intent tools, subject only to R4/R5.
- **R3** WHEN `mcp.expose` is `"essential"`, THE SYSTEM SHALL emit only command-mirror tools whose command appears in `mcp.essentialTools`, plus any intent tool whose name appears there.
- **R3a** WHEN `mcp.expose` is `"essential"` AND `mcp.essentialTools` is empty, THE SYSTEM SHALL fall back to a built-in default essential set: `status, context, check, next, verify, task, approve, report`.
- **R4** WHEN `mcp.includeMeta` is false (default), THE SYSTEM SHALL exclude `update`, `uninstall`, and `schema`.
- **R5** WHEN `mcp.includeOrchestration` is false (default-when-orchestration-disabled), THE SYSTEM SHALL exclude `brain`, `pinky`, and every `brain_*` intent tool.
- **R5a** WHEN `mcp.includeOrchestration` is unset, THE SYSTEM SHALL derive it from `orchestration.enabled` (enabled ⇒ include, disabled ⇒ exclude).
- **R6** WHEN an unknown `expose` value is configured, THE SYSTEM SHALL treat it as `"all"` and write a diagnostic to stderr (never to the protocol stream).
- **R7** THE SYSTEM SHALL keep emitted tool order deterministic and stable (command order, then intent order) regardless of filtering.

## 5. Design

### 5.1 Config schema (`internal/core/specfiles.go`)
```go
type MCPConfig struct {
    Expose               string   `json:"expose"`               // "all" (default) | "essential"
    EssentialTools       []string `json:"essentialTools"`
    IncludeMeta          bool     `json:"includeMeta"`
    IncludeOrchestration *bool    `json:"includeOrchestration"` // pointer: nil => derive from orchestration.enabled
}
```
Add `MCP MCPConfig \`json:"mcp"\`` to `Config`. `IncludeOrchestration` is a
`*bool` so "unset" is distinguishable from "false" (needed for R5a). Wire merge
logic into `LoadConfig`'s overlay so a missing block stays zero-valued.

### 5.2 Default semantics
- Zero value: `Expose == ""` ⇒ treated as `"all"` (R1/R2). Empty `expose` MUST behave exactly like today.
- `DefaultConfig` does **not** set `mcp` fields (zero value is the contract); this guarantees R1 for existing `.specd/`.

### 5.3 Filtering in `buildTools`
Change signature to `buildTools(cfg *core.Config) []toolDef`. A `nil` cfg ⇒ all
(defensive, for tests). Resolution order per command:
1. skip if `metaCommands[c.Command]`.
2. skip meta-risk (`update`/`uninstall`/`schema`) unless `includeMeta`.
3. skip orchestration (`brain`/`pinky`) unless orchestration included.
4. if `expose == essential`, skip unless command ∈ resolved essential set.
Intent tools: skip all if orchestration excluded (every intent is `brain_*`);
under `essential`, keep only those named in the essential set.

Add a helper `resolveMCPExposure(cfg) exposurePlan` (pure, table-testable) that
returns the allow predicate so `buildTools` stays a thin loop and unit tests can
assert the plan directly.

### 5.4 Plumbing cfg into route
`route()`/`Serve` must hold the loaded config. `RunMCP` (`internal/cmd/mcp.go`)
already resolves `--root`; load `core.LoadConfig(root)` there and pass it down to
`Serve`/the conn so `tools/list` calls `buildTools(cfg)`. Keep the `Dispatcher`
injection pattern; add cfg as a sibling field, not a global.

### 5.5 `schema` reclassification
`schema` moves under meta-gating (it is a spec-pack-author tool per plan §A3).
It stays in `core.Commands` (CLI unaffected); only MCP exposure changes.

## 6. Acceptance criteria

- **AC1** `expose:"all"` (and absent block) ⇒ `tools/list` identical to pre-change golden.
- **AC2** `expose:"essential"` with default set ⇒ exactly 8 tools, all `specd_<essential>`.
- **AC3** `includeMeta:false` ⇒ no `specd_update`/`specd_uninstall`/`specd_schema`.
- **AC4** orchestration disabled ⇒ no `specd_brain`/`specd_pinky`/`brain_*`.
- **AC5** orchestration enabled + `expose:"all"` ⇒ orchestration + intent tools present.
- **AC6** unknown `expose` ⇒ all tools + one stderr line; protocol stream clean.

## 7. Testing (per plan §7)
- Unit: `resolveMCPExposure` table tests over the config matrix.
- Unit: `tools_test.go` asserts filtered counts/names per config variant.
- Integration: `integration_test.go` starts server with each config, calls `tools/list`, asserts count.
- Backward-compat: golden test pins `expose:"all"` output byte-for-byte.

## 8. Risks
- **Signature change** ripples to all `buildTools` callers/tests → mechanical, caught by compiler.
- **`*bool` JSON merge** in `LoadConfig` overlay — ensure nil stays nil when key absent (cover with test).
