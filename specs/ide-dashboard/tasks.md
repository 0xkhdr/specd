# tasks.md — IDE Extension + Live Dashboard execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Report data reuse

- [x] **T1 — Map the report data path** ✓ complete · 2026-06-16
  - role: investigator · depends: — · requirements: R2
  - Report how `internal/cmd/report.go` builds `core.ReportData` and calls
    `RenderHTML`, and which fields cover frontier/blockers/gates. file:line only.
  - verify: N/A — complete with `--unverified --evidence "<ReportData map>"`
  - **Evidence:** `RunReport` `internal/cmd/report.go:12` loads state +
    artifacts and builds `core.ReportData` `report.go:32-40` (State + 6
    `ReadArtifact` reads), then calls `core.RenderHTML(data, autoRefresh)`
    `report.go:44` or `RenderMarkdown` `report.go:46`. Type `ReportData`
    `internal/core/report.go:32-40`; `buildSections` `report.go:121-157`.
    Field coverage — **frontier: none stored**; derived live via
    `WaveGraph(state)` in `progressOverview` `report.go:108-113` (uses
    `CountTasks` `render.go:25`). blockers — `State.Blockers` rendered
    `report.go:149-155`. gates — **not in ReportData** (`check` is a separate
    command); acceptance criteria section `report.go:132-148`. A read-only
    `specd serve` should call the identical `RenderHTML(data, …)` so the served
    view is byte-identical to the static report; no mutating routes.

## Wave 2 — Read-only server

- [x] **T2 — `specd serve` read-only HTTP server** ✓ complete · 2026-06-16
  - role: builder · depends: T1 · requirements: R1,R2,R3,R4
  - `internal/cmd/serve.go`: GET `/` → `RenderHTML(data, autoRefresh)`, GET
    `/api/report` → marshaled `ReportData` rebuilt per request. Loopback bind,
    405 on non-GET. Register in `registry.go`.
  - verify: `go test ./internal/cmd/ -run TestServe -race -count=1`
  - **Evidence:** `internal/cmd/serve.go` — `NewServeHandler` (GET `/` → identical
    HTML to `report --format html`; GET `/api/report` → JSON `ReportData` rebuilt
    per request; 405 on non-GET; no mutating routes), `RunServe` binds
    `127.0.0.1:8765` (`--addr`). `loadReportData` shared with `report`. Registered
    in both registries (parity green). `TestServe` passes.

- [x] **T3 — 404 + no-panic on missing spec/root** ✓ complete · 2026-06-16
  - role: builder · depends: T2 · requirements: R5
  - verify: `go test ./internal/cmd/ -run TestServeNotFound -race -count=1`
  - **Evidence:** `serveReportData` returns a clean 404 (never panics) when the
    spec or its state is absent; unknown sub-paths 404 via `http.NotFound`.
    `TestServeNotFound` passes.

## Wave 3 — Parity + extension

- [x] **T4 — Test: served view == static report** ✓ complete · 2026-06-16
  - role: verifier · depends: T2 · requirements: R2,R3
  - Assert `/` HTML and `/api/report` JSON derive from the same `ReportData` as
    `specd report`; assert fresh read of `state.json` per request.
  - verify: `go test ./internal/cmd/ -run TestServeParity -race -count=2`
  - **Evidence:** `cmd/serve_test.go` — `TestServeParity` asserts GET `/` is
    byte-identical to `report --format html`; `TestServeReadOnly` asserts 405 on
    non-GET and 404 (no panic) on a missing spec. Pass `-race`.

- [x] **T5 — VS Code extension webview (separate package)** ✓ complete · 2026-06-16
  - role: builder · depends: T2 · requirements: R6
  - `editors/vscode/`: webview embeds the read-only dashboard. No gate logic in TS.
  - verify: N/A — dependency-free JS extension (no build step); load via VS Code.
  - **Evidence:** `editors/vscode/` (separate package: `package.json`,
    `extension.js`, `README.md`) — `specd.openDashboard` launches `specd serve`
    on loopback and embeds the dashboard in a webview iframe. Zero gate logic in
    JS; strictly read-only (only ever runs `specd serve`). Plain JS, no build
    toolchain or dependencies required.

- [x] **T6 — Review: read-only, no mutating routes** ✓ complete · 2026-06-16
  - role: reviewer · depends: T3,T4 · requirements: R4
  - verify: N/A — complete with `--unverified --evidence "<route audit: GET-only>"`
  - **Evidence:** Reviewed: `NewServeHandler` exposes only GET `/` and
    GET `/api/report`; all other methods → 405, unknown paths → 404. No handler
    writes state. The VS Code extension only consumes the read-only server.

---

## Traceability

| Wave | Tasks | Requirements |
|------|-------|--------------|
| 1 | T1 | R2 |
| 2 | T2–T3 | R1–R5 |
| 3 | T4–T6 | R2, R3, R4, R6 |
