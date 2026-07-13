# Deployment adapter envelope

Deployment adapters are optional external programs or systems. `specd` core does not invoke them,
discover provider credentials, or make network calls. Domain 10 owns adapter execution and validates
its common `adapter/v1` result before passing Domain 08's deployment payload to core through stdin or
an explicitly named file.

## Envelope

`deployment-adapter/v1` is strict JSON. Unknown fields, trailing JSON values, unknown trust sources,
and input larger than 64 KiB fail closed.

```json
{
  "schema_version": "deployment-adapter/v1",
  "kind": "deployment.result",
  "idempotency_key": "provider-operation-id",
  "trust_source": "attested_ci",
  "attestation_ref": "sha256:external-attestation",
  "deployment": {
    "schema": "DeploymentV1",
    "deployment_id": "provider-deployment-id",
    "attempt": 1,
    "release_id": "release-id",
    "git_head": "git-head",
    "artifact_digest": "sha256:artifact",
    "environment": "production",
    "status": "started",
    "strategy": "canary",
    "population": "10%",
    "window": "10m",
    "adapter": "vendor/deploy@1",
    "authority": "ci:production",
    "actor": "ci",
    "idempotency_key": "provider-operation-id",
    "started_at": "2026-07-13T00:00:00Z",
    "telemetry_source": "",
    "evidence_ref": "sha256:evidence",
    "attestation_ref": "sha256:external-attestation"
  }
}
```

Allowed trust sources are `attested_ci`, `signed_runtime`, and `operator_file`. Attested CI and
runtime results require an external attestation reference. Trust and attestation labels remain
visible in the validated record; core does not claim to verify the referenced attestation until the
Domain 08 attestation contract is enabled.

## Credential and instruction boundary

Supply provider credentials only to the external adapter through its explicitly configured
execution environment. Never include credentials in the JSON. Strict decoding rejects credential
fields. Optional `message` is untrusted data, capped at 1 KiB and redacted when it contains a known
credential or instruction-like/multiline prose. It is never added to `AGENTS.md`, roles, steering,
task context, or standing instructions.

## Idempotency

Core validates and appends under the per-spec lock. Replaying the same idempotency key and payload
returns `noop`. Reusing a key with a different payload returns `conflict`, appends no second
deployment, and appends only key, trust source, and existing/incoming SHA-256 digests to
`deployment-conflicts.jsonl`. Both audit facts remain durable without echoing hostile or
credential-bearing content.
