# Spec: MCP Dynamic Tool List

> Plan item: **B3**. Wave 3 — depends on
> [mcp-config-tool-filtering](../mcp-config-tool-filtering/spec.md) (filter) and
> Wave 2 (capabilities to re-advertise).

## 1. Overview

Today `capabilities.tools.listChanged:false` and the tool list is static for the
process lifetime. This spec enables `expose:"phase"`: the server watches
`.specd/` for phase transitions (requirements → design → tasks → executing) and
emits `notifications/tools/list_changed` so the host re-fetches a
phase-appropriate subset. Requires thread-safe tool-list state because the HTTP
transport dispatches concurrently.

## 2. Goals / Non-goals

**Goals**
- `capabilities.tools.listChanged:true` when phase mode is active.
- Background watcher detects spec phase changes.
- Server emits `notifications/tools/list_changed`; `tools/list` returns the phase subset.
- Thread-safe tool-list reads/writes across stdio and HTTP transports.

**Non-goals**
- Resource/prompt subscriptions (separate, future).
- Per-tool streaming.

## 3. Foundational facts (verified)
- `initializeResult` hardcodes `listChanged:false` (server.go:108) — must become config-driven.
- stdio `Serve` (server.go:39) processes requests one at a time (serial); HTTP transport (`transport_http.go`) uses `dispatchLocked` mutex — concurrent. Tool-list state must be guarded for the HTTP path.
- No outbound notification path exists today — `Serve` only responds to requests. A server→client notification writer must be added (write a framed JSON-RPC notification with no `id`).
- specd already has `watch` (`internal/cmd/watch.go`) emitting `FrontierEvent` on frontier change, polling `SPECD_WATCH_INTERVAL_MS` (default 1000ms) — reuse its polling/state-diff approach for phase detection rather than reinventing.
- Phase is derivable from `state.json` per spec (`specd context`/`status` read it).

## 4. Requirements (EARS)

- **R1** WHEN `mcp.expose` is `"phase"`, THE SYSTEM SHALL advertise `capabilities.tools.listChanged:true`; otherwise it SHALL remain `false`.
- **R2** WHEN `expose:"phase"`, `tools/list` SHALL return the subset appropriate to the active spec's current phase.
- **R3** WHEN a watched spec's phase changes, THE SYSTEM SHALL send a `notifications/tools/list_changed` notification to the client.
- **R4** THE SYSTEM SHALL guard all tool-list reads/writes with a mutex so concurrent HTTP dispatch never races (no data race under `-race`).
- **R5** WHEN the input stream closes or the server shuts down, THE SYSTEM SHALL stop the watcher goroutine cleanly (no leak, no panic on closed writer).
- **R6** WHEN `expose` is not `"phase"`, THE SYSTEM SHALL NOT start the watcher and SHALL behave exactly as the static list (no notifications).
- **R7** THE SYSTEM SHALL debounce rapid successive phase changes so it does not flood the host with notifications.

## 5. Design

### 5.1 Phase→tool mapping
Define a table `phaseTools[phase] → []command`:
- requirements/design/tasks (planning): inspect/read/check/approve/context-class tools.
- executing: next/dispatch/verify/task/inspect.
Composite tools (Wave 2) are the natural unit here — phase mode prefers them.

### 5.2 Thread-safe tool state
Introduce a `toolRegistry` holding the current `[]toolDef` behind a `sync.RWMutex`.
`tools/list` reads under RLock; the watcher swaps under Lock. Both transports use
the same registry instance.

### 5.3 Notification writer
Add a `notifier` that writes a framed JSON-RPC notification
`{"jsonrpc":"2.0","method":"notifications/tools/list_changed"}` (no id) to the
same writer the conn uses. For stdio this is the single `w`; serialize writes via
the conn's writer mutex (add one). For HTTP/SSE, route through the SSE channel.

### 5.4 Watcher goroutine
On startup (phase mode only): launch a goroutine polling spec `state.json` phases
(reuse `watch`'s interval + diff pattern). On change: recompute subset, swap
registry, debounce (R7), emit notification (R3). Stop via a `context.Context`
cancelled on stream close (R5).

### 5.5 Capability gating
`initializeResult` reads `expose` from cfg: `listChanged = (expose=="phase")` (R1).

## 6. Acceptance criteria
- **AC1** `expose:"phase"` ⇒ `initialize` shows `tools.listChanged:true`; other modes `false`.
- **AC2** Advancing a spec from design→tasks triggers exactly one `notifications/tools/list_changed` (after debounce).
- **AC3** Post-transition `tools/list` returns the tasks-phase subset, not the design subset.
- **AC4** `go test -race ./internal/mcp/...` clean under concurrent HTTP dispatch + watcher.
- **AC5** Stream close stops the watcher (goroutine count returns to baseline; no panic).
- **AC6** Non-phase modes emit zero notifications and never start the watcher.

## 7. Testing
- Concurrency: `-race` test driving HTTP dispatch while watcher swaps registry (AC4).
- Notification: fake phase transition, assert one notification + new subset (AC2, AC3).
- Lifecycle: close stream, assert goroutine stop (AC5).

## 8. Risks
- **Notification write races** with response writes → single writer mutex per conn.
- **Goroutine leak** on shutdown → context cancellation + test.
- **Notification flooding** → debounce window.
- **Host compatibility:** some hosts ignore `list_changed`; behaviour must degrade gracefully (they keep the initial subset).
