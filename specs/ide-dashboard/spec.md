# spec.md — Editor/IDE Extension + Live HTML Dashboard

**Status:** proposed
**Source:** specd-report.html §8 idea **A3** (impact: med · effort: med · moat: med)
**Date:** 2026-06-16
**Scope:** `specd serve` (read-only HTTP) in the Go binary + a thin VS Code extension consuming it.

---

## 1. Objective

Give humans an ambient, auto-refreshing view of what the agent is doing: the
live wave DAG, the runnable frontier, blockers, and gate status — without
reading raw JSON. Adoption needs a human-facing surface; the deterministic HTML
report already exists, so make it *live*.

> **Hard invariant:** the Go binary stays stdlib-only and deterministic.
> `specd serve` is a read-only `net/http` server over existing report data — it
> mutates nothing and calls no LLM. The VS Code extension is a separate
> deliverable (TypeScript, its own package) that only consumes the server's
> HTTP/JSON; it never re-implements gate logic.

## 2. Context

- `specd report --format html` (`internal/cmd/report.go`, `internal/core/report.go`)
  already renders a self-contained HTML snapshot; `RenderHTML` takes an
  `autoRefreshSeconds` param, and `ReportCfg.AutoRefreshSeconds`
  (`specfiles.go`) already exists — the intent for live refresh is pre-wired.
- `specd status/waves/context` already emit structured JSON for the same data.

## 3. Requirements (EARS)

- **R1 (H)** WHEN `specd serve [--port N] [--root path]` runs, the system SHALL
  start a read-only `net/http` server exposing the HTML report at `/` and the
  underlying report data as JSON at `/api/report`.
- **R2 (H)** THE SYSTEM SHALL serve content derived from the **same**
  `core.ReportData` used by `specd report`, so the served view and the static
  report never diverge.
- **R3 (M)** WHERE `autoRefreshSeconds > 0` in config, the served HTML SHALL
  auto-refresh at that interval (existing `RenderHTML` param), and `/api/report`
  SHALL reflect current on-disk `state.json` on each request (no stale cache).
- **R4 (M)** THE SYSTEM SHALL bind to loopback by default and SHALL expose no
  mutating endpoint (GET-only; any other method ⇒ 405).
- **R5 (M)** IF the requested spec/root does not exist, the system SHALL return
  HTTP 404 with a structured JSON error, never a panic.
- **R6 (L)** A VS Code extension SHALL render the served dashboard in a webview
  panel and SHALL surface frontier/blocker/gate status, consuming only
  `/api/report` (no gate logic re-implemented in TypeScript).

## 4. Design / approach

1. **`internal/cmd/serve.go`** — `net/http` server; handlers call the existing
   `core` report builder per request. No goroutine state beyond the listener.
2. **Reuse render** — `/` returns `core.RenderHTML(data, cfg.AutoRefreshSeconds)`;
   `/api/report` returns the marshaled `ReportData`.
3. **Read-only contract** — register only GET routes; method guard returns 405.
4. **Extension** (`editors/vscode/`, separate package) — webview that polls
   `/api/report` and renders; ships independently of the Go release.

## 5. Non-goals

- No write endpoints; the server never advances phases or flips tasks.
- No WebSocket/SSE push in this spec (poll-refresh only; see `specd watch`, C1).
- No bundled JS framework in the Go binary — HTML stays self-contained.

## 6. Acceptance criteria

- `specd serve` serves the live HTML at `/` and JSON at `/api/report`, both
  derived from the same `ReportData` (asserted equal to `specd report` output).
- Non-GET ⇒ 405; missing spec ⇒ 404 JSON, no panic; binds loopback by default.
- The VS Code extension renders the frontier/blockers/gate status from
  `/api/report` only.
- `make ci` green; Go binary stays stdlib-only.
