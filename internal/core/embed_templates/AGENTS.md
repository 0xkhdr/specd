# AGENTS.md — How any agent drives this repo

This repo uses **specd**, an agent-agnostic, spec-driven harness (Kiro spec workflow + structured reasoning). You drive it entirely through the `specd` CLI via your shell tool. No API, plugin, or
MCP is needed — if you can run a shell command, you can run this harness.

**Foundational Split:** specd core is deterministic and makes zero LLM calls — *you* do all
creative thinking, perceiving, and authoring; the harness only scaffolds and enforces gates.
Brain schedules deterministically; it never thinks. Don't ask the core to reason.

## Five rules (non-negotiable)

1. **Load context first.** At session start run `specd handshake bootstrap --json` when available,
   cache command/config digests, then read always-on steering files
   `.specd/steering/{reasoning,workflow,product,tech,structure}.md`. Before acting on a spec run
   `specd handshake policy <spec> --expect-config-digest <cached> --json` and obey its mode/config
   decision. The sixth steering file, `memory.md`, is loaded phase-scoped (EXECUTE + REFLECT) —
   `specd context <spec>` tells you exactly what to load when.

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
   - Windows orchestration is POSIX-only and fails fast with a clear WSL message; non-orchestration workflow remains portable.
   - `specd init [--orchestration <policy>]` — bootstrap and configure the Brain/Pinky orchestration stack.

   MCP hosts: prefer the **intent-level tools** (`brain_orchestrate`, `brain_status`, …);
   `specd_brain`/`specd_pinky` are raw passthrough for flags the intent tools don't surface —
   see `docs/agent-integration.md`.

4. **Adopt roles** from `.specd/roles/*` when executing: scout (read-only research),
   craftsman (write ONE task), auditor (read-only audit), validator (run checks), brain (deterministic
   controller), or pinky (host worker). If your host has native subagents and
   `config.json.roles.subagentMode = "delegate"`, spawn role-bound subagents for implementation
   work: Simple mode uses `specd dispatch --json` packets, Orchestrated mode uses Brain/Pinky
   missions and the scaffolded `.claude/agents/pinky-{craftsman,scout,auditor,validator}.md`
   workers. If the host lacks subagents, say so inline before work and run the role inline under
   the same constraints.

5. **Evidence gate.** Never mark a task complete without a passing verify or a manual proof, and
   pass that proof as `--evidence`. A craftsman's word is not evidence. Pinky completion reports
   must bind to a matching verification record; host-reported telemetry (tokens, cost, duration) is stored as metadata and is not proof of correctness.

## Optional slash/workflow wrappers

Some hosts can map `/init`, `/steer`, `/spec`, and `/pinky-brain` to the shipped
`scripts/specd-workflow.{sh,py}` wrappers. Treat them as UX glue only: `/spec check`
means native `specd check`, `/spec continue` means `specd context` plus `specd next`
when executing, and `/pinky-brain` delegates to Brain/Pinky or read-only status views.
Wrappers never bypass gates, never complete tasks, never edit `state.json`, and never
forge Pinky reports. If wrapper behavior is unclear, use native `specd` directly.

## Execution mode — Simple vs Orchestrated (per spec, user decides)

Every spec records its own **execution mode** in `state.json` (`specd mode <spec>` shows it).
Simple is the default and the broad-compatibility path; orchestration is always an explicit
opt-in. Capability vs selection are distinct: project `orchestration.enabled` only *permits*
orchestration, while a spec's `executionMode` *selects* it.

1. **Default Simple.** "create/build/spec X" → author the spec in Simple mode. Do **not** start
   Brain/Pinky. In Simple you own every step (`specd next` → implement → `specd verify`).
2. **Explicit opt-in → Orchestrated.** "use Pinky and the Brain", "orchestrate this", "run it
   autonomously" → `specd mode <spec> --set orchestrated`, then drive with `specd brain run`.
   Brain/Pinky **refuse** Simple specs, pointing you back here.
3. **Recommend, don't impose.** After `tasks.md` is approved, consult
   `specd mode <spec> --recommend --json`. On `suggest`/`strong`, surface a one-line suggestion
   (e.g. "23 tasks across wide waves — run with Brain/Pinky, or proceed normally?") and **wait
   for the user**. Never switch without a yes; the verdict is advisory (`userDecides: true`).
4. **Respect the recorded mode.** On later actions read `spec.executionMode` and follow it —
   don't re-litigate each turn.

## What loads when

`specd context <spec>` and its `contextManifest` are authoritative for the minimal file set per
phase, including targeted selectors and over-budget actions. This table is a hint, not a
substitute — **re-run `specd context <spec>` each turn; don't trust this from memory**
(phases change what's in scope).

| Phase | Loads (beyond always-on steering) |
|-------|-----------------------------------|
| INTAKE / PERCEIVE / ANALYZE | spec `requirements.md` as it forms |
| PLAN | `requirements.md`, `design.md`, `tasks.md` |
| EXECUTE | `tasks.md`, `memory.md` |
| VERIFY | `tasks.md`, verification records |
| REFLECT | `memory.md`, `decisions.md` |

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
# Brain decisions: dispatch -> spawn Pinky; wait -> backoff/step; awaiting-approval -> ask human;
# escalate/policy-violation -> stop and report; complete-session -> final summary.
# Pinky lifecycle: claim -> heartbeat/progress -> verify -> report/block -> release.
# Terminal reports require matching --verification-ref; tokens/cost/duration are telemetry only.
# orchestration defaults (approvalPolicy, maxWorkers, maxRetries, sessionTimeoutMinutes,
# leaseSeconds, …) live in config.json.orchestration; set them via `specd init --orchestration*`.
# Flags below override per-run; omit them to use the configured defaults.
# specd brain start my-feature
# specd pinky claim --mission mission.json
# specd pinky heartbeat --session s --worker w --attempt 1
# specd verify my-feature T1
# specd pinky report --session s --worker w --spec my-feature --task T1 --attempt 1 --verification-ref ref --summary "done"
# specd brain step my-feature --session s
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
