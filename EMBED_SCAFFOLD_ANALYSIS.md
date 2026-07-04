# Embed / Scaffold Audit — Cross-Check & Action Plan

**Date:** 2026-07-04
**Reviewer task:** Judge `EMBED_SCAFFOLD_AUDIT.md` against the *original rebuild intent*
(the fresh-start constitution + the authored `specs/`), then produce an action plan for the
new specd version.
**Method:** Every audit claim was re-verified against three levels of authority —
`specs/*/spec.md` (the authored constitution), `fresh-start/00-*.md` (ADRs, roadmap, scope
triage), and the actual new code (`internal/core/{scaffold,roles,agents}.go`,
`embed_templates/`). Reference (`reference/`) was treated as evidence only, **not** as the
target — per AGENTS.md §1, "reference/ is a museum, not a foundation."

---

## Bottom line

**The report is partially valid — and useful — but it grades against the wrong ruler.**

The audit measures the new scaffold against **reference parity**. The rebuild's mandate is
the opposite: *minimal accurate surface, subtractive bias, deliberate cuts* (AGENTS.md §5,
`00-scope-triage.md`). So the audit is right wherever the **new authored spec** also demands
the thing, and over-reaches wherever the missing item was **deliberately cut** in the fresh
tree.

Splitting its findings against the authored `specs/`:

- **~40% are real, spec-mandated gaps** — genuine violations of `specs/06` and `specs/02`
  that must be fixed. The AGENTS.md gap, the `auditor`/`scribe` role error, the dead
  `MergeAgents`, and the `new` artifact drift are all correct and material.
- **~35% are reference nostalgia** — items the new specs deliberately CUT or reconceived
  (`skills/` tree, `config.json`, the 7-key task schema, brain/pinky/reviewer as *roles*).
  Implementing these would *reintroduce* the accretion the rebuild exists to remove.
- **~25% are real but mis-scoped** — legitimate future work that belongs to a *later spec's*
  wave (config.yml → Spec 10, pinky `agents/` prompts → Spec 09, orchestration
  `runtime.gitignore` → opt-in tier), not to the core init scaffold.

The audit's code-level and mechanism-level observations were all **verified accurate**. Its
error is one of *standard*, not of *fact*: it never cross-checks a "gap" against whether the
new constitution still wants it.

---

## Part A — What the audit gets RIGHT (spec-mandated gaps → fix these)

Each row is confirmed against the authored spec, not the reference.

| # | Gap | Authority | Status |
|---|---|---|---|
| A1 | `init` does not emit `AGENTS.md` | **Spec 06 R6.3**: "scaffold `.specd/roles/*`, `.specd/steering/*`, **and an `AGENTS.md`**" | 🔴 Real violation |
| A2 | `MergeAgents` defined but never called on the init path | **Spec 06 R6.4** (marker-merge on init); `agents.go:26` correct but `scaffold.go` never calls it | 🔴 Real violation (dead code) |
| A3 | `auditor` role missing from scaffold | **Spec 06 R6.8** names `scout/validator/auditor` as the read-only roles; §"Minimal surface" lists 🛡️ `auditor` | 🔴 Real violation |
| A4 | `scribe` role invented with no provenance | No ADR, no spec, no reference artifact (`grep` of `00-decisions.md` + all specs = 0 hits). Violates §5 subtractive-bias "record the reasoning" | 🔴 Unrecorded addition |
| A5 | `new` writes `spec.md` placeholder instead of `requirements.md` + `design.md` + `tasks.md` | **Spec 02 line 34**: on-disk surface is `.specd/specs/<slug>/{requirements.md,design.md,tasks.md,state.json,.lock}` | 🔴 Real drift — `design.md` absent blocks the design gate |
| A6 | Role prompt content is a one-liner | `roles.go:10` returns a hardcoded 2-line string; embed `roles/*.md` are 64–71 B stubs. Spec 06 treats a role as a *behavioral contract* the host loads before acting | 🟠 Real, softer (spec doesn't quantify size, but a stub isn't a contract) |

### One extra defect the audit missed

**A7 — Two competing role-text sources.** `roles.go:RolePrompt()` synthesizes role text from
a hardcoded string and **ignores the embedded `roles/*.md` files entirely**, while
`scaffold.go` writes those same embed files to disk. There are two disconnected role-prompt
sources that can drift. This is an architectural smell the audit didn't catch because it only
diffed *content*, not *wiring*. Pick one source of truth (the embed FS) and delete the other.

---

## Part B — Where the audit OVER-REACHES (reference nostalgia → reject)

These are audit "gaps" that the authored specs **deliberately removed**. Implementing them
re-adds cut accretion.

| Audit claim | Why it's wrong per the constitution |
|---|---|
| "Restore `brain`, `pinky`, `reviewer` **roles**" (P1.2) | **Spec 06 defines exactly 4 roles**: scout/craftsman/validator/auditor. `brain` and `pinky` are *orchestration verbs* in an **opt-in tier** (`00-scope-triage.md` Tier 3), not roles. `reviewer` is not in the new surface. This part of the audit's own P1 is self-contradictory with A3. |
| "Restore `skills/` (13 SKILL.md)" 🟠 | **Spec 08** reconceives "progressive disclosure" as **context-manifest item modes** (R8.2: `read-full/read-targeted/summary/defer`), *not* a filesystem SKILL library. The reference skills tree is CUT by redesign, not dropped by accident. |
| "Restore `config.json`" 🟠 | **Spec 10 line 26**, ADR-2: *"Drop legacy `config.json` + `config_migrate.go`."* Explicitly cut. Only `config.yml` survives. |
| "`tasks.md` missing `why` + `contract` (7-key schema)" 🟡 | **Spec 04 subtracts exactly these.** `ParsedTask` = `{ID, Role, Files, DependsOn, Verify, Acceptance, Status}`; "annotations… move to `state.json`, shrinking `tasks.md`." The new skeleton is *correct*; the audit is citing the reference schema. **Reject.** |
| "`new` must scaffold the full six-spec set (`decisions.md`, `memory.md`, `mid-requirements.md`)" 🟡 | Spec 02's minimal surface is 3 doc files + `state.json` + `.lock`. `decisions.md`/`midreq`/`memory` artifacts are created **on demand** by their own commands, not front-loaded at `new`. Only `design.md` (A5) is the real miss. |

---

## Part C — Real but MIS-SCOPED (defer to the owning spec's wave)

Legitimate, but not core-init-scaffold work and not "this week."

| Item | Correct owner | Note |
|---|---|---|
| `config.yml` scaffolding | **Spec 10** (R10.6 config layering) | Config is *loaded if present* (global→project→env). Whether `init` seeds a default `.specd/config.yml` is a Spec 10 decision, not a Spec 06 violation. |
| `agents/` pinky subagent prompts | **Spec 09** (orchestration, opt-in tier) | The `pinky-*` worker prompts belong to the brain/pinky tier. Ship them when that tier ships, gated by `roles.subagent_mode` (named in Spec 06). |
| `runtime.gitignore` | **Spec 09 / opt-in tier** | It protects ACP session/lease files — artifacts that only exist once orchestration is on. No orchestration → nothing to ignore. |

---

## Part D — Corrected root-cause read

The audit's root-cause section is directionally right (a shallow port dropped 4 of 6 embed
dirs) but over-attributes everything to error. Reframed:

1. **A2, A3, A4, A5, A6 are genuine defects** — a shallow port that also silently invented
   `scribe`, dropped `auditor`, and left `MergeAgents` unwired. These bypassed the
   subtractive-bias ADR requirement to *record* cuts. Fix + backfill the reasoning.
2. **The `skills/`, `config.json`, 7-key, and 6-file "gaps" are not defects** — they are the
   rebuild working as designed. The audit lacked the cross-check that would have caught this.
3. **The remaining items are simply not due yet** — they belong to Specs 09/10, whose waves
   haven't run.

---

## Part E — Action plan

Ordered by authority (spec violations first), scoped to the *minimal accurate* fix.

### P1 — Spec-06 / Spec-02 violations (do first)

1. **Wire `AGENTS.md` into `WriteScaffold`** (A1, A2). Add `AGENTS.md` to `embed_templates`,
   embed it, and have `scaffold.go` read any existing root `AGENTS.md`, call
   `MergeAgents(existing, embedded)`, and write the marker-delimited result. Unlocks R6.3 +
   R6.4 in one change. *Verify:* run `init` in a sandbox with a pre-existing `AGENTS.md`
   containing out-of-marker text; assert that text survives and the block is inserted.
2. **Fix the role set to the four the spec names** (A3, A4). Add `auditor`; remove `scribe`
   (or, if `scribe` is genuinely wanted, stop and record an ADR first — do not leave it as a
   silent addition). Final set: `craftsman, scout, validator, auditor`.
3. **Make `new` emit `requirements.md` + `design.md` + `tasks.md`** (A5), matching Spec 02's
   on-disk surface. `design.md` is the blocker — without it the design gate cannot pass.
   Seed each with a minimal EARS / design-section boilerplate (not the full reference stubs).
   *Verify:* `new` then `check` → design gate reachable, not an instant fail.

### P2 — Content depth & wiring hygiene

4. **Collapse the two role-text sources** (A7). Make `RolePrompt` read from the embed FS;
   delete the hardcoded string. One source of truth.
5. **Grow role + steering files from stubs to contracts** (A6). Port the reference
   role-constitution *shape* (mandate / rules / `=== ROLE RESULT ===`) but run it through a
   subtractive pass for the 4-role surface — do **not** bulk-copy the reference. Steering
   files should encode enforceable rules, not sentiment.

### P3 — Record the cuts (close the ADR gap)

6. **Write one ADR** in `fresh-start/00-decisions.md` (or a spec decision log) stating
   explicitly: `skills/` tree → reconceived as context modes (Spec 08); `config.json` →
   cut (ADR-2); reference 7-key task schema → subtracted (Spec 04); pinky `agents/` +
   `runtime.gitignore` → deferred to the orchestration tier (Spec 09). This converts the
   audit's "silent cuts" complaint into recorded decisions and pre-empts the next audit.

### Explicitly NOT doing (rejected from the audit)

- Reintroducing `brain`/`pinky`/`reviewer` as roles, the `skills/` SKILL tree, `config.json`,
  the `why`/`contract` task keys, or the full 6-file `new` set. Each contradicts an authored
  spec. If any is wanted, it needs an ADR that *reverses* the current spec — not a scaffold
  patch.

---

## Verification log (claims cross-checked)

- Spec 06 R6.3/R6.4/R6.8 read directly → confirm AGENTS.md + 4-role (auditor) requirements.
- `grep -rn scribe specs/ fresh-start/00-decisions.md` → 0 hits (no provenance).
- `scaffold.go` / `agents.go` / `roles.go` / `templates.go` read → confirm AGENTS.md not
  emitted, `MergeAgents` uncalled, `RolePrompt` hardcoded & ignores embed files.
- Spec 02 line 34 → confirm `new` must produce requirements/design/tasks, not `spec.md`.
- Spec 04 lines 32/60–62 → confirm `why`/`contract` deliberately removed (audit's 7-key
  finding is invalid).
- Spec 10 line 26 → confirm `config.json` explicitly cut (ADR-2).
- Spec 08 lines 19/37–47 → confirm progressive disclosure = context modes, not `skills/`.
