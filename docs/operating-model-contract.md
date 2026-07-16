# specd — Operating-model contract (draft)

Maintenance is a **parallel, additive record domain**. It never adds a value to the six lifecycle
status states, never reopens a completed spec, and never satisfies a task evidence gate. Every
follow-up is a **new spec** joined to its source by a typed link. Everything below is a pure
function of on-disk `.specd/` records plus validated external envelopes — no LLM in any gate,
projection, drift verdict, recurring result, or report; no daemon, scheduler, or network in core.
This document drafts the record fields the Domain 09 waves (`09b`–`09l`) implement; the canonical
offline fixtures live under `testdata/maintenance/` (see
`internal/core/maintenance_fixtures_test.go`).

Status: **draft** (09a baseline). Field names here are the contract the later waves pin.

## Records

Every record is versioned (a `schema` or `schema_version` field) so an older binary rejects an
unknown schema before mutation rather than silently reinterpreting it. A missing field on an older
on-disk record decodes as unset (back-compat), never as invalid.

### ProgramLinkV2 → `program.json` (R1)

`ProgramLink` gains a typed `kind`; pre-versioned untyped links decode forward to the default
ordering kind (`follows`) and keep existing cycle-detection and ordering semantics. A `supersedes`
edge records replacement **without** mutating, reopening, or editing the superseded spec.

| field | meaning |
|---|---|
| `from`, `to` | `from` depends on / succeeds `to`; `to` must reach completion before `from` executes |
| `kind` | one of `follows`, `regresses`, `maintains`, `supersedes`; absent on decode ⇒ `follows` |
| `reason` | why the successor exists (bounded free text) |
| `created_at` | RFC3339 timestamp |

A `supersedes` link is the only sanctioned "reopen": the completed spec stays immutable and a new
spec carries the work forward. A reopen/rewind/edit attempt on a `complete` spec fails closed with
a message directing the user to create a linked successor (`link --kind ... && new`).

### ProvenanceV1 → `provenance.json` (R2)

Versioned typed intake recorded per spec. The requirements-readiness gate fails a configured
`source_type` when a required field is missing or empty; `unknown` is a sentinel distinct from
empty and never satisfies a required field. A feature spec with intake unconfigured behaves exactly
as today.

| field | meaning |
|---|---|
| `schema_version` | forward-compatible decode version |
| `source_type` | one of `feature`, `incident`, `vulnerability`, `drift`, `dependency`, `migration`, `deprecation`, `policy` |
| `source_ref` | bounded reference to the originating signal (never the raw payload) |
| `systems`, `affected_specs` | impacted systems and prior specs |
| `severity`, `risk` | severity / risk classification |
| `owner` | recordable human owner (an agent is never a recordable owner) |
| `prior_links` | typed links to source specs (see ProgramLinkV2) |

### DecisionV1 → decision/exception records (R3)

Decisions and exceptions gain identity, ownership, and a time-bound lifecycle. Records are
immutable and append-only; a supersession appends a new record and marks the prior one `superseded`
without deleting it. Only active (accepted, unexpired) records enter routine task context;
superseded/expired records stay retrievable in history with the full supersession chain.

| field | meaning |
|---|---|
| `id` | stable record identity |
| `status` | one of `proposed`, `accepted`, `superseded`, `expired`, `revoked` |
| `owner` | recordable human owner (an agent is never a recordable owner) |
| `created_at`, `review_at`, `expires_at` | lifecycle timestamps (RFC3339) |
| `supersedes` | id of the record this one replaces (empty for an original) |
| `affected_invariants` | refs to the persistent invariants this decision/exception governs |

The governance gate fails closed when a blocking exception is past `expires_at` or a required
decision is missing / still `proposed`, naming the owner and review action. An unconfigured project
is unaffected.

### MemoryEntryV1 → promoted memory (R4)

`MemFields` grows aging fields, added backward-compatibly so existing steering files still parse.
The context builder excludes an invalid or expired **critical** memory and emits a visible finding
naming the owner and required revalidation action, rather than silently loading or silently
dropping it. Stable non-expiring constraints (no `expires_at`) are preserved. A forced promotion
carries explicit authority/provenance and is audit-distinguishable from a frequency-threshold
promotion; frequency alone never promotes without ownership.

| field | meaning |
|---|---|
| `key`, `pattern`, `detail` | existing memory identity/content (unchanged) |
| `owner` | recordable human owner |
| `last_validated_at` | when the entry was last confirmed still true (RFC3339) |
| `provenance`, `source_ref` | where the entry came from |
| `confidence` | occurrence count / confidence signal |
| `expires_at` | validity horizon; absent ⇒ a stable non-expiring constraint |
| `supersedes` | id/key of the memory this entry replaces |

## Additivity invariant

The evidence gate and `complete` behave **identically** whether these records are present or
absent. No maintenance record retroactively changes a task's `complete`, reopens a spec, or adds a
seventh lifecycle status. Cycle detection and ordering gates keep their current semantics under
typed links. Drift and recurring-invariant checks are read-only projections plus an
external-scheduler contract — `specd` validates and records, it never schedules, polls, or becomes
a daemon.

## Fixture plan — 9 validation scenarios

Each validation scenario gets a deterministic offline fixture; the four canonical record
fixtures below are landed, and each remaining scenario fixture lands with the rule it
exercises.

| # | scenario | pinned by | landing wave |
|---|---|---|---|
| S1 | Incident caused by a completed change | `program_link.json` (`caused_by`/`regresses`) | 09i (T36/T38) |
| S2 | Recurring invariant fails at a later HEAD | recurring result fixture | 09g/09h (T33/T50) |
| S3 | Critical memory expires | `memory_entry.json` | 09e (T21) |
| S4 | Decision is superseded | `decision.json` | 09d (T15/T18) |
| S5 | Forced promotion | `memory_entry.json` (forced variant) | 09e (T23) |
| S6 | Cross-spec modernization | portfolio status fixture | 09j (T40/T41) |
| S7 | Security exception reaches expiry | `decision.json` (exception variant) | 09d (T17) |
| S8 | External tracker unavailable | export fixture | 09k (T46) |
| S9 | Large portfolio | portfolio scale fixture | 09j (T42) |

The four canonical record fixtures landed by 09a — `program_link.json`, `provenance.json`,
`decision.json`, `memory_entry.json` — pin the field definitions above so the later waves extend
rather than redefine them.
