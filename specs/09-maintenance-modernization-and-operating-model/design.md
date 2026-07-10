# Design — Maintenance, modernization, and the operating model

## Decision

Turn "operate the system over time" into deterministic, additive records without ever touching the
forward-only ratchet. Maintenance never mutates a completed spec; every follow-up is a **new spec**
joined by a typed link (`follows`/`regresses`/`maintains`/`supersedes`) in `program.go`. Intake,
decisions, exceptions, and promoted memory all gain identity, ownership, and time-bound lifecycle,
so stale governance and stale learning fail closed instead of silently persisting. Drift and
recurring-invariant checks are pure offline projections plus an external-scheduler contract —
`specd` validates and records, it never becomes a daemon. Portfolio governance, exports, and
outcome review are deterministic projections that keep "unknown" distinct from success. Task
completion authority is untouched: nothing here substitutes for a passing verify record at a HEAD.

```text
completed spec (immutable) ──> typed link {follows|regresses|maintains|supersedes} ──> successor spec
        ↑ never reopened                                                                  │
external signal (incident/drift/dep/migration/recurring-fail)                             │
        ↓                                                                                 ▼
provenance.json {source_type, source_ref, systems, severity, owner, prior_links} ─> requirements-readiness gate
        ↓ (fail closed if configured fields missing)
decisions/exceptions {id, status, owner, created/review/expires_at, supersedes} ─> governance gate (fail closed on expiry)
        ↓ (only active records enter routine context)
promoted memory {owner, last_validated_at, provenance, confidence, expires_at, supersedes} ─> context builder
        ↓ (exclude invalid/expired critical + emit finding)              │
        └────────────────────────> conflict lint (dup key / contradiction / unowned force)
        ↓
specd drift (read-only) + recurring-check contract (external CI runs; append-only result) ─> typed finding
        ↓ (later failing HEAD never overwrites last pass)
program status {risk, owner, stale governance, shared_outcome} + stable-JSON export + outcome review
        ↓ (unknown stays unknown; no LLM; byte-stable)
decommission/archive (preserve hashes, exclude retired from context)
```

## Typed successor links

`internal/core/program.go` extends `ProgramLink` with a `Kind` field, decoding pre-versioned links
(no kind) to a default ordering kind for backward compatibility. `link`/`unlink` in `internal/cmd`
accept `--kind`. Cycle detection, ordering gates, and status stay unchanged in semantics — a
`supersedes` edge records replacement without rewinding or editing the superseded spec. A guard in
the completion/reopen path fails closed on any attempt to transition a `complete` spec backward and
directs the user to `link --kind ... && new`.

```text
ProgramLinkV2
  from, to, kind ∈ {follows, regresses, maintains, supersedes}, reason, created_at
  (kind == "" on decode ⇒ "follows"/ordering default; cycle set unchanged)
```

## Typed intake provenance

A small versioned `provenance.json` per spec (`internal/core/provenance.go`) records
`source_type`/`source_ref`/`systems`/`severity`/`owner`/`prior_links` with `schema_version` and
forward-compatible decode. A new gate `internal/core/gates/intake.go` runs in the requirements
readiness set: for a configured `source_type`, missing required fields fail closed; "unknown" is a
sentinel distinct from empty and never satisfies a required field. Unconfigured feature specs are
unaffected — the gate is a no-op when no intake policy is declared.

## Decision and exception lifecycle

`internal/core/history.go` (and an exception record) gain
`id`/`status`/`owner`/`created_at`/`review_at`/`expires_at`/`supersedes`/`affected_invariants`.
Records stay append-only; supersession appends and marks the prior superseded, never deleting.
`internal/core/gates/governance.go` fails closed on an expired blocking exception or a missing/
`proposed` required decision, naming owner and review action. The context builder loads only active
(accepted, unexpired) records; history retains the full chain for `report --history`.

## Memory provenance and aging

`MemFields` grows `owner`/`last_validated_at`/`provenance`/`confidence`/`expires_at`/`supersedes`,
added backward-compatibly so existing steering files still parse (missing fields decode as unset,
not invalid). The context builder excludes an invalid/expired **critical** memory and emits a
compact finding (owner + revalidation action) rather than silent load or silent drop; stable
non-expiring constraints (no `expires_at`) are preserved. Forced promotion carries explicit
authority/provenance and is distinguishable in audit from a frequency-threshold promotion.
`internal/core/gates/memorylint.go` flags duplicate normalized keys, contradictory active critical
patterns, and unowned forced promotions **before** context construction.

## Drift and recurring invariants

`internal/cmd/drift.go` is a read-only projection: it compares declared persistent invariants and
active decisions against current code/config evidence and emits byte-stable findings carrying
source, affected path, last passing HEAD, and a suggested successor-spec command — it mutates
nothing and creates no spec. A recurring check is a deterministic command + cadence metadata
(`internal/core/recurring.go`); external CI/schedulers execute it and pass a validated result
envelope that `specd` appends under the spec lock. A later failing HEAD appends a new record and
never overwrites the last pass; failure may optionally scaffold a typed successor (opt-in,
reviewable). Drift distinguishes holds / drifted / not-evaluable / no-invariant — a missing input is
never reported as holding.

## Incident, portfolio, exports, archive

`internal/core/incident.go` + `internal/cmd/incident.go` seed a successor spec from a bounded,
redacted observation reference (source release/deployment/criterion + evidence refs) without loading
raw payloads; originals stay immutable and the link is `caused_by`/`regresses`/`maintains`. Closure
can require preventive evidence (regression test/eval) plus a "why recurrence is now caught"
reference. `internal/core/program.go` adds a portfolio view with risk/owner/stale-governance/
shared-outcome, within the documented scale envelope and never loading the whole portfolio into
context. Exports (`internal/cmd/report.go`) are stable JSON with ids/links/status/risk/evidence
refs, source/context redacted, stdlib-only; an unavailable tracker cannot corrupt state.
Outcome-review joins change evidence to release/incident adapters and keeps unknown distinct from
success. A decommission/archive policy retires specs/ledgers/memories/decisions from active context
while preserving hashes and audit refs, non-destructively and replayably.

## Verification ladder

- L0 — offline fixtures: every link-kind, intake, lifecycle, memory-aging, drift, and recurring rule
  and each of the 9 production validation scenarios validates with networking disabled; missing
  inputs fail closed (never reported as holding/passing).
- L1 — immutability: no maintenance command reopens/edits/adds a status to a `complete` spec; a
  successor traces its source and kind; cycle detection and ordering gates are unchanged.
- L2 — readiness/governance: a maintenance spec with missing configured intake fails requirements
  readiness; an expired blocking exception and a missing/`proposed` required decision fail closed
  naming the owner; feature specs with intake unconfigured are unaffected.
- L3 — memory aging: invalid/expired critical memory is excluded with a visible finding; stable
  constraints survive; forced promotion needs authority and is audit-distinguishable; conflict lint
  catches dup keys / contradictions / unowned forces before context build.
- L4 — drift/recurring append-only: `specd drift` is byte-stable and mutates nothing; a later
  failing HEAD yields a new append-only record and never overwrites the last pass; successor
  scaffolding is opt-in.
- L5 — portfolio/operate: incident seeds a bounded redacted successor with preventive evidence;
  status reports risk/owner/stale/shared-outcome within the scale envelope; exports are stable
  redacted JSON that cannot corrupt state; outcome-review keeps unknown ≠ success; archive preserves
  hashes; `report --history`, docs-lint, and command-reference/CHEATSHEET mirror all new verbs.
