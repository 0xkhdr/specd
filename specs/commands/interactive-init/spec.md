# Spec — Interactive `/init` Command Wrapper

**Priority:** P0 · **Wave:** 1 · **Domain:** slash-command bootstrap UX.

## Introduction

Users need one high-intent `/init` command that bootstraps specd for any supported coding host, presents explicit behavior choices, and delegates enforcement to native `specd init`/`specd doctor`. Implementation must be thin glue: no reimplementation of scaffold policy, no direct state mutation, and no hidden global writes beyond native specd behavior.

## Current-state grounding

- Native `specd init` already scaffolds `.specd/`, detects hosts with `--agent`, supports repair/refresh/dry-run, and accepts orchestration options.
- Native `specd doctor` is the health/diagnostic surface and can expose host/scaffold status.
- The action plan recommends shell + Python wrappers, with shell as POSIX-first and Python as cross-platform fallback.
- Wrappers must preserve exit code semantics: `0` ok, `1` gate/enforcement failure, `2` usage, `3` not found.

## Requirements

### Requirement 1 — Host behavior selection
**Acceptance criteria:**
1. `/init` SHALL present detected hosts plus `all` and `none` when interactive.
2. `/init --agent <name|all|none|auto>` SHALL run non-interactively without prompts.
3. Host detection SHALL prefer structured output from `specd doctor --json` or `specd init --dry-run --json` when available.
4. If host detection fails, `/init` SHALL fall back to the known host list: `claude-code`, `codex`, `cursor`, `antigravity`, `vscode`, `none`.

### Requirement 2 — Orchestration behavior selection
**Acceptance criteria:**
1. `/init` SHALL offer orchestration policy choices: `none`, `manual`, `planning`, `session`.
2. If orchestration is enabled, `/init` SHALL collect or accept flags for workers, retries, timeout, cost limit, role mode, and sandbox.
3. Defaults SHALL match native specd defaults or documented action-plan defaults: 4 workers, 2 retries, 120 minute timeout, cost limit disabled, `inline` mode, `none` sandbox.
4. Non-interactive mode SHALL be fully flag-driven and SHALL NOT block on stdin.

### Requirement 3 — Native command delegation
**Acceptance criteria:**
1. `/init` SHALL invoke `specd init` with selected flags and `--yes` only when user selected or non-interactive mode requires it.
2. `/init --dry-run` SHALL preview native mutations and avoid wrapper-created writes.
3. `/init --repair` and `/init --refresh` SHALL pass through unchanged.
4. Wrapper SHALL never write `.specd/state.json`, spec state, or scaffold files directly.

### Requirement 4 — Robust shell and Python implementations
**Acceptance criteria:**
1. Provide POSIX-compatible shell entry point in the command wrapper bundle.
2. Provide Python 3 fallback with equivalent flags and behavior.
3. Both implementations SHALL work without `jq`; if `jq` is absent, they SHALL use safe fallback parsing or native text mode.
4. Both implementations SHALL quote all user-controlled values and avoid `eval`.

### Requirement 5 — Tests and docs
**Acceptance criteria:**
1. Unit tests SHALL cover command construction for each agent/orchestration choice.
2. Integration tests SHALL run `--dry-run` in a temp repo and assert no unexpected files are written.
3. Docs SHALL show interactive and non-interactive examples.
4. Failure paths SHALL return native-compatible exit codes.

## Design

- Add wrapper implementation under a dedicated slash-command package/script location selected by the implementer, e.g. `scripts/specd-workflow.sh` and `scripts/specd-workflow.py`.
- Use one internal command-builder function shared by interactive and flag-driven paths.
- For JSON probing, treat invalid JSON as no detection rather than fatal.
- Keep all project mutation inside native `specd init`.
- Make shell function name installable as `/init` where host supports slash commands, and callable as `specd-workflow init` otherwise.

## Out of scope

- Changing native scaffold behavior.
- Adding LLM calls.
- Implementing host-specific slash-command plugin formats beyond documented wrapper registration.

## Risks

- **Host schema drift:** Mitigate with fallback host list and schema-tolerant JSON extraction.
- **Prompt blocking in CI:** Require explicit non-interactive flags and detect non-TTY before prompting.
- **Shell injection:** Avoid `eval`; use arrays in Bash mode and careful quoting in POSIX mode.
