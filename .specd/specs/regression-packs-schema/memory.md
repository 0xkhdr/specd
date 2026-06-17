# Memory — Regression: Packs, Schema, Templates (apply/resolve/validate)

<!--
Source-attributed, generalizable learnings (append-only). Use
`specd memory <spec> add --key <slug> --pattern "<one-line>" --body "<detail>"
  --source "<Turn N, Task T?, role>" --criticality <minor|important|critical> [--related k,k]`.
Only generalizable patterns, never raw observations. Promote to project steering at 3+ specs via
`specd memory <spec> promote --key <slug>`. Format:

## <key-slug>
**Pattern:** <one-line generalizable claim>
**Detail:** <why it's true; the mechanism>
**Source:** Task T3, Turn 2, discovered by investigator
**Criticality:** important
**Related:** [[other-key]]
-->

## packs-inventory-T1
**Pattern:** pack/template/schema validation coverage gaps
**Detail:** embed_packs: go-service.json, minimal.json. embed_templates: config.json(JSON) + AGENTS.md + 6 steering + 6 specStubs + 4 roles + 6 skills — all markdown EXCEPT config.json; NONE are state.json. schema/: v1.json only. Existing tests: pack_test(ParsePack valid/bad table, BuiltinPacks sorted+named), pack_resolve_test(builtin+remote resolve, fail-closed pin via httptest), schema_test(ParseSchema, SchemaConformance struct<->schema drift). GAPS: R1.1 no sweep validating `specd new` generated state.json vs schema; R1.2 ValidateState rejection has no core unit test (only internal/cmd/schema_validate_test.go); R2.1 deterministic byte-identical re-apply UNMAPPED; R4 template<->schema trip-wire absent (only config.json schema-bearing; specStubs markdown exempt). R3 pin safety well-covered already.
**Source:** T1
**Criticality:** important
**Related:** —
