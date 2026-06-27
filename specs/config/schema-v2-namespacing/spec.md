# Spec — V2 Schema Namespacing and Compatibility

**Priority:** P0 · **Wave:** 1 · **Domain:** config schema evolution.

## Introduction

The action plan proposes a clearer YAML-facing schema under semantic namespaces (`defaults`, `gates`, `verify`, `orchestration`, `mcp`) while preserving legacy camelCase JSON behavior. This spec defines how specd accepts v2 YAML without breaking existing v1 `.specd/config.json` files or internal callers that already use `Config.DefaultVerify`, `Config.Report`, and related structs.

## Current-state grounding

- Current config fields are a v1 JSON shape in `internal/core/specfiles.go`.
- Existing nested structs already cover `report`, `roles`, `gates`, `verify`, `orchestration`, `mcp`, and resilience options.
- The proposed YAML schema moves top-level `defaultVerify`, `report.format`, `roles.subagentMode`, and `promotionThreshold` under `defaults` with snake_case names.
- YAML also uses human-friendly transport units (`poll_interval_seconds`, `message_ttl_minutes`) while the runtime struct uses millis/seconds.

## Requirements

### Requirement 1 — V2 YAML namespacing
**User story:** As a config author, I want related settings grouped under meaningful namespaces.

**Acceptance criteria:**
1. THE SYSTEM SHALL accept `version: 2` YAML configs with `defaults.verify_command`, `defaults.report_format`, `defaults.subagent_mode`, and `defaults.promotion_threshold`.
2. THE SYSTEM SHALL accept snake_case YAML names throughout v2 config.
3. THE SYSTEM SHALL map v2 fields into the effective runtime `Config` used by existing command/core code.
4. THE SYSTEM SHALL not require project configs to repeat inherited/default fields.

### Requirement 2 — V1 JSON backward compatibility
**User story:** As an existing user, I want my `.specd/config.json` to keep working.

**Acceptance criteria:**
1. THE SYSTEM SHALL parse current v1 JSON keys (`defaultVerify`, `report.format`, `roles.subagentMode`, etc.).
2. THE SYSTEM SHALL preserve the effective runtime behavior of valid v1 config files.
3. THE SYSTEM SHALL keep JSON tags needed by tests and machine-readable policy output.
4. THE SYSTEM SHALL warn/deprecate legacy project JSON only in user-facing diagnostics, not fail it.

### Requirement 3 — Human-friendly time units
**User story:** As a YAML author, I want to write seconds/minutes instead of millisecond-heavy transport settings.

**Acceptance criteria:**
1. YAML SHALL accept `orchestration.transport.poll_interval_seconds` and convert to runtime `PollIntervalMillis`.
2. YAML SHALL accept `orchestration.transport.message_ttl_minutes` and convert to runtime `MessageTTLSeconds`.
3. JSON SHALL continue to accept `pollIntervalMillis` and `messageTTLSeconds` without conversion ambiguity.
4. Fractional seconds SHALL be handled deterministically for poll intervals, with documented rounding behavior.

### Requirement 4 — Validation parity
**User story:** As an agent, I want v2 YAML to be validated as strictly as v1 JSON.

**Acceptance criteria:**
1. Strict config validation SHALL check enum values and ranges for both v1 and v2 input.
2. Diagnostics SHALL report user-facing field paths in the source schema style when possible.
3. Unknown keys SHALL produce warnings at minimum; authority-bearing unknown keys SHOULD fail strict validation if they could imply unsupported behavior.
4. Secret-bearing orchestration config SHALL remain rejected fail-closed.

### Requirement 5 — Embedded defaults are authoritative
**User story:** As a maintainer, I want shipped defaults to match code defaults.

**Acceptance criteria:**
1. THE SYSTEM SHALL add an embedded `config.yml` template representing v2 defaults with comments.
2. Tests SHALL assert that the embedded YAML defaults decode to `DefaultConfig` or the documented v2 default equivalent.
3. `DefaultConfig` SHALL remain the in-code authoritative runtime value unless a separate generated-default mechanism is explicitly implemented.

## Design

- Prefer an internal runtime `Config` shape that minimizes caller churn. Add YAML-specific decode structs or custom unmarshal logic to translate v2 YAML into the existing runtime shape.
- If restructuring `Config` itself, provide compatibility accessors or migration updates for every caller.
- Use pointer-backed partial structs for v1 and v2 decode so merge can distinguish absent from zero.
- Validate both source fields and effective runtime fields. Effective validation catches merged invalid values; source validation catches typos and unknown keys.
- Keep v2 schema comments in `internal/core/embed_templates/config.yml` and keep `config.json` as a legacy fixture/template only if tests or migration need it.

## Out of scope

- Config file discovery and cascading; covered by `yaml-loader-cascade`.
- Writing new project/global config files; covered by `scaffold-global-init`.
- Migration CLI; covered by `migrate-config`.

## Risks

- **Two schema shapes can diverge:** Build tests that decode v1 JSON and equivalent v2 YAML to identical runtime config.
- **Zero-value ambiguity:** Never merge decoded concrete structs without presence information.
- **Rounding surprises:** Specify and test fractional-second conversion.
