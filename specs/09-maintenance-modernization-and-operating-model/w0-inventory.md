# W0 T01 — Current-state inventory: link / memory / decision / report vs Domain 09 P0 gaps

Read-only scout inventory of the CURRENT behavior in `internal/core/program.go`,
`internal/core/memory.go`, `internal/core/history.go` (Record store), and
`internal/cmd/report.go`, mapped against every P0 gap named in the domain doc
(`docs/google-sdlc-alignment/09-*.md` §"Recommended action plan → P0") and
requirements R1–R11. Line refs are current-tree; "absent" means no code surface exists yet.

## P0 gap inventory

| Behavior / Requirement | Current code surface (file:line) | P0 gap |
|---|---|---|
| Completed-spec immutability / reopen guard (P0-1, R1.1) | No guard exists. Completion path lives in `internal/cmd/lifecycle.go`; `specComplete` check at `internal/cmd/link.go:16`; grep for reopen/rewind guard returns nothing in `internal/cmd` or `internal/core`. | Nothing blocks reopening/rewinding/editing a `complete` spec; no actionable "create a linked successor" message. Needs a fail-closed guard on the completion/reopen transition. |
| Typed successor link kinds (P0-1, R1.2) | `ProgramLink` is `{From, To}` only — `internal/core/program.go:19-22`; `AddLink` appends untyped edges — `program.go:83-87`. | No `Kind` field (`follows`/`regresses`/`maintains`/`supersedes`), no reason/created_at, no back-compat kind default on decode. A successor cannot record or trace its source kind/reason. |
| Cycle detection + ordering preserved under kinds (R1.3) | `WouldCycle` — `program.go:116-146`; `Deps`/`Frontier`/`IncompleteDeps` — `program.go:100-176`. | Semantics are correct today but are kind-agnostic; a `supersedes` edge must record replacement without mutating the superseded spec and without breaking cycle/ordering. No kind-aware handling exists. |
| Typed intake provenance + readiness gate (P0-2, R2.1-R2.3) | Absent. No `provenance.json`, no `internal/core/provenance.go`, no `internal/core/gates/intake.go`. Requirements readiness gate set has no intake member. | No typed `source_type`/`source_ref`/`systems`/`severity`/`owner`/`prior_links`; nothing fails readiness on missing configured intake fields; no `unknown`≠empty sentinel. |
| Decision identity + lifecycle (P0-3, R3.1) | `Record` struct = `{Kind, Text, Scope, Gate, ApprovedRevision, Timestamp, GitHead, Actor}` — `internal/core/state.go:25-34`; decisions stamped append-only via `StampRecord` — `state.go:39-44`; projected at `report.go:42-50`. | Decisions have no `id`, `status` {proposed/accepted/superseded/expired/revoked}, `owner`, `review_at`, `expires_at`, `supersedes`, or affected-invariant refs. No supersession chain. |
| Exception lifecycle + governance gate (P0-3, R3.2) | Absent. No `internal/core/exception.go`, no `internal/core/gates/governance.go`. | No time-bound governed exception; nothing fails closed on an expired blocking exception or missing/`proposed` required decision; no owner naming; agents not barred as recordable owners. |
| Active-only decision context (R3.3) | Context builder (`internal/context`) has no decision-status filter; `report.go` history renders all records. | Superseded/expired records would enter routine context and consume budget; no active-only load. |
| Memory provenance / aging fields (P0-4, R4.1) | `MemFields` = `{Key, Pattern, Detail, Source, Criticality, Related}` — `internal/core/memory.go:14-21`; render at `memory.go:26-29`. | No `owner`, `last_validated_at`, `provenance`/`source_ref`, `confidence`/occurrence, `expires_at`, `supersedes`. No back-compat decode for aging fields. |
| Expired/critical memory exclusion + finding (R4.2) | Promotion is pure repeat-count: `CountSpecsWithBlock` — `memory.go:76-85`; context builder loads memory blocks with no validity/expiry check. | Invalid/expired critical memory is silently loaded; no visible finding naming owner + revalidation action; no protection for stable non-expiring constraints. |
| Forced-promotion authority / audit distinction (P0-4, R4.3) | `--force` bypasses threshold with no extra record — `internal/cmd/memory.go:75,87-88`; `RenderPromotion` writes only `Promoted: from spec ... (seen in N)` — `memory.go:90-92`. | Forced promotion is indistinguishable in audit from a frequency promotion; carries no authority/provenance; frequency alone can promote without ownership. |
| Memory conflict/supersession lint (R4.2) | Absent. No `internal/core/gates/memorylint.go`. | Duplicate normalized keys, contradictory active critical patterns, and unowned forced promotions are not detected before context construction. |
| Maintenance templates (P0-5, R5.1-R5.3) | Absent. `internal/core/embed_templates/` has roles/steering only; no `maintenance/{incident,dependency,migration,recurring}.md`. | No source→req→task→evidence→learning templates; no scaffold of intake + successor links; refresh-preservation not exercised for maintenance packs. |
| Drift projection (R6) | Absent. No `internal/core/drift.go`, no `internal/cmd/drift.go`. Reports summarize a spec only (`report.go:18-147`); none compare invariants/decisions vs current evidence. | No read-only drift verdict (holds/drifted/not-evaluable/none); no byte-stable finding with source/path/last-passing-HEAD/suggested successor command. |
| Recurring invariants (R7) | Absent. No `internal/core/recurring.go`. Evidence is HEAD-pinned only at completion (`report.go:55-95`). | No recurring check = command+cadence contract; no append-only later-HEAD result that preserves last pass; no external-scheduler envelope; no opt-in successor scaffold. |
| Incident-to-successor + prevention (R8) | Absent. No `internal/core/incident.go` / `internal/cmd/incident.go`. | No bounded/redacted incident seed into a successor; no preventive-evidence / why-caught requirement; no `caused_by`/`regresses` link keeping the original immutable. |
| Portfolio governance status (R9) | `renderProgram` — `internal/cmd/link.go:113`; ordering-only `Program` graph — `program.go:28-176`. | Status shows ordering only; no per-spec/edge risk, owner, stale/expired governance, unresolved signals, or shared-outcome criterion distinct from ordering; no documented scale envelope. |
| Portfolio exports + outcome review (R10) | `report.go` emits per-spec history/Prometheus (`gatherHistory` 18-147, `gatherPrometheus` 151-189); no portfolio export, no outcome-review join. | No stable-JSON redacted portfolio export resilient to tracker failure; no outcome-review keeping unknown≠success. |
| Decommission / archive (R11) | Absent. No `internal/core/archive.go` / archive verb. | No non-destructive retire-from-context preserving hashes/audit refs; no retention/scale-envelope policy for growing immutable history. |

## 9 validation scenarios — fixture checklist

Each scenario (domain doc §"Production validation scenarios" / design L0) needs a planned
offline fixture; none exist in-tree today. All must fail closed when inputs are missing
(never reported as holding/passing).

- [ ] **S1 Incident caused by a completed change** — successor spec links incident + original via `caused_by`/`regresses`; original state/history/evidence byte-unchanged. (R8.1/R8.3; fixture for T36/T38)
- [ ] **S2 Recurring invariant fails at a later HEAD** — new failing append-only record + drift finding appear; prior passing evidence stays auditable and is not overwritten. (R7.2/R6; T33/T50)
- [ ] **S3 Critical memory expires** — context builder omits/flags it per policy and names owner + revalidation action, not silent load/drop. (R4.2; T21)
- [ ] **S4 Decision is superseded** — only the active decision enters context; history shows the full supersession chain. (R3.1/R3.3; T15/T18)
- [ ] **S5 Forced promotion** — requires explicit authority/provenance and is audit-distinguishable from a frequency promotion. (R4.3; T23)
- [ ] **S6 Cross-spec modernization** — dependency DAG, shared-integration outcome, owners, and release evidence are all visible; satisfied DAG + failed shared release reads as incomplete. (R9.1/R9.3; T40/T41)
- [ ] **S7 Security exception reaches expiry** — configured governance gate fails closed until renewed by an authorized human or removed. (R3.2; T17)
- [ ] **S8 External tracker unavailable** — local ops and reports still work; export retry cannot corrupt spec state. (R10.1; T46)
- [ ] **S9 Large portfolio** — deterministic status/drift completes within the documented scale envelope; bounded context never loads the whole portfolio. (R9.2; T42)

## Notes for downstream waves

- RED-first: T03 (`maintenance_fixtures_test.go` + `testdata/maintenance/*.json`) and T04
  (reopen-guard reproduction) must land failing before their GREEN implementations (T06/T09+).
- Every new field on `ProgramLink` / `MemFields` / decision `Record` must decode existing
  on-disk records unchanged (back-compat, `-count=2` decode tests) — the load paths that must
  stay tolerant are `LoadProgram` (`program.go:41-60`), `ExtractMemBlock` (`memory.go:52-72`),
  and the `Record` unmarshal in `gatherHistory` (`report.go:26-51`).
- No LLM in any gate/projection/drift/recurring/report; no daemon/scheduler/network in core;
  no reopening a completed spec; no `reference/` edits.
