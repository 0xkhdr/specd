# Tasks — Multi-Spec Program Orchestration

## Wave 9 — Program Decisions

- [ ] T1 — Define pure program snapshots and decisions
  - why: Program scheduling should reuse the existing graph while remaining deterministic.
  - role: builder
  - files: internal/core/program_orchestration.go, internal/core/program_orchestration_test.go
  - contract: Build parent/child session snapshots from ProgramGraph and select stable start/wait/escalate/complete decisions.
  - acceptance: Frontier order is wave then slug; cycles, orphans, blocked children, and capacity have exact decisions.
  - verify: go test ./internal/core/... -run 'TestProgramOrchestrationDecide' -count=20
  - depends: brain-core/T3
  - requirements: R6.1, R6.2, R6.3, R6.6

## Wave 11 — Program Child Scheduling

- [x] T2 — Implement child lease and bounded step scheduling ✓ complete · evidence: go test ./internal/core/... -run 'TestProgramOrchestration.*(Lease|Capacity|Frontier)' -race -count=2; go test ./internal/core/... -run 'TestProgramOrchestration|TestOrchestration' -race -count=2; make ci · 2026-06-19
  - why: Parent orchestration must prevent duplicate Spec Brains and goroutine sprawl.
  - role: builder
  - files: internal/core/program_orchestration.go, internal/core/program_orchestration_test.go
  - contract: Acquire child leases, advance children through bounded Brain steps, enforce capacity, and recompute after terminal events/revisions.
  - acceptance: Concurrent parents cannot own the same child; capacity is never exceeded; completion immediately exposes the next frontier.
  - verify: go test ./internal/core/... -run 'TestProgramOrchestration.*(Lease|Capacity|Frontier)' -race -count=2
  - depends: T1, brain-core/T5
  - requirements: R6.3, R6.4, R6.5

## Wave 12 — Program Failure Controls

- [x] T3 — Implement fail-fast escalation and parent controls ✓ complete · evidence: go test ./internal/core/... ./internal/cmd/... -run 'Test.*Embed.*Brain|Test.*Embed.*Pinky|TestInit|TestProgramOrchestration.*(Escalate|Pause|Cancel|Recovery|Complete)' -race -count=2; go test ./internal/core/... -run 'TestProgramOrchestration|TestOrchestration' -race -count=2; make ci · 2026-06-19
  - why: Program-level failure semantics must be explicit and recoverable.
  - role: builder
  - files: internal/core/program_orchestration.go, internal/core/program_orchestration_test.go
  - contract: Pause new work on child escalation, propagate cooperative pause/cancel, preserve child replay links, and derive parent completion only from state.
  - acceptance: Blocked/escalated children cannot unlock dependents; parent restart restores identical control state.
  - verify: go test ./internal/core/... -run 'TestProgramOrchestration.*(Escalate|Pause|Cancel|Recovery|Complete)' -race -count=2
  - depends: T2
  - requirements: R6.6, R6.7, R6.9, R6.11, R6.12

## Wave 13 — Program CLI and Reporting

- [x] T4 — Extend Brain CLI with program sessions ✓ complete · evidence: go test ./internal/integration/... ./internal/core/... ./internal/cmd/... -race -count=2 · 2026-06-19
  - why: Program orchestration should share the canonical Brain interface.
  - role: builder
  - files: internal/cmd/brain.go, internal/cmd/program.go, internal/core/commands.go, internal/cmd/program_test.go
  - contract: Support start --program and program-aware status/pause/resume/cancel with wave, counts, frontier, critical path, children, and escalation.
  - acceptance: Existing `specd program` output remains compatible; orchestration output is deterministic and approval policy is never widened.
  - verify: go test ./internal/cmd/... ./internal/core/... -run 'TestProgram|TestBrain.*Program' -count=2
  - depends: T3
  - requirements: R6.8, R6.10, R6.12

## Wave 14 — Program Stress and Recovery

- [x] T5 — Add multi-process program stress and recovery tests ✓ complete · evidence: make build; make ci · 2026-06-19
  - why: Cross-spec scheduling combines the highest concurrency risks.
  - role: verifier
  - files: internal/integration/program_orchestration_test.go, scripts/stress-program.sh, Makefile
  - contract: Exercise parallel waves, parent contention, child conflict, crash/restart, fail-fast escalation, and all-complete termination.
  - acceptance: No duplicate child ownership, premature dependent start, lost event, race, or nondeterministic ordering.
  - verify: make ci
  - depends: T4, pinky-core/T7
  - requirements: R6.2, R6.3, R6.4, R6.5, R6.6, R6.10, R6.11
