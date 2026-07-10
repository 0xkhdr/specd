# Domain 02 — Context, knowledge, skills

## Goal

Make task context smallest complete deterministic bundle for current action. Harness selects,
validates, budgets, and pins references. Host loads bytes. Model never decides policy.

## Source

Derived from `docs/google-sdlc-alignment/02-context-knowledge-and-skills.md` and alignment
README P0-A/P1/P2. Source analysis identifies current false paths, absent design/files,
undercounted core tokens, unpinned freshness, broad static selection, absent skills, and
unsafe memory promotion.

## Ownership

| Area | Domain 02 owns | Other domain owns |
|---|---|---|
| Context | typed manifest, canonical source, selector, digest, budget, receipt | host fetch/load implementation |
| Guidance | context-visible route/authority metadata | Domain 03 legal next-action semantics |
| Roles/files | manifest consumes validated role/scope | Domain 01 role/task schema; Domain 06 enforcement |
| Evidence | receipt identity/freshness contract | Domain 04 evidence classes/gates |
| Skills | file package schema, static validation, selection | adapters execute no skill code |
| Memory | context selection/provenance/supersession policy | Domain 09 long-term governance |

## External prerequisites

Current task DAG accepts same-spec IDs only. Cross-domain links remain program dependencies until
Domain 01 program-link work exists. Do not encode foreign `T<n>` IDs in local `depends-on`.

- Domain 01 task metadata must expose stable declared files, requirement/design references, and
  known roles before exact selectors replace compatibility fallback.
- Domain 03 must define final actor/tool route fields before bootstrap contract is frozen.
- Domain 04 must consume receipt freshness before completion gates require it.
- Domain 06 must enforce role/scope/security; this domain only carries authoritative metadata.

## Release slices

- P0: truthful manifest path/content/budget/schema and driver bootstrap.
- P1: skills, examples, block-selected memory, provenance, receipt pinning.
- P2: deterministic relevance policy, context effectiveness metrics, portability profile.

## Non-goals

- No model/provider SDK, embedding, network fetch, prompt transcript, executable skill plugin,
  or authority expansion by Markdown.
- No silent required-context truncation, path fallback, or imported record treated as gate pass.
