# CLI Contracts and Help Truth Spec

## Purpose
Ensure every CLI verb, flag, help line, and deferred command follows one deterministic contract.

## Source Gaps
- GAP-ANALYSIS.md domain 3: command truth and deferred/deprecated behavior drift.
- Some usage strings mention flags not wired in handlers.
- Help, docs, MCP, and command registry can diverge.

## Goals
- Use `core.Commands` as canonical source for verb status, usage, flags, aliases, and phase.
- Add tests that compare handler registry against command metadata.
- Reject unknown flags and unknown verbs consistently.
- Keep docs and generated help in sync.

## Non-Goals
- Do not redesign CLI argument parser.
- Do not add shell completion unless separately specified.
- Do not change exit-code contract without tests and docs.

## Required Knowledge
- CLI entry: `main.go`, `internal/cli`.
- Dispatch: `internal/cmd/registry.go`.
- Command metadata: `internal/core/commands.go`.
- Docs: `docs/command-reference.md`, `docs/CHEATSHEET.md`.

## Functional Contract
- Every implemented handler has exactly one command metadata entry.
- Every non-deferred metadata entry has a handler or an explicit refusal path.
- Help output is rendered from metadata.
- Deferred commands print canonical deferral text and exit 0.
- Unknown commands and invalid usage fail closed with exit code 2.

## Acceptance Criteria
- Registry/metadata conformance tests exist.
- Help output includes real flags only.
- Docs lint passes after any command or flag change.
- Command examples in docs execute or are marked conceptual/deferred.

## Invariants
- No silent no-op commands.
- No hidden implemented command missing from docs unless explicitly internal.
- No duplicate flag definitions by hand across docs/MCP/scaffold.

## Verification
- `go test ./internal/cli ./internal/cmd ./internal/core -count=1`
- `./scripts/docs-lint.sh`
- `go vet ./...`

