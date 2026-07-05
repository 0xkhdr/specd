# 04-criterion-evidence — Per-acceptance-criterion verify records

Wave 1. FINDINGS refs: B.1, D-tier1 item 6.

## Problem

Today evidence exists only at task granularity: a task completes against a
passing verify record. Nothing ever individually attests an approved
requirement's acceptance criteria — the gap between "task verify passed"
and "requirement satisfied" is unclosed. v1 had
`verify --criterion <r>.<n> --status pass|fail --evidence`, producing
per-criterion proof records distinct from task-level verify. FINDINGS
verdict: **port** — cheap (one flag path + one record shape), materially
strengthens the evidence story.

## Requirements (EARS)

- R1: WHEN a user runs `specd verify --criterion <r>.<n> --status pass
  --evidence <text-or-path>`, THE SYSTEM SHALL append a criterion record
  {criterion id, status, evidence, git HEAD, timestamp, actor} to the
  spec's evidence store.
- R2: WHEN `--criterion` references a requirement or criterion number that
  does not exist in the approved `requirements.md`, THE SYSTEM SHALL fail
  closed (exit 2) naming the unknown id.
- R3: THE criterion record SHALL pin the current resolvable git HEAD, same
  discipline as task verify records; a dirty-tree or unresolvable HEAD
  follows whatever the task-level verify path already enforces.
- R4: WHEN `--status fail` is recorded, THE SYSTEM SHALL retain the record
  (append-only history); a later pass does not erase prior fails.
- R5: THE evidence gate SHALL expose criterion coverage: `specd status` and
  `report` SHALL show, per requirement, how many criteria have a current
  passing record.
- R6: IF config enables `criteria.required` (opt-in), THEN the approval
  gate for the completion transition SHALL refuse while any acceptance
  criterion lacks a passing record newer than the last requirements
  approval.
- R7: A criterion record SHALL never substitute for a task verify record —
  the two evidence types stay distinct; no bypass path is introduced.

## Design notes / best practice

- Criterion ids: reuse the EARS gate's parse of `requirements.md` to
  enumerate valid `<r>.<n>` ids — one parser, no second source of truth.
- Storage: same evidence store shape as task verify (`evidence.go`),
  discriminated by record `type: "criterion"`; append-only, atomic write,
  under the per-spec lock. Bump/extend the state schema per spec 02's
  migration discipline if state.json shape changes.
- "Current" record (R5/R6): a pass is current if recorded after the latest
  requirements-phase approval timestamp — re-approving requirements
  invalidates stale attestations by construction, no mutation needed.
- Opt-in gate (R6) mirrors the security gate's opt-in pattern; default off
  so existing flows are unbroken, flip in config when a team wants the
  ratchet.
- Determinism: gate/report read records only; no command execution in the
  criterion path (evidence text is operator-supplied, unlike task verify
  which runs a command). Document that asymmetry in validation-gates.md.

## Out of scope

- Auto-mapping tasks to criteria (v1 didn't have it either; acceptance
  column already references requirement ids informally).
- Any weakening of task-level evidence.

## Acceptance

- Record pass/fail for `1.2` on a demo spec; `status` shows coverage n/m;
  unknown `9.9` exits 2; with `criteria.required` on, completion approval
  refuses until all criteria pass post-approval; full suite green.
