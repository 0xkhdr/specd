# tasks.md — Verify Sandboxing execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Runner recon

- [ ] **T1 — Map the current verify execution path**
  - role: investigator · depends: — · requirements: R3,R4
  - Report exactly where `sh -c` runs, env allowlist + NUL rejection live, and
    how the record is written. file:line only.
  - verify: N/A — complete with `--unverified --evidence "<exec path map>"`

## Wave 2 — Runner abstraction

- [ ] **T2 — Extract `Runner` interface; default `shRunner` = today**
  - role: builder · depends: T1 · requirements: R3,R4
  - Behavior byte-identical for `none`. Keep env scrub + NUL reject + print.
  - verify: `go test ./internal/core/ -run TestShRunnerUnchanged -race -count=2`

- [ ] **T3 — Add `Sandbox` to VerificationRecord (back-compat)**
  - role: builder · depends: T1 · requirements: R5
  - verify: `go test ./internal/core/ -run TestRecordSandboxField -race -count=2`

## Wave 3 — Sandbox backends

- [ ] **T4 — `bwrapRunner` (fail-closed if absent)**
  - role: builder · depends: T2 · requirements: R1,R2,R3,R6
  - ro-bind + tmpfs + no-net; preserve env scrub + exit/output passthrough.
  - verify: `go test ./internal/core/ -run TestBwrapRunner -race -count=1`

- [ ] **T5 — `containerRunner` (docker/podman, fail-closed if absent)**
  - role: builder · depends: T2 · requirements: R1,R2,R3,R6
  - verify: `go test ./internal/core/ -run TestContainerRunner -race -count=1`

- [ ] **T6 — Wire `verify.sandbox` config + `--sandbox` flag**
  - role: builder · depends: T4,T5,T3 · requirements: R1,R5
  - verify: `go test ./internal/cmd/ -run TestVerifySandboxSelect -race -count=1`

## Wave 4 — Safety + docs

- [ ] **T7 — Test: fail-closed on missing isolator; `none` regression**
  - role: verifier · depends: T6 · requirements: R2,R4
  - verify: `go test ./... -run 'TestSandboxFailClosed|TestSandboxNoneRegression' -race -count=2`

- [ ] **T8 — Update SECURITY.md isolation + fail-closed contract**
  - role: builder · depends: T7 · requirements: R7
  - verify: N/A — complete with `--unverified --evidence "<SECURITY.md diff>"`

---

## Traceability

| Wave | Tasks | Requirements |
|------|-------|--------------|
| 1 | T1 | R3, R4 |
| 2 | T2–T3 | R3, R4, R5 |
| 3 | T4–T6 | R1, R2, R3, R5, R6 |
| 4 | T7–T8 | R2, R4, R7 |
