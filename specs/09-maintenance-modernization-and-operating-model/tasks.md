# Tasks — Maintenance, modernization, and the operating model

Waves map to the deliverable specs in `README.md`. Each task is one atomic craftsman/scout/validator
unit with a runnable `verify:` line. Maintenance verbs/flags are declared once in
`internal/core/commands.go`, derived into MCP/help, and mirrored in `docs/command-reference.md` and
`docs/CHEATSHEET.md`. No LLM in any gate/projection/drift verdict/recurring result/report; no daemon,
scheduler, or network in core; no reopening of a completed spec; no `reference/` edits. `verify:`
lines target this repo's tree; runtime paths under `.specd/specs/` are exercised in temp trees by the
smoke/regression scripts.

Legend: **role** ∈ {scout=read-only, craftsman=write+verify one task, validator=read-only run verify,
auditor=read-only audit diff}. **deps** are task IDs (and cross-domain notes). **req** cites
`requirements.md`.

## W0 — `09a-maintenance-baseline` (requires —)

| id | role | files | deps | verify | req |
|---|---|---|---|---|---|
| T01 | scout | `docs/google-sdlc-alignment/09-*.md`; `internal/core/program.go`; `internal/core/memory.go`; `internal/core/history.go`; `internal/cmd/report.go` | — | `printf ok` | inventory current link/memory/decision/report behavior for every P0 gap |
| T02 | craftsman | new `docs/operating-model-contract.md` | T01 | `test -s docs/operating-model-contract.md && grep -q supersedes docs/operating-model-contract.md` | R1,R3 draft link-kind/lifecycle/provenance field definitions |
| T03 | craftsman | new `internal/core/maintenance_fixtures_test.go`; `testdata/maintenance/*.json` | T01 | `go test ./internal/core -run TestMaintenanceFixture -count=1` (RED: assert-only skeleton) | R1.2,R2.1,R3.1,R4.1 failing fixtures for every P0 gap |
| T04 | craftsman | `internal/core/program.go` (reopen guard reproduction) | T01 | `go test ./internal/core -run TestReopenRejected -count=1` (RED) | R1.1 reproduce absence of a completed-spec reopen guard |
| T05 | auditor | domain README/requirements/design vs `09-*.md` | T02 | `printf ok` | confirm 9 validation scenarios each have a planned fixture |

> **W0 deviations (recorded per prompt.md §2 cross-wave rule):**
> - T04's reproduction test `TestReopenRejected` lands in `internal/core/program_test.go` (the
>   test file for the declared `program.go`), and is `t.Skip`-ped rather than left RED — a bare
>   RED test would fail the wave gate (prompt.md §3/§5). The skip pins the full R1.1 assertion
>   (reopen fails closed with a successor-directing message); 09b/T09 removes the skip and adds the
>   guard to make it GREEN with the same verify line.
> - T05 audit confirmation is recorded in `w0-inventory.md` §"T05 audit"; the domain doc
>   `docs/google-sdlc-alignment/09-*.md` was read-only cross-checked, not edited.

## W1 — `09b-successor-link-kinds` (requires 09a)

| id | role | files | deps | verify | req |
|---|---|---|---|---|---|
| [x] T06 | craftsman | `internal/core/program.go` (`ProgramLink.Kind`, versioned, back-compat decode) | T03 | `go test ./internal/core -run TestLinkKindDecode -count=2` | R1.2 typed kind; untyped links decode forward |
| [x] T07 | craftsman | `internal/cmd/link.go`; `internal/core/commands.go` (`--kind`) | T06 | `go test ./internal/cmd -run TestLinkKind -count=1` | R1.2 kind/reason recorded; source traceable |
| [x] T08 | craftsman | `internal/core/program.go` (cycle + ordering unchanged with kinds) | T06 | `go test ./internal/core -run TestProgramCycleWithKinds -count=2` | R1.3 cycle detection + ordering preserved |
| [x] T09 | craftsman | `internal/core/state.go`/`internal/cmd` completion path (reopen guard) | T04,T06 | `go test ./internal/core -run TestReopenRejected -count=1` (GREEN) | R1.1 completed spec immutable; directs to successor |
| [x] T10 | craftsman | `docs/command-reference.md`; `docs/CHEATSHEET.md` | T07 | `./scripts/docs-lint.sh` | R1.2 mirror `--kind` verb/flag |

> **W1 deviations (recorded per prompt.md §2 cross-wave rule):**
> - T06/T08 tests land in `internal/core/program_test.go`, and T07 tests land in
>   `internal/cmd/link_test.go`, the existing test companions for their declared source files.
> - T09's guard lands in `internal/core/phases.go`, where `AdvanceStatus` owns lifecycle transition
>   validation; `state.go` stores state but does not decide transitions.

## W2 — `09c-typed-intake-provenance` (requires 09a)

| id | role | files | deps | verify | req |
|---|---|---|---|---|---|
| [x] T11 | craftsman | new `internal/core/provenance.go` (versioned struct, back-compat decode) | T03 | `go test ./internal/core -run TestProvenanceDecode -count=2` | R2.1 typed source/system/severity/owner/prior-links |
| [x] T12 | craftsman | new `internal/core/gates/intake.go`; gates registry | T11 | `go test ./internal/core/gates -run TestIntakeReadiness -count=1` | R2.2 missing configured field fails readiness |
| [x] T13 | craftsman | `internal/core/gates/intake.go` (unknown≠empty; unconfigured no-op) | T12 | `go test ./internal/core/gates -run TestIntakeUnknownSentinel -count=2` | R2.2,R2.3 pure verdict; feature spec unaffected |
| [x] T14 | craftsman | `internal/cmd/report.go` (project provenance into history) | T11 | `go test ./internal/cmd -run TestProvenanceHistory -count=1` | R2.1 provenance visible without LLM path |

> **W2 deviations (recorded per prompt.md §2 cross-wave rule):**
> - T11–T14 tests land in the conventional companion files
>   `internal/core/provenance_test.go`, `internal/core/gates/intake_test.go`, and
>   `internal/cmd/report_provenance_test.go`.
> - T12 wires the pure gate input in `internal/cmd/registry.go`, updates the registry-order test,
>   and updates gate-count documentation; without caller-side loading the registered gate could
>   not evaluate the per-spec provenance file.
> - T14 adds a deterministic provenance source rank in `internal/core/history.go` so history
>   ordering remains total when provenance has no timestamp.

## W3 — `09d-decision-exception-lifecycle` (requires 09a, Domain 06 authority)

> **W3 deviations (recorded per prompt.md §2 cross-wave rule):**
> - T15–T19 tests land in conventional companion files `internal/core/history_lifecycle_test.go`,
>   `internal/core/exception_test.go`, `internal/core/gates/governance_test.go`, and
>   `internal/context/decision_test.go`.
> - T17 also wires immutable governance snapshots in `internal/core/gates/core.go` and updates
>   `internal/cmd/registry.go`, then updates registry-order/gate-count documentation; a registered
>   pure gate needs typed `CheckCtx` inputs loaded by its caller.
> - T18 adds `internal/context/decision.go`, keeping lifecycle filtering separate from generic
>   manifest assembly and available to both manifest versions in later integration work.

| id | role | files | deps | verify | req |
|---|---|---|---|---|---|
| [x] T15 | craftsman | `internal/core/history.go` (id/status/owner/dates/supersedes/affected) | T03 | `go test ./internal/core -run TestDecisionLifecycle -count=1` | R3.1 identity + append-only supersession |
| [x] T16 | craftsman | new `internal/core/exception.go` (time-bound governed deviation) | T15 | `go test ./internal/core -run TestException -count=1` | R3.1 exception id/status/owner/expiry |
| [x] T17 | craftsman | new `internal/core/gates/governance.go`; gates registry | T15,T16 | `go test ./internal/core/gates -run TestGovernanceExpiry -count=1` | R3.2 expired blocking exception fails closed |
| [x] T18 | craftsman | `internal/context` builder (active-only load) | T15 | `go test ./internal/context -run TestActiveDecisionsOnly -count=2` | R3.3 only active records in routine context |
| [x] T19 | validator | agents cannot be recordable owners | T15 | `go test ./internal/core -run TestOwnerNotAgent -count=1` | R3.2 owner is human/team |

## W4 — `09e-memory-provenance-and-aging` (requires 09a, Domain 02 context)

| id | role | files | deps | verify | req |
|---|---|---|---|---|---|
| [x] T20 | craftsman | `internal/core/memory.go` (`MemFields` owner/validated/provenance/expiry/supersedes, back-compat) | T03 | `go test ./internal/core -run TestMemFieldsDecode -count=2` | R4.1 aging fields parse; old files still parse |
| [x] T21 | craftsman | `internal/context` builder (exclude invalid/expired critical + finding) | T20 | `go test ./internal/context -run TestExpiredMemoryExcluded -count=1` | R4.2 visible finding, not silent load/drop |
| [x] T22 | craftsman | `internal/context` builder (preserve stable non-expiring constraints) | T21 | `go test ./internal/context -run TestStableMemoryPreserved -count=1` | R4.2 stable constraints survive aging |
| [x] T23 | craftsman | `internal/core/memory.go` (forced promotion authority/provenance) | T20 | `go test ./internal/core -run TestForcedPromotionAudit -count=1` | R4.3 forced ≠ frequency; audit-distinguishable |

> **W4 deviations (recorded per prompt.md §2 cross-wave rule):**
> - T20–T23 tests land in conventional companion files `internal/core/memory_test.go` and
>   `internal/context/memory_test.go`.
> - T21/T22 add caller-supplied `SelectionContext.AsOf` in `internal/context/steering.go`; explicit
>   evaluation time keeps context aging deterministic and preserves zero-value legacy behavior.
> - T23 wires the forced audit envelope in `internal/cmd/memory.go`, where promotion mode is known;
>   core rendering remains pure and records deterministic authority/provenance.

## W5 — `09f-maintenance-templates` (requires 09b,09c)

| id | role | files | deps | verify | req |
|---|---|---|---|---|---|
| [x] T24 | craftsman | new `internal/core/embed_templates/maintenance/{incident,dependency,migration,recurring}.md`; `roles.go`/`scaffold.go` | T07,T11 | `go test ./internal/core -run TestMaintenanceTemplates -count=1` | R5.1 source→req→task→evidence→learning |
| [x] T25 | craftsman | `internal/core/scaffold.go` (refresh preserves project content) | T24 | `go test ./internal/core -run TestTemplateRefreshPreserves -count=1` | R5.2 inspectable schema/version; content survives refresh |
| [x] T26 | craftsman | templates scaffold intake + successor links per source_type | T24,T12 | `go test ./internal/core -run TestTemplateReadinessPassable -count=1` | R5.3 instantiated spec starts readiness-passable |
| [x] T27 | craftsman | `docs/command-reference.md`; `docs/CHEATSHEET.md` | T24 | `./scripts/docs-lint.sh` | R5.1 document templates |

> **W5 deviations (recorded per prompt.md §2 cross-wave rule):**
> - T24–T26 tests land in conventional companion file
>   `internal/core/maintenance_templates_test.go`.
> - T25 extends the existing managed-asset merge in `internal/core/managed.go`; `scaffold.go`
>   already delegates all scaffold/refresh writes through that byte-preserving path.

## W6 — `09g-drift-projection` (requires 09c,09d, Domain 04 evidence)

| id | role | files | deps | verify | req |
|---|---|---|---|---|---|
| [x] T28 | craftsman | new `internal/core/drift.go` (compare invariants/decisions vs evidence) | T13,T17 | `go test ./internal/core -run TestDriftProjection -count=2` | R6.1 read-only; no mutation/network |
| [x] T29 | craftsman | new `internal/cmd/drift.go`; `internal/core/commands.go` | T28 | `go test ./internal/cmd -run TestDriftByteStable -count=2` | R6.2 byte-stable; finding carries source/path/HEAD/suggested cmd |
| [x] T30 | craftsman | `internal/core/drift.go` (holds/drifted/not-evaluable/none) | T28 | `go test ./internal/core -run TestDriftNotEvaluable -count=1` | R6.3 missing input never reported as holding |
| [x] T31 | craftsman | `docs/command-reference.md`; `docs/CHEATSHEET.md` | T29 | `./scripts/docs-lint.sh` | R6.1 mirror `drift` verb |

> **W6 deviations (recorded per prompt.md §2 cross-wave rule):**
> - T28–T30 tests land in conventional companion files `internal/core/drift_test.go` and
>   `internal/cmd/drift_test.go`.
> - T29 updates `internal/cmd/registry.go`, which owns executable-handler parity, and
>   `scripts/regress-domains.sh`'s intentional CLI surface-count tripwire from 30 to 31.
> - Drift declarations use optional, versioned `.specd/specs/<slug>/drift.json`; absence projects
>   `none`, preserving legacy projects without adding a completeness gate.

## W7 — `09h-recurring-invariants` (requires 09c, Domain 07 measurement)

| id | role | files | deps | verify | req |
|---|---|---|---|---|---|
| [x] T32 | craftsman | new `internal/core/recurring.go` (check = command + cadence, versioned) | T11 | `go test ./internal/core -run TestRecurringDefine -count=1` | R7.1 definition validated; specd is not a daemon |
| [x] T33 | craftsman | `internal/core/recurring.go` (append-only result under spec lock) | T32 | `go test ./internal/core -run TestRecurringAppendOnly -count=2` | R7.2 failing HEAD never overwrites last pass |
| [x] T34 | craftsman | `internal/cmd/recurring.go`; `internal/core/commands.go`; `.github/workflows` example | T32 | `go test ./internal/cmd -run TestRecurringRecord -count=1` | R7.1 external CI runs; specd validates/records |
| [x] T35 | craftsman | `internal/core/recurring.go` (opt-in successor scaffold on failure) | T33 | `go test ./internal/core -run TestRecurringSuccessorOptIn -count=1` | R7.3 no silent mutation of completed record |

> W7 deviation: T34 also updates CLI reference/cheatsheet sync and
> `scripts/regress-domains.sh`'s intentional verb-count tripwire from 31 to 32 for `recurring`.

## W8 — `09i-incident-successor-and-prevention` (requires 09b,09f, Domain 08 observation)

| id | role | files | deps | verify | req |
|---|---|---|---|---|---|
| T36 | craftsman | new `internal/core/incident.go`; `internal/cmd/incident.go` | T24,T07 | `go test ./internal/cmd -run TestIncidentSeed -count=1` | R8.1 bounded redacted refs seed spec; raw payload not loaded |
| T37 | craftsman | `internal/core/incident.go` (preventive-evidence + why-caught ref) | T36 | `go test ./internal/core -run TestPreventiveEvidence -count=1` | R8.2 closure can require regression test/eval |
| T38 | craftsman | `internal/core/incident.go` (`caused_by`/`regresses` link, original immutable) | T36,T09 | `go test ./internal/core -run TestIncidentOriginalImmutable -count=1` | R8.3 original history/evidence unchanged |
| T39 | validator | incident payload redaction/bounding | T36 | `go test ./internal/core -run TestIncidentRedaction -count=1` | R8.1 secrets/customer data bounded, never instruction |

## W9 — `09j-portfolio-governance-status` (requires 09b,09d)

| id | role | files | deps | verify | req |
|---|---|---|---|---|---|
| T40 | craftsman | `internal/core/program.go` (status risk/owner/stale governance) | T07,T17 | `go test ./internal/core -run TestPortfolioStatus -count=2` | R9.1 deterministic risk/owner/stale surface |
| T41 | craftsman | `internal/core/program.go` (shared-outcome criterion distinct from ordering) | T40 | `go test ./internal/core -run TestSharedOutcome -count=1` | R9.3 satisfied DAG + failed shared release visibly incomplete |
| T42 | validator | large-program scale envelope | T40 | `go test ./internal/core -run TestPortfolioScale -count=1` | R9.2 within documented envelope; context not whole portfolio |

## W10 — `09k-memory-conflict-lint` (requires 09e)

| id | role | files | deps | verify | req |
|---|---|---|---|---|---|
| T43 | craftsman | new `internal/core/gates/memorylint.go`; gates registry | T20 | `go test ./internal/core/gates -run TestMemoryConflictLint -count=1` | R4.2 dup keys / contradictory critical / unowned force are findings |
| T44 | validator | lint runs before context construction | T43 | `go test ./internal/context -run TestLintBeforeBuild -count=1` | R4.2 findings surface pre-build |

## W11 — `09l-org-adoption-and-archive` (requires 09i,09j,09k, Domain 10 boundary)

| id | role | files | deps | verify | req |
|---|---|---|---|---|---|
| T45 | craftsman | new `internal/core/embed_templates/policy/{approval,review,security-exception,production-readiness,on-call,incident-response}.md` | T24 | `go test ./internal/core -run TestPolicyTemplates -count=1` | R5.2 inspectable schema/version; content survives refresh |
| T46 | craftsman | `internal/cmd/report.go` (stable-JSON portfolio export, redacted) | T40 | `go test ./internal/cmd -run TestPortfolioExport -count=2` | R10.1 stable JSON; export failure cannot corrupt state |
| T47 | craftsman | `internal/cmd/report.go` (outcome-review; unknown≠success) | T37,T46 | `go test ./internal/cmd -run TestOutcomeReviewUnknown -count=1` | R10.2 unknown stays unknown |
| T48 | craftsman | new `internal/core/archive.go`; `internal/cmd` archive verb | T43,T15 | `go test ./internal/core -run TestArchivePreservesHashes -count=1` | R11.1,R11.2 retire from context; preserve hashes non-destructively |
| T49 | craftsman | `docs/command-reference.md`; `docs/CHEATSHEET.md`; scale-envelope doc | T46,T48 | `./scripts/docs-lint.sh` | R11.3 retention/scale documented; mirror export/archive verbs |
| T50 | validator | 9 production validation scenarios end to end | T38,T41,T44,T47,T48 | `go test ./... -race -count=1 && go test ./... -count=2` | operate proof: all scenarios pass offline; full suite green |

## Cross-wave rules

- Every craftsman task is one atomic unit with an exit-0 git-pinned `verify:` record; read-only tasks
  carry a trivially-passing line (`printf ok`). No bypass flag.
- RED fixtures (T03, T04) land before their GREEN implementation (T06/T09 and downstream); the wave
  is not done until GREEN.
- Any wave touching a verb/flag updates `internal/core/commands.go` once and mirrors
  `docs/command-reference.md` + `docs/CHEATSHEET.md` (`docs-lint.sh`), and keeps high-risk mutations
  out of the general MCP palette.
- `go test ./... -race -count=1` and `-count=2` (iteration-order) gate every wave; `gofmt -l .`,
  `go vet ./...`, and `go mod tidy` must stay clean.
- No wave reopens/edits a completed spec, adds a lifecycle status value, adds a runtime dependency, a
  network call/daemon/scheduler in core, an LLM to a gate/projection/report, or a `reference/` edit.
- Backward-compatible decode is mandatory: every new field on `ProgramLink`/`MemFields`/decision must
  parse existing on-disk records unchanged (`-count=2` decode tests prove it).
