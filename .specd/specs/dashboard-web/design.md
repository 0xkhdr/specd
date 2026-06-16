# Design — Dashboard Decoupling from VS Code

## Overview
The dashboard is already a Go-served, self-contained HTML page (`cmd/serve.go` →
`core.RenderHTML`), and live frontier events already stream over SSE (`cmd/watch_sse.go`,
`/events`). The VS Code extension (`editors/vscode/extension.js`) is a thin wrapper that spawns
`specd serve` and iframes the result — it owns no dashboard logic. So "decoupling" is mostly
*deletion* plus three additive enhancements to `specd serve`: a multi-spec index at `/`, an
embedded SSE live-update client so the page refreshes on frontier change instead of meta-refresh
polling, and a responsive stylesheet. We reuse the existing `sseHandler` rather than inventing a
new stream, keeping the change small and the read-only invariant intact.

## Architecture
```
   Browser (any client)
        │  GET /                 → spec index (new)
        │  GET /s/<slug>         → live report HTML (RenderHTML)
        │  GET /api/report?spec= → JSON ReportData
        │  GET /events           → SSE frontier stream (reused sseHandler)
        ▼
  cmd.NewServeHandler (extended mux, read-only, loopback-default)
        │ per-request rebuild from disk
        ▼
  core.LoadState / loadReportData / RenderHTML   ◄── state.json + artifacts
```
The page subscribes to `/events`; on a frontier delta for the viewed slug it re-fetches
`/api/report` and re-renders — no full reload, no polling. `editors/vscode/` is removed.

## Components and interfaces
- **`editors/vscode/` (removed):** delete the extension package. Backs R1.1.
- **`cmd/serve.go` (extended):** `NewServeHandler` gains a project-scoped mux —
  `GET /` (spec index over `core.ListSpecs`), `GET /s/<slug>` (existing report HTML),
  `GET /api/report?spec=<slug>` (JSON), and mounts the reused `sseHandler` at `/events`. Non-GET
  → 405; unknown spec → 404. `RunServe` keeps the loopback default and `--addr` override. Backs
  R2, R3, R4, R5.
- **`core.RenderHTML` (extended):** inject a small inlined `<script>` that opens an
  `EventSource("/events")` and refreshes on a delta, plus a responsive `<meta viewport>` and
  media-query CSS. Self-contained, no external assets. Backs R4, R6.
- **`docs/dashboard.md` (new) + README/migration note:** browser-first usage and the VS Code
  removal note. Backs R1.2.
- **`cmd/serve_test.go` (extended):** httptest coverage for the index, 404, 405, and SSE mount.

## Data models
- **Spec index entry:** `{slug, title, status, phase}` derived from each spec's `state.json` via
  `core.LoadState` — no new persisted shape.
- **`ReportData`:** unchanged; already the JSON served at `/api/report`.
- **SSE frame:** unchanged `data: <FrontierEvent JSON>\n\n` from `sseHandler`.
- **Route table:** `/`, `/s/<slug>`, `/api/report`, `/events` — all GET, all read-only.

## Error handling
- Unknown spec slug → `404` via `RequireSpec` (existing `serveReportData` pattern).
- Non-GET method on any route → `405` with `Allow: GET` (existing pattern, extended to new routes).
- SSE unsupported (no `http.Flusher`) → `500` (existing `sseHandler` behaviour).
- Bind failure → gate error exit (existing `RunServe`).
- The dashboard exposes no mutating route; a malformed query (`?spec=` missing) renders the index
  rather than erroring, so the page is always reachable.

## Verification strategy
- **Unit (R2, R3, R5):** `cmd/serve_test.go` over `httptest` asserts the index lists known specs,
  `/s/<unknown>` → 404, POST → 405, and the loopback default address. No real port bound.
- **Integration (R4):** test that a frontier change observed by the reused `sseHandler` emits a
  `data:` frame; the embedded client script is asserted present in `RenderHTML` output.
- **System (R1, R6):** confirm `editors/vscode/` is gone (`test ! -d editors/vscode`); manual /
  scripted check that the served HTML carries a viewport meta and EventSource script and renders
  across Chromium/Firefox/WebKit.
- Requirement→test map: R1→dir-removal test; R2→index/404 tests; R3→405/read-only tests;
  R4→SSE + script-presence test; R5→loopback default test; R6→viewport/self-contained assertion.

## Risks and open questions
- **Decision gate:** delete `editors/vscode/` outright vs. leave a deprecation stub. Recommend
  outright removal with a migration note, recorded as an ADR before deletion.
- Reusing `sseHandler` ties the dashboard's liveness to the watch interval; very large projects
  may want a longer interval — exposed via existing `watchInterval()`, not a new knob.
- Browser matrix is asserted manually; we cannot run headless browsers in the deterministic
  harness, so R6.3 is verified by self-contained-HTML construction (no external deps) plus a
  documented manual check, not an automated cross-browser run.
- Mounting `/events` on the serve mux means the dashboard server now streams; ensure graceful
  shutdown matches `runWatchSSE`'s pattern to avoid leaked goroutines.
