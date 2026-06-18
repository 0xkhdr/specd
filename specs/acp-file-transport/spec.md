# Spec: ACP v1 File Transport

Status: proposed — awaiting human review
Scope: durable, local, auditable work exchange between the deterministic Brain controller and host-executed Pinky workers.

## 1. Outcome

Define a versioned Agent Communication Protocol (ACP) and implement its first transport using atomic files under `.specd/runtime/`. ACP complements existing `dispatch`, `watch`, `verify`, and `replay`; it does not create a second source of task truth.

## 2. Requirements

- R2.1 Define typed Go envelopes and a shipped ACP v1 JSON Schema using the existing schema-generation/validation approach.
- R2.2 Support message types `mission`, `accepted`, `heartbeat`, `progress`, `evidence`, `blocker`, `query`, `directive`, and `cancelled`.
- R2.3 Use cryptographically random opaque IDs encoded with the Go standard library; UUID formatting is not required.
- R2.4 Write immutable message files atomically with `0600` minus umask and fsync-before-rename semantics.
- R2.5 Validate version, IDs, slugs, task IDs, role, sender, receiver, payload limits, and all derived paths on read and write.
- R2.6 Provide at-least-once delivery with idempotent consumption keyed by `messageId`; never claim exactly-once delivery.
- R2.7 Add worker leases, heartbeat expiry, message TTL, acknowledgement, retry count, and terminal archival.
- R2.8 Use per-session and per-worker directories so concurrent workers never overwrite one another.
- R2.9 Preserve message order with a monotonic sequence allocated under a session lock.
- R2.10 Bound every message and captured output field; store output tails or artifact references, never unbounded logs.
- R2.11 Archive complete sessions for deterministic replay and retention cleanup.
- R2.12 Recover safely after process crashes, partial files, stale leases, duplicate messages, and interrupted archive moves.

## 3. Layout

```text
.specd/runtime/
└── sessions/<session-id>/
    ├── session.json
    ├── events/                 # immutable ordered ACP envelopes
    ├── workers/<worker-id>/
    │   ├── lease.json
    │   └── cursor.json
    └── artifacts/              # bounded stdout/stderr or host attachments
```

All runtime files are operational records. Existing spec `state.json`, `tasks.md`, verification records, and `program.json` remain authoritative.

## 4. Envelope

Required fields:

- `version`, `messageId`, `sessionId`, `sequence`, `createdAt`, `expiresAt`
- `type`, `from`, `to`, `spec`, optional `task`
- `attempt`, optional `inReplyTo`, `payload`

Mission payloads reference a deterministic dispatch packet digest and include role, contract, declared files, acceptance, verify command, dependencies, and authority limits. Evidence payloads reference the verification record created by `specd verify`; ACP evidence alone cannot complete a task.

## 5. Security and Trust

- `tasks.md`, mission payloads, verify commands, host telemetry, and worker text are untrusted input.
- ACP never executes payload text.
- Paths are derived only after slug/ID validation and containment checks.
- Symlink traversal, hard-link surprises, oversized JSON, duplicate IDs, sequence rollback, and stale writers fail closed.
- Runtime cleanup may delete only validated paths beneath `.specd/runtime/sessions/`.

## 6. Invariants

- V1 ACP cannot mutate task or phase state directly.
- V2 A message is visible only after a complete atomic write.
- V3 Reprocessing a message is safe and produces no duplicate state transition.
- V4 Expired or unleased workers cannot submit accepted evidence.
- V5 All ordering is deterministic for the same event set.
- V6 The implementation uses only Go standard library packages.

## 7. Acceptance

- Unit tests cover schema, path attacks, permissions, atomicity, deduplication, leases, TTL, crash recovery, and archive replay.
- A cross-process stress test proves no lost or overwritten messages.
- `go test ./... -race -count=2` and `make ci` pass.
