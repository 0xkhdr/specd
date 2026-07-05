# Tasks — 02-state-schema-version

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T1 | scout | internal/core/state.go, internal/core/io.go | | `printf ok` | Map of every state.json read/write site and the struct fields comprising current shape; confirms single choke point for load/save |
| T2 | craftsman | internal/core/state.go, internal/core/migrate.go, internal/core/migrate_test.go | T1 | `go test ./internal/core -run TestMigrate -race -count=1` | `schemaVersion` field on state struct (writes as 1); ordered pure migration chain v0→v1; idempotency test (migrating twice = once); golden fixtures for v0 and v1 shapes (R1, R2, R4) |
| T3 | craftsman | internal/core/state.go, internal/core/migrate_test.go | T2 | `go test ./internal/core -run TestMigrate -race -count=1` | Future-version state (schemaVersion > current) fails closed exit-2 semantics with upgrade message; read paths never rewrite disk; migrated state persists only through AtomicWrite+CAS on genuine writes (R3, R4) |
| T4 | craftsman | internal/core/gates/schema.json, internal/core/gates/schema.go, internal/core/gates/schema_test.go | T2 | `go test ./internal/core/gates -run TestSchema -race -count=1` | go:embed JSON Schema for current state shape; minimal stdlib validator (required/types/enums), supported subset documented in schema file header; test validates freshly scaffolded state.json against it (R5) |
| T5 | craftsman | internal/cmd/check.go, internal/cmd/check_test.go | T4 | `go test ./internal/cmd -run TestCheckSchema -race -count=1` | `check --schema` emits schema to stdout; `--schema-only` validates on-disk state, reports violations, skips other gates; corrupted-fixture test (R5) |
| T6 | craftsman | docs/contributor-guide.md, docs/command-reference.md, docs/CHEATSHEET.md | T3,T5 | `./scripts/docs-lint.sh` | Forward-migration policy documented (bump version + migration fn + fixture test); new flags in both command docs (R6) |
| T7 | validator | (read-only) | T3,T5 | `go test ./... -race -count=1` | Full suite green including e2e lifecycle over migrated legacy fixture |
