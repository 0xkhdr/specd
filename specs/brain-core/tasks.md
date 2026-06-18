# Tasks — Brain Deterministic Orchestrator

## Wave 6 — Brain Models

- [x] T1 — Define snapshots, decisions, sessions, and policy boundaries
  - why: The controller needs explicit pure inputs and outputs before side effects are added.
  - role: builder
  - files: internal/core/orchestration.go, internal/core/orchestration_test.go
  - contract: Define immutable snapshot, decision, session, escalation, and lifecycle models with deterministic JSON forms.
  - acceptance: Models cover every status/gate/action and serialize without nil-list or map-order drift.
  - verify: go test ./internal/core/... -run 'TestOrchestrationModel' -count=2
  - depends: config-extension/T3, acp-file-transport/T4
  - requirements: R3.1, R3.4, R3.10

## Wave 7 — Brain Sensing

- [x] T2 — Implement authoritative snapshot sensing
  - why: Brain decisions must use existing specd truth and helpers.
  - role: builder
  - files: internal/core/orchestration_sense.go, internal/core/orchestration_sense_test.go
  - contract: Build a snapshot from LoadSpec, phase/gate results, runnable frontier, verification records, and active ACP leases without shelling out.
  - acceptance: Fixtures for every lifecycle status produce exact stable snapshots and preserve gate/CAS errors.
  - verify: go test ./internal/core/... -run 'TestOrchestrationSense' -count=2
  - depends: T1, acp-file-transport/T5
  - requirements: R3.2, R3.3

## Wave 8 — Brain Decisions

- [x] T3 — Implement pure deterministic decision table
  - why: Scheduling behavior must be reviewable and reproducible.
  - role: builder
  - files: internal/core/orchestration_decide.go, internal/core/orchestration_decide_test.go
  - contract: Map snapshot plus policy to exactly one idle/approval/dispatch/wait/retry/cancel/replan/escalate/complete decision.
  - acceptance: Table tests cover all statuses, human-only gates, limits, active leases, failures, and unknown states; repeated calls are byte-identical.
  - verify: go test ./internal/core/... -run 'TestOrchestrationDecide' -count=20
  - depends: T2
  - requirements: R3.1, R3.4, R3.5, R3.6, R3.13, R3.15

## Wave 9 — Brain Reconciliation

- [ ] T4 — Implement one-step reconciliation engine
  - why: Bounded idempotent steps make recovery and MCP use safe.
  - role: builder
  - files: internal/core/orchestration_engine.go, internal/core/orchestration_engine_test.go
  - contract: Under locks/CAS, record a decision, apply at most one external action, record outcome, and reconcile evidence only through existing verify/task/approve core paths.
  - acceptance: Crash injection and duplicate event tests prove no repeated transition or evidence bypass.
  - verify: go test ./internal/core/... -run 'TestOrchestrationEngine' -race -count=2
  - depends: T3, acp-file-transport/T6
  - requirements: R3.5, R3.6, R3.7, R3.11, R3.12

## Wave 10 — Brain Recovery Controls

- [ ] T5 — Implement pause, resume, cancel, retry, and recovery
  - why: Autonomous sessions need explicit operational control.
  - role: builder
  - files: internal/core/orchestration_engine.go, internal/core/orchestration_recovery_test.go
  - contract: Persist lifecycle controls, stop new dispatch on pause, issue cooperative cancel directives, reclaim expired work, and rebuild state after restart.
  - acceptance: Restart at every event boundary converges to the same state; cancellation never claims host process termination.
  - verify: go test ./internal/core/... -run 'TestOrchestration.*(Pause|Resume|Cancel|Recovery|Retry)' -race -count=2
  - depends: T4
  - requirements: R3.6, R3.9, R3.12, R3.13

## Wave 11 — Brain CLI

- [ ] T6 — Add Brain CLI and registry metadata
  - why: CLI is the canonical orchestration interface and MCP source.
  - role: builder
  - files: internal/cmd/brain.go, internal/cmd/registry.go, internal/core/commands.go, internal/cmd/brain_test.go
  - contract: Add start/status/step/pause/resume/cancel with structured output, explicit policy/limits, foreground bounded behavior, and correct exit codes.
  - acceptance: Help/registry parity passes; one step emits at most one action; invalid scope, policy, or session IDs fail closed.
  - verify: go test ./internal/cmd/... ./internal/core/... -run 'TestBrain|TestRegistryMatchesHelp' -count=2
  - depends: T5
  - requirements: R3.8, R3.9, R3.10

## Wave 12 — Brain Guidance

- [ ] T7 — Add embedded Brain guidance
  - why: Agent hosts need a portable constitution aligned with actual controller behavior.
  - role: builder
  - files: internal/core/embed_templates/roles/brain.md, internal/core/embed_templates/skills/specd-brain/SKILL.md, internal/core/embed.go
  - contract: Document sensing, bounded stepping, approvals, dispatch, escalation, and the no-LLM-in-core boundary.
  - acceptance: Init embeds versioned guidance and tests assert its critical directives.
  - verify: go test ./internal/core/... ./internal/cmd/... -run 'Test.*Embed.*Brain|TestInit' -count=2
  - depends: T6
  - requirements: R3.14

## Wave 14 — Brain End-to-End Hardening

- [ ] T8 — Add deterministic fake-host lifecycle and stress coverage
  - why: The complete controller must be proven without a real model provider.
  - role: verifier
  - files: internal/integration/orchestration_test.go, internal/testharness/orchestration.go, scripts/stress-orchestration.sh, Makefile
  - contract: Exercise planning approval, dispatch, worker failure, retry, verification, completion, pause/restart, and concurrent session contention.
  - acceptance: No gate bypass, duplicate transition, race, nondeterministic output, hidden network call, or external dependency.
  - verify: make ci
  - depends: T7, pinky-core/T7
  - requirements: R3.1, R3.5, R3.7, R3.11, R3.12, R3.15
