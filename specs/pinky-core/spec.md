# Spec: Pinky Worker Contract and Host Adapter

Status: proposed — awaiting human review
Scope: worker mission lifecycle and host-facing protocol; specd itself still performs zero LLM calls.

## 1. Outcome

Pinky is a portable worker contract, not an embedded model runtime. An MCP/CLI-capable coding-agent host accepts a mission, performs the assigned role, and reports progress. specd validates authority, records evidence, and owns all state transitions.

## 2. Requirements

- R4.1 Define a typed mission model derived from the existing `dispatchPacket`.
- R4.2 Include session/worker IDs, attempt, spec/task, role prompt digest, context command, contract, files, acceptance, verify command, dependencies, deadlines, and allowed actions.
- R4.3 Provide host commands `specd pinky claim|heartbeat|progress|report|block|release`.
- R4.4 Claim must atomically acquire a worker lease and reject duplicate or expired ownership.
- R4.5 The worker loads context through `specd context`, role assets, and explicitly listed files; the mission must not embed arbitrary repository contents by default.
- R4.6 Builder authority is limited to the mission contract; investigator/reviewer/verifier remain read-only except for ACP reporting.
- R4.7 Workers do not run task `verify:` commands directly as trusted proof; they request `specd verify`, which executes through the existing scrubbed environment and sandbox runner.
- R4.8 Completion requires the existing `specd task --status complete` integrity path after verification, dependency, scope, and gate checks.
- R4.9 Report stdout/stderr tails, duration, changed files, git head, and verification record reference. Token/cost fields are optional host assertions labeled `hostReported`.
- R4.10 Heartbeats extend leases within policy bounds; missing heartbeats make work reclaimable without accepting late evidence.
- R4.11 Cancellation is cooperative and observable. The host must stop at its next safe point and acknowledge; specd cannot promise process termination.
- R4.12 Add embedded `pinky.md` and `specd-pinky/SKILL.md` guidance usable by any host.
- R4.13 Provide a deterministic fake worker used by integration tests; no real model is required in CI.
- R4.14 Detect undeclared changed files through the existing scope gate and fail closed when configured as `error`.

## 3. Lifecycle

```text
offered -> claimed -> active -> verifying -> reported -> released
                    \-> blocked
                    \-> cancelled
                    \-> lease-expired -> offered(attempt+1)
```

Terminal reports are immutable ACP events. Task state remains pending/in-progress/blocked/complete in `state.json`.

## 4. Trust and Sandbox Model

- A Pinky host runs with the invoking user's OS authority unless the host itself provides isolation.
- `verify.sandbox` protects verification, not arbitrary edits made by the coding agent.
- File declarations are enforceable after the fact through changed-file evidence; strict edit-time isolation is out of scope for v1.
- Mission text, host output, telemetry, and changed-file claims are untrusted until reconciled with specd-generated evidence.
- No provider key, model name, pricing table, or token accounting logic belongs in core specd.

## 5. Invariants

- V1 Pinky cannot directly write `state.json` or flip `tasks.md` checkboxes.
- V2 Only the current lease owner may report progress or evidence.
- V3 Late/duplicate reports are recorded but cannot repeat a state transition.
- V4 Host-reported telemetry never affects correctness; only configured limits may stop future dispatch.
- V5 Read-only roles cannot use the unverified escape hatch without explicit evidence and the existing role rules.
- V6 Fake-worker and real-host flows use the same public contract.

## 6. Acceptance

- Tests cover claim races, heartbeats, lease expiry, cancellation, duplicate reports, stale attempts, role authority, evidence reconciliation, and scope violations.
- A fake host completes one builder mission and one blocker/retry mission end to end.
- `go test ./... -race -count=2` and `make ci` pass.
