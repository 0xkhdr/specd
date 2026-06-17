# Tasks — Regression: Packs, Schema, Templates (apply/resolve/validate)

## Wave 1
- [ ] T1 — Inventory packs, templates, and schema-validation coverage
  - why: know which generated artifacts and templates are currently validated (R1, R4)
  - role: investigator
  - files: internal/core/embed_packs, internal/core/embed_templates, internal/core/schema
  - contract: list embedded packs + templates and which tests validate them; mark gaps; do NOT edit
  - acceptance: coverage table {artifact/template -> validating test or UNMAPPED}
  - verify: N/A
  - depends: —
  - requirements: 1, 4

## Wave 2
- [ ] T2 — Schema-validity sweep over generated artifacts and templates
  - why: R1, R4 demand every generated artifact and template validate
  - role: builder
  - files: internal/core/schema_validate_test.go, internal/core/pack_test.go
  - contract: add a sweep validating `specd new` output and every embedded template against the schema; assert `specd schema` version == state schemaVersion
  - acceptance: R1.1-R1.3 and R4.1-R4.2 pass
  - verify: go test ./internal/core/ -run 'Schema|Template'
  - depends: T1
  - requirements: 1, 4

- [ ] T3 — Deterministic pack application golden tests
  - why: R2 reproducibility — same pack, same bytes, no clobber
  - role: builder
  - files: internal/core/pack_test.go, internal/core/initpack_test.go
  - contract: apply each embedded pack twice to temp dirs, assert empty diff; assert --force guard and --list-packs output
  - acceptance: R2.1-R2.3 pass
  - verify: go test ./internal/core/ -run 'Pack|InitPack'
  - depends: T1
  - requirements: 2

- [ ] T4 — Remote pack digest-safety tests
  - why: R3 supply-chain safety via sha256 pinning
  - role: builder
  - files: internal/core/pack_resolve_test.go
  - contract: fixture server (no live network); table {url, sha, expect apply/refuse}; assert mismatch refused and unpinned requires opt-in
  - acceptance: R3.1-R3.3 pass; tests are hermetic (no external network)
  - verify: go test ./internal/core/ -run 'PackResolve|Remote'
  - depends: T1
  - requirements: 3

## Wave 3
- [ ] T5 — Review pack/schema regression for hermeticity and field-drop safety
  - why: hidden network or silently-dropped fields would void the guarantees
  - role: reviewer
  - files: internal/core
  - contract: review T2-T4 for live-network calls, dropped unknown fields, and weak digest checks; flag only
  - acceptance: zero external-network dependence; unknown fields surfaced (R4.3); no UNMAPPED artifact
  - verify: go test ./internal/core/ -run 'Schema|Pack' -count=2
  - depends: T2, T3, T4
  - requirements: 1, 2, 3, 4
