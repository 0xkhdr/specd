# Tasks: MCP Config-Based Tool Filtering

Legend: `[ ]` todo · `[~]` wip · `[x]` done. Each task lists deps, files, verify.

## T1 — Add `MCPConfig` to Config struct
- **Deps:** none
- **Files:** `internal/core/specfiles.go`
- **Do:** Add `MCPConfig` struct (Expose, EssentialTools, IncludeMeta, `*bool` IncludeOrchestration); add `MCP MCPConfig \`json:"mcp"\`` field to `Config`.
- **Verify:** `go build ./internal/core/...`; struct round-trips via `encoding/json`.
- **Satisfies:** R1 (zero value), schema for R2–R5.

## T2 — Merge `mcp` block in LoadConfig
- **Deps:** T1
- **Files:** `internal/core/specfiles.go` (`LoadConfig`)
- **Do:** Overlay `mcp` block; ensure absent block ⇒ zero value, `includeOrchestration` absent ⇒ nil pointer.
- **Verify:** Unit test: load config without `mcp` ⇒ `Expose==""`, `IncludeOrchestration==nil`. Load with explicit `includeOrchestration:false` ⇒ non-nil false.
- **Satisfies:** R1, R5a.

## T3 — `resolveMCPExposure` helper
- **Deps:** T1
- **Files:** `internal/mcp/tools.go` (new helper) + `tools_test.go`
- **Do:** Pure function `resolveMCPExposure(cfg *core.Config)` returning an allow-plan: effective expose mode, essential set (with default fallback per R3a), includeMeta, includeOrchestration (derived per R5a), and an unknown-expose flag for the stderr diagnostic.
- **Verify:** Table test over config matrix (all/essential/empty-essential/meta/orch combos).
- **Satisfies:** R3, R3a, R4, R5, R5a, R6.

## T4 — Filter in `buildTools(cfg)`
- **Deps:** T3
- **Files:** `internal/mcp/tools.go`
- **Do:** Change `buildTools()` → `buildTools(cfg *core.Config)`. Apply plan: meta-gate (`update`/`uninstall`/`schema`), orchestration-gate (`brain`/`pinky` + `brain_*` intents), essential allowlist. `nil` cfg ⇒ all. Preserve order (R7).
- **Verify:** Unit tests AC1–AC5; emit stderr diagnostic on unknown expose (AC6).
- **Satisfies:** R2–R7.

## T5 — Thread cfg through server
- **Deps:** T4
- **Files:** `internal/mcp/server.go`, `internal/cmd/mcp.go`
- **Do:** Carry `*core.Config` on the conn/Serve alongside `Dispatcher`; `tools/list` calls `buildTools(cfg)`. In `RunMCP`, `core.LoadConfig(root)` and pass down. Update `transport_http.go` if it builds tools independently.
- **Verify:** `go build ./...`; existing server tests pass with cfg plumbed.
- **Satisfies:** R1 (live path).

## T6 — Golden backward-compat test
- **Deps:** T5
- **Files:** `internal/mcp/tools_test.go`
- **Do:** Pin `expose:"all"` (and absent-config) `tools/list` to the pre-change golden tool set.
- **Verify:** `go test ./internal/mcp/...`.
- **Satisfies:** AC1.

## T7 — Integration tests per config variant
- **Deps:** T5
- **Files:** `internal/mcp/integration_test.go`
- **Do:** Boot server with each config; call `tools/list`; assert counts/names (AC2–AC6).
- **Verify:** `go test ./internal/mcp/...`.
- **Satisfies:** AC2–AC6.

## T8 — Docs
- **Deps:** T5
- **Files:** `docs/mcp-guide.md`
- **Do:** Document the `mcp` config block, exposure modes, default essential set, and meta/orchestration gating.
- **Verify:** Manual read; matches implemented field names.
- **Satisfies:** plan §8 migration.

**Wave gate:** T1–T8 green ⇒ Wave 2 may start (depends on `buildTools(cfg)` signature + `mcp` config block).
