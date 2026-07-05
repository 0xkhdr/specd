# 0006 Brain Safety Scope — Cancel, Resume, and Deferred Controls

Status: accepted

Context:
Spec 07 hardens the opt-in orchestration controller against crashes. v1's
controller carried a broader command surface — `cancel`, `resume`, `pause`,
`directive` (operator-injected mid-run instructions), and a live progress feed.
FINDINGS verdict (B.19, C.6, D.9): ship the crash-safety core (`cancel`, crash-safe
`resume`, per-step checkpoint) and defer the interactive-control verbs, which
duplicate what an interactive agent host already provides.

Decision:
- **Write-ahead checkpoint is the recovery primitive.** Each dispatch fsyncs a
  checkpoint (`SaveCheckpoint`) that names a deterministic mission id
  (`session/step/task`) *before* the dispatch becomes visible in the ACP ledger.
  A crash between the two leaves the mission in the checkpoint but absent from the
  ledger; `resume` re-issues exactly that mission. A crash after the ledger append
  leaves the mission present; `resume` does not re-issue. Recovery converges with
  zero double-dispatch (spec 07 R1).
- **`resume` is a pure reconciliation, then a claim.** `PlanResume` compares the
  checkpoint against the ledger and yields exactly one of: re-issue, noop, or an
  irreconcilable conflict (checkpoint and ledger name different tasks for the same
  mission id) which refuses with exit 1 rather than guessing (R3/R4). The resume is
  then *claimed* by a session-revision CAS under the spec lock, so two racing
  resumes conflict and exactly one proceeds.
- **Controller exclusivity is enforced by session-revision CAS, not a wall-clock
  controller heartbeat.** `brain start` writes with expected revision 0, so a second
  start on an existing session fails closed. `resume` bumps the revision, so racing
  resumes yield exactly one holder. A live *task* lease on a still-running session
  blocks `resume` (a controller is mid-flight); an expired or crash-orphaned lease
  is recoverable. We deliberately do **not** add a controller heartbeat/PID liveness
  probe to distinguish a live controller from a recent crash within the task-lease
  TTL — interactive hosts run one controller per spec, and the CAS already
  guarantees a single holder. Revisit on demonstrated multi-controller demand.
- **`crashed` is derived, never persisted.** The persisted session state machine is
  `running → {cancelled | complete}`. `brain status` reports `crashed` when the
  checkpoint outran the ledger (`DeriveStatus`); it is a view over on-disk state,
  not a stored transition (R2/R6).
- **`cancel` touches only the session.** It drives the session to the terminal
  `cancelled` state and releases the lease. Task and evidence state are never
  rewritten — a cancel is not a rollback. A second cancel is idempotent; cancelling
  a `complete` session is refused (R2).

Not brought back (recorded non-goals): live progress SSE/feed — a UI surface with
zero enforcement value (already a program-level non-goal).

Deferred, revisit on demand via a new spec:
- **`pause`/`unpause`** — a soft, resumable stop distinct from terminal `cancel`.
  Interactive hosts already stop and re-invoke the controller between steps, so a
  persisted pause state earns its keep only under batch/unattended demand.
- **`directive`** — operator-injected mid-run instructions. The controller is a
  pure function of on-disk `.specd/` state by design; a directive channel would put
  an out-of-band mutation into the decision path. If needed, it belongs as an
  explicit state input with its own gate, not a controller verb.
