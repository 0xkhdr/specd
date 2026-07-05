# CLAUDE.md — operating brief for specd

`specd` is a spec-driven coding harness CLI: Go, standard library only, zero runtime
dependencies, single static binary. It shifts process enforcement off the LLM's
non-deterministic context and onto a strict, local, tool-gated pipeline —
**the agent reasons, the harness enforces.**

Start here:

- [README.md](README.md) — product overview and documentation map.
- [AGENTS.md](AGENTS.md) — brief for agents working on specd's own codebase, plus the
  runtime integration block that `specd init` writes into user projects.
- [docs/](docs/) — concepts, user guide, command reference, validation gates, agent
  integration, contributor guide. These are the authoritative developer docs.

Orientation:

- `internal/` + `main.go` — the Go binary. One handler per command under
  `internal/cmd/`; pure domain logic under `internal/core/`.
- `specs/` — planning artifacts for in-flight work (e.g. `specs/docs-revamp/`).
- `reference/` — frozen v1 implementation. Read-only museum: never import, build, or
  copy from it.

Non-negotiable guardrails (detail in `docs/contributor-guide.md` §3):

- **Determinism first** — no LLM in any gate, DAG, or report path; all are pure
  functions of on-disk `.specd/` state.
- **Evidence integrity** — no task completes without a passing verify record (exit 0
  pinned to a resolvable git HEAD). There is no bypass flag.
- **Structural invariants** — atomic writes, CAS on the `state.json` revision, reentrant
  per-spec lock, byte-stable tasks parser, `go:embed` templates, zero runtime deps.
- **Subtractive bias** — when unsure, cut or defer, and record the decision.
