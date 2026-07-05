# 03-command-metadata — Rich command metadata with fail-closed phase dispatch

Wave 1. FINDINGS refs: B.2, B.3, C.8, D-tier1 item 5.

## Problem

The current `Command` struct is name/usage/description/flags only. Three
losses versus v1:

1. **Phase/mode compatibility metadata** (B.2): v1 commands declared which
   spec statuses/phases they were valid in, and dispatch failed closed on
   out-of-phase calls (e.g. an agent running execution verbs during
   planning). That was *harness enforcement* — exactly the project thesis —
   and it is gone.
2. **Exit-code metadata + enum-annotated flags** (B.3): v1's `help --json`
   was a machine-readable contract (exit meanings, flag enums, defaults,
   examples) consumable by MCP and role prompts.
3. **Divergence risk** (C.8): help text, MCP tool descriptions, and role
   prompts each independently restate command semantics today; v1 generated
   all three from one `CommandMeta`. Drift grows with every verb added.

## Requirements (EARS)

- R1: THE command declaration in `internal/core/commands.go` SHALL carry,
  per verb: allowed phases/statuses (or `any`), exit-code table (code →
  meaning), per-flag metadata (type, allowed enum values, default,
  description), and at least one usage example.
- R2: WHEN a verb is invoked against a spec whose current phase is not in
  the verb's allowed set, THE SYSTEM SHALL fail closed (exit 2) naming the
  verb, the current phase, and the allowed phases — before any side effect.
- R3: WHEN a flag with declared enum values receives a value outside the
  enum, THE SYSTEM SHALL fail closed (exit 2) naming the flag and allowed
  values.
- R4: WHEN a user runs `help --json`, THE SYSTEM SHALL emit the full
  machine-readable palette (verbs, phases, exit codes, flags with enums and
  defaults, examples) as stable JSON.
- R5: THE MCP server (`specd mcp`) SHALL derive tool descriptions and input
  schemas from the same metadata — no independently authored strings.
- R6: Phase-independent verbs (`help`, `version`, `init`, `handshake`,
  `mcp`) SHALL declare `any` explicitly; nothing defaults silently to
  unrestricted.

## Design notes / best practice

- Single source of truth: extend the existing `Command` struct; keep
  declaration-side (`core`) free of handler imports so gates/MCP/help can
  consume it without cycles.
- Enforcement point: dispatch in `internal/cmd` after spec resolution but
  before handler invocation — one choke point, tested once, covers all
  verbs. Verbs that take no spec slug skip the phase check by construction
  (`any`).
- Deriving the phase matrix: read v1's `PhaseCompatibilityMeta` /
  `ModeCompatibilityMeta` under `reference/` as design input only; re-derive
  each verb's set from the current pipeline (requirements → design → tasks →
  execution) rather than copying — several verbs changed shape.
- `help --json` stability: treat output as an API; add a test asserting it
  unmarshals into a versioned Go struct, and include `schemaVersion` in the
  payload so consumers can detect shape changes (pairs with spec 02
  discipline).
- MCP derivation (R5): map flag enums to JSON Schema `enum`, defaults to
  `default`; the conformance test in `internal/integration/` asserts every
  registered verb appears with a description generated from metadata.
- Exit-code table convention: 0 success, 1 gate/verify failure, 2 usage /
  fail-closed. Declare per-verb deviations explicitly; test asserts every
  verb declares at least codes 0 and 2.

## Out of scope

- Role-prompt regeneration from metadata (follow-up; metadata makes it
  possible).
- Changing any verb's actual behavior beyond the new fail-closed checks.

## Acceptance

- Execution verb invoked on a spec still in requirements phase exits 2 with
  named phases, and `.specd/` state is untouched.
- `help --json | jq .` round-trips; MCP tool list matches registry byte-for
  derived content; enum-violating flag value exits 2.
