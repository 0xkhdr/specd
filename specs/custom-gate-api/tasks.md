# tasks.md — Plugin / Custom-Gate API execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Gate pipeline recon

- [x] **T1 — Map CheckGates pipeline + env-scrub helper** ✓ complete · 2026-06-16
  - role: investigator · depends: — · requirements: R1,R3,R6
  - Report how `CheckGates` runs/merges violations, the exit-code mapping, and
    the reusable verify env-scrub/NUL-reject helper. file:line only.
  - verify: N/A — complete with `--unverified --evidence "<pipeline map>"`
  - **Evidence:** pipeline — `CheckGates []CheckGate`
    `internal/core/gates.go:26-34` (7 ordered gates; order is a stated
    user-visible contract); `CheckGate` func type `gates.go:21`, shared read-only
    `CheckCtx` `gates.go:10-16`. Run/merge — `RunCheck` loop `internal/cmd/check.go:28-32`
    appends each gate's `violations`/`warnings`. Exit mapping — `check.go:53-56`
    (JSON), `check.go:64-76` (human): any violation ⇒ `ExitGate`, else `ExitOK`
    (`internal/core/exit.go:3-8`). Reusable env-scrub — `scrubbedEnv`
    `internal/cmd/verify.go:32-46` (allowlist + `SPECD_*`); NUL-reject pattern
    `verify.go:95-97`; bounded timeout via `context.WithTimeout` `verify.go:215-216`.
    Custom-gate runner reuses `scrubbedEnv` + timeout + NUL-reject, appends a
    `GateCustom` to the pipeline tail (warn/error), and must never load Go
    plugins or touch the network.

## Wave 2 — Contract + runner

- [x] **T2 — Define `CustomGateInput`/`Output` JSON contract** ✓ complete · 2026-06-16
  - role: builder · depends: T1 · requirements: R2,R7
  - verify: `go test ./internal/core/ -run TestCustomGateSchema -race -count=2`
  - **Evidence:** `internal/core/customgate.go` — `CustomGateInput`
    (spec/root/status/tasks, read-only), `CustomGateOutput` (violations/warnings
    of `CustomGateFinding`), `BuildCustomGateInput`. `TestCustomGateSchema` passes.

- [x] **T3 — `customgate.go` runner (bounded timeout, env-scrubbed)** ✓ complete · 2026-06-16
  - role: builder · depends: T1,T2 · requirements: R1,R5,R6
  - JSON in/out; reuse verify env-scrub; bounded timeout; invalid JSON ⇒ error.
  - verify: `go test ./internal/core/ -run TestCustomGateRunner -race -count=1`
  - **Evidence:** `RunCustomGate` writes input JSON to stdin, runs `sh -c` with
    `core.ScrubbedEnv()` (env-scrub now single-sourced; `verify.go` delegates),
    bounded `context.WithTimeout`, NUL-reject; parses stdout with
    `DisallowUnknownFields`. Invalid JSON / non-zero exit / timeout ⇒ error. No Go
    plugins, no network. `TestCustomGateRunner` passes.

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
