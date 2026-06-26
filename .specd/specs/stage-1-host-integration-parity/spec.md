# Stage 1 — Host Integration Parity

## Goal
Replace the dead Gemini host adapter with an Antigravity CLI adapter, without changing the deterministic host-install contract.

## Knowledge gathered
- `internal/integration/registry.go` still registers `NewGeminiAdapter()` in `DefaultRegistry()`.
- `internal/integration/gemini.go` is a native-CLI adapter that targets `.gemini/settings.json` and shells out to `gemini mcp add ...`.
- `internal/integration/hostutil.go` already has the JSON-merge helpers needed for direct project JSON writes.
- `internal/integration/conformance_test.go` already guards adapter determinism and idempotency.
- TASKS.md defines the required Antigravity facts: binary `agy`, project config `.agents/mcp_config.json`, direct JSON write, and `mcpServers` schema.

## Frozen scope
- Remove Gemini adapter code and tests.
- Add Antigravity adapter code and tests.
- Swap the default registry entry.
- Update conformance coverage.
- Update README and AGENTS references so the new host is documented.

## Requirements
1. Default host discovery must select Antigravity instead of Gemini.
2. Antigravity detection must look for `agy` and `.agents/mcp_config.json`.
3. Installation must write the `mcpServers.specd` JSON entry directly and preserve unrelated keys.
4. Install must be idempotent: a second run must report no change.
5. Gemini code paths must be removed from the registry and tests.
6. Documentation must state that `.agents/` is intentionally VCS-tracked.

## Non-goals
- No compatibility shim for Gemini.
- No change to other adapters.
- No host-side CLI subcommand for Antigravity registration.

## Implementation constraints
- Reuse `installProjectJSON` / `inspectJSONServer` patterns.
- Preserve all existing JSON keys in the target file.
- Do not introduce shell-based JSON writes.
- Keep the adapter scope project-only.

## Done criteria
- `DefaultRegistry().Names()` contains `antigravity` and not `gemini`.
- The new adapter passes idempotency and preserve-existing-keys tests.
- The docs mention `.agents/mcp_config.json` and VCS tracking.
