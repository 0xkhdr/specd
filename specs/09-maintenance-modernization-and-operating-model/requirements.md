# Requirements — Maintenance, modernization, and the operating model

## Scope

Add a deterministic, additive maintenance domain around the existing lifecycle: typed successor
link kinds, versioned typed intake provenance with a requirements-readiness gate, decision/exception
identity and lifecycle with a governance gate, aging-aware promoted memory with context exclusion
and a conflict lint, a read-only drift projection, a recurring-invariant contract executed by
external schedulers, incident-to-successor seeding with preventive evidence, portfolio governance
status and stable-JSON exports, outcome-review reports, and a decommission/archive policy. Preserve
the forward-only six-phase ratchet, atomic writes, state CAS, reentrant spec lock, append-only
evidence, no-bypass verify, byte-stable tasks parser, offline/stdlib-only core, `go:embed`
templates, backward-compatible decoding, program cycle detection, and the bounded context manifest.
No LLM in any gate, projection, drift verdict, recurring result, or report. No new runtime
dependency. No daemon, scheduler, or network in core. No reopening of a completed spec.

### R1 — Historical preservation and typed successor links

- R1.1: Completed specs shall remain immutable. No maintenance command shall reopen, rewind, edit,
  or add a status value to a `complete` spec. Any attempt shall fail closed with an actionable
  message directing the user to create a linked successor.
- R1.2: `ProgramLink` shall carry a typed `kind` ∈ {`follows`, `regresses`, `maintains`,
  `supersedes`}, decoding older untyped links forward-compatibly to a default ordering kind. A
  successor shall be able to trace its source spec, kind, and reason from program state.
- R1.3: Typed links shall preserve existing program semantics: cycle detection still rejects a
  dependency cycle, ordering gates still block execution on an incomplete upstream, and status
  output stays deterministic. A `supersedes` link shall not mutate the superseded spec.

### R2 — Typed intake provenance

- R2.1: A versioned `provenance.json` (or equivalent requirements metadata) shall declare
  `source_type` ∈ {feature, incident, vulnerability, drift, dependency, migration, deprecation,
  policy}, `source_ref`, affected `systems`/`affected_specs`, `severity`/`risk`, `owner`, and
  prior-spec links, with `schema_version` and forward-compatible decoding.
- R2.2: A configured requirements-readiness gate shall fail a production/incident/maintenance spec
  when required intake fields for its `source_type` are missing or empty; "unknown" shall be
  distinct from empty and shall not satisfy a required field. A feature spec with intake unconfigured
  shall behave exactly as today.
- R2.3: Intake shall never be an LLM output path. The gate shall be a pure function of the recorded
  provenance file and the configured requirement set; identical inputs yield an identical verdict.

### R3 — Decision and exception lifecycle

- R3.1: Decisions and exceptions shall carry `id`, `status` ∈ {proposed, accepted, superseded,
  expired, revoked}, `owner`, `created_at`, `review_at`, `expires_at`, `supersedes`, and
  affected-invariant refs. Records shall be immutable and append-only; a supersession appends a new
  record and marks the prior one superseded without deleting it.
- R3.2: A configured governance check shall fail closed when a blocking exception is past
  `expires_at` or a required decision is missing/`proposed`, naming the owner and review action. An
  unconfigured project shall be unaffected. Agents shall not be recordable owners.
- R3.3: Only active (accepted, unexpired) decisions and exceptions shall enter routine task context.
  Superseded/expired records shall remain retrievable in history and shall show the complete
  supersession chain, but shall not consume routine context budget.

### R4 — Memory provenance and aging

- R4.1: A promoted memory entry shall carry `owner`, `last_validated_at`, `provenance`/`source_ref`,
  `confidence`/occurrence count, `expires_at`, and `supersedes`, added backward-compatibly to
  `MemFields` so existing steering files still parse.
- R4.2: The context builder shall exclude an invalid or expired critical memory and emit a visible
  finding naming the owner and required revalidation action, rather than silently loading it or
  silently dropping it. Stable non-expiring constraints shall be preserved, not erased.
- R4.3: A forced promotion (bypassing the repetition threshold) shall require explicit
  authority/provenance and shall be distinguishable in audit and reporting from a
  frequency-threshold promotion. Frequency alone shall never promote without ownership.

### R5 — Maintenance templates

- R5.1: `specd` shall supply templates for incident follow-up, dependency/deprecation work,
  migration, and recurring-invariant definition, each mapping source → requirement → task →
  evidence → learning.
- R5.2: Templates shall add no model-dependent gate and shall be inspectable (`schema`/`version`).
  Project-specific content shall survive a template refresh; a refresh shall not clobber
  user-authored sections.
- R5.3: Each template shall scaffold the typed intake fields (R2) and successor links (R1) required
  for its `source_type`, so an instantiated maintenance spec starts readiness-gate-passable.

### R6 — Drift projection

- R6.1: A read-only `specd drift` projection shall compare declared persistent invariants and active
  decisions against current code/config evidence and emit findings. It shall perform no mutation and
  no network call.
- R6.2: Output shall be byte-stable for identical inputs. Each finding shall include the source
  invariant/decision, the affected path, the last passing HEAD, and a suggested successor-spec
  command; it shall never auto-create a spec.
- R6.3: Drift shall distinguish "invariant holds", "invariant drifted", "invariant not evaluable"
  (missing input), and "no declared invariant"; a missing input shall never be reported as holding.

### R7 — Recurring invariants

- R7.1: A recurring check shall be defined as a deterministic command plus cadence/trigger metadata,
  executed by external CI/schedulers. `specd` shall validate the definition and record a result but
  shall not schedule, poll, or become a daemon.
- R7.2: Each recorded result shall pin the check id, HEAD/release/config identity, and verdict,
  appended under the spec lock. A later failing HEAD shall create a new append-only record and shall
  not overwrite or invalidate the last passing evidence, which stays auditable.
- R7.3: A recurring-check failure shall optionally scaffold a typed successor spec (R1, R2) but shall
  never silently mutate a completed record; successor creation shall be opt-in and reviewable.

### R8 — Incident-to-successor and prevention

- R8.1: An incident/observation reference shall seed a new spec recording source
  release/deployment/criterion and bounded, redacted evidence refs. The raw external payload shall
  not be loaded into context by default; original ledgers shall stay immutable.
- R8.2: Incident closure shall be able to require preventive evidence — a regression test/eval — and
  an explicit "why recurrence is now caught" reference bound to the successor's evidence.
- R8.3: An incident successor shall link to the original spec via `caused_by`/`regresses`/`maintains`
  without editing the original; the original's history and evidence shall be unchanged.

### R9 — Portfolio governance status

- R9.1: Program status shall add per-spec/edge `risk`, `owner`, stale-or-expired governance items,
  unresolved production signals, and shared-outcome/integration criteria, remaining deterministic.
- R9.2: Status and drift shall complete within the documented large-program scale envelope, and
  bounded context shall never load the whole portfolio — task agents receive affected dependencies
  and owners only.
- R9.3: Cross-spec ordering satisfaction shall not imply a shared outcome; a shared-outcome criterion
  shall be a distinct, separately-evidenced record so a satisfied DAG with a failed shared release is
  visibly incomplete.

### R10 — Portfolio exports and outcome review

- R10.1: Portfolio exports shall be stable JSON with ids/links/status/risk/evidence references,
  redacting source/context content by default, using only stdlib serialization — no network SDK. An
  export failure or unavailable external tracker shall not corrupt spec state or block local
  operations.
- R10.2: Outcome-review reports shall join change evidence to release/incident feedback adapters.
  Unknown or missing outcome data shall remain "unknown" and shall never be interpreted as success,
  zero, or pass.
- R10.3: Reports shall not rank individuals or agents by shallow throughput/token measures; operating
  health surfaces (throughput, retries, failures, exception age, context/eval quality) inform human
  process decisions and are byte-stable across identical inputs.

### R11 — Decommission and archive

- R11.1: A decommission/archive policy shall retire specs, ledgers, memories, and decisions from
  active context while preserving their content hashes and audit references; archived material shall
  stay retrievable but excluded from routine context construction.
- R11.2: Archival shall be non-destructive to provenance — it shall supersede/relocate, never rewrite
  or delete audit identity — and shall be deterministic and replayable.
- R11.3: Retention/index/export policy shall bound immutable-history growth before portfolio ledgers
  become large, with a documented scale envelope for status/drift/export.

## Non-goals

- No issue tracker, scheduler, incident-management platform, CMDB, or workforce-management system in
  core — optional templates and stable-JSON exports only.
- No reopening, rewinding, or editing of a completed spec; maintenance is always a linked successor.
- No LLM in any gate, drift verdict, recurring result, projection, or report; no autonomous
  successor-spec creation without human review.
- No daemon, scheduler, polling loop, or network call in core; external CI/schedulers trigger
  recurring checks and pass validated envelopes.
- No memory-freshness rule that destructively erases stable constraints; supersede and archive
  rather than rewrite provenance.
- No ranking of individuals/agents by shallow metrics; no raw incident payloads in default context
  or state — bounded facts and redacted references only.
- No new lifecycle status value; maintenance state lives in provenance/decision/memory records and
  typed links, never on the six lifecycle status values.
- No new runtime dependency. No `reference/` edits.
