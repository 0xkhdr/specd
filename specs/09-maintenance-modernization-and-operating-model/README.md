# Domain 09 — Maintenance, modernization, and the operating model

## Goal

Extend a `specd`-managed system's value **past `complete`** — recurring maintenance, incidents,
dependency/deprecation work, migrations, architecture drift, durable learning, portfolio
coordination, and human/agent operating norms — **without ever reopening a completed spec or
putting an LLM in a gate**. Maintenance is an additive, forward-only domain: production signals,
drift findings, and recurring-invariant failures create **linked successor specs**, never phase
rollbacks. `specd` owns deterministic technical records — typed intake provenance, versioned
decisions/exceptions with lifecycle, aging-aware memory, drift projections, recurring-check
contracts, and portfolio governance views — and leaves staffing, accountability, scheduling, and
business decisions to people and external systems. Everything is a pure function of on-disk
`.specd/` state plus validated external envelopes. No daemon, no scheduler, no network in core.

## Source and intent

Derived from `docs/google-sdlc-alignment/README.md` and
`docs/google-sdlc-alignment/09-maintenance-modernization-and-operating-model.md`.
Paper position: the SDLC does not end at a merged change — legacy navigation, persistent
knowledge, governance, production refinement, team norms, on-call work, and hybrid human/agent
roles are all part of operating a software-producing system over time. The unit of work grows from
"generate a change" to "operate a software-producing system." The paper does **not** imply every
organizational practice belongs in a CLI; the best alignment is enforceable technical records and
portable templates, with people owning accountability.

Current state (reusable foundations): `specd memory` / `internal/core/memory.go` promote repeated
learning into shared steering; `specd decision` / `specd midreq` append rationale and mid-stream
requirement changes projected by `internal/cmd/report.go`; `specd link`/`unlink` and
`internal/core/program.go` model cross-spec ordering and block execution on incomplete upstreams;
evidence/criteria/review/submission/ACP ledgers give longitudinal audit material; `report
--history`, metrics, Prometheus exposition, and PR summaries expose state with no LLM report path;
managed steering preserves conventions across conversations.

Gaps: a spec is optimized for forward progression only — no recurring/continuous invariant, no
scheduled audit, no "still true at HEAD" lifecycle. `ProgramLink{From,To}` is untyped ordering with
no `kind`, portfolio goal, risk, owner, or shared outcome. `MemFields` has key/pattern/detail/
source/criticality/related but no owner, validation date, provenance, expiry, or supersession, so
stale guidance keeps entering context. Decisions are narrative with no id/status/review/expiry/
supersession chain. Incidents, vulnerabilities, dependency updates, deprecations, architecture debt,
and migrations have no typed intake or provenance. Reports summarize a spec but never detect
code/spec/architecture drift after completion. There is no deterministic rule for when production
feedback should reopen work, and no archive/decommission policy for aging ledgers.

## Ownership

| Area | Domain 09 owns | Other domain owns |
|---|---|---|
| Successor links | typed `follows`/`regresses`/`maintains`/`supersedes` link kinds in `program.go`, cycle-safe, completed-state-immutable | Domain 01 spec revision/approval + forward-only ratchet; Domain 05 program controller |
| Typed intake | versioned `provenance.json` (source type/ref, system, severity, owner, prior-spec links) + requirements-readiness gate | Domain 06 authority/owner trust; Domain 01 requirements phase gate |
| Decision/exception lifecycle | id/status/owner/created/review/expiry/supersedes on decisions and exceptions + governance gate that fails closed on expiry | Domain 06 security-exception policy; Domain 01 decision record append |
| Memory aging | owner/last-validated/provenance/confidence/supersedes on promoted memory + context exclusion + conflict lint | Domain 02 context manifest construction/budget |
| Drift projection | read-only `specd drift` comparing declared persistent invariants/decisions against current evidence | Domain 04 evidence class/freshness; Domain 07 measurement trust |
| Recurring invariants | deterministic check command + cadence metadata contract; validate/record only, never a daemon | external CI/schedulers execute; Domain 08 release/env identity |
| Portfolio governance | program status with risk/owner/stale governance/shared-outcome + stable JSON exports | Domain 05 mission/lease; Domain 10 export transport |
| Incident lifecycle | bounded incident→successor seeding + preventive-evidence fields | Domain 08 release/deployment observation refs; Domain 07 observation typing |
| Archive/decommission | retention/archive policy preserving hashes/audit refs while excluding retired material from context | Domain 10 data-boundary/redaction policy |

## Deliverable specs

| Wave | Slug | Result | Requires |
|---|---|---|---|
| W0 | `09a-maintenance-baseline` | observed behavior, corrected doc wording, operating-model contract drafts, RED fixtures for every P0 gap and each of the 9 validation scenarios | — |
| W1 | `09b-successor-link-kinds` | typed `follows`/`regresses`/`maintains`/`supersedes` link kinds; completed state unchanged; successor traces its source; cycle detection preserved | 09a |
| W2 | `09c-typed-intake-provenance` | versioned `provenance.json` (source/system/severity/owner/prior links); maintenance specs fail requirements readiness when configured fields are missing | 09a |
| W3 | `09d-decision-exception-lifecycle` | decision/exception id/status/owner/created/review/expiry/supersedes; expired blocking exception fails a governance check; history retains superseded records | 09a, Domain 06 authority |
| W4 | `09e-memory-provenance-and-aging` | owner/last-validated/provenance/supersedes on promoted memory; invalid/expired critical memory excluded from context with a visible finding | 09a, Domain 02 context |
| W5 | `09f-maintenance-templates` | incident/dependency/deprecation/migration/recurring-invariant templates mapping source→requirement→task→evidence→learning, no model-dependent gate | 09b,09c |
| W6 | `09g-drift-projection` | read-only `specd drift`; byte-stable output; findings carry source, affected path, last passing HEAD, and a suggested successor-spec command | 09c,09d, Domain 04 evidence |
| W7 | `09h-recurring-invariants` | recurring check = deterministic command + cadence metadata run by external CI; a later failing HEAD cannot overwrite the last pass and yields an append-only record | 09c, Domain 07 measurement |
| W8 | `09i-incident-successor-and-prevention` | incident→successor linking + preventive-evidence fields; closure can require a regression test/eval and a "why recurrence is now caught" reference | 09b,09f, Domain 08 observation |
| W9 | `09j-portfolio-governance-status` | program status adds risk/owner/stale-or-expired governance/shared-outcome criteria; deterministic and within the documented scale envelope | 09b,09d |
| W10 | `09k-memory-conflict-lint` | duplicate normalized keys, contradictory active critical patterns, and unowned forced promotions are findings before context construction | 09e |
| W11 | `09l-org-adoption-and-archive` | optional team-policy templates, stable-JSON portfolio exports, outcome-review reports (unknown stays unknown), and a decommission/archive policy | 09i,09j,09k, Domain 10 boundary |

## DAG

```text
09a ─┬─> 09b ─┬─────────────────────> 09f ─┬─> 09i ─┐
     │        ├─> 09j ─┐                    │        │
     ├─> 09c ─┼─> 09f ─┘                    │        ├─> 09l
     │        ├─> 09g ─┐                    │        │
     │        └─> 09h ─┼────────────────────┘        │
     ├─> 09d ─┬─> 09g ─┘                             │
     │        └─> 09j ────────────────────────────────┤
     └─> 09e ─────────> 09k ──────────────────────────┘

Domain 06 authority     ─> 09d
Domain 02 context       ─> 09e
Domain 04 evidence      ─> 09g
Domain 07 measurement   ─> 09h
Domain 08 observation   ─> 09i
Domain 10 boundary      ─> 09l
```

## Program rules

1. **Forward-only ratchet is inviolable.** No maintenance workflow reopens, rewinds, edits, or
   adds a status value to a completed spec. Follow-up work is always a new spec joined by a typed
   `follows`/`regresses`/`maintains`/`supersedes` link. A completed record is immutable history.
2. **No LLM, network call, or agent prose in any gate, projection, drift verdict, recurring-check
   result, or report.** Everything is a pure function of `.specd/` records plus validated external
   envelopes. `specd` is not a daemon or scheduler; external CI/schedulers trigger recurring checks.
3. **Task-completion authority is untouched.** Drift findings, recurring-check results, incident
   records, and portfolio state never substitute for a passing verify record at a git HEAD, and no
   maintenance record retroactively changes `complete`.
4. **Agents cannot be accountable owners.** Every governed record (intake, decision, exception,
   promoted memory) carries a human/team owner; an agent-authored candidate is not constitutional
   context until a deterministic promotion workflow validates provenance and ownership — frequency
   alone never promotes.
5. **Lifecycle is time-bound and fails closed.** Decisions, exceptions, memories, and recurring
   checks carry created/review/expiry dates. An expired blocking exception or invalid critical
   memory fails a configured governance/context check; it never silently persists or loads.
6. **Only active records enter routine context.** Superseded/expired decisions and memory stay in
   history; drift/recurring results enter as compact findings and evidence references, never raw
   logs. Portfolio summaries are projections — task agents get affected dependencies and owners,
   not every spec's prose.
7. **Revalidation is append-only.** A later failing HEAD produces a new record and drift finding;
   it can never overwrite or invalidate the last passing evidence, which stays auditable.
8. **Incident data is bounded and redacted.** Store redacted references and hashes, not raw
   payloads that may hold secrets or customer data; sensitive source material stays outside
   `.specd/` unless policy explicitly protects it.
9. **`specd` is not the organization.** It is not an issue tracker, scheduler, incident-management
   platform, CMDB, or workforce system. Team policies are inspectable templates whose
   project-specific content survives refresh; exports are stable JSON, not network SDKs; and no
   report ranks individuals or agents by shallow throughput/token measures.
10. **Stdlib-only, offline, deterministic core, subtractive bias.** Zero new runtime dependencies;
    no `reference/` edits. Maintenance verbs/flags are declared once in
    `internal/core/commands.go`, derived into MCP/help, and mirrored in `docs/command-reference.md`
    and `docs/CHEATSHEET.md`. When unsure, defer or cut and record the decision; supersede and
    archive rather than destructively rewrite provenance.

## Completion claim

The domain is complete when: (1) completed specs are provably immutable and every maintenance
follow-up is a linked successor via typed `follows`/`regresses`/`maintains`/`supersedes` links with
cycle detection intact; (2) production/incident/maintenance specs carry versioned typed intake
provenance and cannot pass requirements readiness with configured fields missing; (3) decisions and
exceptions carry id/status/owner/created/review/expiry/supersedes, an expired blocking exception
fails closed, and history retains the full superseded chain; (4) promoted memory carries
owner/last-validated/provenance/supersession, invalid/expired critical memory is excluded from
context with a visible finding, and a conflict lint catches duplicate keys, contradictory critical
patterns, and unowned forced promotions; (5) a read-only byte-stable `specd drift` projection and a
recurring-check contract exist, external CI runs them at later HEADs, and a failing HEAD yields an
append-only finding that never overwrites the last pass; (6) incidents seed bounded successor specs
with preventive-evidence fields, program status reports risk/owner/stale governance/shared outcomes
within the documented scale envelope, portfolio exports are stable redacted JSON, outcome-review
keeps unknown distinct from success, and an archive policy preserves hashes while excluding retired
material; and every one of the 9 production validation scenarios in
`09-maintenance-modernization-and-operating-model.md` has a deterministic offline fixture.
