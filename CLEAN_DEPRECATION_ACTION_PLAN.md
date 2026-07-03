# Clean Deprecation Action Plan

This document details the clean deprecation requirements for `specd` v0.2.0, aiming to remove all traces of legacy/deprecated command structures, file artifacts, and backward-compatibility layers. The goal is to establish a minimal, highly readable, and optimized codebase.

---

## 1. Deprecated Artifacts & Clean Up Targets

### A. Merge and Delete Stale Command Files
Several `.go` files in `internal/cmd/` no longer register top-level commands in the CLI [internal/cmd/registry.go](file:///var/www/html/rai/up/specd/internal/cmd/registry.go). They only exist as helper functions invoked by other commands. To remove traces of this legacy structure, relocate their logic and delete the files:
1.  **`validate.go` & `schema.go`:**
    *   *Status:* Only `runValidate` and `runSchema` remain. They are called by `RunCheck` in [check.go](file:///var/www/html/rai/up/specd/internal/cmd/check.go).
    *   *Action:* Merge `runValidate` and `runSchema` directly into [check.go](file:///var/www/html/rai/up/specd/internal/cmd/check.go). Delete [validate.go](file:///var/www/html/rai/up/specd/internal/cmd/validate.go) and [schema.go](file:///var/www/html/rai/up/specd/internal/cmd/schema.go).
2.  **`mode.go` & `mode_cmd_test.go`:**
    *   *Status:* Only `runModeSet` and `runModeRecommend` remain. They are called by `RunStatus` in [status.go](file:///var/www/html/rai/up/specd/internal/cmd/status.go).
    *   *Action:* Merge these functions and their tests into [status.go](file:///var/www/html/rai/up/specd/internal/cmd/status.go) and its test suite. Delete [mode.go](file:///var/www/html/rai/up/specd/internal/cmd/mode.go) and [mode_cmd_test.go](file:///var/www/html/rai/up/specd/internal/cmd/mode_cmd_test.go).
3.  **`diff.go`, `replay.go`, `serve.go`, `watch.go`:**
    *   *Status:* Their functions (`runDiff`, `runReplay`, `runServe`, `runWatch`) are only called by `RunReport` in [report.go](file:///var/www/html/rai/up/specd/internal/cmd/report.go).
    *   *Action:* Merge these reporting sub-actions into [report.go](file:///var/www/html/rai/up/specd/internal/cmd/report.go) (or consolidate them in a dedicated reporting submodule). Delete the individual command files in `internal/cmd/`.

### B. Clean Up Config & State Backward Compatibility
1.  **Legacy JSON Configuration:**
    *   *Status:* [internal/core/config_loader.go](file:///var/www/html/rai/up/specd/internal/core/config_loader.go) still contains heavy parsing code to read legacy JSON configs and convert them to YAML.
    *   *Action:* Simplify the configuration loader to support the current standard YAML format only, or reduce the JSON parser to a minimal, non-adaptive reader.
2.  **Schema Version Migrations:**
    *   *Status:* [internal/core/state.go](file:///var/www/html/rai/up/specd/internal/core/state.go) contains logic to migrate `state.json` from schema version v5 to v6.
    *   *Action:* If backward compatibility with old pre-v0.2.0 state versions is no longer required, remove the migration code from [state.go](file:///var/www/html/rai/up/specd/internal/core/state.go) and make schema version 6 the absolute minimum required structure.

---

## 2. Instructions for the Coding Agent

As the coding agent, you are tasked with executing this clean deprecation and refactoring plan:

### Step 1: Analyze & Trace
1.  Verify the usage of all functions in the target command files (`validate.go`, `schema.go`, `diff.go`, `replay.go`, `serve.go`, `watch.go`, `mode.go`). Ensure no other parts of the codebase depend on them.
2.  Inspect configuration loading and state parsing to locate the compatibility blocks.

### Step 2: Plan the Refactoring Spec
1.  Create a new spec directory under `specs/clean-deprecation/`.
2.  Write `specs/clean-deprecation/spec.md` detailing the merge plans, deleted files, and configuration/state cleanups.
3.  Write `specs/clean-deprecation/tasks.md` listing each file consolidation as a task, with dedicated unit test checks as verification gates.

### Step 3: Refactor, Consolidate & Verify
1.  Move the functions and clean up the old files.
2.  Run the unit tests for the updated commands to verify correctness:
    *   `go test ./internal/cmd/... -run 'Check|Status|Report'`
3.  Run the full local CI gate (`make ci`) to ensure no compilation issues or regressions exist.
