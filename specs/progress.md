# Pinky & The Brain — Program Progress

Status: implementation complete; Wave 19 complete; awaiting human review
Last updated: 2026-06-19
Implementation progress: 37/37 tasks complete

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
| 9 | Brain reconciliation, Pinky reporting, program decisions | 3 | complete |
| 10 | Brain recovery and Pinky evidence | 2 | complete |
| 11 | Brain CLI, Pinky CLI, program child scheduling | 3 | complete |
| 12 | Brain/Pinky guidance and program failure controls | 3 | complete |
| 13 | Fake host and program CLI | 2 | complete |
| 14 | Brain/program hardening and generated MCP tools | 3 | complete |
| 15 | Configuration docs and CLI/MCP parity | 2 | complete |
| 16 | Bounded MCP interactions | 1 | complete |
| 17 | Host compatibility | 1 | complete |
| 18 | MCP end-to-end lifecycle | 1 | complete |
| 19 | Documentation and full CI | 1 | complete |

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
| config-extension | 4 | 4 | complete | — |
| acp-file-transport | 7 | 7 | complete | — |
| brain-core | 8 | 8 | complete | — |
| pinky-core | 7 | 7 | complete | — |
| program-orchestration | 5 | 5 | complete | — |
| mcp-integration | 6 | 6 | complete | — |

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
- Wave 10 / `brain-core/T5`: added persisted session lifecycle controls
  (start/pause/resume/cancel) under the session lock with atomic private
  session.json writes; made `StepOrchestration` lifecycle-aware so pause stops
  new dispatch, cancellation issues one cooperative cancel directive per active
  lease (never claiming host-process termination) and drains to complete; added
  privileged expired-lease reclamation enabling retry at the next attempt; and
  file-only recovery that reconciles `lastSequence` from the committed event log
  and converges at every event boundary. Verification passed:
  `go test ./internal/core/... -run 'TestOrchestration.*(Pause|Resume|Cancel|Recovery|Retry)' -race -count=2`,
  and `make ci`.
- Wave 10 / `pinky-core/T4`: routed worker completion through one shared
  `core.CompleteTask` integrity path (also used by `specd task --status
  complete`), and added `ReconcilePinkyEvidence`, which records the lease-gated
  immutable ACP evidence event then accepts it only when it references the
  matching specd verification record (binding command, git head, and run time),
  the reported changed files equal the record and stay inside the declared scope
  contract under `scope=error`, and the role is not read-only — completing
  idempotently. Forged refs, stale heads, changed verify commands, undeclared
  files, missing records, non-owners, and read-only roles all fail closed.
  Verification passed:
  `go test ./internal/core/... ./internal/cmd/... -run 'TestPinky.*Evidence|TestTask.*Gate|TestVerify' -count=2`,
  and `make ci`.
- Backprop B5 / 2026-06-19: Wave 9 engine integration exposed that mission
  authority `allowedActions` were validated against the directive verb set
  (`retry/cancel/...`) rather than worker capabilities (`read/edit/verify/
  report`), so any real dispatch failed envelope validation. Split the
  vocabularies into `acpAuthorityActionSet` and corrected the `validACPMission`
  fixture, which had used placeholder directive verbs. No new product invariant;
  the authority/directive split is now enforced by the engine dispatch test.

- Wave 11 / `brain-core/T6`: added `specd brain start|status|step|pause|resume|cancel`,
  registry metadata, explicit orchestration policy validation, and focused command tests.
  Evidence: `go test ./internal/cmd -run 'Test(Brain|Pinky)'`, `go test ./...`.
- Wave 11 / `pinky-core/T5`: added `specd pinky claim|heartbeat|progress|report|block|release`,
  registry metadata, mission JSON input, report argument validation, and focused command tests.
  Evidence: `go test ./internal/cmd -run 'Test(Brain|Pinky)'`, `go test ./...`.
- Wave 11 / `program-orchestration/T2`: added runtime-backed child Brain leases, bounded one-step child scheduling, program capacity enforcement, terminal-completion lease release, and deterministic wave/slug stepping. Verification passed:
  `go test ./internal/core/... -run 'TestProgramOrchestration.*(Lease|Capacity|Frontier)' -race -count=2`,
  `go test ./internal/core/... -run 'TestProgramOrchestration|TestOrchestration' -race -count=2`,
  `make ci`.
- Wave 12 / `brain-core/T7`: added embedded Brain role and skill guidance, installed by default init, documenting deterministic sensing, bounded one-action steps, manual approvals, dispatch, escalation, replay, and the no-LLM/provider boundary. Verification passed:
  `go test ./internal/core/... ./internal/cmd/... -run 'Test.*Embed.*Brain|Test.*Embed.*Pinky|TestInit|TestProgramOrchestration.*(Escalate|Pause|Cancel|Recovery|Complete)' -race -count=2`,
  `go test ./internal/core/... -run 'TestProgramOrchestration|TestOrchestration' -race -count=2`,
  `make ci`.
- Wave 12 / `pinky-core/T6`: added embedded Pinky role and skill guidance, installed by default init, forbidding direct state edits, direct checkbox flips, forged evidence, and trusted host telemetry while documenting context, claim/heartbeat/report/block/cancel, and `specd verify` flow. Verification passed:
  `go test ./internal/core/... ./internal/cmd/... -run 'Test.*Embed.*Brain|Test.*Embed.*Pinky|TestInit|TestProgramOrchestration.*(Escalate|Pause|Cancel|Recovery|Complete)' -race -count=2`,
  `go test ./internal/core/... -run 'TestProgramOrchestration|TestOrchestration' -race -count=2`,
  `make ci`.
- Wave 12 / `program-orchestration/T3`: added persisted parent program sessions, fail-fast child escalation, replay-linked child session IDs in snapshots, cooperative parent pause/cancel propagation to active child Brain sessions, restart-stable control state, and completion derived only from authoritative child spec states. Verification passed:
  `go test ./internal/core/... ./internal/cmd/... -run 'Test.*Embed.*Brain|Test.*Embed.*Pinky|TestInit|TestProgramOrchestration.*(Escalate|Pause|Cancel|Recovery|Complete)' -race -count=2`,
  `go test ./internal/core/... -run 'TestProgramOrchestration|TestOrchestration' -race -count=2`,
  `make ci`.
- Wave 13 / `pinky-core/T7`: added deterministic fake Pinky worker harness using the public CLI/core host contract, covering success, blocker, retry, cooperative cancel, lease expiry, duplicate terminal report idempotency, and scope-violation rejection without network or provider SDK. Verification passed:
  `go test ./internal/integration/... ./internal/core/... ./internal/cmd/... -race -count=2`,
  `make ci`.
- Wave 13 / `program-orchestration/T4`: extended `specd brain` with program start/step/status/pause/resume/cancel, deterministic program status JSON with counts, waves, frontier, critical path, child snapshots, and escalation, while preserving existing `specd program` JSON output and fail-closed approval policy validation. Verification passed:
  `go test ./internal/integration/... ./internal/core/... ./internal/cmd/... -race -count=2`,
  `make ci`.
- Backprop B6 / 2026-06-19: Wave 14 active-session hardening initially scanned the current session directory after the lock directory existed but before `session.json` was written, causing fresh starts to fail with `orchestration engine: session not found`. Fixed the scan to skip the session under creation before loading persisted sessions. No new invariant was needed because the new fake-host lifecycle test and existing program orchestration tests catch this class.
- Wave 14 / `brain-core/T8`: added deterministic fake-host Brain lifecycle coverage for approval gating, pause/resume, retry after failed verification, evidence-gated completion, session completion, and same-spec active-session contention; added `scripts/stress-orchestration.sh` and CI wiring. Verification passed:
  `go test ./internal/integration/... ./internal/core/... -run 'TestFakeHostBrainLifecycle|TestFakeHostProgram|TestOrchestrationEngine|TestProgramOrchestration' -race -count=2`,
  `./scripts/stress-orchestration.sh`,
  `make ci`.
- Wave 14 / `program-orchestration/T5`: added fake-host program lifecycle coverage for parallel wave scheduling, restart recovery, parent contention, fail-fast escalation, dependent start prevention, and all-complete termination; added `scripts/stress-program.sh` and CI wiring. Verification passed:
  `go test ./internal/integration/... ./internal/core/... -run 'TestFakeHostBrainLifecycle|TestFakeHostProgram|TestOrchestrationEngine|TestProgramOrchestration' -race -count=2`,
  `./scripts/stress-program.sh`,
  `make ci`.
- Wave 14 / `mcp-integration/T1`: hardened generated MCP tool schemas with closed `additionalProperties:false`, completed Brain/Pinky command metadata including Pinky terminal report telemetry flags, refreshed golden schemas, and added parity assertions for orchestration tool annotations and flags. Verification passed:
  `go test ./internal/mcp/... ./internal/cmd/... -run 'Test.*Tool|TestRegistryMatchesHelp' -count=2`,
  `make ci`.
- Wave 15 / `config-extension/T4`: documented orchestration policy fields, defaults, bounds, approval authority, host telemetry semantics, high/critical mid-requirement human-only gates, and backward-compatible fail-closed migration behavior in the command reference, agent integration guide, and validation/security gates. Verification passed:
  `make ci`.
- Wave 15 / `mcp-integration/T2`: added CLI/MCP parity coverage for orchestration status and Brain mutations, plus approval mutation, invalid session IDs, evidence-gate failures, and cancellation semantics using equivalent fresh workspaces for CLI and MCP paths. Verification passed:
  `go test ./internal/mcp/... -run 'Test.*(CLI.*MCP.*Parity|Orchestration)' -count=2`,
  `make ci`.
- Wave 16 / `mcp-integration/T3`: added bounded MCP server instructions, capped stdio and HTTP/SSE request payloads with JSON-RPC error envelopes, and rejected unbounded MCP `watch` calls unless `--once` is used without streaming transports. Verification passed:
  `go test ./internal/mcp/... -run 'TestMCP.*(Bounded|Instructions|HTTP|SSE|Malformed)' -count=2`,
  `make ci`.
- Wave 17 / `mcp-integration/T4`: extended MCP probe evidence to require `specd_brain` and `specd_pinky`, added doctor JSON/text evidence that separates server capability from host-managed reload/trust/start lifecycle, and updated the host compatibility matrix without claiming provider-specific agent-spawn control. Verification passed:
  `go test ./internal/mcp/... ./internal/cmd/... -run 'Test.*(Probe|Doctor|Compatibility)' -count=2`,
  `make ci`.
- Wave 18 / `mcp-integration/T5`: added MCP orchestration lifecycle coverage that drives Brain start/status/pause/resume/step/cancel and Pinky claim/heartbeat/progress/block/report/release through stdio and alternating HTTP `/rpc` + `/sse`, reconciles fake-worker evidence, verifies retry and cooperative cancellation convergence, and compares final state/event summaries across transports. Verification passed:
  `go test ./internal/mcp/... -race -count=2`,
  `make ci`.
- Wave 19 / `mcp-integration/T6`: updated MCP, agent integration, troubleshooting, and command reference documentation for generated `specd_brain`/`specd_pinky` tools, bounded polling/stepping, host-owned worker lifecycle, approvals, evidence binding, cooperative cancellation, and file-backed recovery without promising embedded LLM execution. Verification passed:
  `make ci`.

## Progress Update Rules

- Mark a task complete only after its listed verification passes and evidence is recorded.
- Update the counts and `Next task` column whenever a task changes state.
- Record blockers with the exact task ID and failed command.
- A wave is complete only when every task assigned to it is complete.
- Program completion requires `make ci` passing after Wave 6.
