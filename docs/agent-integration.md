# specd — Agent Integration

> **Status:** Normative documentation for current `specd` behavior.

How an AI coding agent (or an MCP client, or a custom harness) drives specd. specd is
**agent-agnostic**: it constrains *capability* through roles, not *identity*. Anything that
can run a command can drive it.

## Resolve request mode first

Loading a repository does not activate specd. Route the request as `general`, `consult`, or
explicitly activated `managed` before following any lifecycle guidance. General mode runs no
specd command. Managed mode begins with `specd handshake bootstrap <slug> --json`; switching
mode or managed spec invalidates authority issued for the previous route.

Guides report the host assurance level. If the host cannot enforce actor, path, tool, and
network restrictions, that level is **advisory** and must not be described as enforcement.

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
| **validator** | Workspace-read + harness-evidence-write. Runs the task's verify command and reports the specd-generated record. Not read-only: recording evidence is a write. |
| **auditor** | Read-only. Audits a diff against the acceptance criteria; fills the review report. |

Authority comes from the machine-readable role capability contract (`internal/core/roles.go`),
never from role prose. Role Markdown explains the role to a reader; when the two disagree a
conformance test fails rather than a runtime resolving the conflict.

The capability split is currently convention plus structural gates. Role prose does not grant or
revoke host tools, and declared task files are not yet compared with the harness-derived diff.
Until production authority and scope gates land, hosts must enforce tool permissions and reviewers
must inspect actual changes; `specd check --security` remains an explicit migration check.

## Assurance: what is actually enforced, and by whom

Machine-readable responses carry an `assurance` level so a driver never has to infer how much
the session is worth:

| Level | Meaning |
|---|---|
| `advisory` | Findings are reported. Nothing is contained. |
| `gated` | Harness gates block the transition, but execution is not isolated. |
| `sandboxed` | Gated, and the host isolates execution. |

The level only ever moves down. A host that declares no sandbox support in the MCP `initialize`
handshake is reported as `advisory`, and an unrecognized stored level degrades to `advisory`
rather than being guessed upward — advertising containment nobody provides is the one failure
mode worth designing against.

**specd never enforces a host tool permission, a filesystem boundary, or a network policy.** It
has no mechanism to. Isolation is the host's job in every profile; the profiles below change
which *gates* run, not what the harness can contain.

### Default profile vs production profile

The boundary is explicit because the two profiles fail differently, not because one is "stricter":

| | default profile | production profile |
|---|---|---|
| Evidence-gated completion | enforced | enforced |
| Acyclic DAG, task schema, EARS, sync, approval gates | enforced | enforced |
| Design contract present | not checked | required |
| Task → requirement trace | not checked | required |
| Steering memory lint | not checked | required |
| Integration policy | inert | armed |
| Host tool/path/network isolation | **not enforced — host's job** | **not enforced — host's job** |

Everything enforced in the default profile stays enforced in production; production adds
authoring-time checks that would be noise on an early spec and are non-negotiable on a shipped
one. Neither profile makes specd a sandbox. Run `specd check --security` to see the explicit
migration checks between them.

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

**Two steering paths, and they differ.** Plain `specd context <slug> <task-id>` lists *every*
steering file in the directory. `--json` applies `specd-context` selection: only steering files
carrying a `specd-context` block are included, and the `--json` manifest is what drivers, the
brain, and the MCP surface actually consume. A steering file without a `specd-context` block is
silently dropped from the machine path — so trust `--json`, not the plain listing, when checking
what a worker will see. (When *every* steering file is dropped for missing metadata, `specd check`
raises a warning; a single dropped file stays silent.)

`specd next <slug> --dispatch` emits that manifest for the first frontier task — a ready-to-run
dispatch packet for a worker.

**Typed lanes.** Every `--json` item carries a `lane`, and an `existence` plus `loaded` pair, so
"absent" and "shed" stop looking alike:

| lane | meaning |
|---|---|
| `required_input` | a `context`-column file the task must read; missing or unreadable fails closed |
| `optional_existing_output` | a declared output that already exists — current content is loaded |
| `prospective_output` | a declared output that does not exist yet — write authority, no content, no digest, no budget cost |
| `directory_query` | one file matched by an explicit bounded selector |
| `managed_policy` | harness-owned metadata (selected task, role, steering, skills, config) |

A greenfield task therefore dispatches normally: its unwritten outputs arrive as
`prospective_output` lanes rather than failing context. Budget counts only required and loaded
lanes, and a prospective lane is never shed — shedding it would silently revoke write authority.

The manifest also carries `assurance`. `specd` cannot prove host containment from inside the
process, so the packet reports the fail-safe `advisory`: gates ran, nothing was isolated. Treat a
higher level as claimed only when a host proves isolation; nothing in the manifest can raise it.

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

**Worker operations** under Brain:

```bash
specd brain claim payments <mission-id> <worker-id> <role>  # claim a dispatched mission
specd brain heartbeat payments <lease-id> <worker-id>       # renew the lease (keep the mission alive)
specd brain report payments <lease-id> <worker-id>          # report completion and mark task done
specd brain release payments <mission-id>                   # immediately release a mission (R4.3)
```

Brain uses **leases** (a worker claims a mission), an append-only **ACP decision ledger**, and
**brakes/checkpoints** for safe interruption and recovery. Critically, **no LLM sits in the
decision path** — Brain's choices are deterministic functions of on-disk state. It dispatches
role-scoped missions to **Pinky** workers (scout/craftsman/validator/auditor), each of which
claims one mission, does exactly its role's work, and reports evidence back through the same
`specd verify` / `specd check` gates.

`--authority` is required to grant dispatch authority; without it Brain fails closed and only
plans. Dispatch, claim, heartbeat, report, and release preserve session and lease authority
bindings across invocations (R4.5). Zero-progress permanent halt returns non-success with durable
checkpoint effects (R6.3).

### Reaching an approval gate

When the last task completes, Brain runs out of work but the spec still cannot advance: a
lifecycle gate needs approving. That is not a finished run and not an error, so Brain halts with
its own outcome, `waiting_approval`, and returns **non-success** — a pipeline reading exit 0 there
would call an unapproved spec done.

```
brain run: waiting_approval  (waiting_approval: lifecycle gate tasks requires human approval; run `specd approve payments`)
APPROVAL_REQUIRED: brain run reached the tasks approval gate after 2 dispatch(es); ...
```

The halt is recorded on the session as `waiting_approval` and shown by `specd status <spec>`.
Nothing else about the session changes: leases, missions, and the step counter survive, so the
next run resumes where this one stopped rather than restarting.

Two routes clear it, and both converge on the same approval record:

- a human runs `specd approve <spec>`;
- an operator-issued grant is used with `specd delegate approve <spec> --grant <id> --token <bearer>`
  (see [unattended-approval.md](unattended-approval.md)).

Passing `--grant <id>` to `brain run` makes the controller *name* the delegated route when that
grant already covers the transition. It never mints, widens, or spends one — the controller is
never given the bearer token, and a grant that is expired, revoked, exhausted, or out of scope is
simply not offered, leaving the human route standing. If the readiness gates are failing, neither
route is offered: the halt names `specd check <spec>` instead, because no authority approves past
a failing gate.

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
