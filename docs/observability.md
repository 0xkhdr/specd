# specd — Observability

Everything specd reports is a **pure function of on-disk `.specd/` state** — `state.json`, the
verify evidence ledger, the criteria ledger, the submission ledger, and the opt-in ACP ledger.
**No LLM is in any report path**, and reports never mutate state. Two runs over the same tree
produce byte-identical output.

## Logging & telemetry strategy

specd has **no log levels and no verbosity flags** — deliberately. It is a fail-closed CLI, not a
long-running daemon:

- **stdout** carries the requested artifact (a report, a manifest, a rendered status).
- **stderr** carries the single actionable diagnostic when a command fails; the process then
  exits non-zero. See [troubleshooting.md](troubleshooting.md) for the exit-code discipline
  (`0` success, `1` gate/verify failure, `2` usage / fail-closed rejection).
- **Nothing phones home.** specd emits no network telemetry. "Telemetry" here means only the
  worker-reported metrics an agent chooses to record on the evidence ledger (below).

Because output is deterministic and state-derived, the observability surface *is* the report
verbs — there is no separate logging subsystem to configure.

## Reporting surfaces

| Command | Output | Notes |
|---|---|---|
| `specd report <slug>` | Human status + criterion coverage | Default render. |
| `specd report <slug> --json` | `ReportModel` + criteria as JSON | Stable schema; one object. |
| `specd report <slug> --metrics` | Task-count metrics + telemetry totals | See below. |
| `specd report <slug> --pr` | PR-comment summary | Consumed by the GitHub action. |
| `specd report <slug> --history` | Ordered audit trail | Timestamp order; deterministic tie-break. |
| `specd report <slug> --history --json` | One JSON `HistoryEvent` per line | JSONL, replayable. |
| `specd report <slug> --format prometheus` | node_exporter textfile exposition | See below. |
| `specd status <slug> --program` | Cross-spec program roll-up | Deterministic. |
| `specd context <slug> <task> --hud` | Operator HUD for one task's context | Byte-for-byte from the manifest. |

### History ordering

`report --history` folds every ledger into one trail sorted by **timestamp**, with a
deterministic `(SourceRank, Seq)` tie-break so events sharing a timestamp — or missing one —
still order identically on every run. Event kinds: `approval`, `decision`, `midreq`,
`verify:pass`/`verify:fail`, `completion`, `criterion:*`, `submission`, `acp:*`.

## Where worker `--tokens` / `--cost` / `--duration-ms` surface

An agent may annotate a `verify` or `task complete` invocation with `--tokens`, `--cost`, and
`--duration-ms`. specd **stores these verbatim on the evidence record and never computes or
estimates them** — a task that reports nothing is shown as absent, not zero-imputed. They surface,
summed across a spec's verify attempts, in two places:

- **`report --metrics`** — after the `specd_tasks_*` counts, a per-spec and per-task telemetry
  block (`RenderTelemetry`).
- **`report --format prometheus`** — as counter series, cost summed with exact-decimal math and
  duration converted to seconds without float rounding:
  - `specd_worker_tokens_total{spec="<slug>"}`
  - `specd_worker_cost_total{spec="<slug>"}`
  - `specd_worker_duration_seconds_total{spec="<slug>"}`

Prometheus metric **names are an API** — the full family list and naming contract live in
[command-reference.md](command-reference.md); renaming one breaks dashboards, so they must not
churn.

### One metrics export format

**Prometheus text exposition (`report --format prometheus`) is the one supported metrics export
format.** node_exporter textfile output is a file drop, which matches specd's zero-runtime-dependency,
no-daemon posture: nothing to run, nothing to connect to.

**`report --format otel` is deprecated and scheduled for deletion in a follow-up spec.** It has no
consumer: `internal/adapter/otel_export.go` is reached only from `internal/cmd/report_otel.go`, and
every other mention in the tree is its own tests, the command palette
(`internal/core/commands.go`), or generated docs. Nothing in `scripts/`, `.github/`, or
`embed_templates/` drives it. It is also redundant with the documented integration path — the
adapter contract already specifies OTel export as an *external adapter* concern mapping the neutral
`event/v1` JSONL stream (see [adapters/telemetry.md](adapters/telemetry.md)), so a second, built-in
span projection is a duplicate schema with no reader. `--format event` is unaffected: it is that
neutral stream and stays.

Until the deletion lands, `--format otel` remains callable; do not build on it.

## Current limits (P0 honesty baseline)

The observability surface is deliberately narrow today. State these limits plainly so operators
do not over-trust the numbers; the P0 route below is the remediation path (Domain 07 waves W1–W9):

- **History is an audit projection, not a full execution trace.** `report --history` folds the
  on-disk ledgers into an ordered, replayable trail — it is not a span tree and carries no
  run/span correlation identity. A versioned run envelope with correlated attempts lands in
  W1–W2.
- **Telemetry is worker-reported, never measured.** `--tokens`/`--cost`/`--duration-ms` are
  values a worker chooses to record; specd stores them verbatim and computes nothing. The
  `tokens` field is a single conflated scalar (no input/output/cache split) and `cost` is a bare
  decimal with no currency. Provenance and typed decimals land in W1.
- **The cost brake is dormant unless armed.** The orchestration cost limit only halts when
  `MaxCost` is a positive value; an unset limit is a silently disabled brake, not a guarantee.
  An honest, explicit cost brake lands in W4.
- **The context budget bounds size, it does not prove sufficiency.** `ManifestBudget` estimates
  from declared item token counts and never reads the referenced files, so it can underestimate
  the real payload and says nothing about whether the context is *enough*. Measured accounting
  and a sufficiency signal land in W3.

**P0 route:** these are tracked as the Domain 07 W0 contract baseline; each limit above names the
wave that closes it. Until then, treat reported cost/tokens as advisory, not authoritative.

## Privacy and cardinality policy (W5)

The measurement surface is privacy-preserving and bounded-cardinality by
construction:

- **Telemetry is metadata-only.** The `telemetry` object holds only counts,
  cost, duration, provenance, currency, an optional attestation pointer, and a
  schema version — never a prompt, response, chain-of-thought, file content, or
  raw output. Its one free-form field (`attestation_ref`) is scrubbed by the
  central redactor before it reaches the ledger, so a secret or absolute home
  path cannot leak. See [telemetry-schema.md](telemetry-schema.md).
- **Metric labels are allowlisted.** Only `spec`, `status`, `verdict`, and
  `task` may label a metric series. Run/mission/commit/path/model/actor/error
  correlation stays in the trace JSONL, never a label — a label added outside the
  allowlist fails a static contract test.
- **Evidence references stay inside the workspace.** `evidence_ref` must be
  workspace-relative or content-addressed; a URL, absolute path, or `..`
  traversal is rejected in the core schema on both append and decode.

## Crash-safety

The opt-in Brain's ACP ledger is append-only and crash-safe: an interrupted append replays to a
consistent state on the next load. This is proven by `scripts/stress-acp.sh` and
`scripts/stress-checkpoint-fault.sh` (fault-injected interrupted writes), wired into CI. See
[TESTING.md](../TESTING.md) for the stress jobs.

---

**See also:** [command-reference.md](command-reference.md) · [troubleshooting.md](troubleshooting.md)
· [contributor-guide.md](contributor-guide.md)
