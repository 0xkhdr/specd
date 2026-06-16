# tasks.md — Watch Daemon + Event Stream execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Change-signal recon

- [x] **T1 — Map frontier computation + revision signal** ✓ complete · 2026-06-16
  - role: investigator · depends: — · requirements: R1,R5
  - Report how `RunnableFrontier` + program builder compute runnable sets, and
    where the state revision/CAS counter lives. file:line only.
  - verify: N/A — complete with `--unverified --evidence "<signal map>"`
  - **Evidence:** frontier — `RunnableFrontier` `internal/core/dag.go:226-241`
    (filters `isRunnable` `dag.go:149-160`, sorts wave then `ordinal`);
    `NextRunnable` `dag.go:162-224`; per-spec input `DagTasksFromState`
    `internal/core/render.go:43`. Program-level frontier — `BuildProgram` +
    `RunnableFrontier(g.Dag)` in `programRender` `internal/cmd/program.go:89-101`
    (also emits `frontier` JSON `program.go:138-143`). Revision/CAS counter —
    `State.Revision` `internal/core/state.go:96`; bumped + compare-and-swapped in
    `SaveState` `state.go:214-242` (on-disk revision compare `state.go:220-228`,
    `state.Revision++` `state.go:235`). Change signal = monotonically increasing
    `Revision`; emit a `FrontierEvent` only when `RunnableFrontier` output changes
    across revisions. Read-only: daemon reads `LoadState`/`LoadSpec`, never
    `SaveState`.

## Wave 2 — Core loop + events

- [x] **T2 — `FrontierEvent` model + change detector** ✓ complete · 2026-06-16
  - role: builder · depends: T1 · requirements: R1,R5,R6
  - Revision-based change detection; emit only on real frontier change; read-only.
  - verify: `go test ./internal/core/ -run TestFrontierDetect -race -count=2`
  - **Evidence:** `internal/core/frontier.go` — `FrontierEvent`, `FrontierOf`,
    and `FrontierDetector.Observe` (per-spec last-frontier memory; emits only when
    the ordered runnable set changes — unchanged frontier at a higher revision is
    suppressed; computes Added/Removed). Read-only. `TestFrontierDetect` passes.

- [x] **T3 — JSON-lines emitter + `specd watch` command** ✓ complete · 2026-06-16
  - role: builder · depends: T2 · requirements: R1,R2
  - `internal/cmd/watch.go`, registered. NDJSON on stdout.
  - verify: `go test ./internal/cmd/ -run TestWatchNDJSON -race -count=1`
  - **Evidence:** `internal/cmd/watch.go` (`RunWatch`, `watchPass`) emits compact
    NDJSON per changed frontier; `--once` single pass, `--spec` filter, polls at
    `SPECD_WATCH_INTERVAL_MS` otherwise. Registered in both `Registry` and
    `core.Commands` (parity test green). `TestWatchNDJSON` passes.

## Wave 3 — Transports + lifecycle

- [ ] **T4 — SSE transport over net/http**
  - role: builder · depends: T3 · requirements: R3
  - verify: `go test ./internal/cmd/ -run TestWatchSSE -race -count=1`

- [ ] **T5 — Webhook POST with bounded backoff (non-blocking)**
  - role: builder · depends: T3 · requirements: R4
  - Separate goroutine + bounded retry queue; slow endpoint never blocks loop.
  - verify: `go test ./internal/cmd/ -run TestWatchWebhook -race -count=1`

- [ ] **T6 — Signal handling + clean shutdown**
  - role: builder · depends: T3 · requirements: R7
  - verify: `go test ./internal/cmd/ -run TestWatchShutdown -race -count=1`

- [ ] **T7 — Review: read-only, no duplicate events, stdlib-only**
  - role: reviewer · depends: T4,T5,T6 · requirements: R5,R6
  - verify: N/A — complete with `--unverified --evidence "<observer audit>"`

---

## Traceability

| Wave | Tasks | Requirements |
|------|-------|--------------|
| 1 | T1 | R1, R5 |
| 2 | T2–T3 | R1, R2, R5, R6 |
| 3 | T4–T7 | R3, R4, R5, R6, R7 |
