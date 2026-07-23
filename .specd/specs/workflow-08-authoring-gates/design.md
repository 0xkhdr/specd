# Design — workflow-08-authoring-gates

> Replace prompts. Trace every decision to approved requirement IDs.

- references: R1, R2, R3, R4, R5, R6, R7
- disposition: accepted
- owner: project maintainers

## Boundaries

Owned code:

- The tasks-phase gate (`internal/core/gates/task_schema.go` and siblings) gains dispatch-parity
  validation: every row is parsed through the same task contract the dispatcher and session-ack
  path use, so kind, verify/run, evidence, palette, scope, and worker facts are proven at approval
  (R1, R2, R3, R4, R5, R6).
- One shared evidence-cell parser (`internal/core/evidence.go`) read by both the boundary-evidence
  gate and the quality-declaration gate, so a cell is never simultaneously required and rejected (R7.1).
- A worker column on task rows, parsed by the byte-stable tasks parser (`internal/core/tasksparser.go`)
  and validated by the tasks gate (R6.1, R6.2); `drive`/`next` projection reports worker continuation (R6.3).
- A structural lint that fails when a shipped handler recognizes a flag absent from the canonical
  palette (R4.2).

Excluded: extending or redefining the canonical task-kind vocabulary; adding any evidence bypass;
inferring worker policy at execute time; a second evidence store.

## Interfaces

- `check`/`approve` at the tasks→executing gate: new deterministic findings enumerate every
  nonconforming row (kind, run-selector-without-test-file, evidence-producer mismatch, palette
  omission, scope-versus-acceptance, unknown worker id) — all rows reported, not the first (R1.3, R5.1).
- Task row schema gains an optional `worker` column: distinct id = fresh worker per task, shared id =
  one worker carries the sub-chain, `-` = host chooses (R6.1).
- `drive`/`next` wave projection annotates each task with `worker=<id> (fresh|continues)` (R6.3).
- Verify recorder (`internal/core/verify/exec.go`) refuses to record a run that reported "no tests to
  run" for every targeted package, naming the empty selector (R2.3).

## Invariants

- Any task fact enforced at dispatch or completion is also enforced at the tasks gate — approval
  never green-lights a plan the dispatcher will later reject (R1.1, R6.4).
- Worker-column validation never mutates dispatch for `-` rows (edge/failure).
- Passing-verify completion rule and no-bypass invariant are preserved unchanged (non-goal).
- Boundary-evidence and quality-declaration gates read one parser; no cell fails both (R7.1).

## Failure

- Nonconforming kind / unknown worker id / undeclared test file / palette omission / scope mismatch:
  tasks gate refuses with a nameable row, the rejected value, and the accepted form (R1.2, R2.2,
  R4.1, R5.2, R6.4, R7.2, R7.3).
- Mid-execution dispatch to an unnamed worker id is an out-of-scope class refusal, not a warning (R6.4).
- Verify run matching no tests refuses to record as passing, naming the selector (R2.3).

## Integration

- Reuses the existing task contract, tasks parser, evidence parser, verify recorder, gate registry,
  and generated-docs pipeline; adds no dependency. R4.3 requires the gendocs source in scope for any
  verb/flag-adding task so palette and command reference stay in parity.

## Alternatives

- Enforcing task facts only at dispatch/completion (status quo): rejected — a green check strands
  execution after approval.
- Separate evidence parsers per gate: rejected — causes the double-bind R7.1 closes.
- Inferring worker policy at execute time: rejected (non-goal); the plan pins it or `-` retains
  host-chooses.

## Verification

- Table-driven tasks-gate tests: conforming plan passes; each nonconforming fact (kind, run/test-file,
  evidence producer, palette, scope, worker) produces its distinct finding; a mixed plan reports every
  offending row (R1, R2, R3, R4, R5, R6, edge cases).
- Shared-evidence-parser test proving the same cell satisfies both gates (R7.1).
- Verify-recorder test refusing an empty-selector run (R2.3).
- Worker projection test for fresh-versus-continues (R6.3); dash-row dispatch-unchanged test (edge).
- gofmt, vet, structural lint, docs lint, domain regressions.

## Deployment

- Ship behind the normal tasks gate; existing conforming plans keep approving. Nonconforming legacy
  rows surface deterministically so the whole program is corrected before execution (R1.3).

## Rollback

- Trigger: a false refusal blocks a valid plan. Restore the prior gate binary; the new gate adds
  refusals only, never mutates state, so rollback is a binary swap with no data migration.
