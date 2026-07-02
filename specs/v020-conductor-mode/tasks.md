# V6 Tasks — Conductor Mode

Plan coverage: P2.1–P2.6. Dependencies: V1, V3 (V4 for HUD tier display).
Dependents: V7, V12.

## Wave 1 — Micro-task schema (P2.1)

- [ ] `tasksparser.go`: optional `micro:` items (`- [ ] m1: ...`), IDs scoped
  to parent task; linear order preserved.
- [ ] Byte-stable round-trip tests incl. all existing fixtures unchanged;
  fuzz extension for micro syntax.
- **Validation:** `go test ./internal/core/... -run 'Tasks|Micro|Fuzz' -race -count=2`

## Wave 2 — Session engine (P2.2, depends on Wave 1)

- [ ] `internal/core/conductor.go`: session open/close in
  `state.json.conductor` + `conductor.jsonl` (append-only, spec lock).
- [ ] `internal/cmd/conductor.go`: `start|step|accept|reject|stop` (reject
  requires `--reason`); completion gated on all-accepted AND `verify:`
  evidence.
- [ ] `conductor switch orchestrated` / `specd mode --set` equivalence;
  transition + reason recorded in decision log.
- [ ] Lifecycle e2e (start→step→reject→step→accept→verify→complete); lock vs
  brain contention test; `--resume` from ledger.
- **Validation:** `go test ./internal/core/... ./internal/cmd/... -run Conductor -race`

## Wave 3 — Live surface (P2.3 + P2.4, depends on Wave 2)

- [ ] SSE: conductor event types on existing endpoint, versioned schema.
- [ ] MCP `specd_conductor` tool sharing core functions; parity test per verb
  (extend `parity_test.go` pattern); HTTP auth untouched.
- [ ] Context HUD: `specd context <spec> --hud` (files/skills/bytes/mode/tier
  from disk only) + SSE event + MCP resource; stability test.
- **Validation:** `go test ./internal/mcp/... ./internal/context/... -run 'Conductor|Parity|Hud' -race`

## Wave 4 — Replay, analytics, host bindings (P2.5 + P2.6, depends on Wave 3)

- [ ] `conductor replay [--session]` via `session_replay.go`; byte-identical
  replay test.
- [ ] `specd report <spec> --conductor`: rejection-reason clustering
  (exact-string + count) from fixtures.
- [ ] `internal/integration`: VS Code tasks.json entries, Claude Code skill
  stub, Cursor/MCP config scaffolds; adapter conformance tests;
  `init --ide <host>` idempotent.
- **Validation:** `go test ./internal/integration/... ./internal/cmd/... -run 'Adapter|Replay|Report' -count=2`

## Rollout & cleanup

- [ ] Docs: agent-integration (SSE schema, MCP tool), command-reference,
  mcp-guide, AGENTS.md conductor discipline, CHANGELOG; parity tests green.
- **Rollback:** switch mode away from conductor; ledgers/micro items inert.
- **Completion evidence:** `make ci` green; mode-switch continuity test
  (<30s, zero context loss metric) recorded.
