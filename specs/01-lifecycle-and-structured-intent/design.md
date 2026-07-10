# Design — lifecycle structured intent

## Decision

Extend plain-Markdown contracts with deterministic parsers. Persist only validated, compact IDs/digests/record metadata in `state.json`. Keep source Markdown authoritative. New policy is profile-gated except safety fixes that correct documented existing behavior.

Human architecture owner: project approver, recorded by approved design disposition. Harness validates presence/identity/freshness; never judges architecture choice.

## Contracts

| Contract | Source | Parsed identity | Gate use |
|---|---|---|---|
| requirement | `requirements.md` | `R<n>`, `R<n>.<n>` criterion | design/task/coverage refs |
| design unit | `design.md` | stable section/component ID | task refs, boundary policy |
| task | `tasks.md` | `T<n>` plus trace/risk metadata | frontier, planning policy |
| approval | `state.json` record | artifact/config digest + revision | current/stale decision |
| amendment | `state.json` record | change ID, affected IDs, before/after digest | impact/staleness |
| guidance | CLI/MCP JSON | phase, actor, legal actions, blockers | host driving |
| spike | Markdown/state record | question, scope, expiry, output ref | learning without bypass |

## Data and migration

1. Add pure parsers and normalized models before gate registration.
2. Add explicit `schema_version` migration for new state record fields.
3. Preserve old Markdown/task tables. Add extension metadata in backward-compatible parseable block/table shape selected during implementation; parser rewrite must preserve bytes outside changed marker.
4. Store artifact/config/policy digests beside approvals/evidence. Imported/model text never becomes pass until parser + identity/freshness checks succeed.

## Gate flow

```text
requirements parser → requirement gate → human requirements approval(digest)
                                      ↓
design parser/trace → design gate → human design approval(digest)
                                      ↓
task parser/trace/risk → DAG + coverage + verify policy → execution approval
                                      ↓
amendment → impact graph → stale dependent records → dispatch pause/reapproval
```

## Staleness rules

Affected record stale if its direct requirement/design/task dependency intersects amendment affected IDs, or pinned source digest differs. Staleness append-only; historical pass remains visible but cannot authorize next transition. Unaffected records remain valid. Status remains monotonic; legality/dispatch derives from current records.

## Profile policy

`default`: explicit compatibility policy. `production`: activates criterion/current-HEAD review plus risk rules. Config digest participates in approval/evidence freshness. Risk rules must be deterministic mappings, role-aware, explainable, configurable.

## Verification layers

- Unit/golden: parser, migration, graph, digest, policy, stable JSON ordering.
- Command black-box: legal guidance, approval refusal, amendment/reapproval.
- Fresh repo: release binary lifecycle, restart, crash/CAS, concurrent mutation.
- Negative: unknown ID/role, shallow write verify, stale review, omitted integration error path, spike bypass.

## Risks

- Markdown syntax growth → version docs, migration tests, byte-stability tests.
- Verify heuristic false positives → profile/role allowlists, exact finding, no global opaque ban.
- Overbroad invalidation → graph fixtures prove unrelated current records stay current.
- Guidance drift → generate CLI/MCP/scaffold tests from one command metadata source.
