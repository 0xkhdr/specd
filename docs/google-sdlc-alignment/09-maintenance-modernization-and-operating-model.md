# Domain 09 — Maintenance, Modernization, and the Operating Model

## Purpose

Extend the value of structured agentic engineering beyond a feature spec reaching `complete`: recurring maintenance, incidents, migrations, architecture drift, durable learning, portfolio coordination, team ownership, and safe human–agent operating practices.

## Paper position

The comparison document says the paper treats maintenance and organizational change as part of the new SDLC, including legacy navigation, persistent knowledge, governance, production refinement, team norms, on-call work, and hybrid human/agent roles. This expands the unit of work from “generate a change” to “operate a software-producing system over time.”

The paper does not imply that every organizational practice belongs inside a CLI. The best alignment is to make `specd` produce enforceable technical records and portable templates while leaving staffing, accountability, and business decisions to people and existing systems.

## Current `specd` handling

### Existing reusable foundations

- `specd memory` and `internal/core/memory.go` capture structured patterns per spec and deterministically promote repeated learning into shared steering memory.
- `specd decision` and `specd midreq` append human rationale and mid-stream requirement changes to state records; `internal/cmd/report.go` projects them into history.
- `specd link`, `specd unlink`, `internal/core/program.go`, and program status model cross-spec ordering and block an execution transition when upstream work is incomplete.
- Evidence, criteria, review, submission, and ACP ledgers provide raw material for longitudinal audit.
- `report --history`, metrics, Prometheus exposition, and PR summaries expose state without an LLM report path.
- Managed steering files preserve technical, product, structural, workflow, and reasoning conventions across conversations.
- `docs/versioning-policy.md`, the release workflow, regression scripts, and stress tests give the `specd` project itself a maintenance discipline that can inform managed-project checks.

### Current scope limitations

- A spec is optimized for forward progression to completion. There is no first-class recurring/continuous invariant, scheduled audit, or “still true at HEAD” lifecycle.
- Memory promotion is based on repeated keys and an optional force flag; there is no supersession, expiry, owner, validation date, contradiction detection, or rollback for stale guidance.
- Program links express ordering, not portfolio goals, capacity, risk, ownership, release trains, or shared production outcomes.
- Incidents, vulnerabilities, dependency updates, deprecations, architecture debt, and migrations have no typed intake or provenance linking them to requirements/tasks/evidence.
- Decisions have narrative scope but no status such as proposed/accepted/superseded, review date, affected invariants, or decision dependency graph.
- Reports summarize a spec; they do not continuously detect code/spec/architecture/policy drift after completion.
- Team policies, responsibility assignment, review service levels, on-call handoff, security exception expiry, and generated-code ownership are not scaffolded as a coherent operating pack.
- There is no deterministic rule for when production feedback should reopen work. The forward-only ratchet is correct for historical integrity, so maintenance needs a new linked spec rather than rewinding the old one.

## Common contract and fields

| Field | Paper-side purpose | Current `specd` support | Target meaning |
|---|---|---|---|
| `source_type` / `source_ref` | Why work exists | free-text memory/decision source | Typed feature, incident, vulnerability, drift, dependency, migration, deprecation, or policy source with external reference. |
| `owner` / `approvers` | Human accountability | mostly external | Named role/team ownership; agents cannot be accountable owners. |
| `created_at` / `review_at` / `expires_at` | Lifecycle over time | timestamps in records | Time-bound decisions, exceptions, memories, and recurring checks. |
| `status` | Current disposition | spec/task lifecycle | Add maintenance item and decision states without rewriting historical specs. |
| `risk` / `criticality` | Prioritization | memory criticality | Common severity vocabulary tied to required evidence and approval. |
| `invariants` / `criteria` | What must remain true | requirements/criteria | Reusable, versioned assertions that can be re-evaluated at later HEADs. |
| `affected_specs` / `systems` | Portfolio scope | program links | Explicit impacted services/specs/components and dependency direction. |
| `decision_ref` / `supersedes` | Architecture continuity | narrative decisions | Immutable decision identity and replacement chain. |
| `learning_key` / `provenance` / `confidence` | Durable knowledge | memory key/source | Validated source, occurrence count, owner, last confirmation, and supersession. |
| `cadence` / `trigger` | Recurring maintenance | absent | External scheduler trigger plus deterministic local check contract. |
| `evidence_refs` / `head` / `release` | Proof over time | HEAD-pinned task evidence | Each re-check pins code/release/config identity and result. |
| `exception_ref` / `expiry` | Governed deviation | decisions/override reason | Time-limited, approved exception that cannot silently become permanent policy. |
| `outcome` / `followups` | Feedback loop | report history, memory | Measured result and linked next specs/tasks, not an LLM-only summary. |

## Gaps and failure modes

- A completed migration silently regresses months later because evidence was valid only at the completion HEAD and no recurring invariant runs.
- An incident creates an ad hoc patch but never updates requirements, tests, steering, or a preventive invariant.
- Promoted memory becomes obsolete and continues entering context, misleading agents while consuming budget.
- A forced memory promotion bypasses the repetition threshold without an auditable approval/expiry distinction.
- Teams rewind or edit a completed spec to represent maintenance, destroying the historical ratchet instead of creating a linked successor.
- Cross-spec ordering is satisfied, yet a shared release fails because no portfolio outcome or integration evidence joins the specs.
- Generated changes have no durable human owner for review, operation, rollback, or future modification.
- Security and architecture exceptions remain as prose decisions with no expiration or revalidation.

## Target best-practice workflow

1. **Typed intake:** production signal, incident, dependency event, drift finding, or modernization goal creates a new spec linked to the source and affected prior specs.
2. **Historical preservation:** completed specs remain immutable records. Follow-up work uses `supersedes`, `caused_by`, `regresses`, or `maintains` links rather than phase rollback.
3. **Risk-based planning:** intake severity determines required owner, reviewer, eval/security/deployment evidence, and response target.
4. **Execute and prove:** normal `specd` task scope, context, role, verify, eval, review, and approval semantics apply.
5. **Learn:** reflect produces candidate memories, updated invariants, and decision supersession proposals. Promotion requires provenance and ownership, not frequency alone.
6. **Revalidate:** external CI/schedulers run declared recurring checks at a new HEAD/release; failure creates a typed finding and optional successor-spec scaffold, never silently mutates a completed record.
7. **Govern portfolio:** program views show dependencies, shared outcomes, risk, ownership, stale decisions/memory, and unresolved production signals.
8. **Review operating health:** teams use deterministic reports for throughput, retries, failures, exception age, context/eval quality, and production outcomes; people decide process changes.

## Recommended action plan

### P0 — Preserve history and establish maintenance provenance

1. Document and enforce “never reopen a completed spec; create a linked successor.” Add link kinds such as `follows`, `regresses`, `maintains`, and `supersedes` in `internal/core/program.go`/link commands. **Acceptance:** completed state is unchanged; a successor can trace its source and program gates still detect cycles.
2. Define typed intake metadata in requirements or a small versioned `provenance.json`: source type/ref, affected system, severity, owner, and prior-spec links. **Acceptance:** production/incident/maintenance specs cannot pass requirements readiness when configured fields are missing.
3. Add identity and lifecycle fields to decisions and exceptions: id, status, owner, created/review/expiry dates, supersedes. **Acceptance:** expired blocking exceptions fail a configured governance check; history retains superseded records.
4. Add owner, last-validated date, provenance refs, and supersession to promoted memory. **Acceptance:** invalid/expired critical memory is excluded from task context with a visible finding, not silently loaded.
5. Supply templates for incident follow-up, dependency/deprecation work, migration, and recurring invariant definitions. **Acceptance:** each template maps source → requirement → task → evidence → learning without adding a model-dependent gate.

### P1 — Add drift and recurring-invariant workflows

1. Introduce a read-only `specd drift` projection comparing declared persistent invariants/decisions with current code/config evidence. **Acceptance:** identical inputs are byte-stable; findings include source, affected path, last passing HEAD, and suggested successor-spec command.
2. Define recurring checks as deterministic commands plus cadence metadata executed by external CI/schedulers. `specd` validates and records results but does not become a daemon. **Acceptance:** a later failing HEAD cannot overwrite the last pass and yields a new append-only record.
3. Add incident-to-successor linking and preventive-evidence fields. **Acceptance:** closure can require a regression test/eval and an explicit “why recurrence is now caught” reference.
4. Extend program status with risk, owner, stale/expired governance items, and shared outcome criteria. **Acceptance:** output remains deterministic and large-program performance stays within the documented scale envelope.
5. Add memory conflict/supersession lint. **Acceptance:** duplicate normalized keys, contradictory active critical patterns, and unowned forced promotions are findings before context construction.

### P2 — Support organizational adoption without owning the organization

1. Scaffold optional team policy templates: human approval ownership, generated-code review, security exception, production readiness, on-call handoff, and agent incident response. **Acceptance:** template schema/version is inspectable and project-specific content survives refresh.
2. Add portfolio exports for external dashboards/work trackers using stable JSON, not network SDKs. **Acceptance:** exports contain ids/links/status/risk/evidence references and redact source/context content by default.
3. Add outcome-review reports joining change evidence to release/incident feedback adapters. **Acceptance:** unknown/missing outcome data remains unknown rather than being interpreted as success.
4. Define decommission/archive policy for specs, ledgers, memories, and decisions. **Acceptance:** archival preserves hashes and audit references while active context excludes retired material.

## Production validation scenarios

| Scenario | Expected result |
|---|---|
| Incident caused by a completed change | New successor spec links incident and original spec; original history is unchanged. |
| Recurring invariant fails at a later HEAD | New failing record and drift finding appear; old passing evidence remains auditable. |
| Critical memory expires | Context builder omits or flags it according to policy and names the owner/review action. |
| Decision is superseded | Only the active decision enters context; history shows the complete chain. |
| Forced promotion | Requires explicit authority/provenance and is distinguishable in audit/reporting. |
| Cross-spec modernization | Dependency DAG, shared integration outcome, owners, and release evidence are visible. |
| Security exception reaches expiry | Configured gate fails closed until renewed by an authorized human or removed. |
| External tracker unavailable | Local operations and reports still work; export retry cannot corrupt spec state. |
| Large portfolio | Deterministic status/drift completes within documented scale limits and bounded context never loads the whole portfolio. |

## Context-safety considerations

- Load only active, applicable decisions and validated memory for the current task; keep superseded/expired records in history, not routine context.
- Recurring-check results should enter context as compact findings and evidence references, not raw logs.
- Portfolio summaries should be projections; task agents need affected dependencies and owners, not every spec's prose.
- Incident data may contain secrets or customer information. Store redacted references and hashes; keep sensitive source material outside `.specd` unless policy explicitly protects it.
- A candidate learning should not become constitutional context merely because an agent wrote it. Promotion remains a governed deterministic workflow.

## Non-goals and risks

- `specd` is not an issue tracker, scheduler, incident-management platform, CMDB, or workforce-management system.
- Organizational metrics can incentivize harmful behavior. Avoid ranking individuals or agents by shallow throughput/token measures.
- Immutable history can grow; retention/index/export policy is needed before portfolio ledgers become large.
- Automated successor-spec creation should be optional and reviewable; production signals can be noisy.
- Memory freshness rules must not erase useful stable constraints. Supersede and archive rather than destructively rewrite provenance.
