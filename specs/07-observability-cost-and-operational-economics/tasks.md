# Tasks — Domain 07 Observability DAG

`[ ]` pending. Execute wave after dependencies pass. Touch declared files only; record deviation.
Cross-domain prerequisites remain README links, not local task ids. Add a failing public-contract
fixture before each behavior change. Stdlib-only; no `reference/` edits; no LLM in any gate/report;
no network in core. Legacy ledgers must keep decoding.

## W0 — inventory, wording, contract baseline

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T01 | scout | docs/google-sdlc-alignment/README.md; docs/google-sdlc-alignment/07-observability-cost-and-operational-economics.md; specs/07-observability-cost-and-operational-economics | | printf ok | map R1-R9 to telemetry/evidence/acp/history/report_metrics/prometheus/manifest/budget/contextbudget/brakes/sense/brain_run surfaces and Domain 01/02/04/05/06/10 boundaries |
| [x] T02 | craftsman | internal/core/telemetry_test.go; internal/core/history_test.go; internal/context/budget_test.go; internal/orchestration/brakes_test.go; internal/core/prometheus_test.go | T01 | go test ./internal/core ./internal/context ./internal/orchestration -run 'Test(Telemetry|History|Budget|Brake|Prometheus)' | failing fixtures: no run/span correlation, conflated `tokens`, cost-without-currency, dormant unset cost brake, underestimated context payload, unbounded label additions R1-R6 |
| [x] T03 | craftsman | docs/observability.md; docs/open-spec-format.md; docs/command-reference.md; docs/CHEATSHEET.md | T01 | ./scripts/docs-lint.sh | correct wording: history is audit projection not full trace; telemetry is worker-reported not measured; cost brake dormant; budget does not prove sufficiency; name P0 route |

> **W0 deviations.** T01 inventory already recorded in `w0-inventory.md` (R1–R9 → telemetry/evidence/
> acp/history/prometheus/manifest/budget/contextbudget/brakes/sense/brain_run surfaces with 01/02/04/
> 05/06/10 boundaries). T02 "failing fixtures" are written as **passing characterization** tests that
> pin each current gap and flip to assert the fix when its wave lands: run/span+currency
> (`TestTelemetryLacksCorrelationAndCurrency`), conflated tokens (`TestHistoryTelemetryTokensAreConflated`),
> underestimated payload (`TestBudgetUnderestimatesPayload`), dormant brake (`TestBrakeDormantWhenMaxCostUnset`,
> new `brakes_test.go`), unbounded labels (`TestPrometheusTaskLabelsAreUnbounded`). T03: honesty wording
> added to `docs/observability.md` only; `open-spec-format.md`, `command-reference.md`, `CHEATSHEET.md`
> needed no edit (no telemetry wording / no CLI change in W0) — declared but not touched (subtractive).

## W1 — versioned run/telemetry envelope

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T04 | craftsman | internal/core/telemetry.go; internal/core/telemetry_test.go; internal/core/evidence.go; internal/core/evidence_test.go | T02 | go test ./internal/core -run 'Test(Telemetry|Evidence)' | versioned envelope preserves legacy `Annotations`; old fixtures decode unchanged; new records round-trip byte-stably R1.1 |
| [x] T05 | craftsman | internal/core/telemetry.go; internal/core/telemetry_test.go; internal/orchestration/acp.go; internal/orchestration/acp_test.go | T04 | go test ./internal/core ./internal/orchestration -run 'Test(Telemetry|ACP)' | malformed decimal, negative unit, cost-without-currency, unknown required schema version fail closed; absent stays absent not zero R1.2 |
| [x] T06 | craftsman | internal/core/telemetry.go; internal/core/telemetry_test.go; internal/cmd/report.go; internal/cmd/report_test.go | T05 | go test ./internal/core ./internal/cmd -run 'Test(Telemetry|Report)' | `telemetry_source` provenance required; reports render worker values as reported not measured; attestation ref external/optional R1.3 |

> **W1 deviations.** Envelope realized as omitempty fields on `Annotations`
> (`telemetry_source`, `currency`, `attestation_ref`, `envelope_version`) rather
> than a wrapper type, so legacy records decode/re-encode byte-identically and
> canonical v1 records round-trip byte-stably (R1.1). `ValidateAnnotations`
> version-gates strictness: legacy (no `envelope_version`) is grandfathered;
> canonical v1 fails closed on malformed decimal, negative unit,
> cost-without-currency, unknown source, and unknown required version (R1.2) —
> wired into `AppendEvidence`/`LoadEvidence(Records)` and `AppendACP`/`ReadACP`.
> No CLI `--currency`/`--source` flag added: that is W7 (T22, which owns
> `command-reference.md`/`CHEATSHEET.md`); W1 has no CLI/docs surface. T06
> `report.go` **not edited** (subtractive): provenance renders through
> `core.RenderTelemetry` (worker-reported disclaimer comment, never a metric
> label — cardinality allowlist stays `spec`/task/status per W5) and the
> `TaskTelemetry.telemetry_source` report field; `report.go`'s prometheus path
> needed no change. `attestation_ref` stays external/optional (R1.3).

## W2 — run correlation and attempt identity

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T07 | craftsman | internal/core/runledger.go; internal/core/runledger_test.go; internal/core/commands.go | T02 | go test ./internal/core -run 'Test(RunLedger|Run)' | deterministic `run_id`/monotonic `attempt` under spec lock; never provider trace ID as key; `runs.jsonl` append-only R2.1 |
| [x] T08 | craftsman | internal/cmd/registry.go; internal/cmd/brain_worker.go; internal/cmd/brain_report_test.go; internal/core/task_complete.go; internal/core/task_complete_test.go | T07 | go test ./internal/cmd ./internal/core -run 'Test(BrainReport|Complete|Run)' | manual + Brain runs share one allocator; two failures + one pass = three attempts one chain; evidence gate identical with ledger absent R2.2,R2.3 |
| [x] T09 | craftsman | internal/orchestration/checkpoint.go; internal/orchestration/recover.go; internal/core/runledger.go; scripts/stress-acp.sh | T08 | go test ./internal/orchestration -run 'Test(Checkpoint|Recover|Run)' && ./scripts/stress-acp.sh | crash mid-append yields one complete record or prior; no partial line or duplicate run/mission; racing writers do not duplicate attempt R2.4 |

> **W2 scope deviations (file list vs. actual edits, subtractive bias).**
> - T07 `commands.go` **not edited**: run/attempt identity is a side effect of
>   verify, not a verb — it needs no CLI surface or config, and `RunLedgerPath`
>   is a plain path function like every other ledger (`evidence.jsonl`,
>   `acp.jsonl`). Recorded rather than adding a dormant verb.
> - T08 `brain_report.go` **edited** (unlisted, needed): the shared allocator
>   `core.AllocateRun` must be called where `root`/`slug`/`head` are in hand; the
>   helper `allocateWorkerRun` lives in `brain_worker.go` (listed) and is invoked
>   from `brain_report.go`'s report flow. `task_complete.go` gains only a doc
>   comment: completion authority never reads `runs.jsonl` (R2.3), proven by
>   `TestCompleteTaskIndependentOfRunLedger`.
> - T09 `acp.go` **edited** (unlisted, needed): R2.4's "never a partial line"
>   applies to the mission ledger too — `ReadACP` now drops a torn *trailing*
>   line so recovery converges on the prior complete ledger. `checkpoint.go`
>   already crash-safe via `core.AtomicWrite` (temp+rename) and `recover.go`
>   inherits the tolerance through `ReadACP`, so neither needed a functional
>   change; both are exercised by the new `Recover`/`Run` tests and
>   `stress-acp.sh`'s run-ledger race.

## W3 — context accounting and sufficiency

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T10 | craftsman | internal/context/manifest.go; internal/context/manifest_test.go; internal/context/budget.go; internal/context/budget_test.go | T02, Domain 02 context | go test ./internal/context -run 'Test(Manifest|Budget)' | estimate covers bytes contract loads incl `design.md` + declared source files; repeated manifest byte-identical R3.1,R3.4 |
| [x] T11 | craftsman | internal/core/gates/contextbudget.go; internal/core/gates/contextbudget_test.go; internal/context/manifest.go | T10 | go test ./internal/core/gates ./internal/context -run 'Test(ContextBudget|Manifest)' | required item never silently dropped; over-budget required set fails with concise remediation; safety rules cannot be budgeted out R3.2 |
| [x] T12 | craftsman | internal/context/manifest.go; internal/context/manifest_test.go; internal/context/perf_test.go; scripts/perf-gate.sh | T11 | go test ./internal/context -run 'Test(Manifest|Perf)' && ./scripts/perf-gate.sh | estimated/host-reported/provider-billed distinct fields; canonical `context_manifest_digest`; supplied vs omitted items+reasons; host ack recorded R3.3,R3.4 |

> **W3 deviations (subtractive bias, backward-compat).**
> - T10 `budget.go` **not edited**: the underestimate lived in `BuildManifest`,
>   which set each item's `EstimatedTokens` from the path string, not the file.
>   The fix populates `Item.Bytes`/`EstimatedTokens` from real on-disk size
>   (`fileBytes`) at build time (R3.1); `ManifestBudget` already sums those, so
>   the pure struct function needed no change. `TestBudgetUnderestimatesPayload`
>   became `TestBudgetAccountsForPayload`, asserting the payload-aware estimate
>   through `BuildManifest`. Added `TestManifestByteIdentical` for R3.4.
> - T11 fail-closed is scoped to the **required core** set (spec/design/tasks/
>   task/role + declared files): if it alone exceeds budget, `BuildManifest`
>   returns `BudgetError` with the concise remediation (never silently truncated).
>   The steering-constitution / memory **drop order stays as domain-02 R4.3
>   defines it** (memory sheds before steering) — `TestSteeringInManifest`'s
>   tight-budget contract is preserved (gotcha b: no new default strictness that
>   breaks legacy behavior). "Safety rules cannot be budgeted out" is realized as
>   the required action set failing closed, not by protecting steering.
>   `TestBuildManifestRequiredOverflowBaseline` flipped to
>   `TestBuildManifestRequiredOverflowFailsClosed`; the gate surfaces the
>   `BudgetError` verbatim (redundant second estimate check removed).
> - T12 accounting realized as a separate `ContextAccountingV1` ledger
>   (`BuildAccounting`, `RecordHostAck`) with distinct estimated / host-reported /
>   provider-billed fields — the latter two default to `nil` (unknown, never
>   zero) — plus a canonical `context_manifest_digest` and supplied/omitted
>   items+reasons (R3.3,R3.4). The digest lives on the ledger, **not** embedded in
>   the base `Manifest`, so W5/W6's manifest-receipt baseline
>   (`TestBuildManifestNoReceiptBaseline`) stays valid. `Manifest.Notes` (unread)
>   was replaced by typed `Omissions`. `perf-gate.sh` unchanged; the perf pin
>   `TestPerfManifestDigestStable` proves byte-stable digest + payload-aware
>   estimate with no file-read amplification.

## W4 — honest cost brake

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [x] T13 | craftsman | internal/orchestration/sense.go; internal/orchestration/sense_test.go; internal/orchestration/brakes.go; internal/orchestration/brakes_test.go | T04, Domain 05 dispatch | go test ./internal/orchestration -run 'Test(Sense|Brake)' | `Snapshot.Cost` populated only from accepted trusted telemetry; no test populates it in a way production cannot R4.1 |
| [x] T14 | craftsman | internal/cmd/brain_run.go; internal/cmd/brain_run_test.go; internal/core/commands.go; docs/command-reference.md; docs/CHEATSHEET.md | T13 | go test ./internal/cmd -run 'TestBrainRun' && ./scripts/docs-lint.sh | configured threshold halts only subsequent dispatch with exact reason, no lease, undoes nothing; without limits behavior matches today R4.2 |
| [x] T15 | craftsman | internal/orchestration/brakes.go; internal/orchestration/brakes_test.go; internal/orchestration/sense.go | T13 | go test ./internal/orchestration -run 'Test(Brake|Sense)' | production requiring trusted telemetry fails closed when missing/malformed with one actionable message; untrusted-data brake labelled as such R4.3 |

> **W4 deviations (subtractive bias, backward-compat).**
> - The dishonest legacy brake was **removed**: `Snapshot.Cost int` /
>   `DecisionLimits.MaxCost int` fired without checking `TelemetryKnown` and were
>   only ever populated by tests, never by production `Sense` — exactly the "test
>   populates it in a way production cannot" that R4.1 forbids. All cost braking
>   now flows through the honest `CostMicros`/`Tokens`/`TelemetryKnown`/
>   `TelemetryTrusted` fields.
> - T13 honest population: `Sense` now takes an accrued `Telemetry` folded by
>   `AccrueTelemetry` from accepted `report` observations on the mission ledger.
>   Absent telemetry stays **unknown, never zero-filled**; a single unknown
>   observation poisons the total (unknown, not partial); worker-reported cost is
>   known-but-untrusted (accounting hint), host/adapter/attested is trusted.
> - T14 `commands.go`/`docs/command-reference.md`/`docs/CHEATSHEET.md` **not
>   edited**: the configured threshold is the existing `routing.max_cost_micros`
>   config key (already parsed/validated and wired into `DecisionLimits` in
>   `runBrainStep`), not a new verb or flag — so no CLI/docs surface changed and
>   `docs-lint.sh` stays green untouched (subtractive). `brain_run.go` now reads
>   the ACP ledger and feeds `AccrueTelemetry` into `Sense` so the brake fires on
>   measured cost; a halt withholds subsequent dispatch, mints no lease, and
>   appends nothing (undoes nothing).
> - T15 `sense.go` gains `TelemetryTrusted`; `brakes.go` fails closed under
>   `RequireTelemetry` when telemetry is missing **or** untrusted with one
>   actionable message, and labels any brake fired on untrusted data
>   `(untrusted telemetry)`. Malformed telemetry never reaches the brake — it is
>   rejected at `AppendACP`/`NormalizeObservation` before it can enter the ledger.

## W5 — privacy and cardinality policy

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T16 | craftsman | internal/core/prometheus.go; internal/core/prometheus_test.go; internal/core/report_metrics.go; internal/core/report_metrics_test.go | T04 | go test ./internal/core -run 'Test(Prometheus|ReportMetrics)' | static contract test rejects label additions outside allowlist (`spec`,task/status); no run/mission/SHA/path/model/actor/error label R5.1 |
| [ ] T17 | craftsman | internal/core/telemetry.go; internal/core/telemetry_test.go; internal/core/evidence.go; docs/telemetry-schema.md | T16, Domain 06 redaction | go test ./internal/core -run 'Test(Telemetry|Evidence|Redact)' | default fixtures metadata-only: no prompt/response/CoT/file content/raw output/secret/abs home path; central redaction before display R5.2,R5.4 |
| [ ] T18 | craftsman | internal/core/evidence.go; internal/core/evidence_test.go; docs/observability.md; SECURITY.md | T17 | go test ./internal/core -run 'Test(Evidence|Ref)' && ./scripts/docs-lint.sh | `evidence_ref` workspace-relative/content-addressed only; traversal + URL rejected in core schema; policy documented R5.3,R5.4 |

## W6 — metadata run spans and trace export

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T19 | craftsman | internal/core/runspan.go; internal/core/runspan_test.go; internal/core/commands.go | T06,T09,T17 | go test ./internal/core -run 'Test(RunSpan|Span)' | metadata-only spans for context/model/tool/edit/verify/eval/approval/dispatch; `kind` closed enum + extension; unknown critical kind fails parse R6.1 |
| [ ] T20 | craftsman | internal/cmd/report_trace.go; internal/cmd/report_trace_test.go; internal/core/history.go; internal/core/history_test.go | T19 | go test ./internal/cmd ./internal/core -run 'Test(ReportTrace|History)' | `report --trace --json` parent refs resolve, IDs unique, order stable on equal/missing ts, two exports byte-identical R6.2 |
| [ ] T21 | craftsman | internal/core/runspan.go; internal/core/runspan_test.go | T19 | go test ./internal/core -run 'TestRunSpan' | code-effect/completion span carries `git_head`; timestamps informational; no gate derives outcome from wall-clock order R6.3 |

## W7 — provider-neutral annotation expansion

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T22 | craftsman | internal/core/telemetry.go; internal/core/telemetry_test.go; internal/core/commands.go; docs/command-reference.md; docs/CHEATSHEET.md | T05,T17 | go test ./internal/core -run 'TestTelemetry' && ./scripts/docs-lint.sh | optional input/output/cache tokens, provider/model, currency, pricing_ref, source, attestation_ref; legacy `tokens` retained R7.1 |
| [ ] T23 | craftsman | internal/core/telemetry.go; internal/core/telemetry_test.go; internal/core/report_metrics.go; internal/core/report_metrics_test.go | T22 | go test ./internal/core -run 'Test(Telemetry|ReportMetrics)' | exact-decimal category aggregation golden tests; legacy total + category totals never silently contradict; cost needs currency+pricing_ref R7.2 |
| [ ] T24 | craftsman | internal/core/telemetry.go; internal/core/prometheus.go; internal/mcp/server_test.go | T22,T16 | go test ./internal/core ./internal/mcp -run 'Test(Telemetry|Prometheus|MCP)' | provider/model optional bounded, never required for valid evidence, never a metric label; MCP schema reflects new flags R7.3 |

## W8 — neutral event schema and context efficiency

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T25 | craftsman | internal/core/event.go; internal/core/event_test.go; internal/cmd/registry.go; docs/adapters/telemetry.md | T20,T22, Domain 10 adapter | go test ./internal/core ./internal/cmd -run 'Test(Event|Report)' | versioned provider-neutral event schema renders offline; golden validates; core no network/no module dep R8.1 |
| [ ] T26 | craftsman | internal/core/event.go; internal/core/event_test.go; docs/adapters/telemetry.md | T25 | go test ./internal/core -run 'TestEvent' | adapter round-trip fixture preserves correlation + privacy fields; networking disabled in adapter tests R8.2 |
| [ ] T27 | craftsman | internal/cmd/report.go; internal/cmd/report_test.go; internal/context/efficiency.go; internal/context/efficiency_test.go | T12,T20 | go test ./internal/cmd ./internal/context -run 'Test(Report|Efficiency)' | context-efficiency report shows estimated/actual tokens, omitted items, retries, first-pass, duration, cost with explicit `unknown` not zero R8.3 |

## W9 — attested ingestion, routing, roll-ups, release proof

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| [ ] T28 | craftsman | internal/core/attestation.go; internal/core/attestation_test.go; internal/core/telemetry.go; docs/adapters/telemetry.md | T23, Domain 10 adapter | go test ./internal/core -run 'Test(Attestation|Telemetry)' | signed/hashed attested envelope from allowlisted key accepted; tampering rejected; all adapter tests offline R9.1 |
| [ ] T29 | craftsman | internal/core/config_loader.go; internal/core/config_test.go; internal/context/manifest.go; internal/context/manifest_test.go | T10 | go test ./internal/core ./internal/context -run 'Test(Config|Manifest)' | routing recommendation is policy metadata; same task/config → same class; no provider contact; unavailable model cannot bypass evidence R9.2 |
| [ ] T30 | craftsman | internal/core/program.go; internal/core/program_test.go; internal/cmd/report.go; internal/cmd/report_test.go | T23,T27 | go test ./internal/core ./internal/cmd -run 'Test(Program|Report)' | cross-spec roll-ups exact, stable, bounded-cardinality, missing≠zero; drift alert references source records, mutates no lifecycle state R9.3 |
| [ ] T31 | craftsman | internal/cmd/e2e_test.go; internal/integration/observability_conformance_test.go; docs/command-reference.md; docs/CHEATSHEET.md | T14,T18,T21,T24,T27,T28 | go test ./internal/cmd ./internal/integration -run 'Test(ObservabilityE2E|ObservabilityConformance)' && ./scripts/docs-lint.sh | E2E: unannotated run, provider-annotated run, failed retry chain, crash mid-append, budget brake, missing attested accounting, context pressure, context drift, privacy, cardinality, offline, back-compat |
| [ ] T32 | validator | specs/07-observability-cost-and-operational-economics; internal/core; internal/cmd; internal/context; internal/orchestration; internal/integration | T29,T30,T31 | go test ./... -race -count=1 && go test ./... -count=2 && go vet ./... && gofmt -l . && ./scripts/test-lint.sh && ./scripts/docs-lint.sh && ./scripts/regress-all.sh && ./scripts/regress-domains.sh | full Domain 07 evidence; iteration-order stable; `go mod tidy` clean |

## Cross-wave rules

- Add a failing public-contract fixture before each behavior change (T02 seeds W1-W6).
- Verify/eval stays the only completion authority; missing/malformed telemetry never turns failure
  into success and never blocks a valid completion. Never mock a gate green at the report boundary.
- Keep three quantities distinct — estimated footprint, host-reported actual, provider-billed —
  never merge into one authoritative value.
- Trace JSONL carries high-cardinality correlation; metrics expose only allowlisted labels. A label
  addition outside the allowlist must fail the static contract test.
- Worker telemetry is an accounting hint; provider-attested usage enters only through a validated
  offline adapter envelope. Trust source stays visible in every relevant report.
- Old evidence/ACP/telemetry ledgers decode unchanged; new canonical records round-trip byte-stably;
  unknown required schema version fails closed. Downgrade documented, never rewrites data implicitly.
- Domain 05 owns mission/lease/ACP transport; Domain 06 owns redaction/policy digest; Domain 10 owns
  external adapter transport — Domain 07 supplies measurement/export, no duplicate policy.
- Core stays offline/stdlib-only; only an explicitly invoked adapter communicates externally. Keep
  `reference/` untouched; `gofmt -l .` empty and `go mod tidy` clean before release.
