# tasks.md — Plugin / Custom-Gate API execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Gate pipeline recon

- [ ] **T1 — Map CheckGates pipeline + env-scrub helper**
  - role: investigator · depends: — · requirements: R1,R3,R6
  - Report how `CheckGates` runs/merges violations, the exit-code mapping, and
    the reusable verify env-scrub/NUL-reject helper. file:line only.
  - verify: N/A — complete with `--unverified --evidence "<pipeline map>"`

## Wave 2 — Contract + runner

- [ ] **T2 — Define `CustomGateInput`/`Output` JSON contract**
  - role: builder · depends: T1 · requirements: R2,R7
  - verify: `go test ./internal/core/ -run TestCustomGateSchema -race -count=2`

- [ ] **T3 — `customgate.go` runner (bounded timeout, env-scrubbed)**
  - role: builder · depends: T1,T2 · requirements: R1,R5,R6
  - JSON in/out; reuse verify env-scrub; bounded timeout; invalid JSON ⇒ error.
  - verify: `go test ./internal/core/ -run TestCustomGateRunner -race -count=1`

## Wave 3 — Pipeline integration

- [ ] **T4 — `gates.custom` config + pipeline integration (warn/error)**
  - role: builder · depends: T3 · requirements: R1,R2,R4
  - Append synthetic gate; merge violations; level per config.
  - verify: `go test ./internal/core/ -run TestCustomGatePipeline -race -count=2`

- [ ] **T5 — Test: 7 core gates unchanged with/without custom gates**
  - role: verifier · depends: T4 · requirements: R3
  - verify: `go test ./internal/core/ -run TestCoreGatesUnchanged -race -count=2`

- [ ] **T6 — Document the stdin/env/stdout contract**
  - role: builder · depends: T4 · requirements: R7
  - verify: N/A — complete with `--unverified --evidence "<docs diff>"`

- [ ] **T7 — Review: no Go plugin loading, no network**
  - role: reviewer · depends: T5,T6 · requirements: R1
  - verify: N/A — complete with `--unverified --evidence "<exec-only audit>"`

---

## Traceability

| Wave | Tasks | Requirements |
|------|-------|--------------|
| 1 | T1 | R1, R3, R6 |
| 2 | T2–T3 | R1, R2, R5, R6, R7 |
| 3 | T4–T7 | R1, R2, R3, R4, R7 |
