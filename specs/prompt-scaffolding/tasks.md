# tasks.md — One-shot Scaffolding execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Derive constraints from gates

- [ ] **T1 — Map gate constraints to a single source**
  - role: investigator · depends: — · requirements: R2,R5
  - Locate where the EARS forms, the 7 design headers, and the 7 task keys are
    defined (`ears.go`, `gates.go`, `tasksparser.go`). Report the exported
    symbols a brief generator can read instead of re-listing strings.
  - verify: N/A — complete with `--unverified --evidence "<symbol map>"`

## Wave 2 — Persist prompt + brief generator

- [ ] **T2 — Persist `--from` prompt into the spec**
  - role: builder · depends: — · requirements: R1,R6
  - Add optional `Prompt` to `state.json` and inject it into the
    `requirements.md` stub. `--from` omitted ⇒ unchanged behavior.
  - verify: `go test ./internal/cmd/ -run TestNewFrom -race -count=1`

- [ ] **T3 — `authoring.go` gate-shaped brief generator**
  - role: builder · depends: T1 · requirements: R2,R3,R4
  - Pure function returning per-artifact gate constraints, sourced from the gate
    definitions (no duplicated strings). Text + `SPECD_JSON=1` JSON output. No
    network/LLM.
  - verify: `go test ./internal/core/ -run TestAuthoringBrief -race -count=2`

## Wave 3 — Wire & validate

- [ ] **T4 — Wire `--from` into `new` to emit the brief**
  - role: builder · depends: T2,T3 · requirements: R1,R3
  - verify: `go test ./internal/cmd/ -run TestNewFrom -race -count=1`

- [ ] **T5 — Test: brief stays in sync with real gates**
  - role: verifier · depends: T3 · requirements: R2,R5
  - Assert the brief's EARS forms / design headers / task keys equal the values
    the gates enforce (fails if a gate changes but the brief does not).
  - verify: `go test ./internal/core/ -run TestAuthoringSync -race -count=2`

- [ ] **T6 — Test: faithful draft passes `specd check`**
  - role: verifier · depends: T4 · requirements: R5
  - Build a draft per the brief, run the full gate pipeline, assert pass.
  - verify: `make ci`

- [ ] **T7 — Review: no LLM/network leaked into the binary**
  - role: reviewer · depends: T6 · requirements: R4
  - verify: N/A — complete with `--unverified --evidence "<grep: no net/exec to LLM>"`

---

## Traceability

| Wave | Tasks | Requirements |
|------|-------|--------------|
| 1 | T1 | R2, R5 |
| 2 | T2–T3 | R1–R4, R6 |
| 3 | T4–T7 | R1, R3, R4, R5 |
