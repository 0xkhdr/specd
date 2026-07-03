# Domain: Reporting & Observability

## 1. Purpose & value mapping
- **Principles served:** P7 (Deterministic Reporting — reports are projections of
  `state.json`, never LLM output), P1.
- **Paper concept realized:** *observability* — the "Observing the Harness" phase (p.30):
  "the observability layer tracks token costs, latency, and agent drift, allowing human
  engineers to audit exactly why an agent made a specific decision." Reporting is how the
  human stays in control of the delegated 80% without watching every keystroke.
- **Core use case:** `specd status` / `specd report` render the exact state of a spec (task
  progress, gate results, evidence, cost/time telemetry) as deterministic Markdown / PR
  summary that a human or CI reads to audit and approve. No model in the path.
- **If none → CUT:** N/A for the core projections; live streams are triaged below.

## 2. Current-state analysis (from specd)
- **Reference files read:** `internal/cmd/{report.go,report_actions.go,status.go,
  dashboard.go,watch_webhook.go}`, `internal/core/{report.go,report_metrics.go,
  prsummary.go,commitlink.go,telemetry.go,taskview.go,frontier.go,session_replay.go,
  trajectory.go}`, `docs/dashboard.md`, `internal/obs/*`.
- **What exists today; key contracts/invariants:**
  - Deterministic projections: `status.go` (spec status), `report.go` + `prsummary.go`
    (`PRSummaryTask`, `commitlink.go` git links), `report_metrics.go`
    (`RenderPrometheusMetrics`, deterministic textfile), `telemetry.go` (`WaveTelemetry`
    aggregated cost/timing), `taskview.go` (doc-overrides-state projection).
  - **Live surfaces:** `report_actions.go` (611 LOC) includes `runWatch`/`runWatchSSE`
    frontier streaming; `watch_webhook.go` (`webhookSink`) pushes events. `frontier.go`
    emits `FrontierEvent`.
  - `dashboard.go` — a unified read-only project dashboard projection.
  - `session_replay.go` (`SessionTimelineEvent`) + `trajectory.go` (digest-only
    `trajectory.jsonl`) — human-facing orchestration replay/observability.
  - **Invariant:** every report is a pure projection of `state.json` — no LLM, no network in
    the render path (host telemetry is stored verbatim as evidence, never synthesized).
- **Redundancy / complexity / drift found (evidence):**
  - `report_actions.go` bundles deterministic reports *and* three live transports
    (watch/SSE/webhook) in one 611-LOC file — the biggest reporting complexity, most of it
    live-streaming infrastructure.
  - `dashboard` duplicates aggregation that `report`/`status` already produce; it is a second
    rendering of the same projection.
  - `internal/obs/*` (log/metrics/trace) adds an observability layer whose trace path is
    build-tagged — extra surface for the default binary.

## 3. Fresh-start decision
- **Verdict per capability:**
  - `status` + deterministic `report` (Markdown, PR summary, Prometheus textfile) — **KEEP**
    (P7 core; cheap, high-value, auditable).
  - Projection purity (no LLM/network in render) — **KEEP, invariant.**
  - Live frontier streams (watch/SSE/webhook) — **DEFER**: not MVP; a `report watch`
    follow-up once orchestration is exercised in anger. Keep `FrontierEvent` as a type so the
    stream can be added without a rewrite.
  - `dashboard` command — **DEFER → MERGE** into `report --dashboard` if demand appears.
  - `session_replay`/`trajectory` — **DEFER** with the orchestration observability (they only
    matter once Brain sessions run; keep the append-only `trajectory.jsonl` digest as
    evidence, defer the human replay renderer).
  - `internal/obs` trace layer — **SIMPLIFY**: keep a minimal stdlib logger + the
    deterministic metrics textfile; defer the build-tagged trace backend.
- **Minimal accurate surface:**
  - Commands: `status <slug> [--json]`, `report <slug> [--pr|--metrics|--json]`.
  - Modules: `core/{report,prsummary,report_metrics,commitlink,telemetry,taskview}.go`;
    `frontier.go` (event type retained).
  - On-disk: reads `state.json` + evidence ledger; writes nothing that isn't a projection.
- **Architecture & flexibility improvements:**
  - **Renderers are pure functions** `Render(State) → bytes` per format (markdown / pr /
    prometheus / json), each independently golden-tested for byte-stability.
  - **One projection, many formats:** all renderers consume a single `ReportModel` built once
    from `state.json`, so formats cannot disagree about the facts.
  - **Streaming is an adapter over the same events**, added later without touching the pure
    renderers — the deferral costs nothing architecturally.

## 4. Requirements (EARS-shaped) — seed for requirements.md
1. The system shall render every report as a pure function of `state.json` and the evidence
   ledger, invoking no language model and making no network call in the render path.
2. When `report <slug>` is invoked, the system shall produce byte-stable output for a given
   state (golden-testable).
3. When `--pr` is passed, the system shall render a PR summary including per-task status,
   gate results, and git commit links.
4. When `--metrics` is passed, the system shall render a deterministic Prometheus textfile.
5. When host-reported cost/latency telemetry exists, the system shall display it verbatim and
   labeled as reported (not validated) evidence.
6. The system shall not require any live-streaming transport for its core reporting.

## 5. Design notes — seed for design.md
- **Module boundaries:** `core/report.go` builds `ReportModel` from `State`; per-format
  renderers in `core/{prsummary,report_metrics}.go`; `cmd/{status,report}.go` are thin.
- **Key types:** `ReportModel` (facts), `PRSummaryTask`, `WaveTelemetry`, `FrontierEvent`
  (retained for future stream).
- **Data/on-disk contracts:** reads `state.json` + evidence; optional `trajectory.jsonl`
  (append-only digest) as orchestration evidence.
- **Invariants to preserve:** projection purity; byte-stable renders; telemetry verbatim.
- **External interfaces:** JSON/Markdown/PR/Prometheus output formats; `FrontierEvent` as the
  future stream contract.

## 6. Proposed task DAG — seed for tasks.md

### Wave 1 — projection & renderers
| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T11.1 | craftsman | `internal/core/report.go` | — | `go test ./internal/core -run TestReportModel` | one model built from state |
| T11.2 | craftsman | `internal/cmd/status.go` | T11.1 | `go run . status demo --json` | pure state projection |
| T11.3 | craftsman | `internal/core/prsummary.go`, `internal/core/commitlink.go` | T11.1 | `go test ./internal/core -run TestPRSummaryGolden` | byte-stable PR summary |
| T11.4 | craftsman | `internal/core/report_metrics.go` | T11.1 | `go test ./internal/core -run TestMetricsGolden` | deterministic Prometheus textfile |
| T11.5 | validator | `internal/core/report_purity_test.go` | T11.1 | `go test ./internal/core -run TestNoLLMInRender` | no model/network import in render path |

## 7. Risks, open questions, cross-domain dependencies
- **Risk:** deferring streams disappoints orchestration users who want live progress.
  Mitigation: `FrontierEvent` + append-only `trajectory.jsonl` are retained, so a stream is
  additive later.
- **Open question:** commit the evidence/trajectory files to git, or gitignore? Proposed:
  commit report inputs (auditable), gitignore transient stream state.
- **Cross-domain deps:** reads domain 02 (state), domain 05 (evidence), domain 09
  (orchestration telemetry/trajectory); domain 10 (io for any written projection).
