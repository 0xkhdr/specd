# Requirements — workflow-09-driver-session

Release I fixes the driver-session and completion ergonomics that make a serial task chain pay a
close/open/re-verify cycle per task and turn the harness's own bookkeeping into out-of-scope
refusals. Source: `WORKFLOW-FEEDBACK.md` open entries on baseline advancement, harness-written
marker attribution, untracked files, nonce spend on refusal, and the undisclosed completion
preamble.

## R1 — Baseline tracks the driven task

owner: project maintainers
priority: must
risk: high

- R1.1: When a driver session acknowledges a new task, the system shall re-pin the session diff-scope baseline to current HEAD so the previous task committed files fall outside the current task diff.
- R1.2: When a single verb to re-pin the baseline is preferable, the system shall expose a session rotate operation that re-pins without discarding evidence already recorded for the new task.
- R1.3: When the baseline is re-pinned, the system shall continue to derive it from git and pin it before mutable work begins, preserving evidence integrity.

## R2 — Harness-written state is never attributed to the worker

owner: project maintainers
priority: must
risk: high

- R2.1: When a completion diff includes a harness-owned path whose worktree content matches exactly what a specd verb wrote, the system shall not attribute that path to the current task.
- R2.2: When a serial mission inherits a baseline that excludes a prior brain-reported task-marker mutation, the system shall not later classify that controller-owned marker as a worker edit.
- R2.3: When HEAD changes after dispatch, the system shall make a stale-baseline refusal name and support one deterministic reissue command that revokes the stale lease and re-mints the same task at current HEAD.

## R3 — Only task-attributable untracked files block completion

owner: project maintainers
priority: must
risk: medium

- R3.1: When an untracked file predates the session baseline, the system shall not attribute it to the task or block completion on it.
- R3.2: When an untracked file appears after the session baseline, the system shall attribute it to the task and require it in scope as today.

## R4 — Nonce survives a non-mutating refusal

owner: project maintainers
priority: must
risk: medium

- R4.1: When a bound completion refuses without mutating state, the system shall either leave the nonce unspent or print the next usable nonce inline so a corrected retry needs no extra session action round trip.
- R4.2: When a nonce is reported as spent, the system shall name that the nonce is gone in the refusal that consumes it.

## R5 — The completion preamble is disclosed and copy-pasteable

owner: project maintainers
priority: must
risk: medium

- R5.1: When verify records evidence for a task with an open driver session, the system shall emit the fully bound completion command including session and nonce in its success line and JSON output.
- R5.2: When a session is opened, the system shall disclose the ordered ack, action, and bound-completion sequence rather than requiring the caller to discover the ordering by failing.
- R5.3: When completion requires a context acknowledgement, the system shall name the exact acknowledgement command in the binding-missing refusal.

## R6 — Marker sync does not self-refuse

owner: project maintainers
priority: should
risk: low

- R6.1: When completion updates the tasks marker, the system shall not fail its own completion because that marker file is staged or dirty, either by unstaging it before the update or by exempting the harness-written marker from diff scope.

## Edge and failure behavior

- A worktree with a stray scratch file present before the session opened completes normally.
- Re-pinning the baseline never resurrects evidence pinned to a prior HEAD as current.
- The disclosed completion command remains correct across serial tasks in one session.

## Non-goals

- Weakening diff-scope enforcement for files the worker actually changed.
- Removing single-use nonces or the passing-evidence completion rule.
- Persisting driver session context beyond the one-task-per-invocation role contract.
