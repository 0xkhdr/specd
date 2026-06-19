# Tasks: MCP Resources

## T1 — Advertise resources capability
- **Deps:** Wave 1 (conn carries root)
- **Files:** `internal/mcp/server.go` (`initializeResult`)
- **Do:** Add `"resources": map[string]any{"listChanged": false}` to capabilities.
- **Verify:** `initialize` test asserts key present (AC1).
- **Satisfies:** R1.

## T2 — URI scheme parser + containment
- **Deps:** none
- **Files:** `internal/mcp/resources.go` (new)
- **Do:** Parse `specd://specs/<slug>/<artifact>` and `specd://steering/<file>`; map to path under `<root>/.specd/`; `filepath.Clean` + prefix containment; reject traversal.
- **Verify:** Table test with traversal vectors (AC5).
- **Satisfies:** R3, R6.

## T3 — Artifact filename source-of-truth
- **Deps:** none
- **Files:** `internal/mcp/resources.go` (or import from `internal/core`)
- **Do:** Define canonical artifact list (six `specd new` artifacts + `state.json`); add conformance test vs `specd new` output.
- **Verify:** Conformance test green.
- **Satisfies:** R2 (correctness), R7.

## T4 — `resources/list`
- **Deps:** T2, T3
- **Files:** `internal/mcp/resources.go`
- **Do:** Walk specs + steering, emit existing files only, deterministic order, with `uri/name/mimeType`.
- **Verify:** Integration test on 2-spec project (AC2).
- **Satisfies:** R2, R7.

## T5 — `resources/read`
- **Deps:** T2, T3
- **Files:** `internal/mcp/resources.go`
- **Do:** Resolve URI→path (containment), read, infer mime (`.md`→markdown, `.json`→json), return `{contents:[{uri,mimeType,text}]}`. Unknown/missing ⇒ rpcError.
- **Verify:** AC3, AC4, AC6.
- **Satisfies:** R4, R5, R8.

## T6 — Wire into route()
- **Deps:** T4, T5
- **Files:** `internal/mcp/server.go`
- **Do:** Add `resources/list` + `resources/read` cases passing root.
- **Verify:** End-to-end integration test.
- **Satisfies:** R1–R8 live path.

## T7 — Docs
- **Deps:** T6
- **Files:** `docs/mcp-guide.md`
- **Do:** Document resource URIs + that `specd_context` is now optional.
- **Verify:** Manual read.
- **Satisfies:** plan §B1.

**Wave gate:** parallel with mcp-composite-tools/mcp-prompts; the `resources` capability feeds Wave 3 dynamic advertisement.
