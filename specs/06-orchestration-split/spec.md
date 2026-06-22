# Spec 06 — Split `program_orchestration.go`

> Wave: **W3 (P2)** · Priority: **P2** · Source: LEVEL_UP_PLAN §1.3, §2 P2.8

## 1. Problem

`internal/core/program_orchestration.go` is a **1129-LOC god-file**. The
orchestration subsystem is otherwise spread sensibly across ~12 files, but this
one concentrates too many responsibilities and is the **hottest churn area** —
the last 5 commits all touch orchestration/context. A god-file in the
highest-churn path maximizes merge-conflict and review cost.

The file already has clear internal seams (function prefixes reveal the
responsibility bands):

| Band | Representative funcs |
|---|---|
| **plan / snapshot** | `BuildProgramSnapshot`, `BuildProgramSnapshotWithRuntime`, `programRunnableChildren`, `allProgramChildrenComplete` |
| **decide** | `DecideProgram`, `programStatusDecision`, `programControlDecision`, `buildProgramStatusReport` |
| **dispatch / step** | `StepProgramOrchestration`, `SenseProgramOrchestration`, `programLeasesToStep` |
| **session lifecycle** | `LoadProgramSession`, `ensureProgramSession`, `saveProgramSession`, `validateProgramSession`, `Pause/Resume/CancelProgramOrchestration`, `propagateProgramControl` |
| **lease (reconcile)** | `AcquireProgramChildLease`, `ReleaseProgramChildLease`, `markProgramChildLeaseEscalated`, `LoadProgramChildLeases`, `releaseCompleteProgramChildren`, `load/saveProgramChildLease`, `validateProgramChildLease`, `programChildLeaseIsActive`, `withProgramChildLeaseLock` |

## 2. Solution

**Pure mechanical move** — split along the existing seams into ≤~400-LOC files
within the same `package core`. No logic change, no signature change, no API
change. Tests stay green throughout (they reference exported symbols unchanged).

Proposed split (final names at implementer's discretion, keep the `program_`
prefix for grep-ability):

| New file | Contents | ~LOC |
|---|---|---|
| `program_snapshot.go` | snapshot/plan band | ~250 |
| `program_decide.go` | decision band | ~200 |
| `program_step.go` | sense/step/dispatch band | ~250 |
| `program_session.go` | session lifecycle band | ~250 |
| `program_lease.go` | lease reconcile band | ~350 |

`program_orchestration.go` either disappears or retains only top-level types /
the package's shared orchestration doc comment.

## 3. Acceptance criteria

- [ ] No single non-test file in `internal/core` exceeds **~700 LOC** (target
      ≤~400 for the split files).
- [ ] All exported symbols unchanged (no caller edits required).
- [ ] `git mv`-style move only — `git diff` shows relocations, **not** logic
      edits (review as a no-op refactor).
- [ ] Full suite green, including `-race -count=2`.
- [ ] Co-located test files re-homed sensibly if they map 1:1 to a band
      (optional; do not split a test file mid-logic).

## 4. Sequencing note

Cheaper **after Spec 01** (the worker layer extraction reduces nearby churn) and
naturally pairs with **Spec 02** (lease reconcile band is exactly where
recovery events fire). If Spec 02/03 added code here, split *after* they land to
avoid re-conflicting.

## 5. Non-goals

- Renaming exported functions or changing signatures.
- Behavioral refactor / dedup. Move only; behavior changes belong in their own
  spec.
- Splitting `internal/core` into sub-packages (out of scope; same package).

## 6. Risks & mitigations

| Risk | Mitigation |
|---|---|
| Accidental logic change during move | Review `git diff` as pure relocation; suite + `-race` must stay green with zero new/changed assertions |
| Merge conflict with in-flight W3 specs | Land this split last within W3, after lint fixes settle |
| Hidden file-private coupling (unexported helpers) | Keep tightly-coupled unexported helpers in the same band file as their callers |
