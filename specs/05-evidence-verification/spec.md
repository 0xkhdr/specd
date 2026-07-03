# Spec 05 — Evidence & Verification

> **Authoring order:** 5 / 12 · **Critical path:** yes
> **Sources:** `fresh-start/05-evidence-verification.md`, paper pp.29–30
> **ADRs:** ADR-6, ADR-8
> **Reference:** `reference/internal/cmd/{verify,task}.go`, `reference/internal/core/{task_complete,customgate,state}.go`, `reference/docs/validation-gates.md`

This domain *is* P3. A task cannot be marked complete unless a `verify:` command actually ran
and passed, with the result recorded as immutable evidence. The harness — not the model's
self-report — decides "done".

---

## 1. Purpose & principles
- **Principles owned:** P3 (Evidence Gates Every State Change).
- **Paper concept:** *sandboxes / execution environments* + the think→act→observe loop
  (pp.29–30): "the harness provides the execution environment … captures the error output …
  routes it back."

## 2. Verdicts (with citations)

| Capability | Verdict | Why / reference |
|---|---|---|
| Evidence gate (no complete without passing record) | **KEEP, absolute** (never behind a flag) | Guardrail; `reference/docs/validation-gates.md` Gate 5; ADR-8 |
| Shell exec + scrubbed env + NUL rejection + printed command | **KEEP** | Security model sound + cheap. `reference/internal/core/customgate.go` (`ScrubbedEnv`) |
| `VerificationRecord` (fat) | **SIMPLIFY** to core fields; criterion/host-cost → `state.records` | ADR-6 |
| Sandbox (bwrap/container) | **SIMPLIFY** to fail-closed opt-in (`config.verify.sandbox`, default `off`) | Fail closed if binary missing |
| `--revert-on-fail` | **KEEP** but isolate behind single flag, default off | git-worktree op |

**Core record:** `{task, status, exitCode, command, cwd, changedFiles, startedAt, durationMs,
evidenceRef}`. **Minimal surface:** `verify <slug> <task> [--sandbox …] [--revert-on-fail]`,
`task <slug> <id> --status complete`; modules `verify/` (exec+capture), `evidence/`
(record+ledger), `task_complete.go` (integrity).

## 3. Requirements (EARS)
- **R5.1** When a non-read-only task is marked complete, the system shall require a verify
  record whose status is `pass` and shall refuse otherwise.
- **R5.2** When a `verify:` command runs, the system shall execute it with an allowlisted
  scrubbed environment (`PATH,HOME,LANG,LC_ALL,TMPDIR,SPECD_*`) and shall reject any command
  containing a NUL byte.
- **R5.3** Before executing a `verify:` command, the system shall print the exact command and
  working directory.
- **R5.4** When `config.verify.sandbox` names a sandbox whose binary is unavailable, the
  system shall fail closed and not execute the command unsandboxed.
- **R5.5** When a verify run completes, the system shall append an immutable evidence record;
  the system shall never overwrite a prior record.
- **R5.6** When a completion is claimed (by CLI or by an orchestration worker report), the
  system shall accept it only if it references a passing evidence record for that task.
- **R5.7** When `--revert-on-fail` is set and verify fails, the system shall restore the
  working tree to its pre-verify state.

## 4. Design

### Module boundaries (layered runner)
- `internal/core/verify/exec.go` — scrubbed shell exec. `verify/capture.go` — changed files +
  output. `internal/core/evidence/{record.go,ledger.go}` — append-only ledger.
- `internal/core/task_complete.go` — the **one completion path** (both `task --status
  complete` and the Spec 09 worker report go through `CompleteTask`).
- `internal/cmd/{verify,task}.go` — thin wiring.

### Key types
- `VerificationRecord{Task, Status, ExitCode, Command, Cwd, ChangedFiles[], StartedAt,
  DurationMs, Hash}`; `SandboxMode` enum (`off|bwrap|container`); `EvidenceLedger`.

### On-disk contracts
- Append-only evidence ledger under `.specd/specs/<slug>/evidence/…`; completion references a
  specific record **hash**, not "the latest" (`state.tasks[i].evidenceRef`).
- Host telemetry (cost/tokens) stored **verbatim, never trusted** as proof (→ `state.records`).

### External interfaces
- `SPECD_VERIFY_SHELL` (default `sh -c`); the evidence record is the contract consumed by
  Spec 03 (Gate 5 evidence, Gate 9 scope) and Spec 09 (worker report).

## 5. Invariants preserved (ADR-8)
Evidence gate absolute; scrubbed env; NUL rejection; atomic dual-write of
`tasks.md`+`state.json` under lock; host telemetry verbatim.

## 6. Cross-domain dependencies
- Feeds: Spec 03 (evidence + scope gates), Spec 09 (worker reports validated against records).
- Depends on: Spec 02 (`CompleteTask` dual-write + CAS), Spec 10 (lock/io).

## 7. Risks & open questions
- **Risk:** `sh -c` of agent-authored `verify:` lines is real code execution. → trust boundary
  explicit + documented; env scrubbed; sandbox opt-in fail-closed; command printed for audit.
- **Decision:** commit the evidence ledger to git. Evidence is auditable harness state under
  the SDLC paper's model; ignore only transient or oversized logs outside the ledger.
