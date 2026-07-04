# Configuration Reference

> `specd` uses a **fail-loud, pure YAML-subset** configuration: a parse error is a hard
> exit, never a silent default (ADR-2). This file is the single authoritative reference
> for every config key and environment variable.
>
> **Source of truth:** `internal/core/config_loader.go` (struct defaults) and
> `internal/core/config_validate.go` (key parsing + env overrides).

---

## Config file resolution

Configuration layers in priority order (last wins):

```
1. Global config   — $SPECD_CONFIG_GLOBAL  (optional; not yet wired in binary)
2. Project config  — <project-root>/project.yml
3. Environment     — SPECD_* variables
```

> **Open finding (F10):** The binary currently reads `project.yml` (not `config.yml` as ADR-2
> specifies). The rename is planned for Wave P5. The file described here is `project.yml`
> until that wave lands. Config file seeding by `specd init` is also deferred to Wave P5.

---

## YAML format

The YAML subset is deliberately minimal — two-space indentation, no arrays, no multi-line
scalars. Parse errors are hard exits (never silent):

```yaml
version: "1"
agent: codex

context:
  max_tokens: 12000

gates:
  verify: error

orchestration:
  enabled: false
  model: ""

promotion_threshold: 3
```

---

## Config key reference

### `version`

| | |
|---|---|
| **Type** | string |
| **Default** | `"1"` |
| **Env override** | — |

Config schema version. Reserved for future migration gating.

---

### `agent`

| | |
|---|---|
| **Type** | string |
| **Default** | `"codex"` |
| **Env override** | `SPECD_AGENT` |

The default agent harness identifier. Used by `handshake bootstrap` and host-adapter detection.

---

### `context.max_tokens`

| | |
|---|---|
| **Type** | positive integer |
| **Default** | `12000` |
| **Env override** | `SPECD_CONTEXT_MAX_TOKENS` |

Maximum token budget for context manifests. The context engine uses a pure heuristic
(`ceil(len/4)`) — no LLM tokenizer. When the optional context-budget gate is enabled,
manifests that exceed this budget cause `check` to emit an error finding.

Must be a positive integer; zero or non-integer is a hard config error.

---

### `gates.verify`

| | |
|---|---|
| **Type** | string: `warn` \| `error` |
| **Default** | `"error"` |
| **Env override** | `SPECD_GATES_VERIFY` |

Minimum severity floor for the evidence gate (gate 5). Can be raised but not lowered —
the evidence gate severity is always at least `error` per ADR-4. Configuring `warn` here
is a no-op for that gate.

---

### `orchestration.enabled`

| | |
|---|---|
| **Type** | boolean |
| **Default** | `false` |
| **Env override** | `SPECD_ORCHESTRATION_ENABLED` |

Master switch for the orchestration tier. **Fail-closed by default** — the entire Brain/Pinky
tier is inert when this is `false`. CLI output and `check` output are byte-identical whether
orchestration is enabled or not (when it is off).

Setting this to `true` without also setting the spec's mode to `orchestrated` is insufficient;
`brain start` additionally requires `mode: orchestrated` in `state.json`.

---

### `orchestration.model`

| | |
|---|---|
| **Type** | string |
| **Default** | `""` (empty) |
| **Env override** | `SPECD_ORCHESTRATION_MODEL` |

Model hint for the orchestration controller. Currently informational — `Decide()` is a pure
function that never calls an LLM. Reserved for future use when the controller gains a
model-routing tier (deferred, ADR-3).

---

### `promotion_threshold`

| | |
|---|---|
| **Type** | integer ≥ 1 |
| **Default** | `3` |
| **Env override** | `SPECD_PROMOTION_THRESHOLD` |

Number of occurrences before a per-spec steering-memory pattern is automatically promoted
to the global steering memory. Used by `specd memory promote`. Use `--force` to override.

---

## Environment variable index

All `SPECD_*` environment variables override their corresponding config-file keys.
The verify executor also passes these through its scrubbed env allowlist, so
they are available inside `verify:` commands.

| Variable | Config key equivalent | Default |
|---|---|---|
| `SPECD_AGENT` | `agent` | `"codex"` |
| `SPECD_CONTEXT_MAX_TOKENS` | `context.max_tokens` | `12000` |
| `SPECD_GATES_VERIFY` | `gates.verify` | `"error"` |
| `SPECD_ORCHESTRATION_ENABLED` | `orchestration.enabled` | `false` |
| `SPECD_ORCHESTRATION_MODEL` | `orchestration.model` | `""` |
| `SPECD_PROMOTION_THRESHOLD` | `promotion_threshold` | `3` |
| `SPECD_ACTOR` | — (record actor identity) | OS username |
| `SPECD_LOCK_STALE_MS` | — (advisory lock stale reclaim) | `30000` (30s) |
| `SPECD_LOCK_TIMEOUT_MS` | — (advisory lock acquire timeout) | `5000` (5s) |
| `SPECD_VERIFY_SHELL` | — (shell for verify commands) | `"sh"` |

### Scrubbed env allowlist (verify executor)

The verify executor runs `verify:` commands in a scrubbed environment containing only:

```
PATH  HOME  LANG  LC_ALL  TMPDIR  SPECD_*
```

Any variable not in this list is stripped before the subprocess runs. This is a security
and determinism property — `tasks.md` is treated as hostile input and its verify commands
cannot rely on arbitrary env state from the caller's shell.

---

## Secret scrubbing

Config keys containing `secret`, `token`, `apikey`, or `api_key` (case-insensitive, any
separator) are **rejected with a hard error**. This prevents accidentally storing credentials
in `project.yml`. If you need to pass secrets to verify commands, use a wrapper script
that pulls from a vault and receives the secret via a non-`SPECD_*` variable that is then
set inside the script.
