# Workflow — The Spec Lifecycle (frozen steering)

Follow this loop for every change request. Each `→` is a gate. Mutate state **only** through
`specd`. Never hand-edit `state.json`; never flip a `tasks.md` checkbox by hand.

```
0. INTAKE      Classify the request.
               - Question → answer only, stop.
               - Change → continue.
               - Ambiguous AND decision-changing → ask ONE clarification.
               - Else → pick the simplest default, state it, proceed.

1. PERCEIVE    `specd new <feature>`; read .specd/steering/* and the codebase.

2. ANALYZE     Write requirements.md in EARS (§7.1). → `specd check <feature>` (EARS gate).
   ▸ GATE: human approves requirements → `specd approve <feature>` (advances to design).

3. PLAN design Write design.md (approach, components, data, errors, verify strategy, risks).
   ▸ GATE: human approves design → `specd approve <feature>` (advances to tasks).

4. PLAN tasks  Write tasks.md as a wave DAG. Each task carries:
               id, why, role, files, contract, acceptance, verify, depends, requirements.
               → `specd check <feature>` (task-schema + DAG gates).
   ▸ GATE: human approves tasks → `specd approve <feature>` (advances to executing).

5. EXECUTE     Loop until done:
               a. `specd next <feature>`            → focused task.
               b. Adopt/delegate the assigned role  → builder implements ONE task.
               c. VERIFY: run the task's verify line; capture evidence.
               d. `specd task <feature> <Tn> --status complete --evidence "<proof>"`
                  (or `--status blocked --reason "..."` and STOP after one retry).
               e. Append memory.md learnings; `specd decision` for any deviation.
               f. `specd report <feature>` snapshot (optional).
               When the last task completes the spec enters status `verifying` (phase VERIFY).

5b. VERIFY     Spec-level check: run the config `defaultVerify` and confirm the acceptance
               criteria hold across the whole feature.
   ▸ GATE: human accepts verification → `specd approve <feature>` (advances verifying → complete).

6. REFLECT     Final summary. Promote cross-spec patterns: `specd memory <feature> promote`.
```

## Mid-flight feedback

Any new user input during execution →
`specd midreq <feature> "<verbatim>" --impact <low|medium|high|critical>`.
`high|critical` sets `gate = awaiting-approval`. When gated: **stop** (`specd next`/`task` refuse),
present the revised plan, then `specd approve <feature>` clears the gate and work resumes.

## Gates active every turn

- **Intake-first** — classify before acting.
- **Minimum context** — roles summarize, never dump.
- **Spec-grade tasks** — files + contract + verify named, or it is not a task.
- **Verify-before-done** — no `complete` without passing evidence.
- **Durable progress** — state lives in files, not in your head.
- **Blocked-means-stop** — one retry, then block and surface it.
- **Deviations recorded** — `specd decision`, never silent.

## Orientation commands (cheap, run often)

- `specd context <feature>` — phase-scoped briefing: what to load now + the next action. Run this
  at the start of a turn instead of dumping every doc into context.
- `specd status [<feature>]` — where am I.
- `specd next <feature>` — what's my next unit of work.
- `specd check <feature>` — am I allowed to advance.
