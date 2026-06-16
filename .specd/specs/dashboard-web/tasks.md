# Tasks — Dashboard Decoupling from VS Code

## Wave 1
- [x] T1 — Audit the VS Code extension and existing serve/watch surface ✓ complete · evidence: Audit: editors/vscode/extension.js only cp.spawn('specd serve') + iframes http://127.0.0.1:port/ (L43,58-60); owns no dashboard logic. Routes to preserve in serve.go: GET / (RenderHTML), GET /api/report (JSON), 405 non-GET (Allow:GET), 404 via RequireSpec/LoadState. Reusable sseHandler(root,filter) in watch_sse.go mountable at /events; helpers ListSpecs/loadReportData/watchInterval/collectChanges exist. RenderHTML in core/report.go:267. · 2026-06-16T23:01:57.128687844Z
  - why: removal must preserve the read-only HTTP behaviour the extension relied on (R1, R3)
  - role: investigator
  - files: editors/vscode/extension.js, editors/vscode/package.json, internal/cmd/serve.go, internal/cmd/watch_sse.go
  - contract: document what the extension actually does (spawn serve + iframe), the existing serve routes, and the reusable sseHandler; do NOT modify code
  - acceptance: a findings note confirming the extension owns no dashboard logic and listing the routes to preserve
  - verify: N/A
  - depends: —
  - requirements: 1, 3

- [x] T2 — Record the extension-removal decision ✓ complete · evidence: ADR-001 appended to decisions.md: outright removal of editors/vscode/ chosen over deprecation stub; migration path = docs/dashboard.md browser-first specd serve. Consequences recorded. · 2026-06-16T23:02:07.677706411Z
  - why: deleting editors/vscode is a one-way change needing an ADR (R1)
  - role: investigator
  - files: .specd/specs/dashboard-web/decisions.md
  - contract: append an ADR choosing outright removal vs deprecation stub and the migration path; do NOT delete code yet
  - acceptance: decisions.md carries an ADR selecting the removal approach with consequences
  - verify: N/A
  - depends: T1
  - requirements: 1

## Wave 2
- [x] T3 — Extend serve with a multi-spec index and SSE mount ✓ complete · evidence: Extended NewServeHandler: GET / spec index via ListSpecs, GET /s/<slug> report HTML, GET /api/report?spec= JSON (defaults to slug), /events mounts reused sseHandler. 405 non-GET, 404 missing spec, loopback default unchanged. go build + go vet pass (verify exit 0). · 2026-06-16T23:03:36.253330305Z
  - why: the dashboard must list all specs and stream live updates from the Go binary (R2, R4, R5)
  - role: builder
  - files: internal/cmd/serve.go
  - contract: extend NewServeHandler with GET / (spec index via core.ListSpecs), GET /s/<slug> (report HTML), GET /api/report?spec=, and mount the reused sseHandler at /events; keep non-GET 405, unknown spec 404, loopback default; do NOT add any mutating route
  - acceptance: index lists known specs, unknown slug 404s, POST 405s, default bind is 127.0.0.1
  - verify: go build ./... && go vet ./internal/cmd/...
  - depends: T2
  - requirements: 2, 3, 4, 5

- [x] T4 — Make the rendered dashboard live and responsive ✓ complete · evidence: RenderHTML now emits viewport meta, @media responsive CSS, and inlined EventSource('/events') client that re-fetches /api/report + /s/<slug> and swaps body on a frontier delta — self-contained, no external assets, no LLM. New TestRenderHTMLLiveResponsive green. · 2026-06-16T23:04:51.956872148Z
  - why: the page must refresh on frontier change and adapt to any viewport without external assets (R4, R6)
  - role: builder
  - files: internal/core/render.go
  - contract: inject an inlined EventSource(/events) client that re-fetches /api/report on a delta, a viewport meta tag, and media-query CSS; keep the HTML self-contained with no external fetches and no LLM call
  - acceptance: RenderHTML output contains a viewport meta, an EventSource script, and no external asset URLs
  - verify: go test ./internal/core/... -race -count=1 -run RenderHTML
  - depends: T3
  - requirements: 4, 6

- [x] T5 — Remove the VS Code extension and document migration ✓ complete · evidence: Deleted editors/vscode/ (git rm; editors/ dir gone per ADR-001). Added docs/dashboard.md (browser-first usage, routes, live SSE, network safety, VS Code migration steps); linked from docs/README.md; removed editors/ line from concepts.md tree. verify: test ! -d editors/vscode && test -f docs/dashboard.md → exit 0. · 2026-06-16T23:05:54.714865673Z
  - why: complete the decoupling and tell existing users where the dashboard moved (R1)
  - role: builder
  - files: editors/vscode/extension.js, editors/vscode/package.json, docs/dashboard.md
  - contract: delete the editors/vscode package per the T2 ADR and add a browser-first dashboard doc with a migration note; update README references
  - acceptance: editors/vscode no longer exists and docs/dashboard.md documents browser usage
  - verify: test ! -d editors/vscode && test -f docs/dashboard.md
  - depends: T4
  - requirements: 1

## Wave 3
- [x] T6 — Extend serve tests for index, 404, 405, and SSE ✓ complete · evidence: serve_test.go rewritten + commands_test.go serve tests updated for multi-spec routes: TestServeParity(/s/auth byte-identical), TestServeIndex(/ lists auth+billing links), TestServeReadOnly(405 on /,/s/auth,/api/report + 404 /s/nope), TestServeEventsMount(/events SSE text/event-stream + valid FrontierEvent frame). go test -run Serve -race → ok. · 2026-06-16T23:07:50.103463115Z
  - why: the read-only multi-spec contract must be pinned by tests (R2, R3, R4)
  - role: builder
  - files: internal/cmd/serve_test.go
  - contract: add httptest cases for the spec index, unknown-spec 404, non-GET 405, and the /events SSE mount; assert no port is bound and no route mutates state
  - acceptance: go test ./internal/cmd/... -race passes including the new serve cases
  - verify: go test ./internal/cmd/... -race -count=1 -run Serve
  - depends: T5
  - requirements: 2, 3, 4

- [x] T7 — Final gate review of the dashboard spec ✓ complete · evidence: Final review: ./specd check dashboard-web all gates green; go test ./internal/cmd/... ./internal/core/... -race -count=1 → ok ok. Read-only + loopback invariants intact (405/404 tests), editors/vscode/ removed, docs/dashboard.md present. No behaviour added. · 2026-06-16T23:08:36.422390492Z
  - why: confirm removal, index, live updates, safety, and responsiveness cohere before approval (R1-R6)
  - role: verifier
  - files: internal/cmd/serve.go, internal/core/render.go, docs/dashboard.md
  - contract: run the full check and the serve/core test suites; confirm the read-only and loopback invariants hold and the VS Code extension is gone; do NOT add behaviour
  - acceptance: specd check dashboard-web reports no violations and the serve and core suites are green
  - verify: ./specd check dashboard-web && go test ./internal/cmd/... ./internal/core/... -race -count=1
  - depends: T6
  - requirements: 1, 2, 3, 4, 5, 6
