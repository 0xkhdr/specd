# Spec — Slash Workflow Packaging, Tests, and Documentation

**Priority:** P1 · **Wave:** 4 · **Domain:** release-quality wrapper delivery.

## Introduction

The action plan requires four slash workflows (`/init`, `/steer`, `/spec`, `/pinky-brain`) to be delivered as a coherent, testable command pack. This spec covers shared packaging, common utilities, test harness, documentation, and install/source instructions so implementation is high quality rather than isolated scripts.

## Current-state grounding

- Repository is Go stdlib-only for core CLI; wrappers may be shell/Python because action plan recommends them as external workflow glue.
- Existing repo tests use deterministic harnesses and avoid golden-file brittleness.
- Existing scripts directory contains install/uninstall/coverage/stress tooling.
- Existing skills/templates are embedded; changing shipped skill/template content requires rebuild and tests if wrapper is integrated into core assets.

## Requirements

### Requirement 1 — Unified command pack
**Acceptance criteria:**
1. Provide one sourceable shell file exposing `/init`, `/steer`, `/spec`, and `/pinky-brain` aliases/functions where shell permits slash names.
2. Provide equivalent Python CLI with subcommands: `init`, `steer`, `spec`, `pinky-brain`.
3. Shared behavior (root discovery, native command execution, JSON probing, exit propagation) SHALL be centralized to avoid drift.
4. Command pack SHALL work when invoked from repo root or nested project directories.

### Requirement 2 — Installation and host integration docs
**Acceptance criteria:**
1. Docs SHALL explain how to source shell wrapper in Bash/Zsh.
2. Docs SHALL explain how to call Python wrapper on platforms that cannot define slash-named shell functions.
3. Docs SHALL map each slash command to native specd commands.
4. Docs SHALL state wrappers are optional UX glue and enforcement remains in native specd.

### Requirement 3 — Test strategy
**Acceptance criteria:**
1. Add deterministic wrapper tests using fake `specd` binaries/scripts on PATH.
2. Tests SHALL not require network, real host agents, real global config, or real Brain workers.
3. Tests SHALL cover shell and Python parity for core command construction.
4. Tests SHALL run under `make test` or a documented command included in CI guidance.

### Requirement 4 — Safety invariants
**Acceptance criteria:**
1. Tests SHALL assert wrappers never directly edit `state.json`.
2. Tests SHALL assert wrappers never flip `tasks.md` checkboxes.
3. Tests SHALL assert wrappers never auto-complete tasks or forge Pinky reports.
4. Tests SHALL assert all native command failures propagate nonzero exits.

### Requirement 5 — Skill and AGENTS documentation
**Acceptance criteria:**
1. If shipped as part of specd scaffold, update embedded skills or add new skill docs for slash workflow usage.
2. Update AGENTS/user-facing docs with concise quick reference.
3. Docs SHALL preserve progressive disclosure: do not force reading all skill docs before every action.
4. If embedded templates change, tests SHALL validate fresh init includes updated assets.

### Requirement 6 — CI/readiness gate
**Acceptance criteria:**
1. Final implementation SHALL pass `make test`.
2. If core Go code changes, final implementation SHALL pass `make ci` unless explicitly waived.
3. Wrapper test command SHALL be documented and included in local gate.
4. Coverage or lint changes SHALL not lower existing quality gates.

## Design

- Implement command pack in `scripts/` unless a later design chooses embedded templates; keeping wrappers external avoids core CLI churn.
- Add a small test fixture directory for fake `specd` command outputs.
- Prefer behavior tests over exact large golden output; assert important substrings and argv capture.
- Keep dependencies minimal: shell + Python stdlib. `jq` optional only.

## Out of scope

- Publishing marketplace-specific slash command extensions.
- Adding runtime dependencies to Go CLI.
- Rewriting native specd commands.

## Risks

- **Script drift:** Centralize helpers and parity tests.
- **CI portability:** Avoid Bash-only features in POSIX path; if Bash arrays used, document Bash requirement for shell pack.
- **Template rebuild miss:** If embedded assets change, add tests and rebuild guidance.
