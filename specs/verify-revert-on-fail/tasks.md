# tasks.md — Automatic Rollback on Failed Verify execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Verify-path recon

- [ ] **T1 — Map RunVerify exit handling + git usage**
  - role: investigator · depends: — · requirements: R1,R4
  - Report where exit code is evaluated, where the record is written, and the
    existing git HEAD-capture call. file:line only.
  - verify: N/A — complete with `--unverified --evidence "<verify-path map>"`

## Wave 2 — Safe revert

- [ ] **T2 — Repo-safety pre-check (skip+warn on unsafe state)**
  - role: builder · depends: T1 · requirements: R3
  - Detect non-git / merge-rebase-in-progress; refuse to revert.
  - verify: `go test ./internal/cmd/ -run TestRevertSafetyGuard -race -count=1`

- [ ] **T3 — `--revert-on-fail` recoverable stash on non-zero exit**
  - role: builder · depends: T2 · requirements: R1,R2,R4,R5
  - `git stash push -u`; print stash ref; pass/default untouched.
  - verify: `go test ./internal/cmd/ -run TestRevertOnFail -race -count=1`

- [ ] **T4 — Record `Reverted`/`StashRef` in VerificationRecord**
  - role: builder · depends: T3 · requirements: R6
  - verify: `go test ./internal/core/ -run TestRevertRecord -race -count=2`

## Wave 3 — Regression + review

- [ ] **T5 — Test: flag unset is byte-identical; pass never touches tree**
  - role: verifier · depends: T3 · requirements: R4,R5
  - verify: `go test ./... -run 'TestRevertDefaultRegression|TestRevertPassNoop' -race -count=2`

- [ ] **T6 — Review: no reset --hard, evidence gate intact**
  - role: reviewer · depends: T4,T5 · requirements: R2
  - verify: N/A — complete with `--unverified --evidence "<recoverable-only audit>"`

---

## Traceability

| Wave | Tasks | Requirements |
|------|-------|--------------|
| 1 | T1 | R1, R4 |
| 2 | T2–T4 | R1, R2, R3, R4, R5, R6 |
| 3 | T5–T6 | R2, R4, R5 |
