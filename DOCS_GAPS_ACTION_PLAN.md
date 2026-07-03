# Documentation Gaps Action Plan

This document details the discovered documentation gaps in `specd` v0.2.0 and provides instructions for a coding agent to analyze, plan, and implement the necessary updates.

---

## 1. Discovered Gaps & Discrepancies

### A. The Stale `specd program` CLI Reference
*   **Location:** [docs/command-reference.md:L85](file:///var/www/html/rai/up/specd/docs/command-reference.md#L85)
*   **Issue:** The table under "Meta-hidden commands" documents `specd program` as a top-level command, detailing usage for `specd program schedule`, `specd program tick`, `specd program link/unlink`, etc. 
*   **Code Reality:** The top-level `program` command was removed in v0.2.0 (as documented in `CHANGELOG.md`). It is not registered in the CLI's `Registry` or `core.Commands` help metadata. Running `specd program` results in `unknown command: program`.
*   **Correct Usage:** The program sub-verbs are instead handled via `specd status --program <subcommand>` (e.g., `specd status --program schedule`, `specd status --program tick`, `specd status --program link/unlink`).
*   **Resolution:** 
    *   Update [docs/command-reference.md](file:///var/www/html/rai/up/specd/docs/command-reference.md) to document `specd status --program` and its subcommands/flags rather than a top-level `specd program` command.
    *   Verify that no other documentation pages refer to `specd program` as a standalone command.

### B. Missing Help Details for `specd status --program` Subcommands
*   **Location:** Help text in `internal/core/commands.go` (synopsis and descriptions for `status`).
*   **Issue:** Running `specd help status` only lists `--program` as "Show the cross-spec program frontier". It does not document that `status --program` accepts subcommands like `schedule`, `tick`, `link`, or `unlink`, nor does it document the flags required by these subcommands.
*   **Resolution:** 
    *   Enhance the CLI help metadata in [internal/core/commands.go](file:///var/www/html/rai/up/specd/internal/core/commands.go) under the `status` command entry to document the `schedule`, `tick`, `link`, and `unlink` parameters when `--program` is supplied.

### C. Insufficient Detail on New v0.2.0 Features in Core Guides
*   *Harness Quarantine:* Provide more concrete examples in [user-guide.md](file:///var/www/html/rai/up/specd/docs/user-guide.md) showing what happens when a command is quarantined and how to use `specd harness enable`.
*   *Dashboard Details:* Document the read-only dashboard's API endpoints and event streaming model (`/events`) more thoroughly.

---

## 2. Instructions for the Coding Agent

As the coding agent, you are tasked with implementing the documentation fixes by following the structured `specd` workflow. 

### Step 1: Perception & Analysis
1.  Search the entire `docs/` folder, embedded templates, and source comments for any remaining references to standalone `specd program` commands.
2.  Review the command arguments and parsing logic in [internal/cmd/status.go](file:///var/www/html/rai/up/specd/internal/cmd/status.go) and [internal/cmd/program.go](file:///var/www/html/rai/up/specd/internal/cmd/program.go) to ensure all documented options for `status --program` align perfectly with the actual code behavior.

### Step 2: Plan the Implementation
1.  Create a new spec directory under `specs/docs-gaps-update/`.
2.  Write `specs/docs-gaps-update/spec.md` defining the requirements for updating:
    *   [docs/command-reference.md](file:///var/www/html/rai/up/specd/docs/command-reference.md)
    *   [internal/core/commands.go](file:///var/www/html/rai/up/specd/internal/core/commands.go)
    *   Any other matching documents in `docs/`.
3.  Write `specs/docs-gaps-update/tasks.md` detailing the task checklist and verification commands (e.g., `make docs-lint` and running `specd help status` to verify output).

### Step 3: Execution & Verification
1.  Apply the edits.
2.  Run `make docs-lint` to ensure the cheat sheet mirrors are aligned.
3.  Run the docs parity tests (`go test ./internal/cmd/... -run TestCommandReferenceMatchesRegistry`) to verify that the command reference is correct.
4.  Verify that all tests remain green under `make ci`.
