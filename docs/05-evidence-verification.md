# Evidence & Verification

This document details how `specd` enforces evidence-gated task completion through sandbox execution, env scrubbing, and an immutable evidence ledger.

---

## 1. Evidence-Gated Completion

A core invariant of the `specd` harness is **Evidence-Gated Completion (P3)**. A task status cannot transition to `complete` in `state.json` or `tasks.md` unless the task's verification script has executed and returned exit code `0`. 

For read-only tasks (e.g., design review or analysis tasks), the harness provides an explicit `--unverified --evidence` bypass, which requires documenting the manual check.

*Origin:* Enforced inside [task_complete.go](file:///var/www/html/rai/up/specd/reference/internal/core/task_complete.go).

---

## 2. Command Execution & Security Hardening

When running verification scripts, the harness executes the script in `tasks.md` via `sh -c` (overrideable with `SPECD_VERIFY_SHELL`). To prevent malicious behavior or accidental state leakage, `specd` implements the following security measures:

*   **Environment Scrubbing:** The subprocess environment is scrubbed using `ScrubbedEnv` (see [customgate.go](file:///var/www/html/rai/up/specd/reference/internal/core/customgate.go)) to only allow safe keys: `PATH`, `HOME`, `LANG`, `LC_ALL`, `TMPDIR`, and any variables prefixed with `SPECD_`.
*   **NUL Byte Rejection:** Any command containing a NUL byte (`\x00`) is rejected.
*   **Audit Logging:** Before execution, the exact command and working directory are written to stdout.

---

## 3. The Append-Only Evidence Ledger

Every verification run produces a `VerificationRecord` saved under `.specd/specs/<slug>/evidence/<hash>.json`. 

### Record Schema
```json
{
  "task": "T1.1",
  "status": "pass",
  "exitCode": 0,
  "command": "go test ./internal/core -run TestStateCAS",
  "cwd": "/var/www/html/rai/up/specd",
  "changedFiles": [
    "internal/core/state.go"
  ],
  "startedAt": "2026-07-04T13:00:00Z",
  "durationMs": 142,
  "evidenceRef": "sha256:abcd1234..."
}
```

*   **Immutability:** Evidence records are append-only and cryptographically hashed. They are never overwritten.
*   **State Linking:** When a task completes, its entry in `state.json` records a specific `evidenceRef` mapping to the hash of the passing verification run.

*Origin:* Simplified from the metric-bloated structures in [state.go](file:///var/www/html/rai/up/specd/reference/internal/core/state.go).

---

## 4. Sandboxed Verification

`specd` supports running verification commands in a sandbox to isolate execution:

```yaml
# config.yml
verify:
  sandbox: bwrap # off | bwrap | container
```

### Fail-Closed Principle
If `verify.sandbox` is configured to `bwrap` (Bubblewrap) or `container` (Docker/Podman), and the corresponding binary is missing from the host system, the harness **fails closed**. It will print an error and refuse to execute the verification script unsandboxed.

---

## 5. Revert On Fail

When debugging, an agent or developer can run:

```bash
specd verify <slug> <task> --revert-on-fail
```

If the verification script fails (exit code != 0), the harness automatically rolls back all file changes in the workspace using standard git-worktree checkouts to restore the workspace to its pre-verify state.

*Origin:* Refactored out of the main run block in [verify.go](file:///var/www/html/rai/up/specd/reference/internal/cmd/verify.go).
