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
- V10: Command palette, registry, help, and machine surfaces derive from one command set; adding a verb cannot require stale literal counts.
- V11: Troubleshooting docs retain exact tested runtime error vocabulary and exit-code meanings when extended.
- V12: Lifecycle approval advances exactly one status; same-status, skipped, backward, and unknown targets fail before mutation.
- V13: Every public operation declares actor, side effect, phase, authority, and scope semantics once; mutating operation never projects as read-only.
- V14: Generated agent guidance exposes complete executable task loop; verify evidence and task completion stay distinct, narrow, and no-bypass.
- V15: Fresh scaffold ships progressively loaded current-schema skills and production-shaped authoring templates; prose never widens authority.
- V16: Normative docs, command examples, machine guidance, and runtime behavior agree and are black-box tested from fresh init.
- V17: Program rollup derives from domain task truth; clean machine diagnostics return typed empty success, never ambiguous null.

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
| T9 | . | exact lifecycle and approval contract | V12,V16,I1,I2 |
| T10 | . | canonical operation effect/authority contract | V13,V16,I2 |
| T11 | . | executable evidence-to-completion agent loop | V14,V16,I2 |
| T12 | . | progressive skills and production authoring templates | V15,V16,I1 |
| T13 | . | documentation, diagnostics, and rollup truth | V16,V17,I1,I2 |
| T14 | . | fresh-project workflow coherence release proof | V12,V13,V14,V15,V16,V17,I1,I2 |

## §B

| id | date | cause | fix |
|---|---|---|---|
| B1 | 2026-07-12 | managed-marker test hardcoded `v1`; valid template bump failed suite | V9 |
| B2 | 2026-07-12 | schema-migration fixture used never-declared `build` mode; mode validation correctly exposed stale fixture | V7,R3.1 |
| B3 | 2026-07-12 | 08d transition-table test assigned through an out-of-scope local identifier; compile failed before behavior ran | mechanical typo; no new invariant |
| B4 | 2026-07-12 | 08d additive test guessed a nonexistent exported evidence-gate API instead of using package-local gate contract | align test with `evidence(CheckCtx)`; no new invariant |
| B5 | 2026-07-13 | command palette growth broke hardcoded help-count fixture | V10 |
| B6 | 2026-07-13 | W8 conformance test guessed injection rule label instead of asserting existing scanner vocabulary | mechanical test typo; no new invariant |
| B7 | 2026-07-13 | new troubleshooting page documented W8 only and omitted pre-existing tested CAS/exit contracts | V11 |
| B8 | 2026-07-13 | W7 MCP test assumed forbidden `task` command must be exposed as a tool | align test with legal derived palette; V10 already applies |
| B9 | 2026-07-13 | W0 token-conflation characterization correctly failed when planned W7 expansion closed gap | flip seeded characterization to positive R7 contract; no new invariant |
| B10 | 2026-07-13 | previously approved 06 W8 task rows were complete while program rollup checkbox drifted pending | restore rollup from domain truth; existing progress-order regression caught drift |
| B11 | 2026-07-13 | 08l RED tests referenced incident and portfolio contracts before their planned implementation | expected TDD failure; no new invariant |
| B12 | 2026-07-13 | 08l added public `incident` verb while W5 regression still pinned prior verb count 29 | update intentional surface tripwire to 30; V10 already applies |
| B13 | 2026-07-13 | 09b RED test called a nonexistent setup helper before intended contract failures compiled | mechanical test typo; no new invariant |
| B14 | 2026-07-13 | 09b approval marking first appended checkbox as an extra table cell instead of prefixing task id | mechanical formatting error; no new invariant |
| B15 | 2026-07-16 | W2 operation projection changed MCP identity while command-keyed derivation and telemetry fixtures remained stale | align fixtures with operation IDs; V13/V16 already apply |
| B16 | 2026-07-16 | hyphenated `complete-task` exposed docs parity regex and production fixtures pinned to legacy completion route | widen parser and migrate executable fixtures; V10,V14,V16 already apply |
