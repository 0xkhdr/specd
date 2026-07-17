# specd Usage Guide — persistent working notes

> Personal operating guide for driving `specd`. Load this every session as shared memory.
> Source of truth stays the binary + `docs/`; this file is the distilled playbook.

## Mental model

- Process lives in the binary, not the prompt. **Agent reasons; harness enforces.**
- Every gate/DAG/report is a pure function of on-disk `.specd/` state. **No LLM in those paths.**
- Task completes **only** against a passing verify record (exit 0 pinned to a real git HEAD).
  **No bypass flag exists.**
- Subtractive bias: when unsure, cut/defer and record the decision.

## Runtime tree (inside the managed project, e.g. the Laravel repo)

```
project.yml                      ← config, YAML, repo root
AGENTS.md                        ← contract agent loads every session
.specd/roles/*.md                ← scout / craftsman / validator / auditor prompts
.specd/steering/*.md             ← durable constitution (tech/structure/memory)
.specd/specs/<slug>/
    requirements.md  design.md  tasks.md  state.json  .lock
```

## Lifecycle — 6 phases, each = 1 artifact + 1 human approval, fail-closed, no skip

```
perceive   → analyze → plan   → execute        → verify → reflect
requirements  design    tasks    code+evidence
```

## The four roles

| Role | Capability |
|---|---|
| scout | Read-only explore. Reports findings. No writes. |
| craftsman | Write + verify. Edits only declared files. Exactly ONE atomic task per invocation. |
| validator | Read-only. Runs the verify command, reports the record. |
| auditor | Read-only. Audits a diff vs acceptance. Fills review report. |

Caveat: role enforcement today = convention + structural gates. Prose does NOT revoke host
tools, and declared files are NOT yet diffed vs actual changes. Enforce tool limits via the
host (Claude Code permissions) and/or `profile: production`.

## Efficient path (hand-driven CLI, default profile) — the learning spine

```bash
cd <managed-repo>
specd init                                   # scaffolds .specd/ + AGENTS.md
git add AGENTS.md .specd && git commit -m "add specd harness"   # HEAD must be real
specd new <slug>

# requirements (EARS) → check → approve
specd check <slug> && specd approve <slug>
# design (past stub) → approve
specd approve <slug>
# tasks (DAG) → check → approve → now 'executing'
specd check <slug> && specd approve <slug>

# execute loop, per task (always these 4):
specd next <slug>                 # current wave (frontier)
specd context <slug> T1 --hud     # bounded context
#   ...edit code...
specd verify <slug> T1            # runs verify line, records exit0 + HEAD
specd complete-task <slug> T1     # succeeds ONLY if verify passed

# finish
specd status <slug> --json
specd report <slug> --history
specd submit <slug>               # runs ALL gates, streams PR summary
```

Escalation ratchet: 3 consecutive verify fails → task blocked until human clears it:
```bash
specd task T1 --override --reason 'flaky infra, verified manually'
```
`--override` resets ratchet; does NOT complete the task — still need a passing verify.

Mid-stream changes (keep audit trail):
```bash
specd midreq <slug> --text 'add refund path' --scope requirements
specd decision <slug> --text 'defer webhooks to v2' --scope design
```

## EARS requirement shapes

```
The system SHALL <response>.
WHEN <trigger>, the system SHALL <response>.
WHILE <state>, the system SHALL <response>.
IF <condition>, THEN the system SHALL <response>.
```

## Verify-line rules (critical)

- Craftsman (write) task using a trivial verify (`printf ok`, `true`, `:`) is **rejected** —
  it must exercise its own change.
- Read-only roles (scout/validator/auditor) MAY keep a trivial `printf ok`.
- Write the failing test in the SAME craftsman task as the code it checks (one atomic task).
- Commit before first verify — evidence pins to git HEAD.

## Claude Code MCP wiring

Generate the snippet, don't hand-write:
```bash
specd mcp --config claude-code --root /abs/path/to/repo --spec <slug>
```
```json
{ "mcpServers": { "specd": {
  "command": "specd", "args": ["mcp"],
  "cwd": "/abs/path/to/repo", "env": { "SPECD_SPEC": "<slug>" } } } }
```
- Tool surface derived from same palette as CLI — cannot drift.
- State-changing/session verbs refused over MCP by policy (`-32001`): `init`, `approve`,
  `brain`, `task`, `mcp`. Drive those from CLI with a human in the loop.
- Read/query/`verify`/`complete-task` work over MCP. **Approvals stay human at the CLI.**
- Drift guard: `specd handshake bootstrap <slug> --json` pins palette/config/managed digests;
  fails closed if the binary/rules moved under the agent.

## Orchestrator mode (Brain / Pinky) — opt-in, drives EXECUTE only

`project.yml`:
```yaml
orchestration:
  enabled: true
```
- Brain = deterministic controller (no LLM in decision path). Leases + append-only ACP ledger
  + brakes/checkpoints for safe interrupt/resume. Dispatches role-scoped Pinky workers.
- Pinky agent types: `pinky-scout|craftsman|validator|auditor`. Each claims ONE mission, does
  exactly its role work, reports evidence through the same verify/check gates. Spawn only when
  Brain dispatches — don't free-spawn.
```bash
specd brain start <slug> --authority   # REQUIRED; without it Brain only plans, fails closed
specd brain step   <slug>
specd brain run    <slug>              # run to stopping point
specd brain status <slug>
specd brain resume <slug>              # after checkpoint/interrupt
specd brain cancel <slug>
```
Requirements/design/tasks stay human-authored+approved. Brain only runs the wave loop.

## Forcing the agent to respect the philosophy (weak → strong)

1. **AGENTS.md** prose — base loop, loaded every session.
2. **Roles** — `roles` gate rejects unknown; structural only today.
3. **Trivial-verify rejection** — blocks fake verification on write tasks.
4. **`profile: production`** in `project.yml` — arms criterion + review + integration/
   negative-path evidence gates together; under production, MCP task ops require digest-pinned
   `AuthorityV1` mission packets (wrong spec/task/role/out-of-scope → fail closed).
5. **Escalation ratchet** (default 3) — hard-blocks repeated fake verifies until human override.
6. **Steering + memory flywheel** — teach durable rules once, inherited by future specs:
   ```bash
   specd memory <slug> add --key 'eloquent' --pattern 'use form requests for validation' --criticality important
   specd memory <slug> promote --key 'eloquent'
   ```

Standing rule to put in AGENTS.md/steering: never mutate `.specd/` directly; always drive via
CLI verbs; never invent a verify that doesn't exercise the change.

## config knobs (`project.yml`, repo root, YAML)

```yaml
profile: production            # or default (opt-in checks)
agent: codex                   # driving agent id
gates:
  verify: error                # warn|error
context:
  max_tokens: 50000            # context-budget gate ceiling
orchestration:
  enabled: true
  model: <model>               # no secrets in config — scrubbed on load
escalation:
  max_verify_fails: 3          # 0 disables ratchet
review:
  required: true               # arms review gate → specd review before submit
verify:
  timeout_seconds: 0           # 0 = unbounded; timeout recorded as exit 124 FAIL
submit:
  command: <shell line>        # empty = dry-run print summary
security:
  profile: production          # arms verify sandbox requirement
```
Env override precedence: env > project.yml > defaults (e.g. `SPECD_GATES_VERIFY`,
`SPECD_CONTEXT_MAX_TOKENS`, `SPECD_SPEC`).

## Current demo decision (Laravel "todo API")

- Mode chosen: **hand-driven CLI first**. Strictness: **default profile**.
- DAG sketch: W0 = T1 scout(printf ok) + T2 craftsman(migration+model+factory);
  W1 = T3 controller + T4 validation (deps T2, parallel);
  W2 = T5 routes (deps T3,T4) + T6 validator full suite.
- Verify lines = real artisan: `php artisan migrate:fresh && php artisan test --filter=...`.
- Watch-outs: commit before first verify; `migrate:fresh` wipes DB (dev/CI only); test lives
  in the same craftsman task as its code.

## Inspect / observe

```bash
specd status <slug>            # phase + task state (--json machine-readable)
specd status --program         # cross-spec view (links, phase, frontier)
specd report <slug>            # deterministic report (--metrics | --history)
specd next <slug> --dispatch   # dispatch packet (context manifest) for first frontier task
specd link api auth            # 'api' depends on 'auth' (program-link gate on execute)
```
