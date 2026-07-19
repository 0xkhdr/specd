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

### F1 — `brain step` re-dispatches unclaimed missions — **FIXED** (`71a2ec7`)

`Decide` (`internal/orchestration/decide.go:85-89`) returns `frontier[0]` with no check against
already-dispatched missions, and the frontier was filtered on **leases** alone. An unclaimed
mission leaves no lease, so a repeated `step` re-issued it. Observed during the phase 5 run: T2
dispatched three times (`…s2.T2`, `…s3.T2`, `…s4.T2`), all pending at once.

The fix builds `reservations` (leases + pending missions) *before* the frontier and derives the
withheld set from it. That list already existed a few lines below for the snapshot, so it reuses
the existing notion of in-flight rather than adding a second one. Reservations carry an expiry and
`LeaseWorkerState` (`internal/orchestration/decide.go:27`) already honours it, so a dispatch nobody
claims frees its task instead of wedging the frontier.

Regression: `TestBrainStepDoesNotRedispatchUnclaimedMission`, verified to fail without the fix with
the exact observed symptom. Full suite, `-count=2`, vet, gofmt, both lint scripts, and
`regress-domains.sh` all pass.

### F2 — `MissionStatus` was a 9-value enum with 1 value ever written — **FIXED** (`da739d2`)

`MissionPending` was the only constant assigned in non-test code; `Delivered`, `Claimed`, `Active`,
`Reported`, `Expired`, `Cancelled`, `Escalated`, `Terminal` were never written
(`internal/orchestration/mission.go:17-25`). Mission lifecycle is tracked by lease state and by
moving records `PendingMissions → Missions` on claim (`internal/cmd/brain_claim.go:47`), so the
field is vestigial — completed missions still read `"status": "pending"` in `session.json`.

It was a reader trap: it looked like a state machine and is not one. Resolved subtractively per
Step 1 — the eight unwritten constants are deleted and `MissionPending` carries a comment stating
why no later states exist. `ValidateMission` still rejects any non-pending initial status, so the
fail-closed check is unchanged; the one test that used `MissionActive` as a "not pending" probe now
spells the literal. Suite, `-race`, vet, gofmt, both lint scripts, and `regress-domains.sh` pass.

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

### F7 — Criteria parser aliases labelled sub-criteria, so the ratchet undercounts (gate soundness, medium)

Found while executing Step 3. `specd verify <slug> --criterion 2.2` fails closed with `unknown
criterion "2.2" — not an acceptance criterion in approved requirements.md`, even though
`requirements.md` declares `R2.2` and `status` reports `R2 0/2`.

Cause: `CriterionIDs` (`internal/core/gates/criteria.go:46`) reads criteria as *indented sub-bullets
positionally numbered* under a requirement bullet — the style its own test uses
(`criteria_test.go:6-10`, `- **R1** …` plus unlabelled `  - …` children). This project's specs use
the other style: a `## R2` heading with flat, explicitly labelled `- R2.1:` / `- R2.2:` bullets.
`reqBullet` (`criteria.go:34`, `^\*{0,2}R(\d+)\b`) matches `R2` in *both* `R2.1` and `R2.2` — `\b`
holds because `.` is a non-word char — so each labelled criterion is read as a fresh requirement R2
with zero children, and `flush()` emits `<r>.1` for each.

Verified by dumping the parser against the real doc: it yields `1.1, 2.1, 2.1, 3.1` — `2.1`
duplicated, no `2.2`.

The consequence is worse than an unaddressable id. Aliased ids collapse: a **single** `2.1` record
satisfies **both** entries, so coverage now reads `R2 2/2` and `total 4/4` off three records for
four criteria. Under `criteria.required` or a production profile this ratchet **gates approval**, so
a spec written in the labelled style can pass the gate with genuinely missing criterion evidence.
Not triggered in this project (ratchet unarmed, per F4) — a live risk for anyone who arms it.

Fix (not applied — needs an owner call): teach `reqBullet` to capture an optional `.<n>` and emit
that exact id instead of opening a new requirement. Root-cause fix in the shared parser, so both
authoring styles address correctly; rewriting `requirements.md` is the wrong lever — those files are
approved, and retroactively editing an approved requirements doc is what the amendment path exists
to prevent. Whatever lands needs a test pinning the labelled style: `- R2.1:`/`- R2.2:` must yield
`2.1, 2.2`, never a duplicate.

### F5 — Stale duplicate missions in the phase 5 ledger (cosmetic)

`…s3.T2` and `…s4.T2` remain `pending` in `session.json`, artifacts of F1. Harmless; they expire.
Do not hand-edit — they are ledger history.

### F6 — `TestProductionSmokeLane` asserted the wrong workflow — **FIXED** (`ea6b78e`)

`internal/integration/production_smoke_test.go` pinned its assertion to `.github/workflows/ci.yml`.
Phase 4 (`1dfeae4`) split CI into a fast PR tier and a heavy main tier and moved the lane to
`.github/workflows/heavy.yml:83`, where it still runs — so the check was never lost, but the test
had been failing ever since and left `go test ./...` red on this branch.

Fixed by scanning the workflows directory instead of naming one file: what the test protects is
that CI runs the lane at all, not which tier owns it, so a future retiering cannot make it stale
again. Verified by negative control — the test still fails when the lane is genuinely removed.

## 2. Carried-forward decisions from Phase 5

These are **recorded owner calls**, not open questions. Cite `deep-review-phase5-decisions` as
source when implementing.

- **Delete `report --format otel`.** ✅ Done (`20f7557`). No consumer; redundant with the
  adapter-contract path that maps the neutral `event/v1` stream externally. `--format event` and
  `--format prometheus` stay. Scope ran one step wider than written: `ExportNeutralEvents` had no
  consumer outside its own test either, so the whole `adapter/otel_export.go` projection went
  rather than leaving half a dead file. Net −316 lines.
- **Delete `specd recurring` and `specd spike`.** The only write-only verbs: no non-test reader for
  `recurring-results.jsonl`, `PlanRecurringSuccessor` never called outside tests, `State.Spikes()`
  read only by `state.go`'s own re-parse. Deferral dated 2026-07-19, **revisit 2026-10-19**.
- **Do not consolidate the remaining ledger verbs.** Only 4 of 14 are thin append shells; the other
  10 carry distinct authority and validation semantics. Rejected with named consumers per verb.

## 3. Action plan

Ordered by (risk removed ÷ effort). Each is small; resist bundling.

The suite is green as of `ea6b78e`; both defects found this session (F1, F6) are fixed. What is
left is cleanup and one carried-forward deletion spec.

**Step 1 — F2, subtractive.** ✅ Done (`da739d2`) — deleted the 8 unused constants; lease state
already carries the lifecycle.

**Step 2 — the otel deletion spec.** ✅ Done (`20f7557`) — palette enum, both projection entry
points, and 3 files deleted; `gendocs` regenerated the command reference.

**Step 3 — F4.** ✅ Done (`a2cd180`) — criteria recorded, coverage `0/4 → 4/4`. Not as cheap as
billed: R2.2 is unaddressable and surfaced **F7**, which is now the top open item.

**Step 4 — F7, the criteria-parser aliasing.** Open. Highest risk left in this file: it is the only
finding that can make a *gate* pass with missing evidence.

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
| `d9307df` | This handoff doc |
| `71a2ec7` | **F1 fixed** — dispatched missions reserve their task against the frontier |
| `75bc0bc` | Handoff doc updated for F1 + F6 |
| `ea6b78e` | **F6 fixed** — smoke-lane assertion no longer pinned to `ci.yml` |
| `da739d2` | **F2 fixed** — 8 never-written `MissionStatus` constants deleted |

Branch `optimization`. Not merged to `main`.
