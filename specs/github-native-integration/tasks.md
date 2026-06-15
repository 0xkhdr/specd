# tasks.md — GitHub-native Integration execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Output + exit-code recon

- [ ] **T1 — Map check exit codes + report/status output**
  - role: investigator · depends: — · requirements: R1,R2,R3
  - Report `specd check` exit-code mapping and the report/status/waves data the
    PR summary will render. file:line only.
  - verify: N/A — complete with `--unverified --evidence "<output+exit map>"`

## Wave 2 — Deterministic PR summary (specd side, no network)

- [ ] **T2 — `specd report --pr-summary` Markdown + JSON**
  - role: builder · depends: T1 · requirements: R3
  - Wave DAG + gate status; deterministic; zero network.
  - verify: `go test ./internal/cmd/ -run TestPRSummary -race -count=2`

- [ ] **T3 — Commit↔task link map (unreferenced listed, not dropped)**
  - role: builder · depends: T2 · requirements: R4
  - Parse task IDs from commit messages, deterministically.
  - verify: `go test ./internal/core/ -run TestCommitTaskLink -race -count=2`

- [ ] **T4 — Test: PR-summary path makes no network call**
  - role: verifier · depends: T2,T3 · requirements: R3
  - verify: `go test ./internal/cmd/ -run TestPRSummaryNoNetwork -race -count=2`

## Wave 3 — Action wrapper + docs

- [ ] **T5 — Composite Action: check status + upsert PR comment**
  - role: builder · depends: T2 · requirements: R1,R2,R5
  - `.github/actions/specd-pr/`: run check, set status from exit code, upsert
    comment via `GITHUB_TOKEN`; pinned refs, least-privilege permissions.
  - verify: N/A — complete with `--unverified --evidence "<action.yml + actionlint>"`

- [ ] **T6 — Workflow snippet + permissions docs**
  - role: builder · depends: T5 · requirements: R6
  - verify: N/A — complete with `--unverified --evidence "<docs diff>"`

- [ ] **T7 — Review: no network in binary, supply-chain pinned**
  - role: reviewer · depends: T4,T5 · requirements: R3,R5
  - verify: N/A — complete with `--unverified --evidence "<review notes>"`

---

## Traceability

| Wave | Tasks | Requirements |
|------|-------|--------------|
| 1 | T1 | R1, R2, R3 |
| 2 | T2–T4 | R3, R4 |
| 3 | T5–T7 | R1, R2, R5, R6 |
