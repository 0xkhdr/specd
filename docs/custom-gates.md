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
| `sandbox` | Isolation backend for this gate's command: `none` (default), `bwrap`, or `container`. See [Trust boundary](#trust-boundary). |

## Trust boundary

The gate `command` is **trusted operator input** — it comes from your
`.specd/config.json`, not from agent-authored spec content. Because you wrote it,
it runs **on the host with no sandbox by default**, with only a scrubbed
environment (see [The contract](#the-contract)).

This is deliberately asymmetric with `verify`. A `verify` command can be derived
from agent-authored task content, so it runs under a **fail-closed sandbox**
(`bwrap`/container, `--network none`) — see `docs/validation-gates.md`. A custom
gate is operator-authored and operator-opt-in, so the default is host execution.

If you want parity with `verify`'s isolation — for example because your gate
command shells out to code you trust less, or you simply want defense in depth —
set `sandbox`:

```jsonc
{ "name": "no-todos", "command": "./scripts/no-todos.sh", "sandbox": "bwrap" }
```

- `sandbox` reuses the exact `verify` sandbox runner, so the backend semantics
  (read-only root, writable workspace bind, no network) are identical.
- It is **fail-closed**: if the chosen backend is unavailable (e.g. `bwrap` not on
  `PATH`, or `container` with no `SPECD_SANDBOX_IMAGE`), the gate errors rather
  than silently running unisolated.
- The **scrubbed environment is enforced in both modes** — sandboxing never
  widens what the gate can see.
- Leaving `sandbox` unset (or `"none"`) keeps the historical host execution
  byte-for-byte.

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
