# spec.md — Refactor: Replace `boot` + `enrich` commands with an init-scaffolded skill pack

> Status: PROPOSAL — pending approval
> Author: refactor analysis
> Branch target: a new `refactor/skills-over-boot-enrich` off `refactor/v0.1.0`

---

## 1. Problem statement

`specd`'s stated constitution (`README.md` → *Core Philosophy*) is:

> **The agent reasons. The harness enforces.**
>
> Principle 1 — *The Foundational Split*: the agent does the creative thinking; the
> harness enforces process integrity. The harness is deterministic, zero-LLM,
> zero-inference, and writes nothing outside `.specd/`.

Two shipped commands break that split:

### 1.1 `boot` does the agent's *perception* in Go
`internal/cmd/boot.go` + `internal/core/boot.go` + `internal/core/boot_detectors.go`
run hard-coded detectors (Python / Node / Rust / Go only) to infer the stack,
frameworks, layout, CI, and a `defaultVerify`. This is **inference baked into the
binary**:
- It is brittle — anything outside the four detectors gets "no known tech stack."
- It is the harness *perceiving the repo*, which is exactly the reasoning work the
  Foundational Split assigns to the agent.
- It ships a second freshness gate (`check --boot`, `boot.json` drift detection) to
  police output the harness should never have produced.

### 1.2 `enrich` brokers *markdown authoring* through CLI machinery
`internal/cmd/enrich.go` + `internal/core/enrich.go` + `internal/core/enrich_evidence.go`
wrap "the agent writes steering markdown" in a three-verb protocol
(`plan` / `apply` / `status`), a managed `SPECD ENRICH` block, an `enrich.json`
ledger, and a freshness gate (`check --enrich`). The binary "performs zero
inference" — but it has become a heavyweight broker for an act (writing prose into
`product.md` / `structure.md` / `tech.md`) that is pure agent reasoning. The
machinery adds surface area, tests, docs, and two of the nine validation gates for
no enforcement value the agent couldn't self-provide.

### 1.3 Skills already exist but are dead
`internal/core/embed_templates/skills/specd-enrich/SKILL.md` is embedded in the
binary yet **never written by any command** — `grep` over `internal/**/*.go` shows
zero skill-wiring. The project already gestured at the right answer (teach the agent
via a skill) but never connected it.

---

## 2. Goals

1. **Remove `boot` and `enrich`** entirely — commands, core logic, the two
   repo-global freshness gates, `boot.json` / `enrich.json`, and the
   `--boot` / `--enrich` flags on `check`.
2. **`init` becomes the single scaffolder.** It already writes the steering md
   stubs and roles; extend it to also scaffold a **skill pack** under
   `.specd/skills/`.
3. **Teach the agent via modular skills** so it learns to (a) bootstrap + enrich the
   steering md files itself, and (b) drive each lifecycle stage of specd.
4. **Optimize context via progressive disclosure.** Foundational specd knowledge
   lives in small, separately-loadable skills the agent reads *only when it needs
   them* — never one monolith that overloads the window.
5. **Preserve the integrity core unchanged**: the 7 spec gates, evidence-gated
   `task`/`verify`, the DAG, `state.json` CAS, the per-spec lock. This refactor
   removes *perception/authoring brokers*, not *enforcement*.

## 3. Non-goals

- No change to `task`, `verify`, `next`, `dispatch`, `check <slug>` (the 7 gates),
  `approve`, `status`, `report`, `waves`, `program`, `new`, the lock, or CAS.
- No host-specific skill format. Skills are host-neutral SKILL.md under
  `.specd/skills/`, referenced from `AGENTS.md`. We do **not** write to
  `.claude/skills/` or any single agent's directory (keeps the agent-agnostic
  promise — README §"Agent-Agnostic by Design").
- No new runtime dependencies. Stdlib only, zero LLM calls (unchanged invariant).

---

## 4. Design

### 4.1 The new shape

```
specd init        → scaffolds .specd/ : steering stubs + roles + config + AGENTS.md
                    + .specd/skills/<skill>/SKILL.md  (NEW)
(agent)           → reads AGENTS.md skill index, loads the skill for its current stage,
                    inspects the repo, authors steering + spec md itself
specd new <slug>  → spec stubs
specd check ...   → 7 spec gates (unchanged); NO --boot / --enrich
specd next/dispatch/verify/task/approve → execution + enforcement (unchanged)
```

The harness keeps exactly two responsibilities: **scaffold** (`init`, `new`) and
**enforce** (`check`, `verify`, `task`, the gates). Everything `boot` and `enrich`
did — perceive the stack, author steering prose — moves into agent-read skills.

### 4.2 Skill taxonomy (foundations + per-stage)

Each skill is a directory `.specd/skills/<name>/SKILL.md` with YAML frontmatter
(`name`, `description`) so any host that auto-discovers skills can, and any host
that can't still reads it as plain markdown on instruction from `AGENTS.md`.

| Skill | When the agent loads it | Replaces / covers |
|-------|-------------------------|-------------------|
| `specd-foundations` | Once per session, kept short. The constitution: the 8 principles, the Foundational Split, exit codes (`0/1/2/3`), the `.specd/` file map, and a one-line index of every other skill + its trigger. | The "load context first" preamble; the map that makes the rest lazy-loadable. |
| `specd-steering` | After `init`, before authoring specs — and whenever steering drifts. Teaches the agent to **inspect the repo itself** (manifests, dir tree, README, CI), detect stack/layout, and author `product.md` / `structure.md` / `tech.md` + set `config.defaultVerify`. | **`boot` + `enrich`** (the whole replacement). |
| `specd-requirements` | Entering the requirements phase. EARS syntax, the requirements gate, what `specd check` enforces. | The requirements stage knowledge currently spread across docs. |
| `specd-design` | Entering the design phase. The mandatory `design.md` sections, the design gate, traceability to requirements. | Design stage. |
| `specd-tasks` | Entering the tasks phase. The wave DAG, the seven mandatory task keys, acyclicity, deps-in-earlier-wave, traceability. | Tasks stage. |
| `specd-execute` | Entering executing/verifying. The `next → implement → verify → task --status complete` evidence loop, roles, `dispatch` for parallel subagents, the evidence gate. | Execute/verify stage. |

Rationale for granularity (Goal 4): the agent pays context only for the stage it is
in. `specd-foundations` is the always-loaded ~1-screen index; everything else is
pulled on demand. This is the "separate these main skills to optimize the context
and never overload it" requirement made concrete.

### 4.3 `init` changes

`internal/cmd/init.go` gains a skills pass mirroring the existing `steeringFiles` /
`roleFiles` loops:

```go
var skillFiles = []string{
    "specd-foundations", "specd-steering", "specd-requirements",
    "specd-design", "specd-tasks", "specd-execute",
}
// for each: place(core.SkillsDir(root)+"/"+s+"/SKILL.md", "skills/"+s+"/SKILL.md")
```

- Add `core.SkillsDir(root)` to `internal/core/paths.go` → `.specd/skills`.
- Skill templates live under `internal/core/embed_templates/skills/<name>/SKILL.md`
  and ship via the existing `go:embed embed_templates` (no embed.go change needed).
- Same idempotency contract as steering: skip if present, overwrite with `--force`.

### 4.4 `AGENTS.md` template changes

`internal/core/embed_templates/AGENTS.md`:
- Replace the `specd boot` / `specd enrich ...` lines in *Quickstart* with a
  "bootstrap steering by reading `.specd/skills/specd-steering/SKILL.md`" step.
- Add a **Skills** section: a one-line index (mirrors `specd-foundations`) naming
  each skill and its trigger condition, instructing the agent to read a skill's
  SKILL.md *before* entering that stage and not before (progressive disclosure).

### 4.5 Optional enhancement (scoped in, low cost)

`specd context <slug>` already emits a phase-scoped briefing. Have it name the
relevant skill for the current phase ("load `.specd/skills/specd-tasks/SKILL.md`").
This closes the loop: the harness *points* at the right knowledge without *being*
the knowledge. Kept in a late wave; droppable if it grows risky.

### 4.6 Removal surface (precise)

Delete:
- `internal/cmd/boot.go`, `internal/cmd/boot_test.go`
- `internal/cmd/enrich.go`, `internal/cmd/enrich_test.go`
- `internal/core/boot.go`, `internal/core/boot_detectors.go`, `internal/core/boot_test.go`
- `internal/core/enrich.go`, `internal/core/enrich_evidence.go`, `internal/core/enrich_test.go`
- `internal/core/embed_templates/skills/specd-enrich/` (replaced by the new pack)

Edit:
- `internal/cmd/registry.go` — drop `{"boot", RunBoot}`, `{"enrich", RunEnrich}`.
- `internal/core/commands.go` — drop the `boot` and `enrich` `CommandMeta` entries.
- `internal/cmd/check.go` — drop `--boot` / `--enrich` branches and the
  `runBootCheck` / `runEnrichCheck` helpers.
- `internal/cli/args.go` — drop `"boot"` / `"enrich"` from the bool-flag set.
- Parity tests `registry_test.go`, `commands_test.go`, `lifecycle_test.go` — update
  expectations (these *will* fail until updated; that's the safety net working).

Docs to sweep (boot/enrich references): `AGENTS.md` (root + template), `README.md`,
`CHANGELOG.md`, `docs/{concepts,user-guide,command-reference,validation-gates,agent-integration,contributor-guide}.md`,
`TESTING.md`. Gate count drops from "7 (+2 repo-global)" to "7".

---

## 5. Risks & mitigations

| Risk | Mitigation |
|------|------------|
| Losing deterministic `defaultVerify` auto-set (a real convenience) | `specd-steering` skill explicitly instructs the agent to set `config.defaultVerify` after detecting the test command; `new`/templates keep the sensible default. Convenience moves to the agent; enforcement (`verify` runs it) is unchanged. |
| Users/CI depending on `specd boot` / `check --boot` | This is a `v0.1.0` pre-release refactor (branch `refactor/v0.1.0`). Document as a breaking change in `CHANGELOG.md`; exit code for the now-unknown command is the standard unknown-command usage error (2). |
| Skills drift from CLI reality | Skills are shipped templates compiled into the binary; a doc-parity test (new) asserts every skill name in the `init` list has an embedded template, mirroring `TestRegistryMatchesHelp`. |
| Agent-agnostic hosts that can't auto-discover skills | SKILL.md is plain markdown under `.specd/`; `AGENTS.md` tells any shell-running agent exactly which file to read when. No host API required. |

## 6. Acceptance (spec-level)

- `specd boot` and `specd enrich` are unknown commands (exit 2); no boot/enrich code
  or templates remain (`grep -ri 'RunBoot\|RunEnrich\|boot.json\|enrich.json'`
  returns only CHANGELOG/history).
- `specd init` writes `.specd/skills/<6 skills>/SKILL.md`; idempotent; `--force`
  overwrites.
- `specd check --boot` / `--enrich` return usage errors; `specd check <slug>` runs
  the 7 gates unchanged.
- `make ci` green (lint + race + `-count=2` + coverage floor + stress).
- `AGENTS.md` (template) drives a full lifecycle with no reference to boot/enrich.
- Every shipped skill is grounded in real CLI behavior (no command named in a skill
  that doesn't exist).
