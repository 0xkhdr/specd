---
name: specd-review
description: Run the specd review workflow for a spec. Load when a spec is in `verifying` and you must produce `review_report.md` before `specd approve`, or when the `review` gate is blocking completion. Covers the report structure, the reviewer role brief, `review checklist`, and how the gate validates the report.
---

# specd review

`specd review <slug>` scaffolds a structured `review_report.md` in the spec dir
and prints the read-only adversarial reviewer brief. The binary never judges the
change — it only checks that a well-formed report with a verdict exists and is
fresh. **You** do the reviewing.

## Workflow

```
specd review <slug>              # scaffold review_report.md + print reviewer brief
# fill every section against the diff, design.md and tasks.md contracts
specd review <slug> checklist    # deterministic checklist from design + tasks
specd approve <slug>             # verifying -> complete (blocked until report passes gate)
```

## The report

`review_report.md` has six required sections — leave none blank (write
"none found" explicitly):

- **Summary** — what the change does, in your own words.
- **Bugs** — correctness defects, contract drift, missed edge cases.
- **Security** — secrets, injection, unsafe exec, hostile-input gaps. Run
  `specd check --security` and fold its findings in here.
- **Hallucinated Dependencies** — imports/packages/APIs that do not exist.
- **Style** — only nits that change meaning or violate steering conventions.
- **Verdict** — `approve` or `request-changes` with one line of justification.

Tag findings `critical | high | medium | low`, one per line:
`path:line: <severity>: <problem>. <fix>.`

## The gate

When `config.review.required` is on (new inits on, migrated repos off), the
`review` gate blocks `approve` from moving `verifying → complete` until the
report:

- exists and is structurally valid (all sections present),
- carries a `Verdict` of `approve`,
- is **newer than the latest task completion** (a stale report is rejected — so
  edit the code, then re-review).

The verdict is advisory; human `specd approve` stays final. Turn the gate off
per repo with `config.review.required=false`.

## Rules

- Read-only mindset: suggest fixes, do not apply them in the review pass.
- Be adversarial — verify every claim against the code, not the description.
- The reviewer role definition lives at `.specd/roles/reviewer.md`; a reviewer
  sub-agent is seeded from it.
