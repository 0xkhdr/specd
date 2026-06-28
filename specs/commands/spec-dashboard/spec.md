# Spec — `/spec` Workflow Dashboard

**Priority:** P0 · **Wave:** 2 · **Domain:** spec lifecycle UX.

## Introduction

Users need `/spec` as one dashboard for creating specs, continuing current work, checking gates, approving phase transitions, finding next runnable tasks, and rendering reports. The command must preserve specd’s evidence gates and state model by delegating all mutations to native `specd` commands.

## Current-state grounding

- Native lifecycle commands exist: `specd new`, `status`, `context`, `check`, `approve`, `next`, `waves`, `verify`, `task`, `report`.
- `state.json` and task checkboxes must never be hand-edited.
- The action plan includes a `mode` action, but current native command availability must be discovered from `specd help --json`; wrapper must degrade gracefully if `specd mode` is absent.

## Requirements

### Requirement 1 — Dashboard listing
**Acceptance criteria:**
1. `/spec list` SHALL show all specs with slug, status/phase, task counts when available, and next suggested action.
2. Listing SHALL prefer `specd status --json`; if unavailable, it SHALL fall back to text `specd status`.
3. No specs SHALL produce a clear prompt to run `/spec new`.

### Requirement 2 — Spec creation
**Acceptance criteria:**
1. `/spec new <slug> [--title <title>] [--orchestrated]` SHALL delegate to `specd new`.
2. Interactive `new` SHALL prompt for slug/title and optional orchestration flag.
3. Slug validation errors SHALL be left to native specd and return native exit code.
4. After creation, command SHALL print next steps: edit requirements, run check, approve.

### Requirement 3 — Continue flow
**Acceptance criteria:**
1. `/spec continue [slug]` SHALL run `specd context <slug>` and suggest next action based on phase/status.
2. If only one spec exists, missing slug SHALL auto-select it; otherwise prompt/list.
3. In execution phase, command SHALL call `specd next <slug>` and show verify/complete instructions without marking complete.
4. Blocked/no runnable tasks SHALL suggest `specd waves <slug>`.

### Requirement 4 — Gate and report actions
**Acceptance criteria:**
1. `/spec check <slug>` SHALL delegate to `specd check <slug>`.
2. `/spec approve <slug>` SHALL delegate to `specd approve <slug>` and preserve human-gate explicitness.
3. `/spec context <slug>`, `/spec next <slug>`, `/spec waves <slug>`, and `/spec report <slug>` SHALL delegate directly.
4. Exit codes SHALL propagate.

### Requirement 5 — Execution-mode awareness
**Acceptance criteria:**
1. `/spec mode <slug>` SHALL inspect native command availability before using `specd mode` or equivalent.
2. If unsupported, it SHALL explain current repo lacks native mode command and suggest orchestration via `/pinky-brain` or `specd new --orchestrated` when available.
3. Wrapper SHALL NOT hand-edit spec state or config to change mode unless a native supported command exists.

### Requirement 6 — Tests and safety
**Acceptance criteria:**
1. Tests SHALL cover no specs, one spec auto-select, multiple specs prompt, execution next-task guidance, gate delegation, and missing native JSON fallback.
2. Wrapper SHALL not invoke `specd task --status complete` automatically.
3. Wrapper SHALL never edit `state.json` or `tasks.md`.

## Design

- Add `/spec` action dispatcher in shared workflow wrapper.
- Use helper functions: `require_specd_root`, `list_specs`, `select_slug`, `native_has_command`.
- Prefer structured JSON but tolerate absent/changed schema.
- Keep actions thin and deterministic; no LLM-generated content.

## Out of scope

- Authoring requirements/design/tasks content.
- Automatically approving phases.
- Completing tasks or bypassing evidence gate.

## Risks

- **Schema drift:** Use tolerant JSON parsing and text fallback.
- **Evidence bypass temptation:** Print verify/task instructions only; never auto-complete.
- **Ambiguous spec selection:** Auto-select only when exactly one spec exists.
