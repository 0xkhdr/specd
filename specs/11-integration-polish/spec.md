# 11-integration-polish — mcp --config, init repair/refresh/dry-run, handshake digests

Wave 2. FINDINGS refs: B.21, B.22, B.26, D-tier2 item 13 (integration half).

## Problem

Three adoption/maintenance gaps versus v1:

1. **`mcp --config <host>`** (B.22): v1 emitted ready-to-paste MCP config
   snippets per host, plus `--root`/`--spec` pinning. Highest-friction
   adoption step today is hand-writing that JSON. Verdict: **port** — small.
2. **`init` depth** (B.21): v1 had `--repair/--refresh/--force`
   (marker-based managed-asset maintenance) and `--dry-run`. Users *will*
   hand-edit `AGENTS.md`; idempotent re-init with drift repair is the
   difference between a scaffold and a managed asset. Verdict: **port
   `--repair`, `--refresh`, `--dry-run`** (multi-host detection adapted to
   actually-targeted hosts only).
3. **Handshake digests** (B.26): v1 pinned a command-schema digest and
   detected config drift (`--expect-config-digest`). Digest-pinning lets an
   agent detect its cached palette is stale — cheap, on-thesis. Verdict:
   **adapt**.

## Requirements (EARS)

- R1: WHEN a user runs `specd mcp --config <host>` (hosts: at minimum
  `claude-code`; design for additions), THE SYSTEM SHALL print a ready-to-
  paste config snippet for that host wiring `specd mcp`, honoring optional
  `--root <path>` and `--spec <slug>` pinning flags; unknown host exits 2
  listing known hosts.
- R2: THE scaffolder SHALL wrap every managed region it writes (AGENTS.md
  sections, role/steering files) in stable marker comments identifying
  specd-managed content and the template version.
- R3: WHEN a user runs `specd init --repair`, THE SYSTEM SHALL restore
  managed regions that drifted from their template, leaving all content
  outside markers untouched, and report each repaired file.
- R4: WHEN a user runs `specd init --refresh`, THE SYSTEM SHALL update
  managed regions to the current binary's template version (same
  outside-markers guarantee).
- R5: WHEN `--dry-run` accompanies `init`/`--repair`/`--refresh`, THE
  SYSTEM SHALL print what would change (unified-diff style per file) and
  write nothing.
- R6: THE handshake output SHALL include a digest (SHA-256) of the
  machine-readable command palette (spec 03's `help --json` payload) and a
  digest of the effective config; WHEN invoked with
  `--expect-palette-digest <d>` or `--expect-config-digest <d>` and the
  digest differs, THE SYSTEM SHALL exit 1 identifying which digest
  drifted.

## Design notes / best practice

- Markers: `<!-- specd:managed:<asset>:v<N> begin/end -->`; repair =
  regenerate region from embedded template; refresh = same with version
  bump. Idempotence tests: running twice = running once, byte-identical.
- Hand-edited-inside-markers content is *overwritten* by repair — that is
  the contract; `--dry-run` exists so users see it first. Say it loudly in
  docs.
- Digest canonicalization (R6): digest bytes of the canonical JSON
  (stable key order, no whitespace variance) — the `help --json`
  determinism test from spec 03 is the prerequisite; depends on spec 03.
- Config snippets: embedded templates, not string concatenation in code;
  covered by golden tests.
- `--force` semantics from v1 folded into `--repair` + explicit prompt-free
  operation (harness is non-interactive by design).

## Out of scope

- Multi-host detection heuristics (codex/cursor/etc.) until those hosts are
  actually targeted — record as ADR.
- Packs/registry (Wave 3 decision records).

## Acceptance

- `mcp --config claude-code --spec demo` prints valid paste-ready JSON;
  hand-mangled AGENTS.md managed region: `--dry-run` shows diff,
  `--repair` restores it, user content outside markers byte-identical;
  handshake digest changes when a verb is added and `--expect-*-digest`
  catches it. Full suite green.
