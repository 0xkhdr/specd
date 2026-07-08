// Package schema serves specd's published JSON Schema document (embedded
// at build time via go:embed) and provides a minimal decoded view —
// SchemaDoc/SchemaDef — of its $defs and version ids. Full JSON Schema
// validation is intentionally out of scope to preserve the stdlib-only,
// no-validator-dependency invariant; callers use the structural
// definitions for conformance checks and format reporting, not generic
// schema enforcement.
package schema
