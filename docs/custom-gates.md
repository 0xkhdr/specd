# Custom Gates

specd's seven core gates are fixed and enforcement-only. **Custom gates** let a
project add its own checks as ordinary external programs, run after the core
pipeline. They are deliberately *not* Go plugins and *not* network clients —
just a subprocess with a JSON stdin/stdout contract.

```jsonc
// .specd/config.json
{
  "gates": {
    "custom": [
      { "name": "no-todos", "command": "./scripts/no-todos.sh", "severity": "error" }
    ]
  }
}
```

| Field | Meaning |
|-------|---------|
| `name` | Label shown in `specd check` output. |
| `command` | Program run via the verify shell (`sh -c`, or `SPECD_VERIFY_SHELL`). |
| `severity` | `error` (default) maps findings to violations (fails the check); `warn` maps them to warnings. |

## The contract

Each custom gate is executed once per `specd check` with a **bounded timeout**
and a **scrubbed environment** (only the `SPECD_*` namespace and the standard
allowlist survive — inherited secrets are dropped). There is no Go plugin
loading and no network access from specd.

### stdin — `CustomGateInput` (JSON)

```json
{
  "spec": "my-feature",
  "root": "/abs/path/to/project",
  "status": "executing",
  "tasks": [
    { "id": "T1", "status": "complete", "role": "builder", "wave": 1 }
  ]
}
```

### stdout — `CustomGateOutput` (JSON)

```json
{
  "violations": [ { "location": "T3", "message": "task references a TODO" } ],
  "warnings":   [ { "location": "design.md", "message": "section is thin" } ]
}
```

- Emit `{}` (or empty arrays) to pass.
- `violations` fail the check when the gate's severity is `error`.
- A non-zero exit, a timeout, or unparseable stdout is itself treated as a gate
  failure (fail-closed) — a broken custom gate cannot silently pass.

## Guarantees

- The seven core gates run **identically** with or without custom gates
  configured; custom gates only ever *add* findings.
- Custom gates run in a scrubbed env with a bounded timeout; they cannot see
  inherited secrets and cannot wedge the check indefinitely.
- No Go plugin (`plugin` package) loading; no network calls from the specd
  binary.
