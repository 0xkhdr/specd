# Telemetry adapter contract

`specd report <spec> --format event` emits newline-delimited `event/v1` records. Rendering is a
pure local projection: core performs no network call and imports no telemetry SDK. External
adapters may read this stream and map it to OpenTelemetry.

## `event/v1`

Required fields are `schema_version`, `event_id`, `spec_id`, and `kind`. Correlation fields are
`run_id`, `span_id`, `parent_span_id`, `task_id`, and `attempt`. Privacy/provenance fields are
`telemetry_source`, `attestation_ref`, `evidence_ref`, and sorted `redactions`. Effect-bearing
`edit`, `verify`, and `eval` events require `git_head`. Unknown schema versions and non-namespaced
unknown kinds fail closed.

Schema intentionally has no prompt, response, chain-of-thought, source-content, raw-output, or
free-form attribute field. References must be workspace-relative or content-addressed.

## OpenTelemetry mapping

| `event/v1` | OpenTelemetry |
|---|---|
| `run_id` | trace ID / `specd.run_id` |
| `span_id` | span ID |
| `parent_span_id` | parent span ID |
| `kind` | span name / `specd.kind` |
| `timestamp` | event timestamp (informational) |
| `spec_id`, `task_id`, `attempt`, `status` | `specd.*` attributes |
| `telemetry_source`, refs, `redactions` | bounded `specd.*` attributes |

Adapter round trips preserve correlation and privacy fields exactly. Adapter tests use local
fixtures with networking disabled. Transport, endpoints, credentials, sampling, and export
retries belong to external adapter policy—not core.
