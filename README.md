# specd — fresh start

> **This repository is mid-rebuild.** `specd` is being reimplemented from scratch on the
> **minimal accurate path**: keep the core thesis, drop the accretion. There is no
> production code at the root yet — only the rebuild inputs and the frozen previous version.

The thesis, unchanged: **the agent reasons, the harness enforces.** `specd` is an
agent-agnostic, spec-driven coding harness — a deterministic, dependency-free Go CLI that
moves process integrity off the LLM's context window and onto local, tool-gated,
evidence-based gates. It is a direct implementation of *The New SDLC with Vibe Coding*'s
equation `Agent = Model + Harness`.

## Repository layout

| Path | What it is |
|---|---|
| **`AGENTS.md`** | Operating brief for the coding agent doing the rebuild. Start here. |
| **`FRESH_START_BRIEF.md`** | Origin brief: analysis of v1 + the mandate for the rebuild. |
| **`fresh-start/`** | The rebuild inputs: 12 domain analyses + roadmap, ADRs, scope triage. |
| **`reference/`** | The **frozen v1 implementation** — read-only, learn from it, don't build it. |
| **`The_New_SDLC_With_Vibe_Coding.pdf`** | The paper; the philosophical anchor. |

## The rebuild pipeline

```
domain analyses  ──►  spec authoring        ──►  implementation
fresh-start/*.md      spec.md + tasks.md         Go source, built in waves
   (done)             (next, on approval)        (after specs are green)
```

- **Stage 1 — domain analysis: done.** See `fresh-start/` (twelve `NN-*.md` domains plus
  `00-roadmap.md`, `00-decisions.md`, `00-scope-triage.md`).
- **Stage 2 — spec authoring: next.** Each domain becomes a `spec.md` + `tasks.md`, in the
  order set by `fresh-start/00-roadmap.md`.
- **Stage 3 — implementation:** built in cross-domain waves once specs are green.

## Looking for the previous (shipped) version?

Everything that was `specd` v1 now lives under [`reference/`](reference/) — source
(`reference/internal/`, `reference/main.go`), docs (`reference/docs/`), the built binary
(`reference/specd`), and its own `README.md`/`AGENTS.md`. It is preserved to learn from,
not to extend.
