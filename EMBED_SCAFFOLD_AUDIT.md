# Embed / Scaffold Audit — New specd vs. Reference and Constitution

**Date:** 2026-07-04
**Scope:** `internal/core/embed_templates/`, `internal/core/scaffold.go`, `internal/core/roles.go`, `specd init`, `specd new`, plus adjacent artifacts (`CLAUDE.md`, `config.yml`, `specStubs/`, `skills/`)
**Sources consulted**
- Rebuild intent: `fresh-start/00-decisions.md` (ADRs), `fresh-start/00-roadmap.md`, `fresh-start/06-agent-agnostic-integration.md`
- Constitution: `specs/06-agent-agnostic-integration/spec.md`, `specs/01-product-philosophy-core/spec.md`
- Reference (frozen v1): `reference/internal/core/embed_templates/` (full tree, all files read)
- New implementation: `internal/core/embed_templates/`, `internal/core/{scaffold.go,roles.go,agents.go}`, plus sandbox `init`/`new` runs

---

## TL;DR Verdict

**No — the scaffold is not implemented to spec and contains material gaps relative to both the reference and the new constitution.**

The scaffold reaches for the right *mechanism* (`go:embed`, `WriteScaffold`, marker-merge `agents.go`), but what it embeds and emits is a thin subset of what the reference shipped and, more importantly, a direct violation of requirements the new constitution already encoded. The roles are 1-line sentiment stubs where the reference carried 1.6–2.2 KB role constitutions; the `AGENTS.md`, `config.yml`, `config.json`, `runtime.gitignore`, `specStubs/`, and `skills/` trees are entirely absent from the scaffold; `specd new` dumps a placeholder spec.md instead of the six implemented specStubs; and one named non-negotiable role (`auditor`) is missing from the emit loop entirely despite being referenced by verbatim spec requirements.

The rest of this document itemizes every gap, its provenance, and the delta between what the new code does and what both the reference and the fresh-start constitution require.

---

## 1. What the Reference Scaffolded (the "Done" Baseline)

`reference/internal/core/embed_templates/` carried **6 top-level directories plus 6 top-level files**:

| Directory | Content |
|---|---|
| `roles/` | 7 files — `auditor`, `brain`, `craftsman`, `pinky`, `reviewer`, `scout`, `validator` (each 880B–2.2 KB, full role-constitution format) |
| `steering/` | 6 files — `memory`, `product`, `reasoning`, `structure`, `tech`, `workflow` (each ~270B–3.4 KB constitution text) |
| `agents/` | 4 files — `pinky-auditor`, `pinky-craftsman`, `pinky-scout`, `pinky-validator` (managed Pinky subagent prompts) |
| `skills/` | 13 subdirectories — one `SKILL.md` each for every phase (`specd-brain`, `specd-design`, `specd-pinky`, etc.) |
| `specStubs/` | 6 files — `requirements.md`, `design.md`, `tasks.md`, `decisions.md`, `memory.md`, `mid-requirements.md` |
| *(root files)* | `AGENTS.md` (11.6 KB project host guide), `CLAUDE.md` (278 B, `@AGENTS.md` import shim), `config.json`, `config.yml`, `runtime.gitignore` |

`init` materialized **all of that** into the project root. `new` created `.specd/specs/<slug>/requirements.md` (seeded from specStubs — no placeholder), then `design.md`, then `tasks.md`, plus `decisions.md`, `memory.md`, and `mid-requirements.md`.

Each of the 7 role files was a full constitution — sections like *Capability, Mandate, Boundaries/Rules, Voice, Trust labels*, and a structured `=== ROLE RESULT ===` block. These are exactly what the reference `AGENTS.md` told hosts to load before doing any work. They are not optional decoration.

`skills/` was a progressive-disclosure library: the host reads a `SKILL.md` only when entering the corresponding phase. `agents/` was tagged-by-host subagent prompts dropped under `.claude/agents/` during orchestration. `config.{json,yml}` were machine-readable defaults; `runtime.gitignore` protected per-machine ACP session/lease files.

---

## 2. What the New Scaffold Embeds and Emits

### 2.1 `internal/core/embed_templates/` (current)

```
embed_templates/
  roles/        12 files: craftsman, scout, scribe, validator,
                     memory, product, reasoning, structure, tech, workflow
                    (+ 2 duplicates: workflow.md appears twice with 64B each)
  steering/      6 files: memory, product, reasoning, structure, tech, workflow
  templates.go   93 B  (embed.FS mount)
```

`templates.go` currently emits:
```go
//go:embed roles/*.md steering/*.md
var FS embed.FS
```
So **only `roles/*.md` and `steering/*.md`** are shipped. Nothing else is embedded.

### 2.2 `internal/core/scaffold.go` (current)

`WriteScaffold(root)` does two things:
1. `MkdirAll(root/.specd/roles)` + `root/.specd/steering`
2. For each entry in `embedtemplates.FS` under `roles/` and `steering/`, write the file to `.specd/roles/<name>` or `.specd/steering/<name>` if it does not already exist.

It does **not** emit:
- `AGENTS.md` at the project root
- `config.yml` (or `config.json`) at the project root
- `runtime.gitignore` at the project root
- `CLAUDE.md`
- `skills/` directory tree
- `agents/` directory tree
- `specStubs/` directory tree
- Any `specs/*.md` scaffolding during `new`

### 2.3 `specd init` (verified in sandbox)

Running `./specd init` in a fresh directory produces **exactly** `.specd/roles/` and `.specd/steering/` subdirectory trees, with the 1-line stub files. No root `AGENTS.md`, no config, no skills, nothing else.

### 2.4 `specd new` (verified in sandbox)

`specd new testspec --title "Test"` produced only `.specd/specs/testspec/spec.md` (placeholder: *"Replace with requirements and design"*), `tasks.md` (single skeleton task with seven keys), `memory.md` (steering-memory header), and `state.json`.

Reference `new` created six spec files: `requirements.md`, `design.md`, `tasks.md`, `decisions.md`, `memory.md`, `mid-requirements.md`. The design.md is the most central missing piece — without it the design gate cannot be passed and the workflow Spec 01 describes cannot be followed.

---

## 3. Constitution Violation by Domain Area

### 3.1 Spec 06 — Agent-Agnostic Integration (critical)

Two mandatory requirements are violated by the scaffold as-is:

> **R6.3** When `init` runs, the system shall scaffold `.specd/roles/*`, `.specd/steering/*`, and an `AGENTS.md` without overwriting user-authored regions outside the managed markers.
> **R6.4** When `init` merges `AGENTS.md` into an existing file, the system shall replace only the marker-delimited section and preserve all other content.

R6.3 is unmet: `AGENTS.md` is not written. R6.4 is unmet as an *init-path* requirement: `MergeAgents` (in `agents.go`) implements the marker-merge contract correctly, but `scaffold.go` never calls it. The shelf-ready code exists; it is simply wired out of the bootstrap.

The reference AGENTS.md carried the full 16-verb charter, the host-router contract for every verb (`specd context`, `specd check`, `specd verify`, `specd task ... --status complete`, etc.), and the ADR-8 guarantees in user-facing prose. Spec 01's acceptance test — *"charter gate below"* — implicitly depends on this file being present so that every host sees the constitution.

### 3.2 Spec 06 — Role coverage / read-only role contract (critical)

> **R6.8** The system shall not bind a read-only role (`scout`, `validator`, `auditor`) to a write task.

This requirement names three read-only slugs. The new scaffold emits four roles: `craftsman`, `scout`, `validator`, `scribe`. **`auditor` is missing.** `scribe` is a write role per its own text: *"You update specs, tasks, and docs without production behavior changes."* So the new scaffold (a) introduced a role with no reference provenance and (b) dropped a role explicitly named in a requirement.

R6.1 and R6.2 pass mechanically (dedup logic in `roles.go` exists and is correct) but are hollow — the emission content is a one-liner where the reference required a full mandate/rules/result-block constitution.

### 3.3 Spec 01 — P8 (Steering as Constitution) and R1.6 (determinism)

The steering files rendered to disk are:

```markdown
# memory.md   →  "Record durable project facts here after verified changes."
# product.md  →  "specd keeps Agent = Model + Harness, with deterministic harness decisions."
# reasoning.md→  "Prefer small, cited context over broad session history."
# structure.md→  "Core logic lives under internal/core. CLI wiring lives under internal/cmd."
# tech.md     →  "Use Go standard library only. Keep artifacts deterministic and atomic."
# workflow.md →  "Work from the current task frontier. Touch declared files only."
```

These read like lint rules, not steering constitutions. The paper (p.30) is explicit that agent failures are **configuration** failures, and that the Instructions / Rule Files are how the harness makes the plan safely delegable. The reference's `reasoning.md` encoded the six-phase reasoning architecture, the evidence gate, the phase→output mapping, and voice rules — that is *usable* steering, not sentiment.

The determinism requirement (R1.6, ADR-8) is not violated by the content of these stubs, but the risk is increased: by making the steering files too terse to violate determinism, the scaffold also makes them too terse to *enforce* determinism on hosts that need explicit rules.

### 3.4 `specd new` — spec skeleton completeness (medium-high)

The reference's `specStubs/tasks.md` scaffold already embedded the seven mandatory keys as a documentation block:

```
Each task is a checkbox item followed by a metadata block:
## Wave 1
- [ ] T1 — short imperative title
  - why: ...
  - role: ...
  - files: ...
  - contract: ...
  - acceptance: ...
  - verify: ...
  - depends: ...
  - requirements: ...
```

The new `tasks.md` skeleton has only six keys and lacks `why` and `contract`. Spec 04 defines the seven-key schema as a requirement; `new` contradicts its own design spec at scaffold time. Worse, when `specd check` runs the `task-schema` gate (ADR-4, Registry), the seeded skeleton task fails that gate by default — the user's first action after `new` is a failing gate.

The new `spec.md` placeholder ("Replace with requirements and design") also creates a UX gap for a fresh user: the reference seeded `requirements.md` with an EARS boilerplate and `design.md` with a design-section boilerplate. New users get no scaffolding hint at all.

---

## 4. Detailed File-Level Findings

### 4.1 `internal/core/embed_templates/templates.go`

Current:
```go
//go:embed roles/*.md steering/*.md
var FS embed.FS
```

Required (to match reference + constitution at minimum):
```go
//go:embed roles/*.md steering/*.md AGENTS.md config.yml runtime.gitignore
//go:embed agents/*.md skills/*/SKILL.md specStubs/*.md
var FS embed.FS
```
(Each subdir needs its own `//go:embed` line; Go embeds don't recurse.)

### 4.2 `internal/core/embed_templates/roles/` — content deficiency

Reference `roles/craftsman.md` (heads-up):
```markdown
# Role: Craftsman (write)
**Capability:** implement exactly ONE atomic task. **You may write code.**
## Mandate
- Implement the task's `contract` and nothing else. No scope creep.
- Touch only the files named in `files:` (plus their tests). Respect existing patterns.
- Make the task's `acceptance` criteria true.
- Run (or hand to the validator) the task's `verify:` line. Capture the result as evidence.
- Summary ≤1500 tokens. Voice: "what I changed AND why."
## Rules
- ONE task per invocation. Do not start the next task.
- A craftsman's "done" is not evidence — the verify result is. Never claim complete without it.
- If blocked, stop after ONE retry and report `blocked` with the exact blocker.
- Record any deviation from the spec via `specd decision` before finishing.
=== ROLE RESULT ===
role: craftsman
task: <Tn>
status: complete | blocked | failed
files: [<paths you changed>]
findings: [<what changed + why>, ...]
verify: { command: <cmd>, result: passed|failed|blocked }
confidence: high|medium|low
notes: <deviations | exact failure | N/A>
===================
```

New `roles/craftsman.md`:
```markdown
You implement scoped tasks against the spec and verify command.
```

That is a summary card, not a role contract the host is told to read before acting.

Same complaint applies to `scout.md`, `validator.md`. `auditor.md` is entirely absent. `brain.md` and `pinky.md` are also absent. **`scrib`e.md` is invented** — it has no reference-artifact provenance and no ADR backing.

### 4.3 `internal/core/scaffold.go` — AGENTS.md gap

`scaffold.go` iterates `roles/` and `steering/` only. It must also:
- Call `MergeAgents` (already exists in `agents.go`) when emitting `AGENTS.md` so existing user-authored content outside markers is preserved (rereference of `AGENTS.md` is idempotent).
- Emit `config.yml` and `runtime.gitignore` via a simple `WriteFile` (both are already identity files; no merge needed).

The `agents.go` merge helper is correct against the reference's marker format (`agentsBegin` / `agentsEnd` HTML comments). It is currently dead code.

### 4.4 Verify in `embed.FS`: `scripts/specd-workflow.{sh,py}` and `Makefile` from reference?

Not part of `internal/core/embed_templates/`, but noted for completeness: the reference also shipped `scripts/specd-workflow.sh` and `scripts/specd-workflow.py` plus a `Makefile`. None of these are in any `embed_templates/` tree (neither reference nor new) — these are not embedded; they live at the reference root. The new tree does not carry them forward; whether to keep, cut, or rewrite them is a separate review question, but they are not "missing from the scaffold".

---

## 5. Scoring

| Area | Reference | New | Gap |
|---|---|---|---|
| `roles/` — coverage of named roles | 7 full constitutions | 4 stubs + `auditor` absent + invented `scribe` | 🔴 |
| `roles/` — content depth | 880B–2.2 KB · mandate/rules/result-block | 56–71 B · one-liner | 🔴 |
| `steering/` — content depth | 270B–3.4 KB constitutions | 56–75 B stubs | 🔴 |
| `AGENTS.md` embed | 11.6 KB | Absent | 🔴 |
| `AGENTS.md` init-path emission | Yes (root of project) | No | 🔴 |
| `agents.go` usage | MergeAgents called | MergeAgents defined but uncalled | 🟠 |
| `config.yml` / `config.json` | Both present | Absent | 🟠 |
| `runtime.gitignore` | Present | Absent | 🟠 |
| `skills/` tree | 13 SKILL.md files | Absent | 🟠 |
| Agents (Pinky prompts) | 4 templates | Absent | 🟡 |
| `specStubs/` | 6 seeded spec files | Absent | 🟡 |
| `specd new` artifact set | 6 files per spec | 1 placeholder + 1 stub tasks.md | 🟡 |
| `tasks.md` scaffold keys | All 7 keys + schema comment | 6 keys, no schema | 🟡 |
| `go:embed` mechanism | Single embed, all dirs | Single embed, subdirs only | ✅ partial |
| Idempotent init | Yes | Yes (roles/steering only) | ✅ partial |
| Zero-dep emit | Yes | Yes | ✅ |
| Marker-merge API | Correct | Correct (but unused) | ✅ API ✅ |

---

## 6. Root Causes

1. **Shallow port during fresh-start scaffold reorganization.** The reference's `embed_templates/` had six top-level items. When the new tree was carved out, only `roles/` and `steering/` were mirrored into the new `internal/core/embed_templates/`. The other four directories (`agents/`, `skills/`, `specStubs/`, plus the root files) were silently dropped during the initial `mkdir` pass. No ADR records this cut.

2. **Role content truncated without justification.** The role files were likely viewed as "long examples" by whoever first set up the embed and reduced to one-liners. This is a classic prune-by-feel move that bypassed the framework's own ADR requirement: *"Subtractive bias…when unsure whether something is core, default to CUT/DEFER and record the reasoning."* The cuts here were not recorded.

3. **`auditor` and `scribe` appear to be mutual errors.** `auditor` was required by R6.8 but dropped; `scribe` was invented without provenance. Most likely an off-by-one when porting or a read-through of `auditor` (read-only audit function) misread as a writing role and misnamed.

4. **`agents.go` was written as scaffolding but not wired.** `MergeAgents` was authored in the correct marker format against the reference — someone anticipated needing the merge helper but never called it from `scaffold.go`. The result is a spec-level requirement (R6.3, R6.4) met only at the API layer, not at the CLI layer.

5. **`specd new` was minimized without aligning Spec 06.** The spec still requires `new` to scaffold the full six-spec set. The implementation does not. This kind of spec-implementation drift is what ADR-4's pluggable gate registry is designed to catch (through the `task-schema` gate), so the gap is catchable — it just hasn't been caught yet.

---

## 7. Priority Remediation (in recommended order)

**P1 — Constitution coverage (address this week)**

1. **Restore `AGENTS.md` emit and marker-merge.** `scaffold.go` must call `MergeAgents` on `embed_templates/AGENTS.md` before writing `roles/` and `steering/`. This single change unlocks R6.3/R6.4 and restores the user-facing 16-verb charter at bootstrap.

2. **Flesh out all role files to constitutions.** Minimum: full mandate/rules/result-block for at least `craftsman`, `scout`, `validator`, `auditor`, `brain`, `pinky`, `reviewer`. The reference text can be imported as a starting point with a quick compliance pass for the new subtractive surface.

3. **Retire or ADR-record `auditor` and `scribe`.** Either retire `scribe` (no ADR-provenance, not in the reference) and restore `auditor` (required by R6.8), or record an explicit ADR that renames or replaces both. Do not leave `scribe` as a silent addition.

**P2 — `specd new` self-consistency**

4. **Restore `specStubs/`** into the embed and use them inside `new` to populate the six spec files. The alternative is to explicitly scope down `specd new` to a "placeholder only"