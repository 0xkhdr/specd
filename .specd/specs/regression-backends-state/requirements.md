# Requirements — Regression: State Backends (default/git/postgres/redis, locking, CAS)

## Introduction
specd persists spec state through a pluggable backend interface (`internal/core/backend*.go`)
with default (filesystem), git, postgres, and redis implementations. A regression here
guarantees that every backend satisfies the same conformance contract — identical
read/write/CAS semantics — so users can swap storage without changing correctness. Value:
storage choice is an operational decision, never a reliability gamble.

## Requirement 1 — Backend conformance parity
**User story:** As an operator choosing a backend, I want every backend to pass the same
conformance suite, so that behavior is identical regardless of storage.

**Acceptance criteria:**
1. THE SYSTEM SHALL run one shared conformance suite against default, git, postgres, and redis backends
2. WHEN a backend reads a key it previously wrote THE SYSTEM SHALL return the exact bytes written
3. IF a backend is unavailable THEN THE SYSTEM SHALL skip its conformance run with a clear reason, not fail silently

## Requirement 2 — Compare-and-swap integrity
**User story:** As a concurrent writer, I want CAS to reject stale writes, so that lost
updates are impossible.

**Acceptance criteria:**
1. WHEN a write presents the current revision THE SYSTEM SHALL commit and bump the revision
2. IF a write presents a stale revision THEN THE SYSTEM SHALL reject it with a conflict
3. THE SYSTEM SHALL keep revision monotonically increasing across every committed write

## Requirement 3 — Locking & recovery
**User story:** As a multi-process deployment, I want locks to be correct and self-healing,
so that a crashed holder cannot deadlock the system.

**Acceptance criteria:**
1. WHILE a lock is held THE SYSTEM SHALL block or fail competing acquirers deterministically
2. IF a lock holder dies THEN THE SYSTEM SHALL allow recovery without manual intervention
3. THE SYSTEM SHALL release locks on normal completion

## Requirement 4 — Durability & schema validity
**User story:** As a user, I want persisted state to always be schema-valid, so that a crash
mid-write never yields an unreadable spec.

**Acceptance criteria:**
1. THE SYSTEM SHALL leave state schema-valid after every committed write
2. IF a write is interrupted THEN THE SYSTEM SHALL leave either the prior or the new state, never a partial one
3. WHERE the git backend is used THE SYSTEM SHALL produce a clean, replayable commit per write
