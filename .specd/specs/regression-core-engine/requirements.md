# Requirements — Regression: Core Engine (DAG, gates, state, runner, telemetry)

## Introduction
The core engine (`internal/core`) is the reliability spine of specd: the DAG/frontier
solver, phase/gate state machine, evidence-gated status flips, EARS validation, the
runner/sandbox, locking, and telemetry. This spec defines a complete regression contract
over that engine so quality and reliability are non-negotiable — every public behavior
has an enforced acceptance criterion and a mapped test. The user value: agents and humans
can trust that a green specd is a correct specd.

## Requirement 1 — DAG & frontier correctness
**User story:** As an agent orchestrating tasks, I want the DAG solver to expose exactly the
runnable frontier, so that I never start a task whose dependencies are unmet.

**Acceptance criteria:**
1. WHEN a task's dependencies are all satisfied THE SYSTEM SHALL include it in the frontier
2. IF the task graph contains a cycle THEN THE SYSTEM SHALL report the cycle and refuse to emit a frontier
3. THE SYSTEM SHALL compute wave assignment such that every dependency lives in an earlier-or-equal wave
4. WHEN a dependency is incomplete THE SYSTEM SHALL exclude dependents from the frontier

## Requirement 2 — Phase/gate state machine integrity
**User story:** As a spec author, I want phase transitions and gates enforced, so that work
cannot skip analysis, design, or approval.

**Acceptance criteria:**
1. WHEN a phase has an open approval gate THE SYSTEM SHALL block advancement until cleared
2. IF a status flip lacks required evidence THEN THE SYSTEM SHALL reject the flip
3. THE SYSTEM SHALL persist phase, gate, and revision monotonically in state.json
4. WHERE custom gates are configured THE SYSTEM SHALL run them in pipeline order

## Requirement 3 — Evidence-gated task flips
**User story:** As a reviewer, I want task completion to require verifiable evidence, so that
"done" means proven.

**Acceptance criteria:**
1. WHEN a task is flipped to done with evidence THE SYSTEM SHALL record the evidence and timestamp
2. IF a required verify command is absent THEN THE SYSTEM SHALL reject the flip unless `--unverified` is given
3. THE SYSTEM SHALL store telemetry annotations (tokens, cost) without computing them

## Requirement 4 — EARS validation
**User story:** As an author, I want every acceptance criterion validated against EARS, so that
requirements stay testable.

**Acceptance criteria:**
1. IF a criterion matches no EARS pattern THEN THE SYSTEM SHALL flag it
2. THE SYSTEM SHALL accept the five canonical EARS patterns (ubiquitous, event, state, optional, unwanted)

## Requirement 5 — Runner & sandbox isolation
**User story:** As a security-conscious user, I want verify commands run under the configured
sandbox, so that untrusted commands cannot escape.

**Acceptance criteria:**
1. WHERE a sandbox is configured THE SYSTEM SHALL execute verify commands inside it
2. IF a runner command fails THEN THE SYSTEM SHALL surface the exit code and stderr verbatim
3. WHILE sandbox is `none` THE SYSTEM SHALL still capture exit status and output

## Requirement 6 — Concurrency & locking
**User story:** As a multi-agent system, I want concurrent state writes serialized, so that no
update is lost.

**Acceptance criteria:**
1. WHEN two writers contend on state THE SYSTEM SHALL serialize via lock and fail one deterministically
2. IF a stale lock is detected THEN THE SYSTEM SHALL recover without corrupting state
3. THE SYSTEM SHALL keep state.json schema-valid after every committed write
