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

## Decisions

- **Worker round-trip verbs (GAP 5.2, was W5-T2).** GAP-ANALYSIS prescribed `mission claim` /
  `mission report` verbs. Those verbs were **not** built. The worker round-trip instead runs
  through `internal/cmd/brain_worker.go` plus the `pinky-*` subagents, which claim a dispatched
  mission, do the work under the same locks/DAG/evidence path, and report evidence — no verb
  can bypass locks, frontier, or evidence integrity. This is a deliberate divergence: the
  capability exists under different names, so the named verbs were dropped rather than added as
  thin aliases.
- **`brain run` (GAP 5.3, was W5-T3).** Resolved: `brain run` now loops `brain step`
  until the controller brakes (dispatching every ready, unleased frontier task, then stopping),
  with each step persisting its own session CAS and checkpoint so a crash mid-run recovers as a
  sequence of steps would. It is no longer a silent alias of a single `step`.
- **Lease release on completion (GAP 5.4, was W5-T5).** A step now releases the lease of any
  task that has reached a terminal status, so no completed task lingers as a phantom live
  worker in `brain status`. `scripts/stress-orchestration.sh` asserts no stale live lease
  survives contention.

