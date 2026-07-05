# 0002 Review Gate Staging

Status: accepted

Context:
Spec 09 ports v1's review workflow as a gate: `specd review` scaffolds
`review_report.md`, and the opt-in `review.required` gate refuses the
verifying→complete transition without a fresh `approve` verdict pinned to the
current git HEAD. v1 also had richer machinery — `review checklist`
auto-extraction of acceptance items from design/tasks, and multi-reviewer quorum.

Decision:
Ship only the gate and the scaffold now. Defer:

- **Checklist auto-extraction** (`review checklist`): deriving a per-item review
  checklist from design/tasks. The per-task section in the scaffold (id, files,
  acceptance) already gives the reviewer the surface to audit; auto-extraction is
  additive polish, not a correctness requirement.
- **Multi-reviewer quorum**: requiring N distinct approve verdicts. Single
  approve-at-HEAD is the enforceable primitive; quorum is a policy layer to add
  only on demonstrated demand.
- **Reviewer identity verification**: the harness cannot prove who wrote the
  report. It checks *that* an approve exists at this HEAD, not *who* authored it.
  The auditor-fills-the-report / no-self-review rule is documented discipline
  (docs/validation-gates.md, `.claude/agents/pinky-auditor.md`), not a binary
  check — stated honestly rather than faked.

Consequences:
`review.required` is a HEAD-fresh approve check and nothing more. Revisiting any
deferred item means a new spec, not an edit to this record. The scaffold's field
layout (`Git HEAD`, `Verdict`, `Findings`) is the parse contract; changing it
means updating `internal/core/review.go` in lockstep.
