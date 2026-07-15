# Dashboard

specd ships a **browser-native, read-only dashboard** served directly by the Go
binary — no editor extension, no Node build step, no external assets.

```bash
specd report --serve                 # serve the project on http://127.0.0.1:8765/
specd report <slug> --serve          # same, with <slug> as the default spec for /api/report
```

Open `http://127.0.0.1:8765/` in any browser (desktop, tablet, or phone). The
layout is responsive and the page is fully self-contained.

## Routes

| Route | Method | Returns |
|-------|--------|---------|
| `/` | GET | Spec index — every spec under the project with status/phase |
| `/s/<slug>` | GET | That spec's live report HTML (same markup as `report --format html`) |
| `/api/report?spec=<slug>` | GET | `ReportData` JSON (defaults to the served slug) |
| `/events` | GET | Frontier change stream (Server-Sent Events) |

Every route is **GET-only and read-only**: non-GET requests get `405`, an unknown
spec gets `404`, and every response is rebuilt from `state.json` and artifacts per
request. The dashboard never writes spec state.

## Live updates

A spec report subscribes to the reused `/events` SSE stream. When the runnable
frontier of the viewed spec changes, the page re-fetches and re-renders in place —
no polling, no full reload, no LLM call. It is the same stream `specd report --watch`
exposes.

## Network exposure

The server binds loopback (`127.0.0.1:8765`) by default. To expose it elsewhere,
set the bind address explicitly:

```bash
specd report --serve --addr 0.0.0.0:8765   # off-host: read-only, but exposes spec contents
```

A failed bind exits with a gate error.

## The unified `specd dashboard` server

`specd report --serve` (above) serves one project's spec reports. `specd
dashboard` is the **project-wide** view: a single loopback server that aggregates
every spec's status alongside orchestrator waves, conductor sessions, eval
trends, cost, escalations, and the shared harness bundle. It is read-only and
makes zero outbound network calls — every panel renders from local `state.json`
and ledgers.

```bash
specd dashboard                                 # http://127.0.0.1:8765/
specd dashboard --mode cost                      # focus one panel
specd dashboard <slug> --addr 127.0.0.1:9000     # per-spec report as the default target
```

### Endpoints

| Route | Method | Returns |
|-------|--------|---------|
| `/` | GET | Aggregate dashboard HTML across all specs (append `?mode=` to switch panels) |
| `/s/<slug>` | GET | That spec's live report HTML |
| `/api/dashboard` | GET | Deterministic JSON projection of the aggregate view (honors `?mode=`) |
| `/events` | GET | Server-Sent Events stream of frontier changes |

`--mode` (or `?mode=`) selects one of `all` (default), `conductor`,
`orchestrator`, `cost`, or `eval` — switching panels needs no restart. Like
`report --serve`, every route is **GET-only and read-only**: a non-GET request
gets `405`, an unknown spec `404`, and each response is rebuilt from state per
request. The server never mutates spec state.

### The `/events` streaming model

`/events` is a Server-Sent Events (`text/event-stream`) endpoint — the same live
stream `specd report --watch`/`--serve` exposes. The page opens one `EventSource`
connection; when a viewed spec's runnable frontier changes, the server pushes an
event and the page re-fetches and re-renders in place. There is no polling, no
full reload, and no LLM call. The connection is one-way (server → browser) and
carries change signals only, never mutations.

Bind loopback-only by default (`127.0.0.1:8765`); to expose it, put it behind a
TLS-terminating reverse proxy rather than binding a public address.

## Migration from the VS Code extension

Earlier specd shipped a VS Code extension (`editors/vscode/`) whose only job was to
spawn `specd report --serve` and embed the result in a webview iframe — it owned no dashboard
logic. It has been **removed** so no client is privileged. To get the same view:

1. Run `specd report <slug> --serve` in your project.
2. Open `http://127.0.0.1:8765/` in your browser.

This works from any editor or device, not just VS Code.
