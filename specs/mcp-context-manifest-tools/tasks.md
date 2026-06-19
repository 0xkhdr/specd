# Tasks: MCP Context-Manifest-Driven Tool Loading

## T1 — Manifest schema + loader
- **Deps:** Wave 1, Wave 3
- **Files:** `internal/core/` (manifest type + `LoadContextManifest`)
- **Do:** Add `contextManifest.{requiredTools,optionalTools,forbiddenTools}` type; loader returns empty when absent; read-only.
- **Verify:** Unit: present/absent/partial manifests parse correctly.
- **Satisfies:** R1, R5, R6.

## T2 — Filter composition pass
- **Deps:** T1
- **Files:** `internal/mcp/tools.go` (extend exposure plan)
- **Do:** Final pass: restrict to required∪optional, subtract forbidden, re-apply config gates; precedence per §5.2.
- **Verify:** Precedence matrix unit test (AC1–AC3).
- **Satisfies:** R2, R3, R4.

## T3 — Name validation
- **Deps:** T2
- **Files:** `internal/mcp/tools.go`
- **Do:** Validate manifest tool names against live tool set; unknown ⇒ stderr warning, ignore.
- **Verify:** Conformance test.
- **Satisfies:** R4 (diagnostics), robustness.

## T4 — Wire active-spec manifest into build
- **Deps:** T2
- **Files:** `internal/mcp/server.go` (+ watcher from Wave 3)
- **Do:** Resolve active spec → load manifest → apply pass. Optionally trigger `list_changed` on manifest change.
- **Verify:** Integration: manifest spec ⇒ subset (AC1, AC4).
- **Satisfies:** R1, R5.

## T5 — Tests + docs
- **Deps:** T4
- **Files:** `internal/mcp/*_test.go`, `docs/mcp-guide.md`
- **Do:** Integration suite; document manifest tool fields + precedence.
- **Verify:** `go test ./internal/mcp/...`.
- **Satisfies:** AC1–AC4.
