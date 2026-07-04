# Wave 1 — Close the Loop

> **Order:** 2 / 7 · **Depends:** W0 · **Unblocks:** W2, W3, W4
> **Findings:** F2 (loop cannot close), F3 (approvals gate nothing), F5 (ADR-7 unimplemented), F14 (`--unverified` unimplemented)
> **Sources:** PROJECT.md §8 Wave P1, BUILD_REVIEW.md §5 Wave 1, specs/02 + 05, ADR-1/7/8
> **Files (no new packages):** `internal/cmd/{lifecycle,registry}.go`, `internal/core/{state,task_complete}.go`

The product does not exist without this wave. The paper's think→act→observe loop needs
observe→record→advance closure: a completion verb that demands evidence, dispatch that
respects phase approvals, and the conductor/orchestrator mode enum that everything
downstream keys off.

## 1. Purpose & principles

- **Principles owned:** P3 (evidence gates every state change), P6 (human gates at phase boundaries).
- **Harness components:** guardrails (completion + phase gates), orchestration (mode enum).

## 2. Requirements (EARS)

- **R1.1** When `specd task complete <slug> <id>` runs for a non-read-only task
  (role craftsman), the system shall refuse (exit non-zero, reason printed) unless the
  evidence ledger holds a passing verify record for that task at the current git HEAD;
  on success it shall set the task's status in `state.json` under lock + CAS,
  referencing the specific record hash — never "the latest" (ADR-1: state, not
  `tasks.md` markers, is machine truth; `taskStatus` shall read state, not markers).
- **R1.2** When `next` or `verify` runs against a spec whose requirements or design are
  not yet approved in `state.json`, the system shall refuse task dispatch/execution:
  `next` returns an empty frontier with an explanation naming the missing approval;
  after both approvals it returns the runnable frontier. (`loadSpec` shall read
  `state.json`; the phase ratchet becomes consulted, not decorative.)
- **R1.3** The system shall implement ADR-7 end-to-end: `Mode ∈ {simple, orchestrated}`
  (the `default` placeholder removed); `new --mode` sets it (default `simple`);
  the only mode change is the auditable `approve <slug> mode --to orchestrated`;
  `status --json` exposes `mode`, `phase`, and `status`; `brain` eligibility requires
  `mode: orchestrated`.
- **R1.4** When `task complete --unverified --evidence <text>` runs for a read-only
  role task (scout, auditor, validator), the system shall accept it and append a
  distinct evidence kind (`unverified-attestation`) to the ledger; when run for a
  craftsman task it shall refuse. `--unverified` without `--evidence` shall be a usage error.

## 3. Design

- **One completion path (spec 05 contract):** `CompleteTask` in core is the single
  gate; the CLI verb and (later) orchestration worker reports both route through it.
  Dual-write of `tasks.md` checkbox + `state.json` status is atomic under `WithSpecLock`.
- **Phase gating (R1.2):** a `requireApproved(state, "requirements", "design")` check in
  `loadSpec`-adjacent command entry, not inside gate bodies (gates stay pure). Refusal
  uses the Gate exit code with a one-line human reason.
- **Mode enum (R1.3):** enum + validation in `internal/core/state.go`; loud-load rejects
  unknown modes (fail-loud posture). Mode transition writes an approval-shaped record
  (actor/timestamp arrive in W3; write the fields now, populate fully there).
- **Escape hatch (R1.4):** role read from the parsed task's `role:` key — `tasks.md` is
  hostile input, so the role check happens against the parsed, validated task, and the
  attestation text is stored verbatim as untrusted evidence.

## 4. Invariants preserved

- ADR-8: CAS on revision inside the lock; atomic writes; parser byte round-trip
  untouched (checkbox flip = single-line rewrite).
- Evidence integrity: no completion without a passing record; escape hatch explicit and
  role-restricted only.
- Determinism: no new IO in gate bodies; refusal messages are pure functions of state.
