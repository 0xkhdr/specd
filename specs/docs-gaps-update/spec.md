# Docs Gaps Update — Documentation ↔ Codebase Parity (v0.2.0)

## 1. Purpose and requirement coverage

Bring every human- and agent-facing document into agreement with the actual
v0.2.0 command surface and remove all deprecated/stale references. The single
largest defect is a phantom top-level `specd program` command documented across
docs and workspace templates — it is **not** registered; the program sub-verbs
are dispatched only through `specd status --program <schedule|tick|link|unlink>`.
Secondary defects: stale `v0.1.0` version framing and command counts, JSON-config
documentation that contradicts the YAML-only loader from the sibling
`clean-deprecation` spec, missing help metadata for the `status --program`
sub-verbs, and thin coverage of two shipped v0.2.0 features (harness quarantine,
dashboard event API). Scope is **documentation and CLI help metadata only** — no
runtime command behavior changes. Covers DOCS_GAPS_ACTION_PLAN §1.A, §1.B, §1.C
and the user's broader mandate: every `.md` (project `docs/`, root `AGENTS.md`
and peers) plus the `specd init` embedded templates (`internal/core/
embed_templates/**`, including template `AGENTS.md`/`CLAUDE.md` and skill files).

## 2. Verified current state

- **`program` is not a command.** `internal/cmd/registry.go` registers `status`
  (`registry.go:35`) but no `program`. `internal/cmd/program.go` exposes
  `runProgram(args)`, reached **only** from `RunStatus` when `--program` is set;
  it switches on `args.Pos[0]` for `link`/`unlink`/`schedule`/`tick`, else
  renders the frontier. So `specd program …` → "unknown command"; the real
  entrypoint is `specd status --program …`.
- **command-reference.md** still documents the phantom command:
  - `:3` intro claims "specd **v0.1.0** … 16 daily workflow commands and 4
    meta-hidden"; the cheat sheet + daily table now list far more than 16.
  - `:85` a Meta-hidden row for `` `specd program` `` with `link|unlink`,
    `schedule`, `tick` usage — a command that cannot run.
  - `:94` "Merged behavior homes" already correctly states program inspection
    lives under `specd status --program` — the two are contradictory.
  - `:118` a trailing `v0.1.0` reference.
- **Help metadata gap.** `internal/core/commands.go` status entry (`:172`–`:180`)
  documents `--program` only as "Show the cross-spec program frontier"; the
  `schedule`/`tick`/`link`/`unlink` sub-verbs and their flags (`--on`,
  `--interval`, `--command`, `--sandbox`, `--remove`, `--now`) are undocumented,
  even though `programSchedule`/`programTick`/`programMutate` accept them.
- **Workspace templates carry the phantom command:**
  - `embed_templates/skills/specd-maintenance/SKILL.md` — front-matter
    description + body + cron example use `specd program schedule` / `program
    tick` (7 lines).
  - `embed_templates/skills/specd-foundations/SKILL.md:93` and
    `embed_templates/AGENTS.md:118` — index rows say `program schedule`/`program
    tick`.
  - Root `AGENTS.md` has **no** `specd program` reference (clean already).
- **JSON-config docs vs. loader.** Sibling spec `specs/clean-deprecation`
  makes config loading YAML-only. `docs/user-guide.md:140` ("global YAML/JSON →
  project YAML") and `docs/contributor-guide.md:78,82` still describe legacy JSON
  loading as a live path. `specd migrate` (`config_migrate.go`) still *ingests*
  legacy JSON at migration time — that path stays and must not be mis-described
  as removed.
- **Version framing.** `docs/mcp-guide.md:238,323` reference "in v0.1.0".
- **Parity gates already exist.** `internal/cmd/docs_parity_test.go`
  (`TestCommandReferenceMatchesRegistry`) asserts the cheat-sheet command set ==
  registry + `{help,version,mcp}`, and bans known-stale strings. `scripts/
  docs-lint.sh` asserts the `## Cheat sheet` table equals its verbatim mirror
  `.specd/specs/CHEATSHEET.md` (content equality, not row count).

## 3. Proposed design and end-to-end flow

- **G1 — Kill the phantom `specd program` in the reference.** In
  `docs/command-reference.md`: delete the `:85` Meta-hidden `specd program` row.
  Document the program sub-verbs where they actually live — extend the `status`
  daily-table usage and add a short "Program frontier & scheduling" note under
  "Merged behavior homes" enumerating `specd status --program`,
  `… --program schedule <name> --interval <s> --command "<cmd>" [--sandbox <b>]`,
  `… --program schedule` (list), `… --program schedule <name> --remove`,
  `… --program tick [--now <unix>] [--json]`, `… --program <link|unlink> <spec>
  --on <dep>`. Meta-hidden count returns to exactly 4 (`version`, `help`, `mcp`,
  `handshake`).
- **G2 — Refresh version framing & counts.** Rewrite `:3` intro to `v0.2.0`,
  drop "no deprecated commands or aliases" phrasing if inaccurate, and state
  counts derived from the actual tables (recompute; do not hard-guess). Fix `:118`
  and `docs/mcp-guide.md:238,323` `v0.1.0` mentions to the current version or a
  version-neutral phrasing.
- **G3 — Enrich `status --program` help metadata.** In
  `internal/core/commands.go` status entry, expand `LongDescription` to name the
  four sub-verbs and add the program flags (`on`, `interval`, `command`,
  `sandbox`, `remove`, `now`) to the status `Flags`/`Usage`, and add sub-verb
  examples. This regenerates `specd help --all --json`, the reference's stated
  source of truth.
- **G4 — Detemplate the phantom command.** In `embed_templates/skills/
  specd-maintenance/SKILL.md`, `specd-foundations/SKILL.md:93`, and
  `embed_templates/AGENTS.md:118`, rewrite `specd program schedule`/`program
  tick`/`program link|unlink` → `specd status --program schedule|tick|link|
  unlink`, including the front-matter description and cron example.
- **G5 — YAML-only config docs.** Align `docs/user-guide.md:140` and
  `docs/contributor-guide.md:78,82` with the clean-deprecation loader: runtime
  config discovery is YAML-only (embedded defaults → global YAML → project YAML);
  preserve the distinct, still-live `specd migrate` legacy-JSON *ingestion* note
  without implying JSON is a supported *loader* format.
- **G6 — Fill v0.2.0 feature gaps (action-plan §C).** Add to `docs/user-guide.md`
  a concrete harness-quarantine walkthrough (what a quarantined command looks
  like + `specd harness enable <item>` recovery). Expand `docs/dashboard.md`
  with the read-only HTTP endpoints and the `/events` streaming model.
- **G7 — Mirror + broad sweep.** Any cheat-sheet edit is mirrored verbatim into
  `.specd/specs/CHEATSHEET.md`. Grep-sweep all `docs/**/*.md`, root `*.md`, and
  `embed_templates/**` for surviving `specd program`, `v0.1.0`, and JSON-config-as-
  loader phrasing; fix or justify each.

## 4. Interfaces, contracts, data, configuration, dependencies

- **Docs touched:** `docs/command-reference.md`, `docs/mcp-guide.md`,
  `docs/user-guide.md`, `docs/contributor-guide.md`, `docs/dashboard.md`,
  `.specd/specs/CHEATSHEET.md` (mirror).
- **Templates touched:** `internal/core/embed_templates/skills/
  specd-maintenance/SKILL.md`, `…/specd-foundations/SKILL.md`,
  `internal/core/embed_templates/AGENTS.md`.
- **Code touched (metadata only):** `internal/core/commands.go` status entry.
  No `registry.go`, no runtime dispatch, no flag parsing changes.
- **Contract:** the command surface is unchanged; only its *description*
  changes. `TestCommandReferenceMatchesRegistry` must stay green (cheat-sheet
  set unchanged — `program` was never in the cheat sheet, so no set change).
- **Dependency / coordination:** `specs/clean-deprecation` (YAML-only loader).
  G5 must describe the loader as it exists *after* that spec merges. If sequenced
  before it, G5 is gated on decision D2.

## 5. Invariants, security, errors, observability, compatibility, rollback

- No binary behavior, exit codes, or flags change — pure documentation +
  help-string edits. Nothing user-executable is added or removed.
- The reference stays "generated from `specd help --all --json`": G3 changes the
  help source first, so the doc mirrors code, not vice-versa.
- Cheat-sheet ↔ CHEATSHEET.md content equality preserved (docs-lint gate).
- No secrets, network, or security-surface content changes; harness-quarantine
  docs describe existing behavior only.
- **Rollback:** revert the docs commit; no state/schema/config artifact is
  written, so revert is mechanical and side-effect-free.

## 6. Acceptance criteria and validation commands

- `grep -rn "specd program" docs/ internal/core/embed_templates/ *.md` returns
  **zero** hits (all rewritten to `specd status --program`).
- `grep -rn "v0\.1\.0" docs/` returns zero unjustified hits (CHANGELOG history
  excepted).
- `docs/command-reference.md` intro states `v0.2.0` and counts that match the
  rendered tables; no `specd program` row remains; program sub-verbs documented
  under `status --program`.
- `specd help status` (built binary) lists the `schedule`/`tick`/`link`/`unlink`
  sub-verbs and program flags.
- `go test ./internal/cmd/... -run TestCommandReferenceMatchesRegistry` green.
- `make docs-lint` green (cheat sheet == CHEATSHEET.md mirror).
- `make ci` green (build + vet + full suite, no regressions).

## 7. Open decisions and deviations

- **D1 — Program-docs home.** Options: (a) document `status --program`
  sub-verbs inline in the `status` daily-table row + a "Merged behavior homes"
  note; (b) add a dedicated "Program frontier" subsection. **Recommend (a)** —
  keeps the one-command-one-row table invariant and avoids implying `program`
  is its own command. Confirm before adding a new section.
- **D2 — Config-docs sequencing.** If this spec lands **before**
  `clean-deprecation`, G5 would contradict the still-present JSON loader.
  **Recommend** sequencing G5 after clean-deprecation merges, or documenting
  YAML-only as "v0.2.0 target" with a one-line migration note. Confirm ordering.
- **D3 — Version-neutral vs. pinned.** Prefer version-neutral phrasing in prose
  docs (mcp-guide) over pinning `v0.2.0` everywhere, so the next bump needs
  fewer edits; pin only where a concrete release is asserted (reference intro).
