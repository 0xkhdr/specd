# 12-program-links — Cross-spec dependencies and program frontier

Wave 2. FINDINGS refs: B.18, D-tier2 item 14.

## Problem

Two specs cannot express ordering today, so a multi-spec effort loses the
frontier guarantee at the program level — the core DAG/wave discipline
stops at spec boundaries. v1's `status --program` had `link/unlink`
(cycle-refused), a program frontier, plus `schedule`/`tick` maintenance.
FINDINGS verdict: **adapt (link/unlink only)** — port linking and the
program frontier read; skip schedules/tick (host cron + `check` covers it;
record as ADR).

## Requirements (EARS)

- R1: WHEN a user runs `specd link <from-slug> <to-slug>`, THE SYSTEM SHALL
  record that `<from-slug>` depends on `<to-slug>` (to must complete
  first); linking to a nonexistent slug exits 2.
- R2: WHEN a proposed link would create a cycle in the cross-spec graph,
  THE SYSTEM SHALL refuse (exit 1) printing the cycle path.
- R3: WHEN a user runs `specd unlink <from-slug> <to-slug>`, THE SYSTEM
  SHALL remove the link; removing a nonexistent link exits 2.
- R4: WHEN a user runs `specd status --program`, THE SYSTEM SHALL show all
  specs, their links, each spec's phase, and the **program frontier**: the
  set of specs whose dependencies are all complete and are therefore
  actionable now.
- R5: WHILE a spec has incomplete dependencies, THE SYSTEM SHALL refuse
  phase approval into the execution phase for it (exit 1 naming the
  blocking specs) — links are enforcement, not annotation.
- R6: Link state SHALL live at the program level
  (`.specd/program.json` or equivalent), written atomically, versioned
  under spec 02's schema discipline, and never inside another spec's
  `state.json` (single writer per file, lock story stays simple).

## Design notes / best practice

- Reuse the task-level DAG machinery (`dag.go`, `frontier.go`) conceptually
  — same acyclicity check, same frontier computation, different node type.
  Factor shared graph code only if it falls out naturally; do not force an
  abstraction over two call sites.
- Enforcement point (R5): the approval gate consults program links when
  advancing to execution — planning phases stay unblocked (you may plan a
  dependent spec early; you may not execute it). This mirrors how task
  deps gate dispatch, not authoring.
- Locking: program.json gets its own lock file, acquired *after* any
  spec lock if both are needed (fixed order prevents deadlock — document
  the order in lock.go comment).
- Completion definition for R4/R5: spec phase = complete (all tasks
  complete + gates green), the same predicate submit (spec 08) uses — one
  predicate, shared.
- Skip/ADR: `schedule`/`tick` (host cron), per FINDINGS "what NOT to bring
  back".

## Out of scope

- Program-level waves dispatch/orchestration (brain stays single-spec).
- Schedules, tick maintenance.

## Acceptance

- Link a→b, b→c: cycle attempt c→a refused with printed path;
  `status --program` shows frontier = {c} when nothing complete; approving
  a into execution refused naming b; completing deps unblocks; unlink
  removes enforcement. Full suite green, concurrent link/unlink stress
  clean.
