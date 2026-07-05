# 07-brain-safety — cancel, crash-safe resume, per-step checkpoint

Wave 1. FINDINGS refs: B.19, C.6, D-tier1 item 9.

## Problem

The brain controller is `start/step/run/status` only: no `cancel`, no
documented crash-recovery semantics for a half-run session, no
checkpointing. The lease mechanism bounds damage, but the operator has no
verb to act with. FINDINGS: "a controller that cannot be paused,
cancelled, or resumed after a crash is not safe to leave driving waves."
v1 had pause/resume/cancel/checkpoint, `--ledger`, session recovery (17 KB
of recovery tests). Verdict: **port (staged)** — minimum is `cancel`,
crash-safe `resume`, checkpoint on every step; `pause` and `directive`
follow later.

## Requirements (EARS)

- R1 (checkpoint): WHEN the brain completes any step, THE SYSTEM SHALL have
  durably recorded (atomic write) a checkpoint sufficient to reconstruct
  session position: session id, step number, wave, decision issued,
  outstanding dispatches, lease state.
- R2 (cancel): WHEN a user runs `specd brain cancel`, THE SYSTEM SHALL mark
  the session cancelled, release its lease, refuse further `step`/`run`
  against it (exit 1), and leave all task/evidence state untouched —
  in-flight worker results may still be reported through normal verbs but
  trigger no new decisions.
- R3 (resume): WHEN `specd brain resume` runs after a crash (live session
  record + expired-or-orphaned lease), THE SYSTEM SHALL reconstruct from
  the last checkpoint + on-disk facts (ACP ledger, verify records) and
  continue without double-dispatching any mission already dispatched.
- R4 (fail closed on ambiguity): IF checkpoint and ledger disagree in a way
  recovery rules cannot reconcile deterministically, THEN THE SYSTEM SHALL
  refuse to resume (exit 1) and print a diagnostic naming the conflict —
  never guess.
- R5 (single controller): WHILE a session holds a live lease, THE SYSTEM
  SHALL refuse `start` and `resume` of another session on the same spec
  (existing lease semantics extended to the new verbs).
- R6 (status): `brain status` SHALL show session state
  (running|cancelled|crashed|complete), last checkpoint step/time, and
  lease holder/expiry.

## Design notes / best practice

- Checkpoint = pure serialization of the decision loop's inputs, written
  via `core.AtomicWrite` before the step's effects are considered
  committed — write-ahead ordering: checkpoint first, then dispatch
  visible. Recovery replays: last checkpoint + ledger delta.
- Idempotent dispatch (R3): missions carry deterministic ids
  (session/step/task); resume re-issues a dispatch only if no ACP record
  with that id exists. This is the core double-dispatch guard — test it
  with a kill between checkpoint and ledger append.
- Cancel is terminal (no un-cancel); restart = new `start` after lease
  release. Keeps the state machine small: running → cancelled|complete,
  running → (crash) → resumed-running|refused.
- Crash-injection tests: drive the loop, kill at chosen points
  (post-checkpoint/pre-dispatch, post-dispatch/pre-ledger), assert resume
  converges. Use the existing stress-script pattern
  (`scripts/stress*.sh` per spec 00 outcome) for cross-process contention:
  two `resume` calls racing must yield exactly one holder (R5).
- v1's recovery tests under `reference/` are the design checklist for
  cases; re-implement, do not copy.
- `pause/resume-from-pause` and `directive` deferred — record in decision
  notes (spec 00's ADR set covers the deferral).

## Out of scope

- `brain pause` (staged later), `directive`, `--compact`, ledger pretty-
  printing.
- Any change to decide.go decision policy itself.

## Acceptance

- Kill -9 a `brain run` mid-wave; `brain resume` continues, no mission
  dispatched twice (ledger proves it); `brain cancel` frees the lease and
  blocks further steps; racing resumes yield one winner; conflicting
  checkpoint/ledger fixture refuses with named conflict. Full suite +
  stress green.
