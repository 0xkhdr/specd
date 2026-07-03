# Tasks — Docs Gaps Update

Verification runs from repo root. Build once for `specd help` checks:
`go build -o /tmp/specd ./cmd/specd` (adjust to the module's main package).

## T1 — Enrich `status --program` help metadata (source of truth first)
- [ ] In `internal/core/commands.go` status entry (`~:172`–`:180`), expand
      `LongDescription` to name the `schedule`/`tick`/`link`/`unlink` sub-verbs
      and their effect.
- [ ] Add program flags to the status `Flags`: `on`, `interval`, `command`,
      `sandbox`, `remove`, `now` (with descriptions). Extend `Usage` to show
      `--program <schedule|tick|link|unlink>` forms; add sub-verb `Examples`.
- **Verify:** `/tmp/specd help status` lists all four sub-verbs and the new flags;
  `/tmp/specd help --all --json | jq '.[]|select(.command=="status")'` shows them.

## T2 — Remove phantom `specd program` from command-reference
- [ ] Delete the Meta-hidden `` `specd program` `` row (`docs/command-reference.md:85`).
- [ ] Document program sub-verbs under `status` (daily table usage) and a
      "Program frontier & scheduling" note in "Merged behavior homes" (decision D1).
- [ ] Confirm Meta-hidden table now holds exactly 4 rows: `version`, `help`,
      `mcp`, `handshake`.
- **Verify:** `grep -n "specd program" docs/command-reference.md` → 0 hits.

## T3 — Refresh version framing & counts
- [ ] Rewrite intro (`:3`) to `v0.2.0`; recompute and state daily/meta counts
      from the actual tables (do not hard-guess).
- [ ] Fix trailing `v0.1.0` at `:118`.
- [ ] Fix `docs/mcp-guide.md:238,323` `v0.1.0` mentions (version-neutral per D3).
- **Verify:** `grep -rn "v0\.1\.0" docs/` → 0 unjustified hits.

## T4 — Detemplate the phantom command (workspace templates)
- [ ] `embed_templates/skills/specd-maintenance/SKILL.md`: rewrite front-matter
      description, body commands, and cron example from `specd program …` →
      `specd status --program …` (7 lines).
- [ ] `embed_templates/skills/specd-foundations/SKILL.md:93` and
      `embed_templates/AGENTS.md:118`: rewrite the index rows likewise.
- **Verify:** `grep -rn "specd program\|program schedule\|program tick" internal/core/embed_templates/` → 0 hits.

## T5 — YAML-only config docs (coordinate with clean-deprecation, D2)
- [ ] `docs/user-guide.md:140`: config discovery = embedded defaults → global
      YAML → project YAML (drop JSON as a loader format).
- [ ] `docs/contributor-guide.md:78,82`: keep `specd migrate` legacy-JSON
      *ingestion* accurate; stop describing JSON as a live *loader* path.
- **Verify:** wording matches the post-clean-deprecation loader; no doc claims a
  runtime JSON config loader.

## T6 — Fill v0.2.0 feature gaps (action-plan §C)
- [ ] `docs/user-guide.md`: add a concrete harness-quarantine walkthrough +
      `specd harness enable <item>` recovery example.
- [ ] `docs/dashboard.md`: document the read-only HTTP endpoints and the
      `/events` streaming model.
- **Verify:** examples match actual `specd harness`/`specd dashboard` behavior
  (cross-check against the code, not memory).

## T7 — Mirror the cheat sheet + broad sweep
- [ ] Mirror any `## Cheat sheet` edits verbatim into `.specd/specs/CHEATSHEET.md`.
- [ ] Final grep sweep across `docs/**/*.md`, root `*.md`, `embed_templates/**`
      for surviving `specd program`, `v0.1.0`, JSON-config-as-loader phrasing.
- **Verify:** `make docs-lint` green (cheat ↔ mirror content equality).

## T8 — Full gate
- [ ] `go test ./internal/cmd/... -run TestCommandReferenceMatchesRegistry`
- [ ] `make docs-lint`
- [ ] `make ci` (build + vet + full suite)
- **Done when:** all three green and every T1–T7 verify command passes.
