# Tasks — Orchestration Configuration and Policy

## Wave 0 — Configuration Model

- [x] T1 — Add orchestration config models and defaults
  - why: Establish the policy contract before runtime components depend on it.
  - role: builder
  - files: internal/core/specfiles.go
  - contract: Add optional orchestration, transport, and program config structs with disabled/manual/host/file defaults while preserving legacy decode behavior.
  - acceptance: Legacy and partial configs produce the documented effective policy without changing existing fields.
  - verify: go test ./internal/core/... -run 'Test.*Config.*(Legacy|Default|Partial)' -count=2
  - depends: —
  - requirements: R1.1, R1.2, R1.4, R1.7

## Wave 1 — Configuration Validation

- [x] T2 — Validate orchestration policy and bounded values
  - why: Prevent unsafe or ambiguous runtime behavior.
  - role: builder
  - files: internal/core/specfiles.go, internal/core/specfiles_test.go
  - contract: Validate enums, clamp bounded integers through one warning path, reject unsupported modes/transports, and prohibit secret-bearing fields.
  - acceptance: Boundary, invalid enum, malformed, and secret-shaped inputs fail or normalize exactly as specified.
  - verify: go test ./internal/core/... -run 'Test.*OrchestrationConfig.*Validation' -count=2
  - depends: T1
  - requirements: R1.3, R1.5, R1.6, R1.8, R1.9

## Wave 2 — Shipped Defaults

- [x] T3 — Update embedded config and effective-policy rendering
  - why: Keep scaffolded projects and machine consumers aligned with the new policy.
  - role: builder
  - files: internal/core/embed_templates/config.json, internal/core/embed.go, internal/core/specfiles_test.go
  - contract: Ship disabled orchestration defaults and add deterministic effective-policy serialization reusable by Brain status.
  - acceptance: Fresh init output is deterministic, legacy init expectations are intentionally updated, and rendered policy contains no sensitive data.
  - verify: go test ./internal/core/... ./internal/cmd/... -run 'Test.*(Init|Config|Policy).*Determin' -count=2
  - depends: T2
  - requirements: R1.10

## Wave 15 — Configuration Documentation

- [ ] T4 — Document policy, authority, and migration behavior
  - why: Users must understand when automation can approve or dispatch work.
  - role: builder
  - files: docs/command-reference.md, docs/agent-integration.md, docs/validation-gates.md
  - contract: Document all fields, defaults, bounds, approval policies, host telemetry semantics, and backward compatibility.
  - acceptance: Documentation states that orchestration is disabled/manual by default and cannot clear high/critical mid-requirement gates.
  - verify: make ci
  - depends: T3, brain-core/T8, pinky-core/T7
  - requirements: R1.1, R1.4, R1.5, R1.9, R1.10
