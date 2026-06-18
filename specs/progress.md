# Pinky & The Brain — Program Progress

Status: implementation in progress; Wave 0 complete; awaiting human review
Last updated: 2026-06-18
Implementation progress: 1/37 tasks complete

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
| 1 | Configuration validation | 1 | pending |
| 2 | Shipped defaults and ACP protocol | 2 | pending |
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
| config-extension | 4 | 1 | in progress | T2 |
| acp-file-transport | 7 | 0 | proposed | blocked by config-extension/T2 |
| brain-core | 8 | 0 | proposed | blocked by config-extension/T3 and ACP |
| pinky-core | 7 | 0 | proposed | blocked by config-extension/T3 and ACP |
| program-orchestration | 5 | 0 | proposed | blocked by brain-core/T3 |
| mcp-integration | 6 | 0 | proposed | blocked by Brain/Pinky/program CLI |

## Review Decisions Requested

1. Approve the product boundary: specd performs no LLM calls; external hosts execute Pinky missions.
2. Approve manual approval as the default and the bounded `planning`/`session` policies.
3. Approve file ACP v1 with at-least-once, idempotent delivery rather than an exactly-once claim.
4. Approve cooperative cancellation and post-edit scope enforcement as the honest v1 security boundary.
5. Approve the seven-wave implementation order and fail-fast program policy.

## Implementation Evidence

- Wave 0 / `config-extension/T1`: added backward-compatible orchestration,
  transport, and program configuration models with field-level defaults.
  Verification passed:
  `go test ./internal/core/... -run 'Test.*Config.*(Legacy|Default|Partial)' -count=2`,
  `make build`, `make test`, and `make ci`.

## Progress Update Rules

- Mark a task complete only after its listed verification passes and evidence is recorded.
- Update the counts and `Next task` column whenever a task changes state.
- Record blockers with the exact task ID and failed command.
- A wave is complete only when every task assigned to it is complete.
- Program completion requires `make ci` passing after Wave 6.
