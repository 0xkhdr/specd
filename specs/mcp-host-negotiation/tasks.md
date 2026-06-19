# Tasks: MCP Host Capability Negotiation

## T1 — Extend initializeParams parsing
- **Deps:** Wave 1
- **Files:** `internal/mcp/server.go`
- **Do:** Add `capabilities.specd.{maxTools,preferredNamespaces}` to `initializeParams`; lenient parse (ignore garbage). Store on conn.
- **Verify:** Unit: present/absent/garbage parse without error (AC5, R6).
- **Satisfies:** R1, R2, R5, R6.

## T2 — Namespace map
- **Deps:** Wave 2 (composite names)
- **Files:** `internal/mcp/tools.go`
- **Do:** Define prefix→namespace mapping (read/orchestration/meta/lifecycle).
- **Verify:** Unit covers every emitted tool maps to a namespace.
- **Satisfies:** R2.

## T3 — Apply ordering + truncation
- **Deps:** T1, T2
- **Files:** `internal/mcp/tools.go`
- **Do:** Post-filter pass: partition by namespace, preferred-first stable sort, truncate to maxTools, force-keep required/config tools (R4).
- **Verify:** AC1, AC2, AC3.
- **Satisfies:** R1, R2, R4.

## T4 — Session persistence + wiring
- **Deps:** T3
- **Files:** `internal/mcp/server.go`
- **Do:** Apply stored prefs to every `tools/list` incl. dynamic re-fetch (Wave 3).
- **Verify:** Re-fetch after phase change still honours prefs (R5).
- **Satisfies:** R5.

## T5 — Backward-compat + robustness tests, docs
- **Deps:** T4
- **Files:** `internal/mcp/*_test.go`, `docs/mcp-guide.md`
- **Do:** Golden no-field == feature-off (AC4); garbage fuzz (AC5); document the `capabilities.specd` extension as non-standard/optional.
- **Verify:** `go test ./internal/mcp/...`.
- **Satisfies:** AC1–AC5.
