# tasks.md — Replay & Spec Diff execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Audit-source recon

- [ ] **T1 — Inventory on-disk audit records**
  - role: investigator · depends: — · requirements: R1,R3
  - Map status history + verify records in `state.json`, decision/midreq file
    formats, and how phase-entry timestamps relate to git commits. file:line only.
  - verify: N/A — complete with `--unverified --evidence "<audit-source map>"`

## Wave 2 — Replay

- [ ] **T2 — `replay.go` event collector + stable ordering**
  - role: builder · depends: T1 · requirements: R1,R5,R6
  - Normalize sources into `TimelineEvent`; stable sort; tolerate missing/corrupt.
  - verify: `go test ./internal/core/ -run TestReplayTimeline -race -count=2`

- [ ] **T3 — `specd replay` command (text + JSON)**
  - role: builder · depends: T2 · requirements: R1,R2,R4
  - Read-only; `SPECD_JSON=1` typed array.
  - verify: `go test ./internal/cmd/ -run TestReplayCmd -race -count=1`

## Wave 3 — Diff

- [ ] **T4 — `specd diff --from --to` over artifact git history**
  - role: builder · depends: T1 · requirements: R3,R4,R5
  - Map phase transitions to commits; `git diff` artifact paths.
  - verify: `go test ./internal/cmd/ -run TestSpecDiff -race -count=1`

- [ ] **T5 — Test: deterministic output, read-only, no panic on gaps**
  - role: verifier · depends: T3,T4 · requirements: R4,R5,R6
  - verify: `go test ./... -run 'TestReplayDeterministic|TestReplayMissing' -race -count=2`

- [ ] **T6 — Review: no LLM, no mutation**
  - role: reviewer · depends: T5 · requirements: R4
  - verify: N/A — complete with `--unverified --evidence "<read-only audit>"`

---

## Traceability

| Wave | Tasks | Requirements |
|------|-------|--------------|
| 1 | T1 | R1, R3 |
| 2 | T2–T3 | R1, R2, R4, R5, R6 |
| 3 | T4–T6 | R3, R4, R5, R6 |
