# Tasks: MCP Dynamic Tool List

## T1 — Phase→tool mapping table
- **Deps:** Wave 2 (composites)
- **Files:** `internal/mcp/tools.go`
- **Do:** Define `phaseTools[phase] → []command` and a `phaseSubset(cfg, phase)` builder reusing `resolveMCPExposure`.
- **Verify:** Unit test per phase subset.
- **Satisfies:** R2.

## T2 — Thread-safe tool registry
- **Deps:** T1
- **Files:** `internal/mcp/server.go` (new `toolRegistry`)
- **Do:** `toolRegistry` with `sync.RWMutex` holding current `[]toolDef`; `tools/list` reads under RLock; expose `Swap()`.
- **Verify:** `-race` unit test concurrent read/swap (AC4).
- **Satisfies:** R4.

## T3 — Capability gating
- **Deps:** Wave 1
- **Files:** `internal/mcp/server.go` (`initializeResult`)
- **Do:** `listChanged = cfg.MCP.Expose=="phase"`.
- **Verify:** AC1.
- **Satisfies:** R1.

## T4 — Notification writer
- **Deps:** T2
- **Files:** `internal/mcp/server.go`, `internal/mcp/transport.go`, `transport_http.go`
- **Do:** Add per-conn writer mutex; `notifyToolsListChanged()` writes framed notification (no id). stdio: single writer; HTTP/SSE: via SSE channel.
- **Verify:** Unit test asserts framed notification bytes; no interleave with responses.
- **Satisfies:** R3.

## T5 — Phase watcher goroutine
- **Deps:** T1, T2, T4
- **Files:** `internal/mcp/watcher.go` (new)
- **Do:** Phase-mode-only goroutine polling spec `state.json` phases (reuse `watch` interval/diff). On change: recompute subset, `registry.Swap`, debounce (R7), notify (R3). `context.Context` cancel on stream close (R5).
- **Verify:** Fake transition → one notification + new subset (AC2, AC3); debounce test (R7).
- **Satisfies:** R3, R5, R7.

## T6 — Lifecycle wiring
- **Deps:** T5
- **Files:** `internal/mcp/server.go`, `internal/cmd/mcp.go`
- **Do:** Start watcher only when `expose:"phase"` (R6); cancel context on EOF/shutdown.
- **Verify:** AC5 (goroutine stop), AC6 (no watcher off-mode).
- **Satisfies:** R5, R6.

## T7 — Race + integration tests, docs
- **Deps:** T6
- **Files:** `internal/mcp/*_test.go`, `docs/mcp-guide.md`
- **Do:** `-race` suite (AC4); document `expose:"phase"` + host caveats.
- **Verify:** `go test -race ./internal/mcp/...`.
- **Satisfies:** AC1–AC6.

**Wave gate:** Wave 3 done ⇒ Wave 4.
