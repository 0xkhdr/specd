# Design — Regression: Packs, Schema, Templates (apply/resolve/validate)

## Overview
Lock the pack/schema/template contract with golden-output tests and schema-validation
sweeps. Existing tests (pack_test.go, pack_resolve_test.go, schema_validate_test.go,
initpack_test.go) are extended so that every generated artifact and every embedded template
is validated, and pack application is proven deterministic and digest-safe.

## Architecture
```
embed_packs/  --> pack_apply.go --> target/.specd     (deterministic, --force guarded)
embed_templates/ --> new.go --> six artifacts
schema/ (embedded) <-- schema_validate sweep <-- generated artifacts + templates
remote --pack URL --> sha256 verify --> apply | refuse
```

## Components and interfaces
- **pack.go / pack_apply.go / pack_resolve.go** — load, resolve, apply. Contract:
  deterministic, no overwrite without --force, unknown fields surfaced.
- **schema/ + schema_validate_test.go** — validate artifacts and templates.
- **pack_resolve (remote)** — digest pinning. Contract: verify sha256 before apply.

## Data models
Pack = manifest + file set. Schema = embedded JSON Schema (versioned). Templates = the six
artifact stubs. All artifacts must conform to schema version declared in state.json.

## Error handling
Digest mismatch -> refuse. Existing file without --force -> refuse. Schema violation ->
check fails with field path. Unknown field on resolve -> reported, never silently dropped.

## Verification strategy
- Golden: apply each embedded pack to a temp dir twice; diff must be empty (R2).
- Schema sweep: validate generated artifacts + every embedded template (R1, R4).
- Digest: table of {url, sha, expect apply/refuse} (R3).
- `specd schema` output version == state.json schemaVersion (R1.3).

## Risks and open questions
- Remote pack tests need network or a local fixture server; prefer a fixture to keep CI
  hermetic. Open: should unpinned remote packs be hard-blocked or opt-in? R3.3 currently
  requires opt-in — confirm with security owner before tightening to hard-block.
