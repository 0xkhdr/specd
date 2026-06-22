---
name: specd-execute
description: Drive the specd execution loop. Load when entering the executing/verifying (EXECUTE/VERIFY) phase. Covers the next→implement→verify→complete evidence loop, the four roles, `specd dispatch` for parallel subagents, and the `evidence` gate that blocks completion without proof.
---

# specd execute

Phase EXECUTE: build one task at a time, evidence-gated. Run `specd context <slug>`
for the briefing and load `.specd/specs/<slug>/tasks.md` + `memory.md`.

## The loop

```
specd next <slug>                       # → the single next runnable task
# adopt the task's role; implement ONLY that one task
specd verify <slug> <id>                # run the task's verify command, record result
specd task <slug> <id> --status complete --evidence "<proof>"
```

Repeat until the frontier is empty. When the last task completes, the spec
auto-derives to status `verifying`.

- **One task at a time.** `specd next` hands you exactly the runnable frontier;
  don't jump ahead of dependencies.
- If a task is stuck: `specd task <slug> <id> --status blocked --reason "..."`,
  then STOP after one retry and surface it (Blocked-means-stop).
- Record learnings in `memory.md`; record any deviation with `specd decision`.

## Roles

Adopt the task's assigned role from `.specd/roles/*`:

- **investigator** — read-only research (`verify: N/A` allowed).
- **builder** — writes exactly ONE task.
- **reviewer** — read-only audit (`verify: N/A` allowed).
- **verifier** — runs checks.

If your host has native subagents and `config.json.roles.subagentMode = "delegate"`,
delegate the role; otherwise run it inline under the same constraints.

## Parallel dispatch

`specd dispatch <slug> --json` emits ready-to-run packets for the **entire** current
runnable frontier — feed one packet per subagent to fan out a wave. Each subagent
still finishes through the evidence loop above.

## The evidence gate (hardest rule)

A builder's word is **not** evidence. `specd task <slug> <id> --status complete`
fails (exit 1) unless there is a passing verify record, or you pass `--unverified`
with a non-empty `--evidence` manual proof. The `evidence` gate in `specd check`
also flags any completed task lacking proof.

## Spec-level verify and close-out

```
# at status verifying: run the config defaultVerify and confirm acceptance holds
specd verify <slug> --criterion <r>.<n> --status pass --evidence "..."
specd check <slug>                      # all 7 gates green:
                                        #   ears · design · task-schema · dag · sync · traceability · evidence
specd approve <slug>                    # human accepts verification → complete
```

Then REFLECT: promote durable patterns with `specd memory <slug> promote`.
