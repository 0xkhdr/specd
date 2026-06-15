# tasks.md — IDE Extension + Live Dashboard execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Report data reuse

- [ ] **T1 — Map the report data path**
  - role: investigator · depends: — · requirements: R2
  - Report how `internal/cmd/report.go` builds `core.ReportData` and calls
    `RenderHTML`, and which fields cover frontier/blockers/gates. file:line only.
  - verify: N/A — complete with `--unverified --evidence "<ReportData map>"`

## Wave 2 — Read-only server

- [ ] **T2 — `specd serve` read-only HTTP server**
  - role: builder · depends: T1 · requirements: R1,R2,R3,R4
  - `internal/cmd/serve.go`: GET `/` → `RenderHTML(data, autoRefresh)`, GET
    `/api/report` → marshaled `ReportData` rebuilt per request. Loopback bind,
    405 on non-GET. Register in `registry.go`.
  - verify: `go test ./internal/cmd/ -run TestServe -race -count=1`

- [ ] **T3 — 404 + no-panic on missing spec/root**
  - role: builder · depends: T2 · requirements: R5
  - verify: `go test ./internal/cmd/ -run TestServeNotFound -race -count=1`

## Wave 3 — Parity + extension

- [ ] **T4 — Test: served view == static report**
  - role: verifier · depends: T2 · requirements: R2,R3
  - Assert `/` HTML and `/api/report` JSON derive from the same `ReportData` as
    `specd report`; assert fresh read of `state.json` per request.
  - verify: `go test ./internal/cmd/ -run TestServeParity -race -count=2`

- [ ] **T5 — VS Code extension webview (separate package)**
  - role: builder · depends: T2 · requirements: R6
  - `editors/vscode/`: webview polls `/api/report`, renders frontier/blockers/
    gates. No gate logic in TS. Build with the extension's own toolchain.
  - verify: `cd editors/vscode && npm ci && npm run build`

- [ ] **T6 — Review: read-only, no mutating routes**
  - role: reviewer · depends: T3,T4 · requirements: R4
  - verify: N/A — complete with `--unverified --evidence "<route audit: GET-only>"`

---

## Traceability

| Wave | Tasks | Requirements |
|------|-------|--------------|
| 1 | T1 | R2 |
| 2 | T2–T3 | R1–R5 |
| 3 | T4–T6 | R2, R3, R4, R6 |
