# Spec 08 — Tasks (Harness Features)

> **Gate:** do not start until W1 + W2 exit gates are green. Reuses Spec 02
> (recovery invariants) and Spec 03 (structured events).

---

## Wave A — Resume UX (Feature 10)

### [ ] W4.1a — `brain resume` subcommand
- **Files:** `internal/cmd/brain.go`
- **Do:** Add `brain resume --session <id>` routing in `RunBrain`. Delegate to
  the existing resume/reconcile path (`ResumeProgramOrchestration` + session
  load + lease reconcile). Support `--json`; use the standard exit-code contract.
- **Done when:** Command runs, resumes a paused/dead session, respects exit
  codes.

### [ ] W4.1b — Resume UX tests
- **Files:** `internal/cmd/brain_resume_test.go` (new)
- **Do:** Test happy-path resume, unknown-session error (correct exit code),
  already-complete session (no-op + clear message). Reuse Spec 02's recording
  runner to assert no double-dispatch.
- **Done when:** All three paths tested; no double-dispatch.

---

## Wave B — Cost brake (Feature 13)

### [ ] W4.4a — Soft/hard brake logic
- **Files:** `internal/core/` (cost/telemetry path), `internal/cmd/brain.go`
- **Do:** Evaluate host-reported cost against `--cost-limit` /
  `--orchestration-cost-limit`: **warn at ≥80%**, **halt at ≥100%**. On halt,
  stop dispatching new workers, exit with a clear code+message, emit Spec-03
  events (`cost_warn` / `cost_halt`).
- **Done when:** Brake transitions implemented and wired into the drive loop.

### [ ] W4.4b — Deterministic cost-brake test
- **Files:** `internal/core/cost_brake_test.go` (new) or `cmd`-level
- **Do:** Feed **synthetic** host cost reports crossing 80% then 100%. Assert:
  warn event at 80%, halt at 100%, **no new dispatch after halt**, correct exit
  code. No live cost dependency.
- **Done when:** Deterministic, green under `-count=2`.

---

## Wave C — Metrics export (Feature 12)

### [ ] W4.3a — Metrics output format
- **Files:** `internal/cmd/report.go` (or `serve.go`), new formatter
- **Do:** Add opt-in output (`--format prometheus` and/or OTLP-JSON) that
  reshapes the Spec-03 event stream + telemetry roll-up into the target shape.
  **Format only — no new dependency, no new transport.**
- **Done when:** Opt-in flag emits valid Prometheus-textfile / OTLP-JSON.

### [ ] W4.3b — Golden-fixture test
- **Files:** `internal/cmd/report_metrics_test.go` (new) + testdata fixture
- **Do:** Run the formatter over a fixed event stream; diff against a checked-in
  golden file. Assert determinism (stable ordering).
- **Done when:** Golden test green; output deterministic.

---

## Wave D — Adapters (Feature 11, ongoing)

### [ ] W4.2a — Document the adapter-addition pattern
- **Files:** `AGENTS.md` / `docs/` (integration section)
- **Do:** Write the "how to add a host adapter" pattern referencing
  `internal/integration`, and document the `--config` snippet fallback as the
  universal path that works for any host without a bespoke adapter.
- **Done when:** Pattern documented; `--config` fallback verified by an existing
  or new test.

---

## Definition of done (Spec 08)
- [ ] `brain resume --session <id>` first-class + tested (incl. no double-dispatch).
- [ ] Cost brake: warn@80 / halt@100, deterministic synthetic-report test.
- [ ] Opt-in metrics format (Prometheus/OTLP-JSON), golden-fixture tested, no new dep.
- [ ] Adapter pattern + `--config` fallback documented.
- [ ] Update `specs/progress.md` W4 + program exit criteria.
