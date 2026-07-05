# Spec — Documentation Revamp

> **Goal.** Make specd's documentation an accurate, developer-friendly, non-overwhelming
> account of the tool *as it actually behaves today* — and stop it from misguiding
> developers or agents with features that do not exist.

## 1. Context

`specd` is a spec-driven coding harness CLI (Go, zero runtime deps, single static
binary). Its `docs/` set is ~90% excellent and structurally sound, but an audit of the
docs against the source of truth (`internal/`, `main.go`, the built binary) surfaced a
cluster of **factual defects** that misguide readers, plus **stale internal-audit
scaffolding** that overwhelms them.

Ground truth captured during the audit:

- **Command surface = 18 verbs**: `help, init, new, approve, midreq, decision, next,
  status, task, check, verify, context, memory, mcp, handshake, brain, report` + the
  deferred `triage`. Source of truth: `internal/core/commands.go` (`core.Commands`),
  surfaced by `specd help`.
- **Config** is read from **`<root>/project.yml` only** (`ConfigPaths{Project: ...}` in
  `internal/cmd/registry.go`). The global path is never populated; there is no
  `.specd/config.yml` reader.
- **Task completion** requires a passing evidence record (`exit_code == 0` + a
  git-HEAD that resolves to a real commit) — `core.CompleteTask`. There is **no**
  `--unverified`/`--evidence` escape hatch anywhere in the code.
- **Build** is `go build -o specd main.go` (README). There is **no** `Makefile` and
  **no** `scripts/install.sh`.
- **EARS gate** lives at `internal/core/gates/ears.go` only — there is no
  `internal/core/ears.go`.

## 2. Defects to correct (source → reality)

| # | Defect | Appears in | Reality |
|---|--------|-----------|---------|
| D1 | `--unverified --evidence` escape hatch | `docs/user-guide.md`, `docs/validation-gates.md`, `internal/core/embed_templates/AGENTS.md`, `internal/core/embed_templates/steering/reasoning.md` | Flags are never parsed; completion always requires passing evidence. |
| D2 | Config at `.specd/config.yml` + global `$XDG_CONFIG_HOME` | `docs/user-guide.md`, `docs/command-reference.md`, `docs/agent-integration.md` | Only `<root>/project.yml` is read; no global layer active. |
| D3 | `curl .../scripts/install.sh \| bash` installer | `docs/user-guide.md` (×4) | `scripts/install.sh` does not exist. |
| D4 | `make build/install/test/lint` | `docs/contributor-guide.md` | No Makefile; use `go build` / `go test ./...` / `go vet ./...`. |
| D5 | `internal/core/ears.go` path | `docs/validation-gates.md`, `docs/contributor-guide.md` | Path is `internal/core/gates/ears.go`. |
| D6 | Orchestrated mode set via `specd new --agent=` | `docs/agent-integration.md` | `--agent` selects the agent, not the spec mode. |
| D7 | Stale meta-scaffolding: `PROJECT.md`, `specs/`, `progress.md`, P0–P6, ADR-8, "29→16 verbs" | `AGENTS.md` (top brief), `CLAUDE.md`, `docs/charter.md` | None of these artifacts exist in the repo. |

## 3. Requirements (EARS)

- **R1** WHERE documentation describes CLI behavior, THE SYSTEM SHALL describe only
  behavior verifiable against `internal/` and the built binary (code is the source of
  truth).
- **R2** WHEN a reader follows the install or build instructions, THE SYSTEM SHALL
  reference only commands and files that exist (`go build`, `go install`; never a
  nonexistent `install.sh` or `Makefile`).
- **R3** THE SYSTEM SHALL document config as `<root>/project.yml` and SHALL NOT claim a
  `.specd/config.yml` or an active global config layer.
- **R4** THE SYSTEM SHALL NOT reference a `--unverified`/`--evidence` completion escape
  hatch in any doc or shipped template, because none exists.
- **R5** THE SYSTEM SHALL correct every stale file-path and command reference (D5, D6).
- **R6** THE SYSTEM SHALL remove stale internal-audit scaffolding (`docs/charter.md`,
  the `AGENTS.md` top brief, and `CLAUDE.md`'s `PROJECT.md`/`specs/`/`progress.md`
  references) so product docs are not diluted by dead meta.
- **R7** THE SYSTEM SHALL keep the proven doc structure (Concepts, User Guide, Command
  Reference, Validation Gates, Agent Integration, Contributor Guide) and improve it in
  place — this is a correction pass, not a from-scratch rewrite.
- **R8** THE SYSTEM SHALL keep the fixed docs consistent with each other and with a
  single navigation map in `README.md` and `docs/README.md`.
- **R9** WHEN the revamp is complete, THE SYSTEM SHALL still build (`go build ./...`)
  and pass tests (`go test ./...`), since shipped templates are edited.

## 4. Non-goals

- No behavior change to the binary. We reconcile docs *toward* the code; we do not add
  `install.sh`, a `.specd/config.yml` reader, or an `--unverified` flag. (Any such
  desired features are recorded as future code work, not done here.)
- No new documentation domains beyond the existing six + the two READMEs.
- No edits to `reference/` (frozen v1 museum).

## 5. Acceptance

- `grep -rn "install.sh\|make build\|config.yml\|unverified\|core/ears.go\|PROJECT.md"`
  across `docs/`, `README.md`, `AGENTS.md`, `CLAUDE.md`, and `internal/core/embed_templates/`
  returns no misguiding hit.
- `docs/charter.md` is removed; nav maps updated.
- `go build ./...` and `go test ./...` both pass.
- The 18-verb surface, `project.yml` config, and evidence-only completion are stated
  consistently everywhere they appear.
