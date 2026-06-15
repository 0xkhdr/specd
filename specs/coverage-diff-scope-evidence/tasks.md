# tasks.md — Coverage & Diff-scope Evidence execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Record & contract recon

- [ ] **T1 — Map verify record + files-contract plumbing**
  - role: investigator · depends: — · requirements: R1,R2
  - Report `VerificationRecord` shape, where HEAD is captured in
    `RunVerify`, and how `files:` is parsed/stored. file:line only.
  - verify: N/A — complete with `--unverified --evidence "<plumbing map>"`

## Wave 2 — Capture evidence

- [ ] **T2 — Add `ChangedFiles` + `Coverage` to the record (back-compat)**
  - role: builder · depends: T1 · requirements: R1,R3
  - `omitempty` JSON; existing records still parse.
  - verify: `go test ./internal/core/ -run TestVerificationRecordCompat -race -count=2`

- [ ] **T3 — Capture changed files + optional coverage in `RunVerify`**
  - role: builder · depends: T2 · requirements: R1,R3,R4
  - `git diff --name-only` vs base HEAD; parse coverage total if present, else
    "unavailable" (never fail on coverage).
  - verify: `go test ./internal/cmd/ -run TestVerifyCapture -race -count=1`

## Wave 3 — Scope gate + surface

- [ ] **T4 — `GateScope` (warn/error, `*`/unset = no-op)**
  - role: builder · depends: T3 · requirements: R2,R5
  - `filepath.Match` changed files vs task `files:` globs.
  - verify: `go test ./internal/core/ -run TestGateScope -race -count=2`

- [ ] **T5 — Report shows changed-file count + coverage**
  - role: builder · depends: T3 · requirements: R6
  - verify: `go test ./internal/cmd/ -run TestReportScope -race -count=1`

- [ ] **T6 — Review: coverage is evidence, not a binary floor**
  - role: reviewer · depends: T4,T5 · requirements: R4
  - verify: N/A — complete with `--unverified --evidence "<no hardcoded floor>"`

---

## Traceability

| Wave | Tasks | Requirements |
|------|-------|--------------|
| 1 | T1 | R1, R2 |
| 2 | T2–T3 | R1, R3, R4 |
| 3 | T4–T6 | R2, R4, R5, R6 |
