# Workflow feedback log

Append-only log of friction hit while *using* specd to build specs. Input for
improving the harness so agents work with specd as native knowledge.

Do not delete entries. Do not rewrite history — resolved entries get
`Status: resolved (<commit/spec>)`, they stay.

Two entry kinds: **friction** (something blocked or misled the agent) and
**improvement** (workflow succeeded, but a concrete change would make it
faster, clearer, or harder to get wrong).

## Friction format

```markdown
### <YYYY-MM-DD> — friction — <short title>
- **Context:** spec slug, phase, role, exact command run
- **Expected:** what the agent believed would happen and why
- **Actual:** exact output / exit code / blocker (shortest decisive line)
- **Root cause:** harness bug | missing guidance | ambiguous docs | agent error
- **Recommendation:** concrete change (verb/flag/gate/doc/message text)
- **Status:** open
```

## Improvement format

```markdown
### <YYYY-MM-DD> — improvement — <short title>
- **Context:** spec slug, phase, role, command sequence that worked
- **Observation:** what cost time or attention despite succeeding
- **Cost:** turns, re-reads, redundant commands, or context tokens burned
- **Recommendation:** concrete change (verb/flag/gate/doc/message text)
- **Tradeoff:** what it costs — invariant risk, added surface, or none
- **Status:** open
```

Improvement entries must clear a bar: an agent-hours or correctness win a
future run can feel. "Nice to have" is not an entry. Anything that would weaken
determinism, evidence integrity, or add a bypass gets logged with the tradeoff
stated plainly and stays a proposal — never a self-applied change.

---

### 2026-07-20 — friction — `specd help link` omits the allowed `--kind` values
- **Context:** authoring spec `agent-driver-protocol`, phase perceive, role craftsman. `specd link agent-driver-protocol agent-protocol-clarity --kind depends-on --reason "..."`
- **Expected:** `depends-on` accepted, or the allowed set visible in `specd help link` before the call. Help printed `--kind  Link kind (default: follows).` with no enumeration.
- **Actual:** `usage: flag --kind="depends-on" not allowed; expected one of [follows regresses maintains supersedes]` (exit 2)
- **Root cause:** missing guidance — the enumeration exists in the validator but not in the palette help text.
- **Recommendation:** render enumerated flag values in `specd help <verb>`, e.g. `--kind  Link kind: follows|regresses|maintains|supersedes (default: follows).` Palette is the single source of truth, so this is one field, and `docs-lint.sh` keeps the reference in sync.
- **Status:** open

### 2026-07-20 — friction — steering templates never load into the machine manifest
- **Context:** fresh `git init` + `specd init` + `specd new demo` in a throwaway tree, specd 1.0.0. `specd context demo T1 --json`
- **Expected:** the five shipped steering templates appear in `items`, since `specd context demo T1` lists all six.
- **Actual:** all five in `omissions` with reason `missing explicit applicability metadata`; `grep -rl specd-context internal/core/embed_templates/steering/` returns nothing.
- **Root cause:** harness bug — no shipped steering template carries the `specd-context` block `SelectSteering` requires, so every driver runs with no project constitution and nothing reports an error.
- **Recommendation:** ship a permissive `specd-context` block in each steering template, and make total steering omission a warning-severity `specd check` finding. Tracked as spec `template-conformance` R1.
- **Status:** open

### 2026-07-20 — friction — requirements scaffold cannot pass `ParseRequirements`, and the error blames tasks.md
- **Context:** same throwaway tree; scaffolded `requirements.md` emits `## Requirement R1 — <name>` and `- **R1.1** When …`
- **Expected:** filling the scaffold as written yields parseable requirement IDs.
- **Actual:** `RequirementIDSet` is empty because the parser wants `## R1 —` and `- R1.1:`; the failure surfaces as `error task-trace: T1 references unknown requirement R1.1` — against the wrong file, for a reference that is correct.
- **Root cause:** harness bug — template/consumer contract with no test asserting it, plus displaced error attribution.
- **Recommendation:** correct the template to the parsed format, and when `RequirementIDSet` is empty say so against `requirements.md` instead of reporting an unknown reference against `tasks.md`. Tracked as spec `template-conformance` R2.1/R3.1.
- **Status:** open

### 2026-07-20 — friction — tasks template teaches `evidence=tests`, which the quality-declaration gate rejects
- **Context:** same throwaway tree; `tasks.md` example comment ships `evidence=tests`.
- **Expected:** the template's own example value is accepted.
- **Actual:** `QUALITY_DECLARATION_INVALID: "tests" must be class/check-id (valid classes: test, output_eval, trajectory_eval, review)`
- **Root cause:** harness bug — same untested template/consumer contract.
- **Recommendation:** change the example to `evidence=test/<check-id>`. Tracked as spec `template-conformance` R2.2.
- **Status:** open
