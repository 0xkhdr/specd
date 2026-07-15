---
name: specd-brain
description: Operate Brain orchestration sessions. Load before running `specd brain start|run|status|step|why|directive|pause|resume|cancel` or program orchestration. Covers deterministic sensing, bounded stepping, approvals, dispatch, directives, escalation, replay, and the no-LLM-in-core boundary.
---

# specd brain

Brain is deterministic controller logic inside specd. It schedules; it does not think creatively, call models, edit source, or bypass gates.

## Mode gate (orchestrated specs only)

Brain drives a spec **only when its execution mode is `orchestrated`**. `specd brain start|step|run` refuse a Base spec with a remediation pointing at `specd mode <slug> --set orchestrated`. Switching a spec back to Base while a Brain session is active is refused — cancel the session first. Orchestration is always an explicit, per-spec opt-in (see `specd-foundations` and AGENTS.md "Execution mode").

## Boundary

- specd core makes zero LLM calls and imports no provider SDK.
- Hosts perform creative work by accepting Pinky missions.
- Brain senses only specd-owned state: `state.json`, phase/gate helpers, runnable frontier, verification records, ACP leases, and session events.
- Every mutating reconciliation uses existing locks/CAS/idempotency paths.

## Session loop

```
specd brain start <slug> --approval-policy manual --max-workers <n> --max-retries <n> --timeout-seconds <n>
specd brain status --session <id>
specd brain step <slug> --session <id> --approval-policy manual --max-workers <n> --max-retries <n> --timeout-seconds <n>
```

One `step` records one deterministic decision and performs at most one externally visible action: dispatch mission, retry mission, cancel directive, approval request, escalation, wait, compact, or completion. Manual `brain directive` is reserved for bounded worker query replies or operator corrections under an active lease.

## Context compaction (`/clear`)

- A `compact` decision means you (the host) shed conversation context. Read the summary file recorded under `.specd/runtime/sessions/<id>/compact-<phase>-<turn>.md`, run the real `/clear`, keep only that summary, then resume with `specd brain step`.
- Automatic compaction is opt-in via `--compaction-policy <none|phase|budget|both>` (+ `--compaction-threshold <0..1>` for budget mode). Default `none`.
- Manual: `specd brain compact <slug> --session <id> [--reason <text>]`; `clear` is the `manual-clear` alias. `specd brain ledger <slug> --session <id> [--json]` prints peak tokens, compaction points, and budget history with no LLM calls.

## Approval rules

- Manual approval is default.
- Automated approval requires explicit bounded policy.
- High/critical mid-requirement and human-only gates remain human-only.
- Brain may request approval; it must not invent human consent.

## Dispatch and evidence

- Dispatch only tasks from runnable frontier and only within worker/lease limits.
- Each dispatched mission carries a budgeted `contextManifest`: required items first, measured `tokenHint`s summed into `estimatedTokens`, capped by `budget`. `read-targeted` items are slices, not whole files.
- Role prompt bytes appear once per wave via the shared `assets` map; packets reference the role by name + path (use `--inline-roles` only for hosts that cannot resolve paths).
- Pinky reports are untrusted until reconciled against specd verification records.
- Failed verification is never passing evidence; it returns work for correction or escalation.

## Escalate when

Unknown status, invalid graph, orphan/cycle, conflicting lease, policy violation, CAS exhaustion, retry exhaustion, or child/program failure. Escalation is a safe stop, not a bypass.
