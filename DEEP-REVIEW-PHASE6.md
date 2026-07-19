# Deep review — Phase 6 follow-ups

Successor to `DEEP-REVIEW.md` §4 Phase 5, which is **complete** (spec
`deep-review-phase5-decisions`, commits `7823194`, `1d78831`). This file holds what Phase 5
surfaced but did not own, plus defects found while executing it. Written as a cold-start handoff:
every claim below was verified against code or a running binary, and each item names the evidence
so the next session does not have to re-derive it.

## 0. Read this first — how the last session went wrong

Three findings in the Phase 5 session were **wrong on first pass**, all from the same habit:
inferring behavior from `grep` coverage instead of reading the load/contract functions or running
the thing.

- Claimed `decisions.json` having no writer was a defect. It is a declared input, opt-in by
  presence. Corrected on the ledger as `decision:4`.
- Cited `scripts/regress-domains.sh:204` as a live Prometheus driver. Line 206 actually probes
  `--format __bogus__` to assert enum *rejection*.
- Documented the governance gate as arming on file presence. It also requires
  `config.profile = production` (`internal/cmd/registry.go:188`).

Two were caught by subagents refusing to write the claim as fact; one only surfaced by testing
already-committed docs. **Before acting on any item below, read the named function.** Absence of a
writer in this codebase usually means *declared input*, not *defect* — that pattern is load-bearing
and appears at least four times (`decisions.json`, `drift.json`, `exceptions.json`, `allow.json`).

## 1. Findings

### F1 — `brain step` re-dispatches unclaimed missions (defect, medium)

`Decide` (`internal/orchestration/decide.go:85-89`) sorts the frontier and returns `frontier[0]`
with no check against already-dispatched missions. Re-issue is prevented only by **leases**
(`internal/cmd/brain_run.go:121-129`), so a mission that is dispatched but not yet *claimed* leaves
no lease and is re-issued on the next `step`.

The intent is stated in that file's own comment — *"live-leased tasks are withheld so a repeated
step … advances to the next task instead of re-issuing an in-flight one"* — so this is a gap
against declared intent, not a design choice.

Observed: T2 was dispatched three times (`…s2.T2`, `…s3.T2`, `…s4.T2`), all pending simultaneously.

### F2 — `MissionStatus` is a 9-value enum with 1 value ever written (dead flexibility, low)

`MissionPending` is the only constant assigned in non-test code; `Delivered`, `Claimed`, `Active`,
`Reported`, `Expired`, `Cancelled`, `Escalated`, `Terminal` are never written
(`internal/orchestration/mission.go:17-25`). Mission lifecycle is tracked by lease state and by
moving records `PendingMissions → Missions` on claim (`internal/cmd/brain_claim.go:47`), so the
field is vestigial — completed missions still read `"status": "pending"` in `session.json`.

This is a reader trap: it looks like a state machine and is not one.

### F3 — `specd exception` writes a different store than the governance gate reads (naming, medium)

| File | Shape | Written by | Read by |
|---|---|---|---|
| `.specd/specs/<slug>/exceptions.json` | array of `core.ExceptionV1` | **nothing** (declared input) | governance gate, `internal/cmd/registry.go:208` |
| `.specd/security/exceptions.jsonl` | JSONL of `security.Exception` | `specd exception approve\|revoke` | security allowlist |
| `.specd/security/allow.json` | fingerprint + reason | nothing | security gate, only when `exceptions.jsonl` absent |

The verb name points at the wrong file for anyone trying to satisfy the governance gate, and the
two record shapes share no fields. Same collision as `specd decision` vs `decisions.json`. Both are
now documented (`docs/validation-gates.md`), but the code-level collision remains.

### F4 — Phase 5 acceptance criteria never recorded (process residue, low)

`specd status deep-review-phase5-decisions` reports `total 0/4 criteria passing` (R1 0/1, R2 0/2,
R3 0/1) despite all four tasks complete and the spec approved. The criteria ratchet is not armed in
this project (no `criteria.required`, no production profile), so it did not block — but nothing maps
task evidence back to requirement criteria. Record with
`specd verify <slug> --criterion <r>.<n> --status pass --evidence <text>`.

### F5 — Stale duplicate missions in the phase 5 ledger (cosmetic)

`…s3.T2` and `…s4.T2` remain `pending` in `session.json`, artifacts of F1. Harmless; they expire.
Do not hand-edit — they are ledger history.

## 2. Carried-forward decisions from Phase 5

These are **recorded owner calls**, not open questions. Cite `deep-review-phase5-decisions` as
source when implementing.

- **Delete `report --format otel`.** No consumer; redundant with the adapter-contract path that
  maps the neutral `event/v1` stream externally. `--format event` and `--format prometheus` stay.
  *The task list must include `go run ./tools/gendocs`* — removing the enum value from
  `internal/core/commands.go:550` regenerates `docs/command-reference.md`, and `docs-lint.sh` fails
  CI on drift.
- **Delete `specd recurring` and `specd spike`.** The only write-only verbs: no non-test reader for
  `recurring-results.jsonl`, `PlanRecurringSuccessor` never called outside tests, `State.Spikes()`
  read only by `state.go`'s own re-parse. Deferral dated 2026-07-19, **revisit 2026-10-19**.
- **Do not consolidate the remaining ledger verbs.** Only 4 of 14 are thin append shells; the other
  10 carry distinct authority and validation semantics. Rejected with named consumers per verb.

## 3. Action plan

Ordered by (risk removed ÷ effort). Each is small; resist bundling.

**Step 1 — F1, the only real defect.** Withhold tasks with a live *dispatched* mission, not just a
live lease. Likely a few lines in the `withheld` set construction at `internal/cmd/brain_run.go:121`,
reusing `session.PendingMissions`. Check mission expiry so a stale dispatch cannot wedge the
frontier forever. Test: two consecutive `brain step` calls with no claim in between must dispatch
two *different* tasks.

**Step 2 — F2, subtractive.** Either populate the status field or delete the 8 unused constants.
Prefer deletion; lease state already carries the lifecycle. If any is kept, keep only what a reader
is actually shown.

**Step 3 — the otel deletion spec.** Straightforward but touches the palette. Remember `gendocs`.

**Step 4 — F4.** One `specd verify --criterion` call per criterion, or accept the gap and note it.
Cheap either way.

**Defer:** F3 (rename is churn across docs, templates, and muscle memory — the docs fix may be
enough), F5 (self-clearing), and the `recurring`/`spike` deletion until the 2026-10-19 review date.

## 4. Working agreements that earned their keep

- **Read the contract before believing a grep.** See §0.
- **Let the gates refuse.** The scope gate caught a bad completion; two subagents refused to write
  unverified claims. Every refusal in that session was correct.
- **Never commit another task's diff to unblock a worker.** Scope derives from
  `mission.subject_head` (`internal/cmd/lifecycle.go:273`), not the worktree, so committing a stray
  file converts a clearable worktree blocker into a permanent history one. Land stray files
  *before* the mission's baseline, or re-dispatch to re-pin. This cost ~10 minutes waiting out a
  lease.
- **Config lives at `<root>/project.yml`**, never `.specd/project.yml` (`internal/cmd/registry.go:110`).
  A dead duplicate at the wrong path was deleted in `a67a350`.

## 5. Session commits

| Commit | What |
|---|---|
| `7823194` | Phase 5 decisions recorded |
| `1d78831` | Spec marked complete |
| `38207b0` | `decisions.json` / `drift.json` documented as declared inputs |
| `7a390d8` | Corrected governance arming; exceptions + verb collision documented |
| `a67a350` | Dead `.specd/project.yml` deleted |

Branch `optimization`. Not merged to `main`.
