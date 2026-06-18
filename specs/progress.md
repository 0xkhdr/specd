# Pinky & The Brain — Program Progress

Status: implementation in progress; Wave 2 complete; awaiting human review
Last updated: 2026-06-18
Implementation progress: 4/37 tasks complete

## Program Outcome

Deliver deterministic, resumable orchestration while preserving specd's defining contracts:

- Go standard library only; no model/provider SDK and no LLM calls.
- Existing state, gates, locks, CAS, dispatch, verification, program DAG, replay, and MCP remain canonical.
- Brain is a deterministic controller.
- Pinky is a host-executed worker contract.
- Manual approval is the default; evidence remains mandatory.

## Global Waves

| Wave | Goal | Tasks | Status |
|---:|---|---:|---|
| 0 | Configuration model | 1 | complete |
| 1 | Configuration validation | 1 | complete |
| 2 | Shipped defaults and ACP protocol | 2 | complete |
| 3 | Runtime path security | 1 | pending |
| 4 | Atomic ACP event store | 1 | pending |
| 5 | Idempotent ACP delivery | 1 | pending |
| 6 | Worker leases and Brain models | 2 | pending |
| 7 | ACP archive, Brain sensing, Pinky mission contract | 3 | pending |
| 8 | ACP stress, Brain decisions, Pinky claims | 3 | pending |
| 9 | Brain reconciliation, Pinky reporting, program decisions | 3 | pending |
| 10 | Brain recovery and Pinky evidence | 2 | pending |
| 11 | Brain CLI, Pinky CLI, program child scheduling | 3 | pending |
| 12 | Brain/Pinky guidance and program failure controls | 3 | pending |
| 13 | Fake host and program CLI | 2 | pending |
| 14 | Brain/program hardening and generated MCP tools | 3 | pending |
| 15 | Configuration docs and CLI/MCP parity | 2 | pending |
| 16 | Bounded MCP interactions | 1 | pending |
| 17 | Host compatibility | 1 | pending |
| 18 | MCP end-to-end lifecycle | 1 | pending |
| 19 | Documentation and full CI | 1 | pending |

## Dependency Spine

```text
config policy
  -> ACP schema/store
  -> ACP delivery/leases
  -> Brain decision engine + Pinky mission contract
  -> Brain/Pinky session operations
  -> program scheduler
  -> generated MCP surface
  -> end-to-end stress/security/docs
```

Parallel work is allowed only inside a wave when listed dependencies are complete. Cross-spec dependencies use `spec-name/Tn`.

## Spec Progress

| Spec | Tasks | Complete | Current state | Next task |
|---|---:|---:|---|---|
| config-extension | 4 | 3 | in progress | T4 (Wave 15) |
| acp-file-transport | 7 | 1 | in progress | T2 |
| brain-core | 8 | 0 | proposed | blocked by ACP store/delivery |
| pinky-core | 7 | 0 | proposed | blocked by ACP store/delivery |
| program-orchestration | 5 | 0 | proposed | blocked by brain-core/T3 |
| mcp-integration | 6 | 0 | proposed | blocked by Brain/Pinky/program CLI |

## Review Decisions Requested

1. Approve the product boundary: specd performs no LLM calls; external hosts execute Pinky missions.
2. Approve manual approval as the default and the bounded `planning`/`session` policies.
3. Approve file ACP v1 with at-least-once, idempotent delivery rather than an exactly-once claim.
4. Approve cooperative cancellation and post-edit scope enforcement as the honest v1 security boundary.
5. Approve the seven-wave implementation order and fail-fast program policy.

## Implementation Evidence

- Backprop B1 / 2026-06-18: Wave 2 deterministic config parity exposed
  `DefaultConfig.Gates.Custom=nil` while the shipped JSON decoded it as `[]`.
  Added config invariant V7 and a parity test; defaults now use the canonical
  non-null empty list.
- Backprop B2 / 2026-06-18: ACP payload-negative fixtures changed the message
  type without changing sender direction, so direction validation masked the
  intended payload diagnostics. Fixtures now satisfy earlier gates; no new
  invariant was needed because directionality already has a dedicated test.
- Wave 0 / `config-extension/T1`: added backward-compatible orchestration,
  transport, and program configuration models with field-level defaults.
  Verification passed:
  `go test ./internal/core/... -run 'Test.*Config.*(Legacy|Default|Partial)' -count=2`,
  `make build`, `make test`, and `make ci`.
- Wave 1 / `config-extension/T2`: added fail-closed orchestration validation
  for policy enums, worker/transport modes, cost values, timing relationships,
  bounded concurrency/retry/timeout values, and secret-shaped configuration
  fields while preserving unrelated legacy settings. Verification passed:
  `go test ./internal/core/... -run 'Test.*OrchestrationConfig.*Validation' -count=2`,
  `make build`, `make test`, and `make ci`.
- Wave 2 / `config-extension/T3`: embedded the complete disabled/manual/host/file
  orchestration defaults and added canonical effective-policy JSON rendering
  from the concrete policy type, making sensitive fields structurally
  impossible. Verification passed:
  `go test ./internal/core/... ./internal/cmd/... -run 'Test.*(Init|Config|Policy).*Determin' -count=2`,
  `make build`, `make test`, and `make ci`.
- Wave 2 / `acp-file-transport/T1`: added ACP v1 envelopes, nine typed message
  payloads, cryptographically random opaque IDs, strict unknown-field
  rejection, deterministic validation for versions/IDs/direction/slugs/tasks/
  roles/timestamps/limits, and JSON Schema parity coverage. Verification passed:
  `go test ./internal/core/... -run 'TestACP|TestSchema' -count=2`,
  `make build`, `make test`, and `make ci`.

## Progress Update Rules

- Mark a task complete only after its listed verification passes and evidence is recorded.
- Update the counts and `Next task` column whenever a task changes state.
- Record blockers with the exact task ID and failed command.
- A wave is complete only when every task assigned to it is complete.
- Program completion requires `make ci` passing after Wave 6.
