# AGENTS.md — Operating brief for the specd fresh-start rebuild

> **You are the coding agent rebuilding `specd` from scratch.** This file tells you what
> this repository is right now, how to learn from the previous implementation, and the
> exact pipeline that turns the domain analyses into a working, optimized binary.
>
> **North star.** Rebuild `specd` to its *final-form* value on the **minimal accurate
> path** — keep the core thesis (`Agent = Model + Harness`), drop the accretion, ground
> every decision in *The New SDLC with Vibe Coding* and context-engineering foundations.

---

## 1. What this repository is now

This repo has been reorganized for a from-scratch rebuild. There is **no production code at
the root** — only the inputs to the rebuild and the frozen previous version to learn from.

```
/
├── AGENTS.md               ← this file: your operating brief
├── FRESH_START_BRIEF.md    ← the origin brief (Part A analysis + Part B mandate)
├── The_New_SDLC_With_Vibe_Coding.pdf  ← the paper; the philosophical anchor
├── fresh-start/            ← INPUT: 12 domain analyses + roadmap/decisions/triage
│   ├── 00-roadmap.md          spec-authoring order + cross-spec dependency DAG
│   ├── 00-decisions.md        cross-cutting ADRs
│   ├── 00-scope-triage.md     every old command → keep/simplify/cut/defer
│   └── 01…12-*.md             one analysis per domain (the spec precursors)
├── reference/              ← FROZEN previous implementation — READ-ONLY, learn only
│   ├── main.go, internal/, docs/, scripts/, Makefile, go.mod, …
│   ├── specd                  the built v1 binary (run it to observe behavior)
│   ├── AGENTS.md, README.md   the old product's own docs
│   └── …
└── (specs and new source get created here as the rebuild proceeds)
```

**`reference/` is a museum, not a foundation.** Do not import from it, do not `go build` it
into the new tree, do not copy files wholesale. Read it to understand *what* the previous
version did and *why*, then rebuild deliberately per the domain analyses.

---

## 2. The pipeline (where we are, where we're going)

```
  domain analyses            spec authoring              implementation
  (fresh-start/*.md)   ──►   (spec.md + tasks.md)  ──►   (Go source + waves)
  ┌───────────────┐          ┌──────────────────┐        ┌──────────────────┐
  │ DONE          │          │ NEXT (on approval)│        │ AFTER specs green │
  │ 12 domains +  │          │ per 00-roadmap    │        │ build in waves    │
  │ roadmap/ADRs  │          │ order (01→10→02…) │        │ A→H               │
  └───────────────┘          └──────────────────┘        └──────────────────┘
        ▲ you are being asked to review this hand-off before authoring begins
```

**Stage 1 — Domain analysis: COMPLETE.** All 12 `fresh-start/*.md` exist, each following
the brief's template (purpose→current-state→verdict→EARS requirements→design→task DAG→risks).
`00-roadmap.md` sequences them; `00-decisions.md` records the ADRs; `00-scope-triage.md`
classifies every old command.

**Stage 2 — Spec authoring: NEXT, and only after human approval.** Translate each domain
file into a `spec.md` (requirements + design) and a `tasks.md` (DAG of waves), **in the
order given by `fresh-start/00-roadmap.md`** (authoring order: 01 → 10 → 02 → 04 → 05 → 03
→ 08 → 06 → 07 → 09 → 11 → 12). Author a spec only after the domains it structurally
depends on are authored (follow the DAG).

**Stage 3 — Implementation: AFTER specs are green.** Build in the cross-domain waves A–H
from the roadmap. Critical path: `01 → 10 → 02 → 05 → 03 → 08 → 09`.

> **Do not skip ahead.** Do not author specs or write Go until the human approves the
> current organization and explicitly moves you to Stage 2.

---

## 3. How to learn from `reference/`

1. **Path remapping.** The domain analyses cite paths like `internal/core/state.go`. Those
   now live under `reference/` — read them at `reference/internal/core/state.go`. Every
   unqualified `internal/…`, `docs/…`, `main.go` reference in `fresh-start/*.md` and
   `FRESH_START_BRIEF.md` means **`reference/<that path>`**.
2. **Verify, don't inherit.** The domain analyses already contain KEEP / SIMPLIFY /
   REDESIGN / CUT / DEFER verdicts with reasons. Trust the analysis, but when a design
   detail is ambiguous, confirm it against the reference code rather than guessing.
3. **Observe real behavior when useful.** `reference/specd` is the built v1 binary. Run it
   (in a throwaway dir) to see actual output/state shapes when a spec needs a concrete
   contract. Never wire it into the new build.
4. **Cite your sources.** When a spec or a task encodes a decision, cite the exact
   reference file and the domain-analysis verdict it came from.

---

## 4. Sources of truth (precedence, highest first)

1. **`fresh-start/00-decisions.md`** — ADRs. Binding cross-cutting decisions.
2. **`fresh-start/01…12-*.md`** — the per-domain requirements/design/task seeds.
3. **`fresh-start/00-roadmap.md`** — authoring order + dependency DAG + build waves.
4. **`FRESH_START_BRIEF.md`** — the mandate and guardrails.
5. **The paper** (`The_New_SDLC_With_Vibe_Coding.pdf`) — the philosophy behind all of it.
6. **`reference/`** — evidence of what v1 did; the lowest authority (it is what we are
   deliberately improving on).

If two sources conflict, the higher one wins; if a domain analysis contradicts an ADR,
the ADR wins — surface the conflict rather than silently resolving it.

---

## 5. Guardrails (non-negotiable — carried from the brief)

- **Determinism first.** No LLM call may sit inside the harness's decision path. Gates,
  DAG computation, reports, and any Brain/controller decisions stay **pure functions of
  on-disk state**.
- **Evidence integrity is absolute.** No task completes without a passing verify record
  (exit code + git HEAD). Read-only roles use the explicit `--unverified --evidence`
  escape hatch only.
- **Preserve the hard invariants** unless a recorded ADR changes them: atomic writes,
  CAS on `revision`, reentrant per-spec advisory lock, `ParseTasksMd` byte round-trip,
  embedded templates via `go:embed`, **zero runtime dependencies** (Go stdlib only, single
  static binary).
- **Subtractive bias.** When unsure whether something is core, default to CUT/DEFER and
  record the reasoning. The target is the *minimal accurate* surface, not feature parity
  with `reference/`.
- **Context discipline.** Practice what specd designs: lean context, targeted reads, cite
  sources. Use read-only sub-agents for scout/preflight sweeps; keep your own context lean.

---

## 6. Definition of done, per stage

**Stage 2 (spec authoring) — a domain is spec-ready when:**
- [ ] Requirements are EARS-shaped (`When <trigger>, the system shall <response>`) and testable.
- [ ] Design names module boundaries, on-disk contracts, and preserved invariants.
- [ ] `tasks.md` is a DAG with `id / role / files / depends-on / verify / acceptance`, grouped into waves.
- [ ] Every claim cites a `reference/` file + a KEEP/SIMPLIFY/REDESIGN/CUT/DEFER verdict.

**Stage 3 (implementation) — a task is done when:**
- [ ] Its `verify` command passes and the record is written (exit code + HEAD).
- [ ] It touches only the `files:` its task declares.
- [ ] The guardrails in §5 still hold (determinism, zero deps, invariants).

---

## 7. Right now

The repository has just been reorganized: v1 frozen into `reference/`, the 12 domain
analyses and roadmap staged in `fresh-start/`, this brief written. **Await human review of
this organization before beginning Stage 2 (spec authoring).** Do not author specs, scaffold
`.specd/`, or write Go until told to proceed.


<!-- headroom:rtk-instructions -->
# RTK (Rust Token Killer) - Token-Optimized Commands

When running shell commands, **always prefix with `rtk`**. This reduces context
usage by 60-90% with zero behavior change. If rtk has no filter for a command,
it passes through unchanged — so it is always safe to use.

## Key Commands
```bash
# Git (59-80% savings)
rtk git status          rtk git diff            rtk git log

# Files & Search (60-75% savings)
rtk ls <path>           rtk read <file>         rtk grep <pattern>
rtk find <pattern>      rtk diff <file>

# Test (90-99% savings) — shows failures only
rtk pytest tests/       rtk cargo test          rtk test <cmd>

# Build & Lint (80-90% savings) — shows errors only
rtk tsc                 rtk lint                rtk cargo build
rtk prettier --check    rtk mypy                rtk ruff check

# Analysis (70-90% savings)
rtk err <cmd>           rtk log <file>          rtk json <file>
rtk summary <cmd>       rtk deps                rtk env

# GitHub (26-87% savings)
rtk gh pr view <n>      rtk gh run list         rtk gh issue list

# Infrastructure (85% savings)
rtk docker ps           rtk kubectl get         rtk docker logs <c>

# Package managers (70-90% savings)
rtk pip list            rtk pnpm install        rtk npm run <script>
```

## Rules
- In command chains, prefix each segment: `rtk git add . && rtk git commit -m "msg"`
- For debugging, use raw command without rtk prefix
- `rtk proxy <cmd>` runs command without filtering but tracks usage
<!-- /headroom:rtk-instructions -->
