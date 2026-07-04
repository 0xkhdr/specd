# Troubleshooting

> Common errors, their causes, and remedies. If your issue is not listed here, check
> `PROJECT.md §8` for the open findings list or open an issue.

---

## Exit code reference

| Code | Meaning | Common cause |
|---|---|---|
| `0` | Success | — |
| `1` | Error | Gate failure, missing evidence, lock timeout, corrupt state |
| `2` | Usage / unknown command | Wrong argument count or unknown verb |
| `3` | Spec root not found | `.specd/` not present in current dir or any parent |

---

## Lock errors

### `spec lock timeout: .specd/specd.lock`

**Cause:** Another `specd` process is holding the advisory lock and did not release it
within the acquire timeout (default 5 seconds).

**Remedy:**
1. Check for another running `specd` process: `pgrep -l specd`.
2. If no process is running, the lock file is stale — delete it:
   ```bash
   rm .specd/specd.lock
   ```
3. To tune timeouts: `SPECD_LOCK_TIMEOUT_MS=10000 specd ...` (10s timeout) or
   `SPECD_LOCK_STALE_MS=60000` (60s stale threshold).

The lock is automatically reclaimed after 30 seconds of inactivity (configurable via
`SPECD_LOCK_STALE_MS`).

---

## State errors

### `unsupported state schema <N>`

**Cause:** `state.json` has a `schema_version` different from what the binary expects (1).
This usually means a state file written by a newer version of specd is being read by
an older binary, or the file was manually edited incorrectly.

**Remedy:**
- Check the binary version vs the state file.
- If the state was created by a newer build: upgrade the binary.
- If the state was corrupted by manual edit: restore from git (`git checkout .specd/specs/<slug>/state.json`).

---

### `state revision conflict: expected <N>, got <M>`

**Cause:** A compare-and-swap (CAS) failed — two processes tried to write state
concurrently and the second write saw a revision bump from the first.

**Remedy:** Retry the command. This is a transient race condition protected by the
advisory lock; it should not occur under normal use. If it recurs, check for concurrent
agents operating on the same spec.

---

### `invalid state status "<X>"`

**Cause:** `state.json` contains a `status` value that the binary does not recognize.
This usually means a manually edited or externally modified state file.

**Valid status values:** `requirements · design · tasks · executing · verifying · complete · blocked`

**Remedy:** Restore the state file from git or correct the `status` field to a valid value.

---

## Evidence errors

### `warning: git HEAD unresolved ("unknown"); this evidence cannot pin to a commit`

**Cause:** `git rev-parse HEAD` returned an error — usually because the project is not
a git repository, or git is not installed, or the repository has no commits.

**Consequence:** The evidence record is still written with `git_head: "unknown"` but
`specd task complete` will refuse to accept it.

**Remedy:**
- Ensure you are in a git repository with at least one commit.
- Verify `git rev-parse HEAD` returns a commit hash before running `specd verify`.
- If you are in a CI environment with a detached HEAD, the hash should still resolve.

---

### `task complete refused: no passing evidence record at current HEAD`

**Cause:** `specd task complete <spec> <id>` requires a passing evidence record in
`evidence.jsonl` whose `git_head` matches the current `HEAD` and whose `exit_code` is 0.

**Remedy:**
1. Run `specd verify <spec> <task-id>` and ensure it exits 0.
2. Make sure you have not committed since running `verify` (the HEAD must match).
3. Check `evidence.jsonl` to see the recorded records:
   ```bash
   cat .specd/specs/<slug>/evidence.jsonl
   ```

---

## Gate errors (`specd check`)

### `error ears: requirements file is unedited stub`

**Cause:** `requirements.md` still contains the unmodified scaffold stub. The EARS gate
compares the current file against the stub produced by `specd new`.

**Remedy:** Edit `.specd/specs/<slug>/requirements.md` with real EARS-shaped requirements
before approving.

---

### `error design: design file is unedited stub`

**Cause:** `design.md` still contains the unmodified scaffold stub. The design-stub gate
fires when you try to approve the design phase without editing it.

**Remedy:** Edit `.specd/specs/<slug>/design.md` with module boundaries, on-disk contracts,
and invariants.

---

### `error dag: orphan dependency <id>`

**Cause:** A task's `depends-on` field references a task ID that does not exist in the
task table.

**Remedy:** Check the task IDs in `tasks.md` for typos; ensure the referenced task is
present and spelled correctly.

---

### `error dag: cycle detected`

**Cause:** The task dependency graph contains a cycle (e.g. T1 → T2 → T1).

**Remedy:** Review the `depends-on` fields for circular chains and break the cycle.

---

### `error evidence: task <id> has no passing verify record`

**Cause:** The evidence gate (gate 5, always `error`, never opt-out) found a task marked
complete in `tasks.md` or `state.json` without a corresponding passing evidence record.

**Remedy:** Run `specd verify <spec> <id>` to produce a passing evidence record before
marking the task complete.

---

## Approval errors

### `approve refused: readiness gates failing`

**Cause:** One or more `error`-severity gate findings blocked the approval.

**Remedy:** Run `specd check <slug>` to see the exact findings and fix them before
re-running `specd approve`.

---

### `invalid gate "<X>"`

**Cause:** The gate name passed to `specd approve` is not a valid lifecycle gate.

**Valid gates:** `requirements · design · tasks · executing · verifying · complete`

---

## Config errors

### `config line <N>: <message>`

**Cause:** `project.yml` (or the global config file) has a parse error at line N.
The config parser is deliberately strict — truncated scalars, wrong indentation, or
unknown keys are all hard errors.

**Remedy:** Fix the YAML at the indicated line. Common issues:
- Indentation must be exactly **two spaces** (not tabs).
- Unknown keys are rejected: check `docs/configuration.md` for the valid key list.
- Secret keys (`token`, `secret`, `apikey`, `api_key`) are always rejected.

---

## Verify executor errors

### `NUL byte in verify command`

**Cause:** The `verify:` field in `tasks.md` contains a NUL (`\0`) byte.

**Remedy:** The verify command must be a plain shell command string. NUL bytes are
rejected as a security property (ADR-8, evidence integrity). Fix the `tasks.md` row.

---

### `sandbox binary not found`

**Cause:** `--sandbox` was specified but the sandbox binary (`bwrap` or the path given
by `--sandbox-binary`) is not installed or not in `PATH`.

**Consequence:** `verify` fails closed — it never runs the command unsandboxed when a
sandbox was requested.

**Remedy:** Install `bwrap` or provide the correct path via `--sandbox-binary=<path>`.

---

## MCP errors

### Agent called a forbidden tool (`approve`, `init`, `mcp`, `brain`)

**Cause:** The MCP server's `ForbiddenTool` policy blocks these verbs from being called
by an agent over MCP. This is a safety property enforcing P6 (humans gate phase transitions).

**Remedy:** These operations must be performed by a human in the terminal, not by an agent.

---

## Not finding `.specd/`

### `specd: specd root not found` (exit 3)

**Cause:** No `.specd/` directory was found in the current directory or any of its parents.

**Remedy:**
1. Run `specd init` in your project root to create `.specd/`.
2. Make sure you are running the command from within the project tree.
