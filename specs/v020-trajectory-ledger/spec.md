# V3 — Trajectory Ledger

## 1. Purpose and requirement coverage

Append-only per-spec tool-event ledger (`trajectory.jsonl`) so trajectory evals
(V5) score *how* the agent worked, not just outputs. Covers plan task **P1.2**
(P0) — the SDLC "trajectory evals" concept, recorded deterministically.

## 2. Verified current state

- Append/fsync discipline: existing ledger IO in `internal/core` (io helpers);
  lock/stress harness (`make stress`) for concurrent-append patterns.
- Producers available today: pinky structured reports
  (`internal/core/pinky_report.go`), MCP server tool dispatch
  (`internal/mcp/tools.go` — sees every tool call), CLI registry
  (`internal/cmd/registry.go`).
- Replay machinery to reuse later: `internal/core/session_replay.go`,
  `replay.go`.

## 3. Proposed design and end-to-end flow

New `internal/core/trajectory.go`: per-spec
`.specd/specs/<slug>/trajectory.jsonl`, O_APPEND + fsync. Event schema:

```json
{"seq": 41, "time": "...", "actor": "pinky|mcp|cli", "tool": "...",
 "args_digest": "sha256:...", "outcome": "ok|error", "task": "t3"}
```

Args are **digested (sha256), never stored raw** — the ledger must not become
a secrets sink. Ordering by monotonic `seq` (per-file counter under the spec
lock), not wall clock. Producers wired: pinky report/progress paths, MCP
middleware around tool dispatch, and new `specd trace append <spec> --tool ...
--outcome ...` for CLI-only hosts (registry + CommandMeta + parity tests).
Line-level validation on read: reject NUL bytes and oversize lines; skip-and-
report corrupt tail lines (crash-recovery: last partial line tolerated).

## 4. Interfaces, contracts, data, configuration, dependencies

- **New artifact:** `trajectory.jsonl` (CLI-owned, append-only) — the read
  contract for V5 trajectory checks and V6 conductor analytics.
- **New command:** `specd trace append` (invariant 10 registry discipline).
- **Dependencies:** V1 (schema v6 landed first; ledger itself is state-free).
  **Dependents:** V5, V6.

## 5. Invariants, security, errors, observability, compatibility, rollback

- Append-only: no rewrite path exists in the binary (invariant 4 dual-write
  discipline extended).
- Secrets hygiene: digest-only args enforced at the type level (no raw-args
  field to misuse); event size cap.
- Concurrency: appends serialized via existing spec lock; stress-tested.
- FakeClock for time fields (invariant 7).
- Compat: absence of the ledger is valid (v0.1.x specs); consumers treat
  missing file as zero events.
- **Rollback:** stop producing; file remains inert evidence.

## 6. Acceptance criteria and validation commands

- Concurrent-append stress test (reuse lock/stress harness) — no lost or
  interleaved lines; `seq` strictly monotonic.
- NUL/oversize-line rejection tests; corrupt-tail recovery test.
- Producer coverage: one e2e per producer (pinky, MCP, `trace append`).
- MCP/CLI parity test for `trace append`.
- `go test ./internal/core/... -run Trajectory -race -count=2 && make stress`

## 7. Open decisions and deviations

- Path deviation DV1 (`internal/core`, not `internal/spec`).
- Open: whether MCP middleware records specd's *own* MCP tools only or all
  host traffic it can see. Decision: only specd tool calls — specd records
  facts it owns; host-wide tracing arrives via `trace append` from the host.
