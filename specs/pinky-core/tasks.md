# Tasks — Pinky Worker Contract and Host Adapter

## Wave 7 — Pinky Mission Contract

- [x] T1 — Define mission and worker report models
  - why: Hosts need a stable contract derived from existing dispatch semantics.
  - role: builder
  - files: internal/core/pinky.go, internal/core/pinky_test.go, internal/cmd/dispatch.go
  - contract: Model mission, authority, attempt, deadline, heartbeat, progress, blocker, verification reference, changed files, and host-reported telemetry; derive missions from dispatchPacket.
  - acceptance: Mission output is deterministic and includes every task contract field without embedding arbitrary repository contents.
  - verify: go test ./internal/core/... ./internal/cmd/... -run 'TestPinkyMission|TestDispatch' -count=2
  - depends: acp-file-transport/T5, config-extension/T3
  - requirements: R4.1, R4.2, R4.5, R4.9

## Wave 8 — Pinky Claims

- [x] T2 — Implement claim, heartbeat, and release core operations
  - why: A host must prove current ownership before acting.
  - role: builder
  - files: internal/core/pinky.go, internal/core/pinky_claim.go, internal/core/pinky_claim_test.go
  - contract: Claim missions atomically, renew bounded leases, release ownership, and reject duplicates, stale attempts, expired sessions, or wrong workers.
  - acceptance: Claim races have one winner and late workers cannot extend or release another worker's lease.
  - verify: go test ./internal/core/... -run 'TestPinky.*(Claim|Heartbeat|Release)' -race -count=2
  - depends: T1
  - requirements: R4.3, R4.4, R4.10

## Wave 9 — Pinky Reporting

- [ ] T3 — Implement progress, blocker, report, and cancel reconciliation
  - why: Worker output must be auditable without becoming trusted state.
  - role: builder
  - files: internal/core/pinky_report.go, internal/core/pinky_report_test.go
  - contract: Validate lease ownership, append ACP events, label host telemetry, enforce cooperative cancellation, and reject stale/duplicate terminal reports.
  - acceptance: Reports are immutable; duplicates are idempotent; cancelled/expired workers cannot submit accepted evidence.
  - verify: go test ./internal/core/... -run 'TestPinky.*(Progress|Block|Report|Cancel)' -race -count=2
  - depends: T2
  - requirements: R4.9, R4.10, R4.11

## Wave 10 — Pinky Evidence

- [ ] T4 — Reconcile worker completion through existing integrity paths
  - why: Pinky must not create a second verification or task-completion mechanism.
  - role: builder
  - files: internal/core/pinky_report.go, internal/cmd/verify.go, internal/cmd/task.go, internal/core/pinky_evidence_test.go
  - contract: Accept only specd-generated matching verification records, enforce dependencies/roles/scope/gates, and invoke existing completion behavior idempotently.
  - acceptance: Forged ACP evidence, changed verify command, stale git head, undeclared files, missing dependencies, and read-only role misuse fail closed.
  - verify: go test ./internal/core/... ./internal/cmd/... -run 'TestPinky.*Evidence|TestTask.*Gate|TestVerify' -count=2
  - depends: T3, brain-core/T4
  - requirements: R4.6, R4.7, R4.8, R4.14

## Wave 11 — Pinky CLI

- [ ] T5 — Add Pinky CLI and registry metadata
  - why: Hosts require one canonical interface that MCP can expose automatically.
  - role: builder
  - files: internal/cmd/pinky.go, internal/cmd/registry.go, internal/core/commands.go, internal/cmd/pinky_test.go
  - contract: Add claim/heartbeat/progress/report/block/release commands with bounded structured inputs, deterministic outputs, and established exit codes.
  - acceptance: Help/registry parity passes and every mutation validates session, worker, spec, task, attempt, and lease.
  - verify: go test ./internal/cmd/... ./internal/core/... -run 'TestPinky|TestRegistryMatchesHelp' -count=2
  - depends: T4
  - requirements: R4.3

## Wave 12 — Pinky Guidance

- [ ] T6 — Add embedded Pinky role and skill guidance
  - why: Any compatible coding-agent host should execute the same mission protocol.
  - role: builder
  - files: internal/core/embed_templates/roles/pinky.md, internal/core/embed_templates/skills/specd-pinky/SKILL.md, internal/core/embed.go
  - contract: Document role authority, context loading, verify flow, progress/blocker protocol, cancellation, and telemetry trust labels.
  - acceptance: Embedded guidance forbids direct state edits and direct evidence claims and is tested during init.
  - verify: go test ./internal/core/... ./internal/cmd/... -run 'Test.*Embed.*Pinky|TestInit' -count=2
  - depends: T5
  - requirements: R4.5, R4.6, R4.7, R4.8, R4.12

## Wave 13 — Fake Host and Compatibility

- [ ] T7 — Build deterministic fake worker and lifecycle tests
  - why: CI needs complete coverage without an LLM or provider SDK.
  - role: verifier
  - files: internal/testharness/pinky.go, internal/integration/pinky_test.go
  - contract: Implement a scripted worker using public CLI/core contracts and cover success, blocker, retry, cancel, lease expiry, duplicate report, and scope violation.
  - acceptance: Fake and host-facing paths are identical; tests run hermetically with injected time and no network.
  - verify: go test ./internal/integration/... ./internal/core/... ./internal/cmd/... -race -count=2
  - depends: T6
  - requirements: R4.10, R4.11, R4.13, R4.14
