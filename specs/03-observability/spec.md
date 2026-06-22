# Spec 03 — Structured Observability (`slog` + `brain why`)

> Wave: **W2 (P1)** · Priority: **P1** · Source: LEVEL_UP_PLAN §1.2, §2 P1.4/P1.5

## 1. Problem

The harness has **no structured logging**. A grep for `log/slog` in non-test
code finds a single trivial hit in `report.go`; there are only **8**
`fmt.Fprintf(os.Stderr, …)` sites in the whole tree.

An autonomous harness that spawns child processes across waves has **no
field-debuggable trace**. When a worker hangs or a session stalls, the operator
has stdout line-prefixes and nothing else — no leveled log, no per-session log
file, no structured event carrying timestamps, durations, exit codes.

`telemetry.go` is roll-up accounting (operator-annotated cost), **not**
operational tracing. The two are complementary; this spec adds the missing
operational layer.

`brain why` (`cmd/brain.go:631`) exists but at 0% coverage and only prints a
flat header — it cannot surface a session timeline because no structured
timeline is recorded.

## 2. Hard constraint (do not break)

specd's value rests on a **deterministic stdout contract**: `SPECD_JSON=1`
structured output and **byte-stable init receipts**. Logging must be **purely
additive on stderr**. Any byte written to stdout by the logger is a regression.

## 3. Solution

### 3.1 Leveled logger (stdlib `log/slog`, JSON handler — no new dep)

- Central logger constructor in a new small file (e.g.
  `internal/core/logging.go` or `internal/obs/`): `slog.New(slog.NewJSONHandler(
  os.Stderr, &slog.HandlerOptions{Level: …}))`.
- Level from `SPECD_LOG=debug|info|warn` env; **default `warn`** to preserve the
  current quiet UX. Unknown value → `warn`.
- **Route to `os.Stderr` only, always.** Never stdout. A single chokepoint
  constructor makes this auditable.

### 3.2 Per-session log file

- Write the same structured events to `.specd/sessions/<id>/brain.log` (a
  `slog` JSON handler over an append file opened for the session). This persists
  the trace for post-mortem without touching the terminal.
- File is best-effort: if it can't be opened, log a warn to stderr and continue
  (never fail a drive because logging failed).

### 3.3 Event vocabulary

Emit structured events at the orchestration driver boundary (the new
`internal/worker` + `cmd/brain.go` drive loop) with a stable schema:

| field | example | notes |
|---|---|---|
| `event` | `dispatch` | one of: `dispatch`, `reclaim`, `retry`, `escalate`, `timeout`, `complete` |
| `session` | `<id>` | |
| `worker` | `<worker-id>` | |
| `task` | `<task-id>` | |
| `dur_ms` | `1423` | dispatch→complete duration |
| `exit` | `0` | worker exit code (or error class) |

Events fire from the drive loop and from the `Runner.Run` result (durations,
timeouts, exits map cleanly from `worker.Result`). Reclaim/escalate events come
from the lease reconciliation path (ties into Spec 02).

### 3.4 `brain why` / `status` upgrade

- `brainWhy` reads `.specd/sessions/<id>/brain.log`, parses the NDJSON event
  stream, and renders a **session timeline**: waves, decisions, reclaims,
  retries, escalations, timeouts — so an operator diagnoses a stalled session
  without reading raw worker output.
- Support `--json` (emit the parsed timeline as structured JSON on stdout, under
  the existing `SPECD_JSON` discipline) and a human table default.

## 4. Acceptance criteria

- [ ] `SPECD_LOG=debug|info|warn` controls level; default `warn`.
- [ ] **Test proving stdout is byte-unchanged with logging on** (run a drive
      with `SPECD_LOG=debug`, capture stdout, diff against logging-off run —
      must be identical, including `SPECD_JSON=1` receipts).
- [ ] Logs are JSON on **stderr only** + mirrored to `.specd/sessions/<id>/
      brain.log`.
- [ ] All six event types are emitted at the right boundaries with the schema
      in §3.3.
- [ ] `brain why --session <id>` renders the structured timeline (human + JSON);
      `brainWhy` coverage 0% → tested.
- [ ] Stdlib-only invariant intact (no logging dependency added).

## 5. Non-goals

- Metrics export / Prometheus / OTLP — that is **Spec 08** (P3); this spec emits
  the event stream, Spec 08 reshapes it for fleets.
- Reworking `telemetry.go` cost accounting (orthogonal, by design).

## 6. Risks & mitigations

| Risk | Mitigation |
|---|---|
| A stray log to stdout breaks receipts | Single constructor hard-wired to `os.Stderr`; byte-stability test in CI |
| Log volume noise at default | Default `warn`; dispatch/complete at `info`, fine-grained at `debug` |
| Per-session file I/O failure aborts drive | Best-effort open; degrade to stderr-only |
