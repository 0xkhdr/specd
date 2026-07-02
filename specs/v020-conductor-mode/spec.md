# V6 — Conductor Mode

## 1. Purpose and requirement coverage

Third execution mode for the hands-on "last 20%": micro-task protocol in
`tasks.md`, conductor session engine with an append-only ledger, live SSE/MCP
surface, deterministic context HUD, replay + rejection analytics, and host
adapter bindings. Covers plan tasks **P2.1–P2.6** (P0/P1); architecture §5.1,
§5.2 — the protocol ships here, IDE extensions are consumers, not scope.

## 2. Verified current state

- Mode state machine: `internal/core/mode.go` (`simple|orchestrated`),
  `mode_recommend.go` (deterministic advisory); `internal/cmd/mode.go`.
- Parser: `internal/core/tasksparser.go` (byte-stable round-trip discipline).
- Live surface: `serve.go`, `watch.go`, `watch_sse.go` in `internal/cmd`/core;
  MCP server `internal/mcp/` with CLI parity tests (`parity_test.go` pattern).
- Replay: `session_replay.go`, `replay.go`. Context estimation:
  `internal/context/estimate.go`. Host adapters: `internal/integration/`.
- V1 added `conductor` to the mode enum + `state.json.conductor` block.

## 3. Proposed design and end-to-end flow

- **Micro-tasks (P2.1):** optional `micro:` list items under a task
  (`- [ ] m1: rename x`), IDs `m<N>` scoped to the parent. Linear sequence
  inside one task — DAG untouched (sub-DAG deferred). Specs without `micro:`
  parse byte-identically (golden-compatibility on existing fixtures).
- **Session engine (P2.2):** `internal/core/conductor.go` +
  `internal/cmd/conductor.go`. `start` (requires mode `conductor`, takes spec
  lock, opens session in state + `conductor.jsonl`) · `step` (next micro-task
  brief) · `accept [--evidence]` · `reject --reason` (reason mandatory — it is
  the training signal) · `stop`. Task completes only when all micro-tasks
  accepted **and** the normal evidence gate passes — micro-approval never
  bypasses `verify:`. `conductor switch orchestrated` closes the session and
  records transition + reason in the decision log.
- **Live surface (P2.3):** conductor event types on the existing SSE endpoint
  (versioned); MCP `specd_conductor` tool sharing core functions with the CLI
  (no logic in transport; parity tests per verb).
- **Context HUD (P2.4):** `specd context <spec> --hud` — steering files,
  active skills, byte/approx-token counts, mode/tier; counts from files on
  disk only; also an SSE event + MCP resource.
- **Replay + analytics (P2.5):** `conductor replay` via `session_replay.go`;
  `report --conductor` aggregates rejection reasons (exact-string clustering +
  count — no interpretation).
- **Host bindings (P2.6):** `internal/integration` adapters scaffold VS Code
  `tasks.json` entries, Claude Code skill stub, MCP config — the shipped IDE
  integration; `specd init --ide <host>` idempotent (marker-merge).

## 4. Interfaces, contracts, data, configuration, dependencies

- **New artifact:** `conductor.jsonl` (CLI-owned, append-only).
- **New commands:** `conductor start|step|accept|reject|stop|replay|switch`,
  `context --hud`, `report --conductor` (registry discipline).
- **Stable:** `tasks.md` without `micro:` is byte-identical through the
  parser; DAG semantics unchanged; SSE endpoint/auth unchanged.
- **Dependencies:** V1 (mode enum + conductor block), V3 (ledger pattern; HUD
  tier display uses V4 stamps when present). **Dependents:** V7 (escalation
  hands off to conductor), V12.

## 5. Invariants, security, errors, observability, compatibility, rollback

- Integrity core untouched: acceptance never substitutes for `verify:`
  evidence (invariant 5).
- Spec lock prevents a concurrent brain session on the same spec.
- Ledger append-only, digest discipline shared with V3; replay byte-identical
  for the same ledger.
- Mode switching is always explicit + recorded; the harness never
  auto-switches (human at boundaries).
- HTTP/SSE auth unchanged (security model extends, invariant 8).
- **Rollback:** mode back to `simple|orchestrated`; micro-items remain valid
  parseable Markdown; ledgers inert.

## 6. Acceptance criteria and validation commands

- Parser: `micro:` round-trip byte-stable + fuzz; existing fixtures
  byte-identical.
- Lifecycle e2e: start→step→reject→step→accept→verify→complete; reject
  without `--reason` fails.
- Lock contention test: brain start on conductor-locked spec fails cleanly.
- MCP/CLI parity for every conductor verb; SSE schema documented + versioned.
- Replay byte-identical; rejection report deterministic from fixtures; HUD
  stable across runs.
- Adapter conformance tests extended; `init --ide` idempotent.
- `go test ./internal/core/... ./internal/cmd/... ./internal/mcp/... -run 'Conductor|Micro|Hud' -race -count=2`

## 7. Open decisions and deviations

- Path deviation DV1. Sub-DAG micro-tasks explicitly deferred (plan P2.1).
- Open: session resume after crash mid-step. Decision: `start` on an
  open-session spec offers `--resume` reconstructing from ledger (reuses
  replay) — no silent auto-resume.
