# Spec — `/steer` Steering Console

**Priority:** P0 · **Wave:** 1 · **Domain:** steering inspection and authorship UX.

## Introduction

Users need `/steer` to inspect, edit, and bootstrap all files in `.specd/steering/` without bypassing specd workflow rules. The command is a read/display/editor helper, not a state mutator. It should make the six steering files visible, flag missing or stub content, and guide authorship of `product.md`, `tech.md`, and `structure.md`.

## Current-state grounding

- Native specd scaffolds steering files: `reasoning.md`, `workflow.md`, `product.md`, `tech.md`, `structure.md`, `memory.md`.
- `specd context <slug>` emits phase-scoped load manifests, but no command shows all steering files unconditionally.
- Steering files are durable repo constitution; direct edits to these Markdown files are allowed, unlike `state.json`.

## Requirements

### Requirement 1 — Locate steering root
**Acceptance criteria:**
1. `/steer` SHALL find `.specd/steering` by walking upward from cwd.
2. If no steering dir exists, `/steer` SHALL return exit 3 and tell user to run `/init` or `specd init`.
3. Path handling SHALL reject traversal tricks by using resolved project paths only.

### Requirement 2 — Show and status views
**Acceptance criteria:**
1. `/steer show` SHALL list all six canonical steering files with size/status.
2. `/steer show <file>` SHALL print only that canonical file with a clear header.
3. `/steer status` SHALL classify missing, stub, placeholder, and authored files.
4. Stub detection SHALL be deterministic using file size, TODO markers, or known template markers.

### Requirement 3 — Edit and bootstrap flow
**Acceptance criteria:**
1. `/steer edit` SHALL open canonical steering files in `$EDITOR` when set.
2. Without `$EDITOR`, `/steer edit` SHALL fail safely with guidance unless an explicit `--stdin` mode is used.
3. `/steer bootstrap` SHALL guide authorship of `product.md`, `tech.md`, and `structure.md` after suggesting repo inspection commands.
4. `/steer bootstrap --dry-run` SHALL print intended files and prompts without writing.

### Requirement 4 — Memory view
**Acceptance criteria:**
1. `/steer memory` SHALL display `memory.md` with a header.
2. Missing `memory.md` SHALL be reported without creating it directly.
3. Command SHALL remind agents that `memory.md` is phase-scoped in normal `specd context` use.

### Requirement 5 — Portability, safety, tests
**Acceptance criteria:**
1. Shell and Python implementations SHALL have equivalent actions: `show`, `status`, `edit`, `bootstrap`, `memory`.
2. Only canonical steering filenames SHALL be accepted; arbitrary path args SHALL return exit 2.
3. Tests SHALL cover root discovery, missing root, canonical file filtering, stub detection, and editor-disabled behavior.

## Design

- Implement as part of shared workflow wrapper (`specd-workflow steer`) plus host alias `/steer`.
- Centralize canonical filename list in both shell/Python implementations.
- Prefer read-only actions by default. Write only user-authored Markdown through `$EDITOR` or explicit stdin bootstrap mode.
- Do not modify `.specd/config*`, `state.json`, or spec artifacts.

## Out of scope

- Auto-generating steering content with LLMs.
- Running `specd check` gates.
- Promoting spec memory into steering memory automatically.

## Risks

- **Accidental broad file edit:** Accept only canonical filenames.
- **Context bloat:** `show` defaults to summary; require `show all` or file arg for full content if implementer chooses.
- **Non-TTY editor hang:** Detect missing TTY/editor and fail with guidance.
