# Requirements — workflow-05-executable-orchestration

Release E closes authoring, context, evidence, mission, concurrency, and review contract gaps. Source scope:
[implementation tasks T22–T26](../../../specd-workflow-improvements/implementation-tasks.md),
[specification authoring](../../../specd-workflow-improvements/specification-authoring.md),
[task generation and execution](../../../specd-workflow-improvements/task-generation-and-execution.md),
[context management and enforcement](../../../specd-workflow-improvements/context-management-and-enforcement.md),
[debugging and failure recovery](../../../specd-workflow-improvements/debugging-and-failure-recovery.md),
[user experience and steering](../../../specd-workflow-improvements/user-experience-and-steering.md), and
[testing and observability](../../../specd-workflow-improvements/testing-and-observability.md).

## R1 — Shared task and scaffold contracts

owner: project maintainers
priority: must
risk: high

- R1.1: When tasks are parsed, the system shall parse every typed field once through a canonical parser consumed by planning gates, routing, evidence policy, context, review, and documentation.
- R1.2: When a generated scaffold is filled with its canonical examples, the system shall pass every armed consumer under its declared profile.
- R1.3: When legacy field delimiters or values are unambiguous, the system shall normalize them with a stable deprecation warning; otherwise it shall refuse against the authored field.
- R1.4: When task capability is valid for its role and kind, the system shall use the same capability identity in task schema, role policy, routing, mission authority, and docs.

## R2 — Typed context lanes

owner: project maintainers
priority: must
risk: high

- R2.1: When context is built, the system shall distinguish required input, optional existing output, prospective output, bounded directory query, and managed policy lanes.
- R2.2: When a declared output does not yet exist, the system shall retain write authority and omit content without failing context.
- R2.3: When a required input is missing or unreadable, the system shall fail with the exact column, path, and recovery.
- R2.4: When a bare directory is declared, the system shall require an explicit bounded selector instead of reading it as a file or expanding it without limits.
- R2.5: When context budget is evaluated, the system shall count only required and loaded lanes for the selected active or reopened task.
- R2.6: When host containment is unavailable, the system shall report advisory assurance in the authority packet.

## R3 — Strong evidence semantics

owner: project maintainers
priority: must
risk: critical

- R3.1: When a Go verification selector matches no tests, the system shall record invalid or failing evidence and shall not permit task completion.
- R3.2: When evidence is loaded, the system shall distinguish missing, failing, stale, malformed, incompatible, and passing states.
- R3.3: When a task has multiple attempts, the system shall bind verify records and completion to the current task id, attempt, plan revision, subject revision, and baseline.
- R3.4: When a read-only task declares a documented trivial verify, the system shall continue to accept it without weakening write-task verification.

## R4 — Mission lifecycle and safe concurrency

owner: project maintainers
priority: must
risk: critical

- R4.1: When a host does not declare isolated worktrees, the system shall serialize task missions so every dispatched task can satisfy diff scope in the shared worktree.
- R4.2: When a host declares and proves isolation, the system shall bind each concurrent mission to its isolation identity and require explicit integration and revalidation.
- R4.3: When an unclaimed or abandoned mission blocks a ready task, the system shall provide immediate per-mission release without waiting for lease TTL.
- R4.4: When baseline is selected, the system shall prefer the live claimed mission over expired or abandoned mission records.
- R4.5: When dispatch, claim, heartbeat, report, or recovery runs, the system shall preserve session and lease authority bindings and shall not discard caller flags.

## R5 — Non-destructive review workflow

owner: project maintainers
priority: must
risk: high

- R5.1: When a review report already exists, the system shall refuse scaffold overwrite unless an explicit destructive operator action is authorized.
- R5.2: When a review report is restamped to a new subject revision, the system shall preserve the human-authored body byte-for-byte and update only machine-owned provenance.
- R5.3: When review verdict is parsed, the system shall keep the strict verdict token separate from explanatory notes.
- R5.4: When review evidence is checked, the system shall treat the evidence subject revision as normative and prose provenance as a projection.

## R6 — Production orchestration proof

owner: project maintainers
priority: must
risk: critical

- R6.1: When the production orchestration journey runs, the system shall complete dispatch, claim, context receipt, verify, report, review, task completion, and lifecycle checks without profile changes, manual ledger edits, or TTL waits.
- R6.2: When production host capability is insufficient, the system shall refuse before mutable work with an actor-legal recovery instead of deadlocking later.
- R6.3: When zero progress is permanently blocked, the controller shall return a distinct non-success outcome and report durable checkpoint effects.

## R7 — Preserved invariants

owner: project maintainers
priority: must
risk: critical

- R7.1: While execution paths are repaired, the system shall preserve byte-stable task Markdown, current-HEAD evidence integrity, path containment checks, deterministic ordering, and zero runtime dependencies.
- R7.2: When CLI or manifest schemas change, the system shall version machine output and update generated docs and conformance tests in the same task.

## Edge and failure behavior

- Symlink escapes, directory selectors matching zero files, mixed existing and prospective outputs, and completed tasks receive precise lane outcomes.
- Duplicate checkpoints, open sessions, expired leases, abandoned baselines, and merge drift recover deterministically.
- Unknown test runners use declared producer status rather than unreliable generic output heuristics.

## Non-goals

- Parallel dispatch on a shared worktree without proven isolation.
- Placeholder files for prospective outputs or unbounded directory expansion.
- Generic test-count inference for every external test runner.
