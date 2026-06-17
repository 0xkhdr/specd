# Tasks — Regression: Packs, Schema, Templates (apply/resolve/validate)

## Wave 1
- [x] T1 — Inventory packs, templates, and schema-validation coverage ✓ complete · evidence: Inventory in memory packs-inventory-T1. 2 packs, 24 templates (only config.json schema-bearing), schema v1 only. Gaps: R1.1 no generated-artifact sweep, R2.1 deterministic re-apply UNMAPPED, R4 template trip-wire absent. Investigator, verify N/A. · 2026-06-17T16:48:41.921568217Z
  - why: know which generated artifacts and templates are currently validated (R1, R4)
  - role: investigator
  - files: internal/core/embed_packs, internal/core/embed_templates, internal/core/schema
  - contract: list embedded packs + templates and which tests validate them; mark gaps; do NOT edit
  - acceptance: coverage table {artifact/template -> validating test or UNMAPPED}
  - verify: N/A
  - depends: —
  - requirements: 1, 4

## Wave 2
- [x] T2 — Schema-validity sweep over generated artifacts and templates ✓ complete · evidence: schema_sweep_test.go: generated state validates (R1.1), missing-required rejected (R1.2), served schema versions == state (R1.3), all embedded templates well-formed (R4.1), unknown-field lockstep trip-wire (R4.2). go test -run 'Schema|Template' → exit 0. · 2026-06-17T16:54:04.080604215Z
  - why: R1, R4 demand every generated artifact and template validate
  - role: builder
  - files: internal/core/schema_validate_test.go, internal/core/pack_test.go
  - contract: add a sweep validating `specd new` output and every embedded template against the schema; assert `specd schema` version == state schemaVersion
  - acceptance: R1.1-R1.3 and R4.1-R4.2 pass
  - verify: go test ./internal/core/ -run 'Schema|Template'
  - depends: T1
  - requirements: 1, 4

- [x] T3 — Deterministic pack application golden tests ✓ complete · evidence: pack_golden_test.go: byte-identical re-apply minimal+go-service (R2.1), --force guard no-clobber + sentinel survives (R2.3), BuiltinPacks enumerable+sorted (R2.2). go test -run 'Pack|InitPack' → exit 0. · 2026-06-17T17:04:15.234929958Z
  - why: R2 reproducibility — same pack, same bytes, no clobber
  - role: builder
  - files: internal/core/pack_test.go, internal/core/initpack_test.go
  - contract: apply each embedded pack twice to temp dirs, assert empty diff; assert --force guard and --list-packs output
  - acceptance: R2.1-R2.3 pass
  - verify: go test ./internal/core/ -run 'Pack|InitPack'
  - depends: T1
  - requirements: 2

- [x] T4 — Remote pack digest-safety tests ✓ complete · evidence: TestRemotePackDigestSafety: hermetic httptest table {correct→apply, wrong→refuse, absent→refuse, tampered-body→refuse} (R3.1-R3.3). No live network. go test -run 'PackResolve|Remote' → exit 0. · 2026-06-17T17:04:59.463768776Z
  - why: R3 supply-chain safety via sha256 pinning
  - role: builder
  - files: internal/core/pack_resolve_test.go
  - contract: fixture server (no live network); table {url, sha, expect apply/refuse}; assert mismatch refused and unpinned requires opt-in
  - acceptance: R3.1-R3.3 pass; tests are hermetic (no external network)
  - verify: go test ./internal/core/ -run 'PackResolve|Remote'
  - depends: T1
  - requirements: 3

## Wave 3
- [x] T5 — Review pack/schema regression for hermeticity and field-drop safety ✓ complete · evidence: Review T2-T4: remote tests use httptest loopback only — zero live network. ParsePack DisallowUnknownFields surfaces unknown fields not dropped (R4.3, TestPackManifest/unknown_field). Digest check exact lowercased-hex; tampered-body test proves pin trusted over URL. All T1 gaps closed, none UNMAPPED. go test -run 'Schema|Pack' -count=2 → exit 0 (no state leak across runs). · 2026-06-17T17:05:20.08880982Z
  - why: hidden network or silently-dropped fields would void the guarantees
  - role: reviewer
  - files: internal/core
  - contract: review T2-T4 for live-network calls, dropped unknown fields, and weak digest checks; flag only
  - acceptance: zero external-network dependence; unknown fields surfaced (R4.3); no UNMAPPED artifact
  - verify: go test ./internal/core/ -run 'Schema|Pack' -count=2
  - depends: T2, T3, T4
  - requirements: 1, 2, 3, 4
