# Domain 04 — Verification, evals, quality

## Goal

Make `specd` prove exact quality claim for exact subject. Keep `verify` deterministic,
offline, no-bypass base. Add explicit eval/trajectory contracts, current-subject freshness,
risk coverage, external-adapter import, governed datasets/rubrics. Never put model/network
calls in core gates.

## Source and intent

Derived from `docs/google-sdlc-alignment/README.md` and
`docs/google-sdlc-alignment/04-verification-evals-and-quality.md`.
Paper split: tests prove deterministic behavior; output evals assess produced artifact;
trajectory evals assess observable tools/actions. Rubric + labelled cases define quality;
passing command alone cannot solve 80% edge/integration/assumption problem.

Current foundation strong: append-only `verify` record, exit-zero/reachable-HEAD completion,
review gate, byte-stable task parser, optional sandbox. Missing: class semantics, artifact
schema/provenance, freshness, normalized trace, coverage quality, external eval import,
dataset/rubric governance.

## Ownership

| Area | Domain 04 owns | Other domain owns |
|---|---|---|
| Evidence | class envelope, import, freshness, completion composition | Domain 01 requirement/phase provenance |
| Quality | check/eval declarations, coverage lint, risk profile | Domain 06 production authority/security policy |
| Trajectory | observable trace schema and deterministic policy gate | Domain 05 worker dispatch/lease transport |
| Eval runners | adapter artifact contract/import only | Domain 10 transport/capability/data boundary |
| Context/report | quality packet fields and truth labels | Domain 02 selection/budget; Domain 07 telemetry/export |
| Review | quality contract references | Domain 09 recurring learning/incident program |

## Deliverable specs

| Wave | Slug | Result | Requires |
|---|---|---|---|
| W0 | `04a-quality-contract-baseline` | current behavior inventory; failing contract fixtures | — |
| W1 | `04b-evidence-envelope-and-task-contract` | versioned class/check declarations, byte-stable migration | 04a, Domain 01 task metadata |
| W2 | `04c-evidence-import-freshness-and-completion` | offline validated artifacts, current-subject gate | 04b |
| W2 | `04d-trajectory-envelope-and-policy` | sanitized ordered observable trace, required/forbidden checks | 04b, Domain 05 event identity |
| W3 | `04e-risk-coverage-and-verify-quality` | profile/risk acceptance-to-evidence lint | 04c, Domain 06 profile policy |
| W4 | `04f-eval-adapter-and-dataset-governance` | external runner contract, immutable dataset/rubric policy | 04c,04e, Domain 10 adapter contract |
| W5 | `04g-quality-context-review-and-flywheel` | compact quality packet, 80% review, deterministic quality ledger | 04d,04f, Domain 02/07/09 contracts |

## DAG

```text
04a → 04b ─┬─> 04c ─┬─> 04e ─> 04f ─> 04g
           │         │
           └─> 04d ──┘

Domain 05 event identity ─> 04d
Domain 06 production policy ─> 04e
Domain 10 adapter contract ─> 04f
Domain 02/07/09 contracts ─> 04g
```

## Program rules

1. Existing `verify` remains required non-bypass test evidence. Eval/review score cannot rescue it.
2. Core validates local, versioned, content-pinned artifacts. Adapters run judges/models/network.
3. Unknown class/version/id/digest, malformed JSONL, duplicate identity, missing required case,
   stale subject, or unavailable required evidence fails closed with stable local finding.
4. Trace stores observable sanitized events only: tool/result class/path effects/time/correlation.
   No hidden reasoning, raw secret, prompt, production sample body, or unbounded output.
5. Exact subject means reachable `git_head` plus policy-selected diff/output/trace digests. Unknown
   distinct from zero/pass/not-applicable. Historical result readable but never current by accident.
6. Declare contract before implementation task runs. Preserve old task files and CLI output until
   explicit schema migration fixtures pass. Stdlib-only; atomic writes/CAS/locks; no `reference/`.

## Completion claim

Production-profile task completes only after passing existing verify plus every declared evidence
class against current subject. Report names test/output/trajectory/review proof, provenance and
gaps. External judge outage never makes deterministic gate networked or silently green.
