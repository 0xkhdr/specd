# specd Configuration System Analysis & Action Plan

## 1. Executive Summary

This document analyzes the current configuration architecture of **specd** and provides a concrete action plan to migrate from a single JSON-only project config (`.specd/config.json`) to a **dual-layer, format-agnostic configuration system** supporting both **global (static)** and **per-project (dynamic)** configs with **YAML as the primary human-facing format** and **JSON retained for machine-generated state files**.

---

## 2. Current State Analysis

### 2.1 Existing Configuration Architecture

| Aspect | Current Implementation |
|--------|----------------------|
| **Config File** | `.specd/config.json` (single file, project-scoped only) |
| **Format** | JSON exclusively |
| **Embedding** | `internal/core/embed_templates/config.json` — hardcoded default scaffold |
| **Path Resolution** | `ConfigPath(root)` → `filepath.Join(root, ".specd", "config.json")` |
| **Loading** | `LoadConfig(root)` — reads JSON, returns `Config` struct with defaults fallback |
| **Validation** | `LoadConfigStrict(root)` — enum validation, range clamping, diagnostic collection |
| **Environment Override** | `SPECD_*` env vars (e.g., `SPECD_JSON`, `SPECD_VERIFY_TIMEOUT_MS`) |
| **State Separation** | `state.json` (per-spec runtime state) is separate from `config.json` |

### 2.2 Current Config Schema (`.specd/config.json`)

```json
{
  "version": 1,
  "defaultVerify": "npm test",
  "report": { "format": "md", "autoRefreshSeconds": 0 },
  "roles": { "subagentMode": "inline" },
  "promotionThreshold": 3,
  "gates": {
    "traceability": "warn",
    "acceptance": "off",
    "scope": "off",
    "custom": []
  },
  "verify": { "sandbox": "none" },
  "orchestration": {
    "enabled": false,
    "approvalPolicy": "manual",
    "workerMode": "host",
    "maxWorkers": 4,
    "maxRetries": 2,
    "sessionTimeoutMinutes": 120,
    "hostReportedCostLimitUSD": 0,
    "transport": {
      "kind": "file",
      "pollIntervalMillis": 500,
      "messageTTLSeconds": 3600,
      "leaseSeconds": 120,
      "heartbeatSeconds": 30
    },
    "program": { "maxConcurrentSpecs": 2 }
  }
}
```

### 2.3 Pain Points Identified

1. **No Global Config**: Every project duplicates static settings (e.g., `defaultVerify`, `roles.subagentMode`, `gates.custom`).
2. **JSON-Only for Human-Edited Files**: JSON is verbose for human editing — no comments, trailing commas forbidden, noisy quoting.
3. **No Config Hierarchy**: Cannot inherit global defaults and override per-project.
4. **Flat Namespace**: All keys live at the same level; `orchestration.transport.leaseSeconds` is deeply nested while `promotionThreshold` is flat — inconsistent depth.
5. **No Format Flexibility**: The codebase hardcodes `.json` extension in `ConfigPath()`, `scaffold.go`, and `config_validate.go`.
6. **Mixed Concerns**: Static preferences (report format, roles) coexist with dynamic runtime tuning (orchestration workers, timeouts) in the same file.

---

## 3. Target Architecture

### 3.1 Design Principles

| Principle | Rationale |
|-----------|-----------|
| **YAML for Humans, JSON for Machines** | YAML supports comments, is less verbose, and is the de-facto standard in Go tooling (e.g., `golangci.yml`, `.goreleaser.yml`, Kubernetes manifests). |
| **Global → Project Cascade** | Static, cross-project defaults live in global config. Dynamic, project-specific overrides live in project config. |
| **Explicit Namespacing** | Every key must reveal its domain (`verify.sandbox`, `orchestration.transport.lease_seconds`). |
| **Backward Compatibility** | Existing `.specd/config.json` continues to work; new files are opt-in with a migration path. |
| **Fail-Closed Validation** | Invalid config surfaces diagnostics, never silently falls back to dangerous defaults. |

### 3.2 Config File Locations

```
~/.config/specd/config.yml          # Global static config (user-level defaults)
~/.specd.yml                         # Alternative global config (XDG fallback)
<project>/.specd/config.yml          # Project dynamic config (overrides global)
<project>/.specd/config.json         # Legacy support (deprecated, still parsed)
```

### 3.3 Proposed YAML Schema

#### Global Config (`~/.config/specd/config.yml`)

```yaml
# specd global configuration
# Static defaults applied across all projects unless overridden.

version: 2

# --- Core Defaults ---
defaults:
  verify_command: "npm test"
  report_format: md
  subagent_mode: inline
  promotion_threshold: 3

# --- Gate Policies ---
gates:
  traceability: warn
  acceptance: off
  scope: off
  context_budget: off
  custom: []

# --- Verification ---
verify:
  sandbox: none

# --- Orchestration Defaults ---
orchestration:
  enabled: false
  approval_policy: manual
  worker_mode: host
  max_workers: 4
  max_retries: 2
  session_timeout_minutes: 120
  host_reported_cost_limit_usd: 0
  transport:
    kind: file
    poll_interval_seconds: 0.5        # human-friendly: 500ms → 0.5s
    message_ttl_minutes: 60           # human-friendly: 3600s → 60m
    lease_seconds: 120
    heartbeat_seconds: 30
  program:
    max_concurrent_specs: 2

# --- MCP Defaults ---
mcp:
  expose: all
```

#### Project Config (`<project>/.specd/config.yml`)

```yaml
# specd project configuration
# Overrides global defaults for this repository only.

version: 2

# Inherit everything from global; only declare overrides.
# This keeps the project config minimal and focused.

defaults:
  verify_command: "go test ./..."
  report_format: html

gates:
  acceptance: warn
  scope: error

orchestration:
  enabled: true
  approval_policy: planning
  max_workers: 8
```

### 3.4 Format Decision Matrix

| File Type | Format | Reason |
|-----------|--------|--------|
| Global config | YAML | Human-edited, cross-project, needs comments. |
| Project config | YAML | Human-edited, per-project overrides, needs comments. |
| `state.json` | **JSON** | Machine-generated, agent-consumed, needs deterministic byte stability. |
| `program.json` | **JSON** | Machine-generated, inter-spec dependency graph. |
| `session.json` | **JSON** | Machine-generated, Brain/Pinky runtime state. |
| `integrations.json` | **JSON** | Machine-generated, host integration metadata. |
| Embedded templates | YAML | Human-readable defaults in source. |

---

## 4. Action Plan

### Phase 1: Foundation — YAML Parser & Config Loader

**Goal**: Introduce YAML parsing without breaking existing JSON support.

| Task | File(s) | Details |
|------|---------|---------|
| **1.1** Add YAML dependency | `go.mod` | Add `gopkg.in/yaml.v3` (standard, no CGO, MIT license). |
| **1.2** Create `internal/core/config_loader.go` | New file | Implement `LoadConfigFromPath(path string) (Config, []ConfigDiagnostic)` that auto-detects format by extension (`.yml` → YAML, `.json` → JSON). |
| **1.3** Create `internal/core/config_merge.go` | New file | Implement `MergeConfig(global, project Config) Config` — deep merge where project values override global. Nil/empty project values preserve global. |
| **1.4** Update `ConfigPath()` logic | `internal/core/paths.go` | Rename `ConfigPath()` → `LegacyConfigPath()`. Add `ConfigPaths(root) []string` that returns candidate paths in priority order: `[config.yml, config.yaml, config.json]`. |
| **1.5** Global config path resolver | `internal/core/paths.go` | Add `GlobalConfigPaths() []string` returning `[$XDG_CONFIG_HOME/specd/config.yml, $XDG_CONFIG_HOME/specd/config.yaml, ~/.config/specd/config.yml, ~/.config/specd/config.yaml, ~/.specd.yml]`. |

### Phase 2: Schema Refactoring — Meaningful Namespacing

**Goal**: Restructure the `Config` struct for clarity and consistency.

| Task | File(s) | Details |
|------|---------|---------|
| **2.1** Rename `Config` struct fields | `internal/core/config.go` | Adopt snake_case YAML tags while keeping camelCase JSON tags for backward compatibility. |
| **2.2** Group under semantic namespaces | `internal/core/config.go` | Restructure to match the YAML schema above: `defaults`, `gates`, `verify`, `orchestration`, `mcp`. |
| **2.3** Human-friendly time units | `internal/core/config.go` | Parse `poll_interval_seconds`, `message_ttl_minutes` in YAML; convert to millis internally. Keep JSON tags as raw millis for backward compat. |
| **2.4** Update `DefaultConfig` | `internal/core/config.go` | Populate from an embedded YAML template instead of JSON. |
| **2.5** Update validation | `internal/core/config_validate.go` | Validate both YAML and JSON paths. Add new enum/range checks for renamed fields. |

#### Proposed `Config` Struct (YAML + JSON dual-tag)

```go
type Config struct {
    Version int `json:"version" yaml:"version"`

    Defaults struct {
        VerifyCommand       string `json:"defaultVerify" yaml:"verify_command"`
        ReportFormat        string `json:"reportFormat" yaml:"report_format"`
        SubagentMode        string `json:"subagentMode" yaml:"subagent_mode"`
        PromotionThreshold  int    `json:"promotionThreshold" yaml:"promotion_threshold"`
    } `json:"defaults" yaml:"defaults"`

    Gates struct {
        Traceability string        `json:"traceability" yaml:"traceability"`
        Acceptance   string        `json:"acceptance" yaml:"acceptance"`
        Scope        string        `json:"scope" yaml:"scope"`
        ContextBudget string       `json:"contextBudget" yaml:"context_budget"`
        Custom       []CustomGate `json:"custom" yaml:"custom"`
    } `json:"gates" yaml:"gates"`

    Verify struct {
        Sandbox string `json:"sandbox" yaml:"sandbox"`
    } `json:"verify" yaml:"verify"`

    Orchestration OrchestrationCfg `json:"orchestration" yaml:"orchestration"`

    MCP struct {
        Expose string `json:"expose" yaml:"expose"`
    } `json:"mcp" yaml:"mcp"`
}
```

### Phase 3: Global Config Integration

**Goal**: Enable user-level defaults that cascade into projects.

| Task | File(s) | Details |
|------|---------|---------|
| **3.1** Update `LoadConfig(root)` | `internal/core/config_loader.go` | New signature: `LoadConfig(root string) (Config, []ConfigDiagnostic)`. Internally: load global → load project → merge → validate. |
| **3.2** Update `LoadConfigStrict(root)` | `internal/core/config_loader.go` | Same merge-then-validate pattern; diagnostics include source file path (global vs project). |
| **3.3** Scaffold global config | `internal/cmd/init.go` | On `specd init`, if no global config exists, write `~/.config/specd/config.yml` with the embedded YAML template. Print a one-line notice. |
| **3.4** `specd doctor` global check | `internal/cmd/doctor.go` | Verify global config parseability and warn if it uses deprecated JSON. |

### Phase 4: Scaffold & Template Migration

**Goal**: New projects get YAML by default; legacy projects keep working.

| Task | File(s) | Details |
|------|---------|---------|
| **4.1** Create `embed_templates/config.yml` | `internal/core/embed_templates/` | YAML version of the current `config.json` with comments. |
| **4.2** Update `DefaultScaffoldManifest()` | `internal/core/scaffold.go` | Change `config.json` → `config.yml` in the scaffold manifest. Policy: `ScaffoldCreate`. |
| **4.3** Update `init.go` | `internal/cmd/init.go` | Write `config.yml` instead of `config.json`. |
| **4.4** Legacy detection | `internal/cmd/init.go` | If `config.json` exists but `config.yml` does not, print a deprecation notice: `"config.json is deprecated; run specd migrate config to convert to config.yml."` |

### Phase 5: Migration Command

**Goal**: Provide a safe, deterministic path for existing users.

| Task | File(s) | Details |
|------|---------|---------|
| **5.1** `specd migrate config` | `internal/cmd/migrate.go` (new) | Reads `.specd/config.json` → converts to `.specd/config.yml` with comments preserved (best effort) → writes `.specd/config.yml` → renames `.specd/config.json` to `.specd/config.json.bak`. |
| **5.2** `specd migrate config --dry-run` | `internal/cmd/migrate.go` | Preview the YAML output without writing. |
| **5.3** `specd migrate config --global` | `internal/cmd/migrate.go` | Migrate `~/.config/specd/config.json` → `config.yml` if it exists. |

### Phase 6: Environment Variable Alignment

**Goal**: Ensure env vars still override both global and project configs.

| Task | File(s) | Details |
|------|---------|---------|
| **6.1** Update env resolution | `internal/core/env.go` | After loading merged config, apply `SPECD_*` env overrides as today. Document precedence: **Env > Project YAML > Global YAML > Embedded Defaults**. |
| **6.2** Add `SPECD_CONFIG_FORMAT` | `internal/core/env.go` | Optional: force format preference (`json` or `yaml`) for debugging. |

### Phase 7: Documentation & Testing

| Task | File(s) | Details |
|------|---------|---------|
| **7.1** Update `docs/command-reference.md` | `docs/command-reference.md` | Document new config file locations, precedence rules, and `specd migrate config`. |
| **7.2** Update `docs/user-guide.md` | `docs/user-guide.md` | Add "Global vs Project Config" section with YAML examples. |
| **7.3** Add YAML round-trip tests | `internal/core/config_loader_test.go` | Parse YAML → serialize to JSON → parse JSON → assert equal. |
| **7.4** Add merge tests | `internal/core/config_merge_test.go` | Test deep merge: global `orchestration.max_workers: 4` + project `orchestration.enabled: true` → result has both. |
| **7.5** Add backward-compat tests | `internal/core/config_loader_test.go` | Ensure `config.json` from v1 still parses and produces identical runtime behavior. |

---

## 5. File Impact Map

```
internal/core/
├── config.go              ← MODIFY: restructure Config struct, add YAML tags
├── config_validate.go     ← MODIFY: validate both formats, add path diagnostics
├── config_loader.go       ← NEW: unified LoadConfig with format detection + merge
├── config_merge.go        ← NEW: deep merge logic
├── paths.go               ← MODIFY: add GlobalConfigPaths(), ConfigPaths()
├── env.go                 ← MODIFY: env override after merge
├── embed_templates/
│   ├── config.json        ← KEEP: legacy embedded default
│   └── config.yml         ← NEW: YAML embedded default with comments
├── scaffold.go            ← MODIFY: point to config.yml

internal/cmd/
├── init.go                ← MODIFY: write config.yml, scaffold global config
├── doctor.go              ← MODIFY: check global config health
└── migrate.go             ← NEW: specd migrate config command

docs/
├── command-reference.md     ← MODIFY: config schema + env vars
├── user-guide.md           ← MODIFY: global vs project config section
└── contributor-guide.md    ← MODIFY: config architecture notes
```

---

## 6. Risk Assessment & Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| **Breaking existing `config.json` users** | High | Keep JSON parser forever; `config.json` is still a valid candidate in `ConfigPaths()`. Only new projects get YAML. |
| **YAML parsing ambiguity** | Medium | Use `yaml.v3` (strict mode where possible). Reject unknown keys with warnings, not errors. |
| **Time-unit conversion bugs** | Medium | Store internal representation in original units (ms). YAML parser converts human units → internal units at load time. JSON path bypasses conversion. |
| **Global config drift** | Low | `specd doctor` validates global config. `specd init` scaffolds a commented global config on first run. |
| **XDG path portability** | Low | Use `os.UserConfigDir()` for global path; fallback to `~/.config/` then `~/.specd.yml`. |

---

## 7. Acceptance Criteria

- [ ] `specd init` scaffolds `.specd/config.yml` (not `.json`) for new projects.
- [ ] `~/.config/specd/config.yml` is created on first `specd init` if absent.
- [ ] Existing `.specd/config.json` continues to parse without modification.
- [ ] Project `config.yml` overrides global `config.yml` for the same key.
- [ ] `specd migrate config` converts `.specd/config.json` → `.specd/config.yml` deterministically.
- [ ] `specd doctor` reports global config health.
- [ ] All `SPECD_*` env vars still override merged config values.
- [ ] YAML config supports comments and is validated with the same strictness as JSON.
- [ ] `state.json`, `program.json`, `session.json` remain JSON (machine files unchanged).
- [ ] Documentation reflects the new dual-layer, format-agnostic architecture.

---

## 8. Appendix: Precedence Ladder

```
1. Environment Variable (SPECD_*)
       ↓
2. Project Config (.specd/config.yml or .specd/config.json)
       ↓
3. Global Config (~/.config/specd/config.yml)
       ↓
4. Embedded Defaults (internal/core/embed_templates/config.yml)
```

*Lower number wins. Empty/zero values in a higher layer do NOT fall through; only absent keys fall through.*
