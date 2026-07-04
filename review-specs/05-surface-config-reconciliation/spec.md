# Wave 5 — Surface & Config Reconciliation

> **Order:** 6 / 7 · **Depends:** W4 (memory-verb verdict needs P4.3 functional) + W2
> **Findings:** F7 (18 verbs vs 16), F10 (config name/fail-silent/dead flag), F14 (CLI consistency)
> **Sources:** PROJECT.md §8 Wave P5, BUILD_REVIEW.md §5 Wave 5, specs/01 + 10, ADR-2/5/10
> **Files:** `internal/cmd/registry.go`, `internal/core/config.go`, `internal/context/manifest.go`, `docs/charter.md`

Silent drift is the failure mode ADR-10 warns against. The shipped surface is 18 verbs
against a spec'd 16, config loads the wrong filename and swallows errors against
ADR-2's fail-loud mandate, and small CLI inconsistencies break the deterministic
contract consumers depend on. Every divergence is resolved *by recorded decision*,
never left as drift.

## 1. Purpose & principles

- **Principles owned:** P1 (charter discipline), P5 (integration floor), P7 (consistent contracts).
- **Harness components:** instructions (charter/ADR), tools (surface), context (config).

## 2. Requirements (EARS)

- **R5.1** The system shall resolve the 16-vs-18 verb conflict by recorded ADR:
  cut the `triage` verb (registered stub; ADR-5 says no flywheel commands), and fold
  `memory` into scope via a superseding ADR — permitted only because W4 R4.3 made its
  output reach agents. After the ADR, bare `specd` verb count shall equal the count in
  Spec 01 R1.5 and `docs/charter.md`, and a registry↔charter test shall enforce it.
- **R5.2** When any config is loaded, the system shall read `config.yml` (ADR-2 — the
  `project.yml` path removed), seeded by `init`, layered global→project→env; a parse
  error shall be a hard exit with the error printed, never silent defaults. `init
  --agent` shall be either wired (project-scoped adapter install per spec 06) or
  removed from the flag set — an accepted-and-ignored flag shall not remain.
- **R5.3** The system shall make the CLI contract uniform: `task <slug> <id>` (no
  all-spec scan); context-manifest item paths always repo-relative including the
  `.specd/` prefix; `check` prints a one-line green summary (`N gates: all green`) on
  success while remaining silent-parseable via `--json`.

## 3. Design

- **ADR mechanics (R5.1):** the superseding ADR is a `specd decision` record (dogfood,
  W3 gave it content) plus an entry in `docs/charter.md`'s decision log; Spec 01 R1.5
  and the charter verb table updated in the same change. `TestRegistryMatchesCharter`
  joins `TestRegistryMatchesHelp` as CI-blocking.
- **Config (R5.2):** one rename + fail-loud at every `LoadConfig` callsite (`config, _ :=`
  pattern eliminated); `init` seeds a commented `config.yml` from the embedded template.
  Recommended `--agent` verdict: wire it to the ≤1 reference adapter if it exists,
  else remove — decide in the ADR, don't leave both.
- **Path normalization (R5.3):** normalize at manifest build (one place), covered by a
  golden test so the contract can't regress; consumers get one path shape.

## 4. Invariants preserved

- Subtractive bias: verdict defaults to CUT; additions only by superseding ADR.
- ADR-2: fail-loud config, zero-dep YAML subset loader untouched.
- One `[]Command` table drives dispatch/help/MCP — the reconciled count propagates
  everywhere from a single edit.
