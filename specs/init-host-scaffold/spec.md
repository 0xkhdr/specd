# Init Host Scaffold Spec

## Purpose
Make `specd init` produce complete, valid host integrations for Codex, Claude Code, and Pinky-style agents without inert flags or malformed agent files.

## Source Gaps
- GAP-ANALYSIS.md domain 2: host scaffolding incomplete or misleading.
- `--agent=codex` docs/config behavior incomplete.
- Pinky subagent files lack required frontmatter and role-scoped tool declarations.
- MCP host config generation inconsistent across Claude Code and Codex.
- Repair, refresh, and dry-run behavior needs deterministic contract.

## Goals
- `specd init --agent=codex` writes valid Codex-facing docs/config where requested.
- Pinky agent scaffolds include valid frontmatter and role-appropriate tools.
- MCP host config generation works for Claude Code and Codex with one shared host model.
- Init supports idempotent repair, refresh, and dry-run semantics.
- Scaffold tests cover generated file content, not only file existence.

## Non-Goals
- Do not auto-install external host CLIs.
- Do not contact network during init tests.
- Do not alter frozen `reference/`.

## Required Knowledge
- Init command: `internal/cmd/lifecycle.go`.
- Agent scaffold logic: `internal/core/agents.go`, `internal/core/scaffold.go`.
- Embedded templates: `embed_templates/`.
- CLI metadata: `internal/core/commands.go`.
- Tests under `internal/cmd` and `internal/core`.

## Functional Contract
- `--agent` only accepts supported values and rejects unknown values with exit code 2.
- `--mcp` writes host-specific config only when requested.
- `--dry-run` prints planned writes and performs no mutation.
- `--repair` restores missing required scaffold files without clobbering unrelated user files.
- `--refresh` updates specd-owned scaffold blocks while preserving user content outside managed blocks.
- Generated Pinky files include required `name`, `description`, and tool/model metadata for target host.

## Acceptance Criteria
- Codex scaffold contains `AGENTS.md` guidance and `.codex/config.toml` MCP config when `--mcp` is set.
- Claude Code scaffold contains valid `.claude/agents/*.md` frontmatter.
- MCP host definitions are generated from one `MCPHosts()` contract.
- Tests assert idempotence: running init twice yields no unexpected diff.
- Tests assert dry-run leaves target tree unchanged.

## Invariants
- Init never writes outside requested project root.
- Generated files have stable ordering and deterministic content.
- User-authored content outside managed blocks is preserved.
- No runtime dependencies added.

## Verification
- `go test ./internal/core ./internal/cmd -run 'Test.*Init|Test.*Agent|Test.*Scaffold' -count=1`
- `go test ./... -count=1`
- `go test ./... -count=2`

