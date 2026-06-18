# Spec: Orchestration Configuration and Policy

Status: proposed — awaiting human review
Scope: backward-compatible configuration for deterministic Brain orchestration and host-executed Pinky workers.

## 1. Outcome

Add explicit orchestration policy without changing specd's defaults: zero LLM calls, zero runtime dependencies, manual phase approval, file-backed state, and fail-closed validation.

## 2. Product Boundary

- specd configures and enforces orchestration; it does not select an LLM provider or hold provider credentials.
- Brain is disabled by default and cannot bypass existing gates.
- Pinky token/cost telemetry is accepted only when reported by the external host; specd never invents estimates.
- Existing `config.json` files must load with byte-compatible behavior.

## 3. Requirements

- R1.1 Extend `core.Config` with optional `OrchestrationCfg`, preserving all current root fields.
- R1.2 Apply defaults per field after partial JSON decoding; malformed JSON retains current fail-safe behavior.
- R1.3 Validate enums and clamp bounded integers through one warning-producing helper.
- R1.4 Default to `enabled=false`, `approvalPolicy=manual`, `workerMode=host`, and `transport=file`.
- R1.5 Support session limits: maximum concurrent workers, retries, wall-clock duration, and optional host-reported cost.
- R1.6 Support lease, heartbeat, message TTL, and polling intervals with documented bounds.
- R1.7 Support program-level concurrency separately from task-worker concurrency.
- R1.8 Reject unsupported transports or worker modes instead of silently falling back.
- R1.9 Never persist secrets, API keys, shell fragments, or provider credentials in orchestration config.
- R1.10 Render the effective policy in deterministic `--json` status output with secrets structurally impossible.

## 4. Proposed Schema

```json
{
  "orchestration": {
    "enabled": false,
    "approvalPolicy": "manual",
    "workerMode": "host",
    "maxWorkers": 4,
    "maxRetries": 2,
    "sessionTimeoutMinutes": 120,
    "hostReportedCostLimitUSD": 0,
    "transport": {
      "kind": "file",
      "pollIntervalMillis": 500,
      "messageTTLSeconds": 3600,
      "leaseSeconds": 120,
      "heartbeatSeconds": 30
    },
    "program": {
      "maxConcurrentSpecs": 2
    }
  }
}
```

`approvalPolicy` values:

- `manual`: existing human approval semantics; default.
- `planning`: a user-started session may advance requirements/design/tasks after gates pass, but cannot clear mid-requirement or final verification approval.
- `session`: only when explicitly requested for a bounded session; every automated approval is audited.

## 5. Invariants

- V1 Existing config without `orchestration` behaves exactly as before.
- V2 Invalid orchestration policy fails closed before any worker is dispatched.
- V3 Config never authorizes direct source mutation; tasks and evidence still flow through existing commands.
- V4 Limits are enforced at dispatch time, not treated as advisory metadata.
- V5 Automated approval cannot clear `awaiting-approval` caused by high/critical mid-requirements.
- V6 JSON output is deterministic and contains no absolute paths, credentials, or environment dumps.
- V7 The embedded `config.json` decodes exactly to `DefaultConfig`; empty list fields serialize as `[]`, never `null`.

## 6. Interfaces

- `internal/core/specfiles.go`: models, defaults, decode, validation.
- `internal/core/embed_templates/config.json`: shipped defaults.
- Brain and program schedulers consume an immutable effective policy snapshot per decision cycle.
- CLI/MCP session overrides may only narrow configured authority or require explicit user consent to expand it.

## 7. Acceptance

- Legacy, partial, invalid, boundary, and round-trip tests pass under `-race -count=2`.
- Default `specd init` output remains deterministic.
- `make ci` passes with no external dependency added to `go.mod`.
