---
name: specd-design
description: Author design.md for a specd spec. Load when entering the design (PLAN) phase. Covers the seven mandatory design.md sections and the `design` gate that `specd check` enforces, plus traceability back to the approved requirements.
---

# specd design

Phase PLAN (design): decide HOW the approved requirements get satisfied. Load
`.specd/specs/<slug>/requirements.md` plus `.specd/steering/tech.md` and
`structure.md`. Run `specd context <slug>` for the briefing.

## The mandatory `design.md` sections

The `design` gate requires every one of these `## ` headings to be present and
non-empty. `specd check <slug>` fails (exit 1) on any missing or empty section:

1. `## Overview`
2. `## Architecture`
3. `## Components and interfaces`
4. `## Data models`
5. `## Error handling`
6. `## Verification strategy`
7. `## Risks and open questions`

Match the headings exactly (case-insensitive, `## ` level). Empty sections and
`TODO` placeholders fail the gate.

## Grounding and traceability

- Every design choice must serve a requirement from the approved `requirements.md`.
  Do not introduce scope the requirements do not cover.
- `Verification strategy` is where you state how each requirement will be proven —
  it feeds the tasks' `verify:` lines and the spec-level `defaultVerify`.
- Record any notable architectural choice with `specd decision <slug> "<text>"`
  (ADR), especially deviations from steering.

## The gate `specd check` enforces here

- `design` — all seven sections present and non-empty.

## Exit and advance

```
specd check <slug>      # design gate green (exit 0)
specd approve <slug>    # human approves design → advances to tasks
```

Then load `specd-tasks` when the approve advances you into the tasks phase.
