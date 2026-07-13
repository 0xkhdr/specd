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

## Attested usage ingestion

Adapters may emit `attestation/v1` envelopes containing canonical `TelemetryV1` JSON. Operator
config allowlists `key_id` and its verification key. Envelope pins payload with SHA-256 and signs
schema version, key ID, attestation reference, and digest with HMAC-SHA256. Core validates local
bytes only: unknown keys, changed payloads, signature mismatch, non-adapter provenance, and
attestation-reference mismatch fail closed. Key discovery, rotation transport, and asymmetric
provider signature translation remain adapter concerns.

Routing recommendations are provider-neutral policy metadata (`complexity` → declared routing
class). Core never resolves model availability or contacts a provider; adapters map classes to
models, and no mapping can bypass verify evidence.
