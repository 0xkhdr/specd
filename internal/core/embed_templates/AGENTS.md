# AGENTS.md — How any agent drives this repo

This repo uses **specd**, an agent-agnostic, spec-driven harness (Kiro spec workflow + structured reasoning). You drive it entirely through the `specd` CLI via your shell tool. No API, plugin, or
MCP is needed — if you can run a shell command, you can run this harness.

## Five rules (non-negotiable)

1. **Load context first.** At the start of every session, read the always-on steering files
   `.specd/steering/{reasoning,workflow,product,tech,structure}.md`. The sixth, `memory.md`, is
   loaded phase-scoped (EXECUTE + REFLECT) — `specd context <spec>` tells you exactly what to load when.

2. **Follow the workflow** in `.specd/steering/workflow.md` — the INTAKE → PERCEIVE → ANALYZE →
   PLAN → EXECUTE → VERIFY → REFLECT lifecycle. Each `→` is a gate.

3. **Mutate state only through `specd`.** Never hand-edit `state.json`. Never flip a `tasks.md`
   checkbox yourself. Use:
   - `specd context <spec>` — phase-scoped briefing: the minimal files to load now + next action.
   - `specd status [<spec>]` — orient ("where am I").
   - `specd next <spec>` — get your next focused task.
   - `specd check <spec>` — before claiming any phase complete (and CI runs it on every push).
   - `specd approve <spec>` — record a human approval: advances the planning phase
     (requirements → design → tasks → executing), or clears a midreq `awaiting-approval` gate.
   - `specd verify <spec> <id>` — run the task's declared verification command and record its result.
   - `specd task <spec> <id> --status <s> ...` — the only way to flip a task.
   - `specd brain <start|run|step|status|why|directive|pause|resume|cancel> <spec> [flags]` — drive deterministic orchestration and bounded worker directives. (MCP: `specd_brain`)
   - `specd pinky <claim|heartbeat|progress|query|report|block|release|inbox> [flags]` — record deterministic worker leases, telemetry, bounded queries, progress, and terminal reports. (MCP: `specd_pinky`)
   - `specd init [--orchestration <policy>]` — bootstrap and configure the Brain/Pinky orchestration stack.

4. **Adopt roles** from `.specd/roles/*` when executing: investigator (read-only research),
   builder (write ONE task), reviewer (read-only audit), verifier (run checks), brain (deterministic
   controller), or pinky (host worker). If your host has native subagents and
   `config.json.roles.subagentMode = "delegate"`, delegate; otherwise run the role inline
   under the same constraints.

5. **Evidence gate.** Never mark a task complete without a passing verify or a manual proof, and
   pass that proof as `--evidence`. A builder's word is not evidence. Pinky completion reports
   must bind to a matching verification record; host-reported telemetry (tokens, cost, duration) is stored as metadata and is not proof of correctness.

## Skills — progressive disclosure

specd ships a skill pack under `.specd/skills/<name>/SKILL.md` — plain Markdown you
read with your shell. Read a stage skill **before** entering that stage and not
before, so you pay context only for the work in front of you.

| Skill | Read when |
|-------|-----------|
| `specd-foundations` | Once per session — the constitution + this index. |
| `specd-steering` | After `init`, before any spec — inspect the repo and author `product/structure/tech.md` + set `config.defaultVerify`. Replaces the old boot/enrich step. |
| `specd-requirements` | Entering the requirements phase (EARS + the `ears` gate). |
| `specd-design` | Entering the design phase (the 7 `design.md` sections + the `design` gate). |
| `specd-tasks` | Entering the tasks phase (wave DAG, 7 task keys, `task-schema`/`dag` gates). |
| `specd-execute` | Entering executing/verifying (the next→verify→complete loop + `evidence` gate). |
| `specd-brain` | Entering orchestration (sensing, deterministic stepping, program scheduling, no-LLM boundary). |
| `specd-pinky` | Operating a Pinky worker (context, claim, heartbeat, progress, query/inbox, blocker, report, release). |

## Quickstart

```
specd init                       # scaffold .specd/ + the skill pack (already done if you see this file)
# bootstrap steering: read .specd/skills/specd-steering/SKILL.md, then inspect the
# repo (manifests, dir tree, README, CI) and author product.md / structure.md /
# tech.md and set config.defaultVerify yourself — this replaces the old boot/enrich.
specd new my-feature --title "My Feature"
# write .specd/specs/my-feature/requirements.md (EARS), then:
specd check my-feature           # gate: requirements
specd approve my-feature         # human approves → advances to design
# write design.md, then tasks.md (wave DAG), then:
specd check my-feature           # gate: design + tasks + DAG
specd approve my-feature         # approve design → tasks
specd approve my-feature         # approve tasks  → executing
# execute loop (manual):
specd next my-feature            # -> focused task
specd verify my-feature T1       # run declared verification and record the result
specd task my-feature T1 --status complete --evidence "commit abc123; npm test PASS"
# execute loop (orchestrated):
# specd brain start my-feature --approval-policy manual --max-workers 4 --max-retries 2 --timeout-seconds 7200
# specd pinky claim --mission mission.json
# specd pinky heartbeat --session s --worker w --attempt 1
# specd verify my-feature T1
# specd pinky report --session s --worker w --spec my-feature --task T1 --attempt 1 --verification-ref ref --summary "done"
# specd brain step my-feature --session s --approval-policy manual --max-workers 4 --max-retries 2 --timeout-seconds 7200
# when the last task is done the spec enters `verifying`:
specd approve my-feature         # accept spec-level verification → complete
specd report my-feature          # snapshot
```

## The spec folder

Each feature lives in `.specd/specs/<slug>/` with six artifacts:
`requirements.md` (EARS) · `design.md` · `tasks.md` (wave DAG) · `decisions.md` (ADR) ·
`memory.md` (learnings) · `mid-requirements.md` (feedback log) · plus CLI-owned `state.json`.

The markdown files are your authored truth for *intent*. `state.json` is machine truth for
*status* — the CLI keeps `tasks.md` checkboxes and `state.json` in sync. Do not touch it directly.
