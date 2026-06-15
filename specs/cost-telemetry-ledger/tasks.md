# tasks.md — Cost & Telemetry Ledger execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Clock + record recon

- [ ] **T1 — Map status transitions + clock injection points**
  - role: investigator · depends: — · requirements: R1,R4
  - Report where running→complete/blocked transitions happen and how the test
    clock (`internal/testharness/clock.go`) is injected. file:line only.
  - verify: N/A — complete with `--unverified --evidence "<transition+clock map>"`

## Wave 2 — Capture

- [ ] **T2 — Add `Telemetry` to TaskState (omitempty, back-compat)**
  - role: builder · depends: T1 · requirements: R1,R5
  - verify: `go test ./internal/core/ -run TestTelemetryCompat -race -count=2`

- [ ] **T3 — Capture duration/retries/verify-duration via injectable clock**
  - role: builder · depends: T2 · requirements: R1,R4
  - Deterministic timing; retries increment on verify re-run.
  - verify: `go test ./internal/cmd/ -run TestTelemetryCapture -race -count=2`

- [ ] **T4 — `--tokens`/`--cost` annotation flags (stored, not computed)**
  - role: builder · depends: T2 · requirements: R2
  - verify: `go test ./internal/cmd/ -run TestTelemetryAnnotate -race -count=1`

## Wave 3 — Aggregate + render

- [ ] **T5 — Per-wave/per-spec roll-up in report (+ JSON)**
  - role: builder · depends: T3,T4 · requirements: R3,R6
  - Missing telemetry renders "—".
  - verify: `go test ./internal/cmd/ -run TestReportTelemetry -race -count=1`

- [ ] **T6 — Review: no cost computation / pricing API**
  - role: reviewer · depends: T5 · requirements: R2
  - verify: N/A — complete with `--unverified --evidence "<no pricing call>"`

---

## Traceability

| Wave | Tasks | Requirements |
|------|-------|--------------|
| 1 | T1 | R1, R4 |
| 2 | T2–T4 | R1, R2, R4, R5 |
| 3 | T5–T6 | R2, R3, R6 |
