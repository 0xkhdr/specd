# Troubleshooting Guide

This guide helps developers and AI agents diagnose and resolve issues, validation blocks, and platform-specific errors encountered when running `specd`.

---

## 1. Phase and Gate Blocks

### "spec is gated (awaiting-approval)"
* **Cause**: A `high` or `critical` mid-flight requirement update (`specd midreq`) was logged for the spec. This automatically freezes the spec state and blocks progress to ensure the team aligns on requirements before executing further.
* **Remediation**:
  1. Review the mid-requirement updates recorded in `.specd/specs/<slug>/mid-requirements.md`.
  2. Implement any necessary changes to requirements, designs, and tasks.
  3. Approve the gated state to unfreeze the spec:
     ```bash
     specd approve <slug>
     ```

### "requirements do not conform to EARS syntax"
* **Cause**: The `specd check` command failed on Gate 1 (EARS requirement parsing) because one or more criteria in `requirements.md` did not match any of the five EARS patterns.
* **Remediation**:
  * Verify that every criterion starts with an uppercase EARS keyword matching one of these forms:
    * `THE SYSTEM SHALL ...` (Ubiquitous)
    * `WHEN <trigger> THE SYSTEM SHALL ...` (Event-driven)
    * `WHILE <state> THE SYSTEM SHALL ...` (State-driven)
    * `WHERE <feature> THE SYSTEM SHALL ...` (Optional-feature)
    * `IF <condition> THEN THE SYSTEM SHALL ...` (Unwanted)
  * Note that matching is case-insensitive, but spelling must be precise (e.g., `THE SYSTEM SHALL` not `THE SYSTEM SHOULD`).

---

## 2. Concurrency & Lock Failures

### "state.json changed underfoot (concurrent write detected)"
* **Cause**: `specd` uses optimistic concurrency control. When attempting to save the state, `specd` found that the on-disk `revision` number did not match the loaded memory state's expected revision. This occurs when two agents or commands attempt to mutate the same spec state at the same time.
* **Remediation**:
  * The operation has been aborted safely without corrupting the file. Reload the state and retry:
    1. Re-run your query command (e.g. `specd status` or `specd next`) to fetch the latest state from disk.
    2. Re-apply your state mutation command.

### "lock timeout: failed to acquire lock for spec"
* **Cause**: `specd` uses an advisory file-lock system to serialize writes and prevent race conditions. A command timed out waiting to acquire the lock. The wait limit is defined by `SPECD_LOCK_TIMEOUT_MS` (default `5000ms`).
* **Remediation**:
  * Check if another process is hanging or running a long-running verification.
  * If a previous process crashed or was terminated forcefully, an orphaned `.lock` file may remain under `.specd/specs/<slug>/.lock`.
  * **Auto-reclamation**: `specd` automatically reclaims locks older than `SPECD_LOCK_STALE_MS` (default `30000ms` / 30 seconds). Wait 30 seconds and retry.
  * **Manual reclamation**: If necessary, inspect the lock file. It contains the PID and epoch timestamp of the holder. If the process is dead, you can safely delete `.specd/specs/<slug>/.lock`.

---

## 3. Verify Sandbox Errors

### "bubblewrap isolation failed: bwrap not found in PATH"
* **Cause**: You ran `specd verify` with `--sandbox bwrap` (or set `verify.sandbox: "bwrap"` in config), but the `bwrap` command-line utility is missing from the host system.
* **Remediation**:
  * Install Bubblewrap via your system package manager:
    ```bash
    # Ubuntu/Debian
    sudo apt-get install bubblewrap

    # Fedora/RHEL
    sudo dnf install bubblewrap

    # macOS (via MacPorts/Homebrew, though sandbox features are primarily Linux-native)
    brew install bubblewrap
    ```

### "container isolation failed: docker/podman not found in PATH"
* **Cause**: You selected `--sandbox container` but neither `docker` nor `podman` is installed or running on the host system.
* **Remediation**:
  * Install Docker or Podman and verify the daemon is running:
    ```bash
    docker info
    # or
    podman info
    ```
  * Ensure the container image name is configured using the `SPECD_SANDBOX_IMAGE` environment variable.

---

## 4. Verification Failures & Timeouts

### "verification timed out after 10m"
* **Cause**: The task's `verify:` command exceeded the maximum execution limit. The default limit is `600000ms` (10 minutes). The verify command exited with code `124` and was marked failed.
* **Remediation**:
  * Optimize the test command to run faster.
  * If the test suite legitimately takes longer than 10 minutes, override the timeout budget using the `SPECD_VERIFY_TIMEOUT_MS` environment variable:
    ```bash
    export SPECD_VERIFY_TIMEOUT_MS=1200000 # Increase to 20 minutes
    specd verify <slug> <task-id>
    ```

---

## 5. Onboarding & MCP Integration

Start with `specd doctor` — it separates the three failure layers (scaffold, MCP
server, host registration) and prints a remediation command for each. Add `--json`
for machine output, `--fix` to apply safe project-scoped repairs.

### "managed scaffold incomplete" / missing `.specd/` files
* **Cause**: Some specd-managed files were deleted or never written.
* **Remediation**: `specd init --repair` restores missing managed assets **without**
  overwriting your edits. Use `specd init --refresh` to update specd-managed assets
  and `AGENTS.md` marker sections only.

### "my coding agent isn't detected"
* **Cause**: The host executable isn't on `PATH`, or no project config marker exists.
* **Remediation**:
  * Confirm the CLI resolves (`command -v codex` / `claude` / `gemini`).
  * Configure a named host explicitly: `specd init --agent claude-code --yes`.
  * If the host is unmanaged (antigravity, claude-desktop), use a manual snippet:
    `specd mcp --config <host>`.

### "host config present but agent doesn't see specd tools"
* **Cause**: The host needs a restart/reload to pick up a new MCP server, or the
  server failed to start. specd never restarts the host for you.
* **Remediation**:
  * Run `specd doctor` to confirm the MCP handshake passes server-side.
  * Reload/restart the host (e.g. VS Code window reload; enable the server in
    Cursor's Tools & MCP). Trust/approval prompts are host-controlled.

### "global scope requires explicit consent" (exit `2`)
* **Cause**: `--scope global` was requested non-interactively without `--yes`.
* **Remediation**: specd never edits global/user config silently. Re-run with
  `--scope global --yes`, or stay project-scoped (the default).

### "existing host config did not parse" (init fails closed)
* **Cause**: The target config file (e.g. `.mcp.json`) is malformed, so specd refuses
  to mutate it.
* **Remediation**: Fix the JSON/TOML by hand, then re-run. specd backs up an existing
  config (timestamped) before any change and only modifies its own server entry.

### `specd init --dry-run`
* Preview exactly which files and host commands a run would execute — and the absolute
  `specd` path that would be registered — without writing anything. Pair with `--json`
  for scripted review.
