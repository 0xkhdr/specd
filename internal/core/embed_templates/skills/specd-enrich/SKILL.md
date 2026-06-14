---
name: specd-enrich
description: Enrich the specd steering files (product/structure/tech) that `specd boot` left as stubs. Run after `specd boot`. Drives `specd enrich plan` -> author sections from cited evidence -> `specd enrich apply` -> `specd check --enrich`. Works in any host with a shell; portable across agentic tools.
---

# specd enrich

`specd boot` is deterministic and AI-free: it fills `boot.json`, the detected-stack
block of `steering/tech.md`, and `config.defaultVerify`. It deliberately leaves the
inference-heavy steering files (`product.md`, `structure.md`, the conventions of
`tech.md`) as `TODO` stubs. This skill fills those — **you** do the inference; specd
owns the contract and the freshness gate.

## Preconditions
- A `.specd/` exists (`specd init`) and `specd boot` has run (a valid `.specd/boot.json`).
  If `specd enrich plan` errors with "no .specd/boot.json", run `specd boot` first.

## Procedure

1. **Get the brief** (deterministic, no writes):
   ```
   specd enrich plan --json
   ```
   The brief lists, per target: the `file`, current `state` (stub|enriched|stale),
   the `sections` to author, `instructions`, and a token budget. It also lists
   `evidence` — concrete files/dirs to read (README, docs/, manifests, dir tree).

2. **Read the cited evidence.** Open every evidence path before authoring. Ground
   each claim in what is actually in the repo. Do not invent users, scope, or
   conventions. Never duplicate the `SPECD BOOT` block — it owns the detected stack.

3. **Author and apply each target** (markdown on stdin, or `--content-file`):
   ```
   specd enrich apply --target product   < /tmp/product.md
   specd enrich apply --target structure < /tmp/structure.md
   specd enrich apply --target tech      < /tmp/tech.md
   ```
   `apply` merges your markdown into a managed `SPECD ENRICH` block (idempotent —
   re-applying replaces the block, preserves the rest of the file) and records the
   write in `.specd/enrich.json` against the current boot state.

4. **Verify the gate:**
   ```
   specd enrich status        # human; or `specd check --enrich` in CI (exit 1 if stale)
   ```
   Stale/incomplete causes: a target still a stub, boot detection drifted since you
   enriched, or a cited evidence source vanished. Re-run from step 1 when stale.

## Notes
- Every step above is a plain shell command, so this flow works without this skill
  in any host — the skill is just the convenience wrapper. The same steps live in
  `AGENTS.md`.
- Keep each section within its token budget. Prefer precise, repo-grounded prose
  over aspiration.
