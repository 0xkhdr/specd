# Tasks — V2 Schema Namespacing and Compatibility

## Wave 1 — Schema translation
- [x] T1 — Define v2 YAML decode model
  - why: v2 config needs snake_case names without destabilizing runtime callers (Req 1,2)
  - role: builder
  - files: internal/core/config_schema.go (new) or internal/core/config_loader.go, internal/core/specfiles.go
  - contract: add YAML-facing partial structs for `defaults`, `gates`, `verify`, `orchestration`, `mcp`; translate to the runtime `Config`/partial config model.
  - acceptance: minimal v2 YAML with only `defaults.verify_command` produces effective `DefaultVerify`; omitted fields remain absent for merge.
  - verify: go test ./internal/core -run 'ConfigSchema|ConfigLoader'
  - depends: yaml-loader-cascade/T2
  - requirements: 1,2

- [x] T2 — Add YAML tags or custom unmarshal mapping
  - why: YAML field names must be stable and documented (Req 1)
  - role: builder
  - files: internal/core/specfiles.go, internal/core/config_schema.go
  - contract: ensure all public config fields support documented snake_case YAML names; avoid relying on yaml.v3 implicit field-name heuristics.
  - acceptance: sample global/project YAML from the action plan decodes without unknown-key warnings.
  - verify: go test ./internal/core -run ConfigSchema
  - depends: T1
  - requirements: 1

- [x] T3 — Preserve v1 JSON behavior
  - why: legacy projects must not break (Req 2)
  - role: builder
  - files: internal/core/config_loader.go, internal/core/specfiles_test.go, internal/core/config_loader_test.go
  - contract: parse existing `config.json` shape exactly; maintain JSON tags and permissive `LoadConfig` wrapper semantics.
  - acceptance: current config fixture tests pass; equivalent v1 JSON and v2 YAML produce equal effective runtime configs.
  - verify: go test ./internal/core -run 'Specfiles|Config.*Legacy|Backward'
  - depends: T1
  - requirements: 2

## Wave 2 — Units and validation
- [ ] T4 — Implement human-friendly transport unit conversion
  - why: YAML should be readable while runtime stays in current units (Req 3)
  - role: builder
  - files: internal/core/config_schema.go, internal/core/config_loader_test.go
  - contract: convert `poll_interval_seconds` to `PollIntervalMillis`; convert `message_ttl_minutes` to `MessageTTLSeconds`; document and test fractional second rounding.
  - acceptance: `poll_interval_seconds: 0.5` becomes `500`; `message_ttl_minutes: 60` becomes `3600`; JSON millis/seconds remain unchanged.
  - verify: go test ./internal/core -run 'Config.*Time|ConfigSchema'
  - depends: T2
  - requirements: 3

- [ ] T5 — Extend strict validation to v2 paths
  - why: YAML must fail/warn with same safety as JSON (Req 4)
  - role: builder
  - files: internal/core/config_validate.go, internal/core/config_loader.go, internal/core/config_validate_test.go
  - contract: validate report format, subagent mode, gate severities, verify sandbox, orchestration policy/worker/transport/program/resilience values, MCP exposure, and numeric ranges for merged config; include source-style field paths.
  - acceptance: invalid v2 YAML reports e.g. `orchestration.transport.poll_interval_seconds`; invalid v1 JSON reports e.g. `orchestration.transport.pollIntervalMillis`.
  - verify: go test ./internal/core -run 'ConfigStrict|ConfigValidate'
  - depends: T3,T4
  - requirements: 4

- [ ] T6 — Detect unknown keys in YAML/JSON configs
  - why: typos in config should not silently become defaults (Req 4)
  - role: builder
  - files: internal/core/config_loader.go, internal/core/config_validate.go, internal/core/config_validate_test.go
  - contract: add unknown-key detection per schema version; warnings for harmless unknowns; fail strict validation for authority-bearing unknown sections if policy chooses fail-closed.
  - acceptance: `orchestration.max_workerz` emits a source/path diagnostic; permissive `LoadConfig` remains backward compatible.
  - verify: go test ./internal/core -run 'Unknown|ConfigStrict'
  - depends: T5
  - requirements: 4

## Wave 3 — Embedded default template
- [ ] T7 — Add commented `embed_templates/config.yml`
  - why: new human-facing defaults should be readable and scaffolded (Req 5)
  - role: builder
  - files: internal/core/embed_templates/config.yml, internal/core/embed.go
  - contract: create v2 YAML default template with comments; embed it; keep legacy `config.json` if required for compatibility tests or migration output.
  - acceptance: template contains no secrets, uses v2 names, and documents global/project override behavior briefly.
  - verify: go test ./internal/core -run EmbeddedConfig
  - depends: T2,T4
  - requirements: 5

- [ ] T8 — Assert embedded YAML defaults match runtime defaults
  - why: shipped template and code defaults must not drift (Req 5)
  - role: verifier
  - files: internal/core/specfiles_test.go, internal/core/config_loader_test.go
  - contract: decode embedded `config.yml` with the same loader path and compare to `DefaultConfig` after documented normalization.
  - acceptance: test fails on template/code drift; default-off additive fields with `omitempty` are handled explicitly.
  - verify: go test ./internal/core -run EmbeddedConfig
  - depends: T7
  - requirements: 5
