# Spec — Config Corruption & Secret-Diagnostic Matrix (A6)

**Priority:** P2 · **Wave:** 2 · **Domain:** config robustness / secret hygiene.

## Introduction

Config migration (`migrate-config`) and format precedence
(`SPECD_CONFIG_FORMAT`) are well-tested for the happy path. Two gaps remain:
(C-1) no explicit negative/corruption matrix — truncated YAML, duplicate JSON
keys, and dual `config.json` + `config.yml` presence; and (C-2) the env-override
diagnostic path is not unit-tested to never echo a value for a key whose *name*
matches a secret pattern.

This spec adds the corruption matrix and the secret-name diagnostic guard.

## Current-state grounding

- Config cascade: embedded defaults → global → project → `SPECD_*` env → validate;
  secret-bearing keys rejected; byte-identical output via `omitempty`.
- `specs/config/docs-test-hardening`, `migrate-config`, `env-precedence` —
  happy-path coverage exists.
- `doctor` exists for health diagnostics.
- `internal/config/` (cascade + validation + diagnostics).

## Requirements

### Requirement 1 — Corruption matrix
**User story:** As a user, I want corrupt config to fail loudly, so I am never
silently running on a half-parsed file.

**Acceptance criteria:**
1. Truncated YAML mid-document SHALL be rejected with a clear error (no partial
   apply).
2. JSON with duplicate keys SHALL be rejected or its resolution SHALL be defined
   and tested (which key wins).
3. Each corruption case SHALL have a regression test.

### Requirement 2 — Dual-file conflict is announced
**User story:** As a user, I want to be told when both `config.json` and
`config.yml` exist, so a stale file does not silently win.

**Acceptance criteria:**
1. When both files are present, precedence SHALL be deterministic and tested.
2. `doctor` SHALL flag the dual-file case rather than silently picking one.

### Requirement 3 — Secret-name diagnostic never echoes value
**User story:** As a user, I want diagnostics to never print the value of a
secret-named key, so secrets do not leak to logs.

**Acceptance criteria:**
1. The env-override diagnostic path SHALL be unit-tested to never print the value
   for a key whose name matches a secret pattern.
2. The rejection message for secret keys SHALL name the key but NOT its value.

## Design

- Add a table-driven negative test in `internal/config/` covering truncated YAML,
  duplicate JSON keys, and dual-file presence.
- Add/confirm a `doctor` check that detects both config files and reports the
  conflict + the winner.
- Add a focused test feeding a `SPECD_<SECRET>` override through the diagnostic
  path and asserting the value never appears in output.

## Out of scope

- Encrypting config at rest.
- Schema redesign (v2 is settled).

## Risks

- **Defining duplicate-JSON-key behavior:** Go's decoder takes last value; make
  the test assert the *documented* choice rather than inventing new behavior.
