# W0 T01 — Domain 07 Observability Inventory (scout)

Read-only mapping of requirements R1–R9 to current code surfaces, gaps, and cross-domain
boundaries. File:line refs are to the current tree at the time of scouting. No product code
edited. This file is the only artifact written.

## R1–R9 → surface / gap / boundary

| Requirement | Current code surface | Gap | Boundary domain |
|---|---|---|---|
| R1.1 Versioned envelope wraps telemetry/evidence/ACP; legacy decodes; new round-trips; keep `Annotations` | `internal/core/telemetry.go:18` `Annotations{Tokens,Cost,DurationMs}`; `internal/core/evidence.go:15` `EvidenceRecord` (`Telemetry *Annotations` :30); `internal/orchestration/acp.go:23` `ACPEvent` (`Telemetry *core.Annotations` :43) | No `envelope_version`/`schema_version` field anywhere; no versioned wrapper; decode has no unknown-required-version fail-closed path | Core (07); envelope shared shape aligns w/ Domain 10 adapter contract |
| R1.2 Malformed decimal / negative unit / cost-without-currency / unknown required version fail closed; absent≠zero | `internal/core/telemetry.go:33` `ParseAnnotations` (rejects negative tokens/duration :43,:50; `decimalPattern` :27 rejects signs/exponents) | No currency field so "cost-without-currency" cannot be checked; no schema-version check; absent-vs-zero holds for pointers but `TaskTelemetry` zero-values (`Tokens int` etc `:71`) can read as 0 | Core (07) |
| R1.3 Every value carries `telemetry_source` provenance; reports render worker values as reported; attestation external/optional | none — `Annotations` has no source; `internal/core/report_metrics.go:8` `RenderMetrics`; `internal/core/telemetry.go:169` `RenderTelemetry` label everything `spec/task` with no trust label | No `telemetry_source`, no `attestation_ref`; reports present worker cost as plain totals w/o "reported" caveat (see prometheus.go:61-67 "Worker-reported" only in HELP text) | Core (07); attestation ingestion is Domain 10 adapter |
| R2.1 Deterministic `run_id` + monotonic `attempt` under spec lock; never provider trace ID as key; `runs.jsonl` additive | `internal/orchestration/acp.go:39` `Attempt` + `:50` `NextAttempt` (per-task attempt); `MissionID` deterministic `:32`; spec lock exists (`core.WithSpecLock`) | No `run_id`; no `runledger.go`/`runs.jsonl`; verify evidence (`EvidenceRecord`) has no run/attempt at all — attempt lives only in ACP | Core (07); attempt/mission transport overlaps Domain 05 |
| R2.2 Manual + Brain share one run identity per loop; 2 fail+1 pass = 3 attempts one chain; racing writers no dup attempt | ACP path only: `internal/cmd/brain_worker.go` (worker), `internal/cmd/registry.go` verify handler (manual, separate) | Manual verify and Brain use different code paths with no shared allocator; manual runs have no attempt/run identity | Domain 05 (dispatch/lease/worker) owns Brain path; 07 supplies shared allocator |
| R2.3 Run ledger additive; evidence gate identical with ledger absent; completion stays verify/eval | completion authority: `internal/core/evidence.go:113` `HasPassingEvidence` (exit 0 + pinned HEAD); `task_complete.go` | `runs.jsonl` does not exist yet; requirement is to keep gate unchanged when it is added | Core (07) |
| R2.4 Crash mid-append → one complete record or prior; no partial/dup run/mission | append safety: `core.AppendFile`/`AppendEvidence` (`evidence.go:37`); ACP dup guard `acp.go:78` `ErrDuplicateMission`, `HasMission` :81; checkpoint/recover under `internal/orchestration/` | No run-ledger crash test; relies on reusing existing atomic-append + checkpoint replay for the new `runs.jsonl` | Domain 05 (checkpoint/recover); 07 reuses primitive |
| R3.1 Manifest estimate covers bytes host loads incl `design.md` + declared source files | `internal/context/manifest.go:45-53` core items = requirements/tasks/task-row/role, `EstimatedTokens = EstimateText(Kind+Path+TaskID)` (:52); `internal/context/budget.go:5` `ManifestBudget` sums same string estimates | **P0**: `design.md` absent from item list; task's declared `files:` absent; four core items sized from label/path strings, not file contents — undercount. (Also path prefix `specs/%s` not `.specd/specs/%s` :46-49, README finding 1) | Domain 02 (context/knowledge/manifest content) co-owns; 07 owns accounting correctness |
| R3.2 Required items never silently dropped; over-budget required set fails w/ remediation; safety rules cannot be budgeted out | `internal/core/gates/contextbudget.go:9` `contextBudget` (only compares `EstimatedTokens > max` :20); `manifest.go:64` `enforceBudget` sheds items to notes | Budget shedding does not distinguish required vs optional; no "required over budget → fail with remediation"; no protected-safety-rules concept | Domain 02 (required/optional taxonomy); 07 gate enforcement |
| R3.3 estimated / host-reported-actual / provider-billed are distinct fields; passing gate ≠ proof model saw items; host acks digests | `manifest.go:65` `Manifest{EstimatedTokens}` only one field; `contextbudget.go` gate pass is presented as budget compliance | Only one token quantity exists; no host-ack field, no provider-billed field; gate pass conflated with sufficiency | Core (07); host-ack handshake touches Domain 03 |
| R3.4 Repeated manifests byte-identical; canonical `context_manifest_digest` over manifest+config+role+steering; supplied vs omitted+reasons | `manifest.go:55` deterministic `sort.SliceStable`; `enforceBudget` returns `notes` (:64) | No `context_manifest_digest`; notes are freeform not structured supplied/omitted+reason; digest must cover effective config/role/steering versions | Core (07); config/role/steering versions from Domain 01/02/06 |
| R4.1 Cost brake wired from accepted telemetry OR dormant implication removed; no test-only `Snapshot.Cost` | `internal/orchestration/sense.go:17` `Snapshot.Cost int`; `Sense` (:20) never sets Cost; `internal/orchestration/brakes.go:4` `EvaluateBrakes` halts on `snapshot.Cost > limits.MaxCost` | **P0**: `Sense` leaves `Cost` zero → brake is dormant; only tests can populate it (misleading partial feature). `Cost int` also cannot carry exact decimal | Domain 05 (dispatch/decision loop) owns brake wiring; 07 owns telemetry→cost |
| R4.2 Configured threshold halts only subsequent dispatch w/ exact reason, no lease, undoes nothing; no-limit = today | `brakes.go:4` `ActionHalt`+reason "cost limit exceeded"; `internal/cmd/brain_run.go:124` constructs `limits` | Reason string is generic not exact; `brain_run` builds limits with retries/deadline but no cost config wired; halt-only-future semantics untested | Domain 05 (dispatch loop) |
| R4.3 Production requiring trusted telemetry fails closed when missing/malformed; untrusted-data brake says so | none | No trust/attestation gating on brake; brake cannot distinguish trusted vs worker data; no fail-closed path | Domain 06 (policy) + Domain 10 (attested adapter); 07 supplies trust label |
| R5.1 Static contract test rejects Prometheus labels outside allowlist (`spec`,task/status); no run/mission/SHA/path/model/actor/error label | `internal/core/prometheus.go:35` `RenderPrometheus` uses only `spec`,`status`,`verdict` labels (:37-67); `internal/core/report_metrics.go` | No static allowlist contract test guarding future additions; nothing prevents a later label from being added | Core (07) |
| R5.2 Default trace/run fixtures metadata-only: no prompt/response/CoT/file content/raw output/secret/abs home path | `evidence.go:118` `TruncateEvidenceOutput` (64KB cap); verify uses scrubbed env (`verify/exec.go`) | Truncation ≠ redaction; no policy that default fixtures carry zero raw content; run/trace fixtures do not exist yet | Domain 06 (central redaction) owns; 07 sets fixture policy |
| R5.3 `evidence_ref` workspace-relative/content-addressed only; traversal + URL rejected in core schema | `evidence.go:20` `EvidenceRef string` (free string, no validation); ACP has ref fields | No traversal/URL rejection; `EvidenceRef` accepts any string | Core (07) schema; adapter resolution is Domain 10 |
| R5.4 Policy documented in `docs/telemetry-schema.md`,`docs/observability.md`,`SECURITY.md`; central redaction before display/export | `docs/observability.md`, `SECURITY.md` exist | `docs/telemetry-schema.md` absent; redaction-before-export not documented as contract | Domain 06 (redaction) |
| R6.1 Metadata-only spans for context/model/tool/edit/verify/eval/approval/dispatch; `kind` closed enum+extension; unknown critical kind fails parse | `internal/core/history.go` `HistoryEvent` (audit projection, kind strings); ACP `kind` strings | **P0-adjacent**: no `runspan.go`, no `SpanV1`, no `parent_span_id`, no closed `kind` enum; history merges sources after the fact (cannot answer "which context/model call caused this verify") | Core (07) |
| R6.2 `report --trace --json` pure projection: parents resolve, IDs unique, stable order on equal/missing ts, two exports byte-identical | `internal/cmd/report.go` (`--history`,`--metrics`); `history.go` stable total order | No `report --trace`/`report_trace.go`; no span hierarchy to project | Core (07) |
| R6.3 Span claiming code-effect/completion carries `git_head`; timestamps informational; no gate from wall-clock alone | `git_head` present on evidence (:19) and ACP (`acp.go:40`) | No span type to attach `git_head` to yet; must ensure new spans require it for code effects | Core (07) |
| R7.1 Optional input/output/cache tokens, provider, model, currency, pricing_ref, source, attestation_ref; keep legacy `tokens` total | `telemetry.go:19` single `Tokens int`; no currency/provider/model | Only one undifferentiated `tokens`; all listed additive fields absent | Domain 10 (provider/model from adapter); 07 owns schema |
| R7.2 Exact-decimal category aggregation golden tests; legacy total + category totals never contradict; cost needs currency+pricing_ref | `telemetry.go:93` `AggregateTelemetry` uses `big.Rat` (:120,:136) exact; `formatRat` :154 | No category totals to reconcile; no currency/pricing_ref mandate; golden tests are for single total only | Core (07) |
| R7.3 provider/model optional bounded, never required for valid evidence, never a metric label; MCP reflects flags | none; MCP schema in `internal/mcp/` | provider/model absent; must ensure they never become Prometheus labels (ties to R5.1); MCP flag schema needs update | Domain 10 (provider identifiers); 07 schema; Domain 03 (MCP palette) |
| R8.1 Versioned provider-neutral local event schema renders offline; golden validates; core no network/no module dep | none — no `event.go` | No neutral event schema/renderer; OTel mapping must live in external adapter | Domain 10 (adapter→OTel) |
| R8.2 Adapter round-trip fixture preserves correlation + privacy fields; networking disabled | none | No adapter contract/fixture | Domain 10 |
| R8.3 Context-efficiency report: estimated/actual tokens, omitted items, retries, first-pass, duration, cost with explicit `unknown` not zero | `report_metrics.go`; telemetry report `Missing` list (`telemetry.go:85`) marks absence | No efficiency report joining manifest+outcome; retries/first-pass/duration not correlated to a run; needs R3.3 fields | Domain 02 (context) + 07 |
| R9.1 Signed/hashed attested envelope from allowlisted key accepted; tampering rejected; adapter tests offline | none — no `attestation.go` | No attestation ingestion; no allowlist/key config | Domain 10 (adapter) + Domain 06 (key policy) |
| R9.2 Routing recommendation is policy metadata; same task/config→same class; no provider contact; unavailable model cannot bypass evidence | none; config in `internal/core/config_*`, packet fields in `internal/context` | No routing class metadata; must stay deterministic policy, no model execution | Domain 05 (model routing) owns; 07 records |
| R9.3 Cross-spec roll-ups exact/stable/bounded-cardinality, missing≠zero; drift alert references source, mutates no lifecycle state | `internal/core/program.go` (cross-spec program links) | No economic roll-up/drift alert; must reuse exact-decimal math and preserve missing≠zero | Core (07); Domain 09 (drift/maintenance) adjacent |

## P0 gaps (make current claims accurate and safe)

1. **Dormant cost brake (R4.1/R4.2)** — `sense.go:20` `Sense` never sets `Snapshot.Cost`;
   `brakes.go:4` halts on a value only tests can populate. Misleading partial feature: wire from
   accepted telemetry or remove the public implication. Also `Snapshot.Cost int` cannot hold exact
   decimal. **Owner: Domain 05 dispatch loop + 07 telemetry→cost.**
2. **Context under-accounting (R3.1/R3.2)** — `manifest.go:45-53` sizes the four core items from
   `Kind+Path+TaskID` strings, omits `design.md` and the task's declared `files:`. A passing
   `contextbudget.go` gate does not prove sufficiency; over-budget required set is not distinguished
   from optional. **Owner: Domain 02 content + 07 accounting.**
3. **No trust provenance / envelope version (R1.1/R1.3)** — `Annotations` has no
   `telemetry_source`, `attestation_ref`, or `schema_version`; reports render worker cost as plain
   totals. Old fixtures must keep decoding when the versioned envelope lands.
4. **No run correlation (R2.1)** — no `run_id`/`runs.jsonl`; attempt identity lives only in ACP
   (`acp.go:39`), manual verify has none. Must add additive ledger without changing completion
   authority (`evidence.go:113`).
5. **No cardinality/privacy contract (R5.1/R5.3)** — labels are low-cardinality today
   (`prometheus.go`) but nothing statically forbids future run/SHA/path/model/actor labels;
   `EvidenceRef` (`evidence.go:20`) accepts traversal/URLs; `docs/telemetry-schema.md` missing.

## Boundary summary (07 supplies measurement/export; no duplicate policy)

- **Domain 01** — lifecycle/config+role versions feed the `context_manifest_digest` (R3.4).
- **Domain 02** — owns manifest content, required/optional context taxonomy, skills; 07 corrects
  the accounting on top (R3.x, R8.3).
- **Domain 04** — verify/eval stays the only completion authority; 07 enriches diagnosis, never
  turns failure into success (R2.3, R6.3).
- **Domain 05** — owns mission/lease/ACP transport, dispatch loop, model routing execution; 07
  supplies run/attempt allocator, cost sensing input, routing-recommendation metadata (R2.x,R4.x,R9.2).
- **Domain 06** — owns central redaction and key/policy digest; 07 sets fixture privacy policy and
  fails closed on missing trusted telemetry (R4.3, R5.2, R5.4).
- **Domain 10** — owns external adapter transport (OTel, provider-attested ingestion,
  provider/model identifiers); core stays offline/stdlib-only (R7.x, R8.x, R9.1).
