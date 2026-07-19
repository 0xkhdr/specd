# Telemetry schema and privacy policy

Driver: `TestPrometheusLabelAllowlist` (`internal/core/prometheus_test.go:128`) — fails if any renderer emits a label outside `MetricLabelAllowlist` or the allowlist admits a forbidden high-cardinality key.

specd telemetry is **metadata-only by construction**. The on-disk `telemetry`
object on an evidence record (`internal/core/telemetry.go`, type `Annotations`)
carries only these keys — there is no field that can hold a prompt, a model
response, a chain-of-thought, file contents, or raw worker output:

| key | type | meaning |
|---|---|---|
| `tokens` | int | worker-reported token count (accounting hint, not measured) |
| `cost` | decimal string | worker-reported cost; exact-decimal, never float |
| `duration_ms` | int | worker-reported wall-clock milliseconds |
| `telemetry_source` | enum | `worker` \| `provider_adapter` \| `operator` (trust provenance) |
| `currency` | string | ISO currency paired with `cost` on canonical records |
| `attestation_ref` | string | optional pointer to an external provider attestation |
| `envelope_version` | string | `v1`, required on every record; any other value fails closed |

A default fixture is therefore metadata-only: none of prompt / response /
chain-of-thought / file content / raw output / secret / absolute home path can
appear, because the schema has nowhere to put them (spec 07 R5.2).

## Central redaction before display

The one free-form field, `attestation_ref`, is routed through the same central
redactor that guards `command` and `evidence_ref` (`internal/core/verify`
`Redactor`) before a record is written, so a secret or absolute home path
smuggled into it is scrubbed before it can reach the ledger or any report
(R5.2/R5.4). The redactor masks credential patterns and collapses an absolute
home directory (`/home/<u>`, `/Users/<u>`, `/root`) to `~`.

## Metric label cardinality allowlist

Metrics are a bounded-cardinality surface. Only the label keys in
`core.MetricLabelAllowlist` — `spec`, `status`, `verdict`, `task` — may appear on
any series. High-cardinality or sensitive correlation (run, mission, commit/SHA,
path, model, actor, error) is **never** a metric label; it lives in the trace
JSONL instead, where a distinct value does not mint a new time series (R5.1). A
label added outside the allowlist fails the static contract test
`TestPrometheusLabelAllowlist`.

## Evidence reference locator policy

`evidence_ref` on an evidence record must be **workspace-relative or
content-addressed** — never a URL, an absolute path, or a parent-directory
(`..`) traversal. Core never dereferences a ref, but refuses to store or decode
one that points outside the workspace or off to the network (R5.3). See
`SECURITY.md` and `docs/observability.md`.
