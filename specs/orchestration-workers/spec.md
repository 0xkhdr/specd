# Orchestration Worker Rigor Spec

## Purpose
Make brain orchestration claims match implemented worker behavior and prevent fake or ambiguous worker completion.

## Source Gaps
- GAP-ANALYSIS.md domain 5: worker claims and Pinky dispatch mismatch.
- ACP ledger and worker reports can describe capabilities not actually executed.
- Brain docs imply disconnected workers can perform work while code may only simulate or record claims.

## Goals
- Define worker modes explicitly: local deterministic worker, external host dispatch, or unavailable.
- Ensure ACP ledger records requested, dispatched, accepted, rejected, and timed-out states.
- Prevent worker report acceptance unless linked to lease, task, verify evidence, and git HEAD.
- Align docs with actual orchestration support.

## Non-Goals
- Do not add new external host automation unless already supported by local commands.
- Do not weaken task locking or evidence gates for parallelism.
- Do not make brain autonomous by default.

## Required Knowledge
- Brain run loop: `internal/cmd/brain_run.go`.
- Worker acceptance: `internal/cmd/brain_worker.go`.
- Orchestration: `internal/orchestration/acp.go`, `lease.go`, `decide.go`.
- Docs: `docs/agent-integration.md`, `docs/command-reference.md`.

## Functional Contract
- Brain may only claim work was dispatched if dispatch adapter returned a concrete dispatch record.
- Accepted worker reports must match active lease and task id.
- Completed worker tasks must include valid evidence accepted by core completion logic.
- Pinky or host-specific worker mode is either implemented and tested or documented as unavailable/deferred.

## Acceptance Criteria
- Tests cover worker report rejection for wrong task, stale lease, missing evidence, and unpinned HEAD.
- ACP ledger state transitions are deterministic and replayable.
- Docs no longer overclaim Pinky worker execution.
- Brain status reports distinguish planned, dispatched, running, accepted, rejected, and blocked.

## Invariants
- Brain controller contains no LLM decision logic.
- Worker report cannot bypass locks, DAG frontier, or evidence.
- Lease expiry cannot complete task.

## Verification
- `go test ./internal/orchestration ./internal/cmd -run 'Test.*Brain|Test.*Worker|Test.*ACP|Test.*Lease' -count=1`
- `go test ./... -race -count=1`

