# Domain: Evidence & Verification

## 1. Purpose & value mapping
- **Principles served:** P3 (Evidence Gates Every State Change) — this domain *is* P3.
- **Paper concept realized:** *sandboxes / execution environments* + the think→act→observe
  feedback loop (pp.29–30): "the harness provides the execution environment … captures the
  error output … routes it back."
- **Core use case:** a task cannot be marked complete unless a `verify:` command actually
  ran and passed, with the result recorded as evidence. This is the mechanism that makes
  the paper's 80% safely delegable — the harness, not the model's self-report, decides done.
- **If none → CUT:** N/A — without this, every other gate is theater.

## 2. Current-state analysis (from specd)
- **Reference files read:** `internal/cmd/verify.go`, `internal/cmd/task.go`,
  `internal/core/task_complete.go`, `internal/core/customgate.go` (`ScrubbedEnv`),
  `internal/core/state.go` (`VerificationRecord`), `docs/validation-gates.md`
  (security model), `docs/agent-integration.md` (evidence integrity in orchestration).
- **What exists today; key contracts/invariants:**
  - `verify.go` (399) runs each task's `verify:` line via `sh -c` (override
    `SPECD_VERIFY_SHELL`) as the invoking user. **Security hardening**: the child env is
    scrubbed to an allowlist (`PATH,HOME,LANG,LC_ALL,TMPDIR,SPECD_*`) via `ScrubbedEnv`;
    NUL-byte commands rejected; exact command + cwd printed before execution.
  - The **evidence gate** (`task.go` + `task_complete.go`): `--status complete` requires a
    passing verify record (or `--unverified --evidence` for read-only roles) AND all deps
    complete; `CompleteTask` dual-writes `tasks.md` + `state.json` atomically under lock.
  - `VerificationRecord` captures pass/fail, changed files (for the scope gate), timing,
    and host-reported telemetry (stored verbatim, never trusted as proof).
  - Sandbox (bwrap/container) and `--revert-on-fail` exist in the verify path;
    `tasks.md` is treated as hostile input ("only run `specd verify` on a `tasks.md` you
    trust").
- **Redundancy / complexity / drift found (evidence):**
  - `verify.go` at 399 LOC bundles: shell exec + env scrub + NUL check + changed-file
    capture + sandbox selection + revert logic + record writing + criterion recording.
    Several concerns that could be layered.
  - The record carries flywheel-adjacent fields (criterion records for the acceptance
    gate, host cost/tokens for orchestration) that bloat the core evidence shape.

## 3. Fresh-start decision
- **Verdict per capability:**
  - Evidence gate (no complete without passing record) — **KEEP, absolute** (guardrail
    from the brief; `docs/validation-gates.md` Gate 5). Never behind a flag.
  - Shell exec with scrubbed env + NUL rejection + printed command — **KEEP** (security
    model is sound and cheap).
  - `VerificationRecord` — **SIMPLIFY**: core record = `{task, status, exitCode, command,
    cwd, changedFiles, startedAt, durationMs, evidenceRef}`. Criterion records and host
    cost/tokens move to `state.records` (domain 02) as plugin/orchestration extensions.
  - Sandbox (bwrap/container) — **SIMPLIFY to fail-closed opt-in**: `config.verify.sandbox`
    = `off|bwrap|container`, default `off`; when set and the sandbox binary is missing,
    verify **fails closed** (does not silently run unsandboxed).
  - `--revert-on-fail` — **KEEP** but isolate: revert is a git-worktree operation behind a
    single flag; default off.
- **Minimal accurate surface:**
  - Commands: `verify <slug> <task> [--sandbox …] [--revert-on-fail]`,
    `task <slug> <id> --status complete`.
  - Modules: `verify` runner (exec + capture), `evidence` (record read/write),
    `task_complete` (integrity path).
- **Architecture & flexibility improvements:**
  - **Layer the runner:** `exec` (scrubbed shell) → `capture` (changed files, output) →
    `record` (write evidence) → `gate` (completion check). Each independently testable;
    `verify.go` becomes thin wiring.
  - **Evidence as an append-only ledger** (`.specd/specs/<slug>/evidence/…`) so a record is
    never overwritten; completion references a specific record hash, not "the latest".
  - **One completion path.** Both `task --status complete` and the orchestration worker
    report (domain 09) go through `CompleteTask`; a report is accepted only if it matches a
    passing local verify record — the integrity guarantee is enforced in one place.

## 4. Requirements (EARS-shaped) — seed for requirements.md
1. When a non-read-only task is marked complete, the system shall require a verify record
   whose status is `pass` and shall refuse otherwise.
2. When a `verify:` command runs, the system shall execute it with an allowlisted scrubbed
   environment and shall reject any command containing a NUL byte.
3. Before executing a `verify:` command, the system shall print the exact command and
   working directory.
4. When `config.verify.sandbox` names a sandbox whose binary is unavailable, the system
   shall fail closed and not execute the command unsandboxed.
5. When a verify run completes, the system shall append an immutable evidence record; the
   system shall never overwrite a prior record.
6. When a completion is claimed (by CLI or by an orchestration worker report), the system
   shall accept it only if it references a passing evidence record for that task.
7. When `--revert-on-fail` is set and verify fails, the system shall restore the working
   tree to its pre-verify state.

## 5. Design notes — seed for design.md
- **Module boundaries:** `internal/core/verify/{exec.go,capture.go}`,
  `internal/core/evidence/{record.go,ledger.go}`, `internal/core/task_complete.go`;
  `internal/cmd/verify.go` + `task.go` are thin.
- **Key types:** `VerificationRecord{Task,Status,ExitCode,Command,Cwd,ChangedFiles[],
  StartedAt,DurationMs,Hash}`; `SandboxMode` enum; `EvidenceLedger`.
- **Data/on-disk contracts:** append-only evidence ledger under the spec dir; record hash
  referenced from `state.tasks[i].evidenceRef`.
- **Invariants to preserve:** evidence gate absolute; scrubbed env; NUL rejection;
  atomic dual-write of `tasks.md`+`state.json` under lock; host telemetry stored verbatim,
  never trusted.
- **External interfaces:** `SPECD_VERIFY_SHELL`; the evidence record as the contract
  consumed by domain 03 (Gate 5 evidence, Gate 9 scope) and domain 09 (worker report).

## 6. Proposed task DAG — seed for tasks.md

### Wave 1 — runner & evidence
| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T5.1 | craftsman | `internal/core/verify/exec.go`, `internal/core/customgate.go` | — | `go test ./internal/core/verify -run TestScrubbedEnv` | env allowlisted; NUL rejected; command printed |
| T5.2 | craftsman | `internal/core/evidence/ledger.go` | — | `go test ./internal/core/evidence -run TestAppendOnly` | records immutable; hash-referenced |
| T5.3 | craftsman | `internal/core/verify/capture.go` | T5.1 | `go test ./internal/core/verify -run TestChangedFiles` | changed files captured for scope gate |
### Wave 2 — completion integrity
| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T5.4 | craftsman | `internal/core/task_complete.go` | T5.2 | `go test ./internal/core -run TestCompleteRequiresEvidence` | no complete without passing record |
| T5.5 | craftsman | `internal/cmd/verify.go` | T5.1,T5.3 | `go run . verify demo T1` | thin wiring; sandbox fail-closed |
| T5.6 | craftsman | `internal/cmd/verify.go` | T5.5 | `go test ./internal/cmd -run TestRevertOnFail` | working tree restored on failure |
| T5.7 | validator | `internal/core/verify/sandbox_test.go` | T5.5 | `go test ./internal/core/verify -run TestSandboxFailClosed` | missing sandbox binary → fail closed |

## 7. Risks, open questions, cross-domain dependencies
- **Risk:** `sh -c` execution of agent-authored `verify:` lines is real code execution.
  Mitigation (retained): trust boundary is explicit and documented; env scrubbed; sandbox
  opt-in fail-closed; command printed for audit.
- **Open question:** should the evidence ledger be committed to git or gitignored?
  Proposed: committed (evidence is auditable history), but this interacts with domain 11.
- **Cross-domain deps:** domain 03 (evidence + scope gates read records), domain 09 (worker
  reports validated against records), domain 02 (`CompleteTask` dual-write + CAS), domain
  10 (lock/io primitives).
