# specd — Agent Integration

How an AI coding agent (or an MCP client, or a custom harness) drives specd. specd is
**agent-agnostic**: it constrains *capability* through roles, not *identity*. Anything that
can run a command can drive it.

## The `AGENTS.md` loop

`specd init` writes `AGENTS.md` into the target repo root. It is the contract the agent loads:
what the harness expects, the command palette, and the base loop:

```
new → approve requirements → approve design → approve tasks
    → next → context → (edit) → verify → task complete → … → review → submit
```

The agent never mutates `.specd/` state directly — it drives the CLI, and the harness owns
every write.

## The four roles

Roles are prompt files under `.specd/roles/*.md` that constrain what an agent may do in a
task. Each task in `tasks.md` declares one, and the `roles` gate rejects unknown roles.

| Role | Capability |
|---|---|
| **scout** | Read-only exploration. Inspects the repo/steering/spec, reports findings. No writes. |
| **craftsman** | Write + verify. Edits only its declared files, runs `specd verify`, completes **exactly one atomic task** per invocation. |
| **validator** | Read-only. Runs the task's verify command and reports the specd-generated record. |
| **auditor** | Read-only. Audits a diff against the acceptance criteria; fills the review report. |

The capability split is currently convention plus structural gates. Role prose does not grant or
revoke host tools, and declared task files are not yet compared with the harness-derived diff.
Until production authority and scope gates land, hosts must enforce tool permissions and reviewers
must inspect actual changes; `specd check --security` remains an explicit migration check.

## Steering: the constitution

`.specd/steering/*.md` are durable files that outlive any single chat session — the project's
standing rules on structure, tech, product, reasoning discipline, workflow, and accumulated
memory. Agents load them every session. Unlike a spec (scoped, disposable), steering is the
persistent constitution.

### The learning flywheel

`specd memory` promotes durable patterns into steering memory:

```bash
specd memory payments add --key 'atomic writes' \
  --pattern 'use AtomicWrite' --criticality important \
  --related 'state,locking'
specd memory payments promote --key 'atomic writes'
```

`add` appends a pattern (keyed by an H2 heading, with optional wikilinks to related keys);
`promote` graduates a repeatedly-seen pattern into the standing steering set (past a
configurable threshold; `--force` overrides it). This is how one spec's lessons become the
next spec's defaults.

## Bounded context: the manifest

`specd context <slug> <task-id>` builds a **bounded, cited** context manifest for a single
task — only the files that task needs, within the `context.max_tokens` budget the
`context-budget` gate enforces. `--hud` renders an operator view (files, bytes, tokens, mode);
`--json` emits the machine-readable manifest.

`specd next <slug> --dispatch` emits that manifest for the first frontier task — a ready-to-run
dispatch packet for a worker.

Quality declarations add compact packet metadata: class/check IDs, verify command, artifact
refs/digests, freshness, and subject digests. Raw datasets, outputs, and traces stay external.
Auditor review checks integration/error/concurrency/rollback risks; required test evidence stays
mandatory.

## Orchestration: Brain / Pinky (opt-in)

For hands-off execution, the deterministic **Brain** controller drives the wave loop:

```bash
specd brain start payments --authority   # begin (authority is fail-closed by default)
specd brain step payments                # one decision step
specd brain run payments                 # run to a stopping point
specd brain status payments
specd brain resume payments              # after a checkpoint/interruption
specd brain cancel payments
```

Brain uses **leases** (a worker claims a mission), an append-only **ACP decision ledger**, and
**brakes/checkpoints** for safe interruption and recovery. Critically, **no LLM sits in the
decision path** — Brain's choices are deterministic functions of on-disk state. It dispatches
role-scoped missions to **Pinky** workers (scout/craftsman/validator/auditor), each of which
claims one mission, does exactly its role's work, and reports evidence back through the same
`specd verify` / `specd check` gates.

`--authority` is required to grant dispatch authority; without it Brain fails closed and only
plans.

## Cross-spec programs

Large efforts span multiple specs with ordering constraints. Record them:

```bash
specd link api auth        # 'api' depends on 'auth'
specd unlink api auth
specd status --program     # the program view: every spec, its links, phase, and frontier
```

When approving a spec's execution transition, the program-link gate refuses if any cross-spec
dependency is still incomplete. Planning phases are never program-gated.

## MCP

`specd mcp` serves the whole command palette as a stdio MCP server, so an MCP-native client
(Claude Code, Cursor, Antigravity, custom) can call verbs as tools. See
[mcp-guide.md](mcp-guide.md) for host config and the [handshake](mcp-guide.md#handshake).

---

**See also:** [command-reference.md](command-reference.md) · [validation-gates.md](validation-gates.md)
· [contributor-guide.md](contributor-guide.md)
