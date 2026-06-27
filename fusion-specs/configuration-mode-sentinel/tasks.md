# Tasks — Configuration and Mode Sentinel

## Wave 1 — Strict config diagnostics
- [ ] T1 — Implement strict config loader
  - why: malformed config must not look like defaults to agents (Req 1)
  - role: builder
  - files: internal/core/config_validate.go (new), internal/core/specfiles.go
  - contract: parse config with diagnostics for invalid JSON, enum values, and clamped/out-of-range ints; preserve existing `LoadConfig` behavior.
  - acceptance: missing config is defaulted; invalid JSON reports field/path; no callers break.
  - verify: go test ./internal/core/ -run Config
  - depends: —
  - requirements: 1

- [ ] T2 — Validate fusion-relevant enums and ranges
  - why: policy must be trustworthy (Req 1)
  - role: builder
  - files: internal/core/config_validate.go
  - contract: validate `roles.subagentMode`, gate severities, verify sandbox, orchestration approval/worker/transport fields, MCP exposure, and numeric ranges already defined by constants.
  - acceptance: diagnostics include exact field paths and allowed values.
  - verify: go test ./internal/core/ -run ConfigStrict
  - depends: T1
  - requirements: 1

## Wave 2 — Fusion policy command
- [ ] T3 — Add `FusionPolicy` core model
  - why: shared JSON contract for policy summaries (Req 2,3,4)
  - role: builder
  - files: internal/core/fusion.go
  - contract: define policy summary, digest comparison result, spec mode summary, allowed-loop booleans, recommended command family, and diagnostics.
  - acceptance: no slug emits project policy only; slug emits spec mode policy.
  - verify: go test ./internal/core/ -run FusionPolicy
  - depends: T1
  - requirements: 2,3,4

- [ ] T4 — Implement `specd fusion policy`
  - why: agent can check binding constraints before acting (Req 2,3,4)
  - role: builder
  - files: internal/cmd/fusion.go, internal/core/commands.go
  - contract: support optional slug, `--json`, and `--expect-config-digest`; mismatch exits 1 with re-bootstrap recommendation.
  - acceptance: Base, orchestrated-capable, and orchestrated-without-capability cases produce correct booleans and recommendations.
  - verify: go test ./internal/cmd/ -run FusionPolicy
  - depends: T3
  - requirements: 2,3,4

## Wave 3 — Doctor and docs
- [ ] T5 — Add config sentinel to doctor
  - why: existing health check should surface config policy failures (Req 5)
  - role: builder
  - files: internal/cmd/doctor.go, internal/cmd/doctor_test.go
  - contract: include strict config diagnostics in text and JSON doctor output; `--fix` remains non-destructive for invalid custom config.
  - acceptance: malformed config makes doctor fail with clear diagnostics; valid defaults pass.
  - verify: go test ./internal/cmd/ -run Doctor
  - depends: T1, T2
  - requirements: 5

- [ ] T6 — Document policy-before-action protocol
  - why: agents must respect config and mode (Req 2,4)
  - role: builder
  - files: docs/agent-integration.md, internal/core/embed_templates/AGENTS.md
  - contract: document startup digest, `fusion policy <slug>`, Base vs Orchestrated loop selection, and subagentMode obligations.
  - acceptance: docs include exact commands and failure recovery.
  - verify: N/A
  - depends: T4
  - requirements: 2,3,4
