# 0004 Program Links Scope

Status: accepted

Context:
Spec 12 ports v1's cross-spec ordering: `link`/`unlink`, a program frontier read
via `status --program`, and approval-gated enforcement. v1's `status --program`
also carried `schedule`/`tick` maintenance verbs that walked the program on a
timer.

Decision:
- **`schedule`/`tick` are not ported.** Periodic program maintenance is the
  host's job: cron (or any scheduler) re-runs `specd check` / `specd status
  --program` on whatever cadence the operator wants. A timer loop inside the
  binary would duplicate that and drag scheduler state into a tool that is
  otherwise a pure function of on-disk `.specd/` state. This matches FINDINGS
  "what NOT to bring back".
- **Links live in their own file, `.specd/program.json`.** They are never
  written into any spec's `state.json`, so every file keeps a single writer and
  the lock story stays simple (one root lock already serializes program writes;
  no second lock, no deadlock order to reason about).
- **Enforcement gates execution, not planning.** Incomplete dependencies block
  only the transition into `executing` (approval into the execute phase). A
  dependent spec may still be authored and planned early — mirroring how task
  deps gate dispatch, not authoring.
- **One completion predicate.** "Complete" for the frontier and the approval
  gate is the same all-gates-green + all-tasks-complete check `submit` uses; it
  is not redefined.

Consequences:
Program-level wave dispatch/orchestration stays out of scope (the brain remains
single-spec). Revisiting `schedule`/`tick` means a new spec, not an edit to this
record.
