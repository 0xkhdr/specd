# Evidence and Security Gate Spec

## Purpose
Strengthen evidence integrity, sandbox signaling, and security gate consistency without weakening specd's no-bypass guarantee.

## Source Gaps
- GAP-ANALYSIS.md domain 4: evidence pinning and security/sandbox inconsistencies.
- Evidence gate depends on `HeadPinned` semantics and worker reports.
- Output truncation limits differ across code paths.
- Sandbox and clean-worktree policy need clear opt-in contract.

## Goals
- Ensure every completed task has passing verify record pinned to a resolvable git HEAD.
- Make worker-submitted evidence pass same validation as local verify.
- Normalize evidence output truncation and reporting.
- Document and test clean-worktree and sandbox policy.

## Non-Goals
- Do not add a bypass flag.
- Do not require clean worktree by default unless existing config opts in.
- Do not execute untrusted commands outside existing verify executor boundaries.

## Required Knowledge
- Evidence: `internal/core/evidence.go`, `internal/core/task_complete.go`.
- Gates: `internal/core/gates/`, `internal/core/gates/security/`.
- Verify executor: `internal/core/verify/exec.go`.
- Brain worker reports: `internal/cmd/brain_worker.go`, `internal/orchestration/`.
- Config: `internal/core/config_loader.go`.

## Functional Contract
- Completion requires verify exit code 0 and pinned git HEAD.
- Worker evidence cannot mark task complete unless it includes same required fields.
- Evidence output truncation uses one constant and reports truncation clearly.
- Clean-worktree policy is opt-in, config-driven, and tested.
- Sandbox config is surfaced in diagnostics and verify context.

## Acceptance Criteria
- Tests fail if task completion accepts unpinned evidence.
- Tests fail if worker report can complete task with missing or fake HEAD.
- Evidence truncation behavior same for local verify, worker verify, and reports.
- Security gate docs and diagnostics name active policy.

## Invariants
- No evidence bypass path.
- No LLM in evidence validation.
- No network dependency in evidence tests.
- Git HEAD must be resolvable in repository where verify ran.

## Verification
- `go test ./internal/core ./internal/core/gates ./internal/core/verify ./internal/cmd -count=1`
- `go test ./... -race -count=1`

## Decisions

- **Dirty-worktree handling (GAP 4.3, was W6-T5).** GAP-ANALYSIS prescribed a `git_dirty`
  field on the evidence record plus a `verify.require_clean` flag surfaced in `report`. The
  accepted design is instead a config-driven **clean-worktree gate** (`security.clean_worktree`
  = off|warn|error, default off). The gate fails closed when the worktree is dirty under an
  `error` policy, which is a stronger and more uniform contract than a per-record annotation:
  cleanliness is enforced at the gate, not merely recorded after the fact. GAP 4.3 is closed by
  this gate; `git_dirty`/`require_clean` were deliberately not added, to avoid two half-built
  mechanisms for one concern.
- **Verify timeout (GAP 4.2, was W6-T4).** Closed: `verify.timeout_seconds`
  (`SPECD_VERIFY_TIMEOUT_SECONDS`, default `0` = unbounded) bounds a single verify command. A
  timeout is recorded as a **failing** evidence record (exit 124) through the normal evidence
  path — never a crash or a silent hang.

