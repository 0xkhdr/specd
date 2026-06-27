# Tasks — Command Schema Guardrails

## Wave 1 — Metadata shape
- [ ] T1 — Extend command metadata structs
  - why: explicit machine contract for command builders (Req 1,2,4)
  - role: builder
  - files: internal/core/commands.go
  - contract: add `PositionalMeta`, `FlagMeta.Enum`, `FlagMeta.Required`, `FlagMeta.Default`, `CommandMeta.PhaseCompatibility`, and `CommandMeta.ModeCompatibility` with JSON `omitempty` where appropriate.
  - acceptance: existing help JSON remains valid; new fields omit when empty.
  - verify: go test ./internal/core/ -run Help
  - depends: —
  - requirements: 1,2,4

- [ ] T2 — Populate current command registry
  - why: prevent schema gaps (Req 1,2,4)
  - role: builder
  - files: internal/core/commands.go
  - contract: annotate every command's positionals; add enums for closed flags; add phase/mode compatibility for planning, execution, verification, complete, and orchestration surfaces.
  - acceptance: every non-meta command with `<...>` usage has matching positionals; common closed flags expose enums.
  - verify: go test ./internal/core/ -run CommandMeta
  - depends: T1
  - requirements: 1,2,4

## Wave 2 — Help JSON behavior
- [ ] T3 — Implement one-command JSON help
  - why: agents query per invocation (Req 3)
  - role: builder
  - files: internal/core/help.go, main.go
  - contract: add `RenderCommandHelpJSON`; route `specd help <command> --json` to one object; keep `specd help --json` as full array.
  - acceptance: unknown command exits 2; full help unchanged except added fields.
  - verify: go test ./... -run Help
  - depends: T1
  - requirements: 3

- [ ] T4 — Metadata drift tests
  - why: schema must stay trustworthy (Req 1,2,4)
  - role: verifier
  - files: internal/core/help_test.go, internal/cmd/commands_test.go
  - contract: assert registry commands have positionals when synopsis uses args; assert enum flags use known sets; assert orchestration commands declare mode/capability compatibility.
  - acceptance: tests fail on future incomplete command metadata.
  - verify: go test ./internal/core/ ./internal/cmd/ -run "CommandMeta|Help"
  - depends: T2, T3
  - requirements: 1,2,3,4

## Wave 3 — MCP consumption
- [ ] T5 — Include enums in MCP tool schemas
  - why: hosts can validate before tool calls (Req 5)
  - role: builder
  - files: internal/mcp/tools.go, internal/mcp/testdata/tool_schemas.golden.json
  - contract: map `FlagMeta.Enum` into JSON Schema enum; enrich args descriptions with positional names without breaking existing `args` array input.
  - acceptance: MCP schema golden update is intentional; existing tool calls still work.
  - verify: go test ./internal/mcp/ -run Schema
  - depends: T2
  - requirements: 5

- [ ] T6 — Document schema-before-syntax protocol
  - why: agent authors need a binding rule (Req 3,4)
  - role: builder
  - files: docs/agent-integration.md, internal/core/embed_templates/AGENTS.md
  - contract: document `specd help <command> --json` before unfamiliar invocations and exit-code reaction rules.
  - acceptance: docs include examples for `verify`, `mode`, and `brain`.
  - verify: N/A
  - depends: T3
  - requirements: 3,4
