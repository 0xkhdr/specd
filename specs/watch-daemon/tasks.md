# tasks.md — Watch Daemon + Event Stream execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Change-signal recon

- [ ] **T1 — Map frontier computation + revision signal**
  - role: investigator · depends: — · requirements: R1,R5
  - Report how `RunnableFrontier` + program builder compute runnable sets, and
    where the state revision/CAS counter lives. file:line only.
  - verify: N/A — complete with `--unverified --evidence "<signal map>"`

## Wave 2 — Core loop + events

- [ ] **T2 — `FrontierEvent` model + change detector**
  - role: builder · depends: T1 · requirements: R1,R5,R6
  - Revision-based change detection; emit only on real frontier change; read-only.
  - verify: `go test ./internal/core/ -run TestFrontierDetect -race -count=2`

- [ ] **T3 — JSON-lines emitter + `specd watch` command**
  - role: builder · depends: T2 · requirements: R1,R2
  - `internal/cmd/watch.go`, registered. NDJSON on stdout.
  - verify: `go test ./internal/cmd/ -run TestWatchNDJSON -race -count=1`

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
