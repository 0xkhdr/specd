# Requirements — workflow-08-authoring-gates

Release H closes the class of dogfooding findings where the tasks→executing gate approved a plan
that could not be dispatched, verified, or completed. The tasks phase is the point where every
task-row fact is meant to be proven; today several facts are only enforced at dispatch or
completion, after approval, so a green `check` and a passed `approve` still strand execution.
Source: `WORKFLOW-FEEDBACK.md` open entries on task-kind vocabulary, `-run` test-file declaration,
zero-test evidence, verb/flag palette declaration, acceptance/scope mismatch, and worker dispatch
policy.

## R1 — Dispatchable task schema at approval

owner: project maintainers
priority: must
risk: high

- R1.1: When the tasks→executing gate runs, the system shall convert every task row through the same task contract the dispatcher uses, so a row whose kind is outside the canonical vocabulary is rejected at approval rather than at session ack.
- R1.2: When a task row declares an unknown kind, the system shall name the offending row, the rejected value, and the accepted vocabulary in the gate finding.
- R1.3: When the repository ships task rows authored against an older vocabulary, the system shall report each nonconforming row deterministically so the whole program can be corrected before execution.

## R2 — Verify lines coupled to declared artifacts

owner: project maintainers
priority: must
risk: high

- R2.1: When a task verify line contains a run selector naming a test, the system shall require at least one test file path in that row files cell and refuse the tasks gate otherwise.
- R2.2: When a run selector names a test not defined in any file the row declares, the system shall report the selector that matches no declared test as a tasks-phase finding.
- R2.3: When a recorded verify run reports no tests to run for every package it targeted, the system shall refuse to record it as passing evidence and name the selector that matched nothing.

## R3 — Evidence producers planned before execution

owner: project maintainers
priority: must
risk: medium

- R3.1: When a task evidence cell declares a non-test class that a plain verify cannot produce, the system shall warn at the tasks gate that the declared verify line cannot satisfy the declaration and name the producer required.
- R3.2: When a task declares a review or eval class check, the system shall report the exact import command at approval time rather than only at completion.

## R4 — Command-surface tasks declare the palette

owner: project maintainers
priority: must
risk: high

- R4.1: When a task declares a handler file that registers a CLI verb or flag, the system shall require that row to also declare the canonical command declaration file and refuse the tasks gate otherwise.
- R4.2: When a shipped handler recognizes a flag absent from the canonical palette, the system shall fail a deterministic lint so a functional but undocumented flag cannot ship.
- R4.3: When a verb-adding or flag-adding task is approved, the system shall require the generated-docs source in scope so the command reference and the palette stay in parity.

## R5 — Acceptance reachable within declared scope

owner: project maintainers
priority: should
risk: medium

- R5.1: When a task acceptance cites a requirement, the system shall report which repository files already reference that requirement id, so a criterion whose only mentions fall outside the row declared files is surfaced before execution.
- R5.2: When a task claims a production kind but its declared files cannot produce the behaviour its acceptance names, the system shall surface a distinct scope-versus-acceptance finding rather than a generic out-of-scope refusal mid-execution.

## R6 — Worker dispatch policy planned and approved

owner: project maintainers
priority: must
risk: high

- R6.1: When a plan is authored, the system shall accept a worker column on each task row whose value is a worker id, where distinct ids mean a fresh worker per task, a shared id means one worker carries that sub-chain, and a dash retains host-chooses behaviour.
- R6.2: When the tasks gate validates a plan, the system shall validate the worker column so the dispatch policy is pinned by the tasks-phase approval like every other task fact.
- R6.3: When drive and next project the next wave, the system shall report whether each task continues an active worker or dispatches a fresh one.
- R6.4: When a mission is dispatched to a worker id the approved plan did not name, the system shall refuse it as an out-of-scope class refusal rather than a warning.

## R7 — Production-posture evidence policy is authorable

owner: project maintainers
priority: must
risk: critical

- R7.1: When the boundary-evidence gate and the quality-declaration gate read the same evidence cell, the system shall parse that cell through one shared parser so no cell is simultaneously rejected by one gate and required by the other.
- R7.2: When an external boundary requires integration evidence, the system shall accept a task whose kind is integration-equivalent or whose check id contains integration, and name the exact accepted forms in the refusal.
- R7.3: When a boundary-evidence gate refuses, the system shall name the boundary it inspected and the artifact that would satisfy it rather than refusing without a nameable remedy.

## Edge and failure behavior

- A plan mixing conforming and nonconforming rows reports every nonconforming row, not just the first.
- A run selector under a multi-package command is valid when any package executes matching tests.
- Worker-column validation never mutates dispatch behaviour for plans that use a dash.

## Non-goals

- Adding an evidence bypass or weakening the passing-verify completion rule.
- Inferring worker policy at execute time when the plan omits the column.
- Redefining the canonical task-kind vocabulary; this spec enforces it earlier, it does not extend it.
