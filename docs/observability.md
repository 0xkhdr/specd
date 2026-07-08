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

## Crash-safety

The opt-in Brain's ACP ledger is append-only and crash-safe: an interrupted append replays to a
consistent state on the next load. This is proven by `scripts/stress-acp.sh` and
`scripts/stress-checkpoint-fault.sh` (fault-injected interrupted writes), wired into CI. See
[TESTING.md](../TESTING.md) for the stress jobs.

---

**See also:** [command-reference.md](command-reference.md) · [troubleshooting.md](troubleshooting.md)
· [contributor-guide.md](contributor-guide.md)
