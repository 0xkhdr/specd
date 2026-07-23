# Requirements — workflow-10-observability

Release J makes the read surfaces tell the truth and name the next action, and makes machine exit
codes match rendered severity. Every finding here is a case where the information existed but the
agent had to read state.json, guess a verb, or run a write verb to obtain a read. Source:
`WORKFLOW-FEEDBACK.md` open entries on mode readability, criterion coverage, help enumeration,
context-budget diagnostics, check exit semantics, and the review read path.

## R1 — Mode is readable without mutation

owner: project maintainers
priority: must
risk: medium

- R1.1: When status is rendered as JSON, the system shall include the spec current mode.
- R1.2: When status guide renders its header, the system shall report the current mode.
- R1.3: When mode is invoked for a spec with no value, the system shall print the current mode instead of a usage error.

## R2 — Criterion coverage names its own next action

owner: project maintainers
priority: must
risk: medium

- R2.1: When a spec has completed tasks whose acceptance references criteria with no criterion evidence, the system shall list the criterion-verification command in status guide blockers.
- R2.2: When verify is invoked with insufficient arguments, the system shall render the palette usage string that documents both the task form and the criterion form, matching help verify.

## R3 — Help enumerates and routes correctly

owner: project maintainers
priority: must
risk: low

- R3.1: When help renders a flag whose value set is a fixed enumeration, the system shall render the allowed values rather than only the default.
- R3.2: When a verb is invoked with a help flag, the system shall route to command help and exit zero rather than treating the flag as a spec slug.

## R4 — Refusals name contributors and routes

owner: project maintainers
priority: must
risk: medium

- R4.1: When a context-budget refusal is emitted, the system shall include per-source token contributions and an authorized next action for narrowing the task.
- R4.2: When a pre-execution gate blocks a spec, the system shall emit one ordered recovery sequence naming the artifact or role authorized to fix the blocker and the subsequent human-only commands.

## R5 — Machine exit codes match rendered severity

owner: project maintainers
priority: must
risk: high

- R5.1: When check renders findings and any finding is error severity, the system shall exit non-zero in both text and JSON output.
- R5.2: When a controller run halts having dispatched nothing, the system shall exit with a distinct code so an unattended run can distinguish no ready work from a controller that cannot start.

## R6 — The review report has a read path and a safe write

owner: project maintainers
priority: must
risk: critical

- R6.1: When an operator inspects a review, the system shall expose the parsed verdict, reviewer, and HEAD field of the review report in status JSON so no one runs a write verb to read it.
- R6.2: When a review report already exists, the system shall refuse to re-scaffold it regardless of its HEAD field unless a force flag is passed, and name the existing path in the refusal.
- R6.3: When a verdict field carries a qualifier, the system shall parse the first token as the verdict and retain the remainder as a free-text note.
- R6.4: When any verb receives an unrecognized flag, the system shall fail closed with exit two, matching the unknown-verb rule.

## Edge and failure behavior

- A spec whose criteria are fully covered emits no criterion blocker.
- A force re-scaffold of an existing review report still preserves the prior content via backup or explicit overwrite acknowledgement.
- The distinct halt exit code does not change behaviour for runs that dispatch work.

## Non-goals

- Adding an LLM to any guidance, report, or exit-code path.
- Widening any mutation authority; every new surface here is read-only or a stricter refusal.
- Redesigning the review report schema beyond a parseable verdict token and an optional note.
