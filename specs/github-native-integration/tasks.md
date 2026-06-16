# tasks.md — GitHub-native Integration execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Output + exit-code recon

- [x] **T1 — Map check exit codes + report/status output** ✓ complete · 2026-06-16
  - role: investigator · depends: — · requirements: R1,R2,R3
  - Report `specd check` exit-code mapping and the report/status/waves data the
    PR summary will render. file:line only.
  - verify: N/A — complete with `--unverified --evidence "<output+exit map>"`
  - **Evidence:** exit codes — `ExitOK=0`/`ExitGate=1`/`ExitUsage=2`/
    `ExitNotFound=3` `internal/core/exit.go:3-8`; `RunCheck` maps
    `len(violations)==0 ⇒ ExitOK` else `ExitGate` (`check.go:53-56` JSON,
    `check.go:70`/`:76` human); pipeline loop `check.go:28-32`. Render data the
    PR summary reuses — `status --json` rows `internal/cmd/status.go:24-48`
    (+ full state `status.go:77-86`, `next` via `NextRunnable`); `waves --json`
    `internal/cmd/waves.go:21-63` (`waves`, `criticalPath`, `blockers`);
    `report` builds `core.ReportData` `internal/cmd/report.go:32-40` →
    `RenderMarkdown`/`RenderHTML` (`internal/core/report.go:159`/`:185`).
    `check --json` payload `{ok,violations,warnings}` `check.go:49`. All paths
    are deterministic and make zero network calls — the PR-summary builder layers
    on top of these in-process renderers.

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
