# Spec — Rate-Limit Aware Lease (R3)

**Priority:** P1 · **Wave:** 2 · **Gap:** R3 (no graceful rate-limit pause).

## Introduction

When a host hits a provider rate limit, its worker stops heartbeating. The lease expires, and
the Brain records a **retryable failure** — so the task is re-dispatched to a fresh worker that
consumes *more* rate-limit quota and tokens. Rate-limit blocks become false-positive failures
that amplify into retry storms.

This spec adds a **suspend/resume** lease state so the system can distinguish *"worker is
rate-limited and will return"* from *"worker is dead."* A worker that hits a rate limit calls
`pinky suspend` with an expected return window; the Brain extends the lease and does **not**
mark a failure. When the worker returns it calls `pinky resume`. If it never returns by the
extended deadline, normal lease-expiry failure applies.

## Current-state grounding

- Lease model: `ACPLease` (`Status`, `HeartbeatAt`, `LeaseUntil`, `MessageExpiresAt`) and
  `ACPLeaseStatus` (`ACPLeaseActive`, `ACPLeaseReleased`) in `internal/core/acp_lease.go`.
- Lease ops: `ClaimLease`, heartbeat/renew, reclaim-on-expiry logic in the same file; failure
  recording in the orchestration commit path (`recordWorkerFailure`).
- Worker commands: `internal/cmd/pinky.go` (`heartbeat`, `release`, …).
- Transport timing: `TransportCfg{LeaseSeconds, HeartbeatSeconds}` in
  `internal/core/specfiles.go`.

## Requirements

### Requirement 1 — Suspended lease state
**User story:** As the Brain, I want a lease state that means "temporarily away, will return,"
so a rate-limited worker is not treated as dead.

**Acceptance criteria:**
1. THE SYSTEM SHALL add `ACPLeaseSuspended` to `ACPLeaseStatus` and accept it in lease
   validation.
2. WHILE a lease is `suspended` and within its extended deadline THE SYSTEM SHALL NOT reclaim it
   or record a worker failure.
3. THE SYSTEM SHALL persist suspend metadata on the lease: `SuspendedAt`, `SuspendReason`,
   `ResumeDeadline` (all omitempty so non-suspended leases stay byte-stable).

### Requirement 2 — `specd pinky suspend`
**User story:** As a worker that just caught a rate-limit error, I want to suspend my lease for a
stated window, so the Brain holds my task.

**Acceptance criteria:**
1. WHEN a worker runs `specd pinky suspend --session <id> --worker <id> --attempt <n>
   --reason <r> --resume-after-seconds <s>` THE SYSTEM SHALL set the lease to `suspended` and
   extend `LeaseUntil` to `now + s + heartbeatBuffer`.
2. THE SYSTEM SHALL accept reasons `rate-limit`, `context-compaction`, `provider-maintenance`
   and reject others.
3. THE SYSTEM SHALL cap cumulative suspension at `orchestration.resilience.maxSuspendSeconds`
   (default 600); a suspend that would exceed the cap is rejected and the lease is left to expire
   normally.
4. THE SYSTEM SHALL allow repeated `suspend` calls (each re-extends, subject to the cap).
5. IF the caller does not hold the active/suspended lease for `<attempt>` THEN THE SYSTEM SHALL
   fail with non-zero exit.

### Requirement 3 — `specd pinky resume`
**User story:** As a returning worker, I want to clear my suspension and continue, so I keep the
same lease and task.

**Acceptance criteria:**
1. WHEN a worker runs `specd pinky resume --session <id> --worker <id> --attempt <n>` THE SYSTEM
   SHALL return the lease to `active`, refresh `HeartbeatAt`, and reset `LeaseUntil` to the
   normal lease window.
2. IF the resume deadline already passed and the lease was reclaimed THEN THE SYSTEM SHALL report
   that the lease is gone and the worker must re-claim (non-zero exit, clear message).
3. THE SYSTEM SHALL emit a `resume` ACP event recording the suspended duration.

### Requirement 4 — Brain does not false-fail suspended workers
**User story:** As an operator, I want no retry storms from rate limits.

**Acceptance criteria:**
1. WHILE any lease is `suspended` within deadline THE SYSTEM SHALL treat the task as in-flight
   (counts toward `maxWorkers`, not a runnable/failed task).
2. WHEN a suspended lease's `ResumeDeadline` passes without a `resume` THE SYSTEM SHALL then —
   and only then — treat it as a reclaimable expired lease and apply the normal retry policy.
3. THE SYSTEM SHALL keep `DecideOrchestration` pure: suspension state enters via the snapshot in
   `SenseOrchestration`.

### Requirement 5 — Config
**Acceptance criteria:**
1. THE SYSTEM SHALL add `orchestration.resilience.maxSuspendSeconds` (default 600) to the shared
   `Resilience` config block; absent → byte-identical config.
2. THE SYSTEM SHALL validate `0 < maxSuspendSeconds <= 3600`.

## Design

- Extend `ACPLease` with the three omitempty suspend fields and `ACPLeaseSuspended` status.
- New ops in `acp_lease.go`: `SuspendLease(...)` and `ResumeLease(...)`, both CAS-guarded like
  `ClaimLease`. Suspend tracks cumulative suspended seconds on the lease to enforce the cap.
- Reclaim path: the existing expiry check must treat `suspended` leases by `ResumeDeadline`, not
  `LeaseUntil`-vs-heartbeat — adjust the single reclaim predicate, keep everything else.
- Snapshot: add `Suspended bool` (or reuse lease status) on `OrchestrationLeaseSnapshot` so
  `SenseOrchestration` exposes suspension and the in-flight accounting in `DecideOrchestration`
  counts it without disk reads.
- CLI: add `suspend` / `resume` cases to `pinky.go` mirroring the `heartbeat` flag parsing.

## Coordination
- Shares the lease-state-extension surface with `checkpoint-protocol` (which also releases
  leases). Land `checkpoint-protocol` T3 first if both touch `acp_lease.go` concurrently, or
  rebase. Shares the `Resilience` config struct with `auto-resume`/`checkpoint-protocol`.

## Out of scope
- Detecting rate limits inside any specific host — workers call `suspend` explicitly; reference
  worker integration is documented, not enforced.

## Risks
- **Indefinite suspension:** a worker spamming `suspend` could hold a task forever. Mitigated by
  the cumulative `maxSuspendSeconds` cap.
- **Reclaim regression:** mis-editing the expiry predicate could break normal reclaim. Mitigated
  by keeping the change to one predicate + existing reclaim tests staying green.
