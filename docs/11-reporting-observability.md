# Reporting & Observability

This document details the design of the reporting and observability system in `specd` (v2), describing the pure projection architecture, output formats, and telemetry data.

---

## 1. Pure Projection Architecture (P7)

To ensure maximum speed, security, and reproducibility, all report formats (Markdown summaries, PR descriptions, metrics) are built as **pure projections of `state.json` and the evidence ledger**.

*No-LLM Invariant:* The rendering pipeline performs **no model calls and no network requests**. Human-facing logs and statistics are formatted directly from local file data.

*Origin:* Simplified from [report_actions.go](file:///var/www/html/rai/up/specd/reference/internal/cmd/report_actions.go).

---

## 2. Output Formats

The `specd report` command formats metrics and status data into different structures based on flags:

```bash
specd report <slug> [--pr | --metrics | --json]
```

### A. Pull Request Summary (`--pr`)
Produces a Markdown summary suitable for git commit messages or GitHub/GitLab Pull Request descriptions. Includes:
*   Spec status and phase transition progress.
*   Per-task execution status (checkboxes mapped to evidence records).
*   Pluggable validation gate results.
*   Git commit hashes and links derived from verification records via [commitlink.go](file:///var/www/html/rai/up/specd/reference/internal/core/commitlink.go).

### B. Prometheus Textfile (`--metrics`)
Renders a raw text metrics file conforming to Prometheus exposition standards. Contains metrics for:
*   Total tasks complete vs pending.
*   Validation gate failure counts.
*   Total verification duration (in milliseconds).
*   Host-reported LLM API costs.

*Origin:* Metric formatting from [report_metrics.go](file:///var/www/html/rai/up/specd/reference/internal/core/report_metrics.go).

---

## 3. Telemetry Integrity

When orchestration workers execute tasks, they report latency and token usage metrics.
*   **Unverified Evidence:** These metrics are stored verbatim in verification records.
*   **Audit-Only:** Telemetry is displayed in reports with explicit labels indicating it is "host-reported" (untrusted) data rather than verified facts. It is not used to evaluate task completion.

*Origin:* Preserved from [telemetry.go](file:///var/www/html/rai/up/specd/reference/internal/core/telemetry.go).

---

## 4. Deferred Live Surfaces

To keep the MVP lightweight, several complex streaming components have been deferred:

*   **Live Webhook & SSE Streaming:** Commands pushing events to remote webhooks ([watch_webhook.go](file:///var/www/html/rai/up/specd/reference/internal/cmd/watch_webhook.go)) or server-sent event streams are **DEFERRED**.
*   **Visual Dashboard:** The standalone `dashboard` command has been **DEFERRED** and will return in v2 as `report --dashboard`.
*   **Session Replays:** The human-facing CLI session replay timeline is **DEFERRED**, though the underlying `trajectory.jsonl` log file continues to be appended to for future consumption.
