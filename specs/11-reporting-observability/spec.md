# Spec 11 — Reporting & Observability

> **Authoring order:** 11 / 12 · **Critical path:** no (pure projections; can trail)
> **Sources:** `fresh-start/11-reporting-observability.md`, paper p.30
> **ADRs:** ADR-8
> **Reference:** `reference/internal/cmd/{report,report_actions,status,dashboard,watch_webhook}.go`, `reference/internal/core/{report,report_metrics,prsummary,commitlink,telemetry,taskview,frontier}.go`, `reference/internal/obs/*`

`status` / `report` render the exact state of a spec (task progress, gate results, evidence,
cost/time telemetry) as deterministic Markdown / PR summary / Prometheus that a human or CI
reads to audit and approve. **No model in the path.**

---

## 1. Purpose & principles
- **Principles owned:** P7 (Deterministic Reporting — reports are projections of `state.json`,
  never LLM output), P1.
- **Paper concept:** *observability* — "Observing the Harness" (p.30): "the observability layer
  tracks token costs, latency, and agent drift, allowing human engineers to audit exactly why
  an agent made a specific decision."

## 2. Verdicts (with citations)

| Capability | Verdict | Why / reference |
|---|---|---|
| `status` + deterministic `report` (Markdown/PR/Prometheus) | **KEEP** | P7 core; cheap, auditable. `reference/internal/core/{report,prsummary,report_metrics}.go` |
| Projection purity (no LLM/network in render) | **KEEP, invariant** | ADR-8 |
| Live frontier streams (watch/SSE/webhook) | **DEFER** | Not MVP; keep `FrontierEvent` type so stream is additive |
| `dashboard` command | **DEFER → MERGE** into `report --dashboard` | Second rendering of same projection |
| `session_replay` / `trajectory` | **DEFER** with orchestration observability | Keep append-only `trajectory.jsonl` digest as evidence |
| `internal/obs` trace layer | **SIMPLIFY** to stdlib logger + metrics textfile | Defer build-tagged trace backend |

**Minimal surface:** `status <slug> [--json]`, `report <slug> [--pr|--metrics|--json]`; modules
`core/{report,prsummary,report_metrics,commitlink,telemetry,taskview}.go` + `frontier.go`
(event type retained).

## 3. Requirements (EARS)
- **R11.1** The system shall render every report as a pure function of `state.json` and the
  evidence ledger, invoking no language model and making no network call in the render path.
- **R11.2** When `report <slug>` is invoked, the system shall produce byte-stable output for a
  given state (golden-testable).
- **R11.3** When `--pr` is passed, the system shall render a PR summary including per-task
  status, gate results, and git commit links.
- **R11.4** When `--metrics` is passed, the system shall render a deterministic Prometheus
  textfile.
- **R11.5** When host-reported cost/latency telemetry exists, the system shall display it
  verbatim and labeled as reported (not validated) evidence.
- **R11.6** The system shall not require any live-streaming transport for its core reporting.

## 4. Design

### Module boundaries
- `core/report.go` builds one `ReportModel` from `State`; per-format renderers in
  `core/{prsummary,report_metrics}.go`; `cmd/{status,report}.go` are thin.
- **One projection, many formats:** all renderers consume a single `ReportModel` built once
  from `state.json`, so formats cannot disagree about the facts. Each renderer is a pure
  `Render(State) → bytes`, independently golden-tested.

### Key types
- `ReportModel` (facts), `PRSummaryTask`, `WaveTelemetry`, `FrontierEvent` (retained as the
  future stream contract).

### On-disk contracts
- Reads `state.json` + evidence ledger; writes nothing that isn't a projection. Optional
  append-only `trajectory.jsonl` digest as orchestration evidence.

### External interfaces
- JSON / Markdown / PR / Prometheus output formats; `FrontierEvent` as the future stream
  contract (streaming is an adapter over the same events, added later without touching the pure
  renderers).

## 5. Invariants preserved (ADR-8)
Projection purity (no LLM/network in render); byte-stable renders; telemetry verbatim.

## 6. Cross-domain dependencies
- Reads: Spec 02 (state), Spec 05 (evidence), Spec 09 (orchestration telemetry/trajectory).
- Depends on: Spec 10 (io for any written projection).

## 7. Risks & open questions
- **Risk:** deferring streams disappoints orchestration users wanting live progress. →
  `FrontierEvent` + append-only `trajectory.jsonl` retained, so a stream is additive later.
- **Decision:** commit report inputs needed for audit, including evidence and trajectory.
  Gitignore only transient stream state.
