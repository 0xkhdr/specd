# Spec 03 — Tasks (Observability)

> Prereq: **Spec 01 done** (`worker.Result` carries dur/exit/timeout for events).

---

## Wave A — Logger foundation

### [ ] W2.1 — Central `slog` constructor
- **Files:** `internal/obs/log.go` (new; or `internal/core/logging.go`)
- **Do:** One exported constructor returning a `*slog.Logger` over
  `slog.NewJSONHandler(os.Stderr, …)`. Level parsed from `SPECD_LOG`
  (`debug|info|warn`, default `warn`, unknown→`warn`). **Hard-wire stderr** — no
  caller may pass a writer that could be stdout. Add a package doc note stating
  the stdout-contract rationale.
- **Done when:** Constructor compiles; unit test asserts level mapping +
  unknown-value fallback; handler target is `os.Stderr`.

### [ ] W2.2 — Per-session log file sink
- **Files:** `internal/obs/log.go`, drive setup in `internal/cmd/brain.go`
- **Do:** Add a constructor that fans out to **both** stderr and an append file
  `.specd/sessions/<id>/brain.log` (e.g. `slog` over an `io.MultiWriter`, or two
  handlers). File open is **best-effort**: on failure, warn to stderr and return
  the stderr-only logger.
- **Done when:** A drive writes JSON events to the session file; a forced
  open-failure degrades gracefully (tested).

---

## Wave B — Event emission

### [ ] W2.3 — Emit the six driver events
- **Files:** `internal/cmd/brain.go`, `internal/worker/shell_runner.go`,
  lease-reconcile path in `internal/core/program_orchestration.go`
- **Do:** Emit structured events with the §3.3 schema (`event, session, worker,
  task, dur_ms, exit`):
  - `dispatch` before `Runner.Run`; `complete` after (with `dur_ms`, `exit`).
  - `timeout` when `Result.TimedOut`.
  - `reclaim` / `escalate` from lease reconciliation (aligns with Spec 02).
  - `retry` when a task is re-dispatched under policy.
  Levels: dispatch/complete at `info`, retry/reclaim/escalate/timeout at `info`
  or `warn` (timeout/escalate = `warn`); nothing finer than `debug` for chatter.
- **Done when:** A drive over a fake runner emits all six event types with
  correct fields (asserted by parsing the session log).

---

## Wave C — Stdout contract guard

### [ ] W2.4 — Stdout byte-stability test
- **Files:** `internal/cmd/brain_logging_test.go` (new)
- **Do:** Run a deterministic drive twice — once `SPECD_LOG=` (off), once
  `SPECD_LOG=debug` — capturing stdout separately each time. Assert the two
  stdout captures are **byte-identical**, including a `SPECD_JSON=1` variant.
  Assert stderr is non-empty only in the debug run.
- **Done when:** Test green; intentionally writing a log line to stdout makes it
  fail (verify locally).

---

## Wave D — `brain why` timeline

### [ ] W2.5 — Timeline reader + renderer
- **Files:** `internal/cmd/brain.go` (`brainWhy`), `internal/cmd/brain_test.go`
- **Do:** `brainWhy` reads `.specd/sessions/<id>/brain.log`, parses the NDJSON
  events, and renders a timeline (waves, dispatches, reclaims, retries,
  escalations, timeouts) as a human table by default and structured JSON under
  `--json` (respecting `SPECD_JSON`). Handle missing/empty log with a clear
  message + correct exit code.
- **Done when:** `brain why --session <id>` renders both formats; `brainWhy`
  covered by tests (happy path + missing-log path); coverage 0% → tested.

---

## Definition of done (Spec 03)
- [ ] `SPECD_LOG` level control, default `warn`, stderr-only.
- [ ] Per-session `brain.log`, best-effort.
- [ ] Six event types emitted with stable schema.
- [ ] Stdout byte-stability test green (incl. `SPECD_JSON=1`).
- [ ] `brain why` timeline (human + JSON), tested.
- [ ] Stdlib-only intact; update `specs/progress.md` W2.
