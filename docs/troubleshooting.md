# specd — Troubleshooting

The harness fails **closed** and explains why. This page maps the failures you will actually
hit to their cause and fix. Exit codes: `0` success, `1` gate/verify failure, `2` usage error
or fail-closed rejection.

## A task won't complete

`specd task complete <spec> <id>` refuses unless a **passing** verify record exists — exit 0
pinned to a resolvable git HEAD. There is **no bypass flag** (by design).

```bash
specd verify payments T3      # produce the evidence record first
specd task complete payments T3
```

If `verify` exited non-zero, fix the work (or the verify line) and re-run. The task stays
incomplete until the record passes.

## A task is blocked (escalation ratchet)

Repeated verify failures trip the **escalation ratchet**: after `escalation.max_verify_fails`
consecutive failing verify records (default **3**, since the last pass or override) the task is
escalated and blocked until a human clears it.

```bash
specd task T3 --override --reason 'flaky infra, verified manually'
```

`--override` **resets the ratchet** — it does *not* complete the task. You still need a passing
`specd verify`. `--reason` is required and must be non-empty. Set `escalation.max_verify_fails
= 0` in config to disable the ratchet entirely.

## `specd next` shows nothing runnable

The frontier is empty because either every task in the current wave is complete (approve/advance
to reveal the next wave) or a task is blocked (see above). Inspect state:

```bash
specd status payments          # current phase + per-task status
specd next payments --waves    # all wave groups, so you can see what's gating
```

## A verb is rejected for the wrong phase (exit 2)

Verbs are phase-gated. `post-requirements` verbs (`next`, `verify`, `context`, `brain`) fail
closed while the spec is still in `perceive`/requirements; `post-execution` verbs (`review`,
`submit`) need completed work. Check where the spec is with `specd status <spec>` and advance
through the gates with `specd approve`.

## A gate keeps failing on `check` / `approve`

`specd approve` advances a phase **only** when the relevant gates pass. Run the gate registry
directly and read the findings:

```bash
specd check payments            # human-readable findings
specd check payments --json     # machine-readable, one finding per gate
```

Common causes: `design.md`/`tasks.md` still at the scaffold stub (`design` gate), requirements
not in EARS shape (`ears` gate), or `tasks.md` markers disagreeing with `state.json`
(`sync` gate). See [validation-gates.md](validation-gates.md) for each gate's fix.

## `state revision conflict` (CAS failure)

`state.json` mutations compare-and-swap on a revision counter. A `state revision conflict` means
another writer advanced the state between your read and write — a concurrent `specd` process, or
a stale in-memory view. Re-run the command; it reloads the current revision and retries against
fresh state. If it persists, check for a second specd process holding the spec.

## Lock contention

Per-spec work is serialized by a reentrant lock (`.specd/specs/<slug>/.lock`). If a command
hangs waiting on the lock, another specd invocation is mid-write on the same spec — let it
finish. A truly stale `.lock` (from a killed process) can be removed manually; only do so once
you have confirmed no specd process is running against that spec.

## Verify sandbox unavailable

`specd verify --sandbox` runs the verify line inside a bwrap/container sandbox
(`config.verify.sandbox`). If the sandbox binary is missing it **fails closed** (exit 127,
"sandbox binary … unavailable") rather than silently running unsandboxed. Install `bwrap`, or
point `--sandbox-binary=<path>` at your sandbox, or drop `--sandbox` to run directly.

## Schema errors on load

`state.json` is loaded with unknown fields disallowed and validated on every read. A malformed
file surfaces as a `schema` gate finding:

```bash
specd check payments --schema-only
```

Fix the JSON (or restore it from git) — the file is harness-owned, so hand edits are the usual
culprit. See [open-spec-format.md](open-spec-format.md) for the schema.

## Handshake digest mismatch

`handshake bootstrap --expect-palette-digest` / `--expect-config-digest` fail (exit 1) when the
running binary's palette or effective config differs from what your agent pinned. That is the
check working: rebuild against the current binary, or re-pin the digest after an intended
change. See [mcp-guide.md](mcp-guide.md#handshake).

---

**See also:** [user-guide.md](user-guide.md) · [validation-gates.md](validation-gates.md) ·
[command-reference.md](command-reference.md)
