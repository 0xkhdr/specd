# tasks.md — Replay & Spec Diff execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Audit-source recon

- [x] **T1 — Inventory on-disk audit records** ✓ complete · 2026-06-16
  - role: investigator · depends: — · requirements: R1,R3
  - Map status history + verify records in `state.json`, decision/midreq file
    formats, and how phase-entry timestamps relate to git commits. file:line only.
  - verify: N/A — complete with `--unverified --evidence "<audit-source map>"`
  - **Evidence:** state.json audit fields — `TaskState.{StartedAt, FinishedAt,
    Evidence, Verification, Blocker}` `internal/core/state.go:80-84`;
    `VerificationRecord` `state.go:52-62` (incl. `GitHead`, `RanAt`);
    `CriterionRecord` `state.go:64-70`; top-level `CreatedAt`/`UpdatedAt`/`Turn`
    `state.go:101-103`, `Revision` `state.go:96`. Transitions stamped in `RunTask`
    `internal/cmd/task.go:139-184` (`FinishedAt` `:149`, `StartedAt` `:150`/`:171`).
    decisions.md — ADR blocks written by `RunDecision`
    `internal/cmd/decision.go:55-57` (`## ADR-NNN — text · date`), numbered by
    `nextADRNumber`/`adrNumRE` `decision.go:12-24`. mid-requirements.md — Turn
    blocks by `RunMidreq` `internal/cmd/midreq.go:57-59`, `state.Turn++`
    `midreq.go:50`, stamp `midreq.go:57`. git relation — only per-verify
    `GitHead` (`verify.go:254` → `state.go:61`); **no phase-entry→commit log**
    beyond `CreatedAt`/`UpdatedAt`, so replay orders by `RanAt`/`FinishedAt`
    timestamps and ties-breaks deterministically by task `ordinal`.

## Wave 2 — Replay

- [x] **T2 — `replay.go` event collector + stable ordering** ✓ complete · 2026-06-16
  - role: builder · depends: T1 · requirements: R1,R5,R6
  - Normalize sources into `TimelineEvent`; stable sort; tolerate missing/corrupt.
  - verify: `go test ./internal/core/ -run TestReplayTimeline -race -count=2`
  - **Evidence:** `internal/core/replay.go` — `TimelineEvent` + `ReplayTimeline`
    collects task start/finish/verify(±)/block + acceptance records, stable-sorts
    by (timestamp, task ordinal, kind). Total/read-only: nil & empty state and
    missing timestamps are tolerated without panic. `TestReplayTimeline` passes.

- [x] **T3 — `specd replay` command (text + JSON)** ✓ complete · 2026-06-16
  - role: builder · depends: T2 · requirements: R1,R2,R4
  - Read-only; `SPECD_JSON=1` typed array.
  - verify: `go test ./internal/cmd/ -run TestReplayCmd -race -count=1`
  - **Evidence:** `internal/cmd/replay.go` (`RunReplay`) loads state and renders
    text or a typed `[]TimelineEvent` JSON array; never writes. Registered in
    `Registry` + `core.Commands` (parity green). `TestReplayCmd` passes.

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
