# Tasks — Fusion Session Bootstrap

## Wave 1 — Core bootstrap model
- [x] T1 — Define `FusionBootstrap` core model
  - why: stable JSON contract for session startup (Req 1,2,3,4)
  - role: builder
  - files: internal/core/fusion.go (new)
  - contract: define versioned structs for load items, config summary, command schema summary, health checks, active spec summaries, and next actions. No LLM calls. No writes.
  - acceptance: structs marshal deterministically; nil slices render as `[]` through `core.PrintJSON` path.
  - verify: go test ./internal/core/ -run Fusion
  - depends: —
  - requirements: 1,2,3,4

- [x] T2 — Implement bootstrap assembler
  - why: gather existing specd state into one oracle (Req 1,3,4)
  - role: builder
  - files: internal/core/fusion.go, internal/core/paths.go
  - contract: locate root, check steering/role/skill/config/AGENTS files, compute config and command digests, enumerate spec statuses/modes without reading full artifacts.
  - acceptance: absent root returns NotFound; missing files appear in health failures; specs sorted by slug.
  - verify: go test ./internal/core/ -run Fusion
  - depends: T1
  - requirements: 1,3,4

## Wave 2 — CLI surface
- [ ] T3 — Add `specd fusion bootstrap` command
  - why: agent startup entrypoint (Req 1,2)
  - role: builder
  - files: internal/cmd/fusion.go (new), internal/cmd/registry.go, internal/core/commands.go
  - contract: parse subcommand `bootstrap`; support `--json` and `--include-schema`; text mode prints concise startup checklist; JSON mode emits `FusionBootstrap`.
  - acceptance: `--include-schema` includes full command schema; default includes digest/count only; usage errors exit 2.
  - verify: go test ./internal/cmd/ -run Fusion
  - depends: T2
  - requirements: 1,2

- [ ] T4 — Command/help registry parity
  - why: help and dispatch must never drift (Req 1)
  - role: builder
  - files: internal/cmd/commands_test.go, internal/core/commands.go
  - contract: extend existing registry/help parity tests to include `fusion`; assert help JSON contains expected flags.
  - acceptance: `TestRegistryMatchesHelp` remains green; `specd help --json` includes fusion metadata.
  - verify: go test ./internal/cmd/ -run "Registry|Fusion"
  - depends: T3
  - requirements: 1,2

## Wave 3 — Docs and template
- [ ] T5 — AGENTS.md startup instruction
  - why: make bootstrap constitutional for initialized repos (Req 5)
  - role: builder
  - files: internal/core/embed_templates/AGENTS.md, AGENTS.md
  - contract: add a startup rule: run `specd fusion bootstrap --json`; fallback to manual steering/config/help loading when using older specd.
  - acceptance: generated template contains exact command and fallback sequence.
  - verify: go test ./internal/cmd/ -run Init
  - depends: T3
  - requirements: 5

- [ ] T6 — Agent integration docs
  - why: adapter authors need the JSON contract (Req 5)
  - role: builder
  - files: docs/agent-integration.md, docs/command-reference.md
  - contract: document fields, example JSON, fallback sequence, and zero-overhead rationale.
  - acceptance: docs describe `load`, `commands`, `config`, `health`, `modes`, and `nextActions`.
  - verify: N/A
  - depends: T3
  - requirements: 5
