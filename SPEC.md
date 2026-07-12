# SPEC — lifecycle structured intent program

## §G

Make `specd` prove intent → design → task → evidence linkage; preserve human architecture approval, forward-only history, deterministic local gates.

## §C

- Go stdlib only. No LLM/network in core gates.
- Preserve atomic writes, CAS, lock, byte-stable task edits, evidence no-bypass.
- Backward-compatible Markdown/state migration; unknown ≠ pass/zero/empty.
- `reference/` untouched. CLI/docs mirrors stay synced.

## §I

- `.specd/specs/<slug>/{requirements.md,design.md,tasks.md,state.json}`
- `specd new|status|check|approve|midreq|decision|context|verify|report`
- `project.yml`, generated `AGENTS.md`, CLI/MCP machine guidance

## §V

- V1: Gates pure deterministic functions of local validated artifacts/state.
- V2: Requirement IDs, criterion IDs, refs, digests stable and addressable.
- V3: Design/task refs resolve only to approved/current intent; missing/unknown refs fail closed.
- V4: Amendments append history; stale dependent approvals/evidence block unsafe dispatch; status never rewinds.
- V5: Human owns architecture decision; harness proves structure/approval only.
- V6: Write work has risk-proportionate non-trivial evidence; read-only work may use explicit trivial verify.
- V7: Default behavior remains explicit/backward compatible; production policy opt-in and digest-pinned.
- V8: Guidance returns only legal actor-aware next actions for current phase.
- V9: Managed-marker tests derive expected version from `TemplateVersion`; template bumps remain valid.

## §T

| id | status | task | cites |
|---|---|---|---|
| T1 | . | lifecycle contract foundation spec | V1,V2,I1 |
| T2 | . | planning guardrail baseline spec | V1,V6,I2 |
| T3 | . | design trace and decision spec | V2,V3,V5,I1 |
| T4 | . | phase-native guidance spec | V8,I2 |
| T5 | . | task trace/risk spec | V2,V3,V6,I1 |
| T6 | . | coverage gate spec | V1,V3,I1 |
| T7 | . | amendment/staleness spec | V2,V4,I1,I2 |
| T8 | . | production profile/spike/conformance specs | V1,V4,V6,V7,V8,I1,I2 |

## §B

| id | date | cause | fix |
|---|---|---|---|
| B1 | 2026-07-12 | managed-marker test hardcoded `v1`; valid template bump failed suite | V9 |
| B2 | 2026-07-12 | schema-migration fixture used never-declared `build` mode; mode validation correctly exposed stale fixture | V7,R3.1 |
