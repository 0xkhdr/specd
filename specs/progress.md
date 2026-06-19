# Pinky & The Brain — Program Progress

Status: implementation in progress; Wave 9 complete; awaiting human review
Last updated: 2026-06-19
Implementation progress: 18/37 tasks complete

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
| 3 | Runtime path security | 1 | complete |
| 4 | Atomic ACP event store | 1 | complete |
| 5 | Idempotent ACP delivery | 1 | complete |
| 6 | Worker leases and Brain models | 2 | complete |
| 7 | ACP archive, Brain sensing, Pinky mission contract | 3 | complete |
| 8 | ACP stress, Brain decisions, Pinky claims | 3 | complete |
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
| acp-file-transport | 7 | 7 | complete | — |
| brain-core | 8 | 4 | in progress | T5 (Wave 10) |
| pinky-core | 7 | 3 | in progress | T4 (Wave 10) |
| program-orchestration | 5 | 1 | in progress | T2 (Wave 10) |
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
- Backprop B3 / 2026-06-18: the ACP permission fixture created every missing
  parent with read-only permissions, so setup failed before exercising the
  intended session-directory denial; cursor corruption diagnostics also lacked
  a consistent prefix. The fixture now constrains only the target directory and
  cursor validation errors are labeled consistently. No new invariant was
  needed because this was test construction and diagnostic classification, not
  a product contract gap.
- Backprop B4 / 2026-06-18: Wave 7/8 implementation exposed mechanical
  test-construction errors: terminal ACP direction must remain Pinky→Brain
  after changing message type, and stress replay must read fixture session
  actually written. Fixed fixtures. No new invariant added because ACP
  direction and ordered replay already have dedicated tests.
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
- Wave 3 / `acp-file-transport/T2`: added canonical runtime path derivation for
  sessions, events, workers, leases, cursors, archives, and artifacts with
  strict opaque/segment ID validation, deterministic event filenames, canonical
  project roots, lexical containment, and fail-closed rejection of symlinked
  runtime components. Verification passed:
  `go test ./internal/core/... -run 'TestRuntimePath' -count=2`,
  `make build`, `make test`, and `make ci`.
- Wave 4 / `acp-file-transport/T3`: added per-session cross-process sequence
  allocation, immutable no-overwrite event publication, fsync-before-publish,
  private runtime permissions, stale-lock recovery, existing-log validation,
  and gap/rollback/duplicate rejection. Verification passed:
  `go test ./internal/core/... -run 'TestACPStore.*(Write|Concurrent|Permission)' -race -count=2`,
  `make build`, `make test`, and `make ci`.
- Wave 5 / `acp-file-transport/T4`: added ordered fail-closed event reads,
  explicit atomic consumer cursors, replay deduplication by `messageId`, and
  cursor advancement rules that cannot skip unreconciled events while allowing
  acknowledged duplicates. Verification passed:
  `go test ./internal/core/... -run 'TestACPStore.*(Read|Cursor|Duplicate|Corrupt)' -count=2`,
  `make build`, `make test`, and `make ci`.
- Wave 6 / `acp-file-transport/T5`: added atomic worker claim, bounded heartbeat
  renewal, message-TTL enforcement, idempotent release, attempt-monotonic
  reclaim, unique worker identity history, and terminal-report ownership
  validation using the injected clock. Verification passed:
  `go test ./internal/core/... -run 'TestACPLease' -race -count=2`,
  `make build`, `make test`, and `make ci`.
- Wave 6 / `brain-core/T1`: added closed orchestration action, lifecycle, and
  escalation enums; bounded policy conversion; snapshot, decision, session,
  lease, failure, and task models; validation for every current status/gate;
  and canonical deterministic JSON with sorted non-null lists. Verification
  passed:
  `go test ./internal/core/... -run 'TestOrchestrationModel' -count=2`,
  `make build`, `make test`, and `make ci`.

- Wave 7 / `acp-file-transport/T6`: added terminal-session ACP archive
  sealing, ordered archived replay through core/CLI replay path, and
  validated retention cleanup constrained to runtime archive IDs.
  Verification passed:
  `go test ./internal/core/... ./internal/cmd/... -run 'TestACPArchive|TestReplay.*ACP' -count=2`,
  and `make ci`.
- Wave 7 / `brain-core/T2`: added authoritative orchestration snapshot sensing
  from `LoadSpec`, runnable frontier, verification failures, lifecycle gates,
  and active ACP leases without shelling out. Verification passed:
  `go test ./internal/core/... -run 'TestOrchestrationSense' -count=2`,
  and `make ci`.
- Wave 7 / `pinky-core/T1`: added deterministic Pinky mission contract with
  authority, attempt/deadline/heartbeat, task contract fields, verify command,
  dependencies, requirements, and dispatch digest. Verification passed:
  `go test ./internal/core/... ./internal/cmd/... -run 'TestPinkyMission|TestDispatch' -count=2`,
  and `make ci`.
- Wave 8 / `acp-file-transport/T7`: added ACP hostile-input/security and
  stress coverage for traversal, symlinks, oversized payloads, concurrent
  writers, stale leases, and `scripts/stress-acp.sh`/`make stress-acp`.
  Verification passed: `make ci`.
- Wave 8 / `brain-core/T3`: added pure deterministic decision table mapping
  snapshots plus policy to approval, dispatch, wait, escalation, and complete
  actions with stable idempotency keys. Verification passed:
  `go test ./internal/core/... -run 'TestOrchestrationDecide' -count=20`,
  and `make ci`.
- Wave 8 / `pinky-core/T2`: added Pinky claim, heartbeat, and release core
  operations over ACP leases, rejecting duplicate/stale/wrong ownership.
  Verification passed:
  `go test ./internal/core/... -run 'TestPinky.*(Claim|Heartbeat|Release)' -race -count=2`,
  and `make ci`.
- Wave 9 / `brain-core/T4`: added the one-step reconciliation engine wiring
  sense → decide → record under the spec lock, materializing dispatch/retry
  missions and cancel directives into idempotent ACP events keyed by the
  decision idempotency key. Verification passed:
  `go test ./internal/core/... -run 'TestOrchestrationEngine' -race -count=2`,
  and `make ci`.
- Wave 9 / `pinky-core/T3`: added Pinky progress, blocker, terminal evidence,
  and cancellation-acknowledgement reporting over ACP, lease-validated and with
  idempotent terminal evidence. Verification passed:
  `go test ./internal/core/... -run 'TestPinky.*(Progress|Block|Report|Cancel)' -race -count=2`,
  and `make ci`.
- Wave 9 / `program-orchestration/T1`: added pure program snapshot construction
  and the deterministic program decision table (start/wait/escalate/complete)
  over the child-spec DAG with capacity, cycle, orphan, and blocker handling.
  Verification passed:
  `go test ./internal/core/... -run 'TestProgramOrchestration' -count=20`,
  and `make ci`.
- Backprop B5 / 2026-06-19: Wave 9 engine integration exposed that mission
  authority `allowedActions` were validated against the directive verb set
  (`retry/cancel/...`) rather than worker capabilities (`read/edit/verify/
  report`), so any real dispatch failed envelope validation. Split the
  vocabularies into `acpAuthorityActionSet` and corrected the `validACPMission`
  fixture, which had used placeholder directive verbs. No new product invariant;
  the authority/directive split is now enforced by the engine dispatch test.

## Progress Update Rules

- Mark a task complete only after its listed verification passes and evidence is recorded.
- Update the counts and `Next task` column whenever a task changes state.
- Record blockers with the exact task ID and failed command.
- A wave is complete only when every task assigned to it is complete.
- Program completion requires `make ci` passing after Wave 6.
