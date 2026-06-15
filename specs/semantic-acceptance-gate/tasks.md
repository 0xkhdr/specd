# tasks.md — Semantic Acceptance Gate execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Map the existing scaffolding

- [ ] **T1 — Inventory the acceptance stubs**
  - role: investigator · depends: — · requirements: R1,R4
  - Report exact use sites/absence of `CriterionRecord`, `State.Acceptance`,
    `GatesCfg.Acceptance`, and how `LintEars` numbers criteria. file:line only.
  - verify: N/A — complete with `--unverified --evidence "<stub inventory>"`

## Wave 2 — Parse criteria & mapping

- [ ] **T2 — Number EARS acceptance criteria with stable IDs**
  - role: builder · depends: T1 · requirements: R1,R2
  - Extend `ears.go` so each acceptance criterion has a deterministic ID usable
    for mapping. No change when acceptance gate is off.
  - verify: `go test ./internal/core/ -run TestEarsCriterionID -race -count=2`

- [ ] **T3 — Parse `acceptance:` mapping in tasks.md**
  - role: builder · depends: T1 · requirements: R2
  - Extend `tasksparser.go` to read criterion ID → test-name mappings per task.
  - verify: `go test ./internal/core/ -run TestParseAcceptanceMap -race -count=2`

## Wave 3 — Gate 8 + evidence binding

- [ ] **T4 — `GateAcceptance` (off/warn/error) appended to CheckGates**
  - role: builder · depends: T2,T3 · requirements: R1,R3,R5,R6
  - Mirror `GateTraceability` warn/error semantics; unmapped or never-run mapped
    test ⇒ violation. `off` ⇒ no-op.
  - verify: `go test ./internal/core/ -run TestGateAcceptance -race -count=2`

- [ ] **T5 — Record CriterionRecords on completion**
  - role: builder · depends: T4 · requirements: R4,R5
  - In `task.go`/`verify.go`, parse verify output for mapped test names; write
    `CriterionRecord{Status,Evidence:verifyRef}` into `State.Acceptance`.
  - verify: `go test ./internal/cmd/ -run TestAcceptanceRecord -race -count=2`

## Wave 4 — Surface + backward-compat

- [ ] **T6 — Show criterion coverage in `check` + `report`**
  - role: builder · depends: T4,T5 · requirements: R7
  - verify: `go test ./internal/cmd/ -run 'TestCheckAcceptance|TestReportAcceptance' -race -count=1`

- [ ] **T7 — Test: `acceptance: off` is byte-identical to today**
  - role: verifier · depends: T4 · requirements: R6
  - verify: `go test ./... -run TestAcceptanceOffRegression -race -count=2`

- [ ] **T8 — Review: no LLM judgment, enforcement-only**
  - role: reviewer · depends: T6,T7 · requirements: R1
  - Confirm the gate enforces declared mapping + execution, not test semantics.
  - verify: N/A — complete with `--unverified --evidence "<review notes>"`

---

## Traceability

| Wave | Tasks | Requirements |
|------|-------|--------------|
| 1 | T1 | R1, R4 |
| 2 | T2–T3 | R1, R2 |
| 3 | T4–T5 | R1, R3, R4, R5, R6 |
| 4 | T6–T8 | R1, R6, R7 |
